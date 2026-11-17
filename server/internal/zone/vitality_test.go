package zone

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// ---------------------------------------------------------------------------------------
// Vitalidade e recuperação do 2º personagem (tunnel#150)
// ---------------------------------------------------------------------------------------

// joinVit entra um personagem com a vitalidade vinda do banco.
func joinVit(z *Zone, id uint32, x, zz float64, xp int64, v game.VitalityState) (*entity, *fakePeer) {
	peer := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: id, Name: "V" + string(rune('a'+id)), Class: "knight", X: x, Z: zz, XP: xp, Vitality: v, Peer: peer}})
	return z.players[id], peer
}

// lastVitality: o último VitalityState recebido.
func (f *fakePeer) lastVitality() *pb.VitalityState {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if m := f.msgs[i].GetVitality(); m != nil {
			return m
		}
	}
	return nil
}

func nearXP(a, b int64) bool { return a-b <= 1 && b-a <= 1 }

// No login, enche 1,5% do teto por hora desde a última saída, até o teto.
func TestVitalityFillsOfflineOnJoin(t *testing.T) {
	z := newCombatZone()
	c := game.VitalityCap(5)
	for i, tc := range []struct {
		hours float64
		want  int64
	}{{0, 0}, {10, int64(float64(c) * 0.15)}, {40, int64(float64(c) * 0.6)}, {100, c}} {
		seen := noon.Add(-time.Duration(tc.hours * float64(time.Hour)))
		p, peer := joinVit(z, uint32(i+1), 0, 0, game.XPLevelStart(5), game.VitalityState{LastSeen: seen})
		if p.pl.vit.stored != tc.want {
			t.Fatalf("%v h fora: %d, esperava %d", tc.hours, p.pl.vit.stored, tc.want)
		}
		s := peer.lastVitality()
		if s == nil || s.Stored != uint64(tc.want) || s.Cap != uint64(c) || s.Mult != 2 || s.Mastery || s.CatchupMult != 0 {
			t.Fatalf("estado ao entrar: %+v", s)
		}
		if tc.hours > 0 && !peer.systemContains("Você descansou") {
			t.Fatalf("%v h fora: sem o aviso do descanso", tc.hours)
		}
	}
	// O que havia guardado soma e não passa do teto.
	p, _ := joinVit(z, 9, 0, 0, game.XPLevelStart(5), game.VitalityState{Stored: c - 10, LastSeen: noon.Add(-10 * time.Hour)})
	if p.pl.vit.stored != c {
		t.Fatalf("passou do teto: %d de %d", p.pl.vit.stored, c)
	}
}

// Monstro sai em dobro e o bônus sai do estoque; quando acaba, avisa e volta ao normal.
// Missão não dobra nem gasta. O extra vai para a telemetria como "vitality".
func TestVitalityDoublesMonsterXPAndSpends(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 5, 25, 0))
	z.SetEventSink(nopSink{})
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "knight", 23.5, 0, game.XPLevelStart(5))
	drainEvents(z)
	base := game.MonsterXP(m.mon.def, m.level, p.level)

	p.pl.vit.stored = 1000
	before := p.pl.xp
	z.shareMonsterXP(m, []*entity{p})
	if got := p.pl.xp - before; got != 2*base || p.pl.vit.stored != 1000-base {
		t.Fatalf("com vitalidade: +%d (base %d), sobrou %d", got, base, p.pl.vit.stored)
	}

	p.pl.vit.stored = base / 3
	before = p.pl.xp
	z.shareMonsterXP(m, []*entity{p})
	z.runAfterSnapshot()
	if got := p.pl.xp - before; got != base+base/3 || p.pl.vit.stored != 0 {
		t.Fatalf("fim da vitalidade: +%d (base %d), sobrou %d", got, base, p.pl.vit.stored)
	}
	if !peer.systemContains("Sua vitalidade acabou") || peer.lastVitality().Stored != 0 {
		t.Fatal("o fim da vitalidade tem que avisar no chat e mandar o estado")
	}
	before = p.pl.xp
	z.shareMonsterXP(m, []*entity{p})
	if got := p.pl.xp - before; got != base {
		t.Fatalf("sem vitalidade: +%d, esperava %d", got, base)
	}

	p.pl.vit.stored = 1000
	before = p.pl.xp
	z.grantXPFrom(p, xpQuest, 700)
	z.grantXPFrom(p, xpInvasion, 300)
	if p.pl.xp-before != 1000 || p.pl.vit.stored != 1000 {
		t.Fatalf("missão e invasão não dobram nem gastam: +%d, estoque %d", p.pl.xp-before, p.pl.vit.stored)
	}

	z.removePlayer(p.id)
	got := xpBySource(drainEvents(z))
	if got["vitality"] != base+base/3 || got["monster"] != 3*base || got["quest"] != 700 {
		t.Fatalf("telemetria: %v (base %d)", got, base)
	}
}

// Na cidade (área segura), vivo e fora de combate: enche 1,5% do teto por hora. Fora dela
// ou lutando, não.
func TestVitalityFillsInTownOnly(t *testing.T) {
	z := newCombatZone()
	c := game.VitalityCap(30)
	town, _ := joinAt(z, 1, "knight", 2, 0, game.XPLevelStart(30))
	field, _ := joinAt(z, 2, "knight", 35, 0, game.XPLevelStart(30))
	fight, _ := joinAt(z, 3, "knight", -2, 0, game.XPLevelStart(30))
	for range 3600 / vitalityTownEvery { // uma hora
		fight.lastCombat = z.now
		z.updateVitalityTown()
	}
	if want := int64(float64(c) * game.VitalityTownPerHour); !nearXP(town.pl.vit.stored, want) {
		t.Fatalf("uma hora na cidade: %d, esperava ~%d", town.pl.vit.stored, want)
	}
	if field.pl.vit.stored != 0 || fight.pl.vit.stored != 0 {
		t.Fatalf("fora da cidade (%d) ou em combate (%d) não enche", field.pl.vit.stored, fight.pl.vit.stored)
	}

	// Níveis baixos: o teto é pequeno, mas o que encheu não se perde no arredondamento.
	low, _ := joinAt(z, 4, "knight", 0, 3, game.XPLevelStart(5))
	for range 3600 / vitalityTownEvery {
		z.updateVitalityTown()
	}
	if want := int64(float64(game.VitalityCap(5)) * game.VitalityTownPerHour); !nearXP(low.pl.vit.stored, want) {
		t.Fatalf("nível 5, uma hora na cidade: %d, esperava ~%d", low.pl.vit.stored, want)
	}

	// O sistema roda sozinho no tick (1× a cada 10 s).
	stored := town.pl.vit.stored
	z.tickFor(vitalityTownEvery)
	if town.pl.vit.stored <= stored {
		t.Fatal("o tick não encheu a vitalidade na cidade")
	}
}

// No nível máximo, a vitalidade dobra a XP de monstro na barra de Maestria (e espera o
// bônus diário da Maestria acabar).
func TestVitalityGoesToMasteryAtMaxLevel(t *testing.T) {
	z := newCombatZone(fixed("stone_golem", game.MasteryLevel, 30, 0))
	z.SetEventSink(nopSink{})
	m := z.onlyMonster()
	p, peer := at30(z, 1, "knight", 26, 0)
	if s := peer.lastVitality(); s == nil || !s.Mastery || s.Cap != uint64(bar) {
		t.Fatalf("estado no máximo: %+v", s)
	}
	solo := game.MonsterXP(m.mon.def, m.level, game.MasteryLevel)
	p.pl.vit.stored = bar
	z.shareMonsterXP(m, []*entity{p})
	if st := p.pl.mastery.st; st.XP != 2*solo || p.pl.vit.stored != bar-solo {
		t.Fatalf("barra %d (esperava %d), estoque %d", st.XP, 2*solo, p.pl.vit.stored)
	}

	// Com o bônus diário da Maestria guardado, a vitalidade não gasta.
	p.pl.mastery.st.Rested = bar
	stored := p.pl.vit.stored
	z.shareMonsterXP(m, []*entity{p})
	if p.pl.vit.stored != stored {
		t.Fatalf("gastou junto com o bônus diário: %d → %d", stored, p.pl.vit.stored)
	}
}

// Evento (×2) + recuperação (+50%) + vitalidade (+100%) ficam no teto ×3; a vitalidade só
// paga o que cabe.
func TestMonsterXPCapWithEventCatchupAndVitality(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 5, 25, 0))
	z.SetEventSink(nopSink{})
	m := z.onlyMonster()
	p, peer := joinVit(z, 1, 23.5, 0, game.XPLevelStart(5), game.VitalityState{AccountTop: game.MaxLevel})
	if s := peer.lastVitality(); s.CatchupMult != 1.5 || s.CatchupUntil != game.MaxLevel-game.CatchupGap || s.MultCap != 3 {
		t.Fatalf("recuperação no estado: %+v", s)
	}
	drainEvents(z)
	z.xp2.on = true
	base := game.MonsterXP(m.mon.def, m.level, p.level)

	c := game.VitalityCap(5)
	p.pl.vit.stored = c
	before := p.pl.xp
	z.shareMonsterXP(m, []*entity{p})
	gained := p.pl.xp - before
	if !nearXP(gained, 3*base) {
		t.Fatalf("evento + recuperação + vitalidade: +%d, teto ×3 = %d", gained, 3*base)
	}
	if spent := c - p.pl.vit.stored; !nearXP(spent, base/2) {
		t.Fatalf("a vitalidade pagou %d, cabia %d", spent, base/2)
	}

	// Sem vitalidade: evento + recuperação = ×2,5.
	p.pl.vit.stored = 0
	before = p.pl.xp
	z.shareMonsterXP(m, []*entity{p})
	if got := p.pl.xp - before; !nearXP(got, 5*base/2) {
		t.Fatalf("evento + recuperação: +%d, esperava %d", got, 5*base/2)
	}

	// De noite com o evento (×2,5), a recuperação ocupa o que falta para ×3.
	z.night = true
	p.pl.vit.stored = c
	before = p.pl.xp
	z.shareMonsterXP(m, []*entity{p})
	if got := p.pl.xp - before; float64(got) > 3*float64(base)+1 || p.pl.vit.stored != c {
		t.Fatalf("noite + evento + recuperação: +%d (teto %d), estoque %d", got, 3*base, p.pl.vit.stored)
	}

	z.removePlayer(p.id)
	got := xpBySource(drainEvents(z))
	if got["catchup"] <= 0 || got["vitality"] <= 0 || got["event_bonus"] <= 0 {
		t.Fatalf("telemetria: %v", got)
	}
}

// A recuperação liga com alguém da conta no máximo e desliga a 5 níveis dele.
func TestCatchupTogglesAtGap(t *testing.T) {
	z := newCombatZone()
	z.SetEventSink(nopSink{})
	top := game.VitalityState{AccountTop: game.MaxLevel}
	p, peer := joinVit(z, 1, 0, 0, game.XPLevelStart(34), top)
	if !p.pl.vit.catchup || !peer.systemContains("Bônus de recuperação") {
		t.Fatal("nível 34 com a conta no 40: a recuperação vale")
	}
	before := p.pl.xp
	z.grantXPFrom(p, xpQuest, 1000)
	if p.pl.xp-before != 1500 {
		t.Fatalf("missão com recuperação: +%d", p.pl.xp-before)
	}

	z.grantXP(p, game.XPLevelStart(35)-p.pl.xp)
	z.runAfterSnapshot()
	if p.level != 35 || p.pl.vit.catchup || !peer.systemContains("Bônus de recuperação encerrado") {
		t.Fatalf("no 35 a recuperação acaba (nível %d, ligada %v)", p.level, p.pl.vit.catchup)
	}
	if s := peer.lastVitality(); s.CatchupMult != 0 || s.CatchupUntil != 0 {
		t.Fatalf("estado depois do fim: %+v", s)
	}
	before = p.pl.xp
	z.grantXPFrom(p, xpQuest, 1000)
	if p.pl.xp-before != 1000 {
		t.Fatalf("missão sem recuperação: +%d", p.pl.xp-before)
	}

	for i, tc := range []struct {
		level, top int
		want       bool
	}{{35, 40, false}, {20, 39, false}, {20, 0, false}, {1, 40, true}} {
		q, _ := joinVit(z, uint32(10+i), 0, 0, game.XPLevelStart(tc.level), game.VitalityState{AccountTop: tc.top})
		if q.pl.vit.catchup != tc.want {
			t.Fatalf("nível %d, conta no %d: %v", tc.level, tc.top, q.pl.vit.catchup)
		}
	}
}

// Ao subir de nível o estoque (em XP) fica no teto novo; vai no salvamento e volta no
// relogar rápido (o estado de quem acabou de sair vale mais que o banco).
func TestVitalityLevelUpSaveAndRelog(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "knight", 0, 0, game.XPLevelStart(5))
	c5 := game.VitalityCap(5)
	p.pl.vit.stored = c5
	z.grantXP(p, game.XPLevelStart(6)-p.pl.xp)
	if p.level != 6 || p.pl.vit.stored != min(c5, game.VitalityCap(6)) {
		t.Fatalf("subiu: nível %d, estoque %d", p.level, p.pl.vit.stored)
	}
	if s := peer.lastVitality(); s.Cap != uint64(game.VitalityCap(6)) || s.Stored != uint64(p.pl.vit.stored) {
		t.Fatalf("estado depois de subir: %+v", s)
	}
	if z.savedState(p).Vitality != p.pl.vit.stored {
		t.Fatal("a vitalidade tem que ir no salvamento")
	}

	p.pl.vit.stored = 777
	z.removePlayer(p.id)
	q, _ := joinVit(z, 1, 0, 0, game.XPLevelStart(6), game.VitalityState{Stored: 5, LastSeen: noon.Add(-48 * time.Hour)})
	if q.pl.vit.stored != 777 {
		t.Fatalf("relogar rápido: %d (o banco ainda não tinha o estado novo)", q.pl.vit.stored)
	}
}

func TestGMVitalityCommand(t *testing.T) {
	z := newCombatZone()
	gm, peer := joinAt(z, 1, "knight", 0, 0, game.XPLevelStart(20))
	gm.pl.gm = true
	z.command(gm, "/vitalidade 50")
	if want := int64(math.Round(float64(game.VitalityCap(20)) / 2)); gm.pl.vit.stored != want || peer.lastVitality().Stored != uint64(want) {
		t.Fatalf("/vitalidade 50: %d, esperava %d", gm.pl.vit.stored, want)
	}
	z.command(gm, "/vitalidade 120")
	if !peer.systemContains("Porcentagem inválida") {
		t.Fatal("acima de 100 é recusado")
	}
	other, _ := joinAt(z, 2, "knight", 0, 0, game.XPLevelStart(5))
	other.pl.vit.stored = 10
	z.command(other, "/vitalidade 100")
	if other.pl.vit.stored != 10 {
		t.Fatal("só GM")
	}
}

// Com várias zonas: o login pelo gerente enche o que passou fora do jogo e lê o maior
// nível da conta; o portão leva a vitalidade junto (sem encher e sem repetir avisos); só
// a zona com a cidade enche (a Clareira de Testes não tem).
func TestVitalityAcrossZones(t *testing.T) {
	m, a, b := twoZones(t, nil)
	p := &routedFake{}
	m.Join(context.Background(), JoinInfo{ID: 1, Name: "Ana", Class: "knight", X: 0, Z: -14, AccountID: 1, Peer: p,
		XP: game.XPLevelStart(10), Vitality: game.VitalityState{LastSeen: noon.Add(-10 * time.Hour), AccountTop: game.MaxLevel}})
	pump(m.Zones()...)
	ana := a.players[1]
	if ana == nil {
		t.Fatal("não entrou")
	}
	want := int64(float64(game.VitalityCap(10)) * 0.15)
	if ana.pl.vit.stored != want || !ana.pl.vit.catchup {
		t.Fatalf("login pelo gerente: estoque %d (esperava %d), recuperação %v", ana.pl.vit.stored, want, ana.pl.vit.catchup)
	}

	// Na praça do Vale enche.
	a.updateVitalityTown()
	if ana.pl.vit.stored <= want {
		t.Fatal("a praça do Vale não encheu")
	}
	stored := ana.pl.vit.stored

	useGate(m, p, 1)
	got := b.players[1]
	if got == nil {
		t.Fatal("não atravessou")
	}
	if got.pl.vit.stored != stored || got.pl.vit.top != game.MaxLevel || !got.pl.vit.catchup {
		t.Fatalf("depois do portão: %+v, esperava estoque %d", got.pl.vit, stored)
	}
	p.mu.Lock()
	rested := 0
	for _, msg := range p.msgs {
		if c := msg.GetChat(); c != nil && strings.Contains(c.Text, "descansou") {
			rested++
		}
	}
	p.mu.Unlock()
	if rested != 1 {
		t.Fatalf("o aviso do descanso saiu %d vezes (o portão não repete)", rested)
	}

	// Na clareira não há cidade: parado perto do portão, não enche.
	got.pos = Vec2{}
	for range 360 {
		b.updateVitalityTown()
	}
	if got.pl.vit.stored != stored {
		t.Fatalf("a clareira encheu a vitalidade: %d → %d", stored, got.pl.vit.stored)
	}
}

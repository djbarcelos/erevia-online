package zone

import (
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Chefe de campo (tunnel#132): fases por vida, XP e saque por dano, túmulo e renascimento
// salvo no banco (a janela, em minutos), que sobrevive a um reinício do servidor.

// fieldTestDef: um chefe de teste de 3 fases (a Mãe-Lodo com outro nome e outras janelas).
func fieldTestDef(t *testing.T) *game.MonsterDef {
	t.Helper()
	d := *game.Monsters["mae_lodo"]
	d.ID, d.Name = "test_field", "Chefe de Teste"
	d.FieldBoss = &game.FieldBossDef{RespawnMin: 40, RespawnMax: 60, Place: "no campo de teste", Phases: []game.FieldPhase{
		{AtHP: 0.66, Say: "Fase 2!", Summon: "lodo_menor", SummonCount: 3, ReturnAfter: 30, ReturnHeal: 0.2},
		{AtHP: 0.33, Say: "Fase 3!", Enrage: true},
	}}
	d.Abilities = []game.AbilityDef{
		{ID: "f1", Shape: game.ShapeCircle, Target: game.AimSelf, Radius: 4, Delay: 2, Cooldown: 6, First: 3, Mult: 1},
		{ID: "f2", Shape: game.ShapeCone, Target: game.AimSelf, Radius: 7, Arc: 60, MinPhase: 2, Delay: 1.5, Cooldown: 6, First: 2, Mult: 1},
		{ID: "f3", Shape: game.ShapeLine, Target: game.AimSelf, Length: 10, Width: 2, MinPhase: 3, Delay: 1.5, Cooldown: 6, First: 2, Mult: 1},
	}
	if err := game.ValidateFieldBoss(&d); err != nil {
		t.Fatal(err)
	}
	if err := game.ValidateAbilities(&d); err != nil {
		t.Fatal(err)
	}
	game.Monsters[d.ID] = &d
	t.Cleanup(func() { delete(game.Monsters, d.ID) })
	return &d
}

func newFieldZone(t *testing.T, now *time.Time, saved ...BossData) (*Zone, *entity) {
	t.Helper()
	fieldTestDef(t)
	cfg := testConfig()
	cfg.Spawns = []game.Spawn{fixed("test_field", 20, raidX, raidZ)}
	cfg.SafeRadius = 20
	cfg.Bosses = saved
	cfg.Clock = func() time.Time { return *now }
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	return z, z.onlyMonster()
}

func (z *Zone) fieldTelegraphs(m *entity) (shapes []game.AbilityShape) {
	for _, tg := range z.telegraphs {
		if tg.src == m.id {
			shapes = append(shapes, tg.shape)
		}
	}
	return
}

func TestFieldBossThreePhasesSplitLootAndRespawnAfterRestart(t *testing.T) {
	now := noon
	z, m := newFieldZone(t, &now)
	a1, _ := raider(z, 1, "knight", raidX-4, raidZ)
	a2, _ := raider(z, 2, "mage", raidX-5, raidZ)
	solo, soloPeer := raider(z, 3, "rogue", raidX-4, raidZ+2)
	makeParty(z, a1, a2)
	for _, p := range []*entity{a1, a2, solo} {
		p.stats.MaxHP, p.hp = 1e7, 1e7
	}
	m.mon.hate[a1.id] = 1
	z.think(m)
	r := m.mon.raid
	if r == nil || !r.fighting || r.phase != 1 {
		t.Fatal("o chefe de campo usa o estado do raid e começa na fase 1")
	}

	// Fase 1: só o círculo.
	z.tickFor(3.1)
	if got := z.fieldTelegraphs(m); len(got) != 1 || got[0] != game.ShapeCircle {
		t.Fatalf("fase 1: só o círculo, veio %v", got)
	}

	// Fase 2 (66%): chama 3 crias, avisa e ganha o cone.
	m.hp = m.stats.MaxHP * 0.65
	z.think(m)
	if r.phase != 2 || z.aliveSummons() != 3 || r.retAt <= z.now {
		t.Fatalf("fase 2: phase=%d crias=%d", r.phase, z.aliveSummons())
	}
	z.think(m)
	if z.aliveSummons() != 3 {
		t.Fatal("cada fase invoca uma vez só")
	}
	// Fase 3 (33%): fúria e a linha.
	m.hp = m.stats.MaxHP * 0.32
	z.think(m)
	if r.phase != 3 || !r.enraged || m.stats.AtkInterval >= m.mon.def.Interval {
		t.Fatalf("fase 3: phase=%d fúria=%v", r.phase, r.enraged)
	}
	st := z.bossStatusPB(m)
	if st.Phase != 3 || len(st.PhaseMarks) != 2 || math.Abs(float64(st.PhaseMarks[0])-0.66) > 1e-6 || st.Raid {
		t.Fatalf("o mapa e a barra recebem fase e marcas do chefe de campo: %+v", st)
	}

	// Dano por grupo: o grupo de a1 e a2 tem 60%, o solo 40%.
	z.raidCredit(m, a1, 400)
	z.raidCredit(m, a2, 200)
	z.raidCredit(m, solo, 400)
	xp := map[uint32]int64{}
	for _, p := range []*entity{a1, a2, solo} {
		xp[p.id] = p.pl.xp
	}
	killedAt := now
	z.kill(m, solo) // o último golpe não decide
	z.tickFor(0.1)
	if !m.dead {
		t.Fatal("morreu")
	}
	if a1.pl.xp <= xp[a1.id] || a2.pl.xp <= xp[a2.id] || solo.pl.xp <= xp[solo.id] {
		t.Fatal("quem causou 5% ou mais leva XP")
	}
	if !soloPeer.systemContains("40%") {
		t.Fatal("o jogador sabe a parte dele")
	}
	if len(z.ground) == 0 {
		t.Fatal("o chefe de campo deixa saque")
	}
	for _, g := range z.ground {
		if g.owner != a1.id && g.owner != a2.id {
			t.Fatalf("saque reservado ao grupo que mais bateu: owner=%d", g.owner)
		}
		if math.Abs(g.reservedUntil-z.now-reservedFor) > 0.5 {
			t.Fatalf("reserva de %.1f s, como o abate comum (%.0f s)", g.reservedUntil-z.now, reservedFor)
		}
		if z.canTakeReserved(solo, g) {
			t.Fatal("o outro lado não pega o saque reservado")
		}
	}
	if z.aliveSummons() != 0 {
		t.Fatal("as crias somem com o chefe")
	}

	// Túmulo: hora da morte e quem mais causou dano.
	var tomb *entity
	for _, o := range z.objects {
		tomb = o
	}
	if tomb == nil || tomb.model != "tomb" || tomb.pos != m.pos {
		t.Fatalf("o túmulo aparece no corpo: %+v", tomb)
	}
	wantTime := killedAt.In(brasilia).Format("15:04")
	if !strings.Contains(tomb.obj.title, wantTime) || !strings.Contains(tomb.obj.title, "grupo de "+a1.name) {
		t.Fatalf("dica do túmulo: %q", tomb.obj.title)
	}
	if got := z.entityPB(tomb).Title; got != tomb.obj.title || !strings.Contains(tomb.name, "Chefe de Teste") {
		t.Fatalf("o título vai ao cliente: %q", got)
	}
	z.tickFor(tombSeconds - 20)
	if len(z.objects) != 1 {
		t.Fatal("o túmulo dura 10 min")
	}
	z.tickFor(25)
	if len(z.objects) != 0 {
		t.Fatal("o túmulo some depois de 10 min")
	}

	// Renascimento salvo no banco: de 40 a 60 min, em relógio, e o mapa mostra a janela.
	var saved BossData
	select {
	case saved = <-z.bossJobs:
	default:
		t.Fatal("o renascimento vai para o banco")
	}
	d := saved.RespawnAt.Sub(killedAt)
	if saved.ID != "test_field" || d < 40*time.Minute || d > 60*time.Minute || !saved.KilledAt.Equal(killedAt) || saved.TopName != "grupo de "+a1.name {
		t.Fatalf("salvo: %+v (em %v)", saved, d)
	}
	st = z.bossStatusPB(m)
	if st.Alive || st.WindowStartMs != killedAt.Add(40*time.Minute).UnixMilli() || st.WindowEndMs != killedAt.Add(60*time.Minute).UnixMilli() {
		t.Fatalf("o mapa mostra a janela, não o minuto: %+v", st)
	}

	// Reinício do servidor: a zona nova carrega o salvo e o chefe continua morto.
	now = now.Add(time.Minute)
	z2, m2 := newFieldZone(t, &now, saved)
	if !m2.dead || !m2.mon.hidden || z2.bossList()[0].Alive {
		t.Fatal("depois do reinício o chefe continua morto")
	}
	if st := z2.bossList()[0]; st.WindowStartMs != killedAt.Add(40*time.Minute).UnixMilli() || st.WindowEndMs != killedAt.Add(60*time.Minute).UnixMilli() {
		t.Fatalf("a janela sobrevive ao reinício: %+v", st)
	}
	_, peer := raider(z2, 1, "knight", 30, 0)
	now = saved.RespawnAt.Add(-4 * time.Minute)
	z2.tickFor(0.1)
	if !peer.has(announced(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON)) || !m2.dead {
		t.Fatal("5 min antes, todo o mapa é avisado")
	}
	now = saved.RespawnAt.Add(time.Second)
	z2.tickFor(0.1)
	if m2.dead || m2.hp != m2.stats.MaxHP || !peer.has(announced(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SPAWN)) {
		t.Fatal("renasce dentro da janela, inteiro, com anúncio")
	}
	if m2.mon.raid.phase != 1 || len(m2.mon.raid.damage) != 0 {
		t.Fatal("o chefe novo começa limpo")
	}
}

func TestFieldBossWarnsOnlyFiveMinutesBefore(t *testing.T) {
	now := noon
	z, m := newFieldZone(t, &now, BossData{ID: "test_field", KilledAt: now.Add(-time.Hour), RespawnAt: now.Add(20 * time.Minute)})
	_, peer := raider(z, 1, "knight", 30, 0)
	now = now.Add(10 * time.Minute) // faltam 10
	z.tickFor(0.1)
	if peer.has(announced(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON)) || !m.dead {
		t.Fatal("10 min antes ainda não avisa (isso é do raid)")
	}
	now = now.Add(6 * time.Minute) // faltam 4
	z.tickFor(0.1)
	if !peer.has(announced(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON)) {
		t.Fatal("5 min antes avisa")
	}
}

func TestFieldBossSummonsReturnAndHealUnlessKilled(t *testing.T) {
	for _, killThem := range []bool{false, true} {
		now := noon
		z, m := newFieldZone(t, &now)
		p, _ := raider(z, 1, "knight", raidX-4, raidZ)
		p.stats.MaxHP, p.hp = 1e7, 1e7
		m.mon.hate[p.id] = 1
		z.think(m)
		m.hp = m.stats.MaxHP * 0.6
		z.think(m)
		if z.aliveSummons() != 3 {
			t.Fatal("3 crias")
		}
		if killThem {
			for _, g := range z.monsters {
				if g.mon.summon {
					z.kill(g, p)
				}
			}
		}
		hp := m.hp
		for range 31 { // alguém bate sempre: a luta não reinicia por inatividade
			z.raidCredit(m, p, 1)
			z.tickFor(1)
		}
		if z.aliveSummons() != 0 {
			t.Fatal("as crias que sobram voltam para o chefe")
		}
		healed := m.hp - hp
		if killThem && healed > m.stats.MaxHP*0.001 {
			t.Fatalf("quem matou as crias a tempo não deixa curar: %.0f", healed)
		}
		if !killThem && math.Abs(healed-m.stats.MaxHP*0.2) > m.stats.MaxHP*0.01 {
			t.Fatalf("cura de 20%% da vida, veio %.1f%%", healed/m.stats.MaxHP*100)
		}
		if !m.mon.raid.fighting || m.dead {
			t.Fatal("a luta continua")
		}
	}
}

func TestFieldBossResetClearsPhaseAndAddsNoTomb(t *testing.T) {
	now := noon
	z, m := newFieldZone(t, &now)
	p, _ := raider(z, 1, "knight", raidX-4, raidZ)
	m.mon.hate[p.id] = 1
	z.think(m)
	m.hp = m.stats.MaxHP * 0.3
	z.think(m)
	if m.mon.raid.phase != 3 {
		t.Fatal("fase 3")
	}
	p.dead = true
	z.think(m) // todos morreram: volta para casa
	if r := m.mon.raid; r.phase != 1 || r.fighting || r.enraged || r.retAt != 0 || z.aliveSummons() != 0 {
		t.Fatalf("reiniciar limpa a luta: %+v", r)
	}
	if len(z.objects) != 0 {
		t.Fatal("só a morte deixa túmulo")
	}
	if m.stats.AtkInterval != m.mon.def.Interval {
		t.Fatal("a fúria some")
	}
}

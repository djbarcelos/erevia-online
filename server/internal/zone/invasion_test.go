package zone

import (
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// No mapa de verdade, os invasores nascem livres do lado de fora dos 4 portões e chegam
// à praça pelas estradas.
func TestInvadersMarchThroughTheGatesOnTheRealMap(t *testing.T) {
	w, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.World, cfg.Size, cfg.SafeRadius = w, w.Size, w.SafeRadius
	cfg.Spawns = nil
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	defender(z, 1, "Mestre", 0, -120, 10, true) // longe: ninguém atrapalha a marcha
	say(z, 1, "/evento invasao agora")
	z.tickFor(0.1)
	first := z.invaders()
	if len(first) != len(invGates) {
		t.Fatalf("um por portão: %d", len(first))
	}
	for _, m := range first {
		if !z.walkable(m.pos) || math.Hypot(m.pos.X, m.pos.Z) < w.SafeRadius {
			t.Fatalf("%s nasceu fora do lugar: %v", m.name, m.pos)
		}
	}
	for _, s := range invShelters() {
		if p := z.freeSpot(s); !z.pveSafe(p) || dist(p, s) > 2 {
			t.Fatalf("abrigo %v (livre em %v) não é seguro durante a invasão", s, p)
		}
	}
	z.tickFor(25)
	for _, m := range first {
		if d := dist(m.pos, m.mon.home); d > 2 {
			t.Fatalf("%s não chegou à praça: está em %v, a %.1f m de %v", m.name, m.pos, d, m.mon.home)
		}
		if !z.invasionOpen(m.pos) {
			t.Fatalf("o ponto da praça %v tem que estar na área aberta", m.pos)
		}
	}
}

// newTownZone: a cidade com o raio de verdade (40 m) e sem os monstros do mapa.
func newTownZone() *Zone {
	cfg := testConfig()
	cfg.Spawns = nil
	cfg.SafeRadius = 40
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

// defender entra em (x, z) no nível pedido (gm: pode usar /evento).
func defender(z *Zone, id uint32, name string, x, zz float64, level int, gm bool) (*entity, *fakePeer) {
	peer := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: id, Name: name, Class: "knight", X: x, Z: zz, XP: game.XPLevelStart(level),
		AccountID: int64(id) + 100, GM: gm, Peer: peer}})
	return z.players[id], peer
}

func (f *fakePeer) announces() []*pb.Announce {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pb.Announce
	for _, m := range f.msgs {
		if a := m.GetAnnounce(); a != nil {
			out = append(out, a)
		}
	}
	return out
}

func (f *fakePeer) lastEvents() *pb.EventStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if e := f.msgs[i].GetEventStatus(); e != nil {
			return e
		}
	}
	return nil
}

// invaders: os invasores vivos no mundo.
func (z *Zone) invaders() []*entity {
	var out []*entity
	for _, m := range z.monsters {
		if m.mon.invader && !m.dead {
			out = append(out, m)
		}
	}
	return out
}

func TestInvasionWarnsSendsCappedWavesAndRewardsDefenders(t *testing.T) {
	z := newTownZone()
	// Os dois ficam entre as casas (parte que continua segura) e batem de longe (z.damage).
	// Ana no nível máximo: a recompensa em moedas não muda com uma subida de nível.
	ana, anaPeer := defender(z, 1, "Ana", 20, 20, game.MaxLevel, true)
	beto, _ := defender(z, 2, "Beto", 21, 20, 12, false)
	scroll := game.Items["scroll_ench_weapon"]

	say(z, 1, "/evento invasao")
	if z.inv == nil || z.inv.phase != pb.InvasionPhase_INVASION_PHASE_WARNING {
		t.Fatalf("invasão deveria estar anunciada: %+v", z.inv)
	}
	if a := anaPeer.announces(); len(a) != 1 || a[0].Kind != pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON || !strings.Contains(a[0].Text, "2 minutos") {
		t.Fatalf("aviso 2 min antes = %+v", a)
	}
	if e := anaPeer.lastEvents(); e == nil || e.Invasion != pb.InvasionPhase_INVASION_PHASE_WARNING || e.InvasionAtMs != noon.Add(2*60e9).UnixMilli() {
		t.Fatalf("estado no HUD = %+v", e)
	}
	if len(z.invaders()) != 0 || !z.pveSafe(Vec2{}) {
		t.Fatal("durante o aviso não há esqueletos e a praça continua segura")
	}

	z.tickFor(invWarnBefore - 1)
	if z.inv.phase != pb.InvasionPhase_INVASION_PHASE_WARNING {
		t.Fatal("as ondas só começam depois de 2 minutos")
	}
	z.tickFor(1.1)
	if z.inv.phase != pb.InvasionPhase_INVASION_PHASE_ACTIVE || len(z.invaders()) != len(invGates) {
		t.Fatalf("primeira leva: um por portão, vivos = %d", len(z.invaders()))
	}
	for _, m := range z.invaders() {
		if r := math.Hypot(m.pos.X, m.pos.Z); r < 40 {
			t.Fatalf("invasor nasceu dentro da muralha (r = %.1f)", r)
		}
	}

	// Ninguém mata nada por um tempo: a 1ª onda inteira (12) fica na rua, a 2ª vem depois
	// de invWaveStall e as levas param no limite de vivos.
	z.tickFor(invWaveStall + invWavePause + 20)
	if n := len(z.invaders()); n != invMaxAlive {
		t.Fatalf("vivos = %d, limite %d", n, invMaxAlive)
	}

	// Agora Ana mata tudo o que aparece; Beto só arranha (10% da vida de cada um).
	maxAlive, total := 0, 0
	for range 20 * 60 * 6 {
		maxAlive = max(maxAlive, len(z.invaders()))
		for _, m := range z.invaders() {
			z.damage(beto, m, m.stats.MaxHP*0.1, nil)
			z.damage(ana, m, m.hp+1, nil)
			total++
		}
		z.update(0.05)
		if z.inv == nil {
			break
		}
	}
	if z.inv != nil {
		t.Fatalf("a invasão deveria ter acabado com as ondas: %s", z.invasionText())
	}
	want := 0
	for _, w := range invasionWaves {
		want += w.count
		if w.captain != "" {
			want++
		}
	}
	if total != want || maxAlive > invMaxAlive {
		t.Fatalf("abates %d (esperava %d), máximo de vivos %d", total, want, maxAlive)
	}
	a := anaPeer.announces()
	if last := a[len(a)-1]; last.Kind != pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT || !strings.Contains(last.Text, "Ana") {
		t.Fatalf("anúncio da vitória = %+v", last)
	}
	if e := anaPeer.lastEvents(); e.Invasion != pb.InvasionPhase_INVASION_PHASE_NONE {
		t.Fatalf("HUD depois do fim = %+v", e)
	}
	if !z.pveSafe(Vec2{}) {
		t.Fatal("a praça volta a ser segura no fim")
	}

	// Recompensa: Ana (quem mais fez) leva a cheia e o pergaminho de arma; Beto, a parte dele.
	anaGold := int64(math.Round(invGoldBase + invGoldLevel*float64(ana.level)))
	if ana.pl.inv.gold != anaGold || ana.pl.inv.countOf(scroll.ID) != 1 {
		t.Fatalf("Ana: %d moedas (esperava %d), pergaminhos %d", ana.pl.inv.gold, anaGold, ana.pl.inv.countOf(scroll.ID))
	}
	if beto.pl.inv.gold <= 0 || beto.pl.inv.gold >= anaGold/2 || beto.pl.inv.countOf("scroll_ench_armor") != 1 {
		t.Fatalf("Beto: %d moedas, pergaminhos de armadura %d", beto.pl.inv.gold, beto.pl.inv.countOf("scroll_ench_armor"))
	}
	if _, ok := ana.pl.ach.unlocked["invasion"]; !ok {
		t.Fatal("vitória dá a conquista da invasão")
	}
	if !anaPeer.systemContains("Invasão: ") {
		t.Fatal("cada defensor recebe o resumo da recompensa")
	}
}

func TestInvasionTimeLimitClearsInvadersWithHalfReward(t *testing.T) {
	z := newTownZone()
	ana, anaPeer := defender(z, 1, "Ana", 0, -100, 10, true)
	say(z, 1, "/evento invasao agora")
	z.tickFor(1)
	m := z.invaders()[0]
	z.damage(ana, m, 5, nil)

	z.inv.at = z.now // o tempo limite chegou
	z.tickFor(0.05)
	if z.inv != nil || len(z.invaders()) != 0 {
		t.Fatalf("tempo esgotado: os invasores somem (vivos %d)", len(z.invaders()))
	}
	if !m.gone || z.monsters[m.id] != nil {
		t.Fatal("invasor vivo sai do mundo (Despawn pela AOI)")
	}
	a := anaPeer.announces()
	if last := a[len(a)-1]; !strings.Contains(last.Title, "recuam") {
		t.Fatalf("anúncio do fim por tempo = %+v", last)
	}
	if want := int64(math.Round((invGoldBase + invGoldLevel*10) * invLoseMult)); ana.pl.inv.gold != want {
		t.Fatalf("metade da recompensa: %d moedas, esperava %d", ana.pl.inv.gold, want)
	}
	if ana.pl.inv.countOf("scroll_ench_weapon") != 0 {
		t.Fatal("sem itens quando o tempo acaba")
	}
	if _, ok := ana.pl.ach.unlocked["invasion"]; ok {
		t.Fatal("a conquista é só da vitória")
	}
}

func TestInvasionOpensOnlyRoadsAndPlazaForMonsters(t *testing.T) {
	z := newTownZone()
	ana, _ := defender(z, 1, "Ana", 0, 3, 12, true)
	beto, _ := defender(z, 2, "Beto", 0, 4, 12, false)
	open := []Vec2{{}, {X: 30}, {X: -35, Z: 2}, {Z: 36}, {X: 9, Z: -9}}
	closed := []Vec2{{X: 20, Z: 20}, {X: 25, Z: 10}, {X: -15, Z: 28}}

	for _, p := range append(open, closed...) {
		if !z.pveSafe(p) {
			t.Fatalf("sem invasão a cidade inteira é segura (%v)", p)
		}
	}
	say(z, 1, "/evento invasao agora")
	for _, p := range open {
		if z.pveSafe(p) {
			t.Fatalf("durante a invasão, estradas e praça abertas (%v)", p)
		}
	}
	for _, p := range closed {
		if !z.pveSafe(p) {
			t.Fatalf("o resto da cidade continua seguro (%v)", p)
		}
	}

	// Sem PvP: a cidade continua protegida entre jogadores.
	if ok, why, _ := z.canHit(ana, beto, true); ok || !strings.Contains(why, "cidade") {
		t.Fatalf("PvP na cidade durante a invasão: ok=%v %q", ok, why)
	}

	// Um esqueleto na praça ataca Ana, e Ana pode bater nele.
	z.tickFor(0.1)
	m := z.invaders()[0]
	m.teleportTo(Vec2{X: 0, Z: 4.5})
	z.tickFor(4)
	if m.mon.hate[ana.id] == 0 && m.mon.hate[beto.id] == 0 {
		t.Fatal("o invasor deveria ter achado alguém na praça")
	}
	z.handle(attackEvent{id: ana.id, target: m.id})
	hpBefore := m.hp
	z.tickFor(3)
	if m.hp >= hpBefore && !m.dead {
		t.Fatal("Ana deveria conseguir bater no invasor na praça")
	}

	// Morrer para um invasor não custa XP.
	xp := beto.pl.xp
	if !beto.dead {
		z.kill(beto, m)
	}
	if beto.pl.xp != xp {
		t.Fatalf("morte na invasão custou XP: %d → %d", xp, beto.pl.xp)
	}
	// E ele volta num canto seguro, não no meio da luta na praça.
	z.respawnPlayer(beto)
	if !z.pveSafe(beto.pos) || !z.inSafe(beto.pos) {
		t.Fatalf("renasceu em %v, fora do abrigo", beto.pos)
	}

	say(z, 1, "/evento invasao parar")
	if z.inv != nil || !z.pveSafe(Vec2{}) || len(z.invaders()) != 0 {
		t.Fatal("parar fecha a cidade e tira os invasores")
	}
}

func TestInvasionAtNightfallFollowsTheChance(t *testing.T) {
	for _, chance := range []float64{0, 1} {
		now := noon
		z := newClockZone(&now)
		z.cfg.InvasionChance = chance
		z.cfg.SafeRadius = 40
		join(z, 1, "Ana")
		now = midnight
		z.tickFor(0.05)
		if got := z.inv != nil; got != (chance == 1) {
			t.Fatalf("chance %.0f: invasão = %v", chance, got)
		}
	}
	// Ninguém online: não há invasão.
	now := noon
	z := newClockZone(&now)
	z.cfg.InvasionChance = 1
	now = midnight
	z.tickFor(0.05)
	if z.inv != nil {
		t.Fatal("sem jogadores, sem invasão")
	}
}

func TestInvasionCommandIsGMOnlyAndLogged(t *testing.T) {
	z := newTownZone()
	mod := newFakeMod()
	z.SetModBackend(mod)
	_, gm := defender(z, 1, "Mestre", 0, 0, 5, true)
	_, beto := defender(z, 2, "Beto", 0, 0, 5, false)

	say(z, 2, "/evento invasao")
	if z.inv != nil || !strings.Contains(beto.lastSystem(), "Comando desconhecido") {
		t.Fatalf("só GM: %q", beto.lastSystem())
	}
	say(z, 1, "/evento invasao")
	drainMod(z)
	if z.inv == nil || len(mod.logs) != 1 || mod.logs[0].Command != "evento" || mod.logs[0].Target != "invasao" {
		t.Fatalf("gm_log = %+v", mod.logs)
	}
	say(z, 1, "/evento invasao")
	if !strings.Contains(gm.lastSystem(), "Já há uma invasão") {
		t.Fatalf("segunda invasão: %q", gm.lastSystem())
	}
}

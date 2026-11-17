package zone

import (
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// regionPoints: os monstros da região que existem no período (sem o chefe de campo).
func regionPoints(points []game.SpawnPoint, region string, night bool) []game.SpawnPoint {
	var out []game.SpawnPoint
	for _, p := range points {
		if p.Region == region && !p.Monster.Boss && p.When.Active(night) {
			out = append(out, p)
		}
	}
	return out
}

// peak70: o maior número de monstros dentro de um raio de 70 m (a área de interesse do cliente), com o centro do círculo em
// qualquer ponto de uma grade de 5 m em volta da região.
func peak70(ms []game.SpawnPoint, minX, maxX, minZ, maxZ float64) int {
	peak := 0
	for x := minX - 40; x <= maxX+40; x += 5 {
		for z := minZ - 40; z <= maxZ+40; z += 5 {
			n := 0
			for _, p := range ms {
				if math.Hypot(p.X-x, p.Z-z) <= 70 {
					n++
				}
			}
			peak = max(peak, n)
		}
	}
	return peak
}

// O Bosque Gris (tunnel#137) cabe no orçamento do roadmap: 16 monstros ao mesmo tempo (de dia e de noite), do 7 ao 15, o pico
// de monstros num raio de 70 m (a área de interesse do cliente) fica em 14 ou menos, todos nascem em chão livre no setor
// noroeste, e os esqueletos dos anéis saem do setor sem mudar de id.
func TestBosqueBudget(t *testing.T) {
	w, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 12; seed++ {
		points := game.ExpandSpawns(game.DefaultSpawns(), rand.New(rand.NewPCG(seed, seed*7+3)))
		for _, night := range []bool{false, true} {
			bosque := regionPoints(points, game.BosqueRegion, night)
			if len(bosque) != 16 {
				t.Fatalf("seed %d, noite %v: %d monstros no Bosque Gris, quero 16", seed, night, len(bosque))
			}
			for _, p := range bosque {
				r, a := math.Hypot(p.X, p.Z), game.AngleDeg(p.X, p.Z)
				if r < 80-12 || r > 150+12 || a < 100-8 || a > 170+8 {
					t.Errorf("%s em (%.0f, %.0f) fora do setor do Bosque (r %.0f, %.0f°)", p.Monster.ID, p.X, p.Z, r, a)
				}
				if p.Level < 7 || p.Level > 15 {
					t.Errorf("%s no nível %d", p.Monster.ID, p.Level)
				}
				if f := w.NearestFree(world.Point{X: p.X, Z: p.Z}); math.Hypot(f.X-p.X, f.Z-p.Z) > 3 {
					t.Errorf("seed %d: %s em (%.0f, %.0f) preso em obstáculos (livre a %.1f m)", seed, p.Monster.ID, p.X, p.Z, math.Hypot(f.X-p.X, f.Z-p.Z))
				}
			}
			if peak := peak70(bosque, -150, -10, 10, 150); peak > 14 {
				t.Errorf("seed %d, noite %v: pico de %d monstros num raio de 70 m (o teto é 14)", seed, night, peak)
			}
		}
		// Nenhum esqueleto do anel (sem Region) sorteia dentro do setor.
		sector := game.Spawn{RMin: 80, RMax: 212, AngleMin: 100, AngleMax: 170}
		for _, p := range points {
			if p.Region == "" && !p.Monster.Raid && sector.Contains(p.X, p.Z, 0) {
				t.Fatalf("seed %d: %s do anel dentro do setor do Bosque (%.0f, %.0f)", seed, p.Monster.ID, p.X, p.Z)
			}
		}
	}
}

// As Terras Mortas: 14 monstros ao mesmo tempo, do 24 ao 28, pico de 14 ou menos num raio de 70 m (o covil não conta) e
// todos em chão livre, fora do covil do Ozrhal, do Bastião, da Torre e da lápide.
func TestTerrasMortasBudget(t *testing.T) {
	w, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	away := []struct {
		name string
		x, z float64
		m    float64
	}{
		{"covil do Ozrhal", game.RaidLairX, game.RaidLairZ, game.RaidLairRadius + 6}, {"Bastião Cinzento", game.BastionX, game.BastionZ, 22},
		{"Torre do Sino", game.BellX, game.BellZ, 16}, {"Lápide", game.TombstoneX, game.TombstoneZ, 12},
	}
	for seed := uint64(1); seed <= 12; seed++ {
		points := game.ExpandSpawns(game.DefaultSpawns(), rand.New(rand.NewPCG(seed, seed*7+3)))
		for _, night := range []bool{false, true} {
			mortas := regionPoints(points, game.TerrasMortasRegion, night)
			if len(mortas) != 14 {
				t.Fatalf("seed %d, noite %v: %d monstros nas Terras Mortas, quero 14", seed, night, len(mortas))
			}
			for _, p := range mortas {
				if p.Level < 24 || p.Level > 28 {
					t.Errorf("%s no nível %d", p.Monster.ID, p.Level)
				}
				if p.X < 120 || p.X > 270 || p.Z < 30 || p.Z > 150 {
					t.Errorf("%s em (%.0f, %.0f) fora das Terras Mortas", p.Monster.ID, p.X, p.Z)
				}
				for _, a := range away {
					if math.Hypot(p.X-a.x, p.Z-a.z) < a.m {
						t.Errorf("%s em (%.0f, %.0f) a menos de %.0f m de %s", p.Monster.ID, p.X, p.Z, a.m, a.name)
					}
				}
				if f := w.NearestFree(world.Point{X: p.X, Z: p.Z}); math.Hypot(f.X-p.X, f.Z-p.Z) > 3 {
					t.Errorf("seed %d: %s em (%.0f, %.0f) preso em obstáculos (livre a %.1f m)", seed, p.Monster.ID, p.X, p.Z, math.Hypot(f.X-p.X, f.Z-p.Z))
				}
			}
			if peak := peak70(mortas, 120, 270, 30, 150); peak > 14 {
				t.Errorf("seed %d, noite %v: pico de %d monstros num raio de 70 m (o teto é 14)", seed, night, peak)
			}
		}
	}
}

// Os lugares das duas regiões estão em chão livre: a Toca do Corvo, o Bastião, a Torre, a lápide e os dois contadores de história.
func TestBosqueAndMortasPlaces(t *testing.T) {
	w, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]world.Point{
		"Toca do Corvo": {X: game.CrowDenX, Z: game.CrowDenZ}, "Bastião": {X: game.BastionX, Z: game.BastionZ},
		"Torre do Sino": {X: game.BellX - 6, Z: game.BellZ - 6}, "Lápide": {X: game.TombstoneX, Z: game.TombstoneZ},
		"Hélio": {X: -37, Z: 94}, "Caedmon": {X: 199, Z: 93},
	} {
		if f := w.NearestFree(p); math.Hypot(f.X-p.X, f.Z-p.Z) > 2 {
			t.Errorf("%s em (%.0f, %.0f): chão livre só a %.1f m", name, p.X, p.Z, math.Hypot(f.X-p.X, f.Z-p.Z))
		}
	}
	// A Toca do Corvo e o Bastião se alcançam a pé a partir da cidade.
	for name, p := range map[string]world.Point{"Toca do Corvo": {X: game.CrowDenX, Z: game.CrowDenZ}, "Bastião": {X: game.BastionX, Z: game.BastionZ}} {
		path := w.FindPath(world.Point{X: 0, Z: 0}, p)
		if n := len(path); n == 0 || math.Hypot(path[n-1].X-p.X, path[n-1].Z-p.Z) > 4 {
			t.Errorf("sem caminho a pé da cidade até %s", name)
		}
	}
}

// O Bastião Cinzento é uma pedra do mapa principal (nas Terras Mortas), que se descobre a pé: não é mais uma pedra futura do
// mapa das Minas. Os ids dos destinos de antes não mudam (o id é a posição + 1).
func TestBastionStoneIsOnTheMainMap(t *testing.T) {
	i := stoneIndex("bastiao")
	if i != len(teleportStones)-1 {
		t.Fatalf("o Bastião é a última pedra do mapa principal: índice %d de %d", i, len(teleportStones))
	}
	s := allStones[i]
	if s.stoneMap() != world.StarterMap || s.mapID != "" || s.region != game.TerrasMortasRegion || s.price != 260 || s.ferry || s.town {
		t.Errorf("pedra do Bastião: %+v", s)
	}
	if math.Hypot(s.x-game.BastionX, s.z-game.BastionZ) > 8 {
		t.Errorf("a pedra fica no pátio do Bastião: (%.0f, %.0f)", s.x, s.z)
	}
	for _, f := range futureStones {
		if f.key == "bastiao" {
			t.Error("o Bastião não é mais uma pedra futura")
		}
	}
	if allStones[len(teleportStones)].key != "lanterna" {
		t.Errorf("as pedras futuras seguem na mesma ordem: %q", allStones[len(teleportStones)].key)
	}
	z := newMapZone(t)
	if n := z.stoneAt(i); n == nil || n.name != "Bastião Cinzento" || n.npc.title != "Pedra de Teleporte" {
		t.Fatalf("a zona do mapa principal cria a pedra do Bastião: %+v", n)
	}
}

// A Lápide do Último Regicida diz o nome de quem mais causou dano na última morte do Ozrhal (BossData.TopName), e o Irmão
// Caedmon também. Sem nome, a pedra está em branco.
// newMapZone: a zona do mapa principal como o servidor monta (os limites do mapa vão até as Terras Mortas, x 330).
func newMapZone(t *testing.T) *Zone {
	t.Helper()
	cfg, err := MapConfig(testConfig(), world.StarterMap, []string{world.StarterMap, MapTest})
	if err != nil {
		t.Fatal(err)
	}
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func TestTombstoneShowsLastRegicide(t *testing.T) {
	z := newMapZone(t)
	tomb := npcNamed(z, game.TalkTombstone)
	if tomb == nil || tomb.model != "lapide" || tomb.npc.talk == nil || tomb.npc.role != pb.NpcRole_NPC_ROLE_QUEST {
		t.Fatalf("a Lápide é um NPC de conversa: %+v", tomb)
	}
	p, peer, n := nearNPC(t, z, 1, game.TalkTombstone, 30)
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	if d := peer.lastDialog(); d == nil || !strings.Contains(d.Text, "ninguém, ainda") || strings.Contains(d.Text, game.TokenRegicide) {
		t.Fatalf("sem nome, a pedra está em branco: %+v", d)
	}
	z.regicide = "grupo de Rovena"
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	if d := peer.lastDialog(); d == nil || !strings.Contains(d.Text, "grupo de Rovena") {
		t.Fatalf("a Lápide mostra o último regicida: %+v", d)
	}
	// O Irmão Caedmon responde a mesma coisa.
	p2, peer2, caedmon := nearNPC(t, z, 2, game.TalkBrother, 28)
	npcTalk(z, p2, caedmon, pb.NpcTalkKind_NPC_TALK_ASK, "", game.TopicRegicide)
	if d := peer2.lastDialog(); d == nil || !strings.Contains(d.Text, "grupo de Rovena") {
		t.Fatalf("Caedmon diz o último regicida: %+v", d)
	}
}

// O nome vem do banco ao ligar o servidor (BossData.TopName do Ozrhal) e muda quando ele cai de novo.
func TestRegicideLoadsFromTheSavedBoss(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	z, _ := newRaidZone(&now, BossData{ID: "ozrhal", TopName: "clã Ossário", KilledAt: now.Add(-5 * time.Hour), RespawnAt: now.Add(-time.Hour)})
	if z.regicide != "clã Ossário" {
		t.Fatalf("regicida carregado = %q", z.regicide)
	}
	if got := z.fillTokens("foi " + game.TokenRegicide + "."); got != "foi clã Ossário." {
		t.Errorf("texto = %q", got)
	}
	z2, _ := newRaidZone(&now)
	if z2.regicide != "" || z2.fillTokens(game.TokenRegicide) != "ninguém, ainda" {
		t.Errorf("sem banco, a pedra está em branco: %q", z2.regicide)
	}
}

// A luta com o Corvo Rubro (chefe de campo de 3 fases): a Bomba de Fumaça na fase 1, o apito e a Chuva de Facas na 2 e o Salto
// (a marca que segue um jogador, nunca o tanque) na 3.
func TestCrowRubroFightPhases(t *testing.T) {
	cfg := testConfig()
	cfg.Spawns = []game.Spawn{fixed("corvo_rubro", 16, raidX, raidZ)}
	cfg.SafeRadius = 20
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	m := z.onlyMonster()
	tank, _ := raider(z, 1, "knight", raidX-4, raidZ)
	mate, matePeer := raider(z, 2, "mage", raidX-5, raidZ+1)
	for _, p := range []*entity{tank, mate} {
		p.stats.MaxHP, p.hp = 1e7, 1e7
		m.mon.hate[p.id] = 1
	}
	m.act.target = tank.id
	z.think(m)
	r := m.mon.raid
	if r == nil || !r.fighting || r.phase != 1 || r.nextWave != 0 && false {
		t.Fatal("o Corvo Rubro é um chefe de campo: começa a luta na fase 1")
	}
	shapes := func() (out []game.AbilityShape, marks int) {
		for _, tg := range z.telegraphs {
			if tg.src != m.id {
				continue
			}
			out = append(out, tg.shape)
			if tg.follow != 0 {
				marks++
			}
		}
		return
	}
	// Fase 1: a Bomba de Fumaça (círculo de 5 m, 1,5 s de aviso), a 5 s da luta.
	z.tickFor(5.1)
	if got, _ := shapes(); len(got) != 1 || got[0] != game.ShapeCircle {
		t.Fatalf("fase 1: só a bomba de fumaça, veio %v", got)
	}
	// Fase 2 (60%): apita e chama 4 Bandoleiros; a Chuva de Facas (cone) vem 3 s depois.
	m.hp = m.stats.MaxHP * 0.59
	z.think(m)
	if r.phase != 2 || z.aliveSummons() != 4 {
		t.Fatalf("fase 2: phase=%d, %d Bandoleiros", r.phase, z.aliveSummons())
	}
	for _, g := range z.monsters {
		if g.mon.summon && g.mon.def.ID != "bandoleiro_corvo" {
			t.Errorf("o apito chama Bandoleiros: %s", g.mon.def.ID)
		}
	}
	z.tickFor(3.2)
	cone := false
	for _, s := range func() []game.AbilityShape { g, _ := shapes(); return g }() {
		cone = cone || s == game.ShapeCone
	}
	if !cone {
		t.Fatal("fase 2: a Chuva de Facas é um cone")
	}
	// Fase 3 (30%): o Salto marca um jogador que não é o tanque, e o aviso segue o marcado.
	m.hp = m.stats.MaxHP * 0.29
	z.think(m)
	if r.phase != 3 {
		t.Fatalf("fase 3: phase=%d", r.phase)
	}
	z.tickFor(4.2)
	if _, marks := shapes(); marks == 0 || !matePeer.systemContains("saltar") {
		t.Fatalf("fase 3: a marca do Salto segue o jogador que não é o tanque (marcas %d)", marks)
	}
	if st := z.bossStatusPB(m); st.Phase != 3 || len(st.PhaseMarks) != 2 || st.Raid {
		t.Fatalf("o mapa e a barra recebem as marcas das fases: %+v", st)
	}
}

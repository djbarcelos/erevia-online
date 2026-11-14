package zone

import (
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
)

// rareZone: zona de teste com os spawns pedidos e a chance dos raros forçada a 1.
func rareZone(now *time.Time, spawns ...game.Spawn) *Zone {
	z := newClockZone(now, spawns...)
	z.rareSetup().chance = func(*game.RareDef) float64 { return 1 }
	return z
}

// respawn mata o monstro (sem dono) e espera ele renascer.
func (z *Zone) respawn(t *testing.T, m *entity) {
	t.Helper()
	for range 3 { // o Ossudo levanta uma vez antes de morrer
		if m.dead {
			break
		}
		z.kill(m, nil)
	}
	if !m.dead {
		t.Fatal("o monstro não morreu")
	}
	z.tickFor(2*m.mon.spawn.Respawn + 1)
	if m.dead {
		t.Fatal("o monstro não renasceu")
	}
}

func isRare(m *entity, id string) bool { return m.mon.def.ID == id && m.mon.def.Rare != nil }

func TestRareSpawnsInPlaceOfTheHostAndWaits45Minutes(t *testing.T) {
	now := noon
	z := rareZone(&now, fixed("skeleton_minion", 3, 60, 0)) // Arredores (r 46–74)
	m := z.onlyMonster()
	id, home := m.id, m.mon.home
	host := game.Monsters["skeleton_minion"]
	if m.mon.def != host {
		t.Fatal("a zona começa com o hospedeiro, não com o raro")
	}

	z.respawn(t, m)
	if !isRare(m, "rare_ossudo") {
		t.Fatalf("com chance 1, renasceu como %s", m.mon.def.ID)
	}
	if m.id != id || m.mon.home != home || z.monsters[id] != m {
		t.Fatal("o raro devia nascer no lugar do hospedeiro (mesmo id e mesma casa)")
	}
	// Nível do hospedeiro +2, nunca abaixo do 6 da tabela; vida ×4; 1,2× maior.
	if m.level != 6 {
		t.Errorf("nível %d, esperava 6", m.level)
	}
	if want := 4 * game.MonsterStats(host, 6).MaxHP; m.stats.MaxHP != want || m.hp != want {
		t.Errorf("vida %v/%v, esperava %v (×4)", m.hp, m.stats.MaxHP, want)
	}
	if m.scale != host.Scale*game.RareScale || m.name != "Ossudo, o Teimoso" {
		t.Errorf("escala %v e nome %q", m.scale, m.name)
	}
	if e := z.entityPB(m); !e.Rare || e.Name != "Ossudo, o Teimoso" {
		t.Errorf("Entity.rare = %v, nome %q", e.Rare, e.Name)
	}

	z.respawn(t, m)
	if m.mon.def != host || m.name != host.Name || m.level != 3 || m.scale != host.Scale || z.entityPB(m).Rare {
		t.Fatalf("depois do raro, o hospedeiro volta (veio %s nível %d)", m.mon.def.ID, m.level)
	}
	ended := now
	now = ended.Add(44 * time.Minute)
	z.respawn(t, m)
	if m.mon.def != host {
		t.Fatal("o raro nasceu de novo antes dos 45 min")
	}
	now = ended.Add(game.RareCooldown + time.Second)
	z.respawn(t, m)
	if !isRare(m, "rare_ossudo") {
		t.Fatal("passados os 45 min, o raro devia poder nascer de novo")
	}
}

func TestRareConditionsNightAndPartOfTheArea(t *testing.T) {
	now := noon
	campos := fixed("skeleton_warrior", 10, 0, 95) // Campos dos Ossos (r 80–108)
	ermos := fixed("skeleton_warrior", 16, 0, 130) // Ermos: Guerreiro, mas fora da área do Capitão
	z := rareZone(&now, campos, ermos)
	var c, e *entity
	for _, m := range z.monsters {
		if m.mon.home.Z > 120 {
			e = m
		} else {
			c = m
		}
	}
	z.respawn(t, c)
	if c.mon.def.Rare != nil {
		t.Fatal("o Capitão Ossudo só nasce à noite")
	}
	now = midnight
	z.tickFor(0.1)
	z.respawn(t, c)
	z.respawn(t, e)
	if !isRare(c, "rare_captain") || c.level != 14 {
		t.Fatalf("à noite nos Campos, esperava o Capitão de nível 14 (veio %s, %d)", c.mon.def.ID, c.level)
	}
	if e.mon.def.Rare != nil {
		t.Fatal("o Guerreiro dos Ermos não é hospedeiro do Capitão")
	}
	// Amanhecer, fora de combate: ele vai embora e volta o Guerreiro.
	now = noon.Add(game.DayCycle)
	z.tickFor(1.1)
	if !c.dead {
		t.Fatal("o raro noturno devia sumir no amanhecer")
	}
	z.tickFor(2*c.mon.spawn.Respawn + 1)
	if c.dead || c.mon.def.ID != "skeleton_warrior" {
		t.Fatalf("depois do amanhecer, esperava o Guerreiro (veio %s, morto %v)", c.mon.def.ID, c.dead)
	}

	// Ômrun: só na metade sul do Vale.
	south := game.Rares[3]
	if south.ID != "rare_omrun" || !south.Where.Contains(0, -300) || south.Where.Contains(0, -250) || south.Where.Contains(0, -330) {
		t.Fatal("Ômrun: só na metade sul do disco do Vale")
	}
	vossk := game.Rares[2]
	if !vossk.Where.Contains(-128, 5) || vossk.Where.Contains(128, 5) || vossk.Where.Contains(-100, 5) {
		t.Fatal("Vossk: só no oeste dos Ermos")
	}
}

func TestDormantRareRespectsTheCooldown(t *testing.T) {
	now := midnight
	sp := fixed("skeleton_minion", 3, 60, 0)
	sp.When = game.WhenNight // hospedeiro noturno: dorme no amanhecer
	z := rareZone(&now, sp)
	m := z.onlyMonster()
	z.respawn(t, m)
	if !isRare(m, "rare_ossudo") {
		t.Fatal("esperava o raro")
	}
	dawn := noon.Add(game.DayCycle)
	now = dawn
	z.tickFor(1.1)
	if !m.mon.dormant {
		t.Fatal("o raro devia adormecer com o hospedeiro no amanhecer")
	}
	if _, alive := z.rares.alive["rare_ossudo"]; alive {
		t.Fatal("a aparição devia acabar quando ele adormece")
	}
	now = midnight.Add(game.DayCycle) // 20 min depois: acorda, mas como o hospedeiro
	z.tickFor(1.1)
	if m.dead || m.mon.def.Rare != nil {
		t.Fatalf("acordou como %s (morto %v); esperava o hospedeiro", m.mon.def.ID, m.dead)
	}
	z.respawn(t, m)
	if m.mon.def.Rare != nil {
		t.Fatal("renasceu raro 20 min depois de adormecer")
	}
	now = midnight.Add(2 * game.DayCycle) // 60 min depois do fim
	z.tickFor(0.1)
	z.respawn(t, m)
	if !isRare(m, "rare_ossudo") {
		t.Fatal("passados os 45 min, esperava o raro")
	}
}

func TestRareCooldownSurvivesRestart(t *testing.T) {
	now := noon
	z := rareZone(&now, fixed("skeleton_minion", 3, 60, 0))
	m := z.onlyMonster()
	z.respawn(t, m)
	var saved BossData
	for len(z.bossJobs) > 0 {
		saved = <-z.bossJobs
	}
	if saved.ID != "rare:rare_ossudo" || !saved.RespawnAt.Equal(now.Add(game.RareCooldown)) {
		t.Fatalf("salvou %+v; esperava rare:rare_ossudo para daqui a 45 min", saved)
	}
	// O servidor reinicia 10 min depois: o raro ainda espera.
	now = now.Add(10 * time.Minute)
	cfg := testConfig()
	cfg.Spawns = []game.Spawn{fixed("skeleton_minion", 3, 60, 0)}
	cfg.SafeRadius = 20
	cfg.Clock = func() time.Time { return now }
	cfg.Bosses = []BossData{saved}
	z2 := newZoneWith(cfg)
	z2.rareSetup().chance = func(*game.RareDef) float64 { return 1 }
	m2 := z2.onlyMonster()
	z2.respawn(t, m2)
	if m2.mon.def.Rare != nil {
		t.Fatal("reiniciar zerou o mínimo de 45 min")
	}
	now = saved.RespawnAt
	z2.respawn(t, m2)
	if !isRare(m2, "rare_ossudo") {
		t.Fatal("na hora salva, o raro devia poder nascer")
	}
}

func TestRareRumorGuaranteedLootAndRise(t *testing.T) {
	now := noon
	z := rareZone(&now, fixed("skeleton_minion", 3, 60, 0))
	m := z.onlyMonster()
	near, nearPeer := joinAt(z, 1, "knight", 60, 30, game.XPLevelStart(20)) // a 30 m
	_, farPeer := joinAt(z, 2, "knight", -60, 0, 0)                         // a 120 m
	z.respawn(t, m)
	heard := func(f *fakePeer) bool {
		for _, c := range f.chats() {
			if strings.Contains(c.Text, "Algo grande se move no bosque") {
				return true
			}
		}
		return false
	}
	if !heard(nearPeer) || heard(farPeer) {
		t.Fatalf("aviso: perto %v, longe %v; esperava só quem está a até 60 m", heard(nearPeer), heard(farPeer))
	}

	// Mata com o jogador: levanta uma vez com 30% da vida, depois morre e deixa o saque.
	m.mon.hate[near.id] = 1
	z.damage(near, m, m.hp+1, nil)
	if m.dead || m.hp != m.stats.MaxHP*0.3 {
		t.Fatalf("Ossudo devia levantar com 30%% da vida (morto %v, vida %v)", m.dead, m.hp)
	}
	z.damage(near, m, m.hp+1, nil)
	if !m.dead {
		t.Fatal("na segunda vez ele morre")
	}
	skulls := 0
	for _, g := range z.ground {
		if g.def.ID == "skull_cracked" && g.owner == near.id {
			skulls += g.count
		}
	}
	if skulls < 3 {
		t.Fatalf("saque garantido: %d Crânios Rachados, esperava pelo menos 3", skulls)
	}
	if _, alive := z.rares.alive["rare_ossudo"]; alive {
		t.Fatal("a aparição devia acabar na morte")
	}
	if n := near.pl.ach.counts["rare_hunter"]; n != 1 {
		t.Fatalf("Caçador de Raros: progresso %b, esperava o bit do Ossudo", n)
	}
}

func TestRareHunterNeedsTheFourDifferentRares(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "knight", 0, 0, 0)
	for range 4 {
		z.achEvent(p, game.AchKill, "rare_ossudo")
	}
	if _, ok := p.pl.ach.unlocked["rare_hunter"]; ok {
		t.Fatal("matar o mesmo raro 4 vezes não vale")
	}
	if info := z.achInfo(p, game.Achievements["rare_hunter"], false); info.Count != 1 || info.Need != 4 {
		t.Fatalf("progresso %d/%d, esperava 1/4", info.Count, info.Need)
	}
	for _, id := range []string{"rare_captain", "rare_vossk", "rare_omrun"} {
		z.achEvent(p, game.AchKill, id)
	}
	if _, ok := p.pl.ach.unlocked["rare_hunter"]; !ok {
		t.Fatal("os 4 raros diferentes desbloqueiam o Caçador de Raros")
	}
}

func TestCaptainCallsHisShieldedGuards(t *testing.T) {
	now := midnight
	z := rareZone(&now, fixed("skeleton_warrior", 10, 0, 95))
	c := z.onlyMonster()
	z.respawn(t, c)
	if !isRare(c, "rare_captain") {
		t.Fatal("esperava o Capitão")
	}
	p, _ := joinAt(z, 1, "knight", 0, 92, game.XPLevelStart(25))
	c.mon.hate[p.id] = 1
	c.hp = c.stats.MaxHP * 0.45
	z.tickFor(1.1)
	guards := z.rares.guards[c.id]
	if len(guards) != 2 {
		t.Fatalf("%d escudeiros, esperava 2", len(guards))
	}
	z.tickFor(0.2)
	for _, id := range guards {
		g := z.monsters[id]
		if g == nil || !g.mon.summon || !g.has(game.EffectShield, z.now) {
			t.Fatal("o escudeiro chega com escudo e não renasce")
		}
	}
	z.tickFor(2)
	if len(z.rares.guards[c.id]) != 2 {
		t.Fatal("a Formação é uma vez por luta")
	}
}

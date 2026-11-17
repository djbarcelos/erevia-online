package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Setor de exemplo (o formato do Bosque Gris): os mesmos 6 lugares, de dia ladinos e à
// noite magos.
func periodSector(monster string, when game.When) game.Spawn {
	return game.Spawn{Monster: monster, Count: 6, RMin: 80, RMax: 150, LevelMin: 7, LevelMax: 15, Respawn: 5,
		AngleMin: 100, AngleMax: 170, Region: "Bosque de Teste", When: when}
}

// inWorld: vivo e com o corpo no mundo (o que a AOI mostra).
func inWorld(m *entity) bool { return !m.dead && !m.mon.hidden }

// countWorld conta os monstros no mundo por período, e quantos estão fora de combate.
func (z *Zone) countWorld() (byWhen map[game.When]int, calm int) {
	byWhen = map[game.When]int{}
	for _, m := range z.monsters {
		if !inWorld(m) {
			continue
		}
		byWhen[m.mon.spawn.When]++
		if !z.monsterFighting(m) {
			calm++
		}
	}
	return byWhen, calm
}

func TestNightSpawnOnlyExistsAtNight(t *testing.T) {
	now := noon
	z := newClockZone(&now, periodSector("skeleton_rogue", game.WhenDay), periodSector("skeleton_mage", game.WhenNight),
		fixed("skeleton_minion", 3, 30, 0))
	check := func(when string, day, night int) {
		t.Helper()
		got, _ := z.countWorld()
		if got[game.WhenDay] != day || got[game.WhenNight] != night || got[game.WhenAny] != 1 {
			t.Fatalf("%s: %d do dia, %d da noite e %d de sempre no mundo; esperava %d, %d e 1",
				when, got[game.WhenDay], got[game.WhenNight], got[game.WhenAny], day, night)
		}
	}
	check("meio-dia", 6, 0)
	z.tickFor(60) // o dormant não renasce sozinho
	check("meio-dia, 1 min depois", 6, 0)

	now = midnight
	z.tickFor(0.1)
	check("meia-noite", 0, 6)
	for _, m := range z.monsters {
		if m.mon.spawn.When == game.WhenNight && m.mon.spawn.Region != "Bosque de Teste" {
			t.Fatalf("o noturno perdeu a região: %+v", m.mon.spawn)
		}
	}
	z.tickFor(60)
	check("meia-noite, 1 min depois", 0, 6)

	now = noon.Add(game.DayCycle) // o meio-dia do ciclo seguinte
	z.tickFor(0.1)
	check("amanhecer", 6, 0)

	// A zona que já começa à noite.
	now = midnight
	z = newClockZone(&now, periodSector("skeleton_rogue", game.WhenDay), periodSector("skeleton_mage", game.WhenNight),
		fixed("skeleton_minion", 3, 30, 0))
	check("zona criada à noite", 0, 6)
}

func TestOutOfCombatCountIsTheSameDayAndNight(t *testing.T) {
	now := noon
	spawns := append(game.DefaultSpawns(), periodSector("skeleton_rogue", game.WhenDay), periodSector("skeleton_mage", game.WhenNight))
	z := newClockZone(&now, spawns...)
	_, dayCalm := z.countWorld()
	now = midnight
	z.tickFor(1.1)
	_, nightCalm := z.countWorld()
	now = noon.Add(game.DayCycle)
	z.tickFor(1.1)
	_, dawnCalm := z.countWorld()
	if dayCalm != nightCalm || dayCalm != dawnCalm || dayCalm == 0 {
		t.Fatalf("monstros fora de combate: %d de dia, %d à noite e %d no amanhecer", dayCalm, nightCalm, dawnCalm)
	}
}

func TestFightingMonsterStaysUntilTheFightEnds(t *testing.T) {
	now := noon
	day := fixed("skeleton_warrior", 1, 30, 0) // agressivo: briga com quem chegar perto
	day.When = game.WhenDay
	night := fixed("skeleton_mage", 1, -60, 0) // longe do jogador
	night.When = game.WhenNight
	calm := fixed("skeleton_minion", 1, 0, 60) // um do dia, longe da luta
	calm.When = game.WhenDay
	calmNight := fixed("skeleton_minion", 1, 0, 64)
	calmNight.When = game.WhenNight
	z := newClockZone(&now, day, night, calm, calmNight)
	var fighter, nightMon, calmMon *entity
	for _, m := range z.monsters {
		switch {
		case m.mon.def.ID == "skeleton_warrior":
			fighter = m
		case m.mon.def.ID == "skeleton_mage":
			nightMon = m
		case m.mon.spawn.When == game.WhenDay:
			calmMon = m
		}
	}
	p, _ := joinAt(z, 1, "knight", 31, 0, game.XPLevelStart(30))
	z.tickFor(1)
	if !z.monsterFighting(fighter) {
		t.Fatal("o guerreiro devia estar lutando com o jogador")
	}

	now = midnight
	z.tickFor(1.1)
	if !inWorld(fighter) || fighter.mon.dormant {
		t.Fatal("o monstro em combate sumiu no meio da luta")
	}
	if inWorld(calmMon) {
		t.Fatal("o monstro do dia fora de combate devia ter saído à noite")
	}
	// 2 do dia e 2 da noite: o calmo saiu e um noturno entrou; o outro espera a vaga do
	// que está lutando (senão somaria um monstro).
	got, _ := z.countWorld()
	if got[game.WhenDay] != 1 || got[game.WhenNight] != 1 {
		t.Fatalf("na virada: %v, esperava 1 do dia (lutando) e 1 da noite", got)
	}
	if !inWorld(nightMon) {
		t.Fatal("entra primeiro o noturno de menor id")
	}
	z.tickFor(10)
	if !inWorld(fighter) {
		t.Fatal("o monstro saiu com a luta ainda acontecendo")
	}

	// A luta acaba (o jogador vai embora): ele sai, sem morrer, e o noturno entra no lugar.
	ground := len(z.ground)
	z.removePlayer(p.id)
	z.tickFor(outOfCombatAfter + 1.5)
	if inWorld(fighter) || !fighter.mon.dormant {
		t.Fatal("acabou a luta e o monstro do dia continua no mundo à noite")
	}
	if len(z.ground) != ground {
		t.Fatal("sair à noite não é morrer: não deixa saque")
	}
	got, _ = z.countWorld()
	if got[game.WhenDay] != 0 || got[game.WhenNight] != 2 {
		t.Fatalf("depois da luta: %v", got)
	}
}

func TestMonsterKilledOutsideItsPeriodWaitsForIt(t *testing.T) {
	now := noon
	day := fixed("skeleton_warrior", 1, 30, 0)
	day.When = game.WhenDay
	night := fixed("skeleton_mage", 1, -60, 0) // longe do jogador
	night.When = game.WhenNight
	z := newClockZone(&now, day, night)
	var fighter, nightMon *entity
	for _, m := range z.monsters {
		if m.mon.spawn.When == game.WhenDay {
			fighter = m
		} else {
			nightMon = m
		}
	}
	p, _ := joinAt(z, 1, "knight", 31, 0, game.XPLevelStart(30))
	z.tickFor(1)
	now = midnight
	z.tickFor(1.1)
	if !inWorld(fighter) || inWorld(nightMon) {
		t.Fatal("na virada, o do dia em luta fica e o da noite espera")
	}
	z.kill(fighter, p) // morreu à noite: a vaga abre
	z.tickFor(1.1)
	if !inWorld(nightMon) {
		t.Fatal("o noturno não entrou quando o do dia morreu")
	}
	z.tickFor(3 * day.Respawn)
	if inWorld(fighter) || !fighter.mon.dormant {
		t.Fatal("o monstro do dia renasceu à noite")
	}
	now = noon.Add(game.DayCycle)
	z.tickFor(0.1)
	if !inWorld(fighter) || inWorld(nightMon) {
		t.Fatal("no amanhecer, o do dia volta e o da noite sai")
	}
}

// Quem está perto vê a troca: o Despawn do monstro do dia e o Spawn do da noite.
func TestPeriodSwapReachesNearbyPlayers(t *testing.T) {
	now := noon
	day := fixed("skeleton_minion", 1, 30, 0) // não agressivo: não entra em combate sozinho
	day.When = game.WhenDay
	night := fixed("skeleton_minion", 1, 30, 6)
	night.When = game.WhenNight
	z := newClockZone(&now, day, night)
	var dayMon, nightMon *entity
	for _, m := range z.monsters {
		if m.mon.spawn.When == game.WhenDay {
			dayMon = m
		} else {
			nightMon = m
		}
	}
	_, peer := joinAt(z, 1, "knight", 26, 3, game.XPLevelStart(30))
	z.tickFor(0.5)
	if peer.spawned(dayMon.id) == 0 || peer.spawned(nightMon.id) != 0 {
		t.Fatal("de dia, o jogador vê só o monstro do dia")
	}
	now = midnight
	z.tickFor(0.5)
	if !peer.despawned(dayMon.id) || peer.spawned(nightMon.id) == 0 {
		t.Fatal("à noite, o jogador devia receber o Despawn do monstro do dia e o Spawn do da noite")
	}
}

// Monstro que vira dormente com um aviso no chão pendente: o aviso é cancelado (o cliente
// recebe o cancelamento), o golpe não cai e, quando ele volta, as habilidades esperam o
// First de novo.
func TestDormantMonsterCancelsPendingWarnings(t *testing.T) {
	now := noon
	golem := fixed("stone_golem", 25, 40, 0)
	golem.When = game.WhenDay
	z := newClockZone(&now, golem)
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "knight", m.pos.X+2.5, m.pos.Z, 0)
	atLevel(z, p, 25)
	m.mon.hate[p.id] = 1
	slam := m.mon.def.Abilities[0]
	z.tickFor(slam.First + 0.1)
	tg := z.pendingFrom(m)
	if tg == nil {
		t.Fatal("o golem devia estar preparando o golpe")
	}
	gen := m.mon.abilGen
	z.putToSleep(m) // o caminho da troca de período
	if m.mon.abilGen == gen || m.mon.abilFight {
		t.Fatal("dormir não parou as habilidades")
	}
	z.tickFor(slam.Delay + 0.5)
	if z.pendingFrom(m) != nil || abilityHits(peer, p) != 0 {
		t.Fatal("o aviso de um monstro dormente ficou pendurado ou o golpe caiu")
	}
	if !peer.has(func(msg *pb.ServerMessage) bool {
		for _, w := range msg.GetSnapshot().GetTelegraphs() {
			if w.Id == tg.id && w.Cancelled {
				return true
			}
		}
		return false
	}) {
		t.Fatal("o cliente não recebeu o cancelamento do aviso")
	}

	// Volta (o período dele de novo): a luta nova espera o First.
	m.mon.dormant = false
	z.reviveMonster(m)
	m.mon.hate[p.id] = 1
	z.tickFor(1)
	if z.pendingFrom(m) != nil {
		t.Fatal("ao voltar, as habilidades começam do First")
	}
}

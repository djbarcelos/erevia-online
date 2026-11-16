package zone

import (
	"io"
	"log/slog"
	"math"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Um teste por arquétipo de IA (tunnel#129), mais o neutro, o neutro de dia e o papel que
// vai para o cliente.

// tough: o jogador aguenta a luta inteira (os testes medem o monstro, não a morte).
func tough(p *entity) { p.stats.MaxHP, p.hp = 1e7, 1e7 }

// monstersOf: os monstros vivos da def id.
func (z *Zone) monstersOf(id string) []*entity {
	var out []*entity
	for _, m := range z.monsters {
		if m.mon.def.ID == id && !m.dead {
			out = append(out, m)
		}
	}
	return out
}

// fixedSlow: spawn fixo que demora a renascer (o teste não vê o monstro voltar).
func fixedSlow(monster string, level int, x, z float64) game.Spawn {
	s := fixed(monster, level, x, z)
	s.Respawn = 600
	return s
}

// killMonster: p mata m de uma vez (como o /raid matar). A XP pode subir o nível de p, que
// refaz os atributos: p continua aguentando a luta.
func (z *Zone) killMonster(p, m *entity) {
	m.mon.hate[p.id] += m.hp
	z.damage(p, m, m.hp+1, nil)
	tough(p)
}

func TestArchetypeBruteIsTheDefault(t *testing.T) {
	// Nenhum monstro que já existia mudou de papel sem querer.
	for _, id := range []string{"skeleton_minion", "skeleton_warrior", "skeleton_rogue", "skeleton_mage", "stone_golem",
		"bone_lord", "ozrhal", "bone_guard", "ancient_golem", "ossuary_knight", "ossuary_devotee", "broken_reaper",
		"broken_arcanist", "rare_ossudo", "rare_captain", "rare_vossk", "rare_omrun", game.RareGuardID} {
		d := game.Monsters[id]
		if d == nil {
			t.Fatalf("%s sumiu", id)
		}
		if d.Archetype != game.ArchBrute || d.Neutral || d.NeutralByDay || d.HasRole() || rolePB(d) != nil {
			t.Errorf("%s ganhou papel: %v neutro=%v/%v", id, d.Archetype, d.Neutral, d.NeutralByDay)
		}
	}
	// E o bruto pensa como antes: o guerreiro persegue, bate e não recua nem se esconde.
	z := newCombatZone(fixed("skeleton_warrior", 8, 40, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 35, 0, 0)
	tough(p)
	z.tickFor(2)
	if a := m.mon.arch; m.act.target != p.id || a.retreating || a.lurking || a.fused || a.summonFight || len(a.pack) > 0 {
		t.Fatalf("o bruto ataca quem chega perto: act=%+v arch=%+v", m.act, a)
	}
}

func TestArcherKeepsRangeAndRetreatsOnceEvery6s(t *testing.T) {
	z := newCombatZone(fixed("skeleton_thrower", 8, 40, 0))
	m := z.onlyMonster()
	prm := m.mon.def.Arch
	if prm.KeepMin != 7 || prm.KeepMax != 10 || prm.Retreat != 5 || prm.RetreatCD != 6 || m.stats.Range != prm.KeepMax {
		t.Fatalf("números do atirador: %+v (alcance %.1f)", prm, m.stats.Range)
	}
	p, peer := joinAt(z, 1, "knight", 47.5, 0, 0) // 7,5 m: dentro do alcance, fora do recuo
	tough(p)
	z.tickFor(3)
	if len(peer.combatFrom(m.id)) == 0 {
		t.Fatal("o atirador ataca de longe")
	}
	if m.mon.arch.retreating || math.Abs(m.pos.X-40) > 0.1 {
		t.Fatalf("a 7,5 m ele não anda: pos=%v", m.pos)
	}

	// O jogador encosta: recua ~5 m para longe dele.
	before := m.pos
	p.teleportTo(Vec2{X: m.pos.X - 1.5, Z: 0})
	z.tickFor(0.1)
	if !m.mon.arch.retreating {
		t.Fatal("o alvo encostou: o atirador recua")
	}
	first := z.now
	start := m.pos
	z.tickFor(2)
	if moved := dist(m.pos, before); moved < 4.5 || moved > 5.5 || m.pos.X < start.X {
		t.Fatalf("recuou %.1f m (de %v para %v), quero ~5 m para longe do jogador", moved, before, m.pos)
	}

	// Colado nele o tempo todo: recua de novo só 6 s depois de cada recuo.
	starts := []float64{first}
	was := false
	for range int(14 * float64(z.cfg.TickRate)) {
		if !m.mon.arch.retreating {
			p.teleportTo(Vec2{X: m.pos.X - 1.5, Z: m.pos.Z})
		}
		z.update(1 / float64(z.cfg.TickRate))
		if m.mon.arch.retreating && !was {
			starts = append(starts, z.now)
		}
		was = m.mon.arch.retreating
	}
	if len(starts) != 3 {
		t.Fatalf("recuos em 16 s colado: %v, quero 3 (um a cada 6 s)", starts)
	}
	for i := 1; i < len(starts); i++ {
		if gap := starts[i] - starts[i-1]; gap < prm.RetreatCD-0.06 || gap > prm.RetreatCD+0.11 {
			t.Fatalf("recuos %.2f s um do outro (%v), quero 6", gap, starts)
		}
	}
	if hits := len(peer.combatFrom(m.id)); hits < 4 {
		t.Fatalf("entre os recuos ele briga (golpes = %d)", hits)
	}
}

func TestHealerHealsWoundedAllyEvery8s(t *testing.T) {
	z := newCombatZone(fixed("moss_golem", 25, 40, 0), fixed("stone_golem", 25, 46, 0))
	h, g := z.monstersOf("moss_golem")[0], z.monstersOf("stone_golem")[0]
	prm := h.mon.def.Arch
	if prm.HealBelow != 0.5 || prm.HealFrac != 0.15 || prm.HealCD != 8 {
		t.Fatalf("números do curandeiro: %+v", prm)
	}
	// Acima de 50%: nada.
	g.hp = g.stats.MaxHP * 0.6
	g.lastCombat = z.now // em combate: sem a regeneração de fora de combate
	z.tickFor(1)
	if g.hp != g.stats.MaxHP*0.6 {
		t.Fatalf("aliado com 60%% não é curado: %.0f/%.0f", g.hp, g.stats.MaxHP)
	}
	// Abaixo de 50%: +15% da vida máxima.
	g.hp = g.stats.MaxHP * 0.4
	z.tickFor(0.6)
	if want := g.stats.MaxHP*0.4 + math.Round(g.stats.MaxHP*0.15); math.Abs(g.hp-want) > 0.01 {
		t.Fatalf("cura: vida %.1f, quero %.1f", g.hp, want)
	}
	// Recarga de 8 s entre duas curas.
	healAt := func() float64 {
		g.hp = g.stats.MaxHP * 0.3
		for range int(12 * float64(z.cfg.TickRate)) {
			g.lastCombat = z.now
			z.update(1 / float64(z.cfg.TickRate))
			if g.hp > g.stats.MaxHP*0.3 {
				return z.now
			}
		}
		t.Fatal("sem cura em 12 s")
		return 0
	}
	t1 := healAt()
	if t2 := healAt(); math.Abs(t2-t1-prm.HealCD) > 0.06 {
		t.Fatalf("curas %.2f s uma da outra, quero %.0f", t2-t1, prm.HealCD)
	}
	// Ele mesmo não conta: mate o curandeiro primeiro.
	g.hp = g.stats.MaxHP
	h.hp = h.stats.MaxHP * 0.2
	for range int(9 * float64(z.cfg.TickRate)) {
		h.lastCombat = z.now
		z.update(1 / float64(z.cfg.TickRate))
	}
	if h.hp > h.stats.MaxHP*0.21 {
		t.Fatalf("o curandeiro não se cura: %.0f/%.0f", h.hp, h.stats.MaxHP)
	}
}

func TestSummonerCallsTwoEvery20sUpToFourAndTheyDieWithHim(t *testing.T) {
	z := newCombatZone(fixedSlow("skeleton_necromancer", 16, 40, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 45, 0, 0)
	tough(p)
	prm := m.mon.def.Arch
	if prm.SummonCount != 2 || prm.SummonMax != 4 || prm.SummonCD != 20 {
		t.Fatalf("números do invocador: %+v", prm)
	}
	minions := func() []*entity { return z.monstersOf(prm.Summon) }
	z.tickFor(prm.SummonFirst + 0.5)
	got := minions()
	if len(got) != 2 {
		t.Fatalf("a luta começou: 2 lacaios, veio %d", len(got))
	}
	for _, g := range got {
		if !g.mon.summon || g.mon.arch.master != m.id || g.level != m.level-summonLevels || g.mon.hate[p.id] == 0 {
			t.Fatalf("lacaio: invocação %v, mestre %d, nível %d, ódio %v", g.mon.summon, g.mon.arch.master, g.level, g.mon.hate)
		}
		if g.id < summonIDBase {
			t.Fatal("o id do lacaio vem do espaço das invocações (summon do raid)")
		}
	}
	z.tickFor(20)
	if n := len(minions()); n != 4 {
		t.Fatalf("20 s depois: 4 lacaios, veio %d", n)
	}
	z.tickFor(20)
	if n := len(minions()); n != 4 {
		t.Fatalf("no máximo 4 vivos, veio %d", n)
	}
	// Morreu um: o próximo chamado repõe só o que falta.
	z.killMonster(p, minions()[0])
	z.tickFor(20)
	if n := len(minions()); n != 4 {
		t.Fatalf("repõe até 4, veio %d", n)
	}
	// O invocador morre: os lacaios morrem com ele, sem saque, e somem com o corpo.
	z.killMonster(p, m)
	z.tickFor(0.1)
	if n := len(minions()); n != 0 {
		t.Fatalf("os lacaios morrem com o invocador: %d vivos", n)
	}
	z.tickFor(corpseSeconds + 1)
	for _, e := range z.monsters {
		if e.mon.def.ID == prm.Summon {
			t.Fatal("os corpos dos lacaios somem (não renascem)")
		}
	}
}

func TestAmbusherHidesUntilCloseAndHitsHarder(t *testing.T) {
	z := newCombatZone(fixed("skeleton_lurker", 8, 40, 0))
	m := z.onlyMonster()
	if !m.mon.arch.lurking || !m.invisible(z.now) || aoiVisible(m) {
		t.Fatal("o emboscador nasce escondido (invisível e fora da AOI)")
	}
	p, peer := joinAt(z, 1, "knight", 47, 0, 0) // 7 m: um bruto agressivo já viria
	tough(p)
	z.tickFor(2)
	if !m.mon.arch.lurking || len(m.mon.hate) > 0 || peer.spawned(m.id) > 0 {
		t.Fatal("a 7 m o emboscador continua escondido e o cliente nem sabe dele")
	}
	// Não dá para mirar o que não se vê.
	z.handle(attackEvent{id: p.id, target: m.id})
	z.tickFor(0.2)
	if p.act.kind != actNone {
		t.Fatal("o jogador não ataca um monstro escondido")
	}
	if m.mon.def.Arch.AmbushRange < 4 || m.mon.def.Arch.AmbushRange > 5 {
		t.Fatalf("alcance da emboscada %.1f m, quero 4–5", m.mon.def.Arch.AmbushRange)
	}

	// Chegou a 4,2 m: aparece, briga com ele e o primeiro golpe dói 50% a mais.
	p.teleportTo(Vec2{X: 44.2, Z: 0})
	z.tickFor(0.05)
	if m.mon.arch.lurking || m.invisible(z.now) || m.mon.hate[p.id] == 0 || peer.spawned(m.id) != 1 {
		t.Fatalf("perto, ele aparece (escondido=%v, ódio=%v, spawns=%d)", m.mon.arch.lurking, m.mon.hate, peer.spawned(m.id))
	}
	if !m.mon.arch.ambushHit {
		t.Fatal("o golpe da emboscada fica pendente até o primeiro golpe")
	}
	base := z.hitInput(m, p, false, 1, 0)
	if m.mon.arch.ambushHit {
		t.Fatal("o golpe da emboscada só vale uma vez")
	}
	m.mon.arch.ambushHit = true // hitInput acima gastou o bônus; o golpe de verdade é o próximo
	z.tickFor(2)
	hits := peer.combatFrom(m.id)
	if len(hits) == 0 || m.mon.arch.ambushHit {
		t.Fatal("o emboscador bate e gasta o bônus no primeiro golpe")
	}
	if after := z.hitInput(m, p, false, 1, 0); math.Abs(base.Mult/after.Mult-1.5) > 1e-9 {
		t.Fatalf("primeiro golpe ×%.2f do normal, quero ×1,5", base.Mult/after.Mult)
	}

	// O jogador foge para a cidade: o emboscador volta a se esconder quando a luta esfria.
	p.teleportTo(Vec2{})
	z.tickFor(outOfCombatAfter + 4)
	if !m.mon.arch.lurking || !peer.despawned(m.id) {
		t.Fatalf("fora de combate ele some de novo (escondido=%v)", m.mon.arch.lurking)
	}
}

func TestPackPullsTheWholeSpawnGroup(t *testing.T) {
	pack := func(x float64, n int) game.Spawn {
		return game.Spawn{Monster: "skeleton_raider", Count: n, Area: true, X: x, Z: 0, Radius: 2, LevelMin: 9, LevelMax: 9, Respawn: 30}
	}
	z := newCombatZone(pack(40, 3), pack(70, 2))
	var near, far []*entity
	for _, m := range z.monsters {
		if m.pos.X < 55 {
			near = append(near, m)
		} else {
			far = append(far, m)
		}
	}
	if len(near) != 3 || len(far) != 2 || len(near[0].mon.arch.pack) != 2 || len(far[0].mon.arch.pack) != 1 {
		t.Fatalf("matilhas: %d e %d, ligados a %d e %d", len(near), len(far), len(near[0].mon.arch.pack), len(far[0].mon.arch.pack))
	}
	p, _ := joinAt(z, 1, "knight", near[0].pos.X+1.5, near[0].pos.Z, 0)
	tough(p)
	z.tickFor(2)
	for _, m := range near {
		if len(m.mon.hate) > 0 {
			t.Fatal("de dia o Saqueador é neutro: não ataca quem só passa")
		}
	}
	// Bateu em um: o bando inteiro vem, e o outro bando não.
	z.handle(attackEvent{id: p.id, target: near[0].id})
	z.tickFor(2.5)
	for _, m := range near {
		if m.act.target != p.id {
			t.Fatalf("puxar um puxa todos: %d não veio (ódio %v)", m.id, m.mon.hate)
		}
	}
	for _, m := range far {
		if len(m.mon.hate) > 0 {
			t.Fatal("a outra matilha não tem nada com isso")
		}
	}
	// O último vivo entra em frenesi.
	z.killMonster(p, near[1])
	z.killMonster(p, near[2])
	z.tickFor(0.1)
	if v, ok := near[0].effect(game.EffectBuffSpeed, z.now); !ok || v != near[0].mon.def.Arch.PackFrenzy {
		t.Fatal("o último da matilha fica mais rápido")
	}
}

func TestShieldCutsFrontalHitsAndTurnsAt90DegreesPerSecond(t *testing.T) {
	z := newCombatZone(fixed("skeleton_sentinel", 16, 40, 0))
	m := z.onlyMonster()
	prm := m.mon.def.Arch
	if prm.ShieldArc != 120 || prm.TurnRate != 90 || prm.ShieldCut <= 0 {
		t.Fatalf("números do escudo: %+v", prm)
	}
	// Virado para +Z: de frente segura, pelo lado e por trás não, e magia passa.
	at := func(deg float64) *entity { // deg a partir da frente
		a := deg * math.Pi / 180
		return &entity{pos: Vec2{X: m.pos.X + 2*math.Sin(a), Z: m.pos.Z + 2*math.Cos(a)}}
	}
	m.mon.arch.facing = 0
	for _, c := range []struct {
		deg   float64
		magic bool
		want  float64
	}{{0, false, 60}, {55, false, 60}, {-55, false, 60}, {65, false, 0}, {180, false, 0}, {0, true, 0}} {
		if got := z.shieldBlock(at(c.deg), m, 100, c.magic); got != c.want {
			t.Errorf("golpe a %.0f° (magia %v): segura %.0f, quero %.0f", c.deg, c.magic, got, c.want)
		}
	}
	// O jogador chega por trás: o escudo gira a 90°/s até ele.
	p, peer := joinAt(z, 1, "knight", 40, -5, 0)
	tough(p)
	z.tickFor(0.1) // o monstro vê o jogador e mira nele
	start := m.mon.arch.facing
	z.tickFor(1)
	if turned := math.Abs(wrapAngle(m.mon.arch.facing - start)); math.Abs(turned-math.Pi/2) > 0.12 {
		t.Fatalf("em 1 s girou %.0f°, quero 90°", turned*180/math.Pi)
	}
	z.tickFor(1.5)
	if z.shieldBlock(p, m, 100, false) == 0 {
		t.Fatal("depois de virar, o jogador está de frente: o escudo segura")
	}
	// No golpe de verdade, o que o escudo segura vai como absorvido.
	z.handle(attackEvent{id: p.id, target: m.id})
	z.tickFor(2)
	blocked := false
	for _, ev := range peer.combatFrom(p.id) {
		blocked = blocked || (ev.Absorbed > 0 && !ev.Miss)
	}
	if !blocked {
		t.Fatal("o golpe de frente aparece como absorvido no CombatEvent")
	}
}

func TestSplitterBecomesTwoSmallerOnes(t *testing.T) {
	z := newCombatZone(fixedSlow("rubble_golem", 25, 40, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 43, 0, 0)
	tough(p)
	z.tickFor(0.5)
	z.killMonster(p, m)
	shards := z.monstersOf(m.mon.def.Arch.SplitInto)
	if len(shards) != 2 {
		t.Fatalf("ao morrer vira 2, veio %d", len(shards))
	}
	for _, s := range shards {
		if !s.mon.summon || s.level != m.level-game.ArchSplitLevels || s.mon.hate[p.id] == 0 || s.stats.MaxHP >= m.stats.MaxHP*0.5 || s.scale >= m.scale {
			t.Fatalf("pedaço: invocação %v, nível %d, ódio %v, vida %.0f (do pai %.0f), tamanho %.2f",
				s.mon.summon, s.level, s.mon.hate, s.stats.MaxHP, m.stats.MaxHP, s.scale)
		}
	}
	// Os pedaços não dividem de novo.
	z.killMonster(p, shards[0])
	if n := len(z.monstersOf(m.mon.def.Arch.SplitInto)); n != 1 {
		t.Fatalf("o pedaço não divide: %d vivos", n)
	}
	// Sem luta, o que sobrou some (não renasce nem fica parado no mapa).
	p.teleportTo(Vec2{})
	z.tickFor(outOfCombatAfter + 2)
	for _, e := range z.monsters {
		if e.mon.def.ID == m.mon.def.Arch.SplitInto && !e.dead {
			t.Fatal("o pedaço sem luta some")
		}
	}
}

func TestExploderBlowsUpWhenCloseOrWhenItDies(t *testing.T) {
	// Chega perto: para, avisa no chão (2,5 m, 1 s) e explode, morrendo junto.
	z := newCombatZone(fixed("skeleton_volatile", 10, 40, 0))
	m := z.onlyMonster()
	prm := m.mon.def.Arch
	p, peer := joinAt(z, 1, "knight", 46, 0, 0)
	tough(p)
	var tg *telegraph
	for i := 0; i < 100 && tg == nil; i++ {
		z.update(1 / float64(z.cfg.TickRate))
		tg = z.pendingFrom(m)
	}
	if tg == nil || tg.shape != game.ShapeCircle || tg.radius != prm.BlastRadius || math.Abs(tg.at-z.now-prm.BlastDelay) > 0.06 {
		t.Fatalf("o pavio acende com aviso no chão: %+v", tg)
	}
	if prm.BlastRadius != 2.5 || prm.BlastDelay < game.MinDelayCommon {
		t.Fatalf("explosão: %+v", prm)
	}
	pos := m.pos
	z.tickFor(0.5)
	if dist(m.pos, pos) > 0.05 || m.dead {
		t.Fatal("de pavio aceso ele fica parado")
	}
	z.untilLastTick(tg)
	z.tickFor(0.05)
	if abilityHits(peer, p) != 1 || !m.dead {
		t.Fatalf("a explosão acerta quem ficou (golpes %d) e ele morre (morto %v)", abilityHits(peer, p), m.dead)
	}

	// Morto antes de explodir: explode onde caiu, mesmo morto; quem sai do círculo escapa.
	z2 := newCombatZone(fixed("skeleton_volatile", 10, 40, 0), fixed("skeleton_volatile", 10, 40, 30))
	var a, b *entity
	for _, e := range z2.monsters {
		if e.pos.Z < 15 {
			a = e
		} else {
			b = e
		}
	}
	p2, peer2 := joinAt(z2, 1, "knight", 41.5, 0, 0)
	tough(p2)
	z2.killMonster(p2, a)
	tg = z2.pendingFrom(a)
	if tg == nil || !tg.posthumous || tg.pos != a.pos {
		t.Fatalf("morreu: o aviso da explosão fica onde ele caiu: %+v", tg)
	}
	z2.untilLastTick(tg)
	z2.tickFor(0.05)
	if abilityHits(peer2, p2) != 1 {
		t.Fatal("a explosão de quem morreu acerta quem ficou no círculo")
	}
	p2.teleportTo(Vec2{X: b.pos.X + 1.5, Z: b.pos.Z})
	z2.killMonster(p2, b)
	tg = z2.pendingFrom(b)
	p2.teleportTo(Vec2{X: b.pos.X + 6, Z: b.pos.Z}) // saiu do círculo
	z2.untilLastTick(tg)
	z2.tickFor(0.05)
	if abilityHits(peer2, p2) != 1 {
		t.Fatal("quem sai do círculo escapa")
	}
}

func TestNeutralOnlyAttacksWhoHitIt(t *testing.T) {
	z := newCombatZone(fixed("moss_golem", 25, 40, 0))
	m := z.onlyMonster()
	p1, _ := joinAt(z, 1, "knight", 42.5, 0, 0)
	p2, _ := joinAt(z, 2, "knight", 37.5, 0, 0)
	tough(p1)
	tough(p2)
	z.tickFor(3)
	if len(m.mon.hate) > 0 || m.act.kind != actNone {
		t.Fatal("o neutro não ataca quem só está perto")
	}
	z.handle(attackEvent{id: p1.id, target: m.id})
	z.tickFor(3)
	if m.act.target != p1.id || m.mon.hate[p2.id] != 0 {
		t.Fatalf("o neutro briga só com quem bateu nele: alvo %d, ódio %v", m.act.target, m.mon.hate)
	}
	// O neutro vale de dia e de noite.
	now := midnight
	zn := newClockZone(&now, fixed("moss_golem", 25, 40, 0))
	zn.tickFor(0.1)
	pn, _ := joinAt(zn, 1, "knight", 42.5, 0, 0)
	tough(pn)
	zn.tickFor(3)
	if !zn.night || len(zn.onlyMonster().mon.hate) > 0 {
		t.Fatal("o neutro também não ataca à noite")
	}
}

func TestNeutralByDayIsAggressiveAtNight(t *testing.T) {
	spawn := game.Spawn{Monster: "skeleton_raider", Count: 1, Fixed: true, X: 40, Z: 0, LevelMin: 9, Respawn: 30}
	now := noon
	z := newClockZone(&now, spawn)
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 43, 0, 0)
	tough(p)
	z.tickFor(3)
	if z.night || len(m.mon.hate) > 0 {
		t.Fatal("de dia o Saqueador é neutro")
	}
	now = midnight
	z.tickFor(1)
	if !z.night || m.act.target != p.id {
		t.Fatalf("à noite ele ataca sozinho (noite %v, alvo %d)", z.night, m.act.target)
	}
}

func TestRoleGoesToTheClient(t *testing.T) {
	// A ordem do enum do protocolo é a do game.Archetype.
	for a, want := range []pb.MonsterArchetype{
		pb.MonsterArchetype_MONSTER_ARCHETYPE_UNSPECIFIED, pb.MonsterArchetype_MONSTER_ARCHETYPE_ARCHER,
		pb.MonsterArchetype_MONSTER_ARCHETYPE_HEALER, pb.MonsterArchetype_MONSTER_ARCHETYPE_SUMMONER,
		pb.MonsterArchetype_MONSTER_ARCHETYPE_AMBUSHER, pb.MonsterArchetype_MONSTER_ARCHETYPE_PACK,
		pb.MonsterArchetype_MONSTER_ARCHETYPE_SHIELD, pb.MonsterArchetype_MONSTER_ARCHETYPE_SPLITTER,
		pb.MonsterArchetype_MONSTER_ARCHETYPE_EXPLODER,
	} {
		if pb.MonsterArchetype(game.Archetype(a)) != want {
			t.Fatalf("arquétipo %d (%s) vira %v no protocolo", a, game.Archetype(a), want)
		}
	}
	z := newCombatZone(fixed("skeleton_thrower", 8, 30, 0), fixed("skeleton_raider", 8, 40, 0), fixed("skeleton_minion", 3, 50, 0))
	got := map[string]*pb.MonsterRole{}
	for _, m := range z.monsters {
		got[m.mon.def.ID] = z.entityPB(m).Role
	}
	if r := got["skeleton_thrower"]; r == nil || r.Archetype != pb.MonsterArchetype_MONSTER_ARCHETYPE_ARCHER || r.Neutral || r.NeutralByDay {
		t.Errorf("atirador: %+v", r)
	}
	if r := got["skeleton_raider"]; r == nil || r.Archetype != pb.MonsterArchetype_MONSTER_ARCHETYPE_PACK || !r.NeutralByDay {
		t.Errorf("matilha neutra de dia: %+v", r)
	}
	if got["skeleton_minion"] != nil {
		t.Error("o bruto comum não manda papel (campo ausente)")
	}
}

// Cada arquétipo aparece no mapa principal, poucas vezes e em áreas de nível baixo o bastante
// para todo mundo cruzar com ele.
func TestEveryArchetypeIsInTheWorld(t *testing.T) {
	z := New(testConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	seen := map[game.Archetype]int{}
	neutral := 0
	for _, m := range z.monsters {
		d := m.mon.def
		seen[d.Archetype]++
		if d.Neutral || d.NeutralByDay {
			neutral++
		}
	}
	for a := game.ArchArcher; a <= game.ArchExploder; a++ {
		if seen[a] == 0 || seen[a] > 6 {
			t.Errorf("%s: %d no mundo, quero de 1 a 6", a, seen[a])
		}
	}
	if neutral == 0 {
		t.Error("nenhum neutro no mundo")
	}

	// No mapa de verdade: cada um nasce em chão livre, fora da cidade, e o bando dos
	// Saqueadores nasce junto (a matilha é o grupo de spawn).
	w, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.World, cfg.Size, cfg.SafeRadius = w, w.Size, w.SafeRadius
	zw := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	for _, m := range zw.monsters {
		if !m.mon.def.HasRole() {
			continue
		}
		if w.Blocked(m.mon.home.X, m.mon.home.Z) || zw.inSafe(m.mon.home) {
			t.Errorf("%s nasce em (%.0f, %.0f): bloqueado ou na cidade", m.mon.def.ID, m.mon.home.X, m.mon.home.Z)
		}
		for _, id := range m.mon.arch.pack {
			if o := zw.monsters[id]; o == nil || dist(o.mon.home, m.mon.home) > 12 {
				t.Errorf("matilha espalhada: %d longe de %d", id, m.id)
			}
		}
		if m.mon.def.Archetype == game.ArchPack && len(m.mon.arch.pack) != 3 {
			t.Errorf("o bando dos Saqueadores tem 4: %d ligados", len(m.mon.arch.pack))
		}
	}
}

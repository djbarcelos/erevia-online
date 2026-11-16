package zone

import (
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Arquétipos de IA de monstro (game/archetypes.go, tunnel#129). O think de sempre é o do
// bruto; os outros papéis entram por quatro ganchos dele:
//
//   - archBefore, antes de procurar alvo: o emboscador escondido espera alguém chegar perto,
//     o explosivo de pavio aceso fica parado, o atirador termina o recuo e o curandeiro
//     cura (com ou sem luta);
//   - archFight, com alvo: a matilha puxa o bando, o atirador recua, o invocador chama os
//     lacaios e o explosivo acende o pavio;
//   - archCalm, sem alvo: o emboscador volta a se esconder, o invocador dispensa os lacaios
//     parados e as invocações deles (e os pedaços do que divide) somem;
//   - archDeath, na morte (kill): o que divide vira dois, o explosivo explode e os lacaios
//     morrem com o invocador.
//
// O escudo frontal vira para o alvo no começo do think e corta o dano em applyHit
// (shieldBlock); o golpe da emboscada sai em hitInput. Tudo no relógio da zona (z.now) e
// com z.rng: nada de relógio de parede nem sorteio global.

const (
	ambushWindup    = 0.4  // s entre o emboscador aparecer e o primeiro golpe (o Spawn chega antes)
	lurkForever     = 1e6  // a invisibilidade do emboscador só acaba quando ele aparece
	healRetry       = 0.5  // s até o curandeiro procurar de novo quem curar
	summonRadius    = 2.5  // m: os lacaios nascem em volta do invocador
	summonLevels    = 3    // níveis abaixo do invocador
	splitRadius     = 1.5  // m: os pedaços nascem em volta de onde o que divide caiu
	retreatSlack    = 1.0  // s a mais para o recuo terminar (obstáculo no caminho)
	packFrenzySecs  = 30.0 // frenesi do último da matilha
	shieldTurnMaxDt = 0.1  // s: depois de atordoado, o escudo não gira de uma vez
)

// archState é o estado do papel de um monstro (monsterState.arch).
type archState struct {
	// Atirador: quando pode recuar de novo, e o recuo em andamento.
	nextRetreat float64
	retreating  bool
	retreatTo   Vec2
	retreatEnd  float64
	// Curandeiro: quando procura de novo quem curar.
	nextHeal float64
	// Invocador: a luta começou (o primeiro chamado espera SummonFirst), o próximo chamado e
	// os lacaios chamados.
	summonFight bool
	nextSummon  float64
	minions     []uint32
	// Lacaios e pedaços: somem quando a luta acaba. master = o invocador (0 = nenhum).
	vanishIdle bool
	master     uint32
	// Emboscador: escondido (fora da AOI e sem clique) e o golpe da emboscada pendente.
	lurking, ambushHit bool
	// Matilha: os outros do mesmo grupo de spawn, se o bando já foi puxado nesta luta e o
	// frenesi do último vivo.
	pack       []uint32
	packPulled bool
	frenzy     bool
	// Escudo frontal: para onde está virado (rad em Y, 0 = +Z, a convenção dos avisos no
	// chão) e o último giro.
	facing, faceAt float64
	// Explosivo: o pavio já foi aceso nesta vida (explode uma vez só).
	fused bool
}

// rolePB: o papel para o cliente (nil = bruto e não neutro, o de quase todos).
func rolePB(d *game.MonsterDef) *pb.MonsterRole {
	if !d.HasRole() {
		return nil
	}
	return &pb.MonsterRole{Archetype: pb.MonsterArchetype(d.Archetype), Neutral: d.Neutral, NeutralByDay: d.NeutralByDay}
}

// monsterAggressive: o monstro procura briga sozinho agora. O neutro nunca; o NeutralByDay
// só à noite; os invasores sempre.
func (z *Zone) monsterAggressive(m *entity) bool {
	ms := m.mon
	switch {
	case ms.invader:
		return true
	case ms.def.Neutral:
		return false
	case ms.def.NeutralByDay:
		return z.night
	}
	return ms.def.Aggressive
}

// linkPacks liga os membros de cada matilha (o grupo é o spawn de origem).
func (z *Zone) linkPacks(packs map[int][]*entity) {
	for _, members := range packs {
		for _, m := range members {
			for _, o := range members {
				if o != m {
					m.mon.arch.pack = append(m.mon.arch.pack, o.id)
				}
			}
		}
	}
}

// archRevive: o monstro nasceu ou renasceu. O estado do papel recomeça (a matilha continua a
// mesma) e o emboscador se esconde.
func (z *Zone) archRevive(m *entity) {
	a := &m.mon.arch
	*a = archState{pack: a.pack, facing: a.facing}
	if m.mon.def.Archetype == game.ArchAmbusher {
		z.lurk(m)
	}
}

// archReset: o monstro desistiu da luta (resetMonster).
func (z *Zone) archReset(m *entity) {
	a := &m.mon.arch
	a.retreating, a.summonFight, a.fused, a.packPulled, a.ambushHit, a.frenzy = false, false, false, false, false, false
	z.dismissMinions(m, true)
}

// archBefore roda antes de o monstro procurar alvo. true = o think para aqui.
func (z *Zone) archBefore(m *entity) bool {
	a := &m.mon.arch
	switch m.mon.def.Archetype {
	case game.ArchAmbusher:
		if a.lurking {
			return z.lurkWatch(m)
		}
	case game.ArchExploder:
		if a.fused { // pavio aceso: parado, brilhando, até explodir
			m.clearAction()
			m.stopMoving()
			return true
		}
	case game.ArchArcher:
		if a.retreating {
			return z.retreatStep(m)
		}
	case game.ArchHealer:
		z.healerThink(m)
	}
	return false
}

// archFight roda com o monstro brigando com top. true = o think para aqui (sem atacar
// neste tick).
func (z *Zone) archFight(m *entity, top uint32) bool {
	t := z.players[top]
	switch m.mon.def.Archetype {
	case game.ArchPack:
		z.pullPack(m)
		z.packFrenzy(m)
	case game.ArchArcher:
		return t != nil && z.archerRetreat(m, t)
	case game.ArchSummoner:
		z.summonerFight(m)
	case game.ArchExploder:
		if t != nil && dist(m.pos, t.pos) <= m.mon.def.Arch.FuseRange+m.bodyRadius() {
			z.lightFuse(m)
			return true
		}
	}
	return false
}

// archCalm roda com o monstro sem alvo. true = o think para aqui (não vaga).
func (z *Zone) archCalm(m *entity) bool {
	ms := m.mon
	a := &ms.arch
	if a.vanishIdle {
		// Lacaio de um invocador que ainda briga: volta para a luta dele.
		if mm := z.monsters[a.master]; a.master != 0 && mm != nil && !mm.dead && len(mm.mon.hate) > 0 {
			for pid, v := range mm.mon.hate {
				ms.hate[pid] = v
			}
			return true
		}
		if !z.monsterFighting(m) {
			z.removeMonster(m)
		}
		return true
	}
	switch ms.def.Archetype {
	case game.ArchPack:
		a.packPulled, a.frenzy = false, false
	case game.ArchSummoner:
		a.summonFight = false
		z.dismissMinions(m, false)
	case game.ArchAmbusher:
		if !a.lurking && !ms.returning && z.now-m.lastCombat > outOfCombatAfter {
			z.lurk(m)
			return true
		}
	}
	return false
}

// archDeath roda na morte do monstro (kill), com o ódio da luta ainda no lugar.
func (z *Zone) archDeath(dead *entity) {
	ms := dead.mon
	a := &ms.arch
	p := &ms.def.Arch
	a.retreating = false
	switch ms.def.Archetype {
	case game.ArchSummoner:
		for _, id := range a.minions {
			if g := z.monsters[id]; g != nil && !g.dead {
				z.dieQuietly(g)
			}
		}
		a.minions = nil
	case game.ArchSplitter:
		def := game.Monsters[p.SplitInto]
		if def == nil {
			return
		}
		for _, id := range z.summonAround(dead, def, max(1, dead.level-game.ArchSplitLevels), p.SplitCount, splitRadius) {
			g := z.monsters[id]
			g.mon.arch.vanishIdle = true
			g.lastCombat = z.now
		}
	case game.ArchExploder:
		if !a.fused { // morreu antes de explodir: explode onde caiu
			a.fused = true
			z.blast(dead, false)
		}
	}
}

// dieQuietly: o monstro morre sem saque nem XP (os lacaios quando o invocador cai).
func (z *Zone) dieQuietly(g *entity) {
	g.dead, g.hp = true, 0
	for _, ef := range g.effects {
		z.frame.ends = append(z.frame.ends, &pb.EffectEnd{TargetId: g.id, Effect: pb.Effect(ef.kind)})
	}
	g.effects = nil
	g.clearAction()
	g.stopMoving()
	g.mon.hate = map[uint32]float64{}
	g.mon.corpseEnd = z.now + corpseSeconds
	z.frame.deaths = append(z.frame.deaths, g.id)
	z.forgetTarget(g.id)
}

// ---------- Emboscador ----------

// lurk esconde o emboscador: invisível (o efeito de Sumir), fora da AOI e sem clique.
func (z *Zone) lurk(m *entity) {
	a := &m.mon.arch
	a.lurking, a.ambushHit = true, false
	// Direto no monstro, sem EffectEvent: ninguém vê o emboscador escondido.
	m.addEffect(&game.EffectDef{Kind: game.EffectInvisible, Duration: lurkForever}, z.now, m.id)
	m.clearAction()
	m.stopMoving()
	z.forgetTarget(m.id) // quem mirava nele perde o alvo
	z.aoiDirty = true    // o Despawn sai neste tick
}

// lurkWatch: o emboscador escondido espera alguém chegar a AmbushRange. true = continua
// escondido.
func (z *Zone) lurkWatch(m *entity) bool {
	ms := m.mon
	if len(ms.hate) == 0 {
		p := z.nearestPlayer(m.pos, ms.def.Arch.AmbushRange)
		if p == nil {
			return true
		}
		ms.hate[p.id] = 1
	}
	z.unlurk(m)
	return false
}

// unlurk: o emboscador aparece (a AOI manda o Spawn neste tick) e o primeiro golpe dele
// dói mais (hitInput).
func (z *Zone) unlurk(m *entity) {
	a := &m.mon.arch
	a.lurking, a.ambushHit = false, true
	m.removeEffect(game.EffectInvisible)
	m.nextAttack = math.Max(m.nextAttack, z.now+ambushWindup)
	z.aoiDirty = true
}

// ---------- Atirador ----------

// archerRetreat: o alvo chegou a menos de KeepMin; recua Retreat metros para longe dele
// (no máximo uma vez a cada RetreatCD). true = começou a recuar.
func (z *Zone) archerRetreat(m, t *entity) bool {
	ms := m.mon
	a := &ms.arch
	p := &ms.def.Arch
	if z.now < a.nextRetreat || m.rooted(z.now) || dist(m.pos, t.pos) >= p.KeepMin {
		return false
	}
	spot, ok := z.retreatSpot(m, t)
	if !ok {
		a.nextRetreat = z.now + 1 // encurralado: briga e tenta de novo daqui a pouco
		return false
	}
	a.nextRetreat = z.now + p.RetreatCD
	a.retreating, a.retreatTo = true, spot
	a.retreatEnd = z.now + p.Retreat/math.Max(0.5, m.speed(z.now)) + retreatSlack
	m.clearAction()
	m.repathAt = 0
	z.moveToward(m, spot)
	return true
}

// retreatSpot: Retreat metros para longe do alvo (ou um pouco de lado, se a frente estiver
// bloqueada), dentro da coleira e fora da cidade.
func (z *Zone) retreatSpot(m, t *entity) (Vec2, bool) {
	p := &m.mon.def.Arch
	away := Vec2{X: m.pos.X - t.pos.X, Z: m.pos.Z - t.pos.Z}
	if l := math.Hypot(away.X, away.Z); l > 1e-6 {
		away = Vec2{X: away.X / l, Z: away.Z / l}
	} else if m.face != (Vec2{}) {
		away = Vec2{X: -m.face.X, Z: -m.face.Z}
	} else {
		away = Vec2{Z: 1}
	}
	for _, turn := range [...]float64{0, 0.6, -0.6, 1.2, -1.2} {
		c, s := math.Cos(turn), math.Sin(turn)
		dir := Vec2{X: away.X*c - away.Z*s, Z: away.X*s + away.Z*c}
		spot := clampToZone(Vec2{X: m.pos.X + dir.X*p.Retreat, Z: m.pos.Z + dir.Z*p.Retreat}, z.bounds())
		if dist(spot, m.mon.home) > leashOf(m)-1 || z.pveSafe(spot) || !z.walkable(spot) {
			continue
		}
		if w := z.cfg.World; w != nil && !w.SegmentClear(toPt(m.pos), toPt(spot)) {
			continue
		}
		return spot, true
	}
	return Vec2{}, false
}

// retreatStep: o recuo em andamento. true = ainda recuando (não ataca).
func (z *Zone) retreatStep(m *entity) bool {
	a := &m.mon.arch
	if len(m.mon.hate) == 0 || z.now >= a.retreatEnd || dist(m.pos, a.retreatTo) < 0.15 {
		a.retreating = false
		return false
	}
	z.moveToward(m, a.retreatTo)
	return true
}

// ---------- Curandeiro ----------

// healerThink: a cada HealCD, cura HealFrac da vida do aliado mais ferido abaixo de
// HealBelow, a até HealRange (ele mesmo não conta: mate o curandeiro primeiro).
func (z *Zone) healerThink(m *entity) {
	ms := m.mon
	a := &ms.arch
	p := &ms.def.Arch
	if z.now < a.nextHeal {
		return
	}
	var best *entity
	bestFrac := p.HealBelow
	for _, o := range z.monsters {
		if o == m || o.dead || o.mon.unseen() || o.mon.raid != nil || o.mon.invader != ms.invader || o.mon.returning {
			continue
		}
		if dist(m.pos, o.pos) > p.HealRange+o.bodyRadius() {
			continue
		}
		// O mais ferido; empate, o menor id (o mapa não tem ordem).
		if f := o.hp / o.stats.MaxHP; f < bestFrac || (best != nil && f == bestFrac && o.id < best.id) {
			best, bestFrac = o, f
		}
	}
	if best == nil {
		a.nextHeal = z.now + healRetry
		return
	}
	heal := math.Round(best.stats.MaxHP * p.HealFrac)
	best.hp = math.Min(best.stats.MaxHP, best.hp+heal)
	a.nextHeal = z.now + p.HealCD
	z.frame.combat = append(z.frame.combat, &pb.CombatEvent{
		SourceId: m.id, TargetId: best.id, SkillId: game.HitMonsterHeal, Heal: uint32(heal),
	})
}

// ---------- Invocador ----------

// summonerFight: SummonCount lacaios a cada SummonCD de luta, até SummonMax vivos.
func (z *Zone) summonerFight(m *entity) {
	ms := m.mon
	a := &ms.arch
	p := &ms.def.Arch
	if !a.summonFight {
		a.summonFight = true
		a.nextSummon = z.now + p.SummonFirst
	}
	if z.now < a.nextSummon {
		return
	}
	a.nextSummon = z.now + p.SummonCD
	n := min(p.SummonCount, p.SummonMax-z.liveMinions(m))
	def := game.Monsters[p.Summon]
	if n <= 0 || def == nil {
		return
	}
	for _, id := range z.summonAround(m, def, max(1, m.level-summonLevels), n, summonRadius) {
		g := z.monsters[id]
		g.mon.arch.vanishIdle, g.mon.arch.master = true, m.id
		g.lastCombat = z.now
		a.minions = append(a.minions, id)
	}
}

// liveMinions esquece os lacaios que morreram ou sumiram e conta os vivos.
func (z *Zone) liveMinions(m *entity) int {
	a := &m.mon.arch
	kept := a.minions[:0]
	for _, id := range a.minions {
		if g := z.monsters[id]; g != nil && !g.dead {
			kept = append(kept, id)
		}
	}
	clear(a.minions[len(kept):])
	a.minions = kept
	return len(kept)
}

// dismissMinions tira os lacaios do mundo: todos (all) ou só os que não estão brigando.
func (z *Zone) dismissMinions(m *entity, all bool) {
	a := &m.mon.arch
	if len(a.minions) == 0 {
		return
	}
	kept := a.minions[:0]
	for _, id := range a.minions {
		g := z.monsters[id]
		switch {
		case g == nil || g.dead:
		case all || !z.monsterFighting(g):
			z.removeMonster(g)
		default:
			kept = append(kept, id)
		}
	}
	clear(a.minions[len(kept):])
	a.minions = kept
}

// ---------- Matilha ----------

// pullPack: o primeiro da matilha a brigar puxa os outros com o mesmo ódio.
func (z *Zone) pullPack(m *entity) {
	a := &m.mon.arch
	if a.packPulled {
		return
	}
	a.packPulled = true
	for _, id := range a.pack {
		o := z.monsters[id]
		if o == nil || o.dead || o.mon.unseen() || o.mon.dormant || o.mon.returning || len(o.mon.hate) > 0 {
			continue
		}
		for pid, v := range m.mon.hate {
			o.mon.hate[pid] = v
		}
		o.mon.arch.packPulled = true
	}
}

// packFrenzy: o último vivo da matilha fica mais rápido.
func (z *Zone) packFrenzy(m *entity) {
	a := &m.mon.arch
	if a.frenzy || len(a.pack) == 0 {
		return
	}
	for _, id := range a.pack {
		if o := z.monsters[id]; o != nil && !o.dead && !o.mon.unseen() {
			return
		}
	}
	a.frenzy = true
	z.applyEffect(m, m, &game.EffectDef{Kind: game.EffectBuffSpeed, Duration: packFrenzySecs, Value: m.mon.def.Arch.PackFrenzy})
}

// ---------- Escudo frontal ----------

// turnShield vira o escudo, a TurnRate graus por segundo, para quem o monstro ataca (sem
// alvo, para onde anda).
func (z *Zone) turnShield(m *entity) {
	a := &m.mon.arch
	dt := math.Min(z.now-a.faceAt, shieldTurnMaxDt)
	a.faceAt = z.now
	to := m.face
	if t := z.players[m.act.target]; t != nil && !t.dead {
		to = Vec2{X: t.pos.X - m.pos.X, Z: t.pos.Z - m.pos.Z}
	}
	if dt <= 0 || math.Hypot(to.X, to.Z) < 1e-6 {
		return
	}
	diff := wrapAngle(math.Atan2(to.X, to.Z) - a.facing)
	step := m.mon.def.Arch.TurnRate * math.Pi / 180 * dt
	if math.Abs(diff) <= step {
		a.facing = wrapAngle(a.facing + diff)
		return
	}
	a.facing = wrapAngle(a.facing + math.Copysign(step, diff))
}

// shieldBlock: quanto do golpe físico de src o escudo frontal de dst segura (0 = nada).
// Magia passa por cima do escudo.
func (z *Zone) shieldBlock(src, dst *entity, dmg float64, magic bool) float64 {
	if dst.mon == nil || dst.mon.def.Archetype != game.ArchShield || magic || src == dst || dmg <= 0 {
		return 0
	}
	dx, dz := src.pos.X-dst.pos.X, src.pos.Z-dst.pos.Z
	if math.Hypot(dx, dz) < 1e-6 {
		return 0
	}
	p := &dst.mon.def.Arch
	if math.Abs(wrapAngle(math.Atan2(dx, dz)-dst.mon.arch.facing)) > p.ShieldArc*math.Pi/360 {
		return 0 // pelo lado ou pelas costas
	}
	return math.Round(dmg * p.ShieldCut)
}

// wrapAngle leva o ângulo para (-π, π].
func wrapAngle(a float64) float64 {
	a = math.Mod(a, 2*math.Pi)
	switch {
	case a > math.Pi:
		a -= 2 * math.Pi
	case a <= -math.Pi:
		a += 2 * math.Pi
	}
	return a
}

// ---------- Explosivo ----------

// lightFuse: o explosivo chegou perto; para e explode em BlastDelay.
func (z *Zone) lightFuse(m *entity) {
	m.mon.arch.fused = true
	m.clearAction()
	m.stopMoving()
	z.blast(m, true)
}

// blast: o aviso da explosão, um círculo em volta do monstro. Cai mesmo com ele morto
// (posthumous); com die, ele morre quando a explosão cai (se ainda estiver vivo).
func (z *Zone) blast(m *entity, die bool) {
	p := &m.mon.def.Arch
	t := &telegraph{kind: pb.TelegraphKind_TELEGRAPH_KIND_STRIKE, shape: game.ShapeCircle, pos: m.pos,
		radius: p.BlastRadius, mult: p.BlastMult, hitID: game.HitMonsterAbility, posthumous: true}
	if die {
		t.after = func() {
			if z.monsters[m.id] == m && !m.dead {
				z.kill(m, nil) // a XP e o saque vão para quem brigava com ele
			}
		}
	}
	z.pushTelegraph(m, t, p.BlastDelay)
}

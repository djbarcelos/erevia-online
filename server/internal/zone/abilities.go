package zone

import (
	"math"
	"slices"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Habilidades de monstro com aviso no chão (tunnel#130). Cada monstro com
// MonsterDef.Abilities desenha a forma (círculo, cone, linha ou anel) e o golpe cai no fim
// do atraso em quem ainda estiver dentro. O raid boss usa o mesmo caminho, com as fases e
// os relógios dele (raid.go).
//
// Custo: no think, uma comparação de relógio por habilidade, só com o monstro brigando; no
// tick, um passo por aviso pendente; ao cair, um teste de forma por jogador. O aviso vai
// só para quem conhece o monstro ou o marcado (área de interesse, aoi.go).

// abilityTargetSlack: folga além da coleira para um jogador da luta ainda poder ser
// sorteado (a Chuva e a Marca do raid usavam o raio da Onda, 8 m).
const abilityTargetSlack = 8.0

// telegraph é um aviso no chão: o golpe cai em at, em quem estiver dentro da forma.
type telegraph struct {
	id      uint32
	kind    pb.TelegraphKind
	shape   game.AbilityShape
	pos     Vec2 // centro (círculo e anel) ou de onde saem o cone e a linha
	radius  float64
	inner   float64 // anel: buraco seguro
	arc     float64 // cone: abertura total (rad)
	cosHalf float64 // cone: cosseno da meia abertura
	length  float64 // linha
	width   float64 // linha
	angle   float64 // direção do cone e da linha (rad em Y: 0 = +Z)
	dir     Vec2    // (sen angle, cos angle)
	at      float64 // quando o golpe cai (z.now)
	src     uint32
	follow  uint32 // marca: segue o marcado (e não o acerta)
	gen     int    // monsterState.abilGen na hora do aviso
	mult    float64
	hitID   uint32
	effect  *game.EffectDef
}

// aim aponta o cone ou a linha de t.pos para o ponto.
func (t *telegraph) aim(to Vec2) {
	t.angle = math.Atan2(to.X-t.pos.X, to.Z-t.pos.Z)
	t.dir = Vec2{X: math.Sin(t.angle), Z: math.Cos(t.angle)}
}

// contains: p está dentro da forma (a borda conta como dentro).
func (t *telegraph) contains(p Vec2) bool {
	dx, dz := p.X-t.pos.X, p.Z-t.pos.Z
	switch t.shape {
	case game.ShapeCone:
		d := math.Hypot(dx, dz)
		return d <= t.radius && (d < 1e-9 || dx*t.dir.X+dz*t.dir.Z >= t.cosHalf*d)
	case game.ShapeLine:
		along := dx*t.dir.X + dz*t.dir.Z
		side := math.Abs(dx*t.dir.Z - dz*t.dir.X)
		return along >= 0 && along <= t.length && side <= t.width/2
	case game.ShapeDonut:
		d := math.Hypot(dx, dz)
		return d >= t.inner && d <= t.radius
	}
	return math.Hypot(dx, dz) <= t.radius
}

// lookKind: o visual da habilidade no protocolo.
func lookKind(l game.AbilityLook) pb.TelegraphKind {
	switch l {
	case game.LookWave:
		return pb.TelegraphKind_TELEGRAPH_KIND_WAVE
	case game.LookRain:
		return pb.TelegraphKind_TELEGRAPH_KIND_RAIN
	case game.LookMark:
		return pb.TelegraphKind_TELEGRAPH_KIND_MARK
	}
	return pb.TelegraphKind_TELEGRAPH_KIND_STRIKE
}

// clock: quando a habilidade i fica pronta. No raid boss, os relógios são os campos do
// raidState (os testes e as fases mexem neles direto).
func (ms *monsterState) clock(i int) *float64 {
	if ms.raid != nil {
		if c := ms.raid.clock(ms.def.Abilities[i].ID); c != nil {
			return c
		}
	}
	if len(ms.abilNext) != len(ms.def.Abilities) {
		ms.abilNext = make([]float64, len(ms.def.Abilities))
	}
	return &ms.abilNext[i]
}

// monsterPhase: a fase da luta (só o raid tem fases por enquanto).
func monsterPhase(m *entity) int {
	if r := m.mon.raid; r != nil {
		return r.phase
	}
	return 1
}

// startAbilities: a luta começou. As habilidades sem fase esperam o First delas.
func (z *Zone) startAbilities(m *entity) {
	ms := m.mon
	ms.abilFight = true
	for i := range ms.def.Abilities {
		if a := &ms.def.Abilities[i]; a.MinPhase <= 1 {
			*ms.clock(i) = z.now + a.First
		}
	}
}

// phaseAbilities: o monstro chegou à fase; as habilidades dela esperam o First.
func (z *Zone) phaseAbilities(m *entity, phase int) {
	ms := m.mon
	for i := range ms.def.Abilities {
		if a := &ms.def.Abilities[i]; a.MinPhase == phase {
			*ms.clock(i) = z.now + a.First
		}
	}
}

// stopAbilities: o monstro desistiu da luta ou renasceu. Os avisos pendentes dele são
// cancelados no próximo tick, e a próxima luta começa do First de novo.
func (ms *monsterState) stopAbilities() {
	ms.abilGen++
	ms.abilFight = false
}

// thinkAbilities usa as habilidades prontas, na ordem da def. Só roda com o monstro
// brigando (o think chama com um alvo; o raid, durante a luta).
func (z *Zone) thinkAbilities(m *entity) {
	ms := m.mon
	if len(ms.def.Abilities) == 0 {
		return
	}
	if !ms.abilFight {
		z.startAbilities(m)
	}
	phase := monsterPhase(m)
	for i := range ms.def.Abilities {
		a := &ms.def.Abilities[i]
		if phase < a.MinPhase {
			continue
		}
		next := ms.clock(i)
		if z.now < *next {
			continue
		}
		*next = z.now + a.Cooldown
		z.castAbility(m, a)
	}
}

// castAbility: um uso da habilidade (a Chuva do raid cria um aviso por jogador sorteado).
func (z *Zone) castAbility(m *entity, a *game.AbilityDef) {
	switch a.Target {
	case game.AimSelf:
		z.abilityAt(m, a, m.pos, z.aimPoint(m), 0)
	case game.AimCurrent:
		if t := z.players[m.act.target]; t != nil && !t.dead {
			z.abilityAt(m, a, t.pos, t.pos, 0)
		}
	case game.AimRandom:
		for _, t := range z.abilityTargets(m, max(1, a.Count), 0) {
			z.abilityAt(m, a, t.pos, t.pos, 0)
		}
	case game.AimMark:
		// Nunca o tanque (quem o monstro está batendo), se houver mais alguém.
		targets := z.abilityTargets(m, 1, m.act.target)
		if len(targets) == 0 {
			targets = z.abilityTargets(m, 1, 0)
		}
		for _, t := range targets {
			if a.OnMarked != nil {
				z.applyEffect(m, t, a.OnMarked)
			}
			z.abilityAt(m, a, t.pos, t.pos, t.id)
			if a.Warn != "" {
				z.system(t, a.Warn)
			}
		}
	}
}

// aimPoint: para onde o monstro vira o cone ou a linha (quem ele ataca; sem alvo, a frente
// padrão, +Z).
func (z *Zone) aimPoint(m *entity) Vec2 {
	if t := z.players[m.act.target]; t != nil && !t.dead {
		return t.pos
	}
	return Vec2{X: m.pos.X, Z: m.pos.Z + 1}
}

// abilityAt cria o aviso da habilidade: círculo e anel em center; cone e linha saindo do
// monstro na direção de aim.
func (z *Zone) abilityAt(m *entity, a *game.AbilityDef, center, aim Vec2, follow uint32) {
	t := &telegraph{
		kind: lookKind(a.Look), shape: a.Shape, pos: center, radius: a.Radius, inner: a.Inner,
		length: a.Length, width: a.Width, follow: follow, mult: a.Mult, hitID: a.Hit(), effect: a.Effect,
	}
	if a.Shape == game.ShapeCone || a.Shape == game.ShapeLine {
		t.pos = m.pos
		if aim == m.pos {
			aim.Z++
		}
		t.aim(aim)
		t.arc = a.Arc * math.Pi / 180
		t.cosHalf = math.Cos(t.arc / 2)
	}
	z.pushTelegraph(m, t, a.Delay)
}

// abilityTargets sorteia até n jogadores na luta (no ódio, vivos e perto da coleira), sem skip.
func (z *Zone) abilityTargets(m *entity, n int, skip uint32) []*entity {
	ids := make([]uint32, 0, len(m.mon.hate))
	for id := range m.mon.hate {
		p := z.players[id]
		if id == skip || p == nil || p.dead || dist(p.pos, m.mon.home) > leashOf(m)+abilityTargetSlack {
			continue
		}
		ids = append(ids, id)
	}
	slices.Sort(ids) // sorteio estável (o mapa não tem ordem)
	z.rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	out := make([]*entity, 0, n)
	for _, id := range ids[:min(n, len(ids))] {
		out = append(out, z.players[id])
	}
	return out
}

// addTelegraph: aviso em círculo (os golpes do raid nos testes e os comandos de teste).
func (z *Zone) addTelegraph(m *entity, kind pb.TelegraphKind, at Vec2, radius, delay float64, follow uint32, mult float64, hitID uint32, ef *game.EffectDef) {
	z.pushTelegraph(m, &telegraph{kind: kind, shape: game.ShapeCircle, pos: at, radius: radius, follow: follow, mult: mult, hitID: hitID, effect: ef}, delay)
}

// pushTelegraph agenda o golpe e conta aos clientes que o aviso começou.
func (z *Zone) pushTelegraph(m *entity, t *telegraph, delay float64) {
	z.telegraphSeq++
	t.id, t.at, t.src, t.gen = z.telegraphSeq, z.now+delay, m.id, m.mon.abilGen
	z.telegraphs = append(z.telegraphs, t)
	z.frame.telegraphs = append(z.frame.telegraphs, z.telegraphPB(t, uint32(delay*1000)))
}

func (z *Zone) telegraphPB(t *telegraph, ms uint32) *pb.Telegraph {
	return &pb.Telegraph{
		Id: t.id, Kind: t.kind, X: float32(t.pos.X), Z: float32(t.pos.Z), Radius: float32(t.radius),
		DurationMs: ms, SourceId: t.src, FollowId: t.follow,
		Shape: pb.TelegraphShape(t.shape), Angle: float32(t.angle), Arc: float32(t.arc),
		Length: float32(t.length), Width: float32(t.width), InnerRadius: float32(t.inner),
	}
}

// updateTelegraphs: os avisos que venceram resolvem o golpe; os de um monstro que
// desistiu da luta ou morreu são cancelados.
func (z *Zone) updateTelegraphs() {
	if len(z.telegraphs) == 0 {
		return
	}
	keep := z.telegraphs[:0]
	for _, t := range z.telegraphs {
		src := z.monsters[t.src]
		if src == nil || src.dead || src.mon.abilGen != t.gen {
			ev := z.telegraphPB(t, 0)
			ev.Cancelled = true
			z.frame.telegraphs = append(z.frame.telegraphs, ev)
			continue
		}
		if t.follow != 0 {
			if f := z.players[t.follow]; f != nil && !f.dead {
				t.pos = f.pos
			}
		}
		if z.now < t.at {
			keep = append(keep, t)
			continue
		}
		z.resolveTelegraph(src, t)
	}
	clear(z.telegraphs[len(keep):])
	z.telegraphs = keep
}

// resolveTelegraph: o golpe cai em todos os jogadores dentro da forma e fora da zona segura
// (na invasão, as estradas e a praça valem como fora). A marca não acerta
// o próprio marcado: o perigo é para quem está perto dele.
func (z *Zone) resolveTelegraph(src *entity, t *telegraph) {
	ev := z.telegraphPB(t, 0)
	ev.Resolved = true
	z.frame.telegraphs = append(z.frame.telegraphs, ev)
	var ids []uint32
	for id, p := range z.players {
		if p.dead || id == t.follow || !t.contains(p.pos) || z.pveSafe(p.pos) {
			continue
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	magic := src.stats.Magic
	for _, id := range ids {
		p := z.players[id]
		if t.mult > 0 { // 0: só o efeito
			in := z.hitInput(src, p, magic, t.mult, 0)
			in.NoMiss = true
			z.applyHit(src, p, t.hitID, game.RollHit(z.rng, in), magic)
		}
		if t.effect != nil && !p.dead {
			z.applyEffect(src, p, t.effect)
		}
	}
}

// ---------- Comando de teste (só com DEV_LOGIN, nunca em produção) ----------

// devWarnings: /avisos põe 10 avisos sem dano em volta do jogador (as 4 formas), vindos do
// monstro vivo mais perto (o aviso vai para quem conhece o monstro). Mais de 6 de propósito:
// o cliente desenha só os 6 mais próximos.
func (z *Zone) devWarnings(p *entity) {
	var src *entity
	best := aoiRadius - 10
	for _, m := range z.monsters {
		if d := dist(m.pos, p.pos); !m.dead && !m.mon.hidden && d < best {
			src, best = m, d
		}
	}
	if src == nil {
		z.system(p, "Nenhum monstro a até 60 m para ser a origem dos avisos.")
		return
	}
	shapes := []game.AbilityShape{game.ShapeCircle, game.ShapeCone, game.ShapeLine, game.ShapeDonut}
	for i := range 10 {
		a := float64(i) * 2 * math.Pi / 10
		at := Vec2{X: p.pos.X + 9*math.Cos(a), Z: p.pos.Z + 9*math.Sin(a)}
		t := &telegraph{kind: pb.TelegraphKind_TELEGRAPH_KIND_STRIKE, shape: shapes[i%4], pos: at,
			radius: 3.5, inner: 1.5, length: 8, width: 2, hitID: game.HitMonsterAbility}
		t.aim(p.pos)
		t.arc = math.Pi / 2
		t.cosHalf = math.Cos(t.arc / 2)
		z.pushTelegraph(src, t, 2.5+0.2*float64(i))
	}
	z.system(p, "10 avisos de teste (sem dano) em volta de você.")
}

package zone

import (
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const (
	objectIDBase = uint32(3) << 29 // ids de objetos (estandarte): depois dos itens no chão, antes dos monstros
	auraEvery    = 0.5             // auras (estandarte, coragem) a cada meio segundo
	auraLinger   = 1.25            // o efeito da aura dura um pouco mais que o pulso
)

// objectState é um objeto temporário no chão (estandarte).
type objectState struct {
	owner  uint32
	until  float64
	banner *game.BannerDef
}

// ---------------------------------------------------------------------------------------
// Invisibilidade (Sumir)
// ---------------------------------------------------------------------------------------

// hide: os monstros esquecem quem ficou invisível e param de atacá-lo.
func (z *Zone) hide(e *entity) {
	z.forgetTarget(e.id) // monstros e jogadores perdem o alvo
	if e.pl != nil {
		e.pl.ambushCrit = true
		e.clearAction() // não sai atacando sozinho (isso revelaria na hora)
	}
}

// revealIfHidden: atacar ou usar skill ofensiva revela. O crítico garantido fica para o golpe.
func (z *Zone) revealIfHidden(e *entity) {
	if e.removeEffect(game.EffectInvisible) {
		z.frame.ends = append(z.frame.ends, &pb.EffectEnd{TargetId: e.id, Effect: pb.Effect(game.EffectInvisible)})
		if e.pl != nil {
			e.pl.statsDirty = true
		}
	}
}

// ---------------------------------------------------------------------------------------
// Saltos: Investida e Piscar
// ---------------------------------------------------------------------------------------

// chargeLanding: ponto a chargeStop do alvo, do lado de quem salta, se o caminho estiver livre.
func (z *Zone) chargeLanding(src, t *entity) (Vec2, bool) {
	d := dist(src.pos, t.pos)
	if d <= chargeStop+0.3 {
		return src.pos, true
	}
	dir := Vec2{X: (src.pos.X - t.pos.X) / d, Z: (src.pos.Z - t.pos.Z) / d}
	land := clampToZone(Vec2{X: t.pos.X + dir.X*chargeStop, Z: t.pos.Z + dir.Z*chargeStop}, z.bounds())
	if !z.walkable(land) {
		return Vec2{}, false
	}
	if w := z.cfg.World; w != nil && !w.SegmentClear(toPt(src.pos), toPt(land)) {
		return Vec2{}, false
	}
	return land, true
}

// blink teleporta até distance metros para a frente: na direção em que anda, ou do alvo,
// ou a última direção em que andou. Para antes de obstáculos.
func (z *Zone) blink(e *entity, distance float64) {
	dir := Vec2{}
	if d := dist(e.pos, e.dest); d > 0.2 {
		dir = Vec2{X: (e.dest.X - e.pos.X) / d, Z: (e.dest.Z - e.pos.Z) / d}
	} else if t := z.get(e.act.target); t != nil && t != e {
		if d := dist(e.pos, t.pos); d > 0.2 {
			dir = Vec2{X: (t.pos.X - e.pos.X) / d, Z: (t.pos.Z - e.pos.Z) / d}
		}
	}
	if dir == (Vec2{}) {
		dir = e.face
	}
	if dir == (Vec2{}) {
		dir = Vec2{Z: 1}
	}
	best := e.pos
	bounds := z.bounds()
	for step := 0.5; step <= distance+1e-9; step += 0.5 {
		p := Vec2{X: e.pos.X + dir.X*step, Z: e.pos.Z + dir.Z*step}
		if !bounds.Contains(toPt(p)) || !z.walkable(p) {
			break
		}
		if w := z.cfg.World; w != nil && !w.SegmentClear(toPt(e.pos), toPt(p)) {
			break
		}
		best = p
	}
	e.clearAction()
	e.teleportTo(best)
	e.face = dir
}

// ---------------------------------------------------------------------------------------
// Estandarte e auras
// ---------------------------------------------------------------------------------------

func (z *Zone) spawnBanner(owner *entity, def *game.BannerDef) {
	z.objectSeq++
	id := objectIDBase + z.objectSeq
	e := &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_OBJECT,
		name: "Estandarte de " + owner.name, model: "banner", level: owner.level, scale: 1,
		pos: owner.pos, dest: owner.pos,
		stats: game.Stats{MaxHP: 1},
		obj:   &objectState{owner: owner.id, until: z.now + def.Duration, banner: def},
	}
	e.hp = 1
	e.sentHP, e.sentMaxHP = 1, 1
	z.objects[id] = e
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Spawn{Spawn: &pb.Spawn{Entity: z.entityPB(e)}}}, 0)
}

// applyAura renova o efeito de uma aura sem encher a rede: só avisa o cliente quando o
// efeito começa (enquanto a pessoa fica na área, só estende o fim).
func (z *Zone) applyAura(src, e *entity, kind game.EffectKind, value float64) {
	if ef := e.effectRef(kind, z.now); ef != nil && ef.value == value {
		ef.until = z.now + auraLinger
		return
	}
	z.applyEffect(src, e, &game.EffectDef{Kind: kind, Duration: auraLinger, Value: value})
}

// updateAuras: estandartes (defesa perto deles) e Aura de Coragem (regeneração).
func (z *Zone) updateAuras() {
	for id, o := range z.objects {
		if z.now >= o.obj.until {
			delete(z.objects, id)
			z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Despawn{Despawn: &pb.Despawn{Id: id}}}, 0)
			continue
		}
		b := o.obj.banner
		owner := z.players[o.obj.owner]
		for _, p := range z.players {
			if owner != nil && !p.dead && z.allies(owner, p) && dist(p.pos, o.pos) <= b.Radius {
				z.applyAura(o, p, b.Effect, b.Value)
			}
		}
	}
	for _, k := range z.players {
		r := k.pl.rank(game.SkillCourage)
		if r < 1 || k.dead {
			continue
		}
		v := game.Skills[game.SkillCourage].PValue.At(r)
		for _, p := range z.players {
			if !p.dead && z.allies(k, p) && dist(p.pos, k.pos) <= game.CourageRadius {
				z.applyAura(k, p, game.EffectCourage, v)
			}
		}
	}
}

// ---------------------------------------------------------------------------------------
// Dano por segundo (veneno, queimadura, praga)
// ---------------------------------------------------------------------------------------

// tickDoTs causa o dano dos efeitos contínuos, uma vez por segundo de cada efeito.
func (z *Zone) tickDoTs(e *entity) {
	for i := 0; i < len(e.effects) && !e.dead; i++ {
		ef := &e.effects[i]
		if !ef.kind.IsDoT() || z.now < ef.nextTick || ef.until <= z.now-0.001 {
			continue
		}
		ef.nextTick += 1
		src := z.get(ef.source)
		if src == nil {
			src = e // quem envenenou saiu: o dano continua, sem dono
		}
		dmg := math.Max(1, math.Round(ef.value))
		ev := &pb.CombatEvent{SourceId: ef.source, TargetId: e.id, Damage: uint32(dmg), Dot: pb.Effect(ef.kind)}
		z.frame.combat = append(z.frame.combat, ev)
		e.lastCombat = z.now
		if e.mon != nil && src.pl != nil {
			e.mon.hate[src.id] += dmg
			e.mon.returning = false
		}
		killer := src
		if src == e {
			killer = nil
		}
		z.damageFrom(killer, e, dmg, ev)
	}
}

// damageFrom é damage com quem bateu opcional (dano sem dono não dá XP a ninguém).
func (z *Zone) damageFrom(src, dst *entity, dmg float64, ev *pb.CombatEvent) {
	if src == nil {
		src = dst
		if dst.hp-dmg <= 0 && !dst.has(game.EffectUndying, z.now) {
			// Morte sem dono: kill(dst, nil) não dá XP.
			dst.hp -= dmg
			if ev != nil {
				ev.Kill = true
			}
			z.kill(dst, nil)
			return
		}
	}
	z.damage(src, dst, dmg, ev)
}

// spreadPlague: quem morre infectado passa a praga para os inimigos em volta.
func (z *Zone) spreadPlague(dead *entity) {
	ef := dead.effectRef(game.EffectPlague, z.now)
	if ef == nil || dead.mon == nil {
		return
	}
	src := z.get(ef.source)
	if src == nil {
		return
	}
	def := &game.EffectDef{Kind: game.EffectPlague, Duration: game.Skills[game.SkillPlague].OnTarget.Duration.At(1), Value: ef.value}
	for _, m := range z.monsters {
		if m != dead && !m.dead && !m.mon.hidden && dist(m.pos, dead.pos) <= game.PlagueSpread {
			z.applyEffect(src, m, def)
		}
	}
}

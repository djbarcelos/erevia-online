package zone

import (
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const (
	outOfCombatAfter = 5.0  // segundos sem lutar para regenerar rápido
	approachFactor   = 0.75 // para a esta fração do alcance (folga para o alvo andar)
)

// runAction executa a ação atual: anda até o alcance e ataca ou usa a skill.
func (z *Zone) runAction(e *entity) {
	if e.act.kind == actNone {
		return
	}
	if e.stunned(z.now) {
		e.stopMoving()
		return
	}
	t := z.get(e.act.target)
	if t == nil || t.dead || t == e || (t.mon != nil && t.mon.hidden) {
		e.clearAction()
		return
	}

	rng := e.stats.Range
	if e.act.kind == actSkill {
		rng = e.act.skill.Range
	}
	if d := dist(e.pos, t.pos); d > rng {
		dir := Vec2{X: (e.pos.X - t.pos.X) / d, Z: (e.pos.Z - t.pos.Z) / d}
		e.setDest(Vec2{X: t.pos.X + dir.X*rng*approachFactor, Z: t.pos.Z + dir.Z*rng*approachFactor})
		return
	}
	e.stopMoving()

	// Sem combate dentro da cidade (quem está lá dentro só anda).
	if z.inSafe(e.pos) || z.inSafe(t.pos) {
		return
	}

	switch e.act.kind {
	case actAttack:
		if z.now >= e.nextAttack {
			z.basicAttack(e, t)
			e.nextAttack = z.now + e.stats.AtkInterval
		}
	case actSkill:
		s := e.act.skill
		e.act = action{kind: actAttack, target: t.id} // depois da skill, segue no ataque automático
		z.castSkill(e, s, t)
	}
}

func (z *Zone) basicAttack(src, dst *entity) {
	magic := src.stats.Magic
	hit := game.RollHit(z.rng, src.atk(magic, z.now), dst.def(magic, z.now), 1, src.stats.Crit)
	z.applyHit(src, dst, 0, hit)
}

// requestSkill valida o pedido do jogador e executa (si mesmo/área) ou agenda (alvo).
func (z *Zone) requestSkill(p *entity, skillID, targetID uint32) {
	if p.dead || p.stunned(z.now) {
		return
	}
	s := game.Skills[skillID]
	if s == nil || s.Class != p.pl.class {
		return
	}
	if ready := p.pl.cooldowns[s.ID]; z.now < ready {
		z.sendCooldown(p, s.ID, ready-z.now)
		return
	}
	if p.mp < s.MPCost {
		z.system(p, "Mana insuficiente.")
		return
	}
	switch s.Target {
	case game.TargetSelf:
		z.castSkill(p, s, nil)
	case game.TargetAreaSelf:
		if z.inSafe(p.pos) {
			z.system(p, "Não é possível lutar dentro da cidade.")
			return
		}
		z.castSkill(p, s, nil)
	case game.TargetEnemy:
		t := z.monsters[targetID]
		if t == nil || t.dead || t.mon.hidden {
			z.system(p, "Selecione um alvo.")
			return
		}
		p.act = action{kind: actSkill, target: t.id, skill: s}
	}
}

// castSkill paga o custo, inicia a recarga e aplica o efeito.
func (z *Zone) castSkill(src *entity, s *game.SkillDef, target *entity) {
	if src.pl != nil {
		if src.mp < s.MPCost {
			z.system(src, "Mana insuficiente.")
			return
		}
		src.mp -= s.MPCost
		src.pl.cooldowns[s.ID] = z.now + s.Cooldown
		src.pl.statsDirty = true
		z.sendCooldown(src, s.ID, s.Cooldown)
	}

	if s.OnSelf != nil {
		z.applyEffect(src, s.OnSelf)
	}

	switch s.Target {
	case game.TargetEnemy:
		z.skillHit(src, target, s)
	case game.TargetAreaSelf:
		// Mostra a skill mesmo sem acertar ninguém.
		z.frame.combat = append(z.frame.combat, &pb.CombatEvent{SourceId: src.id, TargetId: src.id, SkillId: s.ID, Miss: true})
		for _, m := range z.monsters {
			if m.dead || m.mon.hidden || dist(src.pos, m.pos) > s.Range {
				continue
			}
			z.skillHit(src, m, s)
		}
	case game.TargetSelf:
		z.frame.combat = append(z.frame.combat, &pb.CombatEvent{SourceId: src.id, TargetId: src.id, SkillId: s.ID})
	}
}

func (z *Zone) skillHit(src, dst *entity, s *game.SkillDef) {
	hit := game.RollHit(z.rng, src.atk(s.Magic, z.now), dst.def(s.Magic, z.now), s.Mult, src.stats.Crit+s.ExtraCrit)
	z.applyHit(src, dst, s.ID, hit)
	if !hit.Miss && !dst.dead && s.OnTarget != nil {
		z.applyEffect(dst, s.OnTarget)
	}
}

func (z *Zone) applyEffect(e *entity, def *game.EffectDef) {
	e.addEffect(def, z.now)
	z.frame.effects = append(z.frame.effects, &pb.EffectEvent{
		TargetId: e.id, Effect: pb.Effect(def.Kind), DurationMs: uint32(def.Duration * 1000),
	})
	if def.Kind == game.EffectStun {
		e.stopMoving()
	}
	if def.Kind == game.EffectSlow || def.Kind == game.EffectBuffSpeed {
		e.moved = true // reenvia a velocidade
	}
	if e.pl != nil {
		e.pl.statsDirty = true
	}
}

// applyHit registra o golpe, aplica o dano, ódio e morte.
func (z *Zone) applyHit(src, dst *entity, skillID uint32, hit game.Hit) {
	ev := &pb.CombatEvent{
		SourceId: src.id, TargetId: dst.id, SkillId: skillID,
		Damage: uint32(hit.Damage), Crit: hit.Crit, Miss: hit.Miss,
	}
	z.frame.combat = append(z.frame.combat, ev)
	src.lastCombat, dst.lastCombat = z.now, z.now

	if dst.mon != nil && src.pl != nil {
		dst.mon.hate[src.id] += math.Max(1, hit.Damage)
		dst.mon.returning = false
	}
	if hit.Miss {
		return
	}
	dst.hp -= hit.Damage
	if dst.hp <= 0 {
		ev.Kill = true
		z.kill(dst, src)
	}
}

// regen recupera vida e mana com o tempo (bem mais rápido fora de combate).
func (z *Zone) regen(e *entity, dt float64) {
	calm := z.now-e.lastCombat > outOfCombatAfter
	if e.mon != nil {
		if calm && !e.mon.returning && e.hp < e.stats.MaxHP {
			e.hp = math.Min(e.stats.MaxHP, e.hp+e.stats.MaxHP*0.03*dt)
		}
		return
	}
	hpRate, mpRate := 0.008, 0.015
	if calm {
		hpRate, mpRate = 0.04, 0.05
	}
	e.hp = math.Min(e.stats.MaxHP, e.hp+e.stats.MaxHP*hpRate*dt)
	e.mp = math.Min(e.stats.MaxMP, e.mp+e.stats.MaxMP*mpRate*dt)
}

func (z *Zone) inSafe(p Vec2) bool { return math.Hypot(p.X, p.Z) < z.cfg.SafeRadius }

func (z *Zone) sendCooldown(p *entity, skillID uint32, seconds float64) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_SkillCooldown{SkillCooldown: &pb.SkillCooldown{
		SkillId: skillID, ReadyInMs: uint32(math.Max(0, seconds) * 1000),
	}}})
}

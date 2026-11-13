package zone

import (
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const (
	outOfCombatAfter = 5.0  // segundos sem lutar para regenerar rápido
	approachFactor   = 0.75 // para a esta fração do alcance (folga para o alvo andar)
	calmRegenMult    = 5.0  // fora de combate, a vida regenera 5× mais rápido
	calmMPRegenMult  = 3.3  // e a mana, 3,3×
	tauntHate        = 100.0
	chargeStop       = 1.5 // Investida para a esta distância do alvo
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
	if e.act.kind == actPickup {
		z.runPickup(e)
		return
	}
	t := z.get(e.act.target)
	if t == nil || t.dead || t == e || (t.mon != nil && t.mon.hidden) || (e.mon != nil && t.invisible(z.now)) {
		z.cancelQueuedSkill(e)
		e.clearAction()
		return
	}
	// Jogador contra jogador: as regras podem mudar no caminho (virou aliado, ficou
	// invisível, a flag venceu). Só a cidade faz esperar em vez de desistir.
	if e.pl != nil && t.pl != nil {
		if ok, _, wait := z.canHit(e, t, e.act.force); !ok && !wait {
			z.cancelQueuedSkill(e)
			e.clearAction()
			return
		}
	}

	rng := e.stats.Range
	if e.act.kind == actSkill {
		rng = e.act.skill.Range
	}
	if d := dist(e.pos, t.pos); d > rng {
		if e.has(game.EffectRoot, z.now) {
			e.stopMoving() // preso: espera o alvo chegar perto
			return
		}
		dir := Vec2{X: (e.pos.X - t.pos.X) / d, Z: (e.pos.Z - t.pos.Z) / d}
		z.moveToward(e, Vec2{X: t.pos.X + dir.X*rng*approachFactor, Z: t.pos.Z + dir.Z*rng*approachFactor})
		return
	}
	e.stopMoving()

	// Sem combate dentro da cidade (quem está lá dentro só anda), menos contra quem tem karma.
	townHunt := e.pl != nil && t.pl != nil && z.hostile(e, t, e.act.force)
	if (z.inSafe(e.pos) || z.inSafe(t.pos)) && !townHunt {
		if e.act.kind == actSkill {
			z.system(e, "Não é possível lutar dentro da cidade.")
			z.cancelQueuedSkill(e)
			e.clearAction()
		}
		return
	}

	switch e.act.kind {
	case actAttack:
		if z.now >= e.nextAttack {
			z.basicAttack(e, t)
			e.nextAttack = z.now + e.attackInterval(z.now)
		}
	case actSkill:
		s := e.act.skill
		e.act = action{kind: actAttack, target: t.id, force: e.act.force} // depois da skill, segue no ataque automático
		z.castSkill(e, s, t)
	}
}

// hitInput monta o golpe de src em dst (ataque básico: mult 1).
func (z *Zone) hitInput(src, dst *entity, magic bool, mult, extraCrit float64) game.HitInput {
	in := game.HitInput{
		Atk: z.attackPower(src, magic), Def: dst.def(magic, z.now), Mult: mult,
		Crit: src.stats.Crit + extraCrit, CritMult: src.stats.CritDamage,
		Accuracy: src.accuracy(z.now), Evasion: dst.stats.Evasion,
	}
	// Saindo da invisibilidade: o primeiro golpe é crítico garantido.
	if src.pl != nil && src.pl.ambushCrit {
		in.ForceCrit = true
		src.pl.ambushCrit = false
	}
	return in
}

func (z *Zone) basicAttack(src, dst *entity) {
	z.revealIfHidden(src)
	z.onPvPHit(src, dst)
	magic := src.stats.Magic
	hit := game.RollHit(z.rng, z.hitInput(src, dst, magic, 1, 0))
	z.applyHit(src, dst, 0, hit, magic)
}

// skillMP: custo de mana no rank (Sobrecarga encarece 10%).
func skillMP(p *entity, s *game.SkillDef) float64 {
	cost := s.MPCost(p.pl.rank(s.ID))
	if p.pl.rank(game.SkillOverload) > 0 {
		cost *= game.OverloadMPCost
	}
	return math.Round(cost)
}

// skillCD: recarga no rank, dividida pela velocidade de conjuração.
func skillCD(p *entity, s *game.SkillDef) float64 {
	cs := p.stats.CastSpeed
	if cs <= 0 {
		cs = 1
	}
	return s.Cooldown(p.pl.rank(s.ID)) / cs
}

// requestSkill valida o pedido do jogador e executa (si mesmo/área) ou agenda (alvo).
func (z *Zone) requestSkill(p *entity, skillID, targetID uint32, force bool) {
	if p.dead || p.stunned(z.now) {
		return
	}
	z.cancelChannel(p, "Viagem cancelada.")
	s := game.Skills[skillID]
	if s == nil || s.Class != p.pl.class || s.IsPassive() {
		return
	}
	if p.pl.rank(s.ID) < 1 {
		z.system(p, "Você ainda não aprendeu "+s.Name+".")
		return
	}
	if ready := p.pl.cooldowns[s.ID]; z.now < ready {
		z.sendCooldown(p, s.ID, ready-z.now)
		return
	}
	if p.mp < skillMP(p, s) {
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
		t := z.get(targetID)
		if t != nil && t.pl != nil {
			if ok, why, _ := z.canHit(p, t, force); !ok {
				if why == "" {
					why = "Selecione um alvo."
				}
				z.system(p, why)
				return
			}
		} else if t == nil || t.mon == nil || t.dead || t.mon.hidden {
			z.system(p, "Selecione um alvo.")
			return
		}
		p.act = action{kind: actSkill, target: t.id, skill: s, force: force}
	}
}

// requestAttack: clicar num monstro ataca; num jogador, só se as regras de PvP deixarem.
func (z *Zone) requestAttack(p *entity, target uint32, force bool) {
	z.cancelChannel(p, "Viagem cancelada.")
	t := z.get(target)
	switch {
	case t == nil || t == p || t.dead:
		return
	case t.pl != nil:
		if ok, why, _ := z.canHit(p, t, force); !ok {
			if why != "" {
				z.system(p, why)
			}
			return
		}
	case t.mon == nil || t.mon.hidden:
		return
	}
	if p.act.kind != actAttack || p.act.target != t.id || p.act.force != force {
		p.act = action{kind: actAttack, target: t.id, force: force}
	}
}

// castSkill paga o custo, inicia a recarga e aplica a skill no rank do jogador.
func (z *Zone) castSkill(src *entity, s *game.SkillDef, target *entity) {
	rank := 1
	if src.pl != nil {
		rank = max(1, src.pl.rank(s.ID))
		if ready := src.pl.cooldowns[s.ID]; z.now < ready {
			z.sendCooldown(src, s.ID, ready-z.now)
			return
		}
		cost := skillMP(src, s)
		if src.mp < cost {
			z.system(src, "Mana insuficiente.")
			return
		}
		// Investida: confere o salto antes de cobrar (caminho bloqueado não gasta nada).
		var landing Vec2
		if s.Charge && target != nil {
			var ok bool
			if landing, ok = z.chargeLanding(src, target); !ok {
				z.system(src, "Caminho bloqueado para a Investida.")
				z.sendCooldown(src, s.ID, 0)
				return
			}
		}
		src.mp -= cost
		cd := skillCD(src, s)
		src.pl.cooldowns[s.ID] = z.now + cd
		src.pl.statsDirty = true
		z.sendCooldown(src, s.ID, cd)
		if s.Charge && target != nil {
			src.teleportTo(landing)
		}
	}
	if s.Mult.At(rank) > 0 || s.OnTarget != nil {
		z.revealIfHidden(src) // atacar revela quem está invisível
	}

	if def := s.OnSelf.At(rank); def != nil {
		z.applyEffect(src, src, def)
		if s.AllyRadius > 0 {
			for _, p := range z.players {
				if p != src && !p.dead && z.allies(src, p) && dist(p.pos, src.pos) <= s.AllyRadius {
					z.applyEffect(src, p, def)
				}
			}
		}
	}
	if s.Banner != nil {
		z.spawnBanner(src, s.Banner)
	}
	if s.Blink.Base > 0 {
		z.blink(src, s.Blink.At(rank))
	}

	switch s.Target {
	case game.TargetEnemy:
		if s.Mult.At(rank) <= 0 {
			// Sem golpe não há CombatEvent: o cast leva o alvo para o cliente animar.
			z.frame.casts = append(z.frame.casts, &pb.SkillCast{SourceId: src.id, SkillId: s.ID, TargetId: target.id})
		}
		z.skillHit(src, target, s, rank)
		if s.Splash > 0 {
			for _, m := range z.monsters {
				if m != target && !m.dead && !m.mon.hidden && dist(target.pos, m.pos) <= s.Splash {
					z.skillHit(src, m, s, rank)
				}
			}
			for _, o := range z.players { // área nunca acerta quem não é hostil
				if o != target && o != src && dist(target.pos, o.pos) <= s.Splash && z.hostile(src, o, false) {
					z.skillHit(src, o, s, rank)
				}
			}
		}
	case game.TargetAreaSelf:
		// Mostra a skill mesmo sem acertar ninguém.
		z.frame.casts = append(z.frame.casts, &pb.SkillCast{SourceId: src.id, SkillId: s.ID})
		for _, m := range z.monsters {
			if m.dead || m.mon.hidden || dist(src.pos, m.pos) > s.Range {
				continue
			}
			z.skillHit(src, m, s, rank)
		}
		if src.pl != nil {
			for _, o := range z.players {
				if o != src && dist(src.pos, o.pos) <= s.Range && z.hostile(src, o, false) {
					z.skillHit(src, o, s, rank)
				}
			}
		}
	case game.TargetSelf:
		z.frame.casts = append(z.frame.casts, &pb.SkillCast{SourceId: src.id, SkillId: s.ID})
	}
}

// skillHit aplica a skill num alvo: golpes, efeito no alvo, provocação e queimadura.
func (z *Zone) skillHit(src, dst *entity, s *game.SkillDef, rank int) {
	if dst.dead {
		return
	}
	z.onPvPHit(src, dst)
	landed := s.Mult.At(rank) <= 0 // sem dano: o efeito sempre pega
	if mult := s.Mult.At(rank); mult > 0 {
		if s.ExecuteBelow > 0 && dst.hp < dst.stats.MaxHP*s.ExecuteBelow {
			mult *= s.ExecuteMult
		}
		for i := 0; i < max(1, s.Hits) && !dst.dead; i++ {
			in := z.hitInput(src, dst, s.Magic, mult, s.ExtraCrit)
			in.NoMiss = s.NoMiss
			hit := game.RollHit(z.rng, in)
			z.applyHit(src, dst, s.ID, hit, s.Magic)
			landed = landed || !hit.Miss
		}
	}
	if dst.dead || !landed {
		return
	}
	if s.Taunt && dst.mon != nil {
		top := 0.0
		for _, v := range dst.mon.hate {
			top = math.Max(top, v)
		}
		dst.mon.hate[src.id] = top + tauntHate
		dst.mon.returning = false
	}
	if spec := s.OnTarget; spec != nil && (s.OnTargetNeeds == game.EffectNone || dst.has(s.OnTargetNeeds, z.now)) {
		def := spec.At(rank)
		if spec.AtkFrac {
			def.Value = math.Round(z.attackPower(src, s.Magic) * def.Value)
		}
		z.applyEffect(src, dst, def)
	}
	// Brasa Viva: skills de fogo deixam o alvo queimando.
	if s.Fire && src.pl != nil {
		if r := src.pl.rank(game.SkillEmber); r > 0 {
			v := math.Round(src.atk(true, z.now) * game.Skills[game.SkillEmber].PValue.At(r))
			z.applyEffect(src, dst, &game.EffectDef{Kind: game.EffectBurn, Duration: game.FireBurnSeconds, Value: math.Max(1, v)})
		}
	}
}

func (z *Zone) applyEffect(src, e *entity, def *game.EffectDef) {
	if e.dead {
		return
	}
	e.addEffect(def, z.now, src.id)
	z.frame.effects = append(z.frame.effects, &pb.EffectEvent{
		TargetId: e.id, Effect: pb.Effect(def.Kind), DurationMs: uint32(def.Duration * 1000),
		SourceId: src.id, Value: float32(def.Value),
	})
	switch def.Kind {
	case game.EffectStun, game.EffectRoot:
		e.stopMoving()
	case game.EffectSlow, game.EffectBuffSpeed:
		e.moved = true // reenvia a velocidade
	case game.EffectInvisible:
		z.hide(e)
	case game.EffectTaunt:
		if e.mon != nil {
			e.act = action{kind: actAttack, target: src.id}
		}
	}
	if e.pl != nil {
		e.pl.statsDirty = true
	}
}

// applyHit registra o golpe e passa o dano pelos efeitos: marca, escudos, imortal,
// roubo de vida, armadura de gelo, veneno da lâmina e Muralha Viva.
func (z *Zone) applyHit(src, dst *entity, skillID uint32, hit game.Hit, magic bool) {
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
	dmg := hit.Damage
	if v, ok := dst.effect(game.EffectMark, z.now); ok {
		dmg = math.Round(dmg * (1 + v))
		ev.Damage = uint32(dmg)
	}
	taken := z.damage(src, dst, dmg, ev)

	if src.pl != nil && taken > 0 && !src.dead {
		// Sede de Sangue
		if r := src.pl.rank(game.SkillBloodthirst); r > 0 {
			heal := math.Round(taken * game.Skills[game.SkillBloodthirst].PValue.At(r))
			if heal > 0 && src.hp < src.stats.MaxHP {
				src.hp = math.Min(src.stats.MaxHP, src.hp+heal)
				ev.Heal = uint32(heal)
			}
		}
		// Lâmina Envenenada
		if r := src.pl.rank(game.SkillPoisonedBlade); r > 0 && !dst.dead && z.rng.Float64() < game.Skills[game.SkillPoisonedBlade].PValue.At(r) {
			v := math.Max(1, math.Round(z.attackPower(src, false)*game.PoisonProcFrac))
			z.applyEffect(src, dst, &game.EffectDef{Kind: game.EffectPoison, Duration: game.PoisonProcSecs, Value: v})
		}
	}
	// Armadura de Gelo: quem bate de perto fica lento.
	if !magic && !src.dead && dst.has(game.EffectFrostArmor, z.now) && dist(src.pos, dst.pos) <= game.MeleeRange {
		z.applyEffect(dst, src, &game.EffectDef{Kind: game.EffectSlow, Duration: game.FrostArmorSecs, Value: game.FrostArmorSlow})
	}
}

// damage tira vida de dst passando por escudo, escudo de mana e Imortal. Devolve quanto
// dano de fato foi causado (para o roubo de vida). ev (opcional) recebe o absorvido.
func (z *Zone) damage(src, dst *entity, dmg float64, ev *pb.CombatEvent) float64 {
	if dst.dead || dmg <= 0 {
		return 0
	}
	absorbed := 0.0
	if sh := dst.effectRef(game.EffectShield, z.now); sh != nil {
		a := math.Min(sh.value, dmg)
		sh.value -= a
		dmg -= a
		absorbed += a
		if sh.value <= 0 {
			dst.removeEffect(game.EffectShield)
			z.frame.ends = append(z.frame.ends, &pb.EffectEnd{TargetId: dst.id, Effect: pb.Effect(game.EffectShield)})
		}
	}
	if ms, ok := dst.effect(game.EffectManaShield, z.now); ok && dmg > 0 && dst.mp > 0 {
		perPoint := math.Max(0.1, ms)
		a := math.Min(dmg, dst.mp/perPoint)
		dst.mp -= a * perPoint
		dmg -= a
		absorbed += a
		if dst.pl != nil {
			dst.pl.statsDirty = true
		}
	}
	if ev != nil && absorbed > 0 {
		ev.Absorbed = uint32(math.Round(absorbed))
	}
	if dmg <= 0 {
		return absorbed
	}
	if dst.pl != nil && dst.pl.channel != nil {
		z.cancelChannel(dst, "Você levou dano: a viagem foi interrompida.")
	}
	if dst.has(game.EffectUndying, z.now) && dst.hp-dmg < 1 {
		dmg = math.Max(0, dst.hp-1)
	}
	dst.hp -= dmg
	if dst.hp <= 0 {
		if ev != nil {
			ev.Kill = true
		}
		z.kill(dst, src)
		return dmg + absorbed
	}
	z.checkLastStand(dst)
	return dmg + absorbed
}

// checkLastStand: Muralha Viva (cavaleiro abaixo de 30% ganha um escudo).
func (z *Zone) checkLastStand(p *entity) {
	if p.pl == nil || p.pl.rank(game.SkillLivingWall) < 1 || z.now < p.pl.lastStandReady {
		return
	}
	if p.hp >= p.stats.MaxHP*game.LastStandBelow {
		return
	}
	p.pl.lastStandReady = z.now + game.LastStandCD
	v := math.Round(p.stats.MaxHP * game.Skills[game.SkillLivingWall].PValue.At(1))
	z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectShield, Duration: game.LastStandSeconds, Value: v})
	z.system(p, "Muralha Viva! Um escudo protege você.")
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
	hp, mp := e.stats.HPRegen, e.stats.MPRegen
	if calm {
		hp *= calmRegenMult
		mp *= calmMPRegenMult
	}
	if v, ok := e.effect(game.EffectCourage, z.now); ok {
		hp += v
	}
	// Espírito do Urso
	if e.pl != nil && e.pl.rank(game.SkillBearSpirit) > 0 && e.hp < e.stats.MaxHP*game.BearBelow {
		hp += e.stats.MaxHP * game.Skills[game.SkillBearSpirit].PValue.At(1)
	}
	e.hp = math.Min(e.stats.MaxHP, e.hp+hp*dt)
	e.mp = math.Min(e.stats.MaxMP, e.mp+mp*dt)
}

func (z *Zone) inSafe(p Vec2) bool { return math.Hypot(p.X, p.Z) < z.cfg.SafeRadius }

func (z *Zone) sendCooldown(p *entity, skillID uint32, seconds float64) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_SkillCooldown{SkillCooldown: &pb.SkillCooldown{
		SkillId: skillID, ReadyInMs: uint32(math.Max(0, seconds) * 1000),
	}}})
}

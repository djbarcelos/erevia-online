package zone

import (
	"math"
	"slices"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const (
	outOfCombatAfter = game.OutOfCombatAfter // segundos sem lutar para regenerar rápido
	approachFactor   = 0.75                  // para a esta fração do alcance (folga para o alvo andar)
	calmRegenMult    = game.CalmHPRegenMult  // fora de combate, a vida regenera 5× mais rápido
	calmMPRegenMult  = game.CalmMPRegenMult  // e a mana, 3,3×
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
	if t == nil || t.dead || t == e || (t.mon != nil && t.mon.unseen()) || (e.mon != nil && t.invisible(z.now)) {
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
	// O alcance conta da borda do corpo do alvo (monstros grandes, como o golem).
	if d := dist(e.pos, t.pos); d-t.bodyRadius() > rng {
		if e.has(game.EffectRoot, z.now) {
			e.stopMoving() // preso: espera o alvo chegar perto
			return
		}
		dir := Vec2{X: (e.pos.X - t.pos.X) / d, Z: (e.pos.Z - t.pos.Z) / d}
		stop := rng*approachFactor + t.bodyRadius()
		z.moveToward(e, Vec2{X: t.pos.X + dir.X*stop, Z: t.pos.Z + dir.Z*stop})
		return
	}
	e.stopMoving()

	// Sem combate dentro da cidade (quem está lá dentro só anda), menos contra quem tem karma.
	// Contra monstros vale pveSafe: durante a invasão, as estradas e a praça ficam abertas.
	townHunt := e.pl != nil && t.pl != nil && z.hostile(e, t, e.act.force)
	safe := z.inSafe
	if e.pl == nil || t.pl == nil {
		safe = z.pveSafe
	}
	if (safe(e.pos) || safe(t.pos)) && !townHunt {
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
	in.Mult *= 1 + z.bestiaryBonus(src, dst) // Bestiário: +1% contra o tipo no 500º abate (bestiary.go)
	// Saindo da invisibilidade: o primeiro golpe é crítico garantido.
	if src.pl != nil && src.pl.ambushCrit {
		in.ForceCrit = true
		src.pl.ambushCrit = false
	}
	// Emboscador que acabou de aparecer: o primeiro golpe dói mais (archetypes.go).
	if src.mon != nil && src.mon.arch.ambushHit {
		in.Mult *= src.mon.def.Arch.AmbushMult
		src.mon.arch.ambushHit = false
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
	return game.SkillMPCost(s, p.pl.rank(s.ID), p.pl.passives.Has(game.PassiveOverload))
}

// skillCD: recarga no rank, dividida pela velocidade de conjuração.
func skillCD(p *entity, s *game.SkillDef) float64 {
	return game.SkillCooldownFor(s, p.pl.rank(s.ID), p.stats.CastSpeed)
}

// requestSkill valida o pedido do jogador e executa (si mesmo/área) ou agenda (alvo).
func (z *Zone) requestSkill(p *entity, skillID, targetID uint32, force bool) {
	if p.dead || p.stunned(z.now) {
		return
	}
	z.cancelChannel(p, "Viagem cancelada.")
	s := game.Skills[skillID]
	if !game.SkillAvailable(s, p.pl.class, p.pl.spec) || s.IsPassive() {
		return
	}
	if p.pl.rank(s.ID) < 1 {
		z.system(p, "Você ainda não aprendeu "+s.Name+".")
		return
	}
	if len(s.Weapon) > 0 && !slices.Contains(s.Weapon, p.pl.inv.weaponType()) {
		z.system(p, s.Name+" exige outra arma.")
		return
	}
	s = game.EffectiveSkill(s, p.pl.ranks) // com os modificadores aprendidos
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
		if z.pveSafe(p.pos) { // jogadores na área continuam protegidos (hostile usa inSafe)
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
		} else if t == nil || t.mon == nil || t.dead || t.mon.unseen() {
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
	case t.mon == nil || t.mon.unseen():
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
		z.questEvent(src, game.GoalSkill, "")
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
					z.markHelp(src) // buff em alguém do grupo
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
				if m != target && !m.dead && !m.mon.unseen() && dist(target.pos, m.pos)-m.bodyRadius() <= s.Splash {
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
			if m.dead || m.mon.unseen() || dist(src.pos, m.pos)-m.bodyRadius() > s.Range {
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
	if dst.mon != nil && src.pl != nil {
		z.markHelp(src) // provocar ou enfraquecer também é ajudar o grupo
	}
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
	// Brasa Viva: skills de fogo deixam o alvo queimando (menos as que já queimam por conta
	// própria, como o Inferno: a queimadura mais fraca da Brasa substituiria a delas).
	if s.Fire && src.pl != nil && (s.OnTarget == nil || s.OnTarget.Kind != game.EffectBurn) {
		if pv := src.pl.passives.Of(game.PassiveFireBurn); pv > 0 {
			v := math.Round(src.atk(true, z.now) * pv)
			z.applyEffect(src, dst, &game.EffectDef{Kind: game.EffectBurn, Duration: game.FireBurnSeconds, Value: math.Max(1, v)})
		}
	}
}

func (z *Zone) applyEffect(src, e *entity, def *game.EffectDef) {
	if e.dead || (e.mon != nil && e.mon.def.ImmuneTo(def.Kind)) {
		return
	}
	if e.pl != nil && (def.Kind == game.EffectStun || def.Kind == game.EffectSlow) {
		if v := e.pl.mastery.of(game.MStatCCResist); v > 0 { // Firmeza (Maestria)
			short := *def
			short.Duration *= 1 - v
			def = &short
		}
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
		z.markHelp(src) // conta para a XP do grupo (party.go)
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
	// Escudo frontal (archetypes.go): o golpe físico de frente perde parte do dano, que o
	// cliente mostra como absorvido.
	if b := z.shieldBlock(src, dst, dmg, magic); b > 0 {
		dmg -= b
		ev.Damage = uint32(dmg)
		ev.Absorbed += uint32(b)
	}
	// Bloqueio do jogador com escudo (Fase 4): golpe físico de qualquer direção (o servidor não
	// guarda para onde o jogador olha). O sorteio só acontece com chance > 0.
	if !magic && dst.pl != nil && src != dst && dst.stats.Block > 0 && z.rng.Float64() < dst.stats.Block {
		b := math.Round(dmg * game.BlockCut(dst.stats))
		dmg -= b
		ev.Damage = uint32(dmg)
		ev.Blocked = true
	}
	taken := z.damage(src, dst, dmg, ev)

	if src.pl != nil && taken > 0 && !src.dead {
		// Sede de Sangue
		if pv := src.pl.passives.Of(game.PassiveLifesteal); pv > 0 {
			heal := math.Round(taken * pv)
			if heal > 0 && src.hp < src.stats.MaxHP {
				src.hp = math.Min(src.stats.MaxHP, src.hp+heal)
				ev.Heal = uint32(heal)
				z.areaEventHeal(src, heal)
			}
		}
		// Lâmina Envenenada
		if pv := src.pl.passives.Of(game.PassivePoisonProc); pv > 0 && !dst.dead && z.rng.Float64() < pv {
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
		ev.Absorbed += uint32(math.Round(absorbed))
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
	if dst.mon != nil && dst.mon.raid != nil {
		z.raidCredit(dst, src, math.Min(dmg, dst.hp))
	}
	if dst.mon != nil && dst.mon.invader {
		z.invasionCredit(src, math.Min(dmg, dst.hp))
	}
	if dst.mon != nil && dst.mon.evMob {
		z.areaEventCredit(src, math.Min(dmg, dst.hp))
	}
	if dst.mon != nil && src.pl != nil {
		dst.mon.addDamage(src.id, math.Min(dmg, dst.hp)) // crédito do abate (party.go)
		z.markHelp(src)
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
	if p.pl == nil || p.pl.passives.Of(game.PassiveLastStand) <= 0 || z.now < p.pl.lastStandReady {
		return
	}
	if p.hp >= p.stats.MaxHP*game.LastStandBelow {
		return
	}
	p.pl.lastStandReady = z.now + game.LastStandCD
	v := math.Round(p.stats.MaxHP * p.pl.passives.Of(game.PassiveLastStand))
	z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectShield, Duration: game.LastStandSeconds, Value: v})
	z.system(p, "Muralha Viva! Um escudo protege você.")
}

// regen recupera vida e mana com o tempo (bem mais rápido fora de combate).
func (z *Zone) regen(e *entity, dt float64) {
	calm := z.now-e.lastCombat > outOfCombatAfter
	if e.mon != nil {
		// O raid boss não se cura sozinho: só ao voltar ao trono (reinício).
		if calm && !e.mon.returning && e.mon.raid == nil && e.hp < e.stats.MaxHP {
			e.hp = math.Min(e.stats.MaxHP, e.hp+e.stats.MaxHP*0.03*dt)
		}
		return
	}
	hp, mp := e.stats.HPRegen, e.stats.MPRegen
	if calm {
		hp *= calmRegenMult
		mp *= calmMPRegenMult
		if e.pl != nil { // Fôlego (Maestria)
			v := 1 + e.pl.mastery.of(game.MStatCalmRegen)
			hp, mp = hp*v, mp*v
		}
	}
	if v, ok := e.effect(game.EffectCourage, z.now); ok {
		hp += v
	}
	// Espírito do Urso
	if e.pl != nil && e.pl.passives.Has(game.PassiveBearSpirit) && e.hp < e.stats.MaxHP*game.BearBelow {
		hp += e.stats.MaxHP * e.pl.passives.Of(game.PassiveBearSpirit)
	}
	e.hp = math.Min(e.stats.MaxHP, e.hp+hp*dt)
	e.mp = math.Min(e.stats.MaxMP, e.mp+mp*dt)
}

func (z *Zone) inSafe(p Vec2) bool { return p.X*p.X+p.Z*p.Z < z.cfg.SafeRadius*z.cfg.SafeRadius }

// pveSafe: zona segura para lutar com monstros. Durante a invasão, as estradas dos portões
// e a praça deixam de ser (o PvP continua com inSafe: karma e flag não mudam).
func (z *Zone) pveSafe(p Vec2) bool { return z.inSafe(p) && !z.invasionOpen(p) }

func (z *Zone) sendCooldown(p *entity, skillID uint32, seconds float64) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_SkillCooldown{SkillCooldown: &pb.SkillCooldown{
		SkillId: skillID, ReadyInMs: uint32(math.Max(0, seconds) * 1000),
	}}})
}

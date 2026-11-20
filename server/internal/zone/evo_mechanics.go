package zone

import (
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Mecânicas das evoluções (docs/evolucoes): efeitos a mais, bênçãos que contam só quando
// ajudam, cura ao longo do tempo, purificar, zona no chão contra inimigos, renovar
// bênçãos, redirecionar dano para o protetor (Interceptar, Juramento, Sacrifício) e o
// Último Reduto.

// effectFor: o efeito de uma skill no rank para quem recebe (cura ao longo do tempo vira
// vida por segundo de quem recebe; curas e escudos dados ficam mais fortes com a Graça).
func (z *Zone) effectFor(src, dst *entity, spec *game.EffectSpec, rank int) *game.EffectDef {
	def := spec.At(rank)
	if spec.MaxHPFrac && def.Duration > 0 {
		def.Value = dst.stats.MaxHP * def.Value / def.Duration
	}
	if src.pl != nil && (def.Kind == game.EffectRegen || def.Kind == game.EffectShield) {
		def.Value *= 1 + src.pl.passives.Of(game.PassiveHealPower)
	}
	return def
}

// selfSpecs: OnSelf e os efeitos a mais da skill.
func selfSpecs(s *game.SkillDef) []*game.EffectSpec {
	var out []*game.EffectSpec
	if s.OnSelf != nil {
		out = append(out, s.OnSelf)
	}
	return append(out, s.ExtraOnSelf...)
}

// alliesNear: os aliados vivos a até r de src (sem src), do mais perto para o mais longe.
func (z *Zone) alliesNear(src *entity, r float64) []*entity {
	var out []*entity
	for _, p := range z.players {
		if p != src && !p.dead && z.allies(src, p) && dist(p.pos, src.pos) <= r {
			out = append(out, p)
		}
	}
	for i := 1; i < len(out); i++ { // poucos: inserção basta
		for j := i; j > 0 && dist(out[j].pos, src.pos) < dist(out[j-1].pos, src.pos); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// selfEffects aplica os efeitos "em si" em quem lança (menos AllyOnly) e nos aliados em
// AllyRadius. ResOnAlly: recurso por aliado que ganhou algo (sem o efeito ou sem a vida
// cheia), até o teto de alvos do recurso.
func (z *Zone) selfEffects(src *entity, s *game.SkillDef, rank int) {
	specs := selfSpecs(s)
	if len(specs) == 0 {
		return
	}
	var to []*entity
	if !s.AllyOnly {
		to = append(to, src)
	}
	if s.AllyRadius > 0 {
		to = append(to, z.alliesNear(src, s.AllyRadius)...)
	}
	paid := 0
	for _, p := range to {
		helped := p.hp < p.stats.MaxHP
		for _, spec := range specs {
			def := z.effectFor(src, p, spec, rank)
			helped = helped || !p.has(def.Kind, z.now)
			z.applyEffect(src, p, def)
		}
		if p == src {
			continue
		}
		z.markHelp(src) // buff em alguém do grupo
		if d := resDef(src); s.ResOnAlly > 0 && helped && d != nil && paid < d.TargetCap {
			z.gainRes(src, s.ResOnAlly)
			paid++
		}
	}
}

// negativeEffects: o que Purificar tira (um por pessoa, nesta ordem).
var negativeEffects = []game.EffectKind{game.EffectPoison, game.EffectPlague, game.EffectBurn, game.EffectRoot, game.EffectSlow}

// cleanse: tira 1 efeito negativo de quem lança e de até CleanseAllies aliados (os mais
// perto) e cura CleanseHeal da vida máxima de cada um.
func (z *Zone) cleanse(src *entity, s *game.SkillDef, rank int) {
	n := int(math.Round(s.CleanseAllies.At(rank)))
	to := append([]*entity{src}, z.alliesNear(src, s.AllyRadius)...)
	if len(to) > n+1 {
		to = to[:n+1]
	}
	power := 1.0
	if src.pl != nil {
		power += src.pl.passives.Of(game.PassiveHealPower)
	}
	for _, p := range to {
		for _, k := range negativeEffects {
			if p.removeEffect(k) {
				z.frame.ends = append(z.frame.ends, &pb.EffectEnd{TargetId: p.id, Effect: pb.Effect(k)})
				break
			}
		}
		if heal := math.Round(p.stats.MaxHP * s.CleanseHeal * power); heal > 0 && p.hp < p.stats.MaxHP {
			p.hp = math.Min(p.stats.MaxHP, p.hp+heal)
			z.areaEventHeal(src, heal)
		}
		if p != src {
			z.markHelp(src)
		}
	}
}

// renewBlessings (Bênção Suprema): as bênçãos que quem lança aprendeu (skills em si com
// aliados) voltam em todos a até AllyRadius, com o efeito dobrado por RenewSecs.
func (z *Zone) renewBlessings(src *entity, s *game.SkillDef) {
	const renewSecs = 8.0
	to := append([]*entity{src}, z.alliesNear(src, s.AllyRadius)...)
	for id, r := range src.pl.ranks {
		b := game.EffectiveSkill(game.Skills[id], src.pl.ranks)
		if r < 1 || b == nil || b.ID == s.ID || b.AllyRadius <= 0 || b.AllyOnly || b.IsPassive() || !game.SkillAvailable(b, src.pl.class, src.pl.spec) {
			continue
		}
		for _, spec := range selfSpecs(b) {
			for _, p := range to {
				def := z.effectFor(src, p, spec, r)
				def.Value *= 2
				def.Duration = renewSecs
				z.applyEffect(src, p, def)
			}
		}
	}
}

// spawnZone: área no chão contra inimigos (Zona de Provocação). Usa o objeto do
// estandarte, sem modelo próprio ainda.
func (z *Zone) spawnZone(owner *entity, def *game.ZoneDef, rank int) {
	z.objectSeq++
	id := objectIDBase + z.objectSeq
	e := &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_OBJECT,
		name: "Zona de " + owner.name, model: "banner", level: owner.level, scale: 1,
		pos: owner.pos, dest: owner.pos,
		stats: game.Stats{MaxHP: 1},
		obj:   &objectState{owner: owner.id, until: z.now + def.Duration, zone: def, zoneRank: rank, nextTick: z.now},
	}
	e.hp = 1
	e.sentHP, e.sentMaxHP = 1, 1
	z.objects[id] = e
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Spawn{Spawn: &pb.Spawn{Entity: z.entityPB(e)}}}, 0)
}

// tickZone: a cada Period, provoca e aplica o efeito nos monstros dentro da zona.
func (z *Zone) tickZone(o *entity) {
	def := o.obj.zone
	if z.now < o.obj.nextTick {
		return
	}
	o.obj.nextTick += def.Period
	owner := z.players[o.obj.owner]
	if owner == nil || owner.dead {
		return
	}
	dmg := 0.0
	if def.Damage != (game.Scale{}) {
		dmg = math.Max(1, math.Round(owner.atk(def.Magic, z.now)*def.Damage.At(o.obj.zoneRank)))
	}
	for _, m := range z.monsters {
		if m.dead || m.mon.unseen() || dist(o.pos, m.pos)-m.bodyRadius() > def.Radius {
			continue
		}
		if dmg > 0 { // Nuvem Pestilenta: dano por pulso, como um DoT (não gera recurso)
			ev := &pb.CombatEvent{SourceId: owner.id, TargetId: m.id, Damage: uint32(dmg), Dot: pb.Effect(game.EffectPlague)}
			z.frame.combat = append(z.frame.combat, ev)
			m.lastCombat = z.now
			m.mon.hate[owner.id] += dmg
			m.mon.returning = false
			z.damage(owner, m, dmg, ev)
			if m.dead {
				continue
			}
		}
		if def.Taunt {
			z.taunt(owner, m, def.Period+0.5)
		}
		if def.Effect != game.EffectNone {
			z.applyEffect(owner, m, &game.EffectDef{Kind: def.Effect, Duration: def.EffectSecs, Value: def.Value.At(o.obj.zoneRank)})
		}
	}
}

// taunt: o monstro passa a odiar src acima de todos e é obrigado a atacá-lo por secs.
func (z *Zone) taunt(src, m *entity, secs float64) {
	top := 0.0
	for _, v := range m.mon.hate {
		top = math.Max(top, v)
	}
	m.mon.hate[src.id] = top + tauntHate
	m.mon.returning = false
	z.applyEffect(src, m, &game.EffectDef{Kind: game.EffectTaunt, Duration: secs})
}

// allyDamageCut: Vínculo de Guarda: o maior corte entre os aliados perto (e o próprio),
// sem somar entre Bastiões.
func (z *Zone) allyDamageCut(dst *entity) float64 {
	best := 0.0
	for _, p := range z.players {
		v := p.pl.passives.Of(game.PassiveAllyDamageReduce)
		if v <= best || p.dead || (p != dst && (!z.allies(p, dst) || dist(p.pos, dst.pos) > game.AllyAuraRadius)) {
			continue
		}
		best = v
	}
	return math.Min(best, 0.5)
}

// redirect: parte do dano que dst levaria vai para quem o protege (Interceptar e
// Juramento: fração; Sacrifício: o que passaria do piso de vida). Devolve o dano que fica
// com dst. O protetor apanha sem gerar recurso e sem redirecionar de novo.
func (z *Zone) redirect(src, dst *entity, dmg float64) float64 {
	if dst.pl == nil || dmg <= 0 {
		return dmg
	}
	if ef := dst.effectRef(game.EffectGuarded, z.now); ef != nil {
		if g := z.players[ef.source]; g != nil && g != dst && !g.dead && dist(g.pos, dst.pos) <= game.GuardRange {
			part := math.Round(dmg * ef.value)
			dmg -= part
			z.damageNoRedirect(src, g, part)
		}
	}
	if ef := dst.effectRef(game.EffectSacrifice, z.now); ef != nil {
		floor := dst.stats.MaxHP * ef.value
		if g := z.players[ef.source]; g != nil && g != dst && !g.dead && dst.hp-dmg < floor && dist(g.pos, dst.pos) <= game.GuardRange {
			over := math.Min(dmg, dmg-math.Max(0, dst.hp-floor))
			dmg -= over
			z.damageNoRedirect(src, g, over)
		}
	}
	return dmg
}

// damageNoRedirect: o dano que o protetor recebe no lugar do aliado.
func (z *Zone) damageNoRedirect(src, g *entity, dmg float64) {
	if dmg <= 0 {
		return
	}
	g.pl.redirecting = true
	z.damage(src, g, dmg, nil)
	g.pl.redirecting = false
}

// lastBastion (Último Reduto): o golpe que mataria deixa com 1 de vida, gasta todo o recurso
// e vira um escudo. Devolve o dano que de fato entra.
func (z *Zone) lastBastion(p *entity, dmg float64) float64 {
	if p.pl == nil || dmg < p.hp || z.now < p.pl.lastBastionReady || !p.pl.passives.Has(game.PassiveLastBastion) {
		return dmg
	}
	p.pl.lastBastionReady = z.now + game.LastBastionCD
	spent := p.pl.res.SpendAll()
	if v := math.Round(p.stats.MaxHP * game.LastBastionPer * spent); v > 0 {
		z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectShield, Duration: game.LastBastionSecs, Value: v})
	}
	z.system(p, "Último Reduto! Você se recusa a cair.")
	return math.Max(0, p.hp-1)
}

// faithShield (Escudo da Fé): cada 10 de recurso gasto vira um escudo.
func (z *Zone) faithShield(p *entity, spent float64) {
	pv := p.pl.passives.Of(game.PassiveFaithShield)
	if pv <= 0 || spent < 10 {
		return
	}
	v := math.Round(p.stats.MaxHP * pv * math.Floor(spent/10) * (1 + p.pl.passives.Of(game.PassiveHealPower)))
	z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectShield, Duration: game.FaithShieldSecs, Value: v})
}

// frenzyStack (Sequência Selvagem): básico seguido no mesmo alvo soma um acúmulo (até 5);
// outro alvo ou 3 s sem bater recomeçam.
func (z *Zone) frenzyStack(p, dst *entity) {
	if !p.pl.passives.Has(game.PassiveFrenzyStacks) {
		return
	}
	if p.pl.frenzyTarget != dst.id || z.now-p.pl.frenzyAt > game.FrenzyStackSecs {
		p.pl.frenzyTarget, p.pl.frenzyStacks = dst.id, 0
	}
	p.pl.frenzyStacks = min(game.FrenzyStackMax, p.pl.frenzyStacks+1)
	p.pl.frenzyAt = z.now
	p.pl.statsDirty = true
}

// killReset (Carnificina Final): matar com a skill zera a recarga dela, 1 vez a cada 10 s.
func (z *Zone) killReset(p *entity, s *game.SkillDef) {
	if p.pl == nil || !p.pl.passives.Has(game.PassiveKillReset) || z.now < p.pl.killResetReady {
		return
	}
	p.pl.killResetReady = z.now + game.KillResetCD
	delete(p.pl.cooldowns, s.ID)
	z.sendCooldown(p, s.ID, 0)
}

// leaderAura (Liderança): o maior bônus de dano entre os Senhores da Guerra perto (e o
// próprio), sem somar.
func (z *Zone) leaderAura(src *entity) float64 {
	best := 0.0
	for _, p := range z.players {
		v := p.pl.passives.Of(game.PassiveAllyDamageAura)
		if v <= best || p.dead || (p != src && (!z.allies(p, src) || dist(p.pos, src.pos) > game.LeaderRadius)) {
			continue
		}
		best = v
	}
	return best
}

// chargeMomentum (Ímpeto de Carga): depois de uma investida, dano e velocidade por 5 s.
func (z *Zone) chargeMomentum(p *entity) {
	v := p.pl.passives.Of(game.PassiveChargeMomentum)
	if v <= 0 {
		return
	}
	z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectBuffAtk, Duration: game.MomentumSecs, Value: v})
	z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectBuffSpeed, Duration: game.MomentumSecs, Value: 2 * v})
}

// chainCharge (Investida Implacável): salta até ChainCharge vezes, do alvo para o inimigo
// mais perto ainda não atingido (até ChainChargeRange), batendo em cada um.
func (z *Zone) chainCharge(src *entity, s *game.SkillDef, rank int, first *entity) {
	hit := map[uint32]bool{}
	next := first
	for i := 0; i < s.ChainCharge && next != nil; i++ {
		if landing, ok := z.chargeLanding(src, next); ok {
			src.teleportTo(landing)
		}
		z.skillHit(src, next, s, rank, false)
		hit[next.id] = true
		var best *entity
		for _, m := range z.monsters {
			if m.dead || m.mon.unseen() || hit[m.id] || dist(src.pos, m.pos) > game.ChainChargeRange {
				continue
			}
			if best == nil || dist(src.pos, m.pos) < dist(src.pos, best.pos) {
				best = m
			}
		}
		next = best
	}
	z.chargeMomentum(src)
}

// interrupt: atordoar um monstro com um aviso no chão pendente cancela o golpe (o aviso
// some) e gera recurso para quem atordoou (Ímpeto).
func (z *Zone) interrupt(src, m *entity) {
	for _, t := range z.telegraphs {
		if t.src == m.id && !t.posthumous && t.gen == m.mon.abilGen {
			m.mon.abilGen++
			z.gainEvent(src, game.GenInterrupt)
			return
		}
	}
}

// dominanceAuras (Presença Dominante): os inimigos perto do Senhor da Guerra ficam
// enfraquecidos e lentos enquanto estiverem ali.
func (z *Zone) dominanceAuras() {
	for _, p := range z.players {
		if p.dead || !p.pl.passives.Has(game.PassiveDominance) {
			continue
		}
		for _, m := range z.monsters {
			if m.dead || m.mon.unseen() || dist(p.pos, m.pos)-m.bodyRadius() > game.DominanceRadius {
				continue
			}
			z.applyAura(p, m, game.EffectWeaken, game.DominanceWeaken)
			z.applyAura(p, m, game.EffectSlow, game.DominanceSlow)
		}
	}
}

// curseKinds: o que conta como maldição (Agonia, Abismo Interior, Maldição Persistente).
var curseKinds = []game.EffectKind{game.EffectWeaken, game.EffectMark, game.EffectArmorBreak, game.EffectCurse}

func isCurse(k game.EffectKind) bool {
	for _, c := range curseKinds {
		if c == k {
			return true
		}
	}
	return false
}

// cursed: o alvo tem alguma maldição ativa.
func cursed(e *entity, now float64) bool {
	for _, k := range curseKinds {
		if e.has(k, now) {
			return true
		}
	}
	return false
}

// renewCurses (Maldição Persistente): as outras maldições de src no alvo voltam à duração
// inteira.
func (z *Zone) renewCurses(src, dst *entity) {
	for i := range dst.effects {
		ef := &dst.effects[i]
		if ef.source == src.id && isCurse(ef.kind) && ef.kind != game.EffectCurse && ef.until > z.now {
			ef.until = z.now + ef.duration
		}
	}
}

// spreadBurn (Coração do Vulcão): a queimadura do alvo passa para quem está a até 3 m.
func (z *Zone) spreadBurn(src, dst *entity) {
	ef := dst.effectRef(game.EffectBurn, z.now)
	if ef == nil {
		return
	}
	def := &game.EffectDef{Kind: game.EffectBurn, Duration: ef.until - z.now, Value: ef.value}
	for _, m := range z.monsters {
		if m != dst && !m.dead && !m.mon.unseen() && !m.has(game.EffectBurn, z.now) && dist(m.pos, dst.pos)-m.bodyRadius() <= game.FireHeartSpread {
			z.applyEffect(src, m, def)
		}
	}
}

// pandemic (Pandemia): as pragas e maldições do alvo (com o tempo que falta) passam para os
// inimigos a até PandemicRange.
func (z *Zone) pandemic(src, dst *entity) {
	var defs []*game.EffectDef
	for _, ef := range dst.effects {
		if ef.until > z.now && (ef.kind == game.EffectPlague || isCurse(ef.kind)) {
			defs = append(defs, &game.EffectDef{Kind: ef.kind, Duration: ef.until - z.now, Value: ef.value})
		}
	}
	for _, m := range z.monsters {
		if m == dst || m.dead || m.mon.unseen() || dist(m.pos, dst.pos)-m.bodyRadius() > game.PandemicRange {
			continue
		}
		for _, d := range defs {
			z.applyEffect(src, m, d)
		}
	}
}

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
	for _, m := range z.monsters {
		if m.dead || m.mon.unseen() || dist(o.pos, m.pos)-m.bodyRadius() > def.Radius {
			continue
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

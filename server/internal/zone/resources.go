package zone

import (
	"math"

	"erevia/server/internal/game"
)

// Recurso de classe (game/resources.go): a barra da evolução. Não vai para o banco: começa
// em 0 no login, ao evoluir, ao trocar de subclasse e ao mudar de zona (a entidade é nova).
// Os ganhos saem dos golpes (applyHit/skillHit), dos lançamentos (castSkill), dos efeitos
// novos (applyEffect) e dos abates (kill); DoT e reflexo passam por z.damage e não geram.

func resDef(p *entity) *game.ResourceDef {
	if p == nil || p.pl == nil {
		return nil
	}
	return game.ResourceOf(p.pl.class, p.pl.spec)
}

// gainRes soma n na barra do jogador (respeitando o teto por segundo e o máximo).
func (z *Zone) gainRes(p *entity, n float64) {
	if d := resDef(p); d != nil && n > 0 && !p.dead {
		if p.pl.passives.Has(game.PassiveCommand) && len(z.alliesNear(p, game.LeaderRadius)) > 0 {
			n *= game.CommandResMult // Comando Absoluto
		}
		p.pl.res.Gain(d, n, z.now)
	}
}

// gainEvent soma o que o evento gera no recurso do jogador.
func (z *Zone) gainEvent(p *entity, e game.GenEvent) {
	if d := resDef(p); d != nil {
		z.gainRes(p, d.Gen(e))
	}
}

// resetRes zera a barra (troca de evolução ou de subclasse).
func resetRes(p *entity) {
	if p.pl != nil {
		p.pl.res.Reset()
	}
}

// resInt: o valor inteiro mostrado (o SelfStats só sai quando ele muda).
func resInt(p *entity) int { return int(math.Floor(p.pl.res.Value)) }

// facing: para onde a entidade olha (radianos, 0 = +Z): o escudo dos monstros guarda a
// frente; os outros olham para o alvo da ação ou para onde andam. ok = false se não dá
// para saber (parado e sem alvo).
func (z *Zone) facing(e *entity) (float64, bool) {
	if e.mon != nil && e.mon.def.Archetype == game.ArchShield {
		return e.mon.arch.facing, true
	}
	if t := z.get(e.act.target); t != nil && t != e {
		return math.Atan2(t.pos.X-e.pos.X, t.pos.Z-e.pos.Z), true
	}
	if dx, dz := e.dest.X-e.pos.X, e.dest.Z-e.pos.Z; math.Hypot(dx, dz) > 0.1 {
		return math.Atan2(dx, dz), true
	}
	return 0, false
}

// behind: src está no cone de trás de dst (Golpe pelas costas, Combo).
func (z *Zone) behind(src, dst *entity) bool {
	yaw, ok := z.facing(dst)
	return ok && game.Backstab(yaw, src.pos.X-dst.pos.X, src.pos.Z-dst.pos.Z)
}

// hitGains: o que um golpe que causou dano gera para quem bateu e para quem apanhou.
// engaged: o alvo não estava em combate antes deste golpe.
func (z *Zone) hitGains(src, dst *entity, skillID uint32, crit, blocked, engaged bool) {
	if src.pl != nil && src != dst {
		if d := resDef(src); d != nil {
			n := 0.0
			if skillID == 0 {
				basic := d.Gen(game.GenBasicHit)
				if src.has(game.EffectFuryDouble, z.now) { // Fúria Incontrolável
					basic *= 2
				}
				n += basic
			}
			if crit {
				n += d.Gen(game.GenCrit)
			}
			if engaged && dst.mon != nil {
				n += d.Gen(game.GenEngage)
			}
			if d.Gen(game.GenBackstab) > 0 && z.behind(src, dst) {
				n += d.Gen(game.GenBackstab)
			}
			n += d.RangedGen(dist(src.pos, dst.pos))
			z.gainRes(src, n)
		}
	}
	if dst.pl != nil && src != dst {
		if d := resDef(dst); d != nil {
			n := d.Gen(game.GenDamageTaken)
			if dist(src.pos, dst.pos) <= game.MeleeTakenRange {
				n += d.Gen(game.GenMeleeTaken)
			}
			if blocked {
				n += d.Gen(game.GenBlock)
			}
			z.gainRes(dst, n)
		}
	}
}

// skillTargetGains: a skill pegou num alvo (uma vez por alvo, até o teto de alvos do
// recurso): ganho por skill acertada, por alvo extra e o da própria skill (ResOnHit).
func (z *Zone) skillTargetGains(src *entity, s *game.SkillDef, damaged bool) {
	d := resDef(src)
	if d == nil {
		return
	}
	if src.pl.castTargets >= d.TargetCap {
		return
	}
	n := s.ResOnHit
	if damaged {
		n += d.Gen(game.GenSkillHit)
	}
	if src.pl.castTargets > 0 {
		n += d.Gen(game.GenExtraTarget)
	}
	src.pl.castTargets++
	z.gainRes(src, n)
}

// castResource: confere e cobra o recurso da skill e soma os ganhos de lançar. Devolve
// quanto foi gasto (para as skills que gastam tudo) e false se faltou recurso.
func (z *Zone) castResource(p *entity, s *game.SkillDef) (float64, bool) {
	d := resDef(p)
	if d == nil {
		return 0, s.ResCost <= 0 && !s.ResSpendAll
	}
	spent := 0.0
	switch {
	case s.ResSpendAll:
		if p.pl.res.Value < game.SkillResCost(s) {
			return 0, false
		}
		spent = p.pl.res.SpendAll()
	case s.ResCost > 0:
		cost := game.ResCostFor(s, p.pl.passives)
		if !p.pl.res.Spend(cost) {
			return 0, false
		}
		spent = cost
	}
	p.pl.castTargets = 0
	z.afterResSpend(p, spent)
	n := s.ResOnCast
	if p.pl.res.LastCast != s.ID {
		n += d.Gen(game.GenDistinctCast)
	}
	p.pl.res.LastCast = s.ID
	z.gainRes(p, n)
	return spent, true
}

// resShort: a skill pede mais recurso do que há (mensagem de recusa; "" = dá).
func resShort(p *entity, s *game.SkillDef) string {
	cost := game.ResCostFor(s, p.pl.passives)
	if cost <= 0 {
		return ""
	}
	d := resDef(p)
	if d == nil {
		return "Esta skill exige o recurso de uma evolução."
	}
	if p.pl.res.Value+1e-9 < cost {
		return d.Name + " insuficiente."
	}
	return ""
}

// killGains: abate de monstro: quem causou dano nele ganha (Fé).
func (z *Zone) killGains(dead *entity) {
	if dead.mon == nil {
		return
	}
	for id, h := range dead.mon.hate {
		if p := z.players[id]; p != nil && h > 0 {
			z.gainEvent(p, game.GenKill)
		}
	}
	// Marca da Presa: matar quem você marcou devolve recurso; Caçada Implacável devolve
	// mais e passa a marca para o inimigo mais perto.
	for _, ef := range dead.effects {
		if ef.kind != game.EffectMark || ef.until <= z.now {
			continue
		}
		p := z.players[ef.source]
		if p == nil {
			continue
		}
		z.gainRes(p, markKillRes(p))
		if p.pl.passives.Has(game.PassiveRelentlessHunt) {
			z.gainRes(p, game.RelentlessHuntRes)
			z.passMark(p, dead, ef)
		}
	}
}

// markKillRes: o maior ResOnMarkKill entre as skills aprendidas.
func markKillRes(p *entity) float64 {
	best := 0.0
	for id, r := range p.pl.ranks {
		if s := game.Skills[id]; s != nil && r > 0 && s.ResOnMarkKill > best && game.SkillAvailable(s, p.pl.class, p.pl.spec) {
			best = s.ResOnMarkKill
		}
	}
	return best
}

// fervorFaith: Fé por crítico sagrado com Fervor.
const fervorFaith = 3.0

// skillDamageBonus: multiplicadores de dano da skill no alvo: sagrada (Zelo) e contra
// quem tem um dos efeitos de BonusVs (Veredito).
func (z *Zone) skillDamageBonus(src, dst *entity, s *game.SkillDef) float64 {
	m := 1.0
	if s.Holy && src.pl != nil {
		m *= 1 + src.pl.passives.Of(game.PassiveHolyDamagePct)
	}
	if s.BonusVsPct > 0 {
		for _, k := range s.BonusVs {
			if dst.has(k, z.now) {
				m *= 1 + s.BonusVsPct
				break
			}
		}
	}
	return m
}

// afterResSpend: o que gastar recurso dispara (Escudo da Fé, Fluxo Arcano).
func (z *Zone) afterResSpend(p *entity, spent float64) {
	z.faithShield(p, spent)
	if v := p.pl.passives.Of(game.PassiveFinisherRefund); v > 0 && spent > 0 { // Dança das Lâminas
		p.pl.res.Value = math.Min(resDef(p).Max, p.pl.res.Value+spent*v)
	}
	if v := p.pl.passives.Of(game.PassiveManaOnSpend); v > 0 && spent >= 10 {
		p.mp = math.Min(p.stats.MaxMP, p.mp+p.stats.MaxMP*v*math.Floor(spent/10))
	}
}

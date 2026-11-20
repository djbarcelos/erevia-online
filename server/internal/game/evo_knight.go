package game

import "fmt"

// Conteúdo das evoluções do Cavaleiro (docs/evolucoes/bastion.md e herald.md): as skills
// novas (86–99 e 101–116) e os ajustes nas que vieram do degrau 20 e da especialização.
// Os números são os dos docs; a calibragem fina vem do simulador.

// Ids das skills novas do Cavaleiro (as iniciais 85, 95, 100 e 106 ficam em evolutions.go).
const (
	SkillBastionSteel     uint32 = 86  // Pele de Aço (evol. da Pele de Ferro)
	SkillBastionStance    uint32 = 87  // Postura de Ferro (modificador da Guarda)
	SkillBastionWall      uint32 = 88  // Muralha Viva (evol.)
	SkillBastionRedoubt   uint32 = 89  // Último Reduto (chave)
	SkillBastionLink      uint32 = 90  // Vínculo de Guarda
	SkillBastionShared    uint32 = 91  // Égide Compartilhada (modificador da Égide)
	SkillBastionIntercept uint32 = 92  // Interceptar
	SkillBastionShieldArt uint32 = 93  // Guarda do Escudo
	SkillBastionOath      uint32 = 94  // Juramento de Proteção (chave)
	SkillBastionDefyPlus  uint32 = 96  // Desafio Reforçado (modificador do Desafio)
	SkillBastionZone      uint32 = 97  // Zona de Provocação
	SkillBastionEcho      uint32 = 98  // Eco do Escudo (modificador do Golpe de Escudo)
	SkillBastionPrison    uint32 = 99  // Prisão do Guardião (chave)
	SkillHeraldZeal       uint32 = 101 // Zelo
	SkillHeraldVerdict    uint32 = 102 // Veredito (modificador do Golpe Radiante)
	SkillHeraldFervor     uint32 = 103 // Fervor
	SkillHeraldBlaze      uint32 = 104 // Aurora Ardente (modificador do Martelo)
	SkillHeraldSentence   uint32 = 105 // Sentença Sagrada (chave)
	SkillHeraldCourage    uint32 = 107 // Aura de Coragem (evol.)
	SkillHeraldLonger     uint32 = 108 // Bênção Ampliada (modificador da Bênção da Luz)
	SkillHeraldRenewal    uint32 = 109 // Renovação
	SkillHeraldGrace      uint32 = 110 // Graça
	SkillHeraldSupreme    uint32 = 111 // Bênção Suprema (chave)
	SkillHeraldFaithWard  uint32 = 112 // Escudo da Fé
	SkillHeraldBlinding   uint32 = 113 // Luz Cegante (modificador do Sol Nascente)
	SkillHeraldPurify     uint32 = 114 // Purificar
	SkillHeraldPenance    uint32 = 115 // Penitência
	SkillHeraldSacrifice  uint32 = 116 // Sacrifício (chave)
)

// node preenche classe, evolução, ramo e marco de uma skill nova.
func node(spec string, branch, level int, kind NodeKind, s *SkillDef) *SkillDef {
	s.Class, s.Spec, s.Branch, s.Level, s.Node = Specs[spec].Class, spec, branch, level, kind
	if s.Hits == 0 {
		s.Hits = 1
	}
	return s
}

// evolvedNode: a versão evoluída de uma skill da base, com outro id, na árvore.
func evolvedNode(baseID, id uint32, spec string, branch, level int, change func(*SkillDef)) *SkillDef {
	s := evolved(baseID, id, spec, branch)
	s.Level = level
	if change != nil {
		change(s)
	}
	return s
}

func knightEvoSkills() []*SkillDef {
	const b, h = "bastion", "herald"
	return []*SkillDef{
		// ---------- Bastião: Muralha ----------
		evolvedNode(SkillIronSkin, SkillBastionSteel, b, 0, 20, func(s *SkillDef) {
			s.Name, s.Description = "Pele de Aço", "A armadura já faz parte do corpo."
		}),
		node(b, 0, 25, NodeModifier, &SkillDef{ID: SkillBastionStance, Name: "Postura de Ferro", MaxRank: 3,
			Modifies: SkillBastionGuard, Mod: &SkillMod{EffectDur: Scale{2, 2}}, FX: "b_hide",
			Description: "Quanto mais firme a postura, mais tempo a guarda aguenta.",
			Text:        func(r int) string { return fmt.Sprintf("A Guarda Inabalável dura +%s s.", num(Scale{2, 2}.At(r))) }}),
		evolvedNode(SkillLivingWall, SkillBastionWall, b, 0, 30, func(s *SkillDef) {
			s.MaxRank, s.PValue = 5, Scale{0.12, 0.02}
			s.Text = func(r int) string {
				return fmt.Sprintf("Ao cair abaixo de 30%% de vida, ganha um escudo de %s da vida máxima por 10 s e +30 de Determinação (1 vez a cada 60 s).", pct(Scale{0.12, 0.02}.At(r)))
			}
		}),
		node(b, 0, 40, NodeKey, &SkillDef{ID: SkillBastionRedoubt, Name: "Último Reduto", MaxRank: 1,
			Passive: PassiveLastBastion, PValue: Scale{1, 0}, FX: "b_undying",
			Description: "O Bastião não cai enquanto houver determinação.",
			Text: func(int) string {
				return "Um golpe que mataria deixa você com 1 de vida e gasta toda a Determinação: escudo de 1% da vida máxima por ponto gasto, por 6 s (1 vez a cada 120 s)."
			}}),
		// ---------- Bastião: Protetor ----------
		node(b, 1, 20, NodePassive, &SkillDef{ID: SkillBastionLink, Name: "Vínculo de Guarda", MaxRank: 5,
			Passive: PassiveAllyDamageReduce, PValue: Scale{0.02, 0.02}, FX: "k_aura",
			Description: "Quem luta ao lado do Bastião apanha menos.",
			Text: func(r int) string {
				return fmt.Sprintf("Você e os aliados a até 10 m recebem −%s de dano (não soma entre Bastiões).", pct(Scale{0.02, 0.02}.At(r)))
			}}),
		node(b, 1, 25, NodeModifier, &SkillDef{ID: SkillBastionShared, Name: "Égide Compartilhada", MaxRank: 3,
			Modifies: SkillAegis, Mod: &SkillMod{EffectValPct: Scale{0.1, 0.1}, CD: Scale{-5, -5}}, FX: "m_shield",
			Description: "A Égide cobre mais e volta antes.",
			Text: func(r int) string {
				return fmt.Sprintf("Égide da Ordem: +%s no escudo e −%s s de recarga.", pct(Scale{0.1, 0.1}.At(r)), num(Scale{5, 5}.At(r)))
			}}),
		node(b, 1, 30, NodeActive, &SkillDef{ID: SkillBastionIntercept, Name: "Interceptar", MaxRank: 3,
			Target: TargetSelf, MP: Scale{20, 0}, CD: Scale{20, 0}, AllyRadius: 12, AllyOnly: true,
			OnSelf: &EffectSpec{Kind: EffectGuarded, Duration: Scale{4, 1}, Value: Scale{0.3, 0}},
			FX:     "k_charge", Description: "Põe o escudo entre os aliados e o perigo.",
			Text: func(r int) string {
				return fmt.Sprintf("Por %s s, 30%% do dano que os aliados a até 12 m levariam vai para você.", num(Scale{4, 1}.At(r)))
			}}),
		node(b, 1, 35, NodePassive, &SkillDef{ID: SkillBastionShieldArt, Name: "Guarda do Escudo", MaxRank: 5,
			Passive: PassiveBlockPct, PValue: Scale{0.02, 0.02}, FX: "b_stone",
			Description: "O escudo encontra cada golpe.",
			Text: func(r int) string {
				v := Scale{0.02, 0.02}.At(r)
				return fmt.Sprintf("Com escudo: +%s de chance de bloqueio, e o bloqueio reduz %s do dano.", pct(v), pct(BlockReduceBase+v))
			}}),
		node(b, 1, 40, NodeKey, &SkillDef{ID: SkillBastionOath, Name: "Juramento de Proteção", MaxRank: 1,
			Target: TargetSelf, CD: Scale{90, 0}, AllyRadius: 10, AllyOnly: true, ResCost: 50,
			OnSelf: &EffectSpec{Kind: EffectGuarded, Duration: Scale{8, 0}, Value: Scale{0.4, 0}},
			FX:     "k_banner", Description: "Jura levar os golpes no lugar dos companheiros.",
			Text: func(int) string {
				return "Por 8 s, 40% do dano que os aliados a até 10 m recebem vai para você."
			}}),
		// ---------- Bastião: Provocador ----------
		node(b, 2, 25, NodeModifier, &SkillDef{ID: SkillBastionDefyPlus, Name: "Desafio Reforçado", MaxRank: 3,
			Modifies: SkillBastionDefy, FX: "k_taunt",
			Mod: &SkillMod{Range: Scale{1, 1}, ExtraOnTarget: []*EffectSpec{{Kind: EffectWeaken, Duration: Scale{4, 0}, Value: Scale{0.05, 0.05}}}},
			Description: "O desafio chega mais longe e tira a força de quem atende.",
			Text: func(r int) string {
				return fmt.Sprintf("Desafio do Bastião: +%s m de raio, e os provocados perdem %s de ataque por 4 s.", num(Scale{1, 1}.At(r)), pct(Scale{0.05, 0.05}.At(r)))
			}}),
		node(b, 2, 30, NodeActive, &SkillDef{ID: SkillBastionZone, Name: "Zona de Provocação", MaxRank: 3,
			Target: TargetSelf, MP: Scale{30, 0}, CD: Scale{30, 0}, ResCost: 20,
			Zone: &ZoneDef{Radius: 6, Duration: 8, Period: 2, Taunt: true, Effect: EffectSlow, Value: Scale{0.2, 0.1}, EffectSecs: 2.5},
			FX:   "b_roar", Description: "Marca o chão: ali ninguém escapa do Bastião.",
			Text: func(r int) string {
				return fmt.Sprintf("Marca o chão (6 m) por 8 s: a cada 2 s provoca quem está dentro e o deixa %s mais lento.", pct(Scale{0.2, 0.1}.At(r)))
			}}),
		node(b, 2, 35, NodeModifier, &SkillDef{ID: SkillBastionEcho, Name: "Eco do Escudo", MaxRank: 3,
			Modifies: SkillBastionBash, FX: "b_quake",
			Mod:         &SkillMod{Splash: Scale{3, 0}, SplashMax: 2, SplashMult: Scale{0.5, 0.25}, SplashNoEffect: true},
			Description: "A pancada ecoa nos vizinhos.",
			Text: func(r int) string {
				return fmt.Sprintf("O Golpe de Escudo atinge também até 2 inimigos a até 3 m do alvo, com %s do dano (sem atordoar).", pct(Scale{0.5, 0.25}.At(r)))
			}}),
		node(b, 2, 40, NodeKey, &SkillDef{ID: SkillBastionPrison, Name: "Prisão do Guardião", MaxRank: 1,
			Target: TargetAreaSelf, Range: 8, CD: Scale{60, 0}, ResCost: 60, Taunt: true,
			OnTarget: &EffectSpec{Kind: EffectRoot, Duration: Scale{3, 0}},
			ExtraOnTarget: []*EffectSpec{{Kind: EffectTaunt, Duration: Scale{6, 0}},
				{Kind: EffectWeaken, Duration: Scale{6, 0}, Value: Scale{0.2, 0}}},
			FX: "m_prison", Description: "Ninguém sai, e todos olham para o Bastião.",
			Text: func(int) string {
				return "Prende todos a até 8 m por 3 s e os provoca por 6 s; eles perdem 20% de ataque nesse tempo."
			}}),

		// ---------- Paladino: Julgamento ----------
		node(h, 0, 20, NodePassive, &SkillDef{ID: SkillHeraldZeal, Name: "Zelo", MaxRank: 5,
			Passive: PassiveHolyDamagePct, PValue: Scale{0.02, 0.02}, FX: "k_fencing",
			Description: "A fé guia a lâmina.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de dano das skills sagradas.", pct(Scale{0.02, 0.02}.At(r))) }}),
		node(h, 0, 25, NodeModifier, &SkillDef{ID: SkillHeraldVerdict, Name: "Veredito", MaxRank: 3,
			Modifies: SkillHeraldStrike, FX: "k_spin",
			Mod:         &SkillMod{BonusVs: []EffectKind{EffectWeaken, EffectBlind}, BonusVsPct: Scale{0.1, 0.1}, ResOnHit: Scale{2, 2}},
			Description: "O golpe pune quem já foi julgado.",
			Text: func(r int) string {
				return fmt.Sprintf("Golpe Radiante: +%s contra alvos enfraquecidos ou cegos e +%s de Fé por acerto.", pct(Scale{0.1, 0.1}.At(r)), num(Scale{2, 2}.At(r)))
			}}),
		node(h, 0, 30, NodePassive, &SkillDef{ID: SkillHeraldFervor, Name: "Fervor", MaxRank: 5,
			Passive: PassiveHolyCrit, PValue: Scale{0.01, 0.01}, FX: "r_weak",
			Description: "A luz acha a brecha.",
			Text: func(r int) string {
				return fmt.Sprintf("+%s de chance de crítico nas skills sagradas; crítico sagrado gera +3 de Fé.", pct(Scale{0.01, 0.01}.At(r)))
			}}),
		node(h, 0, 35, NodeModifier, &SkillDef{ID: SkillHeraldBlaze, Name: "Aurora Ardente", MaxRank: 3,
			Modifies: SkillHeraldDawn, Mod: &SkillMod{Splash: Scale{1, 1}, EffectDur: Scale{2, 1}}, FX: "m_inferno",
			Description: "O Martelo abre o dia mais largo.",
			Text: func(r int) string {
				return fmt.Sprintf("Martelo da Aurora: +%s m de raio, e o enfraquecimento dura +%s s.", num(Scale{1, 1}.At(r)), num(Scale{2, 1}.At(r)))
			}}),
		node(h, 0, 40, NodeKey, &SkillDef{ID: SkillHeraldSentence, Name: "Sentença Sagrada", MaxRank: 1,
			Target: TargetEnemy, Range: 2.5, MP: Scale{40, 0}, CD: Scale{45, 0}, Mult: Scale{4.5, 0}, NoMiss: true, Holy: true,
			ResSpendAll: true, ResPerPoint: 0.01, FX: "b_exec",
			Description: "Toda a fé num golpe só.",
			Text: func(int) string {
				return "Causa 450% do ataque, sem errar; gasta toda a Fé e ganha +1% de dano por ponto gasto."
			}}),
		// ---------- Paladino: Bênçãos ----------
		evolvedNode(SkillCourage, SkillHeraldCourage, h, 1, 20, func(s *SkillDef) {
			s.MaxRank = 5
			s.Text = func(r int) string {
				return fmt.Sprintf("Você e os aliados a até 10 m regeneram %s de vida por segundo.", num(Scale{2, 2}.At(r)))
			}
		}),
		node(h, 1, 25, NodeModifier, &SkillDef{ID: SkillHeraldLonger, Name: "Bênção Ampliada", MaxRank: 3,
			Modifies: SkillHeraldLight, Mod: &SkillMod{EffectDur: Scale{5, 5}}, FX: "b_roar",
			Description: "A bênção demora a se apagar.",
			Text:        func(r int) string { return fmt.Sprintf("A Bênção da Luz dura +%s s.", num(Scale{5, 5}.At(r))) }}),
		node(h, 1, 30, NodeActive, &SkillDef{ID: SkillHeraldRenewal, Name: "Renovação", MaxRank: 3,
			Target: TargetSelf, MP: Scale{35, 0}, CD: Scale{40, 0}, AllyRadius: 10, ResCost: 20, ResOnAlly: 4,
			OnSelf: &EffectSpec{Kind: EffectRegen, Duration: Scale{6, 0}, Value: Scale{0.08, 0.02}, MaxHPFrac: true},
			FX:     "m_mind", Description: "Uma luz morna fecha as feridas.",
			Text: func(r int) string {
				return fmt.Sprintf("Você e os aliados a até 10 m recuperam %s da vida máxima ao longo de 6 s.", pct(Scale{0.08, 0.02}.At(r)))
			}}),
		node(h, 1, 35, NodePassive, &SkillDef{ID: SkillHeraldGrace, Name: "Graça", MaxRank: 3,
			Passive: PassiveHealPower, PValue: Scale{0.05, 0.05}, FX: "b_thirst",
			Description: "Cura com as duas mãos.",
			Text:        func(r int) string { return fmt.Sprintf("Curas e escudos que você dá ficam +%s mais fortes.", pct(Scale{0.05, 0.05}.At(r))) }}),
		node(h, 1, 40, NodeKey, &SkillDef{ID: SkillHeraldSupreme, Name: "Bênção Suprema", MaxRank: 1,
			Target: TargetSelf, CD: Scale{90, 0}, AllyRadius: 15, ResCost: 60, RenewBlessings: true,
			FX: "m_arcane", Description: "A ordem inteira canta junto.",
			Text: func(int) string {
				return "Renova as suas bênçãos aprendidas em você e nos aliados a até 15 m, com o efeito dobrado por 8 s."
			}}),
		// ---------- Paladino: Expiação ----------
		node(h, 2, 20, NodePassive, &SkillDef{ID: SkillHeraldFaithWard, Name: "Escudo da Fé", MaxRank: 5,
			Passive: PassiveFaithShield, PValue: Scale{0.01, 0.005}, FX: "m_shield",
			Description: "A fé gasta vira proteção.",
			Text: func(r int) string {
				return fmt.Sprintf("A cada 10 de Fé gasta, ganha um escudo de %s da vida máxima por 10 s.", pct(Scale{0.01, 0.005}.At(r)))
			}}),
		node(h, 2, 25, NodeModifier, &SkillDef{ID: SkillHeraldBlinding, Name: "Luz Cegante", MaxRank: 3,
			Modifies: SkillHeraldSun, Mod: &SkillMod{EffectValue: Scale{0.05, 0.05}, EffectDur: Scale{1, 1}}, FX: "r_smoke",
			Description: "O sol que nasce queima os olhos.",
			Text: func(r int) string {
				return fmt.Sprintf("Sol Nascente: +%s na cegueira e +%s s de duração.", pct(Scale{0.05, 0.05}.At(r)), num(Scale{1, 1}.At(r)))
			}}),
		node(h, 2, 30, NodeActive, &SkillDef{ID: SkillHeraldPurify, Name: "Purificar", MaxRank: 3,
			Target: TargetSelf, MP: Scale{25, 0}, CD: Scale{20, -3}, AllyRadius: 10,
			CleanseAllies: Scale{1, 1}, CleanseHeal: 0.03, FX: "r_flask",
			Description: "A luz tira o mal de dentro.",
			Text: func(r int) string {
				return fmt.Sprintf("Tira 1 efeito negativo de você e de até %s aliado(s) a até 10 m e cura 3%% da vida máxima de cada um.", num(Scale{1, 1}.At(r)))
			}}),
		node(h, 2, 35, NodeActive, &SkillDef{ID: SkillHeraldPenance, Name: "Penitência", MaxRank: 3,
			Target: TargetEnemy, Range: 6, MP: Scale{25, 0}, CD: Scale{20, 0}, Mult: Scale{1.2, 0}, Holy: true,
			OnTarget:      &EffectSpec{Kind: EffectMark, Duration: Scale{8, 0}, Value: Scale{0.08, 0.04}},
			ExtraOnTarget: []*EffectSpec{{Kind: EffectWeaken, Duration: Scale{8, 0}, Value: Scale{0.1, 0}}},
			FX:            "r_mark", Description: "Marca o culpado para o julgamento.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa 120%% do ataque e marca o alvo por 8 s: ele recebe +%s de dano e causa −10%%.", pct(Scale{0.08, 0.04}.At(r)))
			}}),
		node(h, 2, 40, NodeKey, &SkillDef{ID: SkillHeraldSacrifice, Name: "Sacrifício", MaxRank: 1,
			Target: TargetSelf, CD: Scale{120, 0}, AllyRadius: 10, AllyOnly: true, ResCost: 50,
			OnSelf: &EffectSpec{Kind: EffectSacrifice, Duration: Scale{6, 0}, Value: Scale{0.25, 0}},
			FX:     "b_undying", Description: "O Paladino paga pelos outros.",
			Text: func(int) string {
				return "Por 6 s, os aliados a até 10 m não caem abaixo de 25% de vida: o dano que passaria disso vai para você."
			}}),
	}
}

// knightTuning: multiplicadores de dano calibrados no simulador (metas dos docs: Bastião
// ≤ 6,1 s para matar no 20 e +50% a +70% da média no 40; Paladino ≤ 4,8 s no 20).
var knightTuning = struct {
	bash, bulwark, last, radiant Scale
	bashCD, bulwarkCD           float64
}{
	bash: Scale{3.0, 0.35}, bulwark: Scale{2.4, 0.3}, last: Scale{2.4, 0.3}, radiant: Scale{2.5, 0.3},
	bashCD: 5, bulwarkCD: 12,
}

// adjustKnightEvolutions: os ajustes dos docs nas skills que já existiam.
func adjustKnightEvolutions() {
	set := func(id uint32, f func(*SkillDef)) { f(Skills[id]) }
	set(SkillBastionBash, func(s *SkillDef) {
		m := knightTuning.bash
		s.Mult, s.CD = m, Scale{knightTuning.bashCD, 0}
		s.Text = func(r int) string {
			return fmt.Sprintf("Causa %s do ataque e atordoa por %s s.", pct(m.At(r)), num(Scale{1.5, 0.25}.At(r)))
		}
	})
	set(SkillBulwark, func(s *SkillDef) {
		m := knightTuning.bulwark
		s.Mult, s.CD = m, Scale{knightTuning.bulwarkCD, 0}
		s.Text = func(r int) string {
			return fmt.Sprintf("Causa %s do ataque a até 5 m, atordoa por %s s e provoca os atingidos.", pct(m.At(r)), num(s.OnTarget.Duration.At(r)))
		}
	})
	set(SkillHeraldStrike, func(s *SkillDef) {
		m := knightTuning.radiant
		s.Mult = m
		s.Text = func(r int) string { return fmt.Sprintf("Causa %s do ataque.", pct(m.At(r))) }
	})
	set(SkillBastionGuard, func(s *SkillDef) {
		s.ExtraOnSelf = []*EffectSpec{{Kind: EffectBuffBlock, Duration: Scale{8, 0}, Value: Scale{0.2, 0}}}
		s.Text = func(r int) string {
			return fmt.Sprintf("+%s de defesa e +20%% de chance de bloqueio (com escudo) por 8 s.", pct(Scale{0.4, 0.1}.At(r)))
		}
	})
	set(SkillBastionVigor, func(s *SkillDef) {
		s.MaxRank, s.PValue = 5, Scale{0.03, 0.03}
		s.Text = func(r int) string { return fmt.Sprintf("+%s de vida máxima.", pct(Scale{0.03, 0.03}.At(r))) }
	})
	set(SkillBastionLast, func(s *SkillDef) {
		m := knightTuning.last
		s.MaxRank, s.Mult = 3, m
		s.Text = func(r int) string {
			return fmt.Sprintf("Causa %s do ataque a até 6 m, atordoa os inimigos por 2 s e dá +60%% de defesa a você por 8 s.", pct(m.At(r)))
		}
	})
	for _, id := range []uint32{SkillHeraldStrike, SkillHeraldDawn, SkillHorizon, SkillHeraldSun} {
		Skills[id].Holy = true
	}
	set(SkillHeraldLight, func(s *SkillDef) {
		s.ExtraOnSelf = []*EffectSpec{{Kind: EffectBuffDef, Duration: Scale{15, 0}, Value: Scale{0.1, 0.05}}}
		s.ResOnAlly = 4
		s.Text = func(r int) string {
			v := pct(Scale{0.1, 0.05}.At(r))
			return fmt.Sprintf("Você e os aliados a até 12 m ganham +%s de ataque e +%s de defesa por 15 s.", v, v)
		}
	})
	set(SkillHeraldBless, func(s *SkillDef) {
		s.Name, s.ResOnAlly = "Bênção da Prontidão", 4
	})
	set(SkillHeraldSun, func(s *SkillDef) {
		s.MaxRank, s.Mult = 3, Scale{2.5, 0.3}
		s.Text = func(r int) string {
			return fmt.Sprintf("Causa %s do ataque a até 7 m e cega os inimigos (35%% mais erros) por 5 s.", pct(Scale{2.5, 0.3}.At(r)))
		}
	})
}

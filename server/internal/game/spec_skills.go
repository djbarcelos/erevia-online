package game

import "fmt"

// Ramos das especializações: 3 skills por caminho (níveis 20, 24 e 28), todas com
// mecânicas que já existiam na árvore (provocar, escudos, área, execução, golpes seguidos,
// investida, estandarte, praga, marca, prender, cegar, buffs de grupo e passivas genéricas).
// Branch = SpecBranch; Tier 0–2 dentro do caminho; o nó de cima é o do mesmo caminho.

func specTrees() []*SkillDef {
	var out []*SkillDef
	for _, list := range [][]*SkillDef{bastionTree(), heraldTree(), ragerTree(), warlordTree(), elementalistTree(), occultistTree(), cutthroatTree(), hunterTree()} {
		out = append(out, list...)
	}
	return out
}

// specSkill preenche classe, caminho e ramo das skills de um caminho.
func specSkill(spec string, list []*SkillDef) []*SkillDef {
	class := Specs[spec].Class
	for _, s := range list {
		s.Class, s.Spec, s.Branch = class, spec, SpecBranch
	}
	return list
}

func bastionTree() []*SkillDef {
	return specSkill("bastion", []*SkillDef{
		{ID: SkillBastionDefy, Name: "Desafio do Bastião", Tier: 0, MaxRank: 3,
			Target: TargetAreaSelf, Range: 7, MP: Scale{35, 0}, CD: Scale{25, 0}, Taunt: true,
			OnTarget: &EffectSpec{Kind: EffectTaunt, Duration: Scale{4, 0}},
			OnSelf:   &EffectSpec{Kind: EffectShield, Duration: Scale{10, 0}, Value: Scale{300, 150}},
			FX:       "kb_defy", Description: "Bate o escudo e chama todos para cima de si.",
			Text: func(r int) string {
				return fmt.Sprintf("Os inimigos a até 7 m são obrigados a atacar você por 4 s, e você ganha um escudo que absorve até %s de dano por 10 s.",
					num(Scale{300, 150}.At(r)))
			}},
		{ID: SkillBastionVigor, Name: "Vigor do Bastião", Tier: 1, MaxRank: 3,
			Passive: PassiveMaxHPPct, PValue: Scale{0.05, 0.05}, FX: "kb_vigor",
			Description: "Quem segura a linha aprende a não cair.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de vida máxima.", pct(Scale{0.05, 0.05}.At(r))) }},
		{ID: SkillBastionLast, Name: "Último Bastião", Tier: 2, MaxRank: 1,
			Target: TargetAreaSelf, Range: 6, MP: Scale{60, 0}, CD: Scale{60, 0}, Mult: Scale{1.6, 0},
			OnTarget: &EffectSpec{Kind: EffectStun, Duration: Scale{2, 0}},
			OnSelf:   &EffectSpec{Kind: EffectBuffDef, Duration: Scale{8, 0}, Value: Scale{0.6, 0}},
			FX:       "kb_last", Description: "Finca os pés: daqui ninguém passa.",
			Text: func(int) string {
				return "Causa 160% do ataque a até 6 m, atordoa os inimigos por 2 s e dá +60% de defesa a você por 8 s."
			}},
	})
}

func heraldTree() []*SkillDef {
	return specSkill("herald", []*SkillDef{
		{ID: SkillHeraldDawn, Name: "Martelo da Aurora", Tier: 0, MaxRank: 3,
			Target: TargetEnemy, Range: 3, Splash: 3, MP: Scale{30, 0}, CD: Scale{14, 0}, Mult: Scale{1.9, 0.3}, NoMiss: true,
			OnTarget: &EffectSpec{Kind: EffectWeaken, Duration: Scale{6, 0}, Value: Scale{0.15, 0.05}},
			FX:       "kh_dawn", Description: "Um golpe que desce como o primeiro raio do dia.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque no alvo e em todos a até 3 m dele, sem errar; os atingidos perdem %s de ataque por 6 s.",
					pct(Scale{1.9, 0.3}.At(r)), pct(Scale{0.15, 0.05}.At(r)))
			}},
		{ID: SkillHeraldBless, Name: "Bênção do Arauto", Tier: 1, MaxRank: 3,
			Target: TargetSelf, MP: Scale{40, 0}, CD: Scale{40, 0}, AllyRadius: 12,
			OnSelf: &EffectSpec{Kind: EffectBuffAtkSpeed, Duration: Scale{15, 0}, Value: Scale{0.15, 0.05}},
			FX:     "kh_bless", Description: "A luz da ordem apressa as mãos de quem luta ao seu lado.",
			Text: func(r int) string {
				return fmt.Sprintf("Você e os aliados a até 12 m ganham +%s de velocidade de ataque por 15 s.", pct(Scale{0.15, 0.05}.At(r)))
			}},
		{ID: SkillHeraldSun, Name: "Sol Nascente", Tier: 2, MaxRank: 1,
			Target: TargetAreaSelf, Range: 7, MP: Scale{60, 0}, CD: Scale{50, 0}, Mult: Scale{2.5, 0},
			OnTarget: &EffectSpec{Kind: EffectBlind, Duration: Scale{5, 0}, Value: Scale{0.35, 0}},
			FX:       "kh_sun", Description: "Ergue a arma e o campo inteiro se acende.",
			Text: func(int) string {
				return "Causa 250% do ataque a até 7 m e cega os inimigos: eles erram 35% mais por 5 s."
			}},
	})
}

func ragerTree() []*SkillDef {
	return specSkill("bloodrager", []*SkillDef{
		{ID: SkillRagerRed, Name: "Golpe Rubro", Tier: 0, MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, MP: Scale{25, 0}, CD: Scale{10, 0}, Mult: Scale{2.6, 0.35},
			ExecuteBelow: 0.5, ExecuteMult: 1.3,
			FX: "bb_red", Description: "Quanto mais o inimigo sangra, mais fundo o machado entra.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque; 30%% a mais em alvos abaixo de 50%% de vida.", pct(Scale{2.6, 0.35}.At(r)))
			}},
		{ID: SkillRagerBlind, Name: "Fúria Cega", Tier: 1, MaxRank: 3,
			Passive: PassiveCrit, PValue: Scale{0.03, 0.03}, FX: "bb_blind",
			Description: "Não mira: acerta onde dói.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de chance de crítico.", pct(Scale{0.03, 0.03}.At(r))) }},
		{ID: SkillRagerCarnage, Name: "Carnificina", Tier: 2, MaxRank: 1,
			Target: TargetAreaSelf, Range: 5, MP: Scale{60, 0}, CD: Scale{45, 0}, Mult: Scale{1.1, 0}, Hits: 3,
			FX: "bb_carnage", Description: "Três voltas de machado, sem olhar para trás.",
			Text: func(int) string {
				return "Três golpes de 110% do ataque em todos os inimigos a até 5 m (a Sede de Sangue cura a cada golpe)."
			}},
	})
}

func warlordTree() []*SkillDef {
	return specSkill("warlord", []*SkillDef{
		{ID: SkillWarlordCry, Name: "Brado do Caudilho", Tier: 0, MaxRank: 3,
			Target: TargetSelf, MP: Scale{30, 0}, CD: Scale{35, 0}, AllyRadius: 12,
			OnSelf: &EffectSpec{Kind: EffectWarCry, Duration: Scale{20, 0}, Value: Scale{0.12, 0.04}},
			FX:     "bw_cry", Description: "Um brado que põe a tropa inteira em marcha.",
			Text: func(r int) string {
				return fmt.Sprintf("Você e os aliados a até 12 m ganham +%s de ataque por 20 s.", pct(Scale{0.12, 0.04}.At(r)))
			}},
		{ID: SkillWarlordCharge, Name: "Avanço do Caudilho", Tier: 1, MaxRank: 3,
			Target: TargetEnemy, Range: 12, Splash: 4, MP: Scale{40, 0}, CD: Scale{22, 0}, Mult: Scale{1.8, 0.25}, Charge: true,
			OnTarget: &EffectSpec{Kind: EffectStun, Duration: Scale{1, 0.25}},
			FX:       "bw_charge", Description: "Abre caminho no meio da formação inimiga.",
			Text: func(r int) string {
				return fmt.Sprintf("Salta até o alvo (até 12 m), causa %s do ataque nele e em todos a até 4 m e atordoa por %s s.",
					pct(Scale{1.8, 0.25}.At(r)), num(Scale{1, 0.25}.At(r)))
			}},
		{ID: SkillWarlordBanner, Name: "Estandarte de Guerra", Tier: 2, MaxRank: 1,
			Target: TargetAreaSelf, Range: 6, MP: Scale{55, 0}, CD: Scale{75, 0}, Mult: Scale{1.5, 0},
			OnTarget: &EffectSpec{Kind: EffectWeaken, Duration: Scale{8, 0}, Value: Scale{0.2, 0}},
			Banner:   &BannerDef{Radius: 8, Duration: 20, Effect: EffectBanner, Value: 0.2},
			FX:       "bw_banner", Description: "Crava o estandarte no chão com um golpe que espalha o inimigo.",
			Text: func(int) string {
				return "Causa 150% do ataque a até 6 m (os inimigos perdem 20% de ataque por 8 s) e finca um estandarte por 20 s: você e os aliados a até 8 m dele ganham +20% de defesa."
			}},
	})
}

func elementalistTree() []*SkillDef {
	return specSkill("elementalist", []*SkillDef{
		{ID: SkillElemLance, Name: "Lança Ígnea", Tier: 0, MaxRank: 3,
			Target: TargetEnemy, Range: 20, MP: Scale{28, 0}, CD: Scale{6, 0}, Mult: Scale{2.6, 0.35}, Magic: true, Fire: true,
			FX: "me_lance", Description: "Fogo comprimido numa lança que atravessa armaduras.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque mágico no alvo (é uma skill de fogo: a Brasa Viva queima).", pct(Scale{2.6, 0.35}.At(r)))
			}},
		{ID: SkillElemFocus, Name: "Foco Elemental", Tier: 1, MaxRank: 3,
			Passive: PassiveCritDamage, PValue: Scale{0.15, 0.15}, FX: "me_focus",
			Description: "Cada elemento obedece mais à vontade do mago.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de dano crítico.", pct(Scale{0.15, 0.15}.At(r))) }},
		{ID: SkillElemCata, Name: "Cataclismo", Tier: 2, MaxRank: 1,
			Target: TargetEnemy, Range: 20, Splash: 6, MP: Scale{90, 0}, CD: Scale{60, 0}, Mult: Scale{4, 0}, Magic: true, Fire: true,
			OnTarget: &EffectSpec{Kind: EffectSlow, Duration: Scale{4, 0}, Value: Scale{0.5, 0}},
			FX:       "me_cata", Description: "Fogo do céu e gelo no chão, ao mesmo tempo.",
			Text: func(int) string {
				return "Causa 400% do ataque mágico no alvo e em todos a até 6 m dele, que ficam 50% mais lentos por 4 s."
			}},
	})
}

func occultistTree() []*SkillDef {
	return specSkill("occultist", []*SkillDef{
		{ID: SkillOccRot, Name: "Peste Sombria", Tier: 0, MaxRank: 3,
			Target: TargetEnemy, Range: 18, MP: Scale{30, 0}, CD: Scale{12, 0}, Mult: Scale{1.2, 0.2}, Magic: true,
			OnTarget: &EffectSpec{Kind: EffectPlague, Duration: Scale{8, 0}, Value: Scale{0.2, 0.05}, AtkFrac: true},
			FX:       "mo_rot", Description: "Uma praga das sombras que passa de corpo em corpo.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque mágico e infecta o alvo: %s do ataque mágico por segundo por 8 s. Se ele morrer infectado, a peste pula para os inimigos a até 6 m.",
					pct(Scale{1.2, 0.2}.At(r)), pct(Scale{0.2, 0.05}.At(r)))
			}},
		{ID: SkillOccDrain, Name: "Drenar Vontade", Tier: 1, MaxRank: 3,
			Target: TargetEnemy, Range: 16, MP: Scale{30, 0}, CD: Scale{14, 0}, Mult: Scale{1.6, 0.25}, Magic: true,
			OnTarget: &EffectSpec{Kind: EffectWeaken, Duration: Scale{8, 0}, Value: Scale{0.2, 0.05}},
			FX:       "mo_drain", Description: "Arranca a força de vontade do inimigo.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque mágico e o alvo perde %s de ataque por 8 s.", pct(Scale{1.6, 0.25}.At(r)), pct(Scale{0.2, 0.05}.At(r)))
			}},
		{ID: SkillOccAbyss, Name: "Marca do Abismo", Tier: 2, MaxRank: 1,
			Target: TargetEnemy, Range: 18, Splash: 5, MP: Scale{70, 0}, CD: Scale{45, 0}, Mult: Scale{1.5, 0}, Magic: true,
			OnTarget: &EffectSpec{Kind: EffectMark, Duration: Scale{10, 0}, Value: Scale{0.2, 0}},
			FX:       "mo_abyss", Description: "O abismo olha de volta para quem foi marcado.",
			Text: func(int) string {
				return "Causa 150% do ataque mágico no alvo e em todos a até 5 m dele; os atingidos recebem +20% de dano de todas as fontes por 10 s."
			}},
	})
}

func cutthroatTree() []*SkillDef {
	return specSkill("cutthroat", []*SkillDef{
		{ID: SkillCutBlades, Name: "Lâmina Oculta", Tier: 0, MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, MP: Scale{28, 0}, CD: Scale{10, 0}, Mult: Scale{1, 0.15}, Hits: 3, ExtraCrit: 0.2,
			FX: "rc_blades", Description: "Três cortes antes que o alvo perceba o primeiro.",
			Text: func(r int) string {
				return fmt.Sprintf("Três golpes de %s do ataque, com +20%% de chance de crítico.", pct(Scale{1, 0.15}.At(r)))
			}},
		{ID: SkillCutInstinct, Name: "Instinto Assassino", Tier: 1, MaxRank: 3,
			Passive: PassiveCritDamage, PValue: Scale{0.1, 0.1}, FX: "rc_instinct",
			Description: "Sabe que golpe termina a luta.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de dano crítico.", pct(Scale{0.1, 0.1}.At(r))) }},
		{ID: SkillCutSentence, Name: "Sentença", Tier: 2, MaxRank: 1,
			Target: TargetEnemy, Range: 12, MP: Scale{60, 0}, CD: Scale{50, 0}, Mult: Scale{4.5, 0}, Charge: true, NoMiss: true,
			ExecuteBelow: 0.3, ExecuteMult: 1.5,
			FX: "rc_sentence", Description: "O contrato foi assinado: só falta cumprir.",
			Text: func(int) string {
				return "Salta até o alvo (até 12 m) e causa 450% do ataque, sem errar; 50% a mais em alvos abaixo de 30% de vida."
			}},
	})
}

func hunterTree() []*SkillDef {
	return specSkill("hunter", []*SkillDef{
		{ID: SkillHuntKnife, Name: "Faca de Arremesso", Tier: 0, MaxRank: 3,
			Target: TargetEnemy, Range: 16, MP: Scale{20, 0}, CD: Scale{6, 0}, Mult: Scale{1.8, 0.25},
			OnTarget: &EffectSpec{Kind: EffectSlow, Duration: Scale{3, 0}, Value: Scale{0.3, 0}},
			FX:       "rh_knife", Description: "Uma faca na perna faz qualquer presa andar devagar.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque à distância (até 16 m) e deixa o alvo 30%% mais lento por 3 s.", pct(Scale{1.8, 0.25}.At(r)))
			}},
		{ID: SkillHuntNet, Name: "Rede de Caça", Tier: 1, MaxRank: 3,
			Target: TargetEnemy, Range: 14, Splash: 3, MP: Scale{30, 0}, CD: Scale{20, 0}, Mult: Scale{0.6, 0},
			OnTarget: &EffectSpec{Kind: EffectRoot, Duration: Scale{2.5, 0.5}},
			FX:       "rh_net", Description: "Uma rede com pesos nas pontas, para mais de uma presa.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa 60%% do ataque no alvo e em todos a até 3 m dele, que ficam presos no lugar por %s s.", num(Scale{2.5, 0.5}.At(r)))
			}},
		{ID: SkillHuntRain, Name: "Chuva de Lâminas", Tier: 2, MaxRank: 1,
			Target: TargetEnemy, Range: 18, Splash: 5, MP: Scale{60, 0}, CD: Scale{40, 0}, Mult: Scale{1.6, 0}, Hits: 2,
			FX: "rh_rain", Description: "Um punhado de facas para o alto, e o céu faz o resto.",
			Text: func(int) string {
				return "Dois golpes de 160% do ataque no alvo e em todos a até 5 m dele."
			}},
	})
}

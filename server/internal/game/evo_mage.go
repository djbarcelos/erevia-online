package game

import (
	"fmt"
	"strings"
)

// Conteúdo das evoluções do Mago (docs/evolucoes/elementalist.md e occultist.md): as skills
// novas (151–155, 157–166 e 167–184) e os ajustes nas que vieram do degrau 20 e da
// especialização. Os números são os dos docs; a calibragem vem do simulador.

const (
	SkillElemEmber     uint32 = 151 // Brasa Viva (evol.)
	SkillElemLinger    uint32 = 152 // Chama Persistente (modificador da Bola de Fogo)
	SkillElemRain      uint32 = 153 // Chuva de Brasas (evol.)
	SkillElemMeteor    uint32 = 154 // Meteoro (evol.)
	SkillElemVolcano   uint32 = 155 // Coração do Vulcão (chave)
	SkillElemIceLance  uint32 = 157 // Lança de Gelo (evol.)
	SkillElemBiting    uint32 = 158 // Frio Penetrante
	SkillElemWinter    uint32 = 159 // Inverno Eterno (modificador da Nova)
	SkillElemPrison    uint32 = 160 // Prisão Glacial (evol.)
	SkillElemArmor     uint32 = 161 // Armadura de Gelo (evol.)
	SkillElemFrostCore uint32 = 162 // Coração do Inverno (chave)
	SkillElemOverload  uint32 = 163 // Sobrecarga (evol.)
	SkillElemReaction  uint32 = 164 // Reação Elemental
	SkillElemConverge  uint32 = 165 // Convergência (modificador da Reação)
	SkillElemRupture   uint32 = 166 // Ruptura Elemental (chave)
	SkillOccAgony      uint32 = 167 // Agonia
	SkillOccRuin       uint32 = 168 // Ruína (modificador do Drenar)
	SkillOccCurse      uint32 = 169 // Maldição Persistente
	SkillOccWeakness   uint32 = 170 // Fraqueza
	SkillOccAbyssCore  uint32 = 171 // Abismo Interior (chave)
	SkillOccPlague     uint32 = 172 // Praga Arcana
	SkillOccContagion  uint32 = 173 // Contágio
	SkillOccFever      uint32 = 174 // Febre (modificador da Peste)
	SkillOccCloud      uint32 = 175 // Nuvem Pestilenta
	SkillOccRot2       uint32 = 176 // Putrefação
	SkillOccPandemic   uint32 = 177 // Pandemia (chave)
	SkillOccBlink      uint32 = 178 // Piscar (evol.)
	SkillOccShield     uint32 = 179 // Escudo Arcano (evol.)
	SkillOccMind       uint32 = 180 // Mente Afiada
	SkillOccBanish     uint32 = 181 // Prisão Etérea
	SkillOccFocused    uint32 = 182 // Rajada Concentrada (modificador da Rajada)
	SkillOccFlow       uint32 = 183 // Fluxo Arcano
	SkillOccCollapse   uint32 = 184 // Colapso Arcano (chave)
)

func mageEvoSkills() []*SkillDef {
	const e, o = "elementalist", "occultist"
	return []*SkillDef{
		// ---------- Elementalista: Chamas ----------
		evolvedNode(SkillEmber, SkillElemEmber, e, 0, 20, nil),
		node(e, 0, 25, NodeModifier, &SkillDef{ID: SkillElemLinger, Name: "Chama Persistente", MaxRank: 3,
			Modifies: SkillElemFireball, Mod: &SkillMod{BurnExtra: Scale{1, 1}}, FX: "kh_sun",
			Description: "A bola de fogo deixa o alvo queimando mais tempo.",
			Text: func(k int) string {
				return fmt.Sprintf("A queimadura da Bola de Fogo dura +%s s.", num(Scale{1, 1}.At(k)))
			}}),
		evolvedNode(SkillEmberRain, SkillElemRain, e, 0, 30, nil),
		evolvedNode(SkillMeteor, SkillElemMeteor, e, 0, 35, func(s *SkillDef) {
			s.MaxRank, s.Mult, s.MP, s.CD, s.ResCost = 3, Scale{5, 0.5}, Scale{50, 0}, Scale{50, 0}, 30
			s.Text = func(k int) string {
				return fmt.Sprintf("Causa %s do ataque mágico no alvo e a até 3 m e atordoa por 2 s.", pct(Scale{5, 0.5}.At(k)))
			}
		}),
		node(e, 0, 40, NodeKey, &SkillDef{ID: SkillElemVolcano, Name: "Coração do Vulcão", MaxRank: 1,
			Passive: PassiveFireHeart, PValue: Scale{1, 0}, FX: "b_exec",
			Description: "O fogo procura o fogo.",
			Text: func(int) string {
				return "Skills de fogo em alvo queimado causam +20% e espalham a queimadura para quem está a até 3 m."
			}}),
		// ---------- Elementalista: Geada ----------
		evolvedNode(SkillIceLance, SkillElemIceLance, e, 1, 20, nil),
		node(e, 1, 20, NodePassive, &SkillDef{ID: SkillElemBiting, Name: "Frio Penetrante", MaxRank: 5,
			Passive: PassiveSlowedDamagePct, PValue: Scale{0.02, 0.02}, FX: "r_dodge",
			Description: "O frio deixa tudo mais frágil.",
			Text: func(k int) string {
				return fmt.Sprintf("Alvos lentos recebem +%s de dano seu.", pct(Scale{0.02, 0.02}.At(k)))
			}}),
		node(e, 1, 25, NodeModifier, &SkillDef{ID: SkillElemWinter, Name: "Inverno Eterno", MaxRank: 3,
			Modifies: SkillElemNova, Mod: &SkillMod{EffectDur: Scale{1, 1}}, FX: "b_stone",
			Description: "O gelo da Nova demora a derreter.",
			Text: func(k int) string {
				return fmt.Sprintf("A lentidão da Nova de Gelo dura +%s s.", num(Scale{1, 1}.At(k)))
			}}),
		evolvedNode(SkillGlacialPrison, SkillElemPrison, e, 1, 30, func(s *SkillDef) {
			s.MaxRank, s.Mult = 3, Scale{0.5, 0.15}
			s.OnTarget = &EffectSpec{Kind: EffectRoot, Duration: Scale{3, 0.5}}
			s.Text = func(k int) string {
				return fmt.Sprintf("Causa %s do ataque mágico e prende o alvo no lugar por %s s.", pct(Scale{0.5, 0.15}.At(k)), num(Scale{3, 0.5}.At(k)))
			}
		}),
		evolvedNode(SkillIceArmor, SkillElemArmor, e, 1, 35, nil),
		node(e, 1, 40, NodeKey, &SkillDef{ID: SkillElemFrostCore, Name: "Coração do Inverno", MaxRank: 1,
			Passive: PassiveWinterHeart, PValue: Scale{1, 0}, FX: "k_wall",
			Description: "O que está parado quebra fácil.",
			Text: func(int) string {
				return "Seus golpes em alvos congelados (atordoados) ou presos são críticos garantidos."
			}}),
		// ---------- Elementalista: Convergência ----------
		node(e, 2, 20, NodeActive, &SkillDef{ID: SkillElemReaction, Name: "Reação Elemental", MaxRank: 3,
			Target: TargetEnemy, Range: 18, MP: Scale{30, 0}, CD: Scale{10, 0}, Mult: Scale{1.5, 0.25}, Magic: true,
			Reaction: true, ResOnReaction: 10, FX: "k_spin",
			Description: "Fogo e gelo no mesmo alvo explodem.",
			Text: func(k int) string {
				return fmt.Sprintf("Causa %s do ataque mágico; o dobro se o alvo estiver queimado e lento (consome os dois).", pct(Scale{1.5, 0.25}.At(k)))
			}}),
		node(e, 2, 25, NodeModifier, &SkillDef{ID: SkillElemConverge, Name: "Convergência", MaxRank: 3,
			Modifies: SkillElemReaction, Mod: &SkillMod{Splash: Scale{1, 1}}, FX: "b_quake",
			Description: "A reação transborda.",
			Text: func(k int) string {
				return fmt.Sprintf("A Reação Elemental atinge também a até %s m do alvo.", num(Scale{1, 1}.At(k)))
			}}),
		evolvedNode(SkillOverload, SkillElemOverload, e, 2, 35, func(s *SkillDef) {
			s.MaxRank, s.PValue = 3, Scale{0.05, 0.05}
			s.Text = func(k int) string {
				return fmt.Sprintf("+%s de ataque mágico; as skills custam 10%% a mais de mana.", pct(Scale{0.05, 0.05}.At(k)))
			}
		}),
		node(e, 2, 40, NodeKey, &SkillDef{ID: SkillElemRupture, Name: "Ruptura Elemental", MaxRank: 1,
			Target: TargetAreaSelf, Range: 6, MP: Scale{40, 0}, CD: Scale{45, 0}, Mult: Scale{3, 0}, Magic: true,
			ResSpendAll: true, ResPerPoint: 0.02,
			OnTarget:      &EffectSpec{Kind: EffectBurn, Duration: Scale{5, 0}, Value: Scale{0.25, 0}, AtkFrac: true},
			ExtraOnTarget: []*EffectSpec{{Kind: EffectSlow, Duration: Scale{4, 0}, Value: Scale{0.5, 0}}},
			FX:            "b_seismic", Description: "Toda a concentração solta de uma vez.",
			Text: func(int) string {
				return "Causa 300% do ataque mágico a até 6 m, +2% por ponto de Concentração gasto (toda); queima e deixa lento."
			}}),

		// ---------- Arcanista: Maldições ----------
		node(o, 0, 20, NodePassive, &SkillDef{ID: SkillOccAgony, Name: "Agonia", MaxRank: 5,
			Passive: PassiveCursedDamagePct, PValue: Scale{0.02, 0.02}, FX: "r_para",
			Description: "Quem carrega a maldição sofre mais.",
			Text: func(k int) string {
				return fmt.Sprintf("Alvos amaldiçoados (enfraquecidos, marcados, sem defesa ou sob a Maldição) recebem +%s de dano seu.", pct(Scale{0.02, 0.02}.At(k)))
			}}),
		node(o, 0, 25, NodeModifier, &SkillDef{ID: SkillOccRuin, Name: "Ruína", MaxRank: 3,
			Modifies: SkillOccDrain, Mod: &SkillMod{EffectDur: Scale{1, 1}}, FX: "r_weak",
			Description: "A vontade drenada demora a voltar.",
			Text: func(k int) string {
				return fmt.Sprintf("O enfraquecimento do Drenar Vontade dura +%s s.", num(Scale{1, 1}.At(k)))
			}}),
		node(o, 0, 30, NodeActive, &SkillDef{ID: SkillOccCurse, Name: "Maldição Persistente", MaxRank: 3,
			Target: TargetEnemy, Range: 18, MP: Scale{30, 0}, CD: Scale{15, 0}, Magic: true, RenewCurses: true,
			OnTarget: &EffectSpec{Kind: EffectCurse, Duration: Scale{12, 0}, Value: Scale{0.2, 0.05}, AtkFrac: true},
			FX:       "r_plague", Description: "Uma maldição que alimenta as outras.",
			Text: func(k int) string {
				return fmt.Sprintf("Por 12 s, %s do ataque mágico por segundo; ao acertar, renova as outras maldições suas no alvo.", pct(Scale{0.2, 0.05}.At(k)))
			}}),
		node(o, 0, 35, NodeActive, &SkillDef{ID: SkillOccWeakness, Name: "Fraqueza", MaxRank: 3,
			Target: TargetEnemy, Range: 18, MP: Scale{20, 0}, CD: Scale{20, 0}, ResCost: 20, Magic: true,
			OnTarget: &EffectSpec{Kind: EffectArmorBreak, Duration: Scale{8, 0}, Value: Scale{0.1, 0.05}},
			FX:       "r_mark", Description: "Quebra a defesa por dentro.",
			Text: func(k int) string {
				return fmt.Sprintf("O alvo perde %s de defesa por 8 s.", pct(Scale{0.1, 0.05}.At(k)))
			}}),
		node(o, 0, 40, NodeKey, &SkillDef{ID: SkillOccAbyssCore, Name: "Abismo Interior", MaxRank: 1,
			Passive: PassiveDeepAbyss, PValue: Scale{1, 0}, FX: "b_undying",
			Description: "O abismo não solta o que pegou.",
			Text:        func(int) string { return "Suas maldições (enfraquecer, marca, Fraqueza e Maldição) duram +50%." }}),
		// ---------- Arcanista: Pragas ----------
		node(o, 1, 20, NodeActive, &SkillDef{ID: SkillOccPlague, Name: "Praga Arcana", MaxRank: 3,
			Target: TargetEnemy, Range: 18, MP: Scale{25, 0}, CD: Scale{10, 0}, Magic: true,
			OnTarget: &EffectSpec{Kind: EffectPlague, Duration: Scale{10, 0}, Value: Scale{0.15, 0.05}, AtkFrac: true},
			FX:       "r_poison", Description: "Uma praga que corre de um inimigo para o outro.",
			Text: func(k int) string {
				return fmt.Sprintf("Praga de %s do ataque mágico por segundo, por 10 s; se o alvo morrer, ela pula para os vizinhos.", pct(Scale{0.15, 0.05}.At(k)))
			}}),
		node(o, 1, 20, NodePassive, &SkillDef{ID: SkillOccContagion, Name: "Contágio", MaxRank: 5,
			Passive: PassiveSpreadPower, PValue: Scale{0.04, 0.04}, FX: "r_flask",
			Description: "A praga fica mais forte a cada salto.",
			Text: func(k int) string {
				return fmt.Sprintf("A praga que pula leva +%s de força.", pct(Scale{0.04, 0.04}.At(k)))
			}}),
		node(o, 1, 25, NodeModifier, &SkillDef{ID: SkillOccFever, Name: "Febre", MaxRank: 3,
			Modifies: SkillOccRot, Mod: &SkillMod{EffectDur: Scale{2, 2}}, FX: "r_twin",
			Description: "A peste não passa.",
			Text:        func(k int) string { return fmt.Sprintf("A praga da Peste Sombria dura +%s s.", num(Scale{2, 2}.At(k))) }}),
		node(o, 1, 30, NodeActive, &SkillDef{ID: SkillOccCloud, Name: "Nuvem Pestilenta", MaxRank: 3,
			Target: TargetSelf, MP: Scale{40, 0}, CD: Scale{20, 0}, Magic: true,
			Zone: &ZoneDef{Radius: 5, Duration: 6, Period: 1, Damage: Scale{0.25, 0.05}, Magic: true},
			FX:   "r_miasma", Description: "Uma nuvem doente no chão.",
			Text: func(k int) string {
				return fmt.Sprintf("Nuvem de 5 m por 6 s: %s do ataque mágico por segundo nos inimigos dentro.", pct(Scale{0.25, 0.05}.At(k)))
			}}),
		node(o, 1, 35, NodePassive, &SkillDef{ID: SkillOccRot2, Name: "Putrefação", MaxRank: 3,
			Passive: PassiveDotCrit, PValue: Scale{0.05, 0.05}, FX: "r_smoke",
			Description: "Às vezes a praga morde fundo.",
			Text: func(k int) string {
				return fmt.Sprintf("As pragas e maldições podem dar crítico (+%s de chance).", pct(Scale{0.05, 0.05}.At(k)))
			}}),
		node(o, 1, 40, NodeKey, &SkillDef{ID: SkillOccPandemic, Name: "Pandemia", MaxRank: 1,
			Target: TargetEnemy, Range: 18, CD: Scale{45, 0}, ResCost: 60, Magic: true, Pandemic: true,
			FX: "r_vanish", Description: "O que um tem, todos pegam.",
			Text: func(int) string {
				return "Espalha todas as pragas e maldições do alvo para os inimigos a até 8 m."
			}}),
		// ---------- Arcanista: Arcano ----------
		evolvedNode(SkillBlink, SkillOccBlink, o, 2, 20, nil),
		evolvedNode(SkillArcaneShield, SkillOccShield, o, 2, 20, nil),
		node(o, 2, 20, NodePassive, &SkillDef{ID: SkillOccMind, Name: "Mente Afiada", MaxRank: 5,
			Passive: PassiveMPEfficiency, PValue: Scale{0.02, 0.02}, FX: "m_mind",
			Description: "Cada feitiço no tamanho certo.",
			Text: func(k int) string {
				return fmt.Sprintf("−%s no custo de mana das skills.", pct(Scale{0.02, 0.02}.At(k)))
			}}),
		node(o, 2, 25, NodeActive, &SkillDef{ID: SkillOccBanish, Name: "Prisão Etérea", MaxRank: 3,
			Target: TargetEnemy, Range: 15, MP: Scale{30, 0}, CD: Scale{25, 0}, Magic: true,
			OnTarget: &EffectSpec{Kind: EffectBanish, Duration: Scale{2, 0.5}},
			FX:       "m_over", Description: "Tira o alvo deste mundo por um instante.",
			Text: func(k int) string {
				return fmt.Sprintf("O alvo some de cena por %s s: não age, não bate e não apanha (não funciona em chefe).", num(Scale{2, 0.5}.At(k)))
			}}),
		node(o, 2, 30, NodeModifier, &SkillDef{ID: SkillOccFocused, Name: "Rajada Concentrada", MaxRank: 3,
			Modifies: SkillArcaneBlast, Mod: &SkillMod{ResBonusCost: 20, ResBonusPct: Scale{0.3, 0.15}}, FX: "m_lance",
			Description: "A carga vira força pura.",
			Text: func(k int) string {
				return fmt.Sprintf("Com 20 de Carga, a Rajada Arcana gasta a Carga e causa +%s.", pct(Scale{0.3, 0.15}.At(k)))
			}}),
		node(o, 2, 35, NodePassive, &SkillDef{ID: SkillOccFlow, Name: "Fluxo Arcano", MaxRank: 3,
			Passive: PassiveManaOnSpend, PValue: Scale{0.02, 0.01}, FX: "m_zero",
			Description: "A carga gasta volta como mana.",
			Text: func(k int) string {
				return fmt.Sprintf("Cada 10 de Carga gasta devolve %s da mana máxima.", pct(Scale{0.02, 0.01}.At(k)))
			}}),
		node(o, 2, 40, NodeKey, &SkillDef{ID: SkillOccCollapse, Name: "Colapso Arcano", MaxRank: 1,
			Target: TargetAreaSelf, Range: 6, MP: Scale{40, 0}, CD: Scale{40, 0}, Mult: Scale{2.5, 0}, Magic: true,
			ResSpendAll: true, ResPerPoint: 0.03, FX: "m_meteor",
			Description: "A carga inteira desaba sobre todos.",
			Text: func(int) string {
				return "Explosão de 250% do ataque mágico a até 6 m, +3% por ponto de Carga gasto (toda)."
			}}),
	}
}

// mageTuning: calibragem no simulador (metas dos docs: Elementalista entre −25% e −10% do
// tempo para matar médio, Arcanista entre −15% e 0%; o Mago base já matava rápido demais).
// O dano das skills de dano do Elementalista é multiplicado por elemDamage e o custo de
// mana por elemMana (a mana passa a pesar: ele precisa escolher o que lançar).
var mageTuning = struct{ elemDamage, elemMana, occDamage float64 }{elemDamage: 0.5, elemMana: 1.5, occDamage: 0.85}

// retune multiplica o dano e o custo de mana de uma skill e corrige o texto (o primeiro
// "N%" do dano vira o novo valor).
func retune(s *SkillDef, damage, mana float64) {
	old, text := s.Mult, s.Text
	s.Mult = Scale{old.Base * damage, old.Per * damage}
	s.MP = Scale{s.MP.Base * mana, s.MP.Per * mana}
	if damage != 1 && old != (Scale{}) && text != nil {
		s.Text = func(k int) string { return strings.Replace(text(k), pct(old.At(k)), pct(s.Mult.At(k)), 1) }
	}
}

// adjustMageEvolutions: os ajustes dos docs nas skills que já existiam.
func adjustMageEvolutions() {
	set := func(id uint32, f func(*SkillDef)) { f(Skills[id]) }
	defer func() { // depois dos ajustes do doc: a calibragem vale para a árvore inteira
		for _, s := range EvolutionTree("elementalist") {
			if !s.IsPassive() && s.Mult.At(1) > 0 {
				retune(s, mageTuning.elemDamage, mageTuning.elemMana)
			}
		}
		for _, s := range EvolutionTree("occultist") {
			if !s.IsPassive() && s.Mult.At(1) > 0 && mageTuning.occDamage != 1 {
				retune(s, mageTuning.occDamage, 1)
			}
		}
	}()
	set(SkillElemFocus, func(s *SkillDef) {
		s.MaxRank, s.PValue = 5, Scale{0.09, 0.09}
		s.Text = func(k int) string { return fmt.Sprintf("+%s de dano crítico.", pct(Scale{0.09, 0.09}.At(k))) }
	})
	set(SkillElemCata, func(s *SkillDef) {
		s.MaxRank, s.Mult, s.MP = 3, Scale{4, 0.4}, Scale{60, 0}
		s.Text = func(k int) string {
			return fmt.Sprintf("Causa %s do ataque mágico no alvo e a até 6 m, com 50%% de lentidão por 4 s.", pct(Scale{4, 0.4}.At(k)))
		}
	})
	set(SkillOccAbyss, func(s *SkillDef) {
		s.MaxRank, s.Mult = 3, Scale{1.5, 0.2}
		s.Text = func(k int) string {
			return fmt.Sprintf("Causa %s do ataque mágico no alvo e a até 5 m; os atingidos recebem +20%% de dano de todas as fontes por 10 s.", pct(Scale{1.5, 0.2}.At(k)))
		}
	})
}

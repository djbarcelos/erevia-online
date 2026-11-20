package game

import "fmt"

// Conteúdo das evoluções do Bárbaro (docs/evolucoes/bloodrager.md e warlord.md): as skills
// novas (118–122, 124–132 e 133–137, 139–149) e os ajustes nas que vieram do degrau 20 e da
// especialização. Os números são os dos docs; a calibragem vem do simulador.

const (
	SkillRagerHeavy      uint32 = 118 // Machado Pesado (evol.)
	SkillRagerCrush      uint32 = 119 // Golpe Esmagador (modificador do Golpe Brutal)
	SkillRagerRupture    uint32 = 120 // Ruptura
	SkillRagerTremor     uint32 = 121 // Abalo (modificador da Machadada)
	SkillRagerBoundless  uint32 = 122 // Ira Desmedida (chave)
	SkillRagerThirst     uint32 = 124 // Sede de Sangue (evol.)
	SkillRagerFrenzy     uint32 = 125 // Frenesi (evol.)
	SkillRagerVortex     uint32 = 126 // Turbilhão (modificador do Redemoinho)
	SkillRagerSequence   uint32 = 127 // Sequência Selvagem
	SkillRagerUnchained  uint32 = 128 // Fúria Incontrolável
	SkillRagerInsatiate  uint32 = 129 // Sede Insaciável (chave)
	SkillRagerPrey       uint32 = 130 // Marca da Presa
	SkillRagerExecute    uint32 = 131 // Execução (evol.)
	SkillRagerFinale     uint32 = 132 // Carnificina Final (chave)
	SkillWarlordLead     uint32 = 133 // Liderança
	SkillWarlordAdvance  uint32 = 134 // Ordem de Avanço (modificador do Brado)
	SkillWarlordRally    uint32 = 135 // Grito de Reagrupar
	SkillWarlordFlag     uint32 = 136 // Estandarte Tremulante (modificador do Estandarte)
	SkillWarlordCommand2 uint32 = 137 // Comando Absoluto (chave)
	SkillWarlordArm      uint32 = 139 // Braço de Guerra
	SkillWarlordBreak    uint32 = 140 // Quebra-Linha (modificador do Avanço)
	SkillWarlordImpact   uint32 = 141 // Impacto
	SkillWarlordMomentum uint32 = 142 // Ímpeto de Carga
	SkillWarlordRelent   uint32 = 143 // Investida Implacável (chave)
	SkillWarlordRoar     uint32 = 144 // Rugido (evol.)
	SkillWarlordHide     uint32 = 145 // Couro Grosso (evol.)
	SkillWarlordStomp    uint32 = 146 // Pisotear (evol.)
	SkillWarlordDefy     uint32 = 147 // Desafio
	SkillWarlordField    uint32 = 148 // Domínio do Campo (modificador do Pisotear)
	SkillWarlordPresence uint32 = 149 // Presença Dominante (chave)
)

func barbarianEvoSkills() []*SkillDef {
	const r, w = "bloodrager", "warlord"
	return []*SkillDef{
		// ---------- Berserker: Carnificina ----------
		evolvedNode(SkillHeavyAxe, SkillRagerHeavy, r, 0, 20, nil),
		node(r, 0, 25, NodeModifier, &SkillDef{ID: SkillRagerCrush, Name: "Golpe Esmagador", MaxRank: 3,
			Modifies: SkillRagerBrutal, Mod: &SkillMod{MultPct: Scale{0.1, 0.1}, ResOnCrit: Scale{5, 5}}, FX: "k_spin",
			Description: "O golpe brutal quebra o que encontra.",
			Text: func(k int) string {
				return fmt.Sprintf("Golpe Brutal: +%s de dano; no crítico, +%s de Fúria.", pct(Scale{0.1, 0.1}.At(k)), num(Scale{5, 5}.At(k)))
			}}),
		node(r, 0, 30, NodeActive, &SkillDef{ID: SkillRagerRupture, Name: "Ruptura", MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, CD: Scale{12, 0}, Mult: Scale{1.3, 0.2}, ResCost: 30,
			OnTarget: &EffectSpec{Kind: EffectBleed, Duration: Scale{6, 0}, Value: Scale{0.2, 0.1}, AtkFrac: true},
			FX:       "r_twin", Description: "Abre uma ferida que não fecha.",
			Text: func(k int) string {
				return fmt.Sprintf("Causa %s do ataque e um sangramento de %s do ataque por segundo, por 6 s.", pct(Scale{1.3, 0.2}.At(k)), pct(Scale{0.2, 0.1}.At(k)))
			}}),
		node(r, 0, 35, NodeModifier, &SkillDef{ID: SkillRagerTremor, Name: "Abalo", MaxRank: 3,
			Modifies: SkillSeismicAxe, FX: "b_stomp",
			Mod:         &SkillMod{Splash: Scale{1, 1}, ExtraOnTarget: []*EffectSpec{{Kind: EffectSlow, Duration: Scale{2, 1}, Value: Scale{0.3, 0}}}},
			Description: "O chão treme com a Machadada.",
			Text: func(k int) string {
				return fmt.Sprintf("Machadada Sísmica: +%s m de raio, e os atingidos ficam 30%% mais lentos por %s s.", num(Scale{1, 1}.At(k)), num(Scale{2, 1}.At(k)))
			}}),
		node(r, 0, 40, NodeKey, &SkillDef{ID: SkillRagerBoundless, Name: "Ira Desmedida", MaxRank: 1,
			Passive: PassiveResCostCut, PValue: Scale{0.3, 0}, FX: "m_over",
			Description: "A fúria não acaba nunca.",
			Text: func(int) string {
				return "Skills que gastam Fúria custam 30% menos e têm +15% de chance de crítico."
			}}),
		// ---------- Berserker: Frenesi ----------
		evolvedNode(SkillFrenzy, SkillRagerFrenzy, r, 1, 20, nil),
		evolvedNode(SkillBloodthirst, SkillRagerThirst, r, 1, 20, nil),
		node(r, 1, 25, NodeModifier, &SkillDef{ID: SkillRagerVortex, Name: "Turbilhão", MaxRank: 3,
			Modifies: SkillBloodWhirl, Mod: &SkillMod{MultPct: Scale{0.1, 0.05}, Hits: [4]int{0, 0, 0, 1}}, FX: "k_charge",
			Description: "O redemoinho não para.",
			Text: func(k int) string {
				t := fmt.Sprintf("Redemoinho Sangrento: +%s de dano", pct(Scale{0.1, 0.05}.At(k)))
				if k >= 3 {
					t += " e +1 golpe (3 golpes)"
				}
				return t + "."
			}}),
		node(r, 1, 30, NodePassive, &SkillDef{ID: SkillRagerSequence, Name: "Sequência Selvagem", MaxRank: 5,
			Passive: PassiveFrenzyStacks, PValue: Scale{0.02, 0.005}, FX: "r_dodge",
			Description: "Cada golpe chama o próximo.",
			Text: func(k int) string {
				return fmt.Sprintf("Cada ataque básico seguido no mesmo alvo dá +%s de velocidade de ataque, até 5 acúmulos (perde tudo depois de 3 s sem bater).", pct(Scale{0.02, 0.005}.At(k)))
			}}),
		node(r, 1, 35, NodeActive, &SkillDef{ID: SkillRagerUnchained, Name: "Fúria Incontrolável", MaxRank: 3,
			Target: TargetSelf, MP: Scale{20, 0}, CD: Scale{40, -5},
			OnSelf:      &EffectSpec{Kind: EffectFuryDouble, Duration: Scale{8, 0}, Value: Scale{1, 0}},
			ExtraOnSelf: []*EffectSpec{{Kind: EffectMark, Duration: Scale{8, 0}, Value: Scale{0.1, 0}}},
			FX:          "b_roar", Description: "Solta tudo, e não se protege de nada.",
			Text: func(k int) string {
				return fmt.Sprintf("Por 8 s, os ataques básicos geram o dobro de Fúria, e você recebe +10%% de dano (recarga de %s s).", num(Scale{40, -5}.At(k)))
			}}),
		node(r, 1, 40, NodeKey, &SkillDef{ID: SkillRagerInsatiate, Name: "Sede Insaciável", MaxRank: 1,
			Passive: PassiveBloodFrenzy, PValue: Scale{0.2, 0}, FX: "b_undying",
			Description: "Quanto mais sangra, mais bate.",
			Text: func(int) string {
				return "Abaixo de 50% de vida, a Sede de Sangue cura o dobro e você ganha +20% de velocidade de ataque."
			}}),
		// ---------- Berserker: Execução ----------
		node(r, 2, 25, NodeActive, &SkillDef{ID: SkillRagerPrey, Name: "Marca da Presa", MaxRank: 3,
			Target: TargetEnemy, Range: 8, MP: Scale{10, 0}, CD: Scale{15, 0}, ResOnMarkKill: 30,
			OnTarget: &EffectSpec{Kind: EffectMark, Duration: Scale{10, 0}, Value: Scale{0.08, 0.04}},
			FX:       "r_mark", Description: "Escolhe a presa.",
			Text: func(k int) string {
				return fmt.Sprintf("Marca o alvo por 10 s: ele recebe +%s de dano; matá-lo devolve 30 de Fúria.", pct(Scale{0.08, 0.04}.At(k)))
			}}),
		evolvedNode(SkillExecute, SkillRagerExecute, r, 2, 30, func(s *SkillDef) {
			m := Scale{2.5, 0.25} // calibrado (doc: 300→350%)
			s.MaxRank, s.Mult, s.MP, s.CD, s.ResCost, s.ResetOnKill = 3, m, Scale{}, Scale{25, 0}, 35, true
			s.Text = func(k int) string {
				return fmt.Sprintf("Causa %s do ataque; o dobro em alvos abaixo de 30%% de vida (em chefes, no máximo +50%%).", pct(m.At(k)))
			}
		}),
		node(r, 2, 40, NodeKey, &SkillDef{ID: SkillRagerFinale, Name: "Carnificina Final", MaxRank: 1,
			Passive: PassiveKillReset, PValue: Scale{1, 0}, FX: "r_fatal",
			Description: "Quem cai abre caminho para o próximo.",
			Text: func(int) string {
				return "Matar com a Execução ou o Golpe Rubro zera a recarga dele (1 vez a cada 10 s)."
			}}),

		// ---------- Senhor da Guerra: Comando ----------
		node(w, 0, 20, NodePassive, &SkillDef{ID: SkillWarlordLead, Name: "Liderança", MaxRank: 5,
			Passive: PassiveAllyDamageAura, PValue: Scale{0.01, 0.01}, FX: "k_aura",
			Description: "A tropa bate mais forte perto do comandante.",
			Text: func(k int) string {
				return fmt.Sprintf("Você e os aliados a até 12 m causam +%s de dano (não soma entre Senhores da Guerra).", pct(Scale{0.01, 0.01}.At(k)))
			}}),
		node(w, 0, 25, NodeModifier, &SkillDef{ID: SkillWarlordAdvance, Name: "Ordem de Avanço", MaxRank: 3,
			Modifies: SkillWarlordCry, FX: "k_cry",
			Mod:         &SkillMod{ExtraOnSelf: []*EffectSpec{{Kind: EffectBuffSpeed, Duration: Scale{20, 0}, Value: Scale{0.1, 0.05}}}, ResOnAlly: Scale{5, 0}},
			Description: "O brado põe a tropa em marcha.",
			Text: func(k int) string {
				return fmt.Sprintf("O Brado de Guerra também dá +%s de velocidade de movimento; +5 de Ímpeto por aliado.", pct(Scale{0.1, 0.05}.At(k)))
			}}),
		node(w, 0, 30, NodeActive, &SkillDef{ID: SkillWarlordRally, Name: "Grito de Reagrupar", MaxRank: 3,
			Target: TargetSelf, CD: Scale{40, 0}, AllyRadius: 12, ResCost: 25,
			OnSelf: &EffectSpec{Kind: EffectUnstoppable, Duration: Scale{3, 1}, Value: Scale{1, 0}},
			FX:     "m_blink", Description: "Ninguém fica para trás.",
			Text: func(k int) string {
				return fmt.Sprintf("Tira lentidão e prender de você e dos aliados a até 12 m e dá imunidade a esses efeitos por %s s.", num(Scale{3, 1}.At(k)))
			}}),
		node(w, 0, 35, NodeModifier, &SkillDef{ID: SkillWarlordFlag, Name: "Estandarte Tremulante", MaxRank: 3,
			Modifies: SkillWarlordBanner, Mod: &SkillMod{BannerRadius: Scale{2, 1}, BannerDuration: Scale{5, 5}}, FX: "k_banner",
			Description: "O estandarte se vê de longe.",
			Text: func(k int) string {
				return fmt.Sprintf("Estandarte de Guerra: +%s m de raio e +%s s de duração.", num(Scale{2, 1}.At(k)), num(Scale{5, 5}.At(k)))
			}}),
		node(w, 0, 40, NodeKey, &SkillDef{ID: SkillWarlordCommand2, Name: "Comando Absoluto", MaxRank: 1,
			Passive: PassiveCommand, PValue: Scale{1, 0}, FX: "m_arcane",
			Description: "Com a tropa por perto, tudo fica mais fácil.",
			Text: func(int) string {
				return "Com um aliado a até 12 m, você gera +50% de Ímpeto; o Brado e o Estandarte custam 50% menos mana."
			}}),
		// ---------- Senhor da Guerra: Investida ----------
		node(w, 1, 20, NodePassive, &SkillDef{ID: SkillWarlordArm, Name: "Braço de Guerra", MaxRank: 5,
			Passive: PassivePAtkPct, PValue: Scale{0.02, 0.02}, FX: "k_fencing",
			Description: "Anos de guerra no braço.",
			Text:        func(k int) string { return fmt.Sprintf("+%s de ataque físico.", pct(Scale{0.02, 0.02}.At(k))) }}),
		node(w, 1, 25, NodeModifier, &SkillDef{ID: SkillWarlordBreak, Name: "Quebra-Linha", MaxRank: 3,
			Modifies: SkillWarlordCharge, Mod: &SkillMod{CD: Scale{-3, -3}, ResOnHit: Scale{2.5, 2.5}}, FX: "k_charge",
			Description: "O avanço rompe a linha inimiga.",
			Text: func(k int) string {
				return fmt.Sprintf("Avanço de Guerra: −%s s de recarga e +%s de Ímpeto por alvo atingido.", num(Scale{3, 3}.At(k)), num(Scale{2.5, 2.5}.At(k)))
			}}),
		node(w, 1, 30, NodeActive, &SkillDef{ID: SkillWarlordImpact, Name: "Impacto", MaxRank: 3,
			Target: TargetAreaSelf, Range: 4, CD: Scale{14, 0}, Mult: Scale{1.6, 0.2}, ResCost: 25,
			OnTarget: &EffectSpec{Kind: EffectStun, Duration: Scale{1, 0}},
			FX:       "b_quake", Description: "Bate o machado no chão e corta o golpe inimigo.",
			Text: func(k int) string {
				return fmt.Sprintf("Causa %s do ataque a até 4 m e atordoa por 1 s; atordoar um monstro durante o aviso dele cancela o golpe e gera Ímpeto.", pct(Scale{1.6, 0.2}.At(k)))
			}}),
		node(w, 1, 35, NodePassive, &SkillDef{ID: SkillWarlordMomentum, Name: "Ímpeto de Carga", MaxRank: 5,
			Passive: PassiveChargeMomentum, PValue: Scale{0.02, 0.02}, FX: "r_shadow",
			Description: "Depois da carga, ninguém o segura.",
			Text: func(k int) string {
				v := Scale{0.02, 0.02}.At(k)
				return fmt.Sprintf("Por 5 s depois de uma investida: +%s de dano e +%s de velocidade de movimento.", pct(v), pct(2*v))
			}}),
		node(w, 1, 40, NodeKey, &SkillDef{ID: SkillWarlordRelent, Name: "Investida Implacável", MaxRank: 1,
			Target: TargetEnemy, Range: ChainChargeRange, CD: Scale{45, 0}, Mult: Scale{1.5, 0}, NoMiss: true, ResCost: 60,
			ChainCharge: 3, FX: "b_seismic",
			Description: "Atravessa a linha inimiga de ponta a ponta.",
			Text: func(int) string {
				return "Salta até 3 vezes entre inimigos (até 10 m), com 150% do ataque em cada salto, sem errar."
			}}),
		// ---------- Senhor da Guerra: Domínio ----------
		evolvedNode(SkillRoar, SkillWarlordRoar, w, 2, 20, nil),
		evolvedNode(SkillThickHide, SkillWarlordHide, w, 2, 20, nil),
		evolvedNode(SkillStomp, SkillWarlordStomp, w, 2, 25, nil),
		node(w, 2, 30, NodeActive, &SkillDef{ID: SkillWarlordDefy, Name: "Desafio", MaxRank: 3,
			Target: TargetAreaSelf, Range: 6, CD: Scale{25, 0}, ResCost: 20, Taunt: true,
			OnTarget: &EffectSpec{Kind: EffectTaunt, Duration: Scale{3, 1}},
			OnSelf:   &EffectSpec{Kind: EffectBuffDef, Duration: Scale{5, 0}, Value: Scale{0.2, 0}},
			FX:       "k_taunt", Description: "Chama todos para a briga.",
			Text: func(k int) string {
				return fmt.Sprintf("Provoca os inimigos a até 6 m por %s s e dá +20%% de defesa a você por 5 s.", num(Scale{3, 1}.At(k)))
			}}),
		node(w, 2, 35, NodeModifier, &SkillDef{ID: SkillWarlordField, Name: "Domínio do Campo", MaxRank: 3,
			Modifies: SkillWarlordStomp, FX: "m_rain",
			Mod:         &SkillMod{Range: Scale{1, 1}, ExtraOnTarget: []*EffectSpec{{Kind: EffectSlow, Duration: Scale{2, 1}, Value: Scale{0.3, 0}}}},
			Description: "O campo é de quem pisa.",
			Text: func(k int) string {
				return fmt.Sprintf("Pisotear: +%s m de raio e, depois do atordoamento, 30%% de lentidão por %s s.", num(Scale{1, 1}.At(k)), num(Scale{2, 1}.At(k)))
			}}),
		node(w, 2, 40, NodeKey, &SkillDef{ID: SkillWarlordPresence, Name: "Presença Dominante", MaxRank: 1,
			Passive: PassiveDominance, PValue: Scale{1, 0}, FX: "b_bear",
			Description: "Perto dele, ninguém se sente forte.",
			Text: func(int) string {
				return "Os inimigos a até 6 m causam −10% de dano e ficam 15% mais lentos; seus atordoamentos duram +0,5 s."
			}}),
	}
}

// barbarianTuning: multiplicadores calibrados no simulador (metas dos docs: Berserker entre
// −20% e −5% do tempo para matar médio; Senhor da Guerra dentro de ±10%).
var barbarianTuning = struct {
	brutal, red, seismic, command, advance, impact Scale
	commandCD                                      float64
}{
	brutal: Scale{1.7, 0.25}, red: Scale{1.9, 0.3}, seismic: Scale{1.9, 0.3},
	command: Scale{3.2, 0.35}, advance: Scale{2.6, 0.3}, impact: Scale{2.0, 0.25}, commandCD: 5,
}

// adjustBarbarianEvolutions: os ajustes dos docs nas skills que já existiam.
func adjustBarbarianEvolutions() {
	set := func(id uint32, f func(*SkillDef)) { f(Skills[id]) }
	tuneMult := func(id uint32, m Scale, text func(Scale) func(int) string) {
		set(id, func(s *SkillDef) { s.Mult, s.Text = m, text(m) })
	}
	tuneMult(SkillRagerBrutal, barbarianTuning.brutal, func(m Scale) func(int) string {
		return func(k int) string { return fmt.Sprintf("Causa %s do ataque.", pct(m.At(k))) }
	})
	tuneMult(SkillWarlordCommand, barbarianTuning.command, func(m Scale) func(int) string {
		return func(k int) string { return fmt.Sprintf("Causa %s do ataque; gera 10 de Ímpeto.", pct(m.At(k))) }
	})
	tuneMult(SkillRagerRed, barbarianTuning.red, func(m Scale) func(int) string {
		return func(k int) string { return fmt.Sprintf("Causa %s do ataque; +30%% em alvos abaixo de 50%% de vida.", pct(m.At(k))) }
	})
	tuneMult(SkillSeismicAxe, barbarianTuning.seismic, func(m Scale) func(int) string {
		return func(k int) string {
			return fmt.Sprintf("Causa %s do ataque no alvo e a até 3 m; +50%% em quem estiver abaixo de 30%% de vida.", pct(m.At(k)))
		}
	})
	set(SkillWarlordCommand, func(s *SkillDef) { s.CD = Scale{barbarianTuning.commandCD, 0} })
	tuneMult(SkillWarlordCharge, barbarianTuning.advance, func(m Scale) func(int) string {
		return func(k int) string {
			return fmt.Sprintf("Salta até o alvo (até 12 m) e causa %s do ataque nele e a até 4 m dele; atordoa por %s s.", pct(m.At(k)), num(Scale{1, 0.25}.At(k)))
		}
	})
	tuneMult(SkillWarlordImpact, barbarianTuning.impact, func(m Scale) func(int) string {
		return func(k int) string {
			return fmt.Sprintf("Causa %s do ataque a até 4 m e atordoa por 1 s; atordoar um monstro durante o aviso dele cancela o golpe e gera Ímpeto.", pct(m.At(k)))
		}
	})
	set(SkillRagerBlind, func(s *SkillDef) {
		s.MaxRank, s.PValue = 5, Scale{0.02, 0.02}
		s.Text = func(k int) string { return fmt.Sprintf("+%s de chance de crítico.", pct(Scale{0.02, 0.02}.At(k))) }
	})
	set(SkillRagerCarnage, func(s *SkillDef) {
		s.MaxRank, s.Mult = 3, Scale{1.1, 0.15}
		s.Text = func(k int) string {
			return fmt.Sprintf("Três golpes de %s do ataque em todos a até 5 m (a Sede de Sangue cura a cada golpe).", pct(Scale{1.1, 0.15}.At(k)))
		}
	})
	set(SkillRagerRed, func(s *SkillDef) { s.ResetOnKill = true })
	set(SkillWarlordCry, func(s *SkillDef) { s.Name, s.CommandSkill = "Brado de Guerra", true })
	set(SkillWarlordCharge, func(s *SkillDef) { s.Name = "Avanço de Guerra" })
	set(SkillWarlordBanner, func(s *SkillDef) {
		s.MaxRank, s.Mult, s.MP, s.CommandSkill = 3, Scale{1.5, 0.2}, Scale{40, 0}, true
		s.Text = func(k int) string {
			return fmt.Sprintf("Causa %s do ataque a até 6 m (os inimigos perdem 20%% de ataque por 8 s) e finca um estandarte por %s s: +20%% de defesa a até %s m.",
				pct(Scale{1.5, 0.2}.At(k)), num(s.Banner.Duration), num(s.Banner.Radius))
		}
	})
}

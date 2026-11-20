package game

import "fmt"

// Conteúdo das evoluções do Ladino (docs/evolucoes/cutthroat.md e hunter.md): as skills
// novas (186–202 e 204–219) e os ajustes nas que vieram do degrau 20 e da especialização.
// O Patrulheiro usa as adagas até as armas de longe chegarem ao cliente (Fase 3).

const (
	SkillCutTwin        uint32 = 186 // Lâminas Gêmeas (evol.)
	SkillCutDeep        uint32 = 188 // Corte Profundo (modificador das Lâminas Gêmeas)
	SkillCutEdge        uint32 = 189 // Fio Perfeito
	SkillCutDance       uint32 = 190 // Dança das Lâminas (chave)
	SkillCutDodge       uint32 = 192 // Esquiva (evol.)
	SkillCutVanish      uint32 = 193 // Sumir (evol.)
	SkillCutVeil        uint32 = 194 // Véu Mortal
	SkillCutLinger      uint32 = 195 // Sombra Persistente (modificador do Sumir)
	SkillCutShadowKill  uint32 = 196 // Assassino das Sombras (chave)
	SkillCutMark        uint32 = 197 // Marca da Morte (evol.)
	SkillCutWeak        uint32 = 198 // Ponto Fraco (evol.)
	SkillCutDeepMark    uint32 = 199 // Marca Profunda (modificador da Marca)
	SkillCutMercy       uint32 = 200 // Golpe de Misericórdia
	SkillCutBleed       uint32 = 201 // Sangrar
	SkillCutPerfect     uint32 = 202 // Execução Perfeita (chave)
	SkillHuntPierce     uint32 = 204 // Disparo Perfurante
	SkillHuntHawk       uint32 = 205 // Olho de Falcão
	SkillHuntCharged    uint32 = 206 // Flecha Carregada (modificador do Tiro Preciso)
	SkillHuntDeadly     uint32 = 207 // Flecha Mortal
	SkillHuntAim        uint32 = 208 // Mira Perfeita (chave)
	SkillHuntTrap       uint32 = 209 // Armadilha de Caça
	SkillHuntSmoke      uint32 = 210 // Bomba de Fumaça (evol.)
	SkillHuntThorns     uint32 = 211 // Campo de Espinhos
	SkillHuntMaster     uint32 = 212 // Mestre Armadilheiro
	SkillHuntAmbush     uint32 = 213 // Emboscada (chave)
	SkillHuntPrey       uint32 = 214 // Marca da Presa
	SkillHuntStride     uint32 = 215 // Passo de Caça
	SkillHuntRecoil     uint32 = 216 // Recuo
	SkillHuntChase      uint32 = 217 // Perseguição
	SkillHuntInstinct   uint32 = 218 // Instinto do Caçador
	SkillHuntRelentless uint32 = 219 // Caçada Implacável (chave)
)

func rogueEvoSkills() []*SkillDef {
	const c, h = "cutthroat", "hunter"
	return []*SkillDef{
		// ---------- Assassino: Lâminas ----------
		evolvedNode(SkillTwinBlades, SkillCutTwin, c, 0, 20, nil),
		node(c, 0, 25, NodeModifier, &SkillDef{ID: SkillCutDeep, Name: "Corte Profundo", MaxRank: 3,
			Modifies: SkillCutTwin, Mod: &SkillMod{MultPct: Scale{0.1, 0.05}, Hits: [4]int{0, 0, 0, 1}}, FX: "b_frenzy",
			Description: "As duas lâminas entram mais fundo.",
			Text: func(k int) string {
				t := fmt.Sprintf("Lâminas Gêmeas: +%s de dano", pct(Scale{0.1, 0.05}.At(k)))
				if k >= 3 {
					t += " e +1 golpe"
				}
				return t + "."
			}}),
		node(c, 0, 35, NodePassive, &SkillDef{ID: SkillCutEdge, Name: "Fio Perfeito", MaxRank: 3,
			Passive: PassiveCritDamage, PValue: Scale{0.05, 0.05}, FX: "b_heavy",
			Description: "A lâmina perfeita não para no osso.",
			Text:        func(k int) string { return fmt.Sprintf("+%s de dano crítico.", pct(Scale{0.05, 0.05}.At(k))) }}),
		node(c, 0, 40, NodeKey, &SkillDef{ID: SkillCutDance, Name: "Dança das Lâminas", MaxRank: 1,
			Passive: PassiveFinisherRefund, PValue: Scale{FinisherRefund, 0}, FX: "k_spin",
			Description: "Cada final abre o próximo.",
			Text:        func(int) string { return "As finalizações (que gastam Combo) devolvem 30% do Combo gasto." }}),
		// ---------- Assassino: Sombras ----------
		evolvedNode(SkillDodge, SkillCutDodge, c, 1, 20, nil),
		evolvedNode(SkillVanish, SkillCutVanish, c, 1, 25, func(s *SkillDef) {
			s.MaxRank, s.CD = 3, Scale{60, -7.5}
			s.OnSelf = &EffectSpec{Kind: EffectInvisible, Duration: Scale{3, 1}}
			s.Text = func(k int) string {
				return fmt.Sprintf("Fica invisível por %s s; atacar revela, e o primeiro golpe é crítico garantido (recarga de %s s).", num(Scale{3, 1}.At(k)), num(Scale{60, -7.5}.At(k)))
			}
		}),
		node(c, 1, 30, NodeActive, &SkillDef{ID: SkillCutVeil, Name: "Véu Mortal", MaxRank: 3,
			Target: TargetSelf, MP: Scale{20, 0}, CD: Scale{30, 0}, ResCost: 30,
			OnSelf: &EffectSpec{Kind: EffectVeil, Duration: Scale{8, 0}, Value: Scale{2, 0.5}},
			FX:     "m_blink", Description: "Quem cai abre a sombra para o próximo passo.",
			Text: func(k int) string {
				return fmt.Sprintf("Por 8 s, matar um inimigo deixa você invisível por %s s.", num(Scale{2, 0.5}.At(k)))
			}}),
		node(c, 1, 35, NodeModifier, &SkillDef{ID: SkillCutLinger, Name: "Sombra Persistente", MaxRank: 3,
			Modifies: SkillCutVanish, Mod: &SkillMod{RevealSpeed: Scale{0.2, 0.1}}, FX: "r_smoke",
			Description: "A sombra continua correndo depois de aparecer.",
			Text: func(k int) string {
				return fmt.Sprintf("Depois de sair do Sumir, +%s de velocidade por 3 s.", pct(Scale{0.2, 0.1}.At(k)))
			}}),
		node(c, 1, 40, NodeKey, &SkillDef{ID: SkillCutShadowKill, Name: "Assassino das Sombras", MaxRank: 1,
			Passive: PassiveShadowAssassin, PValue: Scale{1, 0}, FX: "b_undying",
			Description: "Da sombra direto para o golpe final.",
			Text: func(int) string {
				return "Sair da invisibilidade dá +40 de Combo e +100% de dano crítico no primeiro golpe."
			}}),
		// ---------- Assassino: Execução ----------
		evolvedNode(SkillDeathMark, SkillCutMark, c, 2, 20, func(s *SkillDef) {
			s.MaxRank = 3
			s.OnTarget = &EffectSpec{Kind: EffectMark, Duration: Scale{10, 0}, Value: Scale{0.15, 0.05}}
			s.Text = func(k int) string {
				return fmt.Sprintf("Causa 20%% do ataque; o alvo recebe +%s de dano de todas as fontes por 10 s.", pct(Scale{0.15, 0.05}.At(k)))
			}
		}),
		evolvedNode(SkillWeakSpot, SkillCutWeak, c, 2, 20, nil),
		node(c, 2, 25, NodeModifier, &SkillDef{ID: SkillCutDeepMark, Name: "Marca Profunda", MaxRank: 3,
			Modifies: SkillCutMark, Mod: &SkillMod{EffectDur: Scale{2, 2}}, FX: "r_para",
			Description: "A marca não sai.",
			Text:        func(k int) string { return fmt.Sprintf("A Marca da Morte dura +%s s.", num(Scale{2, 2}.At(k))) }}),
		node(c, 2, 30, NodeActive, &SkillDef{ID: SkillCutMercy, Name: "Golpe de Misericórdia", MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, MP: Scale{25, 0}, CD: Scale{12, 0}, Mult: Scale{2.5, 0.3}, ResCost: 40,
			ExecuteBelow: 0.3, ExecuteMult: 1.5, FX: "b_exec",
			Description: "Termina o que começou.",
			Text: func(k int) string {
				return fmt.Sprintf("Causa %s do ataque; +50%% em alvos abaixo de 30%% de vida (chefe: no máximo +50%%).", pct(Scale{2.5, 0.3}.At(k)))
			}}),
		node(c, 2, 35, NodePassive, &SkillDef{ID: SkillCutBleed, Name: "Sangrar", MaxRank: 3,
			Passive: PassiveCritBleed, PValue: Scale{0.05, 0.05}, FX: "r_poison",
			Description: "Cada crítico deixa uma ferida aberta.",
			Text: func(k int) string {
				return fmt.Sprintf("Críticos causam sangramento: %s do ataque por segundo, por 4 s.", pct(Scale{0.05, 0.05}.At(k)))
			}}),
		node(c, 2, 40, NodeKey, &SkillDef{ID: SkillCutPerfect, Name: "Execução Perfeita", MaxRank: 1,
			Passive: PassivePerfectExecution, PValue: Scale{1, 0}, FX: "kb_last",
			Description: "Um corpo cai, a sombra já está no próximo.",
			Text: func(int) string {
				return "Matar com uma finalização zera a recarga do Passo Sombrio e do Golpe das Sombras."
			}}),

		// ---------- Patrulheiro: Arqueria ----------
		node(h, 0, 20, NodeActive, &SkillDef{ID: SkillHuntPierce, Name: "Disparo Perfurante", MaxRank: 3,
			Target: TargetEnemy, Range: 14, MP: Scale{25, 0}, CD: Scale{10, 0}, Mult: Scale{1.5, 0.2}, Pierce: 3,
			FX: "k_judg", Description: "Um tiro que atravessa a fila.",
			Text: func(k int) string {
				return fmt.Sprintf("Causa %s do ataque em até 3 inimigos em linha.", pct(Scale{1.5, 0.2}.At(k)))
			}}),
		node(h, 0, 20, NodePassive, &SkillDef{ID: SkillHuntHawk, Name: "Olho de Falcão", MaxRank: 5,
			Passive: PassiveHawkEye, PValue: Scale{0.01, 0.01}, FX: "r_weak",
			Description: "Vê longe e acerta onde vê.",
			Text: func(k int) string {
				v := Scale{0.01, 0.01}.At(k)
				return fmt.Sprintf("+%s de chance de crítico e +%s m de alcance.", pct(v), num(HawkEyeRange*v))
			}}),
		node(h, 0, 25, NodeModifier, &SkillDef{ID: SkillHuntCharged, Name: "Flecha Carregada", MaxRank: 3,
			Modifies: SkillHuntShot, Mod: &SkillMod{ResAbove: 50, ResAbovePct: Scale{0.1, 0.1}}, FX: "rc_blades",
			Description: "Com calma, o tiro pesa mais.",
			Text: func(k int) string {
				return fmt.Sprintf("Com 50 ou mais de Precisão, o Tiro Preciso causa +%s.", pct(Scale{0.1, 0.1}.At(k)))
			}}),
		node(h, 0, 35, NodeActive, &SkillDef{ID: SkillHuntDeadly, Name: "Flecha Mortal", MaxRank: 3,
			Target: TargetEnemy, Range: 14, MP: Scale{30, 0}, CD: Scale{25, 0}, Mult: Scale{3.5, 0.35}, NoMiss: true, ResCost: 40,
			FX: "r_fatal", Description: "O tiro que não erra.",
			Text: func(k int) string { return fmt.Sprintf("Causa %s do ataque, sem errar.", pct(Scale{3.5, 0.35}.At(k))) }}),
		node(h, 0, 40, NodeKey, &SkillDef{ID: SkillHuntAim, Name: "Mira Perfeita", MaxRank: 1,
			Passive: PassivePerfectAim, PValue: Scale{1, 0}, FX: "rc_sentence",
			Description: "Respira, mira, solta.",
			Text:        func(int) string { return "Com a Precisão cheia, seus golpes são críticos garantidos." }}),
		// ---------- Patrulheiro: Armadilhas ----------
		node(h, 1, 20, NodeActive, &SkillDef{ID: SkillHuntTrap, Name: "Armadilha de Caça", MaxRank: 3,
			Target: TargetSelf, MP: Scale{20, 0}, CD: Scale{15, 0}, Trap: true,
			Zone: &ZoneDef{Radius: 1.5, Duration: 30, Period: 0.2, Trap: true, TrapMult: Scale{1.0, 0.2}, Effect: EffectRoot, EffectSecs: 2.5},
			FX:   "k_iron", Description: "Uma armadilha escondida no chão.",
			Text: func(k int) string {
				return fmt.Sprintf("Arma uma armadilha no chão por 30 s: o primeiro inimigo que pisar leva %s do ataque e fica preso por 2,5 s.", pct(Scale{1.0, 0.2}.At(k)))
			}}),
		evolvedNode(SkillSmokeBomb, SkillHuntSmoke, h, 1, 25, nil),
		node(h, 1, 30, NodeActive, &SkillDef{ID: SkillHuntThorns, Name: "Campo de Espinhos", MaxRank: 3,
			Target: TargetSelf, MP: Scale{30, 0}, CD: Scale{30, 0}, ResCost: 25, Trap: true,
			Zone: &ZoneDef{Radius: 6, Duration: 8, Period: 1, Damage: Scale{0.2, 0.05}, Effect: EffectSlow, Value: Scale{0.3, 0}, EffectSecs: 1.5},
			FX:   "r_flask", Description: "Espinhos por todo o chão.",
			Text: func(k int) string {
				return fmt.Sprintf("Área de 6 m por 8 s: %s do ataque por segundo e 30%% de lentidão.", pct(Scale{0.2, 0.05}.At(k)))
			}}),
		node(h, 1, 35, NodePassive, &SkillDef{ID: SkillHuntMaster, Name: "Mestre Armadilheiro", MaxRank: 5,
			Passive: PassiveTrapPower, PValue: Scale{0.06, 0.06}, FX: "kh_dawn",
			Description: "Cada armadilha no lugar certo.",
			Text: func(k int) string {
				v := Scale{0.06, 0.06}.At(k)
				return fmt.Sprintf("Armadilhas e redes causam +%s de dano e prendem por +%s s.", pct(v), num(v*TrapRootPerPower))
			}}),
		node(h, 1, 40, NodeKey, &SkillDef{ID: SkillHuntAmbush, Name: "Emboscada", MaxRank: 1,
			Passive: PassiveAmbush, PValue: Scale{AmbushBonus, 0}, FX: "r_shadow",
			Description: "Preso, é só mirar.",
			Text:        func(int) string { return "Inimigos presos por armadilha ou rede recebem +25% de dano dos seus golpes." }}),
		// ---------- Patrulheiro: Caçada ----------
		node(h, 2, 20, NodeActive, &SkillDef{ID: SkillHuntPrey, Name: "Marca da Presa", MaxRank: 3,
			Target: TargetEnemy, Range: 16, MP: Scale{10, 0}, CD: Scale{15, 0},
			OnTarget: &EffectSpec{Kind: EffectMark, Duration: Scale{12, 0}, Value: Scale{0.08, 0.04}},
			FX:       "m_ember", Description: "Escolhe a presa da caçada.",
			Text: func(k int) string {
				return fmt.Sprintf("Marca o alvo por 12 s: ele recebe +%s de dano.", pct(Scale{0.08, 0.04}.At(k)))
			}}),
		node(h, 2, 20, NodePassive, &SkillDef{ID: SkillHuntStride, Name: "Passo de Caça", MaxRank: 5,
			Passive: PassiveMoveSpeedPct, PValue: Scale{0.02, 0.02}, FX: "shadow_step",
			Description: "Sempre um passo à frente da presa.",
			Text: func(k int) string {
				return fmt.Sprintf("+%s de velocidade de movimento.", pct(Scale{0.02, 0.02}.At(k)))
			}}),
		node(h, 2, 25, NodeActive, &SkillDef{ID: SkillHuntRecoil, Name: "Recuo", MaxRank: 3,
			Target: TargetEnemy, Range: 14, MP: Scale{20, 0}, CD: Scale{18, -3}, Mult: Scale{1.2, 0.2}, Blink: Scale{-6, 0},
			FX: "r_dodge", Description: "Salta para trás e atira no caminho.",
			Text: func(k int) string {
				return fmt.Sprintf("Salta 6 m para trás e causa %s do ataque no alvo (recarga de %s s).", pct(Scale{1.2, 0.2}.At(k)), num(Scale{18, -3}.At(k)))
			}}),
		node(h, 2, 30, NodeActive, &SkillDef{ID: SkillHuntChase, Name: "Perseguição", MaxRank: 3,
			Target: TargetSelf, MP: Scale{10, 0}, CD: Scale{25, 0}, ResCost: 20,
			OnSelf:      &EffectSpec{Kind: EffectBuffSpeed, Duration: Scale{6, 0}, Value: Scale{0.3, 0}},
			ExtraOnSelf: []*EffectSpec{{Kind: EffectHunt, Duration: Scale{6, 0}, Value: Scale{0.1, 0.05}}},
			FX:          "k_charge", Description: "Corre atrás da presa marcada.",
			Text: func(k int) string {
				return fmt.Sprintf("Por 6 s, +30%% de velocidade e +%s de dano contra alvos marcados.", pct(Scale{0.1, 0.05}.At(k)))
			}}),
		node(h, 2, 35, NodePassive, &SkillDef{ID: SkillHuntInstinct, Name: "Instinto do Caçador", MaxRank: 3,
			Passive: PassiveHunterInstinct, PValue: Scale{0.05, 0.05}, FX: "b_bear",
			Description: "Presa lenta é presa morta.",
			Text: func(k int) string {
				return fmt.Sprintf("+%s de dano contra alvos lentos ou presos.", pct(Scale{0.05, 0.05}.At(k)))
			}}),
		node(h, 2, 40, NodeKey, &SkillDef{ID: SkillHuntRelentless, Name: "Caçada Implacável", MaxRank: 1,
			Passive: PassiveRelentlessHunt, PValue: Scale{1, 0}, FX: "b_seismic",
			Description: "Uma presa cai, a próxima já está marcada.",
			Text: func(int) string {
				return "Matar o alvo marcado devolve 50 de Precisão e passa a marca para o inimigo mais próximo."
			}}),
	}
}

// rogueTuning: calibragem no simulador (metas dos docs: Assassino entre −20% e −5% do tempo
// para matar médio, Patrulheiro entre −10% e +5%): o dano das skills de dano de cada árvore.
var rogueTuning = struct{ cutDamage, huntDamage float64 }{cutDamage: 0.65, huntDamage: 1.15}

// adjustRogueEvolutions: os ajustes dos docs nas skills que já existiam.
func adjustRogueEvolutions() {
	set := func(id uint32, f func(*SkillDef)) { f(Skills[id]) }
	defer func() { // depois dos ajustes do doc: a calibragem vale para a árvore inteira
		for spec, f := range map[string]float64{"cutthroat": rogueTuning.cutDamage, "hunter": rogueTuning.huntDamage} {
			for _, s := range EvolutionTree(spec) {
				if !s.IsPassive() && s.Mult.At(1) > 0 && f != 1 {
					retune(s, f, 1)
				}
			}
		}
	}()
	set(SkillCutInstinct, func(s *SkillDef) {
		s.MaxRank, s.PValue = 5, Scale{0.06, 0.06}
		s.Text = func(k int) string { return fmt.Sprintf("+%s de dano crítico.", pct(Scale{0.06, 0.06}.At(k))) }
	})
	set(SkillCutSentence, func(s *SkillDef) {
		s.MaxRank, s.Mult, s.MP = 3, Scale{4.5, 0.45}, Scale{40, 0}
		s.Text = func(k int) string {
			return fmt.Sprintf("Salta até o alvo (até 12 m) e causa %s do ataque, sem errar; +50%% abaixo de 30%% de vida (em chefe, no máximo +50%%).", pct(Scale{4.5, 0.45}.At(k)))
		}
	})
	set(SkillHuntRain, func(s *SkillDef) {
		s.Name, s.MaxRank, s.Mult = "Chuva de Flechas", 3, Scale{1.6, 0.2}
		s.Text = func(k int) string {
			return fmt.Sprintf("Dois golpes de %s do ataque no alvo e a até 5 m.", pct(Scale{1.6, 0.2}.At(k)))
		}
	})
	set(SkillHuntNet, func(s *SkillDef) { s.Trap = true })
}

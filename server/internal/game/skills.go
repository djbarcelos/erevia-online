package game

import (
	"fmt"
	"strings"
)

// EffectKind é um efeito temporário (debuff ou buff). Os valores batem com o enum do protocolo.
type EffectKind uint8

const (
	EffectNone         EffectKind = iota
	EffectStun                    // não age nem anda
	EffectSlow                    // Value = fração de redução de velocidade
	EffectBuffDef                 // Value = +fração de defesa
	EffectBuffAtk                 // Value = +fração de ataque
	EffectBuffSpeed               // Value = +fração de velocidade
	EffectRoot                    // não anda (ainda ataca e usa skills)
	EffectPoison                  // Value = dano por segundo
	EffectBurn                    // Value = dano por segundo
	EffectTaunt                   // monstro obrigado a atacar a fonte do efeito
	EffectShield                  // Value = quanto dano ainda absorve
	EffectManaShield              // Value = MP gasto por ponto de dano (o dano sai da mana)
	EffectInvisible               // monstros não veem; atacar revela; o próximo golpe é crítico
	EffectMark                    // Value = +fração de dano recebido
	EffectUndying                 // a vida não cai abaixo de 1
	EffectBlind                   // Value = chance extra de errar (fração)
	EffectBuffAtkSpeed            // Value = +fração de velocidade de ataque
	EffectWeaken                  // Value = -fração de ataque
	EffectFrostArmor              // Value = +fração de defesa; quem bate de perto fica lento
	EffectPlague                  // Value = dano por segundo; pula para os vizinhos se o alvo morre
	EffectWarCry                  // Value = +fração de ataque (soma com outros buffs)
	EffectBanner                  // Value = +fração de defesa (perto do estandarte)
	EffectCourage                 // Value = vida regenerada por segundo (aura)
	EffectDeathMark               // Marca da Morte (raid): explode em volta do marcado no fim
)

// IsDoT: efeitos que causam dano a cada segundo.
func (k EffectKind) IsDoT() bool { return k == EffectPoison || k == EffectBurn || k == EffectPlague }

// EffectDef é um efeito pronto para aplicar (valores de um rank).
type EffectDef struct {
	Kind     EffectKind
	Duration float64 // segundos
	Value    float64
}

// Scale é um valor que cresce com o rank: Base no rank 1, +Per a cada rank acima.
type Scale struct{ Base, Per float64 }

func (s Scale) At(rank int) float64 {
	if rank < 1 {
		rank = 1
	}
	return s.Base + s.Per*float64(rank-1)
}

// EffectSpec é um efeito de skill com duração/valor por rank.
type EffectSpec struct {
	Kind     EffectKind
	Duration Scale
	Value    Scale
	// AtkFrac: Value é uma fração do ataque de quem lança (dano por segundo de venenos).
	AtkFrac bool
}

func (e *EffectSpec) At(rank int) *EffectDef {
	if e == nil {
		return nil
	}
	return &EffectDef{Kind: e.Kind, Duration: e.Duration.At(rank), Value: e.Value.At(rank)}
}

// SkillTarget diz em quem a skill é usada.
type SkillTarget uint8

const (
	TargetEnemy    SkillTarget = iota + 1 // precisa de um alvo inimigo no alcance
	TargetSelf                            // em si mesmo, na hora
	TargetAreaSelf                        // área ao redor de si, na hora
)

// PassiveKind é o que uma skill passiva faz (Value por rank em SkillDef.PValue).
type PassiveKind uint8

const (
	PassiveNone       PassiveKind = iota
	PassivePDefPct                // +Value de defesa física
	PassiveSwordPAtk              // +Value de ataque físico com espada
	PassiveLastStand              // abaixo de 30% de vida: escudo de Value × vida máx. (recarga 60 s)
	PassiveCourage                // aliados a até 10 m regeneram Value de vida por segundo
	PassiveCritDamage             // +Value no multiplicador do crítico
	PassiveLifesteal              // recupera Value do dano causado como vida
	PassiveMaxHPPct               // +Value de vida máxima
	PassiveBearSpirit             // abaixo de 50% de vida: regenera Value × vida máx. por segundo
	PassiveFireBurn               // skills de fogo queimam: Value × ataque mágico por segundo, 3 s
	PassiveMPRegenPct             // +Value de regeneração de mana
	PassiveOverload               // +Value de ataque mágico; skills custam 10% a mais de mana
	PassiveCrit                   // +Value de chance de crítico
	PassiveEvasion                // +Value de esquiva
	PassivePoisonProc             // Value = chance de envenenar ao acertar (4% do ataque/s, 5 s)
)

// Constantes das passivas e mecânicas especiais.
const (
	LastStandBelow   = 0.30 // Muralha Viva
	LastStandCD      = 60.0
	LastStandSeconds = 10.0
	CourageRadius    = 10.0 // Aura de Coragem
	BearBelow        = 0.50 // Espírito do Urso
	FireBurnSeconds  = 3.0  // Brasa Viva
	OverloadMPCost   = 1.10 // Sobrecarga
	PoisonProcFrac   = 0.04 // Lâmina Envenenada: 4% do ataque por segundo…
	PoisonProcSecs   = 5.0  // …por 5 s
	PlagueSpread     = 6.0  // Praga: raio do contágio
	FrostArmorSlow   = 0.3  // Armadura de Gelo: lentidão em quem bate de perto…
	FrostArmorSecs   = 2.0  // …por 2 s
	MeleeRange       = 4.0  // até onde um golpe conta como "de perto"
)

// BannerDef é o estandarte que fica no chão: aliados perto ganham o efeito.
type BannerDef struct {
	Radius   float64
	Duration float64
	Effect   EffectKind
	Value    float64
}

// TierLevels: nível do personagem que libera cada degrau da árvore.
var TierLevels = [5]int{1, 5, 10, 15, 20}

type SkillDef struct {
	ID      uint32
	Class   string
	Name    string
	Branch  int // 0–2 (SpecBranch nas skills de especialização)
	Tier    int // 0–4 (0–2 nas de especialização)
	MaxRank int
	// Spec: id da especialização dona da skill ("" = árvore comum da classe). Só quem
	// escolheu esse caminho aprende e usa.
	Spec string

	Passive PassiveKind // != 0: skill passiva (não se usa)
	PValue  Scale

	Target    SkillTarget
	Range     float64 // alcance (Enemy) ou raio (AreaSelf)
	Splash    float64 // raio em volta do alvo (Enemy): acerta todos ali
	MP, CD    Scale
	Mult      Scale // multiplicador de dano (0 = sem dano)
	Magic     bool  // usa MAtk/MDef
	Fire      bool  // skill de fogo (Brasa Viva)
	ExtraCrit float64
	Hits      int  // golpes seguidos (0 = 1)
	NoMiss    bool // não erra
	// Execução: abaixo desta fração de vida do alvo, o dano é multiplicado.
	ExecuteBelow, ExecuteMult float64
	Blink                     Scale // teleporta para a frente (metros)
	Charge                    bool  // salta até o alvo antes de bater
	Taunt                     bool  // monstros atingidos passam a odiar quem lançou
	AllyRadius                float64
	Banner                    *BannerDef
	OnTarget, OnSelf          *EffectSpec
	// OnTarget só é aplicado se o alvo já tiver este efeito (Paralisia exige veneno).
	OnTargetNeeds EffectKind

	FX          string // id do visual e do ícone
	Description string
	Text        func(rank int) string // o que faz no rank, para a dica
}

func (s *SkillDef) IsPassive() bool { return s.Passive != PassiveNone }

// LevelReq: nível mínimo para aprender (o do degrau).
func (s *SkillDef) LevelReq() int {
	if s.Spec != "" {
		return SpecTierLevels[s.Tier]
	}
	return TierLevels[s.Tier]
}

func (s *SkillDef) MPCost(rank int) float64   { return s.MP.At(rank) }
func (s *SkillDef) Cooldown(rank int) float64 { return s.CD.At(rank) }

// Parent é o nó de cima no mesmo ramo (nil = raiz).
func (s *SkillDef) Parent() *SkillDef {
	if s.Tier == 0 {
		return nil
	}
	for _, o := range Skills {
		if o.Class == s.Class && o.Spec == s.Spec && o.Branch == s.Branch && o.Tier == s.Tier-1 {
			return o
		}
	}
	return nil
}

// Branch é um ramo da árvore (nome e cor para o cliente).
type Branch struct {
	Name, Description, Color string
}

// Ids das skills. As 8 primeiras são as originais (mantêm o id salvo nos clientes).
const (
	SkillShieldBash uint32 = iota + 1
	SkillFortress
	SkillBrutalStrike
	SkillFury
	SkillFireball
	SkillIceNova
	SkillBackstab
	SkillShadowStep
	// Cavaleiro
	SkillIronSkin
	SkillTaunt
	SkillLivingWall
	SkillFencing
	SkillWhirl
	SkillCharge
	SkillJudgment
	SkillWarCry
	SkillCourage
	SkillBanner
	// Bárbaro
	SkillHeavyAxe
	SkillQuake
	SkillExecute
	SkillBloodthirst
	SkillFrenzy
	SkillUndying
	SkillThickHide
	SkillRoar
	SkillStomp
	SkillBearSpirit
	// Mago
	SkillEmber
	SkillEmberRain
	SkillMeteor
	SkillIceLance
	SkillIceArmor
	SkillGlacialPrison
	SkillClearMind
	SkillArcaneShield
	SkillBlink
	SkillOverload
	// Ladino
	SkillWeakSpot
	SkillTwinBlades
	SkillDeathMark
	SkillDodge
	SkillSmokeBomb
	SkillVanish
	SkillPoisonedBlade
	SkillToxicFlask
	SkillParalysis
	SkillPlague
	// 5º degrau (nível 20), um por ramo. No fim para não mudar os ids salvos.
	SkillBulwark      // Cavaleiro: Baluarte
	SkillHorizon      // Cavaleiro: Corte do Horizonte
	SkillAegis        // Cavaleiro: Égide da Ordem
	SkillSeismicAxe   // Bárbaro: Machadada Sísmica
	SkillBloodWhirl   // Bárbaro: Redemoinho Sangrento
	SkillStoneSkin    // Bárbaro: Pele de Pedra
	SkillInferno      // Mago: Inferno
	SkillAbsoluteZero // Mago: Zero Absoluto
	SkillArcaneBlast  // Mago: Rajada Arcana
	SkillFatalBlow    // Ladino: Golpe Fatal
	SkillShadowStrike // Ladino: Golpe das Sombras
	SkillMiasma       // Ladino: Miasma
	// Especializações (nível 20, 24 e 28), 3 por caminho. No fim, pelo mesmo motivo.
	SkillBastionDefy   // Bastião: Desafio do Bastião
	SkillBastionVigor  // Bastião: Vigor do Bastião
	SkillBastionLast   // Bastião: Último Bastião
	SkillHeraldDawn    // Arauto: Martelo da Aurora
	SkillHeraldBless   // Arauto: Bênção do Arauto
	SkillHeraldSun     // Arauto: Sol Nascente
	SkillRagerRed      // Sanguinário: Golpe Rubro
	SkillRagerBlind    // Sanguinário: Fúria Cega
	SkillRagerCarnage  // Sanguinário: Carnificina
	SkillWarlordCry    // Caudilho: Brado do Caudilho
	SkillWarlordCharge // Caudilho: Avanço do Caudilho
	SkillWarlordBanner // Caudilho: Estandarte de Guerra
	SkillElemLance     // Elementalista: Lança Ígnea
	SkillElemFocus     // Elementalista: Foco Elemental
	SkillElemCata      // Elementalista: Cataclismo
	SkillOccRot        // Ocultista: Peste Sombria
	SkillOccDrain      // Ocultista: Drenar Vontade
	SkillOccAbyss      // Ocultista: Marca do Abismo
	SkillCutBlades     // Sicário: Lâmina Oculta
	SkillCutInstinct   // Sicário: Instinto Assassino
	SkillCutSentence   // Sicário: Sentença
	SkillHuntKnife     // Caçador: Faca de Arremesso
	SkillHuntNet       // Caçador: Rede de Caça
	SkillHuntRain      // Caçador: Chuva de Lâminas
)

// ---------------------------------------------------------------------------------------
// Textos
// ---------------------------------------------------------------------------------------

func pct(v float64) string {
	s := fmt.Sprintf("%.1f", v*100)
	s = strings.TrimSuffix(s, ".0")
	return strings.Replace(s, ".", ",", 1) + "%"
}

func num(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	s = strings.TrimSuffix(s, ".0")
	return strings.Replace(s, ".", ",", 1)
}

func atkName(magic bool) string {
	if magic {
		return "ataque mágico"
	}
	return "ataque"
}

// ---------------------------------------------------------------------------------------
// A árvore
// ---------------------------------------------------------------------------------------

func init() {
	for _, list := range [][]*SkillDef{knightTree(), barbarianTree(), mageTree(), rogueTree(), specTrees()} {
		for _, s := range list {
			if s.Hits == 0 {
				s.Hits = 1
			}
			Skills[s.ID] = s
		}
	}
	Classes["knight"].Branches = [3]Branch{
		{"Guardião", "Defesa e controle da linha de frente.", "#6f93d6"},
		{"Lâmina", "Esgrima e dano corpo a corpo.", "#d6b46e"},
		{"Comando", "Gritos e auras que fortalecem os aliados.", "#5fc4a6"},
	}
	Classes["barbarian"].Branches = [3]Branch{
		{"Massacre", "Golpes pesados que encerram lutas.", "#e06a42"},
		{"Sangue", "Fúria, roubo de vida e teimosia.", "#d04050"},
		{"Selvagem", "Resistência e instinto animal.", "#a6b84a"},
	}
	Classes["mage"].Branches = [3]Branch{
		{"Fogo", "Dano alto à distância.", "#ec7a2c"},
		{"Gelo", "Controle e proteção.", "#6cc4f0"},
		{"Arcano", "Mana, mobilidade e poder bruto.", "#a87cf0"},
	}
	Classes["rogue"].Branches = [3]Branch{
		{"Assassinato", "Críticos e golpes letais.", "#d6b46e"},
		{"Sombras", "Mobilidade, esquiva e invisibilidade.", "#8c7ae0"},
		{"Venenos", "Dano contínuo e controle.", "#6ad06a"},
	}
}

// Skills é o catálogo de todas as skills (preenchido em init).
var Skills = map[uint32]*SkillDef{}

func knightTree() []*SkillDef {
	c := "knight"
	return []*SkillDef{
		{ID: SkillShieldBash, Class: c, Name: "Golpe de Escudo", Branch: 0, Tier: 0, MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, MP: Scale{10, 0}, CD: Scale{8, 0}, Mult: Scale{1.5, 0.2},
			OnTarget: &EffectSpec{Kind: EffectStun, Duration: Scale{1.5, 0.25}},
			FX:       "shield_bash", Description: "Uma pancada seca com o escudo que deixa o inimigo atordoado.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque e atordoa por %s s.", pct(Scale{1.5, 0.2}.At(r)), num(Scale{1.5, 0.25}.At(r)))
			}},
		{ID: SkillIronSkin, Class: c, Name: "Pele de Ferro", Branch: 0, Tier: 1, MaxRank: 5,
			Passive: PassivePDefPct, PValue: Scale{0.04, 0.04}, FX: "k_iron",
			Description: "Anos de armadura pesada endureceram o corpo.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de defesa física.", pct(0.04*float64(r))) }},
		{ID: SkillTaunt, Class: c, Name: "Provocar", Branch: 0, Tier: 2, MaxRank: 3,
			Target: TargetAreaSelf, Range: 6, MP: Scale{18, 0}, CD: Scale{15, 0}, Taunt: true,
			OnTarget: &EffectSpec{Kind: EffectTaunt, Duration: Scale{4, 2}},
			FX:       "k_taunt", Description: "Um grito de desafio que atrai a fúria dos inimigos.",
			Text: func(r int) string {
				return fmt.Sprintf("Inimigos num raio de 6 m são obrigados a atacar você por %s s.", num(Scale{4, 2}.At(r)))
			}},
		{ID: SkillLivingWall, Class: c, Name: "Muralha Viva", Branch: 0, Tier: 3, MaxRank: 1,
			Passive: PassiveLastStand, PValue: Scale{0.15, 0}, FX: "k_wall",
			Description: "Quando tudo parece perdido, o cavaleiro se fecha atrás do escudo.",
			Text: func(int) string {
				return "Ao cair abaixo de 30% de vida, ganha um escudo que absorve 15% da vida máxima por 10 s (1 vez a cada 60 s)."
			}},
		{ID: SkillFencing, Class: c, Name: "Esgrima", Branch: 1, Tier: 0, MaxRank: 5,
			Passive: PassiveSwordPAtk, PValue: Scale{0.03, 0.03}, FX: "k_fencing",
			Description: "Técnica refinada com a espada.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de ataque físico com espadas.", pct(0.03*float64(r))) }},
		{ID: SkillWhirl, Class: c, Name: "Corte Giratório", Branch: 1, Tier: 1, MaxRank: 3,
			Target: TargetAreaSelf, Range: 3, MP: Scale{16, 0}, CD: Scale{9, 0}, Mult: Scale{1.05, 0.15},
			FX: "k_spin", Description: "Gira a espada num arco completo.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque em todos os inimigos a até 3 m.", pct(Scale{1.05, 0.15}.At(r)))
			}},
		{ID: SkillCharge, Class: c, Name: "Investida", Branch: 1, Tier: 2, MaxRank: 3,
			Target: TargetEnemy, Range: 12, MP: Scale{14, 0}, CD: Scale{12, 0}, Mult: Scale{1.2, 0.2}, Charge: true,
			OnTarget: &EffectSpec{Kind: EffectStun, Duration: Scale{1, 0}},
			FX:       "k_charge", Description: "Avança como um aríete contra o alvo.",
			Text: func(r int) string {
				return fmt.Sprintf("Salta até o alvo (até 12 m), causa %s do ataque e atordoa por 1 s.", pct(Scale{1.2, 0.2}.At(r)))
			}},
		{ID: SkillJudgment, Class: c, Name: "Julgamento", Branch: 1, Tier: 3, MaxRank: 1,
			Target: TargetEnemy, Range: 2.5, MP: Scale{40, 0}, CD: Scale{45, 0}, Mult: Scale{4, 0}, NoMiss: true,
			FX: "k_judg", Description: "Um golpe de cima para baixo com todo o peso da justiça.",
			Text: func(int) string { return "Causa 400% do ataque. Não erra." }},
		{ID: SkillFortress, Class: c, Name: "Fortaleza", Branch: 2, Tier: 0, MaxRank: 3,
			Target: TargetSelf, MP: Scale{15, 0}, CD: Scale{25, 0},
			OnSelf: &EffectSpec{Kind: EffectBuffDef, Duration: Scale{10, 0}, Value: Scale{0.5, 0.1}},
			FX:     "fortress", Description: "Firma os pés e ergue a guarda, aguentando muito mais pancada.",
			Text: func(r int) string { return fmt.Sprintf("+%s de defesa por 10 s.", pct(Scale{0.5, 0.1}.At(r))) }},
		{ID: SkillWarCry, Class: c, Name: "Grito de Guerra", Branch: 2, Tier: 1, MaxRank: 3,
			Target: TargetSelf, MP: Scale{20, 0}, CD: Scale{30, 0}, AllyRadius: 10,
			OnSelf: &EffectSpec{Kind: EffectWarCry, Duration: Scale{20, 0}, Value: Scale{0.1, 0.05}},
			FX:     "k_cry", Description: "Um grito que enche os aliados de coragem.",
			Text: func(r int) string {
				return fmt.Sprintf("Você e os aliados a até 10 m ganham +%s de ataque por 20 s.", pct(Scale{0.1, 0.05}.At(r)))
			}},
		{ID: SkillCourage, Class: c, Name: "Aura de Coragem", Branch: 2, Tier: 2, MaxRank: 3,
			Passive: PassiveCourage, PValue: Scale{2, 2}, FX: "k_aura",
			Description: "A presença do cavaleiro mantém os aliados de pé.",
			Text: func(r int) string {
				return fmt.Sprintf("Você e os aliados a até 10 m regeneram +%s de vida por segundo.", num(2*float64(r)))
			}},
		{ID: SkillBanner, Class: c, Name: "Estandarte", Branch: 2, Tier: 3, MaxRank: 1,
			Target: TargetSelf, MP: Scale{45, 0}, CD: Scale{90, 0},
			Banner: &BannerDef{Radius: 8, Duration: 20, Effect: EffectBanner, Value: 0.2},
			FX:     "k_banner", Description: "Finca o estandarte da ordem no chão.",
			Text: func(int) string {
				return "Finca um estandarte por 20 s: você e os aliados a até 8 m dele ganham +20% de defesa."
			}},
		// 5º degrau (nível 20)
		{ID: SkillBulwark, Class: c, Name: "Baluarte", Branch: 0, Tier: 4, MaxRank: 3,
			Target: TargetAreaSelf, Range: 5, MP: Scale{35, 0}, CD: Scale{24, 0}, Mult: Scale{1.4, 0.2}, Taunt: true,
			OnTarget: &EffectSpec{Kind: EffectStun, Duration: Scale{1.5, 0.25}},
			FX:       "k_bulwark", Description: "Crava o escudo no chão e vira o centro da batalha.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque a até 5 m, atordoa por %s s e faz os monstros atingidos atacarem você.",
					pct(Scale{1.4, 0.2}.At(r)), num(Scale{1.5, 0.25}.At(r)))
			}},
		{ID: SkillHorizon, Class: c, Name: "Corte do Horizonte", Branch: 1, Tier: 4, MaxRank: 3,
			Target: TargetEnemy, Range: 3, Splash: 3.5, MP: Scale{32, 0}, CD: Scale{15, 0}, Mult: Scale{2, 0.3},
			OnTarget: &EffectSpec{Kind: EffectSlow, Duration: Scale{3, 0}, Value: Scale{0.4, 0}},
			FX:       "k_horizon", Description: "Um corte tão largo que parece partir o horizonte.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque no alvo e em todos a até 3,5 m dele, que ficam 40%% mais lentos por 3 s.", pct(Scale{2, 0.3}.At(r)))
			}},
		{ID: SkillAegis, Class: c, Name: "Égide da Ordem", Branch: 2, Tier: 4, MaxRank: 3,
			Target: TargetSelf, MP: Scale{45, 0}, CD: Scale{45, 0}, AllyRadius: 10,
			OnSelf: &EffectSpec{Kind: EffectShield, Duration: Scale{8, 0}, Value: Scale{250, 125}},
			FX:     "k_aegis", Description: "O juramento da ordem protege quem luta ao seu lado.",
			Text: func(r int) string {
				return fmt.Sprintf("Você e os aliados a até 10 m ganham um escudo que absorve até %s de dano por 8 s.", num(Scale{250, 125}.At(r)))
			}},
	}
}

func barbarianTree() []*SkillDef {
	c := "barbarian"
	return []*SkillDef{
		{ID: SkillBrutalStrike, Class: c, Name: "Golpe Brutal", Branch: 0, Tier: 0, MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, MP: Scale{10, 0}, CD: Scale{6, 0}, Mult: Scale{2.2, 0.3},
			FX: "brutal_strike", Description: "Um golpe com toda a força do corpo.",
			Text: func(r int) string { return fmt.Sprintf("Causa %s do ataque.", pct(Scale{2.2, 0.3}.At(r))) }},
		{ID: SkillHeavyAxe, Class: c, Name: "Machado Pesado", Branch: 0, Tier: 1, MaxRank: 5,
			Passive: PassiveCritDamage, PValue: Scale{0.08, 0.08}, FX: "b_heavy",
			Description: "Cada golpe crítico abre o inimigo ao meio.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de dano crítico.", pct(0.08*float64(r))) }},
		{ID: SkillQuake, Class: c, Name: "Rachar o Chão", Branch: 0, Tier: 2, MaxRank: 3,
			Target: TargetAreaSelf, Range: 5, MP: Scale{22, 0}, CD: Scale{14, 0}, Mult: Scale{1.25, 0.15},
			OnTarget: &EffectSpec{Kind: EffectSlow, Duration: Scale{3, 0}, Value: Scale{0.4, 0}},
			FX:       "b_quake", Description: "Bate no chão com tanta força que ele racha.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque a até 5 m e deixa os inimigos 40%% mais lentos por 3 s.", pct(Scale{1.25, 0.15}.At(r)))
			}},
		{ID: SkillExecute, Class: c, Name: "Execução", Branch: 0, Tier: 3, MaxRank: 1,
			Target: TargetEnemy, Range: 2.5, MP: Scale{30, 0}, CD: Scale{30, 0}, Mult: Scale{3.5, 0},
			ExecuteBelow: 0.3, ExecuteMult: 2,
			FX: "b_exec", Description: "Termina o serviço.",
			Text: func(int) string { return "Causa 350% do ataque; o dobro em alvos abaixo de 30% de vida." }},
		{ID: SkillFury, Class: c, Name: "Fúria", Branch: 1, Tier: 0, MaxRank: 3,
			Target: TargetSelf, MP: Scale{15, 0}, CD: Scale{25, 0},
			OnSelf: &EffectSpec{Kind: EffectBuffAtk, Duration: Scale{10, 0}, Value: Scale{0.3, 0.1}},
			FX:     "fury", Description: "Entra em fúria e passa a bater mais forte.",
			Text: func(r int) string { return fmt.Sprintf("+%s de ataque por 10 s.", pct(Scale{0.3, 0.1}.At(r))) }},
		{ID: SkillBloodthirst, Class: c, Name: "Sede de Sangue", Branch: 1, Tier: 1, MaxRank: 5,
			Passive: PassiveLifesteal, PValue: Scale{0.02, 0.02}, FX: "b_thirst",
			Description: "Cada ferida aberta alimenta o bárbaro.",
			Text:        func(r int) string { return fmt.Sprintf("Recupera %s do dano causado como vida.", pct(0.02*float64(r))) }},
		{ID: SkillFrenzy, Class: c, Name: "Frenesi", Branch: 1, Tier: 2, MaxRank: 3,
			Target: TargetSelf, MP: Scale{20, 0}, CD: Scale{25, 0},
			OnSelf: &EffectSpec{Kind: EffectBuffAtkSpeed, Duration: Scale{12, 0}, Value: Scale{0.25, 0.1}},
			FX:     "b_frenzy", Description: "Os golpes viram um borrão.",
			Text: func(r int) string {
				return fmt.Sprintf("+%s de velocidade de ataque por 12 s.", pct(Scale{0.25, 0.1}.At(r)))
			}},
		{ID: SkillUndying, Class: c, Name: "Imortal", Branch: 1, Tier: 3, MaxRank: 1,
			Target: TargetSelf, MP: Scale{50, 0}, CD: Scale{120, 0},
			OnSelf: &EffectSpec{Kind: EffectUndying, Duration: Scale{5, 0}},
			FX:     "b_undying", Description: "Recusa-se a cair.",
			Text: func(int) string { return "Por 5 s, sua vida não cai abaixo de 1." }},
		{ID: SkillThickHide, Class: c, Name: "Couro Grosso", Branch: 2, Tier: 0, MaxRank: 5,
			Passive: PassiveMaxHPPct, PValue: Scale{0.05, 0.05}, FX: "b_hide",
			Description: "Pele curtida por mil batalhas.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de vida máxima.", pct(0.05*float64(r))) }},
		{ID: SkillRoar, Class: c, Name: "Rugido", Branch: 2, Tier: 1, MaxRank: 3,
			Target: TargetAreaSelf, Range: 6, MP: Scale{18, 0}, CD: Scale{20, 0},
			OnTarget: &EffectSpec{Kind: EffectWeaken, Duration: Scale{8, 0}, Value: Scale{0.15, 0.05}},
			FX:       "b_roar", Description: "Um rugido que faz os inimigos hesitarem.",
			Text: func(r int) string {
				return fmt.Sprintf("Inimigos a até 6 m perdem %s de ataque por 8 s.", pct(Scale{0.15, 0.05}.At(r)))
			}},
		{ID: SkillStomp, Class: c, Name: "Pisotear", Branch: 2, Tier: 2, MaxRank: 3,
			Target: TargetAreaSelf, Range: 4, MP: Scale{24, 0}, CD: Scale{18, 0}, Mult: Scale{1, 0},
			OnTarget: &EffectSpec{Kind: EffectStun, Duration: Scale{1.5, 0.5}},
			FX:       "b_stomp", Description: "Pisa com tanta força que o chão treme.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa 100%% do ataque a até 4 m e atordoa por %s s.", num(Scale{1.5, 0.5}.At(r)))
			}},
		{ID: SkillBearSpirit, Class: c, Name: "Espírito do Urso", Branch: 2, Tier: 3, MaxRank: 1,
			Passive: PassiveBearSpirit, PValue: Scale{0.03, 0}, FX: "b_bear",
			Description: "Ferido, o bárbaro luta como uma fera.",
			Text:        func(int) string { return "Abaixo de 50% de vida, regenera 3% da vida máxima por segundo." }},
		// 5º degrau (nível 20)
		{ID: SkillSeismicAxe, Class: c, Name: "Machadada Sísmica", Branch: 0, Tier: 4, MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, Splash: 3, MP: Scale{35, 0}, CD: Scale{16, 0}, Mult: Scale{2.4, 0.3},
			ExecuteBelow: 0.3, ExecuteMult: 1.5,
			FX: "b_seismic", Description: "O machado desce com tanta força que o chão em volta explode.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque no alvo e em todos a até 3 m dele; 50%% a mais em quem estiver abaixo de 30%% de vida.", pct(Scale{2.4, 0.3}.At(r)))
			}},
		{ID: SkillBloodWhirl, Class: c, Name: "Redemoinho Sangrento", Branch: 1, Tier: 4, MaxRank: 3,
			Target: TargetAreaSelf, Range: 4, MP: Scale{30, 0}, CD: Scale{12, 0}, Mult: Scale{0.9, 0.15}, Hits: 2,
			FX: "b_whirl", Description: "Gira o machado duas vezes, sem escolher em quem acerta.",
			Text: func(r int) string {
				return fmt.Sprintf("Dois golpes de %s do ataque em todos os inimigos a até 4 m (a Sede de Sangue cura a cada golpe).", pct(Scale{0.9, 0.15}.At(r)))
			}},
		{ID: SkillStoneSkin, Class: c, Name: "Pele de Pedra", Branch: 2, Tier: 4, MaxRank: 3,
			Target: TargetAreaSelf, Range: 5, MP: Scale{35, 0}, CD: Scale{30, 0}, Mult: Scale{1, 0.15},
			OnSelf:   &EffectSpec{Kind: EffectBuffDef, Duration: Scale{10, 0}, Value: Scale{0.35, 0.1}},
			OnTarget: &EffectSpec{Kind: EffectSlow, Duration: Scale{3, 0}, Value: Scale{0.3, 0}},
			FX:       "b_stone", Description: "A pele endurece como rocha e o chão treme em volta.",
			Text: func(r int) string {
				return fmt.Sprintf("+%s de defesa por 10 s; os inimigos a até 5 m levam %s do ataque e ficam 30%% mais lentos por 3 s.",
					pct(Scale{0.35, 0.1}.At(r)), pct(Scale{1, 0.15}.At(r)))
			}},
	}
}

func mageTree() []*SkillDef {
	c := "mage"
	return []*SkillDef{
		{ID: SkillFireball, Class: c, Name: "Bola de Fogo", Branch: 0, Tier: 0, MaxRank: 3,
			Target: TargetEnemy, Range: 20, MP: Scale{15, 0}, CD: Scale{3, 0}, Mult: Scale{2.5, 0.3}, Magic: true, Fire: true,
			FX: "fireball", Description: "Lança uma bola de fogo que explode no alvo à distância.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque mágico no alvo.", pct(Scale{2.5, 0.3}.At(r)))
			}},
		{ID: SkillEmber, Class: c, Name: "Brasa Viva", Branch: 0, Tier: 1, MaxRank: 5,
			Passive: PassiveFireBurn, PValue: Scale{0.06, 0.06}, FX: "m_ember",
			Description: "O fogo do mago não se apaga fácil.",
			Text: func(r int) string {
				return fmt.Sprintf("Skills de fogo queimam o alvo: %s do ataque mágico por segundo, por 3 s.", pct(0.06*float64(r)))
			}},
		{ID: SkillEmberRain, Class: c, Name: "Chuva de Brasas", Branch: 0, Tier: 2, MaxRank: 3,
			Target: TargetEnemy, Range: 16, Splash: 5, MP: Scale{35, 0}, CD: Scale{12, 0}, Mult: Scale{1.6, 0.2}, Magic: true, Fire: true,
			FX: "m_rain", Description: "Brasas caem do céu sobre os inimigos.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque mágico no alvo e em todos a até 5 m dele.", pct(Scale{1.6, 0.2}.At(r)))
			}},
		{ID: SkillMeteor, Class: c, Name: "Meteoro", Branch: 0, Tier: 3, MaxRank: 1,
			Target: TargetEnemy, Range: 20, Splash: 3, MP: Scale{80, 0}, CD: Scale{60, 0}, Mult: Scale{5, 0}, Magic: true, Fire: true,
			OnTarget: &EffectSpec{Kind: EffectStun, Duration: Scale{2, 0}},
			FX:       "m_meteor", Description: "Puxa uma pedra em chamas do céu.",
			Text: func(int) string { return "Causa 500% do ataque mágico no alvo e a até 3 m dele, e atordoa por 2 s." }},
		{ID: SkillIceNova, Class: c, Name: "Nova de Gelo", Branch: 1, Tier: 0, MaxRank: 3,
			Target: TargetAreaSelf, Range: 6, MP: Scale{30, 0}, CD: Scale{10, 0}, Mult: Scale{1.2, 0.2}, Magic: true,
			OnTarget: &EffectSpec{Kind: EffectSlow, Duration: Scale{3, 0.5}, Value: Scale{0.5, 0}},
			FX:       "ice_nova", Description: "Uma onda de gelo explode ao seu redor, ferindo e congelando os inimigos próximos.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque mágico a até 6 m e deixa os inimigos 50%% mais lentos por %s s.", pct(Scale{1.2, 0.2}.At(r)), num(Scale{3, 0.5}.At(r)))
			}},
		{ID: SkillIceLance, Class: c, Name: "Lança de Gelo", Branch: 1, Tier: 1, MaxRank: 3,
			Target: TargetEnemy, Range: 18, MP: Scale{18, 0}, CD: Scale{5, 0}, Mult: Scale{1.7, 0.2}, Magic: true,
			OnTarget: &EffectSpec{Kind: EffectSlow, Duration: Scale{2, 0}, Value: Scale{0.3, 0}},
			FX:       "m_lance", Description: "Uma lança de gelo afiado.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque mágico e deixa o alvo 30%% mais lento por 2 s.", pct(Scale{1.7, 0.2}.At(r)))
			}},
		{ID: SkillIceArmor, Class: c, Name: "Armadura de Gelo", Branch: 1, Tier: 2, MaxRank: 3,
			Target: TargetSelf, MP: Scale{30, 0}, CD: Scale{40, 0},
			OnSelf: &EffectSpec{Kind: EffectFrostArmor, Duration: Scale{20, 0}, Value: Scale{0.3, 0.1}},
			FX:     "m_armor", Description: "Uma casca de gelo cobre o mago.",
			Text: func(r int) string {
				return fmt.Sprintf("+%s de defesa por 20 s; quem bater em você de perto fica 30%% mais lento por 2 s.", pct(Scale{0.3, 0.1}.At(r)))
			}},
		{ID: SkillGlacialPrison, Class: c, Name: "Prisão Glacial", Branch: 1, Tier: 3, MaxRank: 1,
			Target: TargetEnemy, Range: 15, MP: Scale{40, 0}, CD: Scale{30, 0}, Mult: Scale{0.5, 0}, Magic: true,
			OnTarget: &EffectSpec{Kind: EffectRoot, Duration: Scale{4, 0}},
			FX:       "m_prison", Description: "Prende o alvo num bloco de gelo.",
			Text: func(int) string { return "Causa 50% do ataque mágico e prende o alvo no lugar por 4 s." }},
		{ID: SkillClearMind, Class: c, Name: "Mente Clara", Branch: 2, Tier: 0, MaxRank: 5,
			Passive: PassiveMPRegenPct, PValue: Scale{0.1, 0.1}, FX: "m_mind",
			Description: "Meditação constante.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de regeneração de mana.", pct(0.1*float64(r))) }},
		{ID: SkillArcaneShield, Class: c, Name: "Escudo Arcano", Branch: 2, Tier: 1, MaxRank: 3,
			Target: TargetSelf, MP: Scale{25, 0}, CD: Scale{30, 0},
			OnSelf: &EffectSpec{Kind: EffectManaShield, Duration: Scale{8, 2}, Value: Scale{1, 0}},
			FX:     "m_shield", Description: "Uma barreira que se alimenta da mana.",
			Text: func(r int) string {
				return fmt.Sprintf("Por %s s, o dano que você recebe sai da mana no lugar da vida.", num(Scale{8, 2}.At(r)))
			}},
		{ID: SkillBlink, Class: c, Name: "Piscar", Branch: 2, Tier: 2, MaxRank: 3,
			Target: TargetSelf, MP: Scale{20, 0}, CD: Scale{18, 0}, Blink: Scale{8, 2},
			FX: "m_blink", Description: "Some e reaparece mais à frente.",
			Text: func(r int) string {
				return fmt.Sprintf("Teleporta até %s m para a frente (na direção em que você anda ou do alvo).", num(Scale{8, 2}.At(r)))
			}},
		{ID: SkillOverload, Class: c, Name: "Sobrecarga", Branch: 2, Tier: 3, MaxRank: 1,
			Passive: PassiveOverload, PValue: Scale{0.15, 0}, FX: "m_over",
			Description: "Mais poder, mais custo.",
			Text:        func(int) string { return "+15% de ataque mágico; as skills custam 10% a mais de mana." }},
		// 5º degrau (nível 20)
		{ID: SkillInferno, Class: c, Name: "Inferno", Branch: 0, Tier: 4, MaxRank: 3,
			Target: TargetEnemy, Range: 18, Splash: 5, MP: Scale{50, 0}, CD: Scale{18, 0}, Mult: Scale{1.8, 0.25}, Magic: true, Fire: true,
			OnTarget: &EffectSpec{Kind: EffectBurn, Duration: Scale{5, 0}, Value: Scale{0.25, 0.05}, AtkFrac: true},
			FX:       "m_inferno", Description: "Faz chover fogo e deixa o chão em chamas.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque mágico no alvo e em todos a até 5 m dele, que queimam por %s do ataque mágico por segundo durante 5 s.",
					pct(Scale{1.8, 0.25}.At(r)), pct(Scale{0.25, 0.05}.At(r)))
			}},
		{ID: SkillAbsoluteZero, Class: c, Name: "Zero Absoluto", Branch: 1, Tier: 4, MaxRank: 3,
			Target: TargetAreaSelf, Range: 6, MP: Scale{55, 0}, CD: Scale{30, 0}, Mult: Scale{1.5, 0.25}, Magic: true,
			OnTarget: &EffectSpec{Kind: EffectStun, Duration: Scale{1.5, 0.25}},
			FX:       "m_zero", Description: "O ar em volta congela de uma vez.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque mágico a até 6 m e congela os inimigos (atordoa) por %s s.",
					pct(Scale{1.5, 0.25}.At(r)), num(Scale{1.5, 0.25}.At(r)))
			}},
		{ID: SkillArcaneBlast, Class: c, Name: "Rajada Arcana", Branch: 2, Tier: 4, MaxRank: 3,
			Target: TargetEnemy, Range: 20, MP: Scale{45, 0}, CD: Scale{10, 0}, Mult: Scale{3, 0.4}, Magic: true, NoMiss: true,
			FX: "m_arcane", Description: "Mana pura, concentrada num único disparo.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque mágico no alvo. Não erra.", pct(Scale{3, 0.4}.At(r)))
			}},
	}
}

func rogueTree() []*SkillDef {
	c := "rogue"
	return []*SkillDef{
		{ID: SkillBackstab, Class: c, Name: "Golpe Furtivo", Branch: 0, Tier: 0, MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, MP: Scale{10, 0}, CD: Scale{6, 0}, Mult: Scale{2, 0.3}, ExtraCrit: 0.5,
			FX: "backstab", Description: "Um golpe traiçoeiro num ponto fraco, com grande chance de crítico.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque com +50%% de chance de crítico.", pct(Scale{2, 0.3}.At(r)))
			}},
		{ID: SkillWeakSpot, Class: c, Name: "Ponto Fraco", Branch: 0, Tier: 1, MaxRank: 5,
			Passive: PassiveCrit, PValue: Scale{0.02, 0.02}, FX: "r_weak",
			Description: "Sabe exatamente onde dói.",
			Text:        func(r int) string { return fmt.Sprintf("+%s de chance de crítico.", pct(0.02*float64(r))) }},
		{ID: SkillTwinBlades, Class: c, Name: "Lâminas Gêmeas", Branch: 0, Tier: 2, MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, MP: Scale{16, 0}, CD: Scale{8, 0}, Mult: Scale{1.15, 0.15}, Hits: 2,
			FX: "r_twin", Description: "Duas lâminas, um só movimento.",
			Text: func(r int) string {
				return fmt.Sprintf("Dois golpes rápidos de %s do ataque cada.", pct(Scale{1.15, 0.15}.At(r)))
			}},
		{ID: SkillDeathMark, Class: c, Name: "Marca da Morte", Branch: 0, Tier: 3, MaxRank: 1,
			Target: TargetEnemy, Range: 12, MP: Scale{30, 0}, CD: Scale{40, 0}, Mult: Scale{0.2, 0},
			OnTarget: &EffectSpec{Kind: EffectMark, Duration: Scale{10, 0}, Value: Scale{0.25, 0}},
			FX:       "r_mark", Description: "Marca a presa.",
			Text: func(int) string { return "Marca o alvo: ele recebe +25% de dano de todas as fontes por 10 s." }},
		{ID: SkillShadowStep, Class: c, Name: "Passo Sombrio", Branch: 1, Tier: 0, MaxRank: 3,
			Target: TargetSelf, MP: Scale{12, 0}, CD: Scale{20, 0},
			OnSelf: &EffectSpec{Kind: EffectBuffSpeed, Duration: Scale{6, 0}, Value: Scale{0.4, 0.1}},
			FX:     "shadow_step", Description: "Funde-se às sombras e se move muito mais rápido.",
			Text: func(r int) string { return fmt.Sprintf("+%s de velocidade por 6 s.", pct(Scale{0.4, 0.1}.At(r))) }},
		{ID: SkillDodge, Class: c, Name: "Esquiva", Branch: 1, Tier: 1, MaxRank: 5,
			Passive: PassiveEvasion, PValue: Scale{6, 6}, FX: "r_dodge",
			Description: "Nunca está onde o golpe cai.",
			Text:        func(r int) string { return fmt.Sprintf("+%d de esquiva.", 6*r) }},
		{ID: SkillSmokeBomb, Class: c, Name: "Bomba de Fumaça", Branch: 1, Tier: 2, MaxRank: 3,
			Target: TargetAreaSelf, Range: 4, MP: Scale{22, 0}, CD: Scale{25, 0},
			OnTarget: &EffectSpec{Kind: EffectBlind, Duration: Scale{6, 0}, Value: Scale{0.2, 0.1}},
			FX:       "r_smoke", Description: "Uma nuvem negra cega os inimigos.",
			Text: func(r int) string {
				return fmt.Sprintf("Inimigos a até 4 m erram %s mais por 6 s.", pct(Scale{0.2, 0.1}.At(r)))
			}},
		{ID: SkillVanish, Class: c, Name: "Sumir", Branch: 1, Tier: 3, MaxRank: 1,
			Target: TargetSelf, MP: Scale{35, 0}, CD: Scale{60, 0},
			OnSelf: &EffectSpec{Kind: EffectInvisible, Duration: Scale{5, 0}},
			FX:     "r_vanish", Description: "Desaparece nas sombras.",
			Text: func(int) string {
				return "Fica invisível por 5 s: os monstros perdem você de vista. Atacar revela, e o primeiro golpe é crítico garantido."
			}},
		{ID: SkillPoisonedBlade, Class: c, Name: "Lâmina Envenenada", Branch: 2, Tier: 0, MaxRank: 5,
			Passive: PassivePoisonProc, PValue: Scale{0.05, 0.05}, FX: "r_poison",
			Description: "A lâmina sempre úmida de veneno.",
			Text: func(r int) string {
				return fmt.Sprintf("%s de chance de envenenar ao acertar: 4%% do ataque por segundo, por 5 s.", pct(0.05*float64(r)))
			}},
		{ID: SkillToxicFlask, Class: c, Name: "Frasco Tóxico", Branch: 2, Tier: 1, MaxRank: 3,
			Target: TargetEnemy, Range: 14, Splash: 4, MP: Scale{18, 0}, CD: Scale{10, 0},
			OnTarget: &EffectSpec{Kind: EffectPoison, Duration: Scale{4, 0}, Value: Scale{0.25, 0.05}, AtkFrac: true},
			FX:       "r_flask", Description: "Arremessa um frasco de veneno.",
			Text: func(r int) string {
				return fmt.Sprintf("Envenena o alvo e todos a até 4 m dele: %s do ataque por segundo, por 4 s.", pct(Scale{0.25, 0.05}.At(r)))
			}},
		{ID: SkillParalysis, Class: c, Name: "Paralisia", Branch: 2, Tier: 2, MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, MP: Scale{20, 0}, CD: Scale{20, 0}, Mult: Scale{0.8, 0},
			OnTarget: &EffectSpec{Kind: EffectStun, Duration: Scale{2, 0.5}}, OnTargetNeeds: EffectPoison,
			FX: "r_para", Description: "O veneno trava os músculos.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa 80%% do ataque; se o alvo estiver envenenado, atordoa por %s s.", num(Scale{2, 0.5}.At(r)))
			}},
		{ID: SkillPlague, Class: c, Name: "Praga", Branch: 2, Tier: 3, MaxRank: 1,
			Target: TargetEnemy, Range: 12, MP: Scale{40, 0}, CD: Scale{45, 0},
			OnTarget: &EffectSpec{Kind: EffectPlague, Duration: Scale{8, 0}, Value: Scale{0.2, 0}, AtkFrac: true},
			FX:       "r_plague", Description: "Uma doença que passa de corpo em corpo.",
			Text: func(int) string {
				return "Infecta o alvo: 20% do ataque por segundo por 8 s. Se ele morrer infectado, a praga pula para os inimigos a até 6 m."
			}},
		// 5º degrau (nível 20)
		{ID: SkillFatalBlow, Class: c, Name: "Golpe Fatal", Branch: 0, Tier: 4, MaxRank: 3,
			Target: TargetEnemy, Range: 2.5, MP: Scale{35, 0}, CD: Scale{18, 0}, Mult: Scale{2.5, 0.4}, ExtraCrit: 0.5,
			ExecuteBelow: 0.35, ExecuteMult: 1.5,
			FX: "r_fatal", Description: "Um só golpe, no lugar exato.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque com +50%% de chance de crítico; 50%% a mais em alvos abaixo de 35%% de vida.", pct(Scale{2.5, 0.4}.At(r)))
			}},
		{ID: SkillShadowStrike, Class: c, Name: "Golpe das Sombras", Branch: 1, Tier: 4, MaxRank: 3,
			Target: TargetEnemy, Range: 12, MP: Scale{30, 0}, CD: Scale{20, 0}, Mult: Scale{1.8, 0.3}, Charge: true,
			OnTarget: &EffectSpec{Kind: EffectBlind, Duration: Scale{4, 0}, Value: Scale{0.3, 0.1}},
			FX:       "r_shadow", Description: "Sai das sombras colado no alvo e ataca os olhos.",
			Text: func(r int) string {
				return fmt.Sprintf("Salta até o alvo (até 12 m), causa %s do ataque e o cega: ele erra %s mais por 4 s.",
					pct(Scale{1.8, 0.3}.At(r)), pct(Scale{0.3, 0.1}.At(r)))
			}},
		{ID: SkillMiasma, Class: c, Name: "Miasma", Branch: 2, Tier: 4, MaxRank: 3,
			Target: TargetAreaSelf, Range: 6, MP: Scale{40, 0}, CD: Scale{20, 0},
			OnTarget: &EffectSpec{Kind: EffectPoison, Duration: Scale{6, 0}, Value: Scale{0.3, 0.05}, AtkFrac: true},
			FX:       "r_miasma", Description: "Quebra frascos em volta de si e o ar fica venenoso.",
			Text: func(r int) string {
				return fmt.Sprintf("Envenena todos os inimigos a até 6 m: %s do ataque por segundo, por 6 s.", pct(Scale{0.3, 0.05}.At(r)))
			}},
	}
}

// ClassTree devolve as skills da árvore comum da classe, por ramo e degrau (sem as das
// especializações: ver SpecTree).
func ClassTree(class string) []*SkillDef {
	var out []*SkillDef
	for _, s := range Skills {
		if s.Class == class && s.Spec == "" {
			out = append(out, s)
		}
	}
	sortSkills(out)
	return out
}

func sortSkills(list []*SkillDef) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && less(list[j], list[j-1]); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

func less(a, b *SkillDef) bool {
	if a.Branch != b.Branch {
		return a.Branch < b.Branch
	}
	return a.Tier < b.Tier
}

// ClassSkills devolve as 2 skills iniciais da classe (rank 1 de graça), na ordem das teclas.
func ClassSkills(class string) []*SkillDef {
	c, ok := Classes[class]
	if !ok {
		return nil
	}
	return []*SkillDef{Skills[c.Skills[0]], Skills[c.Skills[1]]}
}

// SkillPointsTotal: 1 por nível + 1 (as 2 skills iniciais já usam 2 no nível 1).
func SkillPointsTotal(level int) int { return level + 1 }

// StarterRanks: o que um personagem novo (ou antigo, sem árvore salva) já sabe.
func StarterRanks(class string) map[uint32]int {
	out := map[uint32]int{}
	for _, s := range ClassSkills(class) {
		out[s.ID] = 1
	}
	return out
}

// RanksUsed soma os pontos gastos.
func RanksUsed(ranks map[uint32]int) int {
	n := 0
	for _, r := range ranks {
		n += r
	}
	return n
}

// ValidateRanks confere uma árvore inteira (a atual mais o que se quer aprender):
// skills da classe, ranks dentro do máximo, nível do degrau, nó de cima aprendido, a
// especialização (spec: a do personagem, "" = nenhuma) e pontos.
func ValidateRanks(class, spec string, level int, ranks map[uint32]int) error {
	for id, r := range ranks {
		s := Skills[id]
		if s == nil || s.Class != class {
			return fmt.Errorf("skill %d não é da classe", id)
		}
		if r < 0 || r > s.MaxRank {
			return fmt.Errorf("%s: rank %d inválido", s.Name, r)
		}
		if r == 0 {
			continue
		}
		if s.Spec != "" && s.Spec != spec {
			return fmt.Errorf("%s é do caminho %s", s.Name, SpecName(s.Spec))
		}
		if level < s.LevelReq() {
			return fmt.Errorf("%s exige nível %d", s.Name, s.LevelReq())
		}
		if p := s.Parent(); p != nil && ranks[p.ID] < 1 {
			return fmt.Errorf("%s exige %s", s.Name, p.Name)
		}
	}
	if RanksUsed(ranks) > SkillPointsTotal(level) {
		return fmt.Errorf("pontos de skill insuficientes")
	}
	return nil
}

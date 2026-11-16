package game

import (
	"fmt"
	"math"
	"sort"
)

// Graus de equipamento (tunnel#153), como no Lineage 2: a progressão do equipamento anda
// em graus, cada um com um nível mínimo. A raridade (Grade: comum → lendária) continua
// sendo a cor e a qualidade dentro do grau.
//
//	sem grau  1–9    o que a loja vende e o kit inicial
//	D        10–19   armas, conjuntos e joias dos esqueletos de 10–19 e do Ferreiro
//	C        20–29   golens e o Vale; épicos do Senhor dos Ossos, de Vossk e do Ozrhal
//	B        30–40   elites do 30 ao 40 (com o equipamento ponte); épicos de Ômrun e do
//	                 Ozrhal, e as joias lendárias do Ozrhal
//
// Todo equipamento tem o grau do nível exigido (o que já existia também: a rara do 13 é D,
// a lendária do 20 é C, a ponte é B). Abaixo do nível mínimo do grau, a peça não se equipa;
// do mínimo do grau até o nível exigido da peça, ela se equipa com a penalidade de nível
// (UnderLevelPenalty). Cada grau tem armas para as 4 classes, um conjunto de armadura em
// cada peso (pesada, leve e manto: cinco peças, bônus de 2 e de 4 peças) e escudo, capa e
// joias. As peças comuns caem dos monstros do grau (RollLoot, TierDropChance) e, no D,
// também saem do Ferreiro (recipes.go); as épicas e lendárias, só de chefe e de raid.
//
// Os números saem da reta do orçamento de poder (tierCurve, a mesma do balance): cada peça
// é um perfil fixo de atributos escalado pelo nível e pela raridade, e o teste de orçamento
// (internal/balance) confere peça por peça e o conjunto inteiro com o bônus.

// Tier é o grau de um equipamento.
type Tier uint8

const (
	TierNone Tier = iota // sem grau: 1–9
	TierD                // 10–19
	TierC                // 20–29
	TierB                // 30–40
)

// Tiers: os graus, do menor para o maior.
var Tiers = []Tier{TierNone, TierD, TierC, TierB}

var (
	tierMinLevels = [...]int{TierNone: 1, TierD: 10, TierC: 20, TierB: 30}
	tierLetters   = [...]string{TierNone: "", TierD: "D", TierC: "C", TierB: "B"}
)

// MinLevel: o nível mínimo do grau (abaixo dele a peça não se equipa).
func (t Tier) MinLevel() int {
	if int(t) >= len(tierMinLevels) {
		return 1
	}
	return tierMinLevels[t]
}

// Letter: "D", "C", "B" ("" = sem grau).
func (t Tier) Letter() string {
	if int(t) >= len(tierLetters) {
		return ""
	}
	return tierLetters[t]
}

// Label: "Grau D" ou "Sem grau".
func (t Tier) Label() string {
	if t == TierNone || t.Letter() == "" {
		return "Sem grau"
	}
	return "Grau " + t.Letter()
}

// TierFor: o grau de uma peça de nível exigido level.
func TierFor(level int) Tier {
	t := TierNone
	for _, c := range Tiers {
		if level >= c.MinLevel() {
			t = c
		}
	}
	return t
}

// ArmorWeight é o peso de uma peça de conjunto. Qualquer classe veste qualquer peso: o
// perfil de atributos e o bônus do conjunto é que combinam com um jeito de jogar.
type ArmorWeight uint8

const (
	WeightNone  ArmorWeight = iota
	WeightHeavy             // pesada: defesa física e vida
	WeightLight             // leve: esquiva, precisão e crítico
	WeightRobe              // manto: defesa mágica, mana e conjuração
)

// Weights: os três pesos, na ordem em que aparecem.
var Weights = []ArmorWeight{WeightHeavy, WeightLight, WeightRobe}

// Name: "Pesada", "Leve", "Manto" ("" = sem peso).
func (w ArmorWeight) Name() string {
	switch w {
	case WeightHeavy:
		return "Pesada"
	case WeightLight:
		return "Leve"
	case WeightRobe:
		return "Manto"
	}
	return ""
}

func (w ArmorWeight) id() string {
	switch w {
	case WeightHeavy:
		return "heavy"
	case WeightLight:
		return "light"
	case WeightRobe:
		return "robe"
	}
	return ""
}

// ---------------------------------------------------------------------------------------
// Penalidade de nível e bônus de conjunto: a conta do equipamento vestido
// ---------------------------------------------------------------------------------------

// UnderLevelPenalty: o que vestir uma peça acima do próprio nível tira (vale uma vez, com
// uma peça ou com várias): a chance de acerto cai ~20 pontos percentuais e os golpes e as
// recargas ficam ~1,7× mais lentos. Mais que o ganho de qualquer peça do mesmo grau, então ninguém
// caça melhor assim, mas dá para vestir a peça que caiu e ver o que ela faz.
var UnderLevelPenalty = Stats{Accuracy: -100, AtkSpeed: -0.4, CastSpeed: -0.4}

// AboveLevel: a peça pede mais nível que o personagem tem (a penalidade vale).
func (d *ItemDef) AboveLevel(level int) bool { return d.Equippable() && level < d.ReqLevel }

// Worn é uma peça vestida.
type Worn struct {
	Def     *ItemDef
	Enchant int
}

// SetBonus é o bônus de um conjunto a partir de Pieces peças vestidas.
type SetBonus struct {
	Pieces int
	Bonus  Stats
}

// ItemSet é um conjunto de armadura: as cinco peças de um peso num grau. A versão épica de
// cada peça conta como a comum do mesmo encaixe.
type ItemSet struct {
	ID, Name string
	Tier     Tier
	Weight   ArmorWeight
	Theme    string     // a escolha de jogo, em uma frase (dica do item)
	Pieces   []string   // as peças comuns, na ordem do corpo
	Bonuses  []SetBonus // 2 e 4 peças, em ordem
}

// Bonus: o que o conjunto soma com n peças vestidas (os degraus se somam).
func (s *ItemSet) Bonus(n int) Stats {
	var b Stats
	for _, sb := range s.Bonuses {
		if n >= sb.Pieces {
			b = b.AddBonus(sb.Bonus)
		}
	}
	return b
}

// SetPieces: quantas peças de cada conjunto estão vestidas.
func SetPieces(worn []Worn) map[string]int {
	n := map[string]int{}
	for _, w := range worn {
		if w.Def != nil && w.Def.Set != "" {
			n[w.Def.Set]++
		}
	}
	return n
}

// EquipBonus é tudo o que o equipamento vestido soma aos atributos: o bônus de cada peça e
// do encantamento (AddItemBonus), o dos conjuntos pelas peças vestidas e a penalidade de
// nível, se alguma peça pede mais nível que level. Pura e determinística (os conjuntos
// somam em ordem de id): a zona (computeStats) e o simulador fazem a mesma conta.
func EquipBonus(level int, worn []Worn) Stats {
	var b Stats
	under := false
	for _, w := range worn {
		if w.Def == nil {
			continue
		}
		b = AddItemBonus(b, w.Def, w.Enchant)
		under = under || w.Def.AboveLevel(level)
	}
	b = b.AddBonus(SetsBonus(SetPieces(worn)))
	if under {
		b = b.AddBonus(UnderLevelPenalty)
	}
	return b
}

// SetsBonus: o bônus de todos os conjuntos (peças vestidas por id de conjunto).
func SetsBonus(pieces map[string]int) Stats {
	ids := make([]string, 0, len(pieces))
	for id := range pieces {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var b Stats
	for _, id := range ids {
		if s := ItemSets[id]; s != nil {
			b = b.AddBonus(s.Bonus(pieces[id]))
		}
	}
	return b
}

// ---------------------------------------------------------------------------------------
// As peças dos graus
// ---------------------------------------------------------------------------------------

// TierDropChance: chance, por abate de um monstro comum do 10 em diante, de cair uma peça
// comum do grau dele (uma das que pedem até o nível do monstro + tierDropAhead).
const (
	TierDropChance = 0.02
	tierDropAhead  = 2
)

// tierCurve: a reta do orçamento de poder (balance.ExpectedPower: 6 + 1,4 × (nível − 1)).
// As peças escalam por ela de um nível para o outro.
func tierCurve(level int) float64 { return 6 + 1.4*float64(level-1) }

// Quanto a raridade multiplica o perfil (como o orçamento) e o quanto as épicas e lendárias
// encostam no orçamento delas (as comuns ficam a ~90%: o conjunto completa com o bônus).
const (
	tierEpicMult   = 1.5
	tierLegendMult = 1.7
	tierTopBoost   = 1.08
)

// scaleStats escala um perfil: ataque, defesa, vida, mana, precisão e esquiva em inteiros;
// crítico e velocidades de 1 em 1%, dano crítico de 0,01 e regeneração de 0,1. A velocidade
// de movimento não cresce com o grau.
func scaleStats(s Stats, f float64) Stats {
	i := func(v float64) float64 { return math.Round(v * f) }
	q := func(v, step float64) float64 { return math.Round(v*f/step) * step }
	s.MaxHP, s.MaxMP, s.PAtk, s.MAtk, s.PDef, s.MDef = i(s.MaxHP), i(s.MaxMP), i(s.PAtk), i(s.MAtk), i(s.PDef), i(s.MDef)
	s.Accuracy, s.Evasion = i(s.Accuracy), i(s.Evasion)
	s.Crit, s.AtkSpeed, s.CastSpeed = q(s.Crit, 0.01), q(s.AtkSpeed, 0.01), q(s.CastSpeed, 0.01)
	s.CritDamage = q(s.CritDamage, 0.01)
	s.HPRegen, s.MPRegen = q(s.HPRegen, 0.1), q(s.MPRegen, 0.1)
	return s
}

// gradeFactor: o multiplicador da raridade sobre o perfil comum.
func gradeFactor(g Grade) float64 {
	switch g {
	case GradeEpic:
		return tierEpicMult * tierTopBoost
	case GradeLegendary:
		return tierLegendMult * tierTopBoost
	}
	return 1
}

// Perfis de referência (peça comum). As armas no nível 10, as peças de conjunto no 13 e o
// escudo, a capa e as joias no 16: o poder fica perto do orçamento do nível (armas) ou a
// ~90% dele (o resto).
const (
	refWeaponLevel = 10
	refSetLevel    = 13
	refAccLevel    = 16
)

var weaponProfile = map[string]Stats{
	"knight":    {PAtk: 18, MaxHP: 5},
	"barbarian": {PAtk: 18, MaxHP: 25},
	"mage":      {MAtk: 16, MaxMP: 80},
	"rogue":     {PAtk: 14, Crit: 0.02},
}

// setKinds: as cinco peças de um conjunto, na ordem do corpo.
var setKinds = []ItemKind{KindArmor, KindHelmet, KindLegs, KindGloves, KindBoots}

var setProfile = map[ArmorWeight]map[ItemKind]Stats{
	WeightHeavy: {
		KindArmor:  {PDef: 18, MDef: 7, MaxHP: 80},
		KindHelmet: {PDef: 9, MDef: 5, MaxHP: 50},
		KindLegs:   {PDef: 12, MDef: 5, MaxHP: 45},
		KindGloves: {PDef: 6, MDef: 2, PAtk: 3, MAtk: 3, MaxHP: 30},
		KindBoots:  {PDef: 7, MDef: 3, MaxHP: 20, Speed: 0.3},
	},
	WeightLight: {
		KindArmor:  {PDef: 12, MDef: 9, MaxHP: 70, Evasion: 8},
		KindHelmet: {PDef: 6, MDef: 6, MaxHP: 30, Accuracy: 16},
		KindLegs:   {PDef: 8, MDef: 6, MaxHP: 35, Evasion: 8},
		KindGloves: {PDef: 4, MDef: 3, PAtk: 3, MAtk: 3, MaxHP: 10, Accuracy: 12},
		KindBoots:  {PDef: 5, MDef: 3, Evasion: 8, Speed: 0.3},
	},
	WeightRobe: {
		KindArmor:  {PDef: 8, MDef: 14, MaxMP: 80, MaxHP: 30},
		KindHelmet: {PDef: 4, MDef: 8, MaxMP: 70},
		KindLegs:   {PDef: 6, MDef: 9, MaxMP: 40, MaxHP: 25},
		KindGloves: {PDef: 3, MDef: 4, PAtk: 3, MAtk: 3, MaxMP: 40},
		KindBoots:  {PDef: 4, MDef: 4, MaxMP: 20, MPRegen: 1, Speed: 0.3},
	},
}

// setBonusProfile: os bônus de 2 e de 4 peças no grau D (os outros graus escalam pela reta).
// Cada peso é uma escolha: a pesada aguenta (defesa, vida e regeneração), a leve acerta,
// escapa e crita, o manto vive de mana e conjura mais vezes. O ataque fica quase todo na
// arma: o conjunto muda o jeito de caçar sem mexer no equilíbrio entre as classes (o tempo
// para matar do simulador, internal/balance).
var setBonusProfile = map[ArmorWeight][2]Stats{
	WeightHeavy: {{PDef: 4, MaxHP: 30}, {HPRegen: 2, MDef: 2, PAtk: 2}},
	WeightLight: {{Accuracy: 10, Evasion: 6}, {Crit: 0.02, Evasion: 6, MaxHP: 30}},
	WeightRobe:  {{MaxMP: 40, MPRegen: 1}, {CastSpeed: 0.02, MDef: 4, MPRegen: 1}},
}

var setThemes = map[ArmorWeight]string{
	WeightHeavy: "Muralha: aguentar mais (defesa, vida e regeneração de vida), com um pouco de ataque.",
	WeightLight: "Caçada: acertar e escapar (precisão e esquiva) e mais golpes críticos.",
	WeightRobe:  "Fluxo arcano: mais mana, mana que volta mais rápido e feitiços que recarregam antes.",
}

// accKinds: escudo, capa e joias de cada grau.
var accKinds = []ItemKind{KindShield, KindCape, KindNecklace, KindEarring, KindRing}

var accProfile = map[ItemKind]Stats{
	KindShield:   {PDef: 20, MDef: 8, MaxHP: 40},
	KindCape:     {MDef: 9, Evasion: 6, HPRegen: 1, MaxHP: 20},
	KindNecklace: {MDef: 11, MaxHP: 40, MaxMP: 30, PAtk: 2, MAtk: 2},
	KindEarring:  {MDef: 7, MaxMP: 30, Accuracy: 8, MPRegen: 0.5},
	KindRing:     {MDef: 6, PAtk: 2, MAtk: 2, Crit: 0.01, Accuracy: 6, MaxHP: 20},
}

var accIDs = map[ItemKind]string{KindShield: "shield", KindCape: "cape", KindNecklace: "neck", KindEarring: "ear", KindRing: "ring"}
var accIcons = map[ItemKind]string{KindShield: "shield", KindCape: "cape", KindNecklace: "necklace", KindEarring: "earring", KindRing: "ring"}
var accNouns = map[ItemKind]string{KindShield: "Escudo", KindCape: "Capa", KindNecklace: "Colar", KindEarring: "Brinco", KindRing: "Anel"}

var setIDs = map[ItemKind]string{KindArmor: "armor", KindHelmet: "helm", KindLegs: "legs", KindGloves: "gloves", KindBoots: "boots"}
var setIcons = map[ItemKind]string{KindArmor: "armor", KindHelmet: "helmet", KindLegs: "legs", KindGloves: "gloves", KindBoots: "boots"}

// noun é o nome de uma peça com o gênero e o número (para concordar "Exaltado").
type noun struct {
	name   string
	fem    bool
	plural bool
}

func (n noun) exalted() string {
	s := "Exaltad"
	if n.fem {
		s += "a"
	} else {
		s += "o"
	}
	if n.plural {
		s += "s"
	}
	return n.name + " " + s
}

var setNouns = map[ArmorWeight]map[ItemKind]noun{
	WeightHeavy: {KindArmor: {"Couraça", true, false}, KindHelmet: {"Elmo", false, false}, KindLegs: {"Grevas", true, true},
		KindGloves: {"Manoplas", true, true}, KindBoots: {"Botas", true, true}},
	WeightLight: {KindArmor: {"Gibão", false, false}, KindHelmet: {"Capuz", false, false}, KindLegs: {"Calças", true, true},
		KindGloves: {"Luvas", true, true}, KindBoots: {"Botas", true, true}},
	WeightRobe: {KindArmor: {"Túnica", true, false}, KindHelmet: {"Tiara", true, false}, KindLegs: {"Perneiras", true, true},
		KindGloves: {"Luvas", true, true}, KindBoots: {"Sandálias", true, true}},
}

// tierWeapon: uma arma comum de um grau, por classe.
type tierWeapon struct {
	level            int
	names            map[string]string // classe → nome
	models, offModel map[string]string
	desc             string
}

// setSpec: um conjunto (o sufixo dá nome às peças: "Couraça de Ferro Negro").
type setSpec struct {
	suffix, desc string
}

// tierSpec: o que um grau tem.
type tierSpec struct {
	tier       Tier
	weapons    []tierWeapon
	weaponMult float64 // as armas comuns do grau sobre a reta (D e C: 0,95; B: 1,12: entre as duas pontes no ataque, com mais vida ou mana)
	setLevel   int
	sets       map[ArmorWeight]setSpec
	accLevel   int
	accNames   map[ItemKind]string
	accDesc    string
	accShield  string // modelo do escudo comum
	// Épicas (chefes): nível, o "de quem" dos nomes e a descrição.
	epicLevel               int
	epicTheme               string
	epicDesc                string
	epicKinds               []ItemKind // escudo, capa e joias com versão épica (as armas e o conjunto sempre têm)
	legendKind              []ItemKind // joias lendárias (raid)
	legendTheme, legendDesc string
	legendLevel             int
}

var epicWeaponNouns = map[string]string{"knight": "Lâmina", "barbarian": "Machado", "mage": "Cetro", "rogue": "Presas"}
var epicWeaponModels = map[string][2]string{
	"knight": {"sword_2handed_color", ""}, "barbarian": {"axe_2handed", ""}, "mage": {"staff", ""}, "rogue": {"dagger", "skeleton_blade"},
}

var weaponPrefix = map[string]string{"knight": "sword", "barbarian": "axe", "mage": "staff", "rogue": "daggers"}

// tierClasses: as classes, em ordem fixa (os ids e o sorteio ficam estáveis).
var tierClasses = []string{"knight", "barbarian", "mage", "rogue"}

var tierSpecs = []tierSpec{
	{tier: TierD, weaponMult: 0.95,
		weapons: []tierWeapon{
			{level: 10, desc: "Ferro escurecido na fumaça dos Campos dos Ossos.",
				names:  map[string]string{"knight": "Espada de Ferro Negro", "barbarian": "Machado de Ferro Negro", "mage": "Cajado de Ferro Negro", "rogue": "Adagas de Ferro Negro"},
				models: map[string]string{"knight": "sword_1handed", "barbarian": "axe_1handed", "mage": "staff", "rogue": "dagger"}, offModel: map[string]string{"rogue": "dagger"}},
			{level: 13, desc: "Osso de esqueleto antigo, polido até ficar duro como aço.",
				names:  map[string]string{"knight": "Espada de Osso Polido", "barbarian": "Machado de Osso Polido", "mage": "Cajado de Osso Polido", "rogue": "Lâminas de Osso Polido"},
				models: map[string]string{"knight": "skeleton_blade", "barbarian": "skeleton_axe", "mage": "skeleton_staff", "rogue": "skeleton_blade"}, offModel: map[string]string{"rogue": "skeleton_blade"}},
			{level: 16, desc: "Arma de quem caça nos Ermos, longe da muralha.",
				names:  map[string]string{"knight": "Espada dos Ermos", "barbarian": "Machado dos Ermos", "mage": "Varinha dos Ermos", "rogue": "Adagas dos Ermos"},
				models: map[string]string{"knight": "sword_2handed", "barbarian": "axe_2handed", "mage": "wand", "rogue": "dagger"}, offModel: map[string]string{"rogue": "skeleton_blade"}},
		},
		setLevel: 13,
		sets: map[ArmorWeight]setSpec{
			WeightHeavy: {"de Ferro Negro", "Placas de ferro escurecido, rebitadas pelos ferreiros da muralha."},
			WeightLight: {"do Batedor", "Couro curtido e leve, de quem anda à frente da patrulha."},
			WeightRobe:  {"do Acólito", "Linho tingido de azul, bordado com runas de aprendiz."},
		},
		accLevel: 16, accShield: "shield_square", accDesc: "Achado nos Ermos, entre o que os esqueletos guardam.",
		accNames: map[ItemKind]string{KindShield: "Escudo de Ferro Negro", KindCape: "Capa dos Ermos", KindNecklace: "Colar de Vértebras",
			KindEarring: "Brinco de Ônix Negro", KindRing: "Anel de Ferro Negro"},
		epicLevel: 18, epicTheme: "da Meia-Noite", epicDesc: "Só aparece quando o Capitão Ossudo marcha à noite.",
		epicKinds: accKinds,
	},
	{tier: TierC, weaponMult: 0.95,
		weapons: []tierWeapon{
			{level: 20, desc: "Pedra do Vale talhada como metal: pesa, mas não quebra.",
				names:  map[string]string{"knight": "Espada de Granito", "barbarian": "Machado de Granito", "mage": "Cajado do Geomante", "rogue": "Adagas de Sílex"},
				models: map[string]string{"knight": "sword_2handed", "barbarian": "axe_1handed", "mage": "staff", "rogue": "dagger"}, offModel: map[string]string{"rogue": "dagger"}},
			{level: 23, desc: "Pedra escura do fundo do Vale, mais leve do que parece.",
				names:  map[string]string{"knight": "Espada de Basalto", "barbarian": "Machado de Basalto", "mage": "Cajado de Basalto", "rogue": "Adagas de Basalto"},
				models: map[string]string{"knight": "sword_1handed", "barbarian": "axe_2handed", "mage": "staff", "rogue": "dagger"}, offModel: map[string]string{"rogue": "dagger"}},
			{level: 26, desc: "Gasta pelos golens e afiada pelo vento do Vale.",
				names:  map[string]string{"knight": "Espada do Vale", "barbarian": "Machado Quebra-Pedra", "mage": "Varinha do Vale", "rogue": "Lâminas do Vale"},
				models: map[string]string{"knight": "sword_2handed", "barbarian": "axe_2handed", "mage": "wand", "rogue": "skeleton_blade"}, offModel: map[string]string{"rogue": "skeleton_blade"}},
		},
		setLevel: 23,
		sets: map[ArmorWeight]setSpec{
			WeightHeavy: {"de Granito", "Lascas de golem encaixadas em couro grosso: duras como o Vale."},
			WeightLight: {"do Andarilho", "Couro de quem atravessa o Vale sem acordar as pedras."},
			WeightRobe:  {"do Geomante", "Tecido com pó de runa, que vibra perto da pedra viva."},
		},
		accLevel: 26, accShield: "shield_spikes", accDesc: "Talhado em pedra do Vale das Pedras Vivas.",
		accNames: map[ItemKind]string{KindShield: "Escudo de Granito", KindCape: "Capa do Vale", KindNecklace: "Colar de Pedra Viva",
			KindEarring: "Brinco de Quartzo", KindRing: "Anel de Granito"},
		epicLevel: 28, epicTheme: "do Trono Partido", epicDesc: "Saiu do salão do Senhor dos Ossos, onde o trono rachou.",
		epicKinds: accKinds,
	},
	{tier: TierB, weaponMult: 1.12,
		weapons: []tierWeapon{
			{level: 33, desc: "Feita nas oficinas dos Ilvarin, antes da Partição.",
				names:  map[string]string{"knight": "Espada Ilvarin", "barbarian": "Machado Ilvarin", "mage": "Cajado Ilvarin", "rogue": "Adagas Ilvarin"},
				models: map[string]string{"knight": "sword_2handed", "barbarian": "axe_2handed", "mage": "staff", "rogue": "dagger"}, offModel: map[string]string{"rogue": "dagger"}},
		},
		setLevel: 33,
		sets: map[ArmorWeight]setSpec{
			WeightHeavy: {"da Vigília", "A armadura de quem montava guarda na Cripta antes de Ozrhal acordar."},
			WeightLight: {"do Ceifador", "Couro escuro dos Confins: não faz barulho nem reflete a luz."},
			WeightRobe:  {"do Tecelão", "Os Ilvarin teciam feitiços na própria roupa."},
		},
		accLevel: 36, accShield: "skeleton_shield_large_a", accDesc: "Peça Ilvarin, achada nas ruínas do fundo do Vale.",
		accNames: map[ItemKind]string{KindShield: "Escudo Ilvarin", KindCape: "Capa dos Confins", KindNecklace: "Colar Ilvarin",
			KindEarring: "Brinco Ilvarin", KindRing: "Anel Ilvarin"},
		epicLevel: 38, epicTheme: "de Ômrun", epicDesc: "Carrega o peso de Ômrun, o Golem Ancião.",
		epicKinds:   []ItemKind{KindShield, KindCape},
		legendKind:  []ItemKind{KindNecklace, KindEarring, KindRing},
		legendLevel: 40, legendTheme: "do Abismo Ossário", legendDesc: "Ozrhal guardava isto mais perto do que a própria coroa.",
	},
}

// tierPrice: preço de loja (a venda paga SellRate): cresce com a reta e com a raridade.
func tierPrice(level int, g Grade, kind ItemKind) int64 {
	p := 100 * tierCurve(level)
	switch g {
	case GradeEpic:
		p *= 4
	case GradeLegendary:
		p *= 8
	}
	switch kind {
	case KindWeapon:
		p *= 1.2
	case KindArmor, KindShield:
	default:
		p *= 0.8
	}
	return int64(math.Round(p/10) * 10)
}

// tierGearSet: as peças e os conjuntos gerados (montados uma vez, antes de Items).
type tierGearSet struct {
	items []*ItemDef
	sets  []*ItemSet
}

var tierGear = buildTierGear()

func buildTierGear() tierGearSet {
	var out tierGearSet
	add := func(d *ItemDef) {
		d.Graded = true
		out.items = append(out.items, d)
	}
	for _, ts := range tierSpecs {
		t := ts.tier
		low := toLower(t.Letter())
		// Armas comuns e a épica de cada classe.
		for _, c := range tierClasses {
			ref := weaponProfile[c]
			for _, w := range ts.weapons {
				f := tierCurve(w.level) / tierCurve(refWeaponLevel) * ts.weaponMult
				add(weapon(fmt.Sprintf("%s_%s%d", weaponPrefix[c], low, w.level), w.names[c], c, GradeCommon, w.level,
					scaleStats(ref, f), tierPrice(w.level, GradeCommon, KindWeapon), w.models[c], w.offModel[c], w.desc))
			}
			f := tierCurve(ts.epicLevel) / tierCurve(refWeaponLevel) * gradeFactor(GradeEpic)
			m := epicWeaponModels[c]
			add(weapon(fmt.Sprintf("%s_%s_epic", weaponPrefix[c], low), epicWeaponNouns[c]+" "+ts.epicTheme, c, GradeEpic, ts.epicLevel,
				scaleStats(ref, f), tierPrice(ts.epicLevel, GradeEpic, KindWeapon), m[0], m[1], ts.epicDesc))
		}
		// Conjuntos: um por peso, cinco peças, cada uma comum e épica.
		for _, w := range Weights {
			spec := ts.sets[w]
			set := &ItemSet{ID: fmt.Sprintf("set_%s_%s", w.id(), low), Name: "Conjunto " + spec.suffix, Tier: t, Weight: w,
				Theme: setThemes[w]}
			fb := tierCurve(ts.setLevel) / tierCurve(refSetLevel)
			for i, n := range []int{2, 4} {
				set.Bonuses = append(set.Bonuses, SetBonus{Pieces: n, Bonus: scaleStats(setBonusProfile[w][i], fb)})
			}
			for _, k := range setKinds {
				nn := setNouns[w][k]
				id := fmt.Sprintf("%s_%s_%s", w.id(), low, setIDs[k])
				set.Pieces = append(set.Pieces, id)
				for _, g := range []Grade{GradeCommon, GradeEpic} {
					lvl, name, pid, desc := ts.setLevel, nn.name+" "+spec.suffix, id, spec.desc
					if g == GradeEpic {
						lvl, name, pid, desc = ts.epicLevel, nn.exalted()+" "+spec.suffix, id+"_epic", spec.desc+" "+ts.epicDesc
					}
					f := tierCurve(lvl) / tierCurve(refSetLevel) * gradeFactor(g)
					d := gear(pid, name, k, nil, g, lvl, scaleStats(setProfile[w][k], f), tierPrice(lvl, g, k), "", setIcons[k], desc)
					d.Weight, d.Set = w, set.ID
					add(d)
				}
			}
			out.sets = append(out.sets, set)
		}
		// Escudo, capa e joias: comuns, épicas e (no B) as joias lendárias.
		for _, k := range accKinds {
			classes, model := []string(nil), ""
			if k == KindShield {
				classes, model = shieldClasses, ts.accShield
			}
			f := tierCurve(ts.accLevel) / tierCurve(refAccLevel)
			add(gear(fmt.Sprintf("%s_%s", accIDs[k], low), ts.accNames[k], k, classes, GradeCommon, ts.accLevel,
				scaleStats(accProfile[k], f), tierPrice(ts.accLevel, GradeCommon, k), model, accIcons[k], ts.accDesc))
			top := func(g Grade, lvl int, suffix, theme, desc string) {
				m := model
				if k == KindShield {
					m = "shield_spikes_color"
				}
				f := tierCurve(lvl) / tierCurve(refAccLevel) * gradeFactor(g)
				add(gear(fmt.Sprintf("%s_%s_%s", accIDs[k], low, suffix), accNouns[k]+" "+theme, k, classes, g, lvl,
					scaleStats(accProfile[k], f), tierPrice(lvl, g, k), m, accIcons[k], desc))
			}
			if containsKind(ts.epicKinds, k) {
				top(GradeEpic, ts.epicLevel, "epic", ts.epicTheme, ts.epicDesc)
			}
			if containsKind(ts.legendKind, k) {
				top(GradeLegendary, ts.legendLevel, "legend", ts.legendTheme, ts.legendDesc)
			}
		}
	}
	return out
}

func containsKind(ks []ItemKind, k ItemKind) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// ItemSets: os conjuntos por id; ItemSetList: em ordem (grau, peso).
var (
	ItemSetList = tierGear.sets
	ItemSets    = func() map[string]*ItemSet {
		m := map[string]*ItemSet{}
		for _, s := range tierGear.sets {
			m[s.ID] = s
		}
		return m
	}()
)

// TierPieces: as peças feitas para um grau, de uma raridade, em ordem de id. Lê o grau pelo
// nível (as listas de saque e as receitas são montadas antes de Items marcar o Tier).
func TierPieces(t Tier, g Grade) []*ItemDef {
	var out []*ItemDef
	for _, d := range tierGear.items {
		if TierFor(d.ReqLevel) == t && d.Grade == g {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// tierLoot: todas as peças de um grau e raridade, cada uma com a sua chance (lista de saque
// de um chefe).
func tierLoot(t Tier, g Grade, chance float64) []LootEntry {
	pieces := TierPieces(t, g)
	out := make([]LootEntry, len(pieces))
	for i, d := range pieces {
		out[i] = LootEntry{d.ID, chance, 1, 1}
	}
	return out
}

// tierCommonPools[l]: as peças comuns dos graus que um monstro comum de nível l deixa cair
// (do grau do nível, pedindo até l + tierDropAhead), em ordem de id.
var tierCommonPools = func() [][]*ItemDef {
	out := make([][]*ItemDef, MaxLevel+6)
	for l := range out {
		t := TierFor(l)
		if t == TierNone {
			continue
		}
		for _, d := range tierGear.items {
			if d.Grade == GradeCommon && TierFor(d.ReqLevel) == t && d.ReqLevel <= l+tierDropAhead {
				out[l] = append(out[l], d)
			}
		}
		sort.Slice(out[l], func(i, j int) bool { return out[l][i].ID < out[l][j].ID })
	}
	return out
}()

func tierCommonPool(level int) []*ItemDef {
	if level < 0 || level >= len(tierCommonPools) {
		return nil
	}
	return tierCommonPools[level]
}

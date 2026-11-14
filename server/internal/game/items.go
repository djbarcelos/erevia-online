package game

import (
	"math"
	"math/rand/v2"
	"sort"
)

// ItemKind é o tipo de um item. Os valores batem com o enum do protocolo.
type ItemKind uint8

const (
	KindNone ItemKind = iota
	KindWeapon
	KindShield
	KindArmor
	KindHelmet
	KindPotion
	KindMaterial
	KindGold // só existe no chão: ao pegar, vira saldo de moedas
	KindLegs
	KindGloves
	KindBoots
	KindCape
	KindNecklace
	KindEarring
	KindRing
	KindScroll // pergaminhos (retorno, encantar)
)

// Grade é a raridade de um item (cor do nome).
type Grade uint8

const (
	GradeCommon Grade = iota + 1
	GradeGood
	GradeRare
	GradeEpic
	GradeLegendary // só o raid boss deixa cair
)

// Encaixes de equipamento. Os 4 primeiros são os de antes (os números ficam iguais no
// banco); brincos e anéis têm dois encaixes cada.
const (
	SlotWeapon  = iota // arma
	SlotOffhand        // mão secundária
	SlotArmor          // peito
	SlotHelmet         // cabeça
	SlotLegs
	SlotHands
	SlotFeet
	SlotCape
	SlotNeck
	SlotEar1
	SlotEar2
	SlotRing1
	SlotRing2
	EquipSlots
	NoSlot = -1
)

// SlotNames: nome de cada encaixe (mensagens do servidor).
var SlotNames = [EquipSlots]string{
	"Arma", "Mão secundária", "Peito", "Cabeça", "Pernas", "Mãos", "Pés", "Capa",
	"Colar", "Brinco", "Brinco", "Anel", "Anel",
}

// kindSlots: em quais encaixes cada tipo de item pode ir (vazio = não se equipa).
var kindSlots = map[ItemKind][]int{
	KindWeapon:   {SlotWeapon},
	KindShield:   {SlotOffhand},
	KindArmor:    {SlotArmor},
	KindHelmet:   {SlotHelmet},
	KindLegs:     {SlotLegs},
	KindGloves:   {SlotHands},
	KindBoots:    {SlotFeet},
	KindCape:     {SlotCape},
	KindNecklace: {SlotNeck},
	KindEarring:  {SlotEar1, SlotEar2},
	KindRing:     {SlotRing1, SlotRing2},
}

// Tipos de arma (a passiva Esgrima vale com espadas).
const (
	WeaponSword   = "sword"
	WeaponAxe     = "axe"
	WeaponStaff   = "staff"
	WeaponDaggers = "daggers"
)

// Grupos de recarga de consumíveis.
const (
	CooldownNone = iota
	CooldownHP
	CooldownMP
	CooldownReturn // Pergaminho de Retorno
)

// CooldownSeconds: recarga de cada grupo de consumível.
func CooldownSeconds(group int) float64 {
	if group == CooldownReturn {
		return ReturnCooldown
	}
	return PotionCooldown
}

const (
	BagSize        = 40
	MaxStack       = 999
	SellRate       = 0.4 // a loja paga 40% do preço
	PotionCooldown = 10.0
	ReturnCooldown = 30.0 // Pergaminho de Retorno
	ReturnChannel  = 5.0  // segundos lendo o pergaminho
	StarterGold    = 50
	GoldID         = "gold"
)

// ItemDef descreve um tipo de item. Itens são sempre criados e destruídos pelo servidor.
type ItemDef struct {
	ID          string
	Name        string
	Kind        ItemKind
	Grade       Grade
	Classes     []string // classes que podem equipar (nil = todas)
	WeaponType  string   // armas: espada, machado, cajado ou adagas
	ReqLevel    int
	Bonus       Stats // somado aos atributos quando equipado
	Stack       int   // máximo por espaço (1 = não empilha)
	Price       int64 // preço na loja (a venda paga SellRate disso)
	Model       string
	OffModel    string // arma de duas peças (adagas): modelo da outra mão
	Icon        string
	Description string
	Cooldown    int // grupo de recarga (consumíveis)
	HealHP      float64
	HealMP      float64
	Scroll      ScrollKind // pergaminhos: o que faz
	Blessed     bool       // encantar: falhar não quebra (volta a +0)
	RepOnly     bool       // só na loja de reputação (reputation.go): não cai de monstro
}

// SellPrice é quanto a loja paga por uma unidade.
func (d *ItemDef) SellPrice() int64 { return int64(math.Floor(float64(d.Price) * SellRate)) }

// Slots: encaixes onde o item pode ir (nil = não se equipa).
func (d *ItemDef) Slots() []int { return kindSlots[d.Kind] }

// Slot: o primeiro encaixe do item (NoSlot = não se equipa).
func (d *ItemDef) Slot() int {
	if s := d.Slots(); len(s) > 0 {
		return s[0]
	}
	return NoSlot
}

// Fits informa se o item cabe no encaixe.
func (d *ItemDef) Fits(slot int) bool {
	for _, s := range d.Slots() {
		if s == slot {
			return true
		}
	}
	return false
}

// Equippable informa se o item vai num encaixe de equipamento.
func (d *ItemDef) Equippable() bool { return len(d.Slots()) > 0 }

// UsableBy informa se a classe pode equipar o item.
func (d *ItemDef) UsableBy(class string) bool {
	if len(d.Classes) == 0 {
		return true
	}
	for _, c := range d.Classes {
		if c == class {
			return true
		}
	}
	return false
}

func weapon(id, name, class string, g Grade, lvl int, bonus Stats, price int64, model, off, desc string) *ItemDef {
	return &ItemDef{ID: id, Name: name, Kind: KindWeapon, Grade: g, Classes: []string{class}, WeaponType: weaponTypeOf(class),
		ReqLevel: lvl, Bonus: bonus, Stack: 1, Price: price, Model: model, OffModel: off, Icon: iconFor(class), Description: desc}
}

func weaponTypeOf(class string) string {
	switch class {
	case "barbarian":
		return WeaponAxe
	case "mage":
		return WeaponStaff
	case "rogue":
		return WeaponDaggers
	}
	return WeaponSword
}

func iconFor(class string) string {
	switch class {
	case "knight":
		return "sword"
	case "barbarian":
		return "axe"
	case "mage":
		return "staff"
	case "rogue":
		return "daggers"
	}
	return "sword"
}

func gear(id, name string, kind ItemKind, classes []string, g Grade, lvl int, bonus Stats, price int64, model, icon, desc string) *ItemDef {
	return &ItemDef{ID: id, Name: name, Kind: kind, Grade: g, Classes: classes,
		ReqLevel: lvl, Bonus: bonus, Stack: 1, Price: price, Model: model, Icon: icon, Description: desc}
}

func stackable(id, name string, kind ItemKind, price int64, icon, desc string) *ItemDef {
	return &ItemDef{ID: id, Name: name, Kind: kind, Grade: GradeCommon, Stack: MaxStack,
		Price: price, Icon: icon, Description: desc}
}

func scroll(id, name string, kind ScrollKind, price int64, desc string) *ItemDef {
	d := stackable(id, name, KindScroll, price, "scroll", desc)
	d.Scroll = kind
	if kind == ScrollReturn {
		d.Cooldown = CooldownReturn
	}
	return d
}

// blessed: pergaminho abençoado (raridade boa, falhar não quebra).
func blessed(d *ItemDef) *ItemDef {
	d.Blessed = true
	d.Grade = GradeRare
	return d
}

func potion(id, name string, group int, hp, mp float64, price int64, icon, desc string) *ItemDef {
	d := stackable(id, name, KindPotion, price, icon, desc)
	d.Cooldown, d.HealHP, d.HealMP = group, hp, mp
	return d
}

var shieldClasses = []string{"knight", "barbarian"}

// Items é o catálogo do jogo, por id.
var Items = func() map[string]*ItemDef {
	list := []*ItemDef{
		// Cavaleiro: espadas.
		weapon("sword_common", "Espada de Treino", "knight", GradeCommon, 1, Stats{PAtk: 6}, 100, "sword_1handed", "",
			"Lâmina simples, mas equilibrada."),
		weapon("sword_good", "Espada de Aço", "knight", GradeGood, 6, Stats{PAtk: 14}, 1200, "sword_1handed", "",
			"Aço temperado pelo ferreiro da cidade."),
		weapon("sword_rare", "Lâmina do Vigia", "knight", GradeRare, 13, Stats{PAtk: 24, MaxHP: 30}, 4000, "sword_2handed", "",
			"Arma dos antigos guardiões da muralha."),
		weapon("sword_epic", "Lâmina da Aurora", "knight", GradeEpic, 18, Stats{PAtk: 36, MaxHP: 60, Crit: 0.03}, 12000, "sword_2handed_color", "",
			"Dizem que brilha mais forte perto dos mortos-vivos."),
		// Bárbaro: machados.
		weapon("axe_common", "Machado de Lenhador", "barbarian", GradeCommon, 1, Stats{PAtk: 7}, 100, "axe_1handed", "",
			"Serve para lenha e para ossos."),
		weapon("axe_good", "Machado de Guerra", "barbarian", GradeGood, 6, Stats{PAtk: 16}, 1200, "axe_1handed", "",
			"Lâmina larga e cabo reforçado."),
		weapon("axe_rare", "Machado Rachador", "barbarian", GradeRare, 13, Stats{PAtk: 27, Crit: 0.02}, 4000, "axe_2handed", "",
			"Pesado o bastante para partir um escudo."),
		weapon("axe_epic", "Fúria do Norte", "barbarian", GradeEpic, 18, Stats{PAtk: 40, Crit: 0.04}, 12000, "axe_2handed", "",
			"Forjado no gelo, temperado em sangue."),
		// Mago: cajados.
		weapon("staff_common", "Cajado de Aprendiz", "mage", GradeCommon, 1, Stats{MAtk: 8}, 100, "staff", "",
			"Madeira comum com uma pedra que zumbe."),
		weapon("staff_good", "Cajado de Carvalho", "mage", GradeGood, 6, Stats{MAtk: 17, MaxMP: 20}, 1200, "staff", "",
			"Carvalho antigo, bom condutor de magia."),
		weapon("staff_rare", "Varinha Rúnica", "mage", GradeRare, 13, Stats{MAtk: 28, MaxMP: 50}, 4000, "wand", "",
			"Runas gravadas que guardam o eco de feitiços."),
		weapon("staff_epic", "Cetro do Abismo", "mage", GradeEpic, 18, Stats{MAtk: 42, MaxMP: 90, Crit: 0.03}, 12000, "staff", "",
			"Arrancado do salão do Senhor dos Ossos."),
		// Ladino: pares de adagas.
		weapon("daggers_common", "Adagas Gêmeas", "rogue", GradeCommon, 1, Stats{PAtk: 5}, 100, "dagger", "dagger",
			"Um par de lâminas curtas e rápidas."),
		weapon("daggers_good", "Adagas de Aço", "rogue", GradeGood, 6, Stats{PAtk: 12, Crit: 0.02}, 1200, "dagger", "dagger",
			"Afiadas a ponto de cortar o vento."),
		weapon("daggers_rare", "Presas Sombrias", "rogue", GradeRare, 13, Stats{PAtk: 21, Crit: 0.04}, 4000, "dagger", "dagger",
			"Não refletem a luz."),
		weapon("daggers_epic", "Presas do Crepúsculo", "rogue", GradeEpic, 18, Stats{PAtk: 32, Crit: 0.06}, 12000, "dagger", "dagger",
			"Ninguém viu o golpe; só o resultado."),

		// Escudos (Cavaleiro e Bárbaro).
		gear("shield_badge", "Escudo de Brasão", KindShield, shieldClasses, GradeCommon, 1, Stats{PDef: 6}, 90,
			"shield_badge_color", "shield", "O brasão da cidade, pintado à mão."),
		gear("shield_buckler", "Broquel Bárbaro", KindShield, shieldClasses, GradeCommon, 1, Stats{PDef: 4, PAtk: 1}, 90,
			"shield_round_barbarian", "shield_round", "Pequeno e leve, bom para empurrar."),
		gear("shield_good", "Escudo de Ferro", KindShield, shieldClasses, GradeGood, 6, Stats{PDef: 14, MDef: 4}, 1000,
			"shield_square", "shield", "Placas de ferro rebitadas."),
		gear("shield_rare", "Escudo de Cravos", KindShield, shieldClasses, GradeRare, 13, Stats{PDef: 24, MDef: 8}, 3500,
			"shield_spikes", "shield_round", "Quem bate nele se arrepende."),
		gear("shield_epic", "Égide Óssea", KindShield, shieldClasses, GradeEpic, 18, Stats{PDef: 36, MDef: 14, MaxHP: 50}, 10000,
			"shield_spikes_color", "shield_round", "Feita com a armadura do Senhor dos Ossos."),

		// Armaduras e elmos (qualquer classe). Peças de corpo não aparecem no modelo 3D.
		gear("armor_common", "Gibão de Couro", KindArmor, nil, GradeCommon, 1, Stats{PDef: 5, MDef: 3, MaxHP: 15}, 120,
			"", "armor", "Couro curtido, costurado em camadas."),
		gear("armor_good", "Cota de Malha", KindArmor, nil, GradeGood, 6, Stats{PDef: 13, MDef: 7, MaxHP: 40}, 1100,
			"", "armor", "Milhares de anéis de ferro entrelaçados."),
		gear("armor_rare", "Couraça de Osso", KindArmor, nil, GradeRare, 13, Stats{PDef: 24, MDef: 14, MaxHP: 80}, 3800,
			"", "armor", "Ossos de guerreiros antigos, presos com tiras de couro."),
		gear("armor_epic", "Manto do Senhor dos Ossos", KindArmor, nil, GradeEpic, 18, Stats{PDef: 34, MDef: 26, MaxHP: 130, MaxMP: 40}, 11000,
			"", "armor", "Ainda está frio ao toque."),
		gear("helm_common", "Capuz de Couro", KindHelmet, nil, GradeCommon, 1, Stats{PDef: 2, MDef: 3, MaxMP: 10}, 80,
			"", "helmet", "Protege do sol e de pancadas leves."),
		gear("helm_good", "Elmo de Ferro", KindHelmet, nil, GradeGood, 6, Stats{PDef: 6, MDef: 6, MaxMP: 25}, 800,
			"", "helmet", "Pesado, mas salva cabeças."),
		gear("helm_rare", "Elmo de Osso", KindHelmet, nil, GradeRare, 13, Stats{PDef: 11, MDef: 11, MaxMP: 45}, 3000,
			"", "helmet", "Um crânio enorme, adaptado para cabeças humanas."),

		gear("helm_epic", "Elmo do Senhor dos Ossos", KindHelmet, nil, GradeEpic, 18, Stats{PDef: 16, MDef: 16, MaxHP: 40, MaxMP: 70}, 9000,
			"", "helmet", "Os olhos vazios ainda parecem vigiar."),

		// Armas e escudo que só mudam o visual (opções da criação de personagem).
		weapon("blade_common", "Lâmina Curva", "knight", GradeCommon, 1, Stats{PAtk: 6}, 100, "skeleton_blade", "",
			"Curva e leve: corta mais do que perfura."),
		weapon("axe_jagged_common", "Machado Serrilhado", "barbarian", GradeCommon, 1, Stats{PAtk: 7}, 100, "skeleton_axe", "",
			"Os dentes da lâmina prendem e rasgam."),
		weapon("wand_common", "Varinha de Aprendiz", "mage", GradeCommon, 1, Stats{MAtk: 8}, 100, "wand", "",
			"Curta e precisa, boa para quem está começando."),
		weapon("staff_dark_common", "Cajado Sombrio", "mage", GradeCommon, 1, Stats{MAtk: 8}, 100, "skeleton_staff", "",
			"Madeira escura que parece beber a luz."),
		weapon("blades_common", "Lâminas Curvas", "rogue", GradeCommon, 1, Stats{PAtk: 5}, 100, "skeleton_blade", "skeleton_blade",
			"Duas lâminas curvas, rápidas no corte."),
		weapon("daggers_mixed_common", "Adaga e Lâmina", "rogue", GradeCommon, 1, Stats{PAtk: 5}, 100, "dagger", "skeleton_blade",
			"Uma para aparar, outra para cortar."),
		gear("shield_tower", "Escudo Pesado", KindShield, shieldClasses, GradeCommon, 1, Stats{PDef: 8, Evasion: -3}, 90,
			"skeleton_shield_large_a", "shield", "Grande e pesado: segura tudo, menos a pressa."),

		// Pernas.
		gear("legs_common", "Calças de Couro", KindLegs, nil, GradeCommon, 1, Stats{PDef: 3, MDef: 2, MaxHP: 10}, 90,
			"", "legs", "Couro grosso nos joelhos."),
		gear("legs_good", "Grevas de Malha", KindLegs, nil, GradeGood, 6, Stats{PDef: 9, MDef: 4, MaxHP: 25}, 900,
			"", "legs", "Malha de ferro sobre calças acolchoadas."),
		gear("legs_rare", "Grevas de Osso", KindLegs, nil, GradeRare, 13, Stats{PDef: 16, MDef: 9, MaxHP: 55}, 3200,
			"", "legs", "Placas de osso que estalam a cada passo."),
		gear("legs_epic", "Grevas do Crepúsculo", KindLegs, nil, GradeEpic, 18, Stats{PDef: 24, MDef: 15, MaxHP: 90, Evasion: 4}, 9500,
			"", "legs", "Leves como sombra, duras como pedra."),
		// Mãos.
		gear("gloves_common", "Luvas de Couro", KindGloves, nil, GradeCommon, 1, Stats{PDef: 2, Accuracy: 3}, 70,
			"", "gloves", "Boa pegada no cabo da arma."),
		gear("gloves_good", "Manoplas de Ferro", KindGloves, nil, GradeGood, 6, Stats{PDef: 5, PAtk: 3, MAtk: 3, Accuracy: 5}, 800,
			"", "gloves", "Dedos articulados de ferro batido."),
		gear("gloves_rare", "Manoplas Rúnicas", KindGloves, nil, GradeRare, 13, Stats{PDef: 9, PAtk: 6, MAtk: 6, AtkSpeed: 0.05, CastSpeed: 0.05}, 3400,
			"", "gloves", "As runas acendem quando o portador está em perigo."),
		gear("gloves_epic", "Garras do Ossário", KindGloves, nil, GradeEpic, 18, Stats{PDef: 13, PAtk: 10, MAtk: 10, AtkSpeed: 0.08, CastSpeed: 0.08, Crit: 0.02}, 10500,
			"", "gloves", "Unhas de osso afiadas como navalhas."),
		// Pés.
		gear("boots_common", "Botas de Viajante", KindBoots, nil, GradeCommon, 1, Stats{PDef: 2, Speed: 0.2}, 80,
			"", "boots", "Já andaram por muitas estradas."),
		gear("boots_good", "Botas de Ferro", KindBoots, nil, GradeGood, 6, Stats{PDef: 6, MDef: 3, Speed: 0.3}, 850,
			"", "boots", "Pesadas, mas o passo é firme."),
		gear("boots_rare", "Botas do Vento", KindBoots, nil, GradeRare, 13, Stats{PDef: 10, MDef: 6, Speed: 0.4, Evasion: 4}, 3300,
			"", "boots", "Dizem que não deixam pegadas."),
		gear("boots_epic", "Passos da Sombra", KindBoots, nil, GradeEpic, 18, Stats{PDef: 15, MDef: 10, Speed: 0.5, Evasion: 8}, 10000,
			"", "boots", "Quem as usa chega antes de ser visto."),
		// Capas.
		gear("cape_common", "Capa de Lã", KindCape, nil, GradeCommon, 1, Stats{MDef: 3, Evasion: 2}, 80,
			"", "cape", "Esquenta nas noites frias da estrada."),
		gear("cape_good", "Capa do Patrulheiro", KindCape, nil, GradeGood, 6, Stats{MDef: 7, Evasion: 3, HPRegen: 0.5}, 900,
			"", "cape", "Verde-escura, para sumir entre as árvores."),
		gear("cape_rare", "Capa do Crepúsculo", KindCape, nil, GradeRare, 13, Stats{MDef: 13, Evasion: 5, HPRegen: 1}, 3500,
			"", "cape", "Tecida com fios que bebem a luz."),
		gear("cape_epic", "Manto das Cinzas", KindCape, nil, GradeEpic, 18, Stats{MDef: 21, Evasion: 7, MaxHP: 60, HPRegen: 2}, 10500,
			"", "cape", "Cheira a fogueira apagada há séculos."),
		// Colares.
		gear("neck_common", "Colar de Contas", KindNecklace, nil, GradeCommon, 1, Stats{MDef: 3, MaxMP: 15}, 100,
			"", "necklace", "Contas de madeira pintadas à mão."),
		gear("neck_good", "Colar do Lobo", KindNecklace, nil, GradeGood, 6, Stats{MDef: 8, MaxHP: 30}, 1000,
			"", "necklace", "Um dente de lobo preso num cordão de couro."),
		gear("neck_rare", "Amuleto Rúnico", KindNecklace, nil, GradeRare, 13, Stats{MDef: 14, MaxHP: 50, MaxMP: 40}, 3800,
			"", "necklace", "Uma pedra gravada que esquenta perto de magia."),
		gear("neck_epic", "Coração do Abismo", KindNecklace, nil, GradeEpic, 18, Stats{MDef: 22, MaxHP: 90, MaxMP: 80, PAtk: 6, MAtk: 6}, 11500,
			"", "necklace", "Bate devagar, como um coração de verdade."),
		// Brincos (cabem nos dois encaixes de brinco).
		gear("ear_common", "Brinco de Osso", KindEarring, nil, GradeCommon, 1, Stats{MDef: 2, MPRegen: 0.2}, 60,
			"", "earring", "Um ossinho polido. Simples, mas dá sorte."),
		gear("ear_good", "Brinco de Âmbar", KindEarring, nil, GradeGood, 6, Stats{MDef: 5, MaxMP: 20, MPRegen: 0.4}, 700,
			"", "earring", "Uma gota de resina com um inseto dentro."),
		gear("ear_rare", "Brinco de Safira", KindEarring, nil, GradeRare, 13, Stats{MDef: 9, MaxMP: 35, CastSpeed: 0.04}, 2700,
			"", "earring", "Faz os feitiços saírem mais depressa."),
		gear("ear_epic", "Lágrima da Lua", KindEarring, nil, GradeEpic, 18, Stats{MDef: 14, MaxMP: 60, CastSpeed: 0.06, MAtk: 5}, 8500,
			"", "earring", "Brilha mais forte nas noites sem nuvem."),
		// Anéis (cabem nos dois encaixes de anel).
		gear("ring_common", "Anel de Cobre", KindRing, nil, GradeCommon, 1, Stats{MDef: 2, PAtk: 1, MAtk: 1}, 60,
			"", "ring", "Verde nas bordas, mas ainda serve."),
		gear("ring_good", "Anel de Prata", KindRing, nil, GradeGood, 6, Stats{MDef: 4, Crit: 0.01, Accuracy: 4}, 700,
			"", "ring", "Prata fina, gravada com o brasão da cidade."),
		gear("ring_rare", "Anel de Ônix", KindRing, nil, GradeRare, 13, Stats{MDef: 8, Crit: 0.02, CritDamage: 0.1}, 2700,
			"", "ring", "Uma pedra negra que parece engolir a luz."),
		gear("ring_epic", "Selo do Senhor dos Ossos", KindRing, nil, GradeEpic, 18, Stats{MDef: 12, Crit: 0.03, CritDamage: 0.2, PAtk: 5, MAtk: 5}, 8500,
			"", "ring", "Quem usa o selo dá ordens aos mortos. Ou acha que dá."),

		// Lendários: só o raid boss (Ozrhal) deixa cair.
		weapon("sword_legend", "Lâmina do Rei Ossário", "knight", GradeLegendary, 20, Stats{PAtk: 44, MaxHP: 90, Crit: 0.04}, 30000, "sword_2handed_color", "",
			"Fria ao toque. Os mortos recuam quando ela é erguida."),
		weapon("axe_legend", "Machado do Rei Ossário", "barbarian", GradeLegendary, 20, Stats{PAtk: 49, Crit: 0.05, CritDamage: 0.15}, 30000, "axe_2handed", "",
			"Cada golpe ecoa como um sino de cripta."),
		weapon("staff_legend", "Cetro do Rei Ossário", "mage", GradeLegendary, 20, Stats{MAtk: 51, MaxMP: 120, Crit: 0.04, CastSpeed: 0.05}, 30000, "staff", "",
			"O cetro de Ozrhal. As runas ainda sussurram ordens."),
		weapon("daggers_legend", "Garras do Rei Ossário", "rogue", GradeLegendary, 20, Stats{PAtk: 39, Crit: 0.08, CritDamage: 0.1}, 30000, "dagger", "dagger",
			"Feitas dos dedos do próprio rei. Não pergunte como."),
		gear("ring_legend", "Anel do Rei Ossário", KindRing, nil, GradeLegendary, 20, Stats{MDef: 15, Crit: 0.04, CritDamage: 0.25, PAtk: 7, MAtk: 7}, 26000,
			"", "ring", "Pesa mais do que deveria."),
		gear("neck_legend", "Colar do Rei Ossário", KindNecklace, nil, GradeLegendary, 20, Stats{MDef: 27, MaxHP: 120, MaxMP: 100, PAtk: 8, MAtk: 8}, 28000,
			"", "necklace", "Vértebras de ouro presas num fio de prata."),

		// Poções.
		potion("potion_hp_s", "Poção de Vida Menor", CooldownHP, 80, 0, 20, "potion_hp", "Recupera 80 de vida na hora."),
		potion("potion_hp_l", "Poção de Vida Maior", CooldownHP, 260, 0, 75, "potion_hp_big", "Recupera 260 de vida na hora."),
		potion("potion_mp_s", "Poção de Mana Menor", CooldownMP, 0, 50, 25, "potion_mp", "Recupera 50 de mana na hora."),
		potion("potion_mp_l", "Poção de Mana Maior", CooldownMP, 0, 160, 90, "potion_mp_big", "Recupera 160 de mana na hora."),

		// Pergaminhos.
		scroll("scroll_return", "Pergaminho de Retorno", ScrollReturn, 120,
			"Lido por 5 s, leva você de volta à cidade. Levar dano ou andar interrompe. Não funciona com karma."),
		scroll("scroll_ench_weapon", "Pergaminho de Encantar Arma", ScrollEnchantWeapon, 1500,
			"Sobe a arma em +1. Até +3 é seguro; acima disso pode falhar, e a arma quebra."),
		scroll("scroll_ench_armor", "Pergaminho de Encantar Armadura", ScrollEnchantArmor, 600,
			"Sobe armadura, elmo, escudo, roupa ou joia em +1. Até +3 é seguro; acima disso pode falhar, e o item quebra."),
		blessed(scroll("scroll_ench_weapon_b", "Pergaminho Abençoado de Arma", ScrollEnchantWeapon, 6000,
			"Sobe a arma em +1. Se falhar, a arma não quebra: volta a +0.")),
		blessed(scroll("scroll_ench_armor_b", "Pergaminho Abençoado de Armadura", ScrollEnchantArmor, 2500,
			"Sobe a peça em +1. Se falhar, ela não quebra: volta a +0.")),

		// Materiais (só para vender).
		stackable("bone_old", "Osso Velho", KindMaterial, 10, "bone", "Um osso qualquer. Alguém na cidade compra."),
		stackable("skull_cracked", "Crânio Rachado", KindMaterial, 35, "skull", "Os alquimistas pagam bem por isso."),
		stackable("shadow_essence", "Essência Sombria", KindMaterial, 100, "essence", "Fumaça escura presa num frasco."),
		stackable("rune_core", "Núcleo Rúnico", KindMaterial, 220, "rune_core", "O coração de um Golem de Pedra. Ainda pulsa, morno, na mão."),
		stackable("raid_fragment", "Fragmento do Rei Ossário", KindMaterial, 5000, "crown",
			"Um pedaço da coroa de Ozrhal. Ainda está frio como uma tumba."),
		stackable("broken_crown", "Coroa Partida", KindMaterial, 1500, "crown", "A coroa do Senhor dos Ossos. Vale uma fortuna."),

		// Moedas no chão.
		{ID: GoldID, Name: "Moedas de ouro", Kind: KindGold, Grade: GradeCommon, Stack: math.MaxInt32, Icon: "gold",
			Description: "Dinheiro corrente em Erevia."},
	}
	list = append(list, RepItems...) // as capas das facções (reputation.go)
	m := make(map[string]*ItemDef, len(list))
	for _, d := range list {
		m[d.ID] = d
	}
	return m
}()

// ItemIDs devolve os ids do catálogo em ordem (para enviar e sortear de forma estável).
func ItemIDs() []string {
	ids := make([]string, 0, len(Items))
	for id := range Items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ItemRecord é um item guardado: na bolsa (Equipped=false, Slot = espaço) ou equipado
// (Slot = encaixe). Usado no kit inicial e na persistência.
type ItemRecord struct {
	Equipped bool
	Slot     int
	ItemID   string
	Count    int
	Enchant  int // +N (0 = sem encantamento)
}

// StarterKit: o kit padrão da classe (aparência padrão).
func StarterKit(class string) []ItemRecord { return StarterKitFor(class, DefaultLook(class)) }

// StarterKitFor: a arma (e o escudo) escolhidos na criação, equipados, mais poções. Quem
// escolheu "nada" na mão secundária recebe o escudo padrão na bolsa, para não perder nada.
func StarterKitFor(class string, look Look) []ItemRecord {
	opts, ok := StarterLooks[class]
	if !ok {
		class, opts = "knight", StarterLooks["knight"]
	}
	if !ValidLook(class, look) {
		look = DefaultLook(class)
	}
	kit := []ItemRecord{{Equipped: true, Slot: SlotWeapon, ItemID: look.Weapon, Count: 1}}
	if look.Offhand != "" {
		kit = append(kit, ItemRecord{Equipped: true, Slot: SlotOffhand, ItemID: look.Offhand, Count: 1})
	}
	kit = append(kit, ItemRecord{Slot: 0, ItemID: "potion_hp_s", Count: 5})
	if look.Offhand == "" && opts.Offhands[0] != "" {
		kit = append(kit, ItemRecord{Slot: 1, ItemID: opts.Offhands[0], Count: 1})
	}
	return kit
}

// Lojas dos NPCs: ids de item à venda, na ordem da lista.
var (
	MerchantStock = []string{
		"potion_hp_s", "potion_hp_l", "potion_mp_s", "potion_mp_l", "scroll_return",
		"neck_common", "neck_good", "ear_common", "ear_good", "ring_common", "ring_good",
	}
	SmithStock = []string{
		"sword_common", "blade_common", "sword_good", "axe_common", "axe_jagged_common", "axe_good",
		"staff_common", "wand_common", "staff_dark_common", "staff_good",
		"daggers_common", "blades_common", "daggers_mixed_common", "daggers_good",
		"shield_badge", "shield_buckler", "shield_tower", "shield_good",
		"armor_common", "armor_good", "helm_common", "helm_good", "legs_common", "legs_good",
		"gloves_common", "gloves_good", "boots_common", "boots_good", "cape_common", "cape_good",
		"scroll_ench_weapon", "scroll_ench_armor",
	}
)

// ---------- Drops ----------

// LootEntry é a chance de um item cair, com quantidade sorteada entre Min e Max.
type LootEntry struct {
	ItemID   string
	Chance   float64
	Min, Max int
}

// Drop é um item que caiu de um monstro.
type Drop struct {
	Item  *ItemDef
	Count int
}

// gearByGrade: equipamentos de cada raridade, em ordem de id (sorteio estável).
var gearByGrade = func() map[Grade][]*ItemDef {
	m := map[Grade][]*ItemDef{}
	for _, id := range ItemIDs() {
		d := Items[id]
		if d.Equippable() && !d.RepOnly { // as da loja de reputação não caem
			m[d.Grade] = append(m[d.Grade], d)
		}
	}
	return m
}()

// Saque de equipamento do raid boss.
const (
	RaidEpicDrops       = 2
	RaidLegendaryChance = 0.35
)

// gearChances: chance de cair um equipamento de cada raridade, pelo nível do monstro.
func gearChances(level int, boss bool) map[Grade]float64 {
	switch {
	case boss:
		return map[Grade]float64{GradeEpic: 0.3, GradeRare: 1}
	case level >= 13:
		return map[Grade]float64{GradeRare: 0.015, GradeGood: 0.03}
	case level >= 6:
		return map[Grade]float64{GradeGood: 0.025, GradeCommon: 0.03}
	default:
		return map[Grade]float64{GradeCommon: 0.03}
	}
}

// RollLoot sorteia o que um monstro deixa ao morrer.
func RollLoot(m *MonsterDef, level int, rng *rand.Rand) []Drop {
	var out []Drop
	if m.GoldChance > 0 && rng.Float64() < m.GoldChance {
		per := m.GoldMin + rng.Float64()*(m.GoldMax-m.GoldMin)
		if n := int(math.Round(per * float64(level))); n > 0 {
			out = append(out, Drop{Item: Items[GoldID], Count: n})
		}
	}
	for _, e := range m.Loot {
		if rng.Float64() >= e.Chance {
			continue
		}
		n := e.Min
		if e.Max > e.Min {
			n += rng.IntN(e.Max - e.Min + 1)
		}
		if d := Items[e.ItemID]; d != nil && n > 0 {
			out = append(out, Drop{Item: d, Count: n})
		}
	}
	if m.NoGear {
		return out
	}
	if m.Raid {
		// Raid: 2 épicos garantidos e 35% de chance de um lendário.
		for range RaidEpicDrops {
			pool := gearByGrade[GradeEpic]
			out = append(out, Drop{Item: pool[rng.IntN(len(pool))], Count: 1})
		}
		if rng.Float64() < RaidLegendaryChance {
			pool := gearByGrade[GradeLegendary]
			out = append(out, Drop{Item: pool[rng.IntN(len(pool))], Count: 1})
		}
		return out
	}
	chances := gearChances(level, m.Boss)
	for _, g := range []Grade{GradeEpic, GradeRare, GradeGood, GradeCommon} {
		chance, ok := chances[g]
		if !ok || rng.Float64() >= chance {
			continue
		}
		pool := gearByGrade[g]
		out = append(out, Drop{Item: pool[rng.IntN(len(pool))], Count: 1})
	}
	return out
}

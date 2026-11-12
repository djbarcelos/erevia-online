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
)

// Grade é a raridade de um item (cor do nome).
type Grade uint8

const (
	GradeCommon Grade = iota + 1
	GradeGood
	GradeRare
	GradeEpic
)

// Encaixes de equipamento.
const (
	SlotWeapon = iota
	SlotOffhand
	SlotArmor
	SlotHelmet
	EquipSlots
	NoSlot = -1
)

// Grupos de recarga de consumíveis.
const (
	CooldownNone = iota
	CooldownHP
	CooldownMP
)

const (
	BagSize        = 40
	MaxStack       = 999
	SellRate       = 0.4 // a loja paga 40% do preço
	PotionCooldown = 10.0
	StarterGold    = 50
	GoldID         = "gold"
)

// ItemDef descreve um tipo de item. Itens são sempre criados e destruídos pelo servidor.
type ItemDef struct {
	ID          string
	Name        string
	Kind        ItemKind
	Grade       Grade
	Slot        int      // encaixe de equipamento (NoSlot = não se equipa)
	Classes     []string // classes que podem equipar (nil = todas)
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
}

// SellPrice é quanto a loja paga por uma unidade.
func (d *ItemDef) SellPrice() int64 { return int64(math.Floor(float64(d.Price) * SellRate)) }

// Equippable informa se o item vai num encaixe de equipamento.
func (d *ItemDef) Equippable() bool { return d.Slot != NoSlot }

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
	return &ItemDef{ID: id, Name: name, Kind: KindWeapon, Grade: g, Slot: SlotWeapon, Classes: []string{class},
		ReqLevel: lvl, Bonus: bonus, Stack: 1, Price: price, Model: model, OffModel: off, Icon: iconFor(class), Description: desc}
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

func gear(id, name string, kind ItemKind, slot int, classes []string, g Grade, lvl int, bonus Stats, price int64, model, icon, desc string) *ItemDef {
	return &ItemDef{ID: id, Name: name, Kind: kind, Grade: g, Slot: slot, Classes: classes,
		ReqLevel: lvl, Bonus: bonus, Stack: 1, Price: price, Model: model, Icon: icon, Description: desc}
}

func stackable(id, name string, kind ItemKind, price int64, icon, desc string) *ItemDef {
	return &ItemDef{ID: id, Name: name, Kind: kind, Grade: GradeCommon, Slot: NoSlot, Stack: MaxStack,
		Price: price, Icon: icon, Description: desc}
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
		gear("shield_badge", "Escudo de Brasão", KindShield, SlotOffhand, shieldClasses, GradeCommon, 1, Stats{PDef: 6}, 90,
			"shield_badge_color", "shield", "O brasão da cidade, pintado à mão."),
		gear("shield_buckler", "Broquel Bárbaro", KindShield, SlotOffhand, shieldClasses, GradeCommon, 1, Stats{PDef: 4, PAtk: 1}, 90,
			"shield_round_barbarian", "shield_round", "Pequeno e leve, bom para empurrar."),
		gear("shield_good", "Escudo de Ferro", KindShield, SlotOffhand, shieldClasses, GradeGood, 6, Stats{PDef: 14, MDef: 4}, 1000,
			"shield_square", "shield", "Placas de ferro rebitadas."),
		gear("shield_rare", "Escudo de Cravos", KindShield, SlotOffhand, shieldClasses, GradeRare, 13, Stats{PDef: 24, MDef: 8}, 3500,
			"shield_spikes", "shield_round", "Quem bate nele se arrepende."),
		gear("shield_epic", "Égide Óssea", KindShield, SlotOffhand, shieldClasses, GradeEpic, 18, Stats{PDef: 36, MDef: 14, MaxHP: 50}, 10000,
			"shield_spikes_color", "shield_round", "Feita com a armadura do Senhor dos Ossos."),

		// Armaduras e elmos (qualquer classe). Ainda não aparecem no boneco.
		gear("armor_common", "Gibão de Couro", KindArmor, SlotArmor, nil, GradeCommon, 1, Stats{PDef: 5, MDef: 3, MaxHP: 15}, 120,
			"", "armor", "Couro curtido, costurado em camadas."),
		gear("armor_good", "Cota de Malha", KindArmor, SlotArmor, nil, GradeGood, 6, Stats{PDef: 13, MDef: 7, MaxHP: 40}, 1100,
			"", "armor", "Milhares de anéis de ferro entrelaçados."),
		gear("armor_rare", "Couraça de Osso", KindArmor, SlotArmor, nil, GradeRare, 13, Stats{PDef: 24, MDef: 14, MaxHP: 80}, 3800,
			"", "armor", "Ossos de guerreiros antigos, presos com tiras de couro."),
		gear("armor_epic", "Manto do Senhor dos Ossos", KindArmor, SlotArmor, nil, GradeEpic, 18, Stats{PDef: 34, MDef: 26, MaxHP: 130, MaxMP: 40}, 11000,
			"", "armor", "Ainda está frio ao toque."),
		gear("helm_common", "Capuz de Couro", KindHelmet, SlotHelmet, nil, GradeCommon, 1, Stats{PDef: 2, MDef: 3, MaxMP: 10}, 80,
			"", "helmet", "Protege do sol e de pancadas leves."),
		gear("helm_good", "Elmo de Ferro", KindHelmet, SlotHelmet, nil, GradeGood, 6, Stats{PDef: 6, MDef: 6, MaxMP: 25}, 800,
			"", "helmet", "Pesado, mas salva cabeças."),
		gear("helm_rare", "Elmo de Osso", KindHelmet, SlotHelmet, nil, GradeRare, 13, Stats{PDef: 11, MDef: 11, MaxMP: 45}, 3000,
			"", "helmet", "Um crânio enorme, adaptado para cabeças humanas."),

		// Poções.
		potion("potion_hp_s", "Poção de Vida Menor", CooldownHP, 80, 0, 20, "potion_hp", "Recupera 80 de vida na hora."),
		potion("potion_hp_l", "Poção de Vida Maior", CooldownHP, 260, 0, 75, "potion_hp_big", "Recupera 260 de vida na hora."),
		potion("potion_mp_s", "Poção de Mana Menor", CooldownMP, 0, 50, 25, "potion_mp", "Recupera 50 de mana na hora."),
		potion("potion_mp_l", "Poção de Mana Maior", CooldownMP, 0, 160, 90, "potion_mp_big", "Recupera 160 de mana na hora."),

		// Materiais (só para vender).
		stackable("bone_old", "Osso Velho", KindMaterial, 10, "bone", "Um osso qualquer. Alguém na cidade compra."),
		stackable("skull_cracked", "Crânio Rachado", KindMaterial, 35, "skull", "Os alquimistas pagam bem por isso."),
		stackable("shadow_essence", "Essência Sombria", KindMaterial, 100, "essence", "Fumaça escura presa num frasco."),
		stackable("broken_crown", "Coroa Partida", KindMaterial, 1500, "crown", "A coroa do Senhor dos Ossos. Vale uma fortuna."),

		// Moedas no chão.
		{ID: GoldID, Name: "Moedas de ouro", Kind: KindGold, Grade: GradeCommon, Slot: NoSlot, Stack: math.MaxInt32, Icon: "gold",
			Description: "Dinheiro corrente em Erevia."},
	}
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
}

// StarterKit: arma (e escudo) equipados + algumas poções.
func StarterKit(class string) []ItemRecord {
	weapons := map[string]string{"knight": "sword_common", "barbarian": "axe_common", "mage": "staff_common", "rogue": "daggers_common"}
	w, ok := weapons[class]
	if !ok {
		w = "sword_common"
	}
	kit := []ItemRecord{{Equipped: true, Slot: SlotWeapon, ItemID: w, Count: 1}}
	switch class {
	case "knight":
		kit = append(kit, ItemRecord{Equipped: true, Slot: SlotOffhand, ItemID: "shield_badge", Count: 1})
	case "barbarian":
		kit = append(kit, ItemRecord{Equipped: true, Slot: SlotOffhand, ItemID: "shield_buckler", Count: 1})
	}
	return append(kit, ItemRecord{Slot: 0, ItemID: "potion_hp_s", Count: 5})
}

// Lojas dos NPCs: ids de item à venda, na ordem da lista.
var (
	MerchantStock = []string{"potion_hp_s", "potion_hp_l", "potion_mp_s", "potion_mp_l"}
	SmithStock    = []string{
		"sword_common", "sword_good", "axe_common", "axe_good", "staff_common", "staff_good", "daggers_common", "daggers_good",
		"shield_badge", "shield_buckler", "shield_good", "armor_common", "armor_good", "helm_common", "helm_good",
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
		if d.Equippable() {
			m[d.Grade] = append(m[d.Grade], d)
		}
	}
	return m
}()

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

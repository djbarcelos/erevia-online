package zone

import (
	"erevia/server/internal/game"
)

// Lugares de um item (iguais aos do protocolo).
const (
	placeBag   = 0
	placeEquip = 1
)

type stack struct {
	def     *game.ItemDef
	count   int
	enchant int // +N (só itens que não empilham)
}

func (s stack) empty() bool { return s.def == nil || s.count <= 0 }

// inventory é a bolsa, o equipamento e as moedas de um jogador. Regras puras: quem
// chama decide o que avisar ao cliente.
type inventory struct {
	bag   [game.BagSize]stack
	equip [game.EquipSlots]stack
	gold  int64
}

// loadInventory monta o inventário a partir do que estava salvo. Itens que não existem
// mais no catálogo ou em posições inválidas são ignorados.
func loadInventory(gold int64, records []game.ItemRecord) (inventory, int) {
	inv := inventory{gold: max(0, gold)}
	skipped := 0
	for _, r := range records {
		def := game.Items[r.ItemID]
		if def == nil || def.Kind == game.KindGold || r.Count <= 0 {
			skipped++
			continue
		}
		n := min(r.Count, def.Stack)
		ench := 0
		if def.Stack == 1 {
			ench = min(max(r.Enchant, 0), game.MaxEnchant)
		}
		switch {
		case r.Equipped && r.Slot >= 0 && r.Slot < game.EquipSlots && def.Fits(r.Slot) && inv.equip[r.Slot].empty():
			inv.equip[r.Slot] = stack{def: def, count: 1, enchant: ench}
		case !r.Equipped && r.Slot >= 0 && r.Slot < game.BagSize && inv.bag[r.Slot].empty():
			inv.bag[r.Slot] = stack{def: def, count: n, enchant: ench}
		default:
			// Posição repetida ou inválida: tenta guardar em outro espaço em vez de perder.
			if inv.put(def, n, ench) > 0 {
				skipped++
			}
		}
	}
	return inv, skipped
}

// records é o inverso de loadInventory (para salvar).
func (inv *inventory) records() []game.ItemRecord {
	var out []game.ItemRecord
	for i, s := range inv.equip {
		if !s.empty() {
			out = append(out, game.ItemRecord{Equipped: true, Slot: i, ItemID: s.def.ID, Count: s.count, Enchant: s.enchant})
		}
	}
	for i, s := range inv.bag {
		if !s.empty() {
			out = append(out, game.ItemRecord{Slot: i, ItemID: s.def.ID, Count: s.count, Enchant: s.enchant})
		}
	}
	return out
}

// room diz quantas unidades do item ainda cabem na bolsa.
func (inv *inventory) room(def *game.ItemDef) int {
	n := 0
	for _, s := range inv.bag {
		switch {
		case s.empty():
			n += def.Stack
		case s.def == def:
			n += def.Stack - s.count
		}
	}
	return n
}

func (inv *inventory) canAdd(def *game.ItemDef, count int) bool { return inv.room(def) >= count }

// add guarda o item, primeiro completando pilhas e depois em espaços vazios.
// Devolve quantas unidades não couberam.
func (inv *inventory) add(def *game.ItemDef, count int) int {
	if def.Stack > 1 {
		for i := range inv.bag {
			s := &inv.bag[i]
			if !s.empty() && s.def == def && s.count < def.Stack {
				put := min(count, def.Stack-s.count)
				s.count += put
				count -= put
				if count == 0 {
					return 0
				}
			}
		}
	}
	for i := range inv.bag {
		s := &inv.bag[i]
		if s.empty() {
			put := min(count, def.Stack)
			*s = stack{def: def, count: put}
			count -= put
			if count == 0 {
				return 0
			}
		}
	}
	return count
}

// takeID tira count unidades do item id da bolsa (das pilhas do fim para o começo).
// Devolve false (sem mexer em nada) se não houver o bastante.
func (inv *inventory) takeID(id string, count int) bool {
	if inv.countOf(id) < count {
		return false
	}
	for i := len(inv.bag) - 1; i >= 0 && count > 0; i-- {
		s := &inv.bag[i]
		if s.empty() || s.def.ID != id {
			continue
		}
		n := min(count, s.count)
		s.count -= n
		count -= n
		if s.count == 0 {
			*s = stack{}
		}
	}
	return true
}

// put guarda o item mantendo o encantamento (encantado = sempre um espaço vazio só dele).
// Devolve quantas unidades não couberam.
func (inv *inventory) put(def *game.ItemDef, count, enchant int) int {
	if enchant <= 0 || def.Stack > 1 {
		return inv.add(def, count)
	}
	free := inv.firstFree()
	if free < 0 {
		return count
	}
	inv.bag[free] = stack{def: def, count: 1, enchant: enchant}
	return count - 1
}

// take tira unidades de um espaço da bolsa.
func (inv *inventory) take(slot, count int) bool {
	if slot < 0 || slot >= game.BagSize || count <= 0 {
		return false
	}
	s := &inv.bag[slot]
	if s.empty() || s.count < count {
		return false
	}
	s.count -= count
	if s.count == 0 {
		*s = stack{}
	}
	return true
}

// countOf soma as unidades de um item na bolsa.
func (inv *inventory) countOf(id string) int {
	n := 0
	for _, s := range inv.bag {
		if !s.empty() && s.def.ID == id {
			n += s.count
		}
	}
	return n
}

// slotOf: primeiro espaço da bolsa com o item (-1 = não tem).
func (inv *inventory) slotOf(id string) int {
	for i, s := range inv.bag {
		if !s.empty() && s.def.ID == id {
			return i
		}
	}
	return -1
}

func (inv *inventory) firstFree() int {
	for i, s := range inv.bag {
		if s.empty() {
			return i
		}
	}
	return -1
}

// moveInBag troca dois espaços da bolsa ou junta pilhas do mesmo item.
func (inv *inventory) moveInBag(from, to int) bool {
	if from == to || from < 0 || to < 0 || from >= game.BagSize || to >= game.BagSize || inv.bag[from].empty() {
		return false
	}
	a, b := &inv.bag[from], &inv.bag[to]
	if !b.empty() && a.def == b.def && a.def.Stack > 1 {
		put := min(a.count, a.def.Stack-b.count)
		if put > 0 {
			b.count += put
			a.count -= put
			if a.count == 0 {
				*a = stack{}
			}
			return true
		}
	}
	*a, *b = *b, *a
	return true
}

// canEquip explica por que o item não pode ir no encaixe ("" = pode).
func canEquip(def *game.ItemDef, slot int, class string, level int) string {
	switch {
	case def == nil || !def.Equippable():
		return "Este item não pode ser equipado."
	case slot < 0 || slot >= game.EquipSlots || !def.Fits(slot):
		return "Este item não vai nesse lugar."
	case !def.UsableBy(class):
		return "Sua classe não pode usar este item."
	case level < def.ReqLevel:
		return "Você precisa de mais nível para usar este item."
	}
	return ""
}

// bestSlot: onde equipar o item sem escolher encaixe (duplo clique). Anéis e brincos vão
// no primeiro encaixe livre; com os dois ocupados, trocam com o primeiro.
func (inv *inventory) bestSlot(def *game.ItemDef) int {
	slots := def.Slots()
	if len(slots) == 0 {
		return game.NoSlot
	}
	for _, s := range slots {
		if inv.equip[s].empty() {
			return s
		}
	}
	return slots[0]
}

// equipFromBag põe o item do espaço da bolsa no melhor encaixe; o que estava lá volta
// para o mesmo espaço da bolsa.
func (inv *inventory) equipFromBag(bagSlot int, class string, level int) string {
	if bagSlot < 0 || bagSlot >= game.BagSize || inv.bag[bagSlot].empty() {
		return "?"
	}
	return inv.equipTo(bagSlot, inv.bestSlot(inv.bag[bagSlot].def), class, level)
}

// equipTo põe o item do espaço da bolsa num encaixe escolhido (trocando com o que houver).
func (inv *inventory) equipTo(bagSlot, slot int, class string, level int) string {
	if bagSlot < 0 || bagSlot >= game.BagSize || inv.bag[bagSlot].empty() {
		return "?"
	}
	if why := canEquip(inv.bag[bagSlot].def, slot, class, level); why != "" {
		return why
	}
	inv.bag[bagSlot], inv.equip[slot] = inv.equip[slot], inv.bag[bagSlot]
	return ""
}

// moveInEquip troca dois encaixes (anel 1 ↔ anel 2, brinco 1 ↔ brinco 2).
func (inv *inventory) moveInEquip(from, to int) string {
	if from == to || from < 0 || to < 0 || from >= game.EquipSlots || to >= game.EquipSlots || inv.equip[from].empty() {
		return "?"
	}
	a, b := inv.equip[from], inv.equip[to]
	if !a.def.Fits(to) || (!b.empty() && !b.def.Fits(from)) {
		return "Este item não vai nesse lugar."
	}
	inv.equip[from], inv.equip[to] = b, a
	return ""
}

// unequip devolve o item do encaixe para a bolsa (no espaço pedido, ou no primeiro livre).
// Se o espaço pedido tiver um item que serve no encaixe, os dois trocam de lugar.
func (inv *inventory) unequip(slot, bagSlot int, class string, level int) string {
	if slot < 0 || slot >= game.EquipSlots || inv.equip[slot].empty() {
		return "?"
	}
	if bagSlot >= 0 && bagSlot < game.BagSize {
		if inv.bag[bagSlot].empty() {
			inv.bag[bagSlot], inv.equip[slot] = inv.equip[slot], stack{}
			return ""
		}
		if canEquip(inv.bag[bagSlot].def, slot, class, level) == "" {
			inv.bag[bagSlot], inv.equip[slot] = inv.equip[slot], inv.bag[bagSlot]
			return ""
		}
	}
	free := inv.firstFree()
	if free < 0 {
		return "Sua bolsa está cheia."
	}
	inv.bag[free], inv.equip[slot] = inv.equip[slot], stack{}
	return ""
}

// bonus soma os atributos de todo o equipamento.
func (inv *inventory) bonus() game.Stats {
	var b game.Stats
	for _, s := range inv.equip {
		if !s.empty() {
			b = b.AddBonus(s.def.Bonus)
			if s.enchant > 0 {
				b = b.AddBonus(game.EnchantBonus(s.def, s.enchant))
			}
		}
	}
	return b
}

// weaponType: tipo da arma equipada ("" = sem arma). Esgrima vale com espadas.
func (inv *inventory) weaponType() string {
	if w := inv.equip[game.SlotWeapon]; !w.empty() {
		return w.def.WeaponType
	}
	return ""
}

// weaponEnchant: +N da arma equipada (o brilho que os outros veem).
func (inv *inventory) weaponEnchant() int {
	if w := inv.equip[game.SlotWeapon]; !w.empty() {
		return w.enchant
	}
	return 0
}

// look é o que os outros veem: modelos das armas e a raridade da principal.
func (inv *inventory) look() (weapon, offhand string, grade game.Grade) {
	if w := inv.equip[game.SlotWeapon]; !w.empty() {
		weapon, offhand, grade = w.def.Model, w.def.OffModel, w.def.Grade
	}
	if o := inv.equip[game.SlotOffhand]; !o.empty() {
		offhand = o.def.Model
	}
	return weapon, offhand, grade
}

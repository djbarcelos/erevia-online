package zone

import (
	"fmt"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// enchant usa um pergaminho (na bolsa) num item da bolsa ou equipado. Até +3 é seguro;
// acima, falhar quebra o item (o abençoado só volta a +0). O pergaminho é gasto sempre.
func (z *Zone) enchant(p *entity, scrollSlot, place, slot int) {
	if p.dead {
		return
	}
	inv := &p.pl.inv
	if scrollSlot < 0 || scrollSlot >= game.BagSize || inv.bag[scrollSlot].empty() {
		return
	}
	sc := inv.bag[scrollSlot].def
	if sc.Kind != game.KindScroll || (sc.Scroll != game.ScrollEnchantWeapon && sc.Scroll != game.ScrollEnchantArmor) {
		return
	}
	var target *stack
	switch {
	case place == placeBag && slot >= 0 && slot < game.BagSize:
		target = &inv.bag[slot]
	case place == placeEquip && slot >= 0 && slot < game.EquipSlots:
		target = &inv.equip[slot]
	default:
		return
	}
	switch {
	case target.empty():
		return
	case !target.def.Enchantable(sc.Scroll):
		if sc.Scroll == game.ScrollEnchantWeapon {
			z.system(p, "Este pergaminho só encanta armas.")
		} else {
			z.system(p, "Este pergaminho encanta armaduras, escudos, roupas e joias (não armas).")
		}
		return
	case target.enchant >= game.MaxEnchant:
		z.system(p, fmt.Sprintf("Este item já está no máximo (+%d).", game.MaxEnchant))
		return
	}

	def, before := target.def, target.enchant
	inv.take(scrollSlot, 1)
	equipped := place == placeEquip
	res := &pb.EnchantResult{ItemName: def.Name}
	if z.rng.Float64() < game.EnchantChance(before+1) {
		target.enchant = before + 1
		res.Ok, res.Level = true, uint32(target.enchant)
		z.system(p, fmt.Sprintf("Sucesso! %s.", game.EnchantedName(def, target.enchant)))
		z.achValue(p, game.AchEnchant, "", target.enchant)
	} else if sc.Blessed {
		target.enchant = 0
		z.system(p, fmt.Sprintf("O encantamento falhou. A bênção salvou o item, mas %s voltou a +0.", def.Name))
	} else {
		*target = stack{}
		res.Broke = true
		z.trackItem(p, false, def.ID, 1, "enchant_break")
		z.system(p, fmt.Sprintf("O encantamento falhou e %s quebrou.", game.EnchantedName(def, before)))
	}
	z.log.Info("enchant", "id", p.id, "item", def.ID, "from", before, "ok", res.Ok, "broke", res.Broke)
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_EnchantResult{EnchantResult: res}})
	z.inventoryChanged(p, equipped)
}

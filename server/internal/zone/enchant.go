package zone

import (
	"fmt"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// enchant usa um pergaminho (na bolsa) num item da bolsa ou equipado. Até +3 é seguro;
// acima, falhar quebra o item (o abençoado só volta a +0). O pergaminho e a taxa em moedas
// (game.EnchantFee, pelo grau e pelo nível de destino) são gastos sempre. O pergaminho
// precisa ser do grau da peça (o do D também serve nas sem grau).
func (z *Zone) enchant(p *entity, scrollSlot, place, slot int) {
	if p.dead {
		return
	}
	inv := &p.pl.inv
	if scrollSlot < 0 || scrollSlot >= game.BagSize || inv.bag[scrollSlot].empty() {
		return
	}
	sc := inv.bag[scrollSlot].def
	if !sc.IsEnchantScroll() {
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
	case !game.ScrollFits(sc.ScrollTier, target.def.Tier):
		z.system(p, fmt.Sprintf("Este pergaminho é do grau %s, e %s %s.", sc.ScrollTier.Letter(), target.def.Name, tierPhrase(target.def.Tier)))
		return
	case target.enchant >= game.MaxEnchant:
		z.system(p, fmt.Sprintf("Este item já está no máximo (+%d).", game.MaxEnchant))
		return
	}

	def, before := target.def, target.enchant
	fee := game.EnchantFee(def.Tier, before+1)
	if inv.gold < fee {
		z.system(p, fmt.Sprintf("Encantar a +%d custa %d moedas.", before+1, fee))
		return
	}
	inv.take(scrollSlot, 1)
	z.trackItem(p, false, sc.ID, 1, "enchant")
	if fee > 0 {
		inv.gold -= fee
		z.trackGold(p, "enchant", -fee)
	}
	equipped := place == placeEquip
	res := &pb.EnchantResult{ItemName: def.Name}
	if z.rng.Float64() < game.EnchantChanceFor(def.Tier, before+1) {
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

// tierPhrase: "é do grau C" ou "não tem grau".
func tierPhrase(t game.Tier) string {
	if t == game.TierNone {
		return "não tem grau"
	}
	return "é do grau " + t.Letter()
}

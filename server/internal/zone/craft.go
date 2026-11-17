package zone

import (
	"fmt"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Criação de itens no Ferreiro. Confere materiais, moedas e espaço numa cópia da bolsa
// e só então aplica: nunca some material sem o item aparecer.

func (z *Zone) craft(p *entity, npcID uint32, recipeID string) {
	n := z.shopNPC(p, npcID)
	if n == nil || !n.npc.crafts {
		return
	}
	r := game.Recipes[recipeID]
	if r == nil {
		return
	}
	res := game.Items[r.Result]
	inv := p.pl.inv // cópia (arrays por valor)
	if inv.gold < r.Gold {
		z.system(p, "Moedas insuficientes.")
		return
	}
	for _, m := range r.Materials {
		if have := inv.countOf(m.ID); have < m.Count {
			z.system(p, fmt.Sprintf("Faltam %d× %s.", m.Count-have, game.Items[m.ID].Name))
			return
		}
	}
	for _, m := range r.Materials {
		inv.takeID(m.ID, m.Count)
	}
	inv.gold -= r.Gold
	if inv.add(res, r.Count) > 0 {
		z.system(p, "Sua bolsa está cheia.")
		return
	}
	p.pl.inv = inv
	z.trackGold(p, "craft", -r.Gold)
	for _, m := range r.Materials {
		z.trackItem(p, false, m.ID, m.Count, "craft")
	}
	z.trackItem(p, true, res.ID, r.Count, "craft")
	z.system(p, fmt.Sprintf("%s criou %s para você por %d moedas.", n.name, itemLabel(res, r.Count), r.Gold))
	z.inventoryChanged(p, false) // também agenda o salvamento
	z.achValue(p, game.AchCraft, "", int(res.Grade))
}

func recipesPB() []*pb.RecipeInfo {
	out := make([]*pb.RecipeInfo, 0, len(game.RecipeList))
	for _, r := range game.RecipeList {
		info := &pb.RecipeInfo{Id: r.ID, ResultId: r.Result, Count: uint32(r.Count), Gold: r.Gold}
		for _, m := range r.Materials {
			info.Materials = append(info.Materials, &pb.RecipeMaterial{ItemId: m.ID, Count: uint32(m.Count)})
		}
		out = append(out, info)
	}
	return out
}

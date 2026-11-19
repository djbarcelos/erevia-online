package zone

import (
	"fmt"

	"erevia/server/internal/game"
)

// crystallize transforma uma peça da bolsa (grau D ou maior) nos cristais do grau dela
// (game.CrystalYield, que sobe com o encantamento). Confere o espaço numa cópia da bolsa
// antes de aplicar: a peça só some se os cristais couberem (tunnel#154).
func (z *Zone) crystallize(p *entity, slot int) {
	if p.dead || slot < 0 || slot >= game.BagSize {
		return
	}
	s := p.pl.inv.bag[slot]
	if s.empty() {
		return
	}
	crystal, n := game.CrystalYield(s.def, s.enchant)
	if n <= 0 {
		z.system(p, fmt.Sprintf("%s não se cristaliza: só equipamento de grau D ou maior.", s.def.Name))
		return
	}
	inv := p.pl.inv // cópia (arrays por valor)
	inv.take(slot, 1)
	if inv.add(game.Items[crystal], n) > 0 {
		z.system(p, "Sua bolsa está cheia.")
		return
	}
	p.pl.inv = inv
	z.trackItem(p, false, s.def.ID, 1, "crystal")
	z.trackItem(p, true, crystal, n, "crystal")
	z.system(p, fmt.Sprintf("%s virou %s.", game.EnchantedName(s.def, s.enchant), itemLabel(game.Items[crystal], n)))
	z.inventoryChanged(p, false)
}

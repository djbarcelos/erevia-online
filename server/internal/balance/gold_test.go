package balance

import "testing"

// O ouro que sai em cada grau fica entre 60% e 100% do que entra (tunnel#154): o modelo de
// GoldFlow (caçar sozinho, vender o saque, refazer e encantar o equipamento típico). Se uma
// mudança de saque, preço, taxa ou chance tirar uma faixa daqui, reveja a taxa de encantar
// (game.EnchantFeePerLevel) ou o que mudou.
func TestGoldSinkPerTier(t *testing.T) {
	for _, g := range GoldFlow() {
		if g.In() <= 0 || g.Hours <= 0 {
			t.Fatalf("grau %s: sem ouro entrando (%+v)", g.Tier.Letter(), g)
		}
		if r := g.Ratio(); r < GoldSinkMin || r > GoldSinkMax {
			t.Errorf("grau %s (%d–%d): sai %.0f%% do que entra (meta %.0f%%–%.0f%%): entra %.0f, sai %.0f (poções %.0f, peças %.0f, encantar %.0f)",
				g.Tier.Letter(), g.Lo, g.Hi, r*100, GoldSinkMin*100, GoldSinkMax*100, g.In(), g.Out(), g.OutSupplies, g.OutGear, g.OutEnchant)
		}
	}
}

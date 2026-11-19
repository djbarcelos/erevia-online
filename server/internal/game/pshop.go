package game

import "math"

// Loja pessoal no estilo do Lineage 2 (tunnel#161): o personagem "senta" com uma placa e vende
// itens da bolsa, com preço, enquanto está online. Só na área do mercado da cidade (entre a
// praça e o prédio do Mercado, a sudeste do poço), com no máximo PrivateShopMax vendedores e
// PrivateShopSpacing metros entre eles.
const (
	PrivateShopMax      = 30            // vendedores ao mesmo tempo na área
	PrivateShopSpacing  = 1.8           // metros entre dois vendedores
	PrivateShopItems    = 8             // itens diferentes por loja
	PrivateShopTitleLen = 24            // letras da placa
	PrivateShopMaxPrice = 1_000_000_000 // moedas por unidade
	PrivateShopRange    = 8.0           // metros para ver a loja e comprar
)

// A área do mercado: um círculo entre a praça e o prédio do Mercado (world.json, "market" em
// 17,7; −17,7). Os prédios em volta já bloqueiam o passo (colisores), então quem está dentro
// do círculo está num lugar onde se anda. Com 9 m de raio, tirando as casas e o Mercado,
// cabem ~34 vendedores a 1,9 m uns dos outros (o limite é 30); com 7 m, só 21.
var (
	PrivateShopCenterX, PrivateShopCenterZ = 11.5, -11.5
	PrivateShopRadius                      = 9.0
)

// InPrivateShopArea: o ponto está na área do mercado.
func InPrivateShopArea(x, z float64) bool {
	return math.Hypot(x-PrivateShopCenterX, z-PrivateShopCenterZ) <= PrivateShopRadius
}

// PrivateShopSellable: o item pode ir para a loja (não as moedas nem o que não tem preço).
func PrivateShopSellable(d *ItemDef) bool {
	return d != nil && d.Kind != KindGold
}

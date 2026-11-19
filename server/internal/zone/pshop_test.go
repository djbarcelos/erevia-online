package zone

import (
	"cmp"
	"math"
	"slices"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Loja pessoal (tunnel#161).

func pshopCmd(z *Zone, p *entity, m *pb.PrivateShopCmd) {
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(socialEvent{id: p.id, msg: m})
}

func (f *fakePeer) lastShop() *pb.PrivateShopView {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if v := f.msgs[i].GetPrivateShop(); v != nil {
			return v
		}
	}
	return nil
}

// marketSpots: pontos da área do mercado numa grade hexagonal, 1,9 m uns dos outros (mais
// que o espaçamento mínimo), do centro para fora.
var marketSpots = func() [][2]float64 {
	const s = 1.9
	var out [][2]float64
	for j := -6; j <= 6; j++ {
		for i := -6; i <= 6; i++ {
			x := game.PrivateShopCenterX + float64(i)*s + float64(j&1)*s/2
			z := game.PrivateShopCenterZ + float64(j)*s*math.Sqrt(3)/2
			if game.InPrivateShopArea(x, z) {
				out = append(out, [2]float64{x, z})
			}
		}
	}
	slices.SortStableFunc(out, func(a, b [2]float64) int {
		da := math.Hypot(a[0]-game.PrivateShopCenterX, a[1]-game.PrivateShopCenterZ)
		db := math.Hypot(b[0]-game.PrivateShopCenterX, b[1]-game.PrivateShopCenterZ)
		return cmp.Compare(da, db)
	})
	return out
}()

func marketSpot(i int) (float64, float64) {
	if i >= len(marketSpots) {
		return 999, 999 // fora do mercado
	}
	return marketSpots[i][0], marketSpots[i][1]
}

// openTestShop: Ana vende 5 Poções de Vida Menores (espaço 3) a 20 cada e uma espada +4
// (espaço 4) a 900; Beto está ao lado, com 2.000 moedas.
func openTestShop(t *testing.T, saves chan []SavedState) (*Zone, *entity, *entity, *fakePeer, *fakePeer) {
	t.Helper()
	z := newCombatZone()
	if saves != nil {
		z.saves = saves
	}
	x, zz := marketSpot(0)
	a, pa := joinAt(z, 1, "knight", x, zz, 0)
	b, pb_ := joinAt(z, 2, "mage", x+1.5, zz, 0)
	a.pl.inv.bag = [game.BagSize]stack{}
	b.pl.inv.bag = [game.BagSize]stack{}
	giveItem(a, 3, "potion_hp_s", 5, 0)
	giveItem(a, 4, "sword_good", 1, 4)
	a.pl.inv.gold, b.pl.inv.gold = 100, 2000
	pshopCmd(z, a, &pb.PrivateShopCmd{Kind: pb.PrivateShopCmdKind_PRIVATE_SHOP_CMD_OPEN, Title: "  Poções baratas  ",
		Items: []*pb.PrivateShopItem{{Slot: 3, Count: 5, Price: 20}, {Slot: 4, Price: 900}}})
	if a.pl.pshop == nil || !pa.lastShop().Open || !pa.lastShop().Mine {
		t.Fatalf("a loja deveria abrir: %+v", pa.lastShop())
	}
	return z, a, b, pa, pb_
}

func buy(z *Zone, p *entity, seller uint32, slot, count int, price uint64) {
	pshopCmd(z, p, &pb.PrivateShopCmd{Kind: pb.PrivateShopCmdKind_PRIVATE_SHOP_CMD_BUY, SellerId: seller, Slot: uint32(slot), Count: uint32(count), Price: price})
}

func TestPrivateShopSellsAndSavesBothTogether(t *testing.T) {
	saves := make(chan []SavedState, 16)
	z, a, b, pa, pb_ := openTestShop(t, saves)
	if a.pl.pshop.title != "Poções baratas" || z.statusPB(a).ShopTitle != "Poções baratas" || z.entityPB(a).ShopTitle != "Poções baratas" {
		t.Fatalf("a placa aparece para todos: %q", a.pl.pshop.title)
	}
	pshopCmd(z, b, &pb.PrivateShopCmd{Kind: pb.PrivateShopCmdKind_PRIVATE_SHOP_CMD_VIEW, SellerId: a.id})
	if v := pb_.lastShop(); v == nil || !v.Open || v.Mine || len(v.Items) != 2 || v.Items[1].Enchant != 4 {
		t.Fatalf("Beto vê a loja: %+v", v)
	}
	for len(saves) > 0 {
		<-saves
	}

	buy(z, b, a.id, 3, 3, 20)
	if a.pl.inv.countOf("potion_hp_s") != 2 || b.pl.inv.countOf("potion_hp_s") != 3 || a.pl.inv.gold != 160 || b.pl.inv.gold != 1940 {
		t.Fatalf("compra: Ana %d poções e %d moedas, Beto %d e %d", a.pl.inv.countOf("potion_hp_s"), a.pl.inv.gold, b.pl.inv.countOf("potion_hp_s"), b.pl.inv.gold)
	}
	if v := pb_.lastShop(); v == nil || v.Items[0].Count != 2 {
		t.Fatalf("quem olha vê a quantidade nova: %+v", v)
	}
	select {
	case batch := <-saves:
		ids := map[uint32]bool{}
		for _, s := range batch {
			ids[s.ID] = true
		}
		if !ids[a.id] || !ids[b.id] {
			t.Fatalf("vendedor e comprador na mesma leva: %v", ids)
		}
	default:
		t.Fatal("a compra é salva na hora")
	}

	// A espada leva o encantamento; vendido tudo, a loja fecha.
	buy(z, b, a.id, 4, 1, 900)
	buy(z, b, a.id, 3, 2, 20)
	if got := b.pl.inv.bag[b.pl.inv.slotOf("sword_good")]; got.enchant != 4 {
		t.Fatalf("o encantamento viaja com o item: +%d", got.enchant)
	}
	if a.pl.pshop != nil || pa.lastShop().Open || pb_.lastShop().Open || z.statusPB(a).ShopTitle != "" {
		t.Fatal("vendeu tudo: a loja fecha e a placa sai")
	}
	if a.pl.inv.gold != 100+20*5+900 || b.pl.inv.gold != 2000-20*5-900 {
		t.Fatalf("moedas: Ana %d, Beto %d", a.pl.inv.gold, b.pl.inv.gold)
	}
}

func TestPrivateShopRefusesWithoutChangingAnything(t *testing.T) {
	z, a, b, _, pb_ := openTestShop(t, nil)
	before := func() (int, int64, int, int64) {
		return a.pl.inv.countOf("potion_hp_s"), a.pl.inv.gold, b.pl.inv.countOf("potion_hp_s"), b.pl.inv.gold
	}
	a0, ag0, b0, bg0 := before()
	same := func(why string) {
		t.Helper()
		a1, ag1, b1, bg1 := before()
		if a1 != a0 || ag1 != ag0 || b1 != b0 || bg1 != bg0 {
			t.Fatalf("%s mudou algo: Ana %d/%d, Beto %d/%d", why, a1, ag1, b1, bg1)
		}
	}
	buy(z, b, a.id, 3, 1, 19) // preço que mudou
	same("preço diferente")
	if !pb_.systemContains("O preço mudou") {
		t.Fatal("avisa que o preço mudou")
	}
	buy(z, b, a.id, 3, 6, 20) // mais que à venda
	same("quantidade demais")
	buy(z, b, a.id, 9, 1, 20) // espaço que não está na loja
	same("espaço fora da loja")
	b.pl.inv.gold = 30
	bg0 = 30
	buy(z, b, a.id, 3, 2, 20) // sem moedas
	same("sem moedas")
	b.pl.inv.gold, bg0 = 2000, 2000
	for i := range b.pl.inv.bag { // bolsa cheia de outras coisas
		b.pl.inv.bag[i] = stack{def: item("sword_common"), count: 1}
	}
	b0 = 0
	buy(z, b, a.id, 3, 1, 20)
	same("bolsa cheia")
	if !pb_.systemContains("bolsa está cheia") {
		t.Fatal("avisa a bolsa cheia")
	}
	b.pl.inv.bag = [game.BagSize]stack{}

	// Ana usou as poções (a bolsa mudou): a linha some e a compra recusa.
	a.pl.inv.bag[3] = stack{}
	z.inventoryChanged(a, false)
	a0 = 0
	buy(z, b, a.id, 3, 1, 20)
	same("item que saiu da bolsa")
	if len(a.pl.pshop.items) != 1 || a.pl.pshop.items[0].slot != 4 {
		t.Fatalf("a loja só tem a espada agora: %+v", a.pl.pshop.items)
	}

	// Longe demais.
	b.pos = Vec2{X: a.pos.X + game.PrivateShopRange + 1, Z: a.pos.Z}
	buy(z, b, a.id, 4, 1, 900)
	if b.pl.inv.countOf("sword_good") != 0 {
		t.Fatal("de longe não compra")
	}
}

func TestPrivateShopOnlyInTheMarketWithSpacingAndLimit(t *testing.T) {
	z := newCombatZone()
	open := func(p *entity) {
		giveItem(p, 0, "potion_hp_s", 1, 0)
		pshopCmd(z, p, &pb.PrivateShopCmd{Kind: pb.PrivateShopCmdKind_PRIVATE_SHOP_CMD_OPEN, Items: []*pb.PrivateShopItem{{Slot: 0, Price: 5}}})
	}
	far, pf := joinAt(z, 1, "knight", 0, 30, 0)
	open(far)
	if far.pl.pshop != nil || !pf.systemContains("área do mercado") {
		t.Fatal("fora do mercado não abre")
	}

	x, zz := marketSpot(0)
	a, _ := joinAt(z, 2, "knight", x, zz, 0)
	open(a)
	if a.pl.pshop == nil || a.pl.pshop.title != "Loja de "+a.name {
		t.Fatalf("placa vazia vira «Loja de Nome»: %+v", a.pl.pshop)
	}
	b, pb_ := joinAt(z, 3, "knight", x+game.PrivateShopSpacing/2, zz, 0)
	open(b)
	if b.pl.pshop != nil || !pb_.systemContains("Muito perto") {
		t.Fatal("colado em outro vendedor não abre")
	}

	// Enche o mercado até o limite: o seguinte recusa com aviso.
	opened := 1
	for i := 1; opened < game.PrivateShopMax; i++ {
		px, pz := marketSpot(i)
		if !game.InPrivateShopArea(px, pz) {
			t.Fatalf("o mercado comporta %d vendedores (parou em %d)", game.PrivateShopMax, opened)
		}
		p, _ := joinAt(z, uint32(10+i), "knight", px, pz, 0)
		open(p)
		if p.pl.pshop != nil {
			opened++
		}
	}
	px, pz := marketSpot(opened + 5) // um lugar livre e longe dos outros
	b.pos = Vec2{X: px, Z: pz}
	open(b)
	if b.pl.pshop != nil || !pb_.systemContains("O mercado está cheio") {
		t.Fatalf("com %d lojas, a próxima recusa", game.PrivateShopMax)
	}
}

func TestPrivateShopClosesWhenTheSellerMovesDiesOrLeaves(t *testing.T) {
	z, a, b, pa, pb_ := openTestShop(t, nil)
	pshopCmd(z, b, &pb.PrivateShopCmd{Kind: pb.PrivateShopCmdKind_PRIVATE_SHOP_CMD_VIEW, SellerId: a.id})

	// Andou: fecha no próximo segundo, e quem olhava fica sabendo.
	a.pos.X += 2
	z.checkPrivateShops()
	if a.pl.pshop != nil || !pa.systemContains("saiu do lugar") || pb_.lastShop().Open {
		t.Fatal("andar fecha a loja")
	}

	// Trocar com a loja aberta não pode.
	a.pos.X -= 2
	pshopCmd(z, a, &pb.PrivateShopCmd{Kind: pb.PrivateShopCmdKind_PRIVATE_SHOP_CMD_OPEN, Items: []*pb.PrivateShopItem{{Slot: 3, Price: 20}}})
	if z.tradeBlocked(a) == "" {
		t.Fatal("com a loja aberta, não troca")
	}

	// Saiu do jogo: a loja some com ele (só fica aberta com o dono online).
	z.removePlayer(a.id)
	if a.pl.pshop != nil {
		t.Fatal("sair fecha a loja")
	}
	buy(z, b, a.id, 3, 1, 20)
	if b.pl.inv.countOf("potion_hp_s") != 0 || !pb_.systemContains("Essa loja fechou") {
		t.Fatal("loja de quem saiu não vende")
	}
}

// Queda no meio: o comprador manda a compra e cai no mesmo tick. A compra é aplicada inteira
// antes da saída e as duas bolsas vão na mesma leva; o estado que vai para o banco na saída
// já tem o item. Nada duplica nem some.
func TestPrivateShopBuyerDropsRightAfterBuying(t *testing.T) {
	saves := make(chan []SavedState, 16)
	z, a, b, _, _ := openTestShop(t, saves)
	for len(saves) > 0 {
		<-saves
	}
	buy(z, b, a.id, 3, 2, 20)
	z.removePlayer(b.id)
	var buyer, seller *SavedState
	for len(saves) > 0 {
		for _, s := range <-saves {
			s := s
			switch s.ID {
			case a.id:
				seller = &s
			case b.id:
				buyer = &s
			}
		}
	}
	if buyer == nil || seller == nil {
		t.Fatal("os dois foram salvos")
	}
	count := func(st *SavedState, id string) int {
		n := 0
		for _, it := range st.Items {
			if it.ItemID == id {
				n += it.Count
			}
		}
		return n
	}
	if count(buyer, "potion_hp_s")+count(seller, "potion_hp_s") != 5 || buyer.Gold+seller.Gold != 2100 {
		t.Fatalf("depois da queda: comprador %d poções/%d moedas, vendedor %d/%d", count(buyer, "potion_hp_s"), buyer.Gold, count(seller, "potion_hp_s"), seller.Gold)
	}
	if count(buyer, "potion_hp_s") != 2 {
		t.Fatalf("o comprador ficou com as 2 poções: %d", count(buyer, "potion_hp_s"))
	}
}

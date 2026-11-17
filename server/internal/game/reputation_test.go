package game

import "testing"

func TestRepGrades(t *testing.T) {
	if len(RepGradeNames) != 5 || len(RepGradePoints) != 5 || RepGradeNames[0] != "Neutro" || RepGradeNames[4] != "Exaltado" {
		t.Fatalf("graus: %v %v", RepGradeNames, RepGradePoints)
	}
	cases := map[int]RepGrade{
		0: RepNeutral, 499: RepNeutral, 500: RepFriendly, 1499: RepFriendly, 1500: RepHonored,
		3499: RepHonored, 3500: RepRevered, 6999: RepRevered, 7000: RepExalted, 999999: RepExalted, -5: RepNeutral,
	}
	for pts, want := range cases {
		if got := GradeOf(pts); got != want {
			t.Errorf("GradeOf(%d) = %d, esperava %d", pts, got, want)
		}
	}
	for i := 1; i < len(RepGradePoints); i++ {
		if RepGradePoints[i] <= RepGradePoints[i-1] {
			t.Fatal("os graus têm que crescer")
		}
	}
}

func TestRepPoints(t *testing.T) {
	var r ReputationState
	if g, b, a := r.Add(FactionGuard, 480); g != 480 || b != RepNeutral || a != RepNeutral {
		t.Fatalf("480: %d %d %d", g, b, a)
	}
	if g, b, a := r.Add(FactionGuard, 120); g != 120 || b != RepNeutral || a != RepFriendly || r.Of(FactionGuard) != 600 {
		t.Fatalf("subiu para amigável: %d %d %d (%d)", g, b, a, r.Of(FactionGuard))
	}
	// Teto no exaltado: o que passa não entra.
	r.Add(FactionGuard, 6300)
	if g, _, a := r.Add(FactionGuard, 500); g != 100 || a != RepExalted || r.Of(FactionGuard) != RepMax {
		t.Fatalf("teto: ganhou %d, grau %d, pontos %d", g, a, r.Of(FactionGuard))
	}
	if g, _, _ := r.Add(FactionGuard, 50); g != 0 {
		t.Fatal("passou do teto")
	}
	// Facção que não existe e pontos negativos não mexem em nada.
	if g, _, _ := r.Add("piratas", 100); g != 0 || r.Of("piratas") != 0 {
		t.Fatal("facção inexistente ganhou pontos")
	}
	if g, _, _ := r.Add(FactionBastion, -40); g != 0 || r.Of(FactionBastion) != 0 {
		t.Fatal("pontos negativos")
	}
	n := NormalReputation(ReputationState{Points: map[string]int{FactionBastion: 99999, FactionGuard: -3, "x": 10}})
	if len(n.Points) != 1 || n.Points[FactionBastion] != RepMax {
		t.Fatalf("NormalReputation: %+v", n)
	}
}

func TestFactions(t *testing.T) {
	if len(Factions) != 2 || FactionByID(FactionBastion) == nil || FactionByID(FactionGuard) == nil || FactionByID("x") != nil {
		t.Fatal("as duas facções")
	}
	// Cada facção tem contratos e ofertas.
	for _, f := range Factions {
		contracts, offers := 0, 0
		for _, c := range ContractList {
			if c.Faction == f.ID {
				contracts++
			}
		}
		for _, o := range RepOffers {
			if o.Faction == f.ID {
				offers++
			}
		}
		if contracts == 0 || offers == 0 {
			t.Errorf("%s: %d contratos, %d ofertas", f.ID, contracts, offers)
		}
	}
}

// A loja nunca paga menos que a loja da praça pagaria pelos itens (sem lucro com
// comprar e vender), e toda oferta existe.
func TestRepShopPrices(t *testing.T) {
	seen := map[string]bool{}
	for _, o := range RepOffers {
		if seen[o.ID] || FactionByID(o.Faction) == nil || o.Grade < RepFriendly || o.Grade > RepExalted || o.Price < 1 || o.Text == "" {
			t.Errorf("%s: oferta inválida", o.ID)
		}
		seen[o.ID] = true
		if (o.Item == "") == (o.Title == "") {
			t.Errorf("%s: tem que ser um item ou um título", o.ID)
			continue
		}
		if o.Title != "" {
			if a := Achievements[o.Title]; a == nil || a.Goal != AchRepBuy || a.Target != a.ID {
				t.Errorf("%s: o título %q não é uma conquista da loja", o.ID, o.Title)
			}
			continue
		}
		d := Items[o.Item]
		if d == nil || o.Count < 1 || o.Count > d.Stack {
			t.Errorf("%s: item %q × %d", o.ID, o.Item, o.Count)
			continue
		}
		if o.Price < d.SellPrice()*int64(o.Count) {
			t.Errorf("%s: custa %d e a praça paga %d", o.ID, o.Price, d.SellPrice()*int64(o.Count))
		}
	}
	// Todo título da loja tem a sua oferta.
	for _, a := range RepTitleAchievements {
		found := false
		for _, o := range RepOffers {
			found = found || o.Title == a.ID
		}
		if !found || Achievements[a.ID] != a {
			t.Errorf("título %s sem oferta ou fora do catálogo", a.ID)
		}
	}
}

// Só itens horizontais: consumíveis, pergaminhos e capas com o mesmo poder do Manto das
// Cinzas (a capa épica do Ferreiro). Nada de arma, armadura ou joia, e nada melhor que o que
// o chefe deixa cair.
func TestRepShopIsHorizontal(t *testing.T) {
	ash := Items["cape_epic"]
	for _, o := range RepOffers {
		d := Items[o.Item]
		if d == nil {
			continue
		}
		switch {
		case d.Kind == KindPotion || d.Kind == KindScroll:
		case d.Kind == KindCape:
			if !d.RepOnly || d.Bonus != ash.Bonus || d.Grade != ash.Grade || d.ReqLevel != ash.ReqLevel || d.Stack != 1 {
				t.Errorf("%s: a capa %s não é igual ao Manto das Cinzas", o.ID, d.ID)
			}
		default:
			t.Errorf("%s: %s não é horizontal (tipo %d)", o.ID, d.ID, d.Kind)
		}
	}
	// As capas das facções não caem de monstro nenhum, nem do raid.
	for _, list := range gearByGrade {
		for _, d := range list {
			if d.RepOnly {
				t.Errorf("%s está no sorteio de equipamento", d.ID)
			}
		}
	}
	for _, d := range RepItems {
		if Items[d.ID] != d {
			t.Errorf("%s fora do catálogo", d.ID)
		}
	}
}

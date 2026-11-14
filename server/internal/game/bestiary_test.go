package game

import "testing"

func TestBestiaryMarks(t *testing.T) {
	minion := Monsters["skeleton_minion"]
	for _, c := range []struct{ kills, mark int }{{0, 0}, {9, 0}, {10, 1}, {99, 1}, {100, 2}, {499, 2}, {500, 3}, {10_000, 3}} {
		if got := BestiaryMark(minion, c.kills); got != c.mark {
			t.Errorf("%d abates: marco %d, quero %d", c.kills, got, c.mark)
		}
	}
	// Chefes e raid boss: 1 abate abre tudo.
	for _, id := range []string{"bone_lord", "ozrhal"} {
		m := Monsters[id]
		if !BestiaryOneKill(m) || BestiaryMark(m, 0) != 0 || BestiaryMark(m, 1) != 3 || BestiaryMarks(m) != [3]int{1, 1, 1} {
			t.Errorf("%s: chefe deveria abrir a entrada com 1 abate (%v)", id, BestiaryMarks(m))
		}
	}
	if BestiaryOneKill(minion) || BestiaryOneKill(Monsters["stone_golem"]) {
		t.Error("monstro comum não é chefe")
	}
}

func TestBestiaryDamageBonusAndCap(t *testing.T) {
	golem := Monsters["stone_golem"]
	if BestiaryBonus(golem, BestiaryMarkBonus-1) != 0 {
		t.Error("sem o 500º abate, sem bônus")
	}
	if got := BestiaryBonus(golem, BestiaryMarkBonus); got != BestiaryDamageBonus {
		t.Errorf("bônus no 500º = %v, quero %v", got, BestiaryDamageBonus)
	}
	// O teto: nada passa de BestiaryDamageCap, e o teto é pequeno (o orçamento do simulador
	// confere o efeito na caça, balance/bestiary.go).
	if BestiaryBonus(golem, BestiaryMaxKills) > BestiaryDamageCap || BestiaryDamageCap > 0.05 {
		t.Errorf("bônus %v acima do teto %v", BestiaryBonus(golem, BestiaryMaxKills), BestiaryDamageCap)
	}
	if BestiaryBonus(Monsters["ozrhal"], 1) != 0 || BestiaryBonus(Monsters["bone_lord"], 1000) != 0 {
		t.Error("chefes não dão bônus de dano")
	}
	if BestiaryBonus(nil, 1000) != 0 {
		t.Error("sem monstro, sem bônus")
	}
}

func TestBestiaryFamilies(t *testing.T) {
	skel, golem := BestiaryFamilyByID("skeletons"), BestiaryFamilyByID("golems")
	if skel == nil || golem == nil {
		t.Fatal("as famílias de esqueletos e golens existem")
	}
	for _, id := range []string{"skeleton_minion", "skeleton_warrior", "skeleton_rogue", "skeleton_mage", "bone_lord", "ozrhal", "bone_guard"} {
		if f := BestiaryFamilyOf(Monsters[id]); f != skel {
			t.Errorf("%s deveria ser da família dos esqueletos (é %v)", id, f)
		}
	}
	if BestiaryFamilyOf(Monsters["stone_golem"]) != golem {
		t.Error("o golem é da família dos golens")
	}
	// Um monstro novo com o modelo de esqueleto entra sozinho na família.
	if BestiaryFamilyOf(&MonsterDef{ID: "skeleton_rare", Model: "skeleton_rogue"}) != skel ||
		BestiaryFamilyOf(&MonsterDef{ID: "wolf", Model: "wolf"}) != nil {
		t.Error("família pelo modelo")
	}
	// Invocações pedem só o primeiro marco; chefes, 1; os outros, o último.
	for id, want := range map[string]int{"bone_guard": BestiaryMarkInfo, "bone_lord": 1, "ozrhal": 1, "skeleton_mage": BestiaryMarkBonus} {
		if got := BestiaryFamilyNeed(Monsters[id]); got != want {
			t.Errorf("%s pede %d para a família, quero %d", id, got, want)
		}
	}

	st := BestiaryState{Kills: map[string]BestiaryKill{}}
	members := BestiaryFamilyMembers(skel)
	if len(members) != 7 {
		t.Fatalf("esqueletos: %d entradas, quero 7", len(members))
	}
	for _, e := range members {
		st.Kills[e.Monster.ID] = BestiaryKill{N: BestiaryFamilyNeed(e.Monster)}
	}
	st.Kills["skeleton_rogue"] = BestiaryKill{N: BestiaryMarkBonus - 1}
	if d, n := st.FamilyProgress(skel); d != 6 || n != 7 {
		t.Fatalf("progresso %d/%d, quero 6/7", d, n)
	}
	st.Kills["skeleton_rogue"] = BestiaryKill{N: BestiaryMarkBonus}
	if d, n := st.FamilyProgress(skel); d != n {
		t.Fatalf("família completa: %d/%d", d, n)
	}
}

func TestBestiaryTitlesAndOutfits(t *testing.T) {
	for i, f := range BestiaryFamilies {
		if f.Title == "" || f.Outfit.Name == "" || f.Outfit.Top == "" {
			t.Errorf("%s sem título ou cor", f.ID)
		}
		if TitleOf(f.TitleID()) != f.Title || BestiaryFamilyByTitle(f.TitleID()) != f {
			t.Errorf("título de %s: %q", f.ID, TitleOf(f.TitleID()))
		}
		idx := f.OutfitIndex()
		if idx != BestiaryOutfitBase+i || BestiaryFamilyByOutfit(idx) != f || !IsBestiaryOutfit(idx) {
			t.Errorf("cor de %s no índice %d", f.ID, idx)
		}
		// O barbeiro aceita a cor (a zona confere a família); a criação, não.
		if !ValidAppearance(Appearance{Outfit: idx}) {
			t.Errorf("cor %d do bestiário deveria ser um visual válido", idx)
		}
		l := DefaultLook("knight")
		l.Outfit = idx
		if ValidLook("knight", l) {
			t.Errorf("a criação não aceita a cor %d do bestiário", idx)
		}
	}
	if IsBestiaryOutfit(len(OutfitColors)) || IsBestiaryOutfit(BestiaryOutfitBase+len(BestiaryFamilies)) || IsBestiaryOutfit(0) {
		t.Error("cores comuns e índices de fora não são do bestiário")
	}
	if ValidAppearance(Appearance{Outfit: BestiaryOutfitBase + len(BestiaryFamilies)}) || ValidAppearance(Appearance{Outfit: len(OutfitColors) + 1}) {
		t.Error("índice de cor que não existe")
	}
	if TitleOf("bst:nao_existe") != "" || BestiaryFamilyByTitle("first_kill") != nil {
		t.Error("título que não é de família")
	}
}

func TestBestiaryCatalog(t *testing.T) {
	cat := BestiaryCatalog()
	if len(cat) != len(Monsters) {
		t.Fatalf("catálogo com %d entradas, %d monstros", len(cat), len(Monsters))
	}
	if cat[0].Family != BestiaryFamilies[0] || BestiaryOneKill(cat[0].Monster) {
		t.Errorf("o catálogo começa pelos comuns da primeira família, não por %s", cat[0].Monster.ID)
	}
	seenBoss := map[*BestiaryFamilyDef]bool{}
	for _, e := range cat {
		if BestiaryEntryByID(e.Monster.ID) != e {
			t.Errorf("BestiaryEntryByID(%s)", e.Monster.ID)
		}
		if BestiaryOneKill(e.Monster) {
			seenBoss[e.Family] = true
		} else if seenBoss[e.Family] {
			t.Errorf("%s depois de um chefe da mesma família", e.Monster.ID)
		}
	}
	if BestiaryEntryByID("nao_existe") != nil {
		t.Error("monstro que não existe")
	}
}

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
	skelHi, golemHi := BestiaryFamilyByID("skeletons_ancient"), BestiaryFamilyByID("golems_ancient")
	if skel == nil || golem == nil || skelHi == nil || golemHi == nil {
		t.Fatal("as famílias de base e as Ancestrais existem")
	}
	// Faixa pelo maior nível em que o monstro nasce: até o 30, a de base; acima, a Ancestral.
	for id, want := range map[string]*BestiaryFamilyDef{
		"skeleton_minion": skel, "skeleton_mage": skel, "bone_lord": skel, "stone_golem": golem,
		"ossuary_knight": skelHi, "broken_arcanist": skelHi, "ozrhal": skelHi, "ancient_golem": golemHi,
	} {
		if m := Monsters[id]; m != nil && BestiaryFamilyOf(m) != want {
			t.Errorf("%s (nasce no %d) deveria ser de %s, é de %v", id, BestiarySpawnLevel(m), want.Name, BestiaryFamilyOf(m))
		}
	}
	// Um monstro novo com o modelo de esqueleto entra sozinho na família da faixa dele.
	if BestiaryFamilyOf(&MonsterDef{ID: "skeleton_new", Model: "skeleton_rogue"}) != skel ||
		BestiaryFamilyOf(&MonsterDef{ID: "wolf", Model: "wolf"}) != nil {
		t.Error("família pelo modelo")
	}
	for _, e := range BestiaryCatalog() {
		m := e.Monster
		// Toda família fica na faixa dela: nenhuma de base pede um monstro que nasce acima do 30.
		if e.Family != nil && BestiaryFamilyCounts(m) && !e.Family.High && BestiarySpawnLevel(m) > BestiaryBaseMaxLevel {
			t.Errorf("%s nasce no %d e conta para %s (de base)", m.ID, BestiarySpawnLevel(m), e.Family.Name)
		}
		// Raros e invocações são extras (0); os outros pedem o último marco (chefes: 1).
		want := BestiaryMarks(m)[2]
		if m.Rare != nil || BestiarySpawnLevel(m) == 0 {
			want = 0
		}
		if got := BestiaryFamilyNeed(m); got != want {
			t.Errorf("%s pede %d para a família, quero %d", m.ID, got, want)
		}
		if m.Rare != nil && e.Family != BestiaryFamilyOf(Monsters[m.Rare.Host]) {
			t.Errorf("o raro %s fica na família do hospedeiro %s", m.ID, m.Rare.Host)
		}
	}
	if BestiaryFamilyNeed(Monsters["bone_guard"]) != 0 || BestiaryFamilyNeed(Monsters["bone_lord"]) != 1 || BestiaryFamilyNeed(Monsters["skeleton_mage"]) != BestiaryMarkBonus {
		t.Error("Guarda Ossário é extra, chefe pede 1, comum pede o último marco")
	}

	// Completa com todas as que contam no último marco, sem os extras.
	for _, f := range BestiaryFamilies {
		st := BestiaryState{Kills: map[string]BestiaryKill{}}
		var counted []*BestiaryEntryDef
		for _, e := range BestiaryFamilyMembers(f) {
			if BestiaryFamilyCounts(e.Monster) {
				counted = append(counted, e)
				st.Kills[e.Monster.ID] = BestiaryKill{N: BestiaryFamilyNeed(e.Monster)}
			}
		}
		if len(counted) == 0 {
			t.Errorf("%s não tem nenhuma entrada que conte: nunca completa", f.Name)
			continue
		}
		if d, n := st.FamilyProgress(f); d != n || n != len(counted) {
			t.Errorf("%s completa: %d/%d (quero %d)", f.Name, d, n, len(counted))
		}
		last := counted[len(counted)-1].Monster.ID
		st.Kills[last] = BestiaryKill{N: st.Kills[last].N - 1}
		if d, n := st.FamilyProgress(f); d != n-1 {
			t.Errorf("%s com uma entrada faltando: %d/%d", f.Name, d, n)
		}
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

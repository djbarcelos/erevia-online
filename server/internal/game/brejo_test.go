package game

import "testing"

// Os cinco bichos do Brejo (tunnel#139), a Mãe-Lodo e a Velha Escamosa: papéis, neutralidade e a família do bestiário.
func TestBrejoMonsters(t *testing.T) {
	for id, want := range map[string]struct {
		arch         Archetype
		neutralByDay bool
	}{
		"brejeiro_lanceiro": {ArchPack, true}, "brejeiro_xama": {ArchHealer, true}, "sapo_boi": {ArchBrute, false},
		"lodo_vivo": {ArchSplitter, false}, "afogado": {ArchAmbusher, false}, "mae_lodo": {ArchSummoner, false},
	} {
		m := Monsters[id]
		if m == nil {
			t.Fatalf("falta o monstro %s", id)
		}
		if m.Archetype != want.arch || m.NeutralByDay != want.neutralByDay {
			t.Errorf("%s: papel %v, neutro de dia %v", id, m.Archetype, m.NeutralByDay)
		}
		if want.neutralByDay && (!m.NeutralNow(false) || m.NeutralNow(true)) {
			t.Errorf("%s: neutro de dia e agressivo à noite", id)
		}
	}
	if !Monsters["sapo_boi"].BalanceBaseline() || Monsters["brejeiro_lanceiro"].BalanceBaseline() {
		t.Error("o Sapo-Boi é o único bruto do Brejo (entra nas contas de caça); os de papel ficam fora")
	}
	if m := Monsters["mae_lodo"]; !m.Boss || len(m.Abilities) != 2 {
		t.Errorf("Mãe-Lodo: chefe de campo com Golfada e Maré de Lama: %+v", m)
	}
	r := Monsters["rare_escamosa"]
	if r == nil || r.Rare == nil || r.Archetype != ArchSummoner || r.Arch.Summon != "lodo_menor" || r.Name != "Velha Escamosa" {
		t.Fatalf("Velha Escamosa: %+v", r)
	}
	// Os bichos do Brejo (menos o esqueleto Afogado, da família dos Esqueletos) são da família Gente do Brejo, e só o Sapo-Boi a trava.
	f := BestiaryFamilyByID("brejo")
	if f == nil {
		t.Fatal("falta a família do bestiário")
	}
	for _, id := range []string{"brejeiro_lanceiro", "brejeiro_xama", "sapo_boi", "lodo_vivo", "mae_lodo", "rare_escamosa"} {
		if BestiaryFamilyOf(Monsters[id]) != f {
			t.Errorf("%s fora da família do Brejo", id)
		}
	}
	if BestiaryFamilyOf(Monsters["afogado"]).ID != "skeletons" {
		t.Error("o Afogado é esqueleto")
	}
	if !BestiaryFamilyCounts(Monsters["sapo_boi"]) || BestiaryFamilyCounts(Monsters["lodo_vivo"]) {
		t.Error("só o Sapo-Boi conta para completar a família (os de papel são extras)")
	}
	// Região: só os do Brejo.
	for _, s := range brejoSpawns() {
		if s.Region != BrejoRegion {
			t.Errorf("spawn %s fora da região", s.Monster)
		}
	}
}

// A linha de história do Brejo respeita o orçamento de XP de missões (game/story.go) e as diárias não repetem o sorteio dos outros.
func TestBrejoQuestsFitTheBudget(t *testing.T) {
	for lvl := 14; lvl <= 20; lvl++ {
		var share float64
		for _, q := range storyQuests {
			if questLevel(q) == lvl {
				share += float64(q.Reward.XP) / float64(XPToNext(lvl))
			}
		}
		if share > StoryShareMax+1e-6 {
			t.Errorf("nível %d: %.1f%% de XP de história (o teto é %.0f%%)", lvl, share*100, StoryShareMax*100)
		}
	}
	// As 3 diárias novas estão no fim da lista (acrescentar não muda o sorteio de ninguém).
	n := len(dailyQuests)
	for i, id := range []string{"d_juncos", "d_antidoto", "d_ronda_brejo"} {
		if dailyQuests[n-3+i].ID != id {
			t.Errorf("diária %d do fim = %s, esperava %s", i, dailyQuests[n-3+i].ID, id)
		}
	}
}

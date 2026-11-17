package game

import (
	"slices"
	"testing"
)

// Os monstros do Bosque Gris e das Terras Mortas (tunnel#137): papéis, modelos, o que cai de cada um e o chefe de campo.
func TestBosqueAndTerrasMortasMonsters(t *testing.T) {
	for id, want := range map[string]struct {
		arch  Archetype
		model string
		night bool // só nasce à noite
	}{
		"bandoleiro_corvo": {ArchAmbusher, "rogue", false}, "atirador_facas": {ArchArcher, "rogue", false},
		"brutamontes_corvo": {ArchShield, "barbarian", false}, "lobo_cinzento": {ArchPack, "wolf", false},
		"aranha_musgo":    {ArchAmbusher, "spider", true},
		"ossario_acolito": {ArchHealer, "mage", false}, "ossario_necromante": {ArchSummoner, "skeleton_mage", false},
		"ossario_cavaleiro": {ArchShield, "skeleton_warrior", false}, "carnical": {ArchPack, "skeleton_minion", true},
		"espectro": {ArchAmbusher, "skeleton_mage", true},
	} {
		m := Monsters[id]
		if m == nil {
			t.Fatalf("falta o monstro %s", id)
		}
		if m.Archetype != want.arch || m.Model != want.model || m.Tint == "" {
			t.Errorf("%s: papel %v, modelo %q, cor %q", id, m.Archetype, m.Model, m.Tint)
		}
		if !m.BestiaryExtra || m.BalanceBaseline() || !m.HasRole() {
			t.Errorf("%s: monstro de papel, extra no bestiário e fora das contas de caça", id)
		}
		var any, day, night bool
		for _, s := range DefaultSpawns() {
			if s.Monster == id && s.Count > 0 {
				any, day, night = any || s.When == WhenAny, day || s.When == WhenDay, night || s.When == WhenNight
			}
		}
		if want.night && (any || day || !night) || !want.night && night && !any && !day {
			t.Errorf("%s: sempre %v, de dia %v, à noite %v", id, any, day, night)
		}
	}
	if m := Monsters["ossario_cavaleiro"]; !m.Elite || len(m.Abilities) != 1 || m.Abilities[0].Shape != ShapeLine || m.Abilities[0].Length != 8 {
		t.Errorf("Cavaleiro do Ossário: elite com a Investida em linha de 8 m: %+v", m)
	}
	if m := Monsters["carnical"]; m.Speed <= 5 {
		t.Errorf("o Carniçal corre mais que o jogador (5 m/s): %.1f", m.Speed)
	}
	if m := Monsters["lobo_cinzento"]; m.Speed >= 5 {
		t.Errorf("o Lobo corre menos que o jogador (5 m/s): %.1f", m.Speed)
	}
	if m := Monsters["ossario_necromante"]; m.Arch.Summon != "lacaio_ossario" || !Monsters["lacaio_ossario"].NoGear {
		t.Errorf("Necromante: invoca Lacaios que não deixam saque: %+v", m.Arch)
	}
}

// O Corvo Rubro: chefe de campo de 3 fases na Toca do Corvo, com a janela de 60 a 90 min.
func TestCrowFieldBoss(t *testing.T) {
	m := Monsters["corvo_rubro"]
	if m == nil || !m.Boss || m.Raid || m.FieldBoss == nil || m.FieldBoss.PhaseCount() != 3 {
		t.Fatalf("Corvo Rubro: chefe de campo de 3 fases: %+v", m)
	}
	f := m.FieldBoss
	if f.RespawnMin != 60 || f.RespawnMax != 90 {
		t.Errorf("janela de %.0f a %.0f min, queria 60 a 90", f.RespawnMin, f.RespawnMax)
	}
	if f.Phases[0].AtHP != 0.60 || f.Phases[0].Summon != "bandoleiro_corvo" || f.Phases[0].SummonCount != 4 || f.Phases[1].AtHP != 0.30 {
		t.Errorf("fases: %+v", f.Phases)
	}
	byID := map[string]AbilityDef{}
	for _, a := range m.Abilities {
		byID[a.ID] = a
	}
	if a := byID["corvo_fumaca"]; a.Shape != ShapeCircle || a.Radius != 5 || a.Delay != 1.5 || a.Effect == nil || a.Effect.Kind != EffectBlind || a.Effect.Duration != 3 || a.MinPhase > 1 {
		t.Errorf("Bomba de Fumaça: %+v", a)
	}
	if a := byID["corvo_facas"]; a.Shape != ShapeCone || a.Arc != 90 || a.Radius != 8 || a.MinPhase != 2 {
		t.Errorf("Chuva de Facas: %+v", a)
	}
	if a := byID["corvo_salto"]; a.Target != AimMark || a.Radius != 4 || a.Delay != 3 || a.Mult != 1.4 || a.MinPhase != 3 {
		t.Errorf("Salto: %+v", a)
	}
	found := false
	for _, s := range DefaultSpawns() {
		if s.Monster == "corvo_rubro" {
			found = s.Fixed && s.X == CrowDenX && s.Z == CrowDenZ && s.LevelMin == 16
		}
	}
	if !found {
		t.Error("o Corvo Rubro nasce fixo na Toca do Corvo, no nível 16")
	}
}

// Os spawns do Bosque Gris e das Terras Mortas: dia e noite somam o mesmo, os setores têm dono (sem monstros) e cada grupo
// está na faixa de nível da região.
func TestBosqueAndTerrasMortasSpawns(t *testing.T) {
	for _, r := range []struct {
		region           string
		lo, hi, perPhase int
	}{{BosqueRegion, 7, 15, 16}, {TerrasMortasRegion, 24, 28, 14}} {
		var day, night int
		sector := 0
		for _, s := range DefaultSpawns() {
			if s.Region != r.region {
				continue
			}
			if s.Sector() {
				sector++
				if s.Count != 0 || !s.Owned() {
					t.Errorf("%s: o setor tem dono e nenhum monstro: %+v", r.region, s)
				}
				continue
			}
			if s.Fixed {
				continue
			}
			if !s.Area || s.LevelMin < r.lo || s.LevelMax > r.hi || s.LevelMin > s.LevelMax {
				t.Errorf("%s: %s no nível %d a %d", r.region, s.Monster, s.LevelMin, s.LevelMax)
			}
			if s.When.Active(false) {
				day += s.Count
			}
			if s.When.Active(true) {
				night += s.Count
			}
		}
		if sector != 1 || day != r.perPhase || night != r.perPhase {
			t.Errorf("%s: %d setor(es), %d monstros de dia e %d à noite (queria 1 setor e %d)", r.region, sector, day, night, r.perPhase)
		}
	}
	// Os spawns novos ficam no fim da lista (não mudam os ids de quem já existia).
	spawns := DefaultSpawns()
	first := slices.IndexFunc(spawns, func(s Spawn) bool { return s.Region == BosqueRegion })
	for _, s := range spawns[first:] {
		if s.Region != BosqueRegion && s.Region != TerrasMortasRegion {
			t.Fatalf("spawn de outra região no fim da lista: %+v", s)
		}
	}
}

// As linhas de história do Bosque Gris e das Terras Mortas respeitam o orçamento de XP de missões, e as diárias novas ficam
// depois das de antes (acrescentar não muda o sorteio de ninguém).
func TestBosqueQuestsFitTheBudget(t *testing.T) {
	for lvl := 1; lvl <= MaxLevel; lvl++ {
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
	if len(bosqueQuests) != 5 || len(terrasQuests) != 6 { // o convite e as missões (4 e 5)
		t.Errorf("%d missões no Bosque Gris e %d nas Terras Mortas (o convite e 4; o convite e 5)", len(bosqueQuests), len(terrasQuests))
	}
	if q := Quests["bos_corvo"]; q == nil || q.Steps[0].Target != "corvo_rubro" || q.Level != 16 {
		t.Errorf("a última do Bosque é o Corvo Rubro: %+v", q)
	}
	if q := Quests["tmo_selo"]; q == nil || len(q.Reward.Items) == 0 || q.Reward.Items[0].ID != "seal_fragment" {
		t.Errorf("a última das Terras Mortas dá o Fragmento de Selo: %+v", q)
	}
	ids := []string{"d_peles", "d_presas", "d_ronda_bosque", "d_velas_negras", "d_cinzas", "d_ronda_mortas"}
	first := slices.IndexFunc(dailyQuests, func(q *QuestDef) bool { return q.ID == ids[0] })
	if first < 0 || first+len(ids) != len(dailyQuests) {
		t.Fatalf("as diárias novas ficam no fim: %d de %d", first, len(dailyQuests))
	}
	for i, id := range ids {
		if dailyQuests[first+i].ID != id {
			t.Errorf("diária %d do fim = %s, esperava %s", i, dailyQuests[first+i].ID, id)
		}
	}
	// Todo item e monstro das missões existe.
	for _, q := range append(slices.Clone(bosqueQuests), terrasQuests...) {
		for _, s := range q.Steps {
			if s.Goal == GoalDeliver && Items[s.Target] == nil {
				t.Errorf("%s: item %s", q.ID, s.Target)
			}
		}
		for _, it := range q.Reward.Items {
			if Items[it.ID] == nil {
				t.Errorf("%s: recompensa %s", q.ID, it.ID)
			}
		}
	}
}

// 8 materiais novos e 6 receitas: as capas e luvas de nível 13, o cetro e a varinha épicos do 24 e os dois pergaminhos abençoados.
func TestBosqueMaterialsAndRecipes(t *testing.T) {
	mats := []string{"wolf_pelt", "wolf_fang", "moss_silk", "crow_insignia", "grave_ash", "black_candle", "profane_tome", "seal_fragment"}
	for _, id := range mats {
		if d := Items[id]; d == nil || d.Kind != KindMaterial || d.Icon == "" || d.Description == "" {
			t.Errorf("material %s: %+v", id, d)
		}
	}
	want := map[string]string{
		"craft_cape_corvo": "cape_rare", "craft_gloves_corvo": "gloves_rare", "craft_staff_devout": "staff_devout_epic",
		"craft_wand_devout": "wand_devout_epic", "craft_ench_weapon_b_tomo": "scroll_ench_weapon_b", "craft_ench_armor_b_cinza": "scroll_ench_armor_b",
	}
	for id, result := range want {
		r := Recipes[id]
		if r == nil || r.Result != result {
			t.Errorf("receita %s: %+v", id, r)
			continue
		}
		for _, m := range r.Materials {
			if !slices.Contains(mats, m.ID) && Items[m.ID] == nil {
				t.Errorf("%s: material %s", id, m.ID)
			}
		}
	}
	// Armas épicas do 24 (grau C): do mago, só de receita (fora do sorteio por raridade), abaixo da épica do grau (a escada de poder fica).
	for _, id := range []string{"staff_devout_epic", "wand_devout_epic"} {
		d := Items[id]
		if d == nil || d.Grade != GradeEpic || d.ReqLevel != 24 || d.Tier != TierC || !d.Graded || !d.UsableBy("mage") || d.UsableBy("knight") {
			t.Errorf("%s: %+v", id, d)
		}
		for _, g := range gearByGrade[GradeEpic] {
			if g.ID == id {
				t.Errorf("%s cai no sorteio por raridade", id)
			}
		}
		// Quem faz o cetro e a varinha paga mais que a loja daria por ele (não dá para fazer ouro).
		if r := Recipes[map[string]string{"staff_devout_epic": "craft_staff_devout", "wand_devout_epic": "craft_wand_devout"}[id]]; r.Gold <= d.SellPrice() {
			t.Errorf("%s: %d moedas pela receita, a loja paga %d", id, r.Gold, d.SellPrice())
		}
	}
	if Items["profane_tome"] == nil || Monsters["ossario_cavaleiro"] == nil {
		t.Fatal("o Tomo Profano e o Cavaleiro existem")
	}
	// O Tomo Profano é raro: cai do Cavaleiro com mais chance que do Acólito, e nunca é garantido.
	var knight, acolyte float64
	for _, e := range Monsters["ossario_cavaleiro"].Loot {
		if e.ItemID == "profane_tome" {
			knight = e.Chance
		}
	}
	for _, e := range Monsters["ossario_acolito"].Loot {
		if e.ItemID == "profane_tome" {
			acolyte = e.Chance
		}
	}
	if knight <= acolyte || acolyte <= 0 || knight >= 0.1 {
		t.Errorf("Tomo Profano: %.3f no Cavaleiro e %.3f no Acólito", knight, acolyte)
	}
}

// Os NPCs novos e a Lápide: o texto leva o marcador que a zona troca pelo nome de quem derrubou o Ozrhal.
func TestBosqueTalkers(t *testing.T) {
	for _, name := range []string{TalkCarter, TalkBrother, TalkTombstone} {
		tk := TalkerByName(name)
		if tk == nil || tk.Spawn == nil {
			t.Fatalf("falta o NPC %s", name)
		}
	}
	if tk := TalkerByName(TalkTombstone); tk.Spawn.Model != "lapide" || !containsToken(tk.Greeting) || !containsToken(tk.Idle) {
		t.Errorf("Lápide: %+v", tk)
	}
	if tp := TalkerByName(TalkBrother).Topic(TopicRegicide); tp == nil || !containsToken(tp.Text) {
		t.Error("o Irmão Caedmon diz quem derrubou o Ozrhal")
	}
}

func containsToken(s string) bool {
	for i := 0; i+len(TokenRegicide) <= len(s); i++ {
		if s[i:i+len(TokenRegicide)] == TokenRegicide {
			return true
		}
	}
	return false
}

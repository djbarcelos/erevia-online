package game

// Receitas do Ferreiro: materiais que caem dos monstros + moedas → equipamento raro,
// épico e lendário e pergaminhos abençoados (o Ferreiro só vende comum e bom). Criar
// tira moedas da economia e dá uso aos materiais de cada faixa de caça:
//
//	Osso Velho / Crânio Rachado  esqueletos (nível 1–12)
//	Essência Sombria             magos esqueletos (13–20) e chefes
//	Núcleo Rúnico                Golens de Pedra (20–25)

// Recipe é uma receita: Materials + Gold → Count × Result.
type Recipe struct {
	ID        string
	Result    string
	Count     int
	Materials []QuestItem
	Gold      int64
}

func recipe(id, result string, gold int64, mats ...QuestItem) *Recipe {
	return &Recipe{ID: id, Result: result, Count: 1, Materials: mats, Gold: gold}
}

func mat(id string, n int) QuestItem { return QuestItem{ID: id, Count: n} }

// RecipeList: em ordem de nível (é a ordem que o cliente mostra).
var RecipeList = []*Recipe{
	// Faixa 1 (ossos e crânios).
	recipe("craft_ench_armor", "scroll_ench_armor", 150, mat("bone_old", 15), mat("skull_cracked", 2)),
	recipe("craft_cape_good", "cape_good", 250, mat("bone_old", 20), mat("skull_cracked", 3)),
	// Faixa 2: equipamento raro.
	recipe("craft_ring_rare", "ring_rare", 800, mat("bone_old", 30), mat("skull_cracked", 10)),
	recipe("craft_ear_rare", "ear_rare", 800, mat("bone_old", 30), mat("skull_cracked", 10)),
	recipe("craft_neck_rare", "neck_rare", 1100, mat("bone_old", 35), mat("skull_cracked", 12)),
	recipe("craft_helm_rare", "helm_rare", 900, mat("bone_old", 30), mat("skull_cracked", 12)),
	recipe("craft_gloves_rare", "gloves_rare", 1000, mat("bone_old", 30), mat("skull_cracked", 12)),
	recipe("craft_boots_rare", "boots_rare", 1000, mat("bone_old", 30), mat("skull_cracked", 12)),
	recipe("craft_legs_rare", "legs_rare", 950, mat("bone_old", 30), mat("skull_cracked", 12)),
	recipe("craft_cape_rare", "cape_rare", 1000, mat("bone_old", 30), mat("skull_cracked", 12)),
	recipe("craft_armor_rare", "armor_rare", 1200, mat("bone_old", 40), mat("skull_cracked", 15)),
	// Faixa 3: épico e pergaminho abençoado de arma.
	recipe("craft_ench_weapon_b", "scroll_ench_weapon_b", 2000, mat("shadow_essence", 4), mat("skull_cracked", 6)),
	recipe("craft_ring_epic", "ring_epic", 2500, mat("shadow_essence", 5), mat("skull_cracked", 8)),
	recipe("craft_ear_epic", "ear_epic", 2500, mat("shadow_essence", 5), mat("skull_cracked", 8)),
	recipe("craft_neck_epic", "neck_epic", 3200, mat("shadow_essence", 6), mat("skull_cracked", 10)),
	recipe("craft_helm_epic", "helm_epic", 2800, mat("shadow_essence", 6), mat("skull_cracked", 10)),
	recipe("craft_gloves_epic", "gloves_epic", 3000, mat("shadow_essence", 6), mat("skull_cracked", 10)),
	recipe("craft_boots_epic", "boots_epic", 3000, mat("shadow_essence", 6), mat("skull_cracked", 10)),
	recipe("craft_legs_epic", "legs_epic", 2900, mat("shadow_essence", 6), mat("skull_cracked", 10)),
	recipe("craft_cape_epic", "cape_epic", 3000, mat("shadow_essence", 6), mat("skull_cracked", 10)),
	recipe("craft_armor_epic", "armor_epic", 3500, mat("shadow_essence", 8), mat("skull_cracked", 12)),
	// Faixa 4 (golens): lendário e pergaminho abençoado de armadura.
	recipe("craft_ench_armor_b", "scroll_ench_armor_b", 1000, mat("rune_core", 2), mat("shadow_essence", 2)),
	recipe("craft_ring_legend", "ring_legend", 8000, mat("rune_core", 5), mat("shadow_essence", 5)),
	recipe("craft_neck_legend", "neck_legend", 9000, mat("rune_core", 6), mat("shadow_essence", 6)),
}

// Recipes: as receitas por id.
var Recipes = func() map[string]*Recipe {
	m := make(map[string]*Recipe, len(RecipeList))
	for _, r := range RecipeList {
		m[r.ID] = r
	}
	return m
}()

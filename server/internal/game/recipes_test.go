package game

import "testing"

func TestRecipesAreConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range RecipeList {
		if seen[r.ID] {
			t.Errorf("duplicate recipe %q", r.ID)
		}
		seen[r.ID] = true
		if Items[r.Result] == nil || r.Count < 1 || r.Gold <= 0 || len(r.Materials) == 0 {
			t.Errorf("%s: bad recipe %+v", r.ID, r)
		}
		for _, m := range r.Materials {
			if d := Items[m.ID]; d == nil || d.Kind != KindMaterial || m.Count < 1 {
				t.Errorf("%s: bad material %+v", r.ID, m)
			}
		}
	}
	if len(Recipes) != len(RecipeList) {
		t.Fatal("Recipes map out of sync")
	}
}

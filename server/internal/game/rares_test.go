package game

import (
	"math/rand/v2"
	"testing"
)

func TestRareDefs(t *testing.T) {
	if len(Rares) != 4 {
		t.Fatalf("%d raros; a tabela 5.9 tem 4", len(Rares))
	}
	points := ExpandSpawns(DefaultSpawns(), rand.New(rand.NewPCG(20261112, 20261112^0x9e3779b97f4a7c15)))
	for _, r := range Rares {
		host, def := Monsters[r.Host], r.Monster()
		if host == nil || def == nil || def.Rare != r || def.ID != r.ID || def.Name != r.Name {
			t.Fatalf("%s: def do raro ou do hospedeiro faltando", r.ID)
		}
		if r.Chance < 0.02 || r.Chance > 0.04 {
			t.Errorf("%s: chance %v fora de 2–4%%", r.ID, r.Chance)
		}
		if def.HP != 4*host.HP || def.HPLvl != 4*host.HPLvl || def.Scale != host.Scale*RareScale {
			t.Errorf("%s: vida ×4 e 1,2× maior", r.ID)
		}
		if got := MonsterStats(def, 10).MaxHP; got != 4*MonsterStats(host, 10).MaxHP {
			t.Errorf("%s: vida %v no nível 10, esperava ×4", r.ID, got)
		}
		guaranteed := 0
		for _, e := range r.Loot {
			if Items[e.ItemID] == nil {
				t.Errorf("%s: saque %s não existe", r.ID, e.ItemID)
			}
			if e.Chance >= 1 {
				guaranteed++
			}
		}
		if guaranteed == 0 {
			t.Errorf("%s: sem saque garantido", r.ID)
		}
		// O saque garantido cai sempre.
		for seed := range uint64(20) {
			got := map[string]int{}
			for _, d := range RollLoot(def, r.RareLevel(1), rand.New(rand.NewPCG(seed, seed))) {
				got[d.Item.ID] += d.Count
			}
			for _, e := range r.Loot {
				if e.Chance >= 1 && got[e.ItemID] < e.Min {
					t.Fatalf("%s: o saque garantido %s não caiu (seed %d)", r.ID, e.ItemID, seed)
				}
			}
		}
		// A área tem hospedeiros no mapa de hoje.
		hosts := 0
		for _, p := range points {
			if p.Monster == host && r.Where.Contains(p.X, p.Z) {
				hosts++
				if l := r.RareLevel(p.Level); l != max(p.Level+RareLevelBonus, r.Level) {
					t.Errorf("%s: nível %d sobre o hospedeiro %d", r.ID, l, p.Level)
				}
			}
		}
		if hosts == 0 {
			t.Errorf("%s: nenhum %s na área dele", r.ID, host.Name)
		}
	}
	if a := Achievements["rare_hunter"]; a == nil || !a.Each || a.Count != len(Rares) || a.Target != RareIDs() {
		t.Fatal("Caçador de Raros: os 4 raros, cada um uma vez")
	}
}

func TestEachAchievementCountsDistinctTargets(t *testing.T) {
	a := Achievements["rare_hunter"]
	cur := 0
	for _, id := range []string{"rare_ossudo", "rare_ossudo", "rare_vossk", "skeleton_minion"} {
		cur = a.Mark(cur, id)
	}
	if a.Progress(cur) != 2 {
		t.Fatalf("progresso %d, esperava 2 (Ossudo e Vossk)", a.Progress(cur))
	}
}

package balance

import (
	"math"
	"testing"
)

// A tabela de combate do 20 ao 30 (tunnel#152) roda para todos os caminhos e níveis, com
// tempos finitos contra o monstro comum. As metas ainda não estão travadas: dependem da
// decisão do Daniel na tunnel#152.
func TestCombatTableRuns(t *testing.T) {
	rows := CombatTable()
	if len(rows) != 10*len(Paths()) {
		t.Fatalf("%d linhas, quer %d", len(rows), 10*len(Paths()))
	}
	for _, r := range rows {
		if r.Common <= 0 || math.IsInf(r.Common, 0) || math.IsNaN(r.Common) {
			t.Errorf("%s nv %d: comum %.2f s", r.Path.Name(), r.Level, r.Common)
		}
		if r.PvPKill <= 0 {
			t.Errorf("%s nv %d: PvP %.2f s", r.Path.Name(), r.Level, r.PvPKill)
		}
	}
	if len(BossTable()) == 0 {
		t.Fatal("sem chefes de campo na faixa")
	}
}

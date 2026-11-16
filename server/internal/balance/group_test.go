package balance

import (
	"math"
	"testing"

	"erevia/server/internal/game"
)

// A meta do tunnel#151: um grupo de 3 do mesmo nível rende a cada membro pelo menos 1,2× a
// XP por hora de quem caça sozinho (com a mesma gente na região: o renascimento é dividido).
func TestGroupOfThreeBeatsSolo(t *testing.T) {
	for _, l := range GroupLevels {
		g := BestGroup(GroupTrio, l, ShareRegion)
		if g.Died || g.Ratio() < 1.2-1e-9 {
			t.Errorf("nível %d: grupo de 3 rende %.2f× de quem caça sozinho (quero ≥ 1,2×; %+v)", l, g.Ratio(), g)
		}
	}
}

// Carregar: com mais de 8 níveis de diferença, o baixo rende menos que caçando sozinho.
func TestCarryDoesNotPay(t *testing.T) {
	for _, c := range [][2]int{{30, 21}, {30, 20}, {39, 30}} {
		r := SimulateCarry(c[0], c[1])
		if r.LowSoloXPH <= 0 || r.LowXPH >= r.LowSoloXPH {
			t.Errorf("%d carregando %d: o baixo faz %.0f XP/h, sozinho %.0f", c[0], c[1], r.LowXPH, r.LowSoloXPH)
		}
	}
}

// A divisão do simulador é a da zona: bônus, corte e parte pelo nível.
func TestGroupXPShare(t *testing.T) {
	def := game.Monsters["skeleton_warrior"]
	got := GroupXP(def, 12, []int{17, 8})
	total := float64(game.MonsterXP(def, 12, 17)) * 1.1 * 0.5
	if math.Abs(got[0]-total*17/25) > 1e-9 || math.Abs(got[1]-total*8/25) > 1e-9 {
		t.Fatalf("GroupXP = %v, quero %v e %v", got, total*17/25, total*8/25)
	}
}

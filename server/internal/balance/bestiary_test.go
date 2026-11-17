package balance

import (
	"math"
	"testing"

	"erevia/server/internal/game"
)

// O bônus do Bestiário entra no orçamento de poder (BestiaryViolations faz parte de
// AllViolations, cobrado por TestPowerBudget). Aqui: o simulador usa o mesmo bônus da zona,
// ele chega no dano de toda classe e caminho perto do número do bônus (sem vazar para mais
// que isso) e, em pontos de poder, cabe na folga da arma de todo nível.
func TestBestiaryBonusInPowerBudget(t *testing.T) {
	bonus := BestiaryVeteranBonus()
	zone := game.BestiaryBonus(game.Monsters["stone_golem"], game.BestiaryMarkBonus)
	if bonus != zone || bonus <= 0 || bonus > game.BestiaryDamageCap {
		t.Fatalf("bônus do veterano %v (zona: %v, teto %v)", bonus, zone, game.BestiaryDamageCap)
	}
	for _, v := range BestiaryViolations() {
		t.Errorf("fora do orçamento: %s %+.0f%%: %s", v.Key, v.Excess*100, v.Detail)
	}
	for _, b := range Bands {
		for _, p := range BandEntities(b) {
			if g := BestiaryDPSGain(b, p, bonus); math.Abs(g-bonus) > 0.01 {
				t.Errorf("%s no %s: o Bestiário soma %.2f%% ao dano por segundo (bônus %.0f%%)", p.Name(), b, g*100, bonus*100)
			}
		}
	}
	worst := 0.0
	for l := 1; l <= game.MaxLevel; l++ {
		for _, p := range Paths() {
			if pw, sl := BestiaryPower(p, l, bonus); sl > 0 {
				worst = math.Max(worst, pw/sl)
			}
		}
	}
	t.Logf("o Bestiário no teto usa no máximo %.0f%% da folga da arma", worst*100)
	if worst <= 0 {
		t.Fatal("o bônus do Bestiário não vale nada no orçamento")
	}
}

package game

import (
	"math"
	"strings"
	"testing"
	"time"
)

// A barra custa o mesmo que o último nível (39 → 40). Se a curva mudar, este teste cai de
// propósito: rever MasteryPointXP junto.
func TestMasteryPointXP(t *testing.T) {
	if MasteryPointXP != XPToNext(MaxLevel-1) {
		t.Fatalf("MasteryPointXP = %d, XPToNext(%d) = %d: rever a constante com a curva nova (tunnel#149)",
			MasteryPointXP, MaxLevel-1, XPToNext(MaxLevel-1))
	}
	if MasteryLevel != MaxLevel || XPToNext(MaxLevel) != 0 {
		t.Fatal("a Maestria abre no nível máximo, que não tem próximo nível")
	}
}

// Quem ganhou pontos no 30 (o máximo de antes) continua usando a árvore do 30 ao 39; sem
// pontos, a árvore só abre no 40.
func TestMasteryUsableBelowMaxWithOldPoints(t *testing.T) {
	old := MasteryState{Points: 4, Ranks: map[string]int{"wayfarer": 2}}
	if !old.Usable(30) || !old.Usable(MaxLevel-1) || !old.Usable(MaxLevel) {
		t.Fatal("pontos de antes valem do 30 em diante")
	}
	var none MasteryState
	if none.Usable(MaxLevel-1) || !none.Usable(MaxLevel) || MasteryLevel != 40 {
		t.Fatal("sem pontos, a Maestria abre no 40")
	}
}

// O teto de combate: dano, defesa, vida, crítico e velocidade de ataque, com a árvore
// inteira, somam no máximo 5%. (O teste da zona confere o mesmo nos atributos de verdade.)
func TestMasteryCombatCap(t *testing.T) {
	sum := 0.0
	for _, n := range MasteryNodes {
		if n.Stat.Combat() {
			sum += n.Value(MasteryNodeMax)
		}
	}
	if sum > MasteryCombatCap+1e-9 {
		t.Fatalf("bônus de combate com tudo no máximo = %.2f%%, teto %.0f%%", sum*100, MasteryCombatCap*100)
	}
	full := MasteryState{Points: MasteryMaxPoints, Ranks: map[string]int{}}
	for _, n := range MasteryNodes {
		full.Ranks[n.ID] = MasteryNodeMax
	}
	b := full.Bonus()
	if got := b.Of(MStatDamage) + b.Of(MStatMaxHP); math.Abs(got-sum) > 1e-9 {
		t.Fatalf("Bonus() soma %.4f de combate, os nós %.4f", got, sum)
	}
	if b.Of(MStatCalmSpeed) > MasterySpeedCap+1e-9 {
		t.Fatalf("velocidade fora de combate %.3f passa do teto %.3f", b.Of(MStatCalmSpeed), MasterySpeedCap)
	}
}

func TestMasteryTreeShape(t *testing.T) {
	total := 0.0
	for i, s := range MasteryRankShare {
		total += s
		if i > 0 && s >= MasteryRankShare[i-1] {
			t.Fatalf("retorno decrescente: o ponto %d (%.2f) não vale menos que o anterior", i+1, s)
		}
	}
	if math.Abs(total-1) > 1e-9 {
		t.Fatalf("as parcelas somam %.3f, esperava 1", total)
	}
	perBranch := map[int]int{}
	seen := map[string]bool{}
	for _, n := range MasteryNodes {
		if seen[n.ID] || MasteryNodeByID(n.ID) != n {
			t.Fatalf("nó %q repetido ou fora do índice", n.ID)
		}
		seen[n.ID] = true
		perBranch[n.Branch]++
		if n.Branch < 0 || n.Branch >= len(MasteryBranches) || n.Stat <= MStatNone || n.Stat >= mstatCount || n.Total <= 0 {
			t.Fatalf("nó inválido: %+v", n)
		}
		// O pré-requisito tem de caber nos outros nós do ramo.
		if n.Req > 0 && n.Req > (perBranchNodes(n.Branch)-1)*MasteryNodeMax {
			t.Fatalf("%s pede %d pontos, o ramo não tem", n.Name, n.Req)
		}
		if strings.Contains(n.Text(1), "%!") || !strings.Contains(n.Text(1), "%") {
			t.Fatalf("texto quebrado: %q", n.Text(1))
		}
	}
	if len(perBranch) != 3 || MasteryMaxPoints != len(MasteryNodes)*MasteryNodeMax {
		t.Fatalf("3 ramos, teto = todos os nós no máximo: %v / %d", perBranch, MasteryMaxPoints)
	}
	if got := MasteryNodeByID("field_alchemy").Text(1); got != "Cura das poções: +8%." {
		t.Fatalf("texto do 1º ponto = %q", got)
	}
	if got := MasteryNodeByID("wayfarer").Text(5); got != "Preço das Pedras de Teleporte: −30%." {
		t.Fatalf("texto com o nó cheio = %q", got)
	}
	if got := MasteryNodeByID("tanned_hide").Text(2); got != "Vida máxima: +1%." {
		t.Fatalf("texto com casa decimal = %q", got)
	}
}

func perBranchNodes(b int) int {
	n := 0
	for _, x := range MasteryNodes {
		if x.Branch == b {
			n++
		}
	}
	return n
}

func TestMasteryGainFillsBarAndGivesPoints(t *testing.T) {
	var m MasteryState
	if c, p := m.Gain(MasteryPointXP - 1); c != MasteryPointXP-1 || p != 0 || m.XP != MasteryPointXP-1 {
		t.Fatalf("quase cheia: credited %d, pontos %d, barra %d", c, p, m.XP)
	}
	if _, p := m.Gain(1); p != 1 || m.Points != 1 || m.XP != 0 {
		t.Fatalf("barra cheia vira 1 ponto: %+v", m)
	}
	// Vários pontos de uma vez (XP grande), e o resto fica na barra.
	if _, p := m.Gain(3*MasteryPointXP + 10); p != 3 || m.Points != 4 || m.XP != 10 || m.Free() != 4 {
		t.Fatalf("3 pontos de uma vez: %+v", m)
	}
	// No teto a barra para.
	m = MasteryState{Points: MasteryMaxPoints - 1, XP: MasteryPointXP - 5}
	if _, p := m.Gain(MasteryPointXP * 2); p != 1 || m.Points != MasteryMaxPoints || m.XP != 0 {
		t.Fatalf("chegou ao teto: %+v", m)
	}
	if c, p := m.Gain(1000); c != 0 || p != 0 || m.XP != 0 {
		t.Fatalf("no teto nada entra: %d %d %+v", c, p, m)
	}
}

func TestMasteryDailyBonus(t *testing.T) {
	var m MasteryState
	if !m.Refresh(100) || m.Rested != MasteryPointXP || m.Day != 100 {
		t.Fatalf("o primeiro dia já vem com 1 barra de bônus: %+v", m)
	}
	if m.Refresh(100) {
		t.Fatal("no mesmo dia nada muda")
	}
	// Com o bônus, 1/4 da XP enche a barra inteira, e o bônus acaba junto.
	if c, p := m.Gain(MasteryPointXP/4 - 1); c != (MasteryPointXP/4-1)*4 || p != 0 {
		t.Fatalf("bônus 4×: credited %d, pontos %d", c, p)
	}
	if _, p := m.Gain(1); p != 1 || m.Rested != 0 {
		t.Fatalf("a primeira barra do dia saiu 4× mais rápido: %+v", m)
	}
	if c, _ := m.Gain(100); c != 100 {
		t.Fatalf("sem bônus, XP normal: %d", c)
	}
	// Ganho que passa do bônus: a parte coberta conta 4×, o resto 1×.
	m = MasteryState{Rested: 400, Day: 1}
	if c, _ := m.Gain(300); c != 400+200 || m.Rested != 0 {
		t.Fatalf("100 de XP cobrem o bônus de 400, mais 200 normais: credited %d, %+v", c, m)
	}
	// Troca de dia: +1 barra por dia, até 7 dias guardados.
	m = MasteryState{Day: 10}
	m.Refresh(12)
	if m.Rested != 2*MasteryPointXP || m.Day != 12 {
		t.Fatalf("2 dias depois: %+v", m)
	}
	m.Refresh(40)
	if m.Rested != MasteryRestedDays*MasteryPointXP || RestedCap() != m.Rested {
		t.Fatalf("acumula até %d dias: %+v", MasteryRestedDays, m)
	}
	if m.Refresh(39) || m.Day != 40 {
		t.Fatal("o relógio voltando não tira nem dá bônus")
	}
	// O dia é o de Brasília: 23h59 e 00h01 de lá são dias diferentes.
	brt := time.FixedZone("BRT", -3*3600)
	a := time.Date(2026, 11, 13, 23, 59, 0, 0, brt)
	if MasteryDay(a.Add(2*time.Minute), brt) != MasteryDay(a, brt)+1 || MasteryDay(a, brt) != MasteryDay(a.UTC(), brt) {
		t.Fatal("o dia vira à meia-noite de Brasília")
	}
}

func TestMasteryDeathLoss(t *testing.T) {
	const into = MasteryPointXP / 2
	m := MasteryState{Points: 3, XP: into}
	var none MasteryBonus
	loss := m.DeathLoss(&none)
	if want := int64(math.Round(float64(MasteryPointXP) * DeathXPLossLate)); loss != want {
		t.Fatalf("morte tira %d, esperava 2%% da barra = %d", loss, want)
	}
	if lost := m.Lose(loss); lost != loss || m.XP != into-loss || m.Points != 3 {
		t.Fatalf("perdeu %d: %+v", lost, m)
	}
	if lost := m.Lose(MasteryPointXP); lost != into-loss || m.XP != 0 || m.Points != 3 {
		t.Fatalf("a barra não fica negativa e o ponto não sai: %d %+v", lost, m)
	}
	half := MasteryState{Points: 5, Ranks: map[string]int{"second_chance": 5}}
	b := half.Bonus()
	if got := half.DeathLoss(&b); got != int64(math.Round(float64(MasteryPointXP)*DeathXPLossLate*0.5)) {
		t.Fatalf("Segunda Chance cheia: perde metade, %d", got)
	}
}

func TestMasterySpend(t *testing.T) {
	m := MasteryState{Points: 7}
	if why := m.Spend("nada"); !strings.Contains(why, "não existe") {
		t.Fatalf("nó desconhecido: %q", why)
	}
	if why := m.Spend("keen_eye"); !strings.Contains(why, "5 pontos em Caça") {
		t.Fatalf("pré-requisito: %q", why)
	}
	for range 5 {
		if why := m.Spend("field_alchemy"); why != "" {
			t.Fatal(why)
		}
	}
	if why := m.Spend("field_alchemy"); !strings.Contains(why, "máximo") {
		t.Fatalf("teto do nó: %q", why)
	}
	if why := m.Spend("keen_eye"); why != "" { // 5 em Caça: abriu
		t.Fatal(why)
	}
	if why := m.Spend("hunter_instinct"); !strings.Contains(why, "10 pontos") {
		t.Fatalf("o nó de 10 continua fechado com 6: %q", why)
	}
	if why := m.Spend("steadfast"); why != "" {
		t.Fatal(why)
	}
	if why := m.Spend("wayfarer"); !strings.Contains(why, "Sem pontos") || m.Free() != 0 || m.Spent() != 7 {
		t.Fatalf("pontos insuficientes: %q %+v", why, m)
	}
	if m.BranchPoints(0) != 6 || m.BranchPoints(1) != 1 || m.BranchPoints(2) != 0 {
		t.Fatalf("pontos por ramo: %d %d %d", m.BranchPoints(0), m.BranchPoints(1), m.BranchPoints(2))
	}
	b := m.Bonus()
	if math.Abs(b.Of(MStatPotion)-0.20) > 1e-9 || math.Abs(b.Of(MStatMaterial)-0.06) > 1e-9 || math.Abs(b.Of(MStatCCResist)-0.10) > 1e-9 {
		t.Fatalf("efeitos: poção %.3f, material %.3f, firmeza %.3f", b.Of(MStatPotion), b.Of(MStatMaterial), b.Of(MStatCCResist))
	}
}

func TestMasteryRespecOncePerDay(t *testing.T) {
	m := MasteryState{Points: 2}
	if why := m.Respec(5); !strings.Contains(why, "Nenhum ponto") {
		t.Fatalf("sem nada gasto: %q", why)
	}
	m.Spend("wayfarer")
	m.Spend("wayfarer")
	if !m.CanRespec(5) || m.Respec(5) != "" || m.Spent() != 0 || m.Free() != 2 || m.Points != 2 {
		t.Fatalf("redistribuir devolve os pontos: %+v", m)
	}
	m.Spend("light_step")
	if why := m.Respec(5); !strings.Contains(why, "hoje") || m.Spent() != 1 {
		t.Fatalf("só 1 vez por dia: %q", why)
	}
	if m.CanRespec(5) || !m.CanRespec(6) || m.Respec(6) != "" {
		t.Fatal("no dia seguinte libera de novo")
	}
}

func TestNormalMastery(t *testing.T) {
	ok := NormalMastery(MasteryState{Points: 6, XP: -3, Rested: RestedCap() * 3,
		Ranks: map[string]int{"field_alchemy": 9, "sumiu": 3, "steadfast": 0}})
	if ok.XP != 0 || ok.Rested != RestedCap() || len(ok.Ranks) != 1 || ok.Ranks["field_alchemy"] != MasteryNodeMax {
		t.Fatalf("limites e nós que não existem: %+v", ok)
	}
	// Mais pontos gastos do que ganhos, ou pré-requisito quebrado: os pontos voltam.
	if bad := NormalMastery(MasteryState{Points: 2, Ranks: map[string]int{"wayfarer": 3}}); bad.Ranks != nil {
		t.Fatalf("gastou mais do que tem: %+v", bad)
	}
	if bad := NormalMastery(MasteryState{Points: 10, Ranks: map[string]int{"way_home": 2}}); bad.Ranks != nil {
		t.Fatalf("nó sem o pré-requisito: %+v", bad)
	}
	keep := MasteryState{Points: 12, Ranks: map[string]int{"wayfarer": 5, "light_step": 5, "way_home": 1}}
	if got := NormalMastery(keep); got.Spent() != 11 {
		t.Fatalf("árvore válida fica como está: %+v", got)
	}
	if top := NormalMastery(MasteryState{Points: MasteryMaxPoints + 9, XP: 77}); top.Points != MasteryMaxPoints || top.XP != 0 {
		t.Fatalf("teto: %+v", top)
	}
}

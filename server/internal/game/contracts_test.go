package game

import (
	"slices"
	"strings"
	"testing"
	"time"
)

var brt = time.FixedZone("BRT", -3*3600) // o fuso dos eventos da zona (zone.brasilia)

// "Perto do nível máximo": os 5 últimos níveis, saindo de MaxLevel.
func TestContractMinLevel(t *testing.T) {
	if ContractMinLevel != MaxLevel-5 || ContractMinLevel < 1 {
		t.Fatalf("ContractMinLevel = %d com MaxLevel = %d", ContractMinLevel, MaxLevel)
	}
}

// No nível máximo os diários não são triviais: cada caça e cada chefe mira monstros de uma
// área cujo topo chega perto do máximo, e a coleta usa uma área do 30 ao 40.
func TestContractTargetsNotTrivial(t *testing.T) {
	top := map[string]int{} // área → maior nível de monstro
	for _, sp := range DefaultSpawns() {
		if sp.Region != "" {
			top[sp.Region] = max(top[sp.Region], sp.LevelMax, sp.LevelMin)
		}
	}
	for _, c := range ContractList {
		if c.Slot == ContractWeekly {
			continue
		}
		if c.Area == "" || top[c.Area] < MaxLevel-5 {
			t.Errorf("%s: área %q chega só ao nível %d (o máximo é %d)", c.ID, c.Area, top[c.Area], MaxLevel)
		}
		if c.Goal != CGoalKill {
			continue
		}
		found := false
		for _, sp := range DefaultSpawns() {
			if sp.Region == c.Area && c.Matches(sp.Monster) && max(sp.LevelMax, sp.LevelMin) >= c.MinLevel {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: nenhum %s de nível %d+ nasce em %s", c.ID, c.Target, c.MinLevel, c.Area)
		}
	}
}

func TestContractData(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range ContractList {
		if seen[c.ID] || c.ID == "" || c.Title == "" || c.Text == "" || c.Place == "" {
			t.Errorf("%q: id repetido ou sem texto", c.ID)
		}
		seen[c.ID] = true
		if c.Count < 1 || c.Gold < 1 || c.Rep < 1 || c.XPShare <= 0 || c.XPShare > 1 {
			t.Errorf("%s: contagem ou recompensa fora da faixa", c.ID)
		}
		if FactionByID(c.Faction) == nil {
			t.Errorf("%s: facção %q não existe", c.ID, c.Faction)
		}
		if (c.Slot == ContractWeekly) != (c.Region == "") {
			t.Errorf("%s: diário sem dia da região (ou semanal com)", c.ID)
		}
		for _, id := range splitTargets(c.Target) {
			switch c.Goal {
			case CGoalKill, CGoalRaid:
				if Monsters[id] == nil {
					t.Errorf("%s: monstro %q não existe", c.ID, id)
				}
			case CGoalLoot:
				if d := Items[id]; d == nil || d.Kind != KindMaterial {
					t.Errorf("%s: material %q não existe", c.ID, id)
				}
			}
		}
		if c.Goal == CGoalRaid && (Monsters[c.Target] == nil || !Monsters[c.Target].Raid) {
			t.Errorf("%s: %q não é raid boss", c.ID, c.Target)
		}
	}
	// Cada vaga tem opção nos dois dias da região, e o semanal tem pelo menos 3 (a permutação
	// sem repetir precisa de 3 para não depender do ciclo anterior).
	for _, slot := range DailySlots {
		for _, region := range ContractRegions {
			if len(contractPool(slot, region)) == 0 {
				t.Errorf("vaga %d sem contrato no dia %s", slot, region)
			}
		}
	}
	if n := len(contractPool(ContractWeekly, "")); n < 3 {
		t.Fatalf("só %d semanais", n)
	}
}

func splitTargets(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "|")
}

// O dia de hoje no quadro: 3 diários (caça, chefe, coleta) e o semanal; o mesmo
// personagem no mesmo dia vê sempre o mesmo quadro, e personagens diferentes variam.
func TestDailyDrawIsDeterministicPerCharacterAndDay(t *testing.T) {
	day := ContractDay(time.Date(2026, 11, 13, 15, 0, 0, 0, brt), brt)
	differs := false
	for char := uint32(1); char <= 50; char++ {
		a, b := OfferedContracts(char, day), OfferedContracts(char, day)
		if !slices.Equal(a, b) {
			t.Fatalf("personagem %d: o mesmo dia deu quadros diferentes", char)
		}
		if len(a) != 4 || a[0].Slot != ContractHunt || a[1].Slot != ContractBoss || a[2].Slot != ContractCollect || a[3].Slot != ContractWeekly {
			t.Fatalf("personagem %d: quadro %v", char, ids(a))
		}
		if !slices.Equal(ids(a), ids(OfferedContracts(1, day))) {
			differs = true
		}
	}
	if !differs {
		t.Fatal("50 personagens com o mesmo quadro: o sorteio não usa o personagem")
	}
}

func ids(list []*ContractDef) []string {
	var out []string
	for _, c := range list {
		out = append(out, c.ID)
	}
	return out
}

// Nenhuma vaga repete o contrato do dia anterior (nem o semanal o da semana anterior), e
// em algumas semanas cada contrato aparece.
func TestDrawNeverRepeatsYesterday(t *testing.T) {
	start := ContractDay(time.Date(2026, 1, 1, 12, 0, 0, 0, brt), brt)
	for char := uint32(1); char <= 300; char += 7 {
		shown := map[string]bool{}
		prev := DailyContracts(char, start-1)
		for day := start; day < start+400; day++ {
			cur := DailyContracts(char, day)
			for i := range cur {
				shown[cur[i].ID] = true
				if cur[i] == prev[i] {
					t.Fatalf("personagem %d, dia %d: a vaga %d repetiu %s", char, day, cur[i].Slot, cur[i].ID)
				}
			}
			prev = cur
		}
		prevW := WeeklyContract(char, ContractWeek(start)-1)
		for week := ContractWeek(start); week < ContractWeek(start)+60; week++ {
			w := WeeklyContract(char, week)
			shown[w.ID] = true
			if w == prevW {
				t.Fatalf("personagem %d, semana %d: o semanal repetiu %s", char, week, w.ID)
			}
			prevW = w
		}
		for _, c := range ContractList {
			if !shown[c.ID] {
				t.Errorf("personagem %d: %s nunca saiu em 400 dias", char, c.ID)
			}
		}
	}
}

// A caça e o chefe do dia não enchem a coleta do dia: as três vagas ficam sempre em áreas
// diferentes (e a coleta só conta o saque de monstros que nasceram na área dela).
func TestDailyHuntDoesNotFillCollect(t *testing.T) {
	for char := uint32(1); char <= 40; char++ {
		for day := 20700; day < 20760; day++ {
			list := DailyContracts(char, day)
			hunt, boss, collect := list[0], list[1], list[2]
			if hunt.Area == boss.Area || hunt.Area == collect.Area || boss.Area == collect.Area || hunt.Area != hunt.Region {
				t.Fatalf("dia %d: caça em %s, chefe em %s, coleta em %s", day, hunt.Area, boss.Area, collect.Area)
			}
			if !InContractArea(collect.Area, contractAreas[collect.Area][0].X, contractAreas[collect.Area][0].Z) ||
				InContractArea(collect.Area, contractAreas[hunt.Area][0].X, contractAreas[hunt.Area][0].Z) {
				t.Fatalf("dia %d: as áreas da coleta e da caça se encostam", day)
			}
		}
	}
}

// Renovação à meia-noite de Brasília: 23:59:59 ainda é o dia anterior, 00:00 já é o novo.
// O semanal vira na segunda-feira, 0 h.
func TestContractsRenewAtBrasiliaMidnight(t *testing.T) {
	before := time.Date(2026, 11, 14, 2, 59, 59, 0, time.UTC) // sexta 13/11, 23:59:59 em Brasília
	after := before.Add(time.Second)                          // sábado 14/11, 00:00
	d0, d1 := ContractDay(before, brt), ContractDay(after, brt)
	if d1 != d0+1 {
		t.Fatalf("meia-noite de Brasília: dia %d → %d", d0, d1)
	}
	if got := NextDailyReset(d0, brt); !got.Equal(after) {
		t.Fatalf("próxima renovação %v, esperava %v", got.UTC(), after)
	}
	// Às 23:00 de Brasília já é outro dia em UTC, mas não no quadro.
	if ContractDay(time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC), brt) != d0 {
		t.Fatal("o dia do quadro seguiu o UTC, não Brasília")
	}
	// A semana: domingo 15/11 23:59:59 → segunda 16/11 00:00 (Brasília).
	sun := time.Date(2026, 11, 16, 2, 59, 59, 0, time.UTC)
	mon := sun.Add(time.Second)
	if ContractWeek(ContractDay(mon, brt)) != ContractWeek(ContractDay(sun, brt))+1 {
		t.Fatal("o semanal não virou na segunda-feira")
	}
	if ContractWeek(d0) != ContractWeek(ContractDay(sun, brt)) {
		t.Fatal("sexta e domingo deveriam ser da mesma semana")
	}
	if got := NextWeeklyReset(d0, brt); !got.Equal(mon) {
		t.Fatalf("próximo semanal %v, esperava %v", got.UTC(), mon)
	}
	if time.Date(2026, 11, 16, 0, 0, 0, 0, brt).Weekday() != time.Monday {
		t.Fatal("16/11/2026 deveria ser segunda-feira")
	}
	// O fuso fixo da zona (UTC−3) é o mesmo de America/Sao_Paulo hoje (sem horário de verão
	// desde 2019): confere a meia-noite de cada dia de 2026 com o banco de fusos do Go.
	if sp, err := time.LoadLocation("America/Sao_Paulo"); err == nil {
		for t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, sp); t0.Year() == 2026; t0 = t0.AddDate(0, 0, 1) {
			if ContractDay(t0, brt) != ContractDay(t0, sp) || ContractDay(t0.Add(-time.Second), brt) != ContractDay(t0.Add(-time.Second), sp) {
				t.Fatalf("%v: o dia do quadro difere de America/Sao_Paulo", t0)
			}
		}
	}
}

// Virar o dia tira os diários aceitos ontem e mantém o semanal da semana; virar a semana
// tira o semanal.
func TestContractStateRefresh(t *testing.T) {
	const char = 9
	fri := ContractDay(time.Date(2026, 11, 13, 12, 0, 0, 0, brt), brt)
	st := ContractState{}
	if !st.Refresh(char, fri) || st.Day != fri || st.Week != ContractWeek(fri) {
		t.Fatalf("primeira vez: %+v", st)
	}
	today := OfferedContracts(char, fri)
	st.Taken = map[string]*ContractProgress{today[0].ID: {N: 3}, today[3].ID: {N: 1}}
	if st.Refresh(char, fri) {
		t.Fatal("o mesmo dia não muda nada")
	}
	sat := fri + 1
	st.Refresh(char, sat)
	if st.Taken[today[0].ID] != nil {
		t.Fatal("o diário de ontem continuou")
	}
	if p := st.Taken[today[3].ID]; p == nil || p.N != 1 {
		t.Fatal("o semanal sumiu no meio da semana")
	}
	st.Refresh(char, fri+3) // segunda-feira
	if len(st.Taken) != 0 {
		t.Fatalf("o semanal passou da segunda-feira: %+v", st.Taken)
	}
}

func TestNormalContracts(t *testing.T) {
	st := NormalContracts(ContractState{Day: 5, Taken: map[string]*ContractProgress{
		"nao_existe":          {N: 3},
		"hunt_fundo_golems":   {N: 1_000_000},
		"collect_fundo_cores": {N: -4},
		"weekly_pvp":          {N: 1, Seen: []uint32{1, 2, 3, 4, 5, 6}},
		"weekly_ozrhal":       {Done: true},
		"boss_fundo_elders":   nil,
	}})
	if len(st.Taken) != 4 || st.Taken["hunt_fundo_golems"].N != 70 || st.Taken["collect_fundo_cores"].N != 0 ||
		len(st.Taken["weekly_pvp"].Seen) != 3 || st.Taken["weekly_ozrhal"].N != 1 {
		t.Fatalf("estado limpo errado: %+v", st.Taken)
	}
}

func TestContractRewardXP(t *testing.T) {
	c := Contracts["hunt_fundo_golems"]
	if got, want := c.RewardXP(MaxLevel), int64(float64(MasteryPointXP)*c.XPShare+0.5); got != want {
		t.Fatalf("no nível máximo a XP é da barra de Maestria: %d, esperava %d", got, want)
	}
	if got, want := c.RewardXP(ContractMinLevel), int64(float64(XPToNext(ContractMinLevel))*c.XPShare+0.5); got != want {
		t.Fatalf("no %d: %d, esperava %d", ContractMinLevel, got, want)
	}
}

func TestContractAreas(t *testing.T) {
	cases := []struct {
		area string
		x, z float64
		in   bool
	}{
		{AreaFundo, 0, -300, true}, {AreaFundo, 0, -250, false},
		{AreaVigil, 140, 72, true}, {AreaVigil, 72, 140, true}, {AreaVigil, 108, 108, false},
		{AreaConfins, -122, 122, true}, {AreaConfins, 122, -122, true}, {AreaConfins, 0, 0, false},
		{"", 1, 2, true},
	}
	for _, c := range cases {
		if InContractArea(c.area, c.x, c.z) != c.in {
			t.Errorf("InContractArea(%q, %v, %v) != %v", c.area, c.x, c.z, c.in)
		}
	}
	// Toda área da rotação existe nos spawns.
	for _, r := range ContractRegions {
		if len(contractAreas[r]) == 0 {
			t.Errorf("a área %s não tem spawn", r)
		}
	}
}

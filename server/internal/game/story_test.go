package game

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"
)

// Os passos novos (coletar, visitar e entregar) e as missões com NPC têm o que precisam.
func TestStoryAndDailyStepsAreWellFormed(t *testing.T) {
	for _, q := range QuestsInOrder() {
		for _, s := range q.Steps {
			switch s.Goal {
			case GoalCollect:
				if Items[s.Target] == nil || len(LootSources(s.Target)) == 0 {
					t.Errorf("%s: coletar %q, que nenhum monstro comum deixa cair", q.ID, s.Target)
				}
			case GoalDeliver:
				d := Items[s.Target]
				if d == nil || d.Stack <= 1 || TalkerByName(s.NPC) == nil {
					t.Errorf("%s: entregar %q (empilhável?) a %q (conversa?)", q.ID, s.Target, s.NPC)
				}
			case GoalVisit:
				if s.Target == "" || s.Place == nil || s.Place.Kind != PlaceSpot || s.Place.Radius < 3 || s.Place.Name == "" {
					t.Errorf("%s: visitar precisa de chave e de lugar: %+v", q.ID, s)
				}
			}
		}
		if q.Giver != "" && TalkerByName(q.Giver) == nil || q.TurnIn != "" && TalkerByName(q.TurnIn) == nil {
			t.Errorf("%s: quem dá (%q) ou recebe (%q) não conversa", q.ID, q.Giver, q.TurnIn)
		}
		if q.TurnIn != "" && q.Done == "" {
			t.Errorf("%s: recebida por %s sem fala de entrega", q.ID, q.TurnIn)
		}
		if q.Giver != "" && q.Level < 1 {
			t.Errorf("%s: dada por %s sem nível", q.ID, q.Giver)
		}
	}
}

// Cada região de caça tem a sua linha de história (convite + 3 a 4 missões), contada por um
// NPC que fica no mapa principal, e o convite chega no nível da região.
func TestEveryRegionHasAStoryLine(t *testing.T) {
	regions := []string{"Arredores", "Campos dos Ossos", "Ermos Partidos", "Vale das Pedras Vivas",
		"Fundo do Vale", "Vigília Ossária", "Confins Partidos"}
	for _, r := range regions {
		var line []*QuestDef
		for _, q := range storyQuests {
			if q.Region == r {
				line = append(line, q)
			}
		}
		if n := len(line); n < 4 || n > 5 {
			t.Errorf("%s: %d missões na linha (quero convite + 3 a 4)", r, n)
			continue
		}
		if line[0].Unlock == 0 || line[0].Next != line[1].ID {
			t.Errorf("%s: a linha começa com o convite de nível: %+v", r, line[0])
		}
		giver := TalkerByName(line[1].Giver)
		if giver == nil || giver.Spawn == nil || giver.Region != r {
			t.Errorf("%s: o contador de história %q precisa ser um NPC novo da região", r, line[1].Giver)
		}
		for i, q := range line[1:] {
			if q.Giver != giver.Name || q.Category != QuestRegion {
				t.Errorf("%s: %s dada por %q", r, q.ID, q.Giver)
			}
			if i > 0 && q.Level < line[i].Level {
				t.Errorf("%s: %s (nível %d) antes de %s (nível %d)", r, line[i].ID, line[i].Level, q.ID, q.Level)
			}
		}
	}
}

// As missões de história saem do orçamento de missões: com o contrato do nível, a XP de
// missões de cada nível continua 30% da barra (a curva não muda).
func TestStoryXPComesOutOfTheContract(t *testing.T) {
	for l := ContractFrom; l < MaxLevel; l++ {
		story := StoryXPAt(l)
		if float64(story) > StoryShareMax*float64(XPToNext(l))+1 {
			t.Errorf("nível %d: %d de XP de história (máximo %.0f%% da barra)", l, story, StoryShareMax*100)
		}
		c := Quests[ContractID(l)]
		want := int64(math.Round(float64(XPToNext(l)) * ContractXPShare))
		if c.Reward.XP+story != want {
			t.Errorf("nível %d: contrato %d + história %d ≠ %d", l, c.Reward.XP, story, want)
		}
	}
}

// Três diárias por dia, uma de cada vaga, para todo nível do 5 ao 35; nenhuma fora disso.
func TestDailyDrawCoversLevels(t *testing.T) {
	day := ContractDay(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC), time.UTC)
	for l := 1; l <= MaxLevel; l++ {
		got := DailyQuests(42, day, l)
		if l < DailyMinLevel || l > DailyMaxLevel {
			if len(got) != 0 {
				t.Errorf("nível %d: %d diárias fora da faixa", l, len(got))
			}
			continue
		}
		if len(got) != 3 {
			t.Fatalf("nível %d: %d diárias", l, len(got))
		}
		for i, q := range got {
			if q.Daily != DailyVacancies[i] || l < q.Level || l > q.MaxLevel || q.Giver != TalkHerald {
				t.Errorf("nível %d: %s na vaga %d (faixa %d–%d)", l, q.ID, i, q.Level, q.MaxLevel)
			}
			for _, s := range q.Steps {
				if s.Goal == GoalKill && s.Target != "bone_lord" {
					t.Errorf("%s: as diárias não pedem caça (é o sabor dos contratos de caça)", q.ID)
				}
			}
		}
	}
}

// O sorteio é o mesmo para o mesmo personagem, dia e nível, muda entre personagens e não
// repete ontem quando há mais de uma opção.
func TestDailyDrawIsDeterministicAndRotates(t *testing.T) {
	base := ContractDay(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC), time.UTC)
	ids := func(qs []*QuestDef) string {
		var s []string
		for _, q := range qs {
			s = append(s, q.ID)
		}
		return strings.Join(s, ",")
	}
	if ids(DailyQuests(7, base, 25)) != ids(DailyQuests(7, base, 25)) {
		t.Fatal("o mesmo dia deu sorteios diferentes")
	}
	differ := false
	for c := uint32(1); c < 40; c++ {
		if ids(DailyQuests(c, base, 25)) != ids(DailyQuests(c+1, base, 25)) {
			differ = true
		}
	}
	if !differ {
		t.Fatal("todo personagem tem as mesmas diárias")
	}
	for c := uint32(1); c < 20; c++ {
		for d := base; d < base+30; d++ {
			a, b := DailyQuests(c, d, 25), DailyQuests(c, d+1, 25)
			for i := range a {
				if len(dailyPool(DailyVacancies[i], 25)) > 1 && a[i] == b[i] {
					t.Fatalf("personagem %d repetiu %s nos dias %d e %d", c, a[i].ID, d, d+1)
				}
			}
		}
	}
}

// O rol vira à meia-noite de Brasília: novo sorteio e nenhuma entregue.
func TestDailyStateRefresh(t *testing.T) {
	loc := time.FixedZone("BRT", -3*3600)
	before := ContractDay(time.Date(2026, 11, 16, 23, 59, 0, 0, loc), loc)
	after := ContractDay(time.Date(2026, 11, 17, 0, 1, 0, 0, loc), loc)
	if after != before+1 {
		t.Fatalf("a meia-noite de Brasília não vira o dia: %d → %d", before, after)
	}
	var s DailyState
	if !s.Refresh(9, before, 12) || len(s.Offer) != 3 {
		t.Fatalf("primeiro sorteio: %+v", s)
	}
	s.Done = []string{s.Offer[0]}
	if s.Refresh(9, before, 30) || len(s.Done) != 1 {
		t.Fatal("o mesmo dia não sorteia de novo (nem para quem subiu de nível)")
	}
	if !s.Refresh(9, after, 12) || len(s.Done) != 0 || s.Day != after {
		t.Fatalf("a virada não limpou: %+v", s)
	}
	low := DailyState{}
	low.Refresh(9, after, 3)
	if len(low.Offer) != 0 || !low.Refresh(9, after, 5) || len(low.Offer) != 3 {
		t.Fatalf("quem chega ao 5 no mesmo dia ganha o rol: %+v", low)
	}
	high := DailyState{}
	high.Refresh(9, after, 38)
	if high.Refresh(9, after, 38) {
		t.Fatal("acima do 35 o rol vazio não sorteia de novo a cada chamada")
	}
	n := NormalDaily(&DailyState{Day: 3, Offer: []string{"d_ossos", "tut_gate", "nao_existe", "d_ossos"}, Done: []string{"arr_trigo"}})
	if !slices.Equal(n.Offer, []string{"d_ossos"}) || len(n.Done) != 0 {
		t.Fatalf("NormalDaily: %+v", n)
	}
}

// A XP das diárias é pouca: as 3 do dia valem no máximo 3 minutos de caça no nível.
func TestDailyXPIsSmall(t *testing.T) {
	for l := DailyMinLevel; l <= DailyMaxLevel; l++ {
		for _, q := range DailyQuests(1, 20000, l) {
			xp := q.Reward.XPAt(l)
			kills := float64(xp) / HuntKillXP(l)
			if xp <= 0 || kills > DailyXPKills+0.01 || q.Reward.Gold <= 0 || q.Reward.Rep <= 0 {
				t.Errorf("nível %d, %s: %d XP (%.1f abates), %d moedas, %d de reputação", l, q.ID, xp, kills, q.Reward.Gold, q.Reward.Rep)
			}
		}
	}
	if perDay := 3 * DailyXPKills * SecondsPerKill / 60; perDay > 3 {
		t.Fatalf("as diárias valem %.1f min de caça por dia", perDay)
	}
}

// Os NPCs novos têm nome único e as falas da conversa.
func TestTalkers(t *testing.T) {
	seen := map[string]bool{}
	for _, tk := range Talkers {
		if seen[strings.ToLower(tk.Name)] {
			t.Errorf("%s: nome repetido", tk.Name)
		}
		seen[strings.ToLower(tk.Name)] = true
		if tk.Title == "" || tk.Greeting == "" || len(tk.Topics) == 0 || tk.Idle == "" {
			t.Errorf("%s: falta título, saudação, assunto ou fala de quando não há o que oferecer", tk.Name)
		}
		if tk.Spawn != nil && (tk.Service != "" || tk.Busy == "" || tk.Accepted == "" || tk.Spawn.Model == "" || tk.Spawn.Look != nil && !ValidAppearance(*tk.Spawn.Look)) {
			t.Errorf("%s: NPC novo sem serviço, com fala de missão em andamento e modelo", tk.Name)
		}
		for _, tp := range tk.Topics {
			if tp.ID == "" || tp.Label == "" || tp.Text == "" || tk.Topic(tp.ID) != &tk.Topics[slices.IndexFunc(tk.Topics, func(x TalkTopic) bool { return x.ID == tp.ID })] {
				t.Errorf("%s: assunto %+v", tk.Name, tp)
			}
		}
	}
}

package game

import "testing"

func TestAreaEventsAreValid(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range AreaEvents {
		if ids[e.ID] {
			t.Errorf("evento repetido: %s", e.ID)
		}
		ids[e.ID] = true
		if err := ValidateEvent(e); err != nil {
			t.Error(err)
		}
	}
	b := AreaEventByID("bastiao_resiste")
	if b == nil || !b.Enabled || len(b.Stages) != 3 || b.Post != "bastiao" || b.CloseSecs != 15*60 {
		t.Fatalf("O Bastião resiste: %+v", b)
	}
	kinds := []StageKind{StageKill, StageDefend, StageBoss}
	for i, s := range b.Stages {
		if s.Kind != kinds[i] {
			t.Errorf("etapa %d é %v", i+1, s.Kind)
		}
	}
	// A pedra do posto existe com a mesma chave (zone/teleport.go) e o Farol fica desligado até a Costa.
	if p := PostByKey("bastiao"); p == nil || TalkerByName(p.Talker) == nil {
		t.Fatalf("posto do Bastião: %+v", p)
	}
	if f := AreaEventByID("acender_farol"); f == nil || f.Enabled {
		t.Fatal("Acender o Farol fica desligado até a Costa entrar no mapa")
	}
	if Items[BastionMedal] == nil {
		t.Fatal("falta o item da medalha")
	}
}

func TestRewardTiers(t *testing.T) {
	r := AreaEventByID("bastiao_resiste").Reward
	cases := []struct {
		points float64
		tier   int
		medals int
		share  float64
	}{
		{1000, MedalGold, 3, 1},
		{r.GoldAt * 1000, MedalGold, 3, 1},
		{r.SilverAt * 1000, MedalSilver, 2, 0.85},
		{r.BronzeAt * 1000, MedalBronze, 1, 0.75},
		{r.BronzeAt*1000 - 1, MedalNone, 0, 0},
		{0, MedalNone, 0, 0},
	}
	for _, c := range cases {
		tier := r.Tier(c.points, 1000)
		if tier != c.tier || r.MedalCount(tier) != c.medals || MedalShare[tier] != c.share {
			t.Errorf("%.1f pontos: faixa %d, %d medalhas, %.2f", c.points, tier, r.MedalCount(tier), MedalShare[tier])
		}
	}
	if r.Tier(10, 0) != MedalNone {
		t.Error("sem vida total não há faixa")
	}
}

func TestValidateEventRejectsBadData(t *testing.T) {
	good := func() *EventDef { c := *AreaEventByID("bastiao_resiste"); return &c }
	bads := map[string]func(*EventDef){
		"sem etapas": func(e *EventDef) { e.Stages = nil },
		"monstro ruim": func(e *EventDef) {
			e.Stages = []StageDef{{Kind: StageKill, Label: "x", Limit: 10, Mobs: []string{"nao_existe"}, Count: 1, MaxAlive: 1}}
		},
		"chance ruim":     func(e *EventDef) { e.Trigger.Chance = 2 },
		"faixas":          func(e *EventDef) { e.Reward.SilverAt = e.Reward.GoldAt },
		"posto sem tempo": func(e *EventDef) { e.CloseSecs = 0 },
		"segurar demais": func(e *EventDef) {
			e.Stages = []StageDef{{Kind: StageDefend, Label: "x", Limit: 10, Mobs: []string{"skeleton_minion"}, MaxAlive: 1, Hold: 20, TargetHP: 1, DrainRate: 1}}
		},
	}
	for name, mutate := range bads {
		e := good()
		mutate(e)
		if ValidateEvent(e) == nil {
			t.Errorf("%s: deveria ser inválido", name)
		}
	}
	if err := ValidateEvent(good()); err != nil {
		t.Fatal(err)
	}
}

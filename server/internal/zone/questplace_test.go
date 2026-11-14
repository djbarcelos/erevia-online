package zone

import (
	"io"
	"log/slog"
	"math"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// newQuestZone: a zona com os spawns e o raio seguro de verdade.
func newQuestZone() *Zone {
	cfg := testConfig()
	cfg.Spawns = game.DefaultSpawns()
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func (f *fakePeer) questLogCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.msgs {
		if m.GetQuestLog() != nil {
			n++
		}
	}
	return n
}

func TestPlaceKindsMatchTheProtocol(t *testing.T) {
	for _, c := range []struct {
		g game.PlaceKind
		p pb.QuestPlaceKind
	}{
		{game.PlaceNone, pb.QuestPlaceKind_QUEST_PLACE_KIND_NONE}, {game.PlaceHunt, pb.QuestPlaceKind_QUEST_PLACE_KIND_HUNT},
		{game.PlaceNPC, pb.QuestPlaceKind_QUEST_PLACE_KIND_NPC}, {game.PlaceGate, pb.QuestPlaceKind_QUEST_PLACE_KIND_GATE},
		{game.PlaceStone, pb.QuestPlaceKind_QUEST_PLACE_KIND_STONE}, {game.PlaceShop, pb.QuestPlaceKind_QUEST_PLACE_KIND_SHOP},
	} {
		if pb.QuestPlaceKind(c.g) != c.p {
			t.Errorf("game place %d maps to %v, want %v", c.g, pb.QuestPlaceKind(c.g), c.p)
		}
	}
}

// Todas as missões atuais: cada passo pendente tem o lugar do tipo certo, dentro do mapa.
func TestEveryQuestStepHasThePlaceOfItsGoal(t *testing.T) {
	want := map[game.QuestGoal]pb.QuestPlaceKind{
		game.GoalLeaveTown: pb.QuestPlaceKind_QUEST_PLACE_KIND_GATE,
		game.GoalKill:      pb.QuestPlaceKind_QUEST_PLACE_KIND_HUNT,
		game.GoalSkill:     pb.QuestPlaceKind_QUEST_PLACE_KIND_NONE,
		game.GoalPotion:    pb.QuestPlaceKind_QUEST_PLACE_KIND_NONE,
		game.GoalBuy:       pb.QuestPlaceKind_QUEST_PLACE_KIND_SHOP,
		game.GoalLevel:     pb.QuestPlaceKind_QUEST_PLACE_KIND_HUNT,
		game.GoalTeleport:  pb.QuestPlaceKind_QUEST_PLACE_KIND_STONE,
		game.GoalTalk:      pb.QuestPlaceKind_QUEST_PLACE_KIND_NPC,
		game.GoalSpec:      pb.QuestPlaceKind_QUEST_PLACE_KIND_NPC,
	}
	z := newQuestZone()
	p, _ := joinAt(z, 1, "knight", 3, -2, 0)
	for id, q := range game.Quests {
		p.level = max(1, q.Unlock)
		for i, s := range q.Steps {
			k, ok := want[s.Goal]
			if !ok {
				t.Fatalf("%s step %d: goal %d without an expected place", id, i, s.Goal)
			}
			info := z.stepInfo(p, s, 0)
			if info.Place != k {
				t.Errorf("%s step %d (%s): place %v, want %v", id, i, s.Text, info.Place, k)
				continue
			}
			if k == pb.QuestPlaceKind_QUEST_PLACE_KIND_NONE {
				continue
			}
			if info.Radius <= 0 || info.PlaceName == "" || math.Abs(float64(info.X)) > 150 || info.Z > 150 || info.Z < -330 {
				t.Errorf("%s step %d: bad place %+v", id, i, info)
			}
			if done := z.stepInfo(p, s, s.Count); done.Place != pb.QuestPlaceKind_QUEST_PLACE_KIND_NONE {
				t.Errorf("%s step %d: a finished step must not carry a place", id, i)
			}
		}
	}
}

func TestQuestPlacesPointAtTheRightThings(t *testing.T) {
	z := newQuestZone()
	p, _ := joinAt(z, 1, "knight", 0, -20, 0)
	step := func(id string, i int) *pb.QuestStepInfo { return z.stepInfo(p, game.Quests[id].Steps[i], 0) }

	if g := step("tut_gate", 0); g.PlaceName != "Portão Sul" || g.X != 0 || g.Z != -townGateR {
		t.Fatalf("south of the well the nearest gate is the south one, got %+v", g)
	}
	p.pos = Vec2{X: 30, Z: 4}
	if g := step("tut_gate", 0); g.PlaceName != "Portão Leste" {
		t.Fatalf("east: got %+v", g)
	}

	hunt := step("tut_hunt", 0)
	if r := math.Hypot(float64(hunt.X), float64(hunt.Z)); r < 46 || r > 74 || hunt.PlaceName != "Esqueleto Lacaio" {
		t.Fatalf("minion hunt = %+v (r = %.1f)", hunt, r)
	}
	if hunt.X <= 0 || math.Abs(float64(hunt.Z)/float64(hunt.X)-4.0/30) > 1e-3 {
		t.Fatalf("hunt circle should be toward the player (east), got (%v, %v)", hunt.X, hunt.Z)
	}

	// Loja: a mais perto (Lívia no leste da praça, Bruno no oeste).
	p.pos = Vec2{X: 10, Z: -4}
	if s := step("tut_shop", 0); s.PlaceName != "Lívia" {
		t.Fatalf("east shop = %+v", s)
	}
	p.pos = Vec2{X: -12, Z: -4}
	if s := step("tut_shop", 0); s.PlaceName != "Bruno" {
		t.Fatalf("west shop = %+v", s)
	}
	// Comprar um item que só o Ferreiro vende leva a ele mesmo do leste.
	p.pos = Vec2{X: 10, Z: -4}
	smith := game.SmithStock[0]
	if s := z.stepInfo(p, game.QuestStep{Goal: game.GoalBuy, Target: smith, Count: 1, Text: "x"}, 0); !sells(z, s.PlaceName, smith) {
		t.Fatalf("buying %s: %+v does not sell it", smith, s)
	}

	stone := step("tut_stone", 0)
	if stone.PlaceName != "Pedra de Teleporte" || math.Hypot(float64(stone.X)-7, float64(stone.Z)-7) > 3 {
		t.Fatalf("town stone = %+v", stone)
	}
	talk := step(game.SpecQuest, 0)
	if talk.PlaceName != master.name || math.Hypot(float64(talk.X)-master.x, float64(talk.Z)-master.z) > 3 {
		t.Fatalf("talk place = %+v, want Ysolde", talk)
	}
	if spec := step(game.SpecQuest, 1); spec.PlaceName != master.name {
		t.Fatalf("choosing the path happens at Ysolde, got %+v", spec)
	}
	if s := step("tut_skill", 0); s.Place != pb.QuestPlaceKind_QUEST_PLACE_KIND_NONE {
		t.Fatalf("skill step has a place: %+v", s)
	}
}

func sells(z *Zone, name, item string) bool {
	for _, n := range z.npcs {
		if n.name != name {
			continue
		}
		for _, d := range n.npc.stock {
			if d.ID == item {
				return true
			}
		}
	}
	return false
}

func TestQuestPlaceCanBeFixedInTheData(t *testing.T) {
	z := newQuestZone()
	p, _ := joinAt(z, 1, "knight", 0, 0, 0)
	s := game.QuestStep{Goal: game.GoalKill, Target: "skeleton_minion", Count: 1, Text: "x",
		Place: &game.QuestPlace{Kind: game.PlaceNPC, X: -40, Z: 90, Radius: 5, Name: "Ermitão"}}
	info := z.stepInfo(p, s, 0)
	if info.Place != pb.QuestPlaceKind_QUEST_PLACE_KIND_NPC || info.X != -40 || info.Z != 90 || info.Radius != 5 || info.PlaceName != "Ermitão" {
		t.Fatalf("fixed place = %+v", info)
	}
}

func TestQuestLogSendsPlacesAndRefreshesOnLevelUp(t *testing.T) {
	z := newQuestZone()
	p, peer := joinAt(z, 1, "knight", 0, -20, 0)
	if s := peer.lastQuestLog().Quests[0].Steps[0]; s.Place != pb.QuestPlaceKind_QUEST_PLACE_KIND_GATE || s.PlaceName != "Portão Sul" {
		t.Fatalf("join quest log step = %+v", s)
	}

	// Caçar guerreiros e ladinos: a área anda para fora do anel com o nível.
	p.pl.quests.active = []*activeQuest{{def: game.Quests["tut_bones"], progress: []int{0}}}
	z.grantXP(p, game.XPLevelStart(6)-p.pl.xp)
	before := peer.lastQuestLog().Quests[0].Steps[0]
	n := peer.questLogCount()
	z.grantXP(p, game.XPLevelStart(10)-p.pl.xp)
	if peer.questLogCount() <= n {
		t.Fatal("level up with a hunt step pending must resend the quest log")
	}
	after := peer.lastQuestLog().Quests[0].Steps[0]
	if math.Hypot(float64(after.X), float64(after.Z)) <= math.Hypot(float64(before.X), float64(before.Z)) {
		t.Fatalf("hunt circle should move outward with the level: %+v → %+v", before, after)
	}

	// Sem objetivo que dependa do nível, subir não reenvia (nada de tráfego à toa).
	p.pl.quests.active = []*activeQuest{{def: game.Quests["tut_skill"], progress: []int{0}}}
	for l := 9; l <= 11; l++ { // os contratos do 9 ao 11 chegariam e reenviariam
		p.pl.quests.done[game.ContractID(l)] = true
	}
	n = peer.questLogCount()
	z.grantXP(p, game.XPLevelStart(11)-p.pl.xp)
	if peer.questLogCount() != n {
		t.Fatal("level up without level-bound places must not resend the quest log")
	}

	// O passo muda: o lugar vem junto (só os pendentes).
	p.pl.quests.active = []*activeQuest{{def: game.Quests[game.SpecQuest], progress: []int{0, 0}}}
	z.questEvent(p, game.GoalTalk, game.TalkMaster)
	steps := peer.lastQuestLog().Quests[0].Steps
	if steps[0].Place != pb.QuestPlaceKind_QUEST_PLACE_KIND_NONE || steps[1].Place != pb.QuestPlaceKind_QUEST_PLACE_KIND_NPC {
		t.Fatalf("after talking: %+v", steps)
	}
}

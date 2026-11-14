package game

import (
	"slices"
	"strings"
	"testing"
)

func TestQuestDataIsConsistent(t *testing.T) {
	if Quests[TutorialStart] == nil {
		t.Fatalf("tutorial start %q missing", TutorialStart)
	}
	seen := map[string]bool{}
	for id := TutorialStart; id != ""; id = Quests[id].Next {
		if seen[id] {
			t.Fatalf("quest chain loops at %q", id)
		}
		seen[id] = true
		if Quests[id] == nil {
			t.Fatalf("chain points to missing quest %q", id)
		}
	}
	chained := 0
	for _, q := range Quests {
		if q.Unlock == 0 {
			chained++
		} else if q.Unlock < 1 || q.Unlock > MaxLevel || q.Next != "" && Quests[q.Next] == nil {
			t.Errorf("%s: unlock level %d or next %q is wrong", q.ID, q.Unlock, q.Next)
		}
	}
	if len(seen) != chained {
		t.Fatalf("tutorial chain reaches %d of %d quests", len(seen), chained)
	}
	for _, q := range Quests {
		if q.Title == "" || q.Text == "" || len(q.Steps) == 0 {
			t.Errorf("%s: title, text and steps are required", q.ID)
		}
		for _, s := range q.Steps {
			if s.Count < 1 || s.Text == "" {
				t.Errorf("%s: step %+v needs count and text", q.ID, s)
			}
			if s.Goal == GoalTalk && s.Target != TalkMaster {
				t.Errorf("%s: unknown talk target %q", q.ID, s.Target)
			}
			if s.Goal == GoalKill && s.Target != "" {
				for _, m := range strings.Split(s.Target, "|") {
					if Monsters[m] == nil {
						t.Errorf("%s: unknown monster %q", q.ID, m)
					}
				}
			}
		}
		for _, it := range q.Reward.Items {
			if Items[it.ID] == nil || it.Count < 1 {
				t.Errorf("%s: bad reward item %+v", q.ID, it)
			}
		}
	}
}

func TestQuestStepMatches(t *testing.T) {
	s := QuestStep{Goal: GoalKill, Target: "skeleton_warrior|skeleton_rogue", Count: 5}
	if !s.Matches(GoalKill, "skeleton_rogue") || s.Matches(GoalKill, "skeleton_minion") || s.Matches(GoalSkill, "skeleton_rogue") {
		t.Fatal("target list matching is wrong")
	}
	if !(QuestStep{Goal: GoalBuy, Count: 1}).Matches(GoalBuy, "potion_hp_s") {
		t.Fatal("empty target should match anything of the same goal")
	}
}

// Contratos de caça: um por nível do 5 ao 39, nos monstros do nível, pagando 30% da XP dele,
// e só para quem ainda está perto do nível (Until).
func TestContracts(t *testing.T) {
	for l := ContractFrom; l < MaxLevel; l++ {
		q := Quests[ContractID(l)]
		if q == nil {
			t.Fatalf("sem contrato no %d", l)
		}
		if q.Unlock != l || q.Until != l+ContractWindow || q.Reward.XP <= 0 || q.Reward.XP >= XPToNext(l) {
			t.Errorf("%s: %+v", q.ID, q)
		}
		hunted := huntMonsters(l)
		for _, m := range q.Steps[0].Monsters() {
			if !slices.Contains(hunted, m) {
				t.Errorf("%s pede %s, que não se caça no %d", q.ID, m, l)
			}
		}
	}
	if Quests[ContractID(ContractFrom-1)] != nil || Quests[ContractID(MaxLevel)] != nil {
		t.Fatal("contrato fora da faixa")
	}
}

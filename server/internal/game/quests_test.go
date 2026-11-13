package game

import (
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
	if len(seen) != len(Quests) {
		t.Fatalf("tutorial chain reaches %d of %d quests", len(seen), len(Quests))
	}
	for _, q := range Quests {
		if q.Title == "" || q.Text == "" || len(q.Steps) == 0 {
			t.Errorf("%s: title, text and steps are required", q.ID)
		}
		for _, s := range q.Steps {
			if s.Count < 1 || s.Text == "" {
				t.Errorf("%s: step %+v needs count and text", q.ID, s)
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

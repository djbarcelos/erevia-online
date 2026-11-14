package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

func (f *fakePeer) lastQuestLog() *pb.QuestLog {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if q := f.msgs[i].GetQuestLog(); q != nil {
			return q
		}
	}
	return nil
}

func activeIDs(p *entity) []string {
	var ids []string
	for _, a := range p.pl.quests.active {
		ids = append(ids, a.def.ID)
	}
	return ids
}

func TestNewCharacterStartsTutorialAndVeteransDoNot(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "knight", 0, 0, 0)
	if ids := activeIDs(p); len(ids) != 1 || ids[0] != game.TutorialStart {
		t.Fatalf("new character quests = %v", ids)
	}
	log := peer.lastQuestLog()
	if log == nil || len(log.Quests) != 1 || log.Quests[0].Steps[0].Need != 1 {
		t.Fatalf("join should send the quest log, got %+v", log)
	}

	// O veterano não recebe o tutorial, só o contrato de caça do nível dele.
	vet, _ := joinAt(z, 2, "mage", 0, 0, game.XPLevelStart(game.TutorialSkipLevel))
	if ids := activeIDs(vet); len(ids) != 1 || ids[0] != game.ContractID(game.TutorialSkipLevel) {
		t.Fatalf("veteran without history got %v", ids)
	}
}

func TestLeavingTownCompletesFirstQuestAndStartsNext(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "knight", 0, 0, 0)
	z.tickFor(1.05)
	if activeIDs(p)[0] != game.TutorialStart {
		t.Fatal("still in town: quest must not advance")
	}

	p.teleportTo(Vec2{X: 30, Z: 0}) // fora do raio seguro (20 m nos testes de combate)
	z.tickFor(1.05)
	if ids := activeIDs(p); len(ids) != 1 || ids[0] != "tut_hunt" {
		t.Fatalf("after leaving town quests = %v", ids)
	}
	if p.pl.xp != game.Quests[game.TutorialStart].Reward.XP {
		t.Fatalf("xp = %d, want the quest reward", p.pl.xp)
	}
	log := peer.lastQuestLog()
	if log.Completed != game.Quests[game.TutorialStart].Title || log.Quests[0].Id != "tut_hunt" {
		t.Fatalf("quest log = %+v", log)
	}
	if !peer.systemContains("Missão concluída") || !peer.systemContains("Nova missão") {
		t.Fatal("expected completion and new quest messages")
	}
}

func TestKillingMonstersAdvancesHuntQuest(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 1, 25, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "barbarian", 23.5, 0, 0)
	p.pl.quests.active = []*activeQuest{{def: game.Quests["tut_hunt"], progress: []int{2}}}
	gold := p.pl.inv.gold

	m.hp = 1
	z.handle(attackEvent{id: p.id, target: m.id})
	for i := 0; i < 200 && !m.dead; i++ {
		z.update(0.05)
	}
	if !m.dead {
		t.Fatal("monster should die")
	}
	if ids := activeIDs(p); len(ids) != 1 || ids[0] != "tut_skill" {
		t.Fatalf("third kill should finish the hunt, quests = %v", ids)
	}
	if p.pl.inv.gold != gold+game.Quests["tut_hunt"].Reward.Gold {
		t.Fatalf("gold = %d, want +%d", p.pl.inv.gold, game.Quests["tut_hunt"].Reward.Gold)
	}
	if peer.lastQuestLog().Completed != "Ossos inquietos" {
		t.Fatalf("completed = %q", peer.lastQuestLog().Completed)
	}
}

func TestOnlyMatchingEventsCountAndProgressCaps(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "knight", 0, 0, 0)
	p.pl.quests.active = []*activeQuest{{def: game.Quests["tut_bones"], progress: []int{0}}}

	z.questEvent(p, game.GoalKill, "skeleton_minion") // não conta
	z.questEvent(p, game.GoalSkill, "")               // não conta
	z.questEvent(p, game.GoalKill, "skeleton_rogue")
	z.questEvent(p, game.GoalKill, "skeleton_warrior")
	if got := p.pl.quests.active[0].progress[0]; got != 2 {
		t.Fatalf("progress = %d, want 2", got)
	}
}

func TestLevelGoalAlreadyMetCompletesRightAway(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "knight", 0, 0, game.XPLevelStart(4))
	p.pl.quests.active = []*activeQuest{{def: game.Quests["tut_shop"], progress: []int{0}}}

	z.questEvent(p, game.GoalBuy, "potion_hp_s") // conclui tut_shop e começa tut_level (nível 4 já cumprido)
	if ids := activeIDs(p); len(ids) != 1 || ids[0] != "tut_stone" {
		t.Fatalf("level goal already met should skip ahead, quests = %v", ids)
	}
	if !p.pl.quests.done["tut_shop"] || !p.pl.quests.done["tut_level"] {
		t.Fatalf("done = %v", p.pl.quests.done)
	}
}

func TestRewardItemsGoToBagOrReservedOnGround(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "knight", 0, 0, 0)
	before := p.pl.inv.countOf("potion_mp_s")
	p.pl.quests.active = []*activeQuest{{def: game.Quests["tut_potion"], progress: []int{0}}}
	z.questEvent(p, game.GoalPotion, "potion_hp_s")
	if got := p.pl.inv.countOf("potion_mp_s"); got != before+2 {
		t.Fatalf("mana potions = %d, want %d", got, before+2)
	}

	// Bolsa cheia: o pergaminho vai para o chão, reservado para o jogador.
	filler := game.Items["scroll_ench_armor"]
	for i := range p.pl.inv.bag {
		if p.pl.inv.bag[i].empty() {
			p.pl.inv.bag[i] = stack{def: filler, count: filler.Stack}
		}
	}
	p.pl.quests.active = []*activeQuest{{def: game.Quests["tut_bones"], progress: []int{4}}}
	z.questEvent(p, game.GoalKill, "skeleton_warrior")
	var found *groundItem
	for _, g := range z.ground {
		if g.def.ID == "scroll_ench_weapon" {
			found = g
		}
	}
	if found == nil || found.owner != p.id {
		t.Fatalf("reward should be on the ground reserved for the player, got %+v", found)
	}
	if !peer.systemContains("bolsa está cheia") {
		t.Fatal("expected full-bag notice")
	}
}

func TestQuestProgressSurvivesRelog(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "knight", 0, 0, 0)
	p.pl.quests.active = []*activeQuest{{def: game.Quests["tut_skill"], progress: []int{2}}}
	p.pl.quests.done = map[string]bool{"tut_gate": true, "tut_hunt": true}
	st := z.savedState(p).Quests

	z2 := newCombatZone()
	peer := &fakePeer{}
	z2.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Pknight", Class: "knight", Quests: st, Peer: peer}})
	q := z2.players[1].pl.quests
	if len(q.active) != 1 || q.active[0].def.ID != "tut_skill" || q.active[0].progress[0] != 2 || !q.done["tut_hunt"] {
		t.Fatalf("restored quests = %+v", q)
	}
	if log := peer.lastQuestLog(); log.Quests[0].Steps[0].Count != 2 {
		t.Fatalf("restored log = %+v", log)
	}
}

// Contratos de caça: quem entra recebe só os do próprio nível (até ContractWindow para
// trás), e concluir paga a XP do contrato.
func TestContractsArriveNearTheLevel(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "knight", 0, 0, game.XPLevelStart(30))
	ids := activeIDs(p)
	want := []string{game.ContractID(28), game.ContractID(29), game.ContractID(30)}
	got := []string{}
	for _, id := range ids {
		if game.Quests[id].Unlock >= game.ContractFrom && id != game.SpecQuest {
			got = append(got, id)
		}
	}
	if len(got) != len(want) || got[0] != want[0] || got[2] != want[2] {
		t.Fatalf("contratos ao entrar no 30: %v", ids)
	}
	def := game.Quests[game.ContractID(30)]
	before := p.pl.xp
	for range def.Steps[0].Count {
		z.questEvent(p, game.GoalKill, def.Steps[0].Monsters()[0])
	}
	if !p.pl.quests.done[def.ID] || p.pl.xp-before != def.Reward.XP {
		t.Fatalf("contrato concluído: done %v, XP +%d (want %d)", p.pl.quests.done[def.ID], p.pl.xp-before, def.Reward.XP)
	}
}

package zone

import (
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// achMsgs devolve as mensagens de conquistas recebidas, em ordem.
func (f *fakePeer) achMsgs() []*pb.Achievements {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pb.Achievements
	for _, m := range f.msgs {
		if a := m.GetAchievements(); a != nil {
			out = append(out, a)
		}
	}
	return out
}

// systemCount conta os avisos do sistema que contêm s.
func (f *fakePeer) systemCount(s string) int {
	n := 0
	for _, c := range f.chats() {
		if c.Channel == pb.ChatChannel_CHAT_CHANNEL_SYSTEM && strings.Contains(c.Text, s) {
			n++
		}
	}
	return n
}

func (f *fakePeer) lastStatus(id uint32) *pb.PlayerStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if s := f.msgs[i].GetPlayerStatus(); s != nil && s.Id == id {
			return s
		}
	}
	return nil
}

func unlocked(p *entity, id string) bool {
	_, ok := p.pl.ach.unlocked[id]
	return ok
}

// killFixed mata o monstro m com o jogador p (golpe básico, monstro com 1 de vida).
func killFixed(t *testing.T, z *Zone, p *entity, m *entity) {
	t.Helper()
	m.hp = 1
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(attackEvent{id: p.id, target: m.id})
	for i := 0; i < 200 && !m.dead; i++ {
		z.update(0.05)
	}
	if !m.dead {
		t.Fatal("monster should die")
	}
	z.tickFor(0.1) // avisos que saem depois do snapshot
}

func TestJoinSendsFullCatalog(t *testing.T) {
	z := newCombatZone()
	_, peer := joinAt(z, 1, "knight", 0, 0, 0)
	msgs := peer.achMsgs()
	if len(msgs) == 0 || !msgs[0].Full || len(msgs[0].List) != len(game.AchievementList) {
		t.Fatalf("join should send the whole catalog first, got %+v", msgs)
	}
	first := msgs[0].List[0]
	if first.Name == "" || first.Title == "" || first.Text == "" || first.Unlocked {
		t.Fatalf("catalog entry = %+v", first)
	}
}

func TestKillUnlocksOnceAndCountsSkeletons(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 1, 25, 0), fixed("skeleton_minion", 1, 25, 2))
	var mons []*entity
	for _, m := range z.monsters {
		mons = append(mons, m)
	}
	p, peer := joinAt(z, 1, "barbarian", 23.5, 1, 0)

	killFixed(t, z, p, mons[0])
	if !unlocked(p, "first_kill") {
		t.Fatal("first kill should unlock first_kill")
	}
	if got := p.pl.ach.counts["bones_100"]; got != 1 {
		t.Fatalf("bones_100 progress = %d, want 1", got)
	}
	if peer.systemCount("Conquista desbloqueada: Primeiro Sangue") != 1 {
		t.Fatal("expected one unlock message in the chat")
	}
	var toast *pb.Achievements
	for _, a := range peer.achMsgs() {
		if a.Unlocked != "" {
			toast = a
		}
	}
	if toast == nil || toast.Unlocked != "Primeiro Sangue" || toast.Full {
		t.Fatalf("expected a partial update with the unlock, got %+v", toast)
	}

	killFixed(t, z, p, mons[1])
	if n := peer.systemCount("Conquista desbloqueada: Primeiro Sangue"); n != 1 {
		t.Fatalf("first_kill unlocked %d times", n)
	}
	if got := p.pl.ach.counts["bones_100"]; got != 2 {
		t.Fatalf("bones_100 progress = %d, want 2", got)
	}

	// A centésima caveira desbloqueia e para de contar.
	p.pl.ach.counts["bones_100"] = 99
	z.achEvent(p, game.AchKill, "skeleton_warrior")
	z.achEvent(p, game.AchKill, "stone_golem") // não é esqueleto
	if !unlocked(p, "bones_100") || p.pl.ach.counts["bones_100"] != 0 {
		t.Fatalf("bones_100 should unlock at 100, state %+v", p.pl.ach)
	}
}

func TestLevelAchievementsByEventAndRetroactiveOnJoin(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "knight", 0, 0, 0)
	z.grantXP(p, game.XPLevelStart(10))
	if p.level < 10 || !unlocked(p, "level_10") || unlocked(p, "level_20") {
		t.Fatalf("level %d: achievements %+v", p.level, p.pl.ach.unlocked)
	}

	// Personagem antigo, já no nível 20 e com vitória PvP: ganha ao entrar.
	vet, peer := joinAt(z, 2, "mage", 0, 0, game.XPLevelStart(20))
	vet.pl.pvpKills = 0
	if !unlocked(vet, "level_10") || !unlocked(vet, "level_20") || unlocked(vet, "level_30") {
		t.Fatalf("veteran achievements %+v", vet.pl.ach.unlocked)
	}
	z.tickFor(0.1)
	if peer.systemCount("Conquista desbloqueada") != 2 {
		t.Fatal("veteran should be told about both unlocks")
	}
}

func TestTradeQuestAndPvPHooks(t *testing.T) {
	z, a, b, _, _ := openTestTrade(t, nil)
	giveItem(a, 10, "scroll_ench_weapon", 1, 0)
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_ADD_ITEM, func(m *pb.TradeCmd) { m.Slot = 10 })
	lockAndConfirm(z, a, b)
	if !unlocked(a, "first_trade") || !unlocked(b, "first_trade") {
		t.Fatal("both sides of a trade should unlock first_trade")
	}

	z.finishQuest(a, game.Quests["tut_bones"])
	if !unlocked(a, "tutorial") {
		t.Fatal("finishing the tutorial should unlock it")
	}

	z.achValue(a, game.AchEnchant, "", 6)
	z.achValue(a, game.AchCraft, "", int(game.GradeEpic))
	if unlocked(a, "enchant_7") || unlocked(a, "craft_legend") {
		t.Fatal("+6 and an epic item are not enough")
	}
	z.achValue(a, game.AchEnchant, "", 7)
	z.achValue(a, game.AchCraft, "", int(game.GradeLegendary))
	if !unlocked(a, "enchant_7") || !unlocked(a, "craft_legend") {
		t.Fatal("+7 and a legendary item should unlock")
	}
}

func TestChosenTitleIsSeenByEveryone(t *testing.T) {
	z := newTestZone(nil)
	a, b := join(z, 1, "Ana"), join(z, 2, "Beto")
	ana := z.players[1]

	z.handle(socialEvent{id: 1, msg: &pb.AchievementCmd{Kind: pb.AchievementCmdKind_ACHIEVEMENT_CMD_SET_TITLE, Id: "first_kill"}})
	if ana.pl.ach.title != "" || !a.systemContains("ainda não tem") {
		t.Fatal("a locked title must be refused")
	}

	z.achEvent(ana, game.AchKill, "skeleton_minion")
	ana.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(socialEvent{id: 1, msg: &pb.AchievementCmd{Kind: pb.AchievementCmdKind_ACHIEVEMENT_CMD_SET_TITLE, Id: "first_kill"}})
	want := game.Achievements["first_kill"].Title
	for _, peer := range []*fakePeer{a, b} {
		if s := peer.lastStatus(1); s == nil || s.Title != want {
			t.Fatalf("everyone should get the new title, got %+v", s)
		}
	}
	if msgs := a.achMsgs(); msgs[len(msgs)-1].TitleId != "first_kill" {
		t.Fatal("owner should get the chosen title id")
	}

	// Quem chega depois vê o título no Welcome.
	c := join(z, 3, "Caio")
	var seen string
	for _, m := range c.msgs {
		if w := m.GetWelcome(); w != nil {
			for _, e := range w.Entities {
				if e.Id == 1 {
					seen = e.Title
				}
			}
		}
	}
	if seen != want {
		t.Fatalf("late joiner sees title %q, want %q", seen, want)
	}

	// Tirar o título.
	ana.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(socialEvent{id: 1, msg: &pb.AchievementCmd{Kind: pb.AchievementCmdKind_ACHIEVEMENT_CMD_SET_TITLE}})
	if s := b.lastStatus(1); s.Title != "" {
		t.Fatalf("title should be cleared, got %q", s.Title)
	}
}

func TestAchievementsSurviveRelogin(t *testing.T) {
	saves := make(chan []SavedState, 16)
	z := newTestZone(saves)
	join(z, 1, "Ana")
	ana := z.players[1]
	z.achEvent(ana, game.AchKill, "skeleton_minion")
	ana.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(socialEvent{id: 1, msg: &pb.AchievementCmd{Kind: pb.AchievementCmdKind_ACHIEVEMENT_CMD_SET_TITLE, Id: "first_kill"}})
	z.handle(leaveEvent{id: 1, peer: ana.pl.peer})

	var saved *SavedState
	for len(saves) > 0 {
		for _, s := range <-saves {
			if s.ID == 1 {
				saved = &s
			}
		}
	}
	if saved == nil || saved.Title != "first_kill" || saved.Achievements.Unlocked["first_kill"] == 0 ||
		saved.Achievements.Counts["bones_100"] != 1 {
		t.Fatalf("saved state = %+v", saved)
	}

	// Volta pela memória (estado recente) na mesma zona.
	peer := join(z, 1, "Ana")
	again := z.players[1]
	if !unlocked(again, "first_kill") || again.pl.ach.title != "first_kill" || again.pl.ach.counts["bones_100"] != 1 {
		t.Fatalf("relogin lost achievements: %+v", again.pl.ach)
	}
	z.achEvent(again, game.AchKill, "skeleton_minion")
	z.tickFor(0.1)
	if peer.systemCount("Conquista desbloqueada") != 0 {
		t.Fatal("an unlocked achievement must not unlock again")
	}

	// Volta pelo banco (outra zona, com o que foi salvo).
	z2 := newTestZone(nil)
	p2 := &fakePeer{}
	z2.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Ana", Class: "knight", Peer: p2,
		Achievements: saved.Achievements, Title: saved.Title}})
	if e := z2.entityPB(z2.players[1]); e.Title != game.Achievements["first_kill"].Title {
		t.Fatalf("title after loading from the database = %q", e.Title)
	}
	// Título de conquista que não está desbloqueada é ignorado.
	z3 := newTestZone(nil)
	z3.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Ana", Class: "knight", Peer: &fakePeer{}, Title: "level_30"}})
	if z3.players[1].pl.ach.title != "" {
		t.Fatal("a title without the achievement must be dropped")
	}
}

func TestRaidParticipantsUnlockRegicide(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	z, boss := newRaidZone(&now)
	a, _ := raider(z, 1, "knight", boss.pos.X+2, boss.pos.Z)
	b, _ := raider(z, 2, "mage", boss.pos.X-2, boss.pos.Z)
	idle, _ := raider(z, 3, "rogue", boss.pos.X, boss.pos.Z+3)
	boss.mon.raid.byPlayer[a.id] = 5000
	boss.mon.raid.byPlayer[b.id] = 10 // pouco dano (sem XP), mas participou
	z.kill(boss, a)
	if !unlocked(a, "raid_ozrhal") || !unlocked(b, "raid_ozrhal") || unlocked(idle, "raid_ozrhal") {
		t.Fatalf("raid achievement: a=%v b=%v idle=%v", unlocked(a, "raid_ozrhal"), unlocked(b, "raid_ozrhal"), unlocked(idle, "raid_ozrhal"))
	}
}

func TestRankingRequestUsesCacheAndOwnPosition(t *testing.T) {
	z := newTestZone(nil)
	a := join(z, 1, "Ana")
	z.handle(socialEvent{id: 1, msg: &pb.RankingRequest{}})
	var got *pb.Ranking
	for _, m := range a.msgs {
		if r := m.GetRanking(); r != nil {
			got = r
		}
	}
	if got == nil || got.UpdatedAtMs != 0 || len(got.Level) != 0 {
		t.Fatalf("no ranking yet should answer empty, got %+v", got)
	}

	at := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	z.handle(rankingEvent{data: RankingData{
		At:       at,
		Level:    []RankRow{{CharacterID: 7, Name: "Topo", Class: "mage", Level: 20, TitleID: "level_20"}},
		LevelPos: map[int64]int{7: 1, 1: 42},
		PvPPos:   map[int64]int{},
		ClanPos:  map[int64]int{},
	}})
	z.handle(socialEvent{id: 1, msg: &pb.RankingRequest{}}) // cedo demais: ignorado
	n := 0
	for _, m := range a.msgs {
		if m.GetRanking() != nil {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("requests must be spaced, got %d answers", n)
	}
	z.tickFor(rankingGap + 0.1)
	z.players[1].pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(socialEvent{id: 1, msg: &pb.RankingRequest{}})
	for _, m := range a.msgs {
		if r := m.GetRanking(); r != nil {
			got = r
		}
	}
	if got.UpdatedAtMs != at.UnixMilli() || got.MyLevelPos != 42 || got.MyPvpPos != 0 || len(got.Level) != 1 ||
		got.Level[0].Title != "Veterano" || got.Level[0].Pos != 1 {
		t.Fatalf("ranking = %+v", got)
	}
}

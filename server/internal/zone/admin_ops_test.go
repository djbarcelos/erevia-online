package zone

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// runZone roda o loop da zona até o fim do teste (as ações públicas passam pelo canal).
func runZone(t *testing.T, z *Zone) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { z.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
}

func TestScheduledAnnouncementsGoOutOnTime(t *testing.T) {
	now := noon
	z := newTestZone(nil)
	z.cfg.Clock = func() time.Time { return now }
	z.LoadScheduled([]ScheduledAnnouncement{
		{ID: 2, At: now.Add(10 * time.Minute), Title: "Depois", Text: "segundo"},
		{ID: 1, At: now.Add(5 * time.Minute), Title: "Antes", Text: "primeiro"},
	})
	peer := join(z, 1, "Ana")
	z.tickFor(1)
	if len(peer.announces()) != 0 {
		t.Fatal("nothing goes out before the time")
	}
	now = now.Add(6 * time.Minute)
	z.tickFor(1)
	a := peer.announces()
	if len(a) != 1 || a[0].Title != "Antes" || a[0].Text != "primeiro" {
		t.Fatalf("announces = %+v", a)
	}
	if s := z.opsStatus().Scheduled; len(s) != 1 || s[0].ID != 2 {
		t.Fatalf("still scheduled = %+v", s)
	}
}

func TestScheduleAndCancelThroughTheZone(t *testing.T) {
	z := newTestZone(nil)
	now := time.Now()
	z.cfg.Clock = func() time.Time { return now }
	runZone(t, z)
	ctx := context.Background()
	if err := z.AdminSchedule(ctx, ScheduledAnnouncement{ID: 5, At: now.Add(time.Hour), Text: "  olá\x00 "}); err != nil {
		t.Fatal(err)
	}
	var inv *AdminInvalid
	if err := z.AdminSchedule(ctx, ScheduledAnnouncement{ID: 6, At: now.Add(-time.Minute), Text: "passado"}); !errors.As(err, &inv) {
		t.Fatalf("past time = %v", err)
	}
	if err := z.AdminSchedule(ctx, ScheduledAnnouncement{ID: 7, At: now.Add(time.Hour), Text: "  "}); !errors.As(err, &inv) {
		t.Fatalf("empty text = %v", err)
	}
	st, err := z.AdminOps(ctx)
	if err != nil || len(st.Scheduled) != 1 || st.Scheduled[0].Title != "Anúncio" || st.Scheduled[0].Text != "olá" {
		t.Fatalf("scheduled = %+v, %v", st.Scheduled, err)
	}
	if ok, err := z.AdminUnschedule(ctx, 5); !ok || err != nil {
		t.Fatalf("unschedule = %v, %v", ok, err)
	}
	if ok, _ := z.AdminUnschedule(ctx, 5); ok {
		t.Fatal("unscheduling twice finds nothing")
	}
}

func TestMaintenanceCountsDownForEveryone(t *testing.T) {
	now := noon
	z := newTestZone(nil)
	z.cfg.Clock = func() time.Time { return now }
	peer := join(z, 1, "Ana")
	runZone(t, z)
	at, err := z.AdminMaintenance(context.Background(), 12, "Volta em 5 min.", "GM")
	if err != nil || !at.Equal(now.Add(12*time.Minute)) {
		t.Fatalf("maintenance = %v, %v", at, err)
	}
	if _, err := z.AdminMaintenance(context.Background(), 0, "", "GM"); err == nil {
		t.Fatal("0 minutes is invalid")
	}
	st, _ := z.AdminOps(context.Background())
	if st.Maintenance == nil || st.Maintenance.Message != "Volta em 5 min." {
		t.Fatalf("status = %+v", st.Maintenance)
	}
	texts := func() []string {
		var out []string
		for _, a := range peer.announces() {
			out = append(out, a.Text)
		}
		return out
	}
	if a := texts(); len(a) != 1 || !strings.Contains(a[0], "12 minutos") {
		t.Fatalf("first warning = %v", a)
	}
	// As marcas (10, 5, 3, 2, 1 min…) repetem o aviso; o que já passou não sai de novo.
	step := func(d time.Duration) {
		done := make(chan struct{})
		z.events <- socialDone{fn: func() { now = now.Add(d); z.updateAdminOps(); close(done) }}
		<-done
	}
	step(2*time.Minute + time.Second) // faltam 9:59
	step(time.Second)
	if a := texts(); len(a) != 2 || !strings.Contains(a[1], "10 minutos") {
		t.Fatalf("10 min warning = %v", a)
	}
	// Quem entra durante a contagem é avisado.
	late := &fakePeer{}
	z.Join(context.Background(), JoinInfo{ID: 2, Name: "Bia", Class: "mage", Peer: late, LoadedAt: time.Now()})
	z.Status(context.Background()) // espera o Join passar
	if !strings.Contains(lastChatContaining(late, "Manutenção"), "reinicia em") {
		t.Fatal("a late joiner is told about the restart")
	}
	step(10 * time.Minute)
	if a := texts(); !strings.Contains(a[len(a)-1], "reiniciar agora") {
		t.Fatalf("last warning = %v", a)
	}
	st, _ = z.AdminOps(context.Background())
	if st.Maintenance != nil {
		t.Fatal("the countdown ends at zero")
	}
	// Cancelar avisa todos.
	z.AdminMaintenance(context.Background(), 30, "", "GM")
	if ok, err := z.AdminMaintenanceCancel(context.Background()); !ok || err != nil {
		t.Fatalf("cancel = %v %v", ok, err)
	}
	if an := peer.announces(); !strings.Contains(an[len(an)-1].Title, "cancelada") {
		t.Fatalf("cancel warning = %+v", an[len(an)-1])
	}
	if ok, _ := z.AdminMaintenanceCancel(context.Background()); ok {
		t.Fatal("nothing to cancel")
	}
}

func lastChatContaining(p *fakePeer, s string) string {
	c := p.chats()
	for i := len(c) - 1; i >= 0; i-- {
		if strings.Contains(c[i].Text, s) {
			return c[i].Text
		}
	}
	return ""
}

func TestAdminAnnounceReachesEveryoneAsBannerAndChat(t *testing.T) {
	z := newTestZone(nil)
	a, b := join(z, 1, "Ana"), join(z, 2, "Bia")
	runZone(t, z)
	n, err := z.AdminAnnounce(context.Background(), "", "Bem-vindos!")
	if err != nil || n != 2 {
		t.Fatalf("announce = %d, %v", n, err)
	}
	for _, p := range []*fakePeer{a, b} {
		if an := p.announces(); len(an) != 1 || an[0].Title != "Anúncio" || an[0].Text != "Bem-vindos!" {
			t.Fatalf("announce = %+v", an)
		}
	}
}

func TestAdminEventsInvasionAndXP2(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, brasilia) // quarta: sem XP em dobro pelo horário
	cfg := testConfig()
	cfg.Spawns = nil
	cfg.XP2Days = []time.Weekday{time.Saturday, time.Sunday}
	cfg.InvasionChance = 0.15
	var mu sync.Mutex
	cfg.Clock = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	peer := join(z, 1, "Ana")
	runZone(t, z)
	ctx := context.Background()

	st, err := z.AdminOps(ctx)
	if err != nil || st.XP2.On || st.XP2.NextStart == nil || st.XP2.NextStart.Weekday() != time.Saturday ||
		st.XP2.NextEnd == nil || st.XP2.NextEnd.Weekday() != time.Monday || st.Invasion.Chance != 0.15 || st.NextNight.Before(now) {
		t.Fatalf("ops = %+v, %v", st, err)
	}

	if msg, err := z.AdminXP2(ctx, 3); err != nil || !strings.Contains(msg, "ligada") {
		t.Fatalf("xp2 on = %q, %v", msg, err)
	}
	st, _ = z.AdminOps(ctx)
	if !st.XP2.On || st.XP2.GMUntil == nil || !st.XP2.GMUntil.Equal(now.Add(3*time.Hour)) {
		t.Fatalf("xp2 = %+v", st.XP2)
	}
	if _, err := z.AdminXP2(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if st, _ = z.AdminOps(ctx); st.XP2.On {
		t.Fatal("xp2 off")
	}

	if _, err := z.AdminInvasion(ctx, "parar"); err == nil {
		t.Fatal("no invasion to stop")
	}
	if _, err := z.AdminInvasion(ctx, "anunciar"); err != nil {
		t.Fatal(err)
	}
	st, _ = z.AdminOps(ctx)
	if st.Invasion.Phase != "warning" || st.Invasion.StartsAt == nil {
		t.Fatalf("invasion = %+v", st.Invasion)
	}
	if _, err := z.AdminInvasion(ctx, "anunciar"); err == nil {
		t.Fatal("only one invasion at a time")
	}
	if _, err := z.AdminInvasion(ctx, "agora"); err != nil {
		t.Fatal(err)
	}
	if st, _ = z.AdminOps(ctx); st.Invasion.Phase != "active" || st.Invasion.Wave != 1 {
		t.Fatalf("invasion = %+v", st.Invasion)
	}
	if _, err := z.AdminInvasion(ctx, "parar"); err != nil {
		t.Fatal(err)
	}
	if st, _ = z.AdminOps(ctx); st.Invasion.Phase != "none" {
		t.Fatalf("invasion = %+v", st.Invasion)
	}
	if len(peer.announces()) < 4 { // XP2 ligada e desligada, invasão anunciada, começou e parou
		t.Fatalf("announces = %d", len(peer.announces()))
	}
}

type bossSaves struct {
	mu   sync.Mutex
	list []BossData
}

func (b *bossSaves) SaveBoss(_ context.Context, d BossData) error {
	b.mu.Lock()
	b.list = append(b.list, d)
	b.mu.Unlock()
	return nil
}

func (b *bossSaves) last() (BossData, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.list) == 0 {
		return BossData{}, false
	}
	return b.list[len(b.list)-1], true
}

func TestAdminBossRespawn(t *testing.T) {
	now := time.Now()
	killed := now.Add(-time.Hour)
	cfg := testConfig()
	cfg.Spawns = []game.Spawn{fixed("ozrhal", 25, raidX, raidZ), fixed("bone_lord", 12, -60, 0)}
	cfg.Bosses = []BossData{{ID: "ozrhal", KilledAt: killed, RespawnAt: now.Add(5 * time.Hour)}}
	cfg.Clock = time.Now
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	saves := &bossSaves{}
	z.SetBossBackend(saves)
	var raid, lord *entity
	for _, m := range z.monsters {
		if m.mon.raid != nil {
			raid = m
		} else {
			lord = m
		}
	}
	lord.dead, lord.hp, lord.mon.respawnAt = true, 0, z.now+3600
	peer := join(z, 1, "Ana")
	runZone(t, z)
	ctx := context.Background()

	st, err := z.AdminOps(ctx)
	if err != nil || len(st.Bosses) != 2 {
		t.Fatalf("bosses = %+v, %v", st.Bosses, err)
	}
	for _, b := range st.Bosses {
		if b.Alive || b.RespawnAt == nil {
			t.Fatalf("dead boss shows when it respawns: %+v", b)
		}
	}
	if name, err := z.AdminBossRespawn(ctx, raid.id); err != nil || name == "" {
		t.Fatalf("respawn raid = %q, %v", name, err)
	}
	if _, err := z.AdminBossRespawn(ctx, lord.id); err != nil {
		t.Fatal(err)
	}
	if _, err := z.AdminBossRespawn(ctx, 1); err == nil {
		t.Fatal("a player is not a boss")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		st, _ = z.AdminOps(ctx)
		if st.Bosses[0].Alive && st.Bosses[1].Alive {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("bosses did not respawn: %+v", st.Bosses)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := z.AdminBossRespawn(ctx, lord.id); err == nil {
		t.Fatal("an alive boss cannot be respawned")
	}
	// O raid fica salvo como "renasce agora": um reinício não o esconde de novo.
	deadline = time.Now().Add(2 * time.Second)
	for {
		if b, ok := saves.last(); ok && b.ID == "ozrhal" && !b.RespawnAt.After(time.Now()) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the forced respawn is saved")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !peer.has(announced(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SPAWN)) {
		t.Fatal("everyone hears the raid boss spawn")
	}
}

func TestNextPhase(t *testing.T) {
	now := noon
	n := nextPhase(now, game.NightStart)
	if !n.After(now) || n.Sub(now) > game.DayCycle || !game.IsNight(game.DayPhase(n.Add(time.Second))) {
		t.Fatalf("next night = %v", n)
	}
	d := nextPhase(n.Add(time.Minute), 0)
	if game.IsNight(game.DayPhase(d.Add(time.Second))) || !d.After(n) {
		t.Fatalf("next day = %v", d)
	}
}

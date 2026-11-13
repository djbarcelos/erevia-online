package zone

import (
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// O covil dos testes fica fora da cidade (raio 20) e longe dos jogadores que só observam.
const raidX, raidZ = 100.0, 0.0

func newRaidZone(now *time.Time, saved ...BossData) (*Zone, *entity) {
	cfg := testConfig()
	cfg.Spawns = []game.Spawn{fixed("ozrhal", 25, raidX, raidZ)}
	cfg.SafeRadius = 20
	cfg.Bosses = saved
	if now != nil {
		cfg.Clock = func() time.Time { return *now }
	}
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	return z, z.onlyMonster()
}

// raider: jogador de nível 18 dentro do covil (no 20, o máximo, a XP não sobe mais).
func raider(z *Zone, id uint32, class string, x, zz float64) (*entity, *fakePeer) {
	p, peer := joinKit(z, id, class, x, zz, 0)
	atLevel(z, p, 18)
	return p, peer
}

func (z *Zone) aliveSummons() int {
	n := 0
	for _, m := range z.monsters {
		if m.mon.summon && !m.dead {
			n++
		}
	}
	return n
}

func TestRaidImmuneToControlButTauntable(t *testing.T) {
	z, m := newRaidZone(nil)
	p, _ := raider(z, 1, "knight", raidX-3, raidZ)
	for _, k := range []game.EffectKind{game.EffectStun, game.EffectRoot, game.EffectSlow, game.EffectBlind} {
		z.applyEffect(p, m, &game.EffectDef{Kind: k, Duration: 5, Value: 0.5})
		if m.has(k, z.now) {
			t.Fatalf("raid boss should be immune to effect %d", k)
		}
	}
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectTaunt, Duration: 5})
	if !m.has(game.EffectTaunt, z.now) || m.act.target != p.id {
		t.Fatal("taunt must still work (the tank holds the boss)")
	}
}

func TestRaidPhasesSummonOnceAndResetClearsEverything(t *testing.T) {
	z, m := newRaidZone(nil)
	p, _ := raider(z, 1, "knight", raidX-3, raidZ)
	m.mon.hate[p.id] = 1
	z.think(m)
	r := m.mon.raid
	if !r.fighting {
		t.Fatal("the boss should engage when someone is on its hate list")
	}

	m.hp = m.stats.MaxHP * 0.69
	z.think(m)
	if r.phase != 2 || z.aliveSummons() != raidSummonsP2 {
		t.Fatalf("70%%: phase 2 with %d guards, got phase %d and %d", raidSummonsP2, r.phase, z.aliveSummons())
	}
	z.think(m)
	if z.aliveSummons() != raidSummonsP2 {
		t.Fatal("each phase summons only once")
	}
	m.hp = m.stats.MaxHP * 0.39
	z.think(m)
	if r.phase != 3 || z.aliveSummons() != raidSummonsP2+raidSummonsP3 {
		t.Fatalf("40%%: phase 3 with more guards, got phase %d and %d", r.phase, z.aliveSummons())
	}
	for _, g := range z.monsters {
		if g.mon.summon && g.mon.hate[p.id] == 0 {
			t.Fatal("guards arrive angry at whoever is fighting the boss")
		}
	}

	// Ninguém bate há 15 s: volta ao trono e esquece a luta.
	r.lastHit = z.now - raidIdleReset - 1
	z.think(m)
	if r.phase != 1 || r.fighting || z.aliveSummons() != 0 || len(m.mon.hate) != 0 || !m.mon.returning {
		t.Fatalf("idle reset: phase=%d fight=%v summons=%d hate=%d", r.phase, r.fightStart, z.aliveSummons(), len(m.mon.hate))
	}
	for i := 0; i < 400 && m.mon.returning; i++ {
		z.think(m)
		z.step(m, 0.05)
	}
	if m.hp != m.stats.MaxHP {
		t.Fatal("back on the throne the boss heals to full")
	}
}

func TestRaidLeashResets(t *testing.T) {
	z, m := newRaidZone(nil)
	p, _ := raider(z, 1, "knight", raidX-3, raidZ)
	m.mon.hate[p.id] = 1
	z.think(m)
	m.hp = m.stats.MaxHP * 0.5
	z.think(m)
	m.pos = Vec2{X: raidX - m.mon.def.LeashRadius - 1, Z: raidZ}
	z.think(m)
	if m.mon.raid.fighting || z.aliveSummons() != 0 || !m.mon.returning {
		t.Fatal("pulled out of the lair, the boss resets")
	}
}

func TestRaidDoesNotRegenerateMidFight(t *testing.T) {
	z, m := newRaidZone(nil)
	m.hp = m.stats.MaxHP / 2
	m.lastCombat = -100
	z.regen(m, 5)
	if m.hp != m.stats.MaxHP/2 {
		t.Fatal("the raid boss only heals by going back to the throne")
	}
}

func TestRaidTelegraphHitsOnlyWhoStaysInside(t *testing.T) {
	z, m := newRaidZone(nil)
	at := Vec2{X: 60, Z: 0}
	in, _ := raider(z, 1, "knight", 60, 1)
	out, _ := raider(z, 2, "mage", 60, 6)
	runner, _ := raider(z, 3, "rogue", 61, 0)
	inHP, outHP, runHP := in.hp, out.hp, runner.hp
	z.addTelegraph(m, pb.TelegraphKind_TELEGRAPH_KIND_RAIN, at, 4, 2, 0, 1, RaidHitRain, &game.EffectDef{Kind: game.EffectStun, Duration: 1})
	z.tickFor(1)
	runner.teleportTo(Vec2{X: 70, Z: 0}) // saiu a tempo
	z.tickFor(1.1)
	if in.hp >= inHP || !in.has(game.EffectStun, z.now) {
		t.Fatal("whoever stays inside takes the hit and the stun")
	}
	if out.hp != outHP || runner.hp != runHP {
		t.Fatal("the hit only lands inside the circle when it resolves")
	}
	if len(z.telegraphs) != 0 {
		t.Fatal("resolved telegraphs are removed")
	}
}

func TestRaidDeathMarkFollowsAndHitsNeighbors(t *testing.T) {
	z, m := newRaidZone(nil)
	marked, _ := raider(z, 1, "knight", 60, 0)
	near, _ := raider(z, 2, "mage", 62, 0)
	far, _ := raider(z, 3, "rogue", 60, 14)
	mHP, nHP, fHP := marked.hp, near.hp, far.hp
	z.addTelegraph(m, pb.TelegraphKind_TELEGRAPH_KIND_MARK, marked.pos, raidMarkRadius, raidMarkDelay, marked.id, raidMarkMult, RaidHitMark, nil)
	z.tickFor(2)
	// O marcado corre para perto do outro: a explosão vai junto.
	marked.teleportTo(Vec2{X: 60, Z: 12})
	z.tickFor(raidMarkDelay)
	if marked.hp != mHP {
		t.Fatal("the mark never hurts the marked player")
	}
	if near.hp != nHP {
		t.Fatal("the mark explodes where the marked player is, not where it started")
	}
	if far.hp >= fHP {
		t.Fatal("players next to the marked one take the explosion")
	}
}

func TestRaidResetCancelsPendingTelegraphs(t *testing.T) {
	z, m := newRaidZone(nil)
	p, peer := raider(z, 1, "knight", 60, 0)
	hp := p.hp
	z.addTelegraph(m, pb.TelegraphKind_TELEGRAPH_KIND_RAIN, p.pos, 4, 2, 0, 1, RaidHitRain, nil)
	z.raidReset(m)
	z.tickFor(2.5)
	if p.hp != hp {
		t.Fatal("a reset cancels the pending hits")
	}
	if !peer.has(func(msg *pb.ServerMessage) bool {
		for _, tg := range msg.GetSnapshot().GetTelegraphs() {
			if tg.Cancelled {
				return true
			}
		}
		return false
	}) {
		t.Fatal("clients are told the telegraph was cancelled")
	}
}

func TestRaidEnrageAndFinalEnrage(t *testing.T) {
	z, m := newRaidZone(nil)
	p, _ := raider(z, 1, "knight", raidX-3, raidZ)
	m.mon.hate[p.id] = 1
	z.think(m)
	base := m.stats
	m.hp = m.stats.MaxHP * 0.19
	z.think(m)
	r := m.mon.raid
	if !r.enraged || m.stats.AtkInterval >= base.AtkInterval {
		t.Fatal("below 20% the boss attacks faster")
	}
	r.fightStart = z.now - raidFinalAfter - 1
	r.lastHit = z.now
	z.think(m)
	if !r.final || math.Abs(m.stats.PAtk-base.PAtk*raidFinalMult) > 1e-9 {
		t.Fatal("a fight that drags on triggers the final enrage")
	}
	z.raidReset(m)
	if m.stats != base {
		t.Fatal("a reset undoes the enrage")
	}
}

func TestRaidAbilitiesScheduleByPhase(t *testing.T) {
	z, m := newRaidZone(nil)
	tank, _ := raider(z, 1, "knight", raidX-3, raidZ)
	dps, _ := raider(z, 2, "mage", raidX-10, raidZ)
	m.mon.hate[tank.id] = 100
	m.mon.hate[dps.id] = 10
	z.think(m)
	r := m.mon.raid
	kinds := func() map[pb.TelegraphKind]int {
		out := map[pb.TelegraphKind]int{}
		for _, tg := range z.telegraphs {
			out[tg.kind]++
		}
		return out
	}
	r.nextWave, r.nextRain, r.nextMark = z.now, z.now, z.now
	z.think(m)
	if k := kinds(); k[pb.TelegraphKind_TELEGRAPH_KIND_WAVE] != 1 || k[pb.TelegraphKind_TELEGRAPH_KIND_RAIN] != 0 || k[pb.TelegraphKind_TELEGRAPH_KIND_MARK] != 0 {
		t.Fatalf("phase 1 only has the wave: %v", k)
	}
	z.telegraphs = nil
	m.hp = m.stats.MaxHP * 0.3 // fases 2 e 3 de uma vez
	z.think(m)
	r.nextWave, r.nextRain, r.nextMark = z.now+99, z.now, z.now
	m.act = action{kind: actAttack, target: tank.id}
	z.telegraphs = nil
	z.think(m)
	k := kinds()
	if k[pb.TelegraphKind_TELEGRAPH_KIND_RAIN] != 2 || k[pb.TelegraphKind_TELEGRAPH_KIND_MARK] != 1 {
		t.Fatalf("phase 3: one rain per player (max 3) and one mark: %v", k)
	}
	for _, tg := range z.telegraphs {
		if tg.kind == pb.TelegraphKind_TELEGRAPH_KIND_MARK && tg.follow != dps.id {
			t.Fatal("the mark never goes to the tank when someone else is fighting")
		}
	}
	if !dps.has(game.EffectDeathMark, z.now) {
		t.Fatal("the marked player gets the death mark effect")
	}
}

func TestRaidRewardsSplitByDamage(t *testing.T) {
	now := noon
	z, m := newRaidZone(&now)
	a1, _ := raider(z, 1, "knight", raidX-4, raidZ)
	a2, _ := raider(z, 2, "mage", raidX-5, raidZ)
	b1, _ := raider(z, 3, "rogue", raidX-4, raidZ+2)
	b2, _ := raider(z, 4, "barbarian", raidX-5, raidZ+2)
	solo, soloPeer := raider(z, 5, "mage", raidX-4, raidZ-2)
	makeParty(z, a1, a2)
	makeParty(z, b1, b2)
	z.raidCredit(m, a1, 400)
	z.raidCredit(m, a2, 200)
	z.raidCredit(m, b1, 370)
	z.raidCredit(m, solo, 30)

	xp := map[uint32]int64{}
	for _, p := range []*entity{a1, a2, b1, b2, solo} {
		xp[p.id] = p.pl.xp
	}
	z.kill(m, a1)
	z.tickFor(0.1)

	full := float64(game.MonsterXP(m.mon.def, m.level, 18)) * partyBonus[1]
	for _, c := range []struct {
		p     *entity
		share float64
	}{{a1, 0.6}, {a2, 0.6}, {b1, 0.37}, {b2, 0.37}} {
		want := int64(math.Round(full * c.share / 2))
		if got := c.p.pl.xp - xp[c.p.id]; got != want {
			t.Errorf("%s: XP %d, want %d", c.p.name, got, want)
		}
	}
	if solo.pl.xp != xp[solo.id] || !soloPeer.systemContains("sem XP") {
		t.Error("less than 5% of the damage gives no XP (and says why)")
	}

	// Saque: reservado 60 s para o grupo de maior dano (representado por quem mais bateu).
	if len(z.ground) == 0 {
		t.Fatal("the raid boss always drops loot")
	}
	legend, epics := 0, 0
	for _, g := range z.ground {
		if g.owner != a1.id || math.Abs(g.reservedUntil-z.now-raidReserve) > 0.2 {
			t.Fatalf("raid loot reserved 60 s for the top party: owner=%d left=%.1f", g.owner, g.reservedUntil-z.now)
		}
		if !z.canTakeReserved(a2, g) || z.canTakeReserved(b1, g) || z.canTakeReserved(solo, g) {
			t.Fatal("only the top party can pick up the reserved loot")
		}
		switch g.def.Grade {
		case game.GradeEpic:
			if g.def.Equippable() {
				epics++
			}
		case game.GradeLegendary:
			legend++
		}
	}
	if epics < game.RaidEpicDrops || legend > 1 {
		t.Fatalf("raid gear: %d epics (want %d), %d legendary (max 1)", epics, game.RaidEpicDrops, legend)
	}

	// Renascimento entre 3 e 4 h, salvo no banco e mostrado no mapa como janela.
	r := m.mon.raid
	d := r.spawnAt.Sub(now)
	if d < 3*time.Hour || d > 4*time.Hour {
		t.Fatalf("respawn in %v", d)
	}
	select {
	case b := <-z.bossJobs:
		if b.ID != "ozrhal" || !b.RespawnAt.Equal(r.spawnAt) || b.TopName != "grupo de "+a1.name {
			t.Fatalf("saved boss: %+v", b)
		}
	default:
		t.Fatal("the respawn time goes to the database")
	}
	st := z.bossStatusPB(m)
	if st.Alive || st.WindowStartMs != now.Add(3*time.Hour).UnixMilli() || st.WindowEndMs != now.Add(4*time.Hour).UnixMilli() {
		t.Fatalf("map shows the 1 h window, not the exact time: %+v", st)
	}
	if !soloPeer.has(func(msg *pb.ServerMessage) bool {
		a := msg.GetAnnounce()
		return a != nil && a.Kind == pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT
	}) {
		t.Fatal("everyone hears that the boss was defeated")
	}
}

func TestRaidClanWinsLootWhenNotInParty(t *testing.T) {
	z, m := newRaidZone(nil)
	a, _ := raider(z, 1, "knight", raidX-4, raidZ)
	b, _ := raider(z, 2, "mage", raidX-5, raidZ)
	stranger, _ := raider(z, 3, "rogue", raidX-6, raidZ)
	cl := &clan{id: 9, name: "Lobos"}
	a.pl.clan, b.pl.clan = cl, cl
	z.raidCredit(m, a, 300)
	z.raidCredit(m, b, 400)
	z.raidCredit(m, stranger, 200)
	z.kill(m, b)
	for _, g := range z.ground {
		if g.owner != b.id || g.clan != cl.id {
			t.Fatalf("clan loot: owner=%d clan=%d", g.owner, g.clan)
		}
		if !z.canTakeReserved(a, g) || z.canTakeReserved(stranger, g) {
			t.Fatal("clan members share the reserved loot")
		}
	}
}

func TestRaidSavedRespawnAndAnnouncements(t *testing.T) {
	now := noon
	killed := now.Add(-2 * time.Hour)
	z, m := newRaidZone(&now, BossData{ID: "ozrhal", KilledAt: killed, RespawnAt: now.Add(30 * time.Minute)})
	if !m.dead || !m.mon.hidden {
		t.Fatal("a boss saved as dead starts hidden")
	}
	_, peer := raider(z, 1, "knight", 30, 0)
	for _, w := range peer.welcome().GetEntities() {
		if w.Id == m.id {
			t.Fatal("a hidden boss is not sent to clients")
		}
	}
	bl := peer.welcome().GetBosses()
	if len(bl) != 1 || bl[0].Alive || bl[0].WindowStartMs != killed.Add(3*time.Hour).UnixMilli() {
		t.Fatalf("welcome boss list: %+v", bl)
	}

	now = now.Add(21 * time.Minute) // faltam 9 min
	z.tickFor(0.1)
	if !peer.has(announced(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON)) || !m.dead {
		t.Fatal("10 min before, everyone is warned")
	}
	now = now.Add(10 * time.Minute)
	z.tickFor(0.1)
	if m.dead || !peer.has(announced(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SPAWN)) {
		t.Fatal("the boss spawns at the saved time and everyone hears it")
	}
	if !peer.has(func(msg *pb.ServerMessage) bool { s := msg.GetBossStatus(); return s != nil && s.Alive }) {
		t.Fatal("the map is told the boss is alive")
	}

	// Sem linha no banco (ou com a hora já passada): nasce vivo ao ligar o servidor.
	z2, m2 := newRaidZone(&now, BossData{ID: "ozrhal", RespawnAt: now.Add(-time.Minute)})
	if m2.dead || z2.bossList()[0].Alive != true {
		t.Fatal("a past respawn time means the boss is alive")
	}
}

func TestRaidCreditCountsDamageAndKeepsFightAlive(t *testing.T) {
	z, m := newRaidZone(nil)
	p, _ := raider(z, 1, "mage", raidX-6, raidZ)
	m.mon.hate[p.id] = 1
	z.think(m)
	z.tickFor(3)
	r := m.mon.raid
	before := r.total
	z.damage(p, m, 50, nil)
	if r.total != before+50 || r.lastHit != z.now {
		t.Fatalf("damage on the boss is credited: %v → %v", before, r.total)
	}
	// Burn/veneno de quem saiu (sem dono) não quebra nada.
	z.damageFrom(nil, m, 10, nil)
}

func announced(kind pb.AnnounceKind) func(*pb.ServerMessage) bool {
	return func(msg *pb.ServerMessage) bool {
		a := msg.GetAnnounce()
		return a != nil && a.Kind == kind
	}
}

func (f *fakePeer) welcome() *pb.Welcome {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if w := m.GetWelcome(); w != nil {
			return w
		}
	}
	return nil
}

func TestRaidDevCommandsOnlyInDev(t *testing.T) {
	now := noon
	z, m := newRaidZone(&now)
	p, peer := raider(z, 1, "knight", 30, 0)
	z.handle(chatEvent{id: p.id, text: "/raid matar"})
	if m.dead {
		t.Fatal("/raid must do nothing without DevCommands (production)")
	}
	z.cfg.DevCommands = true
	p.pl.chatTokens = 10
	z.handle(chatEvent{id: p.id, text: "/raid vida 50"})
	if math.Abs(m.hp-m.stats.MaxHP/2) > 1 {
		t.Fatal("/raid vida sets the boss HP")
	}
	z.handle(chatEvent{id: p.id, text: "/raid matar"})
	if !m.dead {
		t.Fatal("/raid matar kills the boss")
	}
	z.handle(chatEvent{id: p.id, text: "/raid nascer"})
	z.tickFor(0.1)
	if m.dead || !peer.has(announced(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SPAWN)) {
		t.Fatal("/raid nascer spawns it now")
	}
}

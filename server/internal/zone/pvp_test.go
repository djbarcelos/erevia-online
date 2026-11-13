package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// makeParty põe os jogadores num grupo (o primeiro é o líder).
func makeParty(z *Zone, members ...*entity) *party {
	pt := &party{id: uint32(len(members)) + 1000, leader: members[0].id}
	for _, m := range members {
		pt.members = append(pt.members, m.id)
		m.pl.party = pt
	}
	return pt
}

// pvpPair: dois jogadores de nível 10 lado a lado, fora da cidade (raio 20 nos testes).
func pvpPair(z *Zone) (a, b *entity, pa, pb_ *fakePeer) {
	a, pa = joinKit(z, 1, "knight", 30, 0, 0)
	b, pb_ = joinKit(z, 2, "mage", 31.5, 0, 0)
	b.name = "Kmage2"
	atLevel(z, a, 10)
	atLevel(z, b, 10)
	return a, b, pa, pb_
}

func TestHostileRules(t *testing.T) {
	z := newCombatZone()
	a, b, _, _ := pvpPair(z)

	if z.hostile(a, b, false) {
		t.Fatal("a normal player is not hostile without force")
	}
	if !z.hostile(a, b, true) {
		t.Fatal("force should allow attacking a normal player outside town")
	}
	b.pl.flagUntil = z.now + 10
	if !z.hostile(a, b, false) {
		t.Fatal("a flagged player is hostile")
	}
	b.pl.flagUntil = 0

	low, _ := joinKit(z, 3, "rogue", 32, 0, 0)
	if z.hostile(a, low, true) || z.hostile(low, a, true) {
		t.Fatal("players below the PvP level are protected and cannot attack")
	}

	makeParty(z, a, b)
	if z.hostile(a, b, true) {
		t.Fatal("party members never hit each other")
	}
	a.pl.party, b.pl.party = nil, nil
	cl := &clan{id: 7, name: "Lobos"}
	a.pl.clan, b.pl.clan = cl, cl
	if z.hostile(a, b, true) {
		t.Fatal("clan members never hit each other")
	}
	a.pl.clan, b.pl.clan = nil, nil

	// Cidade: só contra quem tem karma, e quem tem karma não ataca.
	b.pos = Vec2{X: 5}
	if z.hostile(a, b, true) {
		t.Fatal("no fighting inside town")
	}
	b.pl.karma = 100
	if !z.hostile(a, b, false) {
		t.Fatal("players with karma can be hunted inside town")
	}
	a.pl.karma = 50
	if z.hostile(a, b, false) {
		t.Fatal("players with karma cannot attack inside town")
	}
	a.pl.karma = 0
	b.pos = Vec2{X: 31.5}

	z.applyEffect(b, b, &game.EffectDef{Kind: game.EffectInvisible, Duration: 10})
	if z.hostile(a, b, true) {
		t.Fatal("invisible players cannot be targeted")
	}
}

func TestPvPFlagAndKarma(t *testing.T) {
	z := newCombatZone()
	a, b, pa, pb_ := pvpPair(z)

	z.handle(attackEvent{id: a.id, target: b.id}) // sem Ctrl: recusado
	if a.act.kind != actNone || !pa.systemContains("Ctrl") {
		t.Fatal("attacking a normal player needs force")
	}
	z.handle(attackEvent{id: a.id, target: b.id, force: true})
	z.tickFor(2)
	if z.pvpState(a) != pb.PvpState_PVP_STATE_FLAGGED {
		t.Fatal("hitting a player should flag the attacker")
	}
	if z.pvpState(b) != pb.PvpState_PVP_STATE_NORMAL {
		t.Fatal("being hit does not flag")
	}
	if !pb_.has(func(m *pb.ServerMessage) bool {
		s := m.GetPlayerStatus()
		return s != nil && s.Id == a.id && s.PvpState == pb.PvpState_PVP_STATE_FLAGGED
	}) {
		t.Fatal("everyone should see the attacker turn purple")
	}

	xp := b.pl.xp
	z.damage(a, b, b.hp+10, nil)
	if !b.dead {
		t.Fatal("b should be dead")
	}
	if a.pl.karma != karmaBase || a.pl.pks != 1 {
		t.Fatalf("killing a normal player gives karma: karma=%d pks=%d", a.pl.karma, a.pl.pks)
	}
	if z.pvpState(a) != pb.PvpState_PVP_STATE_CHAOTIC {
		t.Fatal("a player with karma is chaotic (red)")
	}
	if b.pl.xp != xp {
		t.Fatal("dying to a player costs no XP")
	}
	if b.pl.killedBy != a.name || z.selfStats(b).KilledBy != a.name {
		t.Fatal("the dead player should know who killed them")
	}

	// Segundo PK: karma sobe mais.
	z.handle(respawnEvent{id: b.id})
	b.pos = Vec2{X: 31.5}
	z.damage(a, b, b.hp+10, nil)
	if a.pl.karma != 2*karmaBase+karmaPerPK || a.pl.pks != 2 {
		t.Fatalf("second PK: karma=%d pks=%d", a.pl.karma, a.pl.pks)
	}
}

func TestKillingFlaggedPlayerGivesNoKarma(t *testing.T) {
	z := newCombatZone()
	a, b, _, _ := pvpPair(z)
	b.pl.flagUntil = z.now + 20
	z.damage(a, b, b.hp+10, nil)
	if a.pl.karma != 0 || a.pl.pvpKills != 1 {
		t.Fatalf("killing a flagged player is a PvP win: karma=%d wins=%d", a.pl.karma, a.pl.pvpKills)
	}
	if b.pl.flagUntil != 0 {
		t.Fatal("death clears the flag")
	}
}

func TestFlagExpires(t *testing.T) {
	z := newCombatZone()
	a, b, pa, _ := pvpPair(z)
	z.onPvPHit(a, b)
	z.tickFor(flagSeconds - 1)
	if z.pvpState(a) != pb.PvpState_PVP_STATE_FLAGGED {
		t.Fatal("still flagged before 30 s")
	}
	pa.reset()
	z.tickFor(2)
	if z.pvpState(a) != pb.PvpState_PVP_STATE_NORMAL {
		t.Fatal("flag should expire after 30 s")
	}
	if !pa.has(func(m *pb.ServerMessage) bool {
		s := m.GetPlayerStatus()
		return s != nil && s.Id == a.id && s.PvpState == pb.PvpState_PVP_STATE_NORMAL
	}) {
		t.Fatal("the name should go back to normal for everyone")
	}
}

func TestChaoticDeathDropsItemsAndDoubleXP(t *testing.T) {
	z := newCombatZone()
	hunter, red, _, pr := pvpPair(z)
	atLevel(z, red, 12)
	red.pl.inv.equip[game.SlotWeapon].enchant = 5
	wanted := red.pl.inv.equip[game.SlotWeapon].def

	// A chance é sorteada: repete até cair (60% por morte com karma alto).
	dropped := false
	for range 40 {
		red.pl.karma, red.pl.pks = maxKarma, pkDropMostAt
		red.pl.xp = game.XPLevelStart(12) + game.XPToNext(12)/2
		xp := red.pl.xp
		z.damage(hunter, red, red.hp+10, nil)
		want := game.XPAfterDeath(game.XPAfterDeath(xp, 12), 12)
		if red.pl.xp != want {
			t.Fatalf("red players lose double XP: %d, want %d", red.pl.xp, want)
		}
		if len(z.ground) > 0 {
			dropped = true
			break
		}
		z.handle(respawnEvent{id: red.id})
		red.pos = Vec2{X: 31.5}
	}
	if !dropped {
		t.Fatal("a red player should eventually drop items")
	}
	if n := len(z.ground); n == 0 || n > pkDropCount(pkDropMostAt) {
		t.Fatalf("dropped %d items", n)
	}
	var weapon *groundItem
	for _, g := range z.ground {
		if g.owner != hunter.id {
			t.Fatal("PK drops are reserved for the killer")
		}
		if g.def == wanted {
			weapon = g
		}
	}
	// Com 3 itens no kit e 3 sorteios, tudo cai: a arma inclusive, com o encantamento.
	if weapon == nil || weapon.enchant != 5 {
		t.Fatal("the equipped weapon should drop with its enchant")
	}
	if !red.pl.inv.equip[game.SlotWeapon].empty() {
		t.Fatal("the dropped weapon must leave the inventory")
	}
	if !pr.systemContains("Você deixou cair +5") {
		t.Fatal("the red player should be told what dropped")
	}

	// Quem matou pega a arma de volta com o +5.
	z.handle(itemEvent{id: hunter.id, msg: &pb.PickUp{Id: weapon.id}})
	z.tickFor(2)
	found := false
	for _, s := range hunter.pl.inv.bag {
		if s.def == wanted && s.enchant == 5 {
			found = true
		}
	}
	if !found {
		t.Fatal("picking up keeps the enchant")
	}
}

func TestKarmaBurnsOnMonsterKills(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 10, 40, 0))
	m := z.onlyMonster()
	a, _ := joinKit(z, 1, "knight", 38, 0, 0)
	atLevel(z, a, 10)
	a.pl.karma = 20
	z.damage(a, m, m.hp+10, nil)
	if a.pl.karma != 0 {
		t.Fatalf("killing a level 10 monster burns 30 karma: %d left", a.pl.karma)
	}
}

func TestAreaSkillsSkipNormalPlayers(t *testing.T) {
	z := newCombatZone()
	mage, _ := joinKit(z, 1, "mage", 30, 0, 0)
	white, _ := joinKit(z, 2, "knight", 31, 0, 0)
	flagged, _ := joinKit(z, 3, "rogue", 30, 1, 0)
	for _, p := range []*entity{mage, white, flagged} {
		atLevel(z, p, 10)
	}
	flagged.pl.flagUntil = z.now + 20
	learn(z, mage, map[uint32]int{game.SkillIceNova: 1})
	wHP, fHP := white.hp, flagged.hp
	z.handle(skillEvent{id: mage.id, skill: game.SkillIceNova})
	z.tickFor(0.2)
	if white.hp != wHP {
		t.Fatal("area skills must never hit normal players")
	}
	if flagged.hp >= fHP {
		t.Fatal("area skills hit flagged players")
	}
	if z.pvpState(mage) != pb.PvpState_PVP_STATE_FLAGGED {
		t.Fatal("hitting a flagged player also flags the caster")
	}
}

func TestHuntingChaoticPlayerInTown(t *testing.T) {
	z := newCombatZone()
	a, b, _, _ := pvpPair(z)
	a.pos, b.pos = Vec2{X: 5}, Vec2{X: 6}
	a.dest, b.dest = a.pos, b.pos
	b.pl.karma = 300
	hp := b.hp
	z.handle(attackEvent{id: a.id, target: b.id})
	z.tickFor(3)
	if b.hp >= hp {
		t.Fatal("players with karma can be attacked inside town")
	}
	if z.pvpState(a) != pb.PvpState_PVP_STATE_NORMAL {
		t.Fatal("hunting a red player does not flag")
	}
	hp = a.hp
	z.handle(attackEvent{id: b.id, target: a.id, force: true})
	z.tickFor(3)
	if a.hp < hp {
		t.Fatal("red players cannot attack inside town")
	}
}

func TestInvisibilityDropsPlayerTargets(t *testing.T) {
	z := newCombatZone()
	a, b, _, _ := pvpPair(z)
	z.handle(attackEvent{id: a.id, target: b.id, force: true})
	if a.act.target != b.id {
		t.Fatal("attack should be set")
	}
	z.applyEffect(b, b, &game.EffectDef{Kind: game.EffectInvisible, Duration: 10})
	if a.act.kind != actNone {
		t.Fatal("players should lose an invisible target")
	}
}

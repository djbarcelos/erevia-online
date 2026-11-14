package zone

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
)

func (f *fakePeer) spawned(id uint32) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.msgs {
		if s := m.GetSpawn(); s != nil && s.Entity.Id == id {
			n++
		}
		if w := m.GetWelcome(); w != nil {
			for _, e := range w.Entities {
				if e.Id == id {
					n++
				}
			}
		}
	}
	return n
}

func (f *fakePeer) despawned(id uint32) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if d := m.GetDespawn(); d != nil && d.Id == id {
			return true
		}
	}
	return false
}

func (f *fakePeer) sawState(id uint32) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		for _, s := range m.GetSnapshot().GetStates() {
			if s.Id == id {
				return true
			}
		}
	}
	return false
}

func TestFarPlayersDoNotSeeEachOther(t *testing.T) {
	z := newCombatZone()
	a, _ := joinAt(z, 1, "knight", -100, 0, 0)
	_, pb_ := joinAt(z, 2, "mage", 100, 0, 0)
	z.tickFor(0.1)
	a.dest = Vec2{X: -90, Z: 0}
	a.moved = true
	z.tickFor(0.2)
	if pb_.spawned(a.id) != 0 || pb_.sawState(a.id) {
		t.Fatal("a player 200 m away must not reach the other client")
	}
}

func TestEnteringAndLeavingTheRadius(t *testing.T) {
	z := newCombatZone()
	a, _ := joinAt(z, 1, "knight", -100, 0, 0)
	_, pb_ := joinAt(z, 2, "mage", 0, 0, 0)
	z.tickFor(0.1)

	a.teleportTo(Vec2{X: -50, Z: 0}) // 50 m: dentro do raio
	z.tickFor(0.2)
	if pb_.spawned(a.id) != 1 {
		t.Fatalf("entering the radius must send exactly one Spawn, got %d", pb_.spawned(a.id))
	}
	a.teleportTo(Vec2{X: -75, Z: 0}) // 75 m: fora do raio, dentro da folga
	z.tickFor(0.2)
	if pb_.despawned(a.id) {
		t.Fatal("inside the slack band the entity must stay known")
	}
	a.teleportTo(Vec2{X: -95, Z: 0})
	z.tickFor(0.2)
	if !pb_.despawned(a.id) {
		t.Fatal("leaving radius + slack must send a Despawn")
	}
	a.teleportTo(Vec2{X: -20, Z: 0})
	z.tickFor(0.2)
	if pb_.spawned(a.id) != 2 {
		t.Fatal("coming back must send a fresh Spawn")
	}
}

func TestPartyMembersAreAlwaysKnown(t *testing.T) {
	z := newCombatZone()
	a, _ := joinAt(z, 1, "knight", -120, 0, 0)
	b, pb_ := joinAt(z, 2, "mage", 120, 0, 0)
	pt := &party{id: 1, leader: a.id, members: []uint32{a.id, b.id}}
	a.pl.party, b.pl.party = pt, pt
	z.parties[pt.id] = pt
	z.tickFor(0.2)
	if pb_.spawned(a.id) != 1 {
		t.Fatal("a party member must be known anywhere on the map")
	}
}

func TestLeavingPlayerDespawnsForThoseWhoKnewIt(t *testing.T) {
	z := newCombatZone()
	a, _ := joinAt(z, 1, "knight", 0, 0, 0)
	_, pb_ := joinAt(z, 2, "mage", 10, 0, 0)
	z.tickFor(0.2)
	z.handle(leaveEvent{id: a.id, peer: a.pl.peer})
	z.tickFor(0.2)
	if !pb_.despawned(a.id) {
		t.Fatal("a player who leaves must be despawned for neighbours")
	}
}

func TestMonsterCorpseVanishesAndRespawnsThroughAOI(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 1, 30, 0))
	m := z.onlyMonster()
	_, peer := joinAt(z, 1, "knight", 25, 0, 0)
	if peer.spawned(m.id) != 1 {
		t.Fatal("the monster nearby must come in the Welcome")
	}
	z.kill(m, nil)
	m.mon.respawnAt = z.now + 100 // controla quando renasce
	z.tickFor(corpseSeconds + 0.1)
	if !peer.despawned(m.id) {
		t.Fatal("the corpse vanishing must send a Despawn")
	}
	m.mon.respawnAt = z.now
	z.tickFor(0.1)
	if peer.spawned(m.id) != 2 {
		t.Fatal("the respawn must send a Spawn")
	}
}

func TestRespawnInTheSameTickStillSendsAFreshSpawn(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 1, 30, 0))
	m := z.onlyMonster()
	_, peer := joinAt(z, 1, "knight", 25, 0, 0)
	z.kill(m, nil)
	m.mon.respawnAt = m.mon.corpseEnd // some e renasce no mesmo tick
	z.tickFor(corpseSeconds + 0.1)
	if m.dead || !peer.despawned(m.id) || peer.spawned(m.id) != 2 {
		t.Fatalf("same-tick respawn: dead=%v despawned=%v spawns=%d", m.dead, peer.despawned(m.id), peer.spawned(m.id))
	}
}

func TestSnapshotBytesDecodeAndOnlyCarryKnownEntities(t *testing.T) {
	known := &entity{id: 1, encTick: 7}
	known.encState = encField(snapFieldStates, &pb.EntityState{Id: 1, X: 3})
	known.encHP = encField(snapFieldHp, &pb.HpUpdate{Id: 1, Hp: 40})
	stranger := &entity{id: 2, encTick: 7, encState: encField(snapFieldStates, &pb.EntityState{Id: 2})}
	stale := &entity{id: 3, encTick: 6, encState: encField(snapFieldStates, &pb.EntityState{Id: 3})}
	p := &entity{id: 5, pl: &playerState{known: map[uint32]knownEntity{1: {e: known}, 3: {e: stale}}}}
	_ = stranger
	// Avisos no chão: só os do monstro (ou do marcado) que o jogador conhece.
	all := &pb.Snapshot{
		Combat:     []*pb.CombatEvent{{SourceId: 1, TargetId: 9}, {SourceId: 8, TargetId: 9}},
		Telegraphs: []*pb.Telegraph{{Id: 4, SourceId: 1}, {Id: 5, SourceId: 2}, {Id: 6, SourceId: 8, FollowId: 1}},
	}
	var buf []byte
	data := snapshotBytes(p, 7, []uint32{2, 1}, encodeFrame(all), &buf)
	var msg pb.ServerMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		t.Fatal(err)
	}
	s := msg.GetSnapshot()
	if s.Tick != 7 || len(s.States) != 1 || s.States[0].Id != 1 || s.States[0].X != 3 {
		t.Fatalf("states = %+v", s.States)
	}
	if len(s.Hp) != 1 || s.Hp[0].Hp != 40 || len(s.Combat) != 1 || len(s.Telegraphs) != 2 || s.Telegraphs[0].Id != 4 || s.Telegraphs[1].Id != 6 {
		t.Fatalf("hp/combat/telegraphs = %+v %+v %+v", s.Hp, s.Combat, s.Telegraphs)
	}
	if len(s.Deaths) != 1 || s.Deaths[0] != 1 {
		t.Fatalf("deaths = %v", s.Deaths)
	}
	empty := &entity{id: 6, pl: &playerState{known: map[uint32]knownEntity{}}}
	if snapshotBytes(empty, 7, nil, nil, &buf) != nil {
		t.Fatal("nothing known and no events: no message")
	}
}

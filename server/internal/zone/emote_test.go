package zone

import (
	"slices"
	"testing"

	"erevia/server/internal/pb"
)

// emotes: os emotes recebidos, como "id:tipo".
func (f *fakePeer) emotes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, m := range f.msgs {
		if e := m.GetEmote(); e != nil {
			out = append(out, pb.EmoteKind_name[int32(e.Kind)]+"@"+string(rune('0'+e.Id)))
		}
	}
	return out
}

func TestEmoteOnlyNearbyAndEndsWhenMoving(t *testing.T) {
	z := newCombatZone()
	ana, ap := named(z, 1, "Ana", "knight", 50, 0)
	_, bp := named(z, 2, "Beto", "mage", 50, 60)  // a 60 m: vê
	_, cp := named(z, 3, "Caio", "rogue", -50, 0) // a 100 m: não vê

	say(z, 1, "/sentar")
	want := []string{"EMOTE_KIND_SIT@1"}
	if !slices.Equal(ap.emotes(), want) || !slices.Equal(bp.emotes(), want) {
		t.Fatalf("sit: ana=%v beto=%v", ap.emotes(), bp.emotes())
	}
	if len(cp.emotes()) != 0 {
		t.Fatalf("far player saw the emote: %v", cp.emotes())
	}
	// Parado, o emote continua.
	z.tickFor(1)
	if len(bp.emotes()) != 1 {
		t.Fatalf("emote ended while standing: %v", bp.emotes())
	}

	// Andar acaba com o emote (uma vez só).
	z.handle(moveEvent{id: 1, x: 55, z: 0})
	z.tickFor(0.5)
	want = append(want, "EMOTE_KIND_NONE@1")
	if !slices.Equal(bp.emotes(), want) {
		t.Fatalf("after move: %v", bp.emotes())
	}
	if ana.pl.social.emote != pb.EmoteKind_EMOTE_KIND_NONE {
		t.Fatal("emote state not cleared")
	}

	// Com e sem acento; atacar também acaba.
	z.tickFor(2)
	say(z, 1, "/dançar")
	say(z, 1, "/dancar")
	z.handle(attackEvent{id: 1, target: 2})
	want = append(want, "EMOTE_KIND_DANCE@1", "EMOTE_KIND_DANCE@1", "EMOTE_KIND_NONE@1")
	if !slices.Equal(bp.emotes(), want) {
		t.Fatalf("dance + attack: %v", bp.emotes())
	}

	// Acenar toca uma vez: não deixa estado para acabar depois.
	say(z, 1, "/acenar")
	z.handle(moveEvent{id: 1, x: 40, z: 0})
	z.tickFor(0.5)
	want = append(want, "EMOTE_KIND_WAVE@1")
	if !slices.Equal(bp.emotes(), want) {
		t.Fatalf("wave: %v", bp.emotes())
	}
}

func TestEmoteStopsWalkingAndIgnoresDead(t *testing.T) {
	z := newCombatZone()
	ana, ap := named(z, 1, "Ana", "knight", 50, 0)
	z.handle(moveEvent{id: 1, x: 80, z: 0})
	z.tickFor(0.2)
	say(z, 1, "/sentar")
	if ana.dest != ana.pos {
		t.Fatal("emote did not stop the walk")
	}
	z.tickFor(0.5)
	if got := ap.emotes(); !slices.Equal(got, []string{"EMOTE_KIND_SIT@1"}) {
		t.Fatalf("emotes = %v", got)
	}

	// Morrer limpa o emote sem aviso; morto não faz emote.
	z.kill(ana, nil)
	z.tickFor(0.1)
	if ana.pl.social.emote != pb.EmoteKind_EMOTE_KIND_NONE || len(ap.emotes()) != 1 {
		t.Fatalf("dead: state=%v emotes=%v", ana.pl.social.emote, ap.emotes())
	}
	say(z, 1, "/dancar")
	if got := ap.lastSystem(); got != "Você está morto." || len(ap.emotes()) != 1 {
		t.Fatalf("dead emote = %q %v", got, ap.emotes())
	}
}

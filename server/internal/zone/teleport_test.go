package zone

import (
	"io"
	"log/slog"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// stoneAt devolve a pedra de índice i.
func (z *Zone) stoneAt(i int) *entity {
	for _, n := range z.npcs {
		if n.npc.role == pb.NpcRole_NPC_ROLE_TELEPORT && n.npc.stone == i {
			return n
		}
	}
	return nil
}

func teleportReq(z *Zone, p, stone *entity, dest int) {
	z.handle(itemEvent{id: p.id, msg: &pb.Teleport{NpcId: stone.id, DestId: uint32(dest + 1)}})
}

func TestStonesAreListedWithDestinations(t *testing.T) {
	z := newCombatZone()
	n := 0
	for _, info := range z.npcList() {
		if info.Role != pb.NpcRole_NPC_ROLE_TELEPORT {
			continue
		}
		n++
		// As pedras listam as outras pedras; o barco do Brejo lista só o outro lado da lagoa.
		same := 0
		for _, o := range teleportStones {
			if o.ferry == teleportStones[z.npcs[info.Id].npc.stone].ferry {
				same++
			}
		}
		if len(info.Dests) != same-1 {
			t.Fatalf("stone %d lists %d destinations, want %d", info.Id, len(info.Dests), same-1)
		}
		if z.npcs[info.Id].name == info.Dests[0].Name {
			t.Fatal("a stone does not list itself")
		}
	}
	if n != len(teleportStones) {
		t.Fatalf("%d stones, want %d", n, len(teleportStones))
	}
}

func TestValeStoneIsLastAndKeepsOldIDs(t *testing.T) {
	want := []string{"Cidade", "Arredores", "Campos dos Ossos", "Ermos Partidos", "Vale das Pedras Vivas", "Posto do Barqueiro", "Cais do Barqueiro", "Ilha dos Vaga-lumes"}
	if len(teleportStones) != len(want) {
		t.Fatalf("%d pedras, esperava %d", len(teleportStones), len(want))
	}
	for i, n := range want {
		if teleportStones[i].name != n {
			t.Errorf("pedra %d = %q, esperava %q (o id do destino é a posição + 1)", i, teleportStones[i].name, n)
		}
	}
	vale := teleportStones[4]
	if vale.z > -150 || vale.price <= teleportStones[3].price {
		t.Errorf("pedra do vale deveria ficar no sul e custar mais: %+v", vale)
	}
}

func TestTeleportChannelsChargesAndArrives(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 3, 63, 5))
	m := z.onlyMonster()
	town := z.stoneAt(0)
	p, peer := named(z, 1, "Viajante", "knight", town.pos.X+1, town.pos.Z)
	p.pl.inv.gold = 100
	m.mon.hate[p.id] = 50 // um monstro lembra dele

	teleportReq(z, p, town, 1) // Arredores, 40 moedas
	if p.pl.channel == nil || !peer.has(func(msg *pb.ServerMessage) bool {
		b := msg.GetChannelBar()
		return b != nil && b.Kind == pb.ChannelKind_CHANNEL_KIND_TELEPORT && b.TotalMs == 2000
	}) {
		t.Fatal("the stone should start a 2 s channel")
	}
	z.tickFor(1)
	if dist(p.pos, town.pos) > 3 || p.pl.inv.gold != 100 {
		t.Fatal("nothing happens before the channel ends")
	}
	z.tickFor(1.2)
	dest := teleportStones[1]
	if dist(p.pos, Vec2{X: dest.x, Z: dest.z}) > 5 {
		t.Fatalf("should arrive near %s, at %+v", dest.name, p.pos)
	}
	if p.pl.inv.gold != 60 {
		t.Fatalf("the trip costs 40: gold %d", p.pl.inv.gold)
	}
	if _, ok := m.mon.hate[p.id]; ok {
		t.Fatal("monsters forget whoever teleports away")
	}
	if !peer.systemContains("Você chegou em Arredores") {
		t.Fatal("the player should be told")
	}
}

func TestTeleportIsInterrupted(t *testing.T) {
	z := newCombatZone()
	town := z.stoneAt(0)
	p, peer := named(z, 1, "Viajante", "knight", town.pos.X+1, town.pos.Z)
	other, _ := named(z, 2, "Outro", "mage", town.pos.X+2, town.pos.Z)
	p.pl.inv.gold = 500

	teleportReq(z, p, town, 2)
	z.damage(other, p, 3, nil)
	if p.pl.channel != nil || !peer.systemContains("interrompida") {
		t.Fatal("damage interrupts the channel")
	}
	teleportReq(z, p, town, 2)
	z.handle(moveEvent{id: p.id, x: town.pos.X - 3, z: town.pos.Z})
	if p.pl.channel != nil {
		t.Fatal("moving cancels the channel")
	}
	teleportReq(z, p, town, 2)
	z.handle(itemEvent{id: p.id, msg: &pb.CancelChannel{}})
	if p.pl.channel != nil {
		t.Fatal("the cancel button stops the channel")
	}
	z.tickFor(3)
	if p.pl.inv.gold != 500 {
		t.Fatal("cancelled trips cost nothing")
	}
}

func TestTeleportRefusals(t *testing.T) {
	z := newCombatZone()
	town := z.stoneAt(0)
	p, peer := named(z, 1, "Viajante", "knight", town.pos.X+1, town.pos.Z)

	p.pl.inv.gold = 10
	teleportReq(z, p, town, 3)
	if p.pl.channel != nil || !peer.systemContains("custa 160") {
		t.Fatal("not enough gold")
	}
	p.pl.inv.gold = 1000
	p.pl.karma = 50
	teleportReq(z, p, town, 3)
	if p.pl.channel != nil || !peer.systemContains("karma") {
		t.Fatal("players with karma cannot teleport")
	}
	p.pl.karma = 0
	teleportReq(z, p, town, 0)
	if p.pl.channel != nil {
		t.Fatal("cannot travel to the same stone")
	}
	far := z.stoneAt(2)
	teleportReq(z, p, far, 0)
	if p.pl.channel != nil || !peer.systemContains("Chegue mais perto") {
		t.Fatal("must stand next to the stone")
	}
}

func TestReturnScroll(t *testing.T) {
	z := newCombatZone()
	p, peer := named(z, 1, "Caçador", "rogue", 60, 0)
	p.pl.inv.add(item("scroll_return"), 2)
	slot := p.pl.inv.slotOf("scroll_return")

	z.handle(itemEvent{id: p.id, msg: &pb.UseItem{Place: placeBag, Slot: uint32(slot)}})
	if p.pl.channel == nil || p.pl.channel.kind != pb.ChannelKind_CHANNEL_KIND_RETURN {
		t.Fatal("reading the scroll starts a channel")
	}
	z.tickFor(game.ReturnChannel + 0.2)
	if !z.inSafe(p.pos) {
		t.Fatalf("the scroll takes you to town, at %+v", p.pos)
	}
	if p.pl.inv.countOf("scroll_return") != 1 {
		t.Fatal("one scroll is used")
	}
	p.teleportTo(Vec2{X: 60})
	z.handle(itemEvent{id: p.id, msg: &pb.UseItem{Place: placeBag, Slot: uint32(p.pl.inv.slotOf("scroll_return"))}})
	if p.pl.channel != nil {
		t.Fatal("the scroll has a 30 s cooldown")
	}
	z.tickFor(game.ReturnCooldown)
	p.pl.karma = 10
	z.handle(itemEvent{id: p.id, msg: &pb.UseItem{Place: placeBag, Slot: uint32(p.pl.inv.slotOf("scroll_return"))}})
	if p.pl.channel != nil || !peer.systemContains("karma") {
		t.Fatal("the scroll does not work with karma")
	}
}

// O Brejo (tunnel#139): a pedra do Posto custa 180 e as pedras comuns a listam; o barco de Tomé só liga o cais e a ilha
// (10 moedas), e nem a pedra do Posto nem as da cidade listam o barco.
func TestBrejoStoneAndFerry(t *testing.T) {
	cfg, err := MapConfig(testConfig(), world.StarterMap, []string{world.StarterMap, MapTest}) // o mapa de verdade (vai até x = 330)
	if err != nil {
		t.Fatal(err)
	}
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	posto, cais, ilha := 5, 6, 7
	if teleportStones[posto].price != 180 || teleportStones[posto].ferry || !teleportStones[cais].ferry || !teleportStones[ilha].ferry {
		t.Fatalf("pedras do Brejo: %+v", teleportStones[posto:])
	}
	town := z.stoneAt(0)
	for _, d := range z.stoneDests(0) {
		if d.Name == "Cais do Barqueiro" || d.Name == "Ilha dos Vaga-lumes" {
			t.Errorf("a pedra da cidade lista o barco: %s", d.Name)
		}
	}
	// O barco do cais lista só a ilha.
	boat := z.stoneAt(cais)
	if boat.npc.title != "Barco do Tomé" || boat.model != ferryModel {
		t.Fatalf("o barco é %q (%s)", boat.npc.title, boat.model)
	}
	dests := z.stoneDests(cais)
	if len(dests) != 1 || dests[0].Name != "Ilha dos Vaga-lumes" || dests[0].Price != 10 {
		t.Fatalf("destinos do barco: %v", dests)
	}
	// Quem usa a pedra da cidade para ir à ilha é ignorado (o barco não é destino de pedra).
	p, _ := named(z, 1, "Viajante", "knight", town.pos.X+1, town.pos.Z)
	p.pl.inv.gold = 1000
	teleportReq(z, p, town, ilha)
	if p.pl.channel != nil {
		t.Fatal("a pedra da cidade não leva à ilha")
	}
	// No cais: 10 moedas e a chegada é na ilha (dentro do fosso).
	q, _ := named(z, 2, "Remador", "knight", boat.pos.X+1, boat.pos.Z)
	q.pl.inv.gold = 100
	teleportReq(z, q, boat, ilha)
	if q.pl.channel == nil {
		t.Fatalf("o barco devia começar a travessia (jogador em %+v, barco em %+v, ouro %d)", q.pos, boat.pos, q.pl.inv.gold)
	}
	z.tickFor(2.5)
	if d := dist(q.pos, Vec2{X: game.IslandX, Z: game.IslandZ}); d > 5 || q.pl.inv.gold != 90 {
		t.Fatalf("chegou a %.1f m da ilha com %d moedas (esperava 90)", d, q.pl.inv.gold)
	}
	// Posto: a pedra da cidade leva (180 moedas).
	r, _ := named(z, 3, "Viajante2", "knight", town.pos.X+1, town.pos.Z)
	r.pl.inv.gold = 200
	teleportReq(z, r, town, posto)
	if r.pl.channel != nil {
		t.Fatal("o Posto só vale depois de descoberto a pé")
	}
	learnStones(r, "posto")
	r.pl.inv.gold = 100
	teleportReq(z, r, town, posto)
	if r.pl.channel != nil {
		t.Fatal("100 moedas não pagam o Posto do Barqueiro")
	}
	r.pl.inv.gold = 200
	teleportReq(z, r, town, posto)
	z.tickFor(2.5)
	if d := dist(r.pos, Vec2{X: teleportStones[posto].x, Z: teleportStones[posto].z}); d > 5 || r.pl.inv.gold != 20 {
		t.Fatalf("Posto: a %.1f m, %d moedas (esperava 20)", d, r.pl.inv.gold)
	}
}

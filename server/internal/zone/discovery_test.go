package zone

import (
	"context"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// learnStones marca pedras como descobertas (sem passar pela caminhada).
func learnStones(p *entity, keys ...string) {
	for _, k := range keys {
		if !p.pl.disc.known[k] {
			p.pl.disc.known[k] = true
			p.pl.disc.order = append(p.pl.disc.order, k)
		}
	}
}

func joinNew(z *Zone, id uint32, x, zz float64, st game.DiscoveryState) (*entity, *fakePeer) {
	peer := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: id, Name: "Andarilho", Class: "knight", X: x, Z: zz, Items: game.StarterKit("knight"), Peer: peer, Discovered: st}})
	return z.players[id], peer
}

func lastKnown(peer *fakePeer) []uint32 {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	for i := len(peer.msgs) - 1; i >= 0; i-- {
		if k := peer.msgs[i].GetStonesKnown(); k != nil {
			return k.Ids
		}
	}
	return nil
}

// Andar até a pedra a descobre (aviso no chat e a lista de destinos conhecidos); o
// personagem novo já conhece a da cidade.
func TestWalkingToAStoneDiscoversIt(t *testing.T) {
	z := newCombatZone()
	st := teleportStones[1] // Arredores
	p, peer := joinNew(z, 1, st.x-30, st.z, game.DiscoveryState{})
	if !p.pl.disc.known["cidade"] || len(p.pl.disc.known) != 1 {
		t.Fatalf("personagem novo: %v", p.pl.disc.known)
	}
	if got := lastKnown(peer); len(got) != 3 { // cidade + os dois do barco
		t.Fatalf("destinos conhecidos ao entrar: %v", got)
	}
	z.tickFor(1)
	if p.pl.disc.known[st.key] {
		t.Fatal("longe da pedra não descobre")
	}
	z.handle(moveEvent{id: p.id, x: st.x - 2, z: st.z})
	z.tickFor(8)
	if !p.pl.disc.known[st.key] {
		t.Fatalf("chegou a pé em %+v e não descobriu", p.pos)
	}
	if !peer.systemContains("Pedra de Teleporte descoberta: Arredores") {
		t.Fatal("falta o aviso no chat")
	}
	if got := lastKnown(peer); len(got) != 4 {
		t.Fatalf("destinos conhecidos depois: %v", got)
	}
	if s := p.pl.disc.state(); len(s.Stones) != 2 {
		t.Fatalf("estado salvo: %+v", s)
	}
}

// Teleportar até a pedra (ou entrar no jogo, ou qualquer salto) não a descobre; só uma volta a pé.
func TestJumpingNextToAStoneDoesNotDiscoverIt(t *testing.T) {
	z := newCombatZone()
	ossos := teleportStones[2]
	// Entrar no jogo parado ao lado da pedra.
	p, _ := joinNew(z, 1, ossos.x+1, ossos.z, game.DiscoveryState{})
	z.tickFor(1)
	if p.pl.disc.known[ossos.key] {
		t.Fatal("entrar no jogo ao lado da pedra não descobre")
	}
	// Um salto (o que o teleporte faz) também não.
	q, _ := joinNew(z, 2, 0, 0, game.DiscoveryState{})
	q.teleportTo(Vec2{X: ossos.x + 2, Z: ossos.z})
	z.tickFor(1)
	if q.pl.disc.known[ossos.key] {
		t.Fatal("pousar ao lado da pedra não descobre")
	}
	// Saiu do raio e voltou andando: descobriu.
	z.handle(moveEvent{id: q.id, x: ossos.x + 20, z: ossos.z})
	z.tickFor(6)
	z.handle(moveEvent{id: q.id, x: ossos.x, z: ossos.z - 2})
	z.tickFor(8)
	if !q.pl.disc.known[ossos.key] {
		t.Fatalf("voltou andando e não descobriu (em %+v)", q.pos)
	}
	// Chegar pela pedra de verdade: o teleporte vai a uma pedra conhecida e não descobre a vizinha.
	z2 := newCombatZone()
	town := z2.stoneAt(0)
	r, _ := named(z2, 3, "Viajante", "knight", town.pos.X+1, town.pos.Z)
	r.pl.disc = loadDiscovery(game.DiscoveryState{Stones: []string{"cidade", "arredores"}})
	r.pl.inv.gold = 100
	teleportReq(z2, r, town, 1)
	z2.tickFor(3)
	if dist(r.pos, Vec2{X: teleportStones[1].x, Z: teleportStones[1].z}) > 5 {
		t.Fatalf("a viagem aos Arredores: %+v", r.pos)
	}
	if len(r.pl.disc.known) != 2 {
		t.Fatalf("o teleporte não descobre nada: %v", r.pl.disc.known)
	}
}

// Um destino não descoberto não vale (nem com o ouro).
func TestUndiscoveredDestinationIsRefused(t *testing.T) {
	z := newCombatZone()
	town := z.stoneAt(0)
	p, peer := named(z, 1, "Viajante", "knight", town.pos.X+1, town.pos.Z)
	p.pl.disc = loadDiscovery(game.DiscoveryState{Stones: []string{"cidade"}})
	p.pl.inv.gold = 1000
	teleportReq(z, p, town, 3) // Ermos Partidos
	if p.pl.channel != nil || !peer.systemContains("ainda não conhece") {
		t.Fatal("pedra não descoberta")
	}
	learnStones(p, "ermos")
	teleportReq(z, p, town, 3)
	if p.pl.channel == nil {
		t.Fatal("descoberta, a pedra vale")
	}
}

// Os preços de todas as pedras, as de hoje e as dos mapas que vêm.
func TestStonePrices(t *testing.T) {
	want := map[string]int64{
		"cidade": 20, "arredores": 40, "ossos": 90, "ermos": 160, "vale": 220,
		"posto": 180, "bastiao": 260, "lanterna": 240, "maresia": 300, "vigilia": 320, "valdurn": 400,
	}
	seen := map[string]bool{}
	for _, d := range allStones {
		if d.ferry {
			continue
		}
		if seen[d.key] {
			t.Fatalf("chave repetida: %s", d.key)
		}
		seen[d.key] = true
		if want[d.key] != d.price {
			t.Errorf("%s custa %d, esperava %d", d.key, d.price, want[d.key])
		}
		if d.region == "" {
			t.Errorf("%s sem região", d.key)
		}
	}
	if len(seen) != len(want) {
		t.Errorf("%d pedras, esperava %d", len(seen), len(want))
	}
}

// Quem já tinha personagem antes tem as 5 pedras de antes do Brejo, e as do Brejo não.
func TestLegacyStonesCoverTheOldFive(t *testing.T) {
	if len(game.LegacyStoneKeys) != legacyStones {
		t.Fatalf("%d chaves, %d pedras antigas", len(game.LegacyStoneKeys), legacyStones)
	}
	for i, k := range game.LegacyStoneKeys {
		if teleportStones[i].key != k {
			t.Errorf("pedra %d: %s, esperava %s", i, teleportStones[i].key, k)
		}
	}
	d := loadDiscovery(game.DiscoveryState{Stones: game.LegacyStoneKeys})
	if d.known["posto"] || len(d.known) != 5 {
		t.Fatalf("migrado: %v", d.known)
	}
}

// O estado vai e volta: salvar o personagem, entrar de novo.
func TestDiscoverySavesAndReturns(t *testing.T) {
	z := newCombatZone()
	p, _ := joinNew(z, 1, 0, 0, game.DiscoveryState{Stones: []string{"cidade", "ossos", "nao_existe"}, Town: "cidade"})
	learnStones(p, "ermos")
	st := z.savedState(p)
	if got := st.Discovered; len(got.Stones) != 3 || got.Stones[2] != "ermos" || got.Town != "cidade" {
		t.Fatalf("estado salvo: %+v", got)
	}
	z.removePlayer(p.id)
	q, _ := joinNew(z, 1, 0, 0, st.Discovered)
	if !q.pl.disc.known["ermos"] || !q.pl.disc.known["ossos"] || q.pl.disc.known["nao_existe"] || q.pl.disc.town != "cidade" {
		t.Fatalf("depois de voltar: %+v", q.pl.disc.state())
	}
}

// Viagem entre mapas e o Pergaminho de Retorno que leva à última cidade visitada.
func TestCrossMapStoneAndReturnToLastTown(t *testing.T) {
	saved := allStones
	allStones = append(append([]stoneDef{}, saved...), stoneDef{key: "clareira", name: "Clareira", region: "Teste", mapID: MapTest, x: 0, z: 20, price: 50, town: true})
	t.Cleanup(func() { allStones = saved })
	clareira := len(allStones) - 1

	m, a, b := twoZones(t, nil)
	if b.stoneAt(clareira) == nil || a.stoneAt(clareira) != nil {
		t.Fatal("a pedra da clareira fica só na zona dela")
	}
	p := mjoin(m, 1, "Ana", "", 5, 7)
	ana := a.players[1]
	town := a.stoneAt(0)
	// Ainda não descobriu a pedra da clareira: a viagem é recusada.
	a.handle(itemEvent{id: 1, msg: &pb.Teleport{NpcId: town.id, DestId: uint32(clareira + 1)}})
	if ana.pl.channel != nil {
		t.Fatal("pedra não descoberta")
	}
	learnStones(ana, "clareira")
	gold := ana.pl.inv.gold
	a.handle(itemEvent{id: 1, msg: &pb.Teleport{NpcId: town.id, DestId: uint32(clareira + 1)}})
	if ana.pl.channel == nil {
		t.Fatal("a viagem devia começar")
	}
	a.tickFor(2.5)
	pump(m.Zones()...)
	got := b.players[1]
	if got == nil || a.players[1] != nil || p.owner.Load() != b {
		t.Fatal("a viagem entre mapas devia atravessar a zona")
	}
	if got.pl.inv.gold != gold-50 {
		t.Fatalf("ouro %d, esperava %d", got.pl.inv.gold, gold-50)
	}
	b.tickFor(0.2)
	if got.pl.disc.town != "clareira" {
		t.Fatalf("a última cidade é %q", got.pl.disc.town)
	}
	// Volta pelo portão da clareira: o pergaminho leva à clareira, mesmo da praça do Vale.
	b.tickFor(11) // fora de combate
	b.players[1].teleportTo(Vec2{X: 0, Z: 6})
	b.UseGate(context.Background(), 1, gateIn(b))
	pump(m.Zones()...)
	back := a.players[1]
	if back == nil || back.pl.disc.town != "clareira" || !a.inSafe(back.pos) {
		t.Fatal("voltou ao Vale com a clareira como última cidade")
	}
	back.pl.inv.add(item("scroll_return"), 3)
	use := func(e *entity) {
		z := p.owner.Load()
		z.handle(itemEvent{id: 1, msg: &pb.UseItem{Place: placeBag, Slot: uint32(e.pl.inv.slotOf("scroll_return"))}})
	}
	use(back)
	if back.pl.channel == nil {
		t.Fatal("o pergaminho para outra cidade vale mesmo na praça")
	}
	a.tickFor(game.ReturnChannel + 0.3)
	pump(m.Zones()...)
	if p.owner.Load() != b || b.players[1] == nil || dist(b.players[1].pos, Vec2{X: 0, Z: 20}) > 5 {
		t.Fatalf("o pergaminho devia levar à clareira (dona %v)", p.owner.Load() == b)
	}
	// Volta ao Vale, passa pela praça (a última cidade vira Vel'Aran) e lê o pergaminho na estrada.
	b.tickFor(11)
	b.players[1].teleportTo(Vec2{X: 0, Z: 6})
	b.UseGate(context.Background(), 1, gateIn(b))
	pump(m.Zones()...)
	e := a.players[1]
	e.teleportTo(Vec2{X: town.pos.X + 1, Z: town.pos.Z})
	a.tickFor(0.2)
	if e.pl.disc.town != "cidade" {
		t.Fatalf("a última cidade é %q", e.pl.disc.town)
	}
	e.teleportTo(Vec2{X: 60})
	a.tickFor(game.ReturnCooldown)
	use(e)
	a.tickFor(game.ReturnChannel + 0.3)
	if !a.inSafe(e.pos) || p.owner.Load() != a {
		t.Fatalf("o pergaminho devia levar à cidade de Vel'Aran: %+v", e.pos)
	}
}

package zone

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// routedFake é o fakePeer com o lado "roteado" da conexão (como net.session).
type routedFake struct {
	fakePeer
	owner  atomic.Pointer[Zone]
	isDead atomic.Bool
}

func (r *routedFake) SetZone(z *Zone) { r.owner.Store(z) }
func (r *routedFake) Closed() bool    { return r.isDead.Load() }

func (r *routedFake) zoneChanges() []*pb.ZoneChange {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*pb.ZoneChange
	for _, m := range r.msgs {
		if c := m.GetZoneChange(); c != nil {
			out = append(out, c)
		}
	}
	return out
}

func (r *routedFake) lastWelcome() *pb.Welcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.msgs) - 1; i >= 0; i-- {
		if w := r.msgs[i].GetWelcome(); w != nil {
			return w
		}
	}
	return nil
}

func (r *routedFake) lastParty() *pb.PartyState {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.msgs) - 1; i >= 0; i-- {
		if p := r.msgs[i].GetParty(); p != nil {
			return p
		}
	}
	return nil
}

// twoZones: o mapa principal e o de testes (mundos de verdade, sem monstros), num
// gerente que ninguém roda: os testes chamam pump para tratar os eventos.
func twoZones(t testing.TB, saves chan []SavedState) (*Manager, *Zone, *Zone) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	enabled := []string{world.StarterMap, MapTest}
	var zs []*Zone
	for _, id := range enabled {
		cfg, err := MapConfig(testConfig(), id, enabled)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Spawns = nil
		var ch chan<- []SavedState
		if saves != nil {
			ch = saves
		}
		zs = append(zs, New(cfg, log, ch))
	}
	m := NewManager(log, zs...)
	return m, zs[0], zs[1]
}

// pump trata os eventos na fila das zonas até todas esvaziarem.
func pump(zs ...*Zone) {
	for busy := true; busy; {
		busy = false
		for _, z := range zs {
			for {
				select {
				case e := <-z.events:
					z.handle(e)
					busy = true
					continue
				default:
				}
				break
			}
		}
	}
}

// joinAt entra pelo gerente (como o net faz) no ponto (x, z) do mapa gravado.
func mjoin(m *Manager, id uint32, name, mapID string, x, z float64) *routedFake {
	p := &routedFake{}
	m.Join(context.Background(), JoinInfo{ID: id, Name: name, Class: "knight", X: x, Z: z, Zone: mapID,
		Gold: 500, Items: game.StarterKit("knight"), AccountID: int64(id), Peer: p})
	pump(m.Zones()...)
	return p
}

func gateIn(z *Zone) uint32 {
	for id, n := range z.npcs {
		if n.npc.role == pb.NpcRole_NPC_ROLE_GATE {
			return id
		}
	}
	return 0
}

func useGate(m *Manager, p *routedFake, id uint32) {
	z := p.owner.Load()
	z.UseGate(context.Background(), id, gateIn(z))
	pump(m.Zones()...)
}

func TestGateHandsOverWithOneOwner(t *testing.T) {
	saves := make(chan []SavedState, 64)
	m, a, b := twoZones(t, saves)
	p := mjoin(m, 1, "Ana", "", 0, -14)
	before := a.players[1]
	gold, items := before.pl.inv.gold, len(before.pl.inv.records())

	useGate(m, p, 1)

	if a.players[1] != nil || b.players[1] == nil {
		t.Fatalf("depois do portão: em A %v, em B %v", a.players[1] != nil, b.players[1] != nil)
	}
	if p.owner.Load() != b || m.ZoneOf(1) != MapTest {
		t.Fatalf("a conexão não mudou de dona: %v", m.ZoneOf(1))
	}
	got := b.players[1]
	if got.pl.inv.gold != gold || len(got.pl.inv.records()) != items {
		t.Fatalf("estado mudou no portão: ouro %d→%d, itens %d→%d", gold, got.pl.inv.gold, items, len(got.pl.inv.records()))
	}
	if dist(got.pos, Vec2{X: 0, Z: 1}) > 5 {
		t.Fatalf("chegou longe do ponto de chegada: %+v", got.pos)
	}
	zc := p.zoneChanges()
	if len(zc) != 1 || zc[0].Map != MapTest || zc[0].Title != "Clareira de Testes" {
		t.Fatalf("ZoneChange: %v", zc)
	}
	if w := p.lastWelcome(); w == nil || w.Map != MapTest {
		t.Fatalf("o Welcome do mapa novo não chegou: %v", w)
	}
	// A origem gravou o estado com a zona e a chegada do destino.
	var last SavedState
	for len(saves) > 0 {
		for _, s := range <-saves {
			if s.ID == 1 {
				last = s
			}
		}
	}
	if last.Zone != MapTest || last.Gold != gold || last.Z != 1 {
		t.Fatalf("gravação do portão: %+v", last)
	}
	if a.recent[1].state.ID != 0 {
		t.Fatal("a origem não pode guardar recent de quem atravessou")
	}
	if bad := m.bus.check(); bad != "" {
		t.Fatal(bad)
	}

	// E volta.
	useGate(m, p, 1)
	if a.players[1] == nil || b.players[1] != nil || p.owner.Load() != a {
		t.Fatal("não voltou para o mapa principal")
	}
	if w := p.lastWelcome(); w.Map != world.StarterMap {
		t.Fatalf("Welcome da volta: %q", w.Map)
	}
}

func TestGateRules(t *testing.T) {
	m, a, _ := twoZones(t, nil)
	p := mjoin(m, 1, "Ana", "", 0, -14)
	mjoin(m, 2, "Beto", "", 1, -14)
	ana := a.players[1]

	// Longe do portão.
	ana.teleportTo(Vec2{X: 0, Z: 20})
	useGate(m, p, 1)
	if a.players[1] == nil || p.systemCount("Chegue mais perto") != 1 {
		t.Fatal("atravessou de longe")
	}
	ana.teleportTo(Vec2{X: 0, Z: -14})

	// Em combate.
	ana.lastCombat = a.now
	useGate(m, p, 1)
	if a.players[1] == nil || p.systemCount("em combate") != 1 {
		t.Fatal("atravessou em combate")
	}
	ana.lastCombat = -1e9

	// Troca aberta.
	a.openTrade(ana, a.players[2])
	useGate(m, p, 1)
	if a.players[1] == nil || p.systemCount("Feche a troca") != 1 {
		t.Fatal("atravessou com a troca aberta")
	}
	a.closeTrade(ana.pl.trade, "")

	// Morto.
	ana.dead = true
	useGate(m, p, 1)
	if a.players[1] == nil || p.systemCount("Você está morto") != 1 {
		t.Fatal("atravessou morto")
	}
	ana.dead = false

	// Convite de grupo pendente e canalização: cancelados, e atravessa.
	a.partyInvite(a.players[2], ana)
	ana.pl.channel = &channelState{kind: pb.ChannelKind_CHANNEL_KIND_TELEPORT, until: a.now + 5}
	useGate(m, p, 1)
	if a.players[1] != nil {
		t.Fatal("não atravessou")
	}
	if a.pendingInviteTo(1) != nil {
		t.Fatal("o convite pendente ficou")
	}
}

func TestChatAndWhisperAcrossZones(t *testing.T) {
	m, a, b := twoZones(t, nil)
	pa := mjoin(m, 1, "Ana", "", 0, 0)
	pb_ := mjoin(m, 2, "Beto", MapTest, 0, 0)
	if a.players[1] == nil || b.players[2] == nil {
		t.Fatal("cada um no seu mapa")
	}
	a.handle(chatEvent{id: 1, text: "olá do vale"})
	b.handle(chatEvent{id: 2, text: "segredo", to: "ana"})
	if !hasChat(pb_, pb.ChatChannel_CHAT_CHANNEL_GENERAL, "olá do vale") {
		t.Fatal("chat geral não chegou à outra zona")
	}
	if !hasChat(pa, pb.ChatChannel_CHAT_CHANNEL_WHISPER, "segredo") || !hasChat(pb_, pb.ChatChannel_CHAT_CHANNEL_WHISPER, "segredo") {
		t.Fatal("sussurro entre zonas (com eco)")
	}
	b.handle(chatEvent{id: 2, text: "oi", to: "Ninguem"})
	if pb_.systemCount("não está online") != 1 {
		t.Fatal("sussurro para quem não existe")
	}
}

func hasChat(p *routedFake, ch pb.ChatChannel, text string) bool {
	for _, c := range p.chats() {
		if c.Channel == ch && c.Text == text {
			return true
		}
	}
	return false
}

func TestPartyAcrossZones(t *testing.T) {
	m, a, b := twoZones(t, nil)
	pa := mjoin(m, 1, "Ana", "", 0, -14)
	pbeto := mjoin(m, 2, "Beto", "", 1, -14)
	pc := mjoin(m, 3, "Caio", "", 1, -13)
	ana, beto := a.players[1], a.players[2]
	a.partyInvite(ana, beto)
	a.replyInvite(beto, a.pendingInviteTo(2).id, true)
	a.partyInvite(ana, a.players[3])
	a.replyInvite(a.players[3], a.pendingInviteTo(3).id, true)
	if ana.pl.party == nil || len(ana.pl.party.members) != 3 {
		t.Fatal("grupo não formou")
	}

	useGate(m, pbeto, 2) // Beto vai para a clareira e continua no grupo
	if b.players[2].pl.party == nil || m.bus.partyOf(2) == 0 {
		t.Fatal("saiu do grupo ao atravessar")
	}
	a.syncParties()
	ps := pa.lastParty()
	var other *pb.PartyMember
	for _, mem := range ps.Members {
		if mem.Id == 2 {
			other = mem
		}
	}
	if len(ps.Members) != 3 || other == nil || other.Map != "Clareira de Testes" {
		t.Fatalf("a janela do grupo não mostra o outro mapa: %v", ps.Members)
	}
	// XP do grupo: só quem está na mesma zona.
	if got := a.creditGroup(map[uint32]float64{1: 10}, ana.pos); len(got) != 2 {
		t.Fatalf("XP para %d (só os 2 do mesmo mapa)", len(got))
	}
	// Chat do grupo cruza zonas.
	a.handle(chatEvent{id: 1, text: "vem", channel: pb.ChatChannel_CHAT_CHANNEL_PARTY})
	if !hasChat(pbeto, pb.ChatChannel_CHAT_CHANNEL_PARTY, "vem") {
		t.Fatal("chat do grupo não chegou a outra zona")
	}
	// O líder expulsa quem está na outra zona: a zona dele avisa.
	a.partyKick(ana, 2)
	pump(a, b)
	if b.players[2].pl.party != nil || pbeto.systemCount("removido do grupo") != 1 {
		t.Fatal("expulsão entre zonas")
	}
	// Caio sai: o grupo acaba (Ana sozinha).
	a.partyLeave(a.players[3], "Você saiu do grupo.")
	if ana.pl.party != nil || len(a.parties) != 0 || pa.systemCount("O grupo acabou") != 1 {
		t.Fatal("o grupo devia acabar")
	}
	_ = pc
	if bad := m.bus.check(); bad != "" {
		t.Fatal(bad)
	}
	pump(a, b)
	a.checkParties()
	b.checkParties()
}

func TestPartyEndsInOtherZone(t *testing.T) {
	m, a, b := twoZones(t, nil)
	mjoin(m, 1, "Ana", "", 0, -14)
	pbeto := mjoin(m, 2, "Beto", "", 1, -14)
	a.partyInvite(a.players[1], a.players[2])
	a.replyInvite(a.players[2], a.pendingInviteTo(2).id, true)
	useGate(m, pbeto, 2)
	a.partyLeave(a.players[1], "Você saiu do grupo.") // sobrou o Beto, na clareira
	pump(a, b)
	if b.players[2].pl.party != nil || len(b.parties) != 0 || pbeto.systemCount("O grupo acabou") != 1 {
		t.Fatal("o grupo da outra zona não acabou")
	}
}

func TestDisconnectWhileCrossing(t *testing.T) {
	saves := make(chan []SavedState, 64)
	m, a, b := twoZones(t, saves)
	p := mjoin(m, 1, "Ana", "", 0, -14)
	a.players[1].pl.inv.gold = 777
	// A conexão cai com a entrega na fila do destino: o Leave foi para a zona antiga.
	a.UseGate(context.Background(), 1, gateIn(a))
	pump(a)
	p.isDead.Store(true)
	a.Leave(context.Background(), 1, p)
	pump(a, b)
	if a.players[1] != nil || b.players[1] != nil || m.bus.online(1) {
		t.Fatal("fantasma: a conexão caiu no portão e o personagem ficou")
	}
	if st := b.recent[1].state; st.Gold != 777 || st.Zone != MapTest {
		t.Fatalf("o recent do destino guarda o estado: %+v", st)
	}
	// Relogar logo vai para o destino (o recent vale mais que o banco).
	p2 := mjoin(m, 1, "Ana", "", 0, 0)
	if b.players[1] == nil || b.players[1].pl.inv.gold != 777 || p2.owner.Load() != b {
		t.Fatal("o relogin não foi para a zona onde ele estava")
	}
}

func TestReloginGoesToCurrentZone(t *testing.T) {
	m, a, b := twoZones(t, nil)
	p := mjoin(m, 1, "Ana", "", 0, -14)
	useGate(m, p, 1)
	p2 := mjoin(m, 1, "Ana", "", 0, -14) // o banco ainda pode dizer "starter"
	if a.players[1] != nil || b.players[1] == nil || b.players[1].pl.peer != p2 || p.kicked() == "" {
		t.Fatal("relogin com o personagem em outra zona")
	}
	if bad := m.bus.check(); bad != "" {
		t.Fatal(bad)
	}
}

func TestGateClosedWithoutDestination(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg, err := MapConfig(testConfig(), world.StarterMap, []string{world.StarterMap})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Spawns = nil
	z := New(cfg, log, nil)
	if gateIn(z) != 0 {
		t.Fatal("sem o mapa de destino, o portão não aparece (produção não muda)")
	}
}

func TestGMKickAcrossZones(t *testing.T) {
	m, a, b := twoZones(t, nil)
	gm := mjoin(m, 1, "Mestre", "", 0, 0)
	a.players[1].pl.gm = true
	pc := mjoin(m, 2, "Caio", MapTest, 0, 0)
	a.handle(chatEvent{id: 1, text: "/kick caio flood"})
	pump(a, b)
	if b.players[2] != nil || !strings.Contains(pc.kicked(), "GM") {
		t.Fatal("/kick não alcançou a outra zona")
	}
	if gm.systemCount("foi desconectado") != 1 {
		t.Fatal("o GM não recebeu a resposta")
	}
}

func TestEnabledMaps(t *testing.T) {
	if got := EnabledMaps(false, ""); len(got) != 1 || got[0] != world.StarterMap {
		t.Fatalf("produção: só o mapa principal, veio %v", got)
	}
	if got := EnabledMaps(true, ""); len(got) != 2 || got[1] != MapTest {
		t.Fatalf("dev: %v", got)
	}
	if got := EnabledMaps(false, "teste, nada"); len(got) != 2 {
		t.Fatalf("EREVIA_ZONES: %v", got)
	}
}

// Os raros são do mapa principal: na clareira, o hospedeiro (o mesmo esqueleto) nunca
// renasce raro, mesmo com a chance forçada (senão haveria dois do mesmo raro no mundo e
// duas zonas gravando a mesma linha da hora da aparição).
func TestRaresOnlyOnTheMainMap(t *testing.T) {
	now := noon
	z := rareZone(&now, fixed("skeleton_minion", 3, 60, 0))
	z.cfg.NoRares = true
	m := z.onlyMonster()
	z.respawn(t, m)
	if m.mon.def.Rare != nil {
		t.Fatalf("raro (%s) numa zona sem raros", m.mon.def.ID)
	}
	cfg, err := MapConfig(testConfig(), MapTest, []string{world.StarterMap, MapTest})
	if err != nil || !cfg.NoRares {
		t.Fatalf("a clareira tem de vir sem raros (%v)", err)
	}
	if cfg, _ := MapConfig(testConfig(), world.StarterMap, []string{world.StarterMap}); cfg.NoRares {
		t.Fatal("o mapa principal tem raros")
	}
}

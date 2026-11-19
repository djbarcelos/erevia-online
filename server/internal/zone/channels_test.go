package zone

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Canais do mapa principal (tunnel#86).

// channelZones: o mapa principal com n canais (cada um com capacidade cap) e o mapa de
// testes, num gerente que ninguém roda (os testes chamam pump).
func channelZones(t testing.TB, n, capacity int) (*Manager, []*Zone, *Zone) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	enabled := []string{world.StarterMap, MapTest}
	home, err := MapConfig(testConfig(), world.StarterMap, enabled)
	if err != nil {
		t.Fatal(err)
	}
	home.Spawns, home.Capacity, home.Channel = nil, capacity, 1
	test, err := MapConfig(testConfig(), MapTest, enabled)
	if err != nil {
		t.Fatal(err)
	}
	test.Spawns = nil
	ch1, clearing := New(home, log, nil), New(test, log, nil)
	m := NewManager(log, ch1, clearing)
	chans := []*Zone{ch1}
	for i := 2; i <= n; i++ {
		z := New(ChannelConfig(home, i), log, nil)
		m.AddChannel(z)
		chans = append(chans, z)
	}
	return m, chans, clearing
}

func cjoin(m *Manager, id uint32, channel int) (*routedFake, *Zone) {
	p := &routedFake{}
	z := m.Join(context.Background(), JoinInfo{ID: id, Name: "Ana" + string(rune('A'+id%26)) + string(rune('a'+id/26%26)), Class: "knight",
		X: 0, Z: 5, Gold: 100, Items: game.StarterKit("knight"), AccountID: int64(id), Peer: p, Channel: channel})
	pump(m.Zones()...)
	return p, z
}

func (r *routedFake) lastChannels() *pb.ChannelList {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.msgs) - 1; i >= 0; i-- {
		if c := r.msgs[i].GetChannels(); c != nil {
			return c
		}
	}
	return nil
}

func TestChannelJoinPicksRequestedOrLeastFull(t *testing.T) {
	m, ch, _ := channelZones(t, 3, 2)
	// Sem pedido: o menos cheio (o de número menor no empate).
	_, z := cjoin(m, 1, 0)
	if z != ch[0] {
		t.Fatal("o primeiro vai para o Canal 1")
	}
	_, z = cjoin(m, 2, 0)
	if z != ch[1] {
		t.Fatal("o segundo vai para o menos cheio (Canal 2)")
	}
	// Pedido: o canal pedido, se cabe.
	p, z := cjoin(m, 3, 1)
	if z != ch[0] || ch[0].players[3] == nil {
		t.Fatal("pediu o Canal 1 e ele cabe")
	}
	if w := p.lastWelcome(); w == nil || w.Channel != 1 {
		t.Fatalf("o Welcome diz o canal: %+v", w)
	}
	if l := p.lastChannels(); l == nil || l.Current != 1 || len(l.Channels) != 3 || !l.Channels[0].Full || l.Channels[0].Players != 2 {
		t.Fatalf("a lista de canais vai ao entrar: %+v", l)
	}
	// Cheio: vai para outro, com aviso.
	p, z = cjoin(m, 4, 1)
	if z == ch[0] || z == nil {
		t.Fatal("o Canal 1 está cheio: vai para outro")
	}
	z.runAfterSnapshot()
	if !p.systemContains("O Canal 1 está cheio") {
		t.Fatal("avisa que o pedido estava cheio")
	}
	// Todos cheios: recusa.
	cjoin(m, 5, 0)
	cjoin(m, 6, 0)
	if _, z := cjoin(m, 7, 0); z != nil {
		t.Fatal("com todos os canais cheios, a entrada é recusada")
	}
}

func TestChangeChannelInTownCarriesTheCharacter(t *testing.T) {
	m, ch, _ := channelZones(t, 2, 0)
	p, z := cjoin(m, 1, 1)
	if z != ch[0] {
		t.Fatal("entrou no Canal 1")
	}
	ana := ch[0].players[1]
	ana.pl.inv.gold = 777
	pos := ana.pos

	ch[0].ChangeChannel(context.Background(), 1, 2)
	pump(m.Zones()...)
	got := ch[1].players[1]
	if ch[0].players[1] != nil || got == nil || p.owner.Load() != ch[1] {
		t.Fatal("trocou de canal: sai do 1 e entra no 2, com a conexão junto")
	}
	if got.pl.inv.gold != 777 || dist(got.pos, pos) > 3 {
		t.Fatalf("o personagem vem igual e no mesmo lugar: %d moedas, %+v", got.pl.inv.gold, got.pos)
	}
	if w := p.lastWelcome(); w == nil || w.Channel != 2 || w.Map != world.StarterMap {
		t.Fatalf("Welcome do Canal 2: %+v", w)
	}
	if bad := m.bus.check(); bad != "" {
		t.Fatal(bad)
	}

	// Fora da cidade não troca.
	got.pos = Vec2{X: ch[1].cfg.SafeRadius + 20, Z: 0}
	ch[1].ChangeChannel(context.Background(), 1, 1)
	pump(m.Zones()...)
	if ch[1].players[1] == nil || !p.systemContains("dentro da cidade") {
		t.Fatal("fora da cidade, não troca de canal")
	}
	// Em combate não troca.
	got.pos = Vec2{X: 0, Z: 5}
	got.lastCombat = ch[1].now
	ch[1].ChangeChannel(context.Background(), 1, 1)
	pump(m.Zones()...)
	if ch[1].players[1] == nil || !p.systemContains("em combate") {
		t.Fatal("em combate, não troca de canal")
	}
}

func TestChannelTwoLeavesWorldThingsToChannelOne(t *testing.T) {
	m, ch, clearing := channelZones(t, 2, 0)
	cfg := ch[1].cfg
	if !cfg.NoRares || cfg.InvasionChance != 0 || cfg.Bosses != nil || cfg.Clans != nil || len(cfg.Dungeons) != 0 {
		t.Fatalf("o Canal 2 não tem raros, invasão, chefes, clãs nem masmorras: %+v", cfg)
	}
	for _, g := range cfg.Gates {
		if g.Dungeon != "" {
			t.Fatal("a porta da masmorra fica só no Canal 1")
		}
	}
	for _, s := range ChannelConfig(func() Config { c, _ := MapConfig(testConfig(), world.StarterMap, nil); return c }(), 2).Spawns {
		if d := game.Monsters[s.Monster]; d.Boss || d.Raid || d.FieldBoss != nil {
			t.Fatalf("o Canal 2 não tem o chefe %s", s.Monster)
		}
	}

	// Armazém e clã: no Canal 1.
	p, _ := cjoin(m, 1, 2)
	ana := ch[1].players[1]
	keeper := uint32(0)
	for id, n := range ch[1].npcs {
		if n.npc.role == pb.NpcRole_NPC_ROLE_STORAGE {
			keeper = id
			ana.pos = n.pos
		}
	}
	ana.pl.moveTokens = ch[1].cfg.MaxMovesPerSec
	ch[1].handle(itemEvent{id: 1, msg: &pb.StorageGold{NpcId: keeper}})
	if !p.systemContains("O armazém da conta fica no Canal 1") {
		t.Fatal("o Guarda do Armazém manda ir ao Canal 1")
	}
	ana.pl.moveTokens = ch[1].cfg.MaxMovesPerSec
	ch[1].handle(socialEvent{id: 1, msg: &pb.ClanCmd{Kind: pb.ClanCmdKind(1), Name: "Lobos"}})
	if !p.systemContains("Canal 1") || ch[1].clanOf[1] != nil {
		t.Fatal("o clã só é gerenciado no Canal 1")
	}

	// De outro mapa para o principal: o canal menos cheio.
	if m.Arrival(world.StarterMap) != ch[0] {
		t.Fatal("chegando de fora, o canal menos cheio (o Canal 1, vazio)")
	}
	if m.Arrival(MapTest) != clearing {
		t.Fatal("os outros mapas não têm canais")
	}
}

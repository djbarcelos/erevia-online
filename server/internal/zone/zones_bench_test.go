package zone

import (
	"context"
	"fmt"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// BenchmarkGateHop: uma travessia de portão (sair de uma zona, gravar, entregar e entrar
// na outra, com o Welcome inteiro), ida e volta contam 2. Mede o custo nas goroutines
// das zonas, que é o que tira tempo do tick.
// go test ./internal/zone -run x -bench GateHop -benchmem
func BenchmarkGateHop(b *testing.B) {
	m, a, z2 := twoZones(b, nil)
	ctx := context.Background()
	p := &botPeer{}
	m.Join(ctx, JoinInfo{ID: 1, Name: "Ana", Class: "knight", X: 0, Z: -14, Gold: 500, Items: game.StarterKit("knight"), Peer: p})
	pump(a, z2)
	gates := map[*Zone]uint32{a: gateIn(a), z2: gateIn(z2)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		z := p.owner.Load()
		pl := z.players[1]
		pl.pl.moveTokens = 10
		z.useGate(1, gates[z])
		pump(a, z2)
	}
	b.StopTimer()
	if m.Hops() != int64(b.N) {
		b.Fatalf("%d travessias de %d", m.Hops(), b.N)
	}
}

// BenchmarkBusBroadcast: uma mensagem do chat geral para 500 jogadores em 2 zonas
// (serializa 1 vez e entrega pela conexão de cada um).
func BenchmarkBusBroadcast(b *testing.B) {
	m, a, z2 := twoZones(b, nil)
	ctx := context.Background()
	for i := range 500 {
		mapID := ""
		if i%2 == 1 {
			mapID = MapTest
		}
		m.Join(ctx, JoinInfo{ID: uint32(i + 1), Name: fmt.Sprintf("Bot%03d", i), Class: "knight", Zone: mapID, Peer: discardPeer{}})
		pump(a, z2)
	}
	msg := &pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_GENERAL, FromId: 1, FromName: "Bot000", Text: "olá a todos",
	}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.broadcastAll(msg)
	}
}

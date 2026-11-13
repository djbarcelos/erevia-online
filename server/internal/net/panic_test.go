package net

import (
	"context"
	"testing"
	"time"

	"erevia/server/internal/pb"
)

// Um panic no writePump (goroutine sem recover acima) derruba só a conexão: é registrado
// e o contexto da sessão é cancelado (o readPump termina e a zona recebe o Leave).
func TestWritePumpPanicIsRecovered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var where string
	sess := &session{
		conn: nil, // escrever num Conn nulo dá panic
		send: make(chan []byte, 1), closing: make(chan string, 1), cancel: cancel,
		report: func(w string, _ any, _ []byte) { where = w },
	}
	sess.send <- []byte{1}
	done := make(chan struct{})
	go func() {
		sess.writePump(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("writePump must return after a panic")
	}
	if where != "net:writePump" {
		t.Fatalf("panic must be reported, where = %q", where)
	}
	if ctx.Err() == nil {
		t.Fatal("session context must be cancelled")
	}
}

func TestMsgCase(t *testing.T) {
	m := &pb.ClientMessage{Msg: &pb.ClientMessage_PickUp{PickUp: &pb.PickUp{Id: 1}}}
	if got := msgCase(m); got != "PickUp" {
		t.Fatalf("msgCase = %q", got)
	}
	if got := msgCase(&pb.ClientMessage{}); got != "" {
		t.Fatalf("empty msgCase = %q", got)
	}
}

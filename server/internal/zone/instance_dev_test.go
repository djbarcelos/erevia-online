package zone

import (
	"testing"
)

// /catacumbas (só com DEV_LOGIN): entra na cópia de onde estiver, como a porta (os testes de
// arte e de FPS do cliente entram assim). Sem DEV_LOGIN, o comando não existe.
func TestDungeonDevCommand(t *testing.T) {
	e := newDungeonEnv(t)
	p := e.join(1, "Ana")
	e.home.handle(chatEvent{id: 1, text: "/catacumbas"})
	e.pumpAll()
	if p.owner.Load().cfg.Instance != "" || p.systemCount("Comando desconhecido") != 1 {
		t.Fatal("sem DEV_LOGIN o comando não existe")
	}
	e.home.cfg.DevCommands = true
	e.home.handle(chatEvent{id: 1, text: "/catacumbas"})
	e.pumpAll()
	in := p.owner.Load()
	if in.cfg.Instance == "" || in.players[1] == nil {
		t.Fatal("o comando leva à cópia")
	}
	// Dentro da cópia o comando não vale (a cópia não herda os comandos de teste).
	in.handle(chatEvent{id: 1, text: "/catacumbas"})
	e.pumpAll()
	if got := len(e.m.Instances()); got != 1 {
		t.Fatalf("%d cópias", got)
	}
}

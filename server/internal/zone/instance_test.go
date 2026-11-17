package zone

import (
	"context"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Masmorra 1 (tunnel#140): a cópia de grupo das Catacumbas. Os testes usam o mapa de verdade
// (o que o gerador gravou), o gerente sem rodar (manual: quem testa trata os eventos e os
// ticks) e um relógio controlado.

type dungeonEnv struct {
	t    *testing.T
	m    *Manager
	home *Zone
	now  *time.Time
}

func newDungeonEnv(t *testing.T) *dungeonEnv {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := noon
	base := testConfig()
	base.Clock = func() time.Time { return now }
	base.Dungeons = []string{game.CatacombsID}
	cfg, err := MapConfig(base, world.StarterMap, []string{world.StarterMap})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Spawns = nil
	home := New(cfg, log, nil)
	m := NewManager(log, home)
	// A telemetria da cópia vai para a fila da zona (sem o worker, que só roda no Run).
	m.EnableDungeons(base, func(cfg Config, l *slog.Logger) (*Zone, <-chan struct{}) {
		z := New(cfg, l, nil)
		z.SetEventSink(nopSink{})
		return z, nil
	})
	m.inst.manual = true
	return &dungeonEnv{t: t, m: m, home: home, now: &now}
}

// pumpAll trata os eventos das zonas fixas e das cópias até todas esvaziarem.
func (e *dungeonEnv) pumpAll() {
	// A cópia nasce no meio da rodada: a lista de zonas é refeita até não sobrar evento.
	for {
		before := len(e.m.AllZones())
		pump(e.m.AllZones()...)
		if len(e.m.AllZones()) == before {
			return
		}
	}
}

// join entra no mapa principal, ao lado da porta da igreja.
func (e *dungeonEnv) join(id uint32, name string) *routedFake {
	e.t.Helper()
	p := &routedFake{}
	e.m.Join(context.Background(), JoinInfo{ID: id, Name: name, Class: "knight", X: -13, Z: 13, Zone: "",
		Gold: 500, Items: game.StarterKit("knight"), AccountID: int64(id), Peer: p})
	e.pumpAll()
	return p
}

func (e *dungeonEnv) door() uint32 {
	e.t.Helper()
	for id, n := range e.home.npcs {
		if g := e.home.gateOf(n); g != nil && g.Dungeon == game.CatacombsID {
			return id
		}
	}
	e.t.Fatal("a porta das Catacumbas não existe no mapa principal")
	return 0
}

// enter usa a porta e devolve a cópia onde o jogador está agora (nil = ficou fora).
func (e *dungeonEnv) enter(p *routedFake, id uint32) *Zone {
	e.t.Helper()
	z := p.owner.Load()
	z.UseGate(context.Background(), id, e.door())
	e.pumpAll()
	if z = p.owner.Load(); z.cfg.Instance == "" {
		return nil
	}
	return z
}

// advance anda o relógio e roda 1 s de tick em cada cópia (o necessário para a vida dela).
func (e *dungeonEnv) advance(d time.Duration) {
	*e.now = e.now.Add(d)
	for _, z := range e.m.AllZones() {
		if z.cfg.Instance != "" {
			z.updateInstance()
			z.tickFor(0.1) // depois do snapshot: a evacuação sai aqui
		}
	}
	e.pumpAll()
}

func (z *Zone) bossOf(t *testing.T) *entity {
	t.Helper()
	for _, m := range z.monsters {
		if m.mon.def.ID == game.AbadeUlric {
			return m
		}
	}
	t.Fatal("a cópia não tem o Abade Ulric")
	return nil
}

func TestDungeonDoorAndConfig(t *testing.T) {
	e := newDungeonEnv(t)
	e.door()
	cfg, err := DungeonConfig(e.home.cfg, game.CatacombsID, "catacumbas#1")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.NoTown || !cfg.NoRares || cfg.InvasionChance != 0 || len(cfg.Clans) != 0 || cfg.Map != "catacumbas" || cfg.Instance == "" {
		t.Fatalf("a cópia é uma zona sem cidade, raros, invasão nem clãs: %+v", cfg)
	}
	mobs, bosses := 0, 0
	for _, s := range cfg.Spawns {
		def := game.Monsters[s.Monster]
		if def == nil {
			t.Fatalf("spawn de monstro desconhecido: %s", s.Monster)
		}
		mobs += s.Count
		if def.Boss {
			bosses++
		}
		if s.Respawn < 45*60 {
			t.Errorf("%s renasce em %.0f s: a cópia vive 45 min, é uma luta só", s.Monster, s.Respawn)
		}
		if s.LevelMin < game.CatacombsMinLevel || s.LevelMin > game.CatacombsMaxLevel {
			t.Errorf("%s no nível %d, fora da faixa 12–16", s.Monster, s.LevelMin)
		}
	}
	if bosses != 1 || mobs > game.CatacombsMobCap-4 {
		t.Fatalf("%d monstros e %d chefes (teto de %d com as 4 invocações)", mobs, bosses, game.CatacombsMobCap)
	}
	// Duas escadas e a saída são portões da cópia.
	var stairs, exits int
	for _, g := range cfg.Gates {
		switch {
		case g.Stairs:
			stairs++
		case g.To == world.StarterMap:
			exits++
		}
	}
	if stairs != 2 || exits != 1 {
		t.Fatalf("portões da cópia: %d escadas e %d saídas", stairs, exits)
	}
	// Sem a masmorra ligada, o mapa principal não tem a porta.
	off := testConfig()
	cfg2, _ := MapConfig(off, world.StarterMap, []string{world.StarterMap})
	for _, g := range cfg2.Gates {
		if g.Dungeon != "" {
			t.Fatal("a porta só existe com a masmorra ligada")
		}
	}
	if got := EnabledDungeons("off"); len(got) != 0 {
		t.Fatalf("EREVIA_DUNGEONS=off: %v", got)
	}
	if got := EnabledDungeons(""); len(got) != 1 || got[0] != game.CatacombsID {
		t.Fatalf("por padrão a masmorra está ligada: %v", got)
	}
}

func TestDungeonSoloEntersAndLeaderCreatesTheCopy(t *testing.T) {
	e := newDungeonEnv(t)
	p := e.join(1, "Ana")
	in := e.enter(p, 1)
	if in == nil {
		t.Fatal("sozinho também entra (grupo de 1)")
	}
	if got := p.zoneChanges(); len(got) != 1 || got[0].Map != "catacumbas" || got[0].Title != "Catacumbas de Vel'Aran" {
		t.Fatalf("a mudança de zona leva o mapa da masmorra: %+v", got)
	}
	if e.m.ZoneOf(1) != "catacumbas" || in.players[1] == nil || e.home.players[1] != nil {
		t.Fatal("o jogador tem uma dona só: a cópia")
	}
	if w := p.lastWelcome(); w == nil || w.Map != "catacumbas" || w.MapTitle != "Catacumbas de Vel'Aran" {
		t.Fatalf("Welcome da cópia: %+v", w)
	}
	if math.Hypot(in.players[1].pos.X, in.players[1].pos.Z) > 4 {
		t.Fatalf("chega na entrada (a origem), não em %v", in.players[1].pos)
	}
	if bad := e.m.bus.check(); bad != "" {
		t.Fatal(bad)
	}
	// O Welcome mostra só o chefe no mapa (não as dezenas de monstros).
	var zones []*pb.MapZone
	if w := p.lastWelcome(); w != nil {
		zones = w.Zones
	}
	if len(zones) != 1 || !zones[0].Boss {
		t.Fatalf("o mapa mostra só o chefe: %d zonas", len(zones))
	}
}

func TestDungeonSeveralCopiesAtOnceAreIsolated(t *testing.T) {
	e := newDungeonEnv(t)
	pa, pb2, pc := e.join(1, "Ana"), e.join(2, "Beto"), e.join(3, "Caio")
	za, zb := e.enter(pa, 1), e.enter(pb2, 2)
	if za == nil || zb == nil || za == zb {
		t.Fatal("cada grupo tem a sua cópia")
	}
	if len(e.m.Instances()) != 2 {
		t.Fatalf("%d cópias vivas, esperava 2", len(e.m.Instances()))
	}
	// Os monstros são de cada cópia: matar o chefe de uma não mexe na outra.
	a1, b1 := za.bossOf(t), zb.bossOf(t)
	za.kill(a1, za.players[1])
	za.tickFor(0.1)
	if !a1.dead || b1.dead {
		t.Fatal("o chefe de uma cópia caiu e o da outra não")
	}
	// A AOI não mistura: ninguém vê quem está em outra cópia.
	if len(za.players) != 1 || len(zb.players) != 1 {
		t.Fatal("uma cópia por jogador")
	}
	// Chat geral: não atravessa a porta, nem para dentro nem para fora.
	for _, c := range []*routedFake{pa, pb2, pc} {
		c.mu.Lock()
		c.msgs = nil
		c.mu.Unlock()
	}
	za.handle(chatEvent{id: 1, text: "socorro"})
	if hasChat(pa, pb.ChatChannel_CHAT_CHANNEL_GENERAL, "socorro") != true || hasChat(pb2, pb.ChatChannel_CHAT_CHANNEL_GENERAL, "socorro") || hasChat(pc, pb.ChatChannel_CHAT_CHANNEL_GENERAL, "socorro") {
		t.Fatal("o chat geral de dentro fica dentro")
	}
	e.home.handle(chatEvent{id: 3, text: "alguém aí?"})
	if !hasChat(pc, pb.ChatChannel_CHAT_CHANNEL_GENERAL, "alguém aí?") || hasChat(pa, pb.ChatChannel_CHAT_CHANNEL_GENERAL, "alguém aí?") || hasChat(pb2, pb.ChatChannel_CHAT_CHANNEL_GENERAL, "alguém aí?") {
		t.Fatal("o chat geral de fora não entra nas cópias")
	}
	// Sussurro cruza: avisar o amigo lá dentro é permitido.
	e.home.handle(chatEvent{id: 3, text: "e aí", to: "Ana"})
	if !hasChat(pa, pb.ChatChannel_CHAT_CHANNEL_WHISPER, "e aí") {
		t.Fatal("o sussurro chega à cópia")
	}
	// As cópias não aparecem como zonas fixas.
	if len(e.m.Zones()) != 1 || e.m.Zone("catacumbas") != nil {
		t.Fatal("a cópia não entra no mapa de zonas do processo")
	}
}

func TestDungeonPartyJoinWindow(t *testing.T) {
	e := newDungeonEnv(t)
	pa, pb2, pc := e.join(1, "Ana"), e.join(2, "Beto"), e.join(3, "Caio")
	ana, beto := e.home.players[1], e.home.players[2]
	e.home.partyInvite(ana, beto)
	e.home.replyInvite(beto, e.home.pendingInviteTo(2).id, true)
	e.home.partyInvite(ana, e.home.players[3])
	e.home.replyInvite(e.home.players[3], e.home.pendingInviteTo(3).id, true)

	// Quem não é o líder não abre a porta.
	if z := e.enter(pb2, 2); z != nil || pb2.systemCount("Só o líder") != 1 {
		t.Fatal("só o líder cria a cópia")
	}
	in := e.enter(pa, 1)
	if in == nil {
		t.Fatal("o líder entra")
	}
	// Um minuto depois, o Beto entra na mesma cópia; ele é do mesmo grupo.
	e.advance(time.Minute)
	if z := e.enter(pb2, 2); z != in {
		t.Fatal("o membro entra na cópia do líder dentro dos 2 minutos")
	}
	if got := len(e.m.Instances()); got != 1 {
		t.Fatalf("%d cópias para um grupo", got)
	}
	// Depois dos 2 minutos, a porta fecha para quem ainda não entrou; quem entrou volta.
	e.advance(90 * time.Second)
	if z := e.enter(pc, 3); z != nil || pc.systemCount("fechou") != 1 {
		t.Fatalf("passou dos 2 minutos: a porta fecha (%v)", z)
	}
	if bad := e.m.bus.check(); bad != "" {
		t.Fatal(bad)
	}
	// Quem já entrou volta pela mesma porta enquanto a cópia existir: o Beto sai e volta.
	beto = in.players[2]
	in.crossZone(beto, e.home, dungeonDefs[game.CatacombsID].exit, "gate")
	in.tickFor(0.1)
	e.pumpAll()
	if e.home.players[2] == nil {
		t.Fatal("o Beto saiu para a igreja")
	}
	if z := e.enter(pb2, 2); z != in {
		t.Fatal("quem já entrou volta à cópia depois dos 2 minutos")
	}
}

func TestDungeonCopyClosesFiveMinutesAfterEmpty(t *testing.T) {
	e := newDungeonEnv(t)
	p := e.join(1, "Ana")
	in := e.enter(p, 1)
	if in == nil {
		t.Fatal("entrou")
	}
	inst := e.m.Instances()[0]
	// Ana sai pela porta de saída (o portão da cópia leva à igreja).
	exit := uint32(0)
	for id, n := range in.npcs {
		if g := in.gateOf(n); g != nil && g.To == world.StarterMap {
			exit = id
		}
	}
	in.players[1].pos = in.npcs[exit].pos
	in.players[1].lastCombat = -1e9
	in.UseGate(context.Background(), 1, exit)
	e.pumpAll()
	if e.home.players[1] == nil || in.players[1] != nil {
		t.Fatal("a saída leva à igreja")
	}
	pos := e.home.players[1].pos
	d := dungeonDefs[game.CatacombsID]
	if math.Hypot(pos.X-d.exit.X, pos.Z-d.exit.Z) > 4 {
		t.Fatalf("sai ao lado da porta da igreja, não em %v", pos)
	}
	// Vazia: a cópia espera 5 minutos e some.
	e.advance(time.Second) // a cópia vê que está vazia
	e.advance(4 * time.Minute)
	if len(e.m.Instances()) != 1 || in.dg == nil {
		t.Fatal("com 4 minutos de vazia, a cópia ainda vive")
	}
	e.advance(61 * time.Second)
	e.advance(5 * time.Second) // o prazo da última entrega
	if in.dg != nil {
		t.Fatal("passou dos 5 minutos: a cópia fecha")
	}
	// A telemetria sai com a duração e as mortes.
	var ev *GameEvent
	for len(in.telemetry) > 0 {
		x := <-in.telemetry
		if x.Kind == "dungeon" {
			ev = &x
		}
	}
	if ev == nil || ev.Ref != "catacumbas" || ev.Amount < 5*60 || ev.Extra["deaths"] != 0 || ev.Extra["reason"] != "vazia" || ev.Extra["players"] != 1 {
		t.Fatalf("telemetria da cópia: %+v", ev)
	}
	// Em produção a goroutine da cópia a tira do gerente quando a zona para; aqui, na mão.
	e.m.dropInstance(inst)
	if len(e.m.Instances()) != 0 || !in.isClosed() {
		t.Fatal("a cópia fechada sai do gerente")
	}
	// Quem tenta entrar de novo ganha uma cópia nova (a velha fechou).
	if z := e.enter(p, 1); z == nil || z == in {
		t.Fatal("a cópia nova é outra zona")
	}
}

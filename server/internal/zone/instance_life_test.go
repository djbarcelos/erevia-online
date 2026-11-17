package zone

import (
	"context"
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/world"
)

// Vida da cópia (45 minutos, evacuação, reentrada), escadas, baú do dia e dados dos monstros.

func TestDungeonForty5MinutesEvacuatesToTheChurchDoor(t *testing.T) {
	e := newDungeonEnv(t)
	pa, pb2 := e.join(1, "Ana"), e.join(2, "Beto")
	ana, beto := e.home.players[1], e.home.players[2]
	e.home.partyInvite(ana, beto)
	e.home.replyInvite(beto, e.home.pendingInviteTo(2).id, true)
	in := e.enter(pa, 1)
	if in == nil || e.enter(pb2, 2) != in {
		t.Fatal("o grupo entrou")
	}
	inst := e.m.Instances()[0]
	// Aos 40 minutos, o aviso de 5 minutos.
	e.advance(40 * time.Minute)
	if pa.systemCount("desmoronam em 5 minutos") != 1 {
		t.Fatal("aviso de 5 minutos antes do fim")
	}
	e.advance(5*time.Minute + time.Second)
	if e.home.players[1] == nil || e.home.players[2] == nil || len(in.players) != 0 {
		t.Fatalf("aos 45 minutos todos saem para a igreja (em casa: %v %v, na cópia: %d)", e.home.players[1] != nil, e.home.players[2] != nil, len(in.players))
	}
	d := dungeonDefs[game.CatacombsID]
	for _, p := range e.home.players {
		if math.Hypot(p.pos.X-d.exit.X, p.pos.Z-d.exit.Z) > 4 {
			t.Fatalf("%s saiu longe da porta da igreja: %v", p.name, p.pos)
		}
	}
	// Com a porta da cópia fechando, ninguém mais entra.
	if z := e.enter(pa, 1); z == in {
		t.Fatal("a porta da cópia fechando não deixa entrar")
	}
	e.advance(5 * time.Second)
	if in.dg != nil {
		t.Fatal("a cópia parou")
	}
	var ev *GameEvent
	for len(in.telemetry) > 0 {
		if x := <-in.telemetry; x.Kind == "dungeon" {
			ev = &x
		}
	}
	if ev == nil || ev.Amount < 45*60 || ev.Extra["reason"] != "tempo" || ev.Extra["players"] != 2 {
		t.Fatalf("telemetria dos 45 minutos: %+v", ev)
	}
	e.m.dropInstance(inst)
	if bad := e.m.bus.check(); bad != "" {
		t.Fatal(bad)
	}
}

func TestDungeonLoginWithoutTheCopyLandsAtTheChurchDoor(t *testing.T) {
	e := newDungeonEnv(t)
	p := &routedFake{}
	// Gravado dentro de uma cópia que não existe mais (reinício do servidor): x e z são da masmorra.
	e.m.Join(context.Background(), JoinInfo{ID: 9, Name: "Zed", Class: "knight", X: 12, Z: 100, Zone: "catacumbas",
		Gold: 5, Items: game.StarterKit("knight"), AccountID: 9, Peer: p})
	e.pumpAll()
	z := e.home.players[9]
	if z == nil {
		t.Fatal("volta ao mapa principal")
	}
	d := dungeonDefs[game.CatacombsID]
	if math.Hypot(z.pos.X-d.exit.X, z.pos.Z-d.exit.Z) > 4 {
		t.Fatalf("na porta da igreja, não em %v", z.pos)
	}
}

func TestDungeonStairsConnectTheFloors(t *testing.T) {
	e := newDungeonEnv(t)
	p := e.join(1, "Ana")
	in := e.enter(p, 1)
	var up, down uint32
	for id, n := range in.npcs {
		if g := in.gateOf(n); g != nil && g.Stairs {
			if g.ArriveZ > 60 {
				up = id // a escada que sobe leva ao andar 2 (z > 54)
			} else {
				down = id
			}
		}
	}
	if up == 0 || down == 0 {
		t.Fatal("as duas escadas existem")
	}
	pl := in.players[1]
	if pl == nil {
		t.Fatalf("jogador fora da cópia: %d na cópia, %d em casa, dono %s", len(in.players), len(e.home.players), e.m.ZoneOf(1))
	}
	pl.pos = in.npcs[up].pos
	in.UseGate(context.Background(), 1, up)
	e.pumpAll()
	if in.players[1] == nil || pl.pos.Z < 60 {
		t.Fatalf("a escada leva ao andar 2: %v", pl.pos)
	}
	if e.m.ZoneOf(1) != "catacumbas" || len(p.zoneChanges()) != 1 {
		t.Fatal("trocar de andar não troca de zona")
	}
	pl.pos = in.npcs[down].pos
	in.UseGate(context.Background(), 1, down)
	e.pumpAll()
	if pl.pos.Z > 54 {
		t.Fatalf("a outra escada volta ao andar 1: %v", pl.pos)
	}
	// Quem morre volta à entrada (a origem), não à cidade.
	pl.pos = Vec2{X: 10, Z: 30}
	in.kill(pl, in.bossOf(t))
	in.tickFor(0.1)
	in.handle(respawnEvent{id: 1})
	if math.Hypot(pl.pos.X, pl.pos.Z) > 4 {
		t.Fatalf("renasce na entrada da masmorra: %v", pl.pos)
	}
	if in.dg.deaths != 1 {
		t.Fatalf("mortes contadas: %d", in.dg.deaths)
	}
}

// bossDown mata o Abade com o jogador pl (que luta de perto) e deixa o tick entregar o baú.
func bossDown(t *testing.T, in *Zone, pl *entity) {
	t.Helper()
	boss := in.bossOf(t)
	pl.pos = boss.pos
	pl.stats.MaxHP, pl.hp = 1e7, 1e7
	in.raidCredit(boss, pl, 100)
	in.kill(boss, pl)
	in.tickFor(0.2)
}

func TestDungeonBossChestForEveryFighterAndTheBossStaysDead(t *testing.T) {
	e := newDungeonEnv(t)
	pa, pb2 := e.join(1, "Ana"), e.join(2, "Beto")
	ana, beto := e.home.players[1], e.home.players[2]
	e.home.partyInvite(ana, beto)
	e.home.replyInvite(beto, e.home.pendingInviteTo(2).id, true)
	in := e.enter(pa, 1)
	if e.enter(pb2, 2) != in {
		t.Fatal("o grupo entrou")
	}
	a, b := in.players[1], in.players[2]
	boss := in.bossOf(t)
	b.pos = boss.pos
	b.stats.MaxHP, b.hp = 1e7, 1e7
	in.raidCredit(boss, b, 200)
	goldBefore := a.pl.inv.gold
	bossDown(t, in, a)
	for _, c := range []struct {
		p    *entity
		peer *routedFake
	}{{a, pa}, {b, pb2}} {
		if c.peer.systemCount("Baú do Abade") != 1 {
			t.Fatalf("%s não abriu o baú: %v", c.p.name, c.peer.chats())
		}
		if c.p.pl.quests.chests[game.CatacombsID] == 0 {
			t.Fatalf("%s: o dia do baú fica no personagem", c.p.name)
		}
	}
	if a.pl.inv.gold <= goldBefore {
		t.Fatal("o baú traz moedas para a bolsa")
	}
	// Salvo no personagem: o estado que vai para o banco leva o dia.
	if st := in.savedState(a); st.Quests.Chests[game.CatacombsID] == 0 {
		t.Fatal("o baú do dia não foi salvo")
	}
	// O chefe não renasce, nem com o tempo passando.
	*e.now = e.now.Add(2 * time.Hour)
	in.tickFor(30)
	if !boss.dead || !boss.mon.hidden {
		t.Fatal("o chefe da cópia não renasce")
	}
	if in.dg.chests != 2 || !in.dg.bossDown {
		t.Fatalf("a cópia anotou %d baús e chefe caído=%v", in.dg.chests, in.dg.bossDown)
	}
	if len(in.objects) != 0 {
		t.Fatal("sem túmulo: a cópia vai sumir")
	}
}

func TestDungeonChestOncePerDayPerCharacter(t *testing.T) {
	e := newDungeonEnv(t)
	p := e.join(1, "Ana")
	run := func() (*Zone, *entity) {
		p.mu.Lock()
		p.msgs = nil
		p.mu.Unlock()
		in := e.enter(p, 1)
		if in == nil {
			t.Fatal("entrou")
		}
		return in, in.players[1]
	}
	leave := func(in *Zone, pl *entity) {
		in.crossZone(pl, e.home, dungeonDefs[game.CatacombsID].exit, "gate")
		in.tickFor(0.1)
		e.pumpAll()
		for _, i := range e.m.Instances() {
			i.closing = true // a cópia velha não recebe mais ninguém: a próxima é nova
		}
	}
	in, pl := run()
	bossDown(t, in, pl)
	if p.systemCount("Baú do Abade:") != 1 {
		t.Fatal("primeiro baú do dia")
	}
	leave(in, pl)

	// Refaz no mesmo dia, numa cópia nova: só XP e o saque comum do chefe.
	in2, pl2 := run()
	if in2 == in {
		t.Fatal("cópia nova")
	}
	gold, xp := pl2.pl.inv.gold, pl2.pl.xp
	bossDown(t, in2, pl2)
	if p.systemCount("Baú do Abade:") != 0 || p.systemCount("já abriu o Baú do Abade hoje") != 1 {
		t.Fatalf("no mesmo dia o baú não abre de novo: %v", p.chats())
	}
	if pl2.pl.inv.gold != gold {
		t.Fatalf("sem baú, sem moedas na bolsa: %d → %d", gold, pl2.pl.inv.gold)
	}
	if pl2.pl.xp <= xp {
		t.Fatal("a XP do chefe vale sempre")
	}
	leave(in2, pl2)

	// A meia-noite de Brasília passou: o baú volta.
	*e.now = e.now.Add(24 * time.Hour)
	in3, pl3 := run()
	bossDown(t, in3, pl3)
	if p.systemCount("Baú do Abade:") != 1 {
		t.Fatalf("no dia seguinte o baú volta: %v", p.chats())
	}
}

func TestDungeonChestRollIsClassGearOfTheTier(t *testing.T) {
	def := game.DungeonChests[game.CatacombsID]
	if def == nil {
		t.Fatal("as Catacumbas têm baú")
	}
	z := New(testConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	for _, class := range []string{"knight", "barbarian", "mage", "rogue"} {
		sawGear := false
		for seed := range 80 {
			drops := game.RollDungeonChest(def, class, 14, z.rng)
			gear := 0
			for _, d := range drops {
				if d.Item.Kind == game.KindWeapon || d.Item.Kind == game.KindArmor {
					gear++
					sawGear = true
					if !d.Item.UsableBy(class) || game.TierFor(d.Item.ReqLevel) != game.TierD {
						t.Fatalf("%s sorteio %d: a peça %s não é do grau D da classe", class, seed, d.Item.ID)
					}
				}
			}
			if gear > 1 {
				t.Fatalf("%s: o baú traz uma peça de equipamento, veio %d", class, gear)
			}
			if len(drops) == 0 || drops[0].Item.ID != game.GoldID {
				t.Fatalf("%s: o baú começa pelas moedas", class)
			}
		}
		if !sawGear {
			t.Errorf("%s: nenhuma peça em 80 baús", class)
		}
	}
}

func TestDungeonMonstersAreSkeletonsWithRoles(t *testing.T) {
	want := map[string]game.Archetype{
		game.CatacombMinion: game.ArchPack, game.CatacombGuard: game.ArchShield,
		game.CatacombMage: game.ArchArcher, game.CatacombRogue: game.ArchAmbusher,
	}
	for id, arch := range want {
		d := game.Monsters[id]
		if d == nil || d.Archetype != arch || !strings.HasPrefix(d.Model, "skeleton_") {
			t.Errorf("%s: esqueleto do remaster com o papel %v, veio %+v", id, arch, d)
		}
	}
	u := game.Monsters[game.AbadeUlric]
	if u == nil || u.FieldBoss == nil || !u.FieldBoss.Instance || u.FieldBoss.PhaseCount() != 3 || u.Model != "skeleton_mage" || u.Tint == "" || u.Scale < 1.5 {
		t.Fatalf("Abade Ulric: o esqueleto mago com cor e escala, chefe de campo de masmorra em 3 fases: %+v", u)
	}
	// As 3 formas de aviso do raid: anel, círculos sorteados e a marca que segue.
	shapes := map[game.AbilityShape]bool{}
	var mark bool
	for _, a := range u.Abilities {
		shapes[a.Shape] = true
		mark = mark || a.Target == game.AimMark
		if a.ID == "ulric_responso" && (a.Inner != 3 || a.Radius != 9) {
			t.Fatalf("Responso é um anel de 3 a 9 m: %+v", a)
		}
	}
	if !shapes[game.ShapeDonut] || !shapes[game.ShapeCircle] || !mark {
		t.Fatalf("anel, círculo e marca: %v mark=%v", shapes, mark)
	}
	if p := u.FieldBoss.Phases[0]; p.Summon != game.CatacombMinion || p.SummonCount != 4 {
		t.Fatal("Ossuário invoca 4")
	}
}

func TestDungeonCapsCopiesInTheProcess(t *testing.T) {
	e := newDungeonEnv(t)
	e.m.inst.max = 2
	pa, pb2, pc := e.join(1, "Ana"), e.join(2, "Beto"), e.join(3, "Caio")
	if e.enter(pa, 1) == nil || e.enter(pb2, 2) == nil {
		t.Fatal("duas cópias cabem")
	}
	if z := e.enter(pc, 3); z != nil || pc.systemCount("cheias") != 1 {
		t.Fatal("a terceira cópia passa do teto do processo")
	}
}

// Uma cópia que roda de verdade (a goroutine do gerente), sem eventos manuais: entrar, andar e
// ver a cópia fechar sozinha.
func TestDungeonRunsOnItsOwnAndLeavesWhenTheProcessStops(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	base := testConfig()
	base.Dungeons = []string{game.CatacombsID}
	cfg, err := MapConfig(base, world.StarterMap, []string{world.StarterMap})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Spawns = nil
	home := New(cfg, log, nil)
	m := NewManager(log, home)
	m.EnableDungeons(base, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()

	p := &routedFake{}
	m.Join(ctx, JoinInfo{ID: 1, Name: "Ana", Class: "knight", X: -13, Z: 13, Gold: 5, Items: game.StarterKit("knight"), AccountID: 1, Peer: p})
	waitFor(t, func() bool { return m.ZoneOf(1) == "starter" })
	home.UseGate(ctx, 1, func() uint32 {
		for id, n := range home.npcs {
			if g := home.gateOf(n); g != nil && g.Dungeon != "" {
				return id
			}
		}
		return 0
	}())
	waitFor(t, func() bool { return m.ZoneOf(1) == "catacumbas" })
	if got := len(m.Instances()); got != 1 {
		t.Fatalf("%d cópias", got)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("o gerente não esperou as cópias pararem")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("esperou demais")
}

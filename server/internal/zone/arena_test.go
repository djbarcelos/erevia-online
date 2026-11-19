package zone

import (
	"context"
	"sync"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Arena PvP ranqueada (tunnel#81), com o gerente manual das cópias (instance_test.go).

type fakeArena struct {
	mu      sync.Mutex
	matches []ArenaResult
}

func (f *fakeArena) RecordArenaMatch(_ context.Context, r ArenaResult) ([]ArenaChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.matches = append(f.matches, r)
	var out []ArenaChange
	for t, ids := range r.Teams {
		d := -16
		if t == r.Winner {
			d = 16
		}
		for _, id := range ids {
			out = append(out, ArenaChange{ID: id, Before: 1000, After: 1000 + d})
		}
	}
	return out, nil
}

func (f *fakeArena) ArenaRanking(context.Context, int, game.ArenaMode, int) ([]ArenaRank, error) {
	return []ArenaRank{{Name: "Ana", Class: "knight", Level: 12, Rating: 1016, Wins: 1}}, nil
}

func (f *fakeArena) ArenaRatingsOf(context.Context, int, uint32) (map[game.ArenaMode]ArenaRecord, error) {
	return map[game.ArenaMode]ArenaRecord{game.ArenaSolo: {Rating: 1016, Wins: 1}}, nil
}

// arenaJoin: um personagem de nível 12 entra ao lado do Mestre da Arena.
func (e *dungeonEnv) arenaJoin(id uint32, name string) *routedFake {
	e.t.Helper()
	p := &routedFake{}
	e.m.Join(context.Background(), JoinInfo{ID: id, Name: name, Class: "knight", X: arenaMasterSpot.X - 1, Z: arenaMasterSpot.Z,
		XP: game.XPLevelStart(12), Gold: 100, Items: game.StarterKit("knight"), AccountID: int64(id), Peer: p})
	e.pumpAll()
	return p
}

func (e *dungeonEnv) arenaCmd(p *routedFake, id uint32, kind pb.ArenaCmdKind, mode game.ArenaMode) {
	z := p.owner.Load()
	z.players[id].pl.moveTokens = z.cfg.MaxMovesPerSec
	z.ArenaCmd(context.Background(), id, &pb.ArenaCmd{Kind: kind, NpcId: arenaMasterID(), Mode: uint32(mode)})
	e.pumpAll()
}

// runArenaJobs roda os trabalhos de banco da zona na hora (sem o worker do Run).
func runArenaJobs(z *Zone, b ArenaBackend) {
	for {
		select {
		case job := <-z.arenaJobs:
			if fn := job(context.Background(), b); fn != nil {
				fn()
			}
		default:
			return
		}
	}
}

func (r *routedFake) lastArena() *pb.ArenaState {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.msgs) - 1; i >= 0; i-- {
		if a := r.msgs[i].GetArena(); a != nil {
			return a
		}
	}
	return nil
}

func TestArenaSoloMatchFromQueueToResult(t *testing.T) {
	e := newDungeonEnv(t)
	db := &fakeArena{}
	e.home.SetArenaBackend(db)
	pa, pb_ := e.arenaJoin(1, "Ana"), e.arenaJoin(2, "Beto")
	if e.home.npcs[arenaMasterID()] == nil || e.home.npcs[arenaMasterID()].npc.role != pb.NpcRole_NPC_ROLE_ARENA {
		t.Fatal("o Mestre da Arena está na praça")
	}

	// A janela: pontuação e ranking vêm do banco.
	e.arenaCmd(pa, 1, pb.ArenaCmdKind_ARENA_CMD_OPEN, 0)
	runArenaJobs(e.home, db)
	if st := pa.lastArena(); st == nil || !st.Open || len(st.Modes) != 2 || st.Modes[0].Rating != 1016 || len(st.Rankings) != 2 {
		t.Fatalf("janela da arena: %+v", st)
	}

	// Fila: os dois entram no 1x1 e o mapa principal forma a partida.
	e.arenaCmd(pa, 1, pb.ArenaCmdKind_ARENA_CMD_QUEUE, game.ArenaSolo)
	if e.m.arena.queuedMode(1) != game.ArenaSolo || !pa.lastArena().Modes[0].Queued {
		t.Fatal("Ana está na fila do 1x1")
	}
	e.arenaCmd(pb_, 2, pb.ArenaCmdKind_ARENA_CMD_QUEUE, game.ArenaSolo)
	e.home.arenaMatchmake()
	e.pumpAll()
	az := pa.owner.Load()
	if az == e.home || az.arena == nil || pb_.owner.Load() != az || e.m.arena.size(game.ArenaSolo) != 0 {
		t.Fatal("os dois foram para a mesma cópia da arena e saíram da fila")
	}
	az.SetArenaBackend(db)
	ana, beto := az.players[1], az.players[2]
	if ana.pos.X >= 0 || beto.pos.X <= 0 {
		t.Fatalf("cada time num lado: %+v %+v", ana.pos, beto.pos)
	}

	// Preparação: ninguém ataca.
	if ok, _, _ := az.canHit(ana, beto, true); ok {
		t.Fatal("na preparação ninguém ataca")
	}
	e.advance(game.ArenaCountdown + time.Second)
	az.updateArena()
	if az.arena.phase != pb.ArenaPhase_ARENA_PHASE_FIGHT {
		t.Fatalf("depois da preparação, a luta: %v", az.arena.phase)
	}
	if ok, _, _ := az.canHit(ana, beto, false); !ok {
		t.Fatal("na luta, o outro time é alvo (sem Ctrl, sem flag)")
	}

	// Beto cai: fora, sem perder XP nem ganhar karma para Ana.
	xp := beto.pl.xp
	az.kill(beto, ana)
	if !az.arena.out[2] || beto.pl.xp != xp || ana.pl.karma != 0 || ana.pl.flagUntil > az.now {
		t.Fatalf("morte na arena: fora=%v xp %d→%d karma %d", az.arena.out[2], xp, beto.pl.xp, ana.pl.karma)
	}
	az.updateArena()
	if az.arena.phase != pb.ArenaPhase_ARENA_PHASE_DONE || az.arena.winner != 0 {
		t.Fatalf("Ana venceu: fase %v, vencedor %d", az.arena.phase, az.arena.winner)
	}
	runArenaJobs(az, db)
	if len(db.matches) != 1 || db.matches[0].Winner != 0 || db.matches[0].Teams[0][0] != 1 {
		t.Fatalf("a partida foi gravada: %+v", db.matches)
	}
	if m := pa.lastArena().Match; m == nil || m.Winner != 0 || m.RatingAfter != 1016 {
		t.Fatalf("Ana vê o resultado e a pontuação: %+v", m)
	}

	// Depois do resultado, todos voltam à cidade, ao lado do Mestre, com vida cheia.
	e.advance(game.ArenaAfter + time.Second)
	az.updateArena()
	az.tickFor(0.1)
	e.pumpAll()
	if pa.owner.Load() != e.home || pb_.owner.Load() != e.home {
		t.Fatal("os dois voltaram à cidade")
	}
	back := e.home.players[2]
	if back.dead || back.hp != back.stats.MaxHP || dist(back.pos, arenaMasterSpot) > 5 || dist(back.pos, arenaMasterSpot) < 1 {
		t.Fatalf("Beto volta vivo, com vida cheia, perto do Mestre: morto=%v hp %.0f/%.0f %+v", back.dead, back.hp, back.stats.MaxHP, back.pos)
	}
}

func TestArenaQueueRulesAndDisconnect(t *testing.T) {
	e := newDungeonEnv(t)
	pa := e.arenaJoin(1, "Ana")
	// 3x3 sem grupo de 3: recusa.
	e.arenaCmd(pa, 1, pb.ArenaCmdKind_ARENA_CMD_QUEUE, game.ArenaTrios)
	if e.m.arena.queuedMode(1) != 0 || !pa.systemContains("grupo de 3") {
		t.Fatal("o 3x3 é para um grupo de 3")
	}
	// Nível baixo: recusa.
	low := &routedFake{}
	e.m.Join(context.Background(), JoinInfo{ID: 3, Name: "Novato", Class: "knight", X: arenaMasterSpot.X, Z: arenaMasterSpot.Z + 1,
		Items: game.StarterKit("knight"), AccountID: 3, Peer: low})
	e.pumpAll()
	e.arenaCmd(low, 3, pb.ArenaCmdKind_ARENA_CMD_QUEUE, game.ArenaSolo)
	if e.m.arena.queuedMode(3) != 0 {
		t.Fatal("abaixo do nível mínimo, não entra na fila")
	}
	// Entrar, sair, entrar; sair do jogo tira da fila.
	e.arenaCmd(pa, 1, pb.ArenaCmdKind_ARENA_CMD_QUEUE, game.ArenaSolo)
	e.arenaCmd(pa, 1, pb.ArenaCmdKind_ARENA_CMD_LEAVE, 0)
	if e.m.arena.queuedMode(1) != 0 {
		t.Fatal("saiu da fila")
	}
	e.arenaCmd(pa, 1, pb.ArenaCmdKind_ARENA_CMD_QUEUE, game.ArenaSolo)
	e.home.removePlayer(1)
	if e.m.arena.queuedMode(1) != 0 {
		t.Fatal("sair do jogo tira da fila")
	}

	// Na luta, quem sai do jogo perde.
	pa = e.arenaJoin(1, "Ana")
	pb_ := e.arenaJoin(2, "Beto")
	e.arenaCmd(pa, 1, pb.ArenaCmdKind_ARENA_CMD_QUEUE, game.ArenaSolo)
	e.arenaCmd(pb_, 2, pb.ArenaCmdKind_ARENA_CMD_QUEUE, game.ArenaSolo)
	e.home.arenaMatchmake()
	e.pumpAll()
	az := pa.owner.Load()
	e.advance(game.ArenaCountdown + time.Second)
	az.updateArena()
	az.removePlayer(2)
	az.updateArena()
	if az.arena.phase != pb.ArenaPhase_ARENA_PHASE_DONE || az.arena.winner != 0 {
		t.Fatalf("Beto saiu no meio: Ana vence (fase %v, vencedor %d)", az.arena.phase, az.arena.winner)
	}
}

func TestArenaTimeUpGoesToMoreLife(t *testing.T) {
	e := newDungeonEnv(t)
	pa, pb_ := e.arenaJoin(1, "Ana"), e.arenaJoin(2, "Beto")
	e.arenaCmd(pa, 1, pb.ArenaCmdKind_ARENA_CMD_QUEUE, game.ArenaSolo)
	e.arenaCmd(pb_, 2, pb.ArenaCmdKind_ARENA_CMD_QUEUE, game.ArenaSolo)
	e.home.arenaMatchmake()
	e.pumpAll()
	az := pa.owner.Load()
	e.advance(game.ArenaCountdown + time.Second)
	az.updateArena()
	az.players[1].hp = az.players[1].stats.MaxHP * 0.3
	az.players[2].hp = az.players[2].stats.MaxHP * 0.6
	e.advance(game.ArenaFight + time.Second)
	az.updateArena()
	if az.arena.winner != 1 {
		t.Fatalf("no fim do tempo, vence quem tem mais vida (Beto): %d", az.arena.winner)
	}
}

package zone

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Teste de carga das zonas: 2 zonas (o mapa principal com os monstros de sempre e a
// Clareira de Testes) e 50 bots atravessando o portão sem parar, mexendo na bolsa,
// falando no chat geral, sussurrando e em grupos (metade dos bots em duplas). As
// invariantes estão no modo estrito (testes): dona única no barramento (zone_owner,
// Bus.check), grupo, ouro, pilhas... quebra é panic.
//
// No fim confere: cada bot numa zona só; ouro e itens iguais aos do começo (nada
// duplicado nem perdido); a última gravação de cada um igual à memória (e com a zona
// certa); e o tempo do tick de cada zona.
//
// CI: alguns segundos. Versão longa (rodar 1× localmente, sem -race):
//
//	EREVIA_SOAK=10m go test ./internal/zone -run ZonesSoak -count=1 -v -timeout 15m

const soakBots = 50

type botPeer struct {
	owner  atomic.Pointer[Zone]
	dead   atomic.Bool
	msgs   atomic.Int64
	bytes  atomic.Int64
	closes atomic.Int64
}

func (p *botPeer) Send(b []byte) bool { p.msgs.Add(1); p.bytes.Add(int64(len(b))); return true }
func (p *botPeer) Close(string)       { p.closes.Add(1) }
func (p *botPeer) SetZone(z *Zone)    { p.owner.Store(z) }
func (p *botPeer) Closed() bool       { return p.dead.Load() }

// tickRec guarda a duração de cada tick (para média, p99 e pior).
type tickRec struct {
	mu sync.Mutex
	d  []time.Duration
}

func (r *tickRec) add(d time.Duration) { r.mu.Lock(); r.d = append(r.d, d); r.mu.Unlock() }

func (r *tickRec) summary() (mean, p99, worst time.Duration, n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := slices.Clone(r.d)
	slices.Sort(d)
	if len(d) == 0 {
		return
	}
	var sum time.Duration
	for _, x := range d {
		sum += x
	}
	return sum / time.Duration(len(d)), d[len(d)*99/100], d[len(d)-1], len(d)
}

// runTimed é o Run da zona (eventos, tick e salvamento) medindo cada tick.
func runTimed(ctx context.Context, z *Zone, rec *tickRec) {
	dt := time.Second / time.Duration(z.cfg.TickRate)
	ticker := time.NewTicker(dt)
	defer ticker.Stop()
	save := time.NewTicker(z.cfg.SaveEvery)
	defer save.Stop()
	for {
		select {
		case <-ctx.Done():
			z.saveAll(true)
			close(z.saves)
			return
		case e := <-z.events:
			z.handleSafe(e)
		case <-ticker.C:
			start := time.Now()
			z.update(dt.Seconds())
			rec.add(time.Since(start))
		case <-save.C:
			z.saveAll(false)
		}
	}
}

// itemTotals: item → quantidade (bolsa e equipamento), mais o ouro.
func itemTotals(gold int64, items []game.ItemRecord) map[string]int64 {
	out := map[string]int64{"$gold": gold}
	for _, it := range items {
		out[it.ItemID] += int64(it.Count)
	}
	return out
}

func TestZonesSoak50Bots(t *testing.T) {
	dur := 3 * time.Second
	if v := os.Getenv("EREVIA_SOAK"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			t.Fatal(err)
		}
		dur = d
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	enabled := []string{world.StarterMap, MapTest}
	var zs []*Zone
	var saveChans []chan []SavedState
	for _, id := range enabled {
		base := testConfig()
		base.Clock = time.Now // o relógio de verdade (recent, recargas)
		base.SaveEvery = 2 * time.Second
		cfg, err := MapConfig(base, id, enabled)
		if err != nil {
			t.Fatal(err)
		}
		ch := make(chan []SavedState, 4096)
		saveChans = append(saveChans, ch)
		zs = append(zs, New(cfg, log, ch))
	}
	m := NewManager(log, zs...)
	a := zs[0]
	gates := map[*Zone]uint32{}
	for _, z := range zs {
		gates[z] = gateIn(z)
	}

	// Persistência: a última gravação de cada personagem (pelo Seq), das duas zonas.
	var smu sync.Mutex
	saved := map[uint32]SavedState{}
	var saveCount atomic.Int64
	var pwg sync.WaitGroup
	for _, ch := range saveChans {
		pwg.Add(1)
		go func() {
			defer pwg.Done()
			for batch := range ch {
				smu.Lock()
				for _, s := range batch {
					saveCount.Add(1)
					if old, ok := saved[s.ID]; !ok || s.Seq > old.Seq {
						saved[s.ID] = s
					}
				}
				smu.Unlock()
			}
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	recs := []*tickRec{{}, {}}
	var zwg sync.WaitGroup
	for i, z := range zs {
		zwg.Add(1)
		go func() { defer zwg.Done(); runTimed(ctx, z, recs[i]) }()
	}

	// Bots: entram na praça, perto do portão, cada um com ouro e itens próprios.
	peers := make([]*botPeer, soakBots)
	start := map[uint32]map[string]int64{}
	for i := range peers {
		id := uint32(i + 1)
		p := &botPeer{}
		peers[i] = p
		items := game.StarterKit("knight")
		items = append(items, game.ItemRecord{Slot: 20, ItemID: "potion_hp_s", Count: 5 + i})
		gold := int64(1000 + i)
		m.Join(ctx, JoinInfo{ID: id, Name: fmt.Sprintf("Bot%02d", id), Class: "knight", AccountID: int64(id),
			X: float64(i%5) - 2, Z: -14 + float64(i%3), Gold: gold, Items: items, Peer: p})
		start[id] = itemTotals(gold, items)
	}
	// Duplas em grupo (bots 1–20).
	time.Sleep(200 * time.Millisecond)
	done := make(chan struct{})
	a.events <- socialDone{fn: func() {
		for id := uint32(1); id <= 20; id += 2 {
			l, f := a.players[id], a.players[id+1]
			if l == nil || f == nil {
				continue
			}
			a.partyInvite(l, f)
			if inv := a.pendingInviteTo(f.id); inv != nil {
				a.replyInvite(f, inv.id, true)
			}
		}
		close(done)
	}}
	<-done

	// O motorista: cada bot age a cada 100–600 ms.
	rng := rand.New(rand.NewPCG(1, 2))
	next := make([]time.Time, soakBots)
	end := time.Now().Add(dur)
	var gateUses, checks int
	lastCheck := time.Now()
	for time.Now().Before(end) {
		now := time.Now()
		for i, p := range peers {
			if now.Before(next[i]) {
				continue
			}
			next[i] = now.Add(time.Duration(100+rng.IntN(500)) * time.Millisecond)
			z := p.owner.Load()
			id := uint32(i + 1)
			switch r := rng.IntN(100); {
			case r < 55:
				gateUses++
				z.UseGate(ctx, id, gates[z])
			case r < 75:
				z.ItemAction(ctx, id, &pb.MoveItem{FromPlace: placeBag, FromSlot: uint32(rng.IntN(game.BagSize)),
					ToPlace: placeBag, ToSlot: uint32(rng.IntN(game.BagSize))})
			case r < 85:
				z.Chat(ctx, id, "oi de "+z.mapID(), "", pb.ChatChannel_CHAT_CHANNEL_GENERAL)
			case r < 95:
				z.Chat(ctx, id, "psiu", fmt.Sprintf("Bot%02d", rng.IntN(soakBots)+1), pb.ChatChannel_CHAT_CHANNEL_GENERAL)
			default:
				z.Chat(ctx, id, "grupo", "", pb.ChatChannel_CHAT_CHANNEL_PARTY)
			}
		}
		if now.Sub(lastCheck) > 200*time.Millisecond {
			m.checkBus() // dona única e grupos (panic no modo estrito)
			checks++
			lastCheck = now
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Para: espera as entregas em andamento chegarem e desliga as zonas (salvam todos).
	time.Sleep(500 * time.Millisecond)
	m.checkBus()
	cancel()
	zwg.Wait()
	pwg.Wait()

	// Cada bot numa zona só, com o mesmo ouro e os mesmos itens do começo.
	for i := range peers {
		id := uint32(i + 1)
		var where []*Zone
		for _, z := range zs {
			if z.players[id] != nil {
				where = append(where, z)
			}
		}
		if len(where) != 1 {
			t.Fatalf("Bot%02d em %d zonas", id, len(where))
		}
		p := where[0].players[id]
		got := itemTotals(p.pl.inv.gold, p.pl.inv.records())
		if fmt.Sprint(got) != fmt.Sprint(start[id]) {
			t.Fatalf("Bot%02d: itens/ouro mudaram\n antes  %v\n depois %v", id, start[id], got)
		}
		s, ok := saved[id]
		if !ok {
			t.Fatalf("Bot%02d nunca foi gravado", id)
		}
		if fmt.Sprint(itemTotals(s.Gold, s.Items)) != fmt.Sprint(got) || s.Zone != where[0].mapID() {
			t.Fatalf("Bot%02d: a última gravação (zona %s) não bate com a memória (zona %s)", id, s.Zone, where[0].mapID())
		}
		if peers[i].closes.Load() != 0 {
			t.Fatalf("Bot%02d foi desconectado", id)
		}
		if peers[i].owner.Load() != where[0] {
			t.Fatalf("Bot%02d: a conexão aponta para a zona errada", id)
		}
	}
	for i, z := range zs {
		mean, p99, worst, n := recs[i].summary()
		t.Logf("zona %s: %d ticks, média %v, p99 %v, pior %v, broken %d", z.mapID(), n, mean, p99, worst, z.invariants.broken)
		if z.invariants.broken != 0 {
			t.Fatalf("zona %s: %d invariantes quebradas", z.mapID(), z.invariants.broken)
		}
		// Orçamento: o tick abaixo de 5 ms. Com -race (5–10× mais lento) só a média vale.
		if !raceEnabled && p99 > 5*time.Millisecond {
			t.Errorf("zona %s: p99 do tick %v (orçamento 5 ms)", z.mapID(), p99)
		}
		if mean > 5*time.Millisecond {
			t.Errorf("zona %s: tick médio %v (orçamento 5 ms)", z.mapID(), mean)
		}
	}
	var msgs, bytes int64
	for _, p := range peers {
		msgs += p.msgs.Load()
		bytes += p.bytes.Load()
	}
	t.Logf("%d bots, %v: %d travessias (%d pedidos de portão), %d gravações, %d conferências do barramento, %d mensagens (%.1f MB) para os bots",
		soakBots, dur, m.Hops(), gateUses, saveCount.Load(), checks, msgs, float64(bytes)/1e6)
	if m.Hops() < soakBots {
		t.Fatalf("poucas travessias: %d", m.Hops())
	}
}

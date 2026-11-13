// Comando loadbot: teste de carga com bots que falam o protocolo de verdade.
//
// Cada bot entra pelo login de desenvolvimento (só existe com DEV_LOGIN=true, então
// não funciona contra a produção), cria o personagem se precisar, conecta no
// WebSocket, anda pelos anéis de caça, ataca os monstros perto, volta quando morre e
// fala no chat de vez em quando. A cada 10 s imprime mensagens, banda e ping.
//
//	go run ./cmd/loadbot -url http://127.0.0.1:8080 -bots 100 -duration 2m
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
)

var classes = []string{"knight", "barbarian", "mage", "rogue"}

type metrics struct {
	connected atomic.Int64
	msgs      atomic.Int64
	bytes     atomic.Int64
	errors    atomic.Int64
	mu        sync.Mutex
	rtts      []float64
}

func (m *metrics) rtt(ms float64) {
	m.mu.Lock()
	m.rtts = append(m.rtts, ms)
	m.mu.Unlock()
}

func (m *metrics) takeRTTs() []float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rtts
	m.rtts = nil
	return r
}

func main() {
	base := flag.String("url", "http://127.0.0.1:8080", "endereço do servidor (com DEV_LOGIN=true)")
	n := flag.Int("bots", 50, "quantos bots")
	dur := flag.Duration("duration", time.Minute, "quanto tempo rodar depois que todos entram")
	ramp := flag.Duration("ramp", 50*time.Millisecond, "intervalo entre a entrada de cada bot")
	prefix := flag.String("prefix", "Bot", "início do nome dos personagens (letras)")
	chat := flag.Bool("chat", true, "bots falam no chat geral de vez em quando")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(*n)*(*ramp)+*dur)
	defer cancel()

	m := &metrics{}
	var wg sync.WaitGroup
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("%s%03d", *prefix, i)
			if err := runBot(ctx, *base, name, classes[i%len(classes)], *chat, m); err != nil && ctx.Err() == nil {
				m.errors.Add(1)
				log.Printf("%s: %v", name, err)
			}
		}(i)
		select {
		case <-ctx.Done():
		case <-time.After(*ramp):
		}
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	start := time.Now()
	report := func() {
		secs := time.Since(start).Seconds()
		start = time.Now()
		r := m.takeRTTs()
		slices.Sort(r)
		pct := func(p float64) float64 {
			if len(r) == 0 {
				return 0
			}
			return r[min(len(r)-1, int(p*float64(len(r))))]
		}
		bots := m.connected.Load()
		msgs, b := m.msgs.Swap(0), m.bytes.Swap(0)
		perBot := 0.0
		if bots > 0 {
			perBot = float64(b) / secs / float64(bots) / 1024
		}
		fmt.Printf("bots=%d msgs/s=%.0f banda=%.1f KB/s (%.1f KB/s por bot) ping p50=%.1fms p95=%.1fms max=%.1fms erros=%d\n",
			bots, float64(msgs)/secs, float64(b)/secs/1024, perBot, pct(0.5), pct(0.95), pct(1), m.errors.Load())
	}
	for {
		select {
		case <-ctx.Done():
			report()
			wg.Wait()
			return
		case <-ticker.C:
			report()
		}
	}
}

// login entra pelo login de desenvolvimento e garante o personagem. Devolve o id.
func login(ctx context.Context, client *http.Client, base, name, class string) (uint32, error) {
	post := func(path string, body any, out any) error {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode >= 300 {
			return fmt.Errorf("POST %s: %s", path, res.Status)
		}
		if out != nil {
			return json.NewDecoder(res.Body).Decode(out)
		}
		return nil
	}
	if err := post("/api/auth/dev", map[string]string{"username": strings.ToLower(name)}, nil); err != nil {
		return 0, fmt.Errorf("%w (o servidor precisa de DEV_LOGIN=true)", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/characters", nil)
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	var list struct {
		Characters []struct {
			ID   uint32 `json:"id"`
			Name string `json:"name"`
		} `json:"characters"`
	}
	err = json.NewDecoder(res.Body).Decode(&list)
	res.Body.Close()
	if err != nil {
		return 0, err
	}
	for _, c := range list.Characters {
		if strings.EqualFold(c.Name, name) {
			return c.ID, nil
		}
	}
	var created struct {
		ID uint32 `json:"id"`
	}
	if err := post("/api/characters", map[string]string{"name": name, "class": class}, &created); err != nil {
		return 0, err
	}
	return created.ID, nil
}

type vec struct{ x, z float64 }

func runBot(ctx context.Context, base, name, class string, chat bool, m *metrics) error {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	id, err := login(ctx, client, base, name, class)
	if err != nil {
		return err
	}
	u, _ := url.Parse(base)
	h := http.Header{}
	for _, c := range jar.Cookies(u) {
		h.Add("Cookie", c.Name+"="+c.Value)
	}
	wsURL := strings.Replace(strings.Replace(base, "https://", "wss://", 1), "http://", "ws://", 1) + "/ws"
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		return err
	}
	defer c.CloseNow()
	c.SetReadLimit(16 << 20)

	var mu sync.Mutex
	send := func(msg *pb.ClientMessage) {
		b, _ := proto.Marshal(msg)
		mu.Lock()
		defer mu.Unlock()
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = c.Write(wctx, websocket.MessageBinary, b)
	}
	send(&pb.ClientMessage{Msg: &pb.ClientMessage_Join{Join: &pb.Join{CharacterId: id}}})

	// Estado visto pelo bot: monstros vivos e a própria posição.
	var st sync.Mutex
	monsters := map[uint32]vec{}
	me := vec{}
	dead := false
	joined := make(chan struct{})

	go func() {
		defer c.CloseNow()
		first := true
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			m.msgs.Add(1)
			m.bytes.Add(int64(len(data)))
			var msg pb.ServerMessage
			if proto.Unmarshal(data, &msg) != nil {
				continue
			}
			st.Lock()
			switch v := msg.Msg.(type) {
			case *pb.ServerMessage_Welcome:
				for _, e := range v.Welcome.Entities {
					if e.Kind == pb.EntityKind_ENTITY_KIND_MONSTER {
						monsters[e.Id] = vec{float64(e.X), float64(e.Z)}
					}
					if e.Id == id {
						me = vec{float64(e.X), float64(e.Z)}
					}
				}
				if first {
					first = false
					m.connected.Add(1)
					close(joined)
				}
			case *pb.ServerMessage_Spawn:
				if e := v.Spawn.Entity; e.Kind == pb.EntityKind_ENTITY_KIND_MONSTER {
					monsters[e.Id] = vec{float64(e.X), float64(e.Z)}
				}
			case *pb.ServerMessage_Despawn:
				delete(monsters, v.Despawn.Id)
			case *pb.ServerMessage_Snapshot:
				for _, s := range v.Snapshot.States {
					if s.Id == id {
						me = vec{float64(s.X), float64(s.Z)}
					} else if _, ok := monsters[s.Id]; ok {
						monsters[s.Id] = vec{float64(s.X), float64(s.Z)}
					}
				}
				for _, d := range v.Snapshot.Deaths {
					if d == id {
						dead = true
					}
					delete(monsters, d)
				}
				for _, r := range v.Snapshot.Revives {
					if r.Id == id {
						dead = false
					}
				}
			case *pb.ServerMessage_Pong:
				m.rtt(float64(time.Now().UnixNano())/1e6 - v.Pong.ClientTime)
			case *pb.ServerMessage_Kicked:
				log.Printf("%s expulso: %s", name, v.Kicked.Reason)
			}
			st.Unlock()
		}
	}()

	select {
	case <-joined:
	case <-ctx.Done():
		return nil
	case <-time.After(15 * time.Second):
		return fmt.Errorf("sem Welcome em 15 s")
	}
	defer m.connected.Add(-1)

	rng := rand.New(rand.NewPCG(uint64(id), 7))
	act := time.NewTicker(time.Duration(800+rng.IntN(400)) * time.Millisecond)
	defer act.Stop()
	ping := time.NewTicker(2 * time.Second)
	defer ping.Stop()
	nextChat := time.Now().Add(time.Duration(10+rng.IntN(30)) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ping.C:
			send(&pb.ClientMessage{Msg: &pb.ClientMessage_Ping{Ping: &pb.Ping{ClientTime: float64(time.Now().UnixNano()) / 1e6}}})
		case <-act.C:
			st.Lock()
			isDead, pos := dead, me
			var target uint32
			best := 40.0
			for mid, mp := range monsters {
				if d := math.Hypot(mp.x-pos.x, mp.z-pos.z); d < best {
					best, target = d, mid
				}
			}
			st.Unlock()
			switch {
			case isDead:
				send(&pb.ClientMessage{Msg: &pb.ClientMessage_Respawn{Respawn: &pb.Respawn{}}})
			case target != 0 && rng.Float64() < 0.7:
				send(&pb.ClientMessage{Msg: &pb.ClientMessage_Attack{Attack: &pb.Attack{TargetId: target}}})
			default:
				// Anda para um ponto do anel de caça perto de onde está.
				a := math.Atan2(pos.z, pos.x) + (rng.Float64()-0.5)*0.8
				r := 45 + rng.Float64()*30
				send(&pb.ClientMessage{Msg: &pb.ClientMessage_MoveTo{MoveTo: &pb.MoveTo{X: float32(r * math.Cos(a)), Z: float32(r * math.Sin(a))}}})
			}
			if chat && time.Now().After(nextChat) {
				nextChat = time.Now().Add(time.Duration(20+rng.IntN(40)) * time.Second)
				send(&pb.ClientMessage{Msg: &pb.ClientMessage_ChatSend{ChatSend: &pb.ChatSend{Text: "caçando por aqui, " + name}}})
			}
		}
	}
}

package zone

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"erevia/server/internal/world"
)

// Várias zonas no mesmo processo (uma por mapa), com troca pelo portão (gate.go).
//
// Quem é dono do quê:
//   - cada Zone continua dona de todo o estado dos jogadores que estão nela (uma
//     goroutine, eventos por canal). Um personagem está em uma zona só, ou em trânsito
//     entre duas (a entrega do portão), nunca em duas nem em nenhuma;
//   - o Manager cria as zonas, roda as goroutines e decide para qual zona vai cada login
//     (a de agora, a de onde saiu há pouco ou a gravada no banco);
//   - o Bus (bus.go) é o que é global: quem está online e onde, chat geral, sussurro e
//     grupos (party_zones.go).
//
// A conexão (WebSocket) é a mesma do começo ao fim: o portão só troca a zona dona da
// sessão (routedPeer.SetZone).

// routedPeer é a conexão que sabe para qual zona mandar os pedidos (net.session).
type routedPeer interface {
	// SetZone troca a zona que recebe os pedidos desta conexão.
	SetZone(z *Zone)
	// Closed: a conexão já terminou (o Leave dela pode ter ido para a zona antiga).
	Closed() bool
}

// Manager segura as zonas do processo e o barramento entre elas.
type Manager struct {
	log   *slog.Logger
	bus   *Bus
	zones []*Zone // zones[0] = o mapa principal (para onde vai quem não tem zona conhecida)
	byMap map[string]*Zone

	mu  sync.Mutex
	ctx context.Context // o do Run: as entregas do portão desistem quando o processo para

	hops   atomic.Int64 // travessias de portão desde a partida
	lookup ZoneLookup   // mapa gravado no banco (portal: personagem offline)
}

// Hops: travessias de portão desde a partida.
func (m *Manager) Hops() int64 { return m.hops.Load() }

// NewManager cria o gerente com as zonas (a primeira é o mapa principal).
func NewManager(log *slog.Logger, zones ...*Zone) *Manager {
	m := &Manager{log: log, bus: newBus(), byMap: map[string]*Zone{}}
	for _, z := range zones {
		m.Add(z)
	}
	return m
}

// Add põe uma zona no gerente (antes do Run). O id é cfg.Map ("" = starter).
func (m *Manager) Add(z *Zone) {
	z.mgr = m
	if z.cfg.Map == "" {
		z.cfg.Map = world.StarterMap
	}
	m.zones = append(m.zones, z)
	m.byMap[z.cfg.Map] = z
}

// Zones: as zonas, o mapa principal primeiro.
func (m *Manager) Zones() []*Zone { return m.zones }

// Zone: a zona do mapa (nil = não existe ou está desligado).
func (m *Manager) Zone(id string) *Zone {
	if id == "" {
		id = world.StarterMap
	}
	return m.byMap[id]
}

// Home: o mapa principal.
func (m *Manager) Home() *Zone { return m.zones[0] }

// Run roda todas as zonas até ctx acabar e espera cada uma salvar todo mundo.
func (m *Manager) Run(ctx context.Context) {
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, z := range m.zones {
		wg.Add(1)
		go func() { defer wg.Done(); z.Run(ctx) }()
	}
	wg.Add(1)
	go func() { defer wg.Done(); m.watchBus(ctx) }()
	wg.Wait()
}

func (m *Manager) runCtx() context.Context {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

// watchBus confere as invariantes do barramento 1×/s (grupos e índice de nomes).
func (m *Manager) watchBus(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.checkBus()
		}
	}
}

// checkBus: no modo estrito (testes e DEBUG_INVARIANTS=1) quebra é panic; em produção,
// registra e conta.
func (m *Manager) checkBus() {
	bad := m.bus.check()
	if bad == "" {
		return
	}
	if strictInvariants {
		panic(invariantPanic("invariante quebrada: bus: " + bad))
	}
	m.bus.mu.Lock()
	m.bus.broken++
	m.bus.mu.Unlock()
	m.log.Error("bus invariant broken", "detail", bad)
}

// ---------- Login, saída e portão ----------

// Join manda o personagem para a zona certa e liga a conexão a ela:
//  1. online (relogin) ou atravessando um portão: a zona de agora (espera a entrega);
//  2. saiu há pouco: a mesma zona (o recent dela vale mais que o banco);
//  3. senão, a zona gravada no banco (info.Zone); mapa desligado ou desconhecido: o
//     principal, na praça.
func (m *Manager) Join(ctx context.Context, info JoinInfo) *Zone {
	z := m.route(ctx, info.ID)
	if z == nil {
		if z = m.Zone(info.Zone); z == nil {
			z = m.Home()
			info.X, info.Z = 0, 0 // a posição gravada era de outro mapa
		}
	}
	if rp, ok := info.Peer.(routedPeer); ok {
		rp.SetZone(z)
	}
	z.Join(ctx, info)
	return z
}

// route: a zona onde o personagem está (ou acabou de estar). Espera um portão em
// andamento terminar a entrega (no máximo 5 s).
func (m *Manager) route(ctx context.Context, id uint32) *Zone {
	deadline := time.After(5 * time.Second)
	for {
		z, wait := m.bus.route(id, m.Home().cfg.Clock()) // o mesmo relógio do unregister
		if wait == nil {
			return z
		}
		select {
		case <-wait:
		case <-ctx.Done():
			return z
		case <-deadline:
			return z
		}
	}
}

// transfer entrega o personagem (já fora de from, estado já gravado) à zona dest.
// Chamado pela goroutine de from. Primeiro troca a dona no barramento e na conexão,
// depois põe a entrada na fila de dest: um pedido que chegar no meio vai para dest e,
// se ele ainda não chegou, é ignorado (como um clique perdido).
func (m *Manager) transfer(info JoinInfo, dest *Zone) {
	m.hops.Add(1)
	ch := m.bus.moveTo(info.ID, dest)
	if rp, ok := info.Peer.(routedPeer); ok {
		rp.SetZone(dest)
	}
	ev := socialDone{fn: func() { dest.onArrive(info) }}
	select {
	case dest.events <- ev: // caminho comum: cabe na fila
		if ch != nil {
			m.bus.landed(info.ID, ch)
		}
		return
	default:
	}
	// Fila cheia: espera numa goroutine (a zona de origem não pode travar esperando a
	// outra). Se o processo parar antes, vale o que foi gravado (zona e chegada).
	go func() {
		select {
		case dest.events <- ev:
		case <-m.runCtx().Done():
		}
		if ch != nil {
			m.bus.landed(info.ID, ch)
		}
	}()
}

// ZoneOf: o id do mapa onde o personagem está online ("" = offline).
func (m *Manager) ZoneOf(id uint32) string {
	if z := m.bus.zoneOf(id); z != nil {
		return z.cfg.Map
	}
	return ""
}

// ---------- Portal de administração, contas e erros ----------

// Status soma o retrato de todas as zonas (cada uma pelo canal, com prazo).
func (m *Manager) Status(ctx context.Context) (AdminStatus, error) {
	var out AdminStatus
	var errs []error
	for _, z := range m.zones {
		s, err := z.Status(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out.Online += s.Online
	}
	return out, errors.Join(errs...)
}

// AccountDeleted avisa todas as zonas (o personagem pode estar em qualquer uma).
func (m *Manager) AccountDeleted(ctx context.Context, d AccountGone) {
	for _, z := range m.zones {
		z.AccountDeleted(ctx, d)
	}
}

// ReportPanic e ReportDBError vão para a telemetria do mapa principal.
func (m *Manager) ReportPanic(where string, playerID uint32, msg string, r any, stack []byte) {
	m.Home().ReportPanic(where, playerID, msg, r, stack)
}

func (m *Manager) ReportDBError(op string, err error) { m.Home().ReportDBError(op, err) }

// mapTitle: o nome do mapa para o jogador ("Vale de Vel'Aran").
func (z *Zone) mapTitle() string {
	if z.cfg.MapTitle != "" {
		return z.cfg.MapTitle
	}
	return z.mapID()
}

// mapID: o id do mapa da zona ("starter" se não foi dito).
func (z *Zone) mapID() string {
	if z.cfg.Map == "" {
		return world.StarterMap
	}
	return z.cfg.Map
}

package main

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Um bot do caos: uma corrente de sorteios só dele (semente + índice), uma sessão por vez
// (cai e volta), e o estado que o servidor mostra a ele.

type chaosBot struct {
	run     *chaosRun
	idx     int
	name    string
	class   string
	client  *http.Client
	jar     *cookiejar.Jar
	charID  uint32
	created bool
	rng     *rand.Rand
	ctx     context.Context // da rodada (as sessões derivam dele)

	sess    *chaosSession // sessão atual (nil = desconectado)
	offline int           // passos que ainda faltam fora do jogo (queda pedida)
	sig     uint64        // assinatura das escolhas (FNV das ações e sorteios)
}

// slotItem: um espaço da bolsa, do equipamento ou do armazém.
type slotItem struct {
	id      string
	count   uint32
	enchant uint32
}

type seen struct {
	name string
	pos  vec
	kind pb.EntityKind
}

type npcSeen struct {
	pos     vec
	role    pb.NpcRole
	stock   []string
	dests   []uint32
	recipes []string
	crafts  bool
	name    string
}

// chaosSession é uma conexão e o que ela recebeu do servidor.
type chaosSession struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	wmu    sync.Mutex

	mu         sync.Mutex
	welcomed   chan struct{}
	closed     bool
	byUs       bool // a queda foi pedida pelo bot (não conta como erro)
	id         uint32
	pos        vec
	dead       bool
	level      uint32
	gold       uint64
	bag        [game.BagSize]slotItem
	equip      [game.EquipSlots]slotItem
	storage    [60]slotItem
	stGold     uint64
	tradeOpen  bool
	partner    uint32
	mine, them *pb.TradeOffer
	invites    []*pb.Invite
	party      *pb.PartyState
	clan       *pb.ClanState
	ents       map[uint32]seen // jogadores e monstros vistos
	ground     map[uint32]vec
	npcs       map[uint32]*npcSeen
	skills     []uint32
	ranks      map[uint32]uint32
	freeAttrs  uint32
	freeSkills uint32
	specs      []string
	mails      []uint64
	titles     []string
	channel    bool
	asked      uint32 // último convidado para trocar (o bot fica perto dele)
	askedAt    time.Time
	// O Quadro de Contratos (ContractState.board_id) e os contratos dele hoje.
	board     uint32
	contracts []string
	// Casa de leilões: anúncios vistos na busca e os meus; cartas com anexo não retirado.
	aucFound, aucMine []aucSeen
	attach            []uint64
}

// ---------- Passos ----------

func (b *chaosBot) play(ctx context.Context, steps int) {
	b.ctx = ctx
	run := b.run
	defer func() {
		if s := b.sess; s != nil {
			s.close(true, false)
		}
		run.mu.Lock()
		run.sig[b.idx] = b.sig
		run.mu.Unlock()
	}()
	next := time.Now()
	for k := 0; k < steps; k++ {
		if ctx.Err() != nil {
			return
		}
		// Sempre 6 sorteios por passo, aconteça o que acontecer: a corrente não depende do estado.
		var r [6]uint64
		for i := range r {
			r[i] = b.rng.Uint64()
		}
		a := pickAction(r[0])
		h := fnv.New64a()
		fmt.Fprintf(h, "%x|%d|%s|%x", b.sig, k, a.name, r)
		b.sig = h.Sum64()
		run.count(run.actions, a.name, 1)

		switch {
		case b.offline > 0:
			b.offline--
		case b.sess == nil || b.sess.isClosed():
			b.connect(ctx)
		}
		if b.offline == 0 && b.sess != nil && !b.sess.isClosed() {
			a.fn(b, b.sess, r)
		}
		run.mu.Lock()
		run.steps[b.idx] = k + 1
		run.mu.Unlock()

		next = next.Add(run.cfg.step)
		wait := time.Until(next)
		if wait < -2*time.Second { // atrasou demais (conexão lenta): sem rajada para alcançar
			next = time.Now()
			wait = 0
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}
}

// connect abre uma sessão nova (o cookie de login continua valendo).
func (b *chaosBot) connect(ctx context.Context) {
	if s := b.sess; s != nil {
		s.close(false, false)
		if !s.byUs {
			b.run.count(b.run.errs, "queda que o bot não pediu", 1)
		}
		b.sess = nil
	}
	s, err := b.dial(ctx)
	if err != nil {
		if ctx.Err() == nil {
			b.run.count(b.run.errs, "conexão: "+shortErr(err), 1)
		}
		b.offline = 5 // tenta de novo daqui a pouco
		return
	}
	b.sess = s
}

func shortErr(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "connection refused"):
		return "servidor recusou a conexão"
	case strings.Contains(s, "sem Welcome"):
		return "sem Welcome em 10 s"
	case strings.Contains(s, "401"):
		return "401 (sessão)"
	}
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

func (b *chaosBot) dial(ctx context.Context) (*chaosSession, error) {
	base := b.run.cfg.base
	u, _ := url.Parse(base)
	h := http.Header{}
	for _, c := range b.jar.Cookies(u) {
		h.Add("Cookie", c.Name+"="+c.Value)
	}
	wsURL := strings.Replace(strings.Replace(base, "https://", "wss://", 1), "http://", "ws://", 1) + "/ws"
	dctx, dcancel := context.WithTimeout(ctx, 10*time.Second)
	defer dcancel()
	c, _, err := websocket.Dial(dctx, wsURL, &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		return nil, err
	}
	c.SetReadLimit(16 << 20)
	sctx, cancel := context.WithCancel(ctx)
	s := &chaosSession{
		conn: c, ctx: sctx, cancel: cancel, welcomed: make(chan struct{}),
		ents: map[uint32]seen{}, ground: map[uint32]vec{}, npcs: map[uint32]*npcSeen{}, ranks: map[uint32]uint32{},
	}
	go b.read(s)
	s.send(&pb.ClientMessage{Msg: &pb.ClientMessage_Join{Join: &pb.Join{CharacterId: b.charID}}})
	select {
	case <-s.welcomed:
	case <-ctx.Done():
		s.close(true, false)
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		s.close(true, false)
		return nil, errors.New("sem Welcome")
	}
	b.run.mu.Lock()
	b.run.sessions++
	b.run.mu.Unlock()
	return s, nil
}

func (s *chaosSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// close fecha a sessão. byUs: a queda foi pedida (não é erro). abrupt: sem o aperto de mão
// do WebSocket (como fechar a aba ou perder a rede).
func (s *chaosSession) close(byUs, abrupt bool) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.byUs = s.byUs || byUs
	s.mu.Unlock()
	if abrupt {
		s.conn.CloseNow()
	} else {
		go s.conn.Close(websocket.StatusNormalClosure, "")
	}
	s.cancel()
}

func (s *chaosSession) send(msg *pb.ClientMessage) {
	b, _ := proto.Marshal(msg)
	s.sendRaw(websocket.MessageBinary, b)
}

func (s *chaosSession) sendRaw(typ websocket.MessageType, b []byte) {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	wctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	if err := s.conn.Write(wctx, typ, b); err != nil {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
	}
}

// read recebe e guarda o estado; termina quando a conexão cai.
func (b *chaosBot) read(s *chaosSession) {
	run := b.run
	first := true
	defer func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.cancel()
	}()
	for {
		_, data, err := s.conn.Read(s.ctx)
		if err != nil {
			s.mu.Lock()
			byUs := s.byUs
			s.mu.Unlock()
			if st := websocket.CloseStatus(err); st != -1 && !byUs {
				reason := ""
				var ce websocket.CloseError
				if errors.As(err, &ce) {
					reason = ce.Reason
				}
				run.count(run.kicks, fmt.Sprintf("fechada pelo servidor (%d %s)", st, reason), 1)
			}
			return
		}
		var msg pb.ServerMessage
		if proto.Unmarshal(data, &msg) != nil {
			run.count(run.errs, "mensagem do servidor ilegível", 1)
			continue
		}
		s.mu.Lock()
		switch v := msg.Msg.(type) {
		case *pb.ServerMessage_Welcome:
			s.welcome(v.Welcome)
			if first {
				first = false
				close(s.welcomed)
			}
		case *pb.ServerMessage_Spawn:
			if e := v.Spawn.Entity; e != nil && (e.Kind == pb.EntityKind_ENTITY_KIND_PLAYER || e.Kind == pb.EntityKind_ENTITY_KIND_MONSTER) {
				if !e.Dead {
					s.ents[e.Id] = seen{name: e.Name, pos: vec{float64(e.X), float64(e.Z)}, kind: e.Kind}
				}
			}
		case *pb.ServerMessage_Despawn:
			delete(s.ents, v.Despawn.Id)
		case *pb.ServerMessage_Snapshot:
			for _, st := range v.Snapshot.States {
				if st.Id == s.id {
					s.pos = vec{float64(st.X), float64(st.Z)}
				} else if e, ok := s.ents[st.Id]; ok {
					e.pos = vec{float64(st.X), float64(st.Z)}
					s.ents[st.Id] = e
				}
			}
			for _, d := range v.Snapshot.Deaths {
				if d == s.id {
					s.dead = true
				} else if e, ok := s.ents[d]; ok && e.kind == pb.EntityKind_ENTITY_KIND_MONSTER {
					delete(s.ents, d)
				}
			}
			for _, r := range v.Snapshot.Revives {
				if r.Id == s.id {
					s.dead = false
					s.pos = vec{float64(r.X), float64(r.Z)}
				}
			}
		case *pb.ServerMessage_SelfStats:
			s.dead, s.level = v.SelfStats.Dead, v.SelfStats.Level
		case *pb.ServerMessage_Inventory:
			s.inventory(v.Inventory)
		case *pb.ServerMessage_Storage:
			s.storage = [60]slotItem{}
			s.stGold = v.Storage.Gold
			for _, it := range v.Storage.Items {
				if it.Slot < uint32(len(s.storage)) {
					s.storage[it.Slot] = slotItem{it.ItemId, it.Count, it.Enchant}
				}
			}
		case *pb.ServerMessage_GroundAdded:
			for _, g := range v.GroundAdded.Items {
				s.ground[g.Id] = vec{float64(g.X), float64(g.Z)}
			}
		case *pb.ServerMessage_GroundRemoved:
			for _, id := range v.GroundRemoved.Ids {
				delete(s.ground, id)
			}
		case *pb.ServerMessage_Trade:
			t := v.Trade
			s.tradeOpen, s.partner, s.mine, s.them = t.Open, t.PartnerId, t.Mine, t.Theirs
		case *pb.ServerMessage_Invite:
			s.invites = append(s.invites, v.Invite)
			if len(s.invites) > 8 {
				s.invites = s.invites[1:]
			}
		case *pb.ServerMessage_Party:
			s.party = v.Party
		case *pb.ServerMessage_Clan:
			s.clan = v.Clan
		case *pb.ServerMessage_Progress:
			s.progress(v.Progress)
		case *pb.ServerMessage_ChannelBar:
			s.channel = v.ChannelBar.Kind != pb.ChannelKind_CHANNEL_KIND_NONE
		case *pb.ServerMessage_Social:
			if m := v.Social.GetMail(); m != nil && m.Listed {
				s.mails = s.mails[:0]
				for _, ml := range m.Mails {
					s.mails = append(s.mails, ml.Id)
				}
				s.onMailAttachments(m)
			}
		case *pb.ServerMessage_Contracts:
			s.board = v.Contracts.BoardId
			s.contracts = s.contracts[:0]
			for _, c := range v.Contracts.Contracts {
				s.contracts = append(s.contracts, c.Id)
			}
		case *pb.ServerMessage_Auction:
			s.onAuction(v.Auction)
		case *pb.ServerMessage_Achievements:
			if v.Achievements.Full {
				s.titles = s.titles[:0]
				for _, a := range v.Achievements.List {
					s.titles = append(s.titles, a.Id)
				}
			}
		case *pb.ServerMessage_Chat:
			if c := v.Chat; c.Channel == pb.ChatChannel_CHAT_CHANNEL_SYSTEM && strings.Contains(strings.ToLower(c.Text), "troca") {
				run.count(run.trades, tradeNote(c.Text), 1)
			}
		case *pb.ServerMessage_Kicked:
			reason := v.Kicked.Reason
			run.count(run.kicks, reason, 1)
			if strings.Contains(reason, "Erro no servidor") {
				run.mu.Lock()
				run.panics++
				run.mu.Unlock()
			}
		}
		s.mu.Unlock()
	}
}

func (s *chaosSession) welcome(w *pb.Welcome) {
	s.id = w.YourId
	clear(s.ents)
	for _, e := range w.Entities {
		switch {
		case e.Id == w.YourId:
			s.pos, s.dead, s.level = vec{float64(e.X), float64(e.Z)}, e.Dead, e.Level
		case e.Kind == pb.EntityKind_ENTITY_KIND_NPC:
			n := s.npcs[e.Id]
			if n == nil {
				n = &npcSeen{}
				s.npcs[e.Id] = n
			}
			n.pos, n.name = vec{float64(e.X), float64(e.Z)}, e.Name
		case (e.Kind == pb.EntityKind_ENTITY_KIND_PLAYER || e.Kind == pb.EntityKind_ENTITY_KIND_MONSTER) && !e.Dead:
			s.ents[e.Id] = seen{name: e.Name, pos: vec{float64(e.X), float64(e.Z)}, kind: e.Kind}
		}
	}
	for _, ni := range w.Npcs {
		n := s.npcs[ni.Id]
		if n == nil {
			n = &npcSeen{}
			s.npcs[ni.Id] = n
		}
		n.role, n.stock = ni.Role, ni.Stock
		n.dests = n.dests[:0]
		for _, d := range ni.Dests {
			n.dests = append(n.dests, d.Id)
		}
		n.recipes = n.recipes[:0]
		for _, r := range ni.Recipes {
			n.recipes = append(n.recipes, r.Id)
		}
		n.crafts = len(ni.Recipes) > 0
	}
	for _, g := range w.GroundItems {
		s.ground[g.Id] = vec{float64(g.X), float64(g.Z)}
	}
	s.skills = s.skills[:0]
	for _, sk := range w.Skills {
		s.skills = append(s.skills, sk.Id)
	}
	slices.Sort(s.skills)
	for _, sp := range w.Specs {
		s.specs = append(s.specs, sp.Id)
	}
	if w.Inventory != nil {
		s.inventory(w.Inventory)
	}
	if w.Progress != nil {
		s.progress(w.Progress)
	}
	s.clan = w.Clan
}

func (s *chaosSession) inventory(inv *pb.Inventory) {
	s.gold = inv.Gold
	s.bag, s.equip = [game.BagSize]slotItem{}, [game.EquipSlots]slotItem{}
	for _, it := range inv.Items {
		si := slotItem{it.ItemId, it.Count, it.Enchant}
		switch {
		case it.Place == 0 && it.Slot < game.BagSize:
			s.bag[it.Slot] = si
		case it.Place == 1 && it.Slot < game.EquipSlots:
			s.equip[it.Slot] = si
		}
	}
}

func (s *chaosSession) progress(p *pb.Progress) {
	s.freeAttrs, s.freeSkills = p.FreeAttrPoints, p.FreeSkillPoints
	clear(s.ranks)
	for _, r := range p.Ranks {
		s.ranks[r.SkillId] = r.Rank
	}
}

// ---------- Ajudas para escolher a partir dos sorteios ----------

func pickN(r uint64, n int) int {
	if n <= 0 {
		return 0
	}
	return int(r % uint64(n))
}

// pct: um número de 0 a 99 a partir de um sorteio (bits diferentes de pickN).
func pct(r uint64) int { return int((r >> 40) % 100) }

var badSlots = []uint32{40, 41, 60, 61, 255, 1 << 16, 1 << 31, math.MaxUint32}
var badCounts = []uint32{0, math.MaxUint32, 1 << 31, 1000, 999_999}
var badIDs = []uint32{0, 1, math.MaxUint32, 1 << 29, 1<<29 + 999, 1 << 30, 999_999_999}

func badU32(list []uint32, r uint64) uint32 { return list[pickN(r, len(list))] }

func (s *chaosSession) bagSlots(pred func(slotItem) bool) []uint32 {
	var out []uint32
	for i, it := range s.bag {
		if it.id != "" && (pred == nil || pred(it)) {
			out = append(out, uint32(i))
		}
	}
	return out
}

func (s *chaosSession) equipSlots() []uint32 {
	var out []uint32
	for i, it := range s.equip {
		if it.id != "" {
			out = append(out, uint32(i))
		}
	}
	return out
}

func (s *chaosSession) storageSlots() []uint32 {
	var out []uint32
	for i, it := range s.storage {
		if it.id != "" {
			out = append(out, uint32(i))
		}
	}
	return out
}

// someBag: um espaço com item (r escolhe qual); sem nada, um espaço qualquer.
func (s *chaosSession) someBag(r uint64, pred func(slotItem) bool) uint32 {
	if l := s.bagSlots(pred); len(l) > 0 {
		return l[pickN(r, len(l))]
	}
	return uint32(pickN(r, game.BagSize))
}

func isScroll(it slotItem) bool {
	d := game.Items[it.id]
	return d != nil && d.Kind == game.KindScroll
}

func isGear(it slotItem) bool {
	d := game.Items[it.id]
	return d != nil && d.Equippable()
}

// npcBy: um NPC com o papel pedido (r escolhe entre os que servem), o id e a posição.
func (s *chaosSession) npcBy(r uint64, ok func(*npcSeen) bool) (uint32, *npcSeen) {
	var ids []uint32
	for id, n := range s.npcs {
		if ok(n) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	slices.Sort(ids)
	id := ids[pickN(r, len(ids))]
	return id, s.npcs[id]
}

func shopNPC(n *npcSeen) bool    { return n.role == pb.NpcRole_NPC_ROLE_SHOP && len(n.stock) > 0 }
func storageNPC(n *npcSeen) bool { return n.role == pb.NpcRole_NPC_ROLE_STORAGE }
func smithNPC(n *npcSeen) bool   { return n.crafts }

// nearestNPC: o NPC mais perto com o papel pedido (onde o bot está é o que vale).
func (s *chaosSession) nearestNPC(ok func(*npcSeen) bool) uint32 {
	best, id := math.MaxFloat64, uint32(0)
	for nid, n := range s.npcs {
		if !ok(n) {
			continue
		}
		if d := math.Hypot(n.pos.x-s.pos.x, n.pos.z-s.pos.z); d < best || (d == best && nid < id) {
			best, id = d, nid
		}
	}
	return id
}

// nearby: entidades do tipo pedido até maxDist, da mais perto para a mais longe.
func (s *chaosSession) nearby(kind pb.EntityKind, maxDist float64) []uint32 {
	type c struct {
		id uint32
		d  float64
	}
	var l []c
	for id, e := range s.ents {
		if e.kind != kind {
			continue
		}
		if d := math.Hypot(e.pos.x-s.pos.x, e.pos.z-s.pos.z); d <= maxDist {
			l = append(l, c{id, d})
		}
	}
	slices.SortFunc(l, func(a, b c) int {
		if a.d != b.d {
			if a.d < b.d {
				return -1
			}
			return 1
		}
		return int(a.id) - int(b.id)
	})
	out := make([]uint32, len(l))
	for i, x := range l {
		out[i] = x.id
	}
	return out
}

// otherBot: o nome de outro bot da rodada (r escolhe).
func (b *chaosBot) otherBot(r uint64) string {
	i := pickN(r, len(b.run.names)-1)
	if i >= b.idx {
		i++
	}
	return b.run.names[i]
}

func (b *chaosBot) sent(label string) { b.run.count(b.run.sent, label, 1) }

// tradeNote: o aviso de troca sem o nome de quem (para agrupar no relatório).
func tradeNote(text string) string {
	for _, k := range []string{" cancelou a troca", " saiu do jogo: troca cancelada", " não pode trocar agora", " já tem um convite pendente"} {
		if i := strings.Index(text, k); i >= 0 {
			return "(alguém)" + text[i:]
		}
	}
	if strings.HasPrefix(text, "Chegue mais perto de") {
		return "Chegue mais perto de (alguém) para trocar."
	}
	if strings.HasPrefix(text, "A troca foi cancelada: a bolsa de") {
		return "A troca foi cancelada: a bolsa de (alguém) não tem espaço."
	}
	return text
}

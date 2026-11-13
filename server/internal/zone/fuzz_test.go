package zone

// Fuzz da entrada dos jogadores (tunnel#118). Um cliente mal-intencionado pode mandar
// qualquer byte pelo WebSocket; nenhuma mensagem pode derrubar a zona, criar ouro ou
// item, nem tirar alguém do mapa.
//
//   - FuzzZoneBytes: bytes crus → proto.Unmarshal em ClientMessage → zona.
//   - FuzzZoneSemantic: sequências de mensagens bem formadas com valores extremos em cada
//     campo de cada tipo do oneof (montadas por reflexão, então tipos novos entram sozinhos),
//     com ticks entre elas, numa zona com 2 jogadores, NPCs e armazém.
//
// O caminho real (WebSocket → readPump → zona) tem o seu alvo em internal/net
// (FuzzConnBytes). Como rodar: ver o README ("Fuzz da entrada dos jogadores").

import (
	"context"
	"fmt"
	"log/slog"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// ---------- Entrega (o mesmo switch de internal/net.readPump) ----------

// fuzzDeliver entrega a mensagem à zona como o readPump faz: pela API pública, que
// posta no canal de eventos. Devolve false para os tipos que a conexão não repassa à
// zona (Join depois do primeiro, Ping) ou que não conhece.
//
// Se um tipo novo entrar no oneof, TestFuzzDeliverCoversOneof falha até ele ser
// incluído aqui (e no readPump).
func fuzzDeliver(z *Zone, id uint32, msg *pb.ClientMessage) bool {
	ctx := context.Background()
	switch m := msg.Msg.(type) {
	case *pb.ClientMessage_MoveTo:
		if m.MoveTo.Stop {
			z.Stop(ctx, id)
		} else {
			z.Move(ctx, id, float64(m.MoveTo.X), float64(m.MoveTo.Z))
		}
	case *pb.ClientMessage_ChatSend:
		z.Chat(ctx, id, m.ChatSend.Text, m.ChatSend.To, m.ChatSend.Channel)
	case *pb.ClientMessage_Attack:
		z.Attack(ctx, id, m.Attack.TargetId, m.Attack.Force)
	case *pb.ClientMessage_UseSkill:
		z.UseSkill(ctx, id, m.UseSkill.SkillId, m.UseSkill.TargetId, m.UseSkill.Force)
	case *pb.ClientMessage_Respawn:
		z.Respawn(ctx, id)
	case *pb.ClientMessage_PickUp:
		z.ItemAction(ctx, id, m.PickUp)
	case *pb.ClientMessage_MoveItem:
		z.ItemAction(ctx, id, m.MoveItem)
	case *pb.ClientMessage_UseItem:
		z.ItemAction(ctx, id, m.UseItem)
	case *pb.ClientMessage_DestroyItem:
		z.ItemAction(ctx, id, m.DestroyItem)
	case *pb.ClientMessage_Buy:
		z.ItemAction(ctx, id, m.Buy)
	case *pb.ClientMessage_Sell:
		z.ItemAction(ctx, id, m.Sell)
	case *pb.ClientMessage_AllocateAttrs:
		z.ProgressAction(ctx, id, m.AllocateAttrs)
	case *pb.ClientMessage_LearnSkills:
		z.ProgressAction(ctx, id, m.LearnSkills)
	case *pb.ClientMessage_SetHotbar:
		z.ProgressAction(ctx, id, m.SetHotbar)
	case *pb.ClientMessage_Teleport:
		z.ItemAction(ctx, id, m.Teleport)
	case *pb.ClientMessage_CancelChannel:
		z.ItemAction(ctx, id, m.CancelChannel)
	case *pb.ClientMessage_StorageMove:
		z.ItemAction(ctx, id, m.StorageMove)
	case *pb.ClientMessage_StorageGold:
		z.ItemAction(ctx, id, m.StorageGold)
	case *pb.ClientMessage_Enchant:
		z.ItemAction(ctx, id, m.Enchant)
	case *pb.ClientMessage_ChangeLook:
		z.ItemAction(ctx, id, m.ChangeLook)
	case *pb.ClientMessage_PartyCmd:
		z.Social(ctx, id, m.PartyCmd)
	case *pb.ClientMessage_ClanCmd:
		z.Social(ctx, id, m.ClanCmd)
	case *pb.ClientMessage_InviteReply:
		z.Social(ctx, id, m.InviteReply)
	case *pb.ClientMessage_FriendCmd:
		z.Social(ctx, id, m.FriendCmd)
	case *pb.ClientMessage_MailCmd:
		z.Social(ctx, id, m.MailCmd)
	case *pb.ClientMessage_ClientStats:
		z.ClientStats(ctx, id, m.ClientStats)
	case *pb.ClientMessage_Craft:
		z.ItemAction(ctx, id, m.Craft)
	case *pb.ClientMessage_RankingRequest:
		z.Social(ctx, id, m.RankingRequest)
	case *pb.ClientMessage_AchievementCmd:
		z.Social(ctx, id, m.AchievementCmd)
	case *pb.ClientMessage_TradeCmd:
		z.Trade(ctx, id, m.TradeCmd)
	default:
		// Join (só vale como primeira mensagem), Ping (respondido na conexão) e vazio.
		return false
	}
	return true
}

// TestFuzzDeliverCoversOneof garante que fuzzDeliver conhece todos os tipos do oneof.
func TestFuzzDeliverCoversOneof(t *testing.T) {
	notForZone := map[protoreflect.Name]bool{"join": true, "ping": true}
	for _, fd := range fuzzOneofFields() {
		msg := &pb.ClientMessage{}
		m := msg.ProtoReflect()
		m.Set(fd, protoreflect.ValueOfMessage(m.NewField(fd).Message()))
		z := newFuzzBareZone()
		got := fuzzDeliver(z, 1, msg)
		if got == notForZone[fd.Name()] {
			t.Errorf("fuzzDeliver(%s) = %v: atualize fuzzDeliver (e confira o readPump em internal/net)", fd.Name(), got)
		}
	}
}

func newFuzzBareZone() *Zone {
	cfg := testConfig()
	cfg.Spawns = nil
	return New(cfg, slog.New(slog.DiscardHandler), nil)
}

// fuzzOneofFields: os campos do oneof msg de ClientMessage, em ordem de número.
func fuzzOneofFields() []protoreflect.FieldDescriptor {
	od := (&pb.ClientMessage{}).ProtoReflect().Descriptor().Oneofs().ByName("msg")
	out := make([]protoreflect.FieldDescriptor, od.Fields().Len())
	for i := range out {
		out[i] = od.Fields().Get(i)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number() < out[j].Number() })
	return out
}

// ---------- Leitor dos bytes do fuzz ----------

// fuzzReader consome os bytes do fuzz; acabou, devolve zeros.
//
// Com rng, os bytes que faltam vêm de um gerador com semente nos próprios bytes: uma
// entrada curta ainda vira uma sequência longa (o fuzz do Go cresce as entradas devagar),
// e a mesma entrada sempre gera a mesma sequência.
type fuzzReader struct {
	b   []byte
	rng *rand.PCG
}

func newFuzzStream(data []byte) *fuzzReader {
	h := fnv.New64a()
	h.Write(data)
	return &fuzzReader{b: data, rng: rand.NewPCG(h.Sum64(), uint64(len(data)))}
}

func (r *fuzzReader) byte() byte {
	if len(r.b) == 0 {
		if r.rng != nil {
			return byte(r.rng.Uint64())
		}
		return 0
	}
	c := r.b[0]
	r.b = r.b[1:]
	return c
}

func (r *fuzzReader) u32() uint32 {
	return uint32(r.byte()) | uint32(r.byte())<<8 | uint32(r.byte())<<16 | uint32(r.byte())<<24
}

func (r *fuzzReader) u64() uint64 { return uint64(r.u32()) | uint64(r.u32())<<32 }

// pick escolhe um índice em [0, n).
func (r *fuzzReader) pick(n int) int {
	if n <= 0 {
		return 0
	}
	if n <= 256 {
		return int(r.byte()) % n
	}
	return int(uint32(r.byte())|uint32(r.byte())<<8) % n
}

// ---------- Zona de teste ----------

// fuzzWorld: o mapa real (com muralhas, casas e A*), carregado uma vez.
var fuzzWorld = sync.OnceValue(func() *world.World {
	w, err := world.Load()
	if err != nil {
		panic(err)
	}
	return w
})

// fuzzLog guarda as linhas de erro da zona: "marshal" quer dizer que uma mensagem não
// saiu (ex.: texto com UTF-8 inválido num snapshot), e um recover de panic também loga
// como erro. Qualquer erro reprova a entrada.
type fuzzLog struct {
	mu   sync.Mutex
	errs []string
}

func (l *fuzzLog) Enabled(_ context.Context, lv slog.Level) bool { return lv >= slog.LevelError }
func (l *fuzzLog) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
		return true
	})
	l.mu.Lock()
	l.errs = append(l.errs, b.String())
	l.mu.Unlock()
	return nil
}
func (l *fuzzLog) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *fuzzLog) WithGroup(string) slog.Handler      { return l }

// fuzzPeer descarta o que recebe, mas confere que cada mensagem decodifica (o cliente
// não conseguiria ler uma mensagem inválida).
type fuzzPeer struct {
	bad    error
	closed string
}

func (f *fuzzPeer) Send(data []byte) bool {
	var m pb.ServerMessage
	if err := proto.Unmarshal(data, &m); err != nil && f.bad == nil {
		f.bad = err
	}
	return true
}

func (f *fuzzPeer) Close(reason string) { f.closed = reason }

// fuzzHarness: uma zona sem loop (o teste chama handle/update) e os dois jogadores.
type fuzzHarness struct {
	t      *testing.T
	z      *Zone
	log    *fuzzLog
	ids    [2]uint32
	names  [2]string
	peers  [2]*fuzzPeer
	accts  [2]int64
	gm     bool // algum GM: /give cria itens de verdade, então a contabilidade não vale
	spots  []Vec2
	trace  []string
	social *fakeSocial
}

// Lugares de partida: perto de cada NPC (loja, ferreiro, barbeiro, armazém, pedras),
// o centro, fora da cidade e a borda do mapa.
func fuzzSpots(z *Zone) []Vec2 {
	var out []Vec2
	ids := make([]uint32, 0, len(z.npcs))
	for id := range z.npcs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		n := z.npcs[id]
		out = append(out, Vec2{X: n.pos.X + 1.5, Z: n.pos.Z})
	}
	b := z.bounds()
	return append(out, Vec2{}, Vec2{X: 60, Z: 10}, Vec2{X: b.MaxX, Z: b.MaxZ}, Vec2{X: b.MinX + 0.5, Z: 0})
}

var (
	fuzzClasses = func() []string {
		var out []string
		for id := range game.Classes {
			out = append(out, id)
		}
		slices.Sort(out)
		return out
	}()
	fuzzItemIDs = func() []string {
		ids := game.ItemIDs()
		slices.Sort(ids)
		return ids
	}()
	fuzzRecipeIDs = func() []string {
		var out []string
		for _, r := range game.RecipeList {
			out = append(out, r.ID)
		}
		return out
	}()
	fuzzAchIDs = func() []string {
		var out []string
		for _, a := range game.AchievementList {
			out = append(out, a.ID)
		}
		return out
	}()
	fuzzLevels = []int{5, 6, 10, 15, 20, 30}
	fuzzGold   = []int64{0, 1, 149, 5000, 123456, 1_000_000_000}
	fuzzCounts = []int{1, 2, 15, 50, 999}
	fuzzEnch   = []int{0, 3, 4, 9, 10}
)

// newFuzzHarness monta a zona a partir dos primeiros bytes: mapa real ou campo aberto,
// cidade segura ou não, limite de ações, classes, níveis, ouro, itens, armazém, karma,
// mesma conta ou não, GM ou não, e onde cada um começa.
func newFuzzHarness(t *testing.T, r *fuzzReader) *fuzzHarness {
	flags := r.byte()
	cfg := testConfig()
	cfg.Spawns = nil // sem monstros: nada pode criar ouro nem item além da loja e do ferreiro
	if flags&1 != 0 {
		cfg.World = fuzzWorld()
	}
	if flags&2 != 0 {
		cfg.SafeRadius = 0 // PvP em qualquer lugar
	}
	if flags&4 != 0 {
		// Sem limite de ações e de chat: o fuzz chega mais fundo em cada sequência.
		cfg.MaxMovesPerSec, cfg.ChatPerSec, cfg.ChatBurst = 1e6, 1e6, 1e6
	}
	cfg.DevCommands = flags&8 != 0
	lg := &fuzzLog{}
	h := &fuzzHarness{t: t, log: lg, z: New(cfg, slog.New(lg), nil)}
	h.spots = fuzzSpots(h.z)
	h.ids = [2]uint32{1, 2}
	h.names = [2]string{"Ana", "Beto"}
	h.accts = [2]int64{1, 2}
	if flags&16 != 0 {
		h.accts[1] = 1 // os dois personagens da mesma conta (armazém compartilhado)
	}
	h.gm = flags&32 != 0 && flags&64 != 0 // 1 em 4
	h.social = newFakeSocial(FriendData{ID: 1, Name: "Ana"}, FriendData{ID: 2, Name: "Beto"},
		FriendData{ID: 3, Name: "Caio"})
	h.z.SetSocialBackend(h.social)

	first := h.spots[r.pick(len(h.spots))]
	for i := range 2 {
		class := fuzzClasses[r.pick(len(fuzzClasses))]
		level := fuzzLevels[r.pick(len(fuzzLevels))]
		pos := first
		if i == 1 {
			if r.byte()&3 == 0 {
				pos = h.spots[r.pick(len(h.spots))]
			} else {
				pos = Vec2{X: first.X + 2, Z: first.Z + 1} // perto: troca, grupo, PvP
			}
		}
		items := game.StarterKit(class)
		for k, n := 0, r.pick(7); k < n; k++ {
			items = append(items, game.ItemRecord{
				Slot: 8 + k, ItemID: fuzzItemIDs[r.pick(len(fuzzItemIDs))],
				Count: fuzzCounts[r.pick(len(fuzzCounts))], Enchant: fuzzEnch[r.pick(len(fuzzEnch))],
			})
		}
		var st *StorageData
		if r.byte()&1 != 0 {
			st = &StorageData{Gold: fuzzGold[r.pick(len(fuzzGold))]}
			for k, n := 0, r.pick(4); k < n; k++ {
				st.Items = append(st.Items, game.ItemRecord{
					Slot: k * 7, ItemID: fuzzItemIDs[r.pick(len(fuzzItemIDs))],
					Count: fuzzCounts[r.pick(len(fuzzCounts))], Enchant: fuzzEnch[r.pick(len(fuzzEnch))],
				})
			}
		}
		karma := []int{0, 0, 0, 5, 100}[r.pick(5)]
		h.peers[i] = &fuzzPeer{}
		h.z.handle(joinEvent{info: JoinInfo{
			ID: h.ids[i], Name: h.names[i], Class: class, X: pos.X, Z: pos.Z,
			XP: game.XPLevelStart(level), Gold: fuzzGold[r.pick(len(fuzzGold))], Items: items,
			AccountID: h.accts[i], Karma: karma, Storage: st, GM: h.gm && i == 0,
			Peer: h.peers[i],
		}})
	}
	h.pump()
	h.trace = append(h.trace, fmt.Sprintf("setup flags=%08b gm=%v", flags, h.gm))
	return h
}

// pump faz o papel do loop e dos workers: trata os eventos postados e roda na hora os
// trabalhos de clã, moderação e amigos/correio (com o banco falso).
func (h *fuzzHarness) pump() {
	z, ctx := h.z, context.Background()
	for {
		select {
		case e := <-z.events:
			z.handle(e)
		case job := <-z.clanJobs:
			if ev := z.doClanJob(ctx, job); ev != nil {
				z.handle(ev)
			}
		case job := <-z.modJobs:
			if ev := z.doModJob(ctx, job); ev != nil {
				z.handle(ev)
			}
		case job := <-z.socialJobs:
			if fn := job(ctx, z.socialBackend); fn != nil {
				fn()
			}
		case <-z.telemetry:
		case <-z.bossJobs:
		default:
			return
		}
	}
}

func (h *fuzzHarness) ticks(n int) {
	for range n {
		start := time.Now()
		h.z.update(1 / float64(h.z.cfg.TickRate))
		h.pump()
		if d := time.Since(start); d > fuzzSlowMessage {
			h.fail("um tick levou %v (limite %v)", d, fuzzSlowMessage)
		}
	}
}

// send entrega uma mensagem do jogador i (0 ou 1) e trata o que ela gerou. A mensagem
// passa por Marshal/Unmarshal como no fio: o que o protobuf recusa (ex.: string com
// UTF-8 inválido) nunca chega à zona, porque o readPump descarta.
func (h *fuzzHarness) send(i int, msg *pb.ClientMessage) {
	data, err := proto.Marshal(msg)
	if err != nil {
		h.trace = append(h.trace, fmt.Sprintf("%s: (não serializa: %v)", h.names[i], err))
		return
	}
	wire := &pb.ClientMessage{}
	if err := proto.Unmarshal(data, wire); err != nil {
		h.trace = append(h.trace, fmt.Sprintf("%s: (descartada no readPump: %v)", h.names[i], err))
		return
	}
	h.trace = append(h.trace, fmt.Sprintf("%s [%d bytes]: %s", h.names[i], len(data), fuzzShort(wire)))
	start := time.Now()
	fuzzDeliver(h.z, h.ids[i], wire)
	h.pump()
	if d := time.Since(start); d > fuzzSlowMessage {
		h.fail("uma mensagem levou %v na zona (limite %v): trava o tick de todos", d, fuzzSlowMessage)
	}
}

// fuzzSlowMessage: tempo máximo para a zona tratar uma mensagem. Folgado (a máquina do
// fuzz roda vários processos), mas pega um laço que trava o tick de todos.
const fuzzSlowMessage = 2 * time.Second

func fuzzShort(m proto.Message) string {
	s := fmt.Sprintf("%v", m)
	if len(s) > 300 {
		s = s[:300] + fmt.Sprintf("…(%d bytes)", len(s))
	}
	return s
}

func (h *fuzzHarness) fail(format string, args ...any) {
	h.t.Helper()
	h.t.Fatalf("%s\nsequência:\n  %s", fmt.Sprintf(format, args...), strings.Join(h.trace, "\n  "))
}

// ---------- Contabilidade de ouro e itens ----------

// fuzzLedger: tudo o que os dois jogadores têm, onde quer que esteja (bolsa, equipamento,
// armazém, chão; quem saiu, pelo estado guardado em z.recent).
type fuzzLedger struct {
	gold  int64          // moedas na bolsa, no armazém e no chão
	value int64          // gold + o que a loja pagaria por todos os itens
	units map[string]int // unidades por item
}

func (h *fuzzHarness) ledger() fuzzLedger {
	z := h.z
	l := fuzzLedger{units: map[string]int{}}
	addItem := func(def *game.ItemDef, n int) {
		if def == nil || n <= 0 {
			return
		}
		if def.Kind == game.KindGold {
			l.gold += int64(n)
			l.value += int64(n)
			return
		}
		l.units[def.ID] += n
		l.value += def.SellPrice() * int64(n)
	}
	for _, id := range h.ids {
		if p := z.players[id]; p != nil {
			l.gold += p.pl.inv.gold
			l.value += p.pl.inv.gold
			for _, s := range p.pl.inv.bag {
				if !s.empty() {
					addItem(s.def, s.count)
				}
			}
			for _, s := range p.pl.inv.equip {
				if !s.empty() {
					addItem(s.def, s.count)
				}
			}
		} else if r, ok := z.recent[id]; ok {
			l.gold += r.state.Gold
			l.value += r.state.Gold
			for _, it := range r.state.Items {
				addItem(game.Items[it.ItemID], it.Count)
			}
		}
	}
	seen := map[int64]bool{}
	for _, acc := range h.accts {
		if seen[acc] {
			continue
		}
		seen[acc] = true
		if st := z.storages[acc]; st != nil {
			l.gold += st.gold
			l.value += st.gold
			for _, s := range st.slots {
				if !s.empty() {
					addItem(s.def, s.count)
				}
			}
		}
	}
	for _, g := range z.ground {
		addItem(g.def, g.count)
	}
	return l
}

// checkEconomy confere o passo entre before e after. Fontes legítimas na zona de teste
// (sem monstros e sem missões, porque os personagens começam no nível 5 ou mais):
//   - Buy: só o item comprado pode aumentar, e o ouro cai pelo menos o preço cheio;
//   - Craft: só o resultado da receita, no máximo Count, com o ouro e os materiais pagos;
//   - Sell troca item por ouro pelo preço de venda, então o "valor" não muda.
//
// Fora isso, nenhuma unidade de item aparece e o valor total (ouro + preço de venda dos
// itens) nunca sobe. Morrer, destruir, encantar, viajar, cortar o cabelo e o chão só tiram.
func (h *fuzzHarness) checkEconomy(before, after fuzzLedger, msg *pb.ClientMessage) {
	h.t.Helper()
	if h.gm {
		return
	}
	allowValue := int64(0)
	for id, n := range after.units {
		grew := n - before.units[id]
		if grew <= 0 {
			continue
		}
		def := game.Items[id]
		switch {
		case msg.GetBuy() != nil && msg.GetBuy().ItemId == id:
			if paid := before.gold - after.gold; paid < def.Price*int64(grew) {
				h.fail("compra de %d× %s pagou %d moedas (preço %d cada)", grew, id, paid, def.Price)
			}
		case msg.GetCraft() != nil && game.Recipes[msg.GetCraft().RecipeId] != nil &&
			game.Recipes[msg.GetCraft().RecipeId].Result == id:
			rc := game.Recipes[msg.GetCraft().RecipeId]
			if grew > rc.Count {
				h.fail("receita %s criou %d× %s (máximo %d)", rc.ID, grew, id, rc.Count)
			}
			if paid := before.gold - after.gold; paid < rc.Gold {
				h.fail("receita %s pagou %d moedas (custa %d)", rc.ID, paid, rc.Gold)
			}
			for _, m := range rc.Materials {
				if m.ID != id && before.units[m.ID]-after.units[m.ID] < m.Count {
					h.fail("receita %s não gastou %d× %s (antes %d, depois %d)", rc.ID, m.Count, m.ID, before.units[m.ID], after.units[m.ID])
				}
			}
			allowValue += def.SellPrice() * int64(grew)
		default:
			h.fail("%d× %s apareceram do nada (antes %d, depois %d)", grew, id, before.units[id], n)
		}
	}
	if after.value > before.value+allowValue {
		h.fail("valor total subiu de %d para %d (ouro %d → %d)", before.value, after.value, before.gold, after.gold)
	}
}

// ---------- Invariantes do estado ----------

// checkState confere o estado de cada jogador depois de cada passo.
//
// TODO(tunnel#112): quando checkInvariants entrar na main, chamar ela aqui no lugar das
// checagens de posição, vida, mana e inventário abaixo (a contabilidade de ouro e itens
// de checkEconomy é do fuzz e continua).
func (h *fuzzHarness) checkState() {
	h.t.Helper()
	z := h.z
	b := z.bounds()
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	inMap := func(v Vec2) bool { return finite(v.X) && finite(v.Z) && b.Contains(world.Point{X: v.X, Z: v.Z}) }
	for i, id := range h.ids {
		p := z.players[id]
		if p == nil {
			continue // saiu (ex.: expulso); quem saiu entra na contabilidade por z.recent
		}
		if !inMap(p.pos) {
			h.fail("%s fora do mapa: pos=%v (limites %+v)", h.names[i], p.pos, b)
		}
		if !inMap(p.dest) {
			h.fail("%s com destino fora do mapa: dest=%v", h.names[i], p.dest)
		}
		for _, pt := range p.path {
			if !inMap(pt) {
				h.fail("%s com caminho fora do mapa: %v", h.names[i], pt)
			}
		}
		if !finite(p.stats.MaxHP) || p.stats.MaxHP <= 0 || !finite(p.hp) || p.hp < 0 || p.hp > p.stats.MaxHP {
			h.fail("%s com vida %v de %v", h.names[i], p.hp, p.stats.MaxHP)
		}
		if !finite(p.stats.MaxMP) || p.stats.MaxMP < 0 || !finite(p.mp) || p.mp < 0 || p.mp > p.stats.MaxMP {
			h.fail("%s com mana %v de %v", h.names[i], p.mp, p.stats.MaxMP)
		}
		inv := &p.pl.inv
		if inv.gold < 0 {
			h.fail("%s com ouro negativo: %d", h.names[i], inv.gold)
		}
		for s, st := range inv.bag {
			if st.def != nil && (st.count < 0 || st.count > st.def.Stack || st.enchant < 0 || st.enchant > game.MaxEnchant) {
				h.fail("%s: bolsa[%d] inválida: %s×%d +%d", h.names[i], s, st.def.ID, st.count, st.enchant)
			}
		}
		for s, st := range inv.equip {
			if st.empty() {
				continue
			}
			if st.count != 1 || !st.def.Fits(s) || st.enchant < 0 || st.enchant > game.MaxEnchant {
				h.fail("%s: equipamento[%d] inválido: %s×%d +%d", h.names[i], s, st.def.ID, st.count, st.enchant)
			}
		}
		if h.peers[i].bad != nil {
			h.fail("%s recebeu uma mensagem que não decodifica: %v", h.names[i], h.peers[i].bad)
		}
	}
	for acc, st := range z.storages {
		if st.gold < 0 {
			h.fail("armazém da conta %d com ouro negativo: %d", acc, st.gold)
		}
		for s, it := range st.slots {
			if it.def != nil && (it.count < 0 || it.count > it.def.Stack) {
				h.fail("armazém[%d] inválido: %s×%d", s, it.def.ID, it.count)
			}
		}
	}
	for _, g := range z.ground {
		if !inMap(g.pos) || g.count <= 0 {
			h.fail("item no chão inválido: %s×%d em %v", g.def.ID, g.count, g.pos)
		}
	}
	h.log.mu.Lock()
	errs := slices.Clone(h.log.errs)
	h.log.mu.Unlock()
	if len(errs) > 0 {
		h.fail("a zona logou erro: %s", strings.Join(errs, "; "))
	}
}

// step entrega msg do jogador i, roda n ticks e confere tudo.
func (h *fuzzHarness) step(i int, msg *pb.ClientMessage, n int) {
	h.t.Helper()
	before := h.ledger()
	h.send(i, msg)
	h.ticks(n)
	h.checkEconomy(before, h.ledger(), msg)
	h.checkState()
}

// ---------- Alvo 1: bytes ----------

// FuzzZoneBytes: os bytes são uma mensagem do cliente (como chegam do WebSocket). O
// que não decodifica é descartado, como no readPump; o resto vai para a zona, que não
// pode entrar em pânico nem quebrar as invariantes.
//
// O corpus inicial (uma mensagem válida de cada tipo) está em testdata/fuzz/FuzzZoneBytes.
func FuzzZoneBytes(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		var msg pb.ClientMessage
		if proto.Unmarshal(data, &msg) != nil {
			return
		}
		h := newFuzzHarness(t, &fuzzReader{b: fuzzFixedSetup(fuzzSpotFor(&msg))})
		h.step(0, &msg, 2)
		h.step(1, &msg, 40) // o outro lado (alvo/convite) e 2 s para canais e efeitos
	})
}

// fuzzSpotFor: com npc_id de um NPC que existe, os dois começam perto dele (loja,
// ferreiro, barbeiro, pedra ou armazém); senão, perto da Mercadora. O índice é o de
// fuzzSpots (NPCs em ordem de id).
func fuzzSpotFor(msg *pb.ClientMessage) byte {
	m := msg.ProtoReflect()
	od := m.Descriptor().Oneofs().ByName("msg")
	fd := m.WhichOneof(od)
	if fd == nil || fd.Kind() != protoreflect.MessageKind {
		return 0
	}
	inner := m.Get(fd).Message()
	nf := inner.Descriptor().Fields().ByName("npc_id")
	if nf == nil || nf.Kind() != protoreflect.Uint32Kind {
		return 0
	}
	id := uint32(inner.Get(nf).Uint())
	if n := id - npcIDBase; id >= npcIDBase && n < uint32(len(townNPCs)+len(teleportStones)+1) {
		return byte(n)
	}
	return 0
}

// fuzzFixedSetup: os bytes de newFuzzHarness para uma zona fixa: mapa real, sem limite
// de ações, os dois perto do lugar spot de fuzzSpots; Ana (cavaleira, nível 10) com
// pergaminho, poções e materiais, armazém com moedas e item; Beto (mago, com karma).
func fuzzFixedSetup(spot byte) []byte {
	idx := func(list []string, v string) byte { return byte(slices.Index(list, v)) }
	item := func(id string, count, ench int) []byte {
		return []byte{idx(fuzzItemIDs, id), byte(slices.Index(fuzzCounts, count)), byte(slices.Index(fuzzEnch, ench))}
	}
	b := []byte{0b00000101, spot}
	b = append(b, idx(fuzzClasses, "knight"), 2, 4)
	b = append(b, item("scroll_ench_weapon", 2, 0)...)
	b = append(b, item("potion_hp_s", 15, 0)...)
	b = append(b, item("bone_old", 50, 0)...)
	b = append(b, item("skull_cracked", 15, 0)...)
	b = append(b, 1, 3, 1)
	b = append(b, item("scroll_return", 2, 0)...)
	b = append(b, 0, 5)
	b = append(b, idx(fuzzClasses, "mage"), 3, 1, 0, 0, 3, 3)
	return b
}

// fuzzValidMessages: uma mensagem válida de cada tipo do oneof (o corpus inicial).
func fuzzValidMessages() []*pb.ClientMessage {
	npc := npcIDBase
	out := []*pb.ClientMessage{
		{Msg: &pb.ClientMessage_Join{Join: &pb.Join{CharacterId: 1}}},
		{Msg: &pb.ClientMessage_MoveTo{MoveTo: &pb.MoveTo{X: 10, Z: -5}}},
		{Msg: &pb.ClientMessage_Ping{Ping: &pb.Ping{ClientTime: 12345.5}}},
		{Msg: &pb.ClientMessage_ChatSend{ChatSend: &pb.ChatSend{Text: "olá", To: "Beto"}}},
		{Msg: &pb.ClientMessage_Attack{Attack: &pb.Attack{TargetId: 2, Force: true}}},
		{Msg: &pb.ClientMessage_UseSkill{UseSkill: &pb.UseSkill{SkillId: game.ClassSkills("knight")[0].ID, TargetId: 2, Force: true}}},
		{Msg: &pb.ClientMessage_Respawn{Respawn: &pb.Respawn{}}},
		{Msg: &pb.ClientMessage_PickUp{PickUp: &pb.PickUp{Id: groundIDBase + 1}}},
		{Msg: &pb.ClientMessage_MoveItem{MoveItem: &pb.MoveItem{FromPlace: 0, FromSlot: 8, ToPlace: 0, ToSlot: 20}}},
		{Msg: &pb.ClientMessage_UseItem{UseItem: &pb.UseItem{Place: 0, Slot: 0}}},
		{Msg: &pb.ClientMessage_DestroyItem{DestroyItem: &pb.DestroyItem{Slot: 9, Count: 1}}},
		{Msg: &pb.ClientMessage_Buy{Buy: &pb.Buy{NpcId: npc, ItemId: "potion_hp_s", Count: 2}}},
		{Msg: &pb.ClientMessage_Sell{Sell: &pb.Sell{NpcId: npc, Slot: 8, Count: 1}}},
		{Msg: &pb.ClientMessage_AllocateAttrs{AllocateAttrs: &pb.AllocateAttrs{Add: &pb.Attrs{Str: 1, Con: 1}}}},
		{Msg: &pb.ClientMessage_LearnSkills{LearnSkills: &pb.LearnSkills{Ranks: []*pb.SkillRank{{SkillId: game.ClassSkills("knight")[0].ID, Rank: 2}}}}},
		{Msg: &pb.ClientMessage_SetHotbar{SetHotbar: &pb.SetHotbar{Slots: []string{"s1", "potion_hp_s", ""}}}},
		{Msg: &pb.ClientMessage_PartyCmd{PartyCmd: &pb.PartyCmd{Kind: pb.PartyCmdKind(1), Name: "Beto", TargetId: 2}}},
		{Msg: &pb.ClientMessage_ClanCmd{ClanCmd: &pb.ClanCmd{Kind: pb.ClanCmdKind(1), Name: "Lobos"}}},
		{Msg: &pb.ClientMessage_InviteReply{InviteReply: &pb.InviteReply{InviteId: 1, Accept: true}}},
		{Msg: &pb.ClientMessage_Teleport{Teleport: &pb.Teleport{NpcId: npc + uint32(len(townNPCs)), DestId: 1}}},
		{Msg: &pb.ClientMessage_CancelChannel{CancelChannel: &pb.CancelChannel{}}},
		{Msg: &pb.ClientMessage_Enchant{Enchant: &pb.Enchant{ScrollSlot: 8, Place: 1, Slot: game.SlotWeapon}}},
		{Msg: &pb.ClientMessage_StorageMove{StorageMove: &pb.StorageMove{NpcId: npc + uint32(len(townNPCs)+len(teleportStones)), FromPlace: 0, FromSlot: 10, ToPlace: 2, ToSlot: 0}}},
		{Msg: &pb.ClientMessage_StorageGold{StorageGold: &pb.StorageGold{NpcId: npc + uint32(len(townNPCs)+len(teleportStones)), Amount: 10}}},
		// Retiradas do armazém (o outro sentido).
		{Msg: &pb.ClientMessage_StorageMove{StorageMove: &pb.StorageMove{NpcId: npc + uint32(len(townNPCs)+len(teleportStones)), FromPlace: 2, FromSlot: 0, ToPlace: 0, ToSlot: 30}}},
		{Msg: &pb.ClientMessage_StorageGold{StorageGold: &pb.StorageGold{NpcId: npc + uint32(len(townNPCs)+len(teleportStones)), Amount: -10}}},
		{Msg: &pb.ClientMessage_ChangeLook{ChangeLook: &pb.ChangeLook{NpcId: npc + 2, Look: &pb.BodyLook{Skin: 1, Hair: 2, Outfit: 3, Head: "rogue"}}}},
		{Msg: &pb.ClientMessage_TradeCmd{TradeCmd: &pb.TradeCmd{Kind: pb.TradeCmdKind(1), TargetId: 2}}},
		{Msg: &pb.ClientMessage_Craft{Craft: &pb.Craft{NpcId: npc + 1, RecipeId: fuzzRecipeIDs[0]}}},
		{Msg: &pb.ClientMessage_ClientStats{ClientStats: &pb.ClientStats{FpsAvg: 58.5, FpsLow: 41, Device: "desktop windows", Seconds: 60}}},
		{Msg: &pb.ClientMessage_FriendCmd{FriendCmd: &pb.FriendCmd{Kind: pb.FriendCmdKind(1), Name: "Beto"}}},
		{Msg: &pb.ClientMessage_MailCmd{MailCmd: &pb.MailCmd{Kind: pb.MailCmdKind(1), To: "Beto", Subject: "oi", Body: "tudo bem?"}}},
		{Msg: &pb.ClientMessage_RankingRequest{RankingRequest: &pb.RankingRequest{}}},
		{Msg: &pb.ClientMessage_AchievementCmd{AchievementCmd: &pb.AchievementCmd{Kind: pb.AchievementCmdKind(1), Id: fuzzAchIDs[0]}}},
	}
	return out
}

// fuzzSeedCorpus: o corpus inicial dos três alvos, por arquivo (caminho relativo a
// internal/zone). Bytes e conexão: cada mensagem válida serializada. Semântico: a zona
// fixa e a mensagem válida mandada pela Ana e depois pelo Beto.
func fuzzSeedCorpus() map[string][]byte {
	out := map[string][]byte{}
	nFields := len(fuzzOneofFields())
	for i, m := range fuzzValidMessages() {
		name := string(m.ProtoReflect().WhichOneof(m.ProtoReflect().Descriptor().Oneofs().ByName("msg")).Name())
		if _, dup := out["testdata/fuzz/FuzzZoneBytes/valid_"+name]; dup {
			name = fmt.Sprintf("%s_%d", name, i)
		}
		data, err := proto.Marshal(m)
		if err != nil {
			panic(err)
		}
		out["testdata/fuzz/FuzzZoneBytes/valid_"+name] = data
		out["../net/testdata/fuzz/FuzzConnBytes/valid_"+name] = data
		// newFuzzHarness + 2 passos: quem, tipo = "mensagem válida", qual, ticks.
		sem := append(fuzzFixedSetup(fuzzSpotFor(m)), 0, byte(nFields), byte(i), 1, 1, byte(nFields), byte(i), 20)
		out["testdata/fuzz/FuzzZoneSemantic/valid_"+name] = sem
	}
	return out
}

func fuzzCorpusFile(data []byte) string {
	return fmt.Sprintf("go test fuzz v1\n[]byte(%q)\n", data)
}

// TestFuzzSeedCorpus confere que o corpus inicial em testdata está em dia com
// fuzzValidMessages. Para regerar: EREVIA_WRITE_FUZZ_CORPUS=1 go test ./internal/zone -run TestFuzzSeedCorpus
func TestFuzzSeedCorpus(t *testing.T) {
	write := os.Getenv("EREVIA_WRITE_FUZZ_CORPUS") != ""
	for path, data := range fuzzSeedCorpus() {
		want := fuzzCorpusFile(data)
		if write {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil || strings.ReplaceAll(string(got), "\r\n", "\n") != want {
			t.Errorf("%s desatualizado (EREVIA_WRITE_FUZZ_CORPUS=1 go test ./internal/zone -run TestFuzzSeedCorpus)", path)
		}
	}
}

// TestFuzzValidMessagesCoverOneof: o corpus inicial tem uma mensagem de cada tipo.
func TestFuzzValidMessagesCoverOneof(t *testing.T) {
	have := map[protoreflect.Name]bool{}
	for _, m := range fuzzValidMessages() {
		have[m.ProtoReflect().WhichOneof(m.ProtoReflect().Descriptor().Oneofs().ByName("msg")).Name()] = true
	}
	for _, fd := range fuzzOneofFields() {
		if !have[fd.Name()] {
			t.Errorf("fuzzValidMessages não tem %s", fd.Name())
		}
	}
}

// ---------- Alvo 2: semântico ----------

// FuzzZoneSemantic: os bytes escolhem a zona (ver newFuzzHarness) e uma sequência de
// fuzzMaxSteps mensagens: quem manda, o tipo do oneof, o valor de cada campo (de listas
// de valores extremos e de ids que existem na zona) e quantos ticks rodam depois. Quando
// os bytes acabam, o resto sai de um gerador com semente neles (ver fuzzReader).
// Depois de cada mensagem confere a contabilidade e o estado.
//
// O corpus inicial (zona fixa + uma mensagem válida de cada tipo, mandada pelos dois)
// está em testdata/fuzz/FuzzZoneSemantic.
func FuzzZoneSemantic(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		r := newFuzzStream(data)
		h := newFuzzHarness(t, r)
		h.checkState()
		gen := &fuzzGen{h: h, r: r}
		for range fuzzMaxSteps {
			gen.who = int(r.byte() & 1)
			msg := gen.message()
			h.step(gen.who, msg, gen.ticks())
		}
	})
}

const fuzzMaxSteps = 30

// fuzzGen monta mensagens a partir do leitor.
type fuzzGen struct {
	h   *fuzzHarness
	r   *fuzzReader
	who int // quem vai mandar a mensagem (0 ou 1)
}

func (g *fuzzGen) sender() *entity { return g.h.z.players[g.h.ids[g.who]] }

// usedSlot: um espaço ocupado da bolsa, do equipamento ou do armazém de quem manda.
func (g *fuzzGen) usedSlot() (uint32, bool) {
	p := g.sender()
	if p == nil {
		return 0, false
	}
	var used []uint32
	for i, s := range p.pl.inv.bag {
		if !s.empty() {
			used = append(used, uint32(i))
		}
	}
	for i, s := range p.pl.inv.equip {
		if !s.empty() {
			used = append(used, uint32(i))
		}
	}
	if st := g.h.z.storages[p.pl.accountID]; st != nil {
		for i, s := range st.slots {
			if !s.empty() {
				used = append(used, uint32(i))
			}
		}
	}
	if len(used) == 0 {
		return 0, false
	}
	return used[g.r.pick(len(used))], true
}

// nearestNPC: o NPC mais perto de quem manda.
func (g *fuzzGen) nearestNPC() uint32 {
	p, best, bestD := g.sender(), npcIDBase, math.Inf(1)
	if p == nil {
		return best
	}
	for id, n := range g.h.z.npcs {
		if d := dist(p.pos, n.pos); d < bestD || (d == bestD && id < best) {
			best, bestD = id, d
		}
	}
	return best
}

// ticks entre mensagens: quase sempre poucos, às vezes 3 s (canais de 2 s, efeitos, andar).
func (g *fuzzGen) ticks() int {
	b := g.r.byte()
	if b >= 0xF0 {
		return 60
	}
	return int(b % 4)
}

func (g *fuzzGen) message() *pb.ClientMessage {
	fields := fuzzOneofFields()
	k := g.r.pick(len(fields) + 4)
	if k >= len(fields) {
		// Às vezes uma mensagem válida inteira (pedir troca, convidar, comprar…), para as
		// sequências chegarem aos estados do meio.
		valid := fuzzValidMessages()
		return valid[g.r.pick(len(valid))]
	}
	fd := fields[k]
	msg := &pb.ClientMessage{}
	m := msg.ProtoReflect()
	inner := m.NewField(fd).Message()
	g.fill(inner, 0)
	m.Set(fd, protoreflect.ValueOfMessage(inner))
	return msg
}

// fill preenche os campos de m. Cada campo pode ficar no padrão (zero).
func (g *fuzzGen) fill(m protoreflect.Message, depth int) {
	fields := m.Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if g.r.byte()%4 == 0 {
			continue
		}
		switch {
		case fd.IsList():
			l := m.Mutable(fd).List()
			n := g.r.pick(5)
			if n == 4 && g.r.byte()&1 != 0 {
				n = 200 // lista enorme
			}
			for range n {
				if fd.Kind() == protoreflect.MessageKind {
					el := l.NewElement()
					if depth < 3 {
						g.fill(el.Message(), depth+1)
					}
					l.Append(el)
				} else {
					l.Append(g.scalar(fd))
				}
			}
		case fd.IsMap():
			// Nenhuma mensagem do cliente tem mapa hoje.
		case fd.Kind() == protoreflect.MessageKind:
			sub := m.NewField(fd).Message()
			if depth < 3 {
				g.fill(sub, depth+1)
			}
			m.Set(fd, protoreflect.ValueOfMessage(sub))
		default:
			m.Set(fd, g.scalar(fd))
		}
	}
}

func (g *fuzzGen) scalar(fd protoreflect.FieldDescriptor) protoreflect.Value {
	if v, ok := g.named(fd); ok {
		return v
	}
	r := g.r
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(r.byte()&1 != 0)
	case protoreflect.EnumKind:
		vals := fd.Enum().Values()
		k := r.pick(vals.Len() + 3)
		if k < vals.Len() {
			return protoreflect.ValueOfEnum(vals.Get(k).Number())
		}
		return protoreflect.ValueOfEnum([]protoreflect.EnumNumber{99, -1, math.MaxInt32}[k-vals.Len()])
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(g.u32())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32([]int32{0, 1, -1, 2, 5, 40, math.MaxInt32, math.MinInt32, int32(r.u32())}[r.pick(9)])
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(g.i64())
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		vals := []uint64{0, 1, 2, 3, math.MaxUint64, math.MaxInt64, 1 << 63, uint64(g.h.social.seq)}
		if k := r.pick(len(vals) + 1); k < len(vals) {
			return protoreflect.ValueOfUint64(vals[k])
		}
		return protoreflect.ValueOfUint64(r.u64())
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(float32(g.f64()))
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(g.f64())
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(g.str())
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes(slices.Clone(r.b[:min(len(r.b), r.pick(64))]))
	}
	panic("tipo de campo novo no protocolo: " + fd.Kind().String())
}

// named: para campos conhecidos pelo nome, 3 em 4 vezes um valor que faz sentido para
// aquele campo (lugar, espaço, NPC, alvo, skill, convite, item, receita…), para o fuzz
// passar das primeiras validações. No resto (e em campos novos) vale o gerador do tipo.
func (g *fuzzGen) named(fd protoreflect.FieldDescriptor) (protoreflect.Value, bool) {
	r, z := g.r, g.h.z
	if r.byte()%4 == 0 {
		return protoreflect.Value{}, false
	}
	name := string(fd.Name())
	u32 := func(v uint32) (protoreflect.Value, bool) { return protoreflect.ValueOfUint32(v), true }
	str := func(v string) (protoreflect.Value, bool) { return protoreflect.ValueOfString(v), true }
	switch fd.Kind() {
	case protoreflect.Uint32Kind:
		switch {
		case strings.HasSuffix(name, "place"):
			return u32(uint32(r.byte() % 3)) // bolsa, equipado, armazém
		case strings.HasSuffix(name, "slot"):
			if s, ok := g.usedSlot(); ok && r.byte()&1 == 0 {
				return u32(s)
			}
			return u32(uint32(r.byte() % 61)) // bolsa 40, equipamento 13, armazém 60
		case name == "count":
			return u32([]uint32{0, 1, 1, 2, 5, 15, 999, 1000, math.MaxUint32}[r.pick(9)])
		case name == "npc_id":
			if r.byte()&1 == 0 {
				return u32(g.nearestNPC())
			}
			return u32(fuzzPickKey(r, z.npcs, npcIDBase))
		case name == "target_id":
			if r.byte()&3 == 0 {
				return u32(fuzzPickKey(r, z.objects, objectIDBase+1))
			}
			return u32(g.h.ids[r.byte()&1])
		case name == "skill_id":
			if p := z.players[g.h.ids[r.byte()&1]]; p != nil {
				tree := game.ClassTree(p.pl.class)
				return u32(tree[r.pick(len(tree))].ID)
			}
		case name == "invite_id":
			return u32(fuzzPickKey(r, z.invites, 1))
		case name == "dest_id":
			return u32(uint32(r.byte() % 8))
		case name == "id": // PickUp
			return u32(fuzzPickKey(r, z.ground, groundIDBase+1))
		case name == "rank":
			return u32(uint32(r.byte() % 6))
		}
	case protoreflect.Int64Kind:
		if name == "amount" || name == "gold" {
			var gold, stored int64
			if p := z.players[g.h.ids[0]]; p != nil {
				gold = p.pl.inv.gold
			}
			if st := z.storages[g.h.accts[0]]; st != nil {
				stored = st.gold
			}
			return protoreflect.ValueOfInt64([]int64{1, 10, gold, gold + 1, -stored, -stored - 1, -1}[r.pick(7)]), true
		}
	case protoreflect.StringKind:
		switch name {
		case "item_id":
			return str(fuzzItemIDs[r.pick(len(fuzzItemIDs))])
		case "recipe_id":
			return str(fuzzRecipeIDs[r.pick(len(fuzzRecipeIDs))])
		case "id": // AchievementCmd
			return str(fuzzAchIDs[r.pick(len(fuzzAchIDs))])
		case "name", "to":
			return str(g.h.names[r.byte()&1])
		}
	}
	return protoreflect.Value{}, false
}

// u32: slots, quantidades e ids (os que existem na zona, de outra entidade e os absurdos).
func (g *fuzzGen) u32() uint32 {
	r, z := g.r, g.h.z
	switch r.byte() % 8 {
	case 0, 1:
		return uint32(r.byte() % 64) // slots e quantidades pequenas (0–63)
	case 2:
		return []uint32{0, 1, 2, 3, 40, 59, 60, 255, 256, 999, 1000, math.MaxInt32, 1 << 31, math.MaxUint32}[r.pick(14)]
	case 3:
		return g.h.ids[r.byte()&1] // os jogadores
	case 4:
		return fuzzPickKey(r, z.npcs, npcIDBase)
	case 5:
		return fuzzPickKey(r, z.ground, groundIDBase+1)
	case 6:
		// Convites, objetos, monstros que não existem e skills da classe de alguém.
		switch r.byte() % 4 {
		case 0:
			return fuzzPickKey(r, z.invites, 1)
		case 1:
			return fuzzPickKey(r, z.objects, objectIDBase+1)
		case 2:
			return []uint32{monsterIDBase, summonIDBase, 3, 99}[r.pick(4)]
		}
		p := z.players[g.h.ids[r.byte()&1]]
		if p == nil {
			return 1
		}
		tree := game.ClassTree(p.pl.class)
		return tree[r.pick(len(tree))].ID
	}
	return r.u32()
}

// fuzzPickKey escolhe uma chave do mapa (em ordem, para o fuzz ser reproduzível).
func fuzzPickKey[V any](r *fuzzReader, m map[uint32]V, fallback uint32) uint32 {
	if len(m) == 0 {
		return fallback
	}
	keys := make([]uint32, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys[r.pick(len(keys))]
}

func (g *fuzzGen) i64() int64 {
	r := g.r
	var gold int64
	if p := g.h.z.players[g.h.ids[0]]; p != nil {
		gold = p.pl.inv.gold
	}
	vals := []int64{0, 1, -1, 10, 1000, math.MaxInt64, math.MinInt64, math.MaxInt64 / 2, gold, gold + 1, -gold, math.MaxInt32 + 1}
	if k := r.pick(len(vals) + 1); k < len(vals) {
		return vals[k]
	}
	return int64(r.u64())
}

// f64: posições (perto de cada NPC, na borda, fora do mapa) e os valores especiais.
func (g *fuzzGen) f64() float64 {
	r := g.r
	switch r.byte() % 4 {
	case 0:
		s := g.h.spots[r.pick(len(g.h.spots))]
		return []float64{s.X, s.Z}[r.byte()&1]
	case 1:
		return []float64{0, math.Copysign(0, -1), math.NaN(), math.Inf(1), math.Inf(-1),
			math.MaxFloat32, -math.MaxFloat32, math.SmallestNonzeroFloat32, math.MaxFloat64,
			149.999, 150, 150.001, -150, -150.001, 1e9, -1e9}[r.pick(16)]
	case 2:
		return float64(int8(r.byte())) * 2 // -256 a 254 m
	}
	return math.Float64frombits(r.u64())
}

var fuzzCommands = []string{
	"/w", "/r", "/g", "/c", "/convidar", "/amigo", "/amigo remover", "/amigos", "/correio", "/bug",
	"/acenar", "/sentar", "/dancar", "/dançar", "/gm", "/kick", "/mute", "/unmute", "/ban", "/unban",
	"/tp", "/goto", "/summon", "/give", "/anuncio", "/online", "/raid", "/sair", "/clã", "/grupo",
}

// str: vazio, nomes (o próprio, o do outro, offline, com maiúsculas trocadas), ids de
// itens/receitas/conquistas, comandos de chat com argumentos extremos, controle, UTF-8
// inválido, texto enorme e lixo.
func (g *fuzzGen) str() string {
	r := g.r
	word := func() string {
		switch r.byte() % 10 {
		case 0:
			return ""
		case 1:
			return g.h.names[r.byte()&1]
		case 2:
			return []string{"ana", "BETO", "Caio", "Ninguem", " Beto ", "Ana\x00"}[r.pick(6)]
		case 3:
			return fuzzItemIDs[r.pick(len(fuzzItemIDs))]
		case 4:
			return fuzzRecipeIDs[r.pick(len(fuzzRecipeIDs))]
		case 5:
			return fuzzAchIDs[r.pick(len(fuzzAchIDs))]
		case 6:
			return []string{"NaN", "Inf", "-Inf", "1e309", "-0", "0", "1", "-1", "99999999999999999999",
				"9223372036854775807", "-9223372036854775808", "4294967295", "0x10", "1.5", "150", "-150"}[r.pick(16)]
		case 7:
			return []string{"\x00", "\x1b[31m", "\n\r\t", "\xff\xfe", "​", "‮", "á", "😀",
				"<script>", "'; DROP TABLE characters;--", "%s%n", "\U0010FFFF"}[r.pick(12)]
		case 8:
			return strings.Repeat([]string{"x", "é", "😀", "\x00", " "}[r.pick(5)], []int{17, 200, 1000, 5000}[r.pick(4)])
		}
		return string(r.b[:min(len(r.b), r.pick(24))])
	}
	var s string
	switch r.byte() % 4 {
	case 1:
		// Comando de chat com até 3 argumentos.
		parts := []string{fuzzCommands[r.pick(len(fuzzCommands))]}
		for range r.pick(4) {
			parts = append(parts, word())
		}
		s = strings.Join(parts, " ")
	case 2:
		s = word() + word()
	default:
		s = word()
	}
	if r.byte()%16 == 0 {
		return s // às vezes cru: com UTF-8 inválido o protobuf recusa e nada chega à zona
	}
	return strings.ToValidUTF8(s, "�")
}

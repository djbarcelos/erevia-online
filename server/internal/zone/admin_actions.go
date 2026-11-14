package zone

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"erevia/server/internal/game"
)

// Ações do portal de administração sobre jogadores (/admin → Jogadores).
//
// O estado de um personagem tem um dono só, e quem é o dono depende de onde ele está:
//
//   - online: a zona (memória). A ação roda na goroutine da zona e o personagem é salvo
//     na hora, pelo caminho normal (save_seq novo).
//   - "recente" (saiu há menos de recentKeep): também a zona. Ao relogar, z.recent vale
//     mais que o banco (o salvamento da saída pode nem ter terminado), então a ação muda o
//     z.recent e salva esse estado com um save_seq novo. Mexer no banco aqui seria perdido
//     (dar item) ou duplicado (tirar item) no próximo login.
//   - offline de verdade: o banco. A zona põe uma trava no personagem (hold) e só então a
//     API faz a transação; enquanto ela dura, quem tentar entrar com o personagem é
//     recusado. Depois, a zona anota a hora da mudança, e um Join carregado do banco antes
//     disso (JoinInfo.LoadedAt mais velho) também é recusado: o cliente reconecta e carrega
//     de novo, já com a mudança. Assim um login que corre junto com a ação nunca entra com
//     o estado velho e o salva por cima.
//
// Mudanças na conta (silêncio, banimento, GM) vão primeiro para o banco, que é de onde o
// login lê; depois a zona aplica em quem está online e anota a hora, com a mesma recusa
// para um Join carregado antes.

const (
	// adminHoldTTL: prazo da trava de um personagem offline. A transação da API tem prazo
	// bem menor (adminTxTimeout na API); se a API sumir, a trava vence sozinha e vale como
	// uma mudança naquela hora (um Join carregado antes é recusado).
	adminHoldTTL = 30 * time.Second
	// adminStaleKeep: por quanto tempo a zona lembra de uma mudança feita pelo portal.
	adminStaleKeep = recentKeep

	adminStaleReason = "Seu personagem foi alterado por um GM. Entre de novo."
)

// adminActionWait: depois que a zona pegou o pedido, quanto a API espera a resposta. Se
// vencer, a ação pode ter acontecido: a API responde "não confirmado", nunca tenta de novo.
// (Variável só para os testes.)
var adminActionWait = 10 * time.Second

// Erros das ações (a API traduz para o status HTTP).
var (
	// ErrZoneUnconfirmed: a zona pegou o pedido mas não respondeu no prazo. A ação pode ter
	// acontecido ou não; quem pediu deve conferir a ficha antes de repetir.
	ErrZoneUnconfirmed = errors.New("zone took the request but did not answer in time")
	// ErrNotOnline: a ação só existe para quem está online (expulsar).
	ErrNotOnline = errors.New("character is not online")
	// ErrCharacterBusy: outra ação do portal no mesmo personagem offline ainda não acabou.
	ErrCharacterBusy = errors.New("another admin action is running on this character")
)

// AdminInvalid é um pedido que não dá para cumprir (item desconhecido, bolsa cheia…). A
// mensagem vai para o GM como está.
type AdminInvalid struct{ Msg string }

func (e *AdminInvalid) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &AdminInvalid{Msg: fmt.Sprintf(format, args...)}
}

// askZoneAction é o askZone das ações que mudam algo: desiste (ErrZoneBusy) só se a zona
// nem pegou o pedido; depois de pegar, espera até adminActionWait e, se não vier a
// resposta, devolve ErrZoneUnconfirmed (nunca "deu errado", que levaria a repetir e
// duplicar). A espera não depende do contexto de quem pediu: o navegador fechar no meio
// não muda o que a zona já está fazendo.
func askZoneAction[T any](ctx context.Context, z *Zone, fn func(z *Zone) T) (T, error) {
	qctx, cancel := context.WithTimeout(ctx, AdminTimeout)
	defer cancel()
	reply := make(chan T, 1)
	var zero T
	select {
	case z.events <- socialDone{fn: func() { reply <- fn(z) }}:
	case <-qctx.Done():
		return zero, ErrZoneBusy
	}
	t := time.NewTimer(adminActionWait)
	defer t.Stop()
	select {
	case v := <-reply:
		return v, nil
	case <-t.C:
		return zero, ErrZoneUnconfirmed
	}
}

// adminState: o que a zona guarda para o portal (criado no primeiro uso).
type adminState struct {
	holds    map[uint32]adminHold // personagem offline com transação em andamento
	edits    map[uint32]time.Time // personagem → quando o portal o mudou no banco
	accounts map[int64]time.Time  // conta → quando o portal a mudou no banco
	holdSeq  uint64

	ops opsState // anúncios agendados e aviso de manutenção (admin_ops.go)
}

type adminHold struct {
	token   uint64
	expires time.Time
}

func (z *Zone) admin() *adminState {
	if z.adm == nil {
		z.adm = &adminState{holds: map[uint32]adminHold{}, edits: map[uint32]time.Time{}, accounts: map[int64]time.Time{}}
	}
	return z.adm
}

// pruneAdmin esquece mudanças velhas e troca travas vencidas pela hora em que venceram.
func (a *adminState) prune(now time.Time) {
	for id, h := range a.holds {
		if !now.Before(h.expires) {
			delete(a.holds, id)
			a.edits[id] = h.expires
		}
	}
	for id, t := range a.edits {
		if now.Sub(t) > adminStaleKeep {
			delete(a.edits, id)
		}
	}
	for id, t := range a.accounts {
		if now.Sub(t) > adminStaleKeep {
			delete(a.accounts, id)
		}
	}
}

// refuseAdminStale recusa o Join de um personagem que o portal está mudando no banco, ou
// que foi carregado antes da última mudança (o estado do JoinInfo é velho). Sem Kicked: o
// cliente reconecta sozinho e carrega de novo.
func (z *Zone) refuseAdminStale(info JoinInfo) bool {
	if z.adm == nil {
		return false
	}
	a := z.adm
	now := time.Now()
	a.prune(now)
	_, held := a.holds[info.ID]
	stale := held
	if t, ok := a.edits[info.ID]; ok && info.LoadedAt.Before(t) {
		stale = true
	}
	if t, ok := a.accounts[info.AccountID]; ok && info.AccountID != 0 && info.LoadedAt.Before(t) {
		stale = true
	}
	if !stale {
		return false
	}
	z.log.Info("join refused: admin change", "id", info.ID, "held", held)
	info.Peer.Close(adminStaleReason)
	return true
}

// ---------- Edições de personagem (itens, moedas e posição) ----------

// Tipos de edição.
const (
	EditGiveItem = "give_item"
	EditTakeItem = "take_item"
	EditGold     = "gold"     // Gold > 0 dá, < 0 tira
	EditTeleport = "teleport" // X, Z
)

// AdminEdit é uma mudança no personagem pedida pelo portal.
type AdminEdit struct {
	Kind      string
	Item      string // id do item (EditGiveItem/EditTakeItem)
	Count     int
	Gold      int64
	X, Z      float64
	AccountID int64 // dono (telemetria quando o personagem não está online)
	Level     int
}

// AdminEditResult é o que de fato mudou.
type AdminEditResult struct {
	Count int     // unidades dadas ou tiradas (itens)
	Gold  int64   // moedas que entraram (+) ou saíram (−)
	X, Z  float64 // posição final (teleporte)
	Text  string  // resumo para o GM ("3× Poção de Vida")
}

// Onde a ação foi feita.
const (
	WhereOnline  = "online"  // na memória, com o jogador conectado
	WhereRecent  = "recent"  // no estado recente (saiu há pouco), salvo com save_seq novo
	WhereOffline = "offline" // a API precisa fazer a transação no banco (com a trava)
)

// AdminActionResult é a resposta da zona a uma edição.
type AdminActionResult struct {
	Where  string
	Name   string
	Result AdminEditResult // vazio quando Where = offline
	Edit   AdminEdit       // a edição como a zona a aceitou (posição já dentro do mapa)
	Hold   uint64          // offline: a trava a soltar com AdminRelease
	Err    error
}

// maxAdminGold: limite de moedas por ação (o mesmo do /give).
const maxAdminGold = maxGiveGold

// normalize confere o pedido sem olhar o personagem.
func (z *Zone) normalizeEdit(e AdminEdit) (AdminEdit, error) {
	switch e.Kind {
	case EditGiveItem, EditTakeItem:
		def := game.Items[e.Item]
		if def == nil || def.Kind == game.KindGold {
			return e, invalid("Item desconhecido: %s.", e.Item)
		}
		if e.Count < 1 || e.Count > maxGiveCount {
			return e, invalid("Quantidade inválida (1 a %d).", maxGiveCount)
		}
	case EditGold:
		if e.Gold == 0 || e.Gold > maxAdminGold || e.Gold < -maxAdminGold {
			return e, invalid("Quantidade de moedas inválida.")
		}
	case EditTeleport:
		if math.IsNaN(e.X) || math.IsNaN(e.Z) || math.IsInf(e.X, 0) || math.IsInf(e.Z, 0) {
			return e, invalid("Coordenadas inválidas.")
		}
		p := clampToZone(Vec2{X: e.X, Z: e.Z}, z.bounds())
		e.X, e.Z = p.X, p.Z
	default:
		return e, invalid("Ação desconhecida.")
	}
	return e, nil
}

// AdminEdit faz a edição onde o personagem estiver (ver o começo do arquivo). Com
// Where = offline, a zona só travou o personagem: quem chama faz a transação no banco
// (ApplyOfflineEdit) e depois chama AdminRelease com o Hold.
func (z *Zone) AdminEdit(ctx context.Context, id uint32, e AdminEdit) (AdminActionResult, error) {
	r, err := askZoneAction(ctx, z, func(z *Zone) AdminActionResult { return z.adminEdit(id, e) })
	if err != nil {
		return r, err
	}
	return r, r.Err
}

func (z *Zone) adminEdit(id uint32, e AdminEdit) AdminActionResult {
	a := z.admin()
	now := time.Now()
	a.prune(now)
	e, err := z.normalizeEdit(e)
	if err != nil {
		return AdminActionResult{Err: err}
	}
	if p := z.players[id]; p != nil {
		res, err := z.editOnline(p, e)
		return AdminActionResult{Where: WhereOnline, Name: p.name, Result: res, Edit: e, Err: err}
	}
	if _, held := a.holds[id]; held {
		return AdminActionResult{Err: ErrCharacterBusy}
	}
	st, ok := z.recent[id]
	if !ok {
		if p, pend := z.pending[id]; pend {
			st, ok = recentState{state: p}, true
		}
	}
	if ok {
		res, err := z.editRecent(id, st.state, e)
		return AdminActionResult{Where: WhereRecent, Result: res, Edit: e, Err: err}
	}
	a.holdSeq++
	a.holds[id] = adminHold{token: a.holdSeq, expires: now.Add(adminHoldTTL)}
	return AdminActionResult{Where: WhereOffline, Edit: e, Hold: a.holdSeq}
}

// AdminRelease solta a trava de um personagem offline depois da transação (feita ou não)
// e anota a hora: um Join carregado antes disso é recusado.
func (z *Zone) AdminRelease(ctx context.Context, id uint32, hold uint64) error {
	_, err := askZoneAction(ctx, z, func(z *Zone) bool {
		a := z.admin()
		if h, ok := a.holds[id]; ok && h.token == hold {
			delete(a.holds, id)
		}
		a.edits[id] = time.Now()
		return true
	})
	return err
}

// editOnline muda o personagem conectado e salva na hora (não espera o saveDue: um panic
// depois disso voltaria ao último estado salvo, sem a mudança).
func (z *Zone) editOnline(p *entity, e AdminEdit) (AdminEditResult, error) {
	inv := &p.pl.inv
	var res AdminEditResult
	gear := false
	switch e.Kind {
	case EditGiveItem:
		def := game.Items[e.Item]
		given := e.Count - inv.add(def, e.Count)
		if given == 0 {
			return res, invalid("A bolsa de %s está cheia.", p.name)
		}
		res.Count, res.Text = given, itemLabel(def, given)
		z.system(p, "Você recebeu "+res.Text+" de um GM.")
		z.trackItem(p, true, def.ID, given, "gm")
	case EditTakeItem:
		def := game.Items[e.Item]
		took, g, err := takeForAdmin(inv, def, e.Count)
		if err != nil {
			return res, err
		}
		gear = g
		res.Count, res.Text = took, itemLabel(def, took)
		z.system(p, "Um GM retirou "+res.Text+" do seu inventário.")
		z.trackItem(p, false, def.ID, took, "gm")
	case EditGold:
		if e.Gold < 0 && inv.gold < -e.Gold {
			return res, invalid("%s só tem %d moedas.", p.name, inv.gold)
		}
		inv.gold += e.Gold
		res.Gold, res.Text = e.Gold, goldText(e.Gold)
		if e.Gold > 0 {
			z.system(p, fmt.Sprintf("Você recebeu %d moedas de um GM.", e.Gold))
		} else {
			z.system(p, fmt.Sprintf("Um GM retirou %d moedas suas.", -e.Gold))
		}
		z.trackGold(p, "gm", e.Gold)
	case EditTeleport:
		if p.dead {
			return res, invalid("%s está morto: espere renascer.", p.name)
		}
		z.gmPlace(p, Vec2{X: e.X, Z: e.Z}, false)
		res.X, res.Z = p.pos.X, p.pos.Z
		res.Text = fmt.Sprintf("%.0f, %.0f", p.pos.X, p.pos.Z)
		z.system(p, "Um GM levou você a outro lugar.")
		z.saveNow(p)
		return res, nil
	}
	z.inventoryChanged(p, gear)
	z.saveNow(p)
	return res, nil
}

// saveNow salva o jogador já (com save_seq novo).
func (z *Zone) saveNow(p *entity) {
	p.pl.saveAt = 0
	z.save([]SavedState{z.savedState(p)}, false)
}

// editRecent muda o estado de quem saiu há pouco e o salva com um save_seq novo. O
// próximo login usa este estado (z.recent vale mais que o banco).
func (z *Zone) editRecent(id uint32, st SavedState, e AdminEdit) (AdminEditResult, error) {
	off := OfflineState{Gold: st.Gold, Items: slices.Clone(st.Items), X: st.X, Z: st.Z}
	res, ev, err := ApplyOfflineEdit(&off, e)
	if err != nil {
		return res, err
	}
	st.Gold, st.Items, st.X, st.Z = off.Gold, off.Items, off.X, off.Z
	st.Storage = nil // o armazém não mudou (e o estado guardado dele pode ser velho)
	st.Seq = z.nextSeq()
	z.recent[id] = recentState{state: st, at: z.cfg.Clock()}
	z.save([]SavedState{st}, false)
	for _, g := range ev {
		g.At, g.CharacterID, g.AccountID, g.Level, g.X, g.Z = z.cfg.Clock(), int64(id), e.AccountID, st.Level, st.X, st.Z
		z.trackEvent(g)
	}
	return res, nil
}

// trackEvent anota um evento já montado (personagem fora da zona).
func (z *Zone) trackEvent(e GameEvent) {
	if z.eventSink == nil {
		return
	}
	select {
	case z.telemetry <- e:
	default:
		z.droppedEvents++
	}
}

// OfflineState é a parte do personagem que o portal muda fora da zona.
type OfflineState struct {
	Gold  int64
	Items []game.ItemRecord
	X, Z  float64
	// ItemsChanged: os itens mudaram (quem grava troca as linhas de character_items).
	ItemsChanged bool
}

// ApplyOfflineEdit aplica a edição a um personagem que não está na zona (o estado recente
// ou, pela API, a linha do banco dentro da transação), com as mesmas regras de bolsa do
// jogo. Devolve os eventos de telemetria a gravar junto (só Kind, Ref, Amount e Extra).
// Quem chama já passou o pedido pela zona (AdminEdit), que conferiu e normalizou.
func ApplyOfflineEdit(st *OfflineState, e AdminEdit) (AdminEditResult, []GameEvent, error) {
	var res AdminEditResult
	switch e.Kind {
	case EditTeleport:
		st.X, st.Z = e.X, e.Z
		res.X, res.Z, res.Text = e.X, e.Z, fmt.Sprintf("%.0f, %.0f", e.X, e.Z)
		return res, nil, nil
	case EditGold:
		if e.Gold < 0 && st.Gold < -e.Gold {
			return res, nil, invalid("O personagem só tem %d moedas.", st.Gold)
		}
		st.Gold += e.Gold
		res.Gold, res.Text = e.Gold, goldText(e.Gold)
		return res, []GameEvent{{Kind: "gold", Ref: "gm", Amount: e.Gold}}, nil
	case EditGiveItem, EditTakeItem:
	default:
		return res, nil, invalid("Ação desconhecida.")
	}
	def := game.Items[e.Item]
	if def == nil || e.Count < 1 {
		return res, nil, invalid("Item desconhecido: %s.", e.Item)
	}
	inv, skipped := loadInventory(st.Gold, st.Items)
	if skipped > 0 {
		// Regravar a bolsa apagaria os itens que o jogo não reconhece: melhor não mexer.
		return res, nil, invalid("O inventário salvo tem %d item(ns) que o jogo não reconhece; faça a ação com o personagem online.", skipped)
	}
	var ev GameEvent
	if e.Kind == EditGiveItem {
		given := e.Count - inv.add(def, e.Count)
		if given == 0 {
			return res, nil, invalid("A bolsa do personagem está cheia.")
		}
		res.Count = given
		ev = GameEvent{Kind: "item_in", Ref: def.ID, Amount: int64(given), Extra: map[string]any{"how": "gm"}}
	} else {
		took, _, err := takeForAdmin(&inv, def, e.Count)
		if err != nil {
			return res, nil, err
		}
		res.Count = took
		ev = GameEvent{Kind: "item_out", Ref: def.ID, Amount: int64(took), Extra: map[string]any{"how": "gm"}}
	}
	res.Text = itemLabel(def, res.Count)
	st.Items, st.ItemsChanged = inv.records(), true
	return res, []GameEvent{ev}, nil
}

// takeForAdmin tira count unidades do item: primeiro da bolsa, depois do equipamento.
// Sem o bastante, não mexe em nada.
func takeForAdmin(inv *inventory, def *game.ItemDef, count int) (int, bool, error) {
	bag := inv.countOf(def.ID)
	worn := 0
	for _, s := range inv.equip {
		if !s.empty() && s.def == def {
			worn++
		}
	}
	if bag+worn < count {
		return 0, false, invalid("O personagem tem só %d de %s.", bag+worn, def.Name)
	}
	took := min(bag, count)
	if took > 0 {
		inv.takeID(def.ID, took)
	}
	gear := false
	for i := range inv.equip {
		if took >= count {
			break
		}
		if s := &inv.equip[i]; !s.empty() && s.def == def {
			*s = stack{}
			took++
			gear = true
		}
	}
	return count, gear, nil
}

func goldText(n int64) string {
	if n < 0 {
		n = -n
	}
	if n == 1 {
		return "1 moeda"
	}
	return fmt.Sprintf("%d moedas", n)
}

// ---------- Expulsar ----------

// AdminKick derruba o personagem (só online). Devolve o nome.
func (z *Zone) AdminKick(ctx context.Context, id uint32, reason string) (string, error) {
	type out struct {
		name string
		err  error
	}
	r, err := askZoneAction(ctx, z, func(z *Zone) out {
		p := z.players[id]
		if p == nil {
			return out{err: ErrNotOnline}
		}
		name := p.name
		z.kick(p, reasonText("Você foi desconectado por um GM.", reason))
		return out{name: name}
	})
	if err != nil {
		return "", err
	}
	return r.name, r.err
}

func reasonText(text, reason string) string {
	if reason = strings.TrimSpace(reason); reason != "" {
		return text + " Motivo: " + reason
	}
	return text
}

// ---------- Conta (silêncio, banimento, GM) ----------

// AccountChange é uma mudança na conta que o banco já gravou.
type AccountChange struct {
	AccountID  int64
	Mute       bool // muda o silêncio para MutedUntil (zero = tira)
	MutedUntil time.Time
	Ban        bool // derruba todos os personagens da conta
	SetGM      bool // muda o GM para GM
	GM         bool
	Reason     string
}

// AdminAccountChanged aplica em quem está online uma mudança da conta já gravada no banco
// e anota a hora (um Join carregado antes dela é recusado). Devolve os nomes online.
func (z *Zone) AdminAccountChanged(ctx context.Context, c AccountChange) ([]string, error) {
	return askZoneAction(ctx, z, func(z *Zone) []string { return z.accountChanged(c) })
}

func (z *Zone) accountChanged(c AccountChange) []string {
	a := z.admin()
	now := time.Now()
	a.prune(now)
	a.accounts[c.AccountID] = now
	var names []string
	for _, p := range z.players {
		if p.pl.accountID != c.AccountID || c.AccountID == 0 {
			continue
		}
		names = append(names, p.name)
		if c.Mute {
			p.pl.mutedUntil = c.MutedUntil
			if left := z.mutedFor(p); left > 0 {
				z.system(p, reasonText("Você foi silenciado por "+minutesText(left)+".", c.Reason))
			} else {
				z.system(p, "Você pode falar no chat de novo.")
			}
		}
		if c.SetGM && p.pl.gm != c.GM {
			p.pl.gm = c.GM
			if c.GM {
				z.system(p, "Sua conta agora é de GM (/gm lista os comandos).")
			} else {
				z.system(p, "Sua conta deixou de ser de GM.")
			}
		}
	}
	if c.Ban {
		for _, p := range z.players {
			if p.pl.accountID == c.AccountID && c.AccountID != 0 {
				z.kick(p, reasonText("Sua conta foi banida.", c.Reason))
			}
		}
	}
	slices.Sort(names)
	return names
}

// ---------- Leitura (ficha e lista de online) ----------

// AdminOnlinePlayer é uma linha da lista de quem está online.
type AdminOnlinePlayer struct {
	ID        uint32  `json:"id"`
	AccountID int64   `json:"accountId"`
	Name      string  `json:"name"`
	Class     string  `json:"class"`
	Level     int     `json:"level"`
	X         float64 `json:"x"`
	Z         float64 `json:"z"`
	Region    string  `json:"region"`
}

// AdminOnline lista quem está online agora (por nome).
func (z *Zone) AdminOnline(ctx context.Context) ([]AdminOnlinePlayer, error) {
	return askZone(ctx, z, func(z *Zone) []AdminOnlinePlayer {
		out := make([]AdminOnlinePlayer, 0, len(z.players))
		for _, p := range z.players {
			out = append(out, AdminOnlinePlayer{
				ID: p.id, AccountID: p.pl.accountID, Name: p.name, Class: p.pl.class, Level: p.level,
				X: p.pos.X, Z: p.pos.Z, Region: RegionAt(p.pos.X, p.pos.Z),
			})
		}
		slices.SortFunc(out, func(a, b AdminOnlinePlayer) int {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
		return out
	})
}

// AdminLive é o que a zona sabe de um personagem (mais novo que o banco quando Online ou
// Recent). Clã e armazém vêm da zona sempre que ela os tem em memória.
type AdminLive struct {
	Online bool
	Recent bool // saiu há pouco: o estado abaixo é o que vale no próximo login
	Held   bool // o portal está mudando o personagem no banco agora

	Level        int
	XP           int64
	Gold         int64
	Items        []game.ItemRecord
	X, Z         float64
	HP, MP       int // online: atuais; recent: -1 = cheio
	MaxHP, MaxMP int
	Dead         bool
	Karma        int
	PKs          int
	PvPKills     int
	Quests       game.QuestState
	Achievements game.AchievementState
	Title        string
	Spec         string
	GM           bool
	MutedUntil   time.Time
	JoinedAt     time.Time

	Clan    *AdminClan
	Party   []string // nomes do grupo (o líder primeiro)
	Storage *AdminStorage
}

type AdminClan struct {
	ID   int64
	Name string
	Rank string // líder, oficial ou membro
}

type AdminStorage struct {
	Gold  int64
	Items []game.ItemRecord
}

// AdminCharacter pergunta à zona o estado de um personagem.
func (z *Zone) AdminCharacter(ctx context.Context, id uint32, accountID int64) (AdminLive, error) {
	return askZone(ctx, z, func(z *Zone) AdminLive { return z.adminLive(id, accountID) })
}

func (z *Zone) adminLive(id uint32, accountID int64) AdminLive {
	var out AdminLive
	if z.adm != nil {
		_, out.Held = z.adm.holds[id]
	}
	if c := z.clanOf[int64(id)]; c != nil {
		rank := "membro"
		if m := c.members[int64(id)]; m != nil {
			switch {
			case c.leader == int64(id):
				rank = "líder"
			case m.rank == rankOfficer:
				rank = "oficial"
			}
		}
		out.Clan = &AdminClan{ID: c.id, Name: c.name, Rank: rank}
	}
	if st := z.storages[accountID]; st != nil && accountID != 0 {
		out.Storage = &AdminStorage{Gold: st.gold, Items: st.records()}
	}
	if p := z.players[id]; p != nil {
		pl := p.pl
		out.Online = true
		out.Level, out.XP, out.Gold, out.Items = p.level, pl.xp, pl.inv.gold, pl.inv.records()
		out.X, out.Z, out.HP, out.MP, out.MaxHP, out.MaxMP = p.pos.X, p.pos.Z, p.hpInt(), p.mpInt(), p.maxHPInt(), p.maxMPInt()
		out.Dead, out.Karma, out.PKs, out.PvPKills = p.dead, pl.karma, pl.pks, pl.pvpKills
		out.Quests, out.Achievements, out.Title, out.Spec = pl.quests.state(), pl.ach.state(), pl.ach.title, pl.spec
		out.GM, out.MutedUntil, out.JoinedAt = pl.gm, pl.mutedUntil, pl.joinedAt
		if pt := pl.party; pt != nil {
			names := []string{}
			for _, mid := range pt.members {
				if m := z.players[mid]; m != nil {
					if mid == pt.leader {
						names = append([]string{m.name}, names...)
					} else {
						names = append(names, m.name)
					}
				}
			}
			out.Party = names
		}
		return out
	}
	st, ok := z.recent[id]
	if !ok {
		if p, pend := z.pending[id]; pend {
			st, ok = recentState{state: p}, true
		}
	}
	if ok {
		s := st.state
		out.Recent = true
		out.Level, out.XP, out.Gold, out.Items = s.Level, s.XP, s.Gold, slices.Clone(s.Items)
		out.X, out.Z, out.HP, out.MP = s.X, s.Z, s.HP, s.MP
		out.Karma, out.PKs, out.PvPKills = s.Karma, s.PKs, s.PvPKills
		out.Quests, out.Achievements, out.Title, out.Spec = s.Quests, s.Achievements, s.Title, s.Spec
	}
	return out
}

// RegionAt: a região pela posição (os mesmos anéis da função erevia_region do banco).
func RegionAt(x, z float64) string {
	r := math.Hypot(x, z)
	switch {
	case z < -200:
		return "Vale das Pedras Vivas"
	case r < 40:
		return "Cidade"
	case r < 78:
		return "Arredores"
	case r < 112:
		return "Campos dos Ossos"
	}
	return "Ermos Partidos"
}

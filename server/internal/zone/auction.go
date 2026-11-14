package zone

// Casa de leilões (tunnel#78). Severino, o Leiloeiro, na praça, abre a janela: anunciar
// um item da bolsa (ou parte de uma pilha) por um preço total e um prazo, buscar e comprar
// na hora, cancelar, e retirar do correio o que chegou (moedas da venda, item comprado ou
// devolvido).
//
// Onde o item e as moedas ficam em cada momento (nada existe em dois lugares, nem some):
//
//   - Bolsa → custódia. Anunciar tira o item (e a taxa) da bolsa na memória e cria uma
//     operação (AuctionOp "list") que vai junto do personagem no salvamento: o banco grava
//     a bolsa nova e o anúncio na MESMA transação (store.SaveStatesAuction). Até o banco
//     confirmar, a operação vai em todo salvamento do personagem (estado recente, pendente
//     e o "último salvo" depois de um panic também a levam), e o banco a aplica uma vez só
//     (auction_ops: personagem + id da operação). Assim um estado gravado sem o item na
//     bolsa sempre tem o anúncio junto, e um estado que nunca foi gravado (queda do
//     servidor) não tem nenhum dos dois.
//   - Compra. As moedas saem da bolsa do comprador na memória, com a operação "buy" no
//     salvamento dele. Na transação, o banco trava o anúncio: se ainda está ativo (e não
//     venceu no instante do clique), marca vendido e manda duas cartas: ao vendedor, as
//     moedas menos a comissão (ele pode estar offline: nada da memória dele muda); ao
//     comprador, o item. Se outro comprador chegou antes (ou o anúncio venceu, foi
//     cancelado ou mudou), o banco devolve as moedas numa carta. A zona confirma e
//     retira sozinha: o item vai para a bolsa se couber (senão fica no correio) e o
//     reembolso volta na hora.
//   - Cancelar e vencer só mexem no banco (anúncio → carta com o item para o vendedor).
//   - Correio → bolsa. Retirar um anexo põe o item/moedas na memória com a operação
//     "claim" no salvamento: a transação marca o anexo como retirado. Só o dono retira, e
//     a zona lembra o que ele já retirou nesta sessão (uma leitura velha da caixa não deixa
//     retirar de novo). Carta com anexo não pode ser apagada antes de retirar.
//
// Telemetria (gold, item_in e item_out, extra.how), gravada na mesma transação da
// operação (nunca duas vezes, nunca sem a operação): auction_fee (taxa, sai do jogo),
// auction_sale (o item que foi anunciado sai da bolsa; as moedas da venda entram ao
// retirar), auction_buy (as moedas da compra saem; o item comprado entra ao retirar) e
// auction_return (item devolvido ou moedas reembolsadas entram ao retirar). A custódia e
// os anexos ainda não retirados ficam fora das bolsas: entrou − saiu da telemetria fecha
// com o que existe nas bolsas, e a diferença é exatamente o que está nos anúncios ativos
// e nos anexos (menos a comissão, que sai do jogo). O caos confere as duas contas.

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Regras.
const (
	AuctionMaxListings   = 10          // anúncios ativos por conta
	AuctionMinPrice      = 10          // preço total mínimo
	AuctionMaxPrice      = 100_000_000 // e máximo (bem abaixo do teto de moedas)
	AuctionFeePercent    = 2           // taxa do anúncio (não volta)
	AuctionFeeMin        = 10
	AuctionCommissionPct = 5 // sai do que o vendedor recebe
	AuctionPageSize      = 20
	AuctionMineMax       = 30 // meus anúncios: os ativos e os últimos encerrados
	AuctionSender        = "Casa de Leilões"
	AuctionExpireEvery   = 30 * time.Second // o worker devolve os vencidos
	AuctionExpireBatch   = 200

	auctionReadEvery = 0.4 // s entre duas buscas (ou leituras dos meus anúncios)
	auctionMaxOps    = 16  // operações sem confirmação por jogador
	auctionJobQueue  = 512
	auctionNameMax   = 40
	auctionMaxPage   = 500
)

// AuctionHours: prazos aceitos.
var AuctionHours = []int{12, 24, 48}

// Tipos de operação (vão no salvamento do personagem).
const (
	AuctionOpList  = "list"
	AuctionOpBuy   = "buy"
	AuctionOpClaim = "claim"
)

// extra.how da telemetria.
const (
	HowAuctionFee    = "auction_fee"
	HowAuctionSale   = "auction_sale"
	HowAuctionBuy    = "auction_buy"
	HowAuctionReturn = "auction_return"
)

// Motivos de uma operação recusada pelo banco (AuctionResult.Reason).
const (
	AuctionGone  = "gone"  // vendido, vencido, cancelado ou inexistente
	AuctionPrice = "price" // o preço não é o que o comprador viu
	AuctionOwn   = "own"   // anúncio da própria conta
	AuctionLimit = "limit" // a conta já tem o máximo de anúncios
	AuctionClash = "clash" // anexo já retirado (não devia acontecer)
)

// AuctionFee: a taxa de um anúncio pelo preço total.
func AuctionFee(price int64) int64 {
	return max(AuctionFeeMin, (price*AuctionFeePercent+99)/100)
}

// AuctionCommission: o que a casa fica de uma venda.
func AuctionCommission(price int64) int64 { return price * AuctionCommissionPct / 100 }

// AuctionOp é uma operação que mexeu na memória do personagem e vai para o banco na mesma
// transação que o estado dele. Imutável depois de criada.
type AuctionOp struct {
	ID        int64 // único por personagem (o banco aplica uma vez só)
	Kind      string
	At        time.Time
	AccountID int64
	Name      string // nome do personagem (vendedor ou comprador)
	// Anunciar.
	Item           string
	Count, Enchant int
	Price, Fee     int64
	Hours          int
	MaxListings    int
	// Comprar (Price é o que o comprador viu e pagou).
	AuctionID  int64
	Commission int64 // o que a casa fica (calculado na compra)
	// Retirar: a carta e o que a zona pôs na bolsa (o banco confere).
	MailID int64
	Gold   int64
	// Telemetria gravada na mesma transação (só quando a operação é aplicada).
	Events []GameEvent
}

// AuctionResult: o que o banco fez com uma operação.
type AuctionResult struct {
	CharacterID uint32
	OpID        int64
	Kind        string
	Fresh       bool // aplicada agora (false = já tinha sido aplicada num salvamento anterior)
	OK          bool
	Reason      string
	AuctionID   int64
	MailID      int64 // carta para quem fez a operação (item comprado, reembolso ou devolução)
	SellerID    uint32
	SellerName  string
	Item        string
	Count       int
	Enchant     int
	Price       int64
	Gold        int64 // moedas da carta de quem fez (reembolso) ou do vendedor (venda)
}

// AuctionListing é um anúncio como vem do banco.
type AuctionListing struct {
	ID            int64
	SellerID      uint32
	SellerAccount int64
	SellerName    string
	Item          string
	Count         int
	Enchant       int
	Price         int64
	CreatedAt     time.Time
	ExpiresAt     time.Time
	Status        string // active, sold, cancelled, expired
	BuyerName     string
}

// Situação de um anúncio.
const (
	AuctionActive    = "active"
	AuctionSold      = "sold"
	AuctionCancelled = "cancelled"
	AuctionExpired   = "expired"
)

// AuctionSearch: a busca já traduzida (os filtros de item viram a lista de ids).
type AuctionSearch struct {
	ItemIDs            []string // nil = qualquer item
	MinPrice, MaxPrice int64    // 0 = sem limite
	Sort               pb.AuctionSort
	Offset, Limit      int
	Now                time.Time
}

// AuctionPage: uma página da busca.
type AuctionPage struct {
	Listings []AuctionListing
	Total    int
}

// AuctionClosed: anúncio que voltou para o vendedor (cancelado ou vencido), com a carta.
type AuctionClosed struct {
	Listing AuctionListing
	MailID  int64
}

// MailAttachment: anexo de uma carta da casa de leilões.
type MailAttachment struct {
	MailID         int64
	Item           string // "" = só moedas
	Count, Enchant int
	Gold           int64
	How            string // telemetria ao retirar
	Claimed        bool
}

// ErrAuctionGone: o anúncio não está mais ativo (cancelar).
var ErrAuctionGone = fmt.Errorf("auction is not active")

// AuctionBackend: o que a casa de leilões lê e grava no banco fora do salvamento.
type AuctionBackend interface {
	SearchAuctions(ctx context.Context, q AuctionSearch) (AuctionPage, error)
	// MyAuctions: os anúncios do personagem (ativos e os últimos encerrados) e quantos a
	// conta tem ativos.
	MyAuctions(ctx context.Context, characterID uint32, accountID int64, limit int) ([]AuctionListing, int, error)
	CancelAuction(ctx context.Context, sellerID uint32, auctionID int64, at time.Time) (AuctionClosed, error)
	ExpireAuctions(ctx context.Context, now time.Time, limit int) ([]AuctionClosed, error)
	MailAttachments(ctx context.Context, characterID uint32) ([]MailAttachment, error)
}

// auctionJob roda no worker do banco e devolve o que fazer na zona (nil = nada).
type auctionJob func(ctx context.Context, b AuctionBackend) func()

// auctionHouse: o que a zona guarda da casa de leilões.
type auctionHouse struct {
	backend AuctionBackend
	jobs    chan auctionJob
	// buying: anúncio com compra a caminho do banco (de quem e qual operação). Um segundo
	// comprador na mesma zona é recusado na hora; entre zonas, quem decide é o banco.
	buying map[int64]auctionBuyer
}

type auctionBuyer struct {
	char uint32
	op   int64
}

// auctionState: a casa de leilões de um jogador online.
type auctionState struct {
	ops      []AuctionOp              // sem confirmação do banco (vão em todo salvamento)
	seen     map[int64]AuctionListing // anúncios que ele viu (busca e meus anúncios)
	mail     map[int64]MailAttachment // anexos que ele conhece (caixa de entrada e compras)
	claimed  map[int64]bool           // anexos retirados nesta sessão
	active   int                      // anúncios ativos da conta (última leitura)
	loaded   bool                     // já leu os próprios anúncios
	nextRead float64
}

// SetAuctionBackend liga a casa de leilões ao banco (antes de Run). Sem ele, o leiloeiro
// avisa que está fechado.
func (z *Zone) SetAuctionBackend(b AuctionBackend) {
	z.auction.backend = b
	if z.auction.jobs == nil {
		z.auction.jobs = make(chan auctionJob, auctionJobQueue)
	}
}

func (z *Zone) runAuctionWorker(ctx context.Context) {
	if z.auction.backend == nil {
		return
	}
	expire := time.NewTicker(AuctionExpireEvery)
	defer expire.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-z.auction.jobs:
			if fn := job(ctx, z.auction.backend); fn != nil {
				z.post(ctx, socialDone{fn: fn})
			}
		case <-expire.C:
			if fn := z.expireJob()(ctx, z.auction.backend); fn != nil {
				z.post(ctx, socialDone{fn: fn})
			}
		}
	}
}

// queueAuction entrega um trabalho ao worker sem bloquear a zona.
func (z *Zone) queueAuction(job auctionJob) bool {
	if z.auction.backend == nil {
		return false
	}
	select {
	case z.auction.jobs <- job:
		return true
	default:
		z.log.Error("auction queue full")
		return false
	}
}

func (z *Zone) auctionTry(ctx context.Context, what string, fn func(ctx context.Context) error) error {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := fn(cctx)
	if err != nil && err != ErrAuctionGone {
		z.ReportDBError("auction_"+what, err)
	}
	return err
}

// ---------- Leiloeiro ----------

// auctioneer: Severino, o Leiloeiro, ao sul do poço.
var auctioneer = npcDef{name: "Severino", title: "Leiloeiro", model: "barbarian", x: 0, z: -9.5,
	role: pb.NpcRole_NPC_ROLE_AUCTION, look: &game.Appearance{Skin: 2, Hair: 3, Outfit: 5, Head: "rogue"}}

func (z *Zone) spawnAuctioneer(id uint32) {
	d := auctioneer
	pos := z.freeSpot(Vec2{X: d.x, Z: d.z})
	z.npcs[id] = &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_NPC,
		name: d.name, model: d.model, level: 1, scale: 1,
		pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
		npc: &npcState{title: d.title, role: d.role, look: d.look},
	}
}

// auctionNPC: o leiloeiro perto do jogador (senão avisa e devolve nil).
func (z *Zone) auctionNPC(p *entity, id uint32) *entity {
	n := z.shopNPC(p, id)
	if n == nil || n.npc.role != pb.NpcRole_NPC_ROLE_AUCTION {
		return nil
	}
	return n
}

func auctionRulesPB() *pb.AuctionRules {
	r := &pb.AuctionRules{
		MaxListings: AuctionMaxListings, MinPrice: AuctionMinPrice, MaxPrice: AuctionMaxPrice,
		FeePercent: AuctionFeePercent, FeeMin: AuctionFeeMin, CommissionPercent: AuctionCommissionPct,
		PageSize: AuctionPageSize,
	}
	for _, h := range AuctionHours {
		r.Hours = append(r.Hours, uint32(h))
	}
	return r
}

func (z *Zone) sendAuction(p *entity, u *pb.AuctionUpdate) {
	u.Pending = uint32(len(p.pl.auc.ops))
	for _, a := range p.pl.auc.mail {
		if z.mailAttachmentFree(p, a) {
			u.Claimable++
		}
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Auction{Auction: u}})
}

// auctionNotice: aviso no chat e na janela.
func (z *Zone) auctionNotice(p *entity, text string) {
	z.system(p, text)
	z.sendAuction(p, &pb.AuctionUpdate{Notice: text})
}

func (z *Zone) onAuctionCmd(p *entity, m *pb.AuctionCmd) {
	if z.auction.backend == nil {
		z.system(p, "A casa de leilões está fechada agora.")
		return
	}
	switch m.Kind {
	case pb.AuctionCmdKind_AUCTION_CMD_OPEN:
		n := z.auctionNPC(p, m.NpcId)
		if n == nil {
			return
		}
		z.sendAuction(p, &pb.AuctionUpdate{Open: true, NpcId: n.id, Rules: auctionRulesPB()})
		z.auctionMine(p, true)
		z.auctionLoadMail(p)
	case pb.AuctionCmdKind_AUCTION_CMD_SEARCH:
		if z.auctionNPC(p, m.NpcId) != nil {
			z.auctionSearch(p, m.Query)
		}
	case pb.AuctionCmdKind_AUCTION_CMD_MINE:
		z.auctionMine(p, false)
	case pb.AuctionCmdKind_AUCTION_CMD_LIST:
		z.auctionList(p, m)
	case pb.AuctionCmdKind_AUCTION_CMD_BUY:
		z.auctionBuy(p, m)
	case pb.AuctionCmdKind_AUCTION_CMD_CANCEL:
		z.auctionCancel(p, m)
	case pb.AuctionCmdKind_AUCTION_CMD_CLAIM:
		z.auctionClaimCmd(p, int64(m.MailId))
	}
}

// auctionReadOK: limite de leituras (busca, meus anúncios) por jogador.
func (z *Zone) auctionReadOK(p *entity) bool {
	if z.now < p.pl.auc.nextRead {
		return false
	}
	p.pl.auc.nextRead = z.now + auctionReadEvery
	return true
}

// ---------- Busca ----------

var accentFold = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i", "ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u", "ç", "c", "ñ", "n",
)

// foldName: minúsculas e sem acento (a busca por nome não diferencia).
func foldName(s string) string { return accentFold.Replace(strings.ToLower(s)) }

// auctionItemIDs traduz os filtros de item na lista de ids (nil = qualquer; vazia = nenhum).
func auctionItemIDs(q *pb.AuctionQuery, class string) []string {
	name := strings.TrimSpace(foldName(truncateRunes(SanitizeChat(q.GetName()), auctionNameMax)))
	if name == "" && q.GetKind() == 0 && q.GetGrade() == 0 && q.GetMinLevel() == 0 && q.GetMaxLevel() == 0 && !q.GetMyClass() {
		return nil
	}
	out := []string{}
	for _, id := range game.ItemIDs() {
		d := game.Items[id]
		switch {
		case d == nil || auctionBlocked(d) != "",
			name != "" && !strings.Contains(foldName(d.Name), name),
			q.GetKind() != 0 && uint32(d.Kind) != uint32(q.GetKind()),
			q.GetGrade() != 0 && uint32(d.Grade) != q.GetGrade(),
			q.GetMinLevel() != 0 && uint32(max(d.ReqLevel, 0)) < q.GetMinLevel(),
			q.GetMaxLevel() != 0 && uint32(max(d.ReqLevel, 0)) > q.GetMaxLevel(),
			q.GetMyClass() && !d.UsableBy(class):
			continue
		}
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

func (z *Zone) auctionSearch(p *entity, q *pb.AuctionQuery) {
	if !z.auctionReadOK(p) {
		return
	}
	if q == nil {
		q = &pb.AuctionQuery{}
	}
	page := int(min(q.GetPage(), auctionMaxPage))
	s := AuctionSearch{
		ItemIDs: auctionItemIDs(q, p.pl.class), Sort: q.GetSort(),
		MinPrice: min(max(q.GetMinPrice(), 0), AuctionMaxPrice), MaxPrice: min(max(q.GetMaxPrice(), 0), AuctionMaxPrice),
		Offset: page * AuctionPageSize, Limit: AuctionPageSize, Now: z.cfg.Clock(),
	}
	if _, ok := pb.AuctionSort_name[int32(s.Sort)]; !ok {
		s.Sort = pb.AuctionSort_AUCTION_SORT_PRICE_ASC
	}
	if s.ItemIDs != nil && len(s.ItemIDs) == 0 { // nenhum item bate com os filtros
		z.sendAuction(p, &pb.AuctionUpdate{Searched: true, Page: uint32(page)})
		return
	}
	id := p.id
	if !z.queueAuction(func(ctx context.Context, b AuctionBackend) func() {
		var res AuctionPage
		err := z.auctionTry(ctx, "search", func(ctx context.Context) error {
			var err error
			res, err = b.SearchAuctions(ctx, s)
			return err
		})
		return func() {
			if z.players[id] != p {
				return
			}
			if err != nil {
				z.system(p, "Não foi possível buscar agora. Tente de novo em instantes.")
				return
			}
			u := &pb.AuctionUpdate{Searched: true, Total: uint32(res.Total), Page: uint32(page)}
			seen := p.pl.auc.seenSearch()
			maps.DeleteFunc(seen, func(_ int64, l AuctionListing) bool { return l.SellerID != p.id })
			for _, l := range res.Listings {
				seen[l.ID] = l
				u.Results = append(u.Results, z.listingPB(p, l))
			}
			z.sendAuction(p, u)
		}
	}) {
		z.system(p, "Não foi possível buscar agora. Tente de novo em instantes.")
	}
}

// seenSearch: o mapa dos anúncios vistos (criado na primeira vez). A busca nova substitui
// a anterior (os meus anúncios ficam: cancelar usa).
func (s *auctionState) seenSearch() map[int64]AuctionListing {
	if s.seen == nil {
		s.seen = map[int64]AuctionListing{}
	}
	return s.seen
}

func (z *Zone) listingPB(p *entity, l AuctionListing) *pb.AuctionListing {
	st := map[string]pb.AuctionStatus{
		AuctionSold: pb.AuctionStatus_AUCTION_STATUS_SOLD, AuctionCancelled: pb.AuctionStatus_AUCTION_STATUS_CANCELLED,
		AuctionExpired: pb.AuctionStatus_AUCTION_STATUS_EXPIRED,
	}[l.Status]
	return &pb.AuctionListing{
		Id: uint64(l.ID), ItemId: l.Item, Count: uint32(max(l.Count, 0)), Enchant: uint32(max(l.Enchant, 0)), Price: l.Price,
		SellerName: l.SellerName, CreatedAtMs: l.CreatedAt.UnixMilli(), ExpiresAtMs: l.ExpiresAt.UnixMilli(),
		Status: st, BuyerName: l.BuyerName, Mine: l.SellerID == p.id,
	}
}

// auctionMine lê os anúncios do jogador (e quantos a conta tem ativos).
func (z *Zone) auctionMine(p *entity, force bool) {
	if !force && !z.auctionReadOK(p) {
		return
	}
	id, acc := p.id, p.pl.accountID
	z.queueAuction(func(ctx context.Context, b AuctionBackend) func() {
		var list []AuctionListing
		var active int
		err := z.auctionTry(ctx, "mine", func(ctx context.Context) error {
			var err error
			list, active, err = b.MyAuctions(ctx, id, acc, AuctionMineMax)
			return err
		})
		return func() {
			if z.players[id] != p || err != nil {
				return
			}
			a := &p.pl.auc
			a.active, a.loaded = active, true
			u := &pb.AuctionUpdate{MineListed: true, Active: uint32(active)}
			for _, l := range list {
				a.seenSearch()[l.ID] = l
				u.Mine = append(u.Mine, z.listingPB(p, l))
			}
			z.sendAuction(p, u)
		}
	})
}

// ---------- Anunciar ----------

// auctionBlocked diz por que o item não pode ser leiloado ("" = pode). Moedas não são
// item. Hoje não existem itens de missão nem presos à conta; quando existirem, entram aqui.
func auctionBlocked(d *game.ItemDef) string {
	if d.Kind == game.KindGold {
		return "Moedas não se leiloam."
	}
	return ""
}

func (z *Zone) pendingOps(p *entity, kind string) int {
	n := 0
	for _, op := range p.pl.auc.ops {
		if op.Kind == kind {
			n++
		}
	}
	return n
}

// auctionOpOK: o jogador pode criar mais uma operação agora (o banco confirma as outras).
func (z *Zone) auctionOpOK(p *entity) bool {
	if len(p.pl.auc.ops) >= auctionMaxOps {
		z.auctionNotice(p, "A casa de leilões ainda está registrando suas operações. Espere um instante.")
		return false
	}
	return true
}

// auctionEvent monta um evento de telemetria do jogador (vai na transação da operação).
func (z *Zone) auctionEvent(p *entity, kind, ref string, amount int64, how string) GameEvent {
	e := GameEvent{At: z.cfg.Clock(), Kind: kind, Ref: ref, Amount: amount,
		CharacterID: int64(p.id), AccountID: p.pl.accountID, Level: p.level, X: p.pos.X, Z: p.pos.Z}
	if how != "" {
		e.Extra = map[string]any{"how": how}
	}
	return e
}

// addOp guarda a operação e salva o jogador na hora (o banco aplica no mesmo salvamento).
func (z *Zone) addOp(p *entity, op AuctionOp) {
	op.ID, op.At, op.AccountID, op.Name = z.nextSeq(), z.cfg.Clock(), p.pl.accountID, p.name
	p.pl.auc.ops = append(slices.Clone(p.pl.auc.ops), op)
	z.inventoryChanged(p, false)
	z.saveNow(p)
}

func (z *Zone) auctionList(p *entity, m *pb.AuctionCmd) {
	if z.auctionNPC(p, m.NpcId) == nil {
		return
	}
	inv := &p.pl.inv
	slot := int(m.Slot)
	if m.Slot >= game.BagSize || inv.bag[slot].empty() {
		z.auctionNotice(p, "Escolha um item da bolsa para anunciar.")
		return
	}
	s := inv.bag[slot]
	count := s.count
	if m.Count > 0 {
		count = int(min(m.Count, uint32(s.count)))
	}
	price, hours := m.Price, int(m.Hours)
	fee := AuctionFee(price)
	a := &p.pl.auc
	switch {
	case auctionBlocked(s.def) != "":
		z.auctionNotice(p, auctionBlocked(s.def))
		return
	case price < AuctionMinPrice || price > AuctionMaxPrice:
		z.auctionNotice(p, fmt.Sprintf("O preço vai de %d a %d moedas.", AuctionMinPrice, AuctionMaxPrice))
		return
	case !slices.Contains(AuctionHours, hours):
		z.auctionNotice(p, "Escolha o prazo: 12, 24 ou 48 horas.")
		return
	case !a.loaded:
		z.auctionNotice(p, "Fale com o leiloeiro para abrir a casa de leilões.")
		z.auctionMine(p, true)
		return
	case a.active+z.pendingOps(p, AuctionOpList) >= AuctionMaxListings:
		z.auctionNotice(p, fmt.Sprintf("Sua conta já tem %d anúncios ativos (o máximo).", AuctionMaxListings))
		return
	case inv.gold < fee:
		z.auctionNotice(p, fmt.Sprintf("A taxa do anúncio é de %d moedas.", fee))
		return
	case !z.auctionOpOK(p):
		return
	}
	inv.take(slot, count)
	inv.gold -= fee
	op := AuctionOp{Kind: AuctionOpList, Item: s.def.ID, Count: count, Enchant: s.enchant, Price: price, Fee: fee,
		Hours: hours, MaxListings: AuctionMaxListings, Events: []GameEvent{
			z.auctionEvent(p, "gold", HowAuctionFee, -fee, ""),
			z.auctionEvent(p, "item_out", s.def.ID, int64(count), HowAuctionSale),
		}}
	z.addOp(p, op)
	z.auctionNotice(p, fmt.Sprintf("Anunciando %s por %d moedas (taxa de %d).", itemLabelEnchant(s.def, count, s.enchant), price, fee))
}

// ---------- Comprar ----------

// buyingNow: alguém da zona já está comprando o anúncio (com a operação ainda viva).
func (z *Zone) buyingNow(id int64) bool {
	b, ok := z.auction.buying[id]
	if !ok {
		return false
	}
	var ops []AuctionOp
	if q := z.players[b.char]; q != nil {
		ops = q.pl.auc.ops
	} else if r, ok := z.recent[b.char]; ok {
		ops = r.state.Auction
	}
	if slices.ContainsFunc(ops, func(op AuctionOp) bool { return op.ID == b.op }) {
		return true
	}
	delete(z.auction.buying, id) // a operação sumiu (panic: o estado voltou ao último salvo)
	return false
}

func (z *Zone) auctionBuy(p *entity, m *pb.AuctionCmd) {
	if z.auctionNPC(p, m.NpcId) == nil {
		return
	}
	id := int64(m.AuctionId)
	l, ok := p.pl.auc.seen[id]
	now := z.cfg.Clock()
	switch {
	case !ok || l.Status != AuctionActive:
		z.auctionNotice(p, "Este anúncio não está mais na lista. Busque de novo.")
		return
	case !now.Before(l.ExpiresAt):
		z.auctionNotice(p, "Este anúncio venceu.")
		return
	case l.SellerID == p.id || (l.SellerAccount == p.pl.accountID && p.pl.accountID != 0):
		z.auctionNotice(p, "Você não pode comprar um anúncio da sua conta.")
		return
	case m.Price != l.Price:
		z.auctionNotice(p, "O preço deste anúncio mudou. Busque de novo.")
		return
	case p.pl.inv.gold < l.Price:
		z.auctionNotice(p, "Moedas insuficientes.")
		return
	case z.buyingNow(id):
		z.auctionNotice(p, "Alguém já está comprando este anúncio.")
		return
	case !z.auctionOpOK(p):
		return
	}
	p.pl.inv.gold -= l.Price
	z.addOp(p, AuctionOp{Kind: AuctionOpBuy, AuctionID: id, Price: l.Price, Commission: AuctionCommission(l.Price), Events: []GameEvent{
		z.auctionEvent(p, "gold", HowAuctionBuy, -l.Price, ""),
	}})
	if z.auction.buying == nil {
		z.auction.buying = map[int64]auctionBuyer{}
	}
	z.auction.buying[id] = auctionBuyer{char: p.id, op: p.pl.auc.ops[len(p.pl.auc.ops)-1].ID}
	delete(p.pl.auc.seen, id)
	z.sendAuction(p, &pb.AuctionUpdate{GoneId: uint64(id)})
	z.system(p, fmt.Sprintf("Comprando %s por %d moedas…", auctionLabel(l.Item, l.Count, l.Enchant), l.Price))
}

func auctionLabel(item string, count, enchant int) string {
	if d := game.Items[item]; d != nil {
		return itemLabelEnchant(d, count, enchant)
	}
	return item
}

// ---------- Cancelar ----------

func (z *Zone) auctionCancel(p *entity, m *pb.AuctionCmd) {
	if z.auctionNPC(p, m.NpcId) == nil {
		return
	}
	id := int64(m.AuctionId)
	if l, ok := p.pl.auc.seen[id]; !ok || l.SellerID != p.id || l.Status != AuctionActive {
		z.auctionNotice(p, "Escolha um dos seus anúncios ativos.")
		return
	}
	if z.buyingNow(id) {
		z.auctionNotice(p, "Alguém está comprando este anúncio agora.")
		return
	}
	pid, at := p.id, z.cfg.Clock()
	if !z.queueAuction(func(ctx context.Context, b AuctionBackend) func() {
		var c AuctionClosed
		err := z.auctionTry(ctx, "cancel", func(ctx context.Context) error {
			var err error
			c, err = b.CancelAuction(ctx, pid, id, at)
			return err
		})
		return func() {
			if z.players[pid] != p {
				return
			}
			switch {
			case err == ErrAuctionGone:
				z.auctionNotice(p, "Este anúncio já foi vendido ou venceu.")
			case err != nil:
				z.auctionNotice(p, "Não foi possível cancelar agora. Tente de novo em instantes.")
				return
			default:
				l := c.Listing
				p.pl.auc.mail[c.MailID] = MailAttachment{MailID: c.MailID, Item: l.Item, Count: l.Count, Enchant: l.Enchant, How: HowAuctionReturn}
				z.auctionNotice(p, fmt.Sprintf("Anúncio cancelado: %s voltou pelo correio (a taxa não volta).", auctionLabel(l.Item, l.Count, l.Enchant)))
				z.auctionMailCounts(p)
			}
			delete(p.pl.auc.seen, id)
			z.sendAuction(p, &pb.AuctionUpdate{GoneId: uint64(id)})
			z.auctionMine(p, true)
		}
	}) {
		z.auctionNotice(p, "Não foi possível cancelar agora. Tente de novo em instantes.")
	}
}

// ---------- Vencer ----------

// expireJob devolve os anúncios vencidos ao vendedor (pelo correio) e avisa quem está online.
func (z *Zone) expireJob() auctionJob {
	return func(ctx context.Context, b AuctionBackend) func() {
		var list []AuctionClosed
		err := z.auctionTry(ctx, "expire", func(ctx context.Context) error {
			var err error
			list, err = b.ExpireAuctions(ctx, z.cfg.Clock(), AuctionExpireBatch)
			return err
		})
		if err != nil || len(list) == 0 {
			return nil
		}
		return func() {
			for _, c := range list {
				p := z.players[c.Listing.SellerID]
				if p == nil {
					continue
				}
				l := c.Listing
				delete(p.pl.auc.seen, l.ID)
				p.pl.auc.mail[c.MailID] = MailAttachment{MailID: c.MailID, Item: l.Item, Count: l.Count, Enchant: l.Enchant, How: HowAuctionReturn}
				z.auctionNotice(p, fmt.Sprintf("Seu anúncio de %s venceu: o item voltou pelo correio.", auctionLabel(l.Item, l.Count, l.Enchant)))
				z.auctionMailCounts(p)
				z.auctionMine(p, true)
			}
		}
	}
}

// queueAuctionExpire pede uma rodada de vencimento agora (testes; o worker roda sozinho).
func (z *Zone) queueAuctionExpire() bool { return z.queueAuction(z.expireJob()) }

// ---------- Correio: retirar ----------

func attachmentLabel(a MailAttachment) string {
	var parts []string
	if a.Item != "" {
		parts = append(parts, auctionLabel(a.Item, a.Count, a.Enchant))
	}
	if a.Gold > 0 {
		parts = append(parts, goldText(a.Gold))
	}
	return strings.Join(parts, " e ")
}

// mailAttachmentFree: o anexo ainda pode ser retirado nesta sessão.
func (z *Zone) mailAttachmentFree(p *entity, a MailAttachment) bool {
	return !a.Claimed && !p.pl.auc.claimed[a.MailID]
}

func (z *Zone) auctionClaimCmd(p *entity, mailID int64) {
	if mailID != 0 {
		a, ok := p.pl.auc.mail[mailID]
		if !ok || !z.mailAttachmentFree(p, a) {
			z.system(p, "Esta carta não tem nada para retirar.")
			return
		}
		z.auctionTake(p, a, false)
		return
	}
	ids := make([]int64, 0, len(p.pl.auc.mail))
	for id, a := range p.pl.auc.mail {
		if z.mailAttachmentFree(p, a) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	took := 0
	for _, id := range ids {
		if !z.auctionTake(p, p.pl.auc.mail[id], true) {
			break
		}
		took++
	}
	switch {
	case len(ids) == 0:
		z.system(p, "Não há nada para retirar do correio.")
	case took < len(ids):
		z.system(p, fmt.Sprintf("Você retirou %d de %d anexos; o resto não coube na bolsa.", took, len(ids)))
	default:
		z.system(p, fmt.Sprintf("Você retirou %s do correio.", plural(took, "anexo", "anexos")))
	}
}

// auctionTake põe o anexo na bolsa (se couber) e registra a operação. quiet: sem aviso de
// sucesso (retirar tudo).
func (z *Zone) auctionTake(p *entity, a MailAttachment, quiet bool) bool {
	if p.dead {
		z.system(p, "Espere renascer para retirar do correio.")
		return false
	}
	inv := &p.pl.inv
	var def *game.ItemDef
	if a.Item != "" {
		if def = game.Items[a.Item]; def == nil || def.Kind == game.KindGold || a.Count <= 0 {
			z.system(p, "Este anexo tem um item que o jogo não conhece mais.")
			return false
		}
		fits := inv.room(def) >= a.Count
		if a.Enchant > 0 && def.Stack == 1 {
			fits = a.Count == 1 && inv.firstFree() >= 0
		}
		if !fits {
			z.system(p, "Sua bolsa está cheia: o anexo continua no correio (tecla O).")
			return false
		}
	}
	if a.Gold < 0 || inv.gold+a.Gold > maxGold {
		z.system(p, "Você não pode carregar mais moedas.")
		return false
	}
	if len(p.pl.auc.ops) >= auctionMaxOps {
		z.system(p, "A casa de leilões ainda está registrando suas operações. Espere um instante.")
		return false
	}
	var events []GameEvent
	if def != nil {
		inv.put(def, a.Count, a.Enchant)
		events = append(events, z.auctionEvent(p, "item_in", def.ID, int64(a.Count), a.How))
	}
	if a.Gold > 0 {
		inv.gold += a.Gold
		events = append(events, z.auctionEvent(p, "gold", a.How, a.Gold, ""))
	}
	if p.pl.auc.claimed == nil {
		p.pl.auc.claimed = map[int64]bool{}
	}
	p.pl.auc.claimed[a.MailID] = true
	z.addOp(p, AuctionOp{Kind: AuctionOpClaim, MailID: a.MailID, Item: a.Item, Count: a.Count, Enchant: a.Enchant,
		Gold: a.Gold, Events: events})
	if !quiet {
		z.system(p, "Você retirou "+attachmentLabel(a)+" do correio.")
	}
	return true
}

// auctionMailLocked: a carta tem anexo que ainda não foi retirado (não pode ser apagada).
func (z *Zone) auctionMailLocked(p *entity, mailID int64) bool {
	a, ok := p.pl.auc.mail[mailID]
	return ok && z.mailAttachmentFree(p, a)
}

// auctionAttachments lê os anexos da caixa (no worker do correio; nil sem a casa de leilões).
func (z *Zone) auctionAttachments(ctx context.Context, id uint32) ([]MailAttachment, error) {
	b := z.auction.backend
	if b == nil {
		return nil, nil
	}
	var out []MailAttachment
	err := z.auctionTry(ctx, "attachments", func(ctx context.Context) error {
		var err error
		out, err = b.MailAttachments(ctx, id)
		return err
	})
	return out, err
}

// auctionInbox: a caixa chegou; guarda os anexos e os põe nas cartas.
func (z *Zone) auctionInbox(p *entity, box *pb.MailBox, atts []MailAttachment) {
	byID := map[int64]MailAttachment{}
	for _, a := range atts {
		if p.pl.auc.claimed[a.MailID] {
			a.Claimed = true // a leitura é de antes da retirada
		}
		byID[a.MailID] = a
		p.pl.auc.mail[a.MailID] = a
	}
	for _, m := range box.Mails {
		if a, ok := byID[int64(m.Id)]; ok {
			m.Attachment = &pb.MailAttachment{ItemId: a.Item, Count: uint32(max(a.Count, 0)), Enchant: uint32(max(a.Enchant, 0)),
				Gold: a.Gold, Claimed: a.Claimed}
		}
	}
}

// auctionMailCounts manda ao jogador os contadores do correio (carta nova da casa).
func (z *Zone) auctionMailCounts(p *entity) {
	id := p.id
	z.queueSocial(func(ctx context.Context, b SocialBackend) func() {
		var unread, total int
		err := z.dbTry(ctx, "mail summary", true, func(ctx context.Context) error {
			var err error
			unread, total, err = b.MailSummary(ctx, id)
			return err
		})
		return func() {
			if err == nil && z.players[id] == p {
				z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Social{Social: &pb.SocialUpdate{Mail: mailCountsPB(unread, total)}}})
			}
		}
	})
}

// ---------- Salvamento e confirmação ----------

// auctionSave: as operações sem confirmação vão em todo salvamento do personagem.
func (z *Zone) auctionSave(p *entity) []AuctionOp {
	if len(p.pl.auc.ops) == 0 {
		return nil
	}
	return slices.Clone(p.pl.auc.ops)
}

// auctionOnJoin: entrou de novo com operações ainda sem confirmação (do estado recente).
func (z *Zone) auctionOnJoin(p *entity, ops []AuctionOp) {
	a := &p.pl.auc
	a.mail, a.claimed, a.seen = map[int64]MailAttachment{}, map[int64]bool{}, map[int64]AuctionListing{}
	a.ops = slices.Clone(ops)
	for _, op := range ops {
		if op.Kind == AuctionOpClaim {
			a.claimed[op.MailID] = true
		}
	}
}

// keepRecent: o estado recente com operação sem confirmação não pode ser esquecido (o
// próximo login leria o banco sem ela e a aplicaria de novo sobre um estado velho).
func keepRecent(r recentState) bool { return len(r.state.Auction) > 0 }

// AuctionApplied: a persistência gravou as operações (cmd/zone, depois da transação).
func (z *Zone) AuctionApplied(results []AuctionResult) {
	if len(results) == 0 {
		return
	}
	select {
	case z.events <- socialDone{fn: func() { z.onAuctionResults(results) }}:
	case <-z.stopped:
	}
}

func withoutOp(ops []AuctionOp, id int64) []AuctionOp {
	out := make([]AuctionOp, 0, len(ops))
	for _, op := range ops {
		if op.ID != id {
			out = append(out, op)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (z *Zone) onAuctionResults(results []AuctionResult) {
	for _, r := range results {
		p := z.players[r.CharacterID]
		if p != nil {
			p.pl.auc.ops = withoutOp(p.pl.auc.ops, r.OpID)
		} else if rs, ok := z.recent[r.CharacterID]; ok {
			rs.state.Auction = withoutOp(rs.state.Auction, r.OpID)
			z.recent[r.CharacterID] = rs
		}
		if r.Kind == AuctionOpBuy {
			if b, ok := z.auction.buying[r.AuctionID]; ok && b.char == r.CharacterID && b.op == r.OpID {
				delete(z.auction.buying, r.AuctionID)
			}
		}
		if !r.Fresh {
			continue // já tratado quando foi aplicada
		}
		switch r.Kind {
		case AuctionOpList:
			z.onListed(p, r)
		case AuctionOpBuy:
			z.onBought(p, r)
		case AuctionOpClaim:
			if !r.OK {
				z.log.Error("auction claim clash", "char", r.CharacterID, "mail", r.MailID)
				z.ReportDBError("auction_claim_clash", fmt.Errorf("anexo %d já retirado", r.MailID))
			}
		}
	}
}

func (z *Zone) onListed(p *entity, r AuctionResult) {
	if p == nil {
		return
	}
	label := auctionLabel(r.Item, r.Count, r.Enchant)
	if r.OK {
		p.pl.auc.active++
		z.auctionNotice(p, fmt.Sprintf("Anunciado: %s por %d moedas.", label, r.Price))
	} else {
		p.pl.auc.mail[r.MailID] = MailAttachment{MailID: r.MailID, Item: r.Item, Count: r.Count, Enchant: r.Enchant, Gold: r.Gold, How: HowAuctionReturn}
		z.auctionNotice(p, fmt.Sprintf("Sua conta já tem %d anúncios ativos: %s e a taxa voltaram pelo correio.", AuctionMaxListings, label))
		z.auctionMailCounts(p)
	}
	z.auctionMine(p, true)
}

func (z *Zone) onBought(p *entity, r AuctionResult) {
	label := auctionLabel(r.Item, r.Count, r.Enchant)
	if r.OK {
		if s := z.players[r.SellerID]; s != nil && r.SellerID != 0 {
			delete(s.pl.auc.seen, r.AuctionID)
			s.pl.auc.active = max(0, s.pl.auc.active-1)
			z.auctionNotice(s, fmt.Sprintf("Vendido: %s por %d moedas. Você recebe %d (comissão de %d) pelo correio.",
				label, r.Price, r.Gold, r.Price-r.Gold))
			z.auctionMailCounts(s)
			z.auctionMine(s, true)
		}
	}
	if p == nil {
		return // o comprador saiu: o item (ou o reembolso) espera no correio
	}
	if !r.OK {
		why := "foi vendido para outra pessoa, venceu ou foi cancelado"
		switch r.Reason {
		case AuctionPrice:
			why = "mudou de preço"
		case AuctionOwn:
			why = "é da sua conta"
		}
		a := MailAttachment{MailID: r.MailID, Gold: r.Gold, How: HowAuctionReturn}
		p.pl.auc.mail[r.MailID] = a
		z.auctionNotice(p, fmt.Sprintf("Compra não concluída: o anúncio %s. Suas %d moedas voltaram.", why, r.Gold))
		if !z.auctionTake(p, a, true) {
			z.auctionMailCounts(p) // ficou no correio (ex.: muitas operações a caminho)
		}
		return
	}
	a := MailAttachment{MailID: r.MailID, Item: r.Item, Count: r.Count, Enchant: r.Enchant, How: HowAuctionBuy}
	p.pl.auc.mail[r.MailID] = a
	z.sendAuction(p, &pb.AuctionUpdate{GoneId: uint64(r.AuctionID), Notice: fmt.Sprintf("Você comprou %s por %d moedas.", label, r.Price)})
	if z.auctionTake(p, a, true) {
		z.system(p, fmt.Sprintf("Você comprou %s por %d moedas.", label, r.Price))
	} else {
		z.system(p, fmt.Sprintf("Você comprou %s por %d moedas; ele espera no correio (tecla O).", label, r.Price))
		z.auctionMailCounts(p)
	}
}

// ---------- Invariantes ----------

// checkAuction: as operações sem confirmação fazem sentido (não passam do limite, valores
// positivos e ids únicos).
func (z *Zone) checkAuction(p *entity) {
	ops := p.pl.auc.ops
	bad := ""
	seen := map[int64]bool{}
	for _, op := range ops {
		switch {
		case seen[op.ID]:
			bad = fmt.Sprintf("operação %d repetida", op.ID)
		case op.Kind == AuctionOpList && (op.Count <= 0 || op.Price < AuctionMinPrice || op.Fee <= 0):
			bad = fmt.Sprintf("anúncio %s×%d por %d (taxa %d)", op.Item, op.Count, op.Price, op.Fee)
		case op.Kind == AuctionOpBuy && (op.Price <= 0 || op.AuctionID <= 0):
			bad = fmt.Sprintf("compra do anúncio %d por %d", op.AuctionID, op.Price)
		case op.Kind == AuctionOpClaim && (op.MailID <= 0 || op.Gold < 0):
			bad = fmt.Sprintf("retirada da carta %d", op.MailID)
		}
		seen[op.ID] = true
	}
	if len(ops) > auctionMaxOps+1 {
		bad = fmt.Sprintf("%d operações sem confirmação", len(ops))
	}
	if bad != "" {
		z.invariantBroken(p, "auction", bad, nil)
	}
}

// mergeOps: as operações de a mais as de b que a não tem (pelo id).
func mergeOps(a, b []AuctionOp) []AuctionOp {
	out := a
	for _, op := range b {
		if !slices.ContainsFunc(a, func(x AuctionOp) bool { return x.ID == op.ID }) {
			if len(out) == len(a) {
				out = slices.Clone(a)
			}
			out = append(out, op)
		}
	}
	return out
}

// auctionLoadMail lê os anexos do correio (ao abrir a janela): "Retirar tudo" funciona sem
// abrir a caixa de entrada antes.
func (z *Zone) auctionLoadMail(p *entity) {
	id := p.id
	z.queueAuction(func(ctx context.Context, b AuctionBackend) func() {
		atts, err := z.auctionAttachments(ctx, id)
		return func() {
			if err != nil || z.players[id] != p {
				return
			}
			z.auctionInbox(p, &pb.MailBox{}, atts)
			z.sendAuction(p, &pb.AuctionUpdate{})
		}
	})
}

package store

// Casa de leilões (tunnel#78). As operações que mexem na bolsa de um personagem online
// (anunciar, comprar, retirar do correio) chegam junto do estado dele em SaveStatesAuction
// e rodam na MESMA transação: a bolsa nova e o anúncio (ou a compra, ou a retirada) ficam
// gravados juntos ou nenhum dos dois. Cada operação vem em todo salvamento até a zona
// saber que deu certo; auction_ops garante que ela é aplicada uma vez só (repetida, devolve
// o resultado guardado). Cancelar e vencer só mexem aqui (anúncio → carta com o item).
//
// Dois compradores no mesmo anúncio, ou comprar no instante em que ele vence: a compra
// trava a linha do anúncio (FOR UPDATE) e só vende se ele ainda está ativo e não tinha
// vencido na hora do clique; quem perde recebe as moedas de volta numa carta. O vencimento
// usa FOR UPDATE SKIP LOCKED e a mesma condição, então os dois nunca levam o mesmo item.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
)

// ErrAuctionGone: o anúncio não está mais ativo (cancelar).
var ErrAuctionGone = errors.New("auction is not active")

// AuctionSender: quem assina as cartas da casa.
const AuctionSender = "Casa de Leilões"

// auctionOpsKeep: por quanto tempo o banco lembra uma operação aplicada (a zona confirma
// em segundos; o estado recente com operação pendente nunca é esquecido).
const auctionOpsKeep = 24 * time.Hour

// AuctionOp é uma operação da zona (ver zone.AuctionOp).
type AuctionOp struct {
	ID             int64
	Kind           string // list, buy, claim
	At             time.Time
	Name           string
	Item           string
	Count, Enchant int
	Price, Fee     int64
	Hours          int
	MaxListings    int
	AuctionID      int64
	Commission     int64
	MailID         int64
	Gold           int64
	Events         []GameEvent
}

// AuctionResult é o que o banco fez com a operação (guardado em auction_ops.result).
type AuctionResult struct {
	CharacterID int64  `json:"char"`
	OpID        int64  `json:"op"`
	Kind        string `json:"kind"`
	Fresh       bool   `json:"-"`
	OK          bool   `json:"ok"`
	Reason      string `json:"reason,omitempty"`
	AuctionID   int64  `json:"auction,omitempty"`
	MailID      int64  `json:"mail,omitempty"`
	SellerID    int64  `json:"seller,omitempty"`
	SellerName  string `json:"seller_name,omitempty"`
	Item        string `json:"item,omitempty"`
	Count       int    `json:"count,omitempty"`
	Enchant     int    `json:"enchant,omitempty"`
	Price       int64  `json:"price,omitempty"`
	Gold        int64  `json:"gold,omitempty"`
}

// Auction é um anúncio.
type Auction struct {
	ID            int64
	SellerID      int64
	SellerAccount int64
	SellerName    string
	Item          string
	Count         int
	Enchant       int
	Price         int64
	CreatedAt     time.Time
	ExpiresAt     time.Time
	Status        string
	BuyerName     string
}

// AuctionClosed: anúncio devolvido ao vendedor, com a carta do item.
type AuctionClosed struct {
	Auction Auction
	MailID  int64
}

// AuctionSearch: busca já traduzida pela zona.
type AuctionSearch struct {
	ItemIDs            []string // nil = qualquer
	MinPrice, MaxPrice int64    // 0 = sem limite
	Sort               int      // 0 preço ↑, 1 preço ↓, 2 mais novos, 3 vencem antes
	Offset, Limit      int
	Now                time.Time
}

// MailAttachment é o anexo de uma carta.
type MailAttachment struct {
	MailID         int64
	Item           string
	Count, Enchant int
	Gold           int64
	How            string
	Claimed        bool
}

const auctionCols = `id, seller_id, seller_account, seller_name, item_id, count, enchant, price, created_at, expires_at, status, buyer_name`

func scanAuction(row pgx.CollectableRow) (Auction, error) {
	var a Auction
	var enchant int16
	err := row.Scan(&a.ID, &a.SellerID, &a.SellerAccount, &a.SellerName, &a.Item, &a.Count, &enchant, &a.Price,
		&a.CreatedAt, &a.ExpiresAt, &a.Status, &a.BuyerName)
	a.Enchant = int(enchant)
	return a, err
}

func itemName(id string, count, enchant int) string {
	d := game.Items[id]
	if d == nil {
		return id
	}
	if enchant > 0 {
		return game.EnchantedName(d, enchant)
	}
	if count > 1 {
		return fmt.Sprintf("%d× %s", count, d.Name)
	}
	return d.Name
}

// systemMail entrega uma carta da casa com anexo (o limite da caixa não vale: o que é
// do jogador nunca é recusado).
func systemMail(ctx context.Context, tx pgx.Tx, to int64, at time.Time, subject, body string, a MailAttachment, auctionID int64) (int64, error) {
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO mail (to_id, from_id, from_name, subject, body, sent_at)
		VALUES ($1, NULL, $2, $3, $4, $5) RETURNING id`, to, AuctionSender, subject, body, at).Scan(&id); err != nil {
		return 0, err
	}
	_, err := tx.Exec(ctx, `INSERT INTO mail_attachments (mail_id, to_id, item_id, count, enchant, gold, how, auction_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, id, to, a.Item, a.Count, a.Enchant, a.Gold, a.How, auctionID)
	return id, err
}

// applyAuctionOps roda as operações que vieram com os estados (na transação do salvamento).
func applyAuctionOps(ctx context.Context, tx pgx.Tx, states []CharacterState) ([]AuctionResult, error) {
	type key struct{ char, op int64 }
	done := map[key]bool{}
	var out []AuctionResult
	for _, c := range states {
		for _, op := range c.Auction {
			k := key{c.CharacterID, op.ID}
			if done[k] {
				continue
			}
			done[k] = true
			r, err := applyAuctionOp(ctx, tx, c.CharacterID, op)
			if err != nil {
				return nil, fmt.Errorf("auction op %s %d: %w", op.Kind, op.ID, err)
			}
			if r != nil {
				out = append(out, *r)
			}
		}
	}
	return out, nil
}

func applyAuctionOp(ctx context.Context, tx pgx.Tx, char int64, op AuctionOp) (*AuctionResult, error) {
	var account int64
	var name string
	err := tx.QueryRow(ctx, `SELECT account_id, name FROM characters WHERE id = $1`, char).Scan(&account, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // personagem apagado: tudo dele já foi junto
	}
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO auction_ops (character_id, op_id, kind, result) VALUES ($1, $2, $3, '{}')
		ON CONFLICT DO NOTHING`, char, op.ID, op.Kind)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 { // já aplicada num salvamento anterior
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT result FROM auction_ops WHERE character_id = $1 AND op_id = $2`, char, op.ID).Scan(&raw); err != nil {
			return nil, err
		}
		var r AuctionResult
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, err
		}
		r.CharacterID, r.OpID, r.Kind, r.Fresh = char, op.ID, op.Kind, false
		return &r, nil
	}
	r := AuctionResult{CharacterID: char, OpID: op.ID, Kind: op.Kind, Fresh: true,
		Item: op.Item, Count: op.Count, Enchant: op.Enchant, Price: op.Price}
	switch op.Kind {
	case "list":
		err = auctionList(ctx, tx, char, account, name, op, &r)
	case "buy":
		err = auctionBuy(ctx, tx, char, account, name, op, &r)
	case "claim":
		err = auctionClaim(ctx, tx, char, op, &r)
	default:
		r.Reason = "unknown"
	}
	if err != nil {
		return nil, err
	}
	if len(op.Events) > 0 {
		if err := insertEvents(ctx, tx, op.Events); err != nil {
			return nil, err
		}
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE auction_ops SET result = $3 WHERE character_id = $1 AND op_id = $2`, char, op.ID, raw); err != nil {
		return nil, err
	}
	return &r, nil
}

// auctionList: o item (e a taxa) já saíram da bolsa; vira anúncio ou, com a conta no
// limite (outro personagem dela anunciou ao mesmo tempo), volta pelo correio com a taxa.
func auctionList(ctx context.Context, tx pgx.Tx, char, account int64, name string, op AuctionOp, r *AuctionResult) error {
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM auctions WHERE seller_account = $1 AND status = 'active'`, account).
		Scan(&active); err != nil {
		return err
	}
	label := itemName(op.Item, op.Count, op.Enchant)
	if active >= op.MaxListings || op.Count <= 0 || op.Price <= 0 {
		r.Reason, r.Gold = "limit", op.Fee
		id, err := systemMail(ctx, tx, char, op.At, "Anúncio recusado",
			fmt.Sprintf("Sua conta já tem %d anúncios ativos. %s e a taxa voltaram com esta carta.", op.MaxListings, label),
			MailAttachment{Item: op.Item, Count: op.Count, Enchant: op.Enchant, Gold: op.Fee, How: "auction_return"}, 0)
		r.MailID = id
		return err
	}
	r.OK = true
	return tx.QueryRow(ctx, `INSERT INTO auctions (seller_id, seller_account, seller_name, item_id, count, enchant, price, fee, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		char, account, name, op.Item, op.Count, op.Enchant, op.Price, op.Fee, op.At, op.At.Add(time.Duration(op.Hours)*time.Hour)).
		Scan(&r.AuctionID)
}

// auctionBuy: as moedas já saíram da bolsa do comprador. Vende se o anúncio ainda está
// ativo, não tinha vencido no clique e tem o preço que ele viu; senão devolve as moedas.
func auctionBuy(ctx context.Context, tx pgx.Tx, char, account int64, name string, op AuctionOp, r *AuctionResult) error {
	r.AuctionID = op.AuctionID
	rows, err := tx.Query(ctx, `SELECT `+auctionCols+` FROM auctions WHERE id = $1 FOR UPDATE`, op.AuctionID)
	if err != nil {
		return err
	}
	a, err := pgx.CollectExactlyOneRow(rows, scanAuction)
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	switch {
	case !found || a.Status != "active" || !a.ExpiresAt.After(op.At):
		r.Reason = "gone"
	case a.Price != op.Price:
		r.Reason = "price"
	case a.SellerAccount == account:
		r.Reason = "own"
	}
	if found {
		r.Item, r.Count, r.Enchant = a.Item, a.Count, a.Enchant
	}
	if r.Reason != "" {
		r.Gold = op.Price
		id, err := systemMail(ctx, tx, char, op.At, "Compra não concluída",
			fmt.Sprintf("O anúncio de %s não estava mais disponível. Suas %d moedas voltaram com esta carta.", itemName(r.Item, r.Count, r.Enchant), op.Price),
			MailAttachment{Gold: op.Price, How: "auction_return"}, op.AuctionID)
		r.MailID = id
		return err
	}
	commission := min(max(op.Commission, 0), a.Price-1)
	if _, err := tx.Exec(ctx, `UPDATE auctions SET status = 'sold', buyer_id = $2, buyer_name = $3, commission = $4, closed_at = $5 WHERE id = $1`,
		a.ID, char, name, commission, op.At); err != nil {
		return err
	}
	label := itemName(a.Item, a.Count, a.Enchant)
	r.OK, r.SellerID, r.SellerName, r.Gold = true, a.SellerID, a.SellerName, a.Price-commission
	if _, err := systemMail(ctx, tx, a.SellerID, op.At, "Vendido: "+label,
		fmt.Sprintf("%s comprou %s por %d moedas. A comissão foi de %d: você recebe %d.", name, label, a.Price, commission, r.Gold),
		MailAttachment{Gold: r.Gold, How: "auction_sale"}, a.ID); err != nil {
		return err
	}
	r.MailID, err = systemMail(ctx, tx, char, op.At, "Compra: "+label,
		fmt.Sprintf("Você comprou %s de %s por %d moedas.", label, a.SellerName, a.Price),
		MailAttachment{Item: a.Item, Count: a.Count, Enchant: a.Enchant, How: "auction_buy"}, a.ID)
	return err
}

// auctionClaim: a zona já pôs o anexo na bolsa; marca como retirado (e confere que era o
// mesmo conteúdo). Não achar o anexo aberto não devia acontecer (só o dono retira e a zona
// lembra o que ele retirou); vira "clash" e a zona registra o erro.
func auctionClaim(ctx context.Context, tx pgx.Tx, char int64, op AuctionOp, r *AuctionResult) error {
	r.MailID = op.MailID
	var item string
	var count int
	var enchant int16
	var gold int64
	err := tx.QueryRow(ctx, `UPDATE mail_attachments SET claimed_at = $3 WHERE mail_id = $1 AND to_id = $2 AND claimed_at IS NULL
		RETURNING item_id, count, enchant, gold`, op.MailID, char, op.At).Scan(&item, &count, &enchant, &gold)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		r.Reason = "clash"
		return nil
	case err != nil:
		return err
	}
	r.Gold = gold
	if item != op.Item || count != op.Count || int(enchant) != op.Enchant || gold != op.Gold {
		r.Reason = "clash"
		return nil
	}
	r.OK = true
	return nil
}

// SearchAuctions: anúncios ativos (e não vencidos) que batem com a busca.
func (s *Store) SearchAuctions(ctx context.Context, q AuctionSearch) ([]Auction, int, error) {
	maxPrice := q.MaxPrice
	if maxPrice <= 0 {
		maxPrice = 1 << 62
	}
	where := `status = 'active' AND expires_at > $1 AND ($2::text[] IS NULL OR item_id = ANY($2)) AND price >= $3 AND price <= $4`
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM auctions WHERE `+where, q.Now, q.ItemIDs, q.MinPrice, maxPrice).
		Scan(&total); err != nil {
		return nil, 0, err
	}
	order := map[int]string{0: "price, id", 1: "price DESC, id DESC", 2: "created_at DESC, id DESC", 3: "expires_at, id"}[q.Sort]
	if order == "" {
		order = "price, id"
	}
	rows, err := s.pool.Query(ctx, `SELECT `+auctionCols+` FROM auctions WHERE `+where+` ORDER BY `+order+` LIMIT $5 OFFSET $6`,
		q.Now, q.ItemIDs, q.MinPrice, maxPrice, max(q.Limit, 1), max(q.Offset, 0))
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, scanAuction)
	return list, total, err
}

// MyAuctions: os anúncios do personagem (ativos primeiro, depois os últimos encerrados) e
// quantos a conta tem ativos.
func (s *Store) MyAuctions(ctx context.Context, characterID, accountID int64, limit int) ([]Auction, int, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+auctionCols+` FROM auctions WHERE seller_id = $1
		ORDER BY status <> 'active', coalesce(closed_at, created_at) DESC, id DESC LIMIT $2`, characterID, limit)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, scanAuction)
	if err != nil {
		return nil, 0, err
	}
	var active int
	err = s.pool.QueryRow(ctx, `SELECT count(*) FROM auctions WHERE seller_account = $1 AND status = 'active'`, accountID).Scan(&active)
	return list, active, err
}

// closeAuction devolve o item do anúncio (já marcado) ao vendedor pelo correio.
func closeAuction(ctx context.Context, tx pgx.Tx, a Auction, at time.Time, subject, body string) (AuctionClosed, error) {
	id, err := systemMail(ctx, tx, a.SellerID, at, subject, body,
		MailAttachment{Item: a.Item, Count: a.Count, Enchant: a.Enchant, How: "auction_return"}, a.ID)
	return AuctionClosed{Auction: a, MailID: id}, err
}

// CancelAuction cancela um anúncio ativo do vendedor; o item volta pelo correio (a taxa
// não). ErrAuctionGone se ele já foi vendido, cancelado ou devolvido.
func (s *Store) CancelAuction(ctx context.Context, sellerID, auctionID int64, at time.Time) (AuctionClosed, error) {
	var out AuctionClosed
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE auctions SET status = 'cancelled', closed_at = $3
			WHERE id = $1 AND seller_id = $2 AND status = 'active' RETURNING `+auctionCols, auctionID, sellerID, at)
		if err != nil {
			return err
		}
		a, err := pgx.CollectExactlyOneRow(rows, scanAuction)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAuctionGone
		}
		if err != nil {
			return err
		}
		label := itemName(a.Item, a.Count, a.Enchant)
		out, err = closeAuction(ctx, tx, a, at, "Anúncio cancelado: "+label,
			fmt.Sprintf("Você cancelou o anúncio de %s. O item voltou com esta carta (a taxa não volta).", label))
		return err
	})
	return out, err
}

// ExpireAuctions devolve até limit anúncios vencidos e esquece as operações velhas.
func (s *Store) ExpireAuctions(ctx context.Context, now time.Time, limit int) ([]AuctionClosed, error) {
	var out []AuctionClosed
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE auctions SET status = 'expired', closed_at = $1 WHERE id IN (
				SELECT id FROM auctions WHERE status = 'active' AND expires_at <= $1
				ORDER BY expires_at LIMIT $2 FOR UPDATE SKIP LOCKED)
			RETURNING `+auctionCols, now, limit)
		if err != nil {
			return err
		}
		list, err := pgx.CollectRows(rows, scanAuction)
		if err != nil {
			return err
		}
		for _, a := range list {
			label := itemName(a.Item, a.Count, a.Enchant)
			c, err := closeAuction(ctx, tx, a, now, "Anúncio vencido: "+label,
				fmt.Sprintf("Ninguém comprou %s no prazo. O item voltou com esta carta.", label))
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		_, err = tx.Exec(ctx, `DELETE FROM auction_ops WHERE at < $1`, now.Add(-auctionOpsKeep))
		return err
	})
	return out, err
}

// MailAttachments: os anexos das cartas guardadas do personagem.
func (s *Store) MailAttachments(ctx context.Context, characterID int64) ([]MailAttachment, error) {
	rows, err := s.pool.Query(ctx, `SELECT a.mail_id, a.item_id, a.count, a.enchant, a.gold, a.how, a.claimed_at IS NOT NULL
		FROM mail_attachments a JOIN mail m ON m.id = a.mail_id
		WHERE a.to_id = $1 AND m.deleted_at IS NULL ORDER BY a.mail_id`, characterID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (MailAttachment, error) {
		var a MailAttachment
		var enchant int16
		err := row.Scan(&a.MailID, &a.Item, &a.Count, &enchant, &a.Gold, &a.How, &a.Claimed)
		a.Enchant = int(enchant)
		return a, err
	})
}

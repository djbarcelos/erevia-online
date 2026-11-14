package main

import (
	"context"
	"errors"
	"time"

	"erevia/server/internal/store"
	"erevia/server/internal/zone"
)

// Casa de leilões: o worker da zona (busca, meus anúncios, cancelar, vencer, anexos) e as
// operações que vão no salvamento (saveBatch → store.SaveStatesAuction).

type auctionStore struct{ st *store.Store }

func auctionListing(a store.Auction) zone.AuctionListing {
	return zone.AuctionListing{
		ID: a.ID, SellerID: uint32(a.SellerID), SellerAccount: a.SellerAccount, SellerName: a.SellerName,
		Item: a.Item, Count: a.Count, Enchant: a.Enchant, Price: a.Price,
		CreatedAt: a.CreatedAt, ExpiresAt: a.ExpiresAt, Status: a.Status, BuyerName: a.BuyerName,
	}
}

func (s auctionStore) SearchAuctions(ctx context.Context, q zone.AuctionSearch) (zone.AuctionPage, error) {
	list, total, err := s.st.SearchAuctions(ctx, store.AuctionSearch{
		ItemIDs: q.ItemIDs, MinPrice: q.MinPrice, MaxPrice: q.MaxPrice, Sort: int(q.Sort),
		Offset: q.Offset, Limit: q.Limit, Now: q.Now,
	})
	out := zone.AuctionPage{Total: total}
	for _, a := range list {
		out.Listings = append(out.Listings, auctionListing(a))
	}
	return out, err
}

func (s auctionStore) MyAuctions(ctx context.Context, id uint32, account int64, limit int) ([]zone.AuctionListing, int, error) {
	list, active, err := s.st.MyAuctions(ctx, int64(id), account, limit)
	out := make([]zone.AuctionListing, len(list))
	for i, a := range list {
		out[i] = auctionListing(a)
	}
	return out, active, err
}

func auctionClosed(c store.AuctionClosed) zone.AuctionClosed {
	return zone.AuctionClosed{Listing: auctionListing(c.Auction), MailID: c.MailID}
}

func (s auctionStore) CancelAuction(ctx context.Context, seller uint32, id int64, at time.Time) (zone.AuctionClosed, error) {
	c, err := s.st.CancelAuction(ctx, int64(seller), id, at)
	if errors.Is(err, store.ErrAuctionGone) {
		return zone.AuctionClosed{}, zone.ErrAuctionGone
	}
	return auctionClosed(c), err
}

func (s auctionStore) ExpireAuctions(ctx context.Context, now time.Time, limit int) ([]zone.AuctionClosed, error) {
	list, err := s.st.ExpireAuctions(ctx, now, limit)
	out := make([]zone.AuctionClosed, len(list))
	for i, c := range list {
		out[i] = auctionClosed(c)
	}
	return out, err
}

func (s auctionStore) MailAttachments(ctx context.Context, id uint32) ([]zone.MailAttachment, error) {
	list, err := s.st.MailAttachments(ctx, int64(id))
	out := make([]zone.MailAttachment, len(list))
	for i, a := range list {
		out[i] = zone.MailAttachment(a)
	}
	return out, err
}

// auctionOps converte as operações de um estado salvo.
func auctionOps(ops []zone.AuctionOp) []store.AuctionOp {
	if len(ops) == 0 {
		return nil
	}
	out := make([]store.AuctionOp, len(ops))
	for i, op := range ops {
		events := make([]store.GameEvent, len(op.Events))
		for k, e := range op.Events {
			events[k] = store.GameEvent(e)
		}
		out[i] = store.AuctionOp{
			ID: op.ID, Kind: op.Kind, At: op.At, Name: op.Name, Item: op.Item, Count: op.Count, Enchant: op.Enchant,
			Price: op.Price, Fee: op.Fee, Hours: op.Hours, MaxListings: op.MaxListings, AuctionID: op.AuctionID,
			Commission: op.Commission, MailID: op.MailID, Gold: op.Gold, Events: events,
		}
	}
	return out
}

// auctionResults converte o que o banco fez com as operações.
func auctionResults(list []store.AuctionResult) []zone.AuctionResult {
	out := make([]zone.AuctionResult, len(list))
	for i, r := range list {
		out[i] = zone.AuctionResult{
			CharacterID: uint32(r.CharacterID), OpID: r.OpID, Kind: r.Kind, Fresh: r.Fresh, OK: r.OK, Reason: r.Reason,
			AuctionID: r.AuctionID, MailID: r.MailID, SellerID: uint32(r.SellerID), SellerName: r.SellerName,
			Item: r.Item, Count: r.Count, Enchant: r.Enchant, Price: r.Price, Gold: r.Gold,
		}
	}
	return out
}

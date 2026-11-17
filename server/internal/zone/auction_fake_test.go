package zone

import (
	"context"
	"slices"
	"sort"
	"time"
)

// fakeAuction é o banco da casa de leilões em memória, com as mesmas regras do store
// (store/auction.go): anúncios em custódia, cartas com anexo (no correio do fakeSocial) e
// as operações que chegam com o salvamento, aplicadas uma vez só.
type fakeAuction struct {
	social   *fakeSocial
	accounts map[uint32]int64 // personagem → conta (os que "existem" no banco)
	names    map[uint32]string
	auctions map[int64]*AuctionListing
	seq      int64
	atts     map[int64]*MailAttachment // carta → anexo
	attTo    map[int64]uint32          // carta → dono
	applied  map[[2]int64]AuctionResult
	comm     map[int64]int64 // comissão das vendas
	down     bool            // banco fora: o salvamento falha inteiro
	batches  int             // lotes gravados
	events   []GameEvent     // telemetria gravada com as operações
}

func newFakeAuction(social *fakeSocial) *fakeAuction {
	f := &fakeAuction{
		social: social, accounts: map[uint32]int64{}, names: map[uint32]string{},
		auctions: map[int64]*AuctionListing{}, atts: map[int64]*MailAttachment{}, attTo: map[int64]uint32{},
		applied: map[[2]int64]AuctionResult{}, comm: map[int64]int64{},
	}
	social.locked = func(mail int64) bool {
		a := f.atts[mail]
		return a != nil && !a.Claimed
	}
	return f
}

func (f *fakeAuction) char(id uint32, account int64, name string) {
	f.accounts[id], f.names[id] = account, name
}

// mail entrega uma carta da casa com anexo (no correio falso).
func (f *fakeAuction) mail(to uint32, at time.Time, subject string, a MailAttachment) int64 {
	f.social.seq++
	id := f.social.seq
	f.social.mails = append(f.social.mails, fakeMail{MailData: MailData{ID: id, FromName: AuctionSender, Subject: subject, Body: subject, SentAt: at}, to: to})
	a.MailID = id
	f.atts[id], f.attTo[id] = &a, to
	return id
}

// apply faz o papel de store.SaveStatesAuction para um lote (só a parte do leilão).
func (f *fakeAuction) apply(batch []SavedState) ([]AuctionResult, bool) {
	if f.down {
		return nil, false
	}
	f.batches++
	var out []AuctionResult
	for _, s := range batch {
		for _, op := range s.Auction {
			k := [2]int64{int64(s.ID), op.ID}
			acc, exists := f.accounts[s.ID]
			if !exists {
				continue
			}
			if r, ok := f.applied[k]; ok {
				r.Fresh = false
				out = append(out, r)
				continue
			}
			r := AuctionResult{CharacterID: s.ID, OpID: op.ID, Kind: op.Kind, Fresh: true, Item: op.Item, Count: op.Count, Enchant: op.Enchant, Price: op.Price}
			switch op.Kind {
			case AuctionOpList:
				active := 0
				for _, a := range f.auctions {
					if a.SellerAccount == acc && a.Status == AuctionActive {
						active++
					}
				}
				if active >= op.MaxListings {
					r.Reason, r.Gold = AuctionLimit, op.Fee
					r.MailID = f.mail(s.ID, op.At, "Anúncio recusado", MailAttachment{Item: op.Item, Count: op.Count, Enchant: op.Enchant, Gold: op.Fee, How: HowAuctionReturn})
					break
				}
				f.seq++
				f.auctions[f.seq] = &AuctionListing{ID: f.seq, SellerID: s.ID, SellerAccount: acc, SellerName: f.names[s.ID],
					Item: op.Item, Count: op.Count, Enchant: op.Enchant, Price: op.Price, CreatedAt: op.At,
					ExpiresAt: op.At.Add(time.Duration(op.Hours) * time.Hour), Status: AuctionActive}
				r.OK, r.AuctionID = true, f.seq
			case AuctionOpBuy:
				r.AuctionID = op.AuctionID
				a := f.auctions[op.AuctionID]
				switch {
				case a == nil || a.Status != AuctionActive || !a.ExpiresAt.After(op.At):
					r.Reason = AuctionGone
				case a.Price != op.Price:
					r.Reason = AuctionPrice
				case a.SellerAccount == acc:
					r.Reason = AuctionOwn
				}
				if a != nil {
					r.Item, r.Count, r.Enchant = a.Item, a.Count, a.Enchant
				}
				if r.Reason != "" {
					r.Gold = op.Price
					r.MailID = f.mail(s.ID, op.At, "Compra não concluída", MailAttachment{Gold: op.Price, How: HowAuctionReturn})
					break
				}
				c := min(max(op.Commission, 0), a.Price-1)
				a.Status, a.BuyerName = AuctionSold, f.names[s.ID]
				f.comm[a.ID] = c
				r.OK, r.SellerID, r.SellerName, r.Gold = true, a.SellerID, a.SellerName, a.Price-c
				f.mail(a.SellerID, op.At, "Vendido", MailAttachment{Gold: r.Gold, How: HowAuctionSale})
				r.MailID = f.mail(s.ID, op.At, "Compra", MailAttachment{Item: a.Item, Count: a.Count, Enchant: a.Enchant, How: HowAuctionBuy})
			case AuctionOpClaim:
				r.MailID = op.MailID
				a := f.atts[op.MailID]
				if a == nil || a.Claimed || f.attTo[op.MailID] != s.ID {
					r.Reason = AuctionClash
					break
				}
				a.Claimed = true
				r.Gold = a.Gold
				if a.Item != op.Item || a.Count != op.Count || a.Enchant != op.Enchant || a.Gold != op.Gold {
					r.Reason = AuctionClash
					break
				}
				r.OK = true
			}
			f.applied[k] = r
			f.events = append(f.events, op.Events...)
			out = append(out, r)
		}
	}
	return out, true
}

func (f *fakeAuction) SearchAuctions(_ context.Context, q AuctionSearch) (AuctionPage, error) {
	var list []AuctionListing
	for _, a := range f.auctions {
		if a.Status != AuctionActive || !a.ExpiresAt.After(q.Now) || (q.ItemIDs != nil && !slices.Contains(q.ItemIDs, a.Item)) ||
			a.Price < q.MinPrice || (q.MaxPrice > 0 && a.Price > q.MaxPrice) {
			continue
		}
		list = append(list, *a)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Price != list[j].Price {
			return list[i].Price < list[j].Price
		}
		return list[i].ID < list[j].ID
	})
	out := AuctionPage{Total: len(list)}
	if q.Offset < len(list) {
		out.Listings = list[q.Offset:min(len(list), q.Offset+q.Limit)]
	}
	return out, nil
}

func (f *fakeAuction) MyAuctions(_ context.Context, id uint32, acc int64, limit int) ([]AuctionListing, int, error) {
	var out []AuctionListing
	active := 0
	for _, a := range f.auctions {
		if a.SellerID == id {
			out = append(out, *a)
		}
		if a.SellerAccount == acc && a.Status == AuctionActive {
			active++
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out[:min(len(out), limit)], active, nil
}

func (f *fakeAuction) CancelAuction(_ context.Context, seller uint32, id int64, at time.Time) (AuctionClosed, error) {
	a := f.auctions[id]
	if a == nil || a.SellerID != seller || a.Status != AuctionActive {
		return AuctionClosed{}, ErrAuctionGone
	}
	a.Status = AuctionCancelled
	m := f.mail(seller, at, "Anúncio cancelado", MailAttachment{Item: a.Item, Count: a.Count, Enchant: a.Enchant, How: HowAuctionReturn})
	return AuctionClosed{Listing: *a, MailID: m}, nil
}

func (f *fakeAuction) ExpireAuctions(_ context.Context, now time.Time, limit int) ([]AuctionClosed, error) {
	var out []AuctionClosed
	ids := make([]int64, 0, len(f.auctions))
	for id := range f.auctions {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		a := f.auctions[id]
		if a.Status != AuctionActive || a.ExpiresAt.After(now) || len(out) >= limit {
			continue
		}
		a.Status = AuctionExpired
		m := f.mail(a.SellerID, now, "Anúncio vencido", MailAttachment{Item: a.Item, Count: a.Count, Enchant: a.Enchant, How: HowAuctionReturn})
		out = append(out, AuctionClosed{Listing: *a, MailID: m})
	}
	return out, nil
}

func (f *fakeAuction) MailAttachments(_ context.Context, id uint32) ([]MailAttachment, error) {
	var out []MailAttachment
	for _, m := range f.social.mails {
		if a := f.atts[m.ID]; a != nil && m.to == id && !m.deleted {
			out = append(out, *a)
		}
	}
	return out, nil
}

// custody: o que está nos anúncios ativos e nos anexos não retirados de quem está em ids.
func (f *fakeAuction) custody(ids []uint32) (gold int64, units map[string]int) {
	units = map[string]int{}
	for _, a := range f.auctions {
		if a.Status == AuctionActive && slices.Contains(ids, a.SellerID) {
			units[a.Item] += a.Count
		}
	}
	for mail, a := range f.atts {
		if a.Claimed || !slices.Contains(ids, f.attTo[mail]) {
			continue
		}
		gold += a.Gold
		if a.Item != "" {
			units[a.Item] += a.Count
		}
	}
	return gold, units
}

// pumpAuction faz o papel do worker do leilão e da persistência: roda os trabalhos na
// fila, grava os salvamentos (aplicando as operações) e entrega as confirmações à zona.
func (z *Zone) pumpAuction(f *fakeAuction, saves chan []SavedState) {
	for {
		select {
		case job := <-z.auction.jobs:
			if fn := job(context.Background(), z.auction.backend); fn != nil {
				z.handle(socialDone{fn: fn})
			}
		case job := <-z.socialJobs:
			if fn := job(context.Background(), z.socialBackend); fn != nil {
				z.handle(socialDone{fn: fn})
			}
		case b := <-saves:
			if res, ok := f.apply(b); ok {
				z.onAuctionResults(res)
			}
		case e := <-z.events:
			z.handle(e)
		default:
			return
		}
	}
}

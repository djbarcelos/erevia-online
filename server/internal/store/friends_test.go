package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestFriends(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "fr", "fr")
	acc2, _ := s.LoginIdentity(ctx, "dev", "fr2", "fr2")
	ana, _ := s.CreateCharacter(ctx, acc, "Ana", "knight", game.DefaultLook("knight"))
	beto, _ := s.CreateCharacter(ctx, acc2, "Beto", "mage", game.DefaultLook("mage"))
	caio, _ := s.CreateCharacter(ctx, acc2, "Caio", "rogue", game.DefaultLook("rogue"))

	// Pelo nome, sem diferenciar maiúsculas; repetir devolve o mesmo amigo.
	f, err := s.AddFriend(ctx, ana.ID, "bETO", 2)
	if err != nil || f.ID != beto.ID || f.Name != "Beto" || f.Class != "mage" {
		t.Fatalf("AddFriend = %+v, %v", f, err)
	}
	if f, err := s.AddFriend(ctx, ana.ID, "Beto", 2); err != nil || f.ID != beto.ID {
		t.Fatalf("repeated add = %+v, %v", f, err)
	}
	if _, err := s.AddFriend(ctx, ana.ID, "Ninguem", 2); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown name: %v", err)
	}
	if _, err := s.AddFriend(ctx, ana.ID, "ana", 2); !errors.Is(err, store.ErrSelf) {
		t.Fatalf("self: %v", err)
	}
	if _, err := s.AddFriend(ctx, ana.ID, "Caio", 2); err != nil {
		t.Fatal(err)
	}
	// Limite (2 neste teste).
	dora, _ := s.CreateCharacter(ctx, acc2, "Dora", "knight", game.DefaultLook("knight"))
	if _, err := s.AddFriend(ctx, ana.ID, "Dora", 2); !errors.Is(err, store.ErrFriendLimit) {
		t.Fatalf("limit: %v", err)
	}

	// Mão única: Beto não tem Ana na lista.
	list, err := s.Friends(ctx, ana.ID)
	if err != nil || len(list) != 2 || list[0].Name != "Beto" || list[1].Name != "Caio" {
		t.Fatalf("Friends(ana) = %+v, %v", list, err)
	}
	if list, _ := s.Friends(ctx, beto.ID); len(list) != 0 {
		t.Fatalf("Friends(beto) = %+v", list)
	}

	if err := s.RemoveFriend(ctx, ana.ID, caio.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFriend(ctx, ana.ID, "Dora", 2); err != nil {
		t.Fatalf("after remove: %v", err)
	}
	if list, _ := s.Friends(ctx, ana.ID); len(list) != 2 || list[1].ID != dora.ID {
		t.Fatalf("after remove = %+v", list)
	}
}

func TestMail(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "ml", "ml")
	acc2, _ := s.LoginIdentity(ctx, "dev", "ml2", "ml2")
	ana, _ := s.CreateCharacter(ctx, acc, "Ana", "knight", game.DefaultLook("knight"))
	beto, _ := s.CreateCharacter(ctx, acc2, "Beto", "mage", game.DefaultLook("mage"))
	at := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	out := func(to, body string, at time.Time) store.MailOut {
		return store.MailOut{FromID: ana.ID, FromName: "Ana", ToName: to, Subject: "Oi", Body: body, At: at, PerHour: 3, MaxBox: 4}
	}

	sent, err := s.SendMail(ctx, out("beto", "primeira", at))
	if err != nil || sent.ToID != beto.ID || sent.ToName != "Beto" || sent.Unread != 1 || sent.Total != 1 {
		t.Fatalf("SendMail = %+v, %v", sent, err)
	}
	if _, err := s.SendMail(ctx, out("Ninguem", "x", at)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := s.SendMail(ctx, out("Ana", "x", at)); !errors.Is(err, store.ErrSelf) {
		t.Fatalf("self: %v", err)
	}
	s.SendMail(ctx, out("Beto", "segunda", at.Add(time.Minute)))
	s.SendMail(ctx, out("Beto", "terceira", at.Add(2*time.Minute)))
	// 3 por hora neste teste: a quarta espera a primeira sair da janela.
	if _, err := s.SendMail(ctx, out("Beto", "quarta", at.Add(30*time.Minute))); !errors.Is(err, store.ErrMailRate) {
		t.Fatalf("rate: %v", err)
	}

	box, err := s.Inbox(ctx, beto.ID)
	if err != nil || len(box) != 3 || box[0].Body != "terceira" || box[2].Body != "primeira" || box[0].FromName != "Ana" || box[0].Subject != "Oi" {
		t.Fatalf("Inbox = %+v, %v", box, err)
	}
	if !box[2].SentAt.Equal(at) {
		t.Fatalf("sent_at = %v", box[2].SentAt)
	}

	// Ler e apagar: só o dono mexe na própria carta.
	if err := s.MarkMailRead(ctx, ana.ID, box[2].ID); err != nil {
		t.Fatal(err)
	}
	if u, n, _ := s.MailSummary(ctx, beto.ID); u != 3 || n != 3 {
		t.Fatalf("someone else's read counted: %d/%d", u, n)
	}
	s.MarkMailRead(ctx, beto.ID, box[2].ID)
	if u, n, _ := s.MailSummary(ctx, beto.ID); u != 2 || n != 3 {
		t.Fatalf("summary after read = %d/%d", u, n)
	}
	s.DeleteMail(ctx, beto.ID, box[0].ID, at.Add(40*time.Minute))
	if u, n, _ := s.MailSummary(ctx, beto.ID); u != 1 || n != 2 {
		t.Fatalf("summary after delete = %d/%d", u, n)
	}
	// Apagar não devolve o limite de envio da hora.
	if _, err := s.SendMail(ctx, out("Beto", "quarta", at.Add(45*time.Minute))); !errors.Is(err, store.ErrMailRate) {
		t.Fatalf("rate after delete: %v", err)
	}

	// Uma hora depois o remetente pode mandar de novo, até a caixa encher (4 neste teste).
	later := at.Add(2 * time.Hour)
	if _, err := s.SendMail(ctx, out("Beto", "quarta", later)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendMail(ctx, out("Beto", "quinta", later)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendMail(ctx, out("Beto", "sexta", later)); !errors.Is(err, store.ErrMailboxFull) {
		t.Fatalf("full: %v", err)
	}
	if u, n, _ := s.MailSummary(ctx, beto.ID); u != 3 || n != 4 {
		t.Fatalf("summary = %d/%d", u, n)
	}
}

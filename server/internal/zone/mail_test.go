package zone

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/pb"
)

func mailTo(z *Zone, from uint32, to, subject, body string) {
	z.handle(socialEvent{id: from, msg: &pb.MailCmd{Kind: pb.MailCmdKind_MAIL_CMD_SEND, To: to, Subject: subject, Body: body}})
}

func mailCmd(z *Zone, id uint32, kind pb.MailCmdKind, mailID int64) {
	z.handle(socialEvent{id: id, msg: &pb.MailCmd{Kind: kind, MailId: uint64(mailID)}})
	z.drainSocial()
}

func TestMailToOfflineRecipientAndLoginNotice(t *testing.T) {
	z, f := socialZone()
	ana := enter(z, 1, "Ana")

	// Beto está offline: a carta fica no banco.
	say(z, 1, "/correio beto Passa na taverna hoje à noite?")
	z.drainSocial()
	if got := ana.lastSystem(); got != "Carta enviada para Beto." {
		t.Fatalf("sender = %q", got)
	}
	if len(f.mails) != 1 || f.mails[0].to != 2 || f.mails[0].Body != "Passa na taverna hoje à noite?" || f.mails[0].FromName != "Ana" {
		t.Fatalf("db = %+v", f.mails)
	}

	// Ao entrar, Beto vê o aviso e o contador.
	beto := enter(z, 2, "Beto")
	if !slices.Contains(beto.systems(), "Você tem 1 carta não lida no correio (tecla O).") {
		t.Fatalf("login notice missing: %q", beto.systems())
	}
	if m := beto.lastMail(); m == nil || m.Unread != 1 || m.Total != 1 || m.Listed || m.Max != mailBoxMax {
		t.Fatalf("counters = %+v", m)
	}

	// Caixa de entrada, marcar como lida e apagar.
	mailCmd(z, 2, pb.MailCmdKind_MAIL_CMD_LIST, 0)
	box := beto.lastMail()
	if !box.Listed || len(box.Mails) != 1 || box.Mails[0].FromName != "Ana" || box.Mails[0].Read {
		t.Fatalf("inbox = %+v", box)
	}
	id := int64(box.Mails[0].Id)
	mailCmd(z, 2, pb.MailCmdKind_MAIL_CMD_READ, id)
	if box := beto.lastMail(); box.Unread != 0 || !box.Mails[0].Read {
		t.Fatalf("after read = %+v", box)
	}
	// Ana não apaga a carta de Beto.
	mailCmd(z, 1, pb.MailCmdKind_MAIL_CMD_DELETE, id)
	if _, total, _ := f.MailSummary(context.Background(), 2); total != 1 {
		t.Fatal("someone else deleted the letter")
	}
	mailCmd(z, 2, pb.MailCmdKind_MAIL_CMD_DELETE, id)
	if box := beto.lastMail(); len(box.Mails) != 0 || box.Total != 0 {
		t.Fatalf("after delete = %+v", box)
	}
}

func TestMailSenderOfflineWhenDelivered(t *testing.T) {
	z, f := socialZone()
	enter(z, 1, "Ana")
	beto := enter(z, 2, "Beto")

	// Ana manda e sai antes de o banco responder: a carta chega mesmo assim.
	mailTo(z, 1, "Beto", "Troca", "Tenho o escudo que você queria.")
	z.removePlayer(1)
	z.drainSocial()
	if len(f.mails) != 1 || f.mails[0].Subject != "Troca" {
		t.Fatalf("db = %+v", f.mails)
	}
	if got := beto.lastSystem(); got != "Você recebeu uma carta de Ana (tecla O)." {
		t.Fatalf("recipient = %q", got)
	}
	if m := beto.lastMail(); m.Unread != 1 || m.Listed {
		t.Fatalf("counters = %+v", m)
	}

	// Os dois offline na entrega: Beto vê ao voltar.
	enter(z, 1, "Ana")
	z.removePlayer(2)
	mailTo(z, 1, "Beto", "", "Outra carta.")
	z.removePlayer(1)
	z.drainSocial()
	beto = enter(z, 2, "Beto")
	if !slices.Contains(beto.systems(), "Você tem 2 cartas não lidas no correio (tecla O).") {
		t.Fatalf("login notice = %q", beto.systems())
	}
}

func TestMailValidation(t *testing.T) {
	z, f := socialZone()
	ana := enter(z, 1, "Ana")

	cases := []struct{ to, body, want string }{
		{"Ana", "oi", "Você não pode mandar uma carta para si mesmo."},
		{"Beto", "", "Uso: /correio Nome texto (ou a janela Social, tecla O, com assunto)."},
		{"x", "oi", "Uso: /correio Nome texto (ou a janela Social, tecla O, com assunto)."},
		{"Beto", strings.Repeat("a", mailBodyMax+1), "A carta tem no máximo 200 caracteres."},
	}
	for _, c := range cases {
		mailTo(z, 1, c.to, "", c.body)
		if got := ana.lastSystem(); got != c.want {
			t.Errorf("mail to %q (%d chars) = %q, want %q", c.to, len(c.body), got, c.want)
		}
	}
	say(z, 1, "/correio Beto")
	if got := ana.lastSystem(); !strings.HasPrefix(got, "Uso: /correio") {
		t.Fatalf("usage = %q", got)
	}
	// Uma linha de chat no limite pode ter sido cortada: pede a janela.
	say(z, 1, "/correio Beto "+strings.Repeat("b", MaxChatRunes))
	if got := ana.lastSystem(); !strings.Contains(got, "use a janela Social") {
		t.Fatalf("long chat = %q", got)
	}
	// Assunto cortado em 40; quem não existe só se sabe no banco.
	mailTo(z, 1, "Beto", strings.Repeat("s", 60), "texto")
	mailTo(z, 1, "Ninguem", "", "texto")
	z.drainSocial()
	if got := ana.lastSystem(); got != "Não existe personagem chamado Ninguem." {
		t.Fatalf("unknown = %q", got)
	}
	if len(f.mails) != 1 || len(f.mails[0].Subject) != mailSubjectMax {
		t.Fatalf("db = %+v", f.mails)
	}

	// Silenciado não manda carta.
	z.players[1].pl.mutedUntil = z.cfg.Clock().Add(time.Hour)
	say(z, 1, "/correio Beto oi")
	if got := ana.lastSystem(); !strings.HasPrefix(got, "Você está silenciado") {
		t.Fatalf("muted = %q", got)
	}
}

func TestMailSpamLimits(t *testing.T) {
	now := noon
	cfg := testConfig()
	cfg.Spawns = nil
	cfg.Clock = func() time.Time { return now }
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	f := newFakeSocial(FriendData{ID: 1, Name: "Ana"}, FriendData{ID: 2, Name: "Beto"}, FriendData{ID: 3, Name: "Caio"})
	z.SetSocialBackend(f)
	ana := enter(z, 1, "Ana")

	for i := range mailPerHour {
		now = noon.Add(time.Duration(i) * time.Minute)
		say(z, 1, "/correio Beto carta")
	}
	z.drainSocial()
	if f.sends != mailPerHour {
		t.Fatalf("sends = %d", f.sends)
	}
	// A 11ª é recusada na zona (nem chega ao banco), mesmo saindo e voltando.
	z.removePlayer(1)
	ana = enter(z, 1, "Ana")
	now = noon.Add(30 * time.Minute)
	say(z, 1, "/correio Caio carta")
	if got := ana.lastSystem(); got != "Você já mandou 10 cartas na última hora. Espere 30 min." {
		t.Fatalf("rate = %q", got)
	}
	if f.sends != mailPerHour || len(z.socialJobs) != 0 {
		t.Fatalf("refused letter reached the db: sends=%d queued=%d", f.sends, len(z.socialJobs))
	}

	// Uma hora depois da primeira, abre espaço para uma.
	now = noon.Add(time.Hour)
	say(z, 1, "/correio Caio carta")
	z.drainSocial()
	if got := ana.lastSystem(); got != "Carta enviada para Caio." {
		t.Fatalf("after an hour = %q", got)
	}

	// Caixa cheia: recusa e não conta no limite de quem mandou.
	for range mailBoxMax {
		f.mails = append(f.mails, fakeMail{to: 3})
	}
	now = noon.Add(2 * time.Hour)
	say(z, 1, "/correio Caio mais uma")
	z.drainSocial()
	if got := ana.lastSystem(); got != "A caixa de correio de Caio está cheia (100 cartas)." {
		t.Fatalf("full = %q", got)
	}
	if n := len(z.mailRecent(1, now)); n != 0 {
		t.Fatalf("refused letter counted: %d", n)
	}
}

package zone

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"erevia/server/internal/pb"
)

// Correio: carta de texto para um personagem, mesmo offline. Quem recebe é avisado (na
// hora, se estiver online, ou ao entrar) e lê numa caixa de entrada. A zona não guarda as
// cartas: o worker lê e grava no banco, e a caixa vai para o cliente quando ele pede.

const (
	mailBodyMax    = 200 // caracteres do texto
	mailSubjectMax = 40
	mailPerHour    = 10  // cartas por hora por remetente
	mailBoxMax     = 100 // cartas guardadas por destinatário
	mailListEvery  = 0.5 // segundos entre duas leituras da caixa
)

func mailCountsPB(unread, total int) *pb.MailBox {
	return &pb.MailBox{Unread: uint32(unread), Total: uint32(total), Max: mailBoxMax, BodyMax: mailBodyMax, SubjectMax: mailSubjectMax}
}

func (z *Zone) onMailCmd(p *entity, m *pb.MailCmd) {
	switch m.Kind {
	case pb.MailCmdKind_MAIL_CMD_SEND:
		z.sendMail(p, strings.TrimSpace(m.To), m.Subject, m.Body)
	case pb.MailCmdKind_MAIL_CMD_LIST:
		if z.now < p.pl.social.nextMail {
			return
		}
		p.pl.social.nextMail = z.now + mailListEvery
		z.mailJob(p, "inbox", nil)
	case pb.MailCmdKind_MAIL_CMD_READ:
		owner, id := p.id, int64(m.MailId)
		z.mailJob(p, "read mail", func(ctx context.Context, b SocialBackend) error { return b.MarkMailRead(ctx, owner, id) })
	case pb.MailCmdKind_MAIL_CMD_DELETE:
		if z.auctionMailLocked(p, int64(m.MailId)) {
			z.system(p, "Retire o anexo antes de apagar a carta.")
			return
		}
		owner, id, at := p.id, int64(m.MailId), z.cfg.Clock()
		z.mailJob(p, "delete mail", func(ctx context.Context, b SocialBackend) error { return b.DeleteMail(ctx, owner, id, at) })
	}
}

// mailJob aplica op (se houver) e devolve a caixa inteira ao jogador.
func (z *Zone) mailJob(p *entity, what string, op func(ctx context.Context, b SocialBackend) error) {
	id := p.id
	ok := z.queueSocial(func(ctx context.Context, b SocialBackend) func() {
		var err error
		if op != nil {
			err = z.dbTry(ctx, what, true, func(ctx context.Context) error { return op(ctx, b) })
		}
		var mails []MailData
		var unread, total int
		if err == nil {
			err = z.dbTry(ctx, "inbox", true, func(ctx context.Context) error {
				var err error
				if mails, err = b.Inbox(ctx, id); err == nil {
					unread, total, err = b.MailSummary(ctx, id)
				}
				return err
			})
		}
		var atts []MailAttachment
		if err == nil {
			atts, err = z.auctionAttachments(ctx, id)
		}
		return func() {
			if z.players[id] != p {
				return
			}
			if err != nil {
				z.log.Error(what, "err", err, "id", id)
				z.system(p, "Não foi possível abrir o correio agora. Tente de novo em instantes.")
				return
			}
			box := mailCountsPB(unread, total)
			box.Listed = true
			for _, m := range mails {
				box.Mails = append(box.Mails, &pb.Mail{
					Id: uint64(m.ID), FromName: m.FromName, Subject: m.Subject, Body: m.Body,
					SentAtMs: m.SentAt.UnixMilli(), Read: m.Read,
				})
			}
			z.auctionInbox(p, box, atts) // anexos da casa de leilões
			z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Social{Social: &pb.SocialUpdate{Mail: box}}})
		}
	})
	if !ok {
		z.system(p, "Não foi possível abrir o correio agora. Tente de novo em instantes.")
	}
}

// mailCommand trata /correio Nome texto (true = era o comando).
func (z *Zone) mailCommand(p *entity, cmd string, args []string, raw string) bool {
	switch cmd {
	case "/correio", "/carta", "/mail":
	default:
		return false
	}
	if len(args) < 2 {
		z.system(p, "Uso: /correio Nome texto (ou a janela Social, tecla O, com assunto).")
		return true
	}
	if utf8.RuneCountInString(raw) >= MaxChatRunes {
		z.system(p, "Carta longa demais para o chat: use a janela Social (tecla O).")
		return true
	}
	z.sendMail(p, args[0], "", strings.Join(args[1:], " "))
	return true
}

// mailRecent: envios do jogador na última hora (o limite vale também entre relogins).
func (z *Zone) mailRecent(id uint32, now time.Time) []time.Time {
	for k, list := range z.mailSent { // limpa quem não manda nada há uma hora
		if len(list) == 0 || now.Sub(list[len(list)-1]) >= time.Hour {
			delete(z.mailSent, k)
		}
	}
	list := z.mailSent[id]
	i := 0
	for i < len(list) && now.Sub(list[i]) >= time.Hour {
		i++
	}
	if i > 0 {
		list = list[i:]
		z.mailSent[id] = list
	}
	return list
}

func (z *Zone) sendMail(p *entity, to, subject, body string) {
	subject = truncateRunes(SanitizeChat(subject), mailSubjectMax)
	long := utf8.RuneCountInString(strings.TrimSpace(body)) > mailBodyMax
	body = SanitizeChat(body)
	now := z.cfg.Clock()
	if left := z.mutedFor(p); left > 0 {
		z.system(p, "Você está silenciado por mais "+minutesText(left)+".")
		return
	}
	switch {
	case !ValidName(to) || body == "":
		z.system(p, "Uso: /correio Nome texto (ou a janela Social, tecla O, com assunto).")
		return
	case long:
		z.system(p, fmt.Sprintf("A carta tem no máximo %d caracteres.", mailBodyMax))
		return
	case strings.EqualFold(to, p.name):
		z.system(p, "Você não pode mandar uma carta para si mesmo.")
		return
	}
	if recent := z.mailRecent(p.id, now); len(recent) >= mailPerHour {
		wait := time.Hour - now.Sub(recent[0])
		z.system(p, fmt.Sprintf("Você já mandou %d cartas na última hora. Espere %d min.", mailPerHour, int(math.Ceil(wait.Minutes()))))
		return
	}
	out := MailOut{
		FromID: p.id, FromName: p.name, ToName: to, Subject: subject, Body: body,
		At: now, PerHour: mailPerHour, MaxBox: mailBoxMax,
	}
	if !z.queueSocial(func(ctx context.Context, b SocialBackend) func() {
		var sent MailSent
		err := z.dbTry(ctx, "send mail", false, func(ctx context.Context) error {
			var err error
			sent, err = b.SendMail(ctx, out)
			return err
		})
		return func() { z.onMailSent(out, sent, err) }
	}) {
		z.system(p, "Não foi possível enviar a carta agora. Tente de novo em instantes.")
		return
	}
	z.mailSent[p.id] = append(z.mailSent[p.id], now)
}

// onMailSent: o banco respondeu. Remetente e destinatário podem já ter saído (ou nunca
// terem estado online).
func (z *Zone) onMailSent(out MailOut, sent MailSent, err error) {
	from := z.players[out.FromID]
	if err != nil {
		if !errors.Is(err, ErrMailRate) {
			// A carta não saiu: não conta no limite de envio.
			list := z.mailSent[out.FromID]
			for i, t := range list {
				if t.Equal(out.At) {
					z.mailSent[out.FromID] = append(list[:i:i], list[i+1:]...)
					break
				}
			}
		}
		if from == nil {
			return
		}
		switch {
		case errors.Is(err, ErrNoSuchCharacter):
			z.system(from, "Não existe personagem chamado "+out.ToName+".")
		case errors.Is(err, ErrSelf):
			z.system(from, "Você não pode mandar uma carta para si mesmo.")
		case errors.Is(err, ErrMailboxFull):
			z.system(from, fmt.Sprintf("A caixa de correio de %s está cheia (%d cartas).", out.ToName, mailBoxMax))
		case errors.Is(err, ErrMailRate):
			z.system(from, fmt.Sprintf("Você já mandou %d cartas na última hora. Espere um pouco.", mailPerHour))
		default:
			z.log.Error("send mail", "err", err, "from", out.FromID)
			z.system(from, "Não foi possível enviar a carta agora. Tente de novo em instantes.")
		}
		return
	}
	if from != nil {
		z.system(from, "Carta enviada para "+sent.ToName+".")
	}
	if to := z.players[sent.ToID]; to != nil {
		z.system(to, fmt.Sprintf("Você recebeu uma carta de %s (tecla O).", out.FromName))
		z.sendTo(to, &pb.ServerMessage{Msg: &pb.ServerMessage_Social{Social: &pb.SocialUpdate{Mail: mailCountsPB(sent.Unread, sent.Total)}}})
	}
}

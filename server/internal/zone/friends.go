package zone

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"erevia/server/internal/pb"
)

// Amigos e correio. A lista de amigos é de mão única (como "seguir"): quem adiciona vê
// se o outro está online e onde, e é avisado quando ele entra no jogo. Tudo é salvo no
// banco por um worker (a zona nunca espera o banco); os resultados voltam para a zona
// como funções que rodam na goroutine dela. A fila é uma só e em ordem: carregar a lista
// ao entrar sempre vê as mudanças feitas antes de sair.

const (
	friendMax      = 50
	socialJobQueue = 1024
)

var (
	// ErrSelf: o personagem tentou adicionar a si mesmo ou mandar carta para si.
	ErrSelf = errors.New("cannot target yourself")
	// ErrFriendListFull: a lista de amigos já tem o máximo.
	ErrFriendListFull = errors.New("friend list full")
	// ErrMailboxFull: o destinatário já guarda o máximo de cartas.
	ErrMailboxFull = errors.New("mailbox full")
	// ErrMailRate: o remetente mandou cartas demais na última hora.
	ErrMailRate = errors.New("too many letters")
)

// FriendData é um amigo como vem do banco.
type FriendData struct {
	ID    uint32
	Name  string
	Class string
	Level int
}

// MailOut é uma carta a enviar, com os limites contra spam (conferidos de novo no banco).
type MailOut struct {
	FromID   uint32
	FromName string
	ToName   string
	Subject  string
	Body     string
	At       time.Time
	PerHour  int
	MaxBox   int
}

// MailSent: para quem a carta foi e como ficou a caixa dele.
type MailSent struct {
	ToID   uint32
	ToName string
	Unread int
	Total  int
}

// MailData é uma carta da caixa de entrada.
type MailData struct {
	ID       int64
	FromName string
	Subject  string
	Body     string
	SentAt   time.Time
	Read     bool
}

// SocialBackend grava amigos e correio (implementado com o store).
type SocialBackend interface {
	Friends(ctx context.Context, characterID uint32) ([]FriendData, error)
	AddFriend(ctx context.Context, characterID uint32, name string, max int) (FriendData, error)
	RemoveFriend(ctx context.Context, characterID, friendID uint32) error
	SendMail(ctx context.Context, m MailOut) (MailSent, error)
	MailSummary(ctx context.Context, characterID uint32) (unread, total int, err error)
	Inbox(ctx context.Context, characterID uint32) ([]MailData, error)
	MarkMailRead(ctx context.Context, characterID uint32, mailID int64) error
	DeleteMail(ctx context.Context, characterID uint32, mailID int64, at time.Time) error
}

// socialJob roda no worker do banco e devolve o que fazer na zona com o resultado (nil = nada).
type socialJob func(ctx context.Context, b SocialBackend) func()

// socialDone leva o resultado de um socialJob de volta para a goroutine da zona.
type socialDone struct{ fn func() }

func (socialDone) isEvent() {}

// socialState: o lado social de um jogador online.
type socialState struct {
	friends  map[uint32]*FriendData // quem ele segue
	loaded   bool                   // a lista já veio do banco
	emote    pb.EmoteKind           // sentado ou dançando (NONE = nada)
	nextMail float64                // próxima leitura da caixa aceita (tempo de simulação)
}

// SetSocialBackend liga o banco de amigos e correio (antes de Run). Sem ele (a maioria dos
// testes), a lista de amigos começa vazia e nada é salvo.
func (z *Zone) SetSocialBackend(b SocialBackend) { z.socialBackend = b }

func (z *Zone) runSocialWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-z.socialJobs:
			if fn := job(ctx, z.socialBackend); fn != nil {
				z.post(ctx, socialDone{fn: fn})
			}
		}
	}
}

// queueSocial entrega um trabalho ao worker sem bloquear a zona.
func (z *Zone) queueSocial(job socialJob) bool {
	if z.socialBackend == nil {
		return false
	}
	select {
	case z.socialJobs <- job:
		return true
	default:
		z.log.Error("social queue full")
		return false
	}
}

// dbTry roda fn com prazo; com retry (operações que podem repetir sem estrago), tenta até
// 3 vezes se o banco falhar.
func (z *Zone) dbTry(ctx context.Context, what string, retry bool, fn func(ctx context.Context) error) error {
	attempts := 1
	if retry {
		attempts = 3
	}
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err = fn(cctx)
		cancel()
		z.reportSocialDBError(what, err) // db_error (ver errors.go)
		if err == nil || errors.Is(err, ErrNoSuchCharacter) || attempt == attempts {
			break
		}
		z.log.Error(what, "err", err, "attempt", attempt)
		select {
		case <-ctx.Done():
			return err
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	return err
}

// ---------- Entrada e saída ----------

// socialOnJoin carrega amigos e correio do banco e avisa quem segue o jogador que ele chegou.
func (z *Zone) socialOnJoin(p *entity) {
	for _, o := range z.players {
		if o != p && o.pl.social.friends[p.id] != nil {
			if !p.pl.hop.arrived { // chegou pelo portão: não acabou de entrar no jogo
				z.system(o, fmt.Sprintf("Seu amigo %s entrou no jogo.", p.name))
			}
			z.sendFriends(o)
		}
	}
	id := p.id
	if !z.queueSocial(func(ctx context.Context, b SocialBackend) func() {
		var friends []FriendData
		var unread, total int
		err := z.dbTry(ctx, "load friends", true, func(ctx context.Context) error {
			var err error
			friends, err = b.Friends(ctx, id)
			return err
		})
		if err == nil {
			err = z.dbTry(ctx, "mail summary", true, func(ctx context.Context) error {
				var err error
				unread, total, err = b.MailSummary(ctx, id)
				return err
			})
		}
		return func() { z.onSocialLoaded(p, friends, unread, total, err) }
	}) {
		p.pl.social.loaded = z.socialBackend == nil // sem banco: lista vazia
		if !p.pl.social.loaded {
			z.system(p, "Não foi possível carregar seus amigos e o correio agora.")
		}
	}
}

func (z *Zone) onSocialLoaded(p *entity, friends []FriendData, unread, total int, err error) {
	if z.players[p.id] != p {
		return // saiu (ou entrou de novo: a outra entrada tem a própria carga)
	}
	if err != nil {
		z.log.Error("load social", "err", err, "id", p.id)
		z.system(p, "Não foi possível carregar seus amigos e o correio agora.")
		return
	}
	s := &p.pl.social
	s.friends = make(map[uint32]*FriendData, len(friends))
	for _, f := range friends {
		s.friends[f.ID] = &f
	}
	s.loaded = true
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Social{Social: &pb.SocialUpdate{
		Friends: z.friendsPB(p), Mail: mailCountsPB(unread, total),
	}}})
	if on := z.friendsOnline(p); len(on) > 0 {
		z.system(p, "Amigos online: "+strings.Join(on, ", ")+".")
	}
	if unread > 0 {
		z.system(p, fmt.Sprintf("Você tem %s no correio (tecla O).", plural(unread, "carta não lida", "cartas não lidas")))
	}
}

// socialOnLeave: quem segue o jogador vê que ele saiu; o emote acaba junto.
func (z *Zone) socialOnLeave(p *entity) {
	for _, o := range z.players {
		if o != p && o.pl.social.friends[p.id] != nil {
			z.sendFriends(o)
		}
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// ---------- Comandos ----------

func (z *Zone) onFriendCmd(p *entity, m *pb.FriendCmd) {
	name := strings.TrimSpace(m.Name)
	switch m.Kind {
	case pb.FriendCmdKind_FRIEND_CMD_ADD:
		if m.TargetId != 0 {
			t := z.players[m.TargetId]
			if t == nil {
				z.system(p, "Esse jogador não está online.")
				return
			}
			name = t.name
		}
		z.addFriend(p, name)
	case pb.FriendCmdKind_FRIEND_CMD_REMOVE:
		z.removeFriend(p, m.TargetId, name)
	case pb.FriendCmdKind_FRIEND_CMD_LIST:
		z.sendFriends(p)
	}
}

// friendByName acha um amigo da lista pelo nome (sem diferenciar maiúsculas).
func (p *playerState) friendByName(name string) *FriendData {
	for _, f := range p.social.friends {
		if strings.EqualFold(f.Name, name) {
			return f
		}
	}
	return nil
}

func (z *Zone) addFriend(p *entity, name string) {
	s := &p.pl.social
	switch {
	case !s.loaded:
		z.system(p, "Sua lista de amigos ainda está carregando. Tente de novo em instantes.")
	case !ValidName(name):
		z.system(p, "Uso: /amigo Nome (para tirar: /amigo remover Nome)")
	case strings.EqualFold(name, p.name):
		z.system(p, "Você não pode adicionar a si mesmo.")
	case p.pl.friendByName(name) != nil:
		z.system(p, p.pl.friendByName(name).Name+" já está na sua lista de amigos.")
	case len(s.friends) >= friendMax:
		z.system(p, fmt.Sprintf("Sua lista de amigos está cheia (%d).", friendMax))
	default:
		id := p.id
		if !z.queueSocial(func(ctx context.Context, b SocialBackend) func() {
			var f FriendData
			err := z.dbTry(ctx, "add friend", false, func(ctx context.Context) error {
				var err error
				f, err = b.AddFriend(ctx, id, name, friendMax)
				return err
			})
			return func() { z.onFriendAdded(p, name, f, err) }
		}) {
			z.system(p, "Não foi possível adicionar agora. Tente de novo em instantes.")
		}
	}
}

func (z *Zone) onFriendAdded(p *entity, name string, f FriendData, err error) {
	if z.players[p.id] != p {
		return
	}
	switch {
	case errors.Is(err, ErrNoSuchCharacter):
		z.system(p, "Não existe personagem chamado "+name+".")
	case errors.Is(err, ErrSelf):
		z.system(p, "Você não pode adicionar a si mesmo.")
	case errors.Is(err, ErrFriendListFull):
		z.system(p, fmt.Sprintf("Sua lista de amigos está cheia (%d).", friendMax))
	case err != nil:
		z.log.Error("add friend", "err", err, "id", p.id)
		z.system(p, "Não foi possível adicionar agora. Tente de novo em instantes.")
	default:
		p.pl.social.friends[f.ID] = &f
		text := f.Name + " agora está na sua lista de amigos."
		if z.players[f.ID] != nil {
			text += " Está online."
		}
		z.system(p, text)
		z.sendFriends(p)
	}
}

func (z *Zone) removeFriend(p *entity, id uint32, name string) {
	s := &p.pl.social
	f := s.friends[id]
	if f == nil && name != "" {
		f = p.pl.friendByName(name)
	}
	if f == nil {
		if name != "" {
			z.system(p, name+" não está na sua lista de amigos.")
		}
		return
	}
	delete(s.friends, f.ID)
	owner, friend := p.id, f.ID
	z.queueSocial(func(ctx context.Context, b SocialBackend) func() {
		if err := z.dbTry(ctx, "remove friend", true, func(ctx context.Context) error {
			return b.RemoveFriend(ctx, owner, friend)
		}); err != nil {
			z.log.Error("remove friend", "err", err, "id", owner, "friend", friend)
		}
		return nil
	})
	z.system(p, f.Name+" saiu da sua lista de amigos.")
	z.sendFriends(p)
}

// friendsCommand trata /amigo e /amigos (true = era um desses comandos).
func (z *Zone) friendsCommand(p *entity, cmd string, args []string) bool {
	switch cmd {
	case "/amigo", "/amiga", "/friend":
		if len(args) >= 2 && slices.Contains([]string{"remover", "tirar", "remove", "-"}, strings.ToLower(args[0])) {
			z.removeFriend(p, 0, args[1])
		} else if len(args) == 1 {
			z.addFriend(p, args[0])
		} else {
			z.system(p, "Uso: /amigo Nome (para tirar: /amigo remover Nome)")
		}
	case "/amigos", "/friends":
		if !p.pl.social.loaded {
			z.system(p, "Sua lista de amigos ainda está carregando.")
			return true
		}
		on := z.friendsOnline(p)
		total := len(p.pl.social.friends)
		switch {
		case total == 0:
			z.system(p, "Sua lista de amigos está vazia. Adicione alguém com /amigo Nome ou pelo quadro do alvo.")
		case len(on) == 0:
			z.system(p, fmt.Sprintf("Nenhum dos seus %d amigos está online.", total))
		default:
			z.system(p, fmt.Sprintf("Amigos online (%d de %d): %s. A janela Social (O) mostra onde estão.", len(on), total, strings.Join(on, ", ")))
		}
	default:
		return false
	}
	return true
}

// ---------- Protocolo ----------

func (z *Zone) friendsOnline(p *entity) []string {
	var out []string
	for id := range p.pl.social.friends {
		if o := z.players[id]; o != nil {
			out = append(out, o.name)
		} else if f := p.pl.social.friends[id]; z.mgr != nil && z.mgr.bus.online(id) {
			out = append(out, f.Name) // em outro mapa
		}
	}
	slices.SortFunc(out, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return out
}

func (z *Zone) sendFriends(p *entity) {
	if !p.pl.social.loaded {
		return
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Social{Social: &pb.SocialUpdate{Friends: z.friendsPB(p)}}})
}

// friendsPB: a lista vista por p (online primeiro, com a posição de agora; depois pelo nome).
func (z *Zone) friendsPB(p *entity) *pb.FriendList {
	out := &pb.FriendList{Max: friendMax}
	for _, f := range p.pl.social.friends {
		fr := &pb.Friend{CharacterId: f.ID, Name: f.Name, Class: f.Class, Level: uint32(f.Level)}
		if o := z.players[f.ID]; o != nil {
			fr.Online = true
			fr.Level = uint32(o.level)
			fr.X, fr.Z = float32(o.pos.X), float32(o.pos.Z)
		} else if z.mgr != nil && z.mgr.bus.online(f.ID) {
			fr.Online = true // em outro mapa (sem posição)
		}
		out.Friends = append(out.Friends, fr)
	}
	slices.SortFunc(out.Friends, func(a, b *pb.Friend) int {
		if a.Online != b.Online {
			if a.Online {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}

package zone

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/pb"
)

// fakeSocial é o banco de amigos e correio em memória (mesmas regras do store).
type fakeSocial struct {
	chars   map[string]FriendData // nome em minúsculas → personagem
	friends map[uint32][]uint32
	mails   []fakeMail
	seq     int64
	sends   int // chamadas a SendMail que chegaram ao "banco"
}

type fakeMail struct {
	MailData
	to, from uint32
	deleted  bool
}

func newFakeSocial(chars ...FriendData) *fakeSocial {
	f := &fakeSocial{chars: map[string]FriendData{}, friends: map[uint32][]uint32{}}
	for _, c := range chars {
		f.chars[strings.ToLower(c.Name)] = c
	}
	return f
}

func (f *fakeSocial) byID(id uint32) FriendData {
	for _, c := range f.chars {
		if c.ID == id {
			return c
		}
	}
	return FriendData{}
}

func (f *fakeSocial) Friends(_ context.Context, id uint32) ([]FriendData, error) {
	var out []FriendData
	for _, fid := range f.friends[id] {
		out = append(out, f.byID(fid))
	}
	return out, nil
}

func (f *fakeSocial) AddFriend(_ context.Context, id uint32, name string, max int) (FriendData, error) {
	c, ok := f.chars[strings.ToLower(name)]
	switch {
	case !ok:
		return c, ErrNoSuchCharacter
	case c.ID == id:
		return c, ErrSelf
	case slices.Contains(f.friends[id], c.ID):
		return c, nil
	case len(f.friends[id]) >= max:
		return c, ErrFriendListFull
	}
	f.friends[id] = append(f.friends[id], c.ID)
	return c, nil
}

func (f *fakeSocial) RemoveFriend(_ context.Context, id, friend uint32) error {
	f.friends[id] = slices.DeleteFunc(f.friends[id], func(x uint32) bool { return x == friend })
	return nil
}

func (f *fakeSocial) SendMail(_ context.Context, m MailOut) (MailSent, error) {
	f.sends++
	c, ok := f.chars[strings.ToLower(m.ToName)]
	if !ok {
		return MailSent{}, ErrNoSuchCharacter
	}
	if c.ID == m.FromID {
		return MailSent{}, ErrSelf
	}
	if _, total, _ := f.MailSummary(context.Background(), c.ID); total >= m.MaxBox {
		return MailSent{}, ErrMailboxFull
	}
	f.seq++
	f.mails = append(f.mails, fakeMail{MailData: MailData{ID: f.seq, FromName: m.FromName, Subject: m.Subject, Body: m.Body, SentAt: m.At}, to: c.ID, from: m.FromID})
	u, t, _ := f.MailSummary(context.Background(), c.ID)
	return MailSent{ToID: c.ID, ToName: c.Name, Unread: u, Total: t}, nil
}

func (f *fakeSocial) MailSummary(_ context.Context, id uint32) (unread, total int, err error) {
	for _, m := range f.mails {
		if m.to == id && !m.deleted {
			total++
			if !m.Read {
				unread++
			}
		}
	}
	return unread, total, nil
}

func (f *fakeSocial) Inbox(_ context.Context, id uint32) ([]MailData, error) {
	var out []MailData
	for i := len(f.mails) - 1; i >= 0; i-- {
		if m := f.mails[i]; m.to == id && !m.deleted {
			out = append(out, m.MailData)
		}
	}
	return out, nil
}

func (f *fakeSocial) MarkMailRead(_ context.Context, id uint32, mailID int64) error {
	for i := range f.mails {
		if f.mails[i].ID == mailID && f.mails[i].to == id {
			f.mails[i].Read = true
		}
	}
	return nil
}

func (f *fakeSocial) DeleteMail(_ context.Context, id uint32, mailID int64, _ time.Time) error {
	for i := range f.mails {
		if f.mails[i].ID == mailID && f.mails[i].to == id {
			f.mails[i].deleted = true
		}
	}
	return nil
}

// drainSocial faz o papel do worker: roda os trabalhos na fila e aplica os resultados.
func (z *Zone) drainSocial() {
	for {
		select {
		case job := <-z.socialJobs:
			if fn := job(context.Background(), z.socialBackend); fn != nil {
				z.handle(socialDone{fn: fn})
			}
		default:
			return
		}
	}
}

// socialZone: zona com o banco social falso e quatro personagens conhecidos.
func socialZone() (*Zone, *fakeSocial) {
	z := newTestZone(nil)
	f := newFakeSocial(
		FriendData{ID: 1, Name: "Ana", Class: "knight", Level: 3},
		FriendData{ID: 2, Name: "Beto", Class: "mage", Level: 5},
		FriendData{ID: 3, Name: "Caio", Class: "rogue", Level: 7},
		FriendData{ID: 4, Name: "Dora", Class: "barbarian", Level: 9},
	)
	z.SetSocialBackend(f)
	return z, f
}

// enter: o personagem entra e o worker carrega amigos e correio.
func enter(z *Zone, id uint32, name string) *fakePeer {
	p := join(z, id, name)
	z.drainSocial()
	return p
}

func (f *fakePeer) lastFriends() *pb.FriendList {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if s := f.msgs[i].GetSocial(); s != nil && s.Friends != nil {
			return s.Friends
		}
	}
	return nil
}

func (f *fakePeer) lastMail() *pb.MailBox {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if s := f.msgs[i].GetSocial(); s != nil && s.Mail != nil {
			return s.Mail
		}
	}
	return nil
}

func friendNames(l *pb.FriendList) []string {
	var out []string
	for _, f := range l.GetFriends() {
		s := f.Name
		if f.Online {
			s += "*"
		}
		out = append(out, s)
	}
	return out
}

func TestFriendsAddOnlineAndOffline(t *testing.T) {
	z, f := socialZone()
	ana := enter(z, 1, "Ana")
	enter(z, 2, "Beto")
	z.players[2].pos = Vec2{X: 12, Z: -30}

	// Online, pelo botão do quadro do alvo (id); offline, pelo nome (/amigo).
	z.handle(socialEvent{id: 1, msg: &pb.FriendCmd{Kind: pb.FriendCmdKind_FRIEND_CMD_ADD, TargetId: 2}})
	say(z, 1, "/amigo caio")
	z.drainSocial()
	got := ana.lastFriends()
	if names := friendNames(got); !slices.Equal(names, []string{"Beto*", "Caio"}) || got.Max != friendMax {
		t.Fatalf("friends = %v (max %d)", names, got.Max)
	}
	if b := got.Friends[0]; b.X != 12 || b.Z != -30 || b.Level != 1 || b.Class != "mage" {
		t.Fatalf("online friend = %+v", b)
	}
	if c := got.Friends[1]; c.Level != 7 || c.X != 0 {
		t.Fatalf("offline friend = %+v", c)
	}
	if !slices.Equal(f.friends[1], []uint32{2, 3}) {
		t.Fatalf("db friends = %v", f.friends[1])
	}

	// Recusas: si mesmo, repetido, quem não existe.
	say(z, 1, "/amigo ana")
	if got := ana.lastSystem(); got != "Você não pode adicionar a si mesmo." {
		t.Fatalf("self = %q", got)
	}
	say(z, 1, "/amigo BETO")
	if got := ana.lastSystem(); got != "Beto já está na sua lista de amigos." {
		t.Fatalf("dup = %q", got)
	}
	say(z, 1, "/amigo Ninguem")
	z.drainSocial()
	if got := ana.lastSystem(); got != "Não existe personagem chamado Ninguem." {
		t.Fatalf("unknown = %q", got)
	}

	// Mão única: Beto não tem Ana na lista.
	if l := z.players[2].pl.social.friends; len(l) != 0 {
		t.Fatalf("beto's list = %v", l)
	}

	// Remover pelo nome (comando) e pelo id (janela).
	say(z, 1, "/amigo remover caio")
	z.handle(socialEvent{id: 1, msg: &pb.FriendCmd{Kind: pb.FriendCmdKind_FRIEND_CMD_REMOVE, TargetId: 2}})
	z.drainSocial()
	if names := friendNames(ana.lastFriends()); len(names) != 0 {
		t.Fatalf("after remove = %v", names)
	}
	if len(f.friends[1]) != 0 {
		t.Fatalf("db after remove = %v", f.friends[1])
	}
}

func TestFriendsLimit(t *testing.T) {
	z, f := socialZone()
	for i := range friendMax {
		id := uint32(100 + i)
		f.chars[strings.ToLower(fmtName(i))] = FriendData{ID: id, Name: fmtName(i), Class: "knight", Level: 1}
		f.friends[1] = append(f.friends[1], id)
	}
	ana := enter(z, 1, "Ana")
	say(z, 1, "/amigo Beto")
	if got := ana.lastSystem(); !strings.Contains(got, "cheia (50)") {
		t.Fatalf("limit = %q", got)
	}
	if len(z.socialJobs) != 0 {
		t.Fatal("full list still queued a job")
	}
}

func fmtName(i int) string { return "Amigo" + strconv.Itoa(i) }

func TestFriendLoginNoticeAndStatus(t *testing.T) {
	z, f := socialZone()
	f.friends[1] = []uint32{2, 3}
	ana := enter(z, 1, "Ana")
	if names := friendNames(ana.lastFriends()); !slices.Equal(names, []string{"Beto", "Caio"}) {
		t.Fatalf("loaded = %v", names)
	}

	// Beto entra: Ana (que segue Beto) é avisada; Beto não recebe aviso de Ana.
	beto := enter(z, 2, "Beto")
	if got := ana.lastSystem(); got != "Seu amigo Beto entrou no jogo." {
		t.Fatalf("notice = %q", got)
	}
	if names := friendNames(ana.lastFriends()); !slices.Equal(names, []string{"Beto*", "Caio"}) {
		t.Fatalf("after login = %v", names)
	}
	for _, s := range beto.systems() {
		if strings.Contains(s, "Ana") {
			t.Fatalf("one-way friendship notified the other side: %q", s)
		}
	}

	// Quem entra vê os amigos que já estão online.
	f.friends[3] = []uint32{1}
	caio := enter(z, 3, "Caio")
	if !slices.Contains(caio.systems(), "Amigos online: Ana.") {
		t.Fatalf("caio systems = %q", caio.systems())
	}

	// Beto sai: a lista de Ana atualiza (sem aviso no chat).
	before := len(ana.systems())
	z.removePlayer(2)
	if names := friendNames(ana.lastFriends()); !slices.Equal(names, []string{"Caio*", "Beto"}) {
		t.Fatalf("after logout = %v", names)
	}
	if len(ana.systems()) != before {
		t.Fatalf("logout wrote in chat: %q", ana.systems()[before:])
	}

	// /amigos lista quem está online.
	say(z, 1, "/amigos")
	if got := ana.lastSystem(); !strings.HasPrefix(got, "Amigos online (1 de 2): Caio.") {
		t.Fatalf("/amigos = %q", got)
	}
}

func TestFriendRemoveSurvivesQuickRelog(t *testing.T) {
	z, f := socialZone()
	f.friends[1] = []uint32{2}
	enter(z, 1, "Ana")
	say(z, 1, "/amigo remover Beto")
	// Sai e volta antes de o worker gravar: a fila é uma só, então a carga vê a remoção.
	z.removePlayer(1)
	ana := join(z, 1, "Ana")
	z.drainSocial()
	if names := friendNames(ana.lastFriends()); len(names) != 0 {
		t.Fatalf("relog list = %v", names)
	}
}

func TestFriendsWithoutBackendStartEmpty(t *testing.T) {
	z := newTestZone(nil)
	ana := join(z, 1, "Ana")
	say(z, 1, "/amigo Beto")
	if got := ana.lastSystem(); got != "Não foi possível adicionar agora. Tente de novo em instantes." {
		t.Fatalf("no backend = %q", got)
	}
}

package zone

import (
	"context"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/pb"
)

// fakeMod faz o papel do banco da moderação.
type fakeMod struct {
	known  map[string]bool // nomes (minúsculas) que existem no "banco"
	banned map[string]bool
	mutes  map[int64]time.Time
	logs   []GMLog
	bugs   []BugReport
}

func newFakeMod(names ...string) *fakeMod {
	f := &fakeMod{known: map[string]bool{}, banned: map[string]bool{}, mutes: map[int64]time.Time{}}
	for _, n := range names {
		f.known[strings.ToLower(n)] = true
	}
	return f
}

func (f *fakeMod) SetBanned(_ context.Context, name string, banned bool, _ string) error {
	if !f.known[strings.ToLower(name)] {
		return ErrNoSuchCharacter
	}
	f.banned[strings.ToLower(name)] = banned
	return nil
}
func (f *fakeMod) SetMute(_ context.Context, acc int64, until time.Time) error {
	f.mutes[acc] = until
	return nil
}
func (f *fakeMod) LogGM(_ context.Context, e GMLog) error { f.logs = append(f.logs, e); return nil }
func (f *fakeMod) SaveBug(_ context.Context, b BugReport) error {
	f.bugs = append(f.bugs, b)
	return nil
}

// drainMod roda os trabalhos do banco na hora (os testes não rodam o worker).
func drainMod(z *Zone) {
	for {
		select {
		case job := <-z.modJobs:
			if ev := z.doModJob(context.Background(), job); ev != nil {
				z.handle(ev)
			}
		default:
			return
		}
	}
}

// joinAs entra com uma conta própria (id+100) e, opcionalmente, como GM.
func joinAs(z *Zone, id uint32, name string, gm bool) *fakePeer {
	p := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: id, Name: name, Class: "knight", AccountID: int64(id) + 100, GM: gm, Peer: p}})
	return p
}

func (f *fakePeer) systems() []string {
	var out []string
	for _, c := range f.chats() {
		if c.Channel == pb.ChatChannel_CHAT_CHANNEL_SYSTEM {
			out = append(out, c.Text)
		}
	}
	return out
}

func (f *fakePeer) lastSystem() string {
	s := f.systems()
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1]
}

func (f *fakePeer) kicked() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if k := m.GetKicked(); k != nil {
			return k.Reason
		}
	}
	return ""
}

// say manda uma mensagem no canal geral (repõe as fichas de chat antes).
func say(z *Zone, id uint32, text string) {
	if p := z.players[id]; p != nil {
		p.pl.chatTokens = z.cfg.ChatBurst
	}
	z.handle(chatEvent{id: id, text: text})
}

func generalFrom(p *fakePeer, name string) int {
	n := 0
	for _, c := range p.chats() {
		if c.Channel == pb.ChatChannel_CHAT_CHANNEL_GENERAL && c.FromName == name {
			n++
		}
	}
	return n
}

func TestCommandsAreNotBroadcastAndGMOnlyForGMs(t *testing.T) {
	z := newTestZone(nil)
	ana, beto := joinAs(z, 1, "Ana", false), joinAs(z, 2, "Beto", false)

	say(z, 1, "/kick Beto")
	say(z, 1, "/qualquer coisa")
	if got := ana.lastSystem(); got != "Comando desconhecido: /qualquer" {
		t.Fatalf("unknown command reply = %q", got)
	}
	if !slices.Contains(ana.systems(), "Comando desconhecido: /kick") {
		t.Fatalf("non-GM /kick should look unknown, got %q", ana.systems())
	}
	if beto.closed != "" || z.players[2] == nil {
		t.Fatal("non-GM kicked someone")
	}
	if n := generalFrom(beto, "Ana"); n != 0 {
		t.Fatalf("commands leaked to general chat: %d", n)
	}
}

func TestGMKickAndLog(t *testing.T) {
	z := newTestZone(nil)
	mod := newFakeMod()
	z.SetModBackend(mod)
	gm, beto := joinAs(z, 1, "Mestre", true), joinAs(z, 2, "Beto", false)

	say(z, 1, "/kick beto flood no chat")
	drainMod(z)

	if beto.closed == "" || z.players[2] != nil {
		t.Fatal("Beto should be disconnected and removed")
	}
	if r := beto.kicked(); !strings.Contains(r, "flood no chat") {
		t.Fatalf("kick reason = %q", r)
	}
	if got := gm.lastSystem(); got != "Beto foi desconectado." {
		t.Fatalf("gm reply = %q", got)
	}
	if len(mod.logs) != 1 || mod.logs[0].Command != "kick" || mod.logs[0].Target != "Beto" || mod.logs[0].GMName != "Mestre" {
		t.Fatalf("gm log = %+v", mod.logs)
	}
}

func TestMuteBlocksEveryChannelUntilItExpires(t *testing.T) {
	now := noon
	z := newTestZone(nil)
	z.cfg.Clock = func() time.Time { return now }
	mod := newFakeMod()
	z.SetModBackend(mod)
	joinAs(z, 1, "Mestre", true)
	ana, beto := joinAs(z, 2, "Ana", false), joinAs(z, 3, "Beto", false)

	say(z, 1, "/mute Beto 5 xingou")
	drainMod(z)
	if until := mod.mutes[103]; !until.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("saved mute = %v", until)
	}
	if !strings.Contains(beto.lastSystem(), "silenciado por 5 minutos") {
		t.Fatalf("muted player notice = %q", beto.lastSystem())
	}

	say(z, 3, "oi gente")
	z.players[3].pl.chatTokens = z.cfg.ChatBurst
	z.handle(chatEvent{id: 3, text: "psiu", to: "Ana"})
	if generalFrom(ana, "Beto") != 0 || len(ana.chats()) != 0 {
		t.Fatalf("muted messages reached Ana: %+v", ana.chats())
	}
	if !strings.HasPrefix(beto.lastSystem(), "Você está silenciado por mais") {
		t.Fatalf("muted reply = %q", beto.lastSystem())
	}

	// /bug continua funcionando com silêncio.
	say(z, 3, "/bug o golem atravessou a parede")
	drainMod(z)
	if len(mod.bugs) != 1 {
		t.Fatal("muted player should still report bugs")
	}

	now = now.Add(6 * time.Minute)
	say(z, 3, "voltei")
	if generalFrom(ana, "Beto") != 1 {
		t.Fatal("mute should expire")
	}
}

func TestMuteFromDatabaseAppliesOnJoin(t *testing.T) {
	z := newTestZone(nil)
	ana := join(z, 1, "Ana")
	p := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: 2, Name: "Beto", Class: "knight", AccountID: 7, MutedUntil: noon.Add(time.Hour), Peer: p}})

	say(z, 2, "escapei do silêncio?")
	if generalFrom(ana, "Beto") != 0 {
		t.Fatal("mute saved in the database must survive relog")
	}
}

func TestGMBanKicksOnlineAndRepliesFromDatabase(t *testing.T) {
	z := newTestZone(nil)
	mod := newFakeMod("Beto", "Caio")
	z.SetModBackend(mod)
	gm, beto := joinAs(z, 1, "Mestre", true), joinAs(z, 2, "Beto", false)

	say(z, 1, "/ban Beto bot de caça")
	drainMod(z)
	if !mod.banned["beto"] || z.players[2] != nil || !strings.Contains(beto.kicked(), "banida") {
		t.Fatalf("ban: banned=%v online=%v kicked=%q", mod.banned["beto"], z.players[2] != nil, beto.kicked())
	}
	if got := gm.lastSystem(); got != "Beto foi banido." {
		t.Fatalf("gm reply = %q", got)
	}

	say(z, 1, "/ban Caio") // offline: só no banco
	drainMod(z)
	if !mod.banned["caio"] || gm.lastSystem() != "Caio foi banido." {
		t.Fatalf("offline ban: %v %q", mod.banned["caio"], gm.lastSystem())
	}

	say(z, 1, "/ban Ninguem")
	drainMod(z)
	if got := gm.lastSystem(); got != "Não existe personagem chamado Ninguem." {
		t.Fatalf("unknown name reply = %q", got)
	}

	say(z, 1, "/ban mestre")
	if gm.closed != "" || gm.lastSystem() != "Você não pode banir a si mesmo." {
		t.Fatalf("self-ban: closed=%q reply=%q", gm.closed, gm.lastSystem())
	}

	say(z, 1, "/unban Beto")
	drainMod(z)
	if mod.banned["beto"] || gm.lastSystem() != "Beto foi desbanido." {
		t.Fatalf("unban: %v %q", mod.banned["beto"], gm.lastSystem())
	}
}

func TestGMGiveItemsAndGold(t *testing.T) {
	z := newTestZone(nil)
	gm := joinAs(z, 1, "Mestre", true)
	ana := joinAs(z, 2, "Ana", false)
	gold := z.players[2].pl.inv.gold

	say(z, 1, "/give Ana potion_hp_l 3")
	if n := z.players[2].pl.inv.countOf("potion_hp_l"); n != 3 {
		t.Fatalf("potions = %d", n)
	}
	if !strings.Contains(ana.lastSystem(), "de um GM") {
		t.Fatalf("receiver notice = %q", ana.lastSystem())
	}

	say(z, 1, "/give Ana moedas 500")
	if got := z.players[2].pl.inv.gold; got != gold+500 {
		t.Fatalf("gold = %d, want %d", got, gold+500)
	}

	say(z, 1, "/give Ana espada_lendaria")
	if !strings.HasPrefix(gm.lastSystem(), "Item desconhecido") {
		t.Fatalf("unknown item reply = %q", gm.lastSystem())
	}
}

func TestGMTeleportGotoAndSummon(t *testing.T) {
	z := newTestZone(nil)
	joinAs(z, 1, "Mestre", true)
	beto := joinAs(z, 2, "Beto", false)
	m, b := z.players[1], z.players[2]

	say(z, 1, "/tp 60 -70")
	if m.pos != (Vec2{X: 60, Z: -70}) || !m.teleport {
		t.Fatalf("tp: pos=%v teleport=%v", m.pos, m.teleport)
	}

	say(z, 1, "/summon Beto")
	if d := dist(b.pos, m.pos); d > 1+arriveScatter+0.01 {
		t.Fatalf("summon: Beto is %.1f m away", d)
	}
	if beto.lastSystem() != "Um GM levou você até ele." {
		t.Fatalf("summon notice = %q", beto.lastSystem())
	}

	b.teleportTo(Vec2{X: -100, Z: 100})
	say(z, 1, "/goto beto")
	if d := dist(b.pos, m.pos); d > 1+arriveScatter+0.01 {
		t.Fatalf("goto: GM is %.1f m away", d)
	}

	say(z, 1, "/tp 1e999 0")
	if !strings.HasPrefix(z.players[1].pl.peer.(*fakePeer).lastSystem(), "Uso: /tp") {
		t.Fatal("infinite coordinate should be rejected")
	}
	if math.IsInf(m.pos.X, 0) {
		t.Fatal("GM went to infinity")
	}
}

func TestBugReportCarriesContextAndHasCooldown(t *testing.T) {
	z := newTestZone(nil)
	z.cfg.Version = "abc123"
	mod := newFakeMod()
	z.SetModBackend(mod)
	p := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Ana", Class: "mage", AccountID: 9, X: 12, Z: -8, UserAgent: "Firefox", Peer: p}})

	say(z, 1, "/bug ai")
	if !strings.HasPrefix(p.lastSystem(), "Uso: /bug") {
		t.Fatalf("short report reply = %q", p.lastSystem())
	}

	say(z, 1, "/bug   fiquei preso na pedra do portão sul ")
	drainMod(z)
	if len(mod.bugs) != 1 {
		t.Fatalf("bugs = %d", len(mod.bugs))
	}
	b := mod.bugs[0]
	pos := z.players[1].pos
	if b.AccountID != 9 || b.Name != "Ana" || b.Version != "abc123" || b.UserAgent != "Firefox" ||
		b.X != pos.X || b.Z != pos.Z || b.Text != "fiquei preso na pedra do portão sul" {
		t.Fatalf("report = %+v", b)
	}
	if p.lastSystem() != "Obrigado! Seu relato foi enviado." {
		t.Fatalf("reply = %q", p.lastSystem())
	}

	say(z, 1, "/bug outro problema aqui agora")
	if !strings.HasPrefix(p.lastSystem(), "Espere") {
		t.Fatalf("cooldown reply = %q", p.lastSystem())
	}
	z.now += bugCooldown
	say(z, 1, "/bug outro problema aqui agora")
	drainMod(z)
	if len(mod.bugs) != 2 {
		t.Fatalf("after cooldown bugs = %d", len(mod.bugs))
	}
}

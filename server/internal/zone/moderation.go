package zone

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Moderação e relatos: comandos de chat que começam com "/" no canal geral.
// Qualquer jogador usa /bug, /amigo, /amigos, /correio e os emotes (friends.go, mail.go e
// emote.go); os outros comandos são só de GM (accounts.gm) e ficam
// registrados no gm_log. Para quem não é GM, eles nem existem ("comando desconhecido").

const (
	modJobQueue   = 256
	bugCooldown   = 30.0 // segundos entre dois /bug do mesmo jogador
	bugMinRunes   = 8
	maxMuteMin    = 7 * 24 * 60
	maxGiveCount  = 9999
	maxGiveGold   = 1_000_000_000
	maxUserAgent  = 300
	maxOnlineList = 40
)

// ErrNoSuchCharacter: o backend não achou o personagem pelo nome.
var ErrNoSuchCharacter = errors.New("no such character")

// ModBackend grava moderação e relatos (implementado com o store).
type ModBackend interface {
	SetBanned(ctx context.Context, characterName string, banned bool, reason string) error
	SetMute(ctx context.Context, accountID int64, until time.Time) error
	LogGM(ctx context.Context, e GMLog) error
	SaveBug(ctx context.Context, b BugReport) error
}

// GMLog é uma ação de GM.
type GMLog struct {
	GMAccountID int64
	GMName      string
	Command     string
	Target      string
	Detail      string
}

// BugReport é um /bug com o contexto de quem relatou.
type BugReport struct {
	AccountID   int64
	CharacterID uint32
	Name        string
	Level       int
	X, Z        float64
	Version     string
	UserAgent   string
	Text        string
}

// modJob roda no worker do banco; o resultado volta para a zona como mensagem a "reply".
type modJob struct {
	run   func(ctx context.Context, b ModBackend) error
	reply uint32 // quem recebe o resultado (0 = ninguém)
	ok    string
	fail  func(err error) string
}

type modResultEvent struct {
	to   uint32
	text string
}

func (modResultEvent) isEvent() {}

// SetModBackend liga o banco da moderação (antes de Run). Sem ele (testes), os
// trabalhos contam como gravados.
func (z *Zone) SetModBackend(b ModBackend) { z.modBackend = b }

func (z *Zone) runModWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-z.modJobs:
			if ev := z.doModJob(ctx, job); ev != nil {
				z.post(ctx, ev)
			}
		}
	}
}

// doModJob tenta até 3 vezes (menos quando o personagem não existe).
func (z *Zone) doModJob(ctx context.Context, job modJob) event {
	var err error
	if z.modBackend != nil {
		for attempt := 1; attempt <= 3; attempt++ {
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = job.run(cctx, z.modBackend)
			cancel()
			if err == nil || errors.Is(err, ErrNoSuchCharacter) {
				break
			}
			z.log.Error("mod job", "err", err, "attempt", attempt)
			z.ReportDBError("moderation", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
	}
	if job.reply == 0 {
		return nil
	}
	text := job.ok
	if err != nil && job.fail != nil {
		text = job.fail(err)
	}
	if text == "" {
		return nil
	}
	return modResultEvent{to: job.reply, text: text}
}

func (z *Zone) queueMod(job modJob) bool {
	select {
	case z.modJobs <- job:
		return true
	default:
		z.log.Error("mod queue full")
		return false
	}
}

func (z *Zone) onModResult(e modResultEvent) {
	if p := z.players[e.to]; p != nil {
		z.system(p, e.text)
	}
}

// ---------- Silêncio ----------

// mutedFor: quanto falta do silêncio (0 = pode falar).
func (z *Zone) mutedFor(p *entity) time.Duration {
	if p.pl.mutedUntil.IsZero() {
		return 0
	}
	left := p.pl.mutedUntil.Sub(z.cfg.Clock())
	if left <= 0 {
		p.pl.mutedUntil = time.Time{}
		return 0
	}
	return left
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func minutesText(d time.Duration) string {
	m := int(math.Ceil(d.Minutes()))
	if m == 1 {
		return "1 minuto"
	}
	return fmt.Sprintf("%d minutos", m)
}

// ---------- Comandos ----------

// isCommand: texto do canal geral que começa com "/" (o cliente já tratou /w, /g, /c…).
func isCommand(text, to string, channel pb.ChatChannel) bool {
	return to == "" && strings.HasPrefix(text, "/") &&
		channel != pb.ChatChannel_CHAT_CHANNEL_PARTY && channel != pb.ChatChannel_CHAT_CHANNEL_CLAN
}

func (z *Zone) command(p *entity, text string) {
	fields := strings.Fields(text)
	cmd := strings.ToLower(fields[0])
	args := fields[1:]
	if cmd == "/bug" {
		z.bugReport(p, strings.Join(args, " "))
		return
	}
	if z.friendsCommand(p, cmd, args) || z.mailCommand(p, cmd, args, text) || z.emoteCommand(p, cmd) {
		return
	}
	if cmd == "/raid" && z.cfg.DevCommands { // teste do raid boss: só com DEV_LOGIN
		z.devRaid(p, args)
		return
	}
	if cmd == "/raro" && z.cfg.DevCommands { // monstros raros de teste (rares.go)
		z.devRare(p, args)
		return
	}
	if cmd == "/avisos" && z.cfg.DevCommands { // avisos no chão de teste (abilities.go)
		z.devWarnings(p)
		return
	}
	if !p.pl.gm {
		z.system(p, "Comando desconhecido: "+fields[0])
		return
	}
	if z.gmElsewhere(p, cmd, args, text) { // alvo em outra zona: o comando roda lá (zones_mod.go)
		return
	}
	switch cmd {
	case "/gm":
		z.system(p, "GM: /kick Nome [motivo] · /mute Nome minutos [motivo] · /unmute Nome · "+
			"/ban Nome [motivo] · /unban Nome · /tp X Z · /goto Nome · /summon Nome · "+
			"/give Nome item|moedas [qtd] · /anuncio texto · /online · "+
			"/evento invasao [agora|parar] · /evento xp2 horas|off · /maestria xp N [Nome] · /maestria dia [Nome] · "+
			"/contrato andar N [Nome] · /contrato dia [Nome] · /contrato rep bastion|guard N [Nome] · /vitalidade N [Nome] · "+
			"/subclasse passo|recarga [Nome] · /subclasse xp N [Nome]")
	case "/kick":
		z.gmKick(p, args)
	case "/mute":
		z.gmMute(p, args)
	case "/unmute":
		z.gmUnmute(p, args)
	case "/ban":
		z.gmBan(p, args, true)
	case "/unban":
		z.gmBan(p, args, false)
	case "/tp":
		z.gmTeleport(p, args)
	case "/goto":
		z.gmGoto(p, args)
	case "/summon":
		z.gmSummon(p, args)
	case "/give":
		z.gmGive(p, args)
	case "/anuncio", "/anúncio", "/announce":
		z.gmAnnounce(p, strings.Join(args, " "))
	case "/online":
		z.gmOnline(p)
	case "/evento":
		z.gmEvent(p, args)
	case "/contrato":
		z.gmContract(p, args)
	case "/maestria":
		z.gmMastery(p, args)
	case "/subclasse":
		z.gmSubclass(p, args)
	case "/vitalidade":
		z.gmVitality(p, args)
	default:
		z.system(p, "Comando desconhecido: "+fields[0]+" (/gm lista os comandos)")
	}
}

// bugReport: /bug pelo chat (sem o contexto do cliente; o cliente novo manda por
// POST /api/bug-reports, ver bugreport.go).
func (z *Zone) bugReport(p *entity, text string) {
	r, err := z.takeBug(p, text)
	if err != nil {
		z.system(p, BugErrorText(err))
		return
	}
	if !z.queueMod(modJob{
		run:   func(ctx context.Context, b ModBackend) error { return b.SaveBug(ctx, r) },
		reply: p.id,
		ok:    "Obrigado! Seu relato foi enviado.",
		fail:  func(error) string { return "Não foi possível enviar o relato agora. Tente de novo em instantes." },
	}) {
		z.system(p, "Não foi possível enviar o relato agora. Tente de novo em instantes.")
	}
}

// gmLog registra a ação (sem resposta para o GM).
func (z *Zone) gmLog(gm *entity, command, target, detail string) {
	e := GMLog{GMAccountID: gm.pl.accountID, GMName: gm.name, Command: command, Target: target, Detail: detail}
	z.queueMod(modJob{run: func(ctx context.Context, b ModBackend) error { return b.LogGM(ctx, e) }})
	z.log.Info("gm command", "gm", gm.name, "cmd", command, "target", target, "detail", detail)
}

// online acha um jogador pelo nome; avisa o GM se não estiver.
func (z *Zone) online(gm *entity, name string) *entity {
	if id, ok := z.byName[strings.ToLower(name)]; ok {
		return z.players[id]
	}
	if where := z.elsewhere(name); where != "" {
		z.system(gm, name+" está em outro mapa ("+where+").")
		return nil
	}
	z.system(gm, name+" não está online.")
	return nil
}

// sameAccount: todos os personagens online da conta de t (só t, se a conta for desconhecida).
func (z *Zone) sameAccount(t *entity) []*entity {
	if t.pl.accountID == 0 {
		return []*entity{t}
	}
	var out []*entity
	for _, p := range z.players {
		if p.pl.accountID == t.pl.accountID {
			out = append(out, p)
		}
	}
	return out
}

// kick avisa e derruba a conexão do jogador.
func (z *Zone) kick(p *entity, reason string) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Kicked{Kicked: &pb.Kicked{Reason: reason}}})
	p.pl.peer.Close("kick")
	z.removePlayer(p.id)
}

func withReason(text string, args []string) string {
	if r := strings.Join(args, " "); r != "" {
		return text + " Motivo: " + r
	}
	return text
}

func (z *Zone) gmKick(gm *entity, args []string) {
	if len(args) < 1 {
		z.system(gm, "Uso: /kick Nome [motivo]")
		return
	}
	t := z.online(gm, args[0])
	if t == nil {
		return
	}
	name := t.name
	z.kick(t, withReason("Você foi desconectado por um GM.", args[1:]))
	z.system(gm, name+" foi desconectado.")
	z.gmLog(gm, "kick", name, strings.Join(args[1:], " "))
}

func (z *Zone) gmMute(gm *entity, args []string) {
	mins := 0
	if len(args) >= 2 {
		mins, _ = strconv.Atoi(args[1])
	}
	if len(args) < 2 || mins < 1 || mins > maxMuteMin {
		z.system(gm, fmt.Sprintf("Uso: /mute Nome minutos [motivo] (1 a %d)", maxMuteMin))
		return
	}
	t := z.online(gm, args[0])
	if t == nil {
		return
	}
	until := z.cfg.Clock().Add(time.Duration(mins) * time.Minute)
	for _, p := range z.sameAccount(t) {
		p.pl.mutedUntil = until
		z.system(p, withReason(fmt.Sprintf("Você foi silenciado por %s.", minutesText(until.Sub(z.cfg.Clock()))), args[2:]))
	}
	account := t.pl.accountID
	z.queueMod(modJob{run: func(ctx context.Context, b ModBackend) error { return b.SetMute(ctx, account, until) }})
	z.system(gm, fmt.Sprintf("%s silenciado por %d min.", t.name, mins))
	z.gmLog(gm, "mute", t.name, strings.TrimSpace(fmt.Sprintf("%d min %s", mins, strings.Join(args[2:], " "))))
}

func (z *Zone) gmUnmute(gm *entity, args []string) {
	if len(args) < 1 {
		z.system(gm, "Uso: /unmute Nome")
		return
	}
	t := z.online(gm, args[0])
	if t == nil {
		return
	}
	for _, p := range z.sameAccount(t) {
		p.pl.mutedUntil = time.Time{}
		z.system(p, "Você pode falar no chat de novo.")
	}
	account := t.pl.accountID
	z.queueMod(modJob{run: func(ctx context.Context, b ModBackend) error { return b.SetMute(ctx, account, time.Time{}) }})
	z.system(gm, t.name+" pode falar de novo.")
	z.gmLog(gm, "unmute", t.name, "")
}

// gmBan grava no banco (funciona com o personagem offline) e derruba quem estiver online.
func (z *Zone) gmBan(gm *entity, args []string, ban bool) {
	verb := map[bool]string{true: "ban", false: "unban"}[ban]
	if len(args) < 1 {
		z.system(gm, "Uso: /"+verb+" Nome"+map[bool]string{true: " [motivo]", false: ""}[ban])
		return
	}
	name := args[0]
	reason := strings.Join(args[1:], " ")
	if ban {
		if strings.EqualFold(name, gm.name) {
			z.system(gm, "Você não pode banir a si mesmo.")
			return
		}
		if id, ok := z.byName[strings.ToLower(name)]; ok {
			t := z.players[id]
			name = t.name
			for _, p := range z.sameAccount(t) {
				z.kick(p, withReason("Sua conta foi banida.", args[1:]))
			}
		}
	}
	done := map[bool]string{true: name + " foi banido.", false: name + " foi desbanido."}[ban]
	z.queueMod(modJob{
		run:   func(ctx context.Context, b ModBackend) error { return b.SetBanned(ctx, name, ban, reason) },
		reply: gm.id,
		ok:    done,
		fail: func(err error) string {
			if errors.Is(err, ErrNoSuchCharacter) {
				return "Não existe personagem chamado " + name + "."
			}
			return "Erro ao gravar no banco: tente de novo."
		},
	})
	z.gmLog(gm, verb, name, reason)
}

// gmPlace põe o jogador num ponto livre, cancelando ação, canal e alvos dos monstros.
func (z *Zone) gmPlace(p *entity, dest Vec2, scatter bool) {
	z.cancelChannel(p, "")
	p.clearAction()
	if scatter {
		z.arrive(p, dest)
		return
	}
	z.forgetTarget(p.id)
	p.teleportTo(z.freeSpot(clampToZone(dest, z.bounds())))
}

func (z *Zone) gmTeleport(gm *entity, args []string) {
	var x, zz float64
	var errX, errZ error
	if len(args) == 2 {
		x, errX = strconv.ParseFloat(strings.ReplaceAll(args[0], ",", "."), 64)
		zz, errZ = strconv.ParseFloat(strings.ReplaceAll(args[1], ",", "."), 64)
	}
	if len(args) != 2 || errX != nil || errZ != nil || math.IsNaN(x) || math.IsNaN(zz) {
		z.system(gm, "Uso: /tp X Z (as coordenadas embaixo do minimapa)")
		return
	}
	z.gmPlace(gm, Vec2{X: x, Z: zz}, false)
	z.gmLog(gm, "tp", "", fmt.Sprintf("%.0f %.0f", x, zz))
}

func (z *Zone) gmGoto(gm *entity, args []string) {
	if len(args) < 1 {
		z.system(gm, "Uso: /goto Nome")
		return
	}
	if t := z.online(gm, args[0]); t != nil && t != gm {
		z.gmPlace(gm, t.pos, true)
		z.gmLog(gm, "goto", t.name, "")
	}
}

func (z *Zone) gmSummon(gm *entity, args []string) {
	if len(args) < 1 {
		z.system(gm, "Uso: /summon Nome")
		return
	}
	t := z.online(gm, args[0])
	if t == nil || t == gm {
		return
	}
	if t.dead {
		z.system(gm, t.name+" está morto.")
		return
	}
	z.gmPlace(t, gm.pos, true)
	z.system(t, "Um GM levou você até ele.")
	z.gmLog(gm, "summon", t.name, "")
}

func (z *Zone) gmGive(gm *entity, args []string) {
	if len(args) < 2 {
		z.system(gm, "Uso: /give Nome item|moedas [qtd]")
		return
	}
	t := z.online(gm, args[0])
	if t == nil {
		return
	}
	id := strings.ToLower(args[1])
	count := 1
	if len(args) >= 3 {
		n, err := strconv.Atoi(args[2])
		if err != nil || n < 1 {
			z.system(gm, "Quantidade inválida.")
			return
		}
		count = n
	}

	if id == "moedas" || id == "gold" {
		count = min(count, maxGiveGold)
		t.pl.inv.gold += int64(count)
		z.inventoryChanged(t, false)
		z.system(t, fmt.Sprintf("Você recebeu %d moedas de um GM.", count))
		z.system(gm, fmt.Sprintf("%s recebeu %d moedas.", t.name, count))
		z.gmLog(gm, "give", t.name, fmt.Sprintf("gold %d", count))
		z.trackGold(t, "gm", int64(count))
		return
	}
	def := game.Items[id]
	if def == nil {
		z.system(gm, "Item desconhecido: "+args[1]+" (use o id, ex.: potion_hp_s).")
		return
	}
	count = min(count, maxGiveCount)
	given := count - t.pl.inv.add(def, count)
	if given == 0 {
		z.system(gm, "A bolsa de "+t.name+" está cheia.")
		return
	}
	z.inventoryChanged(t, false)
	z.system(t, "Você recebeu "+itemLabel(def, given)+" de um GM.")
	z.system(gm, t.name+" recebeu "+itemLabel(def, given)+".")
	z.gmLog(gm, "give", t.name, fmt.Sprintf("%s %d", def.ID, given))
	z.trackItem(t, true, def.ID, given, "gm")
}

func (z *Zone) gmAnnounce(gm *entity, text string) {
	if text == "" {
		z.system(gm, "Uso: /anuncio texto")
		return
	}
	z.broadcastAll(&pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_SYSTEM, Text: "[Anúncio] " + text,
	}}})
	z.gmLog(gm, "anuncio", "", text)
}

func (z *Zone) gmOnline(gm *entity) {
	names := make([]string, 0, len(z.players))
	for _, p := range z.players {
		names = append(names, p.name)
	}
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	if z.mgr != nil { // todas as zonas, com o mapa de cada um
		names = z.mgr.bus.names()
	}
	text := fmt.Sprintf("%d online: ", len(names))
	if len(names) > maxOnlineList {
		text += strings.Join(names[:maxOnlineList], ", ") + "…"
	} else {
		text += strings.Join(names, ", ")
	}
	z.system(gm, text)
}

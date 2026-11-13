package zone

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"erevia/server/internal/pb"
)

// Eventos do servidor: XP em dobro (pelo horário, Config.XP2Days, ou por GM) e a invasão
// da cidade (invasion.go). O estado vai para o HUD num EventStatus. O que um GM liga com
// prazo fica salvo no banco (server_events): um deploy no meio do fim de semana não
// desliga a XP em dobro.

const (
	xp2Mult     = 2.0
	xp2MaxHours = 7 * 24
	eventXP2    = "xp2"     // GM: XP em dobro até Until
	eventXP2Off = "xp2_off" // GM desligou: o horário não vale até Until
)

// brasilia: fuso padrão dos dias do horário (o Brasil não tem mais horário de verão).
var brasilia = time.FixedZone("BRT", -3*3600)

// EventData é um evento de GM salvo no banco.
type EventData struct {
	ID    string // eventXP2 ou eventXP2Off
	Until time.Time
}

// EventBackend grava os eventos de GM (implementado com o store).
type EventBackend interface {
	SaveEvent(ctx context.Context, e EventData) error
}

// SetEventBackend liga o banco dos eventos (antes de Run). Sem ele (testes), nada é salvo.
func (z *Zone) SetEventBackend(b EventBackend) { z.eventBackend = b }

type xp2State struct {
	gmUntil  time.Time // GM: ligada até aqui
	offUntil time.Time // GM desligou: o horário fica sem efeito até aqui
	on       bool      // o que o servidor anunciou por último
	until    time.Time // e até quando (zero = sem previsão)
}

func (z *Zone) loadEvents(list []EventData) {
	for _, e := range list {
		switch e.ID {
		case eventXP2:
			z.xp2.gmUntil = e.Until
		case eventXP2Off:
			z.xp2.offUntil = e.Until
		}
	}
	// Na partida não há aviso: quem entra vê o indicador no HUD.
	z.xp2.on, z.xp2.until = z.xp2Now(z.cfg.Clock())
}

func (z *Zone) eventLoc() *time.Location {
	if z.cfg.EventLocation != nil {
		return z.cfg.EventLocation
	}
	return brasilia
}

// xp2Day: o dia de t (no fuso dos eventos) é de XP em dobro pelo horário.
func (z *Zone) xp2Day(t time.Time) bool {
	return slices.Contains(z.cfg.XP2Days, t.In(z.eventLoc()).Weekday())
}

// xp2ScheduleEnd: a meia-noite em que acaba a sequência de dias do horário que contém t
// (zero = todos os dias são de XP em dobro).
func (z *Zone) xp2ScheduleEnd(t time.Time) time.Time {
	l := t.In(z.eventLoc())
	day := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, l.Location())
	for range 7 {
		day = day.AddDate(0, 0, 1)
		if !z.xp2Day(day) {
			return day
		}
	}
	return time.Time{}
}

// xp2Now: a XP em dobro vale em t? E até quando (zero = sem previsão).
func (z *Zone) xp2Now(t time.Time) (bool, time.Time) {
	gm := t.Before(z.xp2.gmUntil)
	sched := z.xp2Day(t) && !t.Before(z.xp2.offUntil)
	switch {
	case gm && sched:
		end := z.xp2ScheduleEnd(t)
		if end.IsZero() || z.xp2.gmUntil.After(end) {
			return true, z.xp2.gmUntil
		}
		return true, end
	case gm:
		return true, z.xp2.gmUntil
	case sched:
		return true, z.xp2ScheduleEnd(t)
	}
	return false, time.Time{}
}

// xpEventMult: multiplicador de eventos na XP de monstros.
func (z *Zone) xpEventMult() float64 {
	if z.xp2.on {
		return xp2Mult
	}
	return 1
}

// updateXP2 confere o horário (1×/s e logo depois de um comando de GM) e avisa todos
// quando a XP em dobro liga ou desliga.
func (z *Zone) updateXP2() {
	on, until := z.xp2Now(z.cfg.Clock())
	if on == z.xp2.on && until.Equal(z.xp2.until) {
		return
	}
	was := z.xp2.on
	z.xp2.on, z.xp2.until = on, until
	switch {
	case on && !was:
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT, "XP em dobro!",
			"Os monstros dão o dobro de experiência "+z.untilText(until)+".")
		z.log.Info("event xp2 on", "until", until)
	case !on && was:
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, "Fim da XP em dobro",
			"A experiência dos monstros voltou ao normal.")
		z.log.Info("event xp2 off")
	}
	z.broadcastEvents()
}

var weekdayNames = [...]string{"domingo", "segunda", "terça", "quarta", "quinta", "sexta", "sábado"}

// untilText: "até as 18:00", "até sábado, 18:00" ou "até o fim de domingo".
func (z *Zone) untilText(t time.Time) string {
	if t.IsZero() {
		return "por tempo indeterminado"
	}
	l := t.In(z.eventLoc())
	now := z.cfg.Clock().In(z.eventLoc())
	if l.Hour() == 0 && l.Minute() == 0 {
		return "até o fim de " + weekdayNames[l.AddDate(0, 0, -1).Weekday()]
	}
	if l.YearDay() == now.YearDay() && l.Year() == now.Year() {
		return l.Format("até as 15:04")
	}
	return "até " + weekdayNames[l.Weekday()] + l.Format(", 15:04")
}

// ---------- Estado no HUD ----------

func (z *Zone) eventStatusPB() *pb.EventStatus {
	out := &pb.EventStatus{Xp2: z.xp2.on, Waves: uint32(len(invasionWaves))}
	if z.xp2.on && !z.xp2.until.IsZero() {
		out.Xp2UntilMs = z.xp2.until.UnixMilli()
	}
	if v := z.inv; v != nil {
		out.Invasion = v.phase
		out.InvasionAtMs = z.wallAt(v.at).UnixMilli()
		out.Wave = uint32(v.wave + 1)
		out.Alive = uint32(len(v.alive))
	}
	return out
}

// wallAt converte um instante da simulação (z.now) para o relógio de parede.
func (z *Zone) wallAt(sim float64) time.Time {
	return z.cfg.Clock().Add(time.Duration((sim - z.now) * float64(time.Second)))
}

func (z *Zone) broadcastEvents() {
	if z.inv != nil {
		z.inv.sentAlive = len(z.inv.alive)
	}
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_EventStatus{EventStatus: z.eventStatusPB()}}, 0)
}

// eventsOnJoin: quem entra recebe o estado (só se houver algum evento).
func (z *Zone) eventsOnJoin(p *entity) {
	if z.xp2.on || z.inv != nil {
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_EventStatus{EventStatus: z.eventStatusPB()}})
	}
}

// ---------- Banco ----------

// saveEvent entrega o evento ao worker sem bloquear a zona.
func (z *Zone) saveEvent(e EventData) {
	select {
	case z.eventJobs <- e:
	default:
		z.log.Error("event save queue full", "event", e.ID)
	}
}

// runEventWorker grava os eventos em ordem, tentando de novo se o banco falhar.
func (z *Zone) runEventWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-z.eventJobs:
			if z.eventBackend == nil {
				continue
			}
			for attempt := 1; attempt <= 3; attempt++ {
				cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				err := z.eventBackend.SaveEvent(cctx, e)
				cancel()
				if err == nil {
					break
				}
				z.log.Error("save event", "err", err, "event", e.ID, "attempt", attempt)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(attempt) * time.Second):
				}
			}
		}
	}
}

// ---------- Configuração ----------

// ParseWeekdays lê os dias da XP em dobro ("sab,dom", "sábado domingo", "sat,sun" ou
// 0–6 com 0 = domingo). "" ou "nenhum" = só por GM.
func ParseWeekdays(s string) ([]time.Weekday, error) {
	names := map[string]time.Weekday{
		"dom": time.Sunday, "domingo": time.Sunday, "sun": time.Sunday,
		"seg": time.Monday, "segunda": time.Monday, "mon": time.Monday,
		"ter": time.Tuesday, "terça": time.Tuesday, "terca": time.Tuesday, "tue": time.Tuesday,
		"qua": time.Wednesday, "quarta": time.Wednesday, "wed": time.Wednesday,
		"qui": time.Thursday, "quinta": time.Thursday, "thu": time.Thursday,
		"sex": time.Friday, "sexta": time.Friday, "fri": time.Friday,
		"sab": time.Saturday, "sáb": time.Saturday, "sábado": time.Saturday, "sabado": time.Saturday, "sat": time.Saturday,
	}
	var out []time.Weekday
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		if f == "nenhum" {
			continue
		}
		d, ok := names[f]
		if !ok {
			n, err := strconv.Atoi(f)
			if err != nil || n < 0 || n > 6 {
				return nil, fmt.Errorf("dia desconhecido: %q", f)
			}
			d = time.Weekday(n)
		}
		if !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	return out, nil
}

// ---------- Comando de GM ----------

const eventUsage = "Uso: /evento invasao [agora|parar] · /evento xp2 horas · /evento xp2 off"

// gmEvent: /evento (estado), /evento invasao [agora|parar], /evento xp2 horas|off.
func (z *Zone) gmEvent(gm *entity, args []string) {
	if len(args) == 0 {
		z.system(gm, z.eventsText()+" "+eventUsage)
		return
	}
	sub := ""
	if len(args) > 1 {
		sub = strings.ToLower(args[1])
	}
	switch strings.ToLower(args[0]) {
	case "invasao", "invasão":
		z.gmInvasion(gm, sub)
	case "xp2":
		z.gmXP2(gm, sub)
	default:
		z.system(gm, eventUsage)
	}
}

func (z *Zone) gmInvasion(gm *entity, sub string) {
	switch sub {
	case "":
		if !z.startInvasion(invWarnBefore, "gm") {
			z.system(gm, "Já há uma invasão anunciada ou em andamento ("+z.invasionText()+").")
			return
		}
		z.system(gm, "Invasão anunciada: as ondas começam em 2 minutos.")
	case "agora":
		switch {
		case z.inv == nil:
			z.startInvasion(0, "gm")
		case z.inv.phase == pb.InvasionPhase_INVASION_PHASE_WARNING:
			z.beginInvasion()
		default:
			z.system(gm, "A invasão já está em andamento.")
			return
		}
		z.system(gm, "Invasão começou.")
	case "parar":
		if z.inv == nil {
			z.system(gm, "Não há invasão.")
			return
		}
		z.stopInvasion()
		z.system(gm, "Invasão interrompida (sem recompensa).")
	default:
		z.system(gm, eventUsage)
		return
	}
	if sub == "" {
		sub = "anunciar"
	}
	z.gmLog(gm, "evento", "invasao", sub)
}

func (z *Zone) gmXP2(gm *entity, sub string) {
	wall := z.cfg.Clock()
	switch {
	case sub == "off":
		z.xp2.gmUntil = wall
		z.xp2.offUntil = time.Time{}
		if z.xp2Day(wall) {
			z.xp2.offUntil = z.xp2ScheduleEnd(wall) // o horário de hoje também fica desligado
			if z.xp2.offUntil.IsZero() {
				z.xp2.offUntil = wall.AddDate(1, 0, 0)
			}
		}
		z.saveEvent(EventData{ID: eventXP2, Until: z.xp2.gmUntil})
		z.saveEvent(EventData{ID: eventXP2Off, Until: z.xp2.offUntil})
		z.updateXP2()
		z.system(gm, "XP em dobro desligada.")
		z.gmLog(gm, "evento", "xp2", "off")
	default:
		h, err := strconv.ParseFloat(strings.ReplaceAll(sub, ",", "."), 64)
		if err != nil || math.IsNaN(h) || h <= 0 || h > xp2MaxHours {
			z.system(gm, fmt.Sprintf("Uso: /evento xp2 horas (até %d) · /evento xp2 off", xp2MaxHours))
			return
		}
		z.xp2.gmUntil = wall.Add(time.Duration(h * float64(time.Hour))).Truncate(time.Minute)
		z.xp2.offUntil = time.Time{}
		z.saveEvent(EventData{ID: eventXP2, Until: z.xp2.gmUntil})
		z.saveEvent(EventData{ID: eventXP2Off, Until: z.xp2.offUntil})
		z.updateXP2()
		z.system(gm, "XP em dobro ligada "+z.untilText(z.xp2.until)+".")
		z.gmLog(gm, "evento", "xp2", fmt.Sprintf("%g h", h))
	}
}

// eventsText: o estado dos eventos para o /evento.
func (z *Zone) eventsText() string {
	xp := "desligada"
	if z.xp2.on {
		xp = "ligada " + z.untilText(z.xp2.until)
	}
	days := "só por GM"
	if len(z.cfg.XP2Days) > 0 {
		var names []string
		for _, d := range z.cfg.XP2Days {
			names = append(names, weekdayNames[d])
		}
		days = strings.Join(names, ", ")
	}
	return fmt.Sprintf("XP em dobro: %s (horário: %s). Invasão: %s (chance por noite: %.0f%%).",
		xp, days, z.invasionText(), z.cfg.InvasionChance*100)
}

package zone

import (
	"context"
	"math"
	"slices"
	"sort"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Operação ao vivo pelo portal (/admin → Operação): anúncios (na hora ou agendados), aviso
// de manutenção, eventos (invasão e XP em dobro) e os chefes. Tudo roda na goroutine da
// zona, como os comandos de GM do chat; a API grava o gm_log.
//
// Os anúncios agendados também ficam no banco (a API grava; o cmd/zone carrega os que
// ainda não saíram com LoadScheduled antes do Run), para sobreviver a um reinício. O aviso
// de manutenção é só da memória: o reinício que ele anuncia o encerra.

const (
	maxScheduled      = 50
	maxMaintenanceMin = 24 * 60
	maxAnnounceTitle  = 60
	maxAnnounceText   = 300
)

// maintenanceMarks: quanto falta (s) quando o aviso de manutenção se repete.
var maintenanceMarks = []float64{60 * 60, 30 * 60, 15 * 60, 10 * 60, 5 * 60, 3 * 60, 2 * 60, 60, 30, 10}

// ScheduledAnnouncement é um anúncio marcado para sair numa hora.
type ScheduledAnnouncement struct {
	ID    int64     `json:"id"`
	At    time.Time `json:"at"`
	Title string    `json:"title"`
	Text  string    `json:"text"`
	By    string    `json:"by"`
}

type opsState struct {
	scheduled []ScheduledAnnouncement // em ordem de At
	maint     *maintenance
}

type maintenance struct {
	at      time.Time // quando o servidor reinicia (relógio de parede)
	message string
	by      string
	marks   int // quantas marcas de maintenanceMarks já foram avisadas
}

// LoadScheduled carrega os anúncios agendados do banco (antes do Run; não é thread-safe).
func (z *Zone) LoadScheduled(list []ScheduledAnnouncement) {
	ops := &z.admin().ops
	ops.scheduled = append(ops.scheduled[:0], list...)
	sortScheduled(ops.scheduled)
}

func sortScheduled(list []ScheduledAnnouncement) {
	sort.SliceStable(list, func(i, j int) bool { return list[i].At.Before(list[j].At) })
}

// updateAdminOps (1×/s): solta os anúncios agendados que venceram e repete o aviso de
// manutenção nas marcas.
func (z *Zone) updateAdminOps() {
	if z.adm == nil {
		return
	}
	ops := &z.adm.ops
	now := z.cfg.Clock()
	for len(ops.scheduled) > 0 && !ops.scheduled[0].At.After(now) {
		a := ops.scheduled[0]
		ops.scheduled = ops.scheduled[1:]
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT, a.Title, a.Text)
		z.log.Info("scheduled announcement", "id", a.ID, "by", a.By)
	}
	m := ops.maint
	if m == nil {
		return
	}
	left := m.at.Sub(now).Seconds()
	if left <= 0 {
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, "Manutenção", "O servidor vai reiniciar agora. Até já!")
		z.log.Info("maintenance reached", "by", m.by)
		ops.maint = nil
		return
	}
	due := m.marks
	for due < len(maintenanceMarks) && left <= maintenanceMarks[due] {
		due++
	}
	if due > m.marks {
		m.marks = due
		z.announceMaintenance(m, left)
	}
}

func (z *Zone) announceMaintenance(m *maintenance, left float64) {
	text := "O servidor reinicia em " + durationText(left) + "."
	if m.message != "" {
		text += " " + m.message
	}
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, "Manutenção", text)
}

// adminOpsOnJoin: quem entra durante a contagem recebe o aviso.
func (z *Zone) adminOpsOnJoin(p *entity) {
	if z.adm == nil || z.adm.ops.maint == nil {
		return
	}
	m := z.adm.ops.maint
	if left := m.at.Sub(z.cfg.Clock()).Seconds(); left > 0 {
		text := "Manutenção: o servidor reinicia em " + durationText(left) + "."
		if m.message != "" {
			text += " " + m.message
		}
		z.system(p, text)
	}
}

// ---------- Anúncios ----------

// cleanAnnounce limpa título e texto (o mesmo saneamento do chat).
func cleanAnnounce(title, text string) (string, string, error) {
	title = truncateRunes(SanitizeChat(title), maxAnnounceTitle)
	text = truncateRunes(SanitizeChat(text), maxAnnounceText)
	if text == "" {
		return "", "", invalid("Escreva o texto do anúncio.")
	}
	if title == "" {
		title = "Anúncio"
	}
	return title, text, nil
}

// AdminAnnounce manda o anúncio agora (banner e linha no chat de todos).
func (z *Zone) AdminAnnounce(ctx context.Context, title, text string) (int, error) {
	title, text, err := cleanAnnounce(title, text)
	if err != nil {
		return 0, err
	}
	return askZoneAction(ctx, z, func(z *Zone) int {
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT, title, text)
		return len(z.players)
	})
}

// AdminSchedule marca um anúncio (a API já o gravou no banco, com o ID).
func (z *Zone) AdminSchedule(ctx context.Context, a ScheduledAnnouncement) error {
	title, text, err := cleanAnnounce(a.Title, a.Text)
	if err != nil {
		return err
	}
	a.Title, a.Text = title, text
	r, err := askZoneAction(ctx, z, func(z *Zone) error {
		ops := &z.admin().ops
		if len(ops.scheduled) >= maxScheduled {
			return invalid("Já há %d anúncios agendados.", maxScheduled)
		}
		if !a.At.After(z.cfg.Clock()) {
			return invalid("Escolha uma hora no futuro.")
		}
		ops.scheduled = append(ops.scheduled, a)
		sortScheduled(ops.scheduled)
		return nil
	})
	if err != nil {
		return err
	}
	return r
}

// AdminUnschedule cancela um anúncio agendado. false: ele não estava na fila (já saiu).
func (z *Zone) AdminUnschedule(ctx context.Context, id int64) (bool, error) {
	return askZoneAction(ctx, z, func(z *Zone) bool {
		ops := &z.admin().ops
		i := slices.IndexFunc(ops.scheduled, func(a ScheduledAnnouncement) bool { return a.ID == id })
		if i < 0 {
			return false
		}
		ops.scheduled = slices.Delete(ops.scheduled, i, i+1)
		return true
	})
}

// ---------- Manutenção ----------

// AdminMaintenance começa (ou troca) a contagem regressiva para o reinício.
func (z *Zone) AdminMaintenance(ctx context.Context, minutes float64, message, by string) (time.Time, error) {
	if math.IsNaN(minutes) || minutes < 1 || minutes > maxMaintenanceMin {
		return time.Time{}, invalid("Minutos inválidos (1 a %d).", maxMaintenanceMin)
	}
	message = truncateRunes(SanitizeChat(message), maxAnnounceText)
	return askZoneAction(ctx, z, func(z *Zone) time.Time {
		now := z.cfg.Clock()
		m := &maintenance{at: now.Add(time.Duration(minutes * float64(time.Minute))).Truncate(time.Second), message: message, by: by}
		left := m.at.Sub(now).Seconds()
		for m.marks < len(maintenanceMarks) && left <= maintenanceMarks[m.marks] {
			m.marks++ // as marcas maiores que o tempo pedido já "passaram"
		}
		z.admin().ops.maint = m
		z.announceMaintenance(m, left)
		z.log.Info("maintenance scheduled", "at", m.at, "by", by)
		return m.at
	})
}

// AdminMaintenanceCancel cancela a contagem. false: não havia.
func (z *Zone) AdminMaintenanceCancel(ctx context.Context) (bool, error) {
	return askZoneAction(ctx, z, func(z *Zone) bool {
		ops := &z.admin().ops
		if ops.maint == nil {
			return false
		}
		ops.maint = nil
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT, "Manutenção cancelada", "O servidor não vai mais reiniciar agora. Bom jogo!")
		return true
	})
}

// ---------- Eventos ----------

// AdminInvasion: "anunciar" (aviso de 2 min), "agora" ou "parar" (sem recompensa).
func (z *Zone) AdminInvasion(ctx context.Context, action string) (string, error) {
	type out struct {
		text string
		err  error
	}
	r, err := askZoneAction(ctx, z, func(z *Zone) out {
		switch action {
		case "anunciar":
			if !z.startInvasion(invWarnBefore, "gm") {
				return out{err: invalid("Já há uma invasão anunciada ou em andamento (%s).", z.invasionText())}
			}
			return out{text: "Invasão anunciada: as ondas começam em 2 minutos."}
		case "agora":
			switch {
			case z.inv == nil:
				z.startInvasion(0, "gm")
			case z.inv.phase == pb.InvasionPhase_INVASION_PHASE_WARNING:
				z.beginInvasion()
			default:
				return out{err: invalid("A invasão já está em andamento.")}
			}
			return out{text: "A invasão começou."}
		case "parar":
			if z.inv == nil {
				return out{err: invalid("Não há invasão.")}
			}
			z.stopInvasion()
			return out{text: "Invasão interrompida (sem recompensa)."}
		}
		return out{err: invalid("Ação desconhecida.")}
	})
	if err != nil {
		return "", err
	}
	return r.text, r.err
}

// AdminXP2 liga a XP em dobro por hours horas (0 = desliga, inclusive o horário de hoje).
func (z *Zone) AdminXP2(ctx context.Context, hours float64) (string, error) {
	if math.IsNaN(hours) || hours < 0 || hours > xp2MaxHours {
		return "", invalid("Horas inválidas (até %d).", xp2MaxHours)
	}
	return askZoneAction(ctx, z, func(z *Zone) string {
		if hours == 0 {
			z.xp2TurnOff()
			return "XP em dobro desligada."
		}
		z.xp2TurnOn(hours)
		return "XP em dobro ligada " + z.untilText(z.xp2.until) + "."
	})
}

// ---------- Chefes ----------

// AdminBossRespawn faz um chefe morto renascer agora. Devolve o nome.
func (z *Zone) AdminBossRespawn(ctx context.Context, id uint32) (string, error) {
	type out struct {
		name string
		err  error
	}
	r, err := askZoneAction(ctx, z, func(z *Zone) out {
		m := z.monsters[id]
		if m == nil || !m.mon.def.Boss || m.mon.summon {
			return out{err: invalid("Chefe não encontrado.")}
		}
		if !m.dead {
			return out{err: invalid("%s já está vivo.", m.mon.def.Name)}
		}
		if r := m.mon.raid; r != nil {
			// Como o /raid nascer: o próximo tick o acorda (com o anúncio de sempre). O banco
			// fica com o renascimento "agora", para um reinício não o esconder de novo.
			m.mon.hidden = true
			r.spawnAt, r.warned = z.cfg.Clock(), true
			z.saveBoss(BossData{ID: m.mon.def.ID, RespawnAt: r.spawnAt, KilledAt: r.killedAt})
		} else {
			m.mon.respawnAt = z.now
		}
		z.log.Info("boss respawn forced", "boss", m.mon.def.ID)
		return out{name: m.mon.def.Name}
	})
	if err != nil {
		return "", err
	}
	return r.name, r.err
}

// ---------- Estado (a página Operação) ----------

type OpsStatus struct {
	Now         time.Time               `json:"now"`
	Online      int                     `json:"online"`
	Night       bool                    `json:"night"`
	NextNight   time.Time               `json:"nextNight"` // próximo anoitecer (chance de invasão)
	NextDay     time.Time               `json:"nextDay"`   // próximo amanhecer
	XP2         OpsXP2                  `json:"xp2"`
	Invasion    OpsInvasion             `json:"invasion"`
	Bosses      []OpsBoss               `json:"bosses"`
	Scheduled   []ScheduledAnnouncement `json:"scheduled"`
	Maintenance *OpsMaintenance         `json:"maintenance"`
}

type OpsXP2 struct {
	On       bool       `json:"on"`
	Until    *time.Time `json:"until"`    // nil = sem previsão
	GMUntil  *time.Time `json:"gmUntil"`  // ligada por GM até
	OffUntil *time.Time `json:"offUntil"` // desligada por GM até (o horário não vale)
	Days     []string   `json:"days"`     // dias do horário (XP2_DAYS)
	// Próxima janela do horário (nil = nenhuma nos próximos 7 dias).
	NextStart *time.Time `json:"nextStart"`
	NextEnd   *time.Time `json:"nextEnd"`
}

type OpsInvasion struct {
	Phase    string     `json:"phase"` // none, warning, active
	Reason   string     `json:"reason"`
	StartsAt *time.Time `json:"startsAt"`
	EndsAt   *time.Time `json:"endsAt"`
	Wave     int        `json:"wave"`
	Waves    int        `json:"waves"`
	Alive    int        `json:"alive"`
	Killed   int        `json:"killed"`
	Chance   float64    `json:"chance"` // por noite (INVASION_CHANCE)
}

type OpsBoss struct {
	ID          uint32     `json:"id"`
	Def         string     `json:"def"`
	Name        string     `json:"name"`
	Level       int        `json:"level"`
	Raid        bool       `json:"raid"`
	Alive       bool       `json:"alive"`
	HPPct       float64    `json:"hpPct"`
	Phase       int        `json:"phase"`
	Enraged     bool       `json:"enraged"`
	Final       bool       `json:"final"`
	Fighting    bool       `json:"fighting"`
	FightStart  *time.Time `json:"fightStart"`
	WindowStart *time.Time `json:"windowStart"`
	WindowEnd   *time.Time `json:"windowEnd"`
	RespawnAt   *time.Time `json:"respawnAt"` // a hora exata (só o portal vê)
	KilledAt    *time.Time `json:"killedAt"`
	X           float64    `json:"x"`
	Z           float64    `json:"z"`
}

type OpsMaintenance struct {
	At      time.Time `json:"at"`
	Message string    `json:"message"`
	By      string    `json:"by"`
}

// AdminOps pergunta à zona o estado da operação.
func (z *Zone) AdminOps(ctx context.Context) (OpsStatus, error) {
	return askZone(ctx, z, func(z *Zone) OpsStatus { return z.opsStatus() })
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (z *Zone) opsStatus() OpsStatus {
	now := z.cfg.Clock()
	out := OpsStatus{Now: now, Online: len(z.players), Night: z.night, Bosses: []OpsBoss{}, Scheduled: []ScheduledAnnouncement{}}
	out.NextNight, out.NextDay = nextPhase(now, game.NightStart), nextPhase(now, 0)

	x := OpsXP2{On: z.xp2.on, Until: timePtr(z.xp2.until), Days: []string{}}
	if z.xp2.gmUntil.After(now) {
		x.GMUntil = timePtr(z.xp2.gmUntil)
	}
	if z.xp2.offUntil.After(now) {
		x.OffUntil = timePtr(z.xp2.offUntil)
	}
	for _, d := range z.cfg.XP2Days {
		x.Days = append(x.Days, weekdayNames[d])
	}
	x.NextStart, x.NextEnd = z.nextXP2Window(now)
	out.XP2 = x

	inv := OpsInvasion{Phase: "none", Waves: len(invasionWaves), Chance: z.cfg.InvasionChance}
	if v := z.inv; v != nil {
		inv.Reason, inv.Killed, inv.Alive = v.reason, v.killed, len(v.alive)
		if v.phase == pb.InvasionPhase_INVASION_PHASE_WARNING {
			inv.Phase, inv.StartsAt = "warning", timePtr(z.wallAt(v.at))
		} else {
			inv.Phase, inv.EndsAt, inv.Wave = "active", timePtr(z.wallAt(v.at)), v.wave+1
		}
	}
	out.Invasion = inv

	for _, m := range z.monsters {
		if m.mon.def.Boss && !m.mon.summon {
			out.Bosses = append(out.Bosses, z.opsBoss(m))
		}
	}
	sort.Slice(out.Bosses, func(i, j int) bool { return out.Bosses[i].ID < out.Bosses[j].ID })

	if z.adm != nil {
		out.Scheduled = append(out.Scheduled, z.adm.ops.scheduled...)
		if m := z.adm.ops.maint; m != nil {
			out.Maintenance = &OpsMaintenance{At: m.at, Message: m.message, By: m.by}
		}
	}
	return out
}

func (z *Zone) opsBoss(m *entity) OpsBoss {
	ms := m.mon
	b := OpsBoss{
		ID: m.id, Def: ms.def.ID, Name: ms.def.Name, Level: m.level, Raid: ms.raid != nil, Alive: !m.dead,
		X: ms.home.X, Z: ms.home.Z,
	}
	if m.dead {
		b.WindowStart, b.WindowEnd = timePtr(ms.windowStart), timePtr(ms.windowEnd)
		if r := ms.raid; r != nil {
			b.RespawnAt, b.KilledAt = timePtr(r.spawnAt), timePtr(r.killedAt)
		} else {
			b.RespawnAt = timePtr(z.wallAt(ms.respawnAt))
		}
		return b
	}
	if m.stats.MaxHP > 0 {
		b.HPPct = math.Round(1000*m.hp/m.stats.MaxHP) / 10
	}
	if r := ms.raid; r != nil {
		b.Phase, b.Enraged, b.Final, b.Fighting = r.phase, r.enraged, r.final, r.fighting
		if r.fighting {
			b.FightStart = timePtr(r.fightWall)
		}
	}
	return b
}

// nextPhase: a próxima vez (depois de now) em que o ciclo dia/noite passa pela fase.
func nextPhase(now time.Time, phase float64) time.Time {
	cur := game.DayPhase(now)
	d := phase - cur
	if d <= 0 {
		d++
	}
	return now.Add(time.Duration(d * float64(game.DayCycle))).Truncate(time.Second)
}

// nextXP2Window: a próxima sequência de dias de XP em dobro pelo horário que ainda vai
// valer (depois de um "desligar" do GM), nos próximos 7 dias.
func (z *Zone) nextXP2Window(now time.Time) (*time.Time, *time.Time) {
	if len(z.cfg.XP2Days) == 0 {
		return nil, nil
	}
	l := now.In(z.eventLoc())
	day := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, l.Location())
	off := z.xp2.offUntil
	for range 8 {
		day = day.AddDate(0, 0, 1)
		prev := day.AddDate(0, 0, -1)
		// Começa uma sequência: o dia vale e o anterior não valia (ou estava desligado).
		if z.xp2Day(day) && !day.Before(off) && (!z.xp2Day(prev) || prev.Before(off)) {
			return timePtr(day), timePtr(z.xp2ScheduleEnd(day))
		}
	}
	return nil, nil
}

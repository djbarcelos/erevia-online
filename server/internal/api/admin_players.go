package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/zone"
)

// Portal de administração, seção Jogadores (tunnel#106): busca, ficha, histórico de
// moderação e as ações (expulsar, silenciar, banir, teleportar, dar e tirar item ou moedas,
// GM). Quem decide onde a ação acontece é a zona (zone.AdminEdit): online e "recente" na
// memória dela; offline de verdade, aqui, numa transação, com o personagem travado pela
// zona até o fim (ver server/internal/zone/admin_actions.go).

// PlayersZone é o que a seção Jogadores pede à zona (o *zone.Zone implementa).
type PlayersZone interface {
	AdminOnline(ctx context.Context) ([]zone.AdminOnlinePlayer, error)
	AdminCharacter(ctx context.Context, id uint32, accountID int64) (zone.AdminLive, error)
	AdminEdit(ctx context.Context, id uint32, e zone.AdminEdit) (zone.AdminActionResult, error)
	AdminRelease(ctx context.Context, id uint32, hold uint64) error
	AdminKick(ctx context.Context, id uint32, reason string) (string, error)
	AdminAccountChanged(ctx context.Context, c zone.AccountChange) ([]string, error)
}

const (
	adminTxTimeout = 8 * time.Second // transação offline (a trava da zona dura 30 s)
	maxReasonRunes = 200
	maxMuteMinutes = 7 * 24 * 60 // o mesmo limite do /mute
	maxCoord       = 10000.0
)

func (a *API) registerAdminPlayers(m *http.ServeMux) {
	m.HandleFunc("GET /api/admin/players", a.handleAdminPlayers)
	m.HandleFunc("GET /api/admin/players/{id}", a.handleAdminPlayer)
	m.HandleFunc("GET /api/admin/items", a.handleAdminItems)
	m.HandleFunc("POST /api/admin/players/{id}/kick", a.handleAdminKick)
	m.HandleFunc("POST /api/admin/players/{id}/mute", a.handleAdminMute)
	m.HandleFunc("POST /api/admin/players/{id}/unmute", a.handleAdminUnmute)
	m.HandleFunc("POST /api/admin/players/{id}/ban", a.handleAdminBan)
	m.HandleFunc("POST /api/admin/players/{id}/unban", a.handleAdminUnban)
	m.HandleFunc("POST /api/admin/players/{id}/gm", a.handleAdminSetGM)
	m.HandleFunc("POST /api/admin/players/{id}/teleport", a.handleAdminTeleport)
	m.HandleFunc("POST /api/admin/players/{id}/item", a.handleAdminItem)
	m.HandleFunc("POST /api/admin/players/{id}/gold", a.handleAdminGold)
}

func (a *API) playersZone() PlayersZone {
	if a.admin == nil {
		return nil
	}
	z, _ := a.admin.cfg.Zone.(PlayersZone)
	return z
}

// zoneError responde o erro de uma ação pedida à zona.
func (a *API) zoneError(w http.ResponseWriter, err error) {
	var inv *zone.AdminInvalid
	switch {
	case errors.As(err, &inv):
		writeError(w, http.StatusBadRequest, inv.Msg)
	case errors.Is(err, zone.ErrNotOnline):
		writeError(w, http.StatusConflict, "O personagem não está online.")
	case errors.Is(err, zone.ErrCharacterBusy):
		writeError(w, http.StatusConflict, "Outra ação neste personagem ainda está terminando. Tente de novo em instantes.")
	case errors.Is(err, zone.ErrZoneBusy):
		writeError(w, http.StatusServiceUnavailable, "A zona não respondeu: nada foi feito. Tente de novo.")
	case errors.Is(err, zone.ErrZoneUnconfirmed):
		writeError(w, http.StatusGatewayTimeout, "A zona recebeu o pedido mas não confirmou a tempo. Confira a ficha antes de repetir.")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusBadRequest, "Personagem não encontrado.")
	default:
		a.internalError(w, err)
	}
}

func charID(r *http.Request) (uint32, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	return uint32(id), err == nil && id > 0
}

func cleanReason(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > maxReasonRunes {
		s = string([]rune(s)[:maxReasonRunes])
	}
	return s
}

// ---------- Busca e ficha ----------

type playerRow struct {
	store.PlayerHit
	Online bool   `json:"online"`
	Region string `json:"region,omitempty"`
}

// handleAdminPlayers: sem q, quem está online; com q, a busca no banco (online e offline).
func (a *API) handleAdminPlayers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(q) > 64 {
		writeError(w, http.StatusBadRequest, "Busca longa demais.")
		return
	}
	var online []zone.AdminOnlinePlayer
	zoneOK := false
	if pz := a.playersZone(); pz != nil {
		if list, err := pz.AdminOnline(r.Context()); err == nil {
			online, zoneOK = list, true
		} else {
			a.log.Warn("admin online list", "err", err)
		}
	}
	byID := map[int64]zone.AdminOnlinePlayer{}
	for _, p := range online {
		byID[int64(p.ID)] = p
	}
	rows := []playerRow{}
	if q == "" {
		for _, p := range online {
			rows = append(rows, playerRow{PlayerHit: store.PlayerHit{
				ID: int64(p.ID), Name: p.Name, Class: p.Class, Level: p.Level, AccountID: p.AccountID, Identities: []store.Identity{},
			}, Online: true, Region: p.Region})
		}
	} else {
		hits, err := a.store.SearchPlayers(r.Context(), q)
		if err != nil {
			a.internalError(w, err)
			return
		}
		for _, h := range hits {
			row := playerRow{PlayerHit: h}
			if p, ok := byID[h.ID]; ok {
				row.Online, row.Region, row.Level = true, p.Region, p.Level
			}
			rows = append(rows, row)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"players": rows, "zone": zoneOK, "online": len(online)})
}

type itemOut struct {
	Slot    int    `json:"slot"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Count   int    `json:"count"`
	Enchant int    `json:"enchant"`
	Grade   int    `json:"grade"`
	Where   string `json:"where,omitempty"` // equipado: o encaixe
}

func itemsOut(list []game.ItemRecord) (equip, bag []itemOut) {
	equip, bag = []itemOut{}, []itemOut{}
	for _, it := range list {
		o := itemOut{Slot: it.Slot, ID: it.ItemID, Name: it.ItemID, Count: it.Count, Enchant: it.Enchant}
		if d := game.Items[it.ItemID]; d != nil {
			o.Name, o.Grade = d.Name, int(d.Grade)
		}
		if it.Equipped {
			if it.Slot >= 0 && it.Slot < game.EquipSlots {
				o.Where = game.SlotNames[it.Slot]
			}
			equip = append(equip, o)
		} else {
			bag = append(bag, o)
		}
	}
	slices.SortFunc(equip, func(x, y itemOut) int { return x.Slot - y.Slot })
	slices.SortFunc(bag, func(x, y itemOut) int { return x.Slot - y.Slot })
	return equip, bag
}

type questOut struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Progress string `json:"progress,omitempty"`
}

func questsOut(st game.QuestState) (active, done []questOut) {
	active, done = []questOut{}, []questOut{}
	for id, prog := range st.Active {
		q := questOut{ID: id, Title: id}
		if d := game.Quests[id]; d != nil {
			q.Title = d.Title
			var parts []string
			for i, s := range d.Steps {
				n := 0
				if i < len(prog) {
					n = prog[i]
				}
				parts = append(parts, fmt.Sprintf("%s %d/%d", s.Text, min(n, s.Count), s.Count))
			}
			q.Progress = strings.Join(parts, " · ")
		}
		active = append(active, q)
	}
	slices.SortFunc(active, func(x, y questOut) int { return strings.Compare(x.Title, y.Title) })
	for _, id := range st.Done {
		q := questOut{ID: id, Title: id}
		if d := game.Quests[id]; d != nil {
			q.Title = d.Title
		}
		done = append(done, q)
	}
	return active, done
}

type achOut struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Title string    `json:"title"`
	At    time.Time `json:"at"`
}

func achievementsOut(st game.AchievementState) []achOut {
	out := []achOut{}
	for id, ms := range st.Unlocked {
		x := achOut{ID: id, Name: id, At: time.UnixMilli(ms).UTC()}
		if d := game.Achievements[id]; d != nil {
			x.Name, x.Title = d.Name, d.Title
		}
		out = append(out, x)
	}
	slices.SortFunc(out, func(x, y achOut) int { return x.At.Compare(y.At) })
	return out
}

// handleAdminPlayer: a ficha. O banco dá a base; o que a zona tem em memória (online ou
// recente) vale mais e substitui nível, XP, moedas, itens, posição, missões e conquistas.
func (a *API) handleAdminPlayer(w http.ResponseWriter, r *http.Request) {
	id, ok := charID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "Personagem inválido.")
		return
	}
	sh, err := a.store.PlayerSheet(r.Context(), int64(id))
	if err != nil {
		a.zoneError(w, err)
		return
	}
	c := sh.Character
	var live zone.AdminLive
	zoneOK := false
	if pz := a.playersZone(); pz != nil {
		if l, err := pz.AdminCharacter(r.Context(), id, c.AccountID); err == nil {
			live, zoneOK = l, true
		} else {
			a.log.Warn("admin character", "err", err)
		}
	}
	sessions, events, err := a.store.CharacterEvents(r.Context(), int64(id))
	if err != nil {
		a.internalError(w, err)
		return
	}
	history, err := a.store.ModerationHistory(r.Context(), c.AccountID)
	if err != nil {
		a.internalError(w, err)
		return
	}

	source := "banco"
	level, xp, gold, items, x, z := c.Level, c.XP, c.Gold, c.Items, c.X, c.Z
	hp, mp, maxHP, maxMP, dead := c.HP, c.MP, 0, 0, false
	karma, pks, pvp := c.Karma, c.PKs, c.PvPKills
	quests, ach, title, spec := c.Quests, c.Achievements, c.Title, c.Spec
	gm, muted := sh.Account.GM, sh.Account.MutedUntil
	if live.Online || live.Recent {
		source = "zona"
		if live.Recent {
			source = "recente"
		}
		level, xp, gold, items, x, z = live.Level, live.XP, live.Gold, live.Items, live.X, live.Z
		hp, mp, maxHP, maxMP, dead = live.HP, live.MP, live.MaxHP, live.MaxMP, live.Dead
		karma, pks, pvp = live.Karma, live.PKs, live.PvPKills
		quests, ach, title, spec = live.Quests, live.Achievements, live.Title, live.Spec
	}
	if live.Online {
		gm = live.GM
		muted = nil
		if live.MutedUntil.After(time.Now()) {
			mu := live.MutedUntil
			muted = &mu
		}
	}
	equip, bag := itemsOut(items)
	storageGold, storageItems, storageSource := c.Storage.Gold, c.Storage.Items, "banco"
	if live.Storage != nil {
		storageGold, storageItems, storageSource = live.Storage.Gold, live.Storage.Items, "zona"
	}
	_, storageList := itemsOut(storageItems)
	active, done := questsOut(quests)

	var clan any
	switch {
	case live.Clan != nil:
		clan = map[string]any{"id": live.Clan.ID, "name": live.Clan.Name, "rank": live.Clan.Rank}
	case sh.Clan != nil:
		rank := map[int]string{store.ClanRankLeader: "líder", store.ClanRankOfficer: "oficial"}[sh.Clan.Rank]
		if rank == "" {
			rank = "membro"
		}
		clan = map[string]any{"id": sh.Clan.ID, "name": sh.Clan.Name, "rank": rank}
	}
	var specOut any
	if d := game.SpecFor(c.Class, spec); d != nil {
		specOut = map[string]string{"id": d.ID, "name": d.Name, "color": d.Color}
	}
	party := live.Party
	if party == nil {
		party = []string{}
	}
	className := c.Class
	if d := game.Classes[c.Class]; d != nil {
		className = d.Name
	}
	account := sh.Account
	account.GM, account.MutedUntil = gm, muted

	writeJSON(w, http.StatusOK, map[string]any{
		"id": c.ID, "name": c.Name, "class": c.Class, "className": className,
		"level": level, "xp": xp, "xpLevel": game.XPLevelStart(level), "xpNext": game.XPToNext(level),
		"gold": gold, "x": x, "z": z, "region": zone.RegionAt(x, z),
		"hp": hp, "mp": mp, "maxHp": maxHP, "maxMp": maxMP, "dead": dead,
		"karma": karma, "pks": pks, "pvpKills": pvp,
		"title": game.TitleOf(title), "spec": specOut,
		"online": live.Online, "recent": live.Recent, "held": live.Held, "zone": zoneOK, "source": source,
		"joinedAt": nullTime(live.JoinedAt),
		"equipped": equip, "bag": bag,
		"storage": map[string]any{"gold": storageGold, "items": storageList, "source": storageSource},
		"clan":    clan, "party": party,
		"quests":       map[string]any{"active": active, "done": done},
		"achievements": achievementsOut(ach),
		"account":      account,
		"createdAt":    sh.CreatedAt, "lastSeenAt": sh.LastSeenAt,
		"sessions": sessions, "events": events, "history": history,
	})
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// handleAdminItems: o catálogo para o "dar item".
func (a *API) handleAdminItems(w http.ResponseWriter, r *http.Request) {
	type item struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Grade int    `json:"grade"`
		Stack int    `json:"stack"`
	}
	out := []item{}
	for _, id := range game.ItemIDs() {
		d := game.Items[id]
		if d.Kind == game.KindGold {
			continue
		}
		out = append(out, item{ID: d.ID, Name: d.Name, Grade: int(d.Grade), Stack: d.Stack})
	}
	slices.SortFunc(out, func(x, y item) int { return strings.Compare(x.Name, y.Name) })
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// ---------- Ações ----------

// target resolve o personagem do pedido (conta e nome) e confere que a zona existe.
func (a *API) target(w http.ResponseWriter, r *http.Request) (uint32, int64, string, PlayersZone, bool) {
	id, ok := charID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "Personagem inválido.")
		return 0, 0, "", nil, false
	}
	pz := a.playersZone()
	if pz == nil {
		writeError(w, http.StatusServiceUnavailable, "A zona não está ligada ao portal.")
		return 0, 0, "", nil, false
	}
	acc, name, err := a.store.CharacterOwner(r.Context(), int64(id))
	if err != nil {
		a.zoneError(w, err)
		return 0, 0, "", nil, false
	}
	return id, acc, name, pz, true
}

// auditFailed anota no gm_log que a ação registrada antes não aconteceu (ou não se sabe).
func (a *API) auditFailed(r *http.Request, command, target string, err error) {
	why := "erro"
	var inv *zone.AdminInvalid
	switch {
	case errors.As(err, &inv):
		why = inv.Msg
	case errors.Is(err, zone.ErrZoneUnconfirmed):
		why = "não confirmado pela zona"
	case errors.Is(err, zone.ErrZoneBusy):
		why = "zona não respondeu"
	case errors.Is(err, zone.ErrNotOnline):
		why = "não está online"
	case errors.Is(err, zone.ErrCharacterBusy):
		why = "personagem ocupado"
	}
	a.audit(r, command+":falhou", target, why)
}

func (a *API) handleAdminKick(w http.ResponseWriter, r *http.Request) {
	var body struct{ Reason string }
	if !readJSON(w, r, &body) {
		return
	}
	id, _, name, pz, ok := a.target(w, r)
	if !ok {
		return
	}
	reason := cleanReason(body.Reason)
	if err := a.audit(r, "kick", name, reason); err != nil {
		a.internalError(w, err)
		return
	}
	if _, err := pz.AdminKick(r.Context(), id, reason); err != nil {
		a.auditFailed(r, "kick", name, err)
		a.zoneError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": name + " foi desconectado."})
}

// accountAction grava no banco a mudança da conta e depois pede à zona que aplique em quem
// está online. Se a zona não responder, o banco já vale: no próximo login (e o Join que
// estiver em andamento é recusado assim que a zona processar a mudança... ou, sem ela,
// carrega do banco já mudado).
func (a *API) accountAction(w http.ResponseWriter, r *http.Request, command, detail string, ch zone.AccountChange,
	write func(ctx context.Context, acc int64) error, done string) {
	_, acc, name, pz, ok := a.target(w, r)
	if !ok {
		return
	}
	self := adminFrom(r.Context()).accountID
	if acc == self && (ch.Ban || (ch.SetGM && !ch.GM)) {
		writeError(w, http.StatusBadRequest, "Você não pode fazer isso com a sua própria conta.")
		return
	}
	if err := a.audit(r, command, name, detail); err != nil {
		a.internalError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), adminTxTimeout)
	defer cancel()
	if err := write(ctx, acc); err != nil {
		a.auditFailed(r, command, name, err)
		a.internalError(w, err)
		return
	}
	ch.AccountID = acc
	names, err := pz.AdminAccountChanged(ctx, ch)
	msg := strings.ReplaceAll(done, "{name}", name)
	if err != nil {
		a.log.Warn("admin account change: zone", "err", err, "command", command)
		a.audit(r, command+":zona", name, "gravado no banco; a zona não confirmou (vale no próximo login)")
		msg += " A zona não confirmou: quem estiver online só sente a mudança ao relogar."
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg, "online": names})
}

func (a *API) handleAdminMute(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Minutes int
		Reason  string
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Minutes < 1 || body.Minutes > maxMuteMinutes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Minutos inválidos (1 a %d).", maxMuteMinutes))
		return
	}
	reason := cleanReason(body.Reason)
	until := time.Now().Add(time.Duration(body.Minutes) * time.Minute).Truncate(time.Second)
	a.accountAction(w, r, "mute", strings.TrimSpace(fmt.Sprintf("%d min %s", body.Minutes, reason)),
		zone.AccountChange{Mute: true, MutedUntil: until, Reason: reason},
		func(ctx context.Context, acc int64) error { return a.store.SetMute(ctx, acc, until) },
		fmt.Sprintf("{name} silenciado por %d min.", body.Minutes))
}

func (a *API) handleAdminUnmute(w http.ResponseWriter, r *http.Request) {
	var body struct{ Reason string }
	if !readJSON(w, r, &body) {
		return
	}
	a.accountAction(w, r, "unmute", cleanReason(body.Reason), zone.AccountChange{Mute: true},
		func(ctx context.Context, acc int64) error { return a.store.SetMute(ctx, acc, time.Time{}) },
		"{name} pode falar de novo.")
}

func (a *API) handleAdminBan(w http.ResponseWriter, r *http.Request) {
	var body struct{ Reason string }
	if !readJSON(w, r, &body) {
		return
	}
	reason := cleanReason(body.Reason)
	if reason == "" {
		writeError(w, http.StatusBadRequest, "Escreva o motivo do banimento.")
		return
	}
	a.accountAction(w, r, "ban", reason, zone.AccountChange{Ban: true, Reason: reason},
		func(ctx context.Context, acc int64) error { return a.store.SetAccountBanned(ctx, acc, true, reason) },
		"A conta de {name} foi banida.")
}

func (a *API) handleAdminUnban(w http.ResponseWriter, r *http.Request) {
	var body struct{ Reason string }
	if !readJSON(w, r, &body) {
		return
	}
	reason := cleanReason(body.Reason)
	if reason == "" {
		writeError(w, http.StatusBadRequest, "Escreva o motivo do desbanimento.")
		return
	}
	a.accountAction(w, r, "unban", reason, zone.AccountChange{},
		func(ctx context.Context, acc int64) error { return a.store.SetAccountBanned(ctx, acc, false, "") },
		"A conta de {name} foi desbanida.")
}

func (a *API) handleAdminSetGM(w http.ResponseWriter, r *http.Request) {
	var body struct{ GM bool }
	if !readJSON(w, r, &body) {
		return
	}
	cmd, done := "gm:off", "A conta de {name} deixou de ser GM."
	if body.GM {
		cmd, done = "gm:on", "A conta de {name} agora é GM."
	}
	a.accountAction(w, r, cmd, "", zone.AccountChange{SetGM: true, GM: body.GM},
		func(ctx context.Context, acc int64) error { return a.store.SetGM(ctx, acc, body.GM) }, done)
}

// edit faz uma edição de personagem onde ele estiver: a zona aplica (online ou recente) ou
// trava o personagem e devolve "offline", e então a transação acontece aqui.
func (a *API) edit(w http.ResponseWriter, r *http.Request, command, detail string, e zone.AdminEdit) {
	id, acc, name, pz, ok := a.target(w, r)
	if !ok {
		return
	}
	if err := a.audit(r, command, name, detail); err != nil {
		a.internalError(w, err)
		return
	}
	e.AccountID = acc
	res, err := pz.AdminEdit(r.Context(), id, e)
	if err != nil {
		a.auditFailed(r, command, name, err)
		a.zoneError(w, err)
		return
	}
	if res.Where == zone.WhereOffline {
		res.Result, err = a.editOffline(r, pz, id, res)
		if err != nil {
			a.auditFailed(r, command, name, err)
			a.zoneError(w, err)
			return
		}
	}
	where := map[string]string{
		zone.WhereOnline: "online", zone.WhereRecent: "na memória da zona (saiu há pouco)", zone.WhereOffline: "no banco (offline)",
	}[res.Where]
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "where": res.Where, "result": res.Result.Text,
		"message": fmt.Sprintf("%s: %s (%s).", name, res.Result.Text, where),
	})
}

// editOffline: a transação no banco, com a trava da zona. A trava é solta sempre (deu
// certo ou não); a zona anota a hora e recusa um Join carregado antes.
func (a *API) editOffline(r *http.Request, pz PlayersZone, id uint32, res zone.AdminActionResult) (zone.AdminEditResult, error) {
	base := context.WithoutCancel(r.Context()) // o navegador fechar não deixa a transação pela metade
	defer func() {
		rctx, cancel := context.WithTimeout(base, 5*time.Second)
		defer cancel()
		if err := pz.AdminRelease(rctx, id, res.Hold); err != nil {
			a.log.Warn("admin release hold", "err", err, "id", id) // vence sozinha em 30 s
		}
	}()
	ctx, cancel := context.WithTimeout(base, adminTxTimeout)
	defer cancel()
	var out zone.AdminEditResult
	_, err := a.store.EditOfflineCharacter(ctx, int64(id), time.Now().UnixNano(), func(c *store.OfflineChar) ([]store.GameEvent, error) {
		st := zone.OfflineState{Gold: c.Gold, Items: c.Items, X: c.X, Z: c.Z}
		r, events, err := zone.ApplyOfflineEdit(&st, res.Edit)
		if err != nil {
			return nil, err
		}
		out = r
		c.Gold, c.Items, c.X, c.Z, c.ItemsChanged = st.Gold, st.Items, st.X, st.Z, st.ItemsChanged
		ev := make([]store.GameEvent, len(events))
		for i, e := range events {
			ev[i] = store.GameEvent{Kind: e.Kind, Ref: e.Ref, Amount: e.Amount, Extra: e.Extra}
		}
		return ev, nil
	})
	return out, err
}

func (a *API) handleAdminTeleport(w http.ResponseWriter, r *http.Request) {
	var body struct{ X, Z float64 }
	if !readJSON(w, r, &body) {
		return
	}
	if math.Abs(body.X) > maxCoord || math.Abs(body.Z) > maxCoord {
		writeError(w, http.StatusBadRequest, "Coordenadas fora do mapa.")
		return
	}
	a.edit(w, r, "tp", fmt.Sprintf("%.0f %.0f", body.X, body.Z), zone.AdminEdit{Kind: zone.EditTeleport, X: body.X, Z: body.Z})
}

func (a *API) handleAdminItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Action string // give | take
		Item   string
		Count  int
		Reason string
	}
	if !readJSON(w, r, &body) {
		return
	}
	kind, cmd := zone.EditGiveItem, "give"
	switch body.Action {
	case "give":
	case "take":
		kind, cmd = zone.EditTakeItem, "take"
	default:
		writeError(w, http.StatusBadRequest, "Ação inválida (give ou take).")
		return
	}
	item := strings.TrimSpace(body.Item)
	if game.Items[item] == nil {
		writeError(w, http.StatusBadRequest, "Item desconhecido.")
		return
	}
	detail := strings.TrimSpace(fmt.Sprintf("%s %d %s", item, body.Count, cleanReason(body.Reason)))
	a.edit(w, r, cmd, detail, zone.AdminEdit{Kind: kind, Item: item, Count: body.Count})
}

func (a *API) handleAdminGold(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Amount int64 // > 0 dá, < 0 tira
		Reason string
	}
	if !readJSON(w, r, &body) {
		return
	}
	cmd := "give"
	if body.Amount < 0 {
		cmd = "take"
	}
	detail := strings.TrimSpace(fmt.Sprintf("gold %d %s", body.Amount, cleanReason(body.Reason)))
	a.edit(w, r, cmd, detail, zone.AdminEdit{Kind: zone.EditGold, Gold: body.Amount})
}

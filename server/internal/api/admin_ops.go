package api

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"erevia/server/internal/zone"
)

// Portal de administração, seção Operação (tunnel#110): anúncios (na hora ou agendados),
// aviso de manutenção, eventos (invasão e XP em dobro) e os chefes. Tudo é feito pela zona
// (pelo canal, como os comandos de GM do chat) e registrado no gm_log.

// OpsZone é o que a seção Operação pede à zona (o *zone.Zone implementa).
type OpsZone interface {
	AdminOps(ctx context.Context) (zone.OpsStatus, error)
	AdminAnnounce(ctx context.Context, title, text string) (int, error)
	AdminSchedule(ctx context.Context, a zone.ScheduledAnnouncement) error
	AdminUnschedule(ctx context.Context, id int64) (bool, error)
	AdminMaintenance(ctx context.Context, minutes float64, message, by string) (time.Time, error)
	AdminMaintenanceCancel(ctx context.Context) (bool, error)
	AdminInvasion(ctx context.Context, action string) (string, error)
	AdminXP2(ctx context.Context, hours float64) (string, error)
	AdminBossRespawn(ctx context.Context, id uint32) (string, error)
}

const (
	maxScheduleAhead = 30 * 24 * time.Hour
	minScheduleAhead = 30 * time.Second
)

func (a *API) registerAdminOps(m *http.ServeMux) {
	m.HandleFunc("GET /api/admin/ops", a.handleAdminOps)
	m.HandleFunc("POST /api/admin/ops/announce", a.handleAdminAnnounce)
	m.HandleFunc("POST /api/admin/ops/announce/{id}/cancel", a.handleAdminUnschedule)
	m.HandleFunc("POST /api/admin/ops/maintenance", a.handleAdminMaintenance)
	m.HandleFunc("POST /api/admin/ops/maintenance/cancel", a.handleAdminMaintenanceCancel)
	m.HandleFunc("POST /api/admin/ops/invasion", a.handleAdminInvasion)
	m.HandleFunc("POST /api/admin/ops/xp2", a.handleAdminXP2)
	m.HandleFunc("POST /api/admin/ops/bosses/{id}/respawn", a.handleAdminBossRespawn)
}

func (a *API) opsZone(w http.ResponseWriter) OpsZone {
	var z OpsZone
	if a.admin != nil {
		z, _ = a.admin.cfg.Zone.(OpsZone)
	}
	if z == nil {
		writeError(w, http.StatusServiceUnavailable, "A zona não está ligada ao portal.")
	}
	return z
}

func (a *API) gmName(r *http.Request) string {
	id := adminFrom(r.Context()).accountID
	name, err := a.store.DisplayName(r.Context(), id)
	if err != nil || name == "" {
		return "conta " + strconv.FormatInt(id, 10)
	}
	return name
}

func (a *API) handleAdminOps(w http.ResponseWriter, r *http.Request) {
	z := a.opsZone(w)
	if z == nil {
		return
	}
	st, err := z.AdminOps(r.Context())
	if err != nil {
		a.zoneError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// handleAdminAnnounce: sem "at", o anúncio sai agora; com "at" (ISO 8601), fica agendado
// (no banco, para sobreviver a um reinício, e na fila da zona).
func (a *API) handleAdminAnnounce(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title, Text string
		At          string
	}
	if !readJSON(w, r, &body) {
		return
	}
	z := a.opsZone(w)
	if z == nil {
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, "Escreva o texto do anúncio.")
		return
	}
	title := strings.TrimSpace(body.Title)
	if body.At == "" {
		if err := a.audit(r, "anuncio", "", strings.TrimSpace(title+" · "+text)); err != nil {
			a.internalError(w, err)
			return
		}
		n, err := z.AdminAnnounce(r.Context(), title, text)
		if err != nil {
			a.auditFailed(r, "anuncio", "", err)
			a.zoneError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": fmt.Sprintf("Anúncio enviado para %d jogador(es) online.", n)})
		return
	}
	at, err := time.Parse(time.RFC3339, body.At)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Hora inválida.")
		return
	}
	now := time.Now()
	if at.Before(now.Add(minScheduleAhead)) || at.After(now.Add(maxScheduleAhead)) {
		writeError(w, http.StatusBadRequest, "Agende entre 30 segundos e 30 dias a partir de agora.")
		return
	}
	at = at.Truncate(time.Second)
	if err := a.audit(r, "anuncio:agendar", "", strings.TrimSpace(at.UTC().Format(time.RFC3339)+" · "+title+" · "+text)); err != nil {
		a.internalError(w, err)
		return
	}
	by := a.gmName(r)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), adminTxTimeout)
	defer cancel()
	id, err := a.store.AddScheduledAnnouncement(ctx, at, title, text, adminFrom(r.Context()).accountID, by)
	if err != nil {
		a.auditFailed(r, "anuncio:agendar", "", err)
		a.internalError(w, err)
		return
	}
	if err := z.AdminSchedule(ctx, zone.ScheduledAnnouncement{ID: id, At: at, Title: title, Text: text, By: by}); err != nil {
		// Sem a zona, o agendado não sai: desfaz no banco (para um reinício não o soltar).
		if _, cerr := a.store.CancelScheduledAnnouncement(ctx, id); cerr != nil {
			a.log.Error("cancel scheduled after zone error", "err", cerr, "id", id)
		}
		a.auditFailed(r, "anuncio:agendar", "", err)
		a.zoneError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "message": "Anúncio agendado."})
}

func (a *API) handleAdminUnschedule(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if !readJSON(w, r, &body) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Anúncio inválido.")
		return
	}
	z := a.opsZone(w)
	if z == nil {
		return
	}
	if err := a.audit(r, "anuncio:cancelar", "", strconv.FormatInt(id, 10)); err != nil {
		a.internalError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), adminTxTimeout)
	defer cancel()
	// Banco primeiro: se a zona não responder, o anúncio ainda pode sair agora, mas não
	// volta depois de um reinício.
	if _, err := a.store.CancelScheduledAnnouncement(ctx, id); err != nil {
		a.auditFailed(r, "anuncio:cancelar", "", err)
		a.internalError(w, err)
		return
	}
	found, err := z.AdminUnschedule(ctx, id)
	if err != nil {
		a.auditFailed(r, "anuncio:cancelar", "", err)
		a.zoneError(w, err)
		return
	}
	msg := "Anúncio cancelado."
	if !found {
		msg = "Esse anúncio não estava mais na fila (já saiu ou foi cancelado)."
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

func (a *API) handleAdminMaintenance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Minutes float64
		Message string
	}
	if !readJSON(w, r, &body) {
		return
	}
	z := a.opsZone(w)
	if z == nil {
		return
	}
	if math.IsNaN(body.Minutes) || body.Minutes < 1 || body.Minutes > 24*60 {
		writeError(w, http.StatusBadRequest, "Minutos inválidos (1 a 1440).")
		return
	}
	msg := cleanReason(body.Message)
	if err := a.audit(r, "manutencao", "", strings.TrimSpace(fmt.Sprintf("%g min %s", body.Minutes, msg))); err != nil {
		a.internalError(w, err)
		return
	}
	at, err := z.AdminMaintenance(r.Context(), body.Minutes, msg, a.gmName(r))
	if err != nil {
		a.auditFailed(r, "manutencao", "", err)
		a.zoneError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "at": at, "message": "Aviso de manutenção enviado a todos."})
}

func (a *API) handleAdminMaintenanceCancel(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if !readJSON(w, r, &body) {
		return
	}
	z := a.opsZone(w)
	if z == nil {
		return
	}
	if err := a.audit(r, "manutencao:cancelar", "", ""); err != nil {
		a.internalError(w, err)
		return
	}
	found, err := z.AdminMaintenanceCancel(r.Context())
	if err != nil {
		a.auditFailed(r, "manutencao:cancelar", "", err)
		a.zoneError(w, err)
		return
	}
	msg := "Manutenção cancelada: todos foram avisados."
	if !found {
		msg = "Não havia aviso de manutenção."
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

func (a *API) handleAdminInvasion(w http.ResponseWriter, r *http.Request) {
	var body struct{ Action string }
	if !readJSON(w, r, &body) {
		return
	}
	switch body.Action {
	case "anunciar", "agora", "parar":
	default:
		writeError(w, http.StatusBadRequest, "Ação inválida (anunciar, agora ou parar).")
		return
	}
	z := a.opsZone(w)
	if z == nil {
		return
	}
	if err := a.audit(r, "evento", "invasao", body.Action); err != nil {
		a.internalError(w, err)
		return
	}
	msg, err := z.AdminInvasion(r.Context(), body.Action)
	if err != nil {
		a.auditFailed(r, "evento", "invasao", err)
		a.zoneError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

func (a *API) handleAdminXP2(w http.ResponseWriter, r *http.Request) {
	var body struct{ Hours float64 }
	if !readJSON(w, r, &body) {
		return
	}
	if math.IsNaN(body.Hours) || body.Hours < 0 || body.Hours > 168 {
		writeError(w, http.StatusBadRequest, "Horas inválidas (até 168; 0 desliga).")
		return
	}
	z := a.opsZone(w)
	if z == nil {
		return
	}
	detail := "off"
	if body.Hours > 0 {
		detail = fmt.Sprintf("%g h", body.Hours)
	}
	if err := a.audit(r, "evento", "xp2", detail); err != nil {
		a.internalError(w, err)
		return
	}
	msg, err := z.AdminXP2(r.Context(), body.Hours)
	if err != nil {
		a.auditFailed(r, "evento", "xp2", err)
		a.zoneError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

func (a *API) handleAdminBossRespawn(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if !readJSON(w, r, &body) {
		return
	}
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || id == 0 {
		writeError(w, http.StatusBadRequest, "Chefe inválido.")
		return
	}
	z := a.opsZone(w)
	if z == nil {
		return
	}
	if err := a.audit(r, "chefe:renascer", strconv.FormatUint(id, 10), ""); err != nil {
		a.internalError(w, err)
		return
	}
	name, err := z.AdminBossRespawn(r.Context(), uint32(id))
	if err != nil {
		a.auditFailed(r, "chefe:renascer", strconv.FormatUint(id, 10), err)
		a.zoneError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": name + " vai renascer agora."})
}

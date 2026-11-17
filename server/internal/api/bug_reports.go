package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg" // image.DecodeConfig reconhece JPEG
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"erevia/server/internal/store"
	"erevia/server/internal/zone"
)

// /bug completo (client/src/ui/bug-report.ts):
//
//  1. POST /api/bug-reports {text, characterId, context} → 201 {"id": N}. A zona confere
//     o personagem online da conta, aplica o intervalo do /bug (o mesmo do chat) e carimba
//     nome, nível, posição e versão; a API grava o relato com o contexto (bug_reports.context).
//  2. POST /api/bug-reports/{id}/shot, corpo image/jpeg → 204. Só a conta dona do relato,
//     um print por relato, até BugShotWindow depois do relato.
//
// O portal lê o print em GET /api/admin/bug-reports/{id}/shot (atrás do AdminOnly).
//
// Por que o HTTP faz o relato inteiro (e não o chat responder com o id): o id volta
// direto na resposta, sem mudar o protocolo do jogo nem casar mensagens do chat com
// pedidos HTTP, e quem é o dono do relato é a sessão do cookie, a mesma que autoriza o
// print. O /bug pelo chat continua valendo (clientes antigos, bots), só sem contexto.

// Limites do /bug completo.
const (
	// Texto do relato, em caracteres (o chat corta em 200).
	BugTextMaxRunes = 1000
	// Contexto do cliente: um objeto JSON de até 32 KB.
	BugContextMaxBytes = 32 << 10
	// Print: JPEG de até 100 KB e 1280 px no lado maior (o cliente manda ~640 px, < 80 KB).
	BugShotMaxBytes = 100 << 10
	BugShotMaxSide  = 1280
	// Prazo para anexar o print depois do relato.
	BugShotWindow = 10 * time.Minute

	bugBodyMax = BugContextMaxBytes + 8<<10
	// Por conta: 3 relatos (e 3 prints) de uma vez, depois 1 a cada 30 s. A zona também
	// aplica os 30 s entre dois /bug do mesmo personagem.
	bugBurst   = 3
	bugPerSec  = 1.0 / 30
	bugTimeout = 5 * time.Second
)

// BugZone é o que a API pergunta à zona para um /bug (zone.Zone.PrepareBug).
type BugZone interface {
	PrepareBug(ctx context.Context, accountID, characterID int64, text string) (zone.BugReport, error)
}

type bugState struct {
	zone   BugZone
	create *limiter
	shots  *limiter
}

// SetBugZone liga o /bug completo à zona (antes de servir). Sem ela, POST /api/bug-reports
// responde 503 e o cliente cai no /bug do chat.
func (a *API) SetBugZone(z BugZone) {
	a.bugs().zone = z
}

func (a *API) bugs() *bugState {
	if a.bug == nil {
		a.bug = &bugState{create: newLimiter(bugPerSec, bugBurst), shots: newLimiter(bugPerSec, bugBurst)}
	}
	return a.bug
}

func (a *API) registerBugReports(mux *http.ServeMux) {
	a.bugs()
	mux.HandleFunc("POST /api/bug-reports", a.handleCreateBugReport)
	mux.HandleFunc("POST /api/bug-reports/{id}/shot", a.handleBugShot)
}

// registerAdminBugReports: rotas do portal sobre os relatos (já atrás do AdminOnly).
func (a *API) registerAdminBugReports(m *http.ServeMux) {
	m.HandleFunc("GET /api/admin/bug-reports/{id}/shot", a.handleAdminBugShot)
}

func (a *API) handleCreateBugReport(w http.ResponseWriter, r *http.Request) {
	if !a.checkPost(w, r) {
		return
	}
	accountID, ok := a.requireAccount(w, r)
	if !ok {
		return
	}
	b := a.bugs()
	if !bugAllow(w, b.create, accountID) {
		return
	}
	var body struct {
		Text        string          `json:"text"`
		CharacterID int64           `json:"characterId"`
		Context     json.RawMessage `json:"context"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, bugBodyMax)).Decode(&body); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "Relato grande demais.")
			return
		}
		writeError(w, http.StatusBadRequest, "JSON inválido.")
		return
	}
	text := cutRunes(bugText(body.Text), BugTextMaxRunes)
	bugCtx, ok := bugContext(body.Context)
	if !ok {
		writeError(w, http.StatusBadRequest, "Contexto inválido.")
		return
	}
	if b.zone == nil {
		writeError(w, http.StatusServiceUnavailable, zone.BugErrorText(nil))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), bugTimeout)
	defer cancel()
	rep, err := b.zone.PrepareBug(ctx, accountID, body.CharacterID, text)
	if err != nil {
		var cd *zone.BugCooldownError
		switch {
		case errors.As(err, &cd):
			w.Header().Set("Retry-After", strconv.Itoa(cd.Seconds))
			writeError(w, http.StatusTooManyRequests, zone.BugErrorText(err))
		case errors.Is(err, zone.ErrBugTooShort):
			writeError(w, http.StatusBadRequest, zone.BugErrorText(err))
		case errors.Is(err, zone.ErrBugNotOnline):
			writeError(w, http.StatusConflict, zone.BugErrorText(err))
		default:
			a.log.Warn("bug report zone", "err", err)
			writeError(w, http.StatusServiceUnavailable, zone.BugErrorText(err))
		}
		return
	}
	id, err := a.store.CreateBugReport(context.WithoutCancel(ctx), store.BugReport{
		AccountID: rep.AccountID, CharacterID: int64(rep.CharacterID), CharacterName: rep.Name, Level: rep.Level,
		X: rep.X, Z: rep.Z, Version: rep.Version, UserAgent: rep.UserAgent, Text: rep.Text, Context: bugCtx,
	})
	if err != nil {
		a.internalError(w, err)
		return
	}
	a.log.Info("bug report", "id", id, "account", accountID, "context", len(bugCtx))
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "shotWindowSec": int(BugShotWindow.Seconds())})
}

// handleBugShot recebe o print (JPEG cru) do relato {id}, só da conta dona.
func (a *API) handleBugShot(w http.ResponseWriter, r *http.Request) {
	// image/jpeg não é um tipo "simples": de outra origem, o navegador faria o preflight de
	// CORS, que o servidor não libera. Mesmo assim confere a origem, como no checkPost.
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mt != "image/jpeg" {
		writeError(w, http.StatusUnsupportedMediaType, "Envie um JPEG.")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host {
			writeError(w, http.StatusForbidden, "Origem não permitida.")
			return
		}
	}
	accountID, ok := a.requireAccount(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "Relato não encontrado.")
		return
	}
	if !bugAllow(w, a.bugs().shots, accountID) {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, BugShotMaxBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "Print grande demais.")
			return
		}
		writeError(w, http.StatusBadRequest, "Print incompleto.")
		return
	}
	width, height, ok := jpegSize(data)
	if !ok {
		writeError(w, http.StatusBadRequest, "O print não é um JPEG válido.")
		return
	}
	err = a.store.AttachBugShot(r.Context(), id, accountID, store.BugShot{Width: width, Height: height, JPEG: data}, BugShotWindow)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "Relato não encontrado.")
	case errors.Is(err, store.ErrShotExists):
		writeError(w, http.StatusConflict, "Este relato já tem um print.")
	case err != nil:
		a.internalError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleAdminBugShot entrega o print de um relato ao portal (só GM, pelo AdminOnly).
func (a *API) handleAdminBugShot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		NotFound(w, r)
		return
	}
	shot, err := a.store.BugReportShot(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		NotFound(w, r)
		return
	}
	if err != nil {
		a.internalError(w, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/jpeg")
	h.Set("Content-Length", strconv.Itoa(len(shot.JPEG)))
	h.Set("Content-Disposition", `inline; filename="relato-`+strconv.FormatInt(id, 10)+`.jpg"`)
	h.Set("Last-Modified", shot.At.UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		w.Write(shot.JPEG)
	}
}

// bugAllow gasta uma ficha do limite da conta; sem ficha, responde 429 com Retry-After.
func bugAllow(w http.ResponseWriter, l *limiter, accountID int64) bool {
	ok, wait := l.allow(accountID)
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "Muitos relatos. Espere um pouco.")
	}
	return ok
}

// bugText limpa o texto do relato: sem caracteres de controle (fora quebra de linha e
// tab), sem o byte nulo, sem espaço sobrando nas pontas.
func bugText(s string) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// bugContext aceita ausente/null (relato sem contexto) ou um objeto JSON de até
// BugContextMaxBytes. O Postgres recusa \u0000 em jsonb.
func bugContext(raw json.RawMessage) ([]byte, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, true
	}
	if len(raw) > BugContextMaxBytes || raw[0] != '{' || !json.Valid(raw) || bytes.Contains(raw, []byte(`\u0000`)) || !utf8.Valid(raw) {
		return nil, false
	}
	// Compacta (o cliente pode mandar com espaços): o que vai ao banco é o mesmo objeto.
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// jpegSize confere que data é um JPEG (assinatura e cabeçalho) e devolve o tamanho dele.
func jpegSize(data []byte) (int, int, bool) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 || data[2] != 0xFF {
		return 0, 0, false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "jpeg" || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > BugShotMaxSide || cfg.Height > BugShotMaxSide {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

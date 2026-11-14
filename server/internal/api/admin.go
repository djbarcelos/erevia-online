package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"erevia/server/internal/store"
	"erevia/server/internal/zone"
)

// Portal de administração: a página /admin (servida pelo cmd/zone) e a API /api/admin/*.
//
// Regras, todas aplicadas por um middleware único (AdminOnly) antes de qualquer rota:
//   - só passa sessão válida de conta GM (accounts.gm) e não banida; o resto (deslogado,
//     não-GM, banido) recebe o mesmo 404 de um caminho que não existe, para não revelar
//     que o portal existe;
//   - pedido que muda algo (qualquer método fora GET/HEAD/OPTIONS) exige o cabeçalho
//     X-Erevia-Admin: 1, JSON e a mesma origem (checkPost), contra CSRF;
//   - limite de pedidos por conta (leitura e escrita em baldes separados);
//   - respostas com Cache-Control: no-store, X-Frame-Options: DENY e CSP;
//   - toda ação que muda algo vai para o gm_log (audit), com quem, o quê e em quem.

// AdminHeader é o cabeçalho que o portal manda em todo pedido de escrita. Um site de
// outra origem não consegue mandá-lo sem passar pelo preflight de CORS, que o servidor
// não libera.
const AdminHeader = "X-Erevia-Admin"

const (
	maxAuditDetail = 500 // runas do campo detail do gm_log
	gmLogLimit     = 50
)

// AdminZone é o que o portal pergunta à zona (sempre pelo canal, com prazo).
type AdminZone interface {
	Status(ctx context.Context) (zone.AdminStatus, error)
}

// AdminConfig liga o portal (antes de Register). Os limites zerados usam o padrão.
type AdminConfig struct {
	Version   string    // commit no ar (RAILWAY_GIT_COMMIT_SHA)
	StartedAt time.Time // quando o servidor subiu (uptime)
	Zone      AdminZone // nil: "online agora" fica vazio

	ReadPerSec, ReadBurst   float64 // padrão: 2/s, 60 de uma vez
	WritePerSec, WriteBurst float64 // padrão: 0,5/s, 10 de uma vez
}

type adminState struct {
	cfg    AdminConfig
	reads  *limiter
	writes *limiter
	mux    *http.ServeMux
}

// SetAdmin configura o portal de administração (antes de Register).
func (a *API) SetAdmin(cfg AdminConfig) {
	def := func(v *float64, d float64) {
		if *v <= 0 {
			*v = d
		}
	}
	def(&cfg.ReadPerSec, 2)
	def(&cfg.ReadBurst, 60)
	def(&cfg.WritePerSec, 0.5)
	def(&cfg.WriteBurst, 10)
	if cfg.StartedAt.IsZero() {
		cfg.StartedAt = time.Now()
	}
	a.admin = &adminState{
		cfg:    cfg,
		reads:  newLimiter(cfg.ReadPerSec, cfg.ReadBurst),
		writes: newLimiter(cfg.WritePerSec, cfg.WriteBurst),
		mux:    http.NewServeMux(),
	}
}

// registerAdmin monta as rotas do portal, todas atrás do AdminOnly. Um prefixo só no mux
// principal: método errado também cai no 404 para quem não é GM (rotas com método no mux
// principal dariam 405 e revelariam a rota).
func (a *API) registerAdmin(mux *http.ServeMux) {
	if a.admin == nil {
		a.SetAdmin(AdminConfig{})
	}
	m := a.admin.mux
	m.HandleFunc("GET /api/admin/me", a.handleAdminMe)
	m.HandleFunc("GET /api/admin/overview", a.handleAdminOverview)
	m.HandleFunc("GET /api/admin/gm-log", a.handleAdminGMLog)
	a.registerAdminBugReports(m)
	m.HandleFunc("GET /api/admin/progression", a.handleAdminProgression)
	mux.Handle("/api/admin/", a.AdminOnly(m))
}

// NotFound é o 404 de qualquer caminho que não existe. O portal usa o mesmo para quem não
// é GM, então a resposta não diz se /admin existe.
func NotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.NotFound(w, r)
}

type adminCtxKey struct{}

// adminRequest é o GM do pedido, guardado no contexto pelo AdminOnly.
type adminRequest struct {
	accountID int64
	audited   bool
}

func adminFrom(ctx context.Context) *adminRequest {
	ar, _ := ctx.Value(adminCtxKey{}).(*adminRequest)
	return ar
}

func mutating(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

// AdminOnly protege a API e a página do portal (o cmd/zone embrulha /admin e os assets
// dele com isto).
func (a *API) AdminOnly(next http.Handler) http.Handler {
	if a.admin == nil {
		a.SetAdmin(AdminConfig{})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookie)
		if err != nil || c.Value == "" {
			NotFound(w, r)
			return
		}
		id, err := a.store.GMBySession(r.Context(), c.Value)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				a.log.Error("admin auth", "err", err) // sem o token: só o erro do banco
			}
			NotFound(w, r)
			return
		}

		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")

		write := mutating(r.Method)
		lim := a.admin.reads
		if write {
			lim = a.admin.writes
		}
		if ok, wait := lim.allow(id); !ok {
			h.Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			writeError(w, http.StatusTooManyRequests, "Muitos pedidos. Espere um pouco.")
			return
		}
		if write {
			if r.Header.Get(AdminHeader) != "1" {
				a.log.Warn("admin write without header", "account", id, "method", r.Method, "path", r.URL.Path)
				writeError(w, http.StatusForbidden, "Pedido sem o cabeçalho do portal.")
				return
			}
			if !a.checkPost(w, r) {
				return
			}
		}

		ar := &adminRequest{accountID: id}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		r = r.WithContext(context.WithValue(r.Context(), adminCtxKey{}, ar))
		next.ServeHTTP(rec, r)
		if !write {
			return
		}
		// Só método, rota e resultado: nada de corpo, query ou cookie no log.
		a.log.Info("admin action", "account", id, "method", r.Method, "path", r.URL.Path, "status", rec.status)
		if rec.status < 400 && !ar.audited {
			// Rede de segurança: uma rota que mudou algo sem chamar audit ainda fica no gm_log.
			a.log.Warn("admin action without audit", "method", r.Method, "path", r.URL.Path)
			a.audit(r, "unaudited "+r.Method+" "+r.URL.Path, "", "")
		}
	})
}

// audit grava no gm_log uma ação do portal: quem (a conta GM e o nome dela), o quê
// (command, com o prefixo "portal:") e em quem (target). Toda rota de escrita chama antes
// de responder; se falhar, a rota não deve seguir com a ação.
func (a *API) audit(r *http.Request, command, target, detail string) error {
	ar := adminFrom(r.Context())
	if ar == nil {
		return errors.New("audit outside the admin portal")
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()
	name, err := a.store.DisplayName(ctx, ar.accountID)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(detail) > maxAuditDetail {
		detail = string([]rune(detail)[:maxAuditDetail])
	}
	err = a.store.LogGM(ctx, store.GMEntry{
		GMAccountID: ar.accountID, GMName: name, Command: "portal:" + command, Target: target, Detail: detail,
	})
	if err != nil {
		a.log.Error("admin audit", "err", err, "account", ar.accountID, "command", command)
		return err
	}
	ar.audited = true
	return nil
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status, s.wrote = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	s.wrote = true
	return s.ResponseWriter.Write(b)
}

// Unwrap deixa o http.ResponseController achar o writer de verdade.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// ---------- Rotas ----------

func (a *API) handleAdminMe(w http.ResponseWriter, r *http.Request) {
	id := adminFrom(r.Context()).accountID
	name, err := a.store.DisplayName(r.Context(), id)
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accountId": id, "displayName": name})
}

// handleAdminOverview: a página inicial (online agora, versão, uptime e o resumo do banco).
func (a *API) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	sum, err := a.store.AdminSummary(r.Context())
	if err != nil {
		a.internalError(w, err)
		return
	}
	var online *int // null: a zona não respondeu no prazo
	if z := a.admin.cfg.Zone; z != nil {
		if st, err := z.Status(r.Context()); err == nil {
			online = &st.Online
		} else {
			a.log.Warn("admin zone status", "err", err)
		}
	}
	now := time.Now()
	writeJSON(w, http.StatusOK, map[string]any{
		"online":     online,
		"version":    a.admin.cfg.Version,
		"startedAt":  a.admin.cfg.StartedAt.UTC(),
		"uptimeSec":  int64(now.Sub(a.admin.cfg.StartedAt).Seconds()),
		"serverTime": now.UTC(),
		"summary":    sum,
	})
}

// handleAdminGMLog: as últimas ações de GM, do jogo e do portal.
func (a *API) handleAdminGMLog(w http.ResponseWriter, r *http.Request) {
	limit := gmLogLimit
	if v, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit"))); err == nil && v > 0 && v < gmLogLimit {
		limit = v
	}
	rows, err := a.store.RecentGMLog(r.Context(), limit)
	if err != nil {
		a.internalError(w, err)
		return
	}
	if rows == nil {
		rows = []store.GMLogRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": rows})
}

// handleAdminProgression: as views da progressão (docs/progressao.md), só leitura.
func (a *API) handleAdminProgression(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	tables, err := a.store.ProgressionReport(ctx)
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tables": tables})
}

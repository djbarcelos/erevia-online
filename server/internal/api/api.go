// Package api expõe a API HTTP: login (OAuth e dev), sessão e personagens.
package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/zone"
)

const (
	SessionCookie   = "erevia_session"
	stateCookieBase = "erevia_oauth_"
	maxBodyBytes    = 4 << 10
)

var devUserRe = regexp.MustCompile(`^[A-Za-z0-9]{3,16}$`)

type Config struct {
	PublicURL string // ex.: https://erevia.up.railway.app (sem barra no fim)
	DevLogin  bool   // habilita POST /api/auth/dev (NUNCA em produção)
	Google    *provider
	Discord   *provider
}

type API struct {
	store     *store.Store
	cfg       Config
	log       *slog.Logger
	providers map[string]*provider
	secure    bool
	// onAccountDeleted avisa a zona que uma conta sumiu (derruba quem estiver online).
	onAccountDeleted func(store.DeletedAccount)
	// errLimit: lotes de erros do cliente por IP e por conta (POST /api/client-errors).
	errLimit *rateLimiter
}

// OnAccountDeleted registra quem é avisado depois que uma conta é apagada (antes de servir).
func (a *API) OnAccountDeleted(fn func(store.DeletedAccount)) { a.onAccountDeleted = fn }

func New(st *store.Store, cfg Config, log *slog.Logger) *API {
	a := &API{store: st, cfg: cfg, log: log, providers: map[string]*provider{},
		errLimit: newRateLimiter(ClientErrorsPerMinute, clientErrorRateWindow)}
	for _, p := range []*provider{cfg.Google, cfg.Discord} {
		if p != nil {
			a.providers[p.name] = p
		}
	}
	a.secure = strings.HasPrefix(cfg.PublicURL, "https://")
	return a
}

func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/providers", a.handleProviders)
	mux.HandleFunc("GET /api/auth/{provider}/login", a.handleOAuthLogin)
	mux.HandleFunc("GET /api/auth/{provider}/callback", a.handleOAuthCallback)
	mux.HandleFunc("POST /api/auth/dev", a.handleDevLogin)
	mux.HandleFunc("POST /api/auth/logout", a.handleLogout)
	mux.HandleFunc("GET /api/me", a.handleMe)
	mux.HandleFunc("DELETE /api/account", a.handleDeleteAccount)
	mux.HandleFunc("GET /api/characters", a.handleListCharacters)
	mux.HandleFunc("POST /api/characters", a.handleCreateCharacter)
	mux.HandleFunc("GET /api/creation", a.handleCreation)
	mux.HandleFunc("POST /api/client-errors", a.handleClientErrors)
}

// Authenticate devolve a conta da sessão no cookie (também usado pelo WebSocket).
func (a *API) Authenticate(r *http.Request) (int64, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil || c.Value == "" {
		return 0, store.ErrNotFound
	}
	return a.store.AccountBySession(r.Context(), c.Value)
}

// ---------- Login ----------

func (a *API) handleProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"google":  a.providers["google"] != nil,
		"discord": a.providers["discord"] != nil,
		"dev":     a.cfg.DevLogin,
	})
}

func (a *API) handleOAuthLogin(w http.ResponseWriter, r *http.Request) {
	p := a.providers[r.PathValue("provider")]
	if p == nil {
		writeError(w, http.StatusNotFound, "Login indisponível.")
		return
	}
	state := randomString(24)
	verifier := oauth2.GenerateVerifier()
	http.SetCookie(w, &http.Cookie{
		Name: stateCookieBase + p.name, Value: state + "." + verifier,
		Path: "/api/auth/" + p.name, MaxAge: 600,
		HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, p.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

func (a *API) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	p := a.providers[r.PathValue("provider")]
	if p == nil {
		writeError(w, http.StatusNotFound, "Login indisponível.")
		return
	}
	fail := func(reason string, err error) {
		a.log.Warn("oauth callback failed", "provider", p.name, "reason", reason, "err", err)
		http.Redirect(w, r, "/?login_error="+url.QueryEscape(reason), http.StatusFound)
	}

	// Apaga o cookie de state de qualquer jeito.
	http.SetCookie(w, &http.Cookie{
		Name: stateCookieBase + p.name, Value: "", Path: "/api/auth/" + p.name, MaxAge: -1,
		HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
	})

	if e := r.URL.Query().Get("error"); e != "" {
		fail("cancelado", errors.New(e))
		return
	}
	c, err := r.Cookie(stateCookieBase + p.name)
	if err != nil {
		fail("sessao_expirada", err)
		return
	}
	state, verifier, ok := strings.Cut(c.Value, ".")
	got := r.URL.Query().Get("state")
	if !ok || subtle.ConstantTimeCompare([]byte(state), []byte(got)) != 1 {
		fail("state_invalido", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tok, err := p.oauth.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		fail("troca_falhou", err)
		return
	}
	subject, displayName, err := p.fetchUser(ctx, p.oauth.Client(ctx, tok))
	if err == nil && subject == "" {
		err = errNoSubject
	}
	if err != nil {
		fail("perfil_falhou", err)
		return
	}
	if err := a.startSession(w, r.Context(), p.name, subject, displayName); err != nil {
		fail("erro_interno", err)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *API) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.DevLogin {
		writeError(w, http.StatusNotFound, "Não encontrado.")
		return
	}
	if !a.checkPost(w, r) {
		return
	}
	var body struct {
		Username string `json:"username"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if !devUserRe.MatchString(body.Username) {
		writeError(w, http.StatusBadRequest, "Usuário inválido: 3 a 16 letras ou números.")
		return
	}
	if err := a.startSession(w, r.Context(), "dev", strings.ToLower(body.Username), body.Username); err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) startSession(w http.ResponseWriter, ctx context.Context, provider, subject, displayName string) error {
	accountID, err := a.store.LoginIdentity(ctx, provider, subject, displayName)
	if err != nil {
		return err
	}
	token, err := a.store.CreateSession(ctx, accountID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: token, Path: "/",
		MaxAge:   int(store.SessionTTL.Seconds()),
		HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
	})
	a.log.Info("login", "provider", provider, "account", accountID)
	return nil
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !a.checkPost(w, r) {
		return
	}
	if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
		if err := a.store.DeleteSession(r.Context(), c.Value); err != nil {
			a.internalError(w, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	accountID, ok := a.requireAccount(w, r)
	if !ok {
		return
	}
	name, err := a.store.DisplayName(r.Context(), accountID)
	if err != nil {
		a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accountId": accountID, "displayName": name})
}

// AccountDeleteConfirm é o que o jogador digita para confirmar que quer apagar a conta.
const AccountDeleteConfirm = "APAGAR"

// handleDeleteAccount apaga a conta e tudo o que é dela (LGPD, art. 18), encerra a sessão
// e derruba da zona quem estiver jogando com ela.
func (a *API) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if !a.checkPost(w, r) {
		return
	}
	accountID, ok := a.requireAccount(w, r)
	if !ok {
		return
	}
	var body struct {
		Confirm string `json:"confirm"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Confirm) != AccountDeleteConfirm {
		writeError(w, http.StatusBadRequest, "Digite "+AccountDeleteConfirm+" para confirmar.")
		return
	}
	d, err := a.store.DeleteAccount(r.Context(), accountID)
	if err != nil && !errors.Is(err, store.ErrNotFound) { // não existe mais: já foi apagada
		a.internalError(w, err)
		return
	}
	if err == nil {
		a.log.Info("account deleted", "account", accountID, "characters", len(d.Characters), "clans", len(d.Clans))
		if a.onAccountDeleted != nil {
			a.onAccountDeleted(d)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------- Personagens ----------

func (a *API) handleListCharacters(w http.ResponseWriter, r *http.Request) {
	accountID, ok := a.requireAccount(w, r)
	if !ok {
		return
	}
	chars, err := a.store.ListCharacters(r.Context(), accountID)
	if err != nil {
		a.internalError(w, err)
		return
	}
	if chars == nil {
		chars = []store.Character{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"characters": chars, "max": store.MaxCharsPerAccount})
}

func (a *API) handleCreateCharacter(w http.ResponseWriter, r *http.Request) {
	if !a.checkPost(w, r) {
		return
	}
	accountID, ok := a.requireAccount(w, r)
	if !ok {
		return
	}
	var body struct {
		Name  string     `json:"name"`
		Class string     `json:"class"`
		Look  *game.Look `json:"look"` // opcional: sem ele, a aparência padrão da classe
	}
	if !readJSON(w, r, &body) {
		return
	}
	if !zone.ValidName(body.Name) {
		writeError(w, http.StatusBadRequest, "Nome inválido: use 3 a 16 letras ou números.")
		return
	}
	if !store.ValidClass(body.Class) {
		writeError(w, http.StatusBadRequest, "Classe inválida.")
		return
	}
	look := game.DefaultLook(body.Class)
	if body.Look != nil {
		if !game.ValidLook(body.Class, *body.Look) {
			writeError(w, http.StatusBadRequest, "Aparência inválida.")
			return
		}
		look = *body.Look
	}
	c, err := a.store.CreateCharacter(r.Context(), accountID, body.Name, body.Class, look)
	switch {
	case errors.Is(err, store.ErrNameTaken):
		writeError(w, http.StatusConflict, "Esse nome já está em uso.")
	case errors.Is(err, store.ErrTooManyCharacters):
		writeError(w, http.StatusConflict, "Você já tem o máximo de personagens.")
	case err != nil:
		a.internalError(w, err)
	default:
		writeJSON(w, http.StatusCreated, c)
	}
}

// creationOption é uma arma ou escudo oferecido na criação (id "" = mão vazia).
type creationOption struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Model    string `json:"model"`
	OffModel string `json:"offModel,omitempty"`
}

type creationClass struct {
	Weapons  []creationOption `json:"weapons"`
	Offhands []creationOption `json:"offhands"`
	Emblem   string           `json:"emblem"`
}

// creationOptions é montado uma vez (dados fixos do jogo).
var creationOptions = func() map[string]any {
	opt := func(id string) creationOption {
		if id == "" {
			return creationOption{Name: "Nada"}
		}
		d := game.Items[id]
		return creationOption{ID: id, Name: d.Name, Model: d.Model, OffModel: d.OffModel}
	}
	classes := map[string]creationClass{}
	for c, o := range game.StarterLooks {
		cc := creationClass{Emblem: o.Emblem}
		for _, id := range o.Weapons {
			cc.Weapons = append(cc.Weapons, opt(id))
		}
		for _, id := range o.Offhands {
			cc.Offhands = append(cc.Offhands, opt(id))
		}
		classes[c] = cc
	}
	hats := []string{}
	for c := range game.HatClasses {
		hats = append(hats, c)
	}
	return map[string]any{
		"classes": classes, "emblems": game.Emblems,
		// Visual do corpo: índice 0 = original da classe, i = lista[i-1].
		"skins": game.SkinTones, "hairs": game.HairColors, "outfits": game.OutfitColors,
		"heads": game.Heads, "hatClasses": hats, "barberPrice": game.BarberPrice,
	}
}()

// handleCreation: opções de aparência (armas iniciais, escudos, cores e visual do corpo).
func (a *API) handleCreation(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, creationOptions)
}

// ---------- Utilitários ----------

func (a *API) requireAccount(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := a.Authenticate(r)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "Faça login.")
		return 0, false
	}
	if err != nil {
		a.internalError(w, err)
		return 0, false
	}
	return id, true
}

// checkPost barra CSRF: exige JSON (força preflight em outra origem) e Origin igual ao host.
func (a *API) checkPost(w http.ResponseWriter, r *http.Request) bool {
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Envie JSON.")
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host {
			writeError(w, http.StatusForbidden, "Origem não permitida.")
			return false
		}
	}
	return true
}

func (a *API) internalError(w http.ResponseWriter, err error) {
	a.log.Error("api", "err", err)
	writeError(w, http.StatusInternalServerError, "Erro interno. Tente de novo.")
}

func readJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido.")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func randomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

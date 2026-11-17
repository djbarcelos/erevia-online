package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"erevia/server/internal/store"
)

// Limites de POST /api/client-errors.
const (
	// Corpo do lote: 20 erros com mensagem e stack cheias cabem com folga.
	ClientErrorsMaxBody = 128 << 10
	// Erros por lote (o resto é ignorado).
	ClientErrorsMaxBatch = 20
	// Mensagem (em caracteres) e stack (em bytes): o que passar é cortado.
	ClientErrorMaxMessage = 500
	ClientErrorMaxStack   = 4 << 10
	// Lotes por minuto, por IP e por conta.
	ClientErrorsPerMinute = 30

	clientErrorMaxExtra   = 2 << 10
	clientErrorMaxCount   = 10000
	clientErrorMaxText    = 64 // versão, tipo e região
	clientErrorMaxUA      = 400
	clientErrorRateWindow = time.Minute
)

// clientErrorBatch é o corpo de POST /api/client-errors (ver client/src/net/errors.ts).
// Campos desconhecidos são ignorados: um cliente mais novo pode mandar mais coisas.
type clientErrorBatch struct {
	Version string          `json:"version"`
	Errors  []clientErrorIn `json:"errors"`
}

type clientErrorIn struct {
	Kind    string          `json:"kind"`
	Message string          `json:"message"`
	Stack   string          `json:"stack"`
	Count   int64           `json:"count"`
	Ctx     *clientErrorCtx `json:"ctx"`
	Extra   json.RawMessage `json:"extra"`
}

// clientErrorCtx: onde o jogo estava na última vez que o erro aconteceu (só com o jogo
// rodando).
type clientErrorCtx struct {
	CharacterID int64    `json:"characterId"`
	Region      string   `json:"region"`
	X           *float64 `json:"x"`
	Z           *float64 `json:"z"`
	FPS         *float64 `json:"fps"`
	Ping        *float64 `json:"ping"`
}

// handleClientErrors recebe os erros do navegador em lote. Vale com sessão ou sem (tela de
// login). O navegador manda com fetch (JSON) ou, ao fechar a aba, com sendBeacon, que
// envia o mesmo JSON como text/plain.
func (a *API) handleClientErrors(w http.ResponseWriter, r *http.Request) {
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mt != "application/json" && mt != "text/plain" {
		writeError(w, http.StatusUnsupportedMediaType, "Envie JSON.")
		return
	}
	// text/plain não força preflight; a origem, quando vem (todo navegador manda num POST),
	// tem de ser a nossa. O cookie de sessão é SameSite=Lax e nem vai num POST de outro site.
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host {
			writeError(w, http.StatusForbidden, "Origem não permitida.")
			return
		}
	}
	if !a.errLimit.allow("ip:" + clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "Muitos relatórios. Espere um pouco.")
		return
	}
	accountID, err := a.Authenticate(r)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		a.internalError(w, err)
		return
	}
	if accountID != 0 && !a.errLimit.allow("acc:"+strconv.FormatInt(accountID, 10)) {
		writeError(w, http.StatusTooManyRequests, "Muitos relatórios. Espere um pouco.")
		return
	}

	var batch clientErrorBatch
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, ClientErrorsMaxBody)).Decode(&batch); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "Lote grande demais.")
			return
		}
		writeError(w, http.StatusBadRequest, "JSON inválido.")
		return
	}

	rows := clientErrorRows(batch, accountID, cut(cleanText(r.UserAgent()), clientErrorMaxUA))
	if err := a.store.RecordClientErrors(r.Context(), rows); err != nil {
		a.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clientErrorRows valida, corta e junta (mesmo hash) os erros do lote.
func clientErrorRows(batch clientErrorBatch, accountID int64, userAgent string) []store.ClientError {
	version := cut(cleanText(batch.Version), clientErrorMaxText)
	list := batch.Errors
	if len(list) > ClientErrorsMaxBatch {
		list = list[:ClientErrorsMaxBatch]
	}
	byHash := map[string]int{}
	var out []store.ClientError
	for _, in := range list {
		msg := cutRunes(cleanText(in.Message), ClientErrorMaxMessage)
		if strings.TrimSpace(msg) == "" {
			continue
		}
		stack := cut(cleanText(in.Stack), ClientErrorMaxStack)
		kind := cut(cleanText(in.Kind), clientErrorMaxText)
		if kind == "" {
			kind = "error"
		}
		e := store.ClientError{
			Hash: store.ClientErrorHash(msg, stack), Version: version, Kind: kind, Message: msg, Stack: stack,
			Count: min(max(in.Count, 1), clientErrorMaxCount), UserAgent: userAgent, AccountID: accountID,
		}
		if c := in.Ctx; c != nil {
			if accountID != 0 {
				e.CharacterID = max(c.CharacterID, 0)
			}
			e.Region = cut(cleanText(c.Region), clientErrorMaxText)
			e.X, e.Z = finite32(c.X, 1e6), finite32(c.Z, 1e6)
			e.FPS = finite32(c.FPS, 1000)
			if p := finite32(c.Ping, 600000); p != nil {
				v := int32(*p)
				e.Ping = &v
			}
		}
		e.Extra = validExtra(in.Extra)
		// Repetido no mesmo lote (o cliente já agrupa, mas não confiamos): soma.
		if i, ok := byHash[e.Hash]; ok {
			e.Count = min(out[i].Count+e.Count, clientErrorMaxCount)
			out[i] = e
			continue
		}
		byHash[e.Hash] = len(out)
		out = append(out, e)
	}
	return out
}

// validExtra aceita só um objeto JSON pequeno (o Postgres recusa \u0000 em jsonb).
func validExtra(raw json.RawMessage) []byte {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || len(raw) > clientErrorMaxExtra || raw[0] != '{' || !json.Valid(raw) || bytes.Contains(raw, []byte(`\u0000`)) {
		return nil
	}
	return raw
}

// finite32 devolve nil para número ausente, fora do intervalo [-lim, lim] ou inválido.
func finite32(v *float64, lim float64) *float32 {
	if v == nil || math.IsNaN(*v) || math.Abs(*v) > lim {
		return nil
	}
	f := float32(*v)
	return &f
}

// cleanText tira o byte nulo (o Postgres recusa em TEXT).
func cleanText(s string) string { return strings.ReplaceAll(s, "\x00", "") }

// cut corta em n bytes sem partir um caractere.
func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// cutRunes corta em n caracteres.
func cutRunes(s string, n int) string {
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}

// clientIP: o IP de quem pediu. Em produção (Railway) o proxy acrescenta o IP real no fim
// do X-Forwarded-For; o que vem antes foi o próprio cliente que mandou e não vale.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimiter conta pedidos por chave em janelas fixas (em memória: um servidor só).
type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*rateWindow
	now    func() time.Time
}

type rateWindow struct {
	start time.Time
	n     int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, hits: map[string]*rateWindow{}, now: time.Now}
}

// allow conta mais um pedido da chave e diz se ainda cabe na janela.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.hits) > 50000 { // não cresce sem fim: esquece as janelas vencidas
		for k, w := range l.hits {
			if now.Sub(w.start) >= l.window {
				delete(l.hits, k)
			}
		}
	}
	w := l.hits[key]
	if w == nil || now.Sub(w.start) >= l.window {
		w = &rateWindow{start: now}
		l.hits[key] = w
	}
	w.n++
	return w.n <= l.limit
}

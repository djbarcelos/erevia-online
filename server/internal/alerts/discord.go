package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Envio ao webhook do Discord. A URL do webhook é segredo (tem o token): nunca vai para o
// log nem para as mensagens de erro (ver scrub).

// Cores por gravidade (decimal, como o Discord quer).
const (
	colorCritical = 0xE74C3C // vermelho: erro novo no servidor, pico, queda
	colorHigh     = 0xE67E22 // laranja: erro novo no navegador, tick lento
	colorWarn     = 0xF1C40F // amarelo: invariante quebrada
	colorInfo     = 0x3498DB // azul: /bug, reinício
	colorOK       = 0x2ECC71 // verde: deploy
)

// Limites do Discord (https://discord.com/developers/docs/resources/message#embed-object-embed-limits).
const (
	maxTitle      = 256
	maxDesc       = 4096
	maxFieldName  = 256
	maxFieldValue = 1024
	maxFields     = 25
	maxFooter     = 2048
	maxEmbedTotal = 6000 // soma de todos os textos de todos os embeds de uma mensagem
	maxEmbeds     = 10
)

// Embed é um embed do Discord.
type Embed struct {
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	URL         string  `json:"url,omitempty"`
	Color       int     `json:"color"`
	Fields      []Field `json:"fields,omitempty"`
	Footer      *Footer `json:"footer,omitempty"`
	Timestamp   string  `json:"timestamp,omitempty"`
}

type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type Footer struct {
	Text string `json:"text"`
}

type message struct {
	Username string  `json:"username"`
	Embeds   []Embed `json:"embeds"`
	// Nenhuma menção vale (um relato com "@everyone" não chama ninguém).
	AllowedMentions struct {
		Parse []string `json:"parse"`
	} `json:"allowed_mentions"`
}

func newMessage(embeds []Embed) message {
	if len(embeds) > maxEmbeds {
		embeds = embeds[:maxEmbeds]
	}
	m := message{Username: "Erevia", Embeds: make([]Embed, len(embeds))}
	m.AllowedMentions.Parse = []string{}
	budget := maxEmbedTotal
	for i, e := range embeds {
		m.Embeds[i] = fit(e, budget/(len(embeds)-i))
		budget -= embedSize(m.Embeds[i])
	}
	return m
}

// fit corta o embed nos limites do Discord e num total de no máximo budget caracteres.
func fit(e Embed, budget int) Embed {
	e.Title = cut(e.Title, maxTitle)
	if e.Title == "" {
		e.Title = "Erevia"
	}
	e.Description = cut(e.Description, maxDesc)
	if len(e.Fields) > maxFields {
		e.Fields = e.Fields[:maxFields]
	}
	fields := make([]Field, 0, len(e.Fields))
	for _, f := range e.Fields {
		f.Name, f.Value = cut(f.Name, maxFieldName), cut(f.Value, maxFieldValue)
		if f.Name == "" {
			f.Name = "​"
		}
		if strings.TrimSpace(f.Value) == "" { // o Discord recusa campo vazio
			f.Value = "—"
		}
		fields = append(fields, f)
	}
	e.Fields = fields
	if e.Footer != nil {
		e.Footer = &Footer{Text: cut(e.Footer.Text, maxFooter)}
	}
	// Passou do total: a descrição encolhe primeiro, depois os campos do fim saem.
	if over := embedSize(e) - budget; over > 0 {
		n := utf8.RuneCountInString(e.Description)
		e.Description = cut(e.Description, max(0, n-over))
	}
	for embedSize(e) > budget && len(e.Fields) > 0 {
		e.Fields = e.Fields[:len(e.Fields)-1]
	}
	return e
}

func embedSize(e Embed) int {
	n := utf8.RuneCountInString(e.Title) + utf8.RuneCountInString(e.Description)
	for _, f := range e.Fields {
		n += utf8.RuneCountInString(f.Name) + utf8.RuneCountInString(f.Value)
	}
	if e.Footer != nil {
		n += utf8.RuneCountInString(e.Footer.Text)
	}
	return n
}

// cut corta em n runas (com reticências se cortou). Fecha um bloco de código aberto.
func cut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	out := string(r[:n-1])
	if strings.Count(out, "```")%2 == 1 {
		if n < 5 {
			return ""
		}
		out = string(r[:n-5]) + "…\n```"
		return out
	}
	return out + "…"
}

// ---------- Texto de fora (jogadores, navegadores, stacks) ----------

var mdEscaper = strings.NewReplacer(
	`\`, `\\`, "*", `\*`, "_", `\_`, "~", `\~`, "`", "\\`", "|", `\|`, ">", `\>`,
	"#", `\#`, "[", `\[`, "]", `\]`, "@", "@​", "<", `\<`,
)

// plain escapa o markdown de um texto que veio de fora (o /bug de um jogador não formata,
// não vira link mascarado nem menção).
func plain(s string) string { return mdEscaper.Replace(strings.ToValidUTF8(s, "?")) }

// code põe o texto num bloco de código (sem deixar fechar o bloco antes da hora).
func code(s string) string {
	s = strings.ReplaceAll(strings.ToValidUTF8(s, "?"), "```", "ˋˋˋ")
	return "```\n" + strings.TrimRight(s, "\n") + "\n```"
}

// inline: texto curto num `código` (sem crase dentro).
func inline(s string) string {
	s = strings.ReplaceAll(strings.ToValidUTF8(s, "?"), "`", "ˋ")
	if s == "" {
		return "—"
	}
	return "`" + s + "`"
}

// ---------- Envio ----------

// Erros do envio.
var (
	// errRateLimited: o Discord pediu para esperar mais do que maxRetryWait. O aviso
	// fica para a próxima verificação.
	errRateLimited = errors.New("discord: limite de envio (429)")
)

// permanentError: o Discord recusou de vez (400, 401, 404...). Tentar de novo não adianta.
type permanentError struct {
	status int
	body   string
}

func (e *permanentError) Error() string {
	return fmt.Sprintf("discord: recusado (%d): %s", e.status, e.body)
}

type discord struct {
	url      string
	client   *http.Client
	attempts int           // tentativas por mensagem
	maxWait  time.Duration // espera máxima de um 429 (mais que isso fica para a próxima)
	backoff  time.Duration // espera depois de erro de rede ou 5xx (dobra a cada tentativa)
	sleep    func(context.Context, time.Duration) error
	now      func() time.Time

	mu        sync.Mutex
	notBefore time.Time // o Discord disse que o balde esvaziou (X-RateLimit-Remaining: 0)
}

func newDiscord(webhookURL string, client *http.Client) *discord {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &discord{
		url: webhookURL, client: client, attempts: 4, maxWait: 30 * time.Second, backoff: time.Second,
		sleep: sleepCtx, now: time.Now,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// send manda a mensagem, com timeout por tentativa (o do http.Client), nova tentativa com
// espera em erro de rede e 5xx, e no 429 espera o retry_after que o Discord pediu.
func (d *discord) send(ctx context.Context, m message) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	var last error
	backoff := d.backoff
	for attempt := 0; attempt < d.attempts; attempt++ {
		d.mu.Lock()
		wait := d.notBefore.Sub(d.now())
		d.mu.Unlock()
		if wait > d.maxWait {
			return errRateLimited
		}
		if err := d.sleep(ctx, wait); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url, bytes.NewReader(body))
		if err != nil {
			return errors.New("discord: ALERT_WEBHOOK_URL inválida")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Erevia-Alerts (erevia, 1.0)")
		res, err := d.client.Do(req)
		if err != nil {
			last = scrub(err)
			if ctx.Err() != nil {
				return last
			}
			if err := d.sleep(ctx, backoff); err != nil {
				return err
			}
			backoff *= 2
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		res.Body.Close()
		d.noteBucket(res.Header)
		switch {
		case res.StatusCode >= 200 && res.StatusCode < 300:
			return nil
		case res.StatusCode == http.StatusTooManyRequests:
			wait := retryAfter(res.Header, data)
			last = fmt.Errorf("%w: esperar %s", errRateLimited, wait)
			if wait > d.maxWait {
				return last
			}
			if err := d.sleep(ctx, wait); err != nil {
				return err
			}
		case res.StatusCode >= 500:
			last = fmt.Errorf("discord: %d", res.StatusCode)
			if err := d.sleep(ctx, backoff); err != nil {
				return err
			}
			backoff *= 2
		default:
			return &permanentError{status: res.StatusCode, body: cut(strings.TrimSpace(string(data)), 200)}
		}
	}
	return last
}

// noteBucket: com X-RateLimit-Remaining: 0, o próximo envio espera o balde encher.
func (d *discord) noteBucket(h http.Header) {
	if h.Get("X-RateLimit-Remaining") != "0" {
		return
	}
	secs, err := strconv.ParseFloat(h.Get("X-RateLimit-Reset-After"), 64)
	if err != nil || secs <= 0 {
		return
	}
	d.mu.Lock()
	d.notBefore = d.now().Add(time.Duration(secs * float64(time.Second)))
	d.mu.Unlock()
}

// retryAfter: quanto esperar depois de um 429. O corpo JSON (retry_after, em segundos,
// com fração) vale mais que o cabeçalho Retry-After; sem nenhum, 1 s.
func retryAfter(h http.Header, body []byte) time.Duration {
	var r struct {
		RetryAfter *float64 `json:"retry_after"`
	}
	if json.Unmarshal(body, &r) == nil && r.RetryAfter != nil && *r.RetryAfter >= 0 {
		return time.Duration(*r.RetryAfter * float64(time.Second))
	}
	if secs, err := strconv.ParseFloat(h.Get("Retry-After"), 64); err == nil && secs >= 0 {
		return time.Duration(secs * float64(time.Second))
	}
	return time.Second
}

// scrub tira a URL (com o token do webhook) de um erro do http.Client.
func scrub(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("discord: %s: %v", ue.Op, ue.Err)
	}
	return err
}

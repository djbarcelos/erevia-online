package store

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ClientError é um erro do navegador já validado e cortado pela API.
type ClientError struct {
	Hash      string // ClientErrorHash(Message, Stack)
	Version   string // commit do build do cliente
	Kind      string // error, rejection, webgl, decode, model…
	Message   string
	Stack     string
	Count     int64 // quantas vezes aconteceu desde o último lote
	UserAgent string
	// AccountID vem da sessão (0 = anônimo, ex.: tela de login). CharacterID vem do
	// cliente e só é gravado se o personagem for da conta.
	AccountID   int64
	CharacterID int64
	Region      string
	X, Z        *float32
	FPS         *float32
	Ping        *int32
	Extra       []byte // objeto JSON ou nil
}

// ClientErrorHash agrupa erros iguais: a mensagem e a primeira linha da stack que é um
// quadro de chamada (V8: "at f (url:l:c)"; Firefox/Safari: "f@url:l:c"). O cabeçalho
// "Error: mensagem" do V8 fica de fora, já que repete a mensagem.
func ClientErrorHash(message, stack string) string {
	sum := sha256.Sum256([]byte(message + "\n" + FirstStackFrame(stack)))
	return hex.EncodeToString(sum[:16])
}

// FirstStackFrame devolve a primeira linha de quadro da stack (sem espaços nas pontas), ou
// a primeira linha não vazia se nenhuma parecer um quadro. O cliente (net/errors.ts) usa a
// mesma regra para agrupar antes de mandar.
func FirstStackFrame(stack string) string {
	first := ""
	for line := range strings.SplitSeq(stack, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "at ") || strings.Contains(line, "@") {
			return line
		}
		if first == "" {
			first = line
		}
	}
	return first
}

// RecordClientErrors grava um lote: erro novo vira uma linha; repetido (mesmo hash e
// versão) soma a contagem, atualiza a última vez e guarda o contexto da última ocorrência.
// Mensagem e stack ficam as da primeira vez.
func (s *Store) RecordClientErrors(ctx context.Context, errs []ClientError) error {
	if len(errs) == 0 {
		return nil
	}
	// Mesma ordem em todos os lotes: dois lotes ao mesmo tempo nunca travam um ao outro
	// em ordem cruzada (deadlock).
	errs = slices.Clone(errs)
	slices.SortFunc(errs, func(a, b ClientError) int {
		return cmp.Or(cmp.Compare(a.Hash, b.Hash), cmp.Compare(a.Version, b.Version))
	})
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		b := &pgx.Batch{}
		for _, e := range errs {
			var extra any
			if len(e.Extra) > 0 {
				extra = string(e.Extra)
			}
			// A conta tem de existir (uma apagada no meio do caminho fica anônima) e o
			// personagem tem de ser dela.
			b.Queue(`INSERT INTO client_errors AS ce
				(hash, version, kind, message, stack, count, user_agent, account_id, character_id, region, x, z, fps, ping, extra)
				VALUES ($1, $2, $3, $4, $5, $6, $7,
					(SELECT id FROM accounts WHERE id = $8),
					(SELECT id FROM characters WHERE id = $9 AND account_id = $8),
					$10, $11, $12, $13, $14, $15::jsonb)
				ON CONFLICT (hash, version) DO UPDATE SET
					count = ce.count + EXCLUDED.count,
					last_seen = now(),
					kind = EXCLUDED.kind,
					user_agent = EXCLUDED.user_agent,
					account_id = EXCLUDED.account_id,
					character_id = EXCLUDED.character_id,
					region = EXCLUDED.region,
					x = EXCLUDED.x, z = EXCLUDED.z, fps = EXCLUDED.fps, ping = EXCLUDED.ping,
					extra = EXCLUDED.extra`,
				e.Hash, e.Version, e.Kind, e.Message, e.Stack, e.Count, e.UserAgent,
				e.AccountID, e.CharacterID, e.Region, e.X, e.Z, e.FPS, e.Ping, extra)
		}
		return tx.SendBatch(ctx, b).Close()
	})
}

// anonymizeClientErrors desliga os erros do cliente da conta que está sendo apagada
// (LGPD): o erro continua para a correção, sem dizer de quem era.
func anonymizeClientErrors(ctx context.Context, tx pgx.Tx, accountID int64) error {
	_, err := tx.Exec(ctx, `UPDATE client_errors SET account_id = NULL, character_id = NULL WHERE account_id = $1`, accountID)
	return err
}

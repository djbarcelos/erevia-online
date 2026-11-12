// Package store é o acesso ao PostgreSQL: contas, sessões e personagens.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	mrand "math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

const (
	SessionTTL           = 30 * 24 * time.Hour
	MaxCharsPerAccount   = 3
	spawnRadius          = 8.0
	pgUniqueViolation    = "23505"
	charactersNameIdxKey = "characters_name_lower_idx"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrNameTaken         = errors.New("name already taken")
	ErrTooManyCharacters = errors.New("too many characters")
)

// Classes aceitas (mesmo CHECK da tabela).
var Classes = []string{"knight", "barbarian", "mage", "rogue"}

func ValidClass(c string) bool {
	for _, k := range Classes {
		if c == k {
			return true
		}
	}
	return false
}

type Character struct {
	ID        int64   `json:"id"`
	AccountID int64   `json:"-"`
	Name      string  `json:"name"`
	Class     string  `json:"class"`
	Level     int     `json:"level"`
	XP        int64   `json:"-"`
	X         float64 `json:"-"`
	Z         float64 `json:"-"`
	HP        int     `json:"-"` // -1 = cheio
	MP        int     `json:"-"` // -1 = cheio
}

// CharacterState é o que a zona salva periodicamente.
type CharacterState struct {
	CharacterID int64
	X, Z        float64
	Level       int
	XP          int64
	HP, MP      int // -1 = cheio
}

type Store struct {
	pool *pgxpool.Pool
}

// Open conecta, testa a conexão e aplica as migrações pendentes.
func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: %w", err)
	}
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, mustSub(migrations, "migrations"))
	if err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// ---------- Contas e sessões ----------

// LoginIdentity encontra (ou cria) a conta ligada a provider/subject e registra o login.
func (s *Store) LoginIdentity(ctx context.Context, provider, subject, displayName string) (int64, error) {
	var accountID int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			`UPDATE identities SET display_name = $3 WHERE provider = $1 AND subject = $2 RETURNING account_id`,
			provider, subject, displayName).Scan(&accountID)
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.QueryRow(ctx, `INSERT INTO accounts DEFAULT VALUES RETURNING id`).Scan(&accountID); err != nil {
				return err
			}
			_, err = tx.Exec(ctx,
				`INSERT INTO identities (provider, subject, account_id, display_name) VALUES ($1, $2, $3, $4)`,
				provider, subject, accountID, displayName)
			return err
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE accounts SET last_login_at = now() WHERE id = $1`, accountID)
		return err
	})
	return accountID, err
}

// CreateSession gera um token aleatório, guarda o hash e devolve o token em texto.
func (s *Store) CreateSession(ctx context.Context, accountID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, account_id, expires_at) VALUES ($1, $2, $3)`,
		hashToken(token), accountID, time.Now().Add(SessionTTL))
	if err != nil {
		return "", err
	}
	return token, nil
}

// AccountBySession devolve a conta de uma sessão válida (não expirada, conta não banida).
func (s *Store) AccountBySession(ctx context.Context, token string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		SELECT a.id FROM sessions s JOIN accounts a ON a.id = s.account_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND NOT a.banned`,
		hashToken(token)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

// DisplayName devolve o nome vindo do provedor (para mostrar "logado como …").
func (s *Store) DisplayName(ctx context.Context, accountID int64) (string, error) {
	var name string
	err := s.pool.QueryRow(ctx,
		`SELECT display_name FROM identities WHERE account_id = $1 ORDER BY created_at LIMIT 1`,
		accountID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return name, err
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// ---------- Personagens ----------

func (s *Store) ListCharacters(ctx context.Context, accountID int64) ([]Character, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, account_id, name, class, x, z, level, xp, coalesce(hp, -1), coalesce(mp, -1) FROM characters WHERE account_id = $1 ORDER BY id`, accountID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanCharacter)
}

// CreateCharacter cria um personagem perto do centro do mapa.
// Valide nome e classe antes de chamar.
func (s *Store) CreateCharacter(ctx context.Context, accountID int64, name, class string) (Character, error) {
	angle := mrand.Float64() * 2 * math.Pi
	r := mrand.Float64() * spawnRadius
	x, z := r*math.Cos(angle), r*math.Sin(angle)

	var c Character
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Trava a linha da conta para o limite de personagens valer com pedidos simultâneos.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM accounts WHERE id = $1 FOR UPDATE`, accountID); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM characters WHERE account_id = $1`, accountID).Scan(&n); err != nil {
			return err
		}
		if n >= MaxCharsPerAccount {
			return ErrTooManyCharacters
		}
		rows, err := tx.Query(ctx, `
			INSERT INTO characters (account_id, name, class, x, z) VALUES ($1, $2, $3, $4, $5)
			RETURNING id, account_id, name, class, x, z, level, xp, coalesce(hp, -1), coalesce(mp, -1)`, accountID, name, class, x, z)
		if err != nil {
			return err
		}
		c, err = pgx.CollectExactlyOneRow(rows, scanCharacter)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == charactersNameIdxKey {
		return Character{}, ErrNameTaken
	}
	return c, err
}

// CharacterForAccount carrega o personagem só se ele pertencer à conta.
func (s *Store) CharacterForAccount(ctx context.Context, accountID, characterID int64) (Character, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, account_id, name, class, x, z, level, xp, coalesce(hp, -1), coalesce(mp, -1) FROM characters WHERE id = $1 AND account_id = $2`,
		characterID, accountID)
	if err != nil {
		return Character{}, err
	}
	c, err := pgx.CollectExactlyOneRow(rows, scanCharacter)
	if errors.Is(err, pgx.ErrNoRows) {
		return Character{}, ErrNotFound
	}
	return c, err
}

// SaveStates grava posição e progressão de vários personagens numa única ida ao banco.
func (s *Store) SaveStates(ctx context.Context, states []CharacterState) error {
	if len(states) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, c := range states {
		batch.Queue(`UPDATE characters SET x = $2, z = $3, level = $4, xp = $5, hp = $6, mp = $7, last_seen_at = now()
			WHERE id = $1`, c.CharacterID, c.X, c.Z, c.Level, c.XP, nullIfFull(c.HP), nullIfFull(c.MP))
	}
	return s.pool.SendBatch(ctx, batch).Close()
}

func nullIfFull(v int) *int {
	if v < 0 {
		return nil
	}
	return &v
}

func scanCharacter(row pgx.CollectableRow) (Character, error) {
	var c Character
	var x, z float32
	err := row.Scan(&c.ID, &c.AccountID, &c.Name, &c.Class, &x, &z, &c.Level, &c.XP, &c.HP, &c.MP)
	c.X, c.Z = float64(x), float64(z)
	return c, err
}

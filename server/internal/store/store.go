// Package store é o acesso ao PostgreSQL: contas, sessões e personagens.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	mrand "math/rand/v2"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"erevia/server/internal/game"
)

//go:embed migrations/*.sql
var migrations embed.FS

const (
	SessionTTL           = 30 * 24 * time.Hour
	MaxCharsPerAccount   = 3
	spawnRadius          = 8.0
	pgUniqueViolation    = "23505"
	charactersNameIdxKey = "characters_name_lower_idx"
	characterCols        = "id, account_id, name, class, x, z, level, xp, coalesce(hp, -1), coalesce(mp, -1), gold, kit_given, " +
		"attr_str, attr_dex, attr_con, attr_int, attr_wis, attr_spi, skill_ranks, hotbar, emblem, " +
		"karma, pk_count, pvp_kills, clan_left_at, skin, hair, outfit, head, hide_hat, quests, achievements, title, spec, mastery"
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
	ID        int64             `json:"id"`
	AccountID int64             `json:"-"`
	Name      string            `json:"name"`
	Class     string            `json:"class"`
	Level     int               `json:"level"`
	XP        int64             `json:"-"`
	X         float64           `json:"-"`
	Z         float64           `json:"-"`
	HP        int               `json:"-"` // -1 = cheio
	MP        int               `json:"-"` // -1 = cheio
	Gold      int64             `json:"-"`
	Items     []game.ItemRecord `json:"-"`
	Attrs     game.Attrs        `json:"-"`
	Ranks     map[uint32]int    `json:"-"`
	Hotbar    []string          `json:"-"`
	Emblem    string            `json:"emblem"` // sempre preenchido (padrão da classe se vazio no banco)
	// Look: o que o personagem tem nas mãos agora (para a tela de seleção).
	Look CharacterLook `json:"look"`
	// Appearance: visual do corpo (pele, cabelo, roupa, rosto e chapéu).
	Appearance game.Appearance `json:"appearance"`
	Karma      int             `json:"-"`
	PKs        int             `json:"-"`
	PvPKills   int             `json:"-"`
	ClanLeftAt *time.Time      `json:"-"`
	// Armazém da conta e moderação (só em CharacterForAccount).
	Storage    StorageState    `json:"-"`
	GM         bool            `json:"-"`
	MutedUntil *time.Time      `json:"-"`
	Quests     game.QuestState `json:"-"`
	// Conquistas e o título em uso (id da conquista; "" = nenhum).
	Achievements game.AchievementState `json:"-"`
	Title        string                `json:"-"`
	// Especialização (segunda classe, nível 20; "" = nenhuma).
	Spec string `json:"spec"`
	// Maestria (nível 30): barra, bônus diário e pontos de cada nó.
	Mastery  game.MasteryState `json:"-"`
	kitGiven bool
}

// StorageState é o armazém de uma conta: moedas e itens (Equipped é sempre false).
type StorageState struct {
	AccountID int64
	Gold      int64
	Items     []game.ItemRecord
	Seq       int64 // como save_seq: um estado mais velho que o gravado é ignorado
}

// CharacterLook são os modelos 3D das armas equipadas e a raridade da principal.
type CharacterLook struct {
	Right string `json:"right"`
	Left  string `json:"left"`
	Grade int    `json:"grade"`
}

// CharacterState é o que a zona salva periodicamente.
type CharacterState struct {
	CharacterID int64
	X, Z        float64
	Level       int
	XP          int64
	HP, MP      int // -1 = cheio
	Gold        int64
	Items       []game.ItemRecord
	Attrs       game.Attrs
	Ranks       map[uint32]int
	Hotbar      []string
	Karma       int
	PKs         int
	PvPKills    int
	Appearance  game.Appearance
	Quests      game.QuestState
	// Conquistas e título vão junto com o personagem (mesma linha, mesma transação).
	Achievements game.AchievementState
	Title        string
	Spec         string            // especialização ("" = nenhuma)
	Mastery      game.MasteryState // Maestria (o nível vai também em mastery_level, para o ranking)
	Seq          int64             // salvamento mais velho que o gravado é ignorado
	// Storage (opcional): o armazém da conta mudou e vai junto, na mesma transação.
	Storage *StorageState
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
	return migrateFS(ctx, pool, mustSub(migrations, "migrations"))
}

func migrateFS(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	// Fora de ordem permitido: dois PCs trabalham em paralelo com faixas de números
	// reservadas (PC 1: 00009–00019, PC 2: 00020–00029), então uma migração de número
	// menor pode chegar à produção depois de uma maior. Sem isso o goose recusa
	// ("missing (out-of-order) migration") e o servidor não sobe.
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithAllowOutofOrder(true))
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
		`SELECT `+characterCols+` FROM characters WHERE account_id = $1 ORDER BY id`, accountID)
	if err != nil {
		return nil, err
	}
	chars, err := pgx.CollectRows(rows, scanCharacter)
	if err != nil || len(chars) == 0 {
		return chars, err
	}
	// Armas nas mãos (encaixes 0 e 1) de cada um, para a tela de seleção.
	ids := make([]int64, len(chars))
	for i, c := range chars {
		ids[i] = c.ID
	}
	rows, err = s.pool.Query(ctx,
		`SELECT character_id, slot, item_id FROM character_items
		WHERE place = 1 AND slot IN ($2, $3) AND character_id = ANY($1)`, ids, game.SlotWeapon, game.SlotOffhand)
	if err != nil {
		return nil, err
	}
	type hand struct {
		char int64
		slot int16
		item string
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (hand, error) {
		var h hand
		err := row.Scan(&h.char, &h.slot, &h.item)
		return h, err
	})
	if err != nil {
		return nil, err
	}
	byChar := map[int64][]game.ItemRecord{}
	for _, h := range list {
		byChar[h.char] = append(byChar[h.char], game.ItemRecord{Equipped: true, Slot: int(h.slot), ItemID: h.item, Count: 1})
	}
	for i := range chars {
		chars[i].Look = lookOf(byChar[chars[i].ID])
	}
	return chars, nil
}

// lookOf: modelos das armas equipadas (igual ao que a zona mostra aos outros jogadores).
func lookOf(items []game.ItemRecord) CharacterLook {
	var l CharacterLook
	for _, r := range items {
		d := game.Items[r.ItemID]
		if !r.Equipped || d == nil {
			continue
		}
		switch r.Slot {
		case game.SlotWeapon:
			l.Right, l.Grade = d.Model, int(d.Grade)
			if l.Left == "" {
				l.Left = d.OffModel
			}
		case game.SlotOffhand:
			l.Left = d.Model
		}
	}
	return l
}

// CreateCharacter cria um personagem perto do centro do mapa, já com o kit inicial da
// aparência escolhida. Valide nome, classe e aparência antes de chamar.
func (s *Store) CreateCharacter(ctx context.Context, accountID int64, name, class string, look game.Look) (Character, error) {
	angle := mrand.Float64() * 2 * math.Pi
	r := mrand.Float64() * spawnRadius
	x, z := r*math.Cos(angle), r*math.Sin(angle)

	ap := game.NormalAppearance(class, look.Appearance)
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
			INSERT INTO characters (account_id, name, class, x, z, emblem, skin, hair, outfit, head, hide_hat)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING `+characterCols, accountID, name, class, x, z, look.Emblem,
			ap.Skin, ap.Hair, ap.Outfit, ap.Head, ap.HideHat)
		if err != nil {
			return err
		}
		if c, err = pgx.CollectExactlyOneRow(rows, scanCharacter); err != nil {
			return err
		}
		return giveStarterKit(ctx, tx, &c, game.StarterKitFor(class, look))
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == charactersNameIdxKey {
		return Character{}, ErrNameTaken
	}
	return c, err
}

// CharacterForAccount carrega o personagem (com moedas e itens) só se ele pertencer à
// conta. No primeiro carregamento entrega o kit inicial, na mesma transação.
func (s *Store) CharacterForAccount(ctx context.Context, accountID, characterID int64) (Character, error) {
	var c Character
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT `+characterCols+` FROM characters WHERE id = $1 AND account_id = $2 FOR UPDATE`,
			characterID, accountID)
		if err != nil {
			return err
		}
		if c, err = pgx.CollectExactlyOneRow(rows, scanCharacter); err != nil {
			return err
		}
		if !c.kitGiven { // personagem de antes do kit: recebe o padrão da classe
			if err := giveStarterKit(ctx, tx, &c, game.StarterKit(c.Class)); err != nil {
				return err
			}
		}
		if c.Items, err = loadItems(ctx, tx, c.ID); err != nil {
			return err
		}
		c.Look = lookOf(c.Items)
		if err := tx.QueryRow(ctx, `SELECT gm, muted_until FROM accounts WHERE id = $1`, accountID).
			Scan(&c.GM, &c.MutedUntil); err != nil {
			return err
		}
		c.Storage, err = loadStorage(ctx, tx, accountID)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Character{}, ErrNotFound
	}
	return c, err
}

func giveStarterKit(ctx context.Context, tx pgx.Tx, c *Character, kit []game.ItemRecord) error {
	for _, it := range kit {
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_items (character_id, place, slot, item_id, count) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING`, c.ID, placeOf(it), it.Slot, it.ItemID, it.Count); err != nil {
			return err
		}
	}
	err := tx.QueryRow(ctx, `UPDATE characters SET gold = gold + $2, kit_given = true WHERE id = $1 RETURNING gold`,
		c.ID, game.StarterGold).Scan(&c.Gold)
	c.kitGiven = true
	c.Look = lookOf(kit)
	return err
}

func loadItems(ctx context.Context, tx pgx.Tx, characterID int64) ([]game.ItemRecord, error) {
	rows, err := tx.Query(ctx,
		`SELECT place, slot, item_id, count, enchant FROM character_items WHERE character_id = $1 ORDER BY place DESC, slot`, characterID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (game.ItemRecord, error) {
		var r game.ItemRecord
		var place, slot, enchant int16
		err := row.Scan(&place, &slot, &r.ItemID, &r.Count, &enchant)
		r.Equipped, r.Slot, r.Enchant = place == 1, int(slot), int(enchant)
		return r, err
	})
}

// loadStorage lê o armazém da conta.
func loadStorage(ctx context.Context, tx pgx.Tx, accountID int64) (StorageState, error) {
	st := StorageState{AccountID: accountID}
	if err := tx.QueryRow(ctx, `SELECT stored_gold, storage_seq FROM accounts WHERE id = $1`, accountID).
		Scan(&st.Gold, &st.Seq); err != nil {
		return st, err
	}
	rows, err := tx.Query(ctx,
		`SELECT slot, item_id, count, enchant FROM account_items WHERE account_id = $1 ORDER BY slot`, accountID)
	if err != nil {
		return st, err
	}
	st.Items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (game.ItemRecord, error) {
		var r game.ItemRecord
		var slot, enchant int16
		err := row.Scan(&slot, &r.ItemID, &r.Count, &enchant)
		r.Slot, r.Enchant = int(slot), int(enchant)
		return r, err
	})
	return st, err
}

func placeOf(r game.ItemRecord) int16 {
	if r.Equipped {
		return 1
	}
	return 0
}

// SaveStates grava posição, progressão, moedas e itens de vários personagens numa
// única transação. Um estado mais velho (Seq menor) que o já gravado é ignorado, e os
// itens de um personagem só são trocados junto com a linha dele: nunca fica meio salvo.
func (s *Store) SaveStates(ctx context.Context, states []CharacterState) error {
	// Se o mesmo personagem vier duas vezes, vale o mais novo.
	latest := map[int64]CharacterState{}
	for _, c := range states {
		if old, ok := latest[c.CharacterID]; !ok || c.Seq >= old.Seq {
			latest[c.CharacterID] = c
		}
	}
	if len(latest) == 0 {
		return nil
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		order := make([]CharacterState, 0, len(latest))
		for _, c := range latest {
			order = append(order, c)
			ranks, hotbar, err := progressJSON(c.Ranks, c.Hotbar)
			if err != nil {
				return err
			}
			quests, err := json.Marshal(c.Quests)
			if err != nil {
				return err
			}
			achievements, err := json.Marshal(c.Achievements)
			if err != nil {
				return err
			}
			mastery, err := json.Marshal(c.Mastery)
			if err != nil {
				return err
			}
			a := c.Attrs
			batch.Queue(`UPDATE characters SET x = $2, z = $3, level = $4, xp = $5, hp = $6, mp = $7, gold = $8,
				save_seq = GREATEST(save_seq, $9::bigint), last_seen_at = now(),
				attr_str = $10, attr_dex = $11, attr_con = $12, attr_int = $13, attr_wis = $14, attr_spi = $15,
				skill_ranks = $16, hotbar = $17, karma = $18, pk_count = $19, pvp_kills = $20,
				skin = $21, hair = $22, outfit = $23, head = $24, hide_hat = $25, quests = $26,
				achievements = $27, title = $28, spec = $29, mastery = $30, mastery_level = $31
				WHERE id = $1 AND ($9::bigint = 0 OR save_seq < $9::bigint) RETURNING id`,
				c.CharacterID, c.X, c.Z, c.Level, c.XP, nullIfFull(c.HP), nullIfFull(c.MP), max(c.Gold, 0), c.Seq,
				max(a.Str, 0), max(a.Dex, 0), max(a.Con, 0), max(a.Int, 0), max(a.Wis, 0), max(a.Spi, 0), ranks, hotbar,
				max(c.Karma, 0), max(c.PKs, 0), max(c.PvPKills, 0),
				c.Appearance.Skin, c.Appearance.Hair, c.Appearance.Outfit, c.Appearance.Head, c.Appearance.HideHat, quests,
				achievements, c.Title, c.Spec, mastery, max(c.Mastery.Points, 0))
		}
		br := tx.SendBatch(ctx, batch)
		var ids []int64
		var rows [][]any
		for _, c := range order {
			var id int64
			err := br.QueryRow().Scan(&id)
			if errors.Is(err, pgx.ErrNoRows) {
				continue // já há um salvamento mais novo
			}
			if err != nil {
				br.Close()
				return err
			}
			ids = append(ids, id)
			for _, it := range c.Items {
				rows = append(rows, []any{id, placeOf(it), int16(it.Slot), it.ItemID, int32(it.Count), int16(it.Enchant)})
			}
		}
		if err := br.Close(); err != nil {
			return err
		}
		if len(ids) > 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM character_items WHERE character_id = ANY($1)`, ids); err != nil {
				return err
			}
			if _, err := tx.CopyFrom(ctx, pgx.Identifier{"character_items"},
				[]string{"character_id", "place", "slot", "item_id", "count", "enchant"}, pgx.CopyFromRows(rows)); err != nil {
				return err
			}
		}
		return saveStorages(ctx, tx, states)
	})
}

// saveStorages grava os armazéns que vieram junto (o mais novo de cada conta), na mesma
// transação dos personagens: o item que saiu da bolsa e entrou no armazém nunca fica
// salvo pela metade.
func saveStorages(ctx context.Context, tx pgx.Tx, states []CharacterState) error {
	latest := map[int64]*StorageState{}
	for _, c := range states {
		if st := c.Storage; st != nil {
			if old, ok := latest[st.AccountID]; !ok || st.Seq >= old.Seq {
				latest[st.AccountID] = st
			}
		}
	}
	for _, st := range latest {
		var id int64
		err := tx.QueryRow(ctx, `UPDATE accounts SET stored_gold = $2, storage_seq = GREATEST(storage_seq, $3::bigint)
			WHERE id = $1 AND ($3::bigint = 0 OR storage_seq < $3::bigint) RETURNING id`,
			st.AccountID, max(st.Gold, 0), st.Seq).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue // já há um armazém mais novo gravado
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM account_items WHERE account_id = $1`, id); err != nil {
			return err
		}
		rows := make([][]any, 0, len(st.Items))
		for _, it := range st.Items {
			rows = append(rows, []any{id, int16(it.Slot), it.ItemID, int32(it.Count), int16(it.Enchant)})
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"account_items"},
			[]string{"account_id", "slot", "item_id", "count", "enchant"}, pgx.CopyFromRows(rows)); err != nil {
			return err
		}
	}
	return nil
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
	var ranks, hotbar, quests, achievements, mastery []byte
	var a [6]int16
	var skin, hair, outfit int16
	err := row.Scan(&c.ID, &c.AccountID, &c.Name, &c.Class, &x, &z, &c.Level, &c.XP, &c.HP, &c.MP, &c.Gold, &c.kitGiven,
		&a[0], &a[1], &a[2], &a[3], &a[4], &a[5], &ranks, &hotbar, &c.Emblem,
		&c.Karma, &c.PKs, &c.PvPKills, &c.ClanLeftAt, &skin, &hair, &outfit, &c.Appearance.Head, &c.Appearance.HideHat, &quests,
		&achievements, &c.Title, &c.Spec, &mastery)
	if err != nil {
		return c, err
	}
	if len(mastery) > 0 {
		if err := json.Unmarshal(mastery, &c.Mastery); err != nil {
			return c, fmt.Errorf("mastery: %w", err)
		}
	}
	if len(achievements) > 0 {
		if err := json.Unmarshal(achievements, &c.Achievements); err != nil {
			return c, fmt.Errorf("achievements: %w", err)
		}
	}
	if len(quests) > 0 {
		if err := json.Unmarshal(quests, &c.Quests); err != nil {
			return c, fmt.Errorf("quests: %w", err)
		}
	}
	c.Emblem = game.EmblemOr(c.Class, c.Emblem)
	c.Appearance.Skin, c.Appearance.Hair, c.Appearance.Outfit = int(skin), int(hair), int(outfit)
	if !game.ValidAppearance(c.Appearance) { // paleta encolheu: volta ao original
		c.Appearance = game.Appearance{}
	}
	c.X, c.Z = float64(x), float64(z)
	c.Attrs = game.Attrs{Str: int(a[0]), Dex: int(a[1]), Con: int(a[2]), Int: int(a[3]), Wis: int(a[4]), Spi: int(a[5])}
	c.Ranks, c.Hotbar, err = parseProgress(ranks, hotbar)
	return c, err
}

// progressJSON: ranks como {"id": rank} e a barra como lista (JSONB).
func progressJSON(ranks map[uint32]int, hotbar []string) ([]byte, []byte, error) {
	m := make(map[string]int, len(ranks))
	for id, r := range ranks {
		if r > 0 {
			m[strconv.FormatUint(uint64(id), 10)] = r
		}
	}
	rb, err := json.Marshal(m)
	if err != nil {
		return nil, nil, err
	}
	if hotbar == nil {
		hotbar = []string{}
	}
	hb, err := json.Marshal(hotbar)
	return rb, hb, err
}

func parseProgress(ranksJSON, hotbarJSON []byte) (map[uint32]int, []string, error) {
	var m map[string]int
	if len(ranksJSON) > 0 {
		if err := json.Unmarshal(ranksJSON, &m); err != nil {
			return nil, nil, fmt.Errorf("skill_ranks: %w", err)
		}
	}
	ranks := make(map[uint32]int, len(m))
	for k, r := range m {
		id, err := strconv.ParseUint(k, 10, 32)
		if err == nil && r > 0 {
			ranks[uint32(id)] = r
		}
	}
	var hotbar []string
	if len(hotbarJSON) > 0 {
		if err := json.Unmarshal(hotbarJSON, &hotbar); err != nil {
			return nil, nil, fmt.Errorf("hotbar: %w", err)
		}
	}
	return ranks, hotbar, nil
}

package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// RankRow é uma linha do ranking de personagens.
type RankRow struct {
	CharacterID int64
	Name        string
	Class       string
	Level       int
	PvPKills    int
	ClanName    string
	TitleID     string // conquista do título em uso
	Mastery     int    // nível de Maestria (pontos ganhos no nível máximo)
}

// ClanRankRow é uma linha do ranking de clãs.
type ClanRankRow struct {
	ClanID   int64
	Name     string
	Members  int
	LevelSum int
	LevelAvg float64
	Leader   string
}

// Rankings é um cálculo do ranking: as listas (top limit) e a posição de cada
// personagem e clã (começando em 1).
type Rankings struct {
	At       time.Time
	Level    []RankRow
	PvP      []RankRow
	Clans    []ClanRankRow
	LevelPos map[int64]int
	PvPPos   map[int64]int
	ClanPos  map[int64]int
}

// Ordens do ranking (as mesmas nas listas e nas posições; o id desempata de vez).
const (
	rankLevelOrder = "c.level DESC, c.mastery_level DESC, c.xp DESC, c.id"
	rankPvPOrder   = "c.pvp_kills DESC, c.level DESC, c.mastery_level DESC, c.xp DESC, c.id"
	rankFrom       = " FROM characters c JOIN accounts a ON a.id = c.account_id"
	rankWhere      = " WHERE NOT a.banned"
)

// Rankings calcula o ranking numa transação só de leitura (as três listas veem o mesmo
// momento do banco). Contas banidas ficam de fora; o PvP só lista quem tem vitórias.
// Clãs: pela soma dos níveis dos membros (desempate: média).
func (s *Store) Rankings(ctx context.Context, limit int) (Rankings, error) {
	r := Rankings{At: time.Now(), LevelPos: map[int64]int{}, PvPPos: map[int64]int{}, ClanPos: map[int64]int{}}
	err := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		if r.Level, err = rankRows(ctx, tx, "", rankLevelOrder, limit); err != nil {
			return err
		}
		if r.PvP, err = rankRows(ctx, tx, " AND c.pvp_kills > 0", rankPvPOrder, limit); err != nil {
			return err
		}
		if err := rankPositions(ctx, tx, "", rankLevelOrder, r.LevelPos); err != nil {
			return err
		}
		if err := rankPositions(ctx, tx, " AND c.pvp_kills > 0", rankPvPOrder, r.PvPPos); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT cl.id, cl.name, count(*)::int, sum(c.level)::int, avg(c.level)::float8, coalesce(l.name, '')
			FROM clans cl
			JOIN clan_members m ON m.clan_id = cl.id
			JOIN characters c ON c.id = m.character_id
			LEFT JOIN characters l ON l.id = cl.leader_id
			GROUP BY cl.id, l.name
			ORDER BY sum(c.level) DESC, avg(c.level) DESC, cl.id`)
		if err != nil {
			return err
		}
		all, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ClanRankRow, error) {
			var c ClanRankRow
			err := row.Scan(&c.ClanID, &c.Name, &c.Members, &c.LevelSum, &c.LevelAvg, &c.Leader)
			return c, err
		})
		if err != nil {
			return err
		}
		for i, c := range all {
			r.ClanPos[c.ClanID] = i + 1
		}
		r.Clans = all[:min(limit, len(all))]
		return nil
	})
	return r, err
}

// rankRows: as primeiras limit linhas, com o clã e o título.
func rankRows(ctx context.Context, tx pgx.Tx, where, order string, limit int) ([]RankRow, error) {
	rows, err := tx.Query(ctx, `SELECT c.id, c.name, `+activeClassSQL+`, c.level, c.pvp_kills, coalesce(cl.name, ''), c.title, c.mastery_level`+rankFrom+`
		LEFT JOIN clan_members m ON m.character_id = c.id
		LEFT JOIN clans cl ON cl.id = m.clan_id`+rankWhere+where+`
		ORDER BY `+order+` LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (RankRow, error) {
		var x RankRow
		err := row.Scan(&x.CharacterID, &x.Name, &x.Class, &x.Level, &x.PvPKills, &x.ClanName, &x.TitleID, &x.Mastery)
		return x, err
	})
}

// rankPositions: a posição de cada personagem (só os ids, na ordem do ranking).
func rankPositions(ctx context.Context, tx pgx.Tx, where, order string, out map[int64]int) error {
	rows, err := tx.Query(ctx, `SELECT c.id`+rankFrom+rankWhere+where+` ORDER BY `+order)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	for i, id := range ids {
		out[id] = i + 1
	}
	return err
}

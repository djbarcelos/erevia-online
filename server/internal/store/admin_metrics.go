package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Portal de administração, seção Métricas: as mesmas contas das views da telemetria
// (migração 00011: v_gold_daily, v_level_times, v_deaths_by_region, v_quit_level, v_fps e
// v_items_daily), mas só no período pedido (as views olham a tabela inteira), e os erros
// do servidor (server_error, slow_tick, db_error e invariant).
//
// Desempenho (medido em admin_perf_test.go com uma semana de eventos, ~1 milhão de linhas):
//   - quase tudo é por dia, em [from, to) com from e to à meia-noite (UTC). Um dia que já
//     acabou não muda mais, então a API guarda os dias fechados e só pergunta de novo o dia
//     de hoje (api/admin_panels.go); cada consulta lê um dia só;
//   - a migração 00028 cria índices que cobrem as colunas lidas (index-only scans, sem
//     tocar na tabela);
//   - dentro do intervalo, um dia de cada vez (generate_series + LATERAL): o planejador não
//     sabe estimar quantos grupos dá date_trunc(at) e, achando que seriam milhares,
//     ordenaria todas as linhas em vez de agregar num hash pequeno;
//   - tudo roda numa transação só de leitura sem JIT (ver report), que nessas consultas
//     curtas custa mais do que ganha.

// report roda fn numa transação só de leitura, sem JIT e com um pouco mais de memória para
// agregar (as duas coisas só valem dentro dela).
func (s *Store) report(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL jit = off; SET LOCAL work_mem = '16MB'`); err != nil {
			return err
		}
		return fn(tx)
	})
}

// reportRows é uma consulta de relatório (ver report) já lida em []T.
func reportRows[T any](ctx context.Context, s *Store, scan pgx.RowToFunc[T], sql string, args ...any) ([]T, error) {
	var out []T
	err := s.report(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, scan)
		return err
	})
	return out, err
}

// perDay monta "um dia de cada vez" em [$1, $2): d.day percorre as meias-noites (UTC), e a
// consulta de dentro (inner) filtra por "at >= d.day AND at < n.next". Os outros
// parâmetros começam em $3.
func perDay(cols, inner string) string {
	return `SELECT to_char(d.day AT TIME ZONE 'UTC', 'YYYY-MM-DD'), ` + cols + `
		FROM generate_series($1::timestamptz, $2::timestamptz - interval '24 hours', interval '24 hours') AS d(day)
		CROSS JOIN LATERAL (SELECT d.day + interval '24 hours' AS next) n
		CROSS JOIN LATERAL (` + inner + `) g`
}

// DayStart é a meia-noite (UTC) do dia de t.
func DayStart(t time.Time) time.Time { return t.UTC().Truncate(24 * time.Hour) }

// GoldDayRow: ouro que entrou e saiu num dia, por origem (v_gold_daily).
type GoldDayRow struct {
	Day    string `json:"day"` // AAAA-MM-DD (UTC)
	Source string `json:"source"`
	In     int64  `json:"in"`
	Out    int64  `json:"out"`
}

// GoldDaily é o v_gold_daily de [from, to).
func (s *Store) GoldDaily(ctx context.Context, from, to time.Time) ([]GoldDayRow, error) {
	return reportRows(ctx, s, func(r pgx.CollectableRow) (GoldDayRow, error) {
		var g GoldDayRow
		return g, r.Scan(&g.Day, &g.Source, &g.In, &g.Out)
	}, perDay(`g.ref, g.i, g.o`, `
		SELECT ref, coalesce(sum(amount) FILTER (WHERE amount > 0), 0)::bigint AS i,
		       coalesce(-sum(amount) FILTER (WHERE amount < 0), 0)::bigint AS o
		FROM game_events WHERE kind = 'gold' AND at >= d.day AND at < n.next
		GROUP BY ref`)+` ORDER BY 1, 2`, from, to)
}

// LevelTimeRow: minutos médios (relógio) para chegar a cada nível (v_level_times).
type LevelTimeRow struct {
	Level      int     `json:"level"`
	Characters int64   `json:"characters"`
	AvgMinutes float64 `json:"avgMinutes"`
}

// LevelTimes é o v_level_times das subidas desde since (o nível anterior também tem de
// estar no período). Não dá para fazer por dia: a subida de hoje depende da de ontem.
func (s *Store) LevelTimes(ctx context.Context, since time.Time) ([]LevelTimeRow, error) {
	return reportRows(ctx, s, func(r pgx.CollectableRow) (LevelTimeRow, error) {
		var l LevelTimeRow
		return l, r.Scan(&l.Level, &l.Characters, &l.AvgMinutes)
	}, `
		WITH ups AS (
			SELECT amount AS nivel, at, lag(at) OVER (PARTITION BY character_id ORDER BY at) AS antes
			FROM game_events WHERE kind = 'level' AND at >= $1
		)
		SELECT nivel::int, count(*), round(avg(extract(epoch FROM at - antes)) / 60, 1)::float8
		FROM ups WHERE antes IS NOT NULL
		GROUP BY nivel ORDER BY nivel`, since)
}

// DeathDayRow: mortes num dia por região e por quem matou (v_deaths_by_region, somável).
type DeathDayRow struct {
	Day      string
	Region   string
	Cause    string
	Deaths   int64
	LevelSum int64
}

// DeathsDaily é o v_deaths_by_region de [from, to), por dia.
func (s *Store) DeathsDaily(ctx context.Context, from, to time.Time) ([]DeathDayRow, error) {
	return reportRows(ctx, s, func(r pgx.CollectableRow) (DeathDayRow, error) {
		var d DeathDayRow
		return d, r.Scan(&d.Day, &d.Region, &d.Cause, &d.Deaths, &d.LevelSum)
	}, perDay(`g.region, g.ref, g.n, g.lv`, `
		SELECT erevia_region(x, z) AS region, ref, count(*) AS n, sum(level)::bigint AS lv
		FROM game_events WHERE kind = 'death' AND at >= d.day AND at < n.next
		GROUP BY 1, 2`), from, to)
}

// LevelCount: personagens por nível.
type LevelCount struct {
	Level      int   `json:"level"`
	Characters int64 `json:"characters"`
}

// QuitLevel é o v_quit_level: último nível de quem não joga há mais de 7 dias.
func (s *Store) QuitLevel(ctx context.Context) ([]LevelCount, error) {
	return reportRows(ctx, s, func(r pgx.CollectableRow) (LevelCount, error) {
		var l LevelCount
		return l, r.Scan(&l.Level, &l.Characters)
	}, `SELECT nivel, personagens FROM v_quit_level`)
}

// FPSDayRow: amostras de FPS num dia por região e aparelho (v_fps, somável: médias saem
// das somas).
type FPSDayRow struct {
	Day     string
	Region  string
	Device  string
	Samples int64
	FPSSum  float64
	LowSum  float64
	LowN    int64
}

// FPSDaily é o v_fps de [from, to), por dia. Os grupos são os mesmos da estatística
// game_events_fps_stats (00028), para o planejador saber que são poucos.
func (s *Store) FPSDaily(ctx context.Context, from, to time.Time) ([]FPSDayRow, error) {
	return reportRows(ctx, s, func(r pgx.CollectableRow) (FPSDayRow, error) {
		var f FPSDayRow
		return f, r.Scan(&f.Day, &f.Region, &f.Device, &f.Samples, &f.FPSSum, &f.LowSum, &f.LowN)
	}, perDay(`g.region, g.device, g.n, g.fps, g.low, g.lown`, `
		SELECT erevia_region(x, z) AS region, coalesce(extra->>'device', '') AS device, count(*) AS n,
		       sum(amount)::float8 AS fps, coalesce(sum((extra->>'fps_low')::float8), 0) AS low,
		       count(extra->>'fps_low') AS lown
		FROM game_events WHERE kind = 'fps' AND at >= d.day AND at < n.next
		GROUP BY erevia_region(x, z), extra->>'device'`), from, to)
}

// ItemsDayRow: itens que entraram e saíram do jogo num dia, por meio (v_items_daily
// somado por dia e por "como").
type ItemsDayRow struct {
	Day string `json:"day"`
	How string `json:"how"`
	In  int64  `json:"in"`
	Out int64  `json:"out"`
}

// ItemsDaily é o v_items_daily de [from, to), por dia e meio.
func (s *Store) ItemsDaily(ctx context.Context, from, to time.Time) ([]ItemsDayRow, error) {
	return reportRows(ctx, s, func(r pgx.CollectableRow) (ItemsDayRow, error) {
		var d ItemsDayRow
		return d, r.Scan(&d.Day, &d.How, &d.In, &d.Out)
	}, perDay(`g.how, g.i, g.o`, `
		SELECT coalesce(extra->>'how', '') AS how,
		       coalesce(sum(amount) FILTER (WHERE kind = 'item_in'), 0)::bigint AS i,
		       coalesce(sum(amount) FILTER (WHERE kind = 'item_out'), 0)::bigint AS o
		FROM game_events WHERE kind IN ('item_in', 'item_out') AND at >= d.day AND at < n.next
		GROUP BY 1`)+` ORDER BY 1, 2`, from, to)
}

// ItemFlowRow: um item (entrou e saiu); Day vazio quando é o total do período.
type ItemFlowRow struct {
	Day  string `json:"day,omitempty"`
	Item string `json:"item"`
	In   int64  `json:"in"`
	Out  int64  `json:"out"`
}

// ItemsByItemDaily: cada item que entrou ou saiu, por dia, em [from, to).
func (s *Store) ItemsByItemDaily(ctx context.Context, from, to time.Time) ([]ItemFlowRow, error) {
	return reportRows(ctx, s, func(r pgx.CollectableRow) (ItemFlowRow, error) {
		var f ItemFlowRow
		return f, r.Scan(&f.Day, &f.Item, &f.In, &f.Out)
	}, perDay(`g.ref, g.i, g.o`, `
		SELECT ref, coalesce(sum(amount) FILTER (WHERE kind = 'item_in'), 0)::bigint AS i,
		       coalesce(sum(amount) FILTER (WHERE kind = 'item_out'), 0)::bigint AS o
		FROM game_events WHERE kind IN ('item_in', 'item_out') AND at >= d.day AND at < n.next
		GROUP BY ref`), from, to)
}

// ServerEventKinds são os eventos de erro e saúde do servidor em game_events.
var ServerEventKinds = []string{"server_error", "slow_tick", "db_error", "invariant"}

// ServerEventDay: quantas vezes um grupo (tipo + ref) aconteceu num dia (amount soma as
// repetições de cada linha) e a última vez.
type ServerEventDay struct {
	Day   string    `json:"day"`
	Kind  string    `json:"kind"`
	Ref   string    `json:"ref,omitempty"`
	Count int64     `json:"count"`
	Last  time.Time `json:"-"`
}

// ServerEventsDaily conta os erros do servidor por dia e grupo em [from, to).
func (s *Store) ServerEventsDaily(ctx context.Context, from, to time.Time) ([]ServerEventDay, error) {
	return reportRows(ctx, s, func(r pgx.CollectableRow) (ServerEventDay, error) {
		var d ServerEventDay
		return d, r.Scan(&d.Day, &d.Kind, &d.Ref, &d.Count, &d.Last)
	}, perDay(`g.kind, g.ref, g.n, g.last`, `
		SELECT kind, ref, sum(greatest(amount, 1))::bigint AS n, max(at) AS last
		FROM game_events WHERE kind = ANY($3) AND at >= d.day AND at < n.next
		GROUP BY 1, 2`), from, to, ServerEventKinds)
}

// ServerEventTop: um grupo de erro (tipo + ref) com o contexto da última vez.
type ServerEventTop struct {
	Kind    string    `json:"kind"`
	Ref     string    `json:"ref"`
	Count   int64     `json:"count"`
	Last    time.Time `json:"last"`
	Where   string    `json:"where"`
	Message string    `json:"message"`
	Version string    `json:"version"`
}

// ServerEventContext preenche onde, a mensagem e a versão da última linha de cada grupo
// (kind, ref e last já preenchidos; uma busca pelo índice por grupo).
func (s *Store) ServerEventContext(ctx context.Context, tops []ServerEventTop) error {
	if len(tops) == 0 {
		return nil
	}
	kinds := make([]string, len(tops))
	refs := make([]string, len(tops))
	lasts := make([]time.Time, len(tops))
	for i, t := range tops {
		kinds[i], refs[i], lasts[i] = t.Kind, t.Ref, t.Last
	}
	type ctxRow struct{ where, msg, version string }
	rows, err := reportRows(ctx, s, func(r pgx.CollectableRow) (ctxRow, error) {
		var c ctxRow
		return c, r.Scan(&c.where, &c.msg, &c.version)
	}, `
		SELECT coalesce(e.extra->>'where', ''),
		       coalesce(left(coalesce(e.extra->>'panic', e.extra->>'err', e.extra->>'value', e.extra->>'systems'), 300), ''),
		       coalesce(e.extra->>'version', '')
		FROM unnest($1::text[], $2::text[], $3::timestamptz[]) WITH ORDINALITY AS t(kind, ref, last, i)
		LEFT JOIN LATERAL (
			SELECT extra FROM game_events g WHERE g.kind = t.kind AND g.at = t.last AND g.ref = t.ref LIMIT 1
		) e ON true
		ORDER BY t.i`, kinds, refs, lasts)
	if err != nil {
		return err
	}
	for i := range rows {
		if i < len(tops) {
			tops[i].Where, tops[i].Message, tops[i].Version = rows[i].where, rows[i].msg, rows[i].version
		}
	}
	return nil
}

// PlayersDay: personagens distintos que jogaram num dia (pelos eventos de sessão).
type PlayersDay struct {
	Day        string `json:"day"`
	Characters int64  `json:"characters"`
	Sessions   int64  `json:"sessions"`
	Minutes    int64  `json:"minutes"`
}

// PlayersDaily conta quem jogou por dia (eventos session) em [from, to).
func (s *Store) PlayersDaily(ctx context.Context, from, to time.Time) ([]PlayersDay, error) {
	return reportRows(ctx, s, func(r pgx.CollectableRow) (PlayersDay, error) {
		var p PlayersDay
		return p, r.Scan(&p.Day, &p.Characters, &p.Sessions, &p.Minutes)
	}, perDay(`g.c, g.s, g.m`, `
		SELECT count(DISTINCT character_id) AS c, count(*) FILTER (WHERE ref = 'start') AS s,
		       (coalesce(sum(amount) FILTER (WHERE ref = 'end'), 0) / 60)::bigint AS m
		FROM game_events WHERE kind = 'session' AND at >= d.day AND at < n.next
		HAVING count(*) > 0`)+` ORDER BY 1`, from, to)
}

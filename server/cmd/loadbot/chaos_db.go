package main

import (
	"context"
	"fmt"
	"maps"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"erevia/server/internal/game"
)

// Banco do caos: o preparo dos personagens, o saldo (moedas e itens) e a telemetria.

type chaosDB struct{ pool *pgxpool.Pool }

func openChaosDB(ctx context.Context, url string) (*chaosDB, error) {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(cctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(cctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &chaosDB{pool: pool}, nil
}

func (d *chaosDB) close() { d.pool.Close() }

func (d *chaosDB) accountsOf(ctx context.Context, ids []int64) ([]int64, error) {
	rows, err := d.pool.Query(ctx, `SELECT DISTINCT account_id FROM characters WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// ---------- Preparo ----------

// prepItem é um item dado no preparo (na bolsa ou no armazém).
type prepItem struct {
	id      string
	count   int
	enchant int
}

// prep é o que um personagem recebe antes de jogar (tudo pela semente).
type prep struct {
	level       int
	gold        int64
	bag         []prepItem
	storageGold int64
	storage     []prepItem
	// rep: exaltado com as facções (a loja do Quadro de Contratos abre inteira); só no nível
	// dos contratos.
	rep bool
}

// prepPool: o que vale a pena ter para o caos chegar longe (encantar, criar, trocar, vender).
var prepPool = []prepItem{
	{"scroll_ench_weapon", 4, 0}, {"scroll_ench_armor", 4, 0}, {"scroll_ench_weapon_b", 1, 0},
	{"scroll_ench_armor_b", 1, 0}, {"scroll_return", 2, 0}, {"potion_hp_s", 10, 0}, {"potion_mp_s", 10, 0},
	{"potion_hp_l", 3, 0}, {"bone_old", 45, 0}, {"skull_cracked", 14, 0}, {"shadow_essence", 8, 0},
	{"rune_core", 6, 0}, {"armor_common", 1, 0}, {"helm_common", 1, 2}, {"ring_common", 1, 0},
	{"ring_good", 1, 3}, {"neck_common", 1, 0}, {"ear_common", 1, 1}, {"cape_common", 1, 0},
	{"gloves_common", 1, 4}, {"boots_common", 1, 0}, {"legs_common", 1, 0}, {"sword_common", 1, 3},
	{"staff_common", 1, 0}, {"daggers_common", 1, 5}, {"axe_common", 1, 0}, {"shield_badge", 1, 2},
	{"armor_good", 1, 0}, {"sword_good", 1, 4},
}

// prepFor sorteia o preparo do bot idx (outra corrente da semente: não mexe nas escolhas).
func prepFor(seed uint64, idx int) prep {
	r := rand.New(rand.NewPCG(seed^0xc0ffee, uint64(idx)+7))
	levels := []int{1, 2, 4, 6, 9, 10, 11, 12, 15, 18, 20, 22, game.ContractMinLevel, game.MaxLevel}
	golds := []int64{0, 40, 300, 1500, 5200, 9000, 14000, 30000}
	p := prep{level: levels[r.IntN(len(levels))], gold: golds[r.IntN(len(golds))]}
	pick := func(n int) []prepItem {
		var out []prepItem
		for range n {
			it := prepPool[r.IntN(len(prepPool))]
			def := game.Items[it.id]
			if def == nil {
				continue
			}
			it.count = max(1, min(def.Stack, 1+r.IntN(it.count)))
			if def.Stack > 1 {
				it.enchant = 0
			} else if it.enchant > 0 {
				it.enchant = r.IntN(it.enchant + 1)
			}
			out = append(out, it)
		}
		return out
	}
	p.bag = pick(6 + r.IntN(10))
	p.storageGold = []int64{0, 0, 120, 4000}[r.IntN(4)]
	p.storage = pick(r.IntN(5))
	p.rep = p.level >= game.ContractMinLevel && r.IntN(2) == 0
	return p
}

// prepare grava o preparo (personagem recém-criado, que ainda não entrou no jogo).
func (d *chaosDB) prepare(ctx context.Context, id uint32, class string, p prep) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		var acc int64
		if err := tx.QueryRow(ctx,
			`UPDATE characters SET level = $2, xp = $3, gold = gold + $4, reputation = $5 WHERE id = $1 RETURNING account_id`,
			id, p.level, game.XPLevelStart(p.level), p.gold, prepReputation(p.rep)).Scan(&acc); err != nil {
			return err
		}
		used := map[int]bool{}
		rows, err := tx.Query(ctx, `SELECT slot FROM character_items WHERE character_id = $1 AND place = 0`, id)
		if err != nil {
			return err
		}
		slots, err := pgx.CollectRows(rows, pgx.RowTo[int16])
		if err != nil {
			return err
		}
		for _, s := range slots {
			used[int(s)] = true
		}
		slot := 0
		for _, it := range p.bag {
			for slot < game.BagSize && used[slot] {
				slot++
			}
			if slot >= game.BagSize {
				break
			}
			used[slot] = true
			if _, err := tx.Exec(ctx, `INSERT INTO character_items (character_id, place, slot, item_id, count, enchant)
				VALUES ($1, 0, $2, $3, $4, $5)`, id, slot, it.id, it.count, it.enchant); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE accounts SET stored_gold = stored_gold + $2 WHERE id = $1`, acc, p.storageGold); err != nil {
			return err
		}
		for i, it := range p.storage {
			if _, err := tx.Exec(ctx, `INSERT INTO account_items (account_id, slot, item_id, count, enchant)
				VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`, acc, i, it.id, it.count, it.enchant); err != nil {
				return err
			}
		}
		return nil
	})
}

// ---------- Saldo ----------

// economy: moedas (bolsa + armazém) e itens por id (bolsa, equipamento e armazém).
type economy struct {
	gold      int64
	items     map[string]int64
	unsettled bool // o banco ainda mudava quando a espera acabou
}

func (e economy) totalItems() int64 {
	n := int64(0)
	for _, v := range e.items {
		n += v
	}
	return n
}

func (e economy) equal(o economy) bool { return e.gold == o.gold && maps.Equal(e.items, o.items) }

func (d *chaosDB) economy(ctx context.Context, ids, accounts []int64) (economy, error) {
	e := economy{items: map[string]int64{}}
	var g1, g2 int64
	if err := d.pool.QueryRow(ctx, `SELECT coalesce(sum(gold), 0)::bigint FROM characters WHERE id = ANY($1)`, ids).Scan(&g1); err != nil {
		return e, err
	}
	if err := d.pool.QueryRow(ctx, `SELECT coalesce(sum(stored_gold), 0)::bigint FROM accounts WHERE id = ANY($1)`, accounts).Scan(&g2); err != nil {
		return e, err
	}
	e.gold = g1 + g2
	rows, err := d.pool.Query(ctx, `
		SELECT item_id, sum(count)::bigint FROM (
			SELECT item_id, count FROM character_items WHERE character_id = ANY($1)
			UNION ALL
			SELECT item_id, count FROM account_items WHERE account_id = ANY($2)
		) t GROUP BY item_id`, ids, accounts)
	if err != nil {
		return e, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int64
		if err := rows.Scan(&id, &n); err != nil {
			return e, err
		}
		e.items[id] = n
	}
	return e, rows.Err()
}

// economyFlow: o que a telemetria diz que entrou e saiu (game_events gold, item_in, item_out).
type economyFlow struct {
	gold   int64
	goldBy map[string]int64 // por origem (loot, buy, sell...)
	in     map[string]int64
	out    map[string]int64
	how    map[string]map[string]int64 // item → "in:loot"/"out:sell" → quantidade
}

func (f economyFlow) howText(item string) string {
	m := f.how[item]
	if len(m) == 0 {
		return "sem telemetria"
	}
	var parts []string
	for _, k := range sortedKeys(m) {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

func (d *chaosDB) lastEventID(ctx context.Context) (int64, error) {
	var id int64
	err := d.pool.QueryRow(ctx, `SELECT coalesce(max(id), 0) FROM game_events`).Scan(&id)
	return id, err
}

func (d *chaosDB) flow(ctx context.Context, ids []int64, mark int64) (economyFlow, error) {
	f := economyFlow{goldBy: map[string]int64{}, in: map[string]int64{}, out: map[string]int64{}, how: map[string]map[string]int64{}}
	rows, err := d.pool.Query(ctx, `
		SELECT kind, ref, coalesce(extra->>'how', ''), sum(amount)::bigint FROM game_events
		WHERE id > $1 AND character_id = ANY($2) AND kind IN ('gold', 'item_in', 'item_out')
		GROUP BY 1, 2, 3`, mark, ids)
	if err != nil {
		return f, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, ref, how string
		var n int64
		if err := rows.Scan(&kind, &ref, &how, &n); err != nil {
			return f, err
		}
		switch kind {
		case "gold":
			f.gold += n
			f.goldBy[ref] += n
		case "item_in", "item_out":
			if f.how[ref] == nil {
				f.how[ref] = map[string]int64{}
			}
			if kind == "item_in" {
				f.in[ref] += n
				f.how[ref]["entrou:"+how] += n
			} else {
				f.out[ref] += n
				f.how[ref]["saiu:"+how] += n
			}
		}
	}
	return f, rows.Err()
}

// settledEconomy espera o tempo pedido e então lê saldo e telemetria até duas leituras
// seguidas iguais (os últimos salvamentos e o lote da telemetria podem estar a caminho).
func (d *chaosDB) settledEconomy(ctx context.Context, ids, accounts []int64, mark int64, settle time.Duration) (economy, economyFlow, error) {
	time.Sleep(settle)
	var last economy
	var lastFlow economyFlow
	for i := range 10 {
		e, err := d.economy(ctx, ids, accounts)
		if err != nil {
			return e, lastFlow, err
		}
		f, err := d.flow(ctx, ids, mark)
		if err != nil {
			return e, f, err
		}
		if i > 0 && e.equal(last) && f.gold == lastFlow.gold && maps.Equal(f.in, lastFlow.in) && maps.Equal(f.out, lastFlow.out) {
			return e, f, nil
		}
		last, lastFlow = e, f
		time.Sleep(3 * time.Second)
	}
	last.unsettled = true
	return last, lastFlow, nil
}

// ---------- Problemas ----------

type eventProblem struct {
	kind, ref string
	count     int64
	detail    string
}

// problems: invariantes, server_error e db_error gravados depois da marca (de qualquer
// personagem: um panic de sistema do tick não tem jogador), e a contagem de eventos por tipo.
func (d *chaosDB) problems(ctx context.Context, mark int64) ([]eventProblem, map[string]int64, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT kind, ref, sum(CASE WHEN kind = 'invariant' THEN 1 ELSE greatest(amount, 1) END)::bigint,
			(array_agg(extra ORDER BY id DESC))[1]
		FROM game_events WHERE id > $1 AND kind IN ('invariant', 'server_error', 'db_error')
		GROUP BY kind, ref ORDER BY kind, ref`, mark)
	if err != nil {
		return nil, nil, err
	}
	var out []eventProblem
	for rows.Next() {
		var p eventProblem
		var extra map[string]any
		if err := rows.Scan(&p.kind, &p.ref, &p.count, &extra); err != nil {
			rows.Close()
			return nil, nil, err
		}
		switch p.kind {
		case "invariant":
			p.detail = fmt.Sprintf("valor %v (personagem %v, corrigido: %v)", extra["value"], extra["name"], extra["fixed"])
		case "server_error":
			p.detail = fmt.Sprintf("onde %v, mensagem %v: %v; %v", extra["where"], extra["msg"], extra["panic"], extra["frames"])
		default:
			p.detail = fmt.Sprintf("%v", extra["err"])
		}
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	kinds := map[string]int64{}
	rows, err = d.pool.Query(ctx, `SELECT kind, count(*) FROM game_events WHERE id > $1 GROUP BY kind`, mark)
	if err != nil {
		return out, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int64
		if err := rows.Scan(&k, &n); err != nil {
			return out, kinds, err
		}
		kinds[k] = n
	}
	return out, kinds, rows.Err()
}

// prepReputation: a reputação do preparo (JSON de game.ReputationState).
func prepReputation(exalted bool) string {
	if !exalted {
		return "{}"
	}
	return fmt.Sprintf(`{"points": {%q: %d, %q: %d}}`, game.FactionBastion, game.RepMax, game.FactionGuard, game.RepMax)
}

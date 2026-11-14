package store

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
)

// Portal de administração, seção Economia: ouro e itens em circulação contra o que a
// telemetria diz que entrou e saiu do jogo, histórico por personagem e o gm_log.
//
// Como ler o saldo de um item: o que existe hoje (bolsas, equipamento e armazéns) deveria
// ser no máximo o que entrou (item_in) − o que saiu (item_out) + o kit inicial (que não
// gera evento). A troca entre jogadores e o armazém só mudam o item de lugar e não geram
// evento. Ficar abaixo é normal: poções bebidas, pergaminhos usados, itens que somem do
// chão ou de personagens apagados não são anotados. Ficar ACIMA é item que apareceu sem
// passar pela telemetria: suspeita de duplicação.

// saveLag: o personagem online vai ao banco a cada 30 s (zone.Config.SaveEvery), mas a
// telemetria chega em ~2 s. Saídas (ouro gasto, item vendido) mais novas que isto ficam de
// fora das contas de sobra: o banco ainda pode ter o que acabou de sair.
const saveLag = "2 minutes"

// GoldSupply é o ouro em circulação agora.
type GoldSupply struct {
	Bags       int64 `json:"bags"`    // characters.gold
	Storage    int64 `json:"storage"` // accounts.stored_gold
	Total      int64 `json:"total"`
	Characters int64 `json:"characters"` // personagens que já receberam o kit (e o ouro inicial)
	// Expected é o que a telemetria explica: entrou − saiu (todo o histórico) + ouro inicial.
	Expected int64 `json:"expected"`
	// Surplus = Total − Expected; acima de zero, ouro que apareceu sem evento.
	Surplus int64 `json:"surplus"`
}

// TelemetryTotals é o que entrou e saiu do jogo (ouro e itens) antes de um corte (a
// meia-noite de hoje). O passado não muda, então a API calcula uma vez por dia e só soma o
// de hoje a cada pedido (GoldSupply e ItemBalances).
type TelemetryTotals struct {
	Cut      time.Time
	Gold     int64
	ItemsIn  map[string]int64
	ItemsOut map[string]int64
}

// TelemetryTotalsBefore soma a telemetria de ouro e itens de antes de cut (todo o histórico).
func (s *Store) TelemetryTotalsBefore(ctx context.Context, cut time.Time) (TelemetryTotals, error) {
	t := TelemetryTotals{Cut: cut}
	var err error
	if t.Gold, err = s.GoldTotalBefore(ctx, cut); err != nil {
		return t, err
	}
	t.ItemsIn, t.ItemsOut, err = s.ItemTotalsBefore(ctx, cut)
	return t, err
}

// GoldTotalBefore: o saldo de ouro (entrou − saiu) de toda a telemetria antes de cut.
func (s *Store) GoldTotalBefore(ctx context.Context, cut time.Time) (int64, error) {
	var gold int64
	err := s.report(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT coalesce(sum(amount), 0)::bigint FROM game_events WHERE kind = 'gold' AND at < $1`, cut).Scan(&gold)
	})
	return gold, err
}

// ItemTotalsBefore: quanto de cada item entrou e saiu em toda a telemetria antes de cut.
func (s *Store) ItemTotalsBefore(ctx context.Context, cut time.Time) (in, out map[string]int64, err error) {
	in, out = map[string]int64{}, map[string]int64{}
	err = s.report(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT ref, coalesce(sum(amount) FILTER (WHERE kind = 'item_in'), 0)::bigint,
			       coalesce(sum(amount) FILTER (WHERE kind = 'item_out'), 0)::bigint
			FROM game_events WHERE kind IN ('item_in', 'item_out') AND at < $1 GROUP BY 1`, cut)
		if err != nil {
			return err
		}
		var id string
		var a, b int64
		_, err = pgx.ForEachRow(rows, []any{&id, &a, &b}, func() error {
			in[id], out[id] = a, b
			return nil
		})
		return err
	})
	return in, out, err
}

// GoldSupply soma o ouro de bolsas e armazéns e compara com a telemetria: before (o
// histórico até before.Cut) mais o que veio depois.
func (s *Store) GoldSupply(ctx context.Context, before TelemetryTotals) (GoldSupply, error) {
	var g GoldSupply
	var net int64
	err := s.report(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT
		(SELECT coalesce(sum(gold), 0)::bigint FROM characters),
		(SELECT coalesce(sum(stored_gold), 0)::bigint FROM accounts),
		(SELECT count(*) FROM characters WHERE kit_given),
		(SELECT coalesce(sum(amount) FILTER (WHERE amount > 0 OR at < now() - $1::interval), 0)::bigint
		 FROM game_events WHERE kind = 'gold' AND at >= $2)`, saveLag, before.Cut).
			Scan(&g.Bags, &g.Storage, &g.Characters, &net)
	})
	g.Total = g.Bags + g.Storage
	g.Expected = before.Gold + net + g.Characters*game.StarterGold
	g.Surplus = g.Total - g.Expected
	return g, err
}

// GoldNetDay: ouro que entrou e saiu num dia (todas as origens) e a circulação estimada
// no fim do dia (a de agora menos o saldo dos dias seguintes).
type GoldNetDay struct {
	Day    string `json:"day"`
	In     int64  `json:"in"`
	Out    int64  `json:"out"`
	Net    int64  `json:"net"`
	Supply int64  `json:"supply"`
}

// GoldNetDays junta as origens de cada dia (GoldDaily, em ordem de dia) e estima a
// circulação no fim de cada dia a partir da de agora.
func GoldNetDays(rows []GoldDayRow, supplyNow int64) []GoldNetDay {
	days := []GoldNetDay{}
	for _, r := range rows {
		if n := len(days); n == 0 || days[n-1].Day != r.Day {
			days = append(days, GoldNetDay{Day: r.Day})
		}
		d := &days[len(days)-1]
		d.In += r.In
		d.Out += r.Out
		d.Net = d.In - d.Out
	}
	supply := supplyNow
	for i := len(days) - 1; i >= 0; i-- {
		days[i].Supply = supply
		supply -= days[i].Net
	}
	return days
}

// ItemBalance é o saldo de um item: o que existe contra o que a telemetria explica.
type ItemBalance struct {
	Item     string `json:"item"`
	Name     string `json:"name"`
	Bags     int64  `json:"bags"`    // bolsas e equipamento (character_items)
	Storage  int64  `json:"storage"` // armazéns (account_items)
	Existing int64  `json:"existing"`
	In       int64  `json:"in"`
	Out      int64  `json:"out"`
	Kit      int64  `json:"kit"`      // teto do kit inicial (não gera evento)
	Expected int64  `json:"expected"` // In − Out + Kit
	Surplus  int64  `json:"surplus"`  // Existing − Expected (> 0: suspeita de duplicação)
}

// ItemBalances compara, item a item, o que existe hoje com a telemetria (todo o
// histórico). Os suspeitos (sobra > 0) vêm primeiro, da maior sobra para a menor.
func (s *Store) ItemBalances(ctx context.Context, before TelemetryTotals) ([]ItemBalance, error) {
	byID := map[string]*ItemBalance{}
	get := func(id string) *ItemBalance {
		b := byID[id]
		if b == nil {
			b = &ItemBalance{Item: id}
			byID[id] = b
		}
		return b
	}
	var id string
	var a, b int64
	err := s.report(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
		SELECT item_id, sum(bags)::bigint, sum(stored)::bigint FROM (
			SELECT item_id, count AS bags, 0 AS stored FROM character_items
			UNION ALL
			SELECT item_id, 0, count FROM account_items
		) t GROUP BY 1`)
		if err != nil {
			return err
		}
		if _, err := pgx.ForEachRow(rows, []any{&id, &a, &b}, func() error {
			it := get(id)
			it.Bags, it.Storage = a, b
			return nil
		}); err != nil {
			return err
		}
		// O que saiu há pouco pode ainda estar no banco (o personagem salva a cada 30 s):
		// essas saídas ficam de fora, senão o item vendido agora pareceria duplicado.
		rows, err = tx.Query(ctx, `
		SELECT ref, coalesce(sum(amount) FILTER (WHERE kind = 'item_in'), 0)::bigint,
		       coalesce(sum(amount) FILTER (WHERE kind = 'item_out' AND at < now() - $1::interval), 0)::bigint
		FROM game_events WHERE kind IN ('item_in', 'item_out') AND at >= $2 GROUP BY 1`, saveLag, before.Cut)
		if err != nil {
			return err
		}
		_, err = pgx.ForEachRow(rows, []any{&id, &a, &b}, func() error {
			it := get(id)
			it.In, it.Out = a, b
			return nil
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	for id, n := range before.ItemsIn {
		get(id).In += n
	}
	for id, n := range before.ItemsOut {
		get(id).Out += n
	}
	kit, err := s.kitAllowance(ctx)
	if err != nil {
		return nil, err
	}
	for id, n := range kit {
		get(id).Kit = n
	}
	out := make([]ItemBalance, 0, len(byID))
	for _, it := range byID {
		it.Existing = it.Bags + it.Storage
		it.Expected = it.In - it.Out + it.Kit
		it.Surplus = it.Existing - it.Expected
		if d := game.Items[it.Item]; d != nil {
			it.Name = d.Name
		}
		out = append(out, *it)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Surplus != out[j].Surplus {
			return out[i].Surplus > out[j].Surplus
		}
		return out[i].Item < out[j].Item
	})
	return out, nil
}

// kitAllowance é o teto do kit inicial por item: cada personagem que recebeu o kit ganhou
// uma das armas (e um dos escudos) que a classe oferece, mais as poções. Como a escolha não
// fica gravada, cada opção conta como se todos a tivessem escolhido (um teto, não a conta
// exata): duplicar uma arma inicial só aparece acima dele.
func (s *Store) kitAllowance(ctx context.Context) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT class, count(*) FROM characters WHERE kit_given GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	var class string
	var n int64
	_, err = pgx.ForEachRow(rows, []any{&class, &n}, func() error {
		for id, c := range KitCeiling(class) {
			out[id] += c * n
		}
		return nil
	})
	return out, err
}

// KitCeiling é o máximo de cada item que um personagem da classe pode ter recebido no kit
// inicial (game.StarterKitFor com cada aparência possível).
func KitCeiling(class string) map[string]int64 {
	opts, ok := game.StarterLooks[class]
	if !ok {
		return nil
	}
	out := map[string]int64{}
	for _, w := range opts.Weapons {
		for _, o := range opts.Offhands {
			look := game.DefaultLook(class)
			look.Weapon, look.Offhand = w, o
			for _, it := range game.StarterKitFor(class, look) {
				out[it.ItemID] = max(out[it.ItemID], int64(it.Count))
			}
		}
	}
	return out
}

// CharacterRef identifica um personagem (busca do histórico).
type CharacterRef struct {
	ID        int64  `json:"id"`
	AccountID int64  `json:"accountId"`
	Name      string `json:"name"`
	Class     string `json:"class"`
	Level     int    `json:"level"`
	Gold      int64  `json:"gold"`
}

// FindCharacter acha um personagem pelo nome (sem diferenciar maiúsculas) ou pelo id.
func (s *Store) FindCharacter(ctx context.Context, nameOrID string) (CharacterRef, error) {
	var c CharacterRef
	err := s.pool.QueryRow(ctx, `SELECT id, account_id, name, class, level, gold FROM characters
		WHERE lower(name) = lower($1) OR id::text = $1 ORDER BY (lower(name) = lower($1)) DESC LIMIT 1`,
		strings.TrimSpace(nameOrID)).Scan(&c.ID, &c.AccountID, &c.Name, &c.Class, &c.Level, &c.Gold)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// CharacterDay: ouro e itens de um personagem num dia.
type CharacterDay struct {
	Day      string `json:"day"`
	Level    int    `json:"level"` // o maior nível do dia
	GoldIn   int64  `json:"goldIn"`
	GoldOut  int64  `json:"goldOut"`
	ItemsIn  int64  `json:"itemsIn"`
	ItemsOut int64  `json:"itemsOut"`
}

// CharacterFlow: de onde veio (ou para onde foi) ouro ou item de um personagem no período.
type CharacterFlow struct {
	Kind   string `json:"kind"` // gold, item_in, item_out
	Ref    string `json:"ref"`  // gold: a origem; itens: o item
	How    string `json:"how"`  // itens: loot, buy, craft…
	Amount int64  `json:"amount"`
	Times  int64  `json:"times"`
}

// CharacterHistory é o ouro e os itens do personagem por dia no período, e as origens.
func (s *Store) CharacterHistory(ctx context.Context, characterID int64, since time.Time) ([]CharacterDay, []CharacterFlow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(date_trunc('day', at), 'YYYY-MM-DD'), max(level),
		       coalesce(sum(amount) FILTER (WHERE kind = 'gold' AND amount > 0), 0)::bigint,
		       coalesce(-sum(amount) FILTER (WHERE kind = 'gold' AND amount < 0), 0)::bigint,
		       coalesce(sum(amount) FILTER (WHERE kind = 'item_in'), 0)::bigint,
		       coalesce(sum(amount) FILTER (WHERE kind = 'item_out'), 0)::bigint
		FROM game_events WHERE character_id = $1 AND at >= $2 AND kind IN ('gold', 'item_in', 'item_out', 'level')
		GROUP BY 1 ORDER BY 1`, characterID, since)
	if err != nil {
		return nil, nil, err
	}
	days, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (CharacterDay, error) {
		var d CharacterDay
		return d, r.Scan(&d.Day, &d.Level, &d.GoldIn, &d.GoldOut, &d.ItemsIn, &d.ItemsOut)
	})
	if err != nil {
		return nil, nil, err
	}
	rows, err = s.pool.Query(ctx, `
		SELECT kind, ref, CASE WHEN kind = 'gold' THEN '' ELSE coalesce(extra->>'how', '') END, sum(amount)::bigint, count(*)
		FROM game_events WHERE character_id = $1 AND at >= $2 AND kind IN ('gold', 'item_in', 'item_out')
		GROUP BY 1, 2, 3 ORDER BY abs(sum(amount)) DESC LIMIT 40`, characterID, since)
	if err != nil {
		return nil, nil, err
	}
	flows, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (CharacterFlow, error) {
		var f CharacterFlow
		return f, r.Scan(&f.Kind, &f.Ref, &f.How, &f.Amount, &f.Times)
	})
	return days, flows, err
}

// Spike é um dia fora da curva de um personagem: ganhou (ouro ou itens) Factor vezes a
// média dos outros personagens do mesmo nível (±1) no período.
type Spike struct {
	CharacterID int64   `json:"characterId"`
	Name        string  `json:"name"`
	Day         string  `json:"day"`
	Level       int     `json:"level"`
	What        string  `json:"what"` // gold ou items
	Amount      int64   `json:"amount"`
	Baseline    float64 `json:"baseline"` // média diária dos outros do mesmo nível
	Ratio       float64 `json:"ratio"`
}

// SpikeRule: quando um dia vira alerta.
type SpikeRule struct {
	Factor     float64 // vezes a média (padrão 10)
	MinGold    int64   // ouro mínimo no dia para alertar (padrão 500)
	MinItems   int64   // itens mínimos no dia (padrão 30)
	MinSamples int     // dias-personagem dos outros no mesmo nível para haver média (padrão 3)
}

func (r *SpikeRule) defaults() {
	if r.Factor <= 0 {
		r.Factor = 10
	}
	if r.MinGold <= 0 {
		r.MinGold = 500
	}
	if r.MinItems <= 0 {
		r.MinItems = 30
	}
	if r.MinSamples <= 0 {
		r.MinSamples = 3
	}
}

// CharDay é o que um personagem ganhou num dia (ouro e itens que entraram).
type CharDay struct {
	Day   string
	Char  int64
	Level int // o maior nível do dia
	Gold  int64
	Items int64
}

// SpikeDays lê o ganho de cada personagem por dia em [from, to).
func (s *Store) SpikeDays(ctx context.Context, from, to time.Time) ([]CharDay, error) {
	return reportRows(ctx, s, func(r pgx.CollectableRow) (CharDay, error) {
		var cd CharDay
		return cd, r.Scan(&cd.Day, &cd.Char, &cd.Level, &cd.Gold, &cd.Items)
	}, perDay(`g.character_id, g.lv, g.gold, g.items`, `
		SELECT character_id, max(level) AS lv,
		       coalesce(sum(amount) FILTER (WHERE kind = 'gold' AND amount > 0), 0)::bigint AS gold,
		       coalesce(sum(amount) FILTER (WHERE kind = 'item_in'), 0)::bigint AS items
		FROM game_events
		WHERE kind IN ('gold', 'item_in') AND at >= d.day AND at < n.next AND character_id IS NOT NULL
		GROUP BY character_id`), from, to)
}

// Spikes procura os dias fora da curva desde since (ou de um personagem só,
// characterID > 0), já com os nomes.
func (s *Store) Spikes(ctx context.Context, since time.Time, characterID int64, rule SpikeRule) ([]Spike, error) {
	days, err := s.SpikeDays(ctx, DayStart(since), DayStart(time.Now()).Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	out := FindSpikes(days, characterID, rule)
	return out, s.NameSpikes(ctx, out)
}

// FindSpikes acha os dias fora da curva (ou de um personagem só, characterID > 0). A
// média de comparação é a dos OUTROS personagens do mesmo nível (±1): com poucos
// jogadores, o próprio pico puxaria a média para cima e se esconderia.
func FindSpikes(all []CharDay, characterID int64, rule SpikeRule) []Spike {
	rule.defaults()
	type acc struct {
		n           int
		gold, items int64
	}
	byLevel := map[int]*acc{}
	for _, d := range all {
		a := byLevel[d.Level]
		if a == nil {
			a = &acc{}
			byLevel[d.Level] = a
		}
		a.n++
		a.gold += d.Gold
		a.items += d.Items
	}
	out := []Spike{}
	for _, d := range all {
		if characterID > 0 && d.Char != characterID {
			continue
		}
		var n int
		var gold, items int64
		for l := d.Level - 1; l <= d.Level+1; l++ {
			if a := byLevel[l]; a != nil {
				n, gold, items = n+a.n, gold+a.gold, items+a.items
			}
		}
		n, gold, items = n-1, gold-d.Gold, items-d.Items // sem o próprio dia
		if n < rule.MinSamples {
			continue
		}
		check := func(what string, v, sum, floor int64) {
			base := float64(sum) / float64(n)
			if v < floor || float64(v) < rule.Factor*max(base, 1) {
				return
			}
			out = append(out, Spike{CharacterID: d.Char, Day: d.Day, Level: d.Level, What: what, Amount: v,
				Baseline: float64(int64(base*10)) / 10, Ratio: float64(int64(float64(v)/max(base, 1)*10)) / 10})
		}
		check("gold", d.Gold, gold, rule.MinGold)
		check("items", d.Items, items, rule.MinItems)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ratio > out[j].Ratio })
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}

// NameSpikes preenche o nome do personagem de cada alerta.
func (s *Store) NameSpikes(ctx context.Context, out []Spike) error {
	if len(out) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(out))
	for _, sp := range out {
		ids = append(ids, sp.CharacterID)
	}
	names := map[int64]string{}
	rows, err := s.pool.Query(ctx, `SELECT id, name FROM characters WHERE id = ANY($1)`, ids)
	if err != nil {
		return err
	}
	var id int64
	var name string
	if _, err := pgx.ForEachRow(rows, []any{&id, &name}, func() error {
		names[id] = name
		return nil
	}); err != nil {
		return err
	}
	for i := range out {
		out[i].Name = names[out[i].CharacterID]
		if out[i].Name == "" {
			out[i].Name = "(apagado)"
		}
	}
	return nil
}

// GMLogFilter filtra o gm_log. Campos vazios não filtram.
type GMLogFilter struct {
	GM      string // nome do GM (contém)
	Command string // começo do comando (ex.: "portal:", "give")
	Target  string // alvo (contém)
	Query   string // qualquer campo (contém)
	Since   time.Time
	Before  int64 // paginação: ids menores que este
	Limit   int
}

// GMLog devolve o gm_log filtrado, do mais novo ao mais velho.
func (s *Store) GMLog(ctx context.Context, f GMLogFilter) ([]GMLogRow, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 100
	}
	opt := func(v string) *string {
		if v = strings.TrimSpace(v); v == "" {
			return nil
		}
		p := likePattern(v)
		return &p
	}
	var cmd *string
	if c := strings.TrimSpace(f.Command); c != "" {
		p := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(c) + "%"
		cmd = &p
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, at, gm_account_id, gm_name, command, target, detail FROM gm_log
		WHERE ($1::text IS NULL OR gm_name ILIKE $1)
		  AND ($2::text IS NULL OR command ILIKE $2)
		  AND ($3::text IS NULL OR target ILIKE $3)
		  AND ($4::text IS NULL OR gm_name ILIKE $4 OR command ILIKE $4 OR target ILIKE $4 OR detail ILIKE $4)
		  AND at >= $5 AND ($6 = 0 OR id < $6)
		ORDER BY id DESC LIMIT $7`,
		opt(f.GM), cmd, opt(f.Target), opt(f.Query), f.Since, f.Before, f.Limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (GMLogRow, error) {
		var g GMLogRow
		err := r.Scan(&g.ID, &g.At, &g.GMAccountID, &g.GMName, &g.Command, &g.Target, &g.Detail)
		return g, err
	})
}

// GMLogCommands lista os comandos que já aparecem no gm_log (para o filtro), dos mais usados.
func (s *Store) GMLogCommands(ctx context.Context) ([]FacetCount, error) {
	rows, err := s.pool.Query(ctx, `SELECT command, 0, count(*) FROM gm_log GROUP BY 1 ORDER BY 3 DESC, 1 LIMIT 60`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (FacetCount, error) {
		var f FacetCount
		return f, r.Scan(&f.Value, &f.Open, &f.Total)
	})
}

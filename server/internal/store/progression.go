package store

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgtype"
)

// Seção "Progressão" do portal (views da 00018, docs/progressao.md): cada view vira uma
// tabela com as colunas na ordem do SQL. A lista é fixa no código (nada vem do pedido).

// ProgressionTable é uma view pronta para o portal.
type ProgressionTable struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Note    string   `json:"note"`
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

const progressionRowLimit = 500

var progressionViews = []struct{ view, title, note string }{
	{"v_level_playtime", "Tempo de jogo por nível",
		"Minutos de jogo (soma das sessões) para passar de nível a nível+1, por classe. Só subidas vistas do começo ao fim."},
	{"v_xp_sources", "XP por fonte", "De onde veio a XP em cada faixa (pct da XP ganha na faixa; death_loss é a perdida)."},
	{"v_level_kills_deaths", "Abates e mortes por nível", "Pelo nível do jogador no abate ou na morte."},
	{"v_gold_per_hour", "Ouro por hora de jogo", "Ouro ganho e gasto por hora jogada em cada faixa (sem o ouro criado por GM)."},
	{"v_level_reached", "Funil de níveis", "Personagens que chegaram a cada nível e os que pararam nele (sem entrar há mais de 7 dias)."},
}

// ProgressionReport lê as views da progressão (até progressionRowLimit linhas cada).
func (s *Store) ProgressionReport(ctx context.Context) ([]ProgressionTable, error) {
	out := make([]ProgressionTable, 0, len(progressionViews))
	for _, v := range progressionViews {
		t := ProgressionTable{ID: v.view, Title: v.title, Note: v.note, Rows: [][]any{}}
		rows, err := s.pool.Query(ctx, fmt.Sprintf("SELECT * FROM %s LIMIT %d", v.view, progressionRowLimit))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", v.view, err)
		}
		for _, f := range rows.FieldDescriptions() {
			t.Columns = append(t.Columns, f.Name)
		}
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				rows.Close()
				return nil, fmt.Errorf("%s: %w", v.view, err)
			}
			for i, val := range vals {
				switch n := val.(type) {
				case pgtype.Numeric: // round(...) do SQL: número no JSON
					f, _ := n.Float64Value()
					vals[i] = nil
					if f.Valid && !math.IsNaN(f.Float64) && !math.IsInf(f.Float64, 0) {
						vals[i] = f.Float64
					}
				case float64: // o JSON não tem NaN nem infinito
					if math.IsNaN(n) || math.IsInf(n, 0) {
						vals[i] = nil
					}
				}
			}
			t.Rows = append(t.Rows, vals)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("%s: %w", v.view, err)
		}
		out = append(out, t)
	}
	return out, nil
}

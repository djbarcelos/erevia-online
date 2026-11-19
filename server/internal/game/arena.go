package game

import (
	"math"
	"strconv"
	"time"
)

// Arena PvP ranqueada (tunnel#81): partidas de 1x1 (sozinho) e 3x3 (grupo de 3), numa cópia
// só da partida, com pontuação por personagem (Elo) e ranking por temporada (o mês).
//
// Na arena não há karma, flag, perda de XP nem itens caídos: quem morre fica fora até o fim
// da partida e volta com vida cheia. Vence o time que derrubar o outro; no fim do tempo, o
// que tiver mais vida somada (em fração de cada um). A pontuação muda pela diferença entre a
// média do time e a do adversário (Elo, ArenaK).

// ArenaMode é o tamanho do time.
type ArenaMode int

const (
	ArenaSolo  ArenaMode = 1 // 1x1
	ArenaTrios ArenaMode = 3 // 3x3 (um grupo de 3)
)

// ArenaModes: os modos, na ordem da janela.
var ArenaModes = []ArenaMode{ArenaSolo, ArenaTrios}

// Valid: o modo existe.
func (m ArenaMode) Valid() bool { return m == ArenaSolo || m == ArenaTrios }

// Label: "1x1" ou "3x3".
func (m ArenaMode) Label() string {
	if m == ArenaTrios {
		return "3x3"
	}
	return "1x1"
}

const (
	ArenaMinLevel     = 10               // nível mínimo para entrar na fila
	ArenaStartRating  = 1000             // pontuação de quem nunca lutou na temporada
	ArenaK            = 32               // quanto uma partida move a pontuação (Elo)
	ArenaCountdown    = 10 * time.Second // preparação: ninguém ataca
	ArenaFight        = 3 * time.Minute  // duração da luta
	ArenaAfter        = 8 * time.Second  // resultado na tela antes de voltar à cidade
	ArenaRankingLimit = 20               // linhas do ranking na janela
)

// ArenaSeason: a temporada do instante t (ano × 100 + mês, em Brasília: 202611).
func ArenaSeason(t time.Time) int {
	t = t.In(arenaTZ)
	return t.Year()*100 + int(t.Month())
}

// ArenaSeasonLabel: "novembro de 2026".
func ArenaSeasonLabel(season int) string {
	months := [...]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}
	m := season % 100
	if m < 1 || m > 12 {
		return ""
	}
	return months[m-1] + " de " + strconv.Itoa(season/100)
}

var arenaTZ = time.FixedZone("BRT", -3*3600) // a temporada vira à meia-noite de Brasília

// ArenaExpected: a chance esperada de o time de pontuação a vencer o de pontuação b (Elo).
func ArenaExpected(a, b float64) float64 { return 1 / (1 + math.Pow(10, (b-a)/400)) }

// ArenaDelta: quanto a pontuação de quem estava no time de média a muda contra o time de média
// b (won = venceu). Sempre ao menos 1 ponto para quem vence e para quem perde.
func ArenaDelta(a, b float64, won bool) int {
	score := 0.0
	if won {
		score = 1
	}
	d := int(math.Round(ArenaK * (score - ArenaExpected(a, b))))
	switch {
	case won && d < 1:
		d = 1
	case !won && d > -1:
		d = -1
	}
	return d
}

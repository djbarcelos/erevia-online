package balance

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

// Orçamento de poder (tolerâncias em budget.go): nenhuma peça acima do orçamento do nível
// e da raridade, nenhum +10 passando demais a raridade seguinte e nenhuma classe ou caminho
// longe da média da faixa em tempo para matar.
//
// O jogo de novembro de 2026 já fura vários casos (docs/balanceamento-2026-11.md). Eles
// ficam na lista abaixo com o número de hoje (quanto passou: +0,26 = 26% acima; negativo =
// abaixo da média), para o teste passar hoje e pegar regressões: falha um caso novo, ou um
// conhecido que piore mais que regressionSlack. Quem corrige um caso tira ele da lista (o
// teste avisa no log). Para regerar a lista depois de mudar o balanceamento de propósito:
//
//	BALANCE_PRINT=1 go test ./internal/balance -run TestPowerBudget -v
const regressionSlack = 0.02 // 2 pontos percentuais de folga antes de contar como piora

// Do 20 em diante a lista reflete as árvores das evoluções ainda incompletas (tunnel#180,
// Fase 5: só as skills de antes e as 2 iniciais). As fases de conteúdo de cada classe
// completam as árvores e regeram a lista.
var knownExceptions = map[string]float64{
	"enchant:armor_good":                 0.26,  // +10 Cota de Malha (58,0) contra Couraça de Osso +0 (46,0)
	"enchant:armor_rare":                 0.27,  // +10 Couraça de Osso (98,0) contra Manto do Senhor dos Ossos +0 (77,0)
	"enchant:axe_common":                 0.50,  // +10 Machado de Lenhador (24,0) contra Machado de Guerra +0 (16,0)
	"enchant:axe_epic":                   0.96,  // +10 Fúria do Norte (112,0) contra Machado do Rei Ossário +0 (57,0)
	"enchant:axe_good":                   0.72,  // +10 Machado de Guerra (50,0) contra Machado Rachador +0 (29,0)
	"enchant:axe_rare":                   0.82,  // +10 Machado Rachador (80,0) contra Fúria do Norte +0 (44,0)
	"enchant:boots_common":               0.83,  // +10 Botas de Viajante (22,0) contra Botas de Ferro +0 (12,0)
	"enchant:boots_good":                 1.09,  // +10 Botas de Ferro (46,0) contra Botas do Vento +0 (22,0)
	"enchant:boots_rare":                 1.18,  // +10 Botas do Vento (74,0) contra Passos da Sombra +0 (34,0)
	"enchant:cape_common":                1.32,  // +10 Capa de Lã (22,0) contra Capa do Patrulheiro +0 (9,5)
	"enchant:cape_good":                  1.49,  // +10 Capa do Patrulheiro (43,5) contra Capa do Crepúsculo +0 (17,5)
	"enchant:cape_rare":                  1.01,  // +10 Capa do Crepúsculo (69,5) contra Manto das Cinzas +0 (34,5)
	"enchant:daggers_common":             0.57,  // +10 Adagas Gêmeas (22,0) contra Adagas de Aço +0 (14,0)
	"enchant:daggers_epic":               1.16,  // +10 Presas do Crepúsculo (106,0) contra Garras do Rei Ossário +0 (49,0)
	"enchant:daggers_good":               0.92,  // +10 Adagas de Aço (48,0) contra Presas Sombrias +0 (25,0)
	"enchant:daggers_rare":               1.00,  // +10 Presas Sombrias (76,0) contra Presas do Crepúsculo +0 (38,0)
	"enchant:ear_common":                 0.46,  // +10 Brinco de Osso (11,4) contra Brinco de Âmbar +0 (7,8)
	"enchant:ear_good":                   0.50,  // +10 Brinco de Âmbar (24,8) contra Brinco de Safira +0 (16,5)
	"enchant:ear_rare":                   0.37,  // +10 Brinco de Safira (42,5) contra Lágrima da Lua +0 (31,0)
	"enchant:gloves_common":              1.24,  // +10 Luvas de Couro (20,8) contra Manoplas de Ferro +0 (9,2)
	"enchant:gloves_good":                1.16,  // +10 Manoplas de Ferro (43,2) contra Manoplas Rúnicas +0 (20,0)
	"enchant:gloves_rare":                1.18,  // +10 Manoplas Rúnicas (72,0) contra Garras do Ossário +0 (33,0)
	"enchant:helm_common":                0.66,  // +10 Capuz de Couro (24,0) contra Elmo de Ferro +0 (14,5)
	"enchant:helm_good":                  0.83,  // +10 Elmo de Ferro (48,5) contra Elmo de Osso +0 (26,5)
	"enchant:helm_rare":                  0.83,  // +10 Elmo de Osso (78,5) contra Elmo do Senhor dos Ossos +0 (43,0)
	"enchant:legs_common":                0.55,  // +10 Calças de Couro (24,0) contra Grevas de Malha +0 (15,5)
	"enchant:legs_good":                  0.62,  // +10 Grevas de Malha (49,5) contra Grevas de Osso +0 (30,5)
	"enchant:legs_rare":                  0.65,  // +10 Grevas de Osso (82,5) contra Grevas do Crepúsculo +0 (50,0)
	"enchant:neck_epic":                  0.39,  // +10 Coração do Abismo (79,0) contra Colar do Rei Ossário +0 (57,0)
	"enchant:ring_common":                1.00,  // +10 Anel de Cobre (12,0) contra Anel de Prata +0 (6,0)
	"enchant:ring_epic":                  0.87,  // +10 Selo do Senhor dos Ossos (58,0) contra Anel do Rei Ossário +0 (31,0)
	"enchant:ring_good":                  0.92,  // +10 Anel de Prata (23,0) contra Anel de Ônix +0 (12,0)
	"enchant:ring_rare":                  0.58,  // +10 Anel de Ônix (38,0) contra Selo do Senhor dos Ossos +0 (24,0)
	"enchant:shield_good":                0.62,  // +10 Escudo de Ferro (52,0) contra Escudo de Cravos +0 (32,0)
	"enchant:shield_rare":                0.53,  // +10 Escudo de Cravos (84,0) contra Égide Óssea +0 (55,0)
	"enchant:shield_tower":               0.36,  // +10 Escudo Pesado (24,5) contra Escudo de Ferro +0 (18,0)
	"enchant:staff_common":               0.32,  // +10 Cajado de Aprendiz (25,0) contra Cajado de Carvalho +0 (19,0)
	"enchant:staff_epic":                 0.69,  // +10 Cetro do Abismo (122,0) contra Cetro do Rei Ossário +0 (72,0)
	"enchant:staff_good":                 0.61,  // +10 Cajado de Carvalho (53,0) contra Varinha Rúnica +0 (33,0)
	"enchant:staff_rare":                 0.56,  // +10 Varinha Rúnica (84,0) contra Cetro do Abismo +0 (54,0)
	"enchant:sword_common":               0.64,  // +10 Espada de Treino (23,0) contra Espada de Aço +0 (14,0)
	"enchant:sword_epic":                 0.98,  // +10 Lâmina da Aurora (113,0) contra Lâmina do Rei Ossário +0 (57,0)
	"enchant:sword_good":                 0.78,  // +10 Espada de Aço (48,0) contra Lâmina do Vigia +0 (27,0)
	"enchant:sword_rare":                 0.73,  // +10 Lâmina do Vigia (78,0) contra Lâmina da Aurora +0 (45,0)
	"item:ear_epic":                      0.26,  // Lágrima da Lua (épica, nível 18): poder 31,0, orçamento 24,6
	"item:neck_epic":                     0.26,  // Coração do Abismo (épica, nível 18): poder 45,0, orçamento 35,8
	"item:neck_legend":                   0.29,  // Colar do Rei Ossário (lendária, nível 20): poder 57,0, orçamento 44,3
	"ttk:13–19:Cavaleiro":                0.53,  // Cavaleiro no 13–19: 2,6 s para matar, média da faixa 1,7 s
	"ttk:13–19:Mago":                     -0.45, // Mago no 13–19: 0,9 s para matar, média da faixa 1,7 s
	"ttk:1–5:Cavaleiro":                  0.96,  // Cavaleiro no 1–5: 3,1 s para matar, média da faixa 1,6 s
	"ttk:1–5:Mago":                       -0.86, // Mago no 1–5: 0,2 s para matar, média da faixa 1,6 s
	"ttk:20–24:Bárbaro/Berserker":        -0.56, // Bárbaro/Berserker no 20–24: 2,0 s para matar, média da faixa 4,6 s
	"ttk:20–24:Bárbaro/Senhor da Guerra": 0.27,  // Bárbaro/Senhor da Guerra no 20–24: 5,9 s para matar, média da faixa 4,6 s
	"ttk:20–24:Cavaleiro/Bastião":        1.40,  // Cavaleiro/Bastião no 20–24: 11,1 s para matar, média da faixa 4,6 s
	"ttk:20–24:Cavaleiro/Paladino":       0.27,  // Cavaleiro/Paladino no 20–24: 5,8 s para matar, média da faixa 4,6 s
	"ttk:20–24:Ladino/Assassino":         -0.41, // Ladino/Assassino no 20–24: 2,7 s para matar, média da faixa 4,6 s
	"ttk:20–24:Mago/Arcanista":           -0.23, // Mago/Arcanista no 20–24: 3,6 s para matar, média da faixa 4,6 s
	"ttk:20–24:Mago/Elementalista":       -0.64, // Mago/Elementalista no 20–24: 1,7 s para matar, média da faixa 4,6 s
	"ttk:25–29:Bárbaro/Berserker":        -0.55, // Bárbaro/Berserker no 25–29: 2,0 s para matar, média da faixa 4,5 s
	"ttk:25–29:Bárbaro/Senhor da Guerra": 0.28,  // Bárbaro/Senhor da Guerra no 25–29: 5,8 s para matar, média da faixa 4,5 s
	"ttk:25–29:Cavaleiro/Bastião":        1.50,  // Cavaleiro/Bastião no 25–29: 11,3 s para matar, média da faixa 4,5 s
	"ttk:25–29:Cavaleiro/Paladino":       0.30,  // Cavaleiro/Paladino no 25–29: 5,9 s para matar, média da faixa 4,5 s
	"ttk:25–29:Ladino/Assassino":         -0.44, // Ladino/Assassino no 25–29: 2,6 s para matar, média da faixa 4,5 s
	"ttk:25–29:Mago/Arcanista":           -0.33, // Mago/Arcanista no 25–29: 3,0 s para matar, média da faixa 4,5 s
	"ttk:25–29:Mago/Elementalista":       -0.64, // Mago/Elementalista no 25–29: 1,7 s para matar, média da faixa 4,5 s
	"ttk:30–34:Bárbaro/Berserker":        -0.50, // Bárbaro/Berserker no 30–34: 2,6 s para matar, média da faixa 5,2 s
	"ttk:30–34:Bárbaro/Senhor da Guerra": 0.29,  // Bárbaro/Senhor da Guerra no 30–34: 6,7 s para matar, média da faixa 5,2 s
	"ttk:30–34:Cavaleiro/Bastião":        1.41,  // Cavaleiro/Bastião no 30–34: 12,5 s para matar, média da faixa 5,2 s
	"ttk:30–34:Cavaleiro/Paladino":       0.31,  // Cavaleiro/Paladino no 30–34: 6,8 s para matar, média da faixa 5,2 s
	"ttk:30–34:Ladino/Assassino":         -0.38, // Ladino/Assassino no 30–34: 3,2 s para matar, média da faixa 5,2 s
	"ttk:30–34:Ladino/Patrulheiro":       -0.21, // Ladino/Patrulheiro no 30–34: 4,1 s para matar, média da faixa 5,2 s
	"ttk:30–34:Mago/Arcanista":           -0.30, // Mago/Arcanista no 30–34: 3,6 s para matar, média da faixa 5,2 s
	"ttk:30–34:Mago/Elementalista":       -0.63, // Mago/Elementalista no 30–34: 1,9 s para matar, média da faixa 5,2 s
	"ttk:35–40:Bárbaro/Berserker":        -0.35, // Bárbaro/Berserker no 35–40: 2,6 s para matar, média da faixa 4,0 s
	"ttk:35–40:Bárbaro/Senhor da Guerra": 0.30,  // Bárbaro/Senhor da Guerra no 35–40: 5,1 s para matar, média da faixa 4,0 s
	"ttk:35–40:Cavaleiro/Bastião":        1.45,  // Cavaleiro/Bastião no 35–40: 9,7 s para matar, média da faixa 4,0 s
	"ttk:35–40:Cavaleiro/Paladino":       0.39,  // Cavaleiro/Paladino no 35–40: 5,5 s para matar, média da faixa 4,0 s
	"ttk:35–40:Ladino/Assassino":         -0.45, // Ladino/Assassino no 35–40: 2,2 s para matar, média da faixa 4,0 s
	"ttk:35–40:Ladino/Patrulheiro":       -0.31, // Ladino/Patrulheiro no 35–40: 2,7 s para matar, média da faixa 4,0 s
	"ttk:35–40:Mago/Arcanista":           -0.35, // Mago/Arcanista no 35–40: 2,6 s para matar, média da faixa 4,0 s
	"ttk:35–40:Mago/Elementalista":       -0.67, // Mago/Elementalista no 35–40: 1,3 s para matar, média da faixa 4,0 s
	"ttk:6–12:Bárbaro":                   0.20,  // Bárbaro no 6–12: 2,1 s para matar, média da faixa 1,7 s
	"ttk:6–12:Cavaleiro":                 0.69,  // Cavaleiro no 6–12: 2,9 s para matar, média da faixa 1,7 s
	"ttk:6–12:Mago":                      -0.80, // Mago no 6–12: 0,3 s para matar, média da faixa 1,7 s
}

func TestPowerBudget(t *testing.T) {
	vs := AllViolations()
	if os.Getenv("BALANCE_PRINT") != "" {
		var b strings.Builder
		for _, v := range vs {
			fmt.Fprintf(&b, "\t%q: %.2f, // %s\n", v.Key, v.Excess, v.Detail)
		}
		t.Logf("lista de hoje:\n%s", b.String())
	}
	seen := map[string]bool{}
	for _, v := range vs {
		seen[v.Key] = true
		known, ok := knownExceptions[v.Key]
		switch {
		case !ok:
			t.Errorf("fora do orçamento (caso novo): %s %+.0f%%: %s", v.Key, v.Excess*100, v.Detail)
		case known*v.Excess < 0 || math.Abs(v.Excess) > math.Abs(known)+regressionSlack:
			t.Errorf("piorou: %s era %+.0f%%, agora %+.0f%%: %s", v.Key, known*100, v.Excess*100, v.Detail)
		}
	}
	for k, v := range knownExceptions {
		if !seen[k] {
			t.Logf("%s (%+.0f%%) voltou para dentro da tolerância: tire da lista de exceções", k, v*100)
		}
	}
}

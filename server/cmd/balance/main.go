// Comando balance: o simulador de balanceamento. Para cada classe e caminho, cada nível e o
// equipamento típico da faixa (+0 e +3), calcula o dano por segundo, o tempo para matar e
// para morrer contra o monstro do nível, a XP por hora e o tempo até cada nível e até o máximo,
// com as fórmulas reais do pacote game. Determinístico (média por fórmula).
//
//	go run ./cmd/balance          # tabelas em markdown
//	go run ./cmd/balance -csv     # uma linha por caminho, nível e encantamento
//	go run ./cmd/balance -noite   # monstros e XP da noite
//	go run ./cmd/balance -vitalidade # tempo até o máximo jogando 1 h por dia, com a vitalidade
//	go run ./cmd/balance -grupo      # XP/h de cada membro de um grupo de 3 contra caçar sozinho
//	go run ./cmd/balance -ouro       # ouro que entra e sai em cada grau (tunnel#154)
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"erevia/server/internal/balance"
)

func main() {
	asCSV := flag.Bool("csv", false, "saída em CSV (uma linha por caminho, nível e encantamento)")
	night := flag.Bool("noite", false, "de noite: monstros batem mais e dão mais XP")
	vitality := flag.Bool("vitalidade", false, "tempo até o máximo jogando poucas horas por dia, com a vitalidade (tunnel#150)")
	group := flag.Bool("grupo", false, "XP por hora em grupo contra sozinho, carregar e monstro fraco (tunnel#151)")
	gold := flag.Bool("ouro", false, "ouro que entra e sai em cada grau: saque, poções, peças e encantamento (tunnel#154)")
	flag.Parse()

	if *gold {
		balance.WriteGoldMarkdown(os.Stdout)
		return
	}

	if *group {
		balance.WriteGroupMarkdown(os.Stdout)
		return
	}
	if *vitality {
		balance.WriteVitalityMarkdown(os.Stdout)
		return
	}

	r := balance.BuildReport(*night)
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	if *asCSV {
		if err := r.WriteCSV(w); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	r.WriteMarkdown(w)
}

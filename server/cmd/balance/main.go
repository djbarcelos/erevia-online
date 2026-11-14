// Comando balance: o simulador de balanceamento. Para cada classe e caminho, cada nível e o
// equipamento típico da faixa (+0 e +3), calcula o dano por segundo, o tempo para matar e
// para morrer contra o monstro do nível, a XP por hora e o tempo até cada nível e até o máximo,
// com as fórmulas reais do pacote game. Determinístico (média por fórmula).
//
//	go run ./cmd/balance          # tabelas em markdown
//	go run ./cmd/balance -csv     # uma linha por caminho, nível e encantamento
//	go run ./cmd/balance -noite   # monstros e XP da noite
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
	flag.Parse()

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

package main

import (
	"reflect"
	"testing"

	"erevia/server/internal/game"
)

func TestPickActionCoversEveryAction(t *testing.T) {
	seen := map[string]int{}
	for r := range uint64(chaosTotalWeight) {
		seen[pickAction(r).name]++
	}
	for _, a := range chaosActions {
		if seen[a.name] != a.weight {
			t.Errorf("%s: sorteada %d vezes em %d, peso %d", a.name, seen[a.name], chaosTotalWeight, a.weight)
		}
	}
}

// O preparo vem só da semente (reprodutível) e só usa itens que existem, na quantidade
// que a pilha aceita.
func TestPrepIsDeterministicAndValid(t *testing.T) {
	for i := range 50 {
		a, b := prepFor(317, i), prepFor(317, i)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("bot %d: o mesmo preparo deu resultados diferentes", i)
		}
		if a.level < 1 || a.level > game.MaxLevel || a.gold < 0 {
			t.Fatalf("bot %d: nível %d, moedas %d", i, a.level, a.gold)
		}
		for _, it := range append(a.bag, a.storage...) {
			d := game.Items[it.id]
			if d == nil || it.count < 1 || it.count > d.Stack || (it.enchant > 0 && d.Stack > 1) || it.enchant > game.MaxEnchant {
				t.Fatalf("bot %d: item inválido no preparo %+v", i, it)
			}
		}
	}
	if reflect.DeepEqual(prepFor(1, 0), prepFor(2, 0)) && reflect.DeepEqual(prepFor(1, 1), prepFor(2, 1)) {
		t.Fatal("sementes diferentes deveriam dar preparos diferentes")
	}
}

func TestParseSeed(t *testing.T) {
	if s, err := parseSeed("045"); err != nil || s != 45 { // date +%j tem zero à esquerda
		t.Fatalf("parseSeed(045) = %d, %v", s, err)
	}
	if _, err := parseSeed("abc"); err == nil {
		t.Fatal("semente inválida deveria dar erro")
	}
}

package balance

import (
	"fmt"
	"io"
	"math/rand/v2"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// Ouro que entra e sai por faixa de grau (tunnel#154)
// ---------------------------------------------------------------------------------------
//
// O modelo segue um caçador sozinho, do nível mínimo do grau até o último nível antes do
// grau seguinte, com o equipamento típico (+3) e o ritmo do simulador (Simulate):
//
//	entra = moedas que caem + venda do saque na loja (SellPrice), por abate, menos os
//	        pergaminhos de encantar (que o caçador usa)
//	sai   = poções gastas (PotsH) e 1 Pergaminho de Retorno por hora
//	      + as peças novas do equipamento típico em cada troca da faixa (receita do Ferreiro
//	        ou preço da loja; o que só cai de monstro não custa moedas)
//	      + o encantamento de cada peça nova: a arma a +goldWeaponLevel e o resto a
//	        +goldEnchantLevel, com as tentativas, as taxas e as quebras esperadas (refazer a
//	        peça e voltar do +0). Os pergaminhos que caíram na faixa são usados primeiro; o
//	        que falta sai da loja.
//
// Vender tudo é o teto do que entra: quem cristaliza ou guarda material para receita vende
// menos. A média é das classes (os caminhos do 20 em diante).

// GoldBand é o ouro de uma faixa de grau.
type GoldBand struct {
	Tier    game.Tier
	Lo, Hi  int
	Hours   float64 // horas caçando da faixa (média dos caminhos)
	InDrops float64 // moedas que caem
	InSales float64 // venda do saque

	OutSupplies float64 // poções e retorno
	OutGear     float64 // peças novas
	OutEnchant  float64 // pergaminhos comprados, taxas e peças refeitas depois de quebrar

	ScrollsUsed, ScrollsDropped float64 // pergaminhos de encantar gastos e os que caíram
}

// In: o ouro que entra na faixa.
func (g GoldBand) In() float64 { return g.InDrops + g.InSales }

// Out: o ouro que sai na faixa.
func (g GoldBand) Out() float64 { return g.OutSupplies + g.OutGear + g.OutEnchant }

// Ratio: quanto do ouro que entra sai (0,8 = 80%).
func (g GoldBand) Ratio() float64 {
	if g.In() <= 0 {
		return 0
	}
	return g.Out() / g.In()
}

// Faixa-alvo da saída de ouro (o "Pronto quando" da tunnel#154).
const (
	GoldSinkMin = 0.60
	GoldSinkMax = 1.00
)

// goldEnchantLevel: o encantamento do equipamento típico no modelo; goldWeaponLevel: o da arma.
const (
	goldEnchantLevel = 5
	goldWeaponLevel  = 7
	goldHuntEnchant  = game.EnchantSafe // o encantamento do ritmo de caça (Simulate)
	goldLootSamples  = 600              // abates sorteados por monstro e nível (semente fixa)
)

// GoldFlow: as faixas dos graus D, C e B.
func GoldFlow() []GoldBand {
	var out []GoldBand
	for i, t := range game.Tiers {
		if t == game.TierNone {
			continue
		}
		hi := game.MaxLevel - 1
		if i+1 < len(game.Tiers) {
			hi = game.Tiers[i+1].MinLevel() - 1
		}
		out = append(out, goldBand(t, t.MinLevel(), hi))
	}
	return out
}

func goldBand(t game.Tier, lo, hi int) GoldBand {
	g := GoldBand{Tier: t, Lo: lo, Hi: hi}
	paths := Paths()
	dropped := map[bool]float64{} // pergaminhos do grau que caíram: arma (true) e armadura
	for l := lo; l <= hi; l++ {
		var hours, drops, sales, supplies float64
		got := map[bool]float64{}
		for _, p := range paths {
			r := Simulate(p, l, goldHuntEnchant, false)
			if r.Died || r.LevelTime <= 0 {
				continue
			}
			h := r.LevelTime / 3600
			k := killGold(r.Hunt, l)
			hours += h
			drops += h * r.KillsH * k.drops
			sales += h * r.KillsH * k.sales
			supplies += h * (r.PotsH*potionPrice(l) + float64(game.Items["scroll_return"].Price))
			for id, n := range k.scrolls {
				if sc := game.Items[id]; sc.ScrollTier == t || (t == game.TierD && sc.ScrollTier == game.TierNone) {
					got[sc.Scroll == game.ScrollEnchantWeapon] += h * r.KillsH * n
				}
			}
		}
		n := float64(len(paths))
		g.Hours += hours / n
		g.InDrops += drops / n
		g.InSales += sales / n
		g.OutSupplies += supplies / n
		for w, v := range got {
			dropped[w] += v / n
		}
	}
	classes := Classes()
	for _, c := range classes {
		gear, ench := upgradeGold(c, lo, hi, dropped)
		g.OutGear += gear.gold / float64(len(classes))
		g.OutEnchant += ench.gold / float64(len(classes))
		g.ScrollsUsed += ench.scrolls / float64(len(classes))
	}
	g.ScrollsDropped = dropped[true] + dropped[false]
	return g
}

func potionPrice(level int) float64 {
	if level >= largePotionFrom {
		return float64(game.Items["potion_hp_l"].Price)
	}
	return float64(game.Items["potion_hp_s"].Price)
}

// killLoot: o saque esperado por abate.
type killLoot struct {
	drops, sales float64
	scrolls      map[string]float64 // pergaminhos de encantar (não vendidos: o caçador usa)
}

// killGold: as moedas, a venda do saque e os pergaminhos de encantar esperados por abate na
// região do nível (média dos monstros, pelo número de cada um). Sorteio com semente fixa: o
// resultado não muda.
func killGold(h Hunt, level int) killLoot {
	out := killLoot{scrolls: map[string]float64{}}
	var w float64
	for _, hm := range h.Monsters {
		rng := rand.New(rand.NewPCG(uint64(level), uint64(len(hm.Def.ID))))
		k := float64(hm.Count) / goldLootSamples
		for range goldLootSamples {
			for _, it := range game.RollLoot(hm.Def, level, rng) {
				switch {
				case it.Item.ID == game.GoldID:
					out.drops += k * float64(it.Count)
				case it.Item.IsEnchantScroll():
					out.scrolls[it.Item.ID] += k * float64(it.Count)
				default:
					out.sales += k * float64(it.Item.SellPrice()*int64(it.Count))
				}
			}
		}
		w += float64(hm.Count)
	}
	if w > 0 {
		out.drops, out.sales = out.drops/w, out.sales/w
		for id := range out.scrolls {
			out.scrolls[id] /= w
		}
	}
	return out
}

// spend: moedas e pergaminhos gastos.
type spend struct{ gold, scrolls float64 }

// upgradeGold: o que a classe gasta em peças novas e no encantamento delas nas trocas do
// equipamento típico entre lo e hi. Os pergaminhos que caíram (dropped, por arma/armadura)
// são usados antes de comprar na loja.
func upgradeGold(class string, lo, hi int, dropped map[bool]float64) (gear, enchant spend) {
	need := map[bool]float64{}  // pergaminhos usados
	price := map[bool]float64{} // preço de loja do pergaminho do grau (o mesmo em toda a faixa)
	prev := TypicalGear(class, lo-1, 0)
	for l := lo; l <= hi; l++ {
		cur := TypicalGear(class, l, 0)
		for _, pc := range newPieces(prev, cur) {
			d := pc.Item
			weapon := d.Kind == game.KindWeapon
			target := goldEnchantLevel
			if weapon {
				target = goldWeaponLevel
			}
			piece := pieceGold(d)
			e := enchantCost(d.Tier, piece, target)
			gear.gold += piece
			enchant.gold += e.fees + e.rebuilds
			need[weapon] += e.scrolls
			price[weapon] = float64(game.Items[game.EnchantScrollID(d.Tier, weapon, false)].Price)
		}
		prev = cur
	}
	for w, n := range need {
		enchant.scrolls += n
		enchant.gold += max(0, n-dropped[w]) * price[w]
	}
	return gear, enchant
}

// enchantExpect: o esperado para levar uma peça de +0 a +target.
type enchantExpect struct{ scrolls, fees, rebuilds float64 }

// enchantCost: pergaminhos, taxas e peças refeitas (piece moedas cada) esperados para levar
// uma peça do grau t de +0 a +target com pergaminhos comuns. Acima do seguro, falhar quebra
// a peça: refaz e volta do +0. Para cada nível n, E[n] = custo da tentativa + p·E[n+1] +
// (1−p)·(refazer + E[0]); escrevendo E[n] = A[n] + B[n]·E[0], E[0] = A[0] / (1 − B[0]).
func enchantCost(t game.Tier, piece float64, target int) enchantExpect {
	solve := func(attempt func(n int) float64, rebuild float64) float64 {
		a, b := 0.0, 0.0
		for n := target - 1; n >= 0; n-- {
			p := game.EnchantChanceFor(t, n+1)
			a, b = attempt(n+1)+p*a+(1-p)*rebuild, p*b+(1-p)
		}
		return a / (1 - b)
	}
	return enchantExpect{
		scrolls:  solve(func(int) float64 { return 1 }, 0),
		fees:     solve(func(n int) float64 { return float64(game.EnchantFee(t, n)) }, 0),
		rebuilds: solve(func(int) float64 { return 0 }, piece),
	}
}

// newPieces: as peças de cur que não estavam em prev (contando os encaixes duplos).
func newPieces(prev, cur []Piece) []Piece {
	had := map[string]int{}
	for _, p := range prev {
		had[p.Item.ID]++
	}
	var out []Piece
	for _, p := range cur {
		if had[p.Item.ID] > 0 {
			had[p.Item.ID]--
			continue
		}
		out = append(out, p)
	}
	return out
}

// pieceGold: as moedas de uma peça (a receita do Ferreiro, a loja ou nada: cai de monstro).
func pieceGold(d *game.ItemDef) float64 {
	for _, r := range game.RecipeList {
		if r.Result == d.ID {
			return float64(r.Gold)
		}
	}
	for _, id := range game.SmithStock {
		if id == d.ID {
			return float64(d.Price)
		}
	}
	for _, id := range game.MerchantStock {
		if id == d.ID {
			return float64(d.Price)
		}
	}
	return 0
}

// WriteGoldMarkdown: a tabela do ouro por faixa (go run ./cmd/balance -ouro).
func WriteGoldMarkdown(w io.Writer) {
	pf := func(f string, a ...any) { fmt.Fprintf(w, f, a...) }
	pf("## Ouro que entra e sai por grau (tunnel#154)\n\n")
	pf("Caçando sozinho no ritmo do simulador (equipamento típico +%d), média das classes. ", goldHuntEnchant)
	pf("Entra: as moedas que caem e a venda do saque (menos os pergaminhos de encantar, que o caçador usa). ")
	pf("Sai: poções, um Pergaminho de Retorno por hora, as peças novas de cada troca (receita ou loja) e o encantamento delas: ")
	pf("a arma a +%d e as outras peças a +%d, com as tentativas, a taxa e as quebras esperadas (refazer a peça). ", goldWeaponLevel, goldEnchantLevel)
	pf("Os pergaminhos que caíram são usados primeiro; o resto sai da loja. ")
	pf("Meta: sai de %.0f%% a %.0f%% do que entra.\n\n", GoldSinkMin*100, GoldSinkMax*100)
	pf("| Grau | Níveis | Horas | Moedas que caem | Venda do saque | Poções e retorno | Peças | Encantar | Pergaminhos (usados / caíram) | Entra | Sai | Sai/entra |\n")
	pf("|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, g := range GoldFlow() {
		pf("| %s | %d–%d | %s | %s | %s | %s | %s | %s | %s / %s | %s | %s | %.0f%% |\n", g.Tier.Letter(), g.Lo, g.Hi, num(g.Hours, 1),
			thousands(g.InDrops), thousands(g.InSales), thousands(g.OutSupplies), thousands(g.OutGear), thousands(g.OutEnchant),
			num(g.ScrollsUsed, 0), num(g.ScrollsDropped, 0), thousands(g.In()), thousands(g.Out()), g.Ratio()*100)
	}
	pf("\nTaxa de encantar por nível de destino: ")
	for i, t := range game.Tiers {
		if i > 0 {
			pf(", ")
		}
		pf("%s %d", t.Label(), game.EnchantFeePerLevel(t))
	}
	pf(" moedas.\n")
}

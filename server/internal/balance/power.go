package balance

import (
	"math"
	"slices"
	"sort"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// Poder de um item e orçamento de poder
// ---------------------------------------------------------------------------------------
//
// O poder de um item é a soma dos bônus convertidos em "pontos de poder": 1 ponto = 1 de
// ataque ou de defesa. Os pesos dos outros bônus dizem quanto deles vale 1 ponto.

// Pesos (pontos de poder por unidade do bônus).
const (
	wAtk        = 1.0   // ataque (físico ou mágico; quem dá os dois conta só o maior)
	wDef        = 1.0   // defesa física e mágica, cada uma
	wHP         = 0.1   // 10 de vida = 1 ponto
	wMP         = 0.1   // 10 de mana = 1 ponto
	wCrit       = 100.0 // 1% de crítico = 1 ponto
	wCritDamage = 20.0  // +0,1 no multiplicador do crítico = 2 pontos
	wAccuracy   = 0.25  // 4 de precisão = 1 ponto
	wEvasion    = 0.5   // 2 de esquiva = 1 ponto
	wSpeed      = 10.0  // +0,1 m/s = 1 ponto
	wHaste      = 100.0 // +1% de velocidade de ataque ou de conjuração (conta só a maior) = 1 ponto
	wRegen      = 2.0   // +1 de vida ou mana por segundo = 2 pontos
)

// StatPower converte bônus em pontos de poder.
func StatPower(b game.Stats) float64 {
	return wAtk*math.Max(b.PAtk, b.MAtk) + wDef*(b.PDef+b.MDef) + wHP*b.MaxHP + wMP*b.MaxMP +
		wCrit*b.Crit + wCritDamage*b.CritDamage + wAccuracy*b.Accuracy + wEvasion*b.Evasion +
		wSpeed*b.Speed + wHaste*math.Max(b.AtkSpeed, b.CastSpeed) + wRegen*(b.HPRegen+b.MPRegen)
}

// ItemPower: poder do item com o encantamento (o mesmo bônus que a zona soma).
func ItemPower(d *game.ItemDef, enchant int) float64 {
	return StatPower(game.AddItemBonus(game.Stats{}, d, enchant))
}

// Orçamento de poder: quanto um item deveria valer pelo nível exigido e pela raridade.
//
//	PoderEsperado(tipo, classe, nível, raridade) =
//	    (budgetBase + budgetPerLevel × (nível − 1)) × gradeBudget[raridade] × kindShare[tipo] × classe
//
// A curva é a da arma do cavaleiro (Espada de Treino 6 no 1, Espada de Aço 14 no 6, Lâmina
// do Vigia 27 no 13, Lâmina da Aurora 45 no 18): as outras peças valem uma fração dela
// (kindShare), e as armas seguem o ataque da classe (weaponClassShare).
const (
	budgetBase     = 6.0 // poder da arma comum do nível 1
	budgetPerLevel = 1.4 // poder a mais por nível exigido
)

// gradeBudget: quanto a raridade multiplica o orçamento do nível (índice = game.Grade).
var gradeBudget = [...]float64{
	game.GradeCommon:    1.0,
	game.GradeGood:      1.1,
	game.GradeRare:      1.2,
	game.GradeEpic:      1.5,
	game.GradeLegendary: 1.7,
}

// kindShare: fração do orçamento da arma que cada tipo de peça vale.
var kindShare = map[game.ItemKind]float64{
	game.KindWeapon:   1.0,
	game.KindShield:   1.2,
	game.KindArmor:    1.6,
	game.KindHelmet:   0.95,
	game.KindLegs:     1.05,
	game.KindGloves:   0.7,
	game.KindBoots:    0.75,
	game.KindCape:     0.65,
	game.KindNecklace: 0.8,
	game.KindEarring:  0.55,
	game.KindRing:     0.5,
}

// weaponClassShare: as armas seguem a escala de ataque da classe (o mago vive do ataque
// mágico, que cresce mais por nível; as adagas batem menos e mais vezes).
var weaponClassShare = map[string]float64{
	"knight":    1.0,
	"barbarian": 1.1,
	"mage":      1.3,
	"rogue":     0.9,
}

// ExpectedPower é o orçamento de poder de uma peça (classe só conta para armas).
func ExpectedPower(kind game.ItemKind, class string, level int, grade game.Grade) float64 {
	share, ok := kindShare[kind]
	if !ok || int(grade) >= len(gradeBudget) {
		return 0
	}
	if kind == game.KindWeapon {
		if c, ok := weaponClassShare[class]; ok {
			share *= c
		}
	}
	return (budgetBase + budgetPerLevel*float64(level-1)) * gradeBudget[grade] * share
}

// ItemBudget: o orçamento do item pelo tipo, classe (armas), nível exigido e raridade.
func ItemBudget(d *game.ItemDef) float64 {
	class := ""
	if len(d.Classes) == 1 {
		class = d.Classes[0]
	}
	return ExpectedPower(d.Kind, class, d.ReqLevel, d.Grade)
}

// Gear: os equipamentos do catálogo (sem os que só mudam o visual: a mesma peça com os
// mesmos bônus de outra, de id maior), em ordem de id.
func Gear() []*game.ItemDef {
	var out []*game.ItemDef
	for _, id := range game.ItemIDs() {
		if d := game.Items[id]; d.Equippable() {
			out = append(out, d)
		}
	}
	return out
}

// family: as peças que disputam o mesmo encaixe para a mesma classe (armas por classe).
func family(d *game.ItemDef) string {
	if d.Kind == game.KindWeapon && len(d.Classes) == 1 {
		return "weapon:" + d.Classes[0]
	}
	return "kind:" + kindName(d.Kind)
}

// Ladder: a escada de uma família, uma peça por raridade (a de maior poder, ver better), da
// comum à lendária.
func Ladder(fam string) []*game.ItemDef {
	best := map[game.Grade]*game.ItemDef{}
	for _, d := range Gear() {
		if family(d) != fam {
			continue
		}
		if b := best[d.Grade]; b == nil || better(d, b) {
			best[d.Grade] = d
		}
	}
	var out []*game.ItemDef
	for g := game.GradeCommon; g <= game.GradeLegendary; g++ {
		if d := best[g]; d != nil {
			out = append(out, d)
		}
	}
	return out
}

// better: d vale mais que o atual. Empate (as peças que só mudam o visual): a que vem antes
// na loja do Ferreiro, depois o menor id.
func better(d, cur *game.ItemDef) bool {
	if pd, pc := ItemPower(d, 0), ItemPower(cur, 0); pd != pc {
		return pd > pc
	}
	sd, sc := stockIndex(d.ID), stockIndex(cur.ID)
	if sd != sc {
		return sd < sc
	}
	return d.ID < cur.ID
}

func stockIndex(id string) int {
	if i := slices.Index(game.SmithStock, id); i >= 0 {
		return i
	}
	return len(game.SmithStock)
}

// Families: as famílias de peças, em ordem.
func Families() []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range Gear() {
		if f := family(d); !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

func kindName(k game.ItemKind) string {
	switch k {
	case game.KindWeapon:
		return "arma"
	case game.KindShield:
		return "escudo"
	case game.KindArmor:
		return "peito"
	case game.KindHelmet:
		return "cabeça"
	case game.KindLegs:
		return "pernas"
	case game.KindGloves:
		return "mãos"
	case game.KindBoots:
		return "pés"
	case game.KindCape:
		return "capa"
	case game.KindNecklace:
		return "colar"
	case game.KindEarring:
		return "brinco"
	case game.KindRing:
		return "anel"
	}
	return "?"
}

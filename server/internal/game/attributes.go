package game

// Atributos com pontos livres.
//
// Cada classe começa com uma distribuição própria (soma 60) e ganha AttrPointsPerLevel
// pontos a cada nível acima do 1. Os pontos valem a partir da base da classe: um
// personagem sem pontos distribuídos tem exatamente os valores de ClassStats.
//
// As fórmulas são LINEARES nos atributos (o cliente prevê qualquer distribuição somando
// o efeito de +1 ponto de cada atributo, que o servidor manda em Progress.per_point).

const AttrPointsPerLevel = 3

// Attrs: Força, Destreza, Constituição, Inteligência, Sabedoria, Espírito.
type Attrs struct {
	Str, Dex, Con, Int, Wis, Spi int
}

func (a Attrs) Add(b Attrs) Attrs {
	return Attrs{a.Str + b.Str, a.Dex + b.Dex, a.Con + b.Con, a.Int + b.Int, a.Wis + b.Wis, a.Spi + b.Spi}
}

func (a Attrs) Sum() int { return a.Str + a.Dex + a.Con + a.Int + a.Wis + a.Spi }

// Valid: nenhum valor negativo.
func (a Attrs) Valid() bool {
	return a.Str >= 0 && a.Dex >= 0 && a.Con >= 0 && a.Int >= 0 && a.Wis >= 0 && a.Spi >= 0
}

// Index devolve o atributo i (0 = Força … 5 = Espírito) e Unit o vetor com 1 nele.
func (a Attrs) Index(i int) int {
	return [6]int{a.Str, a.Dex, a.Con, a.Int, a.Wis, a.Spi}[i]
}

func UnitAttr(i int) Attrs {
	var a Attrs
	switch i {
	case 0:
		a.Str = 1
	case 1:
		a.Dex = 1
	case 2:
		a.Con = 1
	case 3:
		a.Int = 1
	case 4:
		a.Wis = 1
	case 5:
		a.Spi = 1
	}
	return a
}

// ClassBaseAttrs é a distribuição inicial de cada classe.
var ClassBaseAttrs = map[string]Attrs{
	"knight":    {Str: 12, Dex: 8, Con: 15, Int: 6, Wis: 9, Spi: 10},
	"barbarian": {Str: 16, Dex: 10, Con: 13, Int: 5, Wis: 7, Spi: 9},
	"mage":      {Str: 5, Dex: 9, Con: 8, Int: 16, Wis: 12, Spi: 10},
	"rogue":     {Str: 11, Dex: 16, Con: 10, Int: 6, Wis: 8, Spi: 9},
}

// AttrPointsTotal: pontos ganhos até o nível (3 por nível acima do 1).
func AttrPointsTotal(level int) int {
	if level < 1 {
		return 0
	}
	return (level - 1) * AttrPointsPerLevel
}

// Quanto cada ponto acima da base vale.
const (
	strPAtk      = 0.02  // +2% de ataque físico
	intMAtk      = 0.025 // +2,5% de ataque mágico
	conHP        = 0.025 // +2,5% de vida
	conPDef      = 0.01  // +1% de defesa física
	spiMP        = 0.03  // +3% de mana
	spiMDef      = 0.015 // +1,5% de defesa mágica
	dexCrit      = 0.004 // +0,4% de crítico
	dexAtkSpeed  = 0.01  // +1% de velocidade de ataque
	wisCastSpeed = 0.015 // +1,5% de velocidade de conjuração (recarga menor)
	conRegen     = 0.4   // +0,4 de vida por segundo
	wisRegen     = 0.3   // +0,3 de mana por segundo
	baseHPRegen  = 0.008 // 0,8% da vida por segundo, em combate
	baseMPRegen  = 0.015 // 1,5% da mana por segundo, em combate
)

func accuracyFor(dex, level int) float64 { return 70 + 1.5*float64(dex) + float64(level) }
func evasionFor(dex, level int) float64  { return 20 + 1.2*float64(dex) + 0.5*float64(level) }

// ApplyAttrs aplica os atributos (base da classe + pontos) aos valores da classe no nível.
// Os bônus de equipamento entram depois (AddBonus), e as passivas por último.
func ApplyAttrs(s Stats, class string, level int, a Attrs) Stats {
	b, ok := ClassBaseAttrs[class]
	if !ok {
		b = ClassBaseAttrs["knight"]
	}
	k := func(v, base int) float64 { return float64(v - base) }
	s.PAtk *= 1 + k(a.Str, b.Str)*strPAtk
	s.MAtk *= 1 + k(a.Int, b.Int)*intMAtk
	s.MaxHP *= 1 + k(a.Con, b.Con)*conHP
	s.PDef *= 1 + k(a.Con, b.Con)*conPDef
	s.MaxMP *= 1 + k(a.Spi, b.Spi)*spiMP
	s.MDef *= 1 + k(a.Spi, b.Spi)*spiMDef
	s.Crit += k(a.Dex, b.Dex) * dexCrit
	// Intervalo = base / (1 + k·1%): ataques por segundo crescem linearmente com a Destreza.
	s.AtkInterval /= 1 + k(a.Dex, b.Dex)*dexAtkSpeed
	s.CastSpeed = 1 + k(a.Wis, b.Wis)*wisCastSpeed
	s.Accuracy = accuracyFor(a.Dex, level)
	s.Evasion = evasionFor(a.Dex, level)
	s.CritDamage = CritMult
	return s
}

// FinishRegen calcula a regeneração depois de vida/mana máximas estarem prontas
// (equipamento e passivas já aplicados).
func FinishRegen(s Stats, class string, a Attrs) Stats {
	b, ok := ClassBaseAttrs[class]
	if !ok {
		b = ClassBaseAttrs["knight"]
	}
	s.HPRegen = s.MaxHP*baseHPRegen + float64(a.Con-b.Con)*conRegen
	s.MPRegen = s.MaxMP*baseMPRegen + float64(a.Wis-b.Wis)*wisRegen
	if s.HPRegen < 0 {
		s.HPRegen = 0
	}
	if s.MPRegen < 0 {
		s.MPRegen = 0
	}
	return s
}

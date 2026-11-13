package game

import (
	"math"
	"slices"
	"strings"
)

// Lugar dos objetivos de missão: para onde o minimapa, o mapa e a seta da borda da tela
// apontam. A zona calcula quando manda o QuestLog (ao mudar o passo e ao subir de nível),
// nunca por tick. Aqui fica a parte que só depende dos dados: a área de caça.

// PlaceKind é o tipo do lugar (o mesmo do QuestPlaceKind do protocolo).
type PlaceKind int

const (
	PlaceNone  PlaceKind = iota // sem lugar (usar skill, tomar poção)
	PlaceHunt                   // área de caça: círculo de centro X/Z e raio Radius
	PlaceNPC                    // um NPC
	PlaceGate                   // portão da cidade
	PlaceStone                  // Pedra de Teleporte
	PlaceShop                   // loja (NPC que vende)
)

// QuestPlace é onde fica um objetivo. Name é o rótulo no mapa.
type QuestPlace struct {
	Kind   PlaceKind
	X, Z   float64
	Radius float64
	Name   string
}

const (
	huntRingShare = 0.3 // raio do círculo num anel: fração da largura do anel
	huntAreaShare = 0.4 // raio do círculo numa área própria: fração do raio da área
	huntBossR     = 8.0 // raio em volta de um chefe de ponto fixo
	lairMargin    = 6.0 // folga do covil do raid boss (o mesmo de ExpandSpawns)
)

// HuntPlace escolhe a área de caça de um objetivo: entre os spawns dos monstros pedidos
// (nil = qualquer monstro que não seja chefe), o de faixa de nível mais perto do nível do
// jogador e, empatado, o mais perto dele. Num anel, o círculo fica na direção do jogador,
// na distância do centro em que os monstros têm o nível dele; numa área própria, na
// profundidade do nível dele. ok = false quando nenhum spawn serve.
func HuntPlace(spawns []Spawn, monsters []string, level int, px, pz float64) (QuestPlace, bool) {
	var best QuestPlace
	bestGap, bestDist := math.MaxInt, math.Inf(1)
	found := false
	for i, s := range spawns {
		def := Monsters[s.Monster]
		if def == nil || s.Count < 1 {
			continue
		}
		if monsters == nil {
			if s.Fixed {
				continue // "qualquer monstro" e "chegar ao nível" não mandam para um chefe
			}
		} else if !slices.Contains(monsters, s.Monster) {
			continue
		}
		lmax := s.LevelMax
		if s.Fixed {
			lmax = s.LevelMin
		}
		gap := levelGap(level, s.LevelMin, lmax)
		pl := spawnPlace(s, level, px, pz)
		d := math.Hypot(pl.X-px, pl.Z-pz)
		if gap > bestGap || gap == bestGap && d >= bestDist {
			continue
		}
		pl.Name = huntName(spawns, i, monsters)
		best, bestGap, bestDist, found = pl, gap, d, true
	}
	return best, found
}

// spawnPlace: o círculo de caça de um spawn para o nível e a posição do jogador.
func spawnPlace(s Spawn, level int, px, pz float64) QuestPlace {
	t := levelShare(level, s.LevelMin, s.LevelMax)
	switch {
	case s.Fixed:
		return QuestPlace{Kind: PlaceHunt, X: s.X, Z: s.Z, Radius: huntBossR}
	case s.Area:
		// O nível sobe do norte (borda Z+Radius) para o sul (ver ExpandSpawns).
		h := s.Radius * huntAreaShare
		z := clampF(s.Z+s.Radius-2*s.Radius*t, s.Z-s.Radius+h, s.Z+s.Radius-h)
		return QuestPlace{Kind: PlaceHunt, X: s.X, Z: z, Radius: h}
	}
	h := (s.RMax - s.RMin) * huntRingShare
	r := clampF(s.RMin+t*(s.RMax-s.RMin), s.RMin+h, s.RMax-h)
	a := 0.0 // no meio da cidade: para o leste (a estrada dos Arredores)
	if math.Hypot(px, pz) > 1 {
		a = math.Atan2(pz, px)
	}
	// Os monstros do anel não nascem no covil do raid boss: gira até sair dele.
	for k := 0; k <= 12; k++ {
		for _, sg := range []float64{1, -1} {
			b := a + sg*float64(k)*math.Pi/12
			x, z := r*math.Cos(b), r*math.Sin(b)
			if math.Hypot(x-RaidLairX, z-RaidLairZ) > RaidLairRadius+lairMargin+h {
				return QuestPlace{Kind: PlaceHunt, X: x, Z: z, Radius: h}
			}
			if k == 0 {
				break
			}
		}
	}
	return QuestPlace{Kind: PlaceHunt, X: r * math.Cos(a), Z: r * math.Sin(a), Radius: h}
}

// huntName: a região (áreas próprias) ou os nomes dos monstros pedidos que dividem o spawn.
func huntName(spawns []Spawn, i int, monsters []string) string {
	s := spawns[i]
	if s.Region != "" {
		return s.Region
	}
	var names []string
	for _, o := range spawns {
		def := Monsters[o.Monster]
		if def == nil || o.Fixed != s.Fixed || o.Area != s.Area || o.RMin != s.RMin || o.RMax != s.RMax ||
			o.X != s.X || o.Z != s.Z || monsters != nil && !slices.Contains(monsters, o.Monster) {
			continue
		}
		if !slices.Contains(names, def.Name) {
			names = append(names, def.Name)
		}
	}
	return strings.Join(names, ", ")
}

// levelGap: quantos níveis o jogador está fora da faixa [lo, hi] (0 = dentro).
func levelGap(level, lo, hi int) int {
	switch {
	case level < lo:
		return lo - level
	case level > hi:
		return level - hi
	}
	return 0
}

// levelShare: onde o nível cai na faixa (0 = LevelMin, 1 = LevelMax).
func levelShare(level, lo, hi int) float64 {
	if hi <= lo {
		return 0
	}
	return clampF(float64(level-lo)/float64(hi-lo), 0, 1)
}

func clampF(v, lo, hi float64) float64 {
	if lo > hi {
		return (lo + hi) / 2
	}
	return math.Max(lo, math.Min(hi, v))
}

// Monsters devolve os ids de monstro do objetivo de matar (nil = qualquer).
func (s QuestStep) Monsters() []string {
	if s.Target == "" {
		return nil
	}
	return strings.Split(s.Target, "|")
}

package game

import (
	"math"
	"sort"
	"strings"
	"sync"
)

// Bestiário: uma entrada por tipo de monstro, aberta por abates. Os marcos de cada entrada:
//
//   - 10 abates: o nível e a vida (do que o jogador já derrotou);
//   - 100: o que ele deixa cair (itens com a chance, moedas e equipamento);
//   - 500: +1% de dano contra aquele tipo (BestiaryDamageBonus, com teto BestiaryDamageCap).
//
// Chefes, raid bosses e raros (BestiaryOneKill) abrem tudo com 1 abate e não dão bônus de
// dano: a entrada deles é a lembrança da luta, não uma vantagem para a próxima.
//
// Famílias juntam as entradas pelo modelo 3D do monstro e pela faixa em que ele nasce: as de
// base (Esqueletos, Golens) têm os monstros que só nascem até o nível 30 (BestiaryBaseMaxLevel)
// e as Ancestrais, os que nascem acima disso (o conteúdo do 30 ao 40). Assim quem parou no 30 completa as de base, e o conteúdo
// novo do 40 abre famílias novas em vez de tirar a conclusão de quem já tinha. Um monstro novo
// com o mesmo modelo entra sozinho na família da faixa dele.
//
// Completar uma família pede o último marco de cada entrada que conta (BestiaryFamilyCounts):
// só os monstros que nascem sozinhos no mapa. Os raros (aparição por sorte, com espera de
// 45 min e às vezes só à noite) e as invocações (Guarda Ossário, Escudeiro Ossudo: só existem
// na luta de outro monstro) ficam na família como extras, sem travar a conclusão. Completar dá
// um título e uma cor de roupa exclusiva para o barbeiro (a recompensa horizontal: só visual,
// nada de poder).

// Marcos de abate de uma entrada comum.
const (
	BestiaryMarkInfo  = 10  // nível e vida
	BestiaryMarkDrops = 100 // o que deixa cair
	BestiaryMarkBonus = 500 // bônus de dano
)

// Bônus de dano do último marco e o teto dele contra um mesmo alvo. O orçamento de poder do
// simulador (balance/bestiary.go, dentro do TestPowerBudget) cobra o teto: com +2% o
// veterano do nível 30 ganhava até 6,2 pontos de poder, mais que a folga da arma típica
// (3,7); com +1% fica em no máximo ~83% dela.
const (
	BestiaryDamageBonus = 0.01
	BestiaryDamageCap   = 0.01
)

// BestiaryMaxKills: a contagem para aqui (o banco guarda JSON; nada passa de int32).
const BestiaryMaxKills = 1_000_000_000

// BestiaryOutfitBase: a cor de roupa exclusiva da família i é Appearance.Outfit =
// BestiaryOutfitBase + i (longe das cores da criação, que podem crescer sem mudar o índice
// de quem já usa uma cor do bestiário). A ordem de BestiaryFamilies só cresce no fim.
const BestiaryOutfitBase = 100

// BestiaryOneKill: 1 abate abre a entrada inteira (chefes, raid e os monstros raros da
// tunnel#133, MonsterDef.Rare).
func BestiaryOneKill(m *MonsterDef) bool {
	return m.Boss || m.Raid || m.Rare != nil
}

// BestiaryFamilyDef é uma família do bestiário.
type BestiaryFamilyDef struct {
	ID     string
	Name   string
	Models []string // prefixos do MonsterDef.Model que entram na família
	High   bool     // família Ancestral: monstros que nascem acima de BestiaryBaseMaxLevel
	Title  string   // título ao completar
	Outfit Shade    // cor de roupa exclusiva ao completar
}

// TitleID é o id do título da família (o mesmo campo das conquistas: AchievementCmd).
func (f *BestiaryFamilyDef) TitleID() string { return bestiaryTitlePrefix + f.ID }

// OutfitIndex é o Appearance.Outfit da cor exclusiva da família.
func (f *BestiaryFamilyDef) OutfitIndex() int {
	for i, g := range BestiaryFamilies {
		if g == f {
			return BestiaryOutfitBase + i
		}
	}
	return 0
}

const bestiaryTitlePrefix = "bst:"

// BestiaryFamilies: só acrescente no fim (o índice é o da cor de roupa).
var BestiaryFamilies = []*BestiaryFamilyDef{
	{ID: "skeletons", Name: "Esqueletos", Models: []string{"skeleton_"}, Title: "Ceifador de Ossos",
		Outfit: Shade{"Marfim Ossário", "#efe6cf", "#8c7a58"}},
	{ID: "golems", Name: "Golens", Models: []string{"stone_golem", "golem_"}, Title: "Quebra-Rochas",
		Outfit: Shade{"Granito Rúnico", "#9fb6c4", "#3c4652"}},
	{ID: "skeletons_ancient", Name: "Esqueletos Ancestrais", Models: []string{"skeleton_"}, High: true, Title: "Algoz da Partição",
		Outfit: Shade{"Violeta Ossário", "#cdb6ec", "#4b3468"}},
	{ID: "golems_ancient", Name: "Golens Ancestrais", Models: []string{"stone_golem", "golem_"}, High: true, Title: "Coração de Pedra Viva",
		Outfit: Shade{"Musgo Rúnico", "#a4d6bb", "#2e5444"}},
}

// BestiaryBaseMaxLevel: as famílias de base têm os monstros que nascem só até este nível (o
// teto antigo do jogo); os que nascem acima dele (o Golem Ancestral, do 30 ao 35, e o que é
// do 31 ao 40) vão para as famílias Ancestrais.
const BestiaryBaseMaxLevel = 30

var (
	spawnLevelOnce sync.Once
	spawnLevel     map[string]int
)

// BestiarySpawnLevel: o maior nível em que o monstro nasce sozinho no mapa (DefaultSpawns).
// O raro usa o do hospedeiro (nasce no lugar dele); 0 = não nasce sozinho (invocação).
func BestiarySpawnLevel(m *MonsterDef) int {
	spawnLevelOnce.Do(func() {
		spawnLevel = map[string]int{}
		for _, s := range DefaultSpawns() {
			spawnLevel[s.Monster] = max(spawnLevel[s.Monster], s.LevelMin, s.LevelMax)
		}
	})
	if m.Rare != nil {
		if h := Monsters[m.Rare.Host]; h != nil && h != m {
			return BestiarySpawnLevel(h)
		}
	}
	return spawnLevel[m.ID]
}

// BestiaryHigh: o monstro é das famílias Ancestrais (nasce acima de BestiaryBaseMaxLevel).
func BestiaryHigh(m *MonsterDef) bool { return BestiarySpawnLevel(m) > BestiaryBaseMaxLevel }

// BestiaryFamilyCounts: a entrada conta para completar a família (nasce sozinha no mapa; os
// raros e as invocações são extras).
func BestiaryFamilyCounts(m *MonsterDef) bool { return m.Rare == nil && BestiarySpawnLevel(m) > 0 }

// BestiaryFamilyOf devolve a família do monstro pelo modelo e pela faixa (nil = sem família).
func BestiaryFamilyOf(m *MonsterDef) *BestiaryFamilyDef {
	high := BestiaryHigh(m)
	for _, f := range BestiaryFamilies {
		if f.High != high {
			continue
		}
		for _, p := range f.Models {
			if strings.HasPrefix(m.Model, p) {
				return f
			}
		}
	}
	return nil
}

// BestiaryFamilyByID devolve a família pelo id (nil = não existe).
func BestiaryFamilyByID(id string) *BestiaryFamilyDef {
	for _, f := range BestiaryFamilies {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// BestiaryFamilyByTitle devolve a família cujo título tem esse id (nil = não é um título
// do bestiário).
func BestiaryFamilyByTitle(id string) *BestiaryFamilyDef {
	if !strings.HasPrefix(id, bestiaryTitlePrefix) {
		return nil
	}
	return BestiaryFamilyByID(strings.TrimPrefix(id, bestiaryTitlePrefix))
}

// BestiaryFamilyByOutfit devolve a família da cor de roupa exclusiva (nil = não é uma).
func BestiaryFamilyByOutfit(outfit int) *BestiaryFamilyDef {
	i := outfit - BestiaryOutfitBase
	if i < 0 || i >= len(BestiaryFamilies) {
		return nil
	}
	return BestiaryFamilies[i]
}

// IsBestiaryOutfit: a cor de roupa é uma recompensa do bestiário (o barbeiro confere se o
// personagem completou a família; a criação não aceita).
func IsBestiaryOutfit(outfit int) bool { return BestiaryFamilyByOutfit(outfit) != nil }

// BestiaryMarks devolve os abates de cada marco do monstro.
func BestiaryMarks(m *MonsterDef) [3]int {
	if BestiaryOneKill(m) {
		return [3]int{1, 1, 1}
	}
	return [3]int{BestiaryMarkInfo, BestiaryMarkDrops, BestiaryMarkBonus}
}

// BestiaryMark: quantos marcos (0–3) os abates abrem.
func BestiaryMark(m *MonsterDef, kills int) int {
	n := 0
	for _, need := range BestiaryMarks(m) {
		if kills >= need {
			n++
		}
	}
	return n
}

// BestiaryBonus: o bônus de dano contra o monstro com esses abates (0 sem o último marco,
// e sempre 0 contra chefes e raros), já limitado ao teto.
func BestiaryBonus(m *MonsterDef, kills int) float64 {
	if m == nil || BestiaryOneKill(m) || BestiaryMark(m, kills) < 3 {
		return 0
	}
	return math.Min(BestiaryDamageBonus, BestiaryDamageCap)
}

// BestiaryFamilyNeed: abates que a entrada pede para a família contar como completa (o
// último marco; 0 = extra, não conta: raro ou invocação, ver BestiaryFamilyCounts).
func BestiaryFamilyNeed(m *MonsterDef) int {
	if !BestiaryFamilyCounts(m) {
		return 0
	}
	return BestiaryMarks(m)[2]
}

// BestiaryEntryDef é uma entrada do catálogo.
type BestiaryEntryDef struct {
	Monster *MonsterDef
	Family  *BestiaryFamilyDef // nil = sem família
}

var (
	bestiaryOnce sync.Once
	bestiaryList []*BestiaryEntryDef
	bestiaryByID map[string]*BestiaryEntryDef
)

// BestiaryCatalog: todas as entradas, na ordem da janela (por família, os comuns pela XP e
// os chefes no fim). Calculado na primeira chamada, com todos os monstros já registrados.
func BestiaryCatalog() []*BestiaryEntryDef {
	bestiaryOnce.Do(func() {
		famIdx := map[*BestiaryFamilyDef]int{}
		for i, f := range BestiaryFamilies {
			famIdx[f] = i
		}
		for _, m := range Monsters {
			bestiaryList = append(bestiaryList, &BestiaryEntryDef{Monster: m, Family: BestiaryFamilyOf(m)})
		}
		rank := func(e *BestiaryEntryDef) int {
			if e.Family == nil {
				return len(BestiaryFamilies)
			}
			return famIdx[e.Family]
		}
		sort.Slice(bestiaryList, func(i, j int) bool {
			a, b := bestiaryList[i], bestiaryList[j]
			if ra, rb := rank(a), rank(b); ra != rb {
				return ra < rb
			}
			if oa, ob := BestiaryOneKill(a.Monster), BestiaryOneKill(b.Monster); oa != ob {
				return ob
			}
			if a.Monster.XPBase != b.Monster.XPBase {
				return a.Monster.XPBase < b.Monster.XPBase
			}
			return a.Monster.ID < b.Monster.ID
		})
		bestiaryByID = make(map[string]*BestiaryEntryDef, len(bestiaryList))
		for _, e := range bestiaryList {
			bestiaryByID[e.Monster.ID] = e
		}
	})
	return bestiaryList
}

// BestiaryEntryByID devolve a entrada do monstro (nil = não está no catálogo).
func BestiaryEntryByID(id string) *BestiaryEntryDef {
	BestiaryCatalog()
	return bestiaryByID[id]
}

// BestiaryFamilyMembers: as entradas da família, na ordem do catálogo.
func BestiaryFamilyMembers(f *BestiaryFamilyDef) []*BestiaryEntryDef {
	var out []*BestiaryEntryDef
	for _, e := range BestiaryCatalog() {
		if e.Family == f {
			out = append(out, e)
		}
	}
	return out
}

// BestiaryKill é o que fica salvo de uma entrada: abates e a faixa de níveis derrotados
// (a janela mostra o nível e a vida do que o jogador já enfrentou).
type BestiaryKill struct {
	N  int `json:"n"`
	Lo int `json:"lo,omitempty"`
	Hi int `json:"hi,omitempty"`
}

// BestiaryState é o que fica salvo no personagem (characters.bestiary).
type BestiaryState struct {
	// Kills: monstro → abates. Monstros que deixaram de existir ficam guardados (podem
	// voltar, ou ser de outra zona).
	Kills map[string]BestiaryKill `json:"kills,omitempty"`
	// Families: família completa → quando (Unix, ms). Uma família que ganhou um monstro novo
	// depois de completa continua completa (o título e a cor não somem).
	Families map[string]int64 `json:"families,omitempty"`
}

// FamilyProgress: entradas da família que contam (BestiaryFamilyCounts) já completas e o
// total delas; os extras (raros e invocações) ficam de fora.
func (s BestiaryState) FamilyProgress(f *BestiaryFamilyDef) (done, total int) {
	for _, e := range BestiaryFamilyMembers(f) {
		if !BestiaryFamilyCounts(e.Monster) {
			continue
		}
		total++
		if s.Kills[e.Monster.ID].N >= BestiaryFamilyNeed(e.Monster) {
			done++
		}
	}
	return done, total
}

// BestiaryTitle devolve o texto do título de família com esse id ("" se não for um).
func BestiaryTitle(id string) string {
	if f := BestiaryFamilyByTitle(id); f != nil {
		return f.Title
	}
	return ""
}

// BestiaryLevelStats: vida do monstro num nível (a janela mostra a do menor e do maior
// nível derrotado) e as moedas por abate nesse nível.
func BestiaryLevelStats(m *MonsterDef, level int) (hp float64, goldMin, goldMax int) {
	hp = MonsterStats(m, level).MaxHP
	return hp, int(math.Round(m.GoldMin * float64(level))), int(math.Round(m.GoldMax * float64(level)))
}

// BestiaryOutfit é a cor de roupa exclusiva de uma família, como a API de paletas mostra.
type BestiaryOutfit struct {
	Index  int    `json:"index"` // Appearance.Outfit
	Family string `json:"family"`
	Shade
}

// BestiaryOutfits: as cores exclusivas das famílias, na ordem do índice.
func BestiaryOutfits() []BestiaryOutfit {
	out := make([]BestiaryOutfit, 0, len(BestiaryFamilies))
	for i, f := range BestiaryFamilies {
		out = append(out, BestiaryOutfit{Index: BestiaryOutfitBase + i, Family: f.ID, Shade: f.Outfit})
	}
	return out
}

package game

import (
	"fmt"
	"strings"
)

// Especializações (segunda classe). No nível 20 cada classe escolhe um de dois caminhos,
// pela missão "A Encruzilhada" (a Mestra dos Caminhos, na praça). A escolha é definitiva
// (dá para trocar pagando SpecChangeCost) e dá:
//   - um bônus passivo de atributos (somado à base da classe e aos pontos distribuídos);
//   - um ramo extra de 3 skills nos níveis 20, 24 e 28, só com mecânicas que já existem.

// SpecLevel: nível em que a especialização é liberada.
const SpecLevel = 20

// SpecQuest: a missão que leva à escolha.
const SpecQuest = "encruzilhada"

// SpecChangeCost: trocar de especialização depois de escolher (os pontos das skills do
// caminho antigo voltam).
const SpecChangeCost int64 = 50_000

// SpecBranch é o índice do ramo das skills de especialização (os 3 da classe são 0–2).
const SpecBranch = 3

// SpecTierLevels: nível que libera cada degrau do ramo da especialização.
var SpecTierLevels = [3]int{20, 24, 28}

// TalkMaster: alvo do objetivo de falar com a Mestra dos Caminhos.
const TalkMaster = "mestre"

// SpecDef descreve uma especialização.
type SpecDef struct {
	ID, Class   string
	Name        string
	Description string
	Color       string // "#rrggbb": título sob o nome, quadro do alvo e ramo na árvore
	Bonus       Attrs  // bônus passivo de atributos
}

// BonusText: "+12 Constituição" ou "+6 Força e +6 Constituição".
func (s *SpecDef) BonusText() string {
	names := [6]string{"Força", "Destreza", "Constituição", "Inteligência", "Sabedoria", "Espírito"}
	var parts []string
	for i, n := range names {
		if v := s.Bonus.Index(i); v != 0 {
			parts = append(parts, fmt.Sprintf("%+d %s", v, n))
		}
	}
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " e " + parts[len(parts)-1]
}

// SpecList: duas por classe, na ordem em que aparecem na janela da Mestra.
var SpecList = []*SpecDef{
	{ID: "bastion", Class: "knight", Name: "Bastião", Color: "#8fb4ff", Bonus: Attrs{Con: 12},
		Description: "A muralha que não cede: provoca, protege e aguenta o que vier."},
	{ID: "herald", Class: "knight", Name: "Arauto", Color: "#ffd86a", Bonus: Attrs{Wis: 12},
		Description: "Leva a luz da ordem ao campo: golpes que enfraquecem e bênçãos para o grupo."},
	{ID: "bloodrager", Class: "barbarian", Name: "Sanguinário", Color: "#e85050", Bonus: Attrs{Str: 12},
		Description: "Vive para a próxima ferida: golpes enormes e críticos sem fim."},
	{ID: "warlord", Class: "barbarian", Name: "Caudilho", Color: "#e0a848", Bonus: Attrs{Str: 6, Con: 6},
		Description: "Comanda a carga: brados para os aliados, avanços e o estandarte de guerra."},
	{ID: "elementalist", Class: "mage", Name: "Elementalista", Color: "#ff9a3c", Bonus: Attrs{Int: 12},
		Description: "Fogo e gelo na forma mais pura: dano alto e cataclismos."},
	{ID: "occultist", Class: "mage", Name: "Ocultista", Color: "#b46ae0", Bonus: Attrs{Int: 6, Spi: 6},
		Description: "Pestes, maldições e marcas que fazem o inimigo apodrecer por dentro."},
	{ID: "cutthroat", Class: "rogue", Name: "Sicário", Color: "#e0607a", Bonus: Attrs{Dex: 12},
		Description: "Mata de perto e rápido: rajadas de lâminas e a sentença final."},
	{ID: "hunter", Class: "rogue", Name: "Caçador", Color: "#8cc850", Bonus: Attrs{Str: 6, Dex: 6},
		Description: "Caça de longe: facas de arremesso, redes e chuva de lâminas."},
}

// Specs: as especializações por id.
var Specs = func() map[string]*SpecDef {
	m := make(map[string]*SpecDef, len(SpecList))
	for _, s := range SpecList {
		m[s.ID] = s
	}
	return m
}()

// ClassSpecs devolve as especializações da classe, na ordem da lista.
func ClassSpecs(class string) []*SpecDef {
	var out []*SpecDef
	for _, s := range SpecList {
		if s.Class == class {
			out = append(out, s)
		}
	}
	return out
}

// SpecFor devolve a especialização se ela for da classe (nil = nenhuma ou inválida).
func SpecFor(class, id string) *SpecDef {
	if s := Specs[id]; s != nil && s.Class == class {
		return s
	}
	return nil
}

// SpecBonus: o bônus de atributos da especialização (zero se não houver).
func SpecBonus(class, id string) Attrs {
	if s := SpecFor(class, id); s != nil {
		return s.Bonus
	}
	return Attrs{}
}

// SpecTree devolve as 3 skills do caminho, por degrau.
func SpecTree(spec string) []*SkillDef {
	var out []*SkillDef
	for _, s := range Skills {
		if s.Spec == spec && spec != "" {
			out = append(out, s)
		}
	}
	sortSkills(out)
	return out
}

// SpecName: nome da especialização ("" = nenhuma).
func SpecName(id string) string {
	if s := Specs[id]; s != nil {
		return s.Name
	}
	return ""
}

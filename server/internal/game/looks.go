package game

// Aparência escolhida na criação: a arma inicial (e o escudo), a cor do emblema e o
// visual do corpo (pele, cabelo, roupa, rosto e chapéu). As armas são itens de verdade do
// kit inicial; mudar de arma depois é só equipar outra. O visual do corpo muda no barbeiro.

// Look é o que o jogador escolhe na criação.
type Look struct {
	Weapon  string `json:"weapon"`  // id do item da arma inicial
	Offhand string `json:"offhand"` // id do escudo inicial ("" = nada)
	Emblem  string `json:"emblem"`  // cor do emblema (#rrggbb, uma de Emblems)
	Appearance
}

// Appearance é o visual do corpo. O valor zero é o visual original da classe, então
// personagens antigos (colunas com o padrão) continuam iguais.
type Appearance struct {
	Skin    int    `json:"skin"`    // 0 = original; i = SkinTones[i-1]
	Hair    int    `json:"hair"`    // 0 = original; i = HairColors[i-1]
	Outfit  int    `json:"outfit"`  // 0 = original; i = OutfitColors[i-1]
	Head    string `json:"head"`    // "" = a cabeça da classe; senão uma de Heads
	HideHat bool   `json:"hideHat"` // esconde elmo/gorro/chapéu (o ladino não tem)
}

// Shade é uma faixa da paleta da textura: degradê de cima para baixo.
type Shade struct {
	Name   string `json:"name"`
	Top    string `json:"top"`
	Bottom string `json:"bottom"`
}

// SkinTones: tons de pele (o original de cada classe é o 0).
var SkinTones = []Shade{
	{"Clara", "#fbd9c0", "#eba98a"},
	{"Rosada", "#f6c7b0", "#d98f7c"},
	{"Dourada", "#e9c08f", "#c48d58"},
	{"Morena", "#c99268", "#9a6440"},
	{"Parda", "#a8714c", "#784a2e"},
	{"Escura", "#7d5038", "#4c2c1e"},
	{"Pálida", "#d9d6cf", "#a9a49a"},
	{"Esverdeada", "#a9c79a", "#6f8f66"},
}

// HairColors: cabelo e barba.
var HairColors = []Shade{
	{"Loiro", "#f3dc97", "#c9a050"},
	{"Ruivo", "#e9874f", "#a24a24"},
	{"Castanho", "#9a6a46", "#5b3a24"},
	{"Preto", "#4a4442", "#1b1918"},
	{"Grisalho", "#cfcac4", "#8c8680"},
	{"Branco", "#ffffff", "#c9ced6"},
	{"Azul", "#7fb2f0", "#2f5ea8"},
	{"Violeta", "#c39ae8", "#6f43a4"},
}

// OutfitColors: cor de destaque da roupa (capa, manto, faixas).
var OutfitColors = []Shade{
	{"Carmim", "#f0646a", "#7a1c2c"},
	{"Real", "#6f9ff0", "#22397a"},
	{"Floresta", "#78c27e", "#24502e"},
	{"Ouro", "#f5df84", "#b2771c"},
	{"Ametista", "#b48ae8", "#45246e"},
	{"Noite", "#5e6676", "#1d2129"},
	{"Neve", "#ffffff", "#aab4c0"},
	{"Terra", "#c58c5c", "#5e3a22"},
}

// Heads: cabeças que qualquer classe pode usar (rosto e corte de cabelo).
var Heads = []struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}{
	{"knight", "Nobre"},
	{"barbarian", "Selvagem"},
	{"mage", "Sábio"},
	{"rogue", "Astuto"},
	{"ranger", "Andarilho"},
}

// HatClasses: classes com chapéu que pode ser escondido.
var HatClasses = map[string]bool{"knight": true, "barbarian": true, "mage": true}

// BarberPrice: moedas para mudar o visual no barbeiro.
const BarberPrice = 300

// ValidAppearance confere o visual (índices dentro das paletas e cabeça conhecida).
func ValidAppearance(a Appearance) bool {
	if a.Skin < 0 || a.Skin > len(SkinTones) || a.Hair < 0 || a.Hair > len(HairColors) ||
		a.Outfit < 0 || a.Outfit > len(OutfitColors) {
		return false
	}
	if a.Head == "" {
		return true
	}
	for _, h := range Heads {
		if h.ID == a.Head {
			return true
		}
	}
	return false
}

// NormalAppearance: a cabeça da própria classe vira "" e o chapéu de quem não tem fica
// visível, para que o mesmo visual tenha uma forma só (comparar e salvar).
func NormalAppearance(class string, a Appearance) Appearance {
	if a.Head == class {
		a.Head = ""
	}
	if !HatClasses[class] {
		a.HideHat = false
	}
	return a
}

// StarterLook: opções de cada classe. A primeira de cada lista é a padrão.
type StarterLook struct {
	Weapons  []string
	Offhands []string // "" = mão vazia
	Emblem   string   // cor padrão
}

// Emblems: cores oferecidas na criação.
var Emblems = []string{"#d6b46e", "#e2674a", "#d04050", "#8c6fd6", "#4f9dff", "#58c7d6", "#5fae7d", "#e8e4dc"}

var StarterLooks = map[string]StarterLook{
	"knight": {
		Weapons:  []string{"sword_common", "blade_common"},
		Offhands: []string{"shield_badge", "shield_buckler", "shield_tower", ""},
		Emblem:   "#d6b46e",
	},
	"barbarian": {
		Weapons:  []string{"axe_common", "axe_jagged_common"},
		Offhands: []string{"shield_buckler", "shield_badge", "shield_tower", ""},
		Emblem:   "#e2674a",
	},
	"mage": {
		Weapons:  []string{"staff_common", "wand_common", "staff_dark_common"},
		Offhands: []string{""},
		Emblem:   "#8c6fd6",
	},
	"rogue": {
		Weapons:  []string{"daggers_common", "blades_common", "daggers_mixed_common"},
		Offhands: []string{""},
		Emblem:   "#5fae7d",
	},
}

// DefaultLook: as primeiras opções e a cor padrão da classe.
func DefaultLook(class string) Look {
	o, ok := StarterLooks[class]
	if !ok {
		o = StarterLooks["knight"]
	}
	return Look{Weapon: o.Weapons[0], Offhand: o.Offhands[0], Emblem: o.Emblem}
}

// ValidLook confere a escolha contra as opções da classe.
func ValidLook(class string, l Look) bool {
	o, ok := StarterLooks[class]
	return ok && contains(o.Weapons, l.Weapon) && contains(o.Offhands, l.Offhand) && ValidEmblem(l.Emblem) &&
		ValidAppearance(l.Appearance)
}

// ValidEmblem: uma das cores oferecidas.
func ValidEmblem(c string) bool { return contains(Emblems, c) }

// EmblemOr devolve a cor salva ou, se não houver (personagem antigo), a padrão da classe.
func EmblemOr(class, emblem string) string {
	if ValidEmblem(emblem) {
		return emblem
	}
	return DefaultLook(class).Emblem
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

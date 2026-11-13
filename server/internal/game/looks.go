package game

// Aparência escolhida na criação: a arma inicial (e o escudo) e a cor do emblema.
// As armas são itens de verdade do kit inicial; mudar de arma depois é só equipar outra.

// Look é o que o jogador escolhe na criação.
type Look struct {
	Weapon  string `json:"weapon"`  // id do item da arma inicial
	Offhand string `json:"offhand"` // id do escudo inicial ("" = nada)
	Emblem  string `json:"emblem"`  // cor do emblema (#rrggbb, uma de Emblems)
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
	return ok && contains(o.Weapons, l.Weapon) && contains(o.Offhands, l.Offhand) && ValidEmblem(l.Emblem)
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

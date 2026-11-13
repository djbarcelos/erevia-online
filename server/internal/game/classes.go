// Package game contém os dados e as fórmulas do jogo: classes, skills, monstros,
// dano e progressão. Sem estado e sem I/O, para ser fácil de testar e balancear.
package game

// MaxLevel é o nível máximo desta fase.
const MaxLevel = 20

// Stats são os atributos de combate de um personagem ou monstro num dado nível.
type Stats struct {
	MaxHP, MaxMP float64
	PAtk, MAtk   float64
	PDef, MDef   float64
	Range        float64 // alcance do ataque básico (m)
	AtkInterval  float64 // segundos entre ataques básicos
	Crit         float64 // chance de crítico (0–1)
	Speed        float64 // m/s
	Magic        bool    // ataque básico usa MAtk/MDef
	// Detalhes (ver attributes.go): precisão contra esquiva decide o erro; a velocidade de
	// conjuração divide a recarga das skills; regen é por segundo em combate.
	Accuracy, Evasion float64
	CastSpeed         float64 // 1 = normal
	HPRegen, MPRegen  float64
	CritDamage        float64 // multiplicador do crítico (2 = dobro)
}

// ClassDef descreve uma classe jogável.
type ClassDef struct {
	ID       string
	Name     string
	Base     Stats // nível 1
	PerLevel Stats // somado a cada nível acima do 1 (só os campos numéricos de recurso/ataque/defesa)
	// Skills que todo personagem da classe já começa sabendo (rank 1): as raízes dos ramos.
	Skills   [2]uint32
	Branches [3]Branch // os 3 ramos da árvore de skills (nomes vêm de skilltree.go)
}

var Classes = map[string]*ClassDef{
	"knight": {
		ID: "knight", Name: "Cavaleiro",
		Base:     Stats{MaxHP: 160, MaxMP: 40, PAtk: 8, MAtk: 4, PDef: 24, MDef: 15, Range: 2.2, AtkInterval: 1.2, Crit: 0.05, Speed: 5},
		PerLevel: Stats{MaxHP: 22, MaxMP: 4, PAtk: 2.2, MAtk: 0.5, PDef: 3.5, MDef: 2},
		Skills:   [2]uint32{SkillShieldBash, SkillFortress},
	},
	"barbarian": {
		ID: "barbarian", Name: "Bárbaro",
		Base:     Stats{MaxHP: 150, MaxMP: 35, PAtk: 10, MAtk: 3, PDef: 16, MDef: 12, Range: 2.2, AtkInterval: 1.4, Crit: 0.10, Speed: 5},
		PerLevel: Stats{MaxHP: 20, MaxMP: 3, PAtk: 3.0, MAtk: 0.4, PDef: 2.5, MDef: 1.5},
		Skills:   [2]uint32{SkillBrutalStrike, SkillFury},
	},
	"mage": {
		ID: "mage", Name: "Mago",
		Base:     Stats{MaxHP: 95, MaxMP: 120, PAtk: 6, MAtk: 12, PDef: 12, MDef: 25, Range: 12, AtkInterval: 1.6, Crit: 0.05, Speed: 5, Magic: true},
		PerLevel: Stats{MaxHP: 11, MaxMP: 14, PAtk: 0.6, MAtk: 3.2, PDef: 1.5, MDef: 3},
		Skills:   [2]uint32{SkillFireball, SkillIceNova},
	},
	"rogue": {
		ID: "rogue", Name: "Ladino",
		Base:     Stats{MaxHP: 120, MaxMP: 60, PAtk: 7, MAtk: 4, PDef: 16, MDef: 12, Range: 2.2, AtkInterval: 0.8, Crit: 0.25, Speed: 5.5},
		PerLevel: Stats{MaxHP: 15, MaxMP: 6, PAtk: 2.0, MAtk: 0.5, PDef: 2, MDef: 1.5},
		Skills:   [2]uint32{SkillBackstab, SkillShadowStep},
	},
}

// ClassStats calcula os atributos da classe num nível. Classe desconhecida usa cavaleiro.
func ClassStats(class string, level int) Stats {
	c, ok := Classes[class]
	if !ok {
		c = Classes["knight"]
	}
	l := float64(clampLevel(level) - 1)
	s := c.Base
	s.MaxHP += c.PerLevel.MaxHP * l
	s.MaxMP += c.PerLevel.MaxMP * l
	s.PAtk += c.PerLevel.PAtk * l
	s.MAtk += c.PerLevel.MAtk * l
	s.PDef += c.PerLevel.PDef * l
	s.MDef += c.PerLevel.MDef * l
	return s
}

// AddBonus soma bônus planos (equipamento) aos atributos de status.
func (s Stats) AddBonus(b Stats) Stats {
	s.MaxHP += b.MaxHP
	s.MaxMP += b.MaxMP
	s.PAtk += b.PAtk
	s.MAtk += b.MAtk
	s.PDef += b.PDef
	s.MDef += b.MDef
	s.Crit += b.Crit
	s.Accuracy += b.Accuracy
	s.Evasion += b.Evasion
	return s
}

func clampLevel(l int) int {
	if l < 1 {
		return 1
	}
	if l > MaxLevel+5 { // monstros podem passar do máximo dos jogadores
		return MaxLevel + 5
	}
	return l
}

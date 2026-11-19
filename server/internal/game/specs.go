package game

import (
	"fmt"
	"strings"
)

// Evoluções de classe (antes "especializações"; o id continua o mesmo no banco e no
// protocolo). No nível 20 cada classe escolhe uma de duas pela missão "A Encruzilhada"
// (Ysolde, a Mestra dos Caminhos, na praça). Ao evoluir:
//   - a árvore base some e entra a árvore da evolução (evolutions.go), com 2 skills iniciais;
//   - ganha um bônus passivo de atributos (somado à base da classe e aos pontos);
//   - passa a usar as armas da evolução e recebe os itens iniciais (uma vez).
// Dá para trocar (EvolutionChangeCost) e redistribuir a árvore (RespecCost) com a Ysolde.

// SpecLevel: nível em que a evolução é liberada.
const SpecLevel = 20

// SpecQuest: a missão que leva à escolha.
const SpecQuest = "encruzilhada"

// TalkMaster: alvo do objetivo de falar com a Mestra dos Caminhos.
const TalkMaster = "mestre"

// SpecDef descreve uma evolução.
type SpecDef struct {
	ID, Class   string
	Name        string
	Description string
	Color       string // "#rrggbb": título sob o nome, quadro do alvo e ramo na árvore
	Bonus       Attrs  // bônus passivo de atributos
	// Armas e mãos secundárias da evolução; vazio = as da classe.
	Weapons, Offhands []string
	Branches          [3]Branch
	// StarterSkills: as 2 iniciais (rank 1 de graça), na ordem das 2 iniciais da classe:
	// cada uma fica na tecla da inicial da base que substitui.
	StarterSkills [2]uint32
	// StarterItems: itens entregues uma vez ao evoluir (StarterItemsFlag).
	StarterItems []string
	// Resource: o recurso de classe (resources.go).
	Resource *ResourceDef
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
		Description:   "A muralha que não cede: provoca, protege e aguenta o que vier.",
		StarterSkills: [2]uint32{SkillBastionBash, SkillBastionGuard},
		Branches: [3]Branch{{"Muralha", "Mitigação e sobrevivência.", "#6f93d6"},
			{"Protetor", "Proteção dos aliados.", "#5fc4a6"}, {"Provocador", "Ameaça e controle.", "#d6b46e"}}},
	{ID: "herald", Class: "knight", Name: "Paladino", Color: "#ffd86a", Bonus: Attrs{Wis: 12},
		Description:   "Leva a luz da ordem ao campo: golpes sagrados e bênçãos para o grupo.",
		StarterSkills: [2]uint32{SkillHeraldStrike, SkillHeraldLight},
		Branches: [3]Branch{{"Julgamento", "Dano sagrado.", "#ffd86a"},
			{"Bênçãos", "Suporte e fortalecimento.", "#5fc4a6"}, {"Expiação", "Debilitação e proteção.", "#c89cff"}}},
	{ID: "bloodrager", Class: "barbarian", Name: "Berserker", Color: "#e85050", Bonus: Attrs{Str: 12},
		Description:   "Vive para a próxima ferida: golpes enormes, fúria e críticos sem fim.",
		StarterSkills: [2]uint32{SkillRagerBrutal, SkillRagerWrath},
		Branches: [3]Branch{{"Carnificina", "Dano e golpes pesados.", "#e06a42"},
			{"Frenesi", "Velocidade e sequência.", "#d04050"}, {"Execução", "Finalização e risco.", "#a6b84a"}}},
	{ID: "warlord", Class: "barbarian", Name: "Senhor da Guerra", Color: "#e0a848", Bonus: Attrs{Str: 6, Con: 6},
		Description:   "Comanda a carga: brados para os aliados, avanços e o estandarte de guerra.",
		StarterSkills: [2]uint32{SkillWarlordCommand, SkillWarlordCry},
		Branches: [3]Branch{{"Comando", "Buffs e liderança.", "#e0a848"},
			{"Investida", "Mobilidade e iniciação.", "#e06a42"}, {"Domínio", "Controle e resistência.", "#a6b84a"}}},
	{ID: "elementalist", Class: "mage", Name: "Elementalista", Color: "#ff9a3c", Bonus: Attrs{Int: 12},
		Description:   "Fogo e gelo na forma mais pura: dano em área e efeitos elementais.",
		StarterSkills: [2]uint32{SkillElemFireball, SkillElemNova},
		Branches: [3]Branch{{"Chamas", "Dano contínuo e explosão.", "#ec7a2c"},
			{"Geada", "Lentidão e congelamento.", "#6cc4f0"}, {"Convergência", "Combinação elemental.", "#e0a0ff"}}},
	{ID: "occultist", Class: "mage", Name: "Arcanista", Color: "#b46ae0", Bonus: Attrs{Int: 6, Spi: 6},
		Description:   "Pragas, maldições e marcas que fazem o inimigo apodrecer por dentro.",
		StarterSkills: [2]uint32{SkillOccDrain, SkillOccRot},
		Branches: [3]Branch{{"Maldições", "Enfraquecimento e marcas.", "#b46ae0"},
			{"Pragas", "Dano contínuo e propagação.", "#6ad06a"}, {"Arcano", "Controle e manipulação.", "#a87cf0"}}},
	{ID: "cutthroat", Class: "rogue", Name: "Assassino", Color: "#e0607a", Bonus: Attrs{Dex: 12},
		Description:   "Mata de perto e rápido: críticos, sombras e a sentença final.",
		StarterSkills: [2]uint32{SkillCutBackstab, SkillCutStep},
		Branches: [3]Branch{{"Lâminas", "Dano direto e críticos.", "#d6b46e"},
			{"Sombras", "Furtividade e mobilidade.", "#8c7ae0"}, {"Execução", "Marcas e finalizações.", "#e0607a"}}},
	{ID: "hunter", Class: "rogue", Name: "Patrulheiro", Color: "#8cc850", Bonus: Attrs{Str: 6, Dex: 6},
		Description:   "Caça de longe: tiros precisos, armadilhas e perseguição.",
		StarterSkills: [2]uint32{SkillHuntShot, SkillHuntKnife},
		Branches: [3]Branch{{"Arqueria", "Dano à distância.", "#d6b46e"},
			{"Armadilhas", "Controle de área.", "#6ad06a"}, {"Caçada", "Marcação e perseguição.", "#8cc850"}}},
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

// ActiveSpec: a evolução que vale para o personagem ("" se não for da classe ou se ele
// estiver abaixo do nível da evolução: o banco não confere).
func ActiveSpec(class, id string, level int) string {
	if level < SpecLevel || SpecFor(class, id) == nil {
		return ""
	}
	return id
}

// SpecBonus: o bônus de atributos da especialização (zero se não houver).
func SpecBonus(class, id string) Attrs {
	if s := SpecFor(class, id); s != nil {
		return s.Bonus
	}
	return Attrs{}
}

// SpecName: nome da especialização ("" = nenhuma).
func SpecName(id string) string {
	if s := Specs[id]; s != nil {
		return s.Name
	}
	return ""
}

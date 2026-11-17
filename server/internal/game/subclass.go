package game

import (
	"maps"
	"slices"
	"time"
)

// Subclasse (tunnel#156): com a classe principal no nível 30, a cadeia de missões do
// Guardião das Sendas (Edrian, na praça) dá uma segunda classe base no mesmo personagem.
// Ela começa no nível 20 e é upada do zero; o jogador troca a classe ativa com o Guardião.
//
// A classe ativa decide as skills aprendidas, a barra, os atributos distribuídos, a
// especialização, o nível e a XP: cada classe guarda os seus (ClassProgress), e os pontos
// livres de skill e de atributo saem do nível e do que ela já gastou. Equipamento,
// inventário, ouro, armazém, clã, karma, missões, conquistas, Maestria e aparência são do
// personagem e não mudam na troca. A subclasse não dá skills da outra classe: o bônus de
// ter duas é pequeno e horizontal (o título Duas Sendas e +5% de XP na classe mais baixa).

const (
	// SubclassQuestLevel: nível da classe principal que traz a missão.
	SubclassQuestLevel = 30
	// SubclassStartLevel: a subclasse começa neste nível, com a XP no início dele.
	SubclassStartLevel = 20
	// MaxSubclasses: quantas subclasses dá para ter hoje. A estrutura (ClassSet) guarda até
	// MaxClassSlots classes, para subir o teto sem mudar o banco.
	MaxSubclasses = 1
	MaxClassSlots = 4 // a principal e até 3 subclasses
	// SubclassSwitchCooldown: espera entre duas trocas de classe.
	SubclassSwitchCooldown = 10 * time.Minute
	// SubclassBonusLevel: uma subclasse neste nível libera o bônus Duas Sendas.
	SubclassBonusLevel = 30
	// SubclassXPBonus: XP a mais para a classe ativa quando ela é a mais baixa de todas.
	SubclassXPBonus = 0.05
)

// Missões da subclasse (game/quests.go): falar com o Guardião, a prova e o juramento.
const (
	SubclassQuestCall  = "sub_chamado"
	SubclassQuestTrial = "sub_prova"
	SubclassQuestOath  = "sub_juramento"
	// TalkGuardian: alvo dos passos de falar com o Guardião das Sendas (o nome do NPC, como
	// os outros alvos de conversa: o lugar do passo é calculado por ele).
	TalkGuardian = "Edrian"
	// SubclassTrialMonster e SubclassTrialKills: a prova (os golens do Fundo do Vale, 30–35).
	SubclassTrialMonster = "ancient_golem"
	SubclassTrialKills   = 30
)

// ClassProgress é a progressão de uma classe do personagem.
type ClassProgress struct {
	Class  string         `json:"class"`
	Level  int            `json:"level"`
	XP     int64          `json:"xp"`
	Attrs  Attrs          `json:"attrs"`
	Ranks  map[uint32]int `json:"ranks,omitempty"`
	Hotbar []string       `json:"hotbar,omitempty"`
	Spec   string         `json:"spec,omitempty"`
}

// ClassSet são as classes do personagem (characters.classes): a principal (sempre a
// primeira da lista; é a do corpo, characters.class), as subclasses e a ativa. As colunas
// de progressão do personagem (level, xp, attr_*, skill_ranks, hotbar, spec) são as da
// classe ativa; a entrada dela na lista é a cópia do último salvamento.
type ClassSet struct {
	Main   string          `json:"main,omitempty"`
	Active string          `json:"active,omitempty"`
	List   []ClassProgress `json:"list,omitempty"`
	// SwitchedAt: a última troca de classe (Unix, s; 0 = nunca). A recarga vale entre
	// sessões: sair e voltar não zera.
	SwitchedAt int64 `json:"switched_at,omitempty"`
}

// Find devolve a progressão da classe (nil = o personagem não tem).
func (s *ClassSet) Find(class string) *ClassProgress {
	for i := range s.List {
		if s.List[i].Class == class {
			return &s.List[i]
		}
	}
	return nil
}

// Has diz se o personagem tem a classe.
func (s *ClassSet) Has(class string) bool { return s.Find(class) != nil }

// Subclasses: quantas subclasses o personagem tem.
func (s *ClassSet) Subclasses() int { return max(0, len(s.List)-1) }

// Clone copia a lista e os mapas (o salvamento vai para outra goroutine).
func (s ClassSet) Clone() ClassSet {
	out := s
	out.List = slices.Clone(s.List)
	for i := range out.List {
		out.List[i].Ranks = maps.Clone(out.List[i].Ranks)
		out.List[i].Hotbar = slices.Clone(out.List[i].Hotbar)
	}
	return out
}

// NewSubclass: a progressão de uma subclasse recém-escolhida (nível 20, XP no início dele,
// sem pontos gastos; as skills iniciais e a barra padrão entram ao carregar).
func NewSubclass(class string) ClassProgress {
	return ClassProgress{Class: class, Level: SubclassStartLevel, XP: XPLevelStart(SubclassStartLevel)}
}

// NormalClassSet arruma o que veio do banco: a principal é main e vem primeiro (personagem
// de antes da subclasse ganha a entrada dela), classes inválidas ou repetidas saem, o
// nível de cada uma é o da XP e a ativa é uma das classes (senão, a principal). A entrada
// da ativa é refeita da progressão das colunas por quem carrega.
func NormalClassSet(main string, s ClassSet) ClassSet {
	out := ClassSet{Main: main, SwitchedAt: max(0, s.SwitchedAt)}
	first := ClassProgress{Class: main, Level: 1}
	if c := s.Find(main); c != nil {
		first = *c
	}
	out.List = append(out.List, first)
	for _, c := range s.List {
		if len(out.List) >= MaxClassSlots {
			break
		}
		if Classes[c.Class] == nil || out.Has(c.Class) {
			continue
		}
		out.List = append(out.List, c)
	}
	for i := range out.List {
		c := &out.List[i]
		c.XP = max(0, c.XP)
		if i > 0 { // subclasse nunca fica abaixo do começo dela
			c.XP = max(c.XP, XPLevelStart(SubclassStartLevel))
		}
		c.Level = LevelForXP(c.XP)
	}
	out.Active = main
	if s.Active != "" && out.Has(s.Active) {
		out.Active = s.Active
	}
	return out
}

// levelOf: o nível da classe, com o da ativa trocado pelo nível de agora (a entrada dela
// na lista só é atualizada ao salvar).
func (s *ClassSet) levelOf(c *ClassProgress, activeLevel int) int {
	if c.Class == s.Active {
		return activeLevel
	}
	return c.Level
}

// BonusUnlocked: alguma subclasse chegou ao SubclassBonusLevel (Duas Sendas).
func (s *ClassSet) BonusUnlocked(activeLevel int) bool {
	for i := 1; i < len(s.List); i++ {
		if s.levelOf(&s.List[i], activeLevel) >= SubclassBonusLevel {
			return true
		}
	}
	return false
}

// TopSubclassLevel: o maior nível entre as subclasses (0 = nenhuma).
func (s *ClassSet) TopSubclassLevel(activeLevel int) int {
	top := 0
	for i := 1; i < len(s.List); i++ {
		top = max(top, s.levelOf(&s.List[i], activeLevel))
	}
	return top
}

// XPMult: o multiplicador da XP da classe ativa. Com o bônus Duas Sendas, a classe ativa
// ganha SubclassXPBonus a mais quando é a mais baixa de todas (empate não conta).
func (s *ClassSet) XPMult(activeLevel int) float64 {
	if len(s.List) < 2 || !s.BonusUnlocked(activeLevel) {
		return 1
	}
	for i := range s.List {
		c := &s.List[i]
		if c.Class != s.Active && c.Level <= activeLevel {
			return 1
		}
	}
	return 1 + SubclassXPBonus
}

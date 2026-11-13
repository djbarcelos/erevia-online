package game

import "strings"

// Missões: definidas por dados. Cada missão tem objetivos que andam juntos; quando
// todos completam, o jogador recebe a recompensa e a próxima missão da sequência (Next).

// QuestGoal é o tipo de objetivo.
type QuestGoal int

const (
	GoalLeaveTown QuestGoal = iota + 1 // sair da cidade (passar do raio seguro)
	GoalKill                           // derrotar monstros (Target: ids separados por "|"; vazio = qualquer)
	GoalSkill                          // usar skills
	GoalPotion                         // tomar poções
	GoalBuy                            // comprar numa loja
	GoalLevel                          // chegar ao nível Count
	GoalTeleport                       // viajar com uma Pedra de Teleporte
	GoalTalk                           // falar com um NPC (Target: TalkMaster…)
	GoalSpec                           // escolher a especialização (segunda classe)
)

// QuestStep é um objetivo: chegar a Count no Goal.
type QuestStep struct {
	Goal   QuestGoal
	Target string
	Count  int
	Text   string
}

// Matches diz se um acontecimento (goal, alvo) conta para este objetivo.
func (s QuestStep) Matches(goal QuestGoal, target string) bool {
	if s.Goal != goal {
		return false
	}
	if s.Target == "" {
		return true
	}
	for _, t := range strings.Split(s.Target, "|") {
		if t == target {
			return true
		}
	}
	return false
}

// QuestItem é um item da recompensa.
type QuestItem struct {
	ID    string
	Count int
}

type QuestReward struct {
	XP    int64
	Gold  int64
	Items []QuestItem
}

type QuestDef struct {
	ID     string
	Title  string
	Text   string // o que fazer, com um pouco de história
	Steps  []QuestStep
	Reward QuestReward
	Next   string // próxima da sequência ("" = fim)
	// Unlock > 0: a missão chega sozinha ao atingir este nível (fora de qualquer sequência).
	Unlock int
}

// QuestState é o que fica salvo: progresso das missões ativas e as concluídas.
type QuestState struct {
	Active map[string][]int `json:"active,omitempty"`
	Done   []string         `json:"done,omitempty"`
}

// TutorialStart é a primeira missão de um personagem novo.
const TutorialStart = "tut_gate"

// TutorialSkipLevel: quem já passou deste nível sem histórico de missões não recebe o
// tutorial (personagens de antes das missões).
const TutorialSkipLevel = 5

// questList: o tutorial dos primeiros ~15 minutos, em ordem.
var questList = []*QuestDef{
	{
		ID: "tut_gate", Title: "Além das muralhas",
		Text:   "Vel'Aran está segura atrás das muralhas, mas a sua jornada começa lá fora. Saia por qualquer portão.",
		Steps:  []QuestStep{{Goal: GoalLeaveTown, Count: 1, Text: "Sair da cidade"}},
		Reward: QuestReward{XP: 40},
		Next:   "tut_hunt",
	},
	{
		ID: "tut_hunt", Title: "Ossos inquietos",
		Text:   "Os Esqueletos Lacaios rondam os Arredores. Clique num deles para atacar.",
		Steps:  []QuestStep{{Goal: GoalKill, Target: "skeleton_minion", Count: 3, Text: "Derrotar Esqueletos Lacaios"}},
		Reward: QuestReward{XP: 120, Gold: 30},
		Next:   "tut_skill",
	},
	{
		ID: "tut_skill", Title: "Golpe especial",
		Text:   "Cada classe tem skills na barra de atalhos (teclas 1 a 0). Use-as em combate.",
		Steps:  []QuestStep{{Goal: GoalSkill, Count: 3, Text: "Usar skills"}},
		Reward: QuestReward{XP: 150},
		Next:   "tut_potion",
	},
	{
		ID: "tut_potion", Title: "Fôlego",
		Text:   "Ferido? Tome uma poção pela barra de atalhos ou com duplo clique no inventário (I).",
		Steps:  []QuestStep{{Goal: GoalPotion, Count: 1, Text: "Tomar uma poção"}},
		Reward: QuestReward{XP: 100, Items: []QuestItem{{ID: "potion_hp_s", Count: 3}, {ID: "potion_mp_s", Count: 2}}},
		Next:   "tut_shop",
	},
	{
		ID: "tut_shop", Title: "Compras na praça",
		Text:   "A Mercadora Lívia e o Ferreiro Bruno ficam na praça. Clique num deles e compre algo.",
		Steps:  []QuestStep{{Goal: GoalBuy, Count: 1, Text: "Comprar numa loja"}},
		Reward: QuestReward{XP: 200, Gold: 40},
		Next:   "tut_level",
	},
	{
		ID: "tut_level", Title: "Mais forte",
		Text:   "Cace nos Arredores até o nível 4. A cada nível, distribua os pontos de atributo (C) e de skill (K).",
		Steps:  []QuestStep{{Goal: GoalLevel, Count: 4, Text: "Chegar ao nível 4"}},
		Reward: QuestReward{Gold: 60, Items: []QuestItem{{ID: "potion_hp_s", Count: 5}}},
		Next:   "tut_stone",
	},
	{
		ID: "tut_stone", Title: "Caminhos de pedra",
		Text:   "As Pedras de Teleporte levam aos acampamentos nas estradas. Use a da praça para viajar.",
		Steps:  []QuestStep{{Goal: GoalTeleport, Count: 1, Text: "Viajar com uma Pedra de Teleporte"}},
		Reward: QuestReward{XP: 300, Gold: 100},
		Next:   "tut_bones",
	},
	{
		ID: "tut_bones", Title: "Os Campos dos Ossos",
		Text:   "Mais longe da cidade, guerreiros e ladinos esqueletos guardam os Campos dos Ossos. Vá preparado.",
		Steps:  []QuestStep{{Goal: GoalKill, Target: "skeleton_warrior|skeleton_rogue", Count: 5, Text: "Derrotar Esqueletos Guerreiros ou Ladinos"}},
		Reward: QuestReward{XP: 1500, Gold: 200, Items: []QuestItem{{ID: "scroll_ench_weapon", Count: 1}, {ID: "scroll_return", Count: 2}}},
	},
	// Segunda classe: chega no nível 20 e termina na escolha do caminho.
	{
		ID: SpecQuest, Title: "A Encruzilhada", Unlock: SpecLevel,
		Text: "Ninguém chega tão longe por acaso. Ysolde, a Mestra dos Caminhos, espera na praça de Vel'Aran " +
			"para mostrar os dois caminhos da sua classe. A escolha é para sempre.",
		Steps: []QuestStep{
			{Goal: GoalTalk, Target: TalkMaster, Count: 1, Text: "Falar com a Mestra Ysolde, na praça"},
			{Goal: GoalSpec, Count: 1, Text: "Escolher o seu caminho"},
		},
		Reward: QuestReward{Gold: 1000, Items: []QuestItem{{ID: "scroll_return", Count: 3}}},
	},
}

// QuestsByUnlock: as missões que chegam sozinhas com o nível, na ordem da lista.
var QuestsByUnlock = func() []*QuestDef {
	var out []*QuestDef
	for _, q := range questList {
		if q.Unlock > 0 {
			out = append(out, q)
		}
	}
	return out
}()

// Quests: as missões por id.
var Quests = func() map[string]*QuestDef {
	m := make(map[string]*QuestDef, len(questList))
	for _, q := range questList {
		m[q.ID] = q
	}
	return m
}()

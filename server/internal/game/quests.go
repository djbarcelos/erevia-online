package game

import (
	"fmt"
	"math"
	"strings"
)

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
	// Place fixa à mão onde fica o objetivo (missões de história). nil = o servidor
	// calcula pelo tipo (área de caça do monstro, NPC, portão, pedra...).
	Place *QuestPlace
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

// QuestCategory agrupa as missões no Diário (J). Os números são os do QuestCategory do
// protocolo.
type QuestCategory int

const (
	QuestStory    QuestCategory = iota + 1 // história
	QuestRegion                            // missões de uma região
	QuestDaily                             // diárias
	QuestTutorial                          // os primeiros passos
)

type QuestDef struct {
	ID     string
	Title  string
	Text   string // o que fazer, com um pouco de história
	Steps  []QuestStep
	Reward QuestReward
	Next   string // próxima da sequência ("" = fim)
	// Unlock > 0: a missão chega sozinha ao atingir este nível (fora de qualquer sequência).
	Unlock int
	// Until > 0: a missão de nível só chega até este nível (quem já passou dele não recebe).
	Until int
	// Category: o grupo no Diário (obrigatório; o teste dos dados confere).
	Category QuestCategory
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

// questList: o tutorial dos primeiros ~15 minutos, em ordem, a segunda classe, a
// subclasse e os contratos de caça.
var questList = append([]*QuestDef{
	{
		ID: "tut_gate", Title: "Além das muralhas", Category: QuestTutorial,
		Text:   "Vel'Aran está segura atrás das muralhas, mas a sua jornada começa lá fora. Saia por qualquer portão.",
		Steps:  []QuestStep{{Goal: GoalLeaveTown, Count: 1, Text: "Sair da cidade"}},
		Reward: QuestReward{XP: 40},
		Next:   "tut_hunt",
	},
	{
		ID: "tut_hunt", Title: "Ossos inquietos", Category: QuestTutorial,
		Text:   "Os Esqueletos Lacaios rondam os Arredores. Clique num deles para atacar.",
		Steps:  []QuestStep{{Goal: GoalKill, Target: "skeleton_minion", Count: 3, Text: "Derrotar Esqueletos Lacaios"}},
		Reward: QuestReward{XP: 120, Gold: 30},
		Next:   "tut_skill",
	},
	{
		ID: "tut_skill", Title: "Golpe especial", Category: QuestTutorial,
		Text:   "Cada classe tem skills na barra de atalhos (teclas 1 a 0). Use-as em combate.",
		Steps:  []QuestStep{{Goal: GoalSkill, Count: 3, Text: "Usar skills"}},
		Reward: QuestReward{XP: 150},
		Next:   "tut_potion",
	},
	{
		ID: "tut_potion", Title: "Fôlego", Category: QuestTutorial,
		Text:   "Ferido? Tome uma poção pela barra de atalhos ou com duplo clique no inventário (I).",
		Steps:  []QuestStep{{Goal: GoalPotion, Count: 1, Text: "Tomar uma poção"}},
		Reward: QuestReward{XP: 100, Items: []QuestItem{{ID: "potion_hp_s", Count: 3}, {ID: "potion_mp_s", Count: 2}}},
		Next:   "tut_shop",
	},
	{
		ID: "tut_shop", Title: "Compras na praça", Category: QuestTutorial,
		Text:   "A Mercadora Lívia e o Ferreiro Bruno ficam na praça. Clique num deles e compre algo.",
		Steps:  []QuestStep{{Goal: GoalBuy, Count: 1, Text: "Comprar numa loja"}},
		Reward: QuestReward{XP: 200, Gold: 40},
		Next:   "tut_level",
	},
	{
		ID: "tut_level", Title: "Mais forte", Category: QuestTutorial,
		Text:   "Cace nos Arredores até o nível 4. A cada nível, distribua os pontos de atributo (C) e de skill (K).",
		Steps:  []QuestStep{{Goal: GoalLevel, Count: 4, Text: "Chegar ao nível 4"}},
		Reward: QuestReward{Gold: 60, Items: []QuestItem{{ID: "potion_hp_s", Count: 5}}},
		Next:   "tut_stone",
	},
	{
		ID: "tut_stone", Title: "Caminhos de pedra", Category: QuestTutorial,
		Text:   "As Pedras de Teleporte levam aos acampamentos nas estradas. Use a da praça para viajar.",
		Steps:  []QuestStep{{Goal: GoalTeleport, Count: 1, Text: "Viajar com uma Pedra de Teleporte"}},
		Reward: QuestReward{XP: 300, Gold: 100},
		Next:   "tut_bones",
	},
	{
		ID: "tut_bones", Title: "Os Campos dos Ossos", Category: QuestTutorial,
		Text:   "Mais longe da cidade, guerreiros e ladinos esqueletos guardam os Campos dos Ossos. Vá preparado.",
		Steps:  []QuestStep{{Goal: GoalKill, Target: "skeleton_warrior|skeleton_rogue", Count: 5, Text: "Derrotar Esqueletos Guerreiros ou Ladinos"}},
		Reward: QuestReward{XP: 1500, Gold: 200, Items: []QuestItem{{ID: "scroll_ench_weapon", Count: 1}, {ID: "scroll_return", Count: 2}}},
	},
	// Segunda classe: chega no nível 20 e termina na escolha do caminho.
	{
		ID: SpecQuest, Title: "A Encruzilhada", Unlock: SpecLevel, Category: QuestStory,
		Text: "Ninguém chega tão longe por acaso. Ysolde, a Mestra dos Caminhos, espera na praça de Vel'Aran " +
			"para mostrar os dois caminhos da sua classe. A escolha é para sempre.",
		Steps: []QuestStep{
			{Goal: GoalTalk, Target: TalkMaster, Count: 1, Text: "Falar com a Mestra Ysolde, na praça"},
			{Goal: GoalSpec, Count: 1, Text: "Escolher o seu caminho"},
		},
		Reward: QuestReward{Gold: 1000, Items: []QuestItem{{ID: "scroll_return", Count: 3}}},
	},
	// Subclasse (tunnel#156): chega com a classe principal no 30; o juramento libera a
	// escolha de uma outra classe base na janela do Guardião (zone/subclass.go).
	{
		ID: SubclassQuestCall, Title: "O Chamado das Sendas", Unlock: SubclassQuestLevel, Category: QuestStory,
		Text: "Quem domina uma classe começa a ouvir as outras. Edrian, o Guardião das Sendas, espera na praça de " +
			"Vel'Aran, ao sul do poço, por quem quer trilhar um segundo caminho.",
		Steps: []QuestStep{{Goal: GoalTalk, Target: TalkGuardian, Count: 1, Text: "Falar com Edrian, o Guardião das Sendas, na praça"}},
		Next:  SubclassQuestTrial,
	},
	{
		ID: SubclassQuestTrial, Title: "A Prova das Duas Sendas", Category: QuestStory,
		Text: "Edrian quer ver se o seu braço aguenta duas vidas. Derrote os Golens Ancestrais no Fundo do Vale, " +
			"na borda sul do Vale das Pedras Vivas.",
		Steps:  []QuestStep{{Goal: GoalKill, Target: SubclassTrialMonster, Count: SubclassTrialKills, Text: "Derrotar Golens Ancestrais"}},
		Reward: QuestReward{Gold: 5000},
		Next:   SubclassQuestOath,
	},
	{
		ID: SubclassQuestOath, Title: "O Juramento", Category: QuestStory,
		Text: "Volte a Edrian e faça o juramento. Ele mostra as outras classes: a que você escolher começa no nível 20, " +
			"e é com ele que você troca de classe depois.",
		Steps:  []QuestStep{{Goal: GoalTalk, Target: TalkGuardian, Count: 1, Text: "Fazer o juramento com Edrian"}},
		Reward: QuestReward{Items: []QuestItem{{ID: "scroll_return", Count: 3}}},
	},
}, contractQuests()...)

// Contratos de caça (tunnel#149): um por nível, do ContractFrom ao 39, gerados dos dados.
// Cada um pede uma parte dos abates do nível nos monstros que se caçam nele e paga
// ContractXPShare da XP do nível: com o tutorial, ~30% da XP de cada faixa vem de missões.
const (
	ContractFrom     = 5
	ContractXPShare  = 0.30
	contractKillPart = 0.25 // fração dos abates do nível que o contrato pede
	// ContractWindow: o contrato do nível L só chega até o L+ContractWindow. Quem já passou
	// (os personagens de antes dos contratos) não recebe uma pilha de contratos velhos.
	ContractWindow = 2
)

// ContractID: o id do contrato do nível.
func ContractID(level int) string { return fmt.Sprintf("contrato_%02d", level) }

// ringRegions: o nome dos anéis em volta da cidade, pelo nível de começo (o mesmo do mapa).
var ringRegions = map[int]string{1: "Arredores", 6: "Campos dos Ossos", 13: "Ermos Partidos"}

func contractQuests() []*QuestDef {
	var out []*QuestDef
	for l := ContractFrom; l < MaxLevel; l++ {
		group := huntGroup(l)
		perKill, _ := huntXP(l)
		if len(group) == 0 || perKill <= 0 {
			continue
		}
		region := group[0].Region
		if region == "" {
			region = ringRegions[group[0].LevelMin]
		}
		ids := huntMonsters(l)
		names := make([]string, len(ids))
		for i, id := range ids {
			names[i] = Monsters[id].Name
		}
		who := strings.Join(names, " ou ")
		xp := int64(math.Round(float64(XPToNext(l)) * ContractXPShare))
		kills := float64(XPToNext(l)-xp) / perKill
		count := max(5, int(math.Round(kills*contractKillPart/5))*5)
		out = append(out, &QuestDef{
			ID: ContractID(l), Title: fmt.Sprintf("Contrato: %s (nível %d)", region, l), Unlock: l, Until: l + ContractWindow,
			Category: QuestRegion,
			Text: fmt.Sprintf("O quadro de contratos da praça de Vel'Aran paga por cada caçada que mantém as estradas abertas. "+
				"Derrote %d %s (%s), perto do nível %d.", count, who, region, l),
			Steps:  []QuestStep{{Goal: GoalKill, Target: strings.Join(ids, "|"), Count: count, Text: "Derrotar " + who}},
			Reward: QuestReward{XP: xp},
		})
	}
	return out
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

// QuestsInOrder: todas as missões na ordem da lista (a aba Concluídas do Diário segue ela).
func QuestsInOrder() []*QuestDef {
	return questList
}

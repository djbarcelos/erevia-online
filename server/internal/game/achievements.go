package game

import "strings"

// Conquistas: definidas por dados, como as missões. Cada uma anda com um acontecimento
// que o servidor já valida (abate, nível, encantamento, troca, criação, missão, PvP e
// raid boss) e, ao desbloquear, dá um título que o jogador pode usar sob o nome.

// AchGoal é o tipo de acontecimento que conta para uma conquista.
type AchGoal int

const (
	AchKill    AchGoal = iota + 1 // derrotar monstros (Target: ids separados por "|"; vazio = qualquer)
	AchLevel                      // chegar ao nível Count
	AchEnchant                    // encantar um item a +Count
	AchTrade                      // concluir trocas com jogadores
	AchCraft                      // criar um item de raridade Count ou maior
	AchQuest                      // concluir a missão Target
	AchPvPWin                     // vencer jogadores em combate (vitórias PvP)
	AchRaid                       // participar (com dano) da morte do raid boss Target
	AchDefense                    // defender a cidade numa invasão vencida (zone/invasion.go)
)

// AchievementDef é uma conquista do catálogo.
type AchievementDef struct {
	ID     string
	Name   string
	Text   string // o que fazer
	Title  string // título que ela dá
	Goal   AchGoal
	Target string
	Count  int
}

// Threshold: o acontecimento traz um valor (nível, +N, raridade) comparado com Count; as
// outras conquistas contam acontecimentos até Count.
func (a *AchievementDef) Threshold() bool {
	return a.Goal == AchLevel || a.Goal == AchEnchant || a.Goal == AchCraft
}

// Matches diz se o alvo do acontecimento vale para esta conquista.
func (a *AchievementDef) Matches(target string) bool {
	if a.Target == "" {
		return true
	}
	for _, t := range strings.Split(a.Target, "|") {
		if t == target {
			return true
		}
	}
	return false
}

// AchievementState é o que fica salvo no personagem (characters.achievements).
type AchievementState struct {
	// Unlocked: conquista → quando foi desbloqueada (Unix, ms).
	Unlocked map[string]int64 `json:"unlocked,omitempty"`
	// Counts: progresso das conquistas de contagem ainda não desbloqueadas.
	Counts map[string]int `json:"counts,omitempty"`
}

// skeletons: monstros que contam para "Quebra-Ossos".
const skeletons = "skeleton_minion|skeleton_warrior|skeleton_rogue|skeleton_mage|bone_guard|bone_lord|ozrhal"

// AchievementList: o catálogo, na ordem em que o cliente mostra.
var AchievementList = []*AchievementDef{
	{ID: "first_kill", Name: "Primeiro Sangue", Text: "Derrotar o primeiro monstro.", Title: "Caçador Novato", Goal: AchKill, Count: 1},
	{ID: "tutorial", Name: "Primeiros Passos", Text: "Terminar o tutorial (Os Campos dos Ossos).", Title: "Recruta de Vel'Aran", Goal: AchQuest, Target: "tut_bones", Count: 1},
	{ID: "bones_100", Name: "Quebra-Ossos", Text: "Derrotar 100 esqueletos.", Title: "Quebra-Ossos", Goal: AchKill, Target: skeletons, Count: 100},
	{ID: "level_10", Name: "Aventureiro", Text: "Chegar ao nível 10.", Title: "Aventureiro", Goal: AchLevel, Count: 10},
	{ID: "level_20", Name: "Veterano", Text: "Chegar ao nível 20.", Title: "Veterano", Goal: AchLevel, Count: 20},
	{ID: "level_30", Name: "Lenda Viva", Text: "Chegar ao nível 30.", Title: "Lenda Viva", Goal: AchLevel, Count: 30},
	{ID: "level_40", Name: "Lenda de Erevia", Text: "Chegar ao nível 40.", Title: "Lenda de Erevia", Goal: AchLevel, Count: MaxLevel},
	{ID: "bone_lord", Name: "O Senhor Caiu", Text: "Derrotar o Senhor dos Ossos.", Title: "Algoz do Senhor dos Ossos", Goal: AchKill, Target: "bone_lord", Count: 1},
	{ID: "raid_ozrhal", Name: "Regicida", Text: "Participar da morte de Ozrhal, o Rei Ossário.", Title: "Regicida", Goal: AchRaid, Target: "ozrhal", Count: 1},
	{ID: "invasion", Name: "A Cidade Resiste", Text: "Ajudar a cidade a resistir a uma invasão de esqueletos.", Title: "Defensor de Vel'Aran", Goal: AchDefense, Count: 1},
	{ID: "enchant_7", Name: "Mão Firme", Text: "Encantar um item a +7.", Title: "Encantador", Goal: AchEnchant, Count: 7},
	{ID: "first_trade", Name: "Negócio Fechado", Text: "Concluir uma troca com outro jogador.", Title: "Mercador", Goal: AchTrade, Count: 1},
	{ID: "craft_legend", Name: "Forja Lendária", Text: "Criar um item lendário no Ferreiro.", Title: "Mestre Ferreiro", Goal: AchCraft, Count: int(GradeLegendary)},
	{ID: "pvp_win", Name: "Duelista", Text: "Vencer um jogador em combate (que estava lutando).", Title: "Duelista", Goal: AchPvPWin, Count: 1},
}

// Achievements: as conquistas por id.
var Achievements = func() map[string]*AchievementDef {
	m := make(map[string]*AchievementDef, len(AchievementList))
	for _, a := range AchievementList {
		m[a.ID] = a
	}
	return m
}()

// AchievementsByGoal: as conquistas de cada tipo (um abate só olha as de abate).
var AchievementsByGoal = func() map[AchGoal][]*AchievementDef {
	m := map[AchGoal][]*AchievementDef{}
	for _, a := range AchievementList {
		m[a.Goal] = append(m[a.Goal], a)
	}
	return m
}()

// TitleOf devolve o título dado pela conquista id ("" se não existir).
func TitleOf(id string) string {
	if a := Achievements[id]; a != nil {
		return a.Title
	}
	return ""
}

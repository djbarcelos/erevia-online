package game

import (
	"fmt"
	"math"
)

// Eventos de área com etapas (tunnel#142, roadmap T17). Um EventDef é só dado: onde acontece (um
// círculo), o que dispara (a noite, uma chance por janela ou um horário fixo, sempre com recarga),
// as etapas em ordem (cada uma com limite de tempo), a recompensa por participação (medalha de
// 100%, 85% ou 75% conforme a contribuição) e o posto que cai se o evento falhar. O motor está em
// zone/areaevent.go. Para criar um evento novo, veja docs/eventos-de-area.md.

// StageKind é o tipo de uma etapa.
type StageKind uint8

const (
	// StageKill: matar Count monstros de Mobs, em levas de até MaxAlive vivos.
	StageKill StageKind = iota + 1
	// StageDefend: segurar o estandarte por Hold segundos. Os monstros de Mobs marcham até ele e o
	// ferem enquanto estiverem perto; a etapa falha se ele cair.
	StageDefend
	// StageBoss: matar o chefe Boss (um monstro de elite com a vida multiplicada por BossHPMult).
	StageBoss
)

func (k StageKind) String() string {
	switch k {
	case StageKill:
		return "kill"
	case StageDefend:
		return "defend"
	case StageBoss:
		return "boss"
	}
	return "?"
}

// TriggerKind é o gatilho do evento.
type TriggerKind uint8

const (
	// TriggerNight: a cada noite que começa, com a chance Chance.
	TriggerNight TriggerKind = iota + 1
	// TriggerWindow: a cada janela de Window segundos, com a chance Chance.
	TriggerWindow
	// TriggerFixed: todo dia, no horário Hour:Minute do fuso dos eventos.
	TriggerFixed
)

// TriggerDef: o que dispara o evento. Cooldown (segundos) é o mínimo entre o fim de um e o começo
// do seguinte, valha qual for o gatilho.
type TriggerDef struct {
	Kind         TriggerKind
	Chance       float64 // 0 a 1 (noite e janela)
	Window       float64 // segundos entre sorteios (janela)
	Hour, Minute int     // horário fixo
	Cooldown     float64
	// Warn: segundos entre o anúncio e a primeira etapa (0 = já começa).
	Warn float64
}

// StageDef é uma etapa do evento.
type StageDef struct {
	Kind  StageKind
	Label string  // o que o painel mostra ("Matar os Devotos")
	Say   string  // o anúncio ao começar a etapa
	Limit float64 // segundos para cumprir (estourou: o evento falha)
	// Matar e defender: o que nasce.
	Mobs     []string
	Count    int     // matar: quantos precisam morrer
	MaxAlive int     // quantos vivos de uma vez
	Every    float64 // segundos entre levas (padrão 2)
	// Defender.
	Hold      float64 // segundos a segurar
	TargetHP  float64 // vida do estandarte
	DrainRate float64 // vida que cada monstro perto tira do estandarte por segundo
	// Chefe.
	Boss       string
	BossHPMult float64
}

// RewardDef: a recompensa por participação. A contribuição de cada jogador soma o dano nos
// monstros do evento, a cura que ele fez (vida roubada) e a presença no círculo
// (PresencePerSec por segundo). As faixas são frações da vida total dos monstros que nasceram:
// abaixo de BronzeAt, nada.
type RewardDef struct {
	PresencePerSec float64
	GoldAt         float64 // 100% da recompensa
	SilverAt       float64 // 85%
	BronzeAt       float64 // 75%
	// Medals: quantas medalhas por faixa (ouro, prata, bronze).
	Medals [3]int
	Item   string // o item "medalha"
	// XPFrac: fração da barra de XP do nível (para 100%). Gold: base + por nível.
	XPFrac               float64
	GoldBase, GoldPerLvl float64
	// FailMult: na falha, quem chegou a uma faixa leva esta fração das moedas e da XP (sem medalha).
	FailMult float64
}

// Faixas de contribuição.
const (
	MedalNone = iota
	MedalBronze
	MedalSilver
	MedalGold
)

// MedalShare: a fração da recompensa de cada faixa (100%, 85% e 75%).
var MedalShare = [4]float64{0, 0.75, 0.85, 1}

// MedalName: o nome da faixa para as mensagens.
var MedalName = [4]string{"nenhuma", "bronze", "prata", "ouro"}

// Tier devolve a faixa de uma contribuição, dada a vida total do evento.
func (r *RewardDef) Tier(points, pool float64) int {
	if pool <= 0 {
		return MedalNone
	}
	f := points / pool
	switch {
	case f >= r.GoldAt:
		return MedalGold
	case f >= r.SilverAt:
		return MedalSilver
	case f >= r.BronzeAt:
		return MedalBronze
	}
	return MedalNone
}

// MedalCount: quantas medalhas a faixa dá.
func (r *RewardDef) MedalCount(tier int) int {
	if tier < MedalBronze || tier > MedalGold {
		return 0
	}
	return r.Medals[MedalGold-tier] // Medals é ouro, prata, bronze
}

// EventDef é um evento de área.
type EventDef struct {
	ID, Name string
	Enabled  bool
	Map      string // "" = o mapa principal
	X, Z     float64
	Radius   float64 // o círculo: a presença conta aqui, e os monstros nascem na borda
	Place    string  // completa os anúncios ("no Bastião Cinzento")
	Level    int     // nível dos monstros
	MinLevel int     // quem tem menos nível não conta (0 = todos)
	Trigger  TriggerDef
	Stages   []StageDef
	Reward   RewardDef
	// Post: o posto (chave da pedra de teleporte) que cai se o evento falhar ("" = nenhum).
	Post       string
	CloseSecs  float64 // quanto tempo o posto fica fechado (15 min)
	SuccessMsg string
	FailMsg    string
}

// PostDef é um posto avançado que pode cair: a pedra de teleporte (de mesma chave), um mercador e
// o NPC de história. Enquanto o posto está fechado, a pedra, o mercador e o NPC recusam.
type PostDef struct {
	Key, Name string
	Talker    string // o NPC de história do posto ("" = nenhum)
	Merchant  *PostMerchant
}

// PostMerchant: o mercador de poções do posto.
type PostMerchant struct {
	Name, Title, Model string
	X, Z               float64
	Look               *Appearance
	Stock              []string
}

// BastionMedal: a medalha do Bastião, o item da recompensa.
const BastionMedal = "bastion_medal"

// AreaEvents: os eventos de área. "Acender o Farol" (Costa) entra quando a Costa existir no mapa.
var AreaEvents = []*EventDef{bastionEvent, lighthouseEvent}

// Posts: os postos que podem cair.
var Posts = []*PostDef{bastionPost}

// bastionPost: o Bastião Cinzento. A chave é a da pedra de teleporte (zone/teleport.go).
var bastionPost = &PostDef{Key: "bastiao", Name: "Bastião Cinzento", Talker: TalkBrother,
	Merchant: &PostMerchant{Name: "Odric", Title: "Intendente do Bastião", Model: "knight",
		X: BastionX - 6, Z: BastionZ + 4, Look: &Appearance{Skin: 3, Hair: 2, Outfit: 4, Head: "knight"},
		Stock: []string{"potion_hp_s", "potion_hp_l", "potion_mp_s", "potion_mp_l", "scroll_return"}}}

// bastionEvent: "O Bastião resiste" (roadmap, seção 7). Três etapas: as ondas dos Devotos, a defesa
// do Estandarte e o Arauto dos Devotos. Falhar derruba o posto por 15 minutos.
var bastionEvent = &EventDef{
	ID: "bastiao_resiste", Name: "O Bastião resiste", Enabled: true,
	X: BastionX, Z: BastionZ, Radius: 28, Place: "no Bastião Cinzento", Level: 26, MinLevel: 20,
	Trigger: TriggerDef{Kind: TriggerNight, Chance: 0.5, Cooldown: 20 * 60, Warn: 60},
	Stages: []StageDef{
		{Kind: StageKill, Label: "Derrotar os Devotos", Limit: 150,
			Say:  "Os Devotos do Ossário atacam o Bastião! Derrotem as ondas que cercam o pátio.",
			Mobs: []string{"ossario_acolito", "ossario_necromante", "carnical", "lacaio_ossario"}, Count: 16, MaxAlive: 8, Every: 3},
		{Kind: StageDefend, Label: "Defender o Estandarte", Limit: 180,
			Say:  "Os Devotos querem derrubar o Estandarte do Bastião! Segurem o pátio até o fim da contagem.",
			Mobs: []string{"carnical", "ossario_acolito", "lacaio_ossario"}, MaxAlive: 8, Every: 4,
			Hold: 90, TargetHP: 100, DrainRate: 1.5},
		{Kind: StageBoss, Label: "Derrotar o Cavaleiro do Ossário", Limit: 180,
			Say:  "O Cavaleiro do Ossário avança pelo portão! Derrubem-no antes que o Bastião ceda.",
			Boss: "ossario_cavaleiro", BossHPMult: 6},
	},
	Reward: RewardDef{PresencePerSec: 6, GoldAt: 0.10, SilverAt: 0.05, BronzeAt: 0.015, Medals: [3]int{3, 2, 1}, Item: BastionMedal,
		XPFrac: 0.15, GoldBase: 100, GoldPerLvl: 60, FailMult: 0.5},
	Post: "bastiao", CloseSecs: 15 * 60,
	SuccessMsg: "O Bastião resistiu! Os Devotos recuam e a Ordem agradece a quem defendeu o pátio.",
	FailMsg:    "O Bastião caiu! A pedra de teleporte e a loja do posto ficam fechadas por 15 minutos.",
}

// lighthouseEvent: "Acender o Farol" (Costa, A6). A Costa dos Naufrágios ainda não existe no mapa:
// o evento fica desativado, só com o esqueleto dos dados, até a T19 entrar.
var lighthouseEvent = &EventDef{
	ID: "acender_farol", Name: "Acender o Farol", Enabled: false, Map: "costa",
	X: 0, Z: 0, Radius: 25, Place: "no Farol da Costa", Level: 30, MinLevel: 26,
	Trigger: TriggerDef{Kind: TriggerNight, Chance: 0.4, Cooldown: 30 * 60, Warn: 60},
	Stages: []StageDef{
		{Kind: StageKill, Label: "Afastar os náufragos", Limit: 150, Say: "Mortos do mar sobem à praia!",
			Mobs: []string{"carnical"}, Count: 12, MaxAlive: 6, Every: 3},
		{Kind: StageDefend, Label: "Manter o Farol aceso", Limit: 180, Say: "Segurem a chama até o amanhecer do farol!",
			Mobs: []string{"carnical"}, MaxAlive: 6, Every: 4, Hold: 90, TargetHP: 100, DrainRate: 1.5},
	},
	Reward: RewardDef{PresencePerSec: 6, GoldAt: 0.10, SilverAt: 0.05, BronzeAt: 0.015, Medals: [3]int{3, 2, 1}, Item: BastionMedal,
		XPFrac: 0.15, GoldBase: 100, GoldPerLvl: 60, FailMult: 0.5},
}

func bastionMedalItem() *ItemDef {
	d := stackable(BastionMedal, "Medalha do Bastião", KindMaterial, 350, "bastion_medal",
		"Cunhada pela Ordem para quem se manteve de pé quando o Bastião foi atacado. Os intendentes pagam bem por ela.")
	return d
}

// ValidateEvent confere um EventDef (roda na carga dos dados).
func ValidateEvent(e *EventDef) error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("evento %s: %s", e.ID, fmt.Sprintf(format, args...))
	}
	if e.ID == "" || e.Name == "" || e.Radius <= 0 || e.Level <= 0 {
		return bad("faltam id, nome, raio ou nível")
	}
	switch t := e.Trigger; t.Kind {
	case TriggerNight:
		if t.Chance <= 0 || t.Chance > 1 {
			return bad("chance %.2f fora de (0, 1]", t.Chance)
		}
	case TriggerWindow:
		if t.Chance <= 0 || t.Chance > 1 || t.Window <= 0 {
			return bad("janela de %.0f s com chance %.2f inválida", t.Window, t.Chance)
		}
	case TriggerFixed:
		if t.Hour < 0 || t.Hour > 23 || t.Minute < 0 || t.Minute > 59 {
			return bad("horário %02d:%02d inválido", t.Hour, t.Minute)
		}
	default:
		return bad("gatilho desconhecido")
	}
	if e.Trigger.Cooldown < 0 || e.Trigger.Warn < 0 {
		return bad("recarga e aviso não podem ser negativos")
	}
	if len(e.Stages) == 0 {
		return bad("sem etapas")
	}
	for i, s := range e.Stages {
		sb := func(format string, args ...any) error {
			return bad("etapa %d: %s", i+1, fmt.Sprintf(format, args...))
		}
		if s.Limit <= 0 || s.Label == "" {
			return sb("precisa de limite de tempo e rótulo")
		}
		for _, id := range s.Mobs {
			if Monsters[id] == nil {
				return sb("monstro %q não existe", id)
			}
		}
		switch s.Kind {
		case StageKill:
			if s.Count <= 0 || s.MaxAlive <= 0 || len(s.Mobs) == 0 {
				return sb("matar precisa de quantidade, vivos e monstros")
			}
		case StageDefend:
			if s.Hold <= 0 || s.Hold >= s.Limit || s.TargetHP <= 0 || s.DrainRate <= 0 || s.MaxAlive <= 0 || len(s.Mobs) == 0 {
				return sb("defender precisa de tempo a segurar (menor que o limite), vida, dano e monstros")
			}
		case StageBoss:
			if Monsters[s.Boss] == nil || s.BossHPMult < 1 {
				return sb("chefe %q com vida ×%.1f inválido", s.Boss, s.BossHPMult)
			}
		default:
			return sb("tipo desconhecido")
		}
	}
	r := e.Reward
	if !(r.BronzeAt > 0 && r.BronzeAt < r.SilverAt && r.SilverAt < r.GoldAt) || r.PresencePerSec < 0 {
		return bad("as faixas de contribuição precisam crescer (bronze < prata < ouro)")
	}
	if !(r.Medals[0] >= r.Medals[1] && r.Medals[1] >= r.Medals[2] && r.Medals[2] > 0) || Items[r.Item] == nil {
		return bad("medalhas %v do item %q inválidas", r.Medals, r.Item)
	}
	if r.FailMult < 0 || r.FailMult > 1 || math.IsNaN(r.XPFrac) || r.XPFrac < 0 {
		return bad("recompensa inválida")
	}
	if e.Post != "" {
		if PostByKey(e.Post) == nil || e.CloseSecs <= 0 {
			return bad("posto %q sem definição ou sem tempo de fechamento", e.Post)
		}
	}
	return nil
}

// PostByKey: o posto de chave key (nil se não existe).
func PostByKey(key string) *PostDef {
	for _, p := range Posts {
		if p.Key == key {
			return p
		}
	}
	return nil
}

// AreaEventByID: o evento de id id (nil se não existe).
func AreaEventByID(id string) *EventDef {
	for _, e := range AreaEvents {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func init() {
	seen := map[string]bool{}
	for _, e := range AreaEvents {
		if seen[e.ID] {
			panic("evento repetido: " + e.ID)
		}
		seen[e.ID] = true
		if err := ValidateEvent(e); err != nil {
			panic(err)
		}
	}
}

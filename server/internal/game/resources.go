package game

import "math"

// Recursos de classe (docs/evolucoes/README.md, seção 3). Cada evolução tem uma barra
// própria, além da mana: enche com o jeito de jogar da evolução e paga as skills de impacto.
// Regras gerais: máximo 100, começa em 0 (login, troca de zona e de subclasse; não vai para o
// banco), ganho de no máximo 25 por segundo, no máximo 3 alvos contam por lançamento, e cai
// 10/s depois de 5 s fora de combate. DoT, reflexo, dano ≤ 0 e erro não geram.

// GenEvent é o que faz um recurso encher.
type GenEvent uint8

const (
	GenBasicHit     GenEvent = iota // ataque básico acertado
	GenSkillHit                     // skill de dano acertada (por alvo, até o teto de alvos)
	GenCrit                         // golpe crítico (básico ou skill)
	GenDamageTaken                  // golpe de inimigo recebido
	GenMeleeTaken                   // golpe de inimigo a até MeleeTakenRange recebido
	GenBlock                        // golpe bloqueado com escudo
	GenDistinctCast                 // lançar uma skill diferente da última
	GenNewEffect                    // aplicar num inimigo um efeito que ele ainda não tinha
	GenKill                         // abate de inimigo em que você causou dano
	GenEngage                       // primeiro golpe num inimigo que não estava lutando
	GenBackstab                     // golpe pelas costas (cone de BackstabCone atrás do alvo)
	GenExtraTarget                  // cada alvo além do primeiro numa skill (até o teto)
	GenInterrupt                    // atordoar um monstro durante o aviso no chão (cancela o golpe)
	genEventCount
)

// MeleeTakenRange: até onde o golpe recebido conta como de perto (Determinação).
const MeleeTakenRange = 5.0

// BackstabCone: abertura (graus) do cone atrás do alvo que conta como "pelas costas".
const BackstabCone = 120.0

// RangeGen: ganho por acerto à distância (o maior degrau alcançado vale).
type RangeGen struct{ MinDist, Amount float64 }

// ResourceDef descreve o recurso de uma evolução.
type ResourceDef struct {
	Name, Color  string
	Max          float64
	PerSecondCap float64 // ganho máximo por segundo
	TargetCap    int     // alvos que contam por lançamento
	CalmDecay    float64 // queda por segundo fora de combate (depois de OutOfCombatAfter)
	// IdleDecay: queda por segundo depois de IdleAfter s sem ganhar (Combo).
	IdleDecay, IdleAfter float64
	ResetOnMiss          bool // errar zera (Precisão)
	Gens                 [genEventCount]float64
	Ranged               []RangeGen // acertos à distância (Precisão)
}

// Gen: quanto o evento gera (0 = nada).
func (d *ResourceDef) Gen(e GenEvent) float64 {
	if d == nil || e >= genEventCount {
		return 0
	}
	return d.Gens[e]
}

// RangedGen: quanto um acerto à distância d gera (0 = perto demais).
func (d *ResourceDef) RangedGen(dist float64) float64 {
	best := 0.0
	if d == nil {
		return 0
	}
	for _, r := range d.Ranged {
		if dist >= r.MinDist {
			best = math.Max(best, r.Amount)
		}
	}
	return best
}

// newResource: as regras gerais com os ganhos da evolução.
func newResource(name, color string, gens map[GenEvent]float64) *ResourceDef {
	d := &ResourceDef{Name: name, Color: color, Max: 100, PerSecondCap: 25, TargetCap: 3, CalmDecay: 10}
	for e, v := range gens {
		d.Gens[e] = v
	}
	return d
}

// ResourceState é a barra de um personagem (ou de uma build no simulador).
type ResourceState struct {
	Value    float64
	window   float64 // início da janela de 1 s do teto de ganho
	gained   float64 // ganho dentro da janela
	lastGain float64
	LastCast uint32 // última skill lançada (Concentração)
}

// Gain soma até o teto por segundo e o máximo; devolve quanto entrou.
func (s *ResourceState) Gain(d *ResourceDef, n, now float64) float64 {
	if d == nil || n <= 0 || math.IsNaN(n) {
		return 0
	}
	if now-s.window >= 1 || now < s.window {
		s.window, s.gained = now, 0
	}
	n = math.Min(n, math.Min(d.PerSecondCap-s.gained, d.Max-s.Value))
	if n <= 0 {
		return 0
	}
	s.Value += n
	s.gained += n
	s.lastGain = now
	return n
}

// Spend gasta n se houver (false = não deu).
func (s *ResourceState) Spend(n float64) bool {
	if n <= 0 {
		return true
	}
	if s.Value+1e-9 < n {
		return false
	}
	s.Value = math.Max(0, s.Value-n)
	return true
}

// SpendAll gasta tudo e devolve quanto era.
func (s *ResourceState) SpendAll() float64 {
	v := s.Value
	s.Value = 0
	return v
}

// Tick aplica as quedas: fora de combate (lastCombat) e sem ganhar (Combo).
func (s *ResourceState) Tick(d *ResourceDef, now, lastCombat, dt float64) {
	if d == nil || s.Value <= 0 {
		return
	}
	decay := 0.0
	if now-lastCombat > OutOfCombatAfter {
		decay = d.CalmDecay
	}
	if d.IdleDecay > 0 && now-s.lastGain > d.IdleAfter {
		decay = math.Max(decay, d.IdleDecay)
	}
	s.Value = math.Max(0, s.Value-decay*dt)
}

// Miss: errar zera a barra quando o recurso pede (Precisão).
func (s *ResourceState) Miss(d *ResourceDef) {
	if d != nil && d.ResetOnMiss {
		s.Value = 0
	}
}

// Reset zera a barra (login, troca de zona, de evolução ou de subclasse).
func (s *ResourceState) Reset() { *s = ResourceState{} }

// ResourceOf: o recurso da evolução (nil = sem evolução ou sem recurso).
func ResourceOf(class, spec string) *ResourceDef {
	if d := SpecFor(class, spec); d != nil {
		return d.Resource
	}
	return nil
}

// ResCostFor: o custo com as passivas (Ira Desmedida corta, arredondado).
func ResCostFor(s *SkillDef, pv Passives) float64 {
	c := SkillResCost(s)
	if c > 0 && !s.ResSpendAll {
		c = math.Round(c * (1 - math.Min(0.9, pv.Of(PassiveResCostCut))))
	}
	return c
}

// SkillResCost: o custo em recurso da skill (0 = nenhum; ResSpendAll pede ao menos 1).
func SkillResCost(s *SkillDef) float64 {
	if s.ResSpendAll {
		return math.Max(1, s.ResCost)
	}
	return s.ResCost
}

// SpendAllMult: o multiplicador de dano de uma skill que gasta tudo (ResPerPoint por ponto).
func SpendAllMult(s *SkillDef, spent float64) float64 {
	return 1 + s.ResPerPoint*spent
}

// Backstab: o atacante está no cone atrás do alvo (yaw do alvo em radianos, 0 = +Z, como
// no cliente e no servidor: forward = (sin yaw, cos yaw)).
func Backstab(targetYaw, dx, dz float64) bool {
	l := math.Hypot(dx, dz)
	if l == 0 {
		return false
	}
	fx, fz := math.Sin(targetYaw), math.Cos(targetYaw)
	// Vetor do alvo para o atacante, contra a frente do alvo: atrás = ângulo > 180 - cone/2.
	cos := (fx*dx + fz*dz) / l
	return cos <= math.Cos((180-BackstabCone/2)*math.Pi/180)
}

// resources: o recurso de cada evolução (docs/evolucoes/<id>.md, seção Recurso). Os ganhos
// ligados a uma skill (Golpe de Escudo +10, Cólera +20…) ficam em SkillDef.ResOnHit/ResOnCast.
func resources() map[string]*ResourceDef {
	combo := newResource("Combo", "#e0607a", map[GenEvent]float64{GenBasicHit: 5, GenSkillHit: 8, GenBackstab: 5, GenCrit: 3})
	combo.IdleDecay, combo.IdleAfter = 15, 3
	precision := newResource("Precisão", "#8cc850", nil)
	precision.Ranged = []RangeGen{{6, 6}, {12, 10}}
	precision.ResetOnMiss = true
	return map[string]*ResourceDef{
		"bastion":      newResource("Determinação", "#8fb4ff", map[GenEvent]float64{GenBlock: 6, GenMeleeTaken: 1}),
		"herald":       newResource("Fé", "#ffd86a", map[GenEvent]float64{GenKill: 10}),
		"bloodrager":   newResource("Fúria", "#e85050", map[GenEvent]float64{GenBasicHit: 6, GenSkillHit: 4, GenCrit: 3, GenDamageTaken: 2}),
		"warlord":      newResource("Ímpeto", "#e0a848", map[GenEvent]float64{GenEngage: 15, GenBasicHit: 3, GenExtraTarget: 3, GenInterrupt: 15}),
		"elementalist": newResource("Concentração", "#ff9a3c", map[GenEvent]float64{GenDistinctCast: 10, GenBasicHit: 4, GenCrit: 3}),
		"occultist":    newResource("Carga Arcana", "#b46ae0", map[GenEvent]float64{GenNewEffect: 8}),
		"cutthroat":    combo,
		"hunter":       precision,
	}
}

// skillResources: custos e ganhos de recurso das skills que já existem (docs/evolucoes).
type skillRes struct {
	cost, onCast, onHit float64
	mp                  *Scale // novo custo de mana (nil = mantém)
}

var skillResourceTable = map[uint32]skillRes{
	// Bastião: Golpe de Escudo +10 ao acertar, Guarda +15, Desafio +5 por inimigo (até 3);
	// Último Bastião gasta 40.
	SkillBastionBash: {onHit: 10}, SkillBastionGuard: {onCast: 15}, SkillBastionDefy: {onHit: 5},
	SkillBastionLast: {cost: 40, mp: &Scale{30, 0}},
	// Paladino: skills sagradas +8 por alvo; Bênção da Luz em si +4.
	SkillHeraldStrike: {onHit: 8}, SkillHeraldDawn: {onHit: 8}, SkillHorizon: {onHit: 8}, SkillHeraldSun: {onHit: 8},
	SkillHeraldLight: {onCast: 4},
	// Berserker: Golpe Brutal +10, Cólera +20; os golpes pesados gastam Fúria em vez de mana.
	SkillRagerBrutal: {onHit: 10}, SkillRagerWrath: {onCast: 20},
	SkillRagerRed: {cost: 20, mp: &Scale{}}, SkillSeismicAxe: {cost: 25, mp: &Scale{}},
	SkillBloodWhirl: {cost: 30, mp: &Scale{}}, SkillRagerCarnage: {cost: 50, mp: &Scale{}},
	// Senhor da Guerra: Golpe de Comando +10.
	SkillWarlordCommand: {onHit: 10},
	// Elementalista: Cataclismo gasta 40.
	SkillElemCata: {cost: 40},
	// Arcanista: Rajada Arcana +4.
	SkillArcaneBlast: {onHit: 4},
	// Assassino: Sentença gasta 50.
	SkillCutSentence: {cost: 50},
}

func applySkillResources() {
	for id, r := range skillResourceTable {
		s := Skills[id]
		s.ResCost, s.ResOnCast, s.ResOnHit = r.cost, r.onCast, r.onHit
		if r.mp != nil {
			s.MP = *r.mp
		}
	}
	res := resources()
	for _, sp := range SpecList {
		sp.Resource = res[sp.ID]
	}
}

package game

// EffectKind é um efeito temporário (debuff ou buff). Os valores batem com o enum do protocolo.
type EffectKind uint8

const (
	EffectNone      EffectKind = iota
	EffectStun                 // não age nem anda
	EffectSlow                 // Value = fração de redução de velocidade
	EffectBuffDef              // Value = +fração de defesa
	EffectBuffAtk              // Value = +fração de ataque
	EffectBuffSpeed            // Value = +fração de velocidade
)

type EffectDef struct {
	Kind     EffectKind
	Duration float64 // segundos
	Value    float64
}

// SkillTarget diz em quem a skill é usada.
type SkillTarget uint8

const (
	TargetEnemy    SkillTarget = iota + 1 // precisa de um alvo inimigo no alcance
	TargetSelf                            // em si mesmo, na hora
	TargetAreaSelf                        // área ao redor de si, na hora
)

type SkillDef struct {
	ID        uint32
	Class     string
	Name      string
	Key       int // tecla (1 ou 2)
	MPCost    float64
	Cooldown  float64 // segundos
	Range     float64 // alcance (TargetEnemy) ou raio (TargetAreaSelf)
	Target    SkillTarget
	Mult      float64 // multiplicador de dano (0 = sem dano)
	Magic     bool    // usa MAtk/MDef
	ExtraCrit float64 // somado à chance de crítico
	OnTarget  *EffectDef
	OnSelf    *EffectDef
	// Apresentação (cliente).
	FX          string // id do visual e do ícone
	Description string
}

const (
	SkillShieldBash uint32 = iota + 1
	SkillFortress
	SkillBrutalStrike
	SkillFury
	SkillFireball
	SkillIceNova
	SkillBackstab
	SkillShadowStep
)

var Skills = map[uint32]*SkillDef{
	SkillShieldBash: {ID: SkillShieldBash, Class: "knight", Name: "Golpe de Escudo", Key: 1, MPCost: 10, Cooldown: 8,
		Range: 2.5, Target: TargetEnemy, Mult: 1.5, OnTarget: &EffectDef{Kind: EffectStun, Duration: 1.5},
		FX: "shield_bash", Description: "Uma pancada seca com o escudo que deixa o inimigo atordoado."},
	SkillFortress: {ID: SkillFortress, Class: "knight", Name: "Fortaleza", Key: 2, MPCost: 15, Cooldown: 25,
		Target: TargetSelf, OnSelf: &EffectDef{Kind: EffectBuffDef, Duration: 10, Value: 0.5},
		FX: "fortress", Description: "Firma os pés e ergue a guarda, aguentando muito mais pancada."},

	SkillBrutalStrike: {ID: SkillBrutalStrike, Class: "barbarian", Name: "Golpe Brutal", Key: 1, MPCost: 10, Cooldown: 6,
		Range: 2.5, Target: TargetEnemy, Mult: 2.2,
		FX: "brutal_strike", Description: "Um golpe com toda a força do corpo."},
	SkillFury: {ID: SkillFury, Class: "barbarian", Name: "Fúria", Key: 2, MPCost: 15, Cooldown: 25,
		Target: TargetSelf, OnSelf: &EffectDef{Kind: EffectBuffAtk, Duration: 10, Value: 0.3},
		FX: "fury", Description: "Entra em fúria e passa a bater mais forte."},

	SkillFireball: {ID: SkillFireball, Class: "mage", Name: "Bola de Fogo", Key: 1, MPCost: 15, Cooldown: 3,
		Range: 20, Target: TargetEnemy, Mult: 2.5, Magic: true,
		FX: "fireball", Description: "Lança uma bola de fogo que explode no alvo à distância."},
	SkillIceNova: {ID: SkillIceNova, Class: "mage", Name: "Nova de Gelo", Key: 2, MPCost: 30, Cooldown: 10,
		Range: 6, Target: TargetAreaSelf, Mult: 1.2, Magic: true, OnTarget: &EffectDef{Kind: EffectSlow, Duration: 3, Value: 0.5},
		FX: "ice_nova", Description: "Uma onda de gelo explode ao seu redor, ferindo e congelando os inimigos próximos."},

	SkillBackstab: {ID: SkillBackstab, Class: "rogue", Name: "Golpe Furtivo", Key: 1, MPCost: 10, Cooldown: 6,
		Range: 2.5, Target: TargetEnemy, Mult: 2.0, ExtraCrit: 0.5,
		FX: "backstab", Description: "Um golpe traiçoeiro num ponto fraco, com grande chance de crítico."},
	SkillShadowStep: {ID: SkillShadowStep, Class: "rogue", Name: "Passo Sombrio", Key: 2, MPCost: 12, Cooldown: 20,
		Target: TargetSelf, OnSelf: &EffectDef{Kind: EffectBuffSpeed, Duration: 6, Value: 0.4},
		FX: "shadow_step", Description: "Funde-se às sombras e se move muito mais rápido."},
}

// ClassSkills devolve as skills de uma classe, na ordem das teclas.
func ClassSkills(class string) []*SkillDef {
	c, ok := Classes[class]
	if !ok {
		return nil
	}
	return []*SkillDef{Skills[c.Skills[0]], Skills[c.Skills[1]]}
}

package balance

import (
	"math"
	"sort"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// A luta: um personagem típico contra um monstro, em passos do tick da zona
// ---------------------------------------------------------------------------------------
//
// Cada golpe vale o dano médio (game.ExpectedHit): sem sorteio, a mesma luta dá sempre o
// mesmo número. As regras seguem a zona (combat.go e effects.go): skills e ataque básico
// correm em paralelo (a skill não espera o ataque), venenos e
// queimaduras batem 1×/s sem passar pela defesa, a Marca soma ao golpe, atordoar para o
// monstro, Enfraquecer tira ataque, Cegar tira precisão, os buffs de defesa somam.

// Suposições da luta.
const (
	tickDT  = 0.05 // passo da simulação: o tick da zona (20 por segundo)
	timeEps = 1e-6 // folga nas comparações de tempo (somas de 0,05 não são exatas)
	// Como a auto-caça do cliente (client/src/world/autohunt.ts, padrões de Configurações):
	// uma skill por segundo, na ordem de prioridade; poção de vida abaixo de 45% e de mana
	// abaixo de 20% (recarga de 10 s de cada grupo, game.PotionCooldown).
	skillGap        = 1.0
	potionHPBelow   = 0.45
	potionMPBelow   = 0.20
	largePotionFrom = 10   // do nível 10 em diante usa as poções maiores (antes, as menores)
	fightCap        = 300  // luta que passa de 5 min: o monstro não cai (ou o jogador não morre)
	approachFactor  = 0.75 // para a esta fração do alcance (como a zona)
	// A luta abre no alcance do ataque básico; o monstro anda até o próprio alcance (a
	// aproximação é dano de graça para quem bate de longe). Ninguém recua nem foge: a luta é
	// parada. Os efeitos no alvo pegam sempre (a chance de errar já entra no dano médio).
)

// effect ativo numa das pontas (índice = game.EffectKind).
type effect struct {
	value, until, nextTick float64
}

type effects [32]effect

func (e *effects) active(k game.EffectKind, now float64) (float64, bool) {
	ef := e[k]
	if ef.until > now {
		return ef.value, true
	}
	return 0, false
}

func (e *effects) add(k game.EffectKind, value, duration, now float64) {
	e[k] = effect{value: value, until: now + duration, nextTick: now + 1}
}

// rotation: as skills ativas que a luta usa, na ordem de prioridade.
type rotSkill struct {
	s       *game.SkillDef
	rank    int
	mp, cd  float64
	kind    int // 0 buff em si, 1 enfraquecer/marcar o alvo (sem dano), 2 dano
	expMult float64
}

// Efeitos que a simulação entende (os outros, como lentidão, provocar ou correr, não mudam
// uma luta parada contra um monstro só).
func modeledOnSelf(k game.EffectKind) bool {
	switch k {
	case game.EffectBuffAtk, game.EffectWarCry, game.EffectBuffAtkSpeed, game.EffectBuffDef,
		game.EffectFrostArmor, game.EffectShield:
		return true
	}
	return false
}

func modeledOnTarget(k game.EffectKind) bool {
	switch k {
	case game.EffectStun, game.EffectWeaken, game.EffectBlind, game.EffectMark,
		game.EffectPoison, game.EffectBurn, game.EffectPlague:
		return true
	}
	return false
}

// rotation monta a rotação: buffs que faltam, depois enfraquecer/marcar, depois as skills de
// dano da que mais bate para a que menos bate. Ficam de fora as de fuga e de utilidade
// (Piscar, Passo Sombrio, Sumir, Escudo Arcano, Imortal) e provocar sozinho.
func rotation(b *Build) []rotSkill {
	var out []rotSkill
	overload := b.rank(game.SkillOverload) > 0
	for id, r := range b.Ranks {
		s := game.Skills[id]
		if s == nil || r < 1 || s.IsPassive() || s.Blink.Base > 0 {
			continue
		}
		rs := rotSkill{s: s, rank: r, mp: game.SkillMPCost(s, r, overload), cd: game.SkillCooldownFor(s, r, b.Stats.CastSpeed)}
		self := s.OnSelf != nil && modeledOnSelf(s.OnSelf.Kind) || s.Banner != nil
		target := s.OnTarget != nil && modeledOnTarget(s.OnTarget.Kind)
		switch {
		case s.OnTarget != nil && s.OnTarget.Kind == game.EffectMark:
			rs.kind = 1 // a Marca vem antes do dano, mesmo quando também bate
		case s.Mult.At(r) > 0 && s.Target != game.TargetSelf:
			rs.kind = 2
			rs.expMult = s.Mult.At(r) * float64(max(1, s.Hits))
		case s.Target == game.TargetSelf && self:
			rs.kind = 0
		case s.Target != game.TargetSelf && (target || self):
			rs.kind = 1
			if s.OnTarget == nil || !target {
				rs.kind = 0 // só o efeito em si (Desafio do Bastião: o escudo)
			}
		default:
			continue
		}
		out = append(out, rs)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.expMult != b.expMult {
			return a.expMult > b.expMult
		}
		return a.s.ID < b.s.ID
	})
	return out
}

// Fight é o resultado de uma luta.
type Fight struct {
	Time       float64 // segundos até o monstro cair (ou o jogador morrer, ou o limite)
	Died       bool    // o jogador morreu
	Capped     bool    // passou do limite (fightCap)
	HP, MP     float64 // vida e mana no fim
	PotsHP     int
	PotsMP     int
	DamageDone float64
}

// fight é o caçador e o monstro da vez. immortal: o monstro não cai (tempo para morrer);
// potion liga as poções.
type fight struct {
	b      *Build
	p, m   game.Stats
	def    *game.MonsterDef
	night  bool
	rot    []rotSkill
	potHP  *game.ItemDef
	potMP  *game.ItemDef
	potion bool

	now                float64
	hp, mp, mhp        float64
	pNext, mNext       float64
	nextSkill          float64
	cds                [128]float64 // skill → instante em que fica pronta (ids < 128)
	pfx, mfx           effects
	hpPotAt, mpPotAt   float64
	lastStandAt        float64
	dist               float64
	poisonAcc          float64
	out                Fight
	immortal           bool
	bodyRadius, mRange float64
}

// newFight prepara o caçador (vida e mana cheias, nada em recarga). O mesmo caçador luta
// uma vez depois da outra (engage): recargas, buffs, poções e a Muralha Viva seguem de uma
// luta para a próxima, como na zona.
func newFight(b *Build, night, potions bool) *fight {
	f := &fight{b: b, p: b.Stats, night: night, rot: rotation(b), potion: potions}
	f.potHP, f.potMP = game.Items["potion_hp_s"], game.Items["potion_mp_s"]
	if b.Level >= largePotionFrom {
		f.potHP, f.potMP = game.Items["potion_hp_l"], game.Items["potion_mp_l"]
	}
	f.hp, f.mp = f.p.MaxHP, f.p.MaxMP
	return f
}

// engage põe um monstro novo na frente, no alcance do ataque básico (da borda do corpo).
func (f *fight) engage(def *game.MonsterDef, level int) {
	f.def, f.m = def, game.MonsterStats(def, level)
	f.mhp, f.mfx, f.mNext, f.poisonAcc = f.m.MaxHP, effects{}, 0, 0
	f.bodyRadius, f.mRange = def.BodyRadius, f.m.Range
	f.dist = f.p.Range*approachFactor + f.bodyRadius
	f.out = Fight{}
}

// rest: o tempo passa sem luta (andar ou descansar), com a regeneração de fora de combate
// depois dos primeiros 5 s.
func (f *fight) rest(t float64) {
	courage := 0.0
	if r := f.b.rank(game.SkillCourage); r > 0 {
		courage = game.Skills[game.SkillCourage].PValue.At(r)
	}
	f.hp = regenFor(f.hp, f.p.MaxHP, t, f.p.HPRegen, courage, game.CalmHPRegenMult)
	f.mp = regenFor(f.mp, f.p.MaxMP, t, f.p.MPRegen, 0, game.CalmMPRegenMult)
	f.now += t
}

// restNeeded: quanto descansar para chegar às metas de vida e mana.
func (f *fight) restNeeded() float64 {
	courage := 0.0
	if r := f.b.rank(game.SkillCourage); r > 0 {
		courage = game.Skills[game.SkillCourage].PValue.At(r)
	}
	return math.Max(
		regenTo(f.hp, restHPTarget*f.p.MaxHP, f.p.HPRegen, courage, game.CalmHPRegenMult),
		regenTo(f.mp, restMPTarget*f.p.MaxMP, f.p.MPRegen, 0, game.CalmMPRegenMult))
}

// Atributos com os efeitos ativos (as mesmas contas de combatant.go).
func (f *fight) pAtk(magic bool) float64 {
	a := f.p.PAtk
	if magic {
		a = f.p.MAtk
	}
	bonus := 0.0
	if v, ok := f.pfx.active(game.EffectBuffAtk, f.now); ok {
		bonus += v
	}
	if v, ok := f.pfx.active(game.EffectWarCry, f.now); ok {
		bonus += v
	}
	return a * math.Max(0.1, 1+bonus)
}

func (f *fight) pDef(magic bool) float64 {
	d := f.p.PDef
	if magic {
		d = f.p.MDef
	}
	bonus := 0.0
	for _, k := range []game.EffectKind{game.EffectBuffDef, game.EffectBanner, game.EffectFrostArmor} {
		if v, ok := f.pfx.active(k, f.now); ok {
			bonus += v
		}
	}
	return d * (1 + bonus)
}

func (f *fight) pInterval() float64 {
	iv := f.p.AtkInterval
	if v, ok := f.pfx.active(game.EffectBuffAtkSpeed, f.now); ok {
		iv /= 1 + v
	}
	return iv
}

func (f *fight) mAtk() float64 {
	a := f.m.PAtk
	if f.m.Magic {
		a = f.m.MAtk
	}
	if v, ok := f.mfx.active(game.EffectWeaken, f.now); ok {
		a *= math.Max(0.1, 1-v)
	}
	if f.night {
		a *= game.NightMonsterAtk
	}
	return a
}

func (f *fight) mDef(magic bool) float64 {
	if magic {
		return f.m.MDef
	}
	return f.m.PDef
}

func (f *fight) mStunned() bool {
	_, ok := f.mfx.active(game.EffectStun, f.now)
	return ok
}

func (f *fight) mDead() bool { return !f.immortal && f.mhp <= 0 }

// hit: o golpe do jogador no monstro (dano médio), com a Marca, o roubo de vida e o veneno
// da Lâmina Envenenada.
func (f *fight) hit(magic bool, mult, extraCrit float64, noMiss bool) {
	in := game.HitInput{Atk: f.pAtk(magic), Def: f.mDef(magic), Mult: mult,
		Crit: f.p.Crit + extraCrit, CritMult: f.p.CritDamage, Accuracy: f.p.Accuracy, Evasion: f.m.Evasion, NoMiss: noMiss}
	dmg := game.ExpectedHit(in)
	if v, ok := f.mfx.active(game.EffectMark, f.now); ok {
		dmg *= 1 + v
	}
	f.damageMonster(dmg)
	if r := f.b.rank(game.SkillBloodthirst); r > 0 {
		f.hp = math.Min(f.p.MaxHP, f.hp+dmg*game.Skills[game.SkillBloodthirst].PValue.At(r))
	}
	if r := f.b.rank(game.SkillPoisonedBlade); r > 0 && !f.mDead() {
		chance := 1.0
		if !noMiss {
			chance = game.HitChance(in.Accuracy, in.Evasion)
		}
		f.poisonAcc += chance * game.Skills[game.SkillPoisonedBlade].PValue.At(r)
		if f.poisonAcc >= 1 {
			f.poisonAcc--
			v := math.Max(1, math.Round(f.pAtk(false)*game.PoisonProcFrac))
			f.mfx.add(game.EffectPoison, v, game.PoisonProcSecs, f.now)
		}
	}
}

func (f *fight) damageMonster(dmg float64) {
	f.out.DamageDone += dmg
	if !f.immortal {
		f.mhp -= dmg
	}
}

// cast usa a skill (custo, recarga, efeitos), como castSkill + skillHit da zona.
func (f *fight) cast(rs rotSkill) {
	s, r := rs.s, rs.rank
	f.mp -= rs.mp
	f.cds[s.ID] = f.now + rs.cd
	if def := s.OnSelf.At(r); def != nil && modeledOnSelf(def.Kind) {
		f.pfx.add(def.Kind, def.Value, def.Duration, f.now)
	}
	if s.Banner != nil {
		f.pfx.add(s.Banner.Effect, s.Banner.Value, s.Banner.Duration, f.now)
	}
	if s.Target == game.TargetSelf {
		return
	}
	landed := s.Mult.At(r) <= 0
	if mult := s.Mult.At(r); mult > 0 {
		if s.ExecuteBelow > 0 && !f.immortal && f.mhp < f.m.MaxHP*s.ExecuteBelow {
			mult *= s.ExecuteMult
		}
		for i := 0; i < max(1, s.Hits) && !f.mDead(); i++ {
			f.hit(s.Magic, mult, s.ExtraCrit, s.NoMiss)
		}
		landed = true // o efeito pega na média (a chance de errar já entrou no dano)
	}
	if f.mDead() || !landed {
		return
	}
	if spec := s.OnTarget; spec != nil && modeledOnTarget(spec.Kind) &&
		(s.OnTargetNeeds == game.EffectNone || f.mfxActive(s.OnTargetNeeds)) && !f.def.ImmuneTo(spec.Kind) {
		def := spec.At(r)
		if spec.AtkFrac {
			def.Value = math.Round(f.pAtk(s.Magic) * def.Value)
		}
		f.mfx.add(def.Kind, def.Value, def.Duration, f.now)
	}
	if s.Fire && (s.OnTarget == nil || s.OnTarget.Kind != game.EffectBurn) {
		if er := f.b.rank(game.SkillEmber); er > 0 {
			v := math.Round(f.pAtk(true) * game.Skills[game.SkillEmber].PValue.At(er))
			f.mfx.add(game.EffectBurn, math.Max(1, v), game.FireBurnSeconds, f.now)
		}
	}
}

func (f *fight) mfxActive(k game.EffectKind) bool {
	_, ok := f.mfx.active(k, f.now)
	return ok
}

// inRange: a skill alcança o monstro (da borda do corpo dele).
func (f *fight) inRange(rng float64) bool { return f.dist-f.bodyRadius <= rng }

func (f *fight) wants(rs rotSkill) bool {
	s := rs.s
	if f.now+timeEps < f.cds[s.ID] || f.mp < rs.mp {
		return false
	}
	if s.Target != game.TargetSelf && !f.inRange(s.Range) {
		return false
	}
	switch rs.kind {
	case 0: // buff: só se não estiver ativo
		if s.OnSelf != nil {
			if _, ok := f.pfx.active(s.OnSelf.Kind, f.now); ok {
				return false
			}
		}
		if s.Banner != nil {
			if _, ok := f.pfx.active(s.Banner.Effect, f.now); ok {
				return false
			}
		}
	case 1: // efeito no alvo: só se não estiver ativo
		if _, ok := f.mfx.active(s.OnTarget.Kind, f.now); ok {
			return false
		}
	}
	return true
}

// monsterHit: o golpe do monstro no jogador, passando pelo escudo e pela Muralha Viva.
func (f *fight) monsterHit() {
	acc := f.m.Accuracy
	if v, ok := f.mfx.active(game.EffectBlind, f.now); ok {
		acc -= v * 500
	}
	dmg := game.ExpectedHit(game.HitInput{Atk: f.mAtk(), Def: f.pDef(f.m.Magic), Mult: 1,
		Crit: f.m.Crit, CritMult: f.m.CritDamage, Accuracy: acc, Evasion: f.p.Evasion})
	if sh := &f.pfx[game.EffectShield]; sh.until > f.now && sh.value > 0 {
		a := math.Min(sh.value, dmg)
		sh.value -= a
		dmg -= a
		if sh.value <= 0 {
			sh.until = 0
		}
	}
	f.hp -= dmg
	if f.b.rank(game.SkillLivingWall) > 0 && f.hp > 0 && f.now+timeEps >= f.lastStandAt && f.hp < f.p.MaxHP*game.LastStandBelow {
		f.lastStandAt = f.now + game.LastStandCD
		v := math.Round(f.p.MaxHP * game.Skills[game.SkillLivingWall].PValue.At(1))
		f.pfx.add(game.EffectShield, v, game.LastStandSeconds, f.now)
	}
}

// tickDoTs: venenos, queimaduras e pragas no monstro, 1×/s (sem defesa nem Marca).
func (f *fight) tickDoTs() {
	for _, k := range []game.EffectKind{game.EffectPoison, game.EffectBurn, game.EffectPlague} {
		ef := &f.mfx[k]
		for ef.until > f.now-0.001 && f.now+timeEps >= ef.nextTick && !f.mDead() {
			ef.nextTick++
			f.damageMonster(math.Max(1, math.Round(ef.value)))
		}
	}
}

func (f *fight) regen() {
	hp, mp := f.p.HPRegen, f.p.MPRegen
	if r := f.b.rank(game.SkillCourage); r > 0 { // a aura vale para quem a tem
		hp += game.Skills[game.SkillCourage].PValue.At(r)
	}
	if f.b.rank(game.SkillBearSpirit) > 0 && f.hp < f.p.MaxHP*game.BearBelow {
		hp += f.p.MaxHP * game.Skills[game.SkillBearSpirit].PValue.At(1)
	}
	f.hp = math.Min(f.p.MaxHP, f.hp+hp*tickDT)
	f.mp = math.Min(f.p.MaxMP, f.mp+mp*tickDT)
}

func (f *fight) potions() {
	if !f.potion {
		return
	}
	if f.hp < f.p.MaxHP*potionHPBelow && f.now+timeEps >= f.hpPotAt {
		f.hp = math.Min(f.p.MaxHP, f.hp+f.potHP.HealHP)
		f.hpPotAt = f.now + game.CooldownSeconds(f.potHP.Cooldown)
		f.out.PotsHP++
	}
	if f.mp < f.p.MaxMP*potionMPBelow && f.now+timeEps >= f.mpPotAt {
		f.mp = math.Min(f.p.MaxMP, f.mp+f.potMP.HealMP)
		f.mpPotAt = f.now + game.CooldownSeconds(f.potMP.Cooldown)
		f.out.PotsMP++
	}
}

// moveMonster: o monstro anda até o próprio alcance (parado se atordoado ou preso).
func (f *fight) moveMonster() {
	stop := f.mRange * approachFactor
	if f.dist <= stop {
		return
	}
	if f.mStunned() || f.mfxActive(game.EffectRoot) {
		return
	}
	speed := f.m.Speed
	if v, ok := f.mfx.active(game.EffectSlow, f.now); ok {
		speed *= 1 - v
	}
	f.dist = math.Max(stop, f.dist-speed*tickDT)
}

// run simula até o monstro cair, o jogador morrer ou o limite (a partir do instante atual
// do caçador).
func (f *fight) run() Fight {
	start := f.now
	for step := 1; ; step++ {
		f.now = start + float64(step)*tickDT
		if f.now-start > fightCap {
			f.out.Capped = true
			break
		}
		// Contra o monstro que não cai: vida cheia de novo depois de 30 s de luta = aguenta.
		if f.immortal && step%600 == 0 && f.hp >= f.p.MaxHP {
			f.out.Capped = true
			break
		}
		f.tickDoTs()
		if f.mDead() {
			break
		}
		// Jogador: uma skill por segundo e o ataque básico quando a recarga dele permite.
		for _, rs := range f.rot {
			if f.now+timeEps < f.nextSkill {
				break
			}
			if f.wants(rs) {
				f.cast(rs)
				f.nextSkill = f.now + skillGap
				break
			}
		}
		if !f.mDead() && f.now+timeEps >= f.pNext && f.inRange(f.p.Range) {
			f.hit(f.p.Magic, 1, 0, false)
			f.pNext = f.now + f.pInterval()
		}
		if f.mDead() {
			break
		}
		// Monstro.
		f.moveMonster()
		if !f.mStunned() && f.dist <= f.mRange && f.now+timeEps >= f.mNext {
			f.monsterHit()
			f.mNext = f.now + f.m.AtkInterval
		}
		if f.hp <= 0 {
			f.out.Died = true
			break
		}
		f.regen()
		f.potions()
	}
	f.out.Time = f.now - start
	f.hp = math.Max(0, f.hp)
	f.out.HP, f.out.MP = f.hp, f.mp
	return f.out
}

// KillFight: uma luta só, da vida e mana cheias e sem nada em recarga (a abertura).
func KillFight(b *Build, def *game.MonsterDef, level int, night bool) Fight {
	f := newFight(b, night, true)
	f.engage(def, level)
	return f.run()
}

// TimeToDie: quanto o jogador aguenta lutando contra o monstro sem fim (o monstro não cai),
// com regeneração e, se potions, poções (+Inf = aguenta mais que fightCap), e o dano por
// segundo da rotação nessa luta longa (recargas e mana de verdade, sem sobra de golpe).
func TimeToDie(b *Build, def *game.MonsterDef, level int, night, potions bool) (ttd, dps float64) {
	f := newFight(b, night, potions)
	f.engage(def, level)
	f.immortal = true
	out := f.run()
	dps = out.DamageDone / out.Time
	if !out.Died {
		return math.Inf(1), dps
	}
	return out.Time, dps
}

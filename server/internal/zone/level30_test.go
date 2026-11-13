package zone

import (
	"math"
	"strconv"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// ---------------------------------------------------------------------------------------
// Nível 30 e o 5º degrau da árvore
// ---------------------------------------------------------------------------------------

// dummies: alvos de treino com muita vida (nada morre no meio do teste), na ordem pedida.
func dummies(z *Zone, xs ...float64) []*entity {
	out := make([]*entity, len(xs))
	for _, m := range z.monsters {
		for i, x := range xs {
			if math.Abs(m.pos.X-x) < 0.01 {
				out[i] = m
			}
		}
		m.stats.MaxHP, m.hp = 1e6, 1e6
	}
	return out
}

// castUntil lança a skill (recarga e mana zeradas a cada vez) até ok() valer; o efeito no
// alvo só pega quando o golpe acerta.
func castUntil(z *Zone, p *entity, id uint32, target *entity, ok func() bool) bool {
	for i := 0; i < 30; i++ {
		p.mp = p.stats.MaxMP
		p.pl.cooldowns = map[uint32]float64{}
		z.castSkill(p, game.Skills[id], target)
		if ok() {
			return true
		}
	}
	return false
}

// hitsSince conta os CombatEvents do frame (desde o índice n) por alvo.
func hitsSince(z *Zone, src *entity, n int) map[uint32]int {
	out := map[uint32]int{}
	for _, c := range z.frame.combat[n:] {
		if c.SourceId == src.id {
			out[c.TargetId]++
		}
	}
	return out
}

func TestProgressionToLevel30(t *testing.T) {
	z := newCombatZone(fixed("stone_golem", 28, 34, 0))
	p, peer := joinAt(z, 1, "knight", 30, 0, 0)

	// Do 1 ao 30 só com XP: sobe nível a nível, sem pular nenhum aviso.
	for p.level < game.MaxLevel {
		z.grantXP(p, game.XPToNext(p.level)/3+1)
	}
	if p.level != 30 || p.pl.xp < game.XPLevelStart(30) {
		t.Fatalf("nível %d com %d de XP", p.level, p.pl.xp)
	}
	z.tickFor(0.1)
	ups := 0
	for _, m := range peer.msgs {
		for _, l := range m.GetSnapshot().GetLevelUps() {
			if l.Id == p.id {
				ups++
			}
		}
	}
	if ups != 29 {
		t.Errorf("avisos de nível: %d, want 29", ups)
	}
	// No 30: 31 pontos de skill (2 nas iniciais) e 3 de atributo por nível.
	prog := z.progressPB(p)
	if prog.FreeSkillPoints != 29 || prog.FreeAttrPoints != 87 {
		t.Errorf("pontos no 30: skill %d (want 29), atributo %d (want 87)", prog.FreeSkillPoints, prog.FreeAttrPoints)
	}
	if s := z.selfStats(p); s.Level != 30 || s.XpNextLevel != 0 {
		t.Errorf("barra de XP no 30: nível %d, próximo %d", s.Level, s.XpNextLevel)
	}
	// Passou do máximo: a XP para de contar.
	xp := p.pl.xp
	z.grantXP(p, 1e6)
	if p.pl.xp != xp || p.level != 30 {
		t.Error("XP no nível máximo deveria ser ignorada")
	}

	// Relogar com a XP do 30 volta no 30.
	z2 := newCombatZone()
	q, _ := joinAt(z2, 2, "mage", 30, 0, game.XPLevelStart(30)+5)
	if q.level != 30 {
		t.Fatalf("relogar com XP do 30 deu nível %d", q.level)
	}

	// Golem do fundo do vale dá XP de verdade para quem está no 28.
	g := z.onlyMonster()
	r, _ := joinAt(z, 3, "rogue", 31, 0, 0)
	atLevel(z, r, 28)
	before := r.pl.xp
	g.mon.hate = map[uint32]float64{r.id: 1} // o cavaleiro do começo não bateu nele
	z.damage(r, g, g.hp+1, nil)
	if !g.dead {
		t.Fatal("golem não morreu")
	}
	got := r.pl.xp - before
	if want := game.MonsterXP(g.mon.def, 28, 28); math.Abs(float64(got-want)) > float64(want)*0.6 {
		t.Errorf("golem nv 28 deu %d de XP, esperava perto de %d", got, want)
	}
	if kills := float64(game.XPToNext(28)) / float64(got); kills < 10 || kills > 45 {
		t.Errorf("nível 28 pede %.0f golens", kills)
	}
}

func TestLearnFifthTierAtLevel20(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "mage", 30, 0, 0)
	atLevel(z, p, 19)
	branch := []*pb.SkillRank{
		{SkillId: game.SkillEmber, Rank: 1}, {SkillId: game.SkillEmberRain, Rank: 1},
		{SkillId: game.SkillMeteor, Rank: 1}, {SkillId: game.SkillInferno, Rank: 1},
	}
	z.handle(progressEvent{id: p.id, msg: &pb.LearnSkills{Ranks: branch}})
	if p.pl.rank(game.SkillInferno) != 0 {
		t.Fatal("5º degrau aprendido no nível 19")
	}
	atLevel(z, p, 20)
	z.tickFor(1)
	z.handle(progressEvent{id: p.id, msg: &pb.LearnSkills{Ranks: branch}})
	if p.pl.rank(game.SkillInferno) != 1 || p.pl.rank(game.SkillMeteor) != 1 {
		t.Fatalf("5º degrau não aprendido no 20: %v", p.pl.ranks)
	}
	// Aprendida, a skill entra na barra de atalhos.
	slot := "skill:" + strconv.Itoa(int(game.SkillInferno))
	z.tickFor(1)
	slots := make([]string, HotbarSize)
	slots[0] = slot
	z.handle(progressEvent{id: p.id, msg: &pb.SetHotbar{Slots: slots}})
	if p.pl.hotbar[0] != slot {
		t.Errorf("Inferno não foi para a barra: %v", p.pl.hotbar)
	}
}

func TestFifthTierKnight(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0), fixed("skeleton_warrior", 20, 34, 0), fixed("skeleton_warrior", 20, 44, 0))
	ms := dummies(z, 32, 34, 44)
	near, near2, far := ms[0], ms[1], ms[2]
	k, _ := joinAt(z, 1, "knight", 30, 0, 0)
	ally, _ := joinAt(z, 2, "mage", 33, 3, 0)
	stranger, _ := joinAt(z, 3, "rogue", 31, 2, 0)
	atLevel(z, k, 20)
	learn(z, k, map[uint32]int{game.SkillBulwark: 1, game.SkillHorizon: 1, game.SkillAegis: 3})

	// Baluarte: dano e atordoamento em volta, e os monstros passam a odiar o cavaleiro.
	near.mon.hate[ally.id] = 5000
	n := len(z.frame.combat)
	if !castUntil(z, k, game.SkillBulwark, nil, func() bool { return near.stunned(z.now) && near2.stunned(z.now) }) {
		t.Fatal("Baluarte deveria atordoar quem está a até 5 m")
	}
	if hit := hitsSince(z, k, n); hit[far.id] > 0 || far.stunned(z.now) {
		t.Error("Baluarte acertou quem está longe")
	}
	if topHate(near.mon.hate) != k.id {
		t.Error("Baluarte deveria puxar o ódio para o cavaleiro")
	}

	// Corte do Horizonte: o alvo e quem está a até 3,5 m dele, mais lentidão.
	near.removeEffect(game.EffectSlow)
	n = len(z.frame.combat)
	if !castUntil(z, k, game.SkillHorizon, near, func() bool { return near.has(game.EffectSlow, z.now) }) {
		t.Fatal("Corte do Horizonte deveria deixar o alvo lento")
	}
	if hit := hitsSince(z, k, n); hit[near.id] == 0 || hit[near2.id] == 0 || hit[far.id] > 0 {
		t.Errorf("Corte do Horizonte: %v", hit)
	}

	// Égide da Ordem (rank 3): escudo de 500 em você e no grupo perto; estranhos não.
	makeParty(z, k, ally)
	z.castSkill(k, game.Skills[game.SkillAegis], nil)
	for _, p := range []*entity{k, ally} {
		if sh := p.effectRef(game.EffectShield, z.now); sh == nil || sh.value != 500 {
			t.Fatalf("%s sem o escudo da Égide: %+v", p.name, sh)
		}
	}
	if stranger.has(game.EffectShield, z.now) {
		t.Error("Égide protegeu quem não é do grupo")
	}
	hp := ally.hp
	z.damage(near, ally, 300, nil)
	if ally.hp != hp {
		t.Error("o escudo deveria absorver o dano")
	}
}

func TestFifthTierBarbarian(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0), fixed("skeleton_warrior", 20, 34, 0), fixed("skeleton_warrior", 20, 44, 0))
	ms := dummies(z, 32, 34, 44)
	near, near2, far := ms[0], ms[1], ms[2]
	b, _ := joinAt(z, 1, "barbarian", 31, 0, 0)
	atLevel(z, b, 20)
	learn(z, b, map[uint32]int{game.SkillSeismicAxe: 1, game.SkillBloodWhirl: 1, game.SkillStoneSkin: 2})

	// Machadada Sísmica: acerta em volta do alvo e bate 50% a mais abaixo de 30% de vida.
	n := len(z.frame.combat)
	z.castSkill(b, game.Skills[game.SkillSeismicAxe], near)
	if hit := hitsSince(z, b, n); hit[near.id] != 1 || hit[near2.id] != 1 || hit[far.id] > 0 {
		t.Errorf("Machadada Sísmica: %v", hit)
	}
	avg := func(frac float64) float64 {
		total, k := 0.0, 0
		for i := 0; i < 80; i++ {
			near.hp, near.dead = near.stats.MaxHP*frac, false
			n := len(z.frame.combat)
			z.skillHit(b, near, game.Skills[game.SkillSeismicAxe], 1)
			if c := z.frame.combat[n]; !c.Miss && !c.Crit {
				total += float64(c.Damage)
				k++
			}
		}
		return total / float64(max(1, k))
	}
	if high, low := avg(1), avg(0.2); math.Abs(low/high-1.5) > 0.2 {
		t.Errorf("Machadada Sísmica abaixo de 30%%: %.0f vs %.0f", low, high)
	}
	near.hp = near.stats.MaxHP

	// Redemoinho Sangrento: dois golpes em cada um a até 4 m.
	n = len(z.frame.combat)
	b.pl.cooldowns = map[uint32]float64{}
	b.mp = b.stats.MaxMP
	z.castSkill(b, game.Skills[game.SkillBloodWhirl], nil)
	if hit := hitsSince(z, b, n); hit[near.id] != 2 || hit[near2.id] != 2 || hit[far.id] > 0 {
		t.Errorf("Redemoinho Sangrento: %v", hit)
	}

	// Pele de Pedra (rank 2): +45% de defesa em si e lentidão em volta.
	def := b.def(false, z.now)
	if !castUntil(z, b, game.SkillStoneSkin, nil, func() bool { return near.has(game.EffectSlow, z.now) }) {
		t.Fatal("Pele de Pedra deveria deixar os vizinhos lentos")
	}
	if v, ok := b.effect(game.EffectBuffDef, z.now); !ok || math.Abs(v-0.45) > 1e-9 {
		t.Fatalf("Pele de Pedra: defesa %+v %v", v, ok)
	}
	if got := b.def(false, z.now); math.Abs(got/def-1.45) > 0.01 {
		t.Errorf("defesa %.1f → %.1f", def, got)
	}
	if far.has(game.EffectSlow, z.now) {
		t.Error("Pele de Pedra alcançou quem está longe")
	}
}

func TestFifthTierMage(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0), fixed("skeleton_warrior", 20, 34, 0), fixed("skeleton_warrior", 20, 44, 0))
	ms := dummies(z, 32, 34, 44)
	near, near2, far := ms[0], ms[1], ms[2]
	m, _ := joinAt(z, 1, "mage", 30, 0, 0)
	atLevel(z, m, 20)
	learn(z, m, map[uint32]int{game.SkillEmber: 2, game.SkillInferno: 1, game.SkillAbsoluteZero: 1, game.SkillArcaneBlast: 1})

	// Inferno: área de 5 m em volta do alvo e uma queimadura própria, mais forte que a da
	// Brasa Viva (que não pode substituí-la).
	n := len(z.frame.combat)
	if !castUntil(z, m, game.SkillInferno, near, func() bool { return near.has(game.EffectBurn, z.now) }) {
		t.Fatal("Inferno deveria queimar")
	}
	if hit := hitsSince(z, m, n); hit[near.id] == 0 || hit[near2.id] == 0 || hit[far.id] > 0 {
		t.Errorf("Inferno: %v", hit)
	}
	want := math.Round(z.attackPower(m, true) * 0.25)
	if v, _ := near.effect(game.EffectBurn, z.now); v != want {
		t.Errorf("queimadura do Inferno %.0f, want %.0f (a Brasa Viva daria %.0f)", v, want,
			math.Round(m.atk(true, z.now)*game.Skills[game.SkillEmber].PValue.At(2)))
	}

	// Zero Absoluto: congela (atordoa) todos a até 6 m.
	if !castUntil(z, m, game.SkillAbsoluteZero, nil, func() bool { return near.stunned(z.now) && near2.stunned(z.now) }) {
		t.Fatal("Zero Absoluto deveria atordoar em volta")
	}
	if far.stunned(z.now) {
		t.Error("Zero Absoluto alcançou quem está a 14 m")
	}

	// Rajada Arcana nunca erra, nem cego.
	z.applyEffect(near, m, &game.EffectDef{Kind: game.EffectBlind, Duration: 60, Value: 0.9})
	for i := 0; i < 30; i++ {
		n := len(z.frame.combat)
		m.mp = m.stats.MaxMP
		m.pl.cooldowns = map[uint32]float64{}
		z.castSkill(m, game.Skills[game.SkillArcaneBlast], far)
		if c := z.frame.combat[n]; c.Miss || c.TargetId != far.id {
			t.Fatalf("Rajada Arcana: %+v", c)
		}
	}
}

func TestFifthTierRogue(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0), fixed("skeleton_warrior", 20, 34, 0), fixed("skeleton_warrior", 20, 44, 0))
	ms := dummies(z, 32, 34, 44)
	near, near2, far := ms[0], ms[1], ms[2]
	r, _ := joinAt(z, 1, "rogue", 31, 0, 0)
	atLevel(z, r, 20)
	learn(z, r, map[uint32]int{game.SkillFatalBlow: 1, game.SkillShadowStrike: 1, game.SkillMiasma: 2})

	// Golpe Fatal: 50% a mais abaixo de 35% de vida (golpes sem crítico).
	avg := func(frac float64) float64 {
		total, k := 0.0, 0
		for i := 0; i < 200; i++ {
			near.hp, near.dead = near.stats.MaxHP*frac, false
			n := len(z.frame.combat)
			z.skillHit(r, near, game.Skills[game.SkillFatalBlow], 1)
			if c := z.frame.combat[n]; !c.Miss && !c.Crit {
				total += float64(c.Damage)
				k++
			}
		}
		return total / float64(max(1, k))
	}
	if high, low := avg(1), avg(0.3); math.Abs(low/high-1.5) > 0.2 {
		t.Errorf("Golpe Fatal abaixo de 35%%: %.0f vs %.0f", low, high)
	}
	near.hp = near.stats.MaxHP

	// Miasma (rank 2): veneno de 35% do ataque em todos a até 6 m.
	if !castUntil(z, r, game.SkillMiasma, nil, func() bool { return near.has(game.EffectPoison, z.now) && near2.has(game.EffectPoison, z.now) }) {
		t.Fatal("Miasma deveria envenenar em volta")
	}
	if far.has(game.EffectPoison, z.now) {
		t.Error("Miasma alcançou quem está a 13 m")
	}
	if v, _ := near.effect(game.EffectPoison, z.now); v != math.Round(z.attackPower(r, false)*0.35) {
		t.Errorf("veneno do Miasma %.0f", v)
	}

	// Golpe das Sombras: salta até o alvo (a 10 m) e o cega.
	s, _ := joinAt(z, 2, "rogue", 38, 8, 0)
	atLevel(z, s, 20)
	learn(z, s, map[uint32]int{game.SkillShadowStrike: 1})
	if !castUntil(z, s, game.SkillShadowStrike, far, func() bool { return far.has(game.EffectBlind, z.now) }) {
		t.Fatal("Golpe das Sombras deveria cegar")
	}
	if d := dist(s.pos, far.pos); d > chargeStop+0.1 {
		t.Errorf("Golpe das Sombras deveria parar colado no alvo: %.2f m", d)
	}
}

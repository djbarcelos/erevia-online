package zone

import (
	"math"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// atLevel põe o jogador num nível (XP correspondente) e recalcula os atributos.
func atLevel(z *Zone, p *entity, level int) {
	p.level = level
	p.pl.xp = game.XPLevelStart(level)
	z.refreshStats(p)
	p.hp, p.mp = p.stats.MaxHP, p.stats.MaxMP
}

// learn dá ranks sem validar (para testar a mecânica de uma skill isolada).
func learn(z *Zone, p *entity, ranks map[uint32]int) {
	for id, r := range ranks {
		p.pl.ranks[id] = r
	}
	z.refreshStats(p)
}

func (f *fakePeer) lastProgress() *pb.Progress {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if pr := f.msgs[i].GetProgress(); pr != nil {
			return pr
		}
	}
	return nil
}

func (f *fakePeer) lastHotbar() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if h := f.msgs[i].GetHotbar(); h != nil {
			return h.Slots
		}
	}
	return nil
}

// ---------------------------------------------------------------------------------------
// Atributos, árvore e barra
// ---------------------------------------------------------------------------------------

func TestNewCharacterStartsWithStarterSkillsAndDefaultHotbar(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "mage", 30, 0, 0)
	if p.pl.rank(game.SkillFireball) != 1 || p.pl.rank(game.SkillIceNova) != 1 || len(p.pl.ranks) != 2 {
		t.Fatalf("starter ranks: %v", p.pl.ranks)
	}
	if p.pl.hotbar[0] != "skill:5" || p.pl.hotbar[2] != "item:potion_hp_s" || len(p.pl.hotbar) != HotbarSize {
		t.Fatalf("default hotbar: %v", p.pl.hotbar)
	}
	var w *pb.Welcome
	for _, m := range peer.msgs {
		if m.GetWelcome() != nil {
			w = m.GetWelcome()
		}
	}
	if w == nil || len(w.Skills) != 15+6 || len(w.Branches) != 3 || len(w.Specs) != len(game.SpecList) || w.Progress.FreeSkillPoints != 0 || w.Progress.FreeAttrPoints != 0 {
		t.Fatalf("welcome progression wrong")
	}
	for _, s := range w.Skills {
		if len(s.Ranks) != int(s.MaxRank) || s.Ranks[0].Text == "" {
			t.Fatalf("%s: rank table incomplete", s.Name)
		}
	}
}

func TestAllocateAttributes(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "knight", 30, 0, 0)
	atLevel(z, p, 5) // 12 pontos
	before := p.stats
	prog := z.progressPB(p)
	if prog.FreeAttrPoints != 12 {
		t.Fatalf("free points %d", prog.FreeAttrPoints)
	}
	perCon := prog.PerPoint[2]

	z.handle(progressEvent{id: p.id, msg: &pb.AllocateAttrs{Add: &pb.Attrs{Con: 4, Str: 2}}})
	if p.pl.attrs.Con != 4 || p.pl.attrs.Str != 2 {
		t.Fatalf("attrs not applied: %+v", p.pl.attrs)
	}
	if got, want := p.stats.MaxHP-before.MaxHP, float64(perCon.MaxHp)*4; math.Abs(got-want) > 0.5 {
		t.Errorf("preview per point (%.2f) does not match real change (%.2f)", want, got)
	}
	if p.stats.PAtk <= before.PAtk {
		t.Error("strength should raise attack")
	}
	if pr := peer.lastProgress(); pr == nil || pr.FreeAttrPoints != 6 || pr.Spent.Con != 4 {
		t.Fatalf("progress not sent: %+v", pr)
	}
	// Mais pontos do que tem: recusado inteiro.
	z.tickFor(1)
	z.handle(progressEvent{id: p.id, msg: &pb.AllocateAttrs{Add: &pb.Attrs{Dex: 7}}})
	if p.pl.attrs.Dex != 0 {
		t.Error("over-allocation accepted")
	}
}

func TestLearnSkillsRules(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "knight", 30, 0, 0)
	atLevel(z, p, 10) // 11 pontos, 2 gastos
	z.handle(progressEvent{id: p.id, msg: &pb.LearnSkills{Ranks: []*pb.SkillRank{
		{SkillId: game.SkillIronSkin, Rank: 2}, {SkillId: game.SkillTaunt, Rank: 1},
	}}})
	if p.pl.rank(game.SkillIronSkin) != 2 || p.pl.rank(game.SkillTaunt) != 1 {
		t.Fatalf("ranks not learned: %v", p.pl.ranks)
	}
	z.tickFor(1)
	// Degrau 3 exige nível 15.
	z.handle(progressEvent{id: p.id, msg: &pb.LearnSkills{Ranks: []*pb.SkillRank{{SkillId: game.SkillLivingWall, Rank: 1}}}})
	if p.pl.rank(game.SkillLivingWall) != 0 {
		t.Error("tier 3 learned at level 10")
	}
	z.tickFor(1)
	// Não desaprende.
	z.handle(progressEvent{id: p.id, msg: &pb.LearnSkills{Ranks: []*pb.SkillRank{{SkillId: game.SkillIronSkin, Rank: 1}}}})
	if p.pl.rank(game.SkillIronSkin) != 2 {
		t.Error("unlearned a rank")
	}
	z.tickFor(1)
	// Sem pontos (11 no total, 5 gastos: tentar +7).
	z.handle(progressEvent{id: p.id, msg: &pb.LearnSkills{Ranks: []*pb.SkillRank{{SkillId: game.SkillFencing, Rank: 5}, {SkillId: game.SkillWhirl, Rank: 2}}}})
	if p.pl.rank(game.SkillFencing) != 0 {
		t.Error("learned without points")
	}
}

func TestPassiveChangesStats(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "knight", 30, 0, 0)
	def0 := p.stats.PDef
	learn(z, p, map[uint32]int{game.SkillIronSkin: 5})
	if math.Abs(p.stats.PDef-def0*1.2) > 0.01 {
		t.Fatalf("iron skin 5 should give +20%% defense: %.2f → %.2f", def0, p.stats.PDef)
	}
	// Esgrima só com espada (o kit do cavaleiro não está equipado neste teste: sem efeito).
	atk0 := p.stats.PAtk
	learn(z, p, map[uint32]int{game.SkillFencing: 3})
	if p.stats.PAtk != atk0 {
		t.Error("fencing without a sword should do nothing")
	}
}

func TestUnlearnedSkillCannotBeUsed(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 3, 31, 0))
	p, peer := joinAt(z, 1, "knight", 30, 0, 0)
	atLevel(z, p, 20)
	z.handle(skillEvent{id: p.id, skill: game.SkillWhirl})
	if p.pl.cooldowns[game.SkillWhirl] > 0 || !peer.systemContains("ainda não aprendeu") {
		t.Fatal("used an unlearned skill")
	}
	// Passiva nunca se usa.
	learn(z, p, map[uint32]int{game.SkillIronSkin: 1})
	z.tickFor(0.2)
	z.handle(skillEvent{id: p.id, skill: game.SkillIronSkin})
	if p.pl.cooldowns[game.SkillIronSkin] > 0 {
		t.Fatal("passive skill was used")
	}
}

func TestHotbarValidation(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "rogue", 30, 0, 0)
	ok := []string{"skill:7", "skill:8", "item:potion_hp_l", "", "", "", "", "", "", "item:potion_mp_s"}
	z.handle(progressEvent{id: p.id, msg: &pb.SetHotbar{Slots: ok}})
	if p.pl.hotbar[2] != "item:potion_hp_l" || p.pl.hotbar[9] != "item:potion_mp_s" {
		t.Fatalf("valid hotbar rejected: %v", p.pl.hotbar)
	}
	for _, bad := range [][]string{
		{"skill:5"},                  // de outra classe
		{"skill:41"},                 // não aprendida
		{"item:bone_old"},            // não é consumível
		{"skill:abc"},                // lixo
		make([]string, HotbarSize+1), // grande demais
	} {
		z.tickFor(0.5)
		z.handle(progressEvent{id: p.id, msg: &pb.SetHotbar{Slots: bad}})
		if p.pl.hotbar[0] != "skill:7" {
			t.Fatalf("invalid hotbar %v accepted: %v", bad, p.pl.hotbar)
		}
	}
	if peer.lastHotbar() == nil {
		t.Fatal("server should answer with the hotbar")
	}
}

func TestProgressSurvivesRelogin(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "barbarian", 30, 0, 0)
	atLevel(z, p, 6)
	z.handle(progressEvent{id: p.id, msg: &pb.AllocateAttrs{Add: &pb.Attrs{Str: 3}}})
	z.tickFor(0.5)
	z.handle(progressEvent{id: p.id, msg: &pb.LearnSkills{Ranks: []*pb.SkillRank{{SkillId: game.SkillHeavyAxe, Rank: 2}}}})
	z.tickFor(0.5)
	z.handle(progressEvent{id: p.id, msg: &pb.SetHotbar{Slots: []string{"skill:4", "skill:3"}}})
	st := z.savedState(p)
	if st.Attrs.Str != 3 || st.Ranks[game.SkillHeavyAxe] != 2 || st.Hotbar[0] != "skill:4" {
		t.Fatalf("saved state misses progress: %+v", st)
	}
	z.handle(leaveEvent{id: p.id, peer: p.pl.peer})
	p2, _ := joinAt(z, 1, "barbarian", 30, 0, game.XPLevelStart(6))
	if p2.pl.attrs.Str != 3 || p2.pl.rank(game.SkillHeavyAxe) != 2 || p2.pl.hotbar[0] != "skill:4" {
		t.Fatalf("progress lost on relogin: %+v %v %v", p2.pl.attrs, p2.pl.ranks, p2.pl.hotbar)
	}
	if p2.stats.CritDamage <= game.CritMult {
		t.Error("heavy axe passive not applied after relogin")
	}
}

func TestLoadProgressRepairsBadData(t *testing.T) {
	attrs, ranks, bar := loadProgress("knight", "", 1, game.Attrs{Str: 99}, map[uint32]int{game.SkillJudgment: 1, game.SkillFireball: 3}, nil)
	if attrs.Sum() != 0 {
		t.Error("too many attribute points kept")
	}
	if ranks[game.SkillJudgment] != 0 || ranks[game.SkillFireball] != 0 || ranks[game.SkillShieldBash] != 1 {
		t.Errorf("bad ranks kept: %v", ranks)
	}
	if bar[0] == "" {
		t.Error("empty hotbar should get the default")
	}
}

func TestCastSpeedAndOverload(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "mage", 30, 0, 0)
	atLevel(z, p, 20)
	s := game.Skills[game.SkillFireball]
	cd0 := skillCD(p, s)
	p.pl.attrs.Wis = 20
	z.refreshStats(p)
	if skillCD(p, s) >= cd0 {
		t.Error("wisdom should shorten cooldowns")
	}
	mp0 := skillMP(p, s)
	learn(z, p, map[uint32]int{game.SkillOverload: 1})
	if skillMP(p, s) <= mp0 {
		t.Error("overload should raise mana cost")
	}
}

// ---------------------------------------------------------------------------------------
// Mecânicas
// ---------------------------------------------------------------------------------------

func TestRootStopsMovementButNotAttacks(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 3, 34, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "mage", 30, 0, 0)
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectRoot, Duration: 4})
	start := m.pos
	m.mon.hate[p.id] = 10
	z.tickFor(2)
	if dist(start, m.pos) > 0.01 {
		t.Fatal("rooted monster moved")
	}
	z.tickFor(3)
	if dist(start, m.pos) < 0.5 {
		t.Fatal("monster should move after root ends")
	}
}

func TestTauntForcesTarget(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 5, 33, 0))
	m := z.onlyMonster()
	tank, _ := joinAt(z, 1, "knight", 31, 0, 0)
	other, _ := joinAt(z, 2, "mage", 35, 0, 0)
	atLevel(z, tank, 10)
	learn(z, tank, map[uint32]int{game.SkillIronSkin: 1, game.SkillTaunt: 1})
	m.mon.hate[other.id] = 500
	z.castSkill(tank, game.Skills[game.SkillTaunt], nil)
	if !m.has(game.EffectTaunt, z.now) {
		t.Fatal("monster not taunted")
	}
	z.tickFor(0.2)
	if m.act.target != tank.id {
		t.Fatalf("taunted monster targets %d, want the knight", m.act.target)
	}
}

func TestLivingWallShieldAbsorbs(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 3, 34, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 30, 0, 0)
	atLevel(z, p, 15)
	learn(z, p, map[uint32]int{game.SkillLivingWall: 1})
	z.damage(m, p, p.stats.MaxHP*0.75, nil)
	sh := p.effectRef(game.EffectShield, z.now)
	if sh == nil {
		t.Fatal("living wall did not trigger below 30%")
	}
	hp := p.hp
	ev := &pb.CombatEvent{}
	z.damage(m, p, 10, ev)
	if p.hp != hp || ev.Absorbed != 10 {
		t.Fatalf("shield should absorb: hp %.0f→%.0f absorbed %d", hp, p.hp, ev.Absorbed)
	}
	// Não dispara de novo antes de 60 s.
	p.removeEffect(game.EffectShield)
	z.damage(m, p, 1, nil)
	if p.has(game.EffectShield, z.now) {
		t.Fatal("living wall retriggered inside its cooldown")
	}
}

func TestManaShieldAndUndying(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 3, 34, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "mage", 30, 0, 0)
	z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectManaShield, Duration: 8, Value: 1})
	hp, mp := p.hp, p.mp
	z.damage(m, p, 20, nil)
	if p.hp != hp || math.Abs(p.mp-(mp-20)) > 0.01 {
		t.Fatalf("mana shield: hp %.0f→%.0f mp %.0f→%.0f", hp, p.hp, mp, p.mp)
	}
	b, _ := joinAt(z, 2, "barbarian", 30, 2, 0)
	z.applyEffect(b, b, &game.EffectDef{Kind: game.EffectUndying, Duration: 5})
	z.damage(m, b, b.hp*10, nil)
	if b.dead || b.hp < 1 {
		t.Fatalf("undying died: hp %.1f", b.hp)
	}
}

func TestInvisibilityDropsAggroAndAmbushCrits(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 3, 32, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "rogue", 30, 0, 0)
	atLevel(z, p, 15)
	learn(z, p, map[uint32]int{game.SkillVanish: 1})
	m.mon.hate[p.id] = 50
	m.act = action{kind: actAttack, target: p.id}
	z.castSkill(p, game.Skills[game.SkillVanish], nil)
	if !p.invisible(z.now) || len(m.mon.hate) != 0 || m.act.kind != actNone {
		t.Fatal("monster should lose the invisible rogue")
	}
	z.tickFor(1)
	if m.act.target == p.id || len(m.mon.hate) != 0 {
		t.Fatal("aggressive monster re-acquired an invisible player")
	}
	// Atacar revela, e o golpe é crítico.
	z.basicAttack(p, m)
	if p.invisible(z.now) {
		t.Fatal("attacking should reveal")
	}
	evs := z.frame.combat
	if last := evs[len(evs)-1]; !last.Crit || last.Miss {
		t.Fatalf("ambush hit should crit: %+v", last)
	}
}

func TestMarkWeakenBlindAndFrenzy(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 5, 34, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "rogue", 30, 0, 0)
	// Marca: +25% de dano.
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectMark, Duration: 10, Value: 0.25})
	hp := m.hp
	ev := &pb.CombatEvent{}
	z.applyHit(p, m, 0, game.Hit{Damage: 100}, false)
	ev = z.frame.combat[len(z.frame.combat)-1]
	if ev.Damage != 125 || math.Abs((hp-m.hp)-125) > 0.01 {
		t.Fatalf("mark should add 25%%: %d (hp %.0f→%.0f)", ev.Damage, hp, m.hp)
	}
	// Enfraquecer: -20% de ataque.
	atk := m.atk(false, z.now)
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectWeaken, Duration: 8, Value: 0.2})
	if math.Abs(m.atk(false, z.now)-atk*0.8) > 0.01 {
		t.Error("weaken should cut attack")
	}
	// Cegueira: precisão menor.
	acc := m.accuracy(z.now)
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectBlind, Duration: 6, Value: 0.3})
	if m.accuracy(z.now) >= acc {
		t.Error("blind should lower accuracy")
	}
	// Frenesi: ataca mais rápido.
	iv := p.attackInterval(z.now)
	z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectBuffAtkSpeed, Duration: 12, Value: 0.25})
	if math.Abs(p.attackInterval(z.now)-iv/1.25) > 1e-9 {
		t.Error("frenzy should speed up attacks")
	}
}

func TestFrostArmorSlowsMeleeAttackers(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 3, 31, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "mage", 30, 0, 0)
	z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectFrostArmor, Duration: 20, Value: 0.3})
	z.applyHit(m, p, 0, game.Hit{Damage: 5}, false)
	if !m.has(game.EffectSlow, z.now) {
		t.Fatal("melee attacker should be slowed by frost armor")
	}
}

func TestPoisonTicksAndGivesKillCredit(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 1, 34, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "rogue", 30, 0, 0)
	m.hp = 25
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectPoison, Duration: 5, Value: 10})
	z.tickFor(3.2)
	if !m.dead {
		t.Fatalf("poison should kill (hp %.0f)", m.hp)
	}
	if p.pl.xp == 0 {
		t.Fatal("poisoner should get the XP")
	}
	dots := 0
	for _, c := range peer.combatFrom(p.id) {
		if c.Dot == pb.Effect_EFFECT_POISON {
			dots++
		}
	}
	if dots < 2 {
		t.Fatalf("poison tick events: %d", dots)
	}
}

func TestEmberBurnsAndPlagueSpreads(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 3, 34, 0), fixed("skeleton_minion", 3, 36, 0), fixed("skeleton_minion", 3, 38, 1))
	var ms []*entity
	for _, m := range z.monsters {
		ms = append(ms, m)
	}
	for _, m := range ms { // alvos de treino: não morrem com um golpe
		m.stats.MaxHP, m.hp = 1e6, 1e6
	}
	mage, _ := joinAt(z, 1, "mage", 30, 0, 0)
	atLevel(z, mage, 10)
	learn(z, mage, map[uint32]int{game.SkillEmber: 2})
	for i := 0; i < 10 && !ms[0].has(game.EffectBurn, z.now); i++ {
		mage.mp = mage.stats.MaxMP
		mage.pl.cooldowns = map[uint32]float64{}
		z.castSkill(mage, game.Skills[game.SkillFireball], ms[0])
	}
	if !ms[0].has(game.EffectBurn, z.now) {
		t.Fatal("fire skill should burn with ember")
	}
	rogue, _ := joinAt(z, 2, "rogue", 30, 3, 0)
	atLevel(z, rogue, 20)
	learn(z, rogue, map[uint32]int{game.SkillPlague: 1})
	z.castSkill(rogue, game.Skills[game.SkillPlague], ms[1])
	if !ms[1].has(game.EffectPlague, z.now) {
		t.Fatal("plague not applied")
	}
	z.damage(rogue, ms[1], ms[1].hp+10, nil)
	spread := 0
	for _, m := range ms {
		if m != ms[1] && m.has(game.EffectPlague, z.now) {
			spread++
		}
	}
	if spread == 0 {
		t.Fatal("plague should jump to neighbours when the host dies")
	}
}

func TestWarCryBannerAndCourageHelpAllies(t *testing.T) {
	z := newCombatZone()
	k, _ := joinAt(z, 1, "knight", 30, 0, 0)
	ally, _ := joinAt(z, 2, "mage", 33, 0, 0)
	far, _ := joinAt(z, 3, "rogue", 60, 0, 0)
	stranger, _ := joinAt(z, 4, "rogue", 31, 0, 0) // perto, mas fora do grupo
	makeParty(z, k, ally, far)
	atLevel(z, k, 20)
	learn(z, k, map[uint32]int{game.SkillWarCry: 1, game.SkillCourage: 2, game.SkillBanner: 1})
	z.castSkill(k, game.Skills[game.SkillWarCry], nil)
	if !k.has(game.EffectWarCry, z.now) || !ally.has(game.EffectWarCry, z.now) || far.has(game.EffectWarCry, z.now) {
		t.Fatal("war cry should reach allies within 10 m only")
	}
	if stranger.has(game.EffectWarCry, z.now) {
		t.Fatal("war cry should not reach players outside the party or clan")
	}
	z.castSkill(k, game.Skills[game.SkillBanner], nil)
	if len(z.objects) != 1 {
		t.Fatal("banner object not spawned")
	}
	z.tickFor(1)
	if !ally.has(game.EffectBanner, z.now) || far.has(game.EffectBanner, z.now) {
		t.Fatal("banner should defend allies nearby")
	}
	if !ally.has(game.EffectCourage, z.now) {
		t.Fatal("courage aura should reach allies")
	}
	if stranger.has(game.EffectBanner, z.now) || stranger.has(game.EffectCourage, z.now) {
		t.Fatal("banner and courage should skip strangers")
	}
	ally.hp = 1
	ally.lastCombat = z.now // em combate: só a regeneração base + coragem
	hp := ally.hp
	z.tickFor(1)
	if ally.hp-hp < 4 {
		t.Fatalf("courage should add regen: +%.1f", ally.hp-hp)
	}
	z.tickFor(20)
	if len(z.objects) != 0 {
		t.Fatal("banner should vanish after 20 s")
	}
	z.tickFor(2)
	if ally.has(game.EffectBanner, z.now) {
		t.Fatal("banner buff should end with the banner")
	}
}

func TestBlinkAndCharge(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 3, 40, 0))
	m := z.onlyMonster()
	mage, _ := joinAt(z, 1, "mage", 25, 10, 0)
	atLevel(z, mage, 10)
	learn(z, mage, map[uint32]int{game.SkillClearMind: 1, game.SkillArcaneShield: 1, game.SkillBlink: 1})
	mage.face = Vec2{X: 1}
	start := mage.pos
	z.castSkill(mage, game.Skills[game.SkillBlink], nil)
	if d := mage.pos.X - start.X; math.Abs(d-8) > 0.6 || !mage.teleport {
		t.Fatalf("blink moved %.2f m", d)
	}
	k, _ := joinAt(z, 2, "knight", 30, 0, 0)
	atLevel(z, k, 10)
	learn(z, k, map[uint32]int{game.SkillFencing: 1, game.SkillWhirl: 1, game.SkillCharge: 1})
	z.castSkill(k, game.Skills[game.SkillCharge], m)
	if d := dist(k.pos, m.pos); d > chargeStop+0.1 {
		t.Fatalf("charge should land next to the target, %.2f m away", d)
	}
}

func TestExecuteTwinBladesJudgmentLifestealSplashParalysis(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 10, 32, 0), fixed("skeleton_warrior", 10, 34, 0))
	var a, b *entity
	for _, m := range z.monsters {
		if a == nil {
			a = m
		} else {
			b = m
		}
	}
	barb, _ := joinAt(z, 1, "barbarian", 31, 0, 0)
	atLevel(z, barb, 20)
	learn(z, barb, map[uint32]int{game.SkillExecute: 1, game.SkillBloodthirst: 5})

	// Alvos de treino: muita vida, para nada morrer no meio do teste.
	for _, m := range []*entity{a, b} {
		m.stats.MaxHP, m.hp = 1e6, 1e6
	}
	// Execução: o dobro abaixo de 30% (média do dano por golpe nos eventos).
	avg := func(frac float64) float64 {
		total, n := 0.0, 0
		for i := 0; i < 60; i++ {
			a.hp, a.dead = a.stats.MaxHP*frac, false
			k := len(z.frame.combat)
			z.skillHit(barb, a, game.Skills[game.SkillExecute], 1)
			if c := z.frame.combat[k]; !c.Miss && !c.Crit {
				total += float64(c.Damage)
				n++
			}
		}
		return total / float64(max(1, n))
	}
	high, low := avg(1), avg(0.25)
	if math.Abs(low/high-2) > 0.25 {
		t.Errorf("execute should double below 30%%: %.0f vs %.0f", low, high)
	}
	a.hp, a.dead = a.stats.MaxHP, false

	// Roubo de vida.
	barb.hp = barb.stats.MaxHP / 2
	hp := barb.hp
	for i := 0; i < 5; i++ {
		z.basicAttack(barb, a)
	}
	if barb.hp <= hp {
		t.Error("bloodthirst should heal")
	}

	// Lâminas Gêmeas: 2 golpes.
	r, _ := joinAt(z, 2, "rogue", 31, 1, 0)
	atLevel(z, r, 20)
	n := len(z.frame.combat)
	z.skillHit(r, a, game.Skills[game.SkillTwinBlades], 1)
	if len(z.frame.combat)-n != 2 {
		t.Errorf("twin blades hits: %d", len(z.frame.combat)-n)
	}
	// Paralisia só atordoa envenenado.
	b.removeEffect(game.EffectStun)
	for i := 0; i < 5; i++ {
		z.skillHit(r, b, game.Skills[game.SkillParalysis], 1)
	}
	if b.stunned(z.now) {
		t.Error("paralysis stunned a target without poison")
	}
	z.applyEffect(r, b, &game.EffectDef{Kind: game.EffectPoison, Duration: 5, Value: 1})
	for i := 0; i < 10 && !b.stunned(z.now); i++ {
		z.skillHit(r, b, game.Skills[game.SkillParalysis], 1)
	}
	if !b.stunned(z.now) {
		t.Error("paralysis should stun a poisoned target")
	}

	// Julgamento nunca erra (mesmo cego).
	k, _ := joinAt(z, 3, "knight", 31, 2, 0)
	atLevel(z, k, 20)
	z.applyEffect(a, k, &game.EffectDef{Kind: game.EffectBlind, Duration: 60, Value: 0.9})
	for i := 0; i < 30; i++ {
		a.hp, a.dead = a.stats.MaxHP, false
		n := len(z.frame.combat)
		z.skillHit(k, a, game.Skills[game.SkillJudgment], 1)
		if z.frame.combat[n].Miss {
			t.Fatal("judgment missed")
		}
	}

	// Chuva de Brasas acerta em volta do alvo.
	mage, _ := joinAt(z, 4, "mage", 25, 0, 0)
	atLevel(z, mage, 20)
	learn(z, mage, map[uint32]int{game.SkillEmberRain: 1})
	a.hp, b.hp = a.stats.MaxHP, b.stats.MaxHP
	n = len(z.frame.combat)
	z.castSkill(mage, game.Skills[game.SkillEmberRain], a)
	hit := map[uint32]bool{}
	for _, c := range z.frame.combat[n:] {
		hit[c.TargetId] = true
	}
	if !hit[a.id] || !hit[b.id] {
		t.Errorf("ember rain should splash: %v", hit)
	}
}

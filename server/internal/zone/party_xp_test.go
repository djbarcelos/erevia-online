package zone

import (
	"math"
	"testing"

	"erevia/server/internal/game"
)

// XP em grupo e diferença de nível (tunnel#151). As regras estão em game/party.go e no fim
// de party.go.

// gains devolve quanto de XP cada um ganhou desde a chamada.
func gains(ps ...*entity) func() []int64 {
	before := make([]int64, len(ps))
	for i, p := range ps {
		before[i] = p.pl.xp
	}
	return func() []int64 {
		out := make([]int64, len(ps))
		for i, p := range ps {
			out[i] = p.pl.xp - before[i]
		}
		return out
	}
}

// share: a parte de quem tem o nível lvl num abate com XP total total e níveis somados sum.
func share(total float64, lvl, sum int) int64 {
	return int64(math.Round(total * float64(lvl) / float64(sum)))
}

func TestPartyXPNumbers(t *testing.T) {
	for n, want := range map[int]float64{0: 1, 1: 1, 2: 1.1, 3: 1.2, 4: 1.3, 5: 1.4, 6: 1.5, 7: 1.5} {
		if got := game.PartyXPBonus(n); math.Abs(got-want) > 1e-9 {
			t.Errorf("PartyXPBonus(%d) = %v, quero %v", n, got, want)
		}
	}
	for gap, want := range map[int]float64{0: 1, 5: 1, 8: 1, 9: 0.5, 10: 0.4, 11: 0.3, 12: 0.2, 13: 0.1, 20: 0.1, 39: 0.1} {
		if got := game.PartyGapMult(gap); math.Abs(got-want) > 1e-9 {
			t.Errorf("PartyGapMult(%d) = %v, quero %v", gap, got, want)
		}
	}
}

// Bônus: +10% por membro que conta além do primeiro, dividido pelo nível de cada um.
func TestPartyXPBonusSplitByLevel(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 8, 40, 0))
	m := z.onlyMonster()
	a, _ := named(z, 1, "Ana", "knight", 38, 0)
	b, _ := named(z, 2, "Beto", "mage", 39, 1)
	c, _ := named(z, 3, "Caio", "rogue", 37, 0)
	atLevel(z, a, 8)
	atLevel(z, b, 6)
	atLevel(z, c, 8)
	makeParty(z, a, b, c)
	z.markHelp(b)
	z.markHelp(c)
	got := gains(a, b, c)
	z.damage(a, m, m.hp+10, nil)

	total := float64(game.MonsterXP(m.mon.def, 8, 8)) * 1.2
	want := []int64{share(total, 8, 22), share(total, 6, 22), share(total, 8, 22)}
	for i, g := range got() {
		if g != want[i] {
			t.Errorf("membro %d: +%d XP, quero %d (%v)", i, g, want[i], got())
		}
	}
}

// Distância: conta quem está a até 40 m do monstro.
func TestPartyXPDistance(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 8, 70, 0))
	m := z.onlyMonster()
	a, _ := named(z, 1, "Ana", "knight", 68, 0)
	near, _ := named(z, 2, "Perto", "mage", 31, 0) // 39 m
	far, _ := named(z, 3, "Longe", "rogue", 29, 0) // 41 m
	for _, p := range []*entity{a, near, far} {
		atLevel(z, p, 8)
		z.markHelp(p)
	}
	makeParty(z, a, near, far)
	got := gains(a, near, far)
	z.damage(a, m, m.hp+10, nil)

	total := float64(game.MonsterXP(m.mon.def, 8, 8)) * 1.1
	if g := got(); g[0] != share(total, 8, 16) || g[1] != share(total, 8, 16) || g[2] != 0 {
		t.Fatalf("XP %v: quem está a 39 m conta, a 41 m não", g)
	}
}

// Ajuda: só conta quem deu dano, curou ou buffou nos últimos 30 s. Quem ficou parado não
// leva nada nem aumenta o bônus; quem buffou há mais de 30 s também não.
func TestPartyXPNeedsHelp(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 20, 40, 0), fixed("skeleton_minion", 20, 50, 0))
	var m, other *entity
	for _, e := range z.monsters {
		if e.mon.home.X < 45 {
			m = e
		} else {
			other = e
		}
	}
	a, _ := named(z, 1, "Ana", "mage", 36, 0)
	idle, _ := named(z, 2, "Parado", "rogue", 37, 2)
	k, _ := named(z, 3, "Kael", "knight", 38, 1)
	old, _ := named(z, 4, "Antigo", "rogue", 37, -2)
	for _, p := range []*entity{a, idle, k, old} {
		atLevel(z, p, 20)
	}
	makeParty(z, a, idle, k, old)

	z.markHelp(old)
	z.tickFor(game.PartyHelpSeconds + 1) // a ajuda dele venceu
	// Kael buffa o grupo com o Grito de Guerra (alcança os aliados a até 10 m).
	learn(z, k, map[uint32]int{game.SkillWarCry: 1})
	z.castSkill(k, game.Skills[game.SkillWarCry], nil)
	if !a.has(game.EffectWarCry, z.now) {
		t.Fatal("o Grito de Guerra deveria alcançar a Ana")
	}
	got := gains(a, idle, k, old)
	z.damage(a, m, m.hp+10, nil)

	total := float64(game.MonsterXP(m.mon.def, 20, 20)) * 1.1
	if g := got(); g[0] != share(total, 20, 40) || g[1] != 0 || g[2] != share(total, 20, 40) || g[3] != 0 {
		t.Fatalf("XP %v: contam quem bateu e quem buffou agora; quem ficou parado ou ajudou há 31 s, não", g)
	}
	if n := idle.pl.bst.kills["skeleton_minion"].N; n != 0 {
		t.Errorf("quem não ajudou não conta o abate no bestiário (%d)", n)
	}
	if n := k.pl.bst.kills["skeleton_minion"].N; n != 1 {
		t.Errorf("quem buffou conta o abate no bestiário (%d)", n)
	}

	// Dano em qualquer monstro é ajuda (até 30 s depois).
	if z.now < idle.pl.helpUntil {
		t.Fatal("quem só recebeu buff não ajudou")
	}
	z.damage(idle, other, 1, nil)
	if z.now >= idle.pl.helpUntil {
		t.Fatal("dano em monstro conta como ajuda")
	}
}

// Diferença de nível: até 8 entre o maior e o menor dos que contam, nada; com 9, a XP do
// grupo inteiro cai pela metade (e mais 10 pontos por nível, até 10%).
func TestPartyXPLevelGap(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 12, 40, 0), fixed("skeleton_warrior", 12, 40, 6), fixed("skeleton_warrior", 12, 40, -6))
	ms := []*entity{}
	for _, m := range z.monsters {
		ms = append(ms, m)
	}
	hi, _ := named(z, 1, "Alto", "knight", 38, 2)
	lo, loPeer := named(z, 2, "Baixo", "mage", 39, 2)
	atLevel(z, hi, 16)
	atLevel(z, lo, 8) // 8 abaixo: sem corte
	makeParty(z, hi, lo)
	def := ms[0].mon.def

	z.markHelp(lo)
	got := gains(hi, lo)
	z.damage(hi, ms[0], ms[0].hp+10, nil)
	total := float64(game.MonsterXP(def, 12, 16)) * 1.1
	if g := got(); g[0] != share(total, 16, 24) || g[1] != share(total, 8, 24) {
		t.Fatalf("diferença 8: XP %v, quero %d/%d", g, share(total, 16, 24), share(total, 8, 24))
	}

	atLevel(z, hi, 17) // 9 de diferença: metade
	atLevel(z, lo, 8)
	z.markHelp(lo)
	got = gains(hi, lo)
	z.damage(hi, ms[1], ms[1].hp+10, nil)
	total = float64(game.MonsterXP(def, 12, 17)) * 1.1 * 0.5
	if g := got(); g[0] != share(total, 17, 25) || g[1] != share(total, 8, 25) {
		t.Fatalf("diferença 9: XP %v, quero %d/%d", g, share(total, 17, 25), share(total, 8, 25))
	}
	// A janela do grupo avisa o corte.
	z.syncParties()
	if st := loPeer.lastParty(); st.XpNear != 2 || st.XpBonusPct != 10 || st.XpLevelGap != 9 || st.XpGapCutPct != 50 {
		t.Fatalf("janela do grupo: perto %d, bônus %d%%, diferença %d, corte %d%%", st.XpNear, st.XpBonusPct, st.XpLevelGap, st.XpGapCutPct)
	}

	// Quem não ajudou não conta, então também não corta a XP de quem caçou.
	atLevel(z, hi, 17)
	atLevel(z, lo, 8)
	lo.pl.helpUntil = 0
	got = gains(hi, lo)
	z.damage(hi, ms[2], ms[2].hp+10, nil)
	if g := got(); g[0] != game.MonsterXP(def, 12, 17) || g[1] != 0 {
		t.Fatalf("baixo parado: XP %v, quero a XP cheia do alto e nada do baixo", g)
	}
}

// Sem grupo: o abate é de quem causou mais dano, não de quem deu o último golpe (XP,
// saque reservado e bestiário).
func TestKillCreditTopDamageNotLastHit(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 8, 40, 0))
	m := z.onlyMonster()
	solo, _ := named(z, 1, "Solo", "rogue", 39, 0)
	thief, _ := named(z, 2, "Ladrao", "rogue", 39, 1)
	atLevel(z, solo, 8)
	atLevel(z, thief, 8)
	z.damage(solo, m, m.hp*0.7, nil)
	z.damage(thief, m, 1, nil) // o ódio dele ficaria maior com provocação; o dano, não
	m.mon.hate[thief.id] = 1e6
	got := gains(solo, thief)
	z.damage(thief, m, m.hp+10, nil)
	if g := got(); g[0] != game.MonsterXP(m.mon.def, 8, 8) || g[1] != 0 {
		t.Fatalf("XP %v: vai para quem causou mais dano", g)
	}
	if solo.pl.bst.kills["skeleton_warrior"].N != 1 || thief.pl.bst.kills["skeleton_warrior"].N != 0 {
		t.Fatal("o bestiário segue quem levou a XP")
	}
	for _, g := range z.ground {
		if g.owner != solo.id {
			t.Fatalf("saque reservado para %d, quero quem causou mais dano (%d)", g.owner, solo.id)
		}
	}
	if len(m.mon.dmg) != 0 || len(m.mon.hate) != 0 {
		t.Fatal("o registro de dano é limpo na morte")
	}
}

// Grupo contra sozinho: o dano somado do grupo conta como um lado só.
func TestKillCreditPartyVsSolo(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 8, 40, 0), fixed("skeleton_warrior", 8, 40, 6))
	ms := []*entity{}
	for _, m := range z.monsters {
		ms = append(ms, m)
	}
	p1, _ := named(z, 1, "Um", "knight", 38, 2)
	p2, _ := named(z, 2, "Dois", "mage", 39, 3)
	solo, _ := named(z, 3, "Solo", "rogue", 39, 4)
	for _, p := range []*entity{p1, p2, solo} {
		atLevel(z, p, 8)
	}
	makeParty(z, p1, p2)

	// 35% + 25% do grupo contra 40% do solo: o grupo leva, mesmo sem o maior dano sozinho.
	m := ms[0]
	hp := m.hp
	z.damage(p1, m, hp*0.35, nil)
	z.damage(p2, m, hp*0.25, nil)
	got := gains(p1, p2, solo)
	z.damage(solo, m, hp, nil)
	total := float64(game.MonsterXP(m.mon.def, 8, 8)) * 1.1
	if g := got(); g[0] != share(total, 8, 16) || g[1] != share(total, 8, 16) || g[2] != 0 {
		t.Fatalf("XP %v: o grupo somou mais dano", g)
	}
	for _, g := range z.ground {
		if g.owner != p1.id {
			t.Fatalf("saque reservado para %d, quero quem do grupo mais bateu (%d)", g.owner, p1.id)
		}
	}

	// 20% + 10% do grupo contra 70% do solo: o solo leva.
	m = ms[1]
	hp = m.hp
	z.damage(p1, m, hp*0.2, nil)
	z.damage(p2, m, hp*0.1, nil)
	got = gains(p1, p2, solo)
	z.damage(solo, m, hp, nil)
	if g := got(); g[0] != 0 || g[1] != 0 || g[2] != game.MonsterXP(m.mon.def, 8, 8) {
		t.Fatalf("XP %v: o solo causou mais dano que o grupo somado", g)
	}
}

// Cada membro aplica os próprios extras na parte dele: XP em dobro para todos, vitalidade
// só para quem tem.
func TestPartyXPEachMemberOwnBonuses(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 8, 40, 0))
	m := z.onlyMonster()
	a, _ := named(z, 1, "Ana", "knight", 38, 0)
	b, _ := named(z, 2, "Beto", "mage", 39, 1)
	atLevel(z, a, 8)
	atLevel(z, b, 8)
	makeParty(z, a, b)
	z.markHelp(b)
	z.xp2.on = true
	a.pl.vit.stored = 1_000_000
	got := gains(a, b)
	z.damage(a, m, m.hp+10, nil)

	part := share(float64(game.MonsterXP(m.mon.def, 8, 8))*xp2Mult*1.1, 8, 16)
	_, vit := game.MonsterXPExtras(part, xp2Mult, false, 1_000_000)
	if g := got(); vit <= 0 || g[0] != part+vit || g[1] != part {
		t.Fatalf("XP %v: quero %d (com vitalidade %d) e %d", g, part+vit, vit, part)
	}
}

// O registro de dano não passa de uma luta para a outra: o monstro que desiste esquece.
func TestKillCreditForgetsOnReset(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 8, 40, 0))
	m := z.onlyMonster()
	a, _ := named(z, 1, "Ana", "rogue", 39, 0)
	z.damage(a, m, 5, nil)
	if m.mon.dmg[a.id] != 5 {
		t.Fatalf("dano registrado %v", m.mon.dmg)
	}
	z.resetMonster(m)
	if m.mon.dmg != nil || len(m.mon.hate) != 0 {
		t.Fatal("o monstro que desiste esquece o dano")
	}
}

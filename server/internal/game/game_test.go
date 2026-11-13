package game

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestClassStatsGrowWithLevel(t *testing.T) {
	for id := range Classes {
		l1, l10 := ClassStats(id, 1), ClassStats(id, 10)
		if l10.MaxHP <= l1.MaxHP || l10.PDef <= l1.PDef {
			t.Errorf("%s should grow: %+v → %+v", id, l1, l10)
		}
	}
	if ClassStats("mage", 1).Range <= ClassStats("knight", 1).Range {
		t.Error("mage should have longer range")
	}
	if !ClassStats("mage", 1).Magic {
		t.Error("mage basic attack should be magic")
	}
}

func TestEveryClassHasTwoStarterSkills(t *testing.T) {
	for id := range Classes {
		s := ClassSkills(id)
		if len(s) != 2 || s[0] == nil || s[1] == nil || s[0].Class != id || s[0].Tier != 0 || s[1].Tier != 0 || s[0].IsPassive() || s[1].IsPassive() {
			t.Errorf("%s starter skills wrong: %+v", id, s)
		}
	}
}

func TestSkillTreeShape(t *testing.T) {
	for id, c := range Classes {
		tree := ClassTree(id)
		if len(tree) != 12 {
			t.Fatalf("%s: %d skills in the tree, want 12", id, len(tree))
		}
		seen := map[[2]int]bool{}
		for _, s := range tree {
			key := [2]int{s.Branch, s.Tier}
			if seen[key] {
				t.Errorf("%s: two skills at branch %d tier %d", id, s.Branch, s.Tier)
			}
			seen[key] = true
			if s.MaxRank < 1 || s.Text == nil || s.Text(1) == "" || s.FX == "" {
				t.Errorf("%s: incomplete", s.Name)
			}
			if s.ID >= 1000 {
				t.Errorf("%s: skill ids must stay below 1000 (potions use 1000+)", s.Name)
			}
			if s.Tier > 0 && s.Parent() == nil {
				t.Errorf("%s: no parent", s.Name)
			}
			if !s.IsPassive() && s.Target == 0 {
				t.Errorf("%s: active skill without target", s.Name)
			}
		}
		for _, b := range c.Branches {
			if b.Name == "" || b.Color == "" {
				t.Errorf("%s: branch without name", id)
			}
		}
	}
}

func TestRankScaling(t *testing.T) {
	b := Skills[SkillShieldBash]
	if b.Mult.At(1) != 1.5 || b.Mult.At(3) <= b.Mult.At(1) {
		t.Errorf("shield bash rank scaling: %v %v", b.Mult.At(1), b.Mult.At(3))
	}
	if d := b.OnTarget.At(2).Duration; d != 1.75 {
		t.Errorf("stun at rank 2 = %v", d)
	}
}

func TestValidateRanks(t *testing.T) {
	start := StarterRanks("knight")
	if err := ValidateRanks("knight", 1, start); err != nil {
		t.Fatalf("starter ranks invalid: %v", err)
	}
	// Nível 1: nenhum ponto livre.
	more := map[uint32]int{SkillShieldBash: 2, SkillFortress: 1}
	if ValidateRanks("knight", 1, more) == nil {
		t.Error("should be out of points at level 1")
	}
	if ValidateRanks("knight", 2, more) != nil {
		t.Error("level 2 has one free point")
	}
	// Degrau exige nível e o nó de cima.
	if ValidateRanks("knight", 20, map[uint32]int{SkillTaunt: 1}) == nil {
		t.Error("taunt needs iron skin")
	}
	if ValidateRanks("knight", 9, map[uint32]int{SkillShieldBash: 1, SkillIronSkin: 1, SkillTaunt: 1}) == nil {
		t.Error("taunt needs level 10")
	}
	if ValidateRanks("knight", 10, map[uint32]int{SkillShieldBash: 1, SkillIronSkin: 1, SkillTaunt: 1}) != nil {
		t.Error("taunt at level 10 with parent should work")
	}
	if ValidateRanks("knight", 20, map[uint32]int{SkillFireball: 1}) == nil {
		t.Error("other class skill accepted")
	}
	if ValidateRanks("knight", 20, map[uint32]int{SkillJudgment: 2}) == nil {
		t.Error("rank above max accepted")
	}
}

func TestAttributesAreNeutralAtClassBase(t *testing.T) {
	for id := range Classes {
		base := ClassStats(id, 7)
		got := ApplyAttrs(base, id, 7, ClassBaseAttrs[id])
		if got.MaxHP != base.MaxHP || got.PAtk != base.PAtk || got.MDef != base.MDef || got.AtkInterval != base.AtkInterval {
			t.Errorf("%s: class base attributes should not change stats", id)
		}
		if ClassBaseAttrs[id].Sum() != 60 {
			t.Errorf("%s: base attributes sum %d", id, ClassBaseAttrs[id].Sum())
		}
	}
}

func TestAttributesAreLinear(t *testing.T) {
	// O cliente prevê a distribuição somando o efeito de +1 ponto: as fórmulas precisam ser lineares.
	stat := func(a Attrs) Stats {
		s := ApplyAttrs(ClassStats("rogue", 9), "rogue", 9, a)
		return FinishRegen(s, "rogue", a)
	}
	base := ClassBaseAttrs["rogue"]
	s0 := stat(base)
	for i := 0; i < 6; i++ {
		s1 := stat(base.Add(UnitAttr(i)))
		s3 := stat(base.Add(UnitAttr(i)).Add(UnitAttr(i)).Add(UnitAttr(i)))
		for _, pair := range [][3]float64{
			{s0.MaxHP, s1.MaxHP, s3.MaxHP}, {s0.PAtk, s1.PAtk, s3.PAtk}, {s0.Crit, s1.Crit, s3.Crit},
			{1 / s0.AtkInterval, 1 / s1.AtkInterval, 1 / s3.AtkInterval}, {s0.HPRegen, s1.HPRegen, s3.HPRegen},
			{s0.MPRegen, s1.MPRegen, s3.MPRegen}, {s0.CastSpeed, s1.CastSpeed, s3.CastSpeed}, {s0.Evasion, s1.Evasion, s3.Evasion},
		} {
			if math.Abs((pair[2]-pair[0])-3*(pair[1]-pair[0])) > 1e-9 {
				t.Errorf("attribute %d not linear: %v", i, pair)
			}
		}
	}
	if stat(base.Add(Attrs{Str: 5})).PAtk <= s0.PAtk || stat(base.Add(Attrs{Con: 5})).MaxHP <= s0.MaxHP {
		t.Error("points should raise stats")
	}
}

func TestRollHit(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	var misses, crits int
	var sum float64
	n := 20000
	for i := 0; i < n; i++ {
		h := RollHit(rng, HitInput{Atk: 100, Def: 100, Mult: 1, Crit: 0.2, Accuracy: 100, Evasion: 50})
		switch {
		case h.Miss:
			misses++
		case h.Crit:
			crits++
		default:
			sum += h.Damage
			if h.Damage < 44 || h.Damage > 56 {
				t.Fatalf("damage out of range: %v", h.Damage)
			}
		}
	}
	if r := float64(misses) / float64(n); math.Abs(r-(1-HitChance(100, 50))) > 0.01 {
		t.Errorf("miss rate %.3f", r)
	}
	if r := float64(crits) / float64(n-misses); math.Abs(r-0.2) > 0.02 {
		t.Errorf("crit rate %.3f", r)
	}
	if h := RollHit(rng, HitInput{Atk: 1, Def: 10000, Mult: 1}); !h.Miss && h.Damage < 1 {
		t.Error("damage should be at least 1")
	}
	// Sem erro e crítico forçado (Julgamento, golpe saindo da invisibilidade).
	for i := 0; i < 200; i++ {
		if h := RollHit(rng, HitInput{Atk: 50, Mult: 1, NoMiss: true, Accuracy: 0, Evasion: 1000}); h.Miss {
			t.Fatal("NoMiss missed")
		}
		if h := RollHit(rng, HitInput{Atk: 50, Mult: 1, ForceCrit: true, CritMult: 2.5}); !h.Crit || h.Damage < 50*2.5*0.9-1 {
			t.Fatalf("ForceCrit: %+v", h)
		}
	}
}

func TestXPCurve(t *testing.T) {
	if XPLevelStart(1) != 0 || XPLevelStart(2) != XPToNext(1) {
		t.Fatal("level start wrong")
	}
	if XPToNext(MaxLevel) != 0 {
		t.Fatal("no XP needed past max level")
	}
	for l := 1; l <= MaxLevel; l++ {
		if got := LevelForXP(XPLevelStart(l)); got != l {
			t.Errorf("LevelForXP(start of %d) = %d", l, got)
		}
	}
	if LevelForXP(math.MaxInt64/2) != MaxLevel {
		t.Error("huge xp should cap at max level")
	}
	// Ritmo: matar monstros do próprio nível sobe de nível com ~5–40 abates.
	for l := 1; l < MaxLevel; l++ {
		kills := float64(XPToNext(l)) / float64(MonsterXP(Monsters["skeleton_warrior"], l, l))
		if kills < 4 || kills > 45 {
			t.Errorf("level %d needs %.0f kills", l, kills)
		}
	}
}

func TestMonsterXPPenalty(t *testing.T) {
	m := Monsters["skeleton_minion"]
	full := MonsterXP(m, 3, 3)
	if MonsterXP(m, 3, 8) != full {
		t.Error("5 levels above should still give full XP")
	}
	if MonsterXP(m, 3, 15) >= full {
		t.Error("much stronger player should get less XP")
	}
	if MonsterXP(m, 1, 20) < 1 {
		t.Error("XP should never be zero")
	}
}

func TestDeathNeverDropsLevel(t *testing.T) {
	start := XPLevelStart(5)
	if got := XPAfterDeath(start+1, 5); got != start {
		t.Fatalf("should clamp to level start: %d", got)
	}
	mid := start + XPToNext(5)/2
	want := mid - int64(math.Round(float64(XPToNext(5))*DeathXPLoss))
	if got := XPAfterDeath(mid, 5); got != want {
		t.Fatalf("XPAfterDeath = %d, want %d", got, want)
	}
}

func TestExpandSpawnsDeterministicAndInRing(t *testing.T) {
	a := ExpandSpawns(DefaultSpawns(), rand.New(rand.NewPCG(7, 7)))
	b := ExpandSpawns(DefaultSpawns(), rand.New(rand.NewPCG(7, 7)))
	if len(a) != len(b) || len(a) < 50 {
		t.Fatalf("spawns: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("same seed must give same spawns")
		}
	}
	// Cada grupo no seu lugar: anéis em volta da cidade, áreas no próprio disco.
	rng := rand.New(rand.NewPCG(9, 9))
	for _, s := range DefaultSpawns() {
		for _, p := range ExpandSpawns([]Spawn{s}, rng) {
			if p.Level < s.LevelMin || p.Level > max(s.LevelMin, s.LevelMax) {
				t.Errorf("%s: nível %d fora de %d–%d", s.Monster, p.Level, s.LevelMin, s.LevelMax)
			}
			switch {
			case s.Area:
				if math.Hypot(p.X-s.X, p.Z-s.Z) > s.Radius+1e-9 {
					t.Errorf("%s fora da área: (%.1f, %.1f)", s.Monster, p.X, p.Z)
				}
			case s.Fixed:
			default:
				r := math.Hypot(p.X, p.Z)
				if r < 44 || r > 145 { // fora da cidade (raio 40) e dentro do quadrado original (±150)
					t.Errorf("spawn outside hunting area: r=%.1f", r)
				}
			}
		}
	}
}

func TestStoneGolemIsSlowTankInSouthValley(t *testing.T) {
	golem, warrior := Monsters["stone_golem"], Monsters["skeleton_warrior"]
	if golem == nil {
		t.Fatal("golem não existe")
	}
	g, w := MonsterStats(golem, 17), MonsterStats(warrior, 17)
	if g.MaxHP < 1.8*w.MaxHP || g.PDef <= w.PDef || g.PAtk <= w.PAtk {
		t.Errorf("golem deveria ser bem mais resistente e forte: %+v vs %+v", g, w)
	}
	if g.Speed >= w.Speed || g.AtkInterval <= w.AtkInterval {
		t.Errorf("golem deveria ser lento: speed %.1f, intervalo %.1f", g.Speed, g.AtkInterval)
	}
	// Só no Vale das Pedras Vivas (sul, além do quadrado original), níveis 20–25,
	// mais fortes quanto mais fundo.
	var n int
	shallow, deep := SpawnPoint{Z: -1e9}, SpawnPoint{Z: 1e9}
	for _, sp := range ExpandSpawns(DefaultSpawns(), rand.New(rand.NewPCG(3, 3))) {
		if sp.Monster != golem {
			continue
		}
		n++
		if math.Hypot(sp.X, sp.Z+270) > 52 || sp.Z > -150 || sp.Level < 20 || sp.Level > 25 {
			t.Errorf("golem fora do vale: (%.1f, %.1f) nv %d", sp.X, sp.Z, sp.Level)
		}
		if sp.Z > shallow.Z {
			shallow = sp
		}
		if sp.Z < deep.Z {
			deep = sp
		}
	}
	if n != 12 {
		t.Errorf("esperava 12 golens, veio %d", n)
	}
	if deep.Level <= shallow.Level {
		t.Errorf("o golem mais fundo (nv %d) deveria ser mais forte que o da entrada (nv %d)", deep.Level, shallow.Level)
	}
	var core bool
	for _, e := range golem.Loot {
		core = core || e.ItemID == "rune_core"
	}
	if !core {
		t.Error("golem deveria derrubar Núcleo Rúnico")
	}
}

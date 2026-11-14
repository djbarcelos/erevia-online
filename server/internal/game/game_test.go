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
		if len(tree) != 15 {
			t.Fatalf("%s: %d skills in the tree, want 15 (3 ramos × 5 degraus)", id, len(tree))
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
	if err := ValidateRanks("knight", "", 1, start); err != nil {
		t.Fatalf("starter ranks invalid: %v", err)
	}
	// Nível 1: nenhum ponto livre.
	more := map[uint32]int{SkillShieldBash: 2, SkillFortress: 1}
	if ValidateRanks("knight", "", 1, more) == nil {
		t.Error("should be out of points at level 1")
	}
	if ValidateRanks("knight", "", 2, more) != nil {
		t.Error("level 2 has one free point")
	}
	// Degrau exige nível e o nó de cima.
	if ValidateRanks("knight", "", 20, map[uint32]int{SkillTaunt: 1}) == nil {
		t.Error("taunt needs iron skin")
	}
	if ValidateRanks("knight", "", 9, map[uint32]int{SkillShieldBash: 1, SkillIronSkin: 1, SkillTaunt: 1}) == nil {
		t.Error("taunt needs level 10")
	}
	if ValidateRanks("knight", "", 10, map[uint32]int{SkillShieldBash: 1, SkillIronSkin: 1, SkillTaunt: 1}) != nil {
		t.Error("taunt at level 10 with parent should work")
	}
	if ValidateRanks("knight", "", 20, map[uint32]int{SkillFireball: 1}) == nil {
		t.Error("other class skill accepted")
	}
	if ValidateRanks("knight", "", 20, map[uint32]int{SkillJudgment: 2}) == nil {
		t.Error("rank above max accepted")
	}
	// 5º degrau: nível 20 e o degrau de cima (Muralha Viva, que pede Provocar, que pede…).
	chain := map[uint32]int{SkillShieldBash: 1, SkillIronSkin: 1, SkillTaunt: 1, SkillLivingWall: 1, SkillBulwark: 1}
	if ValidateRanks("knight", "", 19, chain) == nil {
		t.Error("tier 5 needs level 20")
	}
	if err := ValidateRanks("knight", "", 20, chain); err != nil {
		t.Errorf("tier 5 at level 20 with the branch learned should work: %v", err)
	}
	if ValidateRanks("knight", "", 30, map[uint32]int{SkillShieldBash: 1, SkillIronSkin: 1, SkillTaunt: 1, SkillBulwark: 1}) == nil {
		t.Error("tier 5 needs the tier 4 skill of the branch")
	}
	chain[SkillBulwark] = 3
	if ValidateRanks("knight", "", 20, chain) != nil {
		t.Error("rank 3 of the tier 5 should work")
	}
	chain[SkillBulwark] = 4
	if ValidateRanks("knight", "", 30, chain) == nil {
		t.Error("tier 5 above rank 3 accepted")
	}
}

func TestFifthTierSkills(t *testing.T) {
	if TierLevels[4] != 20 {
		t.Fatalf("5º degrau no nível %d, want 20", TierLevels[4])
	}
	for id := range Classes {
		var fifth []*SkillDef
		total := 0
		for _, s := range ClassTree(id) {
			total += s.MaxRank
			if s.Tier == 4 {
				fifth = append(fifth, s)
			}
		}
		if len(fifth) != 3 {
			t.Fatalf("%s: %d skills no 5º degrau, want 3", id, len(fifth))
		}
		for _, s := range fifth {
			if s.LevelReq() != 20 || s.IsPassive() || s.MaxRank != 3 {
				t.Errorf("%s: nível %d, passiva %v, rank máx %d", s.Name, s.LevelReq(), s.IsPassive(), s.MaxRank)
			}
			if p := s.Parent(); p == nil || p.Tier != 3 || p.Branch != s.Branch {
				t.Errorf("%s: o pai deveria ser o 4º degrau do mesmo ramo", s.Name)
			}
			// Cada skill nova faz alguma coisa (dano ou efeito) e cresce com o rank.
			if s.Mult.At(1) <= 0 && s.OnTarget == nil && s.OnSelf == nil {
				t.Errorf("%s: não faz nada", s.Name)
			}
			if s.Text(1) == s.Text(3) {
				t.Errorf("%s: o texto do rank 3 deveria mudar", s.Name)
			}
			if s.MP.At(1) <= 0 || s.CD.At(1) <= 0 {
				t.Errorf("%s: sem custo ou recarga", s.Name)
			}
		}
		// No nível máximo ainda não dá para aprender tudo: escolher continua valendo. Os
		// pontos de skill param no 30 (do 31 ao 40 só os de atributo sobem).
		if SkillPointsTotal(MaxLevel) >= total || SkillPointsTotal(MaxLevel) != SkillPointsTotal(SkillPointMaxLevel) {
			t.Errorf("%s: %d pontos no nível máximo para %d ranks", id, SkillPointsTotal(MaxLevel), total)
		}
		if AttrPointsTotal(MaxLevel) <= AttrPointsTotal(SkillPointMaxLevel) {
			t.Errorf("os pontos de atributo continuam por nível depois do %d", SkillPointMaxLevel)
		}
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
	// Ritmo: abates do próprio nível (no grupo de monstros que se caça em cada nível, sem
	// contar a XP das missões) para subir. Cresce com o nível e fica dentro de cada faixa:
	// ~25 min por nível no começo, ~10 h no fim (25 s por abate).
	limits := map[int][2]float64{1: {2, 200}, 10: {150, 500}, 20: {400, 1300}, 30: {1100, 2700}}
	prev := 0.0
	for l := 1; l < MaxLevel; l++ {
		perKill, _ := huntXP(l)
		kills := float64(XPToNext(l)) / perKill
		lim := limits[l/10*10]
		if l < 10 {
			lim = limits[1]
		}
		if kills < lim[0] || kills > lim[1] || kills < prev*0.95 {
			t.Errorf("nível %d pede %.0f abates (faixa %v, o anterior pedia %.0f)", l, kills, lim, prev)
		}
		prev = kills
	}
	if MaxLevel != 40 {
		t.Fatalf("MaxLevel = %d", MaxLevel)
	}
	for l := 2; l < MaxLevel; l++ {
		if XPToNext(l) <= XPToNext(l-1) {
			t.Errorf("XPToNext(%d) = %d não cresce", l, XPToNext(l))
		}
	}
	// O peso fica no fim, como no Lineage 2: cada faixa de 10 níveis pede bem mais XP que
	// tudo o que veio antes dela.
	for _, f := range []int{20, 30, 40} {
		if band, before := XPLevelStart(f)-XPLevelStart(f-10), XPLevelStart(f-10); band < 2*before {
			t.Errorf("%d → %d pede %d de XP, 1 → %d pede %d", f-10, f, band, f-10, before)
		}
	}
}

// A curva 1 (a de antes) continua igual para converter a XP salva: 1.839.820 até o 30 e
// 335.517 até o 20.
func TestXPCurveV1(t *testing.T) {
	if xpLevelStartV1(30) != 1_839_820 || xpLevelStartV1(20) != 335_517 || xpToNextV1(30) != 0 ||
		xpToNextV1(19) != int64(math.Round(80*math.Pow(19, 2.2))) {
		t.Fatalf("curva 1: início do 30 = %d, do 20 = %d", xpLevelStartV1(30), xpLevelStartV1(20))
	}
}

// Ninguém perde nível nem progresso: a XP salva na curva 1 vira a mesma fração do mesmo
// nível na curva 2. No 30 (o máximo de antes) a barra estava cheia ou travada: começa o
// 30 → 31 do zero.
func TestConvertXPKeepsLevelAndFraction(t *testing.T) {
	for _, l := range []int{1, 19, 20, 29} {
		for _, frac := range []float64{0, 0.5, 0.999} {
			old := xpLevelStartV1(l) + int64(frac*float64(xpToNextV1(l)))
			got := ConvertXP(old, 1)
			if lv := LevelForXP(got); lv != l {
				t.Fatalf("nível %d, %.1f%%: virou o nível %d", l, frac*100, lv)
			}
			had := float64(old-xpLevelStartV1(l)) / float64(xpToNextV1(l))
			now := float64(got-XPLevelStart(l)) / float64(XPToNext(l))
			// Até 1 de XP de arredondamento (o nível 1 → 2 pede só 43).
			if math.Abs(had-now) > 1/float64(XPToNext(l))+1e-4 {
				t.Fatalf("nível %d: fração %.5f virou %.5f", l, had, now)
			}
		}
	}
	for _, xp := range []int64{xpLevelStartV1(30), xpLevelStartV1(30) + 123_456, 1 << 40} {
		if got := ConvertXP(xp, 1); got != XPLevelStart(30) || LevelForXP(got) != 30 {
			t.Fatalf("no 30 da curva 1 (%d de XP): %d, nível %d", xp, got, LevelForXP(got))
		}
	}
	if ConvertXP(0, 1) != 0 || ConvertXP(-5, 1) != 0 {
		t.Fatal("XP zero continua zero")
	}
	// Já na curva de hoje: não mexe.
	if x := XPLevelStart(33) + 7; ConvertXP(x, XPCurve) != x {
		t.Fatal("a curva de hoje não converte de novo")
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
	// Do 30 em diante a morte tira 2% (um nível leva de 6 a 12 h).
	late := XPLevelStart(35) + XPToNext(35)/2
	if got := XPAfterDeath(late, 35); got != late-int64(math.Round(float64(XPToNext(35))*DeathXPLossLate)) {
		t.Fatalf("XPAfterDeath no 35 = %d", got)
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
	// Só no Vale das Pedras Vivas (sul, além do quadrado original), níveis 20–30,
	// mais fortes quanto mais fundo.
	var n int
	shallow, deep := SpawnPoint{Z: -1e9}, SpawnPoint{Z: 1e9}
	for _, sp := range ExpandSpawns(DefaultSpawns(), rand.New(rand.NewPCG(3, 3))) {
		if sp.Monster != golem {
			continue
		}
		n++
		if math.Hypot(sp.X, sp.Z+270) > 52 || sp.Z > -150 || sp.Level < 20 || sp.Level > 30 {
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
	if deep.Level < 27 {
		t.Errorf("o fundo do vale deveria ser caça de 25–30 (o mais fundo veio nv %d)", deep.Level)
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

// Guarda do deploy: XP da curva 1 numa linha marcada na curva de hoje (o servidor antigo
// gravou depois do novo converter) é convertida de novo pelo nível salvo; XP legítima não
// muda; o que não cabe em nenhuma fica no início do nível salvo. O nível nunca desce.
func TestCheckSavedXP(t *testing.T) {
	for _, c := range []struct {
		level int
		xp    int64
	}{{30, xpLevelStartV1(30)}, {30, xpLevelStartV1(30) + 99_999}, {15, xpLevelStartV1(15) + xpToNextV1(15)/2}} {
		got, how := CheckSavedXP(c.xp, c.level)
		if how != XPFromV1 || got != ConvertXP(c.xp, 1) || LevelForXP(got) != c.level {
			t.Fatalf("XP %d da curva 1 no nível %d: %d (%v), nível %d", c.xp, c.level, got, how, LevelForXP(got))
		}
	}
	for _, l := range []int{1, 15, 30, 39, MaxLevel} {
		for _, xp := range []int64{XPLevelStart(l), XPLevelStart(l) + XPToNext(l)/2, XPLevelStart(l+1) - 1} {
			if l == MaxLevel {
				xp = XPLevelStart(l)
			}
			if got, how := CheckSavedXP(xp, l); how != XPOK || got != xp {
				t.Fatalf("XP legítima %d no nível %d mudou: %d (%v)", xp, l, got, how)
			}
		}
	}
	// Nem uma curva nem a outra: no início do nível salvo, sem descer (35 não existe na 1).
	if got, how := CheckSavedXP(5_000_000, 35); how != XPClamped || got != XPLevelStart(35) {
		t.Fatalf("XP sem curva no 35: %d (%v)", got, how)
	}
	if got, how := CheckSavedXP(XPLevelStart(38), 33); how != XPClamped || got != XPLevelStart(33) {
		t.Fatalf("XP acima do nível salvo: %d (%v)", got, how)
	}
}

package zone

import (
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// newCombatZone cria uma zona com os monstros pedidos (sem os padrões).
func newCombatZone(spawns ...game.Spawn) *Zone {
	cfg := testConfig()
	cfg.Spawns = spawns
	cfg.SafeRadius = 20 // os testes de combate lutam a 20–40 m da origem
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func fixed(monster string, level int, x, z float64) game.Spawn {
	return game.Spawn{Monster: monster, Count: 1, Fixed: true, X: x, Z: z, LevelMin: level, Respawn: 5}
}

func (z *Zone) tickFor(seconds float64) {
	for i := 0; i < int(math.Round(seconds*float64(z.cfg.TickRate))); i++ {
		z.update(1 / float64(z.cfg.TickRate))
	}
}

func (z *Zone) onlyMonster() *entity {
	for _, m := range z.monsters {
		return m
	}
	return nil
}

func joinAt(z *Zone, id uint32, class string, x, zz float64, xp int64) (*entity, *fakePeer) {
	peer := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: id, Name: "P" + class, Class: class, X: x, Z: zz, XP: xp, Peer: peer}})
	return z.players[id], peer
}

// combatFrom devolve os CombatEvents recebidos cujo autor é src.
func (f *fakePeer) combatFrom(src uint32) []*pb.CombatEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pb.CombatEvent
	for _, m := range f.msgs {
		for _, c := range m.GetSnapshot().GetCombat() {
			if c.SourceId == src {
				out = append(out, c)
			}
		}
	}
	return out
}

func (f *fakePeer) has(match func(*pb.ServerMessage) bool) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if match(m) {
			return true
		}
	}
	return false
}

func (f *fakePeer) systemContains(s string) bool {
	return f.has(func(m *pb.ServerMessage) bool {
		c := m.GetChat()
		return c != nil && c.Channel == pb.ChatChannel_CHAT_CHANNEL_SYSTEM && strings.Contains(c.Text, s)
	})
}

func (f *fakePeer) reset() {
	f.mu.Lock()
	f.msgs = nil
	f.mu.Unlock()
}

func TestAggressiveMonsterChasesThenGivesUpAndHeals(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 6, 40, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 34, 0, 0)

	z.tickFor(0.2)
	if m.act.target != p.id {
		t.Fatalf("aggressive monster should target player within %v m", aggroRange)
	}

	// O jogador foge para longe: o monstro persegue até o limite e desiste.
	m.hp = 10
	p.pos, p.dest = Vec2{X: 95, Z: 0}, Vec2{X: 95, Z: 0}
	gaveUp := false
	for i := 0; i < 400 && !gaveUp; i++ {
		z.update(0.05)
		gaveUp = m.mon.returning
	}
	if !gaveUp {
		t.Fatalf("monster should give up after leaving leash (pos %v)", m.pos)
	}
	z.tickFor(15)
	if dist(m.pos, m.mon.home) > 0.5 || m.hp != m.stats.MaxHP {
		t.Fatalf("monster should be home with full hp: pos=%v hp=%v/%v", m.pos, m.hp, m.stats.MaxHP)
	}
}

func TestPassiveMonsterOnlyRetaliates(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 3, 25, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 22, 0, 0)

	z.tickFor(3)
	if m.act.kind != actNone {
		t.Fatal("passive monster must not attack first")
	}
	z.handle(attackEvent{id: p.id, target: m.id})
	z.tickFor(3)
	if m.act.target != p.id {
		t.Fatal("passive monster should fight back")
	}
}

func TestAutoAttackApproachesAndRespectsRate(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 10, 30, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "knight", 18, 0, 0)

	z.handle(attackEvent{id: p.id, target: m.id})
	var firstHitAt float64 = -1
	for i := 0; i < 400 && firstHitAt < 0; i++ {
		z.update(0.05)
		if len(peer.combatFrom(p.id)) > 0 {
			firstHitAt = z.now
			if d := dist(p.pos, m.pos); d > p.stats.Range+0.3 {
				t.Fatalf("first hit from %.1f m (range %.1f)", d, p.stats.Range)
			}
		}
	}
	if firstHitAt < 0 {
		t.Fatal("player never attacked")
	}
	peer.reset()
	z.tickFor(6)
	n := len(peer.combatFrom(p.id))
	want := 6 / p.stats.AtkInterval
	if math.Abs(float64(n)-want) > 1.01 {
		t.Fatalf("%d attacks in 6 s, want ~%.0f (interval %.1f s)", n, want, p.stats.AtkInterval)
	}

	// Clicar no chão cancela o ataque.
	z.handle(moveEvent{id: p.id, x: 0, z: 0})
	if p.act.kind != actNone {
		t.Fatal("MoveTo should cancel attack")
	}
}

func TestSkillCooldownAndMana(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 10, 40, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "mage", 28, 0, 0) // 12 m: dentro do alcance da Bola de Fogo (20 m)
	mp0 := p.mp

	z.handle(skillEvent{id: p.id, skill: game.SkillFireball, target: m.id})
	z.tickFor(0.1)
	var casts int
	for _, c := range peer.combatFrom(p.id) {
		if c.SkillId == game.SkillFireball {
			casts++
		}
	}
	if casts != 1 || p.mp > mp0-game.Skills[game.SkillFireball].MPCost(1)+1 {
		t.Fatalf("fireball should hit once and cost mana: casts=%d mp %.0f→%.0f", casts, mp0, p.mp)
	}

	peer.reset()
	z.handle(skillEvent{id: p.id, skill: game.SkillFireball, target: m.id})
	z.tickFor(0.1)
	for _, c := range peer.combatFrom(p.id) {
		if c.SkillId == game.SkillFireball {
			t.Fatal("skill on cooldown must not fire")
		}
	}
	if !peer.has(func(m *pb.ServerMessage) bool { return m.GetSkillCooldown().GetReadyInMs() > 0 }) {
		t.Fatal("should tell the client the remaining cooldown")
	}

	// Skill de outra classe é ignorada; sem mana avisa.
	z.handle(skillEvent{id: p.id, skill: game.SkillShieldBash, target: m.id})
	p.mp = 1
	p.pl.cooldowns = map[uint32]float64{}
	z.handle(skillEvent{id: p.id, skill: game.SkillFireball, target: m.id})
	if !peer.systemContains("Mana insuficiente") {
		t.Fatal("expected not-enough-mana message")
	}
}

func TestStunStopsMonster(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 6, 30, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "knight", 28.5, 0, 0)
	z.tickFor(0.5) // monstro agressivo engaja

	bash := game.Skills[game.SkillShieldBash]
	for i := 0; i < 20 && !m.stunned(z.now); i++ {
		p.mp = p.stats.MaxMP
		z.castSkill(p, bash, m)
	}
	if !m.stunned(z.now) {
		t.Fatal("shield bash never stunned")
	}
	peer.reset()
	pos := m.pos
	z.tickFor(1.2)
	if len(peer.combatFrom(m.id)) != 0 || m.pos != pos {
		t.Fatal("stunned monster must not act or move")
	}
}

func TestSlowAndBuffChangeStats(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "rogue", 0, 0, 0)
	base := p.speed(z.now)
	z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectSlow, Duration: 2, Value: 0.5})
	if got := p.speed(z.now); math.Abs(got-base*0.5) > 1e-9 {
		t.Fatalf("slow: %v → %v", base, got)
	}
	z.tickFor(2.1)
	if p.speed(z.now) != base {
		t.Fatal("slow should expire")
	}
	def := p.def(false, z.now)
	z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectBuffDef, Duration: 5, Value: 0.5})
	if p.def(false, z.now) != def*1.5 {
		t.Fatal("defense buff not applied")
	}
}

func TestKillGivesXPAndLevelsUp(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 2, 25, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "barbarian", 23.5, 0, game.XPLevelStart(2)-1)
	if p.level != 1 {
		t.Fatalf("level = %d", p.level)
	}
	p.hp = 10
	m.hp = 1
	z.handle(attackEvent{id: p.id, target: m.id})
	for i := 0; i < 200 && !m.dead; i++ {
		z.update(0.05)
	}
	if !m.dead {
		t.Fatal("monster should die")
	}
	z.tickFor(0.05)
	if p.level != 2 || p.hp != p.stats.MaxHP {
		t.Fatalf("should level up with full hp: level=%d hp=%v/%v", p.level, p.hp, p.stats.MaxHP)
	}
	if !peer.systemContains("XP") || !peer.systemContains("nível 2") {
		t.Fatal("expected XP and level-up messages")
	}
	if !peer.has(func(m *pb.ServerMessage) bool { return len(m.GetSnapshot().GetLevelUps()) > 0 }) {
		t.Fatal("expected LevelUp broadcast")
	}
	if !peer.has(func(m *pb.ServerMessage) bool { return m.GetSelfStats().GetLevel() == 2 }) {
		t.Fatal("expected SelfStats with new level")
	}
}

func TestDeathLosesXPAndRespawnRestores(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 12, 40, 0))
	start := game.XPLevelStart(5)
	xp := start + game.XPToNext(5)/2
	p, peer := joinAt(z, 1, "mage", 38, 0, xp)
	p.hp = 1

	for i := 0; i < 400 && !p.dead; i++ {
		z.update(0.05)
	}
	if !p.dead {
		t.Fatal("player should die")
	}
	if p.pl.xp >= xp || p.pl.xp < start || p.level != 5 {
		t.Fatalf("death penalty wrong: xp %d → %d (level start %d), level %d", xp, p.pl.xp, start, p.level)
	}
	if !peer.systemContains("Você morreu") {
		t.Fatal("expected death message")
	}
	m := z.onlyMonster()
	if _, hates := m.mon.hate[p.id]; hates || m.act.target == p.id {
		t.Fatal("monster should forget dead player")
	}

	// Mortos não andam nem atacam.
	z.handle(moveEvent{id: p.id, x: 10, z: 10})
	if p.dest != p.pos {
		t.Fatal("dead player must not move")
	}

	z.handle(respawnEvent{id: p.id})
	z.tickFor(0.05)
	if p.dead || p.hp != p.stats.MaxHP || math.Hypot(p.pos.X, p.pos.Z) > townSpawnRadius {
		t.Fatalf("respawn wrong: dead=%v hp=%v pos=%v", p.dead, p.hp, p.pos)
	}
	if !peer.has(func(m *pb.ServerMessage) bool { return len(m.GetSnapshot().GetRevives()) > 0 }) {
		t.Fatal("expected Revive broadcast")
	}
}

func TestSafeZoneBlocksCombat(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 6, 20, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "mage", 13, 0, 0) // dentro da cidade (raio 15), a 7 m do monstro

	z.tickFor(2)
	if len(m.mon.hate) != 0 || m.act.kind != actNone {
		t.Fatal("monster must not aggro a player inside town")
	}
	z.handle(attackEvent{id: p.id, target: m.id}) // mago alcança 12 m, mas está na cidade
	z.tickFor(2)
	if len(peer.combatFrom(p.id)) != 0 {
		t.Fatal("no combat from inside town")
	}
	z.handle(skillEvent{id: p.id, skill: game.SkillIceNova})
	if !peer.systemContains("cidade") {
		t.Fatal("area skill inside town should be refused")
	}
}

func TestMonsterCorpseVanishesAndRespawns(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 1, 25, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "knight", 23.5, 0, 0)
	m.hp = 1
	z.handle(attackEvent{id: p.id, target: m.id})
	for i := 0; i < 200 && !m.dead; i++ {
		z.update(0.05)
	}
	if !m.dead {
		t.Fatal("monster should die")
	}
	z.tickFor(corpseSeconds + 0.1)
	if !m.mon.hidden || !peer.has(func(x *pb.ServerMessage) bool { return x.GetDespawn().GetId() == m.id }) {
		t.Fatal("corpse should vanish with Despawn")
	}
	z.tickFor(2 * 5) // respawn entre 5 e 10 s
	if m.dead || m.hp != m.stats.MaxHP || dist(m.pos, m.mon.home) > wanderRadius+0.5 {
		t.Fatalf("monster should respawn at home with full hp: dead=%v hp=%v", m.dead, m.hp)
	}
	if !peer.has(func(x *pb.ServerMessage) bool { return x.GetSpawn().GetEntity().GetId() == m.id }) {
		t.Fatal("expected Spawn on respawn")
	}
}

func TestDefaultZoneHasMonstersInRings(t *testing.T) {
	z := New(testConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	if len(z.monsters) < 50 {
		t.Fatalf("only %d monsters", len(z.monsters))
	}
	for _, m := range z.monsters {
		if z.inSafe(m.pos) {
			t.Fatalf("monster %s spawned inside town", m.name)
		}
		if m.id < monsterIDBase {
			t.Fatal("monster id collides with player ids")
		}
	}
	// Uma simulação longa sem jogadores não quebra nada.
	z.tickFor(30)
}

// O golem é grande: o golpe conta da borda do corpo, então o cavaleiro para encostado
// nas pedras (não dentro delas) e acerta de onde um esqueleto já estaria longe demais.
func TestBigMonsterIsHitFromTheEdgeOfItsBody(t *testing.T) {
	z := newCombatZone(fixed("stone_golem", 20, 40, 0))
	m := z.onlyMonster()
	r := m.bodyRadius()
	if r < 0.8 {
		t.Fatalf("golem deveria ter corpo grande, raio %.2f", r)
	}
	m.mon.def = &game.MonsterDef{}
	*m.mon.def = *game.Monsters["stone_golem"]
	m.mon.def.Aggressive = false // só o jogador age neste teste

	// Começa longe: anda até o golem e para fora do corpo.
	p, _ := joinAt(z, 1, "knight", 28, 0, 0)
	z.handle(attackEvent{id: p.id, target: m.id})
	z.tickFor(4)
	d := dist(p.pos, m.pos)
	if d < r || d > r+p.stats.Range {
		t.Fatalf("o cavaleiro deveria parar encostado no golem: distância %.2f (corpo %.2f, alcance %.2f)", d, r, p.stats.Range)
	}
	if m.hp >= m.stats.MaxHP {
		t.Fatal("o cavaleiro deveria ter acertado o golem")
	}

	// Parado a alcance + corpo − folga: acerta sem precisar andar.
	m.hp = m.stats.MaxHP
	at := Vec2{X: m.pos.X - (p.stats.Range + r - 0.1), Z: m.pos.Z}
	p.pos, p.dest = at, at
	p.nextAttack = 0
	z.tickFor(0.2)
	if p.pos != at {
		t.Fatalf("já estava no alcance (pela borda do corpo) e andou para %v", p.pos)
	}
	if m.hp >= m.stats.MaxHP {
		t.Fatal("deveria acertar da borda do corpo")
	}
}

package zone

import (
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// snapshots devolve os snapshots recebidos pelo peer.
func (f *fakePeer) snapshots() []*pb.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pb.Snapshot
	for _, m := range f.msgs {
		if s := m.GetSnapshot(); s != nil {
			out = append(out, s)
		}
	}
	return out
}

func TestCooldownSurvivesRelog(t *testing.T) {
	now := noon
	z := newClockZone(&now)
	p, _ := joinAt(z, 7, "knight", 30, 0, 0)
	z.handle(skillEvent{id: p.id, skill: game.SkillFortress})
	if p.pl.cooldowns[game.SkillFortress] <= z.now {
		t.Fatal("Fortaleza deveria entrar em recarga")
	}

	z.removePlayer(p.id)
	now = now.Add(5 * time.Second) // 5 s fora do jogo
	z.tickFor(5)

	p2, peer := joinAt(z, 7, "knight", 30, 0, 0)
	left := p2.pl.cooldowns[game.SkillFortress] - z.now
	want := game.Skills[game.SkillFortress].Cooldown - 5
	if left < want-0.1 || left > want+0.1 {
		t.Fatalf("recarga depois de voltar: %.2f s, esperava ~%.0f s", left, want)
	}
	if !peer.has(func(m *pb.ServerMessage) bool {
		c := m.GetSkillCooldown()
		return c != nil && c.SkillId == game.SkillFortress && c.ReadyInMs > 0
	}) {
		t.Fatal("o cliente precisa saber da recarga ao entrar")
	}

	// Depois que vence, sair e voltar não guarda nada.
	z.removePlayer(p2.id)
	now = now.Add(time.Minute)
	p3, _ := joinAt(z, 7, "knight", 30, 0, 0)
	if p3.pl.cooldowns[game.SkillFortress] > z.now {
		t.Fatal("recarga vencida não deveria voltar")
	}
}

func TestCastSkillRechecksCooldown(t *testing.T) {
	z := newCombatZone()
	p, _ := joinAt(z, 1, "knight", 30, 0, 0)
	s := game.Skills[game.SkillFortress]
	mp0 := p.mp
	z.castSkill(p, s, nil)
	z.castSkill(p, s, nil) // chamada direta, sem passar por requestSkill
	if spent := mp0 - p.mp; spent != s.MPCost {
		t.Fatalf("gastou %.0f de mana, esperava %.0f (uma vez só)", spent, s.MPCost)
	}
}

func TestSelfAndAreaSkillsSendCast(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "mage", 30, 0, 0)
	z.handle(skillEvent{id: p.id, skill: game.SkillIceNova})
	z.tickFor(0.1)
	var casts int
	for _, s := range peer.snapshots() {
		for _, c := range s.Casts {
			if c.SourceId == p.id && c.SkillId == game.SkillIceNova {
				casts++
			}
		}
	}
	if casts != 1 {
		t.Fatalf("esperava 1 SkillCast da Nova, veio %d", casts)
	}
	for _, c := range peer.combatFrom(p.id) {
		if c.TargetId == p.id {
			t.Fatalf("não deveria mais haver CombatEvent falso em si mesmo: %+v", c)
		}
	}
}

func TestEffectEndOnExpiryAndDeath(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 1, 40, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "mage", 36, 0, 0) // 4 m: dentro da Nova (6 m)
	m.hp = 1e6                                // não morre com a Nova
	z.handle(skillEvent{id: p.id, skill: game.SkillIceNova})
	z.tickFor(0.1)

	var applied *pb.EffectEvent
	for _, s := range peer.snapshots() {
		for _, e := range s.Effects {
			if e.TargetId == m.id && e.Effect == pb.Effect_EFFECT_SLOW {
				applied = e
			}
		}
	}
	if applied == nil || applied.SourceId != p.id || applied.Value != 0.5 {
		t.Fatalf("lentidão sem origem/valor: %+v", applied)
	}

	ended := func(target uint32, eff pb.Effect) bool {
		for _, s := range peer.snapshots() {
			for _, e := range s.EffectEnds {
				if e.TargetId == target && e.Effect == eff {
					return true
				}
			}
		}
		return false
	}
	z.tickFor(3.2)
	if !ended(m.id, pb.Effect_EFFECT_SLOW) {
		t.Fatal("faltou EffectEnd quando a lentidão venceu")
	}

	// Morte encerra os efeitos ativos.
	peer.reset()
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectStun, Duration: 10})
	z.kill(m, p)
	z.tickFor(0.1)
	if !ended(m.id, pb.Effect_EFFECT_STUN) {
		t.Fatal("faltou EffectEnd do atordoamento ao morrer")
	}
}

func TestQueuedSkillInSafeZoneIsCancelled(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 1, 5, 0)) // dentro da cidade (raio 20)
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "knight", 3, 0, 0)
	z.handle(skillEvent{id: p.id, skill: game.SkillShieldBash, target: m.id})
	z.tickFor(0.2)
	if p.act.kind != actNone {
		t.Fatal("a skill na fila deveria ser cancelada dentro da cidade")
	}
	if !peer.systemContains("dentro da cidade") {
		t.Fatal("faltou avisar que não se luta na cidade")
	}
	if !peer.has(func(m *pb.ServerMessage) bool {
		c := m.GetSkillCooldown()
		return c != nil && c.SkillId == game.SkillShieldBash && c.ReadyInMs == 0
	}) {
		t.Fatal("o cliente precisa saber que a skill saiu da fila (recarga 0)")
	}
	if p.pl.cooldowns[game.SkillShieldBash] > z.now {
		t.Fatal("skill cancelada não pode entrar em recarga")
	}
}

func TestSkillInfoDescribesEffects(t *testing.T) {
	nova := skillInfoPB(game.Skills[game.SkillIceNova])
	if nova.Fx != "ice_nova" || nova.Description == "" || nova.Effect != pb.Effect_EFFECT_SLOW ||
		nova.EffectMs != 3000 || nova.EffectOnSelf || !nova.Magic || nova.Power != 1.2 {
		t.Fatalf("Nova de Gelo mal descrita: %+v", nova)
	}
	fort := skillInfoPB(game.Skills[game.SkillFortress])
	if !fort.EffectOnSelf || fort.Effect != pb.Effect_EFFECT_BUFF_DEF || fort.EffectValue != 0.5 {
		t.Fatalf("Fortaleza mal descrita: %+v", fort)
	}
	if got := len(skillCatalog()); got != len(game.Skills) {
		t.Fatalf("catálogo com %d skills, esperava %d", got, len(game.Skills))
	}
	for _, s := range game.Skills {
		if s.FX == "" || s.Description == "" {
			t.Errorf("%s sem FX ou descrição", s.Name)
		}
	}
}

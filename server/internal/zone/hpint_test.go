package zone

import (
	"math"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// A regra de inteiros da vida e da mana: o atual nunca passa do máximo, vivo nunca
// aparece com 0 e cheio aparece cheio.
func TestHPAndMPIntegerRule(t *testing.T) {
	cases := []struct {
		name               string
		hp, maxHP, mp, max float64
		wantHP, wantMaxHP  int
		wantMP, wantMaxMP  int
	}{
		{"vida cheia com máximo 408,4", 408.4, 408.4, 50, 50, 408, 408, 50, 50},
		{"vida cheia com máximo 408,6", 408.6, 408.6, 50, 50, 409, 409, 50, 50},
		{"quase cheia com máximo 408,4", 408.1, 408.4, 50, 50, 408, 408, 50, 50},
		{"vivo com fração de vida", 0.3, 100, 0, 50, 1, 100, 0, 50},
		{"vida negativa nunca vira número enorme", -3.2, 100, -1, 50, 0, 100, 0, 50},
		{"mana cheia com máximo 200,6", 100, 100, 200.6, 200.6, 100, 100, 201, 201},
		{"mana quase cheia com máximo 200,6", 100, 100, 200.5, 200.6, 100, 100, 200, 201},
		{"mana cheia com máximo 200,4", 100, 100, 200.4, 200.4, 100, 100, 200, 200},
		{"vida acima do máximo (bug) não passa do máximo", 500, 408.4, 300, 200.4, 408, 408, 200, 200},
	}
	for _, c := range cases {
		e := &entity{hp: c.hp, mp: c.mp, stats: game.Stats{MaxHP: c.maxHP, MaxMP: c.max}}
		if e.hpInt() != c.wantHP || e.maxHPInt() != c.wantMaxHP || e.mpInt() != c.wantMP || e.maxMPInt() != c.wantMaxMP {
			t.Errorf("%s: %d / %d, %d / %d; quer %d / %d, %d / %d", c.name,
				e.hpInt(), e.maxHPInt(), e.mpInt(), e.maxMPInt(), c.wantHP, c.wantMaxHP, c.wantMP, c.wantMaxMP)
		}
	}
}

func frac(v float64) float64 { return v - math.Floor(v) }

// Com o máximo fracionário (o ,4 que dava "409 / 408") e a vida cheia, toda mensagem
// que leva vida ou mana como inteiro mostra atual == máximo.
func TestFullHPWithFractionalMaxIsFullInEveryMessage(t *testing.T) {
	z := newCombatZone()
	// Procura pontos de Constituição e Espírito que deixem a vida máxima com fração
	// abaixo de 0,5 (o caso do bug) e a mana com fração acima de 0,5.
	var p *entity
	var peer *fakePeer
	id := uint32(1)
	for con := 0; con <= 12 && p == nil; con++ {
		for spi := 0; con+spi <= 12 && p == nil; spi++ {
			a := game.Attrs{Con: con, Spi: spi}
			peer = &fakePeer{}
			z.handle(joinEvent{info: JoinInfo{ID: id, Name: "Ana", Class: "knight", X: 30, Z: 0,
				XP: game.XPLevelStart(5), Attrs: a, Peer: peer}})
			c := z.players[id]
			if fh, fm := frac(c.stats.MaxHP), frac(c.stats.MaxMP); fh > 0.01 && fh < 0.49 && fm > 0.51 && fm < 0.99 {
				p = c
				break
			}
			z.handle(leaveEvent{id: id, peer: peer})
			id++
		}
	}
	if p == nil {
		t.Fatal("nenhuma combinação de pontos deu vida e mana máximas fracionárias")
	}
	if p.hp != p.stats.MaxHP || p.mp != p.stats.MaxMP {
		t.Fatalf("entrou sem vida/mana cheias: %v/%v %v/%v", p.hp, p.stats.MaxHP, p.mp, p.stats.MaxMP)
	}
	maxHP, maxMP := uint32(math.Round(p.stats.MaxHP)), uint32(math.Round(p.stats.MaxMP))

	// SelfStats.
	z.sendSelfStats(p)
	var ss *pb.SelfStats
	for _, m := range peer.msgs {
		if s := m.GetSelfStats(); s != nil {
			ss = s
		}
	}
	if ss == nil || ss.Hp != maxHP || ss.MaxHp != maxHP || ss.Mp != maxMP || ss.MaxMp != maxMP {
		t.Fatalf("SelfStats = %d / %d, %d / %d; quer %d / %d, %d / %d", ss.GetHp(), ss.GetMaxHp(), ss.GetMp(), ss.GetMaxMp(), maxHP, maxHP, maxMP, maxMP)
	}

	// Entity (Spawn e Welcome).
	if e := z.entityPB(p); e.Hp != maxHP || e.MaxHp != maxHP {
		t.Fatalf("Entity = %d / %d; quer %d", e.Hp, e.MaxHp, maxHP)
	}

	// PartyMember.
	b, _ := joinAt(z, 100, "mage", 31, 0, 0)
	z.partyJoin(p, b)
	for _, m := range z.partyPB(p.pl.party).Members {
		if m.Id == p.id && (m.Hp != maxHP || m.MaxHp != maxHP || m.Mp != maxMP || m.MaxMp != maxMP) {
			t.Fatalf("PartyMember = %d / %d, %d / %d; quer %d, %d", m.Hp, m.MaxHp, m.Mp, m.MaxMp, maxHP, maxMP)
		}
	}

	// HpUpdate (snapshot): perde vida e volta a ficar cheio.
	p.hp = 10
	z.update(0.05)
	p.hp = p.stats.MaxHP
	peer.msgs = nil
	z.update(0.05)
	var hu *pb.HpUpdate
	for _, m := range peer.msgs {
		for _, h := range m.GetSnapshot().GetHp() {
			if h.Id == p.id {
				hu = h
			}
		}
	}
	if hu == nil || hu.Hp != maxHP || hu.MaxHp != maxHP {
		t.Fatalf("HpUpdate = %v; quer %d / %d", hu, maxHP, maxHP)
	}

	// Salvamento: cheio é salvo como cheio.
	if s := z.savedState(p); s.HP != -1 || s.MP != -1 {
		t.Fatalf("salvo com vida/mana %d/%d, quer cheio (-1)", s.HP, s.MP)
	}

	// Revive.
	z.kill(p, nil)
	z.respawnPlayer(p)
	if len(z.frame.revives) != 1 {
		t.Fatalf("revives = %v", z.frame.revives)
	}
	if r := z.frame.revives[0]; r.Hp != maxHP || r.MaxHp != maxHP {
		t.Fatalf("Revive = %d / %d; quer %d", r.Hp, r.MaxHp, maxHP)
	}
	z.update(0.05)

	// LevelUp: o máximo anunciado é o mesmo que o SelfStats mostra, e a vida está cheia.
	z.grantXP(p, game.XPLevelStart(p.level+1)-p.pl.xp)
	if len(z.frame.levelUps) != 1 {
		t.Fatalf("levelUps = %v", z.frame.levelUps)
	}
	lu := z.frame.levelUps[0]
	if s := z.selfStats(p); lu.MaxHp != s.MaxHp || s.Hp != s.MaxHp || s.Mp != s.MaxMp {
		t.Fatalf("LevelUp.MaxHp = %d, SelfStats = %d / %d, %d / %d", lu.MaxHp, s.Hp, s.MaxHp, s.Mp, s.MaxMp)
	}
}

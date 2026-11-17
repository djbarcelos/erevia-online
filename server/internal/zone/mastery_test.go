package zone

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// ---------------------------------------------------------------------------------------
// Maestria (nível máximo, 40)
// ---------------------------------------------------------------------------------------

const bar = game.MasteryPointXP

// at30 entra um personagem no nível máximo (o nome é de quando ele era 30), sem bônus
// diário (as contas ficam simples).
func at30(z *Zone, id uint32, class string, x, zz float64) (*entity, *fakePeer) {
	p, peer := joinAt(z, id, class, x, zz, game.XPLevelStart(game.MasteryLevel))
	p.pl.mastery.st.Rested = 0
	return p, peer
}

// setTree põe os pontos pedidos na árvore (com pontos de sobra) e aplica os efeitos.
func setTree(z *Zone, p *entity, ranks map[string]int) {
	p.pl.mastery.st.Points = game.MasteryMaxPoints
	p.pl.mastery.st.Ranks = ranks
	z.masteryChanged(p)
}

func fullTree() map[string]int {
	out := map[string]int{}
	for _, n := range game.MasteryNodes {
		out[n.ID] = game.MasteryNodeMax
	}
	return out
}

func masteryCmd(z *Zone, p *entity, kind pb.MasteryCmdKind, node string) {
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(progressEvent{id: p.id, msg: &pb.MasteryCmd{Kind: kind, Node: node}})
}

// lastMastery: o último MasteryState recebido.
func (f *fakePeer) lastMastery() *pb.MasteryState {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if m := f.msgs[i].GetMastery(); m != nil {
			return m
		}
	}
	return nil
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestMasteryBarFillsFromMonsterXPWithGroupAndDoubleXP(t *testing.T) {
	z := newCombatZone(fixed("stone_golem", game.MasteryLevel, 30, 0))
	z.SetEventSink(nopSink{})
	m := z.onlyMonster()
	p, peer := at30(z, 1, "knight", 26, 0)
	start := p.pl.xp

	// Ao entrar no 30 o HUD recebe a barra (com o bônus do primeiro dia).
	if s := peer.lastMastery(); s == nil || !s.Active || s.Need != uint64(bar) || s.Rested != uint64(bar) || s.RestedMult != 4 {
		t.Fatalf("estado ao entrar: %+v", s)
	}

	solo := game.MonsterXP(m.mon.def, m.level, game.MasteryLevel)
	z.shareMonsterXP(m, []*entity{p})
	if p.pl.mastery.st.XP != solo || p.pl.xp != start {
		t.Fatalf("solo: barra %d (esperava %d), XP do nível %d → %d", p.pl.mastery.st.XP, solo, start, p.pl.xp)
	}

	// Grupo de 2 no 30: +10% e a divisão pelo nível, como sempre.
	q, _ := at30(z, 2, "mage", 27, 0)
	z.shareMonsterXP(m, []*entity{p, q})
	each := int64(math.Round(float64(solo) * 1.1 / 2))
	if q.pl.mastery.st.XP != each || p.pl.mastery.st.XP != solo+each {
		t.Fatalf("grupo: %d e %d, esperava %d cada", p.pl.mastery.st.XP-solo, q.pl.mastery.st.XP, each)
	}

	// XP em dobro também vale na barra.
	z.xp2.on = true
	before := q.pl.mastery.st.XP
	z.shareMonsterXP(m, []*entity{q})
	if got := q.pl.mastery.st.XP - before; got != 2*solo {
		t.Fatalf("XP em dobro: %d, esperava %d", got, 2*solo)
	}
	z.xp2.on = false

	// Vários pontos de uma vez: o resto fica na barra, o chat avisa e a telemetria anota.
	drainEvents(z)
	peer.reset()
	had := p.pl.mastery.st.XP
	z.grantXP(p, 3*bar)
	z.tickFor(0.1)
	st := p.pl.mastery.st
	if st.Points != 3 || st.XP != had || st.Free() != 3 {
		t.Fatalf("3 barras: %+v", st)
	}
	if !peer.systemContains("Você ganhou 3 pontos de Maestria! Agora é Maestria 3.") {
		t.Fatal("o chat avisa o ponto ganho")
	}
	if s := peer.lastMastery(); s == nil || s.Level != 3 || s.Free != 3 || s.Xp != uint64(had) {
		t.Fatalf("o HUD recebe a barra nova: %+v", s)
	}
	ev := findEvent(drainEvents(z), "mastery", "point")
	if ev == nil || ev.Amount != 3 || ev.Extra["gained"] != 3 || ev.Level != game.MasteryLevel {
		t.Fatalf("telemetria: %+v", ev)
	}
	if p.pl.saveAt == 0 {
		t.Fatal("ponto ganho salva logo")
	}
}

func TestMasteryOpensAtMaxLevelAndKeepsTheOverflow(t *testing.T) {
	z := newCombatZone()
	top := game.MasteryLevel
	p, peer := joinAt(z, 1, "rogue", 0, 0, game.XPLevelStart(top-1))
	if peer.lastMastery() != nil {
		t.Fatal("abaixo do 40 a barra de Maestria não vem")
	}
	z.grantXP(p, game.XPToNext(top-1)+1000)
	z.tickFor(0.1)
	if p.level != top || p.pl.xp != game.XPLevelStart(top) {
		t.Fatalf("nível %d, XP %d", p.level, p.pl.xp)
	}
	// Os 1.000 que passaram do nível entram na barra, já com o bônus do primeiro dia (4×).
	st := p.pl.mastery.st
	if st.XP != 4000 || st.Rested != bar-4000 {
		t.Fatalf("sobra do nível na barra: %+v", st)
	}
	if !peer.systemContains("a XP enche a barra de Maestria") {
		t.Fatal("o jogador fica sabendo que a Maestria abriu")
	}
	// Missão e evento usam o mesmo caminho (grantXP); a recompensa da invasão no 30 é
	// calculada em cima da barra (antes era zero).
	if game.XPToNext(top) != 0 {
		t.Fatal("no 40 não há próximo nível")
	}
}

func TestMasteryDailyBonusAndDayChange(t *testing.T) {
	brt := time.FixedZone("BRT", -3*3600)
	now := time.Date(2026, 11, 13, 23, 50, 0, 0, brt)
	z := newClockZone(&now)
	p, peer := joinAt(z, 1, "knight", 0, 0, game.XPLevelStart(game.MasteryLevel))
	st := &p.pl.mastery.st
	if st.Rested != bar || st.Day != game.MasteryDay(now, brt) {
		t.Fatalf("primeiro dia: %+v", st)
	}
	// A primeira barra do dia: 1/4 da XP.
	z.grantXP(p, (bar+3)/4) // arredonda para cima: o bônus cobre a barra inteira
	if st.Points != 1 || st.Rested != 0 {
		t.Fatalf("barra 4× mais rápida: %+v", st)
	}
	z.grantXP(p, 100)
	if st.XP != 100 {
		t.Fatalf("depois do bônus, XP normal: %+v", st)
	}
	// Meia-noite de Brasília: mais um dia de bônus.
	now = now.Add(15 * time.Minute)
	z.grantXP(p, 100)
	if st.XP != 100+400 || st.Rested != bar-400 {
		t.Fatalf("dia novo: %+v", st)
	}
	z.tickFor(0.1)
	if !peer.systemContains("bônus diário") {
		t.Fatal("o chat mostra o bônus diário")
	}
	// Dez dias fora: guarda só 7.
	now = now.Add(10 * 24 * time.Hour)
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_GET, "")
	if s := peer.lastMastery(); st.Rested != game.RestedCap() || s.Rested != uint64(game.RestedCap()) || s.RestedDays != 7 {
		t.Fatalf("acúmulo até 7 dias: %+v / %+v", st, s)
	}
}

func TestMasteryDeathTakesBarNotPoints(t *testing.T) {
	z := newCombatZone(fixed("stone_golem", game.MasteryLevel, 30, 0))
	golem := z.onlyMonster()
	p, peer := at30(z, 1, "knight", 26, 0)
	st := &p.pl.mastery.st
	const into = bar / 2
	st.Points, st.XP = 2, into
	loss := int64(math.Round(float64(bar) * game.DeathXPLossLate))

	z.kill(p, golem)
	if st.XP != into-loss || st.Points != 2 || p.pl.xp != game.XPLevelStart(game.MasteryLevel) {
		t.Fatalf("morte no 40: %+v (perda %d)", st, loss)
	}
	if !peer.systemContains("XP da barra de Maestria") {
		t.Fatal("a mensagem de morte fala da barra")
	}
	z.respawnPlayer(p)

	// Segunda Chance cheia: metade.
	st.XP = into
	setTree(z, p, map[string]int{"second_chance": 5})
	z.kill(p, golem)
	if want := into - int64(math.Round(float64(bar)*game.DeathXPLossLate*0.5)); st.XP != want {
		t.Fatalf("com Segunda Chance: %d, esperava %d", st.XP, want)
	}
	z.respawnPlayer(p)

	// Karma: o dobro. Barra quase vazia: para no zero, sem tirar ponto.
	st.XP, p.pl.karma = 1000, 50
	z.kill(p, golem)
	if st.XP != 0 || st.Points != game.MasteryMaxPoints {
		t.Fatalf("barra não fica negativa: %+v", st)
	}
	z.respawnPlayer(p)

	// Morrer para outro jogador não custa nada.
	p.pl.karma = 0
	st.XP = 50_000
	other, _ := at30(z, 2, "rogue", 27, 0)
	z.kill(p, other)
	if st.XP != 50_000 {
		t.Fatalf("PvP não custa XP: %d", st.XP)
	}
}

func TestMasterySpendCommand(t *testing.T) {
	z := newCombatZone()
	low, lowPeer := joinAt(z, 1, "mage", 0, 0, game.XPLevelStart(12))
	masteryCmd(z, low, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "field_alchemy")
	if len(low.pl.mastery.st.Ranks) != 0 || !lowPeer.systemContains("abre no nível 40") {
		t.Fatal("abaixo do 40 não gasta")
	}
	masteryCmd(z, low, pb.MasteryCmdKind_MASTERY_CMD_RESPEC, "")
	// Pedir o estado funciona para todos (a árvore aparece para consulta).
	lowPeer.reset()
	masteryCmd(z, low, pb.MasteryCmdKind_MASTERY_CMD_GET, "")
	if s := lowPeer.lastMastery(); s == nil || s.Active || !lowPeer.has(func(m *pb.ServerMessage) bool {
		tr := m.GetMasteryTree()
		return tr != nil && len(tr.Nodes) == len(game.MasteryNodes) && len(tr.Branches) == 3 && len(tr.Nodes[0].Ranks) == 5 && tr.CombatCap == float32(game.MasteryCombatCap)
	}) {
		t.Fatalf("GET manda a árvore e o estado: %+v", s)
	}

	p, peer := at30(z, 2, "knight", 0, 0)
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "field_alchemy")
	if !peer.systemContains("Sem pontos de Maestria") {
		t.Fatal("pontos insuficientes")
	}
	p.pl.mastery.st.Points = 6
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "inventado")
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "keen_eye")
	if !peer.systemContains("não existe") || !peer.systemContains("pede 5 pontos em Caça") {
		t.Fatal("nó inválido e pré-requisito")
	}
	for range 6 {
		masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "field_alchemy")
	}
	if p.pl.mastery.st.Ranks["field_alchemy"] != 5 || !peer.systemContains("já está no máximo") {
		t.Fatalf("teto do nó: %+v", p.pl.mastery.st.Ranks)
	}
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "keen_eye")
	s := peer.lastMastery()
	if p.pl.mastery.st.Ranks["keen_eye"] != 1 || s.Free != 0 || len(s.Ranks) != 2 || s.Ranks[0].Node != "field_alchemy" || s.Ranks[0].Rank != 5 {
		t.Fatalf("depois de gastar: %+v", s)
	}
	if !near(p.pl.mastery.of(game.MStatPotion), 0.2) || p.pl.saveAt == 0 {
		t.Fatal("o efeito vale na hora e o personagem é salvo logo")
	}
}

func TestMasteryRespecOncePerDay(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	z := newClockZone(&now)
	p, peer := at30(z, 1, "barbarian", 0, 0)
	base := p.stats.MaxHP
	p.pl.mastery.st.Points = 15
	for range 5 {
		masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "steadfast")
		masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "second_chance")
	}
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "tanned_hide")
	if p.stats.MaxHP <= base || p.pl.mastery.st.Free() != 4 {
		t.Fatalf("Couro Curtido: vida %.1f → %.1f, livres %d", base, p.stats.MaxHP, p.pl.mastery.st.Free())
	}
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_RESPEC, "")
	if p.pl.mastery.st.Spent() != 0 || p.pl.mastery.st.Free() != 15 || !near(p.stats.MaxHP, base) || peer.lastMastery().RespecReady {
		t.Fatalf("redistribuir devolve os pontos e tira os efeitos: %+v, vida %.1f", p.pl.mastery.st, p.stats.MaxHP)
	}
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "wayfarer")
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_RESPEC, "")
	if p.pl.mastery.st.Spent() != 1 || !peer.systemContains("já redistribuiu a Maestria hoje") {
		t.Fatal("só 1 vez por dia")
	}
	now = now.Add(24 * time.Hour)
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_RESPEC, "")
	if p.pl.mastery.st.Spent() != 0 {
		t.Fatal("no dia seguinte libera de novo")
	}
}

func TestMasteryEffectsApply(t *testing.T) {
	z := newCombatZone()
	p, _ := at30(z, 1, "knight", 30, 0)

	t.Run("poções", func(t *testing.T) {
		setTree(z, p, map[string]int{"field_alchemy": 5})
		potion := item("potion_hp_s")
		p.pl.inv.add(potion, 1)
		p.hp = 10
		z.drink(p, p.pl.inv.slotOf("potion_hp_s"), potion)
		if !near(p.hp, 10+potion.HealHP*1.2) {
			t.Fatalf("poção com +20%%: vida %.1f", p.hp)
		}
	})
	t.Run("moedas e material", func(t *testing.T) {
		setTree(z, p, map[string]int{"full_purse": 5, "keen_eye": 5, "field_alchemy": 5})
		gold := []game.Drop{{Item: game.Items[game.GoldID], Count: 100}}
		z.masteryLoot(p.id, game.Monsters["stone_golem"], gold)
		if gold[0].Count != 115 {
			t.Fatalf("Bolsa Farta: %d moedas, esperava 115", gold[0].Count)
		}
		extra := 0
		for range 4000 {
			d := []game.Drop{{Item: game.Items["rune_core"], Count: 1}}
			z.masteryLoot(p.id, game.Monsters["stone_golem"], d)
			extra += d[0].Count - 1
		}
		if f := float64(extra) / 4000; f < 0.12 || f > 0.18 {
			t.Fatalf("Olho do Coletor: %.3f de chance, esperava ~0,15", f)
		}
		for range 200 {
			d := []game.Drop{{Item: game.Items["broken_crown"], Count: 1}}
			z.masteryLoot(p.id, game.Monsters["bone_lord"], d)
			if d[0].Count != 1 {
				t.Fatal("chefe não dá material extra")
			}
		}
		other := []game.Drop{{Item: game.Items[game.GoldID], Count: 100}}
		z.masteryLoot(999, game.Monsters["stone_golem"], other)
		if other[0].Count != 100 {
			t.Fatal("só quem leva o saque e tem o nó")
		}
	})
	t.Run("atordoamento e lentidão", func(t *testing.T) {
		setTree(z, p, map[string]int{"steadfast": 5})
		z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectStun, Duration: 2})
		z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectSlow, Duration: 4, Value: 0.3})
		z.applyEffect(p, p, &game.EffectDef{Kind: game.EffectRoot, Duration: 2})
		for _, ef := range p.effects {
			want := map[game.EffectKind]float64{game.EffectStun: 1.5, game.EffectSlow: 3, game.EffectRoot: 2}[ef.kind]
			if !near(ef.until-z.now, want) {
				t.Fatalf("efeito %d dura %.2f, esperava %.2f", ef.kind, ef.until-z.now, want)
			}
		}
		p.effects = nil
	})
	t.Run("regeneração e velocidade fora de combate", func(t *testing.T) {
		setTree(z, p, nil)
		p.lastCombat = -1e9
		p.hp, p.mp = 1, 1
		z.regen(p, 1)
		hp0, mp0 := p.hp-1, p.mp-1
		speed0 := p.speed(z.now)
		setTree(z, p, map[string]int{"steadfast": 5, "second_wind": 5, "light_step": 5})
		p.hp, p.mp = 1, 1
		z.regen(p, 1)
		if !near(p.hp-1, hp0*1.4) || !near(p.mp-1, mp0*1.4) {
			t.Fatalf("Fôlego: vida %.2f → %.2f, mana %.2f → %.2f", hp0, p.hp-1, mp0, p.mp-1)
		}
		if !near(p.speed(z.now), speed0*1.04) {
			t.Fatalf("Passo Leve: %.3f → %.3f", speed0, p.speed(z.now))
		}
		p.lastCombat = z.now
		if !near(p.speed(z.now), speed0) {
			t.Fatal("em combate a velocidade é a normal")
		}
		p.hp, p.mp = 1, 1
		z.regen(p, 1)
		if p.hp-1 >= hp0 {
			t.Fatal("em combate o Fôlego não vale")
		}
		p.lastCombat = -1e9
	})
	t.Run("viagem", func(t *testing.T) {
		setTree(z, p, map[string]int{"wayfarer": 5, "light_step": 5, "quick_read": 5, "way_home": 5})
		if z.teleportPrice(p, 100) != 70 || z.teleportPrice(p, 20) != 14 {
			t.Fatalf("Viajante: %d e %d", z.teleportPrice(p, 100), z.teleportPrice(p, 20))
		}
		if s := z.masteryPB(p); !near(float64(s.TeleportDiscount), 0.3) {
			t.Fatalf("o cliente recebe o desconto: %v", s.TeleportDiscount)
		}
		if !near(z.channelTime(p, 2), 1.2) || !near(z.itemCooldown(p, game.CooldownReturn), game.ReturnCooldown/2) ||
			!near(z.itemCooldown(p, game.CooldownHP), game.CooldownSeconds(game.CooldownHP)) {
			t.Fatal("Leitura Rápida e Caminho de Casa")
		}
	})
	t.Run("dano e vida", func(t *testing.T) {
		setTree(z, p, nil)
		s0 := p.stats
		setTree(z, p, map[string]int{"hunter_instinct": 5, "tanned_hide": 5})
		if !near(p.stats.PAtk, s0.PAtk*1.02) || !near(p.stats.MAtk, s0.MAtk*1.02) || !near(p.stats.MaxHP, s0.MaxHP*1.015) {
			t.Fatalf("Instinto e Couro: atk %.1f → %.1f, vida %.1f → %.1f", s0.PAtk, p.stats.PAtk, s0.MaxHP, p.stats.MaxHP)
		}
	})
}

// O teto de combate nos atributos de verdade: com a árvore inteira, a soma do que muda em
// dano, defesa, vida, crítico e velocidade de ataque não passa de 5%, em todas as classes.
func TestMasteryCombatCapOnRealStats(t *testing.T) {
	z := newCombatZone()
	for i, class := range []string{"knight", "barbarian", "mage", "rogue"} {
		p, _ := at30(z, uint32(i+1), class, 0, 0)
		setTree(z, p, nil)
		s0 := p.stats
		setTree(z, p, fullTree())
		s1 := p.stats
		rel := func(a, b float64) float64 {
			if a == 0 {
				return 0
			}
			return math.Abs(b/a - 1)
		}
		sum := math.Max(rel(s0.PAtk, s1.PAtk), rel(s0.MAtk, s1.MAtk)) + // dano
			math.Max(rel(s0.PDef, s1.PDef), rel(s0.MDef, s1.MDef)) + // defesa
			rel(s0.MaxHP, s1.MaxHP) + // vida
			rel(s0.Crit, s1.Crit) + rel(s0.CritDamage, s1.CritDamage) + // crítico
			rel(s0.AtkInterval, s1.AtkInterval) // velocidade de ataque
		if sum > game.MasteryCombatCap+1e-9 || sum == 0 {
			t.Fatalf("%s: a árvore inteira muda %.2f%% do combate (teto %.0f%%)", class, sum*100, game.MasteryCombatCap*100)
		}
	}
}

func TestMasterySaveAndLoad(t *testing.T) {
	saves := make(chan []SavedState, 8)
	z := newTestZone(saves)
	peer := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Mestre", Class: "knight", XP: game.XPLevelStart(game.MasteryLevel), Peer: peer}})
	p := z.players[1]
	base := p.stats.MaxHP
	st := &p.pl.mastery.st
	st.Points, st.XP = 7, 1234
	for range 5 {
		st.Spend("second_chance")
	}
	st.Spend("tanned_hide") // fechado (pede 10): não entra
	st.Spend("steadfast")
	z.masteryChanged(p)

	saved := z.savedState(p)
	st.Ranks["steadfast"] = 4 // mexer depois não muda o que foi para o banco (outra goroutine)
	if saved.Mastery.Points != 7 || saved.Mastery.XP != 1234 || saved.Mastery.Ranks["second_chance"] != 5 || saved.Mastery.Ranks["steadfast"] != 1 {
		t.Fatalf("salvo: %+v (o mapa salvo tem de ser uma cópia)", saved.Mastery)
	}
	st.Ranks["steadfast"] = 1

	// Carregar do banco: o estado volta, os efeitos valem e a árvore inválida é corrigida.
	z2 := newTestZone(nil)
	peer2 := &fakePeer{}
	z2.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Mestre", Class: "knight", XP: game.XPLevelStart(game.MasteryLevel), Mastery: saved.Mastery, Peer: peer2}})
	q := z2.players[1]
	if q.pl.mastery.st.Points != 7 || q.pl.mastery.st.Spent() != 6 || !near(q.pl.mastery.of(game.MStatDeathLoss), 0.5) {
		t.Fatalf("carregado: %+v", q.pl.mastery.st)
	}
	if s := peer2.lastMastery(); s == nil || s.Level != 7 || s.Free != 1 || s.Xp != 1234 {
		t.Fatalf("o HUD recebe o estado carregado: %+v", s)
	}
	bad := saved.Mastery
	bad.Points = 2 // gastou mais do que tem: os pontos voltam
	z3 := newTestZone(nil)
	z3.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Mestre", Class: "knight", XP: game.XPLevelStart(game.MasteryLevel), Mastery: bad, Peer: &fakePeer{}}})
	if r := z3.players[1].pl.mastery; r.st.Spent() != 0 || r.of(game.MStatDeathLoss) != 0 || !near(z3.players[1].stats.MaxHP, base) {
		t.Fatalf("árvore inválida no banco: %+v", r.st)
	}

	// Relogar rápido usa o estado da memória.
	z.removePlayer(1)
	z.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Mestre", Class: "knight", XP: game.XPLevelStart(game.MasteryLevel), Peer: &fakePeer{}}})
	if r := z.players[1].pl.mastery.st; r.Points != 7 || r.XP != 1234 || r.Ranks["second_chance"] != 5 {
		t.Fatalf("relogin: %+v", r)
	}
}

func TestMasteryGMCommand(t *testing.T) {
	z := newTownZone()
	gm, gmPeer := defender(z, 1, "Guia", 0, 0, game.MasteryLevel, true)
	ana, anaPeer := defender(z, 2, "Ana", 1, 0, game.MasteryLevel, false)
	gm.pl.mastery.st.Rested = 0

	say(z, 2, "/maestria xp 999999")
	if ana.pl.mastery.st.XP != 0 || !anaPeer.systemContains("Comando desconhecido") {
		t.Fatal("/maestria é só de GM")
	}
	say(z, 1, fmt.Sprint("/maestria xp ", bar+10))
	if gm.pl.mastery.st.Points != 1 || gm.pl.mastery.st.XP != 10 {
		t.Fatalf("/maestria xp: %+v", gm.pl.mastery.st)
	}
	say(z, 1, "/maestria xp 100 Ana")
	if ana.pl.mastery.st.XP != 400 { // Ana ainda tem o bônus do dia
		t.Fatalf("/maestria xp Nome: %+v", ana.pl.mastery.st)
	}
	ana.pl.mastery.st.Rested, ana.pl.mastery.st.RespecDay = 0, z.masteryDay()
	say(z, 1, "/maestria dia Ana")
	if ana.pl.mastery.st.Rested != bar || !ana.pl.mastery.st.CanRespec(z.masteryDay()) {
		t.Fatalf("/maestria dia: %+v", ana.pl.mastery.st)
	}
	say(z, 1, "/maestria")
	if !gmPeer.systemContains("Uso: /maestria") {
		t.Fatal("uso")
	}
}

func TestRankingShowsMastery(t *testing.T) {
	z := newCombatZone()
	z.setRanking(RankingData{Level: []RankRow{{CharacterID: 1, Name: "Dono", Class: "knight", Level: 30, Mastery: 12}}})
	if e := z.ranking.level[0]; e.Mastery != 12 || e.Level != 30 {
		t.Fatalf("ranking: %+v", e)
	}
	if !strings.Contains(game.MasteryNodeByID("light_step").Text(5), "4%") {
		t.Fatal("o Passo Leve mostra o teto")
	}
}

func TestMasteryTravelChargesLessAndIsFaster(t *testing.T) {
	z := newCombatZone()
	town := z.stoneAt(0)
	p, peer := at30(z, 1, "knight", town.pos.X+1, town.pos.Z)
	setTree(z, p, map[string]int{"wayfarer": 5, "light_step": 5, "quick_read": 5, "way_home": 5})
	p.pl.inv.gold = 100

	teleportReq(z, p, town, 1) // Arredores: 40 moedas → 28
	if p.pl.channel == nil || !peer.has(func(m *pb.ServerMessage) bool {
		b := m.GetChannelBar()
		return b != nil && b.Kind == pb.ChannelKind_CHANNEL_KIND_TELEPORT && b.TotalMs == 1200
	}) {
		t.Fatal("Leitura Rápida: a pedra canaliza 1,2 s")
	}
	z.tickFor(1.3)
	if p.pl.channel != nil || p.pl.inv.gold != 72 {
		t.Fatalf("Viajante: pagou %d (esperava 28)", 100-p.pl.inv.gold)
	}

	p.pl.inv.add(item("scroll_return"), 2)
	z.handle(itemEvent{id: p.id, msg: &pb.UseItem{Place: placeBag, Slot: uint32(p.pl.inv.slotOf("scroll_return"))}})
	z.tickFor(game.ReturnChannel*0.6 + 0.1)
	if !z.inSafe(p.pos) {
		t.Fatal("o pergaminho leva à cidade em 3 s")
	}
	key := uint32(itemCooldownBase + game.CooldownReturn)
	if left := p.pl.cooldowns[key] - z.now; left > game.ReturnCooldown/2 || left < game.ReturnCooldown/2-1 {
		t.Fatalf("Caminho de Casa: recarga de %.1f s", left)
	}
	if !peer.has(func(m *pb.ServerMessage) bool {
		c := m.GetItemCooldown()
		return c != nil && c.Group == game.CooldownReturn && c.TotalMs == uint32(game.ReturnCooldown/2*1000)
	}) {
		t.Fatal("o cliente recebe a recarga menor")
	}
}

// Quem ganhou pontos quando o máximo era 30 fica com eles: do 30 ao 39 recebe o estado ao
// entrar, gasta e redistribui, e os efeitos valem; mas a XP sobe nível, sem ponto novo.
func TestMasteryOldPointsBelowMaxLevel(t *testing.T) {
	z := newCombatZone()
	peer := &fakePeer{}
	old := game.MasteryState{Points: 6, XP: 1234, Ranks: map[string]int{"wayfarer": 2}}
	z.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Veterana", Class: "knight", XP: game.XPLevelStart(30), Mastery: old, Peer: peer}})
	p := z.players[1]
	s := peer.lastMastery()
	if p.level != 30 || s == nil || s.Active || s.Level != 6 || s.Free != 4 || s.OpenLevel != 40 {
		t.Fatalf("estado ao entrar no 30 com pontos de antes: %+v", s)
	}
	if p.pl.mastery.of(game.MStatTeleport) <= 0 {
		t.Fatal("os nós gastos continuam valendo")
	}
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "light_step")
	if p.pl.mastery.st.Ranks["light_step"] != 1 {
		t.Fatalf("gasta os pontos que já tinha: %+v", p.pl.mastery.st)
	}
	masteryCmd(z, p, pb.MasteryCmdKind_MASTERY_CMD_RESPEC, "")
	if p.pl.mastery.st.Spent() != 0 || p.pl.mastery.st.Points != 6 {
		t.Fatalf("redistribui: %+v", p.pl.mastery.st)
	}
	// A XP sobe nível e não enche a barra (nem com uma barra inteira de XP).
	z.grantXP(p, bar)
	if st := p.pl.mastery.st; st.Points != 6 || st.XP != 1234 || p.level <= 30 {
		t.Fatalf("entre o 30 e o 39 não há ponto novo: nível %d, %+v", p.level, st)
	}
	// Sem pontos de antes, abaixo do 40 a árvore não abre.
	q, qPeer := joinAt(z, 2, "mage", 0, 0, game.XPLevelStart(35))
	if qPeer.lastMastery() != nil {
		t.Fatal("sem pontos, a Maestria não vem antes do 40")
	}
	masteryCmd(z, q, pb.MasteryCmdKind_MASTERY_CMD_SPEND, "light_step")
	if len(q.pl.mastery.st.Ranks) != 0 {
		t.Fatal("sem pontos não gasta")
	}
}

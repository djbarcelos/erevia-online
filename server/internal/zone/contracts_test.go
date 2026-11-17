package zone

import (
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// ---------------------------------------------------------------------------------------
// Contratos do quadro e reputação (tunnel#157)
// ---------------------------------------------------------------------------------------

func (f *fakePeer) lastContracts() *pb.ContractState {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if m := f.msgs[i].GetContracts(); m != nil {
			return m
		}
	}
	return nil
}

func (f *fakePeer) lastReputation() *pb.ReputationState {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if m := f.msgs[i].GetReputation(); m != nil {
			return m
		}
	}
	return nil
}

func contractInfo(s *pb.ContractState, id string) *pb.ContractInfo {
	for _, c := range s.GetContracts() {
		if c.Id == id {
			return c
		}
	}
	return nil
}

// charWith: um id de personagem que tem o contrato no quadro no dia do relógio da zona.
func charWith(t *testing.T, z *Zone, contract string) uint32 {
	t.Helper()
	day := game.ContractDay(z.cfg.Clock(), z.eventLoc())
	for id := uint32(1); id < 500; id++ {
		for _, c := range game.OfferedContracts(id, day) {
			if c.ID == contract {
				return id
			}
		}
	}
	t.Fatalf("ninguém tem %s no dia %d", contract, day)
	return 0
}

// atBoardLevel: entra perto do quadro no nível pedido (sem bônus diário da Maestria).
func atBoardLevel(z *Zone, id uint32, level int, gold int64) (*entity, *fakePeer) {
	b := z.npcs[boardID]
	peer := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: id, Name: "C" + string(rune('A'+id%26)) + strings.Repeat("x", int(id%5)), Class: "knight",
		X: b.pos.X + 1.5, Z: b.pos.Z, XP: game.XPLevelStart(level), Gold: gold, AccountID: int64(id), Peer: peer}})
	p := z.players[id]
	p.pl.mastery.st.Rested = 0
	return p, peer
}

func contractCmd(z *Zone, p *entity, kind pb.ContractCmdKind, id string) {
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(progressEvent{id: p.id, msg: &pb.ContractCmd{Kind: kind, Contract: id, NpcId: boardID}})
}

func repCmd(z *Zone, p *entity, kind pb.ReputationCmdKind, offer string) {
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(progressEvent{id: p.id, msg: &pb.ReputationCmd{Kind: kind, Offer: offer, NpcId: boardID}})
}

func TestContractStateOnJoinHasTheBoard(t *testing.T) {
	z := newTestZone(nil)
	b := z.npcs[boardID]
	if b == nil || b.name != "Iselda" || b.npc.title != "Quadro de Contratos" || len(b.npc.stock) != 0 {
		t.Fatalf("quadro: %+v", b)
	}
	if !z.inSafe(b.pos) {
		t.Fatal("o quadro tem que ficar na cidade")
	}
	_, peer := atBoardLevel(z, 3, 5, 0)
	s := peer.lastContracts()
	if s == nil || s.BoardId != boardID || s.MinLevel != game.ContractMinLevel || len(s.Contracts) != 4 {
		t.Fatalf("ContractState ao entrar: %+v", s)
	}
	for _, c := range s.Contracts {
		if c.Status != pb.ContractStatus_CONTRACT_STATUS_OPEN || c.Need == 0 || c.Reward == "" {
			t.Fatalf("contrato %+v", c)
		}
	}
	if r := peer.lastReputation(); r == nil || len(r.Factions) != 2 || len(r.GradeNames) != 5 || len(r.Offers) != 0 {
		t.Fatalf("ReputationState ao entrar: %+v", r)
	}
	// Renovação: a próxima meia-noite de Brasília depois do relógio da zona.
	now := z.cfg.Clock()
	reset := time.UnixMilli(s.DailyResetMs)
	if !reset.After(now) || reset.Sub(now) > 24*time.Hour || reset.In(brasilia).Hour() != 0 {
		t.Fatalf("renovação em %v (agora %v)", reset, now)
	}
}

func TestContractAcceptRules(t *testing.T) {
	z := newTestZone(nil)
	z.SetEventSink(nopSink{})
	id := charWith(t, z, "hunt_fundo_golems")
	p, peer := atBoardLevel(z, id, game.ContractMinLevel-1, 0)

	// Abaixo do nível: recusado.
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "hunt_fundo_golems")
	if len(p.pl.contracts.st.Taken) != 0 || !peer.systemContains("nível") {
		t.Fatal("aceitou abaixo do nível")
	}
	// No nível, mas longe do quadro: recusado.
	p.level = game.ContractMinLevel
	p.pos = Vec2{X: 30, Z: 30}
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "hunt_fundo_golems")
	if len(p.pl.contracts.st.Taken) != 0 {
		t.Fatal("aceitou longe do quadro")
	}
	p.pos = Vec2{X: z.npcs[boardID].pos.X + 1, Z: z.npcs[boardID].pos.Z}
	// Um contrato que não é dele hoje: recusado.
	other := ""
	for _, c := range game.ContractList {
		if contractInfo(peer.lastContracts(), c.ID) == nil {
			other = c.ID
			break
		}
	}
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, other)
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "nao_existe")
	if len(p.pl.contracts.st.Taken) != 0 {
		t.Fatal("aceitou contrato fora do quadro")
	}
	drainEvents(z)
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "hunt_fundo_golems")
	c := contractInfo(peer.lastContracts(), "hunt_fundo_golems")
	if c == nil || c.Status != pb.ContractStatus_CONTRACT_STATUS_ACTIVE || c.Count != 0 {
		t.Fatalf("depois de aceitar: %+v", c)
	}
	if e := findEvent(drainEvents(z), "contract_accept", "hunt_fundo_golems"); e == nil || e.CharacterID != int64(id) {
		t.Fatal("sem telemetria contract_accept")
	}
	// Aceitar de novo não zera o progresso.
	p.pl.contracts.st.Taken["hunt_fundo_golems"].N = 7
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "hunt_fundo_golems")
	if p.pl.contracts.st.Taken["hunt_fundo_golems"].N != 7 {
		t.Fatal("aceitar de novo zerou o progresso")
	}
}

// Caça: só conta Golem Ancestral do Fundo do Vale (onde nasceu), aceito, com XP; ao completar, a recompensa
// sai (moedas, Maestria e reputação), com telemetria e o aviso do HUD.
func TestContractHuntProgressAndReward(t *testing.T) {
	z := newCombatZone(fixed("ancient_golem", 35, 30, 0))
	z.SetEventSink(nopSink{})
	m := z.onlyMonster()
	m.mon.home = Vec2{X: 0, Z: -300} // nasceu no Fundo do Vale
	id := charWith(t, z, "hunt_fundo_golems")
	p, peer := atBoardLevel(z, id, game.MaxLevel, 100)

	z.shareMonsterXP(m, []*entity{p}) // antes de aceitar: não conta
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "hunt_fundo_golems")
	z.shareMonsterXP(m, []*entity{p})
	if n := p.pl.contracts.st.Taken["hunt_fundo_golems"].N; n != 1 {
		t.Fatalf("progresso %d depois de 1 golem", n)
	}
	// Golem que nasceu fora do Fundo do Vale ou invasor não contam.
	m.mon.home = Vec2{X: 0, Z: 0}
	z.shareMonsterXP(m, []*entity{p})
	m.mon.home = Vec2{X: 0, Z: -300}
	m.mon.invader = true
	z.shareMonsterXP(m, []*entity{p})
	m.mon.invader = false
	if n := p.pl.contracts.st.Taken["hunt_fundo_golems"].N; n != 1 {
		t.Fatalf("contou golem de fora: %d", n)
	}

	def := game.Contracts["hunt_fundo_golems"]
	p.pl.contracts.st.Taken[def.ID].N = def.Count - 1
	gold, rep, mxp := p.pl.inv.gold, p.pl.contracts.rep.Of(def.Faction), p.pl.mastery.st.XP
	drainEvents(z)
	z.shareMonsterXP(m, []*entity{p})
	pr := p.pl.contracts.st.Taken[def.ID]
	if !pr.Done || pr.N != def.Count {
		t.Fatalf("não concluiu: %+v", pr)
	}
	if p.pl.inv.gold != gold+def.Gold {
		t.Fatalf("moedas %d → %d", gold, p.pl.inv.gold)
	}
	if got := p.pl.contracts.rep.Of(def.Faction); got != rep+def.Rep {
		t.Fatalf("reputação %d → %d", rep, got)
	}
	monsterXP := game.MonsterXP(m.mon.def, m.level, p.level)
	if got := p.pl.mastery.st.XP - mxp; got != monsterXP+def.RewardXP(game.MaxLevel) {
		t.Fatalf("Maestria +%d, esperava %d (golem) + %d (contrato)", got, monsterXP, def.RewardXP(game.MaxLevel))
	}
	s := peer.lastContracts()
	if s.Completed != def.Title || contractInfo(s, def.ID).Status != pb.ContractStatus_CONTRACT_STATUS_DONE {
		t.Fatalf("aviso do HUD: %+v", s)
	}
	evs := drainEvents(z)
	if e := findEvent(evs, "contract_done", def.ID); e == nil || e.Amount != def.Gold || e.Extra["rep"] != def.Rep {
		t.Fatalf("telemetria contract_done: %+v", e)
	}
	if e := findEvent(evs, "gold", "contract"); e == nil || e.Amount != def.Gold {
		t.Fatal("o ouro do contrato não foi para a telemetria de ouro")
	}
	z.update(0.05) // o aviso no chat sai depois do snapshot
	if !peer.systemContains("Contrato concluído: " + def.Title) {
		t.Fatal("sem aviso no chat")
	}
	// Concluído não anda mais nem paga de novo.
	z.shareMonsterXP(m, []*entity{p})
	if p.pl.inv.gold != gold+def.Gold {
		t.Fatal("pagou duas vezes")
	}
}

// Nível mínimo do monstro e a área (o disco do Fundo, não o resto do Vale).
func TestContractKillLevelAndArea(t *testing.T) {
	z := newCombatZone(fixed("ancient_golem", 31, 30, 0))
	m := z.onlyMonster()
	id := charWith(t, z, "hunt_fundo_deep")
	p, _ := atBoardLevel(z, id, game.MaxLevel, 0)
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "hunt_fundo_deep")
	m.mon.home = Vec2{X: 4, Z: -298}
	z.shareMonsterXP(m, []*entity{p}) // nível 31: abaixo do mínimo (32)
	m.level = 34
	z.shareMonsterXP(m, []*entity{p})
	m.mon.home = Vec2{X: 0, Z: -250} // o Vale das Pedras Vivas, fora do Fundo
	z.shareMonsterXP(m, []*entity{p})
	if n := p.pl.contracts.st.Taken["hunt_fundo_deep"].N; n != 1 {
		t.Fatalf("progresso %d, esperava 1", n)
	}
}

// Coleta: conta o material pego do chão que caiu de monstro da área; o mesmo item de outro
// lugar (ou que não veio de monstro) não conta.
func TestContractCollectCountsLootPickups(t *testing.T) {
	z := newCombatZone()
	id := charWith(t, z, "collect_fundo_cores")
	p, _ := atBoardLevel(z, id, game.MaxLevel, 0)
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "collect_fundo_cores")
	pick := func(loot bool, from Vec2, n int) {
		g := z.placeItem(p.pos, game.Items["rune_core"], n, 0, 0, 0)
		g.loot, g.lootFrom = loot, from
		p.act = action{kind: actPickup, target: g.id}
		z.runPickup(p)
	}
	pick(true, Vec2{X: 5, Z: -300}, 3)
	pick(false, Vec2{X: 5, Z: -300}, 4)  // não veio de monstro (ex.: caiu de quem morreu com karma)
	pick(true, Vec2{X: -122, Z: 122}, 5) // dos Confins (núcleo rachado): não é do Fundo
	if n := p.pl.contracts.st.Taken["collect_fundo_cores"].N; n != 3 {
		t.Fatalf("coleta %d, esperava 3", n)
	}
	if got := p.pl.inv.countOf("rune_core"); got != 12 {
		t.Fatalf("os núcleos ficam na bolsa: %d", got)
	}
}

// Semanal de PvP: vitória contra jogador roxo ou com karma, de outra conta e do nível
// mínimo para cima; cada personagem conta uma vez.
func TestContractPvPWeekly(t *testing.T) {
	z := newTestZone(nil)
	id := charWith(t, z, "weekly_pvp")
	p, _ := atBoardLevel(z, id, game.MaxLevel, 0)
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "weekly_pvp")
	other := uint32(1)
	if other == id {
		other = 2
	}
	q, _ := atBoardLevel(z, other, game.ContractPvPMinLevel+2, 0)
	z.contractPvP(p, q)
	z.contractPvP(p, q) // o mesmo de novo
	low, _ := atBoardLevel(z, 900, 10, 0)
	z.contractPvP(p, low)
	alt, _ := atBoardLevel(z, 901, 30, 0)
	alt.pl.accountID = p.pl.accountID
	z.contractPvP(p, alt)
	if pr := p.pl.contracts.st.Taken["weekly_pvp"]; pr.N != 1 || len(pr.Seen) != 1 || pr.Seen[0] != q.id {
		t.Fatalf("PvP: %+v", pr)
	}
	// Pelo caminho de verdade: matar quem tem karma conta.
	r, _ := atBoardLevel(z, 902, 30, 0)
	r.pl.karma = 50
	if !z.pvpDeath(r, p) || p.pl.contracts.st.Taken["weekly_pvp"].N != 2 {
		t.Fatalf("vitória sobre quem tem karma não contou: %+v", p.pl.contracts.st.Taken["weekly_pvp"])
	}
	// Matar quem não estava lutando (dá karma) não conta.
	s, _ := atBoardLevel(z, 903, 30, 0)
	z.pvpDeath(s, p)
	if p.pl.contracts.st.Taken["weekly_pvp"].N != 2 {
		t.Fatal("PK contou para o contrato")
	}
}

// Virada do dia em Brasília: o diário aceito antes da meia-noite some depois dela (o
// progresso não passa para o dia seguinte) e o quadro muda.
func TestContractsRenewAtBrasiliaMidnightInZone(t *testing.T) {
	now := time.Date(2026, 11, 14, 2, 59, 0, 0, time.UTC) // 23:59 de sexta em Brasília
	z := newClockZone(&now)
	id := charWith(t, z, "hunt_fundo_golems")
	p, peer := atBoardLevel(z, id, game.MaxLevel, 0)
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "hunt_fundo_golems")
	p.pl.contracts.st.Taken["hunt_fundo_golems"].N = 50
	before := peer.lastContracts()

	now = now.Add(59 * time.Second) // 23:59:59: ainda é sexta
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_GET, "")
	if c := contractInfo(peer.lastContracts(), "hunt_fundo_golems"); c == nil || c.Count != 50 {
		t.Fatal("a renovação veio antes da meia-noite")
	}
	now = now.Add(time.Second) // sábado, 00:00 em Brasília (03:00 UTC)
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_GET, "")
	after := peer.lastContracts()
	if p.pl.contracts.st.Taken["hunt_fundo_golems"] != nil {
		t.Fatal("o diário de ontem continuou aceito")
	}
	for i := 0; i < 3; i++ {
		if after.Contracts[i].Id == before.Contracts[i].Id {
			t.Fatalf("a vaga %d repetiu %s depois da meia-noite", i, after.Contracts[i].Id)
		}
		if after.Contracts[i].Status != pb.ContractStatus_CONTRACT_STATUS_OPEN {
			t.Fatal("contrato novo já aceito")
		}
	}
	if after.DailyResetMs-before.DailyResetMs != 24*3600*1000 {
		t.Fatalf("próxima renovação andou %d ms", after.DailyResetMs-before.DailyResetMs)
	}
}

// Graus: a reputação sobe com os contratos e o grau novo vem no aviso.
func TestContractReputationGradeUp(t *testing.T) {
	z := newTestZone(nil)
	id := charWith(t, z, "weekly_pvp")
	p, peer := atBoardLevel(z, id, game.MaxLevel, 0)
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "weekly_pvp")
	p.pl.contracts.rep.Points = map[string]int{game.FactionGuard: game.RepGradePoints[game.RepHonored] - 10}
	z.finishContract(p, game.Contracts["weekly_pvp"])
	r := peer.lastReputation()
	var guard *pb.FactionInfo
	for _, f := range r.Factions {
		if f.Id == game.FactionGuard {
			guard = f
		}
	}
	if guard == nil || guard.Grade != uint32(game.RepHonored) || guard.GradeNext != uint32(game.RepGradePoints[game.RepRevered]) ||
		!strings.Contains(r.GradeUp, "Honrado") {
		t.Fatalf("grau novo: %+v (aviso %q)", guard, r.GradeUp)
	}
}

// Loja: grau, moedas, bolsa e o título (uma vez só); a compra passa pela telemetria.
func TestRepShopBuy(t *testing.T) {
	z := newTestZone(nil)
	z.SetEventSink(nopSink{})
	p, peer := atBoardLevel(z, 7, game.MaxLevel, 60000)

	repCmd(z, p, pb.ReputationCmdKind_REPUTATION_CMD_GET, "")
	if r := peer.lastReputation(); len(r.Offers) != len(game.RepOffers) {
		t.Fatalf("a loja veio com %d ofertas", len(r.Offers))
	}
	// Neutro: nada.
	repCmd(z, p, pb.ReputationCmdKind_REPUTATION_CMD_BUY, "bastion_hp_l")
	if p.pl.inv.gold != 60000 || p.pl.inv.countOf("potion_hp_l") != 0 {
		t.Fatal("comprou sem o grau")
	}
	p.pl.contracts.rep.Points = map[string]int{game.FactionBastion: game.RepGradePoints[game.RepRevered]}
	drainEvents(z)
	repCmd(z, p, pb.ReputationCmdKind_REPUTATION_CMD_BUY, "bastion_hp_l")
	o := game.RepOfferByID("bastion_hp_l")
	if p.pl.inv.gold != 60000-o.Price || p.pl.inv.countOf("potion_hp_l") != o.Count {
		t.Fatalf("compra: %d moedas, %d poções", p.pl.inv.gold, p.pl.inv.countOf("potion_hp_l"))
	}
	evs := drainEvents(z)
	if e := findEvent(evs, "gold", "rep_shop"); e == nil || e.Amount != -o.Price {
		t.Fatal("sem telemetria do ouro da loja")
	}
	if e := findEvent(evs, "item_in", "potion_hp_l"); e == nil || e.Extra["how"] != "rep_shop" || e.Amount != int64(o.Count) {
		t.Fatal("sem telemetria do item da loja")
	}
	// A capa (Reverenciado) entra na bolsa.
	repCmd(z, p, pb.ReputationCmdKind_REPUTATION_CMD_BUY, "bastion_cape")
	if p.pl.inv.countOf("cape_rep_bastion") != 1 {
		t.Fatal("sem a capa")
	}
	// Exaltado é mais que reverenciado: a oferta continua fechada.
	gold := p.pl.inv.gold
	repCmd(z, p, pb.ReputationCmdKind_REPUTATION_CMD_BUY, "bastion_title_hand")
	if p.pl.inv.gold != gold {
		t.Fatal("comprou oferta de exaltado sendo reverenciado")
	}
	// Título: desbloqueia a conquista (escolhida na Glória) e não vende duas vezes.
	repCmd(z, p, pb.ReputationCmdKind_REPUTATION_CMD_BUY, "bastion_title_watch")
	if _, ok := p.pl.ach.unlocked["rep_bastion_honored"]; !ok || p.pl.inv.gold != gold-game.RepOfferByID("bastion_title_watch").Price {
		t.Fatal("o título não veio")
	}
	repCmd(z, p, pb.ReputationCmdKind_REPUTATION_CMD_BUY, "bastion_title_watch")
	if p.pl.inv.gold != gold-game.RepOfferByID("bastion_title_watch").Price || !peer.systemContains("já tem esse título") {
		t.Fatal("vendeu o título duas vezes")
	}
	z.setTitle(p, "rep_bastion_honored")
	if p.pl.ach.titleText() != "Vigia da Cripta" {
		t.Fatalf("título %q", p.pl.ach.titleText())
	}
	r := peer.lastReputation()
	for _, off := range r.Offers {
		if off.Id == "bastion_title_watch" && !off.Owned {
			t.Fatal("a loja não marcou o título como comprado")
		}
	}
	// Sem moedas e longe do quadro.
	p.pl.inv.gold = 10
	repCmd(z, p, pb.ReputationCmdKind_REPUTATION_CMD_BUY, "bastion_return")
	if p.pl.inv.countOf("scroll_return") != 0 {
		t.Fatal("comprou sem moedas")
	}
	p.pl.inv.gold = 5000
	p.pos = Vec2{X: 30, Z: 30}
	repCmd(z, p, pb.ReputationCmdKind_REPUTATION_CMD_BUY, "bastion_return")
	if p.pl.inv.countOf("scroll_return") != 0 {
		t.Fatal("comprou longe do quadro")
	}
}

// O estado vai e volta pelo salvamento (e pelo z.recent de quem relogou rápido).
func TestContractsSurviveRelog(t *testing.T) {
	z := newTestZone(nil)
	id := charWith(t, z, "collect_fundo_cores")
	p, _ := atBoardLevel(z, id, game.MaxLevel, 0)
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "collect_fundo_cores")
	p.pl.contracts.st.Taken["collect_fundo_cores"].N = 11
	p.pl.contracts.rep.Points = map[string]int{game.FactionBastion: 777}
	st := z.savedState(p)
	if st.Contracts.Taken["collect_fundo_cores"].N != 11 || st.Reputation.Points[game.FactionBastion] != 777 {
		t.Fatalf("salvamento: %+v %+v", st.Contracts, st.Reputation)
	}
	st.Contracts.Taken["collect_fundo_cores"].N = 99 // a cópia é independente
	if p.pl.contracts.st.Taken["collect_fundo_cores"].N != 11 {
		t.Fatal("o salvamento divide o mapa com a zona")
	}
	z.removePlayer(id)
	q, _ := atBoardLevel(z, id, game.MaxLevel, 0)
	if q.pl.contracts.st.Taken["collect_fundo_cores"].N != 11 || q.pl.contracts.rep.Of(game.FactionBastion) != 777 {
		t.Fatalf("relogar perdeu: %+v", q.pl.contracts)
	}
}

func TestGMContractCommands(t *testing.T) {
	z := newTestZone(nil)
	id := charWith(t, z, "hunt_fundo_golems")
	p, peer := atBoardLevel(z, id, game.MaxLevel, 0)
	p.pl.gm = true
	contractCmd(z, p, pb.ContractCmdKind_CONTRACT_CMD_ACCEPT, "hunt_fundo_golems")
	z.command(p, "/contrato andar 1000")
	if !p.pl.contracts.st.Taken["hunt_fundo_golems"].Done || peer.lastContracts().Completed == "" {
		t.Fatal("/contrato andar não concluiu")
	}
	z.command(p, "/contrato dia")
	if p.pl.contracts.st.Taken["hunt_fundo_golems"] != nil {
		t.Fatal("/contrato dia não liberou o diário")
	}
	z.command(p, "/contrato rep guard 1600")
	if p.pl.contracts.rep.Of(game.FactionGuard) != 1600 {
		t.Fatal("/contrato rep")
	}
}

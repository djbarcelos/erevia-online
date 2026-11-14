package zone

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Contratos do nível máximo e reputação com as facções (tunnel#157; dados em
// game/contracts.go e game/reputation.go). O quadro da praça é um NPC: ali o jogador aceita
// os contratos do dia e compra na loja de reputação. O progresso anda com o que o servidor
// já valida (abate com XP, saque pego do chão, raid boss, invasão vencida e vitória PvP) e
// só para quem aceitou; nada roda por tick. O dia vira (meia-noite de Brasília) na próxima
// vez que o jogador mexe nos contratos, ganha progresso ou entra.
//
// A recompensa (moedas, XP ou Maestria e reputação) passa pela economia de sempre: moedas
// com trackGold("contract"), XP com grantXPFrom(xpContract), compras com trackGold e
// trackItem("rep_shop"). Telemetria: contract_accept e contract_done em game_events.

// contractsState: contratos e reputação em memória.
type contractsState struct {
	st  game.ContractState
	rep game.ReputationState
}

func loadContracts(c game.ContractState, r game.ReputationState) contractsState {
	return contractsState{st: game.NormalContracts(c), rep: game.NormalReputation(r)}
}

// save: cópias para o salvamento (os mapas vão para outra goroutine).
func (c *contractsState) save() (game.ContractState, game.ReputationState) {
	st := game.ContractState{Day: c.st.Day, Week: c.st.Week}
	if len(c.st.Taken) > 0 {
		st.Taken = make(map[string]*game.ContractProgress, len(c.st.Taken))
		for id, p := range c.st.Taken {
			q := *p
			q.Seen = slices.Clone(p.Seen)
			st.Taken[id] = &q
		}
	}
	return st, game.ReputationState{Points: maps.Clone(c.rep.Points)}
}

// ---------- O quadro ----------

// contractBoard: Iselda, a Mestra dos Contratos, ao sul do poço (entre as tendas da
// Mercadora e do Ferreiro). Sem estoque: clicar nela abre a janela de contratos.
var contractBoard = npcDef{name: "Iselda", title: "Quadro de Contratos", model: "knight", x: -1.5, z: -9,
	look: &game.Appearance{Skin: 2, Hair: 3, Outfit: 2, Head: "knight"}}

// boardID: o id do quadro (logo depois da Mestra dos Caminhos, em spawnNPCs).
var boardID = npcIDBase + uint32(len(townNPCs)+len(teleportStones)+2)

func (z *Zone) spawnBoard(id uint32) {
	d := contractBoard
	pos := z.freeSpot(Vec2{X: d.x, Z: d.z})
	z.npcs[id] = &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_NPC,
		name: d.name, model: d.model, level: 1, scale: 1,
		pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
		npc: &npcState{title: d.title, look: d.look},
	}
}

// atBoard: o jogador está perto do quadro (senão avisa).
func (z *Zone) atBoard(p *entity, npcID uint32) bool {
	if npcID != boardID {
		return false
	}
	return z.shopNPC(p, npcID) != nil
}

// ---------- Dia e estado ----------

// contractDay: hoje (Brasília) para o jogador; vira o dia e a semana se mudaram.
func (z *Zone) contractDay(p *entity) int {
	day := game.ContractDay(z.cfg.Clock(), z.eventLoc())
	if p.pl.contracts.st.Refresh(p.id, day) {
		z.saveSoon(p)
	}
	return day
}

func contractSlotPB(s game.ContractSlot) pb.ContractSlot {
	switch s {
	case game.ContractHunt:
		return pb.ContractSlot_CONTRACT_SLOT_HUNT
	case game.ContractBoss:
		return pb.ContractSlot_CONTRACT_SLOT_BOSS
	case game.ContractCollect:
		return pb.ContractSlot_CONTRACT_SLOT_COLLECT
	case game.ContractWeekly:
		return pb.ContractSlot_CONTRACT_SLOT_WEEKLY
	}
	return pb.ContractSlot_CONTRACT_SLOT_UNSPECIFIED
}

// contractXPLabel: "XP" ou, no nível máximo, "XP de Maestria".
func contractXPLabel(level int) string {
	if level >= game.MasteryLevel {
		return "XP de Maestria"
	}
	return "XP"
}

func (z *Zone) contractRewardText(p *entity, def *game.ContractDef) string {
	parts := []string{fmt.Sprintf("%d moedas", def.Gold)}
	if xp := def.RewardXP(p.level); xp > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", xp, contractXPLabel(p.level)))
	}
	if f := game.FactionByID(def.Faction); f != nil {
		parts = append(parts, fmt.Sprintf("%d de reputação (%s)", def.Rep, f.Name))
	}
	return strings.Join(parts, " · ")
}

func (z *Zone) contractsPB(p *entity, completed string) *pb.ContractState {
	day := z.contractDay(p)
	loc := z.eventLoc()
	out := &pb.ContractState{
		BoardId: boardID, MinLevel: game.ContractMinLevel, Completed: completed,
		DailyResetMs:  game.NextDailyReset(day, loc).UnixMilli(),
		WeeklyResetMs: game.NextWeeklyReset(day, loc).UnixMilli(),
	}
	for _, def := range game.OfferedContracts(p.id, day) {
		info := &pb.ContractInfo{
			Id: def.ID, Title: def.Title, Text: def.Text, Slot: contractSlotPB(def.Slot),
			Status: pb.ContractStatus_CONTRACT_STATUS_OPEN, Need: uint32(def.Count),
			Place: def.Place, Faction: def.Faction, Reward: z.contractRewardText(p, def),
			Gold: def.Gold, Xp: def.RewardXP(p.level), Rep: uint32(def.Rep),
		}
		if pr := p.pl.contracts.st.Taken[def.ID]; pr != nil {
			info.Count = uint32(pr.N)
			info.Status = pb.ContractStatus_CONTRACT_STATUS_ACTIVE
			if pr.Done {
				info.Status = pb.ContractStatus_CONTRACT_STATUS_DONE
			}
		}
		out.Contracts = append(out.Contracts, info)
	}
	return out
}

func (z *Zone) sendContracts(p *entity, completed string) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Contracts{Contracts: z.contractsPB(p, completed)}})
}

// contractsOnJoin: o estado dos contratos (com o quadro) e as facções.
func (z *Zone) contractsOnJoin(p *entity) {
	z.sendContracts(p, "")
	z.sendReputation(p, false, "")
}

// ---------- Comandos ----------

func (z *Zone) onContractCmd(p *entity, m *pb.ContractCmd) {
	switch m.Kind {
	case pb.ContractCmdKind_CONTRACT_CMD_GET:
		z.sendContracts(p, "")
	case pb.ContractCmdKind_CONTRACT_CMD_ACCEPT:
		z.acceptContract(p, m.NpcId, m.Contract)
	}
}

func (z *Zone) acceptContract(p *entity, npcID uint32, id string) {
	if !z.atBoard(p, npcID) {
		return
	}
	if p.level < game.ContractMinLevel {
		z.system(p, fmt.Sprintf("Os contratos do quadro são para quem já chegou ao nível %d.", game.ContractMinLevel))
		return
	}
	day := z.contractDay(p)
	def := game.Contracts[id]
	if def == nil || !slices.Contains(game.OfferedContracts(p.id, day), def) {
		z.system(p, "Esse contrato não está no quadro hoje.")
		z.sendContracts(p, "")
		return
	}
	st := &p.pl.contracts.st
	if st.Taken[id] != nil {
		z.system(p, "Você já aceitou esse contrato.")
		return
	}
	if st.Taken == nil {
		st.Taken = map[string]*game.ContractProgress{}
	}
	st.Taken[id] = &game.ContractProgress{}
	z.saveSoon(p)
	z.track(p, "contract_accept", def.ID, 0, map[string]any{"slot": int(def.Slot), "faction": def.Faction, "day": st.Day, "week": st.Week})
	z.system(p, fmt.Sprintf("Contrato aceito: %s. %s", def.Title, def.Place))
	z.sendContracts(p, "")
}

// ---------- Progresso ----------

// contractAdvance soma a cada contrato aceito (e não concluído) do tipo goal o que match
// devolve, conclui o que chegou lá e avisa o cliente se mudou.
func (z *Zone) contractAdvance(p *entity, goal game.ContractGoal, match func(def *game.ContractDef, pr *game.ContractProgress) int) {
	if p.pl == nil || len(p.pl.contracts.st.Taken) == 0 {
		return
	}
	z.contractDay(p) // o progresso de ontem não conta hoje
	st := &p.pl.contracts.st
	ids := slices.Sorted(maps.Keys(st.Taken))
	changed := false
	var done []*game.ContractDef
	for _, id := range ids {
		def, pr := game.Contracts[id], st.Taken[id]
		if def == nil || pr.Done || def.Goal != goal {
			continue
		}
		add := match(def, pr)
		if add <= 0 {
			continue
		}
		pr.N = min(def.Count, pr.N+add)
		changed = true
		if pr.N >= def.Count {
			pr.Done = true
			done = append(done, def)
		}
	}
	if !changed {
		return
	}
	z.saveSoon(p)
	completed := ""
	for _, def := range done {
		z.finishContract(p, def)
		completed = def.Title
	}
	z.sendContracts(p, completed)
}

// finishContract entrega a recompensa: moedas, XP (Maestria no nível máximo) e reputação.
func (z *Zone) finishContract(p *entity, def *game.ContractDef) {
	st := &p.pl.contracts.st
	xp := def.RewardXP(p.level)
	z.systemAfterFlush(p, fmt.Sprintf("Contrato concluído: %s. Recompensa: %s.", def.Title, z.contractRewardText(p, def)))
	if def.Gold > 0 {
		p.pl.inv.gold += def.Gold
		z.trackGold(p, "contract", def.Gold)
		z.inventoryChanged(p, false)
	}
	gained, before, after := p.pl.contracts.rep.Add(def.Faction, def.Rep)
	z.track(p, "contract_done", def.ID, def.Gold, map[string]any{
		"slot": int(def.Slot), "faction": def.Faction, "rep": gained, "xp": xp, "day": st.Day, "week": st.Week,
	})
	z.log.Info("contract done", "id", p.id, "name", p.name, "contract", def.ID)
	if xp > 0 {
		z.grantXPFrom(p, xpContract, xp)
	}
	up := ""
	if after > before {
		if f := game.FactionByID(def.Faction); f != nil {
			up = fmt.Sprintf("%s com a %s", game.RepGradeNames[after], f.Name)
			z.systemAfterFlush(p, fmt.Sprintf("Sua reputação com a %s subiu para %s! A loja do Quadro de Contratos tem ofertas novas.", f.Name, game.RepGradeNames[after]))
		}
	}
	z.sendReputation(p, false, up)
}

// contractKill: abate com XP (shareMonsterXPScaled). Invasores, invocações e o raid boss
// ficam de fora (o raid tem o seu gancho).
func (z *Zone) contractKill(p, dead *entity) {
	if dead.mon == nil || dead.mon.invader || dead.mon.summon || dead.mon.def.Raid {
		return
	}
	id, home, level := dead.mon.def.ID, dead.mon.home, dead.level
	z.contractAdvance(p, game.CGoalKill, func(def *game.ContractDef, _ *game.ContractProgress) int {
		if def.Matches(id) && level >= def.MinLevel && game.InContractArea(def.Area, home.X, home.Z) {
			return 1
		}
		return 0
	})
}

// contractLoot: pegou do chão um material do saque de um monstro (loot.go).
func (z *Zone) contractLoot(p *entity, g *groundItem) {
	if !g.loot || g.def.Kind != game.KindMaterial {
		return
	}
	id, from, n := g.def.ID, g.lootFrom, g.count
	z.contractAdvance(p, game.CGoalLoot, func(def *game.ContractDef, _ *game.ContractProgress) int {
		if def.Matches(id) && game.InContractArea(def.Area, from.X, from.Z) {
			return n
		}
		return 0
	})
}

// contractRaid: participou (com dano) da morte do raid boss.
func (z *Zone) contractRaid(p *entity, boss string) {
	z.contractAdvance(p, game.CGoalRaid, func(def *game.ContractDef, _ *game.ContractProgress) int {
		if def.Matches(boss) {
			return 1
		}
		return 0
	})
}

// contractDefense: defendeu a cidade numa invasão vencida.
func (z *Zone) contractDefense(p *entity) {
	z.contractAdvance(p, game.CGoalDefense, func(*game.ContractDef, *game.ContractProgress) int { return 1 })
}

// contractPvP: venceu dead (roxo ou com karma; social.go). Só vale de outra conta, do nível
// mínimo para cima, e cada personagem conta uma vez no contrato.
func (z *Zone) contractPvP(killer, dead *entity) {
	if killer.pl == nil || dead.pl == nil || dead.level < game.ContractPvPMinLevel ||
		dead.pl.accountID == killer.pl.accountID {
		return
	}
	z.contractAdvance(killer, game.CGoalPvP, func(_ *game.ContractDef, pr *game.ContractProgress) int {
		if slices.Contains(pr.Seen, dead.id) {
			return 0
		}
		pr.Seen = append(pr.Seen, dead.id)
		return 1
	})
}

// ---------- Reputação e loja ----------

func (z *Zone) reputationPB(p *entity, offers bool, gradeUp string) *pb.ReputationState {
	rep := &p.pl.contracts.rep
	out := &pb.ReputationState{GradeNames: game.RepGradeNames, GradeUp: gradeUp}
	for _, n := range game.RepGradePoints {
		out.GradePoints = append(out.GradePoints, uint32(n))
	}
	for _, f := range game.Factions {
		pts := rep.Of(f.ID)
		g := game.GradeOf(pts)
		info := &pb.FactionInfo{Id: f.ID, Name: f.Name, Text: f.Text, Color: f.Color, Points: uint32(pts),
			Grade: uint32(g), GradeStart: uint32(game.RepGradePoints[g])}
		if int(g)+1 < len(game.RepGradePoints) {
			info.GradeNext = uint32(game.RepGradePoints[g+1])
		}
		out.Factions = append(out.Factions, info)
	}
	if offers {
		for _, o := range game.RepOffers {
			info := &pb.RepOffer{Id: o.ID, Faction: o.Faction, Grade: uint32(o.Grade), ItemId: o.Item, Count: uint32(o.Count),
				Price: o.Price, Text: o.Text}
			if o.Title != "" {
				info.Title = game.TitleOf(o.Title)
				_, info.Owned = p.pl.ach.unlocked[o.Title]
			}
			out.Offers = append(out.Offers, info)
		}
	}
	return out
}

func (z *Zone) sendReputation(p *entity, offers bool, gradeUp string) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Reputation{Reputation: z.reputationPB(p, offers, gradeUp)}})
}

func (z *Zone) onReputationCmd(p *entity, m *pb.ReputationCmd) {
	switch m.Kind {
	case pb.ReputationCmdKind_REPUTATION_CMD_GET:
		z.sendReputation(p, true, "")
	case pb.ReputationCmdKind_REPUTATION_CMD_BUY:
		z.buyRepOffer(p, m.NpcId, m.Offer)
	}
}

// buyRepOffer: compra na loja da facção (perto do quadro, com o grau e as moedas).
func (z *Zone) buyRepOffer(p *entity, npcID uint32, id string) {
	if !z.atBoard(p, npcID) {
		return
	}
	o := game.RepOfferByID(id)
	if o == nil {
		return
	}
	f := game.FactionByID(o.Faction)
	if f == nil {
		return
	}
	if g := game.GradeOf(p.pl.contracts.rep.Of(o.Faction)); g < o.Grade {
		z.system(p, fmt.Sprintf("Essa oferta da %s pede o grau %s (você é %s).", f.Name, game.RepGradeNames[o.Grade], game.RepGradeNames[g]))
		return
	}
	if _, ok := p.pl.ach.unlocked[o.Title]; ok && o.Title != "" {
		z.system(p, "Você já tem esse título.")
		return
	}
	inv := &p.pl.inv
	if inv.gold < o.Price {
		z.system(p, "Moedas insuficientes.")
		return
	}
	if o.Title != "" {
		inv.gold -= o.Price
		z.trackGold(p, "rep_shop", -o.Price)
		z.inventoryChanged(p, false)
		z.achValue(p, game.AchRepBuy, o.Title, 1) // desbloqueia (e avisa) como uma conquista
		z.system(p, fmt.Sprintf("Você comprou o título «%s» por %d moedas. Escolha na janela Glória (H).", game.TitleOf(o.Title), o.Price))
	} else {
		def := game.Items[o.Item]
		if def == nil {
			return
		}
		if !inv.canAdd(def, o.Count) {
			z.system(p, "Sua bolsa está cheia.")
			return
		}
		inv.gold -= o.Price
		inv.add(def, o.Count)
		z.trackGold(p, "rep_shop", -o.Price)
		z.trackItem(p, true, def.ID, o.Count, "rep_shop")
		z.system(p, fmt.Sprintf("Você comprou %s por %d moedas.", itemLabel(def, o.Count), o.Price))
		z.inventoryChanged(p, false)
	}
	z.sendReputation(p, true, "")
}

// ---------- GM ----------

// gmContract: /contrato andar N [Nome] (soma N a cada contrato aceito, como se fosse o
// acontecimento), /contrato dia [Nome] (esquece os diários aceitos hoje, como na virada) e
// /contrato rep facção N [Nome] (reputação).
func (z *Zone) gmContract(gm *entity, args []string) {
	usage := "Uso: /contrato andar N [Nome] · /contrato dia [Nome] · /contrato rep bastion|guard N [Nome]"
	if len(args) < 1 {
		z.system(gm, usage)
		return
	}
	target := func(i int) *entity {
		if len(args) > i {
			return z.online(gm, args[i])
		}
		return gm
	}
	switch strings.ToLower(args[0]) {
	case "andar":
		n, err := strconv.Atoi(argAt(args, 1))
		t := target(2)
		if err != nil || n < 1 || t == nil {
			z.system(gm, usage)
			return
		}
		for _, goal := range []game.ContractGoal{game.CGoalKill, game.CGoalLoot, game.CGoalRaid, game.CGoalDefense, game.CGoalPvP} {
			z.contractAdvance(t, goal, func(*game.ContractDef, *game.ContractProgress) int { return n })
		}
		z.system(gm, fmt.Sprintf("%s: contratos aceitos +%d.", t.name, n))
		z.gmLog(gm, "contrato", t.name, fmt.Sprintf("andar %d", n))
	case "dia":
		t := target(1)
		if t == nil {
			return
		}
		st := &t.pl.contracts.st
		for id := range st.Taken {
			if def := game.Contracts[id]; def != nil && def.Slot != game.ContractWeekly {
				delete(st.Taken, id)
			}
		}
		z.saveSoon(t)
		z.sendContracts(t, "")
		z.system(gm, t.name+": diários de hoje liberados de novo.")
		z.gmLog(gm, "contrato", t.name, "dia")
	case "rep":
		n, err := strconv.Atoi(argAt(args, 2))
		t := target(3)
		if err != nil || game.FactionByID(argAt(args, 1)) == nil || t == nil {
			z.system(gm, usage)
			return
		}
		_, _, after := t.pl.contracts.rep.Add(argAt(args, 1), n)
		z.saveSoon(t)
		z.sendReputation(t, true, "")
		z.system(gm, fmt.Sprintf("%s: %d de reputação com %s (%s).", t.name, t.pl.contracts.rep.Of(argAt(args, 1)), argAt(args, 1), game.RepGradeNames[after]))
		z.gmLog(gm, "contrato", t.name, fmt.Sprintf("rep %s %d", argAt(args, 1), n))
	default:
		z.system(gm, usage)
	}
}

func argAt(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}

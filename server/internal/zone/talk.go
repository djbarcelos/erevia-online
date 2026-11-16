package zone

import (
	"fmt"
	"slices"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Conversa com NPCs (tunnel#71; falas e missões em game/talk.go, game/story.go e
// game/dailies.go). O cliente manda NpcTalk e recebe NpcDialog: a fala e as opções. O
// servidor confere tudo a cada pedido: o NPC perto (shopRange), a missão disponível para
// aquele NPC e nível, os passos cumpridos, os itens na bolsa e a diária do dia certo. Uma
// missão entregue sai da lista de ativas na mesma hora, então a segunda entrega não acha
// nada para entregar.
//
// Recompensas e itens entregues passam pela economia de sempre: moedas com
// trackGold("quest"), itens com trackItem("quest") ao entrar e trackItem("quest_deliver") ao
// sair. Nada roda por tick: só o 1×/s dos lugares para visitar (checkVisits).

// talkerFirstID: o primeiro NPC novo (o Arauto e os contadores de história), logo depois do
// Guardião das Sendas, o maior id da cidade (ver spawnNPCs).
var talkerFirstID = guardianID + 1

// lastNPCID: o maior id de NPC fixo (fora os portões, que têm faixa própria em gateIDBase).
func lastNPCID() uint32 { return talkerFirstID + uint32(len(game.SpawnedTalkers())) - 1 }

// spawnTalkers cria os NPCs novos (na zona do mapa de cada um) e liga a conversa dos NPCs
// da praça pelo nome.
func (z *Zone) spawnTalkers() {
	for i, t := range game.SpawnedTalkers() {
		m := t.Spawn.Map
		if m == "" {
			m = world.StarterMap
		}
		if m != z.mapID() {
			continue
		}
		pos := z.freeSpot(Vec2{X: t.Spawn.X, Z: t.Spawn.Z})
		id := talkerFirstID + uint32(i)
		z.addNPC(&entity{
			id: id, kind: pb.EntityKind_ENTITY_KIND_NPC,
			name: t.Name, model: t.Spawn.Model, level: 1, scale: 1,
			pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
			npc: &npcState{title: t.Title, role: pb.NpcRole_NPC_ROLE_QUEST, look: t.Spawn.Look, talk: t},
		})
	}
	for _, n := range z.npcs {
		if n.npc.talk == nil && n.npc.role != pb.NpcRole_NPC_ROLE_TELEPORT && n.npc.role != pb.NpcRole_NPC_ROLE_GATE {
			n.npc.talk = game.TalkerByName(n.name)
		}
	}
}

// addNPC põe o NPC na zona. Id repetido é erro de dados (dois NPCs com o mesmo id: um some
// e o clique vai para o outro): para a partida.
func (z *Zone) addNPC(n *entity) {
	if old := z.npcs[n.id]; old != nil {
		panic(fmt.Sprintf("NPC %d repetido: %s e %s", n.id, old.name, n.name))
	}
	z.npcs[n.id] = n
}

// ---------- Diárias ----------

// dailyRefresh vira o dia das diárias (meia-noite de Brasília, o mesmo dia dos contratos):
// sorteia o rol novo e tira as aceitas em outro dia (mesmo que a mesma diária saia de novo
// hoje, o progresso de ontem não vale). Devolve se tirou alguma.
func (z *Zone) dailyRefresh(p *entity) bool {
	if p.pl == nil {
		return false
	}
	q := &p.pl.quests
	old := q.daily.Day
	if !q.daily.Refresh(p.id, game.ContractDay(z.cfg.Clock(), z.eventLoc()), p.level) {
		return false
	}
	z.saveSoon(p)
	n := len(q.active)
	q.active = slices.DeleteFunc(q.active, func(a *activeQuest) bool {
		return a.def.Daily != 0 && (old != q.daily.Day || !q.daily.Offered(a.def.ID) || q.daily.Delivered(a.def.ID))
	})
	return len(q.active) != n
}

// ---------- Conversa ----------

func (z *Zone) onNpcTalk(p *entity, m *pb.NpcTalk) {
	if p.dead {
		return
	}
	n := z.npcs[m.NpcId]
	if n == nil || n.npc.talk == nil || z.shopNPC(p, n.id) == nil {
		return
	}
	switch m.Kind {
	case pb.NpcTalkKind_NPC_TALK_OPEN:
		z.talkOpen(p, n)
	case pb.NpcTalkKind_NPC_TALK_ACCEPT:
		z.talkAccept(p, n, m.QuestId)
	case pb.NpcTalkKind_NPC_TALK_TURN_IN:
		z.talkTurnIn(p, n, m.QuestId)
	case pb.NpcTalkKind_NPC_TALK_ASK:
		if tp := n.npc.talk.Topic(m.Topic); tp != nil {
			z.sendDialog(p, n, tp.Text)
		}
	}
}

// talkOpen: falar com o NPC conta para os passos de "falar com ele" (e a fala do passo, se
// houver, é a resposta). NPC de serviço sem nada de missão para o jogador abre o serviço
// direto, como antes das conversas.
func (z *Zone) talkOpen(p *entity, n *entity) {
	if z.dailyRefresh(p) {
		z.sendQuests(p, "")
	}
	say := ""
	for _, a := range p.pl.quests.active {
		for i, s := range a.def.Steps {
			if s.Goal == game.GoalTalk && s.Say != "" && a.progress[i] < s.Count && s.Matches(game.GoalTalk, n.name) {
				say = s.Say
			}
		}
	}
	z.questEvent(p, game.GoalTalk, n.name)
	t := n.npc.talk
	if t.Service != "" && say == "" && len(z.questOptions(p, n)) == 0 {
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_NpcDialog{NpcDialog: &pb.NpcDialog{
			NpcId: n.id, Name: n.name, Title: n.npc.title, Service: true}}})
		return
	}
	z.sendDialog(p, n, say)
}

// available: a missão que o NPC pode oferecer agora a este jogador.
func (z *Zone) available(p *entity, n *entity, def *game.QuestDef) bool {
	q := &p.pl.quests
	if def == nil || def.Giver != n.name || p.level < def.Level || q.done[def.ID] || q.has(def.ID) {
		return false
	}
	if def.Daily != 0 {
		return q.daily.Offered(def.ID) && !q.daily.Delivered(def.ID)
	}
	prev := storyPrev[def.ID] // a anterior da linha (o convite não conta: quem passou do nível dele também pode)
	return prev == nil || prev.Giver == "" || q.done[prev.ID]
}

// storyPrev: a missão anterior de cada missão encadeada (pelo Next).
var storyPrev = func() map[string]*game.QuestDef {
	m := map[string]*game.QuestDef{}
	for _, q := range game.QuestsInOrder() {
		if q.Next != "" {
			m[q.Next] = q
		}
	}
	return m
}()

// offers: as missões que o NPC tem para o jogador, na ordem dos dados (diárias: a do rol).
func (z *Zone) offers(p *entity, n *entity) []*game.QuestDef {
	var out []*game.QuestDef
	if n.name == game.TalkHerald {
		for _, id := range p.pl.quests.daily.Offer {
			if def := game.Quests[id]; z.available(p, n, def) {
				out = append(out, def)
			}
		}
		return out
	}
	for _, def := range game.QuestsInOrder() {
		if def.Giver == n.name && z.available(p, n, def) {
			out = append(out, def)
		}
	}
	return out
}

// turnIns: as missões ativas que dá para entregar a este NPC agora: todos os passos
// cumpridos e ele é quem recebe, ou uma entrega pendente para ele com os itens na bolsa.
func (z *Zone) turnIns(p *entity, n *entity) []*activeQuest {
	var out []*activeQuest
	for _, a := range p.pl.quests.active {
		if a.def.TurnIn == n.name && a.complete() || z.deliverReady(p, a, n) {
			out = append(out, a)
		}
	}
	return out
}

// deliverReady: a missão tem uma entrega pendente para n, os passos de antes dela estão
// cumpridos e a bolsa tem os itens.
func (z *Zone) deliverReady(p *entity, a *activeQuest, n *entity) bool {
	for i, s := range a.def.Steps {
		if a.progress[i] >= s.Count {
			continue
		}
		return s.Goal == game.GoalDeliver && s.NPC == n.name && p.pl.inv.countOf(s.Target) >= s.Count
	}
	return false
}

func (z *Zone) questOptions(p *entity, n *entity) []*pb.DialogOption {
	var out []*pb.DialogOption
	for _, a := range z.turnIns(p, n) {
		out = append(out, &pb.DialogOption{Kind: pb.DialogOptionKind_DIALOG_OPTION_TURN_IN, Label: "Entregar: " + a.def.Title,
			QuestId: a.def.ID, Reward: rewardText(a.def.Reward, p.level)})
	}
	for _, def := range z.offers(p, n) {
		label := "Aceitar: " + def.Title
		if def.Daily != 0 {
			label = "Serviço do dia: " + def.Title
		}
		out = append(out, &pb.DialogOption{Kind: pb.DialogOptionKind_DIALOG_OPTION_ACCEPT, Label: label,
			QuestId: def.ID, Detail: def.Text, Reward: rewardText(def.Reward, p.level)})
	}
	return out
}

// sendDialog: a janela de conversa com a fala text ("" = a fala de agora do NPC).
func (z *Zone) sendDialog(p *entity, n *entity, text string) {
	t := n.npc.talk
	opts := z.questOptions(p, n)
	if text == "" {
		text = z.npcLine(p, n, len(opts) > 0)
	}
	for _, tp := range t.Topics {
		opts = append(opts, &pb.DialogOption{Kind: pb.DialogOptionKind_DIALOG_OPTION_ASK, Label: tp.Label, Topic: tp.ID})
	}
	if t.Service != "" {
		opts = append(opts, &pb.DialogOption{Kind: pb.DialogOptionKind_DIALOG_OPTION_SERVICE, Label: t.Service})
	}
	opts = append(opts, &pb.DialogOption{Kind: pb.DialogOptionKind_DIALOG_OPTION_LEAVE, Label: "Até mais."})
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_NpcDialog{NpcDialog: &pb.NpcDialog{
		NpcId: n.id, Name: n.name, Title: n.npc.title, Text: text, Options: opts}}})
}

// npcLine: a saudação quando há o que oferecer ou entregar; com uma missão dele em
// andamento, a fala de espera; cedo demais, quando voltar; senão, a de quando não há nada.
func (z *Zone) npcLine(p *entity, n *entity, hasOptions bool) string {
	t := n.npc.talk
	if hasOptions {
		return t.Greeting
	}
	if t.Busy != "" && slices.ContainsFunc(p.pl.quests.active, func(a *activeQuest) bool { return a.def.Giver == n.name }) {
		return t.Busy
	}
	if lvl := z.nextLevel(p, n); lvl > p.level {
		return fmt.Sprintf("%s Mas ainda é cedo para você: volte quando chegar ao nível %d.", t.Greeting, lvl)
	}
	if n.name == game.TalkHerald && p.level > game.DailyMaxLevel && len(p.pl.quests.daily.Offer) == 0 {
		return "O rol do Arauto é para quem ainda está subindo. No topo, quem paga é o Quadro de Contratos da Iselda."
	}
	return t.Idle
}

// nextLevel: o nível da próxima missão que o NPC ainda vai oferecer (0 = nenhuma).
func (z *Zone) nextLevel(p *entity, n *entity) int {
	if n.name == game.TalkHerald {
		if p.level < game.DailyMinLevel {
			return game.DailyMinLevel
		}
		return 0
	}
	q := &p.pl.quests
	for _, def := range game.QuestsInOrder() {
		if def.Giver != n.name || def.Daily != 0 || q.done[def.ID] || q.has(def.ID) {
			continue
		}
		if prev := storyPrev[def.ID]; prev == nil || prev.Giver == "" || q.done[prev.ID] {
			return def.Level
		}
	}
	return 0
}

func (z *Zone) talkAccept(p *entity, n *entity, id string) {
	if z.dailyRefresh(p) {
		z.sendQuests(p, "")
	}
	def := game.Quests[id]
	if !z.available(p, n, def) {
		if def != nil && def.Giver == n.name && p.level < def.Level {
			z.system(p, fmt.Sprintf("%s: «Volte quando chegar ao nível %d.»", n.name, def.Level))
		}
		z.sendDialog(p, n, "")
		return
	}
	p.pl.quests.active = append(p.pl.quests.active, &activeQuest{def: def, progress: make([]int, len(def.Steps))})
	z.saveSoon(p)
	z.track(p, "quest_accept", def.ID, int64(p.level), map[string]any{"region": def.Region, "daily": def.Daily != 0})
	z.system(p, "Nova missão: "+def.Title+".")
	z.questLevel(p) // objetivo de nível que já está cumprido
	z.sendQuests(p, "")
	z.sendDialog(p, n, n.npc.talk.Accepted)
}

// talkTurnIn: entrega a missão (ou os itens de uma entrega) ao NPC.
func (z *Zone) talkTurnIn(p *entity, n *entity, id string) {
	i := slices.IndexFunc(p.pl.quests.active, func(a *activeQuest) bool { return a.def.ID == id })
	if i < 0 || !slices.Contains(z.turnIns(p, n), p.pl.quests.active[i]) {
		z.sendDialog(p, n, "")
		return
	}
	a := p.pl.quests.active[i]
	delivered := false
	for k, s := range a.def.Steps {
		if a.progress[k] >= s.Count {
			continue
		}
		if s.Goal != game.GoalDeliver || s.NPC != n.name || !p.pl.inv.takeID(s.Target, s.Count) {
			break
		}
		z.trackItem(p, false, s.Target, s.Count, "quest_deliver")
		if d := game.Items[s.Target]; d != nil {
			z.system(p, fmt.Sprintf("Você entregou %s a %s.", itemLabel(d, s.Count), n.name))
		}
		a.progress[k] = s.Count
		delivered = true
	}
	if delivered {
		z.inventoryChanged(p, false)
		z.saveSoon(p)
	}
	if !a.complete() || a.def.TurnIn != n.name && a.def.TurnIn != "" {
		z.sendQuests(p, "")
		z.sendDialog(p, n, "")
		return
	}
	p.pl.quests.active = slices.Delete(p.pl.quests.active, i, i+1)
	z.saveSoon(p)
	z.finishQuest(p, a.def)
	z.sendQuests(p, a.def.Title)
	line := a.def.Done
	if line == "" {
		line = n.npc.talk.Idle
	}
	z.sendDialog(p, n, line)
}

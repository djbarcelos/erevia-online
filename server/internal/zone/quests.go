package zone

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Missões: o progresso anda com acontecimentos que o servidor já valida (matar, usar
// skill, tomar poção, comprar, viajar, subir de nível, sair da cidade). O cliente só
// recebe um QuestLog quando algo muda: nada por quadro. Cada passo pendente vai com o
// lugar dele (questplace.go).

const questRewardReserve = 120.0 // segundos: recompensa que não coube na bolsa fica reservada no chão

type activeQuest struct {
	def      *game.QuestDef
	progress []int
}

func (q *activeQuest) complete() bool {
	for i, s := range q.def.Steps {
		if q.progress[i] < s.Count {
			return false
		}
	}
	return true
}

type questLog struct {
	active []*activeQuest
	done   map[string]bool
	daily  game.DailyState // as diárias do dia (dailies.go); as aceitas ficam em active
	chests map[string]int  // o dia em que o baú de cada masmorra foi aberto (instance.go)
	// doneSent: a lista de concluídas já foi ao cliente nesta sessão (depois disso ela só vai
	// de novo quando uma missão termina).
	doneSent bool
}

// loadQuests monta as missões salvas. Personagem novo (sem histórico e abaixo do nível
// de corte) começa o tutorial.
func loadQuests(st game.QuestState, level int) questLog {
	q := questLog{done: map[string]bool{}}
	for _, id := range st.Done {
		if def := game.Quests[id]; def == nil || def.Daily == 0 { // diária nunca fica concluída para sempre
			q.done[id] = true
		}
	}
	if d := game.NormalDaily(st.Daily); d != nil {
		q.daily = *d
	}
	q.chests = game.NormalChests(st.Chests)
	ids := make([]string, 0, len(st.Active))
	for id := range st.Active {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		def := game.Quests[id]
		if def == nil || q.done[id] || def.Daily != 0 && (!q.daily.Offered(id) || q.daily.Delivered(id)) {
			continue // missão que deixou de existir (ou diária fora do rol)
		}
		prog := make([]int, len(def.Steps))
		copy(prog, st.Active[id])
		q.active = append(q.active, &activeQuest{def: def, progress: prog})
	}
	if len(q.active) == 0 && len(q.done) == 0 && level < game.TutorialSkipLevel {
		if def := game.Quests[game.TutorialStart]; def != nil {
			q.active = append(q.active, &activeQuest{def: def, progress: make([]int, len(def.Steps))})
		}
	}
	return q
}

func (q *questLog) state() game.QuestState {
	st := game.QuestState{}
	if len(q.active) > 0 {
		st.Active = make(map[string][]int, len(q.active))
		for _, a := range q.active {
			st.Active[a.def.ID] = slices.Clone(a.progress)
		}
	}
	for id := range q.done {
		st.Done = append(st.Done, id)
	}
	slices.Sort(st.Done)
	if q.daily.Day != 0 {
		d := game.DailyState{Day: q.daily.Day, Offer: slices.Clone(q.daily.Offer), Done: slices.Clone(q.daily.Done)}
		st.Daily = &d
	}
	st.Chests = maps.Clone(q.chests)
	return st
}

// questEvent soma 1 aos objetivos que batem com o acontecimento.
func (z *Zone) questEvent(p *entity, goal game.QuestGoal, target string) {
	z.questEventN(p, goal, target, 1)
}

// questEventN soma n (itens pegos do chão, por exemplo). Devolve se mandou o QuestLog.
func (z *Zone) questEventN(p *entity, goal game.QuestGoal, target string, n int) bool {
	if p.pl == nil || n <= 0 {
		return false
	}
	return z.questUpdate(p, func(s game.QuestStep, cur int) int {
		if s.Matches(goal, target) {
			return cur + n
		}
		return cur
	})
}

// questLevel acompanha os objetivos de nível e entrega as missões que chegam com o nível.
// Devolve se mandou o QuestLog.
func (z *Zone) questLevel(p *entity) bool {
	sent := z.questUnlock(p)
	return z.questUpdate(p, func(s game.QuestStep, cur int) int {
		if s.Goal == game.GoalLevel {
			return max(cur, p.level)
		}
		return cur
	}) || sent
}

// questLevelUp: subiu de nível. Além de questLevel, reenvia o QuestLog quando um objetivo
// pendente tem o lugar ligado ao nível (a área de caça certa muda com ele).
func (z *Zone) questLevelUp(p *entity) {
	if p.pl == nil || z.questLevel(p) {
		return
	}
	for _, a := range p.pl.quests.active {
		for i, s := range a.def.Steps {
			if a.progress[i] < s.Count && placeByLevel(s) {
				z.sendQuests(p, "")
				return
			}
		}
	}
}

// questUpdate aplica f a cada objetivo, conclui o que completou e avisa o cliente se mudou
// (devolve se mandou o QuestLog).
func (z *Zone) questUpdate(p *entity, f func(s game.QuestStep, cur int) int) bool {
	stale := z.dailyRefresh(p) // a diária de ontem não anda hoje
	changed := false
	for _, a := range p.pl.quests.active {
		for i, s := range a.def.Steps {
			if v := min(s.Count, f(s, a.progress[i])); v != a.progress[i] {
				a.progress[i] = v
				changed = true
			}
		}
	}
	if !changed {
		if stale {
			z.sendQuests(p, "")
		}
		return stale
	}
	z.saveSoon(p)
	completed := ""
	for {
		i := slices.IndexFunc(p.pl.quests.active, (*activeQuest).finishesAlone)
		if i < 0 {
			break
		}
		a := p.pl.quests.active[i]
		p.pl.quests.active = slices.Delete(p.pl.quests.active, i, i+1)
		completed = a.def.Title
		z.finishQuest(p, a.def)
	}
	z.sendQuests(p, completed)
	return true
}

// finishQuest entrega a recompensa e começa a próxima missão da sequência.
func (z *Zone) finishQuest(p *entity, def *game.QuestDef) {
	if def.Daily != 0 {
		p.pl.quests.daily.Done = append(p.pl.quests.daily.Done, def.ID) // volta outro dia, se sair no sorteio
	} else {
		p.pl.quests.done[def.ID] = true
	}
	z.achEvent(p, game.AchQuest, def.ID)
	r := def.Reward
	xp := r.XPAt(p.level)
	z.system(p, fmt.Sprintf("Missão concluída: %s. Recompensa: %s.", def.Title, rewardText(r, p.level)))
	if def.Giver != "" { // telemetria das missões de NPC (linhas de história e diárias)
		z.track(p, "quest_done", def.ID, xp, map[string]any{"region": def.Region, "daily": def.Daily != 0})
	}
	if r.Gold > 0 {
		p.pl.inv.gold += r.Gold
		z.trackGold(p, "quest", r.Gold)
	}
	var dropped []*groundItem
	for _, it := range r.Items {
		d := game.Items[it.ID]
		if d == nil {
			continue
		}
		left := p.pl.inv.add(d, it.Count)
		if it.Count > left { // o que ficou no chão entra como loot quando alguém pegar
			z.trackItem(p, true, d.ID, it.Count-left, "quest")
		}
		if left > 0 {
			if g := z.dropItem(p.pos, d, left, 0, p.id, questRewardReserve); g != nil {
				dropped = append(dropped, g)
			}
		}
	}
	if len(dropped) > 0 {
		added := &pb.GroundItemsAdded{}
		for _, g := range dropped {
			added.Items = append(added.Items, z.groundPB(g))
		}
		z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_GroundAdded{GroundAdded: added}}, 0)
		z.system(p, "Sua bolsa está cheia: parte da recompensa ficou no chão, reservada para você.")
	}
	if r.Gold > 0 || len(r.Items) > 0 {
		z.inventoryChanged(p, false)
	}
	if f := game.FactionByID(r.Faction); f != nil && r.Rep > 0 {
		_, before, after := p.pl.contracts.rep.Add(r.Faction, r.Rep)
		up := ""
		if after > before {
			up = fmt.Sprintf("%s com a %s", game.RepGradeNames[after], f.Name)
			z.systemAfterFlush(p, fmt.Sprintf("Sua reputação com a %s subiu para %s!", f.Name, game.RepGradeNames[after]))
		}
		z.sendReputation(p, false, up)
	}
	if xp > 0 {
		z.grantXPFrom(p, xpQuest, xp) // pode subir de nível (e andar objetivos de nível da próxima)
	}
	if next := game.Quests[def.Next]; next != nil && next.Giver != "" {
		// A próxima da linha é oferecida na conversa (talk.go), não chega sozinha.
		if !p.pl.quests.done[next.ID] && !p.pl.quests.has(next.ID) {
			z.systemAfterFlush(p, fmt.Sprintf("%s tem mais a dizer: «%s».", next.Giver, next.Title))
		}
	} else if next != nil && !p.pl.quests.done[next.ID] {
		p.pl.quests.active = append(p.pl.quests.active, &activeQuest{def: next, progress: make([]int, len(next.Steps))})
		z.system(p, "Nova missão: "+next.Title+".")
		z.questLevel(p) // objetivo de nível que já está cumprido
	}
}

// questUnlock entrega as missões de nível (Unlock) que o jogador já alcançou e ainda não
// tem nem concluiu. Quem chegou lá antes da missão existir recebe ao entrar. Devolve se
// mandou o QuestLog.
func (z *Zone) questUnlock(p *entity) bool {
	if p.pl == nil {
		return false
	}
	added := false
	for _, def := range game.QuestsByUnlock {
		if p.level < def.Unlock || def.Until > 0 && p.level > def.Until || p.pl.quests.done[def.ID] || p.pl.quests.has(def.ID) {
			continue
		}
		p.pl.quests.active = append(p.pl.quests.active, &activeQuest{def: def, progress: make([]int, len(def.Steps))})
		z.systemAfterFlush(p, "Nova missão: "+def.Title+".")
		added = true
	}
	if added {
		z.saveSoon(p)
		z.sendQuests(p, "")
	}
	return added
}

// has diz se a missão está ativa.
func (q *questLog) has(id string) bool {
	return slices.ContainsFunc(q.active, func(a *activeQuest) bool { return a.def.ID == id })
}

func rewardText(r game.QuestReward, level int) string {
	var parts []string
	if xp := r.XPAt(level); xp > 0 {
		parts = append(parts, fmt.Sprintf("%d XP", xp))
	}
	if r.Gold > 0 {
		parts = append(parts, fmt.Sprintf("%d moedas", r.Gold))
	}
	for _, it := range r.Items {
		if d := game.Items[it.ID]; d != nil {
			parts = append(parts, itemLabel(d, it.Count))
		}
	}
	if f := game.FactionByID(r.Faction); f != nil && r.Rep > 0 {
		parts = append(parts, fmt.Sprintf("%d de reputação (%s)", r.Rep, f.Name))
	}
	if len(parts) == 0 {
		return "nenhuma"
	}
	return strings.Join(parts, " · ")
}

// sendQuests manda as missões ativas. As concluídas (só id, título e categoria) vão no
// primeiro QuestLog da sessão e quando alguma termina; nos outros, o cliente guarda as
// últimas (o QuestLog vai a cada monstro da missão e a lista só cresce).
func (z *Zone) sendQuests(p *entity, completed string) {
	log := &pb.QuestLog{Completed: completed}
	if p.pl.quests.daily.Day != 0 {
		log.DailyResetMs = game.NextDailyReset(p.pl.quests.daily.Day, z.eventLoc()).UnixMilli()
	}
	for _, a := range p.pl.quests.active {
		q := &pb.QuestInfo{Id: a.def.ID, Title: a.def.Title, Text: a.def.Text, Reward: rewardText(a.def.Reward, p.level),
			Category: questCategoryPB(a.def.Category), Region: a.def.Region}
		for i, s := range a.def.Steps {
			q.Steps = append(q.Steps, z.stepInfo(p, s, a.progress[i]))
		}
		if a.def.TurnIn != "" { // o último passo: voltar a quem recebe (o "Ir" aponta para ele)
			q.Steps = append(q.Steps, z.stepInfo(p, turnInStep(a.def), 0))
		}
		log.Quests = append(log.Quests, q)
	}
	if !p.pl.quests.doneSent || completed != "" {
		p.pl.quests.doneSent = true
		log.DoneFull = true
		log.Done = p.pl.quests.donePB()
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_QuestLog{QuestLog: log}})
}

// donePB: as concluídas na ordem da lista de missões (as que deixaram de existir saem).
func (q *questLog) donePB() []*pb.QuestDone {
	var out []*pb.QuestDone
	for _, def := range game.QuestsInOrder() {
		if q.done[def.ID] {
			out = append(out, &pb.QuestDone{Id: def.ID, Title: def.Title, Category: questCategoryPB(def.Category)})
		}
	}
	return out
}

func questCategoryPB(c game.QuestCategory) pb.QuestCategory {
	switch c {
	case game.QuestStory:
		return pb.QuestCategory_QUEST_CATEGORY_STORY
	case game.QuestRegion:
		return pb.QuestCategory_QUEST_CATEGORY_REGION
	case game.QuestDaily:
		return pb.QuestCategory_QUEST_CATEGORY_DAILY
	case game.QuestTutorial:
		return pb.QuestCategory_QUEST_CATEGORY_TUTORIAL
	}
	return pb.QuestCategory_QUEST_CATEGORY_UNSPECIFIED
}

// finishesAlone: a missão terminou e não espera entrega a ninguém.
func (q *activeQuest) finishesAlone() bool { return q.def.TurnIn == "" && q.complete() }

// turnInStep: o passo de voltar a quem recebe a missão (só no QuestLog: a entrega é na
// conversa, NPC_TALK_TURN_IN).
func turnInStep(def *game.QuestDef) game.QuestStep {
	return game.QuestStep{Goal: game.GoalTalk, Target: def.TurnIn, Count: 1, Text: "Voltar a " + def.TurnIn}
}

// checkVisits: 1×/s, só para quem tem um lugar para visitar pendente (GoalVisit).
func (z *Zone) checkVisits() {
	for _, p := range z.players {
		if p.dead {
			continue
		}
		var reached []string
		for _, a := range p.pl.quests.active {
			for i, s := range a.def.Steps {
				if s.Goal == game.GoalVisit && a.progress[i] < s.Count && s.Place != nil &&
					math.Hypot(p.pos.X-s.Place.X, p.pos.Z-s.Place.Z) <= s.Place.Radius && !slices.Contains(reached, s.Target) {
					reached = append(reached, s.Target)
				}
			}
		}
		for _, key := range reached {
			z.questEvent(p, game.GoalVisit, key)
		}
	}
}

// questCollect: pegou do chão um material do saque de um monstro (loot.go). Itens que o
// jogador largou ou que sobraram de recompensa não contam.
func (z *Zone) questCollect(p *entity, g *groundItem) {
	if !g.loot || g.def.Kind != game.KindMaterial {
		z.questHave(p, g.def.ID)
		return
	}
	if !z.questEventN(p, game.GoalCollect, g.def.ID, g.count) {
		z.questHave(p, g.def.ID)
	}
}

// questHave: a bolsa ganhou um item que uma entrega pendente pede; o QuestLog vai de novo
// (o passo mostra quantos o jogador tem, e o "Ir" troca de "onde achar" para "a quem levar").
func (z *Zone) questHave(p *entity, item string) {
	if p.pl == nil {
		return
	}
	for _, a := range p.pl.quests.active {
		for i, s := range a.def.Steps {
			if s.Goal == game.GoalDeliver && s.Target == item && a.progress[i] < s.Count {
				z.sendQuests(p, "")
				return
			}
		}
	}
}

// checkLeaveTown: 1×/s, só para quem tem o objetivo de sair da cidade pendente.
func (z *Zone) checkLeaveTown() {
	for _, p := range z.players {
		if p.dead || z.inSafe(p.pos) {
			continue
		}
		for _, a := range p.pl.quests.active {
			if i := slices.IndexFunc(a.def.Steps, func(s game.QuestStep) bool { return s.Goal == game.GoalLeaveTown }); i >= 0 &&
				a.progress[i] < a.def.Steps[i].Count {
				z.questEvent(p, game.GoalLeaveTown, "")
				break
			}
		}
	}
}

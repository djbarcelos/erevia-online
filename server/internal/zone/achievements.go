package zone

import (
	"maps"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Conquistas e títulos. Os ganchos ficam nos pontos que o servidor já valida (abate,
// nível, encantamento, troca, criação, missão, vitória PvP e raid boss). O cliente recebe
// o catálogo ao entrar e, depois, só o que mudou. O título escolhido vai para todos no
// Entity.title e no PlayerStatus: muda só quando o jogador troca de título.

type achievements struct {
	unlocked map[string]int64 // conquista → quando (Unix, ms)
	counts   map[string]int   // progresso das conquistas de contagem
	title    string           // conquista cujo título está em uso ("" = nenhum)
}

// loadAchievements monta o estado salvo, ignorando conquistas que deixaram de existir.
func loadAchievements(st game.AchievementState, title string) achievements {
	a := achievements{unlocked: map[string]int64{}, counts: map[string]int{}}
	for id, at := range st.Unlocked {
		if game.Achievements[id] != nil {
			a.unlocked[id] = at
		}
	}
	for id, n := range st.Counts {
		if game.Achievements[id] != nil && n > 0 {
			if _, done := a.unlocked[id]; !done {
				a.counts[id] = n
			}
		}
	}
	if _, ok := a.unlocked[title]; ok || game.BestiaryFamilyByTitle(title) != nil { // o de família: bestiaryOnJoin confere
		a.title = title
	}
	return a
}

func (a *achievements) state() game.AchievementState {
	st := game.AchievementState{}
	if len(a.unlocked) > 0 {
		st.Unlocked = maps.Clone(a.unlocked)
	}
	if len(a.counts) > 0 {
		st.Counts = maps.Clone(a.counts)
	}
	return st
}

// titleText é o título que aparece sob o nome ("" = nenhum).
func (a *achievements) titleText() string { return game.TitleOf(a.title) }

// achEvent: aconteceu uma vez (abate, troca, missão, raid). Conta 1 nas conquistas de
// contagem que batem com o alvo.
func (z *Zone) achEvent(p *entity, goal game.AchGoal, target string) {
	z.achUpdate(p, goal, target, func(def *game.AchievementDef, cur int) int { return cur + 1 })
}

// achValue: o acontecimento traz um valor absoluto (nível, +N do encantamento, raridade
// criada, total de vitórias PvP). Nas de limite, desbloqueia com valor >= Count; nas de
// contagem, o progresso vira o maior entre o atual e o valor.
func (z *Zone) achValue(p *entity, goal game.AchGoal, target string, value int) {
	z.achUpdate(p, goal, target, func(def *game.AchievementDef, cur int) int { return max(cur, value) })
}

// achUpdate aplica f às conquistas ainda bloqueadas do tipo goal, desbloqueia as que
// chegaram lá e avisa o cliente do que mudou. Um abate só olha as conquistas de abate.
func (z *Zone) achUpdate(p *entity, goal game.AchGoal, target string, f func(def *game.AchievementDef, cur int) int) {
	if p.pl == nil {
		return
	}
	a := &p.pl.ach
	var changed []*pb.AchievementInfo
	var unlocked []*game.AchievementDef
	for _, def := range game.AchievementsByGoal[goal] {
		if _, done := a.unlocked[def.ID]; done || !def.Matches(target) {
			continue
		}
		cur := a.counts[def.ID]
		next := f(def, cur)
		if def.Each {
			next = def.Mark(cur, target) // cada alvo conta uma vez (Caçador de Raros)
		}
		if def.Progress(next) >= def.Count {
			delete(a.counts, def.ID)
			a.unlocked[def.ID] = z.cfg.Clock().UnixMilli()
			unlocked = append(unlocked, def)
			changed = append(changed, z.achInfo(p, def, false))
			continue
		}
		if def.Threshold() || next == cur {
			continue // limite não atingido: nada a guardar
		}
		a.counts[def.ID] = next
		changed = append(changed, z.achInfo(p, def, false))
	}
	if len(changed) == 0 {
		return
	}
	if len(unlocked) == 0 {
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Achievements{Achievements: &pb.Achievements{List: changed, TitleId: a.title}}})
		return
	}
	z.saveSoon(p)
	for _, def := range unlocked {
		z.log.Info("achievement", "id", p.id, "name", p.name, "achievement", def.ID)
		z.track(p, "achievement", def.ID, 0, nil)
	}
	// O aviso sai depois do snapshot: o abate aparece antes da conquista.
	z.afterFlush(func() {
		if z.players[p.id] != p {
			return
		}
		for _, def := range unlocked {
			z.system(p, "Conquista desbloqueada: "+def.Name+"! Novo título: «"+def.Title+"» (escolha na janela Glória).")
		}
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Achievements{Achievements: &pb.Achievements{
			List: changed, TitleId: a.title, Unlocked: unlocked[len(unlocked)-1].Name,
		}}})
	})
}

// achInfo: full = com os textos (catálogo); senão só id, progresso e desbloqueio.
func (z *Zone) achInfo(p *entity, def *game.AchievementDef, full bool) *pb.AchievementInfo {
	a := &p.pl.ach
	info := &pb.AchievementInfo{Id: def.ID, Need: uint32(def.Count), Count: uint32(min(def.Progress(a.counts[def.ID]), def.Count))}
	if at, ok := a.unlocked[def.ID]; ok {
		info.Unlocked, info.UnlockedAtMs, info.Count = true, at, uint32(def.Count)
	}
	if def.Threshold() {
		info.Need = 0 // nível, +N e raridade: não há barra de progresso
		info.Count = 0
	}
	if full {
		info.Name, info.Text, info.Title = def.Name, def.Text, def.Title
	}
	return info
}

// achOnJoin manda o catálogo e desbloqueia o que o personagem já cumpriu antes das
// conquistas existirem (nível, vitórias PvP, tutorial).
func (z *Zone) achOnJoin(p *entity) {
	list := make([]*pb.AchievementInfo, 0, len(game.AchievementList))
	for _, def := range game.AchievementList {
		list = append(list, z.achInfo(p, def, true))
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Achievements{Achievements: &pb.Achievements{
		Full: true, List: list, TitleId: p.pl.ach.title,
	}}})
	z.achValue(p, game.AchLevel, "", p.level)
	if p.pl.pvpKills > 0 {
		z.achValue(p, game.AchPvPWin, "", p.pl.pvpKills)
	}
	for id := range p.pl.quests.done {
		z.achEvent(p, game.AchQuest, id)
	}
}

// onAchievementCmd: escolher (ou tirar) o título.
func (z *Zone) onAchievementCmd(p *entity, m *pb.AchievementCmd) {
	if m.Kind != pb.AchievementCmdKind_ACHIEVEMENT_CMD_SET_TITLE {
		return
	}
	z.setTitle(p, m.Id)
}

// setTitle troca o título: só de uma conquista desbloqueada ("" tira). Todos veem.
func (z *Zone) setTitle(p *entity, id string) {
	a := &p.pl.ach
	if id != "" {
		if _, ok := a.unlocked[id]; !ok && !z.bestiaryHasTitle(p, id) {
			z.system(p, "Você ainda não tem esse título.")
			return
		}
	}
	if id == a.title {
		return
	}
	a.title = id
	z.saveSoon(p)
	z.broadcastStatus(p)
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Achievements{Achievements: &pb.Achievements{TitleId: id}}})
	if id == "" {
		z.system(p, "Você está sem título.")
	} else {
		z.system(p, "Seu título agora é «"+a.titleText()+"».")
	}
}

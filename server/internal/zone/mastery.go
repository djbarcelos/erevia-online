package zone

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Maestria (game/mastery.go): no nível máximo (40) a XP enche a barra de Maestria e cada
// barra cheia dá 1 ponto para a árvore. Quem ganhou pontos no 30 (o máximo de antes) ainda
// usa a árvore do 30 ao 39, mas a XP volta a subir nível. Nada roda por tick: o estado só muda quando o jogador
// ganha XP, morre, gasta ou redistribui. Os efeitos ficam somados em masteryState.bonus
// (recalculado quando a árvore muda) e cada sistema lê só o seu valor.

// masteryState: a Maestria do jogador em memória.
type masteryState struct {
	st    game.MasteryState
	bonus game.MasteryBonus
}

// of: o valor de um efeito da árvore (0 = nenhum ponto nele).
func (m *masteryState) of(s game.MasteryStat) float64 { return m.bonus.Of(s) }

// save: cópia para o salvamento (o mapa vai para outra goroutine).
func (m *masteryState) save() game.MasteryState {
	out := m.st
	out.Ranks = maps.Clone(m.st.Ranks)
	return out
}

func loadMastery(st game.MasteryState) masteryState {
	m := masteryState{st: game.NormalMastery(st)}
	m.bonus = m.st.Bonus()
	return m
}

// masteryDay: hoje no fuso dos eventos (Brasília, o mesmo da XP em dobro).
func (z *Zone) masteryDay() int { return game.MasteryDay(z.cfg.Clock(), z.eventLoc()) }

// masteryTree: a árvore, montada uma vez (dados fixos).
var masteryTree = func() *pb.MasteryTree {
	t := &pb.MasteryTree{NodeMax: game.MasteryNodeMax, CombatCap: game.MasteryCombatCap}
	for _, b := range game.MasteryBranches {
		t.Branches = append(t.Branches, &pb.MasteryBranchInfo{Name: b.Name, Description: b.Description, Color: b.Color})
	}
	for _, n := range game.MasteryNodes {
		info := &pb.MasteryNodeInfo{Id: n.ID, Name: n.Name, Branch: uint32(n.Branch), Req: uint32(n.Req), Icon: n.Icon, Combat: n.Stat.Combat()}
		for r := 1; r <= game.MasteryNodeMax; r++ {
			info.Ranks = append(info.Ranks, n.Text(r))
		}
		t.Nodes = append(t.Nodes, info)
	}
	return t
}()

func (z *Zone) masteryPB(p *entity) *pb.MasteryState {
	m := &p.pl.mastery
	st := &m.st
	out := &pb.MasteryState{
		Active: p.level >= game.MasteryLevel,
		Level:  uint32(st.Points), Free: uint32(st.Free()),
		Xp: uint64(st.XP), Need: uint64(game.MasteryPointXP),
		Rested: uint64(st.Rested), RestedMax: uint64(game.RestedCap()),
		RestedMult: game.MasteryRestedMult, RestedDays: game.MasteryRestedDays,
		RespecReady:      st.CanRespec(z.masteryDay()),
		MaxLevel:         uint32(game.MasteryMaxPoints),
		TeleportDiscount: float32(m.of(game.MStatTeleport)),
		OpenLevel:        game.MasteryLevel,
	}
	for _, n := range game.MasteryNodes { // na ordem da árvore
		if r := st.Ranks[n.ID]; r > 0 {
			out.Ranks = append(out.Ranks, &pb.MasteryRank{Node: n.ID, Rank: uint32(r)})
		}
	}
	return out
}

func (z *Zone) sendMastery(p *entity) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Mastery{Mastery: z.masteryPB(p)}})
}

// masteryOnJoin: no nível máximo, credita os dias de bônus que passaram e manda o estado (o
// HUD troca a barra de XP pela de Maestria). Abaixo dele, só manda para quem já tem pontos
// (a aba da árvore continua valendo).
func (z *Zone) masteryOnJoin(p *entity) {
	st := &p.pl.mastery.st
	if !st.Usable(p.level) {
		return
	}
	if p.level >= game.MasteryLevel {
		st.Refresh(z.masteryDay())
	}
	z.sendMastery(p)
}

// masteryOpened: acabou de chegar ao nível máximo. A XP que passou do nível já vai para a
// barra.
func (z *Zone) masteryOpened(p *entity) {
	extra := p.pl.xp - game.XPLevelStart(game.MasteryLevel)
	if extra > 0 {
		p.pl.xp -= extra
	}
	p.pl.mastery.st.Refresh(z.masteryDay())
	z.systemAfterFlush(p, fmt.Sprintf("Nível máximo! Agora a XP enche a barra de Maestria: cada barra cheia dá 1 ponto para a árvore (aba Maestria das skills). A primeira barra de cada dia sai %d× mais rápido.", game.MasteryRestedMult))
	if extra > 0 {
		z.grantMasteryXP(p, extra)
		return
	}
	z.sendMastery(p)
}

// grantMasteryXP: XP ganha no nível máximo (já com grupo, diferença de nível e XP em dobro).
func (z *Zone) grantMasteryXP(p *entity, xp int64) {
	st := &p.pl.mastery.st
	if xp <= 0 || st.Points >= game.MasteryMaxPoints {
		return
	}
	st.Refresh(z.masteryDay())
	credited, points := st.Gain(xp)
	text := fmt.Sprintf("+%d XP de Maestria", credited)
	if credited > xp {
		text += fmt.Sprintf(" (bônus diário ×%d)", game.MasteryRestedMult)
	}
	z.systemAfterFlush(p, text)
	if points > 0 {
		msg := fmt.Sprintf("Você ganhou 1 ponto de Maestria! Agora é Maestria %d.", st.Points)
		if points > 1 {
			msg = fmt.Sprintf("Você ganhou %d pontos de Maestria! Agora é Maestria %d.", points, st.Points)
		}
		msg += " Gaste na aba Maestria da janela de skills (K)."
		if st.Points >= game.MasteryMaxPoints {
			msg += " É o teto: a árvore inteira cabe nos seus pontos."
		}
		z.systemAfterFlush(p, msg)
		z.trackMastery(p, st.Points, points)
		z.saveSoon(p)
		z.log.Info("mastery point", "id", p.id, "name", p.name, "mastery", st.Points)
	}
	z.afterFlush(func() { // junto com o "+XP", depois do golpe que matou
		if z.players[p.id] == p {
			z.sendMastery(p)
		}
	})
}

// masteryDeath: morrer no nível máximo tira game.DeathLossShare da barra (menos o desconto
// da Segunda Chance), times vezes (karma: o dobro), sem tirar ponto. Devolve quanto saiu.
func (z *Zone) masteryDeath(p *entity, times int) int64 {
	m := &p.pl.mastery
	lost := m.st.Lose(m.st.DeathLoss(&m.bonus) * int64(times))
	if lost > 0 {
		z.sendMastery(p)
	}
	return lost
}

func (z *Zone) onMasteryCmd(p *entity, m *pb.MasteryCmd) {
	st := &p.pl.mastery.st
	switch m.Kind {
	case pb.MasteryCmdKind_MASTERY_CMD_GET:
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_MasteryTree{MasteryTree: masteryTree}})
		if p.level >= game.MasteryLevel {
			st.Refresh(z.masteryDay())
		}
		z.sendMastery(p)
	case pb.MasteryCmdKind_MASTERY_CMD_SPEND:
		if !z.masteryOpen(p) {
			return
		}
		if why := st.Spend(m.Node); why != "" {
			z.system(p, why)
			z.sendMastery(p) // a janela volta ao que vale
			return
		}
		z.masteryChanged(p)
	case pb.MasteryCmdKind_MASTERY_CMD_RESPEC:
		if !z.masteryOpen(p) {
			return
		}
		if why := st.Respec(z.masteryDay()); why != "" {
			z.system(p, why)
			z.sendMastery(p)
			return
		}
		z.masteryChanged(p)
		z.track(p, "mastery", "respec", int64(st.Points), nil)
		z.system(p, "Os pontos de Maestria voltaram. O próximo Redistribuir grátis libera amanhã (meia-noite de Brasília).")
	}
}

// masteryOpen: gastar e redistribuir no nível máximo, ou com pontos de antes (ganhos quando
// o máximo era 30).
func (z *Zone) masteryOpen(p *entity) bool {
	if !p.pl.mastery.st.Usable(p.level) {
		z.system(p, fmt.Sprintf("A Maestria abre no nível %d.", game.MasteryLevel))
		return false
	}
	return true
}

// masteryChanged: a árvore mudou. Soma os efeitos de novo e recalcula os atributos (vida
// máxima e dano).
func (z *Zone) masteryChanged(p *entity) {
	m := &p.pl.mastery
	m.bonus = m.st.Bonus()
	z.refreshStats(p)
	z.sendMastery(p)
	z.saveSoon(p)
}

// applyMasteryStats: os dois efeitos de combate da árvore (sempre dentro do teto de 5%).
func applyMasteryStats(s game.Stats, p *entity) game.Stats {
	b := &p.pl.mastery.bonus
	if v := b.Of(game.MStatMaxHP); v > 0 {
		s.MaxHP *= 1 + v
	}
	if v := b.Of(game.MStatDamage); v > 0 {
		s.PAtk *= 1 + v
		s.MAtk *= 1 + v
	}
	return s
}

// teleportPrice: o preço da viagem com o desconto do Viajante.
func (z *Zone) teleportPrice(p *entity, price int64) int64 {
	if v := p.pl.mastery.of(game.MStatTeleport); v > 0 {
		return int64(float64(price)*(1-v) + 0.5)
	}
	return price
}

// channelTime: a canalização da pedra ou do pergaminho com a Leitura Rápida.
func (z *Zone) channelTime(p *entity, seconds float64) float64 {
	return seconds * (1 - p.pl.mastery.of(game.MStatChannel))
}

// itemCooldown: a recarga de um grupo de consumível (o Pergaminho de Retorno com o
// Caminho de Casa).
func (z *Zone) itemCooldown(p *entity, group int) float64 {
	s := game.CooldownSeconds(group)
	if group == game.CooldownReturn && p.pl != nil {
		s *= 1 - p.pl.mastery.of(game.MStatReturnCD)
	}
	return s
}

// masteryLoot: Bolsa Farta (moedas) e Olho do Coletor (material extra, só de monstros
// comuns) para quem leva o saque. Só sorteia quem tem ponto no nó.
func (z *Zone) masteryLoot(owner uint32, def *game.MonsterDef, drops []game.Drop) {
	p := z.players[owner]
	if p == nil {
		return
	}
	gold, mat := p.pl.mastery.of(game.MStatGold), p.pl.mastery.of(game.MStatMaterial)
	if gold == 0 && mat == 0 {
		return
	}
	for i := range drops {
		d := &drops[i]
		switch {
		case d.Item.Kind == game.KindGold && gold > 0:
			d.Count = int(float64(d.Count)*(1+gold) + 0.5)
		case d.Item.Kind == game.KindMaterial && mat > 0 && !def.Boss && !def.Raid:
			if z.rng.Float64() < mat {
				d.Count++
			}
		}
	}
}

// gmMastery: /maestria xp N [Nome] (dá XP como se fosse caçada, com o bônus diário) e
// /maestria dia [Nome] (simula a virada do dia: +1 dia de bônus e o Redistribuir livre).
func (z *Zone) gmMastery(gm *entity, args []string) {
	usage := "Uso: /maestria xp N [Nome] · /maestria dia [Nome]"
	if len(args) < 1 {
		z.system(gm, usage)
		return
	}
	t, rest := gm, args[1:]
	pick := func(i int) bool {
		if len(rest) > i {
			if t = z.online(gm, rest[i]); t == nil {
				return false
			}
		}
		return true
	}
	switch strings.ToLower(args[0]) {
	case "xp":
		if len(rest) < 1 {
			z.system(gm, usage)
			return
		}
		n, err := strconv.ParseInt(rest[0], 10, 64)
		if err != nil || n < 1 {
			z.system(gm, "Quantidade inválida.")
			return
		}
		if !pick(1) {
			return
		}
		n = min(n, game.MasteryPointXP*int64(game.MasteryMaxPoints))
		z.grantXP(t, n)
		z.system(gm, fmt.Sprintf("%s recebeu %d de XP (Maestria %d).", t.name, n, t.pl.mastery.st.Points))
		z.gmLog(gm, "maestria", t.name, fmt.Sprintf("xp %d", n))
	case "dia":
		if !pick(0) {
			return
		}
		st := &t.pl.mastery.st
		if st.Day > 0 {
			st.Day--
		}
		st.RespecDay = 0
		if t.level >= game.MasteryLevel {
			st.Refresh(z.masteryDay())
		}
		z.sendMastery(t)
		z.system(gm, fmt.Sprintf("%s: dia virado (bônus guardado %d de XP, Redistribuir livre).", t.name, st.Rested))
		z.gmLog(gm, "maestria", t.name, "dia")
	default:
		z.system(gm, usage)
	}
}

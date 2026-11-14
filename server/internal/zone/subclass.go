package zone

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Subclasse (tunnel#156, game/subclass.go). Edrian, o Guardião das Sendas, ao sul do poço,
// conduz a cadeia de missões que chega com a classe principal no 30 (o chamado, a prova
// e o juramento) e, depois dela, a janela das classes: escolher a subclasse (uma outra
// classe base, que começa no 20) e trocar a classe ativa.
//
// A classe ativa mora nos campos de sempre do jogador (pl.class, pl.xp, pl.attrs,
// pl.ranks, pl.hotbar, pl.spec e o nível da entidade): combate, skills, equipamento,
// invariantes e salvamento continuam lendo os mesmos campos. As outras classes ficam em
// pl.classes; a troca guarda a ativa lá e traz a nova para os campos. Nada roda por tick.

// guardian: Edrian, o Guardião das Sendas, ao sul do poço (o Barbeiro fica ao norte).
var guardian = npcDef{name: game.TalkGuardian, title: "Guardião das Sendas", model: "knight", x: 0, z: -9.5,
	role: pb.NpcRole_NPC_ROLE_SUBCLASS, look: &game.Appearance{Skin: 4, Hair: 5, Outfit: 6, Head: "ranger"}}

// guardianID: depois do Leiloeiro (o último NPC da cidade).
var guardianID = auctioneerID + 1

// spawnGuardian: o Guardião só existe no mapa principal (a troca de classe é na cidade).
func (z *Zone) spawnGuardian(id uint32) {
	if z.mapID() != world.StarterMap {
		return
	}
	d := guardian
	pos := z.freeSpot(Vec2{X: d.x, Z: d.z})
	z.npcs[id] = &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_NPC,
		name: d.name, model: d.model, level: 1, scale: 1,
		pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
		npc: &npcState{title: d.title, role: d.role, look: d.look},
	}
}

// classTreePB: a árvore da classe (ativas e passivas, mais as skills dos dois caminhos: a
// janela da Mestra mostra os dois; a árvore, o escolhido) e os nomes dos 3 ramos. Vai no
// Welcome e, depois de uma troca de classe, no SubclassState.
func classTreePB(class string) ([]*pb.SkillInfo, []*pb.SkillBranch) {
	var skills []*pb.SkillInfo
	for _, s := range game.ClassTree(class) {
		skills = append(skills, skillInfoPB(s))
	}
	for _, sp := range game.ClassSpecs(class) {
		for _, s := range game.SpecTree(sp.ID) {
			skills = append(skills, skillInfoPB(s))
		}
	}
	var branches []*pb.SkillBranch
	if c := game.Classes[class]; c != nil {
		for _, b := range c.Branches {
			branches = append(branches, &pb.SkillBranch{Name: b.Name, Description: b.Description, Color: b.Color})
		}
	}
	return skills, branches
}

// storeActive copia a progressão da classe ativa (os campos do jogador) para a entrada
// dela em set.
func storeActive(p *entity, set *game.ClassSet) {
	if c := set.Find(p.pl.class); c != nil {
		c.Level, c.XP, c.Attrs, c.Spec = p.level, p.pl.xp, p.pl.attrs, p.pl.spec
		c.Ranks, c.Hotbar = maps.Clone(p.pl.ranks), slices.Clone(p.pl.hotbar)
	}
}

// classSnapshot: as classes para o salvamento, com a ativa como está agora.
func classSnapshot(p *entity) game.ClassSet {
	set := p.pl.classes.Clone()
	storeActive(p, &set)
	return set
}

// switchLeft: quanto falta para poder trocar de classe de novo (0 = livre).
func (z *Zone) switchLeft(p *entity) time.Duration {
	at := p.pl.classes.SwitchedAt
	if at <= 0 {
		return 0
	}
	return max(0, time.Unix(at, 0).Add(game.SubclassSwitchCooldown).Sub(z.cfg.Clock()))
}

// canChooseSubclass: o juramento foi feito e ainda há vaga de subclasse.
func canChooseSubclass(p *entity) bool {
	return p.pl.quests.done[game.SubclassQuestOath] && p.pl.classes.Subclasses() < game.MaxSubclasses
}

// subclassPB: as classes do personagem. npcID abre a janela do Guardião; switched leva a
// árvore da classe ativa (o cliente troca a dele).
func (z *Zone) subclassPB(p *entity, npcID uint32, switched bool) *pb.SubclassState {
	set := &p.pl.classes
	out := &pb.SubclassState{
		NpcId: npcID, Main: set.Main, Active: p.pl.class, CanChoose: canChooseSubclass(p),
		MaxSubclasses:   game.MaxSubclasses,
		CooldownMs:      uint32(z.switchLeft(p).Milliseconds()),
		CooldownTotalMs: uint32(game.SubclassSwitchCooldown.Milliseconds()),
		Bonus:           set.BonusUnlocked(p.level),
		BonusXp:         game.SubclassXPBonus,
		BonusLevel:      game.SubclassBonusLevel,
		StartLevel:      game.SubclassStartLevel,
		QuestLevel:      game.SubclassQuestLevel,
		Switched:        switched,
	}
	for i, c := range set.List {
		lv, spec := c.Level, c.Spec
		if c.Class == p.pl.class {
			lv, spec = p.level, p.pl.spec
		}
		out.Classes = append(out.Classes, &pb.ClassLevel{Class: c.Class, Level: uint32(lv), Main: i == 0, Spec: spec})
	}
	if switched {
		out.Skills, out.Branches = classTreePB(p.pl.class)
	}
	return out
}

func (z *Zone) sendSubclass(p *entity, npcID uint32, switched bool) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Subclass{Subclass: z.subclassPB(p, npcID, switched)}})
}

// subclassOnJoin: o cliente fica sabendo das classes (a principal e a ativa) e o título
// Duas Sendas confere o nível das subclasses (de quem chegou ao 30 antes da conquista).
func (z *Zone) subclassOnJoin(p *entity) {
	z.sendSubclass(p, 0, false)
	if top := p.pl.classes.TopSubclassLevel(p.level); top > 0 {
		z.achValue(p, game.AchSubclass, "", top)
	}
}

func (z *Zone) onSubclassCmd(p *entity, m *pb.SubclassCmd) {
	if p.dead {
		return
	}
	n := z.npcs[m.NpcId]
	if n == nil || n.npc.role != pb.NpcRole_NPC_ROLE_SUBCLASS || z.shopNPC(p, n.id) == nil {
		return
	}
	switch m.Kind {
	case pb.SubclassCmdKind_SUBCLASS_CMD_TALK:
		z.talkGuardian(p, n)
	case pb.SubclassCmdKind_SUBCLASS_CMD_CHOOSE:
		z.chooseSubclass(p, n, m.Class)
	case pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH:
		z.switchClass(p, n, m.Class)
	}
}

// talkGuardian anda a missão (os passos de falar com ele) e abre a janela das classes para
// quem já tem subclasse ou acabou de fazer o juramento; os outros recebem um conselho.
func (z *Zone) talkGuardian(p *entity, n *entity) {
	z.questUnlock(p) // quem chegou ao 30 antes da missão existir
	z.questEvent(p, game.GoalTalk, game.TalkGuardian)
	if p.pl.classes.Subclasses() > 0 || canChooseSubclass(p) {
		z.sendSubclass(p, n.id, false)
		return
	}
	switch {
	case p.pl.quests.has(game.SubclassQuestTrial):
		z.system(p, fmt.Sprintf("%s: «Derrote os Golens Ancestrais no Fundo do Vale e volte. Uma segunda senda pede um braço firme.»", n.name))
	case p.level < game.SubclassQuestLevel:
		z.system(p, fmt.Sprintf("%s: «Domine primeiro a sua classe. Quando chegar ao nível %d, eu mostro as outras sendas.»", n.name, game.SubclassQuestLevel))
	default:
		z.system(p, fmt.Sprintf("%s: «As sendas estão abertas para você. Siga o Diário (J).»", n.name))
	}
}

// chooseSubclass: depois do juramento, uma outra classe base vira subclasse (no nível 20,
// sem pontos gastos). A classe ativa não muda: a troca é outro pedido (switchClass).
func (z *Zone) chooseSubclass(p *entity, n *entity, class string) {
	set := &p.pl.classes
	def := game.Classes[class]
	switch {
	case def == nil:
		return
	case class == set.Main:
		z.system(p, fmt.Sprintf("%s já é a sua classe principal: escolha uma outra classe.", def.Name))
		return
	case set.Has(class):
		z.system(p, fmt.Sprintf("Você já tem a subclasse %s.", def.Name))
		return
	case set.Subclasses() >= game.MaxSubclasses:
		z.system(p, "Você já tem uma subclasse.")
		return
	case !p.pl.quests.done[game.SubclassQuestOath]:
		z.system(p, fmt.Sprintf("Faça o juramento com %s antes de escolher (missões da história, no Diário).", n.name))
		return
	}
	set.List = append(set.List, game.NewSubclass(class))
	z.saveSoon(p)
	z.trackSubclass(p, class)
	z.log.Info("subclass", "id", p.id, "name", p.name, "class", class, "main", set.Main)
	z.system(p, fmt.Sprintf("Você ganhou a subclasse %s! Ela começa no nível %d: troque de classe aqui com %s quando quiser (fora de combate, a cada %d minutos).",
		def.Name, game.SubclassStartLevel, n.name, int(game.SubclassSwitchCooldown.Minutes())))
	z.sendSubclass(p, n.id, false)
}

// switchClass troca a classe ativa: só vivo, fora de combate, sem troca de itens aberta e
// depois da recarga. O equipamento que a classe nova não usa (outra classe ou nível acima
// do dela) vai para a bolsa; sem espaço, a troca é recusada.
func (z *Zone) switchClass(p *entity, n *entity, class string) {
	pl := p.pl
	set := &pl.classes
	def := game.Classes[class]
	target := set.Find(class)
	switch {
	case def == nil || target == nil:
		return
	case class == pl.class:
		z.system(p, fmt.Sprintf("Você já está na classe %s.", def.Name))
		return
	case z.now-p.lastCombat <= outOfCombatAfter || pl.flagUntil > z.now:
		z.system(p, "Saia de combate para trocar de classe.")
		return
	case pl.trade != nil:
		z.system(p, "Termine a troca de itens antes de mudar de classe.")
		return
	}
	if left := z.switchLeft(p); left > 0 {
		z.system(p, fmt.Sprintf("Você trocou de classe há pouco: espere mais %s.", durationText(left.Seconds())))
		z.sendSubclass(p, n.id, false)
		return
	}
	level := game.LevelForXP(target.XP)
	inv := &pl.inv
	var off []int // encaixes com o que a classe nova não usa
	for i := range inv.equip {
		if s := inv.equip[i]; !s.empty() && canEquip(s.def, i, class, level) != "" {
			off = append(off, i)
		}
	}
	free := 0
	for _, s := range inv.bag {
		if s.empty() {
			free++
		}
	}
	if len(off) > free {
		z.system(p, fmt.Sprintf("Sua bolsa está cheia: libere %d espaço(s) para guardar o equipamento que a classe %s não pode usar.", len(off)-free, def.Name))
		return
	}

	from, fromLevel := pl.class, p.level
	z.endXPTally(p) // a XP somada até aqui fica com a classe e o nível de antes
	z.cancelChannel(p, "")
	p.clearAction()
	storeActive(p, set)
	for _, i := range off {
		b := inv.firstFree()
		inv.bag[b], inv.equip[i] = inv.equip[i], stack{}
	}
	hpFrac, mpFrac := barFrac(p.hp, p.stats.MaxHP), barFrac(p.mp, p.stats.MaxMP)
	spec := ""
	if game.SpecFor(class, target.Spec) != nil {
		spec = target.Spec
	}
	pl.class, pl.xp, p.level, pl.spec = class, target.XP, level, spec
	pl.attrs, pl.ranks, pl.hotbar = loadProgress(class, spec, level, target.Attrs, target.Ranks, target.Hotbar)
	set.Active = class
	set.SwitchedAt = z.cfg.Clock().Unix()
	for id := range pl.cooldowns { // as recargas das skills da classe de antes saem
		if game.Skills[id] != nil && id < itemCooldownBase {
			delete(pl.cooldowns, id)
		}
	}
	// Os efeitos que ele pôs em si mesmo com skills da classe de antes também (os de
	// aliados, como bênçãos do grupo, ficam).
	kept := p.effects[:0]
	for _, ef := range p.effects {
		if ef.source == p.id {
			z.frame.ends = append(z.frame.ends, &pb.EffectEnd{TargetId: p.id, Effect: pb.Effect(ef.kind)})
			continue
		}
		kept = append(kept, ef)
	}
	p.effects = kept
	pl.lastStandReady, pl.ambushCrit = 0, false
	z.refreshStats(p)
	p.hp, p.mp = math.Max(1, hpFrac*p.stats.MaxHP), mpFrac*p.stats.MaxMP

	// A árvore nova chega antes do Progress e da barra, para o cliente montar as janelas.
	z.sendSubclass(p, n.id, true)
	z.sendProgress(p)
	z.sendHotbar(p)
	z.inventoryChanged(p, len(off) > 0)
	z.sendMastery(p)
	z.vitalityLevelUp(p) // o teto da vitalidade é o do nível da classe ativa
	z.sendQuests(p, "")  // o lugar dos objetivos depende do nível
	z.questLevel(p)
	z.broadcastStatus(p)
	z.classChangedSocial(p)
	z.trackClassSwitch(p, from, fromLevel, len(off))
	z.log.Info("class switch", "id", p.id, "name", p.name, "class", class, "level", level, "from", from)
	msg := fmt.Sprintf("Agora você é %s, nível %d.", def.Name, level)
	if len(off) > 0 {
		msg += fmt.Sprintf(" %d peça(s) de equipamento que esta classe não usa foram para a bolsa.", len(off))
	}
	z.system(p, msg)
}

// classChangedSocial: o grupo e o clã mostram a classe e o nível de cada membro.
func (z *Zone) classChangedSocial(p *entity) {
	if pt := p.pl.party; pt != nil {
		z.sendParty(pt)
	}
	if c := p.pl.clan; c != nil {
		if m := c.members[int64(p.id)]; m != nil {
			m.class, m.level = p.pl.class, p.level
			z.sendClan(c)
		}
	}
}

// barFrac: quanto da barra está cheio (0–1).
func barFrac(v, maxV float64) float64 {
	if maxV <= 0 {
		return 1
	}
	return math.Min(1, math.Max(0, v/maxV))
}

// subclassXP: o bônus Duas Sendas sobre a XP da classe ativa (0 = nenhum).
func subclassXP(p *entity, xp int64) int64 {
	m := p.pl.classes.XPMult(p.level)
	if m <= 1 || xp <= 0 {
		return 0
	}
	return int64(math.Round(float64(xp) * (m - 1)))
}

// gmSubclass: /subclasse passo [Nome] (cumpre o passo da missão da subclasse em que o
// jogador está), /subclasse recarga [Nome] (libera a troca agora) e /subclasse xp N [Nome]
// (XP para a classe ativa, como caçada).
func (z *Zone) gmSubclass(gm *entity, args []string) {
	usage := "Uso: /subclasse passo [Nome] · /subclasse recarga [Nome] · /subclasse xp N [Nome]"
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
	case "passo":
		if !pick(0) {
			return
		}
		z.gmSubclassStep(gm, t)
	case "recarga":
		if !pick(0) {
			return
		}
		t.pl.classes.SwitchedAt = 0
		z.saveSoon(t)
		z.system(gm, t.name+" pode trocar de classe agora.")
		z.gmLog(gm, "subclasse", t.name, "recarga")
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
		rest = rest[1:]
		if !pick(0) {
			return
		}
		n = min(n, game.XPLevelStart(game.MaxLevel))
		z.grantXP(t, n)
		z.system(gm, fmt.Sprintf("%s recebeu %d de XP (%s, nível %d).", t.name, n, game.Classes[t.pl.class].Name, t.level))
		z.gmLog(gm, "subclasse", t.name, fmt.Sprintf("xp %d", n))
	default:
		z.system(gm, usage)
	}
}

// gmSubclassStep cumpre o passo atual da cadeia (o chamado ainda não chegou: entrega).
func (z *Zone) gmSubclassStep(gm, t *entity) {
	for _, id := range []string{game.SubclassQuestCall, game.SubclassQuestTrial, game.SubclassQuestOath} {
		if !t.pl.quests.has(id) {
			continue
		}
		z.questUpdate(t, func(s game.QuestStep, cur int) int {
			if game.Quests[id] != nil && slices.Contains(game.Quests[id].Steps, s) {
				return s.Count
			}
			return cur
		})
		z.system(gm, fmt.Sprintf("%s: passo de «%s» cumprido.", t.name, game.Quests[id].Title))
		z.gmLog(gm, "subclasse", t.name, "passo "+id)
		return
	}
	if t.pl.quests.done[game.SubclassQuestOath] {
		z.system(gm, t.name+" já fez o juramento.")
		return
	}
	def := game.Quests[game.SubclassQuestCall]
	t.pl.quests.active = append(t.pl.quests.active, &activeQuest{def: def, progress: make([]int, len(def.Steps))})
	z.saveSoon(t)
	z.sendQuests(t, "")
	z.system(gm, fmt.Sprintf("%s recebeu «%s».", t.name, def.Title))
	z.gmLog(gm, "subclasse", t.name, "chamado")
}

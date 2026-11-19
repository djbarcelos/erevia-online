package zone

import (
	"fmt"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Especialização (segunda classe). A Mestra dos Caminhos, na praça, conduz a missão "A
// Encruzilhada": falar com ela (nível 20+) abre a janela com os dois caminhos da classe, e
// escolher conclui a missão. A escolha fica salva no personagem e vai para todos no
// Entity.spec e no PlayerStatus (estado do jogador, como o título: muda só quando ele
// escolhe ou troca). Nada roda por tick.

// master: Ysolde, a Mestra dos Caminhos, do lado leste do poço (o Otto fica no oeste), a
// 4,5 m da Pedra de Teleporte da praça (7, 7): perto demais, o clique pegava um pelo outro.
var master = npcDef{name: "Ysolde", title: "Mestra dos Caminhos", model: "mage", x: 9.5, z: 2.5,
	role: pb.NpcRole_NPC_ROLE_MASTER, look: &game.Appearance{Skin: 3, Hair: 5, Outfit: 6, Head: "mage"}}

func (z *Zone) spawnMaster(id uint32) {
	d := master
	pos := z.freeSpot(Vec2{X: d.x, Z: d.z})
	z.addNPC(&entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_NPC,
		name: d.name, model: d.model, level: 1, scale: 1,
		pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
		npc: &npcState{title: d.title, role: d.role, look: d.look},
	})
}

// specCatalog: as 8 especializações (o cliente mostra o nome e a cor das dos outros).
func specCatalog() []*pb.SpecInfo {
	out := make([]*pb.SpecInfo, 0, len(game.SpecList))
	for _, s := range game.SpecList {
		out = append(out, &pb.SpecInfo{
			Id: s.ID, Class: s.Class, Name: s.Name, Description: s.Description, Color: s.Color,
			Bonus: attrsPB(s.Bonus), BonusText: s.BonusText(),
			Weapons: s.Weapons, Offhands: s.Offhands, StarterSkills: s.StarterSkills[:],
		})
		if r := s.Resource; r != nil {
			out[len(out)-1].Resource = &pb.ResourceInfo{Name: r.Name, Color: r.Color, Max: uint32(r.Max)}
		}
		for _, b := range s.Branches {
			out[len(out)-1].Branches = append(out[len(out)-1].Branches, &pb.SkillBranch{Name: b.Name, Description: b.Description, Color: b.Color})
		}
	}
	return out
}

func (z *Zone) onSpecCmd(p *entity, m *pb.SpecCmd) {
	if p.dead {
		return
	}
	n := z.npcs[m.NpcId]
	if n == nil || n.npc.role != pb.NpcRole_NPC_ROLE_MASTER || z.shopNPC(p, n.id) == nil {
		return
	}
	switch m.Kind {
	case pb.SpecCmdKind_SPEC_CMD_TALK:
		z.talkMaster(p, n)
	case pb.SpecCmdKind_SPEC_CMD_CHOOSE:
		z.chooseSpec(p, n, m.Spec)
	case pb.SpecCmdKind_SPEC_CMD_RESPEC:
		z.respecTree(p, n)
	}
}

// talkMaster: abaixo do 20 ela só dá um conselho; do 20 em diante anda a missão e abre
// a janela da escolha (ou da troca, para quem já escolheu).
func (z *Zone) talkMaster(p *entity, n *entity) {
	if p.level < game.SpecLevel {
		z.system(p, fmt.Sprintf("%s: «Volte quando chegar ao nível %d. Aí conversamos sobre o seu caminho.»", n.name, game.SpecLevel))
		return
	}
	z.questUnlock(p) // quem chegou ao 20 antes da missão existir
	z.questEvent(p, game.GoalTalk, game.TalkMaster)
	z.sendSpecChoice(p, n)
}

// sendSpecChoice abre (ou atualiza) a janela da Ysolde com os custos no nível de hoje.
func (z *Zone) sendSpecChoice(p *entity, n *entity) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_SpecChoice{SpecChoice: &pb.SpecChoice{
		NpcId: n.id, Current: p.pl.spec, ChangeCost: uint64(game.EvolutionChangeCost(p.level)),
		RespecCost: uint64(game.RespecCost(p.level)),
	}}})
}

// chooseSpec: evoluir (a primeira escolha precisa da missão) ou trocar de evolução
// (EvolutionChangeCost). Nos dois casos a árvore em uso (a base ou a da evolução antiga)
// some, os pontos voltam, entram as 2 iniciais da evolução nas teclas das iniciais de antes,
// o equipamento que ela não usa vai para a bolsa e os itens iniciais saem uma vez.
func (z *Zone) chooseSpec(p *entity, n *entity, id string) {
	def := game.SpecFor(p.pl.class, id)
	switch {
	case def == nil:
		return
	case p.level < game.SpecLevel:
		z.system(p, fmt.Sprintf("A evolução chega no nível %d.", game.SpecLevel))
		return
	case def.ID == p.pl.spec:
		z.system(p, "Você já é "+def.Name+".")
		return
	}
	items := z.pendingStarterItems(p, def)
	// Equipamento que a evolução não usa vai para a bolsa, e os itens iniciais também
	// precisam de espaço: sem espaço, nada muda.
	if need := len(p.pl.inv.invalidGear(p.pl.class, def.ID, p.level)) + len(items); need > p.pl.inv.freeSlots() {
		z.system(p, fmt.Sprintf("Sua bolsa está cheia: libere %d espaço(s) para evoluir para %s.", need-p.pl.inv.freeSlots(), def.Name))
		return
	}
	old := p.pl.spec
	cost := game.EvolutionChangeCost(p.level)
	if old == "" {
		if !p.pl.quests.has(game.SpecQuest) && !p.pl.quests.done[game.SpecQuest] {
			z.system(p, "Fale com "+n.name+" para começar a missão A Encruzilhada.")
			return
		}
	} else {
		if p.pl.trade != nil {
			z.system(p, "Termine a troca antes de mudar de evolução.")
			return
		}
		if p.pl.inv.gold < cost {
			z.system(p, fmt.Sprintf("Trocar de evolução custa %d moedas.", cost))
			return
		}
		p.pl.inv.gold -= cost
		z.trackGold(p, "spec_change", -cost)
		z.inventoryChanged(p, false)
	}
	prevStarters := starterIDs(p.pl.class, old)
	p.pl.spec = def.ID
	if moved, _ := p.pl.inv.dropInvalidGear(p.pl.class, def.ID, p.level); moved > 0 {
		z.inventoryChanged(p, true)
	}
	z.giveStarterItems(p, def, items)
	z.resetTree(p)
	resetRes(p)
	// As iniciais de antes passam a tecla para as iniciais da evolução.
	for i, e := range p.pl.hotbar {
		for k, from := range prevStarters {
			if e == skillEntry(from) {
				p.pl.hotbar[i] = skillEntry(def.StarterSkills[k])
			}
		}
	}
	z.cleanHotbar(p)
	z.refreshStats(p) // bônus de atributos e passivas
	z.sendProgress(p)
	z.broadcastStatus(p)
	z.saveSoon(p)
	z.track(p, "spec", def.ID, 0, map[string]any{"from": old})
	z.log.Info("spec", "id", p.id, "name", p.name, "spec", def.ID, "from", old)
	free := freeSkillPoints(p)
	if old == "" {
		z.system(p, fmt.Sprintf("Você evoluiu para %s (%s). A árvore de skills (K) agora é a da evolução: %d pontos para distribuir.", def.Name, def.BonusText(), free))
	} else {
		z.system(p, fmt.Sprintf("Você agora é %s (%s). Os pontos da árvore voltaram: %d para distribuir.", def.Name, def.BonusText(), free))
	}
	z.questEvent(p, game.GoalTalk, game.TalkMaster) // escolheu ao lado dela: conta como conversa
	z.questEvent(p, game.GoalSpec, def.ID)
	z.sendSpecChoice(p, n)
}

// respecTree: a Ysolde redistribui a árvore da evolução (RespecCost): volta às iniciais e
// os pontos ficam livres.
func (z *Zone) respecTree(p *entity, n *entity) {
	if p.pl.spec == "" {
		z.system(p, n.name+": «Primeiro escolha a sua evolução.»")
		return
	}
	if p.pl.trade != nil {
		z.system(p, "Termine a troca antes de redistribuir a árvore.")
		return
	}
	cost := game.RespecCost(p.level)
	if p.pl.inv.gold < cost {
		z.system(p, fmt.Sprintf("Redistribuir a árvore custa %d moedas.", cost))
		return
	}
	p.pl.inv.gold -= cost
	z.trackGold(p, "respec", -cost)
	z.inventoryChanged(p, false)
	z.resetTree(p)
	z.cleanHotbar(p)
	z.refreshStats(p)
	z.sendProgress(p)
	z.saveSoon(p)
	z.track(p, "respec", p.pl.spec, 0, nil)
	z.system(p, fmt.Sprintf("Árvore redistribuída: %d pontos para distribuir.", freeSkillPoints(p)))
	z.sendSpecChoice(p, n)
}

// resetTree volta a árvore às iniciais da árvore em uso e zera as recargas das skills.
func (z *Zone) resetTree(p *entity) {
	p.pl.ranks = game.StarterRanks(p.pl.class, p.pl.spec)
	for id := range p.pl.cooldowns {
		if id < itemCooldownBase {
			delete(p.pl.cooldowns, id)
		}
	}
}

// cleanHotbar esvazia as teclas que não valem mais e manda a barra.
func (z *Zone) cleanHotbar(p *entity) {
	for i, e := range p.pl.hotbar {
		if !validHotbarEntry(p, e) {
			p.pl.hotbar[i] = ""
		}
	}
	z.sendHotbar(p)
}

// starterIDs: as 2 iniciais da árvore (da classe sem evolução, ou da evolução).
func starterIDs(class, spec string) []uint32 {
	if d := game.SpecFor(class, spec); d != nil {
		return d.StarterSkills[:]
	}
	var out []uint32
	for _, s := range game.ClassSkills(class) {
		out = append(out, s.ID)
	}
	return out
}

// pendingStarterItems: os itens iniciais da evolução que o personagem ainda não ganhou.
func (z *Zone) pendingStarterItems(p *entity, def *game.SpecDef) []*game.ItemDef {
	if p.pl.quests.done[game.StarterItemsFlag(def.ID)] {
		return nil
	}
	var out []*game.ItemDef
	for _, id := range def.StarterItems {
		if d := game.Items[id]; d != nil {
			out = append(out, d)
		}
	}
	return out
}

// giveStarterItems entrega os itens iniciais (uma vez por evolução).
func (z *Zone) giveStarterItems(p *entity, def *game.SpecDef, items []*game.ItemDef) {
	if len(def.StarterItems) == 0 || p.pl.quests.done[game.StarterItemsFlag(def.ID)] {
		return
	}
	for _, d := range items {
		p.pl.inv.add(d, 1)
	}
	p.pl.quests.done[game.StarterItemsFlag(def.ID)] = true
	if len(items) > 0 {
		z.inventoryChanged(p, true)
	}
}

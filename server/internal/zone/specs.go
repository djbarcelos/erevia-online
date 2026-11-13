package zone

import (
	"fmt"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Especialização (segunda classe). A Mestra dos Caminhos, na praça, conduz a missão "A
// Encruzilhada": falar com ela (nível 20+) abre a janela com os dois caminhos da classe, e
// escolher conclui a missão. A escolha fica salva no personagem e vai para todos no
// Entity.spec e no PlayerStatus (estado do jogador, como o título: muda só quando ele
// escolhe ou troca). Nada roda por tick.

// master: Ysolde, a Mestra dos Caminhos, do lado leste do poço (o Otto fica no oeste).
var master = npcDef{name: "Ysolde", title: "Mestra dos Caminhos", model: "mage", x: 8, z: 6,
	role: pb.NpcRole_NPC_ROLE_MASTER, look: &game.Appearance{Skin: 3, Hair: 5, Outfit: 6, Head: "mage"}}

func (z *Zone) spawnMaster(id uint32) {
	d := master
	pos := z.freeSpot(Vec2{X: d.x, Z: d.z})
	z.npcs[id] = &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_NPC,
		name: d.name, model: d.model, level: 1, scale: 1,
		pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
		npc: &npcState{title: d.title, role: d.role, look: d.look},
	}
}

// specCatalog: as 8 especializações (o cliente mostra o nome e a cor das dos outros).
func specCatalog() []*pb.SpecInfo {
	out := make([]*pb.SpecInfo, 0, len(game.SpecList))
	for _, s := range game.SpecList {
		out = append(out, &pb.SpecInfo{
			Id: s.ID, Class: s.Class, Name: s.Name, Description: s.Description, Color: s.Color,
			Bonus: attrsPB(s.Bonus), BonusText: s.BonusText(),
		})
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
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_SpecChoice{SpecChoice: &pb.SpecChoice{
		NpcId: n.id, Current: p.pl.spec, ChangeCost: uint64(game.SpecChangeCost),
	}}})
}

// chooseSpec: a primeira escolha precisa da missão; trocar depois custa SpecChangeCost e
// devolve os pontos das skills do caminho antigo.
func (z *Zone) chooseSpec(p *entity, n *entity, id string) {
	def := game.SpecFor(p.pl.class, id)
	switch {
	case def == nil:
		return
	case p.level < game.SpecLevel:
		z.system(p, fmt.Sprintf("A especialização chega no nível %d.", game.SpecLevel))
		return
	case def.ID == p.pl.spec:
		z.system(p, "Você já segue o caminho "+def.Name+".")
		return
	}
	old := p.pl.spec
	if old == "" {
		if !p.pl.quests.has(game.SpecQuest) && !p.pl.quests.done[game.SpecQuest] {
			z.system(p, "Fale com "+n.name+" para começar a missão A Encruzilhada.")
			return
		}
	} else {
		if p.pl.trade != nil {
			z.system(p, "Termine a troca antes de mudar de caminho.")
			return
		}
		if p.pl.inv.gold < game.SpecChangeCost {
			z.system(p, fmt.Sprintf("Trocar de caminho custa %d moedas.", game.SpecChangeCost))
			return
		}
		p.pl.inv.gold -= game.SpecChangeCost
		z.trackGold(p, "spec_change", -game.SpecChangeCost)
		z.inventoryChanged(p, false)
	}
	p.pl.spec = def.ID
	// As skills do caminho antigo somem (os pontos voltam) e saem da barra.
	if old != "" {
		for _, s := range game.SpecTree(old) {
			delete(p.pl.ranks, s.ID)
			delete(p.pl.cooldowns, s.ID)
		}
		cleaned := false
		for i, e := range p.pl.hotbar {
			if strings.HasPrefix(e, "skill:") && !validHotbarEntry(p, e) {
				p.pl.hotbar[i] = ""
				cleaned = true
			}
		}
		if cleaned {
			z.sendHotbar(p)
		}
	}
	z.refreshStats(p) // bônus de atributos (e passivas do caminho antigo)
	z.sendProgress(p)
	z.broadcastStatus(p)
	z.saveSoon(p)
	z.track(p, "spec", def.ID, 0, map[string]any{"from": old})
	z.log.Info("spec", "id", p.id, "name", p.name, "spec", def.ID, "from", old)
	if old == "" {
		z.system(p, fmt.Sprintf("Você escolheu o caminho %s (%s). Três skills novas na árvore (K), nos níveis %d, %d e %d.",
			def.Name, def.BonusText(), game.SpecTierLevels[0], game.SpecTierLevels[1], game.SpecTierLevels[2]))
	} else {
		z.system(p, fmt.Sprintf("Você trocou de caminho: agora é %s (%s). Os pontos das skills do caminho antigo voltaram.", def.Name, def.BonusText()))
	}
	z.questEvent(p, game.GoalTalk, game.TalkMaster) // escolheu ao lado dela: conta como conversa
	z.questEvent(p, game.GoalSpec, def.ID)
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_SpecChoice{SpecChoice: &pb.SpecChoice{
		NpcId: n.id, Current: p.pl.spec, ChangeCost: uint64(game.SpecChangeCost),
	}}})
}

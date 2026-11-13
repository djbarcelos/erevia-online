package zone

import (
	"math"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Lugar de cada objetivo de missão (para onde o minimapa, o mapa e a seta apontam). Vai
// no QuestStepInfo e é calculado só quando o QuestLog é enviado: ao mudar um passo e ao
// subir de nível (a área de caça depende do nível). Nada por tick.

const (
	townGateR  = 38.0 // portões no meio de cada lado da muralha (apótema do world.json)
	gatePlaceR = 3.0  // raio do marcador do portão
	npcPlaceR  = 2.5  // raio do marcador de um NPC ou pedra
)

// townGates: os 4 portões da cidade, nos eixos.
var townGates = []struct {
	name string
	x, z float64
}{
	{"Portão Leste", townGateR, 0},
	{"Portão Norte", 0, townGateR},
	{"Portão Oeste", -townGateR, 0},
	{"Portão Sul", 0, -townGateR},
}

// questPlace devolve o lugar do objetivo para este jogador agora (ok = false: sem lugar).
func (z *Zone) questPlace(p *entity, s game.QuestStep) (game.QuestPlace, bool) {
	if s.Place != nil {
		return *s.Place, true
	}
	switch s.Goal {
	case game.GoalKill:
		return game.HuntPlace(z.cfg.Spawns, s.Monsters(), p.level, p.pos.X, p.pos.Z)
	case game.GoalLevel:
		return game.HuntPlace(z.cfg.Spawns, nil, p.level, p.pos.X, p.pos.Z)
	case game.GoalLeaveTown:
		return nearestGate(p.pos), true
	case game.GoalTeleport:
		return z.townStonePlace()
	case game.GoalBuy:
		return z.shopPlace(p, s)
	case game.GoalTalk, game.GoalSpec:
		return z.talkPlace(p, s)
	}
	return game.QuestPlace{}, false // usar skill, tomar poção: em qualquer lugar
}

// placeByLevel diz se o lugar do objetivo muda com o nível (reenviar ao subir).
func placeByLevel(s game.QuestStep) bool {
	return s.Place == nil && (s.Goal == game.GoalKill || s.Goal == game.GoalLevel)
}

func nearestGate(pos Vec2) game.QuestPlace {
	best, bd := 0, math.Inf(1)
	for i, g := range townGates {
		if d := math.Hypot(pos.X-g.x, pos.Z-g.z); d < bd {
			best, bd = i, d
		}
	}
	g := townGates[best]
	return game.QuestPlace{Kind: game.PlaceGate, X: g.x, Z: g.z, Radius: gatePlaceR, Name: g.name}
}

// townStonePlace: a Pedra de Teleporte da praça (a primeira da lista).
func (z *Zone) townStonePlace() (game.QuestPlace, bool) {
	for _, n := range z.npcs {
		if n.npc.role == pb.NpcRole_NPC_ROLE_TELEPORT && n.npc.stone == 0 {
			return game.QuestPlace{Kind: game.PlaceStone, X: n.pos.X, Z: n.pos.Z, Radius: npcPlaceR, Name: "Pedra de Teleporte"}, true
		}
	}
	return game.QuestPlace{}, false
}

// shopPlace: a loja mais perto que vende o item pedido (sem alvo: qualquer loja).
func (z *Zone) shopPlace(p *entity, s game.QuestStep) (game.QuestPlace, bool) {
	n := z.nearestNPC(p, func(n *entity) bool {
		if len(n.npc.stock) == 0 {
			return false
		}
		for _, d := range n.npc.stock {
			if s.Matches(game.GoalBuy, d.ID) {
				return true
			}
		}
		return false
	})
	if n == nil {
		return game.QuestPlace{}, false
	}
	return game.QuestPlace{Kind: game.PlaceShop, X: n.pos.X, Z: n.pos.Z, Radius: npcPlaceR, Name: n.name}, true
}

// talkPlace: o NPC da conversa. TalkMaster (e escolher o caminho) é a Mestra dos Caminhos;
// outro alvo é o nome do NPC.
func (z *Zone) talkPlace(p *entity, s game.QuestStep) (game.QuestPlace, bool) {
	n := z.nearestNPC(p, func(n *entity) bool {
		if s.Goal == game.GoalSpec || s.Target == game.TalkMaster {
			return n.npc.role == pb.NpcRole_NPC_ROLE_MASTER
		}
		return strings.EqualFold(n.name, s.Target)
	})
	if n == nil {
		return game.QuestPlace{}, false
	}
	return game.QuestPlace{Kind: game.PlaceNPC, X: n.pos.X, Z: n.pos.Z, Radius: npcPlaceR, Name: n.name}, true
}

// nearestNPC: o NPC mais perto do jogador que passa no filtro (empate: o menor id).
func (z *Zone) nearestNPC(p *entity, ok func(n *entity) bool) *entity {
	var best *entity
	bd := math.Inf(1)
	for _, n := range z.npcs {
		if !ok(n) {
			continue
		}
		if d := dist(p.pos, n.pos); d < bd || d == bd && n.id < best.id {
			best, bd = n, d
		}
	}
	return best
}

// stepInfo monta o passo para o cliente, com o lugar quando ele ainda falta.
func (z *Zone) stepInfo(p *entity, s game.QuestStep, progress int) *pb.QuestStepInfo {
	info := &pb.QuestStepInfo{Text: s.Text, Count: uint32(progress), Need: uint32(s.Count)}
	if progress >= s.Count {
		return info
	}
	if pl, ok := z.questPlace(p, s); ok && pl.Kind != game.PlaceNone {
		info.Place = pb.QuestPlaceKind(pl.Kind)
		info.X, info.Z, info.Radius = float32(pl.X), float32(pl.Z), float32(pl.Radius)
		info.PlaceName = pl.Name
	}
	return info
}

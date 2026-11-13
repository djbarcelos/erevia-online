package zone

import (
	"fmt"
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Teleporte: Pedras de Teleporte na cidade e nos acampamentos das estradas levam de uma
// para outra (pagando o preço do destino), e o Pergaminho de Retorno leva à cidade. Os
// dois "canalizam" alguns segundos: levar dano, andar ou fazer outra coisa interrompe.
// Quem tem karma não usa nenhum dos dois.
const (
	teleportChannel = 2.0 // segundos na pedra antes de viajar
	stoneModel      = "teleport_stone"
	arriveScatter   = 2.5 // espalha quem chega para não ficar um em cima do outro
)

// stoneDef é uma Pedra de Teleporte (também é o destino de quem vem de outra).
type stoneDef struct {
	name  string
	x, z  float64
	price int64 // preço para chegar aqui
}

// teleportStones: praça da cidade, um acampamento em cada anel de caça e um no Vale das
// Pedras Vivas, à beira das estradas (as estradas são seguras de obstáculos).
var teleportStones = []stoneDef{
	{name: "Cidade", x: 7, z: 7, price: 20},
	{name: "Arredores", x: 60, z: 5, price: 40},
	{name: "Campos dos Ossos", x: 5, z: 94, price: 90},
	{name: "Ermos Partidos", x: -128, z: 5, price: 160},
	// No fim da estrada sul, na entrada do vale dos golens (sempre no fim da lista: o id
	// de cada destino é a posição + 1).
	{name: "Vale das Pedras Vivas", x: 6, z: -198, price: 220},
}

// channelState: o que o jogador está canalizando (pedra ou pergaminho).
type channelState struct {
	kind   pb.ChannelKind
	until  float64
	dest   Vec2
	place  string
	price  int64
	itemID string // pergaminho: consumido só no fim
}

func (z *Zone) spawnStones(firstID uint32) {
	for i, d := range teleportStones {
		pos := z.freeSpot(Vec2{X: d.x, Z: d.z})
		z.npcs[firstID+uint32(i)] = &entity{
			id: firstID + uint32(i), kind: pb.EntityKind_ENTITY_KIND_NPC,
			name: d.name, model: stoneModel, level: 1, scale: 1,
			pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
			npc: &npcState{title: "Pedra de Teleporte", role: pb.NpcRole_NPC_ROLE_TELEPORT, stone: i},
		}
	}
}

// stoneDests: para onde se pode ir a partir da pedra `from` (id = índice + 1).
func (z *Zone) stoneDests(from int) []*pb.TeleportDest {
	var out []*pb.TeleportDest
	for i, d := range teleportStones {
		if i == from {
			continue
		}
		out = append(out, &pb.TeleportDest{Id: uint32(i + 1), Name: d.name, X: float32(d.x), Z: float32(d.z), Price: uint32(d.price)})
	}
	return out
}

// startTeleport: o jogador escolheu um destino numa pedra.
func (z *Zone) startTeleport(p *entity, npcID, destID uint32) {
	n := z.shopNPC(p, npcID)
	if n == nil || n.npc.role != pb.NpcRole_NPC_ROLE_TELEPORT {
		return
	}
	i := int(destID) - 1
	if i < 0 || i >= len(teleportStones) || i == n.npc.stone {
		return
	}
	d := teleportStones[i]
	switch {
	case p.pl.karma > 0:
		z.system(p, "A pedra não responde a quem tem karma.")
	case p.pl.inv.gold < d.price:
		z.system(p, fmt.Sprintf("A viagem para %s custa %d moedas.", d.name, d.price))
	default:
		z.startChannel(p, &channelState{
			kind: pb.ChannelKind_CHANNEL_KIND_TELEPORT, until: z.now + teleportChannel,
			dest: Vec2{X: d.x, Z: d.z}, place: d.name, price: d.price,
		}, "Viajando para "+d.name)
	}
}

// startReturn: ler o Pergaminho de Retorno.
func (z *Zone) startReturn(p *entity, def *game.ItemDef) {
	key := uint32(itemCooldownBase + def.Cooldown)
	switch {
	case p.pl.karma > 0:
		z.system(p, "O pergaminho não funciona para quem tem karma.")
	case z.now < p.pl.cooldowns[key]:
		z.sendItemCooldown(p, def.Cooldown, p.pl.cooldowns[key]-z.now)
	case z.inSafe(p.pos):
		z.system(p, "Você já está na cidade.")
	default:
		z.startChannel(p, &channelState{
			kind: pb.ChannelKind_CHANNEL_KIND_RETURN, until: z.now + game.ReturnChannel,
			dest: Vec2{}, place: "Cidade", itemID: def.ID,
		}, "Lendo o Pergaminho de Retorno")
	}
}

func (z *Zone) startChannel(p *entity, ch *channelState, label string) {
	z.cancelQueuedSkill(p)
	p.clearAction()
	p.stopMoving()
	p.pl.channel = ch
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_ChannelBar{ChannelBar: &pb.ChannelBar{
		Kind: ch.kind, TotalMs: uint32(math.Round((ch.until - z.now) * 1000)), Label: label,
	}}})
}

// cancelChannel interrompe a canalização (msg vazia = sem aviso no chat).
func (z *Zone) cancelChannel(p *entity, msg string) {
	if p.pl == nil || p.pl.channel == nil {
		return
	}
	p.pl.channel = nil
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_ChannelBar{ChannelBar: &pb.ChannelBar{}}})
	if msg != "" {
		z.system(p, msg)
	}
}

// updateChannel termina a canalização quando o tempo acaba.
func (z *Zone) updateChannel(p *entity) {
	ch := p.pl.channel
	if ch == nil || z.now < ch.until {
		return
	}
	p.pl.channel = nil
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_ChannelBar{ChannelBar: &pb.ChannelBar{}}})
	inv := &p.pl.inv
	switch ch.kind {
	case pb.ChannelKind_CHANNEL_KIND_TELEPORT:
		if inv.gold < ch.price {
			z.system(p, "Moedas insuficientes para a viagem.")
			return
		}
		inv.gold -= ch.price
	case pb.ChannelKind_CHANNEL_KIND_RETURN:
		slot := inv.slotOf(ch.itemID)
		if slot < 0 {
			z.system(p, "Você não tem mais o pergaminho.")
			return
		}
		inv.take(slot, 1)
		def := game.Items[ch.itemID]
		p.pl.cooldowns[uint32(itemCooldownBase+def.Cooldown)] = z.now + game.CooldownSeconds(def.Cooldown)
		z.sendItemCooldown(p, def.Cooldown, game.CooldownSeconds(def.Cooldown))
	}
	z.arrive(p, ch.dest)
	z.system(p, fmt.Sprintf("Você chegou em %s.", ch.place))
	z.inventoryChanged(p, false)
	if ch.kind == pb.ChannelKind_CHANNEL_KIND_TELEPORT {
		z.questEvent(p, game.GoalTeleport, "")
	}
}

// arrive põe o jogador no destino: os monstros esquecem dele e ninguém mais o mira.
func (z *Zone) arrive(p *entity, dest Vec2) {
	a := z.rng.Float64() * 2 * math.Pi
	r := 1 + z.rng.Float64()*arriveScatter
	spot := z.freeSpot(clampToZone(Vec2{X: dest.X + r*math.Cos(a), Z: dest.Z + r*math.Sin(a)}, z.bounds()))
	z.forgetTarget(p.id)
	p.teleportTo(spot)
	p.lastCombat = -math.MaxFloat64 / 2
}

package zone

import (
	"context"
	"fmt"
	"math"
	"slices"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Canais (tunnel#86): cópias do mapa principal no mesmo processo, para crescer além de um
// mapa cheio e limitar quantos jogadores aparecem na tela. Cada canal é uma Zone comum (a
// sua goroutine, os seus monstros), com o mesmo id de mapa ("starter"): o banco grava o
// mapa, não o canal, e o próximo login escolhe o canal de novo.
//
//   - Entrar: o canal pedido (Join.channel); cheio, o menos cheio com aviso; todos cheios,
//     a entrada é recusada. Pelo portão ou pela pedra de outro mapa, o menos cheio.
//   - Trocar: na cidade, fora de combate, sem troca nem loja aberta (ChangeChannel). É uma
//     travessia de zona como a do portão (crossZone): sai de um, entra no outro no mesmo
//     lugar, com o estado passado de mão em mão.
//   - O que é do mundo fica no Canal 1, como já acontece nos outros mapas (MapConfig,
//     noTown): o raid boss e os chefes de campo (uma linha por chefe no banco), os
//     monstros raros, a invasão da cidade, as portas das masmorras, os clãs (a zona da
//     cidade é a dona deles) e o armazém da conta (só uma zona pode mudá-lo: dois canais
//     com dois personagens da mesma conta duplicariam itens). Do Canal 2 em diante o
//     Guarda do Armazém e o clã mandam ir ao Canal 1. Grupo, chat geral, sussurro, amigos,
//     correio e casa de leilões já são de todas as zonas (bus.go e banco).

// ChannelConfig: a configuração do canal n (≥ 2) a partir da do mapa principal (MapConfig).
func ChannelConfig(home Config, n int) Config {
	cfg := home
	cfg.Channel = n
	cfg.MapTitle = fmt.Sprintf("%s · Canal %d", mapDefs[world.StarterMap].title, n)
	cfg.NoRares = true
	cfg.InvasionChance = 0
	cfg.Clans, cfg.Bosses = nil, nil // os eventos de GM (XP em dobro) valem em todos os canais
	cfg.Dungeons = nil
	cfg.Gates = slices.DeleteFunc(slices.Clone(home.Gates), func(g GateDef) bool { return g.Dungeon != "" })
	cfg.Spawns = slices.DeleteFunc(slices.Clone(home.Spawns), func(s game.Spawn) bool {
		d := game.Monsters[s.Monster]
		return d != nil && (d.Boss || d.Raid || d.FieldBoss != nil)
	})
	return cfg
}

// mainChannel: esta zona é a dona do que é do mundo (o Canal 1, ou um processo sem canais).
func (z *Zone) mainChannel() bool { return z.cfg.Channel <= 1 }

// channelName: "Canal 2" ("" = o processo não tem canais).
func (z *Zone) channelName() string {
	if z.cfg.Channel == 0 {
		return ""
	}
	return fmt.Sprintf("Canal %d", z.cfg.Channel)
}

// AddChannel põe um canal do mapa principal no gerente (antes do Run). O primeiro canal é o
// mapa principal que já foi adicionado com Add (cfg.Channel = 1).
func (m *Manager) AddChannel(z *Zone) {
	z.mgr = m
	if len(m.channels) == 0 {
		home := m.Zone(world.StarterMap)
		home.cfg.Channel = 1
		m.channels = append(m.channels, home)
	}
	z.cfg.Map = world.StarterMap
	m.zones = append(m.zones, z)
	m.channels = append(m.channels, z)
}

// Channels: os canais do mapa principal (vazio = o processo não tem canais).
func (m *Manager) Channels() []*Zone { return m.channels }

// channelLoad: quantos jogadores o canal tem (online nele ou chegando).
func (m *Manager) channelLoad(z *Zone) int { return m.bus.countZone(z) }

func (m *Manager) channelFull(z *Zone) bool {
	return z.cfg.Capacity > 0 && m.channelLoad(z) >= z.cfg.Capacity
}

// pickChannel: o canal want (1, 2…) se ele existe e cabe; senão o menos cheio que cabe (o de
// número menor no empate). nil = todos cheios. moved = não deu o pedido.
func (m *Manager) pickChannel(want int) (z *Zone, moved bool) {
	if want >= 1 && want <= len(m.channels) && !m.channelFull(m.channels[want-1]) {
		return m.channels[want-1], false
	}
	best, bestLoad := (*Zone)(nil), math.MaxInt
	for _, c := range m.channels {
		if l := m.channelLoad(c); !m.channelFull(c) && l < bestLoad {
			best, bestLoad = c, l
		}
	}
	return best, want != 0 && best != nil
}

// Arrival: a zona onde chega quem vem de outro lugar para o mapa id (portão, pedra, saída de
// masmorra). No mapa principal com canais, o menos cheio (nil = todos cheios).
func (m *Manager) Arrival(id string) *Zone {
	if (id == "" || id == world.StarterMap) && len(m.channels) > 1 {
		z, _ := m.pickChannel(0)
		return z
	}
	return m.Zone(id)
}

// ChannelsPB: a lista para o cliente (current = o canal de z; 0 = z não é um canal).
func (m *Manager) ChannelsPB(z *Zone) *pb.ChannelList {
	out := &pb.ChannelList{}
	if len(m.channels) < 2 {
		return out
	}
	for i, c := range m.channels {
		if c == z {
			out.Current = uint32(i + 1)
		}
		out.Channels = append(out.Channels, &pb.ChannelInfo{Number: uint32(i + 1), Players: uint32(m.channelLoad(c)),
			Capacity: uint32(max(0, c.cfg.Capacity)), Full: m.channelFull(c)})
	}
	return out
}

// sendChannels manda a lista de canais ao jogador (nada se o processo não tem canais).
func (z *Zone) sendChannels(p *entity) {
	if z.mgr == nil || len(z.mgr.channels) < 2 {
		return
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Channels{Channels: z.mgr.ChannelsPB(z)}})
}

// ChangeChannel repassa o pedido de trocar de canal.
func (z *Zone) ChangeChannel(ctx context.Context, id uint32, channel int) {
	z.post(ctx, socialDone{fn: func() { z.changeChannel(id, channel) }})
}

// changeChannel: trocar para o canal n (0 = só manda a lista).
func (z *Zone) changeChannel(id uint32, n int) {
	p := z.players[id]
	if p == nil || !z.spendAction(p) {
		return
	}
	if n == 0 || z.mgr == nil || len(z.mgr.channels) < 2 {
		z.sendChannels(p)
		return
	}
	if n < 1 || n > len(z.mgr.channels) {
		z.system(p, "Esse canal não existe.")
		return
	}
	dest := z.mgr.channels[n-1]
	switch {
	case dest == z:
		z.system(p, fmt.Sprintf("Você já está no Canal %d.", n))
	case !z.isChannel():
		z.system(p, "Só dá para trocar de canal no mapa principal.")
	case p.dead:
		z.system(p, "Você está morto.")
	case math.Hypot(p.pos.X, p.pos.Z) > z.cfg.SafeRadius:
		z.system(p, "Só dá para trocar de canal dentro da cidade.")
	case p.pl.trade != nil:
		z.system(p, "Feche a troca antes de trocar de canal.")
	case z.now-p.lastCombat < gateCombatWait:
		left := int(math.Ceil(gateCombatWait - (z.now - p.lastCombat)))
		z.system(p, fmt.Sprintf("Você está em combate: espere %d s para trocar de canal.", left))
	case z.mgr.channelFull(dest):
		z.system(p, fmt.Sprintf("O Canal %d está cheio. Tente outro.", n))
		z.sendChannels(p)
	default:
		z.crossZone(p, dest, p.pos, "channel")
	}
}

// isChannel: esta zona é um dos canais do mapa principal.
func (z *Zone) isChannel() bool {
	return z.mgr != nil && slices.Contains(z.mgr.channels, z)
}

// ChannelStatus é um canal para quem está fora do jogo (a tela de personagens).
type ChannelStatus struct {
	Number, Players, Capacity int
	Full                      bool
}

// ChannelStatuses: os canais do mapa principal (vazio = o processo não tem canais). Lê só o
// barramento (pode ser chamado de qualquer goroutine).
func (m *Manager) ChannelStatuses() []ChannelStatus {
	if len(m.channels) < 2 {
		return nil
	}
	out := make([]ChannelStatus, len(m.channels))
	for i, c := range m.channels {
		out[i] = ChannelStatus{Number: i + 1, Players: m.channelLoad(c), Capacity: max(0, c.cfg.Capacity), Full: m.channelFull(c)}
	}
	return out
}

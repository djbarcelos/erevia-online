package zone

import (
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Portão: NPC que leva o personagem para outra zona (outro mapa), na mesma conexão.
//
// Dona única: a zona de origem tira o jogador (removePlayer), grava o estado (com a zona
// e o ponto de chegada do destino) e só então o entrega à zona de destino, com o estado
// passado de mão em mão (JoinInfo). Entre as duas, o personagem está "em trânsito" no
// barramento: nunca nas duas zonas, nunca em nenhuma. Se o processo parar no meio, vale
// o que foi gravado (o próximo login entra no destino, no ponto de chegada).
//
// Regras do portão (perto dele, gateRange):
//   - morto, em troca aberta ou em combate (bateu ou apanhou há menos de gateCombatWait):
//     não atravessa (aviso no chat);
//   - canalização (pedra ou pergaminho): o portão cancela e atravessa;
//   - convites pendentes (grupo, clã ou troca, enviados ou recebidos): cancelados;
//   - o grupo continua: os membros em outro mapa aparecem como "em outro mapa" e a XP só
//     divide entre quem está na mesma zona;
//   - a sessão continua (telemetria, recargas das skills, silêncio de GM).
const (
	gateRange      = 6.0  // distância para usar o portão (a mesma das lojas)
	gateCombatWait = 10.0 // segundos sem lutar para atravessar
	gateIDBase     = npcIDBase + 1<<16
	gateModel      = stoneModel // por enquanto, a pedra rúnica (o cliente já desenha)
)

// GateDef é um portão da zona.
type GateDef struct {
	Name     string  // nome sobre o portão ("Portão da Clareira")
	X, Z     float64 // onde fica
	To       string  // id do mapa de destino
	ArriveX  float64 // ponto de chegada no destino (o jogador é espalhado em volta)
	ArriveZ  float64
	ToTitle  string // nome do mapa de destino (preenchido pelo gerente; "" = o id)
	disabled bool
}

// hopState: o lado "entre zonas" de um jogador.
type hopState struct {
	out     *hopOut      // saindo pelo portão: removePlayer entrega em vez de esquecer
	arrived bool         // chegou pelo portão (não é sessão nova)
	storage *StorageData // mapa sem armazém: o armazém como veio (vai de volta para a cidade)
}

type hopOut struct {
	dest *Zone
	at   Vec2
}

// hopIn: o que só a entrega do portão traz no JoinInfo.
type hopIn struct {
	from      string
	seq       int64
	joinedAt  time.Time
	cooldowns map[uint32]time.Time
	// auction: operações da casa de leilões sem confirmação do banco. Também foram na
	// gravação da origem; o banco aplica cada uma uma vez só (repetida volta Fresh=false).
	auction []AuctionOp
}

// UseGate repassa o pedido de atravessar o portão.
func (z *Zone) UseGate(ctx context.Context, id, npcID uint32) {
	z.post(ctx, socialDone{fn: func() { z.useGate(id, npcID) }})
}

// spawnGates cria os portões cujo destino existe neste processo.
func (z *Zone) spawnGates() {
	for i, g := range z.cfg.Gates {
		if g.disabled {
			continue
		}
		pos := z.freeSpot(Vec2{X: g.X, Z: g.Z})
		id := gateIDBase + uint32(i)
		z.npcs[id] = &entity{
			id: id, kind: pb.EntityKind_ENTITY_KIND_NPC,
			name: g.Name, model: gateModel, level: 1, scale: 1,
			pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
			npc: &npcState{title: "Portão para " + g.title(), role: pb.NpcRole_NPC_ROLE_GATE, stone: i},
		}
	}
}

func (g GateDef) title() string {
	if g.ToTitle != "" {
		return g.ToTitle
	}
	return g.To
}

// gateOf: o portão do NPC (nil = não é portão).
func (z *Zone) gateOf(n *entity) *GateDef {
	if n == nil || n.npc == nil || n.npc.role != pb.NpcRole_NPC_ROLE_GATE {
		return nil
	}
	return &z.cfg.Gates[n.npc.stone]
}

// useGate: o jogador pediu para atravessar o portão npcID.
func (z *Zone) useGate(id, npcID uint32) {
	p := z.players[id]
	if p == nil || !z.spendAction(p) {
		return
	}
	n := z.npcs[npcID]
	g := z.gateOf(n)
	if g == nil {
		return
	}
	if dist(p.pos, n.pos) > gateRange {
		z.system(p, fmt.Sprintf("Chegue mais perto de %s.", n.name))
		return
	}
	var dest *Zone
	if z.mgr != nil {
		dest = z.mgr.Zone(g.To)
	}
	switch {
	case dest == nil || dest == z:
		z.system(p, "O portão está fechado.")
		return
	case p.dead:
		z.system(p, "Você está morto.")
		return
	case p.pl.trade != nil:
		z.system(p, "Feche a troca antes de atravessar o portão.")
		return
	case z.now-p.lastCombat < gateCombatWait:
		left := int(math.Ceil(gateCombatWait - (z.now - p.lastCombat)))
		z.system(p, fmt.Sprintf("Você está em combate: espere %d s para atravessar o portão.", left))
		return
	}
	z.cancelChannel(p, "")
	z.dropInvites(p.id)
	z.endEmote(p)
	p.clearAction()
	p.stopMoving()
	at := Vec2{X: g.ArriveX, Z: g.ArriveZ}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_ZoneChange{ZoneChange: &pb.ZoneChange{
		Map: dest.mapID(), Title: dest.mapTitle(), X: float32(at.X), Z: float32(at.Z),
	}}})
	z.track(p, "gate", dest.mapID(), int64(p.level), map[string]any{"from": z.mapID()})
	p.pl.hop.out = &hopOut{dest: dest, at: at}
	z.removePlayer(p.id) // grava e entrega (handOff)
}

// hopping: o jogador está saindo pelo portão (e não caiu por panic).
func (z *Zone) hopping(p *entity) bool {
	return p.pl.hop.out != nil && !z.errs.broken[p.id]
}

// handOff entrega à zona de destino quem está saindo pelo portão. st é o estado que vai
// ser gravado: ganha a zona e o ponto de chegada do destino. false = não é portão.
func (z *Zone) handOff(p *entity, st *SavedState) bool {
	o := p.pl.hop.out
	if o == nil || z.mgr == nil {
		return false
	}
	st.X, st.Z, st.Zone = o.at.X, o.at.Z, o.dest.mapID()
	pl := p.pl
	// Cópias novas (nada é dividido com o estado que vai para a persistência).
	info := JoinInfo{
		ID: p.id, Name: p.name, Class: pl.class, X: st.X, Z: st.Z, Zone: st.Zone,
		XP: pl.xp, HP: st.HP, MP: st.MP, Gold: pl.inv.gold, Items: pl.inv.records(),
		Attrs: pl.attrs, Ranks: maps.Clone(pl.ranks), Hotbar: slices.Clone(pl.hotbar),
		Emblem: pl.emblem, Look: pl.look,
		AccountID: pl.accountID, Karma: pl.karma, PKs: pl.pks, PvPKills: pl.pvpKills,
		ClanLeftAt: pl.clanLeftAt,
		GM:         pl.gm, MutedUntil: pl.mutedUntil, UserAgent: pl.userAgent,
		Quests: pl.quests.state(), Achievements: pl.ach.state(), Title: pl.ach.title,
		Spec: pl.spec, Mastery: pl.mastery.save(), Peer: pl.peer,
		Bestiary: pl.bst.state(),
		hop: &hopIn{from: z.mapID(), seq: st.Seq, joinedAt: pl.joinedAt, cooldowns: z.savedCooldowns[p.id],
			auction: slices.Clone(st.Auction)},
		// O estado é o da memória agora (já com o que o portal mudou enquanto ele estava
		// online): a recusa de Join "carregado antes de uma mudança" não vale para ele.
		LoadedAt: time.Now(),
	}
	info.Contracts, info.Reputation = pl.contracts.save() // contratos e reputação são do personagem
	if s := z.storageOf(p); s != nil {
		info.Storage = &StorageData{Gold: s.gold, Items: s.records()}
	} else {
		info.Storage = pl.hop.storage
	}
	delete(z.savedCooldowns, p.id)
	delete(z.recent, p.id)
	z.mgr.transfer(info, o.dest)
	return true
}

// onArrive: a entrega do portão chegou (goroutine da zona de destino).
func (z *Zone) onArrive(info JoinInfo) {
	z.lastSeq = max(z.lastSeq, info.hop.seq) // o próximo salvamento é mais novo que o da origem
	if info.hop.cooldowns != nil {
		z.savedCooldowns[info.ID] = info.hop.cooldowns
	}
	z.onJoin(info)
	p := z.players[info.ID]
	if p == nil || p.pl.peer != info.Peer { // recusado (conta apagada no caminho)
		z.mgr.bus.unregister(info.ID, z, info.Peer, z.cfg.Clock())
		return
	}
	// A conexão caiu no caminho: o Leave dela pode ter ido para a outra zona. Sai agora
	// (grava e fica no recent, como uma saída normal).
	if rp, ok := info.Peer.(routedPeer); ok && rp.Closed() {
		z.removePlayer(info.ID)
	}
}

// zonesOnJoin: entrou nesta zona (login ou portão). Registra no barramento e liga ao
// grupo, se ele tem um em outra zona.
func (z *Zone) zonesOnJoin(p *entity) {
	if z.mgr == nil {
		return
	}
	bus := z.mgr.bus
	if bad := bus.register(p, z); bad != "" {
		z.invariantBroken(p, "zone_owner", bad, nil)
	}
	if id := bus.partyOf(p.id); id != 0 {
		if v, ok := bus.party(id); ok {
			pt := z.applyParty(v)
			z.sendParty(pt)
		}
	}
}

// zonesOnLeave: saiu desta zona. Pelo portão, continua no barramento (já com a zona de
// destino); senão, sai dele. A cópia do grupo some se ninguém daqui está nele.
func (z *Zone) zonesOnLeave(p *entity, hop bool) {
	if z.mgr == nil {
		return
	}
	if pt := p.pl.party; pt != nil {
		p.pl.party = nil
		z.dropPartyCopy(pt)
	}
	z.mgr.bus.left(p.id, z)
	if !hop {
		z.mgr.bus.unregister(p.id, z, p.pl.peer, z.cfg.Clock())
	}
}

// gateList: os portões para o Welcome (cliente: tooltip e clique).
func (z *Zone) gateTo(n *entity) string {
	if g := z.gateOf(n); g != nil {
		return g.title()
	}
	return ""
}

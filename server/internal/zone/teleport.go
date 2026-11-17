package zone

import (
	"fmt"
	"math"
	"slices"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Teleporte: Pedras de Teleporte na cidade e nos acampamentos das estradas levam de uma
// para outra (pagando o preço do destino), e o Pergaminho de Retorno leva à última cidade
// visitada. Os dois "canalizam" alguns segundos: levar dano, andar ou fazer outra coisa
// interrompe. Quem tem karma não usa nenhum dos dois.
//
// Descoberta (tunnel#141): um destino só vale depois que o personagem tocou a pedra a pé,
// ou seja, chegou a discoverRange dela andando (teleporte, login e qualquer salto não
// contam: ver discovery). As pedras são por mapa (stoneDef.mapID); viajar para uma de outro
// mapa atravessa a zona como o portão (crossZone). A balsa do Brejo não entra na descoberta.
const (
	teleportChannel = 2.0 // segundos na pedra antes de viajar
	discoverRange   = 5.0 // a que distância de uma pedra, a pé, ela é descoberta
	stoneModel      = "teleport_stone"
	arriveScatter   = 2.5 // espalha quem chega para não ficar um em cima do outro
)

// stoneDef é uma Pedra de Teleporte (também é o destino de quem vem de outra).
type stoneDef struct {
	key    string // id estável, salvo em characters.discovered
	name   string
	region string // nome da região (o que a janela mostra de uma pedra ainda não descoberta)
	mapID  string // mapa onde a pedra fica ("" = o principal)
	x, z   float64
	price  int64 // preço para chegar aqui
	// town: cidade, destino do Pergaminho de Retorno (returnPoint: onde ele deixa o jogador).
	town bool
	// ferry: o barco do Brejo (Cais do Barqueiro e Ilha dos Vaga-lumes). Só liga os dois, e as
	// pedras comuns não o listam.
	ferry bool
}

// teleportStones: praça da cidade, um acampamento em cada anel de caça e um no Vale das
// Pedras Vivas, à beira das estradas (as estradas são seguras de obstáculos).
var teleportStones = []stoneDef{
	{key: "cidade", name: "Cidade", region: regionVale, x: 7, z: 7, price: 20, town: true},
	{key: "arredores", name: "Arredores", region: regionVale, x: 60, z: 5, price: 40},
	{key: "ossos", name: "Campos dos Ossos", region: regionVale, x: 5, z: 94, price: 90},
	{key: "ermos", name: "Ermos Partidos", region: regionVale, x: -128, z: 5, price: 160},
	// No fim da estrada sul, na entrada do vale dos golens (sempre no fim da lista: o id
	// de cada destino é a posição + 1).
	{key: "vale", name: "Vale das Pedras Vivas", region: regionVale, x: 6, z: -198, price: 220},
	// Brejo de Lamagris (tunnel#139): a pedra do Posto do Barqueiro, na entrada do pântano.
	{key: "posto", name: "Posto do Barqueiro", region: regionBrejo, x: 162, z: 5, price: 180},
	// O barco de Tomé: do cais até a Ilha dos Vaga-lumes (e de volta) por 10 moedas.
	{key: "cais", name: "Cais do Barqueiro", region: regionBrejo, x: game.DockX, z: game.DockZ, price: 10, ferry: true},
	{key: "ilha", name: "Ilha dos Vaga-lumes", region: regionBrejo, x: game.IslandX, z: game.IslandZ, price: 10, ferry: true},
	// Bastião Cinzento (tunnel#137): o posto avançado das Terras Mortas, no mapa principal. Descobre-se a pé, como as outras.
	{key: "bastiao", name: "Bastião Cinzento", region: game.TerrasMortasRegion, x: game.BastionX - 3, z: game.BastionZ - 3, price: 260},
}

const (
	regionVale  = "Vale de Vel'Aran"
	regionBrejo = "Brejo de Lamagris"
)

// futureStones: as pedras dos mapas que vêm (Minas, Ruínas e Costa), com o preço já definido.
// Ficam depois das de teleportStones (o id do destino é a posição em allStones + 1) e só
// aparecem para o jogador quando o mapa delas existe neste processo (maps.go). As posições
// valem até o mapa entrar. Maresia e Valdurn são cidades: o Pergaminho de Retorno leva a elas.
var futureStones = []stoneDef{
	{key: "lanterna", name: "Lanterna", region: "Ruínas", mapID: "ruinas", x: 0, z: 5, price: 240},
	{key: "maresia", name: "Maresia", region: "Costa", mapID: "costa", x: 7, z: 7, price: 300, town: true},
	{key: "vigilia", name: "Vigília", region: "Ruínas", mapID: "ruinas", x: 40, z: 5, price: 320},
	{key: "valdurn", name: "Valdurn", region: "Minas", mapID: "minas", x: 7, z: 7, price: 400, town: true},
}

// allStones: todas as pedras, na ordem dos ids de destino (posição + 1).
var allStones = append(slices.Clone(teleportStones), futureStones...)

// stoneIndex: o índice da pedra de chave key em allStones (-1 = não existe).
func stoneIndex(key string) int {
	return slices.IndexFunc(allStones, func(d stoneDef) bool { return d.key == key })
}

// stoneMap: o mapa da pedra (o principal quando vazio).
func (d stoneDef) stoneMap() string {
	if d.mapID == "" {
		return world.StarterMap
	}
	return d.mapID
}

// returnPoint: onde o Pergaminho de Retorno deixa o jogador: a praça de Vel'Aran (a origem) ou,
// nas outras cidades, a pedra delas.
func (d stoneDef) returnPoint() Vec2 {
	if d.key == teleportStones[0].key {
		return Vec2{}
	}
	return Vec2{X: d.x, Z: d.z}
}

// stoneIDBase: o id do NPC das pedras dos outros mapas (cada mapa tem a sua zona).
const stoneIDBase = npcIDBase + 2<<16

// ferryModel: o barco de Tomé (o cliente desenha um bote, sem arquivo 3D).
const ferryModel = "ferry_boat"

// channelState: o que o jogador está canalizando (pedra ou pergaminho).
type channelState struct {
	mapID  string // mapa do destino ("" = este)
	kind   pb.ChannelKind
	until  float64
	dest   Vec2
	place  string
	price  int64
	itemID string // pergaminho: consumido só no fim
	stone  int    // teleporte: o índice da pedra de destino em allStones (+1; 0 = não é pedra)
}

// legacyStones: as pedras que já existiam antes do Brejo. Os ids delas (e de tudo que vem depois na praça) não
// mudam: as pedras e os barcos do Brejo ficam depois dos contadores de história (extraStoneID).
const legacyStones = 5

// extraStoneID: o id do NPC da pedra i (as do Brejo vêm depois do último contador de história).
func extraStoneID(i int) uint32 {
	return talkerFirstID + uint32(len(game.SpawnedTalkers())) + uint32(i-legacyStones)
}

func (z *Zone) spawnStones(firstID uint32) {
	z.stoneIdx = nil
	for i, d := range allStones {
		if d.stoneMap() != z.mapID() {
			continue
		}
		id := stoneIDBase + uint32(i)
		switch {
		case i < legacyStones:
			id = firstID + uint32(i)
		case i < len(teleportStones):
			id = extraStoneID(i)
		}
		pos := z.freeSpot(Vec2{X: d.x, Z: d.z})
		model, title := stoneModel, "Pedra de Teleporte"
		if d.ferry {
			model, title = ferryModel, "Barco do Tomé"
		}
		z.addNPC(&entity{
			id: id, kind: pb.EntityKind_ENTITY_KIND_NPC,
			name: d.name, model: model, level: 1, scale: 1,
			pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
			npc: &npcState{title: title, role: pb.NpcRole_NPC_ROLE_TELEPORT, stone: i, post: postKeyOf(d.key)},
		})
		z.stoneIdx = append(z.stoneIdx, i)
	}
}

// spawnMapStones: as pedras de um mapa que não é o principal (o principal sai em spawnNPCs,
// com a praça; os outros não têm praça).
func (z *Zone) spawnMapStones() {
	if z.mapID() != world.StarterMap {
		z.spawnStones(0)
	}
}

// stoneReachable: a pedra i é um destino que existe neste processo (a zona do mapa dela
// está ligada). Sem gerente (testes de uma zona só), só as pedras do mapa desta zona.
func (z *Zone) stoneReachable(i int) bool {
	m := allStones[i].stoneMap()
	return m == z.mapID() || (z.mgr != nil && z.mgr.Zone(m) != nil)
}

// stoneDests: para onde se pode ir a partir da pedra `from` (id = índice + 1). Vai todo
// mundo (com o nome); quem ainda não descobriu uma pedra só vê a região (StonesKnown diz
// quais o jogador já conhece).
func (z *Zone) stoneDests(from int) []*pb.TeleportDest {
	var out []*pb.TeleportDest
	for i, d := range allStones {
		if i == from || d.ferry != allStones[from].ferry || !z.stoneReachable(i) {
			continue
		}
		out = append(out, &pb.TeleportDest{Id: uint32(i + 1), Name: d.name, X: float32(d.x), Z: float32(d.z), Price: uint32(d.price), Region: d.region})
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
	if i < 0 || i >= len(allStones) || i == n.npc.stone || allStones[i].ferry != allStones[n.npc.stone].ferry || !z.stoneReachable(i) {
		return
	}
	d := allStones[i]
	switch {
	case p.pl.karma > 0:
		z.system(p, "A pedra não responde a quem tem karma.")
	case z.stoneClosed(i):
		z.system(p, z.stoneRefusal(i))
	case !d.ferry && !p.pl.disc.known[d.key]:
		z.system(p, "Você ainda não conhece essa pedra: chegue até ela a pé.")
	case p.pl.inv.gold < z.teleportPrice(p, d.price):
		z.system(p, fmt.Sprintf("A viagem para %s custa %d moedas.", d.name, z.teleportPrice(p, d.price)))
	case p.pl.trade != nil && d.stoneMap() != z.mapID():
		z.system(p, "Feche a troca antes de viajar para outro mapa.")
	default:
		z.startChannel(p, &channelState{
			kind: pb.ChannelKind_CHANNEL_KIND_TELEPORT, until: z.now + z.channelTime(p, teleportChannel),
			dest: Vec2{X: d.x, Z: d.z}, mapID: d.stoneMap(), place: d.name, price: z.teleportPrice(p, d.price), stone: i + 1,
		}, "Viajando para "+d.name)
	}
}

// returnTown: a última cidade visitada (a pedra); Vel'Aran se não houver ou se o mapa dela
// não estiver ligado.
func (z *Zone) returnTown(p *entity) stoneDef {
	if i := stoneIndex(p.pl.disc.town); i >= 0 && allStones[i].town && z.stoneReachable(i) {
		return allStones[i]
	}
	return allStones[0]
}

// startReturn: ler o Pergaminho de Retorno.
func (z *Zone) startReturn(p *entity, def *game.ItemDef) {
	key := uint32(itemCooldownBase + def.Cooldown)
	town := z.returnTown(p)
	here := town.stoneMap() == z.mapID()
	switch {
	case p.pl.karma > 0:
		z.system(p, "O pergaminho não funciona para quem tem karma.")
	case z.now < p.pl.cooldowns[key]:
		z.sendItemCooldown(p, def.Cooldown, p.pl.cooldowns[key]-z.now)
	case here && z.inSafe(p.pos):
		z.system(p, "Você já está na cidade.")
	case p.pl.trade != nil && !here:
		z.system(p, "Feche a troca antes de viajar para outro mapa.")
	default:
		z.startChannel(p, &channelState{
			kind: pb.ChannelKind_CHANNEL_KIND_RETURN, until: z.now + z.channelTime(p, game.ReturnChannel),
			dest: town.returnPoint(), mapID: town.stoneMap(), place: town.name, itemID: def.ID,
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
	other := ch.mapID != "" && ch.mapID != z.mapID()
	var destZone *Zone
	if other {
		if z.mgr != nil {
			destZone = z.mgr.Zone(ch.mapID)
		}
		if destZone == nil || destZone == z {
			z.system(p, "O caminho está fechado.")
			return
		}
		if p.pl.trade != nil {
			z.system(p, "Feche a troca antes de viajar para outro mapa.")
			return
		}
	}
	switch ch.kind {
	case pb.ChannelKind_CHANNEL_KIND_TELEPORT:
		if ch.stone > 0 && z.stoneClosed(ch.stone-1) {
			z.system(p, z.stoneRefusal(ch.stone-1))
			return
		}
		if inv.gold < ch.price {
			z.system(p, "Moedas insuficientes para a viagem.")
			return
		}
		inv.gold -= ch.price
		z.trackGold(p, "teleport", -ch.price)
	case pb.ChannelKind_CHANNEL_KIND_RETURN:
		slot := inv.slotOf(ch.itemID)
		if slot < 0 {
			z.system(p, "Você não tem mais o pergaminho.")
			return
		}
		inv.take(slot, 1)
		z.trackItem(p, false, ch.itemID, 1, "return")
		def := game.Items[ch.itemID]
		cd := z.itemCooldown(p, def.Cooldown) // Caminho de Casa (Maestria)
		p.pl.cooldowns[uint32(itemCooldownBase+def.Cooldown)] = z.now + cd
		z.sendItemCooldown(p, def.Cooldown, cd)
	}
	if !other {
		z.arrive(p, ch.dest)
	}
	z.system(p, fmt.Sprintf("Você chegou em %s.", ch.place))
	z.inventoryChanged(p, false)
	if ch.kind == pb.ChannelKind_CHANNEL_KIND_TELEPORT {
		z.questEvent(p, game.GoalTeleport, "")
	}
	if other {
		// Atravessa a zona depois do snapshot (como o portão; o laço dos jogadores não pode
		// perder quem está nele no meio do tick).
		a := z.rng.Float64() * 2 * math.Pi
		r := 1 + z.rng.Float64()*arriveScatter
		at := Vec2{X: ch.dest.X + r*math.Cos(a), Z: ch.dest.Z + r*math.Sin(a)}
		z.afterFlush(func() {
			if z.players[p.id] == p && !p.dead && p.pl.trade == nil {
				z.crossZone(p, destZone, at, "teleport_map")
			}
		})
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

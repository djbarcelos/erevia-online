package zone

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Invasão da cidade: em algumas noites (Config.InvasionChance) ou por GM (/evento
// invasao), os esqueletos atacam Vel'Aran. O aviso sai 2 min antes, com o mesmo Announce
// do raid boss; depois as ondas nascem do lado de fora dos 4 portões e marcham para a
// praça, brigando com quem encontram no caminho. Enquanto dura, as estradas dos portões e
// a praça deixam de ser zona segura para lutar com monstros (pveSafe); o PvP continua
// proibido na cidade inteira, e karma e flag não mudam. Morrer para um invasor não custa
// XP. Acaba quando as ondas acabam (vitória) ou no tempo limite; a recompensa (XP, moedas
// e, na vitória, pergaminhos para os 3 melhores) é proporcional aos pontos de cada um:
// o dano causado nos invasores mais um bônus por abate.
//
// Desempenho (a cidade é a região mais pesada do cliente): no máximo invMaxAlive
// esqueletos vivos de cada vez, nascendo aos poucos (um por portão a cada invSpawnEvery).
// Eles não deixam saque no chão, procuram alvo a cada invAggroEvery ticks e chegam aos
// clientes pela AOI, como qualquer monstro. No cliente não há efeito novo por quadro.

const (
	defaultInvasionChance = 0.15
	invWarnBefore         = 120.0 // segundos entre o aviso e a primeira onda
	invTimeLimit          = 8 * 60.0
	invMaxAlive           = 20
	invSpawnEvery         = 2.0  // segundos entre levas (um esqueleto por portão)
	invWavePause          = 10.0 // segundos entre o fim de uma onda e a próxima
	invWaveStall          = 60.0 // a próxima onda vem mesmo com sobreviventes da anterior
	invAggroEvery         = 4    // ticks entre buscas de alvo de cada invasor
	invLeash              = 60.0 // coleira contada da praça
	invGateR              = 44.0 // nascem do lado de fora da muralha (a 38 m do centro)
	invPlazaR             = 8.0  // e marcham até aqui, na praça
	invOpenPlaza          = 13.0 // raio da praça aberta (as casas do anel interno ficam a 17,5 m)
	invOpenRoad           = 6.0  // meia largura das estradas abertas, do portão à praça
	invShelterR           = 15.0 // quem morre volta aqui, entre as estradas (parte segura)
	invNear               = 80.0 // quem está a esta distância do centro define o nível
	invKillPoints         = 0.25 // o abate vale 25% da vida do invasor em pontos
	invMinShare           = 0.05 // mínimo, em fração dos pontos do melhor, para ser recompensado
	invRewardXP           = 0.12 // fração da XP do nível para quem mais contribuiu
	invGoldBase           = 50.0
	invGoldLevel          = 50.0 // moedas por nível para quem mais contribuiu
	invLoseMult           = 0.5  // tempo esgotado: metade da recompensa, sem itens
	invStatusEvery        = 1.0  // segundos entre atualizações do HUD (vivos)
)

type invWave struct {
	count   int
	mix     []string // monstros sorteados
	captain string   // nasce por último, 3 níveis acima ("" = nenhum)
}

var invasionWaves = []invWave{
	{count: 12, mix: []string{"skeleton_minion", "skeleton_minion", "skeleton_warrior"}},
	{count: 16, mix: []string{"skeleton_minion", "skeleton_warrior", "skeleton_rogue"}},
	{count: 20, mix: []string{"skeleton_warrior", "skeleton_rogue", "skeleton_mage"}},
	{count: 22, mix: []string{"skeleton_warrior", "skeleton_rogue", "skeleton_mage"}, captain: "bone_lord"},
}

// invGates: as direções dos 4 portões (nos eixos).
var invGates = [4]Vec2{{X: 1}, {Z: 1}, {X: -1}, {Z: -1}}

// invTopItems: o que os 3 que mais contribuíram ganham numa vitória.
var invTopItems = []string{"scroll_ench_weapon", "scroll_ench_armor", "scroll_ench_armor"}

type invasion struct {
	phase      pb.InvasionPhase
	at         float64 // aviso: quando as ondas começam; ativa: o tempo limite (z.now)
	reason     string  // "noite" ou "gm"
	level      int
	wave       int  // índice da onda atual
	left       int  // ainda por nascer na onda
	captain    bool // o capitão da onda ainda não nasceu
	nextSpawn  float64
	waveDone   float64 // quando a onda terminou de nascer
	gate       int
	alive      []uint32
	points     map[uint32]float64 // dano + abates de cada jogador
	kills      map[uint32]int
	killed     int
	sentAlive  int
	nextStatus float64
}

// nightInvasion: a noite caiu; com a chance configurada, os esqueletos vêm.
func (z *Zone) nightInvasion() {
	if z.cfg.InvasionChance <= 0 || z.inv != nil || len(z.players) == 0 {
		return
	}
	if z.rng.Float64() >= z.cfg.InvasionChance {
		return
	}
	z.startInvasion(invWarnBefore, "noite")
}

// startInvasion anuncia a invasão (warn segundos antes das ondas; 0 = já começa).
func (z *Zone) startInvasion(warn float64, reason string) bool {
	if z.inv != nil {
		return false
	}
	z.inv = &invasion{phase: pb.InvasionPhase_INVASION_PHASE_WARNING, at: z.now + warn, reason: reason}
	z.log.Info("invasion announced", "reason", reason, "warn_s", warn)
	if warn <= 0 {
		z.beginInvasion()
		return true
	}
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, "Invasão!",
		fmt.Sprintf("Esqueletos marcham para Vel'Aran! Em %s eles chegam aos 4 portões. Defendam a praça!", durationText(warn)))
	z.broadcastEvents()
	return true
}

func durationText(secs float64) string {
	if m := int(math.Round(secs / 60)); m >= 1 {
		if m == 1 {
			return "1 minuto"
		}
		return fmt.Sprintf("%d minutos", m)
	}
	return fmt.Sprintf("%d segundos", int(math.Ceil(secs)))
}

// beginInvasion: a primeira onda sai dos portões e a cidade se abre para a luta.
func (z *Zone) beginInvasion() {
	v := z.inv
	v.phase = pb.InvasionPhase_INVASION_PHASE_ACTIVE
	v.at = z.now + invTimeLimit
	v.level = z.invasionLevel()
	v.wave = 0
	v.left, v.captain = invasionWaves[0].count, invasionWaves[0].captain != ""
	v.nextSpawn = z.now
	v.points, v.kills = map[uint32]float64{}, map[uint32]int{}
	v.nextStatus = z.now + invStatusEvery
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SPAWN, "A cidade está sob ataque!",
		"Os esqueletos entram pelos portões. Até o fim da invasão, as estradas e a praça deixam de ser zona segura (o PvP continua proibido).")
	z.broadcastEvents()
	z.log.Info("invasion started", "reason", v.reason, "level", v.level)
}

// invasionLevel: a média de quem está perto da cidade (ou de todos), um nível abaixo.
func (z *Zone) invasionLevel() int {
	sum, n := 0, 0
	for _, p := range z.players {
		if math.Hypot(p.pos.X, p.pos.Z) <= invNear {
			sum, n = sum+p.level, n+1
		}
	}
	if n == 0 {
		for _, p := range z.players {
			sum, n = sum+p.level, n+1
		}
	}
	if n == 0 {
		return 3
	}
	return min(max(2, int(math.Round(float64(sum)/float64(n)))-1), game.MaxLevel-6)
}

// invasionOpen: durante a invasão, as estradas dos portões e a praça.
func (z *Zone) invasionOpen(p Vec2) bool {
	if z.inv == nil || z.inv.phase != pb.InvasionPhase_INVASION_PHASE_ACTIVE {
		return false
	}
	return math.Abs(p.X) < invOpenRoad || math.Abs(p.Z) < invOpenRoad || p.X*p.X+p.Z*p.Z < invOpenPlaza*invOpenPlaza
}

// invShelters: os 4 cantos seguros da cidade durante a invasão, nas diagonais (entre as
// estradas e fora da praça).
func invShelters() [4]Vec2 {
	var out [4]Vec2
	for i := range out {
		a := (float64(i) + 0.5) * math.Pi / 2
		out[i] = Vec2{X: invShelterR * math.Cos(a), Z: invShelterR * math.Sin(a)}
	}
	return out
}

// updateInvasion roda a cada tick (sem invasão, só confere o ponteiro).
func (z *Zone) updateInvasion() {
	v := z.inv
	if v == nil {
		return
	}
	if v.phase == pb.InvasionPhase_INVASION_PHASE_WARNING {
		if z.now >= v.at {
			z.beginInvasion()
		}
		return
	}
	if z.now >= v.at {
		z.endInvasion(false)
		return
	}
	z.invSpawn()
	if z.inv == v && z.now >= v.nextStatus {
		v.nextStatus = z.now + invStatusEvery
		if len(v.alive) != v.sentAlive {
			z.broadcastEvents()
		}
	}
}

// invSpawn: uma leva por vez (um por portão), sem passar de invMaxAlive vivos; a onda
// seguinte vem quando esta cai (ou depois de invWaveStall); sem mais ondas, vitória.
func (z *Zone) invSpawn() {
	v := z.inv
	if z.now < v.nextSpawn {
		return
	}
	if v.left == 0 && !v.captain {
		last := v.wave+1 >= len(invasionWaves)
		switch {
		case last && len(v.alive) == 0:
			z.endInvasion(true)
		case last, len(v.alive) > 0 && z.now-v.waveDone < invWaveStall:
		default:
			v.wave++
			w := invasionWaves[v.wave]
			v.left, v.captain = w.count, w.captain != ""
			v.nextSpawn = z.now + invWavePause
			z.broadcastSystem(fmt.Sprintf("Onda %d de %d: mais esqueletos se aproximam dos portões!", v.wave+1, len(invasionWaves)))
			z.broadcastEvents()
		}
		return
	}
	v.nextSpawn = z.now + invSpawnEvery
	w := invasionWaves[v.wave]
	for range invGates {
		if len(v.alive) >= invMaxAlive || (v.left == 0 && !v.captain) {
			break
		}
		id, level := "", v.level+v.wave
		if v.left > 0 {
			id = w.mix[z.rng.IntN(len(w.mix))]
			v.left--
		} else {
			id, level = w.captain, level+3
			v.captain = false
		}
		if def := game.Monsters[id]; def != nil {
			z.spawnInvader(def, min(level, game.MaxLevel), invGates[v.gate])
		}
		v.gate = (v.gate + 1) % len(invGates)
		if v.left == 0 && !v.captain {
			v.waveDone = z.now
		}
	}
}

// spawnInvader põe um esqueleto do lado de fora do portão dir, já marchando para a praça.
func (z *Zone) spawnInvader(def *game.MonsterDef, level int, dir Vec2) {
	side := Vec2{X: -dir.Z, Z: dir.X}
	j, k := (z.rng.Float64()*2-1)*2.5, (z.rng.Float64()*2-1)*3
	spot := z.freeSpot(Vec2{X: dir.X*invGateR + side.X*j, Z: dir.Z*invGateR + side.Z*j})
	home := z.freeSpot(Vec2{X: dir.X*invPlazaR + side.X*k, Z: dir.Z*invPlazaR + side.Z*k})
	z.summonSeq++
	id := summonIDBase + z.summonSeq
	m := &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_MONSTER,
		name: def.Name, model: def.Model, level: level, scale: def.Scale,
		pos: spot, dest: spot,
		stats: game.MonsterStats(def, level),
		mon: &monsterState{
			def: def, spawn: game.SpawnPoint{Monster: def, Level: level, X: spot.X, Z: spot.Z},
			home: home, hate: map[uint32]float64{}, summon: true, invader: true, leash: invLeash,
		},
	}
	m.hp = m.stats.MaxHP
	m.sentHP, m.sentMaxHP = m.hpInt(), m.maxHPInt()
	z.monsters[id] = m // Spawn pela AOI, só para quem está perto
	z.inv.alive = append(z.inv.alive, id)
	z.aoiDirty = true
	z.goTo(m, home)
}

// marchInvader: sem alvo, o invasor segue para o seu ponto na praça.
func (z *Zone) marchInvader(m *entity) {
	home := m.mon.home
	if dist(m.pos, home) < 1 {
		return // chegou
	}
	final := m.dest
	if len(m.path) > 0 {
		final = m.path[len(m.path)-1]
	}
	if m.dest != m.pos && dist(final, home) < 1.5 {
		return // já está a caminho (o caminho de uma perseguição que acabou não conta)
	}
	z.moveToward(m, home)
}

// invasionCredit soma o dano de um jogador num invasor (antes da vida cair).
func (z *Zone) invasionCredit(src *entity, amount float64) {
	if z.inv == nil || z.inv.points == nil || src == nil || src.pl == nil || amount <= 0 {
		return
	}
	z.inv.points[src.id] += amount
}

// invasionKilled: um invasor morreu. O abate vai para quem deu o golpe (ou, se foi um
// veneno de quem saiu, para quem mais causou dano).
func (z *Zone) invasionKilled(m, killer *entity) {
	v := z.inv
	if v == nil {
		return
	}
	if i := indexOf(v.alive, m.id); i >= 0 {
		v.alive = append(v.alive[:i], v.alive[i+1:]...)
	}
	v.killed++
	credit := killer
	if credit == nil || credit.pl == nil {
		credit = z.players[topHate(m.mon.hate)]
	}
	if credit != nil && credit.pl != nil && v.points != nil {
		v.points[credit.id] += m.stats.MaxHP * invKillPoints
		v.kills[credit.id]++
	}
}

func indexOf(ids []uint32, id uint32) int {
	for i, x := range ids {
		if x == id {
			return i
		}
	}
	return -1
}

// stopInvasion: o GM interrompeu (sem recompensa).
func (z *Zone) stopInvasion() {
	v := z.inv
	z.clearInvasion()
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, "Invasão interrompida",
		"Os esqueletos voltam para a escuridão. A cidade é zona segura de novo.")
	z.broadcastEvents()
	z.log.Info("invasion stopped", "reason", v.reason)
}

// clearInvasion tira os invasores vivos do mundo e fecha a cidade de novo.
func (z *Zone) clearInvasion() {
	v := z.inv
	for _, id := range v.alive {
		if m := z.monsters[id]; m != nil {
			z.removeMonster(m) // Despawn pela AOI
		}
	}
	v.alive = nil
	z.inv = nil
}

// endInvasion: vitória (as ondas acabaram) ou tempo esgotado. Recompensa e anúncio.
func (z *Zone) endInvasion(won bool) {
	v := z.inv
	z.clearInvasion()
	top := z.invRewards(v, won)
	if won {
		text := fmt.Sprintf("Vel'Aran derrotou %d esqueletos.", v.killed)
		if top != "" {
			text += " Maior defensor: " + top + "."
		}
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT, "A cidade resistiu!", text)
	} else {
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, "Os esqueletos recuam",
			"O tempo acabou e as hordas voltam para a escuridão. Os defensores recebem metade da recompensa.")
	}
	z.broadcastEvents()
	z.log.Info("invasion ended", "won", won, "killed", v.killed, "wave", v.wave+1, "top", top)
}

type invShare struct {
	p      *entity
	points float64
}

// invRewards paga quem está online e defendeu: XP e moedas proporcionais aos pontos
// (em relação a quem mais fez) e, na vitória, um pergaminho para os 3 melhores. Devolve
// o nome de quem mais contribuiu.
func (z *Zone) invRewards(v *invasion, won bool) string {
	var list []invShare
	for id, pts := range v.points {
		if p := z.players[id]; p != nil && pts > 0 {
			list = append(list, invShare{p, pts})
		}
	}
	if len(list) == 0 {
		return ""
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].points != list[j].points {
			return list[i].points > list[j].points
		}
		return list[i].p.id < list[j].p.id
	})
	best := list[0].points
	mult := 1.0
	if !won {
		mult = invLoseMult
	}
	for rank, s := range list {
		p := s.p
		w := s.points / best
		if w < invMinShare {
			z.system(p, "Invasão: a sua ajuda foi pequena demais para receber recompensa.")
			continue
		}
		var parts []string
		gold := int64(math.Round((invGoldBase + invGoldLevel*float64(p.level)) * w * mult))
		if gold > 0 {
			p.pl.inv.gold += gold
			z.trackGold(p, "invasion", gold)
			parts = append(parts, fmt.Sprintf("%d moedas", gold))
		}
		if def := game.Items[invTopItemFor(rank, won)]; def != nil {
			parts = append(parts, itemLabel(def, 1))
			if p.pl.inv.add(def, 1) == 0 {
				z.trackItem(p, true, def.ID, 1, "invasion") // o que ficar no chão entra como loot ao ser pego
			} else {
				if g := z.dropItem(p.pos, def, 1, 0, p.id, questRewardReserve); g != nil {
					z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_GroundAdded{GroundAdded: &pb.GroundItemsAdded{Items: []*pb.GroundItem{z.groundPB(g)}}}}, 0)
					z.system(p, "Sua bolsa está cheia: o pergaminho ficou no chão, reservado para você.")
				}
			}
		}
		z.inventoryChanged(p, false)
		bar := game.XPToNext(p.level)
		if p.level >= game.MasteryLevel {
			bar = game.MasteryPointXP // no máximo, a recompensa enche a barra de Maestria
		}
		xp := int64(math.Round(float64(bar) * invRewardXP * w * mult))
		if xp > 0 {
			parts = append(parts, fmt.Sprintf("%d de XP", xp))
		}
		z.system(p, fmt.Sprintf("Invasão: %d abates, %.0f%% da contribuição do melhor defensor. Recompensa: %s.",
			v.kills[p.id], w*100, strings.Join(parts, ", ")))
		if xp > 0 {
			z.grantXPFrom(p, xpInvasion, xp)
		}
		z.track(p, "invasion", v.reason, int64(math.Round(s.points)), map[string]any{"won": won, "rank": rank + 1, "kills": v.kills[p.id]})
		if won {
			z.achEvent(p, game.AchDefense, "")
		}
	}
	return list[0].p.name
}

func invTopItemFor(rank int, won bool) string {
	if !won || rank >= len(invTopItems) {
		return ""
	}
	return invTopItems[rank]
}

// broadcastSystem: linha do sistema para todos.
func (z *Zone) broadcastSystem(text string) {
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_SYSTEM, Text: text,
	}}}, 0)
}

// invasionText: o estado da invasão para o /evento.
func (z *Zone) invasionText() string {
	v := z.inv
	switch {
	case v == nil:
		return "nenhuma"
	case v.phase == pb.InvasionPhase_INVASION_PHASE_WARNING:
		return "começa em " + durationText(v.at-z.now)
	}
	return fmt.Sprintf("onda %d de %d, %d esqueletos vivos, %d derrotados, acaba em %s",
		v.wave+1, len(invasionWaves), len(v.alive), v.killed, durationText(v.at-z.now))
}

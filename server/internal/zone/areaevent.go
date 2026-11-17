package zone

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Eventos de área com etapas (tunnel#142, roadmap T17). O motor genérico de game.EventDef:
// o gatilho (noite, janela ou horário fixo, com recarga) anuncia o evento e ele passa pelas
// etapas em ordem (matar ondas, defender o estandarte, chefe), cada uma com limite de tempo.
// No fim, quem contribuiu leva a medalha (100%, 85% ou 75% da recompensa, pela faixa), e, se o
// evento falhou, o posto cai: a pedra de teleporte, o mercador e o NPC dele recusam por
// EventDef.CloseSecs, até reabrirem sozinhos.
//
// Tudo é da goroutine da zona, no relógio z.now e no sorteio z.rng. A invasão da cidade
// (invasion.go) e a XP em dobro (events.go) seguem como estavam: são sistemas à parte. Os
// monstros do evento são invocações marcadas (monsterState.evMob): sempre agressivos,
// marcham para o ponto do evento, não deixam saque (a recompensa sai no fim) e somam o dano
// de cada jogador. Os anúncios e o painel vão só para esta zona, ou seja, para o mapa do evento.
//
// O estado é de memória: um reinício da zona apaga o evento e reabre os postos.

const (
	aeSpotJitter  = 3.5 // um monstro "perto" do estandarte fere ele a esta distância
	aeStatusEvery = 1.0
	aeNearMap     = 2 // quantos nascem por leva
)

// postIDBase: os NPCs dos postos (o mercador do Bastião), numa faixa própria.
const postIDBase = npcIDBase + 3<<16

// postBoard: os postos fechados, visíveis por todas as zonas (a pedra de um posto é destino de
// pedras de outros mapas). Os instantes são de relógio de parede.
type postBoard struct {
	mu     sync.Mutex
	closed map[string]time.Time
}

func (b *postBoard) set(key string, until time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed == nil {
		b.closed = map[string]time.Time{}
	}
	if until.IsZero() {
		delete(b.closed, key)
		return
	}
	b.closed[key] = until
}

func (b *postBoard) isClosed(key string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return now.Before(b.closed[key])
}

type areaEvent struct {
	def       *game.EventDef
	reason    string
	warning   bool
	startAt   float64 // aviso: quando a primeira etapa começa
	stage     int     // índice da etapa atual
	stageAt   float64 // quando a etapa começou
	stageEnd  float64 // limite da etapa (z.now)
	spawned   int
	killed    int
	alive     []uint32
	nextSpawn float64
	banner    uint32
	bannerHP  float64
	boss      uint32
	pool      float64 // a vida total dos monstros que nasceram
	dmg       map[uint32]float64
	heal      map[uint32]float64
	present   map[uint32]float64 // segundos no círculo
	nextSec   float64
}

// areaCenter: o centro do círculo do evento.
func (e *areaEvent) center() Vec2 { return Vec2{X: e.def.X, Z: e.def.Z} }

func eventMap(d *game.EventDef) string {
	if d.Map == "" {
		return world.StarterMap
	}
	return d.Map
}

// ---------- Gatilhos ----------

// areaEventNight: a noite caiu (updateDaytime): os eventos de gatilho "noite" desta zona sorteiam.
func (z *Zone) areaEventNight() {
	for _, d := range game.AreaEvents {
		if !d.Enabled || d.Trigger.Kind != game.TriggerNight || eventMap(d) != z.mapID() || !z.aeCanStart(d) {
			continue
		}
		if z.rng.Float64() < d.Trigger.Chance {
			z.startAreaEvent(d, "noite")
			return
		}
	}
}

// aeCanStart: há jogadores, nenhum evento em andamento e a recarga já passou.
func (z *Zone) aeCanStart(d *game.EventDef) bool {
	return z.ae == nil && len(z.players) > 0 && z.now >= z.aeCool[d.ID]
}

// aeTriggers: os gatilhos de janela e de horário fixo (1×/s).
func (z *Zone) aeTriggers() {
	for _, d := range game.AreaEvents {
		if !d.Enabled || eventMap(d) != z.mapID() || !z.aeCanStart(d) {
			continue
		}
		switch d.Trigger.Kind {
		case game.TriggerWindow:
			next, ok := z.aeWindow[d.ID]
			if !ok {
				z.aeWindow[d.ID] = z.now + d.Trigger.Window
				continue
			}
			if z.now < next {
				continue
			}
			z.aeWindow[d.ID] = z.now + d.Trigger.Window
			if z.rng.Float64() < d.Trigger.Chance {
				z.startAreaEvent(d, "janela")
			}
		case game.TriggerFixed:
			t := z.cfg.Clock().In(z.eventLoc())
			day := t.Year()*1000 + t.YearDay()
			if t.Hour() == d.Trigger.Hour && t.Minute() == d.Trigger.Minute && z.aeDay[d.ID] != day {
				z.aeDay[d.ID] = day
				z.startAreaEvent(d, "horário")
			}
		}
	}
}

// ---------- Ciclo ----------

// startAreaEvent anuncia o evento (e começa a primeira etapa depois do aviso do gatilho).
func (z *Zone) startAreaEvent(d *game.EventDef, reason string) bool {
	if z.ae != nil {
		return false
	}
	z.ae = &areaEvent{def: d, reason: reason, dmg: map[uint32]float64{}, heal: map[uint32]float64{}, present: map[uint32]float64{}}
	z.log.Info("area event announced", "event", d.ID, "reason", reason)
	if w := d.Trigger.Warn; w > 0 {
		z.ae.warning, z.ae.startAt = true, z.now+w
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, d.Name,
			fmt.Sprintf("Perigo %s! Em %s os Devotos atacam. Quem defender o posto leva a medalha.", d.Place, durationText(w)))
		z.flushAreaStatus()
		return true
	}
	z.beginAreaEvent()
	return true
}

func (z *Zone) beginAreaEvent() {
	e := z.ae
	e.warning = false
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SPAWN, e.def.Name, fmt.Sprintf("O ataque começou %s!", e.def.Place))
	z.beginStage(0)
}

func (z *Zone) beginStage(i int) {
	e := z.ae
	s := &e.def.Stages[i]
	e.stage, e.stageAt, e.stageEnd = i, z.now, z.now+s.Limit
	e.spawned, e.killed, e.nextSpawn = 0, 0, z.now
	e.boss = 0
	switch s.Kind {
	case game.StageDefend:
		e.bannerHP = s.TargetHP
		z.aeSpawnBanner()
	case game.StageBoss:
		z.aeSpawnBoss(s)
	}
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, fmt.Sprintf("%s: etapa %d de %d", e.def.Name, i+1, len(e.def.Stages)), s.Say)
	z.flushAreaStatus()
}

// updateAreaEvents roda a cada tick: reabre postos, dispara gatilhos e leva o evento adiante.
func (z *Zone) updateAreaEvents() {
	for key, until := range z.postUntil {
		if z.now >= until {
			z.reopenPost(key)
		}
	}
	if z.tick%uint32(z.cfg.TickRate) == 0 && z.ae == nil {
		z.aeTriggers()
	}
	if e := z.ae; e != nil {
		z.aeUpdate(e)
	}
	if z.aeDirty && z.now >= z.aeNextSend {
		z.flushAreaStatus()
	}
}

// flushAreaStatus manda o painel para a zona agora (e segura o próximo por 1 s).
func (z *Zone) flushAreaStatus() {
	z.aeNextSend = z.now + aeStatusEvery
	z.aeDirty = false
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_AreaEvent{AreaEvent: z.areaStatusPB()}}, 0)
}

func (z *Zone) aeUpdate(e *areaEvent) {
	if e.warning {
		if z.now >= e.startAt {
			z.beginAreaEvent()
		}
		return
	}
	s := &e.def.Stages[e.stage]
	if z.now >= e.nextSec { // 1×/s: presença e dano ao estandarte
		e.nextSec = z.now + 1
		z.aePresence(e)
		if s.Kind == game.StageDefend {
			z.aeDrain(e, s)
			if e.bannerHP <= 0 {
				z.endAreaEvent(false, "O Estandarte caiu.")
				return
			}
		}
	}
	switch s.Kind {
	case game.StageKill:
		z.aeSpawnWave(e, s, e.spawned < s.Count)
		if e.killed >= s.Count {
			z.aeNextStage()
			return
		}
	case game.StageDefend:
		z.aeSpawnWave(e, s, true)
		if z.now >= e.stageAt+s.Hold {
			z.aeNextStage()
			return
		}
	case game.StageBoss:
		if e.boss != 0 && z.monsters[e.boss] == nil {
			z.aeNextStage()
			return
		}
	}
	if z.now >= e.stageEnd {
		z.endAreaEvent(false, "O tempo da etapa acabou.")
	}
}

// aeNextStage: a etapa foi cumprida; sobra o que ficou vivo, e a próxima começa ou o evento vence.
func (z *Zone) aeNextStage() {
	e := z.ae
	z.aeClearMobs(e)
	z.aeRemoveBanner(e)
	if e.stage+1 >= len(e.def.Stages) {
		z.endAreaEvent(true, "")
		return
	}
	z.beginStage(e.stage + 1)
}

// aePresence soma 1 s a quem está vivo no círculo.
func (z *Zone) aePresence(e *areaEvent) {
	c := e.center()
	for id, p := range z.players {
		if !p.dead && dist(p.pos, c) <= e.def.Radius {
			e.present[id]++
		}
	}
	z.aeDirty = true // o tempo e o progresso andam
}

// aeDrain: cada monstro do evento perto do estandarte tira dele DrainRate por segundo.
func (z *Zone) aeDrain(e *areaEvent, s *game.StageDef) {
	b := z.objects[e.banner]
	if b == nil {
		return
	}
	near := 0
	for _, id := range e.alive {
		if m := z.monsters[id]; m != nil && !m.dead && dist(m.pos, b.pos) <= aeSpotJitter {
			near++
		}
	}
	e.bannerHP = math.Max(0, e.bannerHP-float64(near)*s.DrainRate)
}

// ---------- Monstros do evento ----------

// aeSpawnWave: uma leva (até aeNearMap monstros) a cada Every segundos, sem passar de MaxAlive vivos.
func (z *Zone) aeSpawnWave(e *areaEvent, s *game.StageDef, more bool) {
	if !more || z.now < e.nextSpawn {
		return
	}
	every := s.Every
	if every <= 0 {
		every = 2
	}
	e.nextSpawn = z.now + every
	for range aeNearMap {
		if len(e.alive) >= s.MaxAlive || (s.Kind == game.StageKill && e.spawned >= s.Count) {
			return
		}
		def := game.Monsters[s.Mobs[z.rng.IntN(len(s.Mobs))]]
		e.spawned++
		z.aeSpawnMob(e, def, e.def.Level, 1, false)
	}
}

// aeSpawnMob põe um monstro na borda do círculo, já marchando para o centro (ou o estandarte).
func (z *Zone) aeSpawnMob(e *areaEvent, def *game.MonsterDef, level int, hpMult float64, boss bool) *entity {
	c := e.center()
	a := z.rng.Float64() * 2 * math.Pi
	r := e.def.Radius * (0.8 + 0.15*z.rng.Float64())
	spot := z.freeSpot(Vec2{X: c.X + r*math.Cos(a), Z: c.Z + r*math.Sin(a)})
	if boss {
		spot = z.freeSpot(Vec2{X: c.X + 0.5*e.def.Radius*math.Cos(a), Z: c.Z + 0.5*e.def.Radius*math.Sin(a)})
	}
	focus := c
	if b := z.objects[e.banner]; b != nil {
		focus = b.pos
	}
	j := z.rng.Float64() * 2 * math.Pi
	home := z.freeSpot(Vec2{X: focus.X + 2.5*math.Cos(j), Z: focus.Z + 2.5*math.Sin(j)})
	z.summonSeq++
	id := summonIDBase + z.summonSeq
	scale := def.Scale
	if boss {
		scale *= 1.25
	}
	m := &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_MONSTER,
		name: def.Name, model: def.Model, level: level, scale: scale,
		pos: spot, dest: spot,
		stats: game.MonsterStats(def, level),
		mon: &monsterState{
			def: def, spawn: game.SpawnPoint{Monster: def, Level: level, X: spot.X, Z: spot.Z},
			home: home, hate: map[uint32]float64{}, summon: true, evMob: true, leash: e.def.Radius*3 + 30,
		},
	}
	m.stats.MaxHP *= hpMult
	m.hp = m.stats.MaxHP
	m.sentHP, m.sentMaxHP = m.hpInt(), m.maxHPInt()
	z.monsters[id] = m // Spawn pela AOI
	e.alive = append(e.alive, id)
	e.pool += m.stats.MaxHP
	z.aoiDirty = true
	z.goTo(m, home)
	return m
}

func (z *Zone) aeSpawnBoss(s *game.StageDef) {
	e := z.ae
	def := game.Monsters[s.Boss]
	m := z.aeSpawnMob(e, def, e.def.Level+2, s.BossHPMult, true)
	e.boss = m.id
	m.name = def.Name + " (chefe)"
}

func (z *Zone) aeSpawnBanner() {
	e := z.ae
	z.objectSeq++
	id := objectIDBase + z.objectSeq
	pos := z.freeSpot(e.center())
	b := &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_OBJECT,
		name: "Estandarte do Bastião", model: "banner", level: e.def.Level, scale: 1.6,
		pos: pos, dest: pos,
		stats: game.Stats{MaxHP: 1},
		obj:   &objectState{until: z.now + 24*3600, title: "Defenda o estandarte"},
	}
	b.hp = 1
	b.sentHP, b.sentMaxHP = 1, 1
	z.objects[id] = b
	e.banner = id
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Spawn{Spawn: &pb.Spawn{Entity: z.entityPB(b)}}}, 0)
}

func (z *Zone) aeRemoveBanner(e *areaEvent) {
	if e.banner == 0 {
		return
	}
	if z.objects[e.banner] != nil {
		delete(z.objects, e.banner)
		z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Despawn{Despawn: &pb.Despawn{Id: e.banner}}}, 0)
	}
	e.banner = 0
}

func (z *Zone) aeClearMobs(e *areaEvent) {
	for _, id := range e.alive {
		if m := z.monsters[id]; m != nil {
			z.removeMonster(m) // Despawn pela AOI
		}
	}
	e.alive = nil
}

// areaEventCredit: o dano de um jogador num monstro do evento.
func (z *Zone) areaEventCredit(src *entity, amount float64) {
	if z.ae == nil || src == nil || src.pl == nil || amount <= 0 {
		return
	}
	z.ae.dmg[src.id] += amount
}

// areaEventHeal: a cura de um jogador (vida roubada) enquanto o evento corre, dentro do círculo.
func (z *Zone) areaEventHeal(p *entity, amount float64) {
	e := z.ae
	if e == nil || e.warning || p == nil || p.pl == nil || amount <= 0 || dist(p.pos, e.center()) > e.def.Radius {
		return
	}
	e.heal[p.id] += amount
}

// areaEventKilled: um monstro do evento morreu.
func (z *Zone) areaEventKilled(m *entity) {
	e := z.ae
	if e == nil {
		return
	}
	if i := indexOf(e.alive, m.id); i >= 0 {
		e.alive = append(e.alive[:i], e.alive[i+1:]...)
		if s := &e.def.Stages[e.stage]; s.Kind == game.StageKill {
			e.killed++
		}
		z.aeDirty = true
	}
}

// ---------- Fim e recompensa ----------

// endAreaEvent fecha o evento: sucesso ou falha (o posto cai), com o pagamento por faixa.
func (z *Zone) endAreaEvent(won bool, why string) {
	e := z.ae
	d := e.def
	z.aeClearMobs(e)
	z.aeRemoveBanner(e)
	top := z.aeRewards(e, won)
	z.ae = nil
	z.aeCool[d.ID] = z.now + d.Trigger.Cooldown
	if won {
		text := d.SuccessMsg
		if top != "" {
			text += " Maior defensor: " + top + "."
		}
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT, d.Name+": vitória!", text)
	} else {
		text := d.FailMsg
		if text == "" {
			text = "O evento falhou."
		}
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, d.Name+": o posto caiu", strings.TrimSpace(why+" "+text))
		if d.Post != "" {
			z.closePost(d.Post, d.CloseSecs)
		}
	}
	z.flushAreaStatus()
	z.log.Info("area event ended", "event", d.ID, "won", won, "top", top)
}

// stopAreaEvent: o GM interrompeu (sem recompensa e sem derrubar o posto).
func (z *Zone) stopAreaEvent() {
	e := z.ae
	z.aeClearMobs(e)
	z.aeRemoveBanner(e)
	z.ae = nil
	z.aeCool[e.def.ID] = z.now
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, e.def.Name+" interrompido", "Os Devotos recuam. O posto segue de pé.")
	z.flushAreaStatus()
}

type aeShare struct {
	p      *entity
	points float64
	tier   int
}

// aeRewards paga quem está online e contribuiu (dano + cura + presença no círculo) pela faixa:
// moedas e XP proporcionais (100%, 85% ou 75%) e a medalha; na falha, só uma fração das moedas e
// da XP, sem medalha. Devolve o nome de quem mais contribuiu.
func (z *Zone) aeRewards(e *areaEvent, won bool) string {
	d := e.def
	r := &d.Reward
	ids := map[uint32]bool{}
	for _, m := range []map[uint32]float64{e.dmg, e.heal, e.present} {
		for id := range m {
			ids[id] = true
		}
	}
	var list []aeShare
	for id := range ids {
		p := z.players[id]
		if p == nil {
			continue
		}
		pts := e.dmg[id] + e.heal[id] + e.present[id]*r.PresencePerSec
		if p.level < d.MinLevel {
			z.system(p, d.Name+": o seu nível é baixo demais para a recompensa deste evento.")
			continue
		}
		list = append(list, aeShare{p, pts, r.Tier(pts, e.pool)})
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
	for _, s := range list {
		z.aePay(e, s, won)
	}
	return list[0].p.name
}

func (z *Zone) aePay(e *areaEvent, s aeShare, won bool) {
	d, p := e.def, s.p
	r := &d.Reward
	if s.tier == game.MedalNone {
		z.system(p, fmt.Sprintf("%s: a sua ajuda foi pequena demais para receber recompensa.", d.Name))
		return
	}
	share := game.MedalShare[s.tier]
	mult := share
	medals := r.MedalCount(s.tier)
	if !won {
		mult *= r.FailMult
		medals = 0
	}
	var parts []string
	gold := int64(math.Round((r.GoldBase + r.GoldPerLvl*float64(p.level)) * mult))
	if gold > 0 {
		p.pl.inv.gold += gold
		z.trackGold(p, "area_event", gold)
		parts = append(parts, fmt.Sprintf("%d moedas", gold))
	}
	if def := game.Items[r.Item]; def != nil && medals > 0 {
		z.aeGiveItem(p, def, medals)
		parts = append(parts, itemLabel(def, medals))
	}
	z.inventoryChanged(p, false)
	bar := game.XPToNext(p.level)
	if p.level >= game.MasteryLevel {
		bar = game.MasteryPointXP
	}
	xp := int64(math.Round(float64(bar) * r.XPFrac * mult))
	if xp > 0 {
		parts = append(parts, fmt.Sprintf("%d de XP", xp))
	}
	z.system(p, fmt.Sprintf("%s: faixa %s (%.0f%% da recompensa). Recebeu: %s.", d.Name, game.MedalName[s.tier], share*100, strings.Join(parts, ", ")))
	if xp > 0 {
		z.grantXPFrom(p, xpAreaEvent, xp)
	}
	z.track(p, "area_event", d.ID, int64(math.Round(s.points)), map[string]any{"won": won, "tier": s.tier, "medals": medals})
}

// aeGiveItem: o item vai para a bolsa; o que não couber fica no chão, reservado para o jogador.
func (z *Zone) aeGiveItem(p *entity, def *game.ItemDef, count int) {
	left := p.pl.inv.add(def, count)
	if got := count - left; got > 0 {
		z.trackItem(p, true, def.ID, got, "area_event")
	}
	if left > 0 {
		if g := z.dropItem(p.pos, def, left, 0, p.id, questRewardReserve); g != nil {
			z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_GroundAdded{GroundAdded: &pb.GroundItemsAdded{Items: []*pb.GroundItem{z.groundPB(g)}}}}, 0)
			z.system(p, "Sua bolsa está cheia: o que sobrou ficou no chão, reservado para você.")
		}
	}
}

// ---------- Postos ----------

// postClosed: o posto de chave key está fechado agora (nesta zona).
func (z *Zone) postClosed(key string) bool {
	u, ok := z.postUntil[key]
	return ok && z.now < u
}

// stoneClosed: a pedra i está fechada (o posto dela caiu), na zona dela ou, para quem vem de
// outro mapa, pelo quadro dos postos do gerente.
func (z *Zone) stoneClosed(i int) bool {
	d := allStones[i]
	if d.stoneMap() == z.mapID() {
		return z.postClosed(d.key)
	}
	return z.mgr != nil && z.mgr.posts.isClosed(d.key, z.cfg.Clock())
}

// closePost derruba o posto por secs segundos.
func (z *Zone) closePost(key string, secs float64) {
	z.postUntil[key] = z.now + secs
	if z.mgr != nil {
		z.mgr.posts.set(key, z.wallAt(z.now+secs))
	}
	z.flushAreaStatus()
	z.log.Info("post closed", "post", key, "secs", secs)
}

// reopenPost: o posto volta a funcionar (o tempo passou ou o GM reabriu).
func (z *Zone) reopenPost(key string) {
	delete(z.postUntil, key)
	if z.mgr != nil {
		z.mgr.posts.set(key, time.Time{})
	}
	name := key
	if p := game.PostByKey(key); p != nil {
		name = p.Name
	}
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT, name+" reabriu",
		"A pedra de teleporte, a loja e o Irmão do posto voltam a atender.")
	z.flushAreaStatus()
	z.log.Info("post reopened", "post", key)
}

// postRefusal: a fala de quem fecha na cara de um jogador.
func (z *Zone) postRefusal(key string) string {
	name := key
	if p := game.PostByKey(key); p != nil {
		name = p.Name
	}
	left := z.postUntil[key] - z.now
	return fmt.Sprintf("O %s caiu: está fechado por mais %s.", name, durationText(math.Max(1, left)))
}

// spawnPostNPCs cria o mercador de cada posto cuja pedra está neste mapa.
func (z *Zone) spawnPostNPCs() {
	for i, p := range game.Posts {
		m := p.Merchant
		si := stoneIndex(p.Key)
		if m == nil || si < 0 || allStones[si].stoneMap() != z.mapID() {
			continue
		}
		pos := z.freeSpot(Vec2{X: m.X, Z: m.Z})
		n := &entity{
			id: postIDBase + uint32(i), kind: pb.EntityKind_ENTITY_KIND_NPC,
			name: m.Name, model: m.Model, level: 1, scale: 1,
			pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
			npc: &npcState{title: m.Title, role: pb.NpcRole_NPC_ROLE_SHOP, look: m.Look, post: p.Key},
		}
		for _, id := range m.Stock {
			if def := game.Items[id]; def != nil {
				n.npc.stock = append(n.npc.stock, def)
			}
		}
		z.addNPC(n)
	}
}

// tagPostNPC marca o NPC de história de um posto (o Irmão Caedmon) para fechar com ele.
func tagPostNPC(n *entity) {
	for _, p := range game.Posts {
		if p.Talker != "" && p.Talker == n.name && n.npc.post == "" {
			n.npc.post = p.Key
		}
	}
}

// ---------- Estado para o cliente ----------

func (z *Zone) areaStatusPB() *pb.AreaEventStatus {
	out := &pb.AreaEventStatus{}
	if e := z.ae; e != nil {
		out.Active = true
		out.Name = e.def.Name
		out.Stages = uint32(len(e.def.Stages))
		if e.warning {
			out.Warning = true
			out.Label = "O ataque começa em breve"
			out.EndsAtMs = z.wallAt(e.startAt).UnixMilli()
		} else {
			s := &e.def.Stages[e.stage]
			out.Stage = uint32(e.stage + 1)
			out.Label = s.Label
			out.EndsAtMs = z.wallAt(e.stageEnd).UnixMilli()
			switch s.Kind {
			case game.StageKill:
				out.Progress, out.Goal = uint32(e.killed), uint32(s.Count)
			case game.StageDefend:
				out.Progress, out.Goal = uint32(min(z.now-e.stageAt, s.Hold)), uint32(s.Hold)
				out.Detail = fmt.Sprintf("Estandarte %.0f%%", 100*e.bannerHP/s.TargetHP)
			case game.StageBoss:
				out.Goal = 1
				if m := z.monsters[e.boss]; m != nil && !m.dead {
					out.Detail = fmt.Sprintf("Chefe %.0f%%", 100*m.hp/m.stats.MaxHP)
				}
			}
		}
	}
	keys := make([]string, 0, len(z.postUntil))
	for k := range z.postUntil {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		cp := &pb.ClosedPost{Key: k, Name: k, ReopensAtMs: z.wallAt(z.postUntil[k]).UnixMilli()}
		if p := game.PostByKey(k); p != nil {
			cp.Name = p.Name
		}
		if si := stoneIndex(k); si >= 0 {
			cp.StoneId = uint32(si + 1)
		}
		for _, n := range z.npcs {
			if n.npc.post == k {
				cp.NpcIds = append(cp.NpcIds, n.id)
			}
		}
		slices.Sort(cp.NpcIds)
		out.Closed = append(out.Closed, cp)
	}
	return out
}

// areaEventsOnJoin: quem entra recebe o estado (só se houver evento ou posto fechado).
func (z *Zone) areaEventsOnJoin(p *entity) {
	if z.ae != nil || len(z.postUntil) > 0 {
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_AreaEvent{AreaEvent: z.areaStatusPB()}})
	}
}

// ---------- Comando de GM ----------

const areaUsage = "Uso: /evento area · /evento area <id> [agora|parar] · /evento area reabrir <posto>"

// areaEventsText: o estado dos eventos de área para o /evento.
func (z *Zone) areaEventsText() string {
	var parts []string
	for _, d := range game.AreaEvents {
		state := "desligado"
		if d.Enabled {
			state = "pronto"
		}
		if z.ae != nil && z.ae.def == d {
			state = "em andamento"
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", d.ID, state))
	}
	for k := range z.postUntil {
		parts = append(parts, fmt.Sprintf("posto %s fechado por %s", k, durationText(math.Max(1, z.postUntil[k]-z.now))))
	}
	return "Eventos de área: " + strings.Join(parts, ", ") + "."
}

func (z *Zone) gmAreaEvent(gm *entity, args []string) {
	if len(args) == 0 {
		z.system(gm, z.areaEventsText()+" "+areaUsage)
		return
	}
	if strings.ToLower(args[0]) == "reabrir" {
		if len(args) < 2 || !z.postClosed(args[1]) {
			z.system(gm, "Nenhum posto fechado com essa chave. "+areaUsage)
			return
		}
		z.reopenPost(args[1])
		z.system(gm, "Posto reaberto.")
		z.gmLog(gm, "evento", "area", "reabrir "+args[1])
		return
	}
	d := game.AreaEventByID(args[0])
	if d == nil || !d.Enabled || eventMap(d) != z.mapID() {
		z.system(gm, "Evento desconhecido neste mapa. "+areaUsage)
		return
	}
	sub := ""
	if len(args) > 1 {
		sub = strings.ToLower(args[1])
	}
	switch sub {
	case "", "agora":
		if z.ae != nil {
			z.system(gm, "Já há um evento de área em andamento.")
			return
		}
		z.startAreaEvent(d, "gm")
		if sub == "agora" && z.ae != nil && z.ae.warning {
			z.beginAreaEvent()
		}
		z.system(gm, d.Name+" disparado.")
	case "parar":
		if z.ae == nil || z.ae.def != d {
			z.system(gm, "Esse evento não está em andamento.")
			return
		}
		z.stopAreaEvent()
		z.system(gm, d.Name+" interrompido (sem recompensa).")
	default:
		z.system(gm, areaUsage)
		return
	}
	z.gmLog(gm, "evento", "area:"+d.ID, sub)
}

// postKeyOf: a chave do posto cuja pedra tem a chave key ("" = a pedra não é de um posto).
func postKeyOf(key string) string {
	if game.PostByKey(key) != nil {
		return key
	}
	return ""
}

// stoneRefusal: a recusa da pedra i fechada.
func (z *Zone) stoneRefusal(i int) string {
	d := allStones[i]
	if z.postClosed(d.key) {
		return z.postRefusal(d.key)
	}
	return fmt.Sprintf("A pedra de %s está apagada: o posto caiu e ainda não reabriu.", d.name)
}

package zone

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Raid boss: um chefe para vários grupos, preso ao covil, com fases, avisos no chão
// (o golpe cai no fim do aviso, em quem ainda estiver dentro), invocações e fúria. A XP
// é dividida pelo dano de cada grupo; o saque fica reservado para quem mais causou dano.
// O renascimento (horas) é em relógio de parede e fica salvo no banco.
const (
	raidIdleReset   = 15.0 // segundos sem ninguém bater: o chefe volta ao trono
	raidMinShare    = 0.05 // fração mínima do dano para receber XP
	raidReserve     = 60.0 // segundos de saque reservado
	raidNear        = 60.0 // quem está a esta distância ouve o chefe
	raidWarnBefore  = 10 * time.Minute
	raidPhase2At    = 0.70
	raidPhase3At    = 0.40
	raidEnrageAt    = 0.20
	raidEnrageSpeed = 1.4  // fúria: ataques 40% mais rápidos
	raidFinalAfter  = 720. // 12 min de luta: fúria final
	raidFinalMult   = 3.0
	raidLootSpread  = 4.0

	// Os golpes de área (Onda, Chuva e Marca) são dados: game.MonsterDef.Abilities do Ozrhal.
	raidMarkRadius, raidMarkDelay, raidMarkMult = game.RaidMarkRadius, game.RaidMarkDelay, game.RaidMarkMult
	raidSummonsP2, raidSummonsP3                = 4, 6

	summonIDBase = monsterIDBase + 1<<20 // invocações (não colidem com os monstros fixos)
)

// Golpes do raid nos CombatEvent: o cliente mostra o dano sem animar o chefe batendo
// (o visual sai do Telegraph).
const (
	RaidHitWave = game.HitRaidWave
	RaidHitRain = game.HitRaidRain
	RaidHitMark = game.HitRaidMark
)

type raidState struct {
	phase               int
	fighting            bool
	fightStart, lastHit float64 // z.now
	fightWall           time.Time
	nextWave, nextRain  float64
	nextMark            float64
	enraged, final      bool
	summons             []uint32
	damage              map[raidKey]float64
	byPlayer            map[uint32]float64
	names               map[raidKey]string // nome para o anúncio (grupo de X, clã Y)
	total               float64
	spawnAt, killedAt   time.Time // morto: renasce em spawnAt (relógio)
	warned              bool
	lastStatusPhase     int
	lastStatusEnraged   bool
	lastStatusFinal     bool
	lastStatusFighting  bool
	// Chefe de campo (fieldboss.go): a volta das invocações que curam e o túmulo.
	retAt, retHeal float64
	retCount       int
	tomb           uint32
}

func newRaidState() *raidState {
	return &raidState{phase: 1, damage: map[raidKey]float64{}, byPlayer: map[uint32]float64{}, names: map[raidKey]string{}}
}

// clock: o relógio de cada golpe do Ozrhal (id da habilidade na def).
func (r *raidState) clock(id string) *float64 {
	switch id {
	case "wave":
		return &r.nextWave
	case "rain":
		return &r.nextRain
	case "mark":
		return &r.nextMark
	}
	return nil
}

// initRaid prepara o estado do raid boss e aplica o renascimento salvo.
func (z *Zone) initRaid(m *entity) { // também do chefe de campo (fieldboss.go)
	r := newRaidState()
	m.mon.raid = r
	for _, b := range z.cfg.Bosses {
		if b.ID != m.mon.def.ID {
			continue
		}
		if b.RespawnAt.After(z.cfg.Clock()) {
			m.dead, m.hp = true, 0
			m.mon.hidden = true
			r.spawnAt, r.killedAt = b.RespawnAt, b.KilledAt
			z.setWindow(m, b.KilledAt, b.RespawnAt)
		}
	}
}

// thinkRaid roda dentro do think, depois de esquecer quem morreu ou fugiu. Devolve true
// se o chefe reiniciou (o think para por aqui).
func (z *Zone) thinkRaid(m *entity) bool {
	r := m.mon.raid
	if !r.fighting {
		if len(m.mon.hate) > 0 {
			z.raidEngage(m)
		}
		return false
	}
	// Todos morreram ou fugiram, ou ninguém bate há um tempo: volta ao trono.
	if len(m.mon.hate) == 0 || z.now-r.lastHit > raidIdleReset {
		z.resetMonster(m)
		return true
	}
	if m.mon.def.FieldBoss != nil {
		z.fieldPhases(m) // chefe de campo: fases por dados, sem fúria final
		z.thinkAbilities(m)
		z.raidStatusIfChanged(m)
		return false
	}
	z.raidPhases(m)
	z.thinkAbilities(m)
	if !r.final && z.now-r.fightStart >= raidFinalAfter {
		r.final = true
		m.stats.PAtk *= raidFinalMult
		m.stats.MAtk *= raidFinalMult
		z.raidSay(m, fmt.Sprintf("%s perde a paciência: os golpes ficam devastadores!", m.name))
	}
	z.raidStatusIfChanged(m)
	return false
}

func (z *Zone) raidEngage(m *entity) {
	r := m.mon.raid
	r.fighting = true
	r.fightStart, r.lastHit = z.now, z.now
	r.fightWall = z.cfg.Clock()
	z.startAbilities(m)
	if m.mon.def.FieldBoss == nil {
		z.raidSay(m, fmt.Sprintf("%s se ergue do trono!", m.name))
	}
	z.raidStatusIfChanged(m)
}

// raidPhases: troca de fase pela vida (com invocações) e fúria.
func (z *Zone) raidPhases(m *entity) {
	r := m.mon.raid
	frac := m.hp / m.stats.MaxHP
	if r.phase < 2 && frac <= raidPhase2At {
		r.phase = 2
		z.phaseAbilities(m, 2)
		z.summon(m, raidSummonsP2)
		z.raidSay(m, fmt.Sprintf("%s: \"Levantem-se, meus guardas!\"", m.name))
	}
	if r.phase < 3 && frac <= raidPhase3At {
		r.phase = 3
		z.phaseAbilities(m, 3)
		z.summon(m, raidSummonsP3)
		z.raidSay(m, fmt.Sprintf("%s: \"A morte marca os que me desafiam!\"", m.name))
	}
	if !r.enraged && frac <= raidEnrageAt {
		r.enraged = true
		m.stats.AtkInterval = m.mon.def.Interval / raidEnrageSpeed
		z.raidSay(m, fmt.Sprintf("%s entra em fúria!", m.name))
	}
}

// summon chama n Guardas Ossários em volta do chefe (não renascem).
func (z *Zone) summon(m *entity, n int) {
	def := game.Monsters["bone_guard"]
	if def == nil {
		return
	}
	m.mon.raid.summons = append(m.mon.raid.summons, z.summonAround(m, def, max(1, m.level-5), n, 4)...)
}

// dismissSummons tira as invocações do chefe do mundo.
func (z *Zone) dismissSummons(m *entity) {
	for _, id := range m.mon.raid.summons {
		if g := z.monsters[id]; g != nil {
			z.removeMonster(g)
		}
	}
	m.mon.raid.summons = nil
}

// raidReset: o chefe desistiu da luta. Volta à fase 1, sem fúria, sem invocações e sem a
// contagem de dano (o resetMonster leva de volta ao trono, onde a vida enche).
func (z *Zone) raidReset(m *entity) {
	r := m.mon.raid
	if r.fighting && m.mon.def.FieldBoss == nil {
		z.raidSay(m, fmt.Sprintf("%s voltou ao trono.", m.name))
	}
	z.raidClear(m)
	z.raidStatusIfChanged(m)
}

func (z *Zone) raidClear(m *entity) {
	r := m.mon.raid
	z.dismissSummons(m)
	m.mon.stopAbilities() // cancela os avisos pendentes
	r.phase, r.fighting, r.fightStart, r.lastHit = 1, false, 0, 0
	r.fightWall = time.Time{}
	r.enraged, r.final = false, false
	r.retAt, r.retHeal, r.retCount = 0, 0, 0
	clear(r.damage)
	clear(r.byPlayer)
	clear(r.names)
	r.total = 0
	m.stats = game.MonsterStats(m.mon.def, m.level) // desfaz a fúria
}

// raidCredit soma o dano de um jogador no chefe (antes da vida cair).
func (z *Zone) raidCredit(m, src *entity, amount float64) {
	r := m.mon.raid
	if r == nil || src == nil || src.pl == nil || amount <= 0 {
		return
	}
	r.lastHit = z.now
	k, name := z.raidKeyOf(src)
	r.damage[k] += amount
	r.names[k] = name
	r.byPlayer[src.id] += amount
	r.total += amount
}

// raidKilled: XP por dano, saque reservado para quem mais causou dano, anúncio e
// renascimento salvo.
func (z *Zone) raidKilled(m *entity) {
	r := m.mon.raid
	ms := m.mon
	pools := r.pools()

	// Saque: reservado para a pontuação com mais dano (o representante é quem dela mais bateu).
	var owner uint32
	var ownerClan int64
	topName := ""
	if len(pools) > 0 {
		win := pools[0].key
		topName = r.names[win]
		if win.kind == 'c' {
			ownerClan = win.id
		}
		best := -1.0
		for _, p := range z.poolMembers(win) {
			if v := r.byPlayer[p.id]; v > best {
				owner, best = p.id, v
			}
		}
	}
	z.dropRaidLoot(m, owner, ownerClan)

	// XP: cada pontuação com 5% ou mais do dano leva a parte dela.
	for _, pl := range pools {
		share := pl.damage / math.Max(1, r.total)
		members := z.poolMembers(pl.key)
		if share < raidMinShare {
			for _, p := range members {
				z.systemAfterFlush(p, fmt.Sprintf("Seu lado causou só %.0f%% do dano em %s: sem XP (mínimo de %.0f%%).", share*100, m.name, raidMinShare*100))
			}
			continue
		}
		takers := z.raidTakers(members, m.pos)
		for _, p := range takers {
			z.systemAfterFlush(p, fmt.Sprintf("Seu lado causou %.0f%% do dano em %s.", share*100, m.name))
		}
		z.shareMonsterXPScaled(m, takers, share)
	}
	// Conquista: todo mundo que causou dano e ainda está aqui participou. O chefe de campo
	// só abre o Bestiário (a conquista e o contrato de raid são do raid).
	for _, id := range slices.Sorted(maps.Keys(r.byPlayer)) {
		if p := z.players[id]; p != nil {
			if ms.def.Raid {
				z.achEvent(p, game.AchRaid, ms.def.ID)
				z.contractRaid(p, ms.def.ID)
			}
			z.bestiaryKill(p, m) // Bestiário: 1 abate abre a entrada do chefe
		}
	}

	title := "Raid boss derrotado"
	if ms.def.FieldBoss != nil {
		title = "Chefe derrotado"
	}
	text := fmt.Sprintf("%s foi derrotado!", m.name)
	if topName != "" {
		text += " Maior dano: " + topName + "."
	}
	z.afterFlush(func() { z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT, title, text) })

	z.raidClear(m)
	wall := z.cfg.Clock()
	def := ms.def
	lo, hi := respawnSeconds(def)
	delay := lo + z.rng.Float64()*(hi-lo)
	r.killedAt = wall
	r.spawnAt = wall.Add(time.Duration(delay * float64(time.Second)))
	r.warned = false
	z.setWindow(m, wall, r.spawnAt)
	ms.forget()
	ms.corpseEnd = z.now + corpseSeconds*2
	z.saveBoss(BossData{ID: def.ID, RespawnAt: r.spawnAt, KilledAt: wall, TopName: topName})
	z.log.Info("raid boss killed", "boss", def.ID, "top", topName, "respawn_at", r.spawnAt)
	if def.FieldBoss != nil {
		z.spawnTomb(m, wall, topName)
	}
	z.bossChanged(m)
}

// updateDeadRaid: aviso antes (10 min no raid, 5 no chefe de campo) e renascimento na hora
// marcada (relógio).
func (z *Zone) updateDeadRaid(m *entity) {
	r := m.mon.raid
	if !m.mon.hidden || r.spawnAt.IsZero() {
		return
	}
	wall := z.cfg.Clock()
	if f := m.mon.def.FieldBoss; f != nil {
		z.updateDeadField(m, f, wall)
		return
	}
	if !r.warned && !wall.Before(r.spawnAt.Add(-raidWarnBefore)) && wall.Before(r.spawnAt) {
		r.warned = true
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, "O chão treme…",
			fmt.Sprintf("Algo se mexe na Cripta Esquecida. %s desperta em breve.", m.mon.def.Name))
	}
	if wall.Before(r.spawnAt) {
		return
	}
	r.spawnAt, r.warned = time.Time{}, false
	z.reviveMonster(m)
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SPAWN, "Raid boss",
		fmt.Sprintf("%s despertou na Cripta Esquecida!", m.mon.def.Name))
	z.log.Info("raid boss spawned", "boss", m.mon.def.ID)
	z.bossChanged(m)
}

// raidSay: aviso do sistema para quem está perto do chefe.
func (z *Zone) raidSay(m *entity, text string) {
	for _, id := range z.sortedPlayerIDs() {
		if p := z.players[id]; dist(p.pos, m.pos) <= raidNear {
			z.system(p, text)
		}
	}
}

// ---------- Estado dos chefes (mapa e barra do raid) ----------

// raidStatusIfChanged envia o estado só quando a fase, a fúria ou a luta mudaram.
func (z *Zone) raidStatusIfChanged(m *entity) {
	r := m.mon.raid
	if r.phase != r.lastStatusPhase || r.enraged != r.lastStatusEnraged || r.final != r.lastStatusFinal || r.fighting != r.lastStatusFighting {
		z.bossChanged(m)
	}
}

// ---------- Banco ----------

// ---------- Comandos de teste (só com DEV_LOGIN, nunca em produção) ----------

// devRaid: /raid nascer | avisar | ir | vida N | matar | reiniciar
func (z *Zone) devRaid(p *entity, args []string) {
	var m *entity
	for _, e := range z.monsters {
		if e.mon.raid != nil {
			m = e
		}
	}
	if m == nil || len(args) == 0 {
		z.system(p, "Uso: /raid nascer | avisar | ir | vida N | matar | reiniciar")
		return
	}
	r := m.mon.raid
	switch args[0] {
	case "nascer":
		if !m.dead {
			z.system(p, m.name+" já está vivo.")
			return
		}
		m.mon.hidden = true
		r.spawnAt = z.cfg.Clock()
		r.warned = true
	case "avisar":
		if !m.dead {
			z.system(p, m.name+" está vivo.")
			return
		}
		m.mon.hidden = true
		r.spawnAt = z.cfg.Clock().Add(raidWarnBefore - time.Minute)
		r.warned = false
	case "ir":
		if p.dead {
			return
		}
		p.clearAction()
		p.teleportTo(z.freeSpot(Vec2{X: m.mon.home.X - 14, Z: m.mon.home.Z - 14}))
		z.forgetTarget(p.id)
	case "vida":
		if m.dead || len(args) < 2 {
			return
		}
		var pct float64
		if _, err := fmt.Sscanf(args[1], "%g", &pct); err != nil || pct <= 0 || pct > 100 {
			z.system(p, "Uso: /raid vida 1–100")
			return
		}
		m.hp = m.stats.MaxHP * pct / 100
	case "matar":
		if m.dead {
			return
		}
		m.mon.hate[p.id] += m.hp
		z.damage(p, m, m.hp+1, nil)
	case "reiniciar":
		if !m.dead {
			z.resetMonster(m)
		}
	default:
		z.system(p, "Uso: /raid nascer | avisar | ir | vida N | matar | reiniciar")
	}
}

package zone

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Monstros raros (named; game/rares.go, tunnel#133). Quando um hospedeiro renasce
// (reviveMonster), com a chance do raro e se a condição deixar (período, parte da área e
// 45 min desde a última aparição), ele renasce como o raro: o mesmo monstro (mesmo id e
// mesma casa), com a def do raro, nível +2, vida ×4 e 1,2× maior. A AOI manda o Spawn novo
// (spawnSeq), com Entity.rare para o nome dourado. No renascimento seguinte ele volta a
// ser o hospedeiro.
//
// Só um de cada raro no mundo. A aparição acaba quando ele morre, adormece (troca dia/noite
// do hospedeiro) ou vai embora (o raro noturno some no amanhecer, fora de combate); o
// mínimo de 45 min conta do fim (e nunca é menos que 45 min desde o nascimento). A hora
// fica no banco, na tabela dos chefes (world_bosses, id "rare:<id do raro>", respawn_at =
// a primeira hora em que ele pode nascer de novo): reiniciar o servidor não zera.

const rareBossPrefix = "rare:"

type rareWorld struct {
	ready bool
	rng   *rand.Rand           // sorteio próprio: não muda a sequência do z.rng (dano, saque)
	next  map[string]time.Time // raro → a primeira hora em que pode nascer de novo (relógio)
	alive map[string]uint32    // raro → id do monstro que está com ele
	// Mecânicas, por monstro (limpas quando ele volta a ser o hospedeiro).
	risen  map[uint32]bool     // Ossudo já levantou nesta vida
	formed map[uint32]bool     // Capitão já chamou a guarda nesta luta
	guards map[uint32][]uint32 // Capitão → escudeiros (invocações)
	// chance: testes forçam a chance (nil = a da def).
	chance func(*game.RareDef) float64
}

// rareSetup prepara o estado (na primeira vez) e lê a hora salva de cada raro.
func (z *Zone) rareSetup() *rareWorld {
	rw := &z.rares
	if rw.ready {
		return rw
	}
	rw.ready = true
	rw.rng = rand.New(rand.NewPCG(z.cfg.Seed^0x7261726573, z.cfg.Seed))
	rw.next, rw.alive = map[string]time.Time{}, map[string]uint32{}
	rw.risen, rw.formed, rw.guards = map[uint32]bool{}, map[uint32]bool{}, map[uint32][]uint32{}
	for _, b := range z.cfg.Bosses {
		if id, ok := strings.CutPrefix(b.ID, rareBossPrefix); ok {
			rw.next[id] = b.RespawnAt
		}
	}
	return rw
}

// rareRevive roda no começo de reviveMonster: o raro que renasce volta a ser o hospedeiro,
// e o hospedeiro pode renascer como raro.
func (z *Zone) rareRevive(m *entity) {
	ms := m.mon
	if ms.summon || ms.raid != nil || ms.invader {
		return
	}
	if ms.def.Rare != nil {
		z.rareEnd(m)
		z.rareRestore(m)
	}
	if rares := game.RaresByHost[ms.def.ID]; len(rares) > 0 {
		z.rareRoll(m, rares)
	}
}

// rareRoll sorteia, entre os raros do hospedeiro que a condição deixa, se ele renasce raro.
func (z *Zone) rareRoll(m *entity, rares []*game.RareDef) {
	rw := z.rareSetup()
	wall := z.cfg.Clock()
	for _, r := range rares {
		if !z.rareAllowed(r, m, wall) {
			continue
		}
		chance := r.Chance
		if rw.chance != nil {
			chance = rw.chance(r)
		}
		if rw.rng.Float64() < chance {
			z.rareBecome(m, r)
			return
		}
	}
}

// rareAllowed: o raro pode nascer neste hospedeiro agora (período, área, ninguém com ele
// e o mínimo de 45 min).
func (z *Zone) rareAllowed(r *game.RareDef, m *entity, wall time.Time) bool {
	rw := z.rareSetup()
	if !r.When.Active(z.night) || !r.Where.Contains(m.mon.home.X, m.mon.home.Z) {
		return false
	}
	if _, taken := rw.alive[r.ID]; taken {
		return false
	}
	return !wall.Before(rw.next[r.ID])
}

// rareBecome transforma o hospedeiro (que vai renascer) no raro e avisa quem está perto.
func (z *Zone) rareBecome(m *entity, r *game.RareDef) {
	rw := z.rareSetup()
	def := r.Monster()
	ms := m.mon
	ms.def = def
	m.name, m.model, m.scale = def.Name, def.Model, def.Scale
	m.level = r.RareLevel(ms.spawn.Level)
	m.stats = game.MonsterStats(def, m.level)
	ms.abilNext = nil
	delete(rw.risen, m.id)
	delete(rw.formed, m.id)
	wall := z.cfg.Clock()
	rw.alive[r.ID] = m.id
	z.rareSaveNext(r, wall, wall.Add(game.RareCooldown))
	z.log.Info("rare spawn", "rare", r.ID, "id", m.id, "level", m.level, "x", ms.home.X, "z", ms.home.Z)
	z.trackAt(nil, "rare_spawn", r.ID, int64(m.level), ms.home, map[string]any{"host": r.Host, "night": z.night})
	z.rareSay(ms.home, r.Rumor)
}

// rareRestore: o raro volta a ser o hospedeiro (renasce como ele).
func (z *Zone) rareRestore(m *entity) {
	ms := m.mon
	host := game.Monsters[ms.def.Rare.Host]
	ms.def = host
	m.name, m.model, m.scale = host.Name, host.Model, host.Scale
	m.level = ms.spawn.Level
	m.stats = game.MonsterStats(host, m.level)
	ms.abilNext = nil
	rw := z.rareSetup()
	delete(rw.risen, m.id)
	delete(rw.formed, m.id)
	z.rareDismissGuards(m.id, true)
}

// rareEnd: a aparição acabou (morte, sono ou saída). O próximo só depois de 45 min.
func (z *Zone) rareEnd(m *entity) {
	r := m.mon.def.Rare
	rw := z.rareSetup()
	if r == nil || rw.alive[r.ID] != m.id {
		return
	}
	delete(rw.alive, r.ID)
	wall := z.cfg.Clock()
	next := wall.Add(game.RareCooldown)
	if rw.next[r.ID].After(next) {
		next = rw.next[r.ID]
	}
	z.rareSaveNext(r, wall, next)
}

func (z *Zone) rareSaveNext(r *game.RareDef, at, next time.Time) {
	z.rares.next[r.ID] = next
	z.saveBoss(BossData{ID: rareBossPrefix + r.ID, RespawnAt: next, KilledAt: at})
}

// rareKilled: o raro morreu (kill). Conta como o hospedeiro nas missões, anota e avisa.
func (z *Zone) rareKilled(dead *entity, takers []*entity) {
	r := dead.mon.def.Rare
	if r == nil {
		return
	}
	z.rareEnd(dead)
	names := make([]string, 0, len(takers))
	for _, p := range takers {
		names = append(names, p.name)
		z.questEvent(p, game.GoalKill, r.Host)
		z.trackAt(p, "rare_kill", r.ID, int64(dead.level), dead.pos, map[string]any{"party": len(takers)})
	}
	z.log.Info("rare kill", "rare", r.ID, "id", dead.id, "by", names)
	if len(names) > 0 {
		z.rareSay(dead.pos, fmt.Sprintf("%s foi derrotado por %s!", r.Name, strings.Join(names, ", ")))
	}
}

// rareRise: Ossudo, o Teimoso, levanta uma vez com parte da vida em vez de morrer. Roda
// no começo de kill; true = não morreu.
func (z *Zone) rareRise(m *entity) bool {
	r := m.mon.def.Rare
	if r == nil || r.Rise <= 0 {
		return false
	}
	rw := z.rareSetup()
	if rw.risen[m.id] {
		return false
	}
	rw.risen[m.id] = true
	m.hp = math.Max(1, m.stats.MaxHP*r.Rise)
	z.rareSay(m.pos, r.Shout)
	return true
}

// updateRares (1×/s): fecha as aparições que acabaram, tira o raro noturno no amanhecer e
// chama a guarda do Capitão.
func (z *Zone) updateRares() {
	rw := z.rareSetup()
	for _, r := range game.Rares {
		id, ok := rw.alive[r.ID]
		if !ok {
			continue
		}
		m := z.monsters[id]
		if m == nil || m.mon.def != r.Monster() {
			delete(rw.alive, r.ID)
			continue
		}
		switch {
		case m.dead: // morreu ou adormeceu (putToSleep)
			z.rareEnd(m)
		case !r.When.Active(z.night) && !z.monsterFighting(m):
			z.rareLeave(m)
		default:
			z.rareFormation(m, r)
		}
	}
	for cid := range rw.guards {
		c := z.monsters[cid]
		z.rareDismissGuards(cid, c == nil || c.dead || c.mon.def.Rare == nil)
	}
}

// rareLeave: o raro some do mundo sem morrer (sem saque nem XP) e renasce como o
// hospedeiro no tempo de sempre.
func (z *Zone) rareLeave(m *entity) {
	ms := m.mon
	z.rareEnd(m)
	z.rareSay(m.pos, m.name+" desaparece com a luz do dia.")
	m.dead = true
	ms.corpseEnd = z.now + 1.5/float64(z.cfg.TickRate) // o Despawn sai pela AOI
	ms.respawnAt = z.now + ms.spawn.Respawn*(1+z.rares.rng.Float64())
	ms.returning = false
	ms.hate = map[uint32]float64{}
	ms.stopAbilities()
	for _, ef := range m.effects {
		z.frame.ends = append(z.frame.ends, &pb.EffectEnd{TargetId: m.id, Effect: pb.Effect(ef.kind)})
	}
	m.effects = nil
	m.clearAction()
	m.stopMoving()
	z.forgetTarget(m.id)
	z.rareDismissGuards(m.id, true)
}

// rareFormation: "Formação!" do Capitão Ossudo, uma vez por luta, ao chegar a GuardsAt.
func (z *Zone) rareFormation(m *entity, r *game.RareDef) {
	if r.Guards <= 0 {
		return
	}
	rw := z.rareSetup()
	ms := m.mon
	if len(ms.hate) == 0 {
		if m.hp >= m.stats.MaxHP {
			delete(rw.formed, m.id) // voltou para casa inteiro: a próxima luta chama de novo
		}
		return
	}
	if rw.formed[m.id] || m.hp > m.stats.MaxHP*r.GuardsAt {
		return
	}
	rw.formed[m.id] = true
	z.rareSay(m.pos, r.Shout)
	def := game.Monsters[game.RareGuardID]
	level := max(1, m.level-2)
	for i := range r.Guards {
		a := 2 * math.Pi * float64(i) / float64(r.Guards)
		spot := z.freeSpot(Vec2{X: m.pos.X + 2.5*math.Cos(a), Z: m.pos.Z + 2.5*math.Sin(a)})
		z.summonSeq++
		g := &entity{
			id: summonIDBase + z.summonSeq, kind: pb.EntityKind_ENTITY_KIND_MONSTER,
			name: def.Name, model: def.Model, level: level, scale: def.Scale,
			pos: spot, dest: spot, lastCombat: z.now,
			stats: game.MonsterStats(def, level),
			mon: &monsterState{
				def: def, spawn: game.SpawnPoint{Monster: def, Level: level, X: spot.X, Z: spot.Z},
				home: ms.home, hate: map[uint32]float64{}, summon: true,
			},
		}
		g.hp = g.stats.MaxHP
		g.sentHP, g.sentMaxHP = g.hpInt(), g.maxHPInt()
		for pid, v := range ms.hate {
			g.mon.hate[pid] = v // já chegam bravos com quem está batendo no Capitão
		}
		z.monsters[g.id] = g // Spawn pela AOI
		rw.guards[m.id] = append(rw.guards[m.id], g.id)
		// O escudo vai no tick seguinte, quando o cliente já recebeu o Spawn.
		shield := &game.EffectDef{Kind: game.EffectShield, Duration: game.RareGuardShieldTime, Value: g.stats.MaxHP * game.RareGuardShield}
		z.afterFlush(func() {
			if z.monsters[g.id] == g && !g.dead {
				z.applyEffect(g, g, shield)
			}
		})
	}
	z.aoiDirty = true
}

// rareDismissGuards tira do mundo os escudeiros do Capitão cid que estão fora de combate,
// quando o Capitão saiu da luta ou acabou (gone: morreu, foi embora ou voltou a ser o
// hospedeiro). Quem ainda luta fica até a luta acabar.
func (z *Zone) rareDismissGuards(cid uint32, gone bool) {
	rw := z.rareSetup()
	ids, ok := rw.guards[cid]
	if !ok {
		return
	}
	c := z.monsters[cid]
	idle := gone || c == nil || len(c.mon.hate) == 0
	kept := ids[:0]
	for _, id := range ids {
		g := z.monsters[id]
		switch {
		case g == nil:
		case idle && !z.monsterFighting(g):
			z.removeMonster(g)
		default:
			kept = append(kept, id)
		}
	}
	if len(kept) == 0 {
		delete(rw.guards, cid)
	} else {
		rw.guards[cid] = kept
	}
}

// rareSay: aviso do sistema para quem está a até RareRumorRange de at.
func (z *Zone) rareSay(at Vec2, text string) {
	if text == "" {
		return
	}
	for _, id := range z.sortedPlayerIDs() {
		if p := z.players[id]; dist(p.pos, at) <= game.RareRumorRange {
			z.system(p, text)
		}
	}
}

// trackAt: track com a posição do acontecimento (o raro nasce sem jogador por perto).
func (z *Zone) trackAt(p *entity, kind, ref string, amount int64, at Vec2, extra map[string]any) {
	if z.eventSink == nil {
		return
	}
	e := GameEvent{At: z.cfg.Clock(), Kind: kind, Ref: ref, Amount: amount, Extra: extra}
	if p != nil && p.pl != nil {
		e.CharacterID, e.AccountID, e.Level = int64(p.id), p.pl.accountID, p.level
	}
	e.X, e.Z = at.X, at.Z
	select {
	case z.telemetry <- e:
	default:
		z.droppedEvents++
	}
}

// ---------- Comando de teste (só com DEV_LOGIN, nunca em produção) ----------

// devRare: /raro [ossudo | capitao | vossk | omrun] [ir]. Sem nome, lista os raros. Com o
// nome, o hospedeiro mais perto (vivo, fora de combate) renasce já como o raro, sem olhar
// chance, condição nem os 45 min; "ir" leva você para perto dele.
func (z *Zone) devRare(p *entity, args []string) {
	rw := z.rareSetup()
	if len(args) == 0 {
		wall := z.cfg.Clock()
		for _, r := range game.Rares {
			state := "pode nascer"
			if id, ok := rw.alive[r.ID]; ok {
				m := z.monsters[id]
				state = fmt.Sprintf("vivo em (%.0f, %.0f), nível %d", m.pos.X, m.pos.Z, m.level)
			} else if left := rw.next[r.ID].Sub(wall); left > 0 {
				state = fmt.Sprintf("espera %s", left.Round(time.Second))
			}
			z.system(p, fmt.Sprintf("%s (%s): %s.", r.Name, devRareKey(r), state))
		}
		z.system(p, "Uso: /raro ossudo | capitao | vossk | omrun [ir]")
		return
	}
	var r *game.RareDef
	for _, d := range game.Rares {
		if devRareKey(d) == strings.ToLower(args[0]) {
			r = d
		}
	}
	if r == nil {
		z.system(p, "Uso: /raro ossudo | capitao | vossk | omrun [ir]")
		return
	}
	m := z.monsters[rw.alive[r.ID]]
	if _, ok := rw.alive[r.ID]; !ok {
		m = nil
		best := math.MaxFloat64
		for _, e := range z.monsters {
			ms := e.mon
			if ms.def.ID != r.Host || ms.summon || ms.invader || e.dead || ms.dormant || z.monsterFighting(e) {
				continue
			}
			if d := dist(e.pos, p.pos); m == nil || d < best || (d == best && e.id < m.id) {
				m, best = e, d
			}
		}
		if m == nil {
			z.system(p, "Nenhum "+game.Monsters[r.Host].Name+" livre para virar "+r.Name+".")
			return
		}
		z.rareBecome(m, r)
		z.reviveDevRare(m)
	}
	if len(args) > 1 && args[1] == "ir" && !p.dead {
		p.clearAction()
		p.teleportTo(z.freeSpot(Vec2{X: m.pos.X + 6, Z: m.pos.Z}))
		z.forgetTarget(p.id)
	}
	z.system(p, fmt.Sprintf("%s está em (%.0f, %.0f), nível %d.", r.Name, m.pos.X, m.pos.Z, m.level))
}

// reviveDevRare: renasce o monstro já transformado, no ponto de origem (o reviveMonster
// sem o sorteio do rareRevive).
func (z *Zone) reviveDevRare(m *entity) {
	ms := m.mon
	m.dead = false
	ms.hidden, ms.returning = false, false
	ms.hate = map[uint32]float64{}
	ms.stopAbilities()
	ms.windowStart, ms.windowEnd = time.Time{}, time.Time{}
	m.pos, m.dest, m.path = ms.home, ms.home, nil
	m.hp = m.stats.MaxHP
	m.effects = nil
	m.clearAction()
	m.sentHP, m.sentMaxHP = m.hpInt(), m.maxHPInt()
	m.spawnSeq++
	z.aoiDirty = true
}

func devRareKey(r *game.RareDef) string {
	switch r.ID {
	case "rare_captain":
		return "capitao"
	}
	return strings.TrimPrefix(r.ID, "rare_")
}

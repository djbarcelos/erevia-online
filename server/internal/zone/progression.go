package zone

import (
	"fmt"
	"math"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const townSpawnRadius = 5.0

// kill trata a morte de qualquer entidade.
func (z *Zone) kill(dead, killer *entity) {
	dead.dead = true
	dead.hp = 0
	z.spreadPlague(dead)
	for _, ef := range dead.effects {
		z.frame.ends = append(z.frame.ends, &pb.EffectEnd{TargetId: dead.id, Effect: pb.Effect(ef.kind)})
	}
	dead.effects = nil
	dead.clearAction()
	dead.stopMoving()
	z.frame.deaths = append(z.frame.deaths, dead.id)

	if dead.mon != nil && dead.mon.raid != nil {
		z.raidKilled(dead)
		return
	}
	if dead.mon != nil {
		ms := dead.mon
		// O drop fica reservado para quem causou mais dano (lido antes de zerar o ódio), e a
		// XP vai para o grupo (ou jogador) que causou mais dano no total.
		z.dropLoot(dead, topHate(ms.hate))
		takers := z.creditGroup(ms.hate, dead.pos)
		if len(takers) == 0 && killer != nil && killer.pl != nil && !killer.dead {
			takers = []*entity{killer} // sem ódio registrado (ex.: veneno de quem saiu)
		}
		ms.hate = map[uint32]float64{}
		ms.corpseEnd = z.now + corpseSeconds
		ms.respawnAt = z.now + ms.spawn.Respawn*(1+z.rng.Float64())
		if ms.def.Boss && !ms.summon {
			// Janela mostrada no mapa: de Respawn a 2×Respawn a partir de agora.
			wall := z.cfg.Clock()
			ms.windowStart = wall.Add(time.Duration(ms.spawn.Respawn * float64(time.Second)))
			ms.windowEnd = wall.Add(time.Duration(2 * ms.spawn.Respawn * float64(time.Second)))
			z.bossChanged(dead)
		}
		z.shareMonsterXP(dead, takers)
		return
	}

	z.cancelChannel(dead, "")

	// Jogador. Para outro jogador: karma/vitória para quem matou e nenhuma XP perdida
	// (a não ser por quem tem karma, que perde o dobro e pode deixar itens caírem).
	var pk *entity
	if killer != nil && killer.pl != nil && killer != dead {
		pk = killer
	}
	byPlayer := z.pvpDeath(dead, pk)
	chaotic := dead.pl.karma > 0
	dead.pl.flagUntil = 0
	if killer != nil && killer != dead {
		dead.pl.killedBy = killer.name
	}
	cause := ""
	switch {
	case killer != nil && killer.mon != nil:
		cause = "monster:" + killer.mon.def.ID
	case killer != nil && killer.pl != nil && killer != dead:
		cause = "player"
	}
	z.track(dead, "death", cause, 0, nil)
	before := dead.pl.xp
	switch {
	case chaotic:
		for range pkXPLossFactor {
			dead.pl.xp = game.XPAfterDeath(dead.pl.xp, dead.level)
		}
	case !byPlayer:
		dead.pl.xp = game.XPAfterDeath(dead.pl.xp, dead.level)
	}
	dead.pl.statsDirty = true
	z.forgetTarget(dead.id)
	if chaotic {
		var owner uint32
		if pk != nil {
			owner = pk.id
		}
		z.dropPKItems(dead, owner)
	}
	z.pvpChanged(dead)

	msg := "Você morreu"
	if byPlayer {
		msg += " para " + pk.name
	}
	if lost := before - dead.pl.xp; lost > 0 {
		msg += fmt.Sprintf(" e perdeu %d de XP", lost)
	}
	z.system(dead, msg+". Clique em \"Voltar à cidade\".")
}

// grantXP soma XP e sobe de nível quantas vezes for preciso.
func (z *Zone) grantXP(p *entity, xp int64) {
	if p.level >= game.MaxLevel {
		return
	}
	p.pl.xp += xp
	p.pl.statsDirty = true
	z.systemAfterFlush(p, fmt.Sprintf("+%d XP", xp)) // depois do golpe que matou

	leveled := false
	for p.level < game.MaxLevel && p.pl.xp >= game.XPLevelStart(p.level+1) {
		p.level++
		leveled = true
	}
	if !leveled {
		return
	}
	z.refreshStats(p)
	p.hp, p.mp = p.stats.MaxHP, p.stats.MaxMP
	z.sendProgress(p) // pontos novos de atributo e de skill
	z.clanLevelUp(p)
	z.frame.levelUps = append(z.frame.levelUps, &pb.LevelUp{Id: p.id, Level: uint32(p.level), MaxHp: uint32(p.maxHPInt())})
	z.systemAfterFlush(p, fmt.Sprintf("Você subiu para o nível %d!", p.level))
	z.track(p, "level", "", int64(p.level), nil)
	z.questLevel(p)
	z.log.Info("level up", "id", p.id, "name", p.name, "level", p.level)
}

// respawnPlayer leva o jogador morto de volta à cidade, com vida e mana cheias.
func (z *Zone) respawnPlayer(p *entity) {
	if !p.dead {
		return
	}
	a := z.rng.Float64() * 2 * math.Pi
	r := z.rng.Float64() * townSpawnRadius
	p.pos = z.freeSpot(Vec2{X: r * math.Cos(a), Z: r * math.Sin(a)})
	p.dest = p.pos
	p.path = nil
	p.dead = false
	p.pl.killedBy = ""
	p.hp, p.mp = p.stats.MaxHP, p.stats.MaxMP
	p.lastCombat = -math.MaxFloat64 / 2
	p.pl.statsDirty = true
	p.sentHP = p.hpInt()
	z.frame.revives = append(z.frame.revives, &pb.Revive{
		Id: p.id, X: float32(p.pos.X), Z: float32(p.pos.Z), Hp: uint32(p.hpInt()), MaxHp: uint32(p.maxHPInt()),
	})
}

func (z *Zone) selfStats(p *entity) *pb.SelfStats {
	next := uint64(0)
	if p.level < game.MaxLevel {
		next = uint64(game.XPLevelStart(p.level + 1))
	}
	return &pb.SelfStats{
		Hp: uint32(p.hpInt()), MaxHp: uint32(p.maxHPInt()),
		Mp: uint32(math.Floor(p.mp)), MaxMp: uint32(math.Round(p.stats.MaxMP)),
		Level: uint32(p.level), Xp: uint64(p.pl.xp),
		XpLevelStart: uint64(game.XPLevelStart(p.level)), XpNextLevel: next,
		PAtk:     uint32(math.Round(p.atk(false, z.now))),
		MAtk:     uint32(math.Round(p.atk(true, z.now))),
		PDef:     uint32(math.Round(p.def(false, z.now))),
		MDef:     uint32(math.Round(p.def(true, z.now))),
		Crit:     float32(p.stats.Crit),
		Speed:    float32(p.speed(z.now)),
		Dead:     p.dead,
		Accuracy: float32(p.accuracy(z.now)), Evasion: float32(p.stats.Evasion),
		AtkSpeed: float32(1 / p.attackInterval(z.now)), CastSpeed: float32(p.stats.CastSpeed),
		HpRegen: float32(p.stats.HPRegen), MpRegen: float32(p.stats.MPRegen), CritDamage: float32(p.stats.CritDamage),
		KilledBy: p.pl.killedBy,
	}
}

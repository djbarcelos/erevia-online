package zone

import (
	"fmt"
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const townSpawnRadius = 5.0

// kill trata a morte de qualquer entidade.
func (z *Zone) kill(dead, killer *entity) {
	dead.dead = true
	dead.hp = 0
	for _, ef := range dead.effects {
		z.frame.ends = append(z.frame.ends, &pb.EffectEnd{TargetId: dead.id, Effect: pb.Effect(ef.kind)})
	}
	dead.effects = nil
	dead.clearAction()
	dead.stopMoving()
	z.frame.deaths = append(z.frame.deaths, dead.id)

	if dead.mon != nil {
		ms := dead.mon
		ms.hate = map[uint32]float64{}
		ms.corpseEnd = z.now + corpseSeconds
		ms.respawnAt = z.now + ms.spawn.Respawn*(1+z.rng.Float64())
		if killer != nil && killer.pl != nil {
			xp := game.MonsterXP(ms.def, dead.level, killer.level)
			if z.night {
				xp = int64(math.Round(float64(xp) * game.NightXP))
			}
			z.grantXP(killer, xp)
		}
		return
	}

	// Jogador: perde XP (sem cair de nível) e os monstros esquecem dele.
	before := dead.pl.xp
	dead.pl.xp = game.XPAfterDeath(dead.pl.xp, dead.level)
	dead.pl.statsDirty = true
	for _, m := range z.monsters {
		delete(m.mon.hate, dead.id)
		if m.act.target == dead.id {
			m.clearAction()
		}
	}
	msg := "Você morreu."
	if lost := before - dead.pl.xp; lost > 0 {
		msg = fmt.Sprintf("Você morreu e perdeu %d de XP.", lost)
	}
	z.system(dead, msg+" Clique em \"Voltar à cidade\".")
}

// grantXP soma XP e sobe de nível quantas vezes for preciso.
func (z *Zone) grantXP(p *entity, xp int64) {
	if p.level >= game.MaxLevel {
		return
	}
	p.pl.xp += xp
	p.pl.statsDirty = true
	z.system(p, fmt.Sprintf("+%d XP", xp))

	leveled := false
	for p.level < game.MaxLevel && p.pl.xp >= game.XPLevelStart(p.level+1) {
		p.level++
		leveled = true
	}
	if !leveled {
		return
	}
	p.stats = game.ClassStats(p.pl.class, p.level)
	p.hp, p.mp = p.stats.MaxHP, p.stats.MaxMP
	z.frame.levelUps = append(z.frame.levelUps, &pb.LevelUp{Id: p.id, Level: uint32(p.level), MaxHp: uint32(p.maxHPInt())})
	z.system(p, fmt.Sprintf("Você subiu para o nível %d!", p.level))
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
		PAtk:  uint32(math.Round(p.atk(false, z.now))),
		MAtk:  uint32(math.Round(p.atk(true, z.now))),
		PDef:  uint32(math.Round(p.def(false, z.now))),
		Speed: float32(p.speed(z.now)),
		Dead:  p.dead,
	}
}

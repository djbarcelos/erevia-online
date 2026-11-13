package zone

import (
	"fmt"
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// PvP: atacar um jogador deixa o atacante "em combate" (nome roxo) por flagSeconds.
// Matar quem não estava em combate nem tinha karma dá karma (nome vermelho); quem tem
// karma arrisca deixar itens caírem ao morrer. A cidade protege, menos quem tem karma.
const (
	flagSeconds    = 30.0
	pvpMinLevel    = 6    // abaixo disso, ninguém ataca nem é atacado por jogadores
	karmaBase      = 120  // karma por PK…
	karmaPerPK     = 60   // …mais isto por PK anterior
	maxKarma       = 5000 //
	karmaPerMobLvl = 3    // cada monstro abatido tira nível × 3 de karma
	pkDropBase     = 0.10 // chance de deixar item cair: 10% + karma/40 %…
	pkDropPerKarma = 1.0 / 4000
	pkDropMax      = 0.60 // …até 60%
	pkXPLossFactor = 2    // quem tem karma perde o dobro de XP
	pkDropReserve  = 15.0 // o item do PK fica reservado para quem matou
	pkDropMoreAt   = 5    // +1 item com 5 PKs…
	pkDropMostAt   = 10   // …e +1 com 10
)

// pvpState: normal, em combate (roxo) ou com karma (vermelho).
func (z *Zone) pvpState(e *entity) pb.PvpState {
	switch {
	case e.pl == nil:
		return pb.PvpState_PVP_STATE_NORMAL
	case e.pl.karma > 0:
		return pb.PvpState_PVP_STATE_CHAOTIC
	case z.now < e.pl.flagUntil:
		return pb.PvpState_PVP_STATE_FLAGGED
	}
	return pb.PvpState_PVP_STATE_NORMAL
}

// allies: mesmo grupo ou mesmo clã (sem fogo amigo, recebem os buffs de aliado).
func (z *Zone) allies(a, b *entity) bool {
	if a == b {
		return true
	}
	if a.pl == nil || b.pl == nil {
		return false
	}
	if a.pl.party != nil && a.pl.party == b.pl.party {
		return true
	}
	return a.pl.clan != nil && a.pl.clan == b.pl.clan
}

// canHit diz se a pode atacar b agora (force = Ctrl + clique num jogador "branco").
// why explica a recusa ("" = pode). waitSafe: a recusa é só porque alguém está na cidade
// (quem persegue espera em vez de desistir).
func (z *Zone) canHit(a, b *entity, force bool) (ok bool, why string, waitSafe bool) {
	switch {
	case b == nil || b.dead || a == b:
		return false, "", false
	case b.mon != nil:
		return !b.mon.hidden, "", false
	case b.pl == nil || a.pl == nil:
		return false, "", false
	case z.allies(a, b):
		return false, "Você não pode atacar um aliado.", false
	case a.level < pvpMinLevel:
		return false, fmt.Sprintf("Você só pode lutar contra jogadores a partir do nível %d.", pvpMinLevel), false
	case b.level < pvpMinLevel:
		return false, fmt.Sprintf("Jogadores abaixo do nível %d estão protegidos.", pvpMinLevel), false
	case b.invisible(z.now):
		return false, "", false
	}
	if z.inSafe(a.pos) || z.inSafe(b.pos) {
		// Na cidade só se luta contra quem tem karma, e quem tem karma não ataca.
		if b.pl.karma > 0 && a.pl.karma == 0 {
			return true, "", false
		}
		return false, "Não é possível lutar dentro da cidade.", true
	}
	if force || z.pvpState(b) != pb.PvpState_PVP_STATE_NORMAL {
		return true, "", false
	}
	return false, "Segure Ctrl (ou use o botão Atacar) para atacar este jogador.", false
}

// hostile: canHit sem explicação (alvos de área nunca são forçados).
func (z *Zone) hostile(a, b *entity, force bool) bool {
	ok, _, _ := z.canHit(a, b, force)
	return ok
}

// onPvPHit: um jogador acertou (ou mirou um efeito em) outro. Quem ataca fica em combate,
// a não ser que o alvo tenha karma (caçar PK é livre).
func (z *Zone) onPvPHit(src, dst *entity) {
	if src.pl == nil || dst.pl == nil || src == dst || dst.pl.karma > 0 {
		return
	}
	was := z.pvpState(src)
	src.pl.flagUntil = z.now + flagSeconds
	if was != z.pvpState(src) {
		z.pvpChanged(src)
	}
}

// pvpChanged avisa todos da nova cor do nome e o dono dos números.
func (z *Zone) pvpChanged(p *entity) {
	st := z.pvpState(p)
	if st != p.pl.sentPvp {
		p.pl.sentPvp = st
		z.broadcastStatus(p)
	}
	z.sendSelfPvp(p)
}

func (z *Zone) broadcastStatus(p *entity) {
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_PlayerStatus{PlayerStatus: z.statusPB(p)}}, 0)
}

func (z *Zone) statusPB(p *entity) *pb.PlayerStatus {
	return &pb.PlayerStatus{Id: p.id, PvpState: z.pvpState(p), ClanName: z.clanName(p)}
}

func (z *Zone) selfPvpPB(p *entity) *pb.SelfPvp {
	out := &pb.SelfPvp{Karma: uint32(p.pl.karma), PkCount: uint32(p.pl.pks), PvpKills: uint32(p.pl.pvpKills)}
	if left := p.pl.flagUntil - z.now; left > 0 {
		out.FlagMs = uint32(left * 1000)
	}
	return out
}

func (z *Zone) sendSelfPvp(p *entity) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_SelfPvp{SelfPvp: z.selfPvpPB(p)}})
}

// expireFlags: a flag vence sozinha (conferido 1×/s).
func (z *Zone) expireFlags() {
	for _, p := range z.players {
		if p.pl.sentPvp != z.pvpState(p) {
			z.pvpChanged(p)
		}
	}
}

// pvpDeath trata a morte de um jogador para outro: karma, PK, vitórias e mensagens.
// Devolve se a morte foi para um jogador (sem perda de XP para quem não tem karma).
func (z *Zone) pvpDeath(dead, killer *entity) bool {
	if killer == nil || killer.pl == nil || killer == dead {
		return false
	}
	switch z.pvpStateAtDeath(dead) {
	case pb.PvpState_PVP_STATE_NORMAL:
		killer.pl.karma = min(maxKarma, killer.pl.karma+karmaBase+karmaPerPK*killer.pl.pks)
		killer.pl.pks++
		z.system(killer, fmt.Sprintf("Você matou %s, que não estava lutando. Seu karma subiu para %d.", dead.name, killer.pl.karma))
	case pb.PvpState_PVP_STATE_FLAGGED:
		killer.pl.pvpKills++
		z.system(killer, fmt.Sprintf("Você venceu %s em combate.", dead.name))
	default:
		z.system(killer, fmt.Sprintf("Você derrotou %s, que tinha karma.", dead.name))
	}
	killer.pl.statsDirty = true
	z.pvpChanged(killer)
	z.saveSoon(killer)
	return true
}

// pvpStateAtDeath: a flag de quem morre ainda vale no instante da morte.
func (z *Zone) pvpStateAtDeath(dead *entity) pb.PvpState { return z.pvpState(dead) }

// burnKarma: caçar monstros limpa o karma aos poucos.
func (z *Zone) burnKarma(p *entity, monsterLevel int) {
	if p.pl == nil || p.pl.karma <= 0 {
		return
	}
	p.pl.karma = max(0, p.pl.karma-monsterLevel*karmaPerMobLvl)
	if p.pl.karma == 0 {
		z.system(p, "Seu karma acabou. Seu nome voltou ao normal.")
	}
	z.pvpChanged(p)
}

// pkDropChance: chance de quem tem karma deixar itens caírem ao morrer.
func pkDropChance(karma int) float64 {
	if karma <= 0 {
		return 0
	}
	return math.Min(pkDropMax, pkDropBase+float64(karma)*pkDropPerKarma)
}

// pkDropCount: quantos itens caem (mais com muitos PKs).
func pkDropCount(pks int) int {
	n := 1
	if pks >= pkDropMoreAt {
		n++
	}
	if pks >= pkDropMostAt {
		n++
	}
	return n
}

// dropPKItems: quem morre com karma pode deixar itens da bolsa ou equipados no chão.
func (z *Zone) dropPKItems(dead *entity, killer uint32) {
	if z.rng.Float64() >= pkDropChance(dead.pl.karma) {
		return
	}
	type ref struct {
		equip bool
		slot  int
	}
	inv := &dead.pl.inv
	var pool []ref
	for i, s := range inv.equip {
		if !s.empty() {
			pool = append(pool, ref{true, i})
		}
	}
	for i, s := range inv.bag {
		if !s.empty() {
			pool = append(pool, ref{false, i})
		}
	}
	gear := false
	added := &pb.GroundItemsAdded{}
	for n := pkDropCount(dead.pl.pks); n > 0 && len(pool) > 0; n-- {
		i := z.rng.IntN(len(pool))
		r := pool[i]
		pool = append(pool[:i], pool[i+1:]...)
		var s stack
		if r.equip {
			s, inv.equip[r.slot] = inv.equip[r.slot], stack{}
			gear = true
		} else {
			s, inv.bag[r.slot] = inv.bag[r.slot], stack{}
		}
		if g := z.dropItem(dead.pos, s.def, s.count, s.enchant, killer, pkDropReserve); g != nil {
			added.Items = append(added.Items, z.groundPB(g))
		}
		z.system(dead, fmt.Sprintf("Você deixou cair %s.", itemLabelEnchant(s.def, s.count, s.enchant)))
		z.log.Info("pk drop", "id", dead.id, "item", s.def.ID, "count", s.count, "enchant", s.enchant)
	}
	if len(added.Items) > 0 {
		z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_GroundAdded{GroundAdded: added}}, 0)
	}
	z.inventoryChanged(dead, gear)
}

// forgetTarget: ninguém continua mirando quem ficou invisível ou saiu.
func (z *Zone) forgetTarget(id uint32) {
	for _, m := range z.monsters {
		delete(m.mon.hate, id)
		if m.act.target == id {
			m.clearAction()
		}
	}
	for _, p := range z.players {
		if p.id != id && p.act.target == id && p.act.kind != actPickup {
			z.cancelQueuedSkill(p)
			p.clearAction()
		}
	}
}

// clanName do jogador ("" = sem clã).
func (z *Zone) clanName(p *entity) string {
	if p.pl == nil || p.pl.clan == nil {
		return ""
	}
	return p.pl.clan.name
}

// enchantRulesPB: as regras do encantamento para o cliente mostrar chance e bônus.
func enchantRulesPB() *pb.EnchantRules {
	out := &pb.EnchantRules{Safe: game.EnchantSafe, Max: game.MaxEnchant}
	for _, c := range game.EnchantChances() {
		out.Chance = append(out.Chance, float32(c))
	}
	for g := game.Grade(0); g <= game.GradeLegendary; g++ {
		out.WeaponBonus = append(out.WeaponBonus, uint32(game.EnchantPerGrade(g)))
	}
	return out
}

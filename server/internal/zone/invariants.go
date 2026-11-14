package zone

import (
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"erevia/server/internal/game"
)

// Invariantes do estado do jogo: o próprio servidor confere, para cada jogador, o que
// nunca pode acontecer (vida acima do máximo, ouro negativo, pilha maior que o item
// permite, item equipado no encaixe errado, personagem dentro de parede, grupo ou clã
// apontando para quem não aponta de volta...). Roda 1×/s por jogador, espalhado entre
// os ticks (cada um no tick id % TickRate), e antes de cada salvamento.
//
// Em teste (testing.Testing) ou com DEBUG_INVARIANTS=1, invariante quebrada é panic com
// a regra, o personagem e o valor: os testes, o fuzz e o teste de caos acham o bug na
// hora. Em produção, corrige quando é seguro (limita a vida, zera ouro negativo, tira da
// parede...) e anota um evento "invariant" na telemetria (game_events), no máximo um por
// regra e personagem a cada 10 min.
//
// Encaixe repetido na bolsa, no equipamento ou no armazém não tem como existir na
// memória (são arrays indexados pelo encaixe; loadInventory e newStorage já descartam
// repetidos do banco). A lista que pode repetir encaixe é a oferta da troca, conferida
// na regra "trade".

const (
	// maxGold: teto de moedas (bolsa ou armazém). Bem acima do que dá para juntar
	// jogando; passar dele é sinal de duplicação.
	maxGold = 1_000_000_000
	// invariantReportEvery: no máximo um registro por regra e personagem nesse intervalo.
	invariantReportEvery = 10 * time.Minute
)

// invariantPanic é o panic do modo estrito. Os recovers da zona (errors.go) deixam passar,
// para a quebra derrubar o teste ou o servidor de verdade.
type invariantPanic string

// strictInvariants: invariante quebrada vira panic (testes e DEBUG_INVARIANTS=1).
var strictInvariants = testing.Testing() || os.Getenv("DEBUG_INVARIANTS") == "1"

type invariantKey struct {
	rule string
	id   uint32
}

// invariantGuard é o estado das invariantes na zona.
type invariantGuard struct {
	// mode: 0 = padrão (strictInvariants), 1 = sempre panic, -1 = sempre corrige e anota
	// (os testes usam -1 para conferir o comportamento de produção).
	mode     int8
	reported map[invariantKey]time.Time // último registro por regra e personagem
	broken   int                        // quebras desde a partida (para os testes e o log)
	// off: regras desligadas (testes que montam um estado de propósito, ex.: learn dá
	// ranks sem validar a árvore).
	off map[string]bool
}

func (g *invariantGuard) strict() bool {
	if g.mode == 0 {
		return strictInvariants
	}
	return g.mode > 0
}

// disable desliga uma regra nesta zona (só testes que montam um estado de propósito).
func (g *invariantGuard) disable(rule string) {
	if g.off == nil {
		g.off = map[string]bool{}
	}
	g.off[rule] = true
}

// checkInvariantsDue: chamada a cada tick; checa os jogadores cuja vez é este tick
// (cada um 1×/s) e, 1×/s, a tabela de grupos.
func (z *Zone) checkInvariantsDue() {
	rate := uint32(max(1, z.cfg.TickRate))
	turn := z.tick % rate
	for _, p := range z.players {
		if p.id%rate == turn {
			z.checkPlayer(p, false)
		}
	}
	if turn == 0 {
		z.checkParties()
	}
}

// checkInvariants confere todas as regras de um jogador (antes de salvar e nos testes).
func (z *Zone) checkInvariants(p *entity) { z.checkPlayer(p, true) }

// checkPlayer confere as regras. deep: também confere se cada item é o do catálogo (uma
// busca no mapa por pilha; fica para o salvamento, a checagem de 1×/s pula).
func (z *Zone) checkPlayer(p *entity, deep bool) {
	if p.pl == nil {
		return
	}
	z.checkVitals(p)
	z.checkGold(p)
	z.checkItems(p, deep)
	z.checkProgress(p)
	z.checkClasses(p)
	z.checkPosition(p)
	z.checkPvP(p)
	z.checkParty(p)
	z.checkClan(p)
	z.checkTrade(p)
}

// invariantBroken trata uma quebra: panic no modo estrito; senão anota e aplica fix
// (nil = não há correção segura, só anota).
func (z *Zone) invariantBroken(p *entity, rule string, bad any, fix func()) {
	if z.invariants.off[rule] {
		return
	}
	z.invariants.broken++
	var id uint32
	name, x, zz := "", 0.0, 0.0
	if p != nil {
		id, name, x, zz = p.id, p.name, p.pos.X, p.pos.Z
	}
	if z.invariants.strict() {
		panic(invariantPanic(fmt.Sprintf("invariante %q quebrada: personagem %d (%s), valor %v, posição (%.1f, %.1f)", rule, id, name, bad, x, zz)))
	}
	if z.invariantReportDue(rule, id) {
		// Anota antes de corrigir: a posição e o valor são os do problema.
		z.track(p, "invariant", rule, 0, map[string]any{
			"value": fmt.Sprint(bad), "fixed": fix != nil, "version": z.cfg.Version, "name": name,
		})
		z.log.Warn("invariant broken", "rule", rule, "id", id, "name", name, "value", fmt.Sprint(bad),
			"x", x, "z", zz, "fixed", fix != nil)
	}
	if fix != nil {
		fix()
	}
}

// invariantReportDue aplica o limite de um registro por regra e personagem a cada 10 min.
func (z *Zone) invariantReportDue(rule string, id uint32) bool {
	g := &z.invariants
	now := z.cfg.Clock()
	if g.reported == nil {
		g.reported = map[invariantKey]time.Time{}
	}
	if len(g.reported) > 4096 { // esquece os vencidos para o mapa não crescer sem fim
		for k, at := range g.reported {
			if now.Sub(at) >= invariantReportEvery {
				delete(g.reported, k)
			}
		}
	}
	k := invariantKey{rule, id}
	if at, ok := g.reported[k]; ok && now.Sub(at) < invariantReportEvery {
		return false
	}
	g.reported[k] = now
	return true
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// checkVitals: vida e mana entre 0 e o máximo; vivo tem vida, morto tem 0.
func (z *Zone) checkVitals(p *entity) {
	top := p.stats.MaxHP
	switch {
	case !finite(top) || top <= 0 || !finite(p.stats.MaxMP) || p.stats.MaxMP < 0:
		z.invariantBroken(p, "max_hp", fmt.Sprintf("máximos %v / %v", top, p.stats.MaxMP), nil)
	case p.dead && p.hp != 0:
		z.invariantBroken(p, "hp", fmt.Sprintf("morto com %v", p.hp), func() { p.hp = 0 })
	case !p.dead && !(p.hp > 0 && p.hp <= top): // NaN também cai aqui
		z.invariantBroken(p, "hp", fmt.Sprintf("%v / %v", p.hp, top), func() {
			if p.hp > 0 || math.IsNaN(p.hp) {
				p.hp = top
			} else {
				p.hp = math.Min(1, top) // vivo sem vida: fica com 1 (matar não é seguro)
			}
		})
	}
	if finite(p.stats.MaxMP) && !(p.mp >= 0 && p.mp <= p.stats.MaxMP) {
		z.invariantBroken(p, "mp", fmt.Sprintf("%v / %v", p.mp, p.stats.MaxMP), func() {
			if math.IsNaN(p.mp) {
				p.mp = 0
			}
			p.mp = math.Min(math.Max(p.mp, 0), p.stats.MaxMP)
		})
	}
}

// checkGold: moedas da bolsa e do armazém entre 0 e o teto.
func (z *Zone) checkGold(p *entity) {
	inv := &p.pl.inv
	switch {
	case inv.gold < 0:
		z.invariantBroken(p, "gold", inv.gold, func() {
			inv.gold = 0
			z.inventoryChanged(p, false)
		})
	case inv.gold > maxGold: // pode ser duplicação, mas tirar moedas não é seguro
		z.invariantBroken(p, "gold", inv.gold, nil)
	}
	st := z.storageOf(p)
	if st == nil {
		return
	}
	switch {
	case st.account != p.pl.accountID:
		z.invariantBroken(p, "storage", fmt.Sprintf("armazém da conta %d no personagem da conta %d", st.account, p.pl.accountID), nil)
	case st.gold < 0:
		z.invariantBroken(p, "storage_gold", st.gold, func() {
			st.gold = 0
			st.dirty = true
			z.saveSoon(p)
		})
	case st.gold > maxGold:
		z.invariantBroken(p, "storage_gold", st.gold, nil)
	}
}

// stackProblem diz o que há de errado na pilha ("" = nada) e corrige s se fix.
// equipped: no equipamento a pilha é sempre de 1.
func stackProblem(s *stack, equipped, deep, fix bool) string {
	if s.def == nil {
		if s.count == 0 && s.enchant == 0 {
			return ""
		}
		bad := fmt.Sprintf("espaço vazio com %d +%d", s.count, s.enchant)
		if fix {
			*s = stack{}
		}
		return bad
	}
	limit := s.def.Stack
	if equipped {
		limit = 1
	}
	outside := s.def.Kind == game.KindGold || (deep && game.Items[s.def.ID] != s.def)
	badEnchant := s.enchant < 0 || s.enchant > game.MaxEnchant || (s.enchant > 0 && s.def.Stack > 1)
	if !outside && s.count > 0 && s.count <= limit && !badEnchant {
		return "" // o caminho comum: nada de formatar texto
	}
	bad := fmt.Sprintf("%s x%d +%d", s.def.ID, s.count, s.enchant)
	switch {
	case outside:
		return bad + " (fora do catálogo)" // sem correção segura
	case !fix:
	case s.count <= 0:
		*s = stack{}
	case s.count > limit:
		s.count = limit
	default:
		s.enchant = min(max(s.enchant, 0), game.MaxEnchant)
		if s.def.Stack > 1 {
			s.enchant = 0
		}
	}
	return bad
}

// checkItems: pilhas válidas na bolsa, no equipamento e no armazém, e cada item
// equipado no encaixe certo, da classe e do nível do personagem.
func (z *Zone) checkItems(p *entity, deep bool) {
	inv := &p.pl.inv
	invFixed := func(gear bool) func() { return func() { z.inventoryChanged(p, gear) } }
	for i := range inv.bag {
		if bad := stackProblem(&inv.bag[i], false, deep, false); bad != "" {
			z.invariantBroken(p, "stack", fmt.Sprintf("bolsa[%d] %s", i, bad), stackFix(&inv.bag[i], false, invFixed(false)))
		}
	}
	for i := range inv.equip {
		s := &inv.equip[i]
		if bad := stackProblem(s, true, deep, false); bad != "" {
			z.invariantBroken(p, "stack", fmt.Sprintf("equipado[%d] %s", i, bad), stackFix(s, true, invFixed(true)))
			continue
		}
		if s.empty() {
			continue
		}
		if why := canEquip(s.def, i, p.pl.class, p.level); why != "" {
			var fix func()
			if free := inv.firstFree(); free >= 0 { // volta para a bolsa (não some nada)
				fix = func() {
					inv.bag[free], inv.equip[i] = inv.equip[i], stack{}
					z.inventoryChanged(p, true)
				}
			}
			z.invariantBroken(p, "equip", fmt.Sprintf("equipado[%d] %s: %s", i, s.def.ID, why), fix)
		}
	}
	st := z.storageOf(p)
	if st == nil {
		return
	}
	for i := range st.slots {
		if bad := stackProblem(&st.slots[i], false, deep, false); bad != "" {
			z.invariantBroken(p, "stack", fmt.Sprintf("armazém[%d] %s", i, bad), stackFix(&st.slots[i], false, func() {
				st.dirty = true
				z.saveSoon(p)
			}))
		}
	}
}

// stackFix: a correção da pilha, seguida de then (nil se não há uma segura, ex.: item
// fora do catálogo).
func stackFix(s *stack, equipped bool, then func()) func() {
	if s.def != nil && (game.Items[s.def.ID] != s.def || s.def.Kind == game.KindGold) {
		return nil
	}
	return func() {
		stackProblem(s, equipped, true, true)
		then()
	}
}

// checkProgress: nível dentro da faixa e coerente com a XP; pontos gastos cabem no nível.
func (z *Zone) checkProgress(p *entity) {
	xp, lv := p.pl.xp, p.level
	switch {
	case lv < 1 || lv > game.MaxLevel || xp < 0:
		z.invariantBroken(p, "level", fmt.Sprintf("nível %d, XP %d", lv, xp), nil)
	case xp < game.XPLevelStart(lv):
		// XP abaixo do começo do nível: sobe até o começo (ninguém perde nada).
		z.invariantBroken(p, "level", fmt.Sprintf("nível %d com XP %d (começa em %d)", lv, xp, game.XPLevelStart(lv)), func() {
			p.pl.xp = game.XPLevelStart(lv)
			p.pl.statsDirty = true
		})
	case lv < game.MaxLevel && xp >= game.XPLevelStart(lv+1):
		z.invariantBroken(p, "level", fmt.Sprintf("nível %d com XP %d (o %d começa em %d)", lv, xp, lv+1, game.XPLevelStart(lv+1)), nil)
	}
	if lv < 1 || lv > game.MaxLevel {
		return
	}
	if a := p.pl.attrs; !a.Valid() || a.Sum() > game.AttrPointsTotal(lv) {
		z.invariantBroken(p, "points", fmt.Sprintf("atributos %+v (nível %d dá %d)", a, lv, game.AttrPointsTotal(lv)), nil)
	}
	if err := game.ValidateRanks(p.pl.class, p.pl.spec, lv, p.pl.ranks); err != nil {
		z.invariantBroken(p, "points", "skills: "+err.Error(), nil)
	}
}

// checkClasses: subclasse (subclass.go). A classe ativa é a dos campos do jogador (as
// regras de nível, XP, pontos, skills e equipamento acima já olham só ela, então trocar de
// classe não acusa nada); aqui, que ela é uma das classes do personagem, que a principal
// é a do corpo e que cada classe guardada tem o nível da própria XP.
func (z *Zone) checkClasses(p *entity) {
	set := &p.pl.classes
	switch {
	case len(set.List) == 0 || set.List[0].Class != p.pl.mainClass || set.Main != p.pl.mainClass:
		z.invariantBroken(p, "classes", fmt.Sprintf("principal %q, lista %+v", p.pl.mainClass, set.List), nil)
		return
	case set.Active != p.pl.class || !set.Has(p.pl.class):
		z.invariantBroken(p, "classes", fmt.Sprintf("ativa %q no jogador e %q na lista", p.pl.class, set.Active), nil)
		return
	case len(set.List) > 1+game.MaxSubclasses:
		z.invariantBroken(p, "classes", fmt.Sprintf("%d subclasses (máximo %d)", set.Subclasses(), game.MaxSubclasses), nil)
	}
	for i := range set.List {
		c := &set.List[i]
		if c.Class == p.pl.class {
			continue // a ativa está nos campos do jogador (checkProgress)
		}
		if lv := game.LevelForXP(c.XP); c.Level != lv || c.XP < 0 {
			z.invariantBroken(p, "classes", fmt.Sprintf("%s nível %d com XP %d", c.Class, c.Level, c.XP), func() {
				c.Level = lv
			})
		}
	}
}

// checkPosition: dentro do mapa e fora dos colisores.
func (z *Zone) checkPosition(p *entity) {
	pos := p.pos
	if finite(pos.X) && finite(pos.Z) && z.bounds().Contains(toPt(pos)) && z.walkable(pos) {
		return
	}
	z.invariantBroken(p, "position", fmt.Sprintf("(%v, %v)", pos.X, pos.Z), func() {
		if !finite(pos.X) || !finite(pos.Z) {
			pos = Vec2{} // perdido: volta para a praça
		}
		p.teleportTo(z.freeSpot(clampToZone(pos, z.bounds())))
	})
}

// checkPvP: karma e contadores de PvP nunca negativos.
func (z *Zone) checkPvP(p *entity) {
	pl := p.pl
	if pl.karma < 0 || pl.pks < 0 || pl.pvpKills < 0 {
		z.invariantBroken(p, "karma", fmt.Sprintf("karma %d, PKs %d, vitórias %d", pl.karma, pl.pks, pl.pvpKills), func() {
			pl.karma, pl.pks, pl.pvpKills = max(0, pl.karma), max(0, pl.pks), max(0, pl.pvpKills)
		})
	}
}

// checkParty: o grupo do jogador existe, tem ele e o líder, tem de 2 a 6 pessoas, e
// cada colega está online apontando para o mesmo grupo.
func (z *Zone) checkParty(p *entity) {
	pt := p.pl.party
	if pt == nil {
		return
	}
	bad := ""
	switch {
	case z.parties[pt.id] != pt:
		bad = "grupo fora da tabela"
	case !pt.has(p.id):
		bad = "não está na lista do próprio grupo"
	case !pt.has(pt.leader):
		bad = fmt.Sprintf("líder %d fora do grupo", pt.leader)
	case len(pt.members) < 2 || len(pt.members) > partyMax:
		bad = fmt.Sprintf("%d membros", len(pt.members))
	default:
		for _, id := range pt.members {
			if id == p.id {
				continue
			}
			if m := z.players[id]; m == nil || m.pl.party != pt {
				bad = fmt.Sprintf("colega %d não aponta para o grupo", id)
				break
			}
		}
	}
	if bad != "" {
		z.invariantBroken(p, "party", fmt.Sprintf("grupo %d: %s", pt.id, bad), nil)
	}
}

// checkParties: nenhum grupo na tabela sem ninguém apontando para ele (o outro sentido
// do checkParty).
func (z *Zone) checkParties() {
	for id, pt := range z.parties {
		var who *entity
		for _, mid := range pt.members {
			if m := z.players[mid]; m != nil && m.pl.party == pt {
				who = m
				break
			}
		}
		if who == nil || pt.id != id {
			z.invariantBroken(who, "party", fmt.Sprintf("grupo %d (chave %d) sem membros apontando para ele", pt.id, id), nil)
		}
	}
}

// checkClan: o clã do jogador é o do índice (clanOf), está na tabela e tem ele e o líder.
func (z *Zone) checkClan(p *entity) {
	c, idx := p.pl.clan, z.clanOf[int64(p.id)]
	bad := ""
	switch {
	case c != idx:
		bad = "clã do jogador diferente do índice"
	case c == nil:
		return
	case z.clans[c.id] != c:
		bad = "clã fora da tabela"
	case c.members[int64(p.id)] == nil:
		bad = "não está na lista do próprio clã"
	case c.members[c.leader] == nil:
		bad = fmt.Sprintf("líder %d fora do clã", c.leader)
	default:
		return
	}
	id := int64(0)
	if c != nil {
		id = c.id
	} else if idx != nil {
		id = idx.id
	}
	z.invariantBroken(p, "clan", fmt.Sprintf("clã %d: %s", id, bad), nil)
}

// checkTrade: a troca aberta aponta para os dois lados, o outro está online, e a oferta
// cabe no que o jogador tem (sem encaixe repetido). Corrigir = cancelar a troca, que
// não mexe em item nenhum.
func (z *Zone) checkTrade(p *entity) {
	t := p.pl.trade
	if t == nil {
		return
	}
	side := t.side(p)
	other := t.p[1-side]
	o := t.offer[side]
	bad := ""
	switch {
	case t.p[side] != p:
		bad = "o jogador não é um dos lados"
	case other == nil || other == p || z.players[other.id] != other || other.pl.trade != t:
		bad = "o outro lado não aponta para a troca"
	case o.gold < 0 || o.gold > p.pl.inv.gold:
		bad = fmt.Sprintf("oferece %d moedas e tem %d", o.gold, p.pl.inv.gold)
	case len(o.items) > tradeMaxItems:
		bad = fmt.Sprintf("%d itens na oferta", len(o.items))
	default:
		var seen [game.BagSize]bool
		for _, it := range o.items {
			switch {
			case it.slot < 0 || it.slot >= game.BagSize:
				bad = fmt.Sprintf("encaixe %d fora da bolsa", it.slot)
			case seen[it.slot]:
				bad = fmt.Sprintf("encaixe %d repetido na oferta", it.slot)
			case !offerStillThere(&p.pl.inv, it):
				bad = fmt.Sprintf("encaixe %d não tem mais %s x%d", it.slot, it.def.ID, it.count)
			default:
				seen[it.slot] = true
				continue
			}
			break
		}
	}
	if bad != "" {
		z.invariantBroken(p, "trade", bad, func() { z.closeTrade(t, "A troca foi cancelada.") })
	}
}

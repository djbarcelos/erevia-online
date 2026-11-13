package zone

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/world"
)

// Cada invariante: a quebra é detectada (panic no modo estrito, o dos testes) e, no
// modo de produção, corrigida quando há correção segura e anotada na telemetria.

func invZone(t *testing.T, w *world.World) (*Zone, *entity) {
	t.Helper()
	cfg := testConfig()
	cfg.Spawns = nil
	cfg.World = w
	cfg.Version = "abc123"
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	z.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Ana", Class: "knight", X: 30, Z: 0,
		Items: game.StarterKit("knight"), Gold: 500, AccountID: 7, Peer: &fakePeer{}}})
	p := z.players[1]
	z.checkInvariants(p) // o estado de partida é válido
	if z.invariants.broken != 0 {
		t.Fatalf("estado inicial quebrou %d invariantes", z.invariants.broken)
	}
	return z, p
}

// mustPanicRule roda f e exige o panic da regra.
func mustPanicRule(t *testing.T, rule string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		msg := fmt.Sprint(recover())
		if !strings.Contains(msg, fmt.Sprintf("invariante %q quebrada", rule)) {
			t.Fatalf("esperava panic da invariante %q, veio %q", rule, msg)
		}
	}()
	f()
}

// itemWhere: o primeiro item do catálogo que satisfaz ok (em ordem de id, estável).
func itemWhere(t *testing.T, ok func(*game.ItemDef) bool) *game.ItemDef {
	t.Helper()
	var best *game.ItemDef
	for _, d := range game.Items {
		if ok(d) && (best == nil || d.ID < best.ID) {
			best = d
		}
	}
	if best == nil {
		t.Fatal("nenhum item serve para o teste")
	}
	return best
}

type invCase struct {
	name string
	rule string
	w    *world.World // mapa (nil = campo aberto)
	brk  func(t *testing.T, z *Zone, p *entity)
	// fixed confere a correção; nil = não há correção segura (só anota).
	fixed func(t *testing.T, z *Zone, p *entity)
}

func invCases() []invCase {
	stackable := func(t *testing.T) *game.ItemDef {
		return itemWhere(t, func(d *game.ItemDef) bool { return d.Stack > 1 && d.Kind != game.KindGold })
	}
	return []invCase{
		{name: "vida acima do máximo", rule: "hp",
			brk:   func(t *testing.T, z *Zone, p *entity) { p.hp = p.stats.MaxHP + 1.5 },
			fixed: func(t *testing.T, z *Zone, p *entity) { wantF(t, "hp", p.hp, p.stats.MaxHP) }},
		{name: "vivo sem vida", rule: "hp",
			brk:   func(t *testing.T, z *Zone, p *entity) { p.hp = -3 },
			fixed: func(t *testing.T, z *Zone, p *entity) { wantF(t, "hp", p.hp, 1) }},
		{name: "vida NaN", rule: "hp",
			brk:   func(t *testing.T, z *Zone, p *entity) { p.hp = math.NaN() },
			fixed: func(t *testing.T, z *Zone, p *entity) { wantF(t, "hp", p.hp, p.stats.MaxHP) }},
		{name: "morto com vida", rule: "hp",
			brk:   func(t *testing.T, z *Zone, p *entity) { p.dead, p.hp = true, 12 },
			fixed: func(t *testing.T, z *Zone, p *entity) { wantF(t, "hp", p.hp, 0) }},
		{name: "vida máxima inválida", rule: "max_hp",
			brk: func(t *testing.T, z *Zone, p *entity) { p.stats.MaxHP = 0 }},
		{name: "mana negativa", rule: "mp",
			brk:   func(t *testing.T, z *Zone, p *entity) { p.mp = -2 },
			fixed: func(t *testing.T, z *Zone, p *entity) { wantF(t, "mp", p.mp, 0) }},
		{name: "mana acima do máximo", rule: "mp",
			brk:   func(t *testing.T, z *Zone, p *entity) { p.mp = p.stats.MaxMP * 2 },
			fixed: func(t *testing.T, z *Zone, p *entity) { wantF(t, "mp", p.mp, p.stats.MaxMP) }},
		{name: "ouro negativo", rule: "gold",
			brk:   func(t *testing.T, z *Zone, p *entity) { p.pl.inv.gold = -10 },
			fixed: func(t *testing.T, z *Zone, p *entity) { want(t, "ouro", p.pl.inv.gold, 0) }},
		{name: "ouro acima do teto", rule: "gold",
			brk: func(t *testing.T, z *Zone, p *entity) { p.pl.inv.gold = maxGold + 1 }},
		{name: "ouro do armazém negativo", rule: "storage_gold",
			brk: func(t *testing.T, z *Zone, p *entity) { z.storageOf(p).gold = -1 },
			fixed: func(t *testing.T, z *Zone, p *entity) {
				st := z.storageOf(p)
				if st.gold != 0 || !st.dirty {
					t.Fatalf("armazém: ouro %d, dirty %v", st.gold, st.dirty)
				}
			}},
		{name: "armazém de outra conta", rule: "storage",
			brk: func(t *testing.T, z *Zone, p *entity) { z.storageOf(p).account = 99 }},
		{name: "pilha maior que o máximo do item", rule: "stack",
			brk: func(t *testing.T, z *Zone, p *entity) {
				d := stackable(t)
				p.pl.inv.bag[30] = stack{def: d, count: d.Stack + 5}
			},
			fixed: func(t *testing.T, z *Zone, p *entity) { want(t, "pilha", p.pl.inv.bag[30].count, stackable(t).Stack) }},
		{name: "pilha com quantidade zero", rule: "stack",
			brk: func(t *testing.T, z *Zone, p *entity) { p.pl.inv.bag[30] = stack{def: stackable(t), count: 0} },
			fixed: func(t *testing.T, z *Zone, p *entity) {
				if p.pl.inv.bag[30] != (stack{}) {
					t.Fatalf("espaço = %+v, quer vazio", p.pl.inv.bag[30])
				}
			}},
		{name: "encantamento em item que empilha", rule: "stack",
			brk: func(t *testing.T, z *Zone, p *entity) {
				p.pl.inv.bag[30] = stack{def: stackable(t), count: 3, enchant: 2}
			},
			fixed: func(t *testing.T, z *Zone, p *entity) { want(t, "encantamento", p.pl.inv.bag[30].enchant, 0) }},
		{name: "encantamento acima do máximo", rule: "stack",
			brk: func(t *testing.T, z *Zone, p *entity) {
				w := p.pl.inv.equip[game.SlotWeapon]
				w.enchant = game.MaxEnchant + 3
				p.pl.inv.equip[game.SlotWeapon] = w
			},
			fixed: func(t *testing.T, z *Zone, p *entity) {
				want(t, "encantamento", p.pl.inv.equip[game.SlotWeapon].enchant, game.MaxEnchant)
			}},
		{name: "equipado com quantidade 2", rule: "stack",
			brk:   func(t *testing.T, z *Zone, p *entity) { p.pl.inv.equip[game.SlotWeapon].count = 2 },
			fixed: func(t *testing.T, z *Zone, p *entity) { want(t, "equipado", p.pl.inv.equip[game.SlotWeapon].count, 1) }},
		{name: "item fora do catálogo", rule: "stack",
			brk: func(t *testing.T, z *Zone, p *entity) {
				p.pl.inv.bag[30] = stack{def: &game.ItemDef{ID: "falso", Stack: 1}, count: 1}
			}},
		{name: "pilha inválida no armazém", rule: "stack",
			brk: func(t *testing.T, z *Zone, p *entity) {
				d := stackable(t)
				z.storageOf(p).slots[4] = stack{def: d, count: d.Stack * 2}
			},
			fixed: func(t *testing.T, z *Zone, p *entity) {
				st := z.storageOf(p)
				if st.slots[4].count != stackable(t).Stack || !st.dirty {
					t.Fatalf("armazém[4] = %+v, dirty %v", st.slots[4], st.dirty)
				}
			}},
		{name: "item de outra classe equipado", rule: "equip",
			brk: func(t *testing.T, z *Zone, p *entity) {
				d := itemWhere(t, func(d *game.ItemDef) bool {
					return d.Fits(game.SlotWeapon) && !d.UsableBy("knight") && d.ReqLevel <= 1
				})
				p.pl.inv.bag[35] = stack{} // espaço garantido na bolsa
				p.pl.inv.equip[game.SlotWeapon] = stack{def: d, count: 1}
			},
			fixed: func(t *testing.T, z *Zone, p *entity) {
				if !p.pl.inv.equip[game.SlotWeapon].empty() || p.pl.inv.slotOfNonKnight() < 0 {
					t.Fatalf("o item da outra classe devia voltar para a bolsa: %+v", p.pl.inv.equip[game.SlotWeapon])
				}
			}},
		{name: "item no encaixe errado", rule: "equip",
			brk: func(t *testing.T, z *Zone, p *entity) {
				d := itemWhere(t, func(d *game.ItemDef) bool {
					return d.Equippable() && !d.Fits(game.SlotWeapon) && d.UsableBy("knight") && d.ReqLevel <= 1
				})
				p.pl.inv.equip[game.SlotWeapon] = stack{def: d, count: 1}
			},
			fixed: func(t *testing.T, z *Zone, p *entity) {
				if !p.pl.inv.equip[game.SlotWeapon].empty() {
					t.Fatalf("o item no encaixe errado devia voltar para a bolsa")
				}
			}},
		{name: "item acima do nível", rule: "equip",
			brk: func(t *testing.T, z *Zone, p *entity) {
				d := itemWhere(t, func(d *game.ItemDef) bool {
					return d.Fits(game.SlotWeapon) && d.UsableBy("knight") && d.ReqLevel > 1
				})
				p.pl.inv.equip[game.SlotWeapon] = stack{def: d, count: 1}
			},
			fixed: func(t *testing.T, z *Zone, p *entity) {
				if !p.pl.inv.equip[game.SlotWeapon].empty() {
					t.Fatalf("o item acima do nível devia voltar para a bolsa")
				}
			}},
		{name: "XP abaixo do começo do nível", rule: "level",
			brk:   func(t *testing.T, z *Zone, p *entity) { p.level = 5 },
			fixed: func(t *testing.T, z *Zone, p *entity) { want(t, "XP", p.pl.xp, game.XPLevelStart(5)) }},
		{name: "XP de um nível acima", rule: "level",
			brk: func(t *testing.T, z *Zone, p *entity) { p.pl.xp = game.XPLevelStart(3) }},
		{name: "nível fora da faixa", rule: "level",
			brk: func(t *testing.T, z *Zone, p *entity) { p.level = game.MaxLevel + 1 }},
		{name: "XP negativa", rule: "level",
			brk: func(t *testing.T, z *Zone, p *entity) { p.pl.xp = -1 }},
		{name: "atributos acima do nível", rule: "points",
			brk: func(t *testing.T, z *Zone, p *entity) { p.pl.attrs.Str = 50 }},
		{name: "skill sem nível", rule: "points",
			brk: func(t *testing.T, z *Zone, p *entity) {
				for id, s := range game.Skills {
					if s.Class == "knight" && s.LevelReq() > 1 {
						p.pl.ranks[id] = 1
						return
					}
				}
				t.Fatal("nenhuma skill de cavaleiro com nível mínimo")
			}},
		{name: "fora do mapa", rule: "position",
			brk: func(t *testing.T, z *Zone, p *entity) { p.pos = Vec2{X: 1e6, Z: -1e6} },
			fixed: func(t *testing.T, z *Zone, p *entity) {
				if !z.bounds().Contains(toPt(p.pos)) || !p.teleport {
					t.Fatalf("posição %+v (teleporte %v)", p.pos, p.teleport)
				}
			}},
		{name: "posição NaN", rule: "position",
			brk: func(t *testing.T, z *Zone, p *entity) { p.pos = Vec2{X: math.NaN(), Z: 3} },
			fixed: func(t *testing.T, z *Zone, p *entity) {
				if p.pos != (Vec2{}) {
					t.Fatalf("posição %+v, quer a praça", p.pos)
				}
			}},
		{name: "dentro de um colisor", rule: "position", w: world.New(100, []world.Collider{{X: 30, Z: 10, R: 3}}),
			brk: func(t *testing.T, z *Zone, p *entity) { p.pos = Vec2{X: 30, Z: 10} },
			fixed: func(t *testing.T, z *Zone, p *entity) {
				if !z.walkable(p.pos) {
					t.Fatalf("continua dentro da parede: %+v", p.pos)
				}
			}},
		{name: "karma negativo", rule: "karma",
			brk:   func(t *testing.T, z *Zone, p *entity) { p.pl.karma, p.pl.pks = -5, -1 },
			fixed: func(t *testing.T, z *Zone, p *entity) { want(t, "karma", p.pl.karma+p.pl.pks, 0) }},
		{name: "fora da lista do próprio grupo", rule: "party",
			brk: func(t *testing.T, z *Zone, p *entity) {
				b, _ := joinAt(z, 2, "mage", 31, 0, 0)
				c, _ := joinAt(z, 3, "rogue", 32, 0, 0)
				z.partyJoin(p, b)
				z.partyJoin(p, c)
				pt := p.pl.party
				pt.members = []uint32{2, 3}
				pt.leader = 2
			}},
		{name: "colega aponta para outro grupo", rule: "party",
			brk: func(t *testing.T, z *Zone, p *entity) {
				b, _ := joinAt(z, 2, "mage", 31, 0, 0)
				z.partyJoin(p, b)
				b.pl.party = &party{id: 50, leader: 2, members: []uint32{2}}
			}},
		{name: "clã diferente do índice", rule: "clan",
			brk: func(t *testing.T, z *Zone, p *entity) {
				p.pl.clan = &clan{id: 9, name: "Fantasma", leader: 1, members: map[int64]*clanMember{1: {id: 1}}}
			}},
		{name: "fora da lista do próprio clã", rule: "clan",
			brk: func(t *testing.T, z *Zone, p *entity) {
				c := &clan{id: 9, name: "Ossos", leader: 2, members: map[int64]*clanMember{2: {id: 2}}}
				z.clans[c.id] = c
				z.clanOf[1] = c
				p.pl.clan = c
			}},
		{name: "troca que o outro lado não conhece", rule: "trade",
			brk: func(t *testing.T, z *Zone, p *entity) {
				b, _ := joinAt(z, 2, "mage", 31, 0, 0)
				p.pl.trade = &trade{p: [2]*entity{p, b}}
			},
			fixed: func(t *testing.T, z *Zone, p *entity) {
				if p.pl.trade != nil {
					t.Fatal("a troca quebrada devia ser cancelada")
				}
			}},
		{name: "encaixe repetido na oferta", rule: "trade",
			brk: func(t *testing.T, z *Zone, p *entity) {
				b, _ := joinAt(z, 2, "mage", 31, 0, 0)
				d := stackable(t)
				p.pl.inv.bag[30] = stack{def: d, count: 4}
				tr := &trade{p: [2]*entity{p, b}}
				tr.offer[0].items = []tradeItem{{slot: 30, def: d, count: 2}, {slot: 30, def: d, count: 2}}
				p.pl.trade, b.pl.trade = tr, tr
			},
			fixed: func(t *testing.T, z *Zone, p *entity) {
				if p.pl.trade != nil || z.players[2].pl.trade != nil {
					t.Fatal("a troca quebrada devia ser cancelada nos dois lados")
				}
				want(t, "pilha", p.pl.inv.bag[30].count, 4) // cancelar não mexe em item
			}},
		{name: "oferta com mais ouro do que tem", rule: "trade",
			brk: func(t *testing.T, z *Zone, p *entity) {
				b, _ := joinAt(z, 2, "mage", 31, 0, 0)
				tr := &trade{p: [2]*entity{p, b}}
				tr.offer[0].gold = p.pl.inv.gold + 1
				p.pl.trade, b.pl.trade = tr, tr
			},
			fixed: func(t *testing.T, z *Zone, p *entity) {
				if p.pl.trade != nil {
					t.Fatal("a troca quebrada devia ser cancelada")
				}
			}},
	}
}

// slotOfNonKnight: o espaço da bolsa com um item que o cavaleiro não usa (-1 = nenhum).
func (inv *inventory) slotOfNonKnight() int {
	for i, s := range inv.bag {
		if !s.empty() && !s.def.UsableBy("knight") {
			return i
		}
	}
	return -1
}

func want[T comparable](t *testing.T, what string, got, wantV T) {
	t.Helper()
	if got != wantV {
		t.Fatalf("%s = %v, quer %v", what, got, wantV)
	}
}

func wantF(t *testing.T, what string, got, wantV float64) {
	t.Helper()
	if math.Abs(got-wantV) > 1e-9 || math.IsNaN(got) {
		t.Fatalf("%s = %v, quer %v", what, got, wantV)
	}
}

func TestInvariantsStrictModePanics(t *testing.T) {
	for _, c := range invCases() {
		t.Run(c.name, func(t *testing.T) {
			z, p := invZone(t, c.w)
			c.brk(t, z, p)
			mustPanicRule(t, c.rule, func() { z.checkInvariants(p) })
		})
	}
}

func TestInvariantsProductionFixesAndReports(t *testing.T) {
	for _, c := range invCases() {
		t.Run(c.name, func(t *testing.T) {
			z, p := invZone(t, c.w)
			z.SetEventSink(nopSink{})
			drainEvents(z)
			z.invariants.mode = -1
			c.brk(t, z, p)
			z.checkInvariants(p)
			if z.invariants.broken == 0 {
				t.Fatal("a quebra não foi detectada")
			}
			e := findEvent(drainEvents(z), "invariant", c.rule)
			if e == nil {
				t.Fatalf("sem evento invariant/%s", c.rule)
			}
			if e.CharacterID != int64(p.id) || e.Extra["version"] != "abc123" || e.Extra["value"] == "" || e.Extra["fixed"] != (c.fixed != nil) {
				t.Fatalf("evento = %+v", e)
			}
			if c.fixed == nil {
				return
			}
			c.fixed(t, z, p)
			// Corrigido: a próxima checagem não acha mais nada (nem no modo estrito).
			z.invariants.mode = 1
			z.checkInvariants(p)
		})
	}
}

// O panic do modo estrito traz a regra, o personagem e o valor ruim.
func TestInvariantPanicMessage(t *testing.T) {
	z, p := invZone(t, nil)
	p.hp = p.stats.MaxHP + 1
	defer func() {
		msg := fmt.Sprint(recover())
		for _, part := range []string{`"hp"`, "personagem 1 (Ana)", fmt.Sprint(p.hp), "posição (30.0, 0.0)"} {
			if !strings.Contains(msg, part) {
				t.Fatalf("mensagem %q sem %q", msg, part)
			}
		}
	}()
	z.checkInvariants(p)
}

// Os testes rodam no modo estrito sem configurar nada (testing.Testing).
func TestInvariantsStrictByDefaultInTests(t *testing.T) {
	if !strictInvariants {
		t.Fatal("nos testes as invariantes devem ser estritas")
	}
	z, p := invZone(t, nil)
	p.pl.inv.gold = -1
	mustPanicRule(t, "gold", func() { z.checkInvariants(p) })
}

// O tick checa cada jogador 1×/s, e o salvamento checa antes de gravar.
func TestInvariantsRunEverySecondAndBeforeSave(t *testing.T) {
	z, p := invZone(t, nil)
	p.pl.karma = -2 // (vida acima do máximo a regeneração do tick já limita)
	mustPanicRule(t, "karma", func() { z.tickFor(1) })

	z, p = invZone(t, nil)
	p.pl.inv.gold = -5
	mustPanicRule(t, "gold", func() { z.savedState(p) })

	// Em produção o salvamento já sai corrigido.
	z, p = invZone(t, nil)
	z.invariants.mode = -1
	p.pl.inv.gold = -5
	if s := z.savedState(p); s.Gold != 0 {
		t.Fatalf("salvou ouro %d", s.Gold)
	}
}

// Espalhado: em cada tick só os jogadores da vez são checados, e todos 1×/s.
func TestInvariantsSpreadAcrossTicks(t *testing.T) {
	z, _ := invZone(t, nil)
	for id := uint32(2); id <= 40; id++ {
		joinAt(z, id, "mage", 30, float64(id%5), 0)
	}
	z.invariants.mode = -1
	for _, p := range z.players {
		p.pl.karma = -1 // todos quebrados: cada checagem conta uma quebra
	}
	for range z.cfg.TickRate {
		before := z.invariants.broken
		z.update(1 / float64(z.cfg.TickRate))
		n := z.invariants.broken - before
		if n > (len(z.players)+z.cfg.TickRate-1)/z.cfg.TickRate {
			t.Fatalf("%d checagens num tick só (%d jogadores)", n, len(z.players))
		}
	}
	for _, p := range z.players {
		if p.pl.karma != 0 {
			t.Fatalf("jogador %d não foi checado em 1 s", p.id)
		}
	}
}

// No máximo um registro por regra e personagem a cada 10 min (a correção vale sempre).
func TestInvariantReportRateLimit(t *testing.T) {
	now := noon
	cfg := testConfig()
	cfg.Spawns = nil
	cfg.Clock = func() time.Time { return now }
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	z.SetEventSink(nopSink{})
	p, _ := joinAt(z, 1, "knight", 30, 0, 0)
	z.invariants.mode = -1
	count := func() int {
		n := 0
		for _, e := range drainEvents(z) {
			if e.Kind == "invariant" {
				n++
			}
		}
		return n
	}
	drainEvents(z)
	for i := range 3 {
		p.hp = p.stats.MaxHP + 1
		z.checkInvariants(p)
		wantF(t, "vida corrigida", p.hp, p.stats.MaxHP)
		if i == 0 {
			p.pl.karma = -1 // outra regra no mesmo personagem conta à parte
			z.checkInvariants(p)
		}
	}
	want(t, "registros em 10 min", count(), 2)
	now = now.Add(invariantReportEvery)
	p.hp = p.stats.MaxHP + 1
	z.checkInvariants(p)
	want(t, "registros depois de 10 min", count(), 1)
}

// Custo de checar um jogador (bolsa, equipamento e armazém cheios de itens).
func BenchmarkCheckInvariants(b *testing.B) {
	cfg := testConfig()
	cfg.Spawns = nil
	if w, err := world.Load(); err == nil {
		cfg.World = w
	}
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	z.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Ana", Class: "knight", X: 5, Z: 5,
		Items: game.StarterKit("knight"), Gold: 500, AccountID: 7, Peer: &fakePeer{}}})
	p := z.players[1]
	var pot *game.ItemDef
	for _, d := range game.Items {
		if d.Stack > 1 && d.Kind != game.KindGold {
			pot = d
			break
		}
	}
	for i := range p.pl.inv.bag {
		if p.pl.inv.bag[i].empty() {
			p.pl.inv.bag[i] = stack{def: pot, count: 1}
		}
	}
	st := z.storageOf(p)
	for i := range st.slots {
		st.slots[i] = stack{def: pot, count: 1}
	}
	for _, deep := range []bool{false, true} {
		name := "tick"
		if deep {
			name = "salvamento"
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				z.checkPlayer(p, deep)
			}
		})
	}
	if z.invariants.broken != 0 {
		b.Fatal("o estado do benchmark quebrou invariantes")
	}
}

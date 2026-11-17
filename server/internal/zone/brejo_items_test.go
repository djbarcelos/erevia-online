package zone

import (
	"io"
	"log/slog"
	"testing"

	"erevia/server/internal/game"
)

// O Antídoto (tunnel#139): limpa veneno e peste na hora, gasta uma poção e tem recarga própria; sem veneno, não gasta.
func TestAntidoteCleansPoison(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "knight", 0, 0, 0)
	def := game.Items["potion_antidote"]
	if def == nil || !def.Cleanse || def.Kind != game.KindPotion || def.Cooldown != game.CooldownAntidote {
		t.Fatalf("antídoto: %+v", def)
	}
	p.pl.inv.add(def, 3)
	slot := p.pl.inv.slotOf("potion_antidote")

	z.useItem(p, placeBag, slot) // sem veneno: nada acontece
	if p.pl.inv.countOf("potion_antidote") != 3 || !peer.systemContains("não está envenenado") {
		t.Fatal("sem veneno o Antídoto não se gasta")
	}

	other, _ := joinAt(z, 2, "mage", 3, 0, 0)
	z.applyEffect(other, p, &game.EffectDef{Kind: game.EffectPoison, Duration: 20, Value: 5})
	z.applyEffect(other, p, &game.EffectDef{Kind: game.EffectPlague, Duration: 20, Value: 5})
	if !p.has(game.EffectPoison, z.now) || !p.has(game.EffectPlague, z.now) {
		t.Fatal("o veneno devia ter pegado")
	}
	z.useItem(p, placeBag, slot)
	if p.has(game.EffectPoison, z.now) || p.has(game.EffectPlague, z.now) || p.pl.inv.countOf("potion_antidote") != 2 {
		t.Fatalf("o Antídoto devia limpar o veneno e a peste e gastar um (sobraram %d)", p.pl.inv.countOf("potion_antidote"))
	}
	// A recarga própria: um segundo Antídoto logo em seguida espera.
	z.applyEffect(other, p, &game.EffectDef{Kind: game.EffectPoison, Duration: 20, Value: 5})
	z.useItem(p, placeBag, slot)
	if !p.has(game.EffectPoison, z.now) || p.pl.inv.countOf("potion_antidote") != 2 {
		t.Fatal("o Antídoto tem recarga")
	}
	z.tickFor(game.CooldownSeconds(game.CooldownAntidote) + 0.5)
	z.useItem(p, placeBag, slot)
	if p.has(game.EffectPoison, z.now) || p.pl.inv.countOf("potion_antidote") != 1 {
		t.Fatal("depois da recarga o Antídoto limpa de novo")
	}
}

// A receita do Ferreiro: 3 Antídotos por 2 Glândulas de Sapo e 1 Junco-de-Prata.
func TestAntidoteRecipe(t *testing.T) {
	r := game.Recipes["craft_potion_antidote"]
	if r == nil || r.Result != "potion_antidote" || r.Count != 3 {
		t.Fatalf("receita: %+v", r)
	}
	for _, m := range r.Materials {
		if d := game.Items[m.ID]; d == nil || d.Kind != game.KindMaterial {
			t.Errorf("material %s", m.ID)
		}
	}
}

// Tomé, o Barqueiro, fica no Posto e entrega o convite da linha do Brejo; a linha termina na Mãe-Lodo.
func TestFerrymanLine(t *testing.T) {
	z := New(testConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	n := npcNamed(z, game.TalkFerry)
	if n == nil || n.npc.talk == nil || dist(n.pos, Vec2{X: game.PostX, Z: game.PostZ}) > 12 {
		t.Fatalf("Tomé não está no Posto: %+v", n)
	}
	steps := 0
	for _, q := range game.Quests {
		if q.Region == game.BrejoRegion {
			steps++
			if q.Level != 0 && (q.Level < 14 || q.Level > 20) {
				t.Errorf("%s no nível %d", q.ID, q.Level)
			}
		}
	}
	if steps != 5 { // o convite e 4 missões
		t.Fatalf("a linha do Brejo tem %d missões, esperava o convite e 4", steps)
	}
}

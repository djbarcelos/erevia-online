package zone

import (
	"testing"

	"erevia/server/internal/game"
)

// twoHandedAxe: um machado de duas mãos de teste (nenhum item do catálogo usa o tipo ainda).
// O bárbaro passa a empunhar o tipo só durante o teste.
func twoHandedAxe(t *testing.T) *game.ItemDef {
	t.Helper()
	c := game.Classes["barbarian"]
	saved := c.Weapons
	t.Cleanup(func() { c.Weapons = saved })
	c.Weapons = append(append([]string{}, saved...), game.WeaponAxe2H)
	return &game.ItemDef{ID: "axe2h_test", Name: "Machado Grande", Kind: game.KindWeapon, Classes: []string{"barbarian"},
		WeaponType: game.WeaponAxe2H, ReqLevel: 1, Stack: 1}
}

func fillBag(inv *inventory, def *game.ItemDef) {
	for i := range inv.bag {
		if inv.bag[i].empty() {
			inv.bag[i] = stack{def: def, count: 1}
		}
	}
}

func TestTwoHandedWeaponSendsOffhandToBag(t *testing.T) {
	axe := twoHandedAxe(t)
	shield := item("shield_badge")

	// Com espaço: o escudo vai para a bolsa e a arma entra.
	var inv inventory
	inv.equip[game.SlotOffhand] = stack{def: shield, count: 1}
	inv.bag[3] = stack{def: axe, count: 1}
	if why := inv.equipTo(3, game.SlotWeapon, "barbarian", "", 1); why != "" {
		t.Fatal(why)
	}
	if inv.equip[game.SlotWeapon].def != axe || !inv.equip[game.SlotOffhand].empty() {
		t.Fatal("a arma de duas mãos deveria entrar e esvaziar a mão secundária")
	}
	found := false
	for _, s := range inv.bag {
		found = found || s.def == shield
	}
	if !found {
		t.Fatal("o escudo sumiu")
	}

	// Com a arma de duas mãos equipada, a mão secundária é recusada.
	inv.bag[5] = stack{def: shield, count: 1}
	if why := inv.equipTo(5, game.SlotOffhand, "barbarian", "", 1); why == "" || !inv.equip[game.SlotOffhand].empty() {
		t.Fatalf("escudo com arma de duas mãos deveria ser recusado (%q)", why)
	}

	// Bolsa cheia e arma antiga no lugar: sem espaço para o escudo, recusa e nada muda.
	var full inventory
	old := item("axe_common")
	full.equip[game.SlotWeapon] = stack{def: old, count: 1}
	full.equip[game.SlotOffhand] = stack{def: shield, count: 1}
	fillBag(&full, item("potion_hp_s"))
	full.bag[0] = stack{def: axe, count: 1}
	if why := full.equipTo(0, game.SlotWeapon, "barbarian", "", 1); why == "" {
		t.Fatal("bolsa cheia deveria recusar")
	}
	if full.equip[game.SlotWeapon].def != old || full.equip[game.SlotOffhand].def != shield || full.bag[0].def != axe {
		t.Fatal("a recusa não pode mexer em nada")
	}

	// Bolsa cheia, mas sem arma equipada: o espaço da arma que entra recebe o escudo.
	var tight inventory
	tight.equip[game.SlotOffhand] = stack{def: shield, count: 1}
	fillBag(&tight, item("potion_hp_s"))
	tight.bag[7] = stack{def: axe, count: 1}
	if why := tight.equipTo(7, game.SlotWeapon, "barbarian", "", 1); why != "" {
		t.Fatal(why)
	}
	if tight.bag[7].def != shield || tight.equip[game.SlotWeapon].def != axe {
		t.Fatal("o escudo deveria ocupar o espaço da arma que entrou")
	}
}

func TestDualWeaponTakesOffhand(t *testing.T) {
	if !item("daggers_common").TwoHanded() {
		t.Fatal("adagas são duplas")
	}
}

func TestWandEquipsOnMageAndNotOnKnight(t *testing.T) {
	wand := item("wand_common")
	if why := canEquip(wand, game.SlotWeapon, "mage", "", 1); why != "" {
		t.Fatal(why)
	}
	if why := canEquip(wand, game.SlotWeapon, "knight", "", 1); why == "" {
		t.Fatal("o cavaleiro não usa varinha")
	}
	if why := canEquip(item("shield_badge"), game.SlotOffhand, "mage", "", 1); why == "" {
		t.Fatal("o mago não usa escudo")
	}
}

func TestDropInvalidGear(t *testing.T) {
	axe := twoHandedAxe(t)
	shield := item("shield_badge")

	// Troca para o mago: arma, escudo e o resto que ele não usa vão para a bolsa.
	var inv inventory
	inv.equip[game.SlotWeapon] = stack{def: item("axe_common"), count: 1}
	inv.equip[game.SlotOffhand] = stack{def: shield, count: 1}
	if moved, stuck := inv.dropInvalidGear("mage", "", 1); moved != 2 || stuck != 0 {
		t.Fatalf("moved %d stuck %d", moved, stuck)
	}
	if !inv.equip[game.SlotWeapon].empty() || !inv.equip[game.SlotOffhand].empty() {
		t.Fatal("deveria desequipar tudo")
	}

	// Arma de duas mãos válida junto de escudo válido: só a mão secundária sai.
	var both inventory
	both.equip[game.SlotWeapon] = stack{def: axe, count: 1}
	both.equip[game.SlotOffhand] = stack{def: shield, count: 1}
	if got := both.invalidGear("barbarian", "", 1); len(got) != 1 || got[0] != game.SlotOffhand {
		t.Fatalf("invalidGear = %v", got)
	}

	// Sem espaço: o que não cabe fica equipado (nada se perde).
	var full inventory
	full.equip[game.SlotWeapon] = stack{def: item("axe_common"), count: 1}
	fillBag(&full, item("potion_hp_s"))
	if moved, stuck := full.dropInvalidGear("mage", "", 1); moved != 0 || stuck != 1 || full.equip[game.SlotWeapon].empty() {
		t.Fatalf("moved %d stuck %d", moved, stuck)
	}
}

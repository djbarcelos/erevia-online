package zone

import (
	"testing"

	"erevia/server/internal/game"
)

// Graus (tunnel#153): abaixo do nível mínimo do grau a peça não entra; do mínimo do grau até
// o nível da peça ela entra com a penalidade de nível, que some ao chegar no nível.
func TestEquipTierLevelAndPenalty(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "knight", 0, 0, 0)
	atLevel(z, p, 9)
	armor := game.Items["heavy_d_armor"] // grau D, nível 13
	p.pl.inv.add(armor, 1)
	slot := p.pl.inv.slotOf(armor.ID)
	z.useItem(p, placeBag, slot)
	if !p.pl.inv.equip[game.SlotArmor].empty() || !peer.systemContains("Grau D: precisa do nível 10") {
		t.Fatal("no 9 a peça do grau D não entra")
	}

	atLevel(z, p, 12)
	plain := p.stats
	z.useItem(p, placeBag, slot)
	if p.pl.inv.equip[game.SlotArmor].def != armor || !peer.systemContains("pede o nível 13") {
		t.Fatal("no 12 a peça do 13 entra, com aviso")
	}
	if p.stats.Accuracy != plain.Accuracy-100 || p.stats.AtkInterval <= plain.AtkInterval*1.6 || p.stats.PDef <= plain.PDef {
		t.Fatalf("penalidade: %+v, antes %+v", p.stats, plain)
	}
	atLevel(z, p, 13)
	if p.stats.Accuracy != plain.Accuracy+1 { // +1 de precisão por nível, sem penalidade
		t.Fatalf("no 13 a penalidade some: precisão %.0f, antes %.0f", p.stats.Accuracy, plain.Accuracy)
	}

	// Duas peças do conjunto: o bônus de 2 peças entra na conta.
	helm := game.Items["heavy_d_helm"]
	before := p.stats
	p.pl.inv.add(helm, 1)
	z.useItem(p, placeBag, p.pl.inv.slotOf(helm.ID))
	bonus := game.ItemSets["set_heavy_d"].Bonus(2)
	if want := before.PDef + helm.Bonus.PDef + bonus.PDef; p.stats.PDef < want-0.01 {
		t.Fatalf("defesa com 2 peças: %.1f, quero ao menos %.1f", p.stats.PDef, want)
	}
}

// O cliente recebe o grau, o nível mínimo, o peso, o conjunto e a penalidade em cada peça.
func TestItemInfoCarriesTierAndSet(t *testing.T) {
	info := itemInfoPB(game.Items["light_c_gloves"])
	if info.Tier != uint32(game.TierC) || info.TierLevel != 20 || info.ArmorWeight != uint32(game.WeightLight) ||
		info.GetSet().GetId() != "set_light_c" || info.GetSet().GetName() == "" || len(info.GetSet().GetPieces()) != 0 ||
		info.GetUnderLevel().GetAccuracy() != -100 {
		t.Fatalf("luvas leves do C: %+v", info)
	}
	// O conjunto inteiro vem na primeira peça.
	set := itemInfoPB(game.Items["light_c_armor"]).GetSet()
	if set.GetId() != "set_light_c" || len(set.GetPieces()) != 5 || len(set.GetBonuses()) != 2 || set.GetBonuses()[1].GetPieces() != 4 ||
		set.GetTheme() == "" {
		t.Fatalf("conjunto leve do C: %+v", set)
	}
	if itemInfoPB(game.Items["sword_c20"]).GetUnderLevel() != nil {
		t.Fatal("a peça do nível mínimo do grau não tem penalidade a mostrar")
	}
	if old := itemInfoPB(game.Items["sword_rare"]); old.Tier != uint32(game.TierD) || old.GetSet() != nil {
		t.Fatalf("a espada rara do 13 é do D, sem conjunto: %+v", old)
	}
	if potion := itemInfoPB(game.Items["potion_hp_s"]); potion.Tier != 0 || potion.GetUnderLevel() != nil {
		t.Fatalf("poção sem grau: %+v", potion)
	}
}

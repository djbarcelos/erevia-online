package zone

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

func (f *fakePeer) bestiaryMsgs() []*pb.Bestiary {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pb.Bestiary
	for _, m := range f.msgs {
		if b := m.GetBestiary(); b != nil {
			out = append(out, b)
		}
	}
	return out
}

func (f *fakePeer) bestiaryMarks() []*pb.BestiaryMark {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pb.BestiaryMark
	for _, m := range f.msgs {
		if b := m.GetBestiaryMark(); b != nil {
			out = append(out, b)
		}
	}
	return out
}

// lastBestiaryEntry: a última entrada do monstro id recebida (full ou parcial).
func (f *fakePeer) lastBestiaryEntry(id string) *pb.BestiaryEntry {
	msgs := f.bestiaryMsgs()
	for i := len(msgs) - 1; i >= 0; i-- {
		for _, e := range msgs[i].Entries {
			if e.Id == id {
				return e
			}
		}
	}
	return nil
}

// killAgain: o monstro m morre de novo para p pelo caminho normal do abate (kill), com o
// ódio de p, e volta à vida para o próximo.
func killAgain(z *Zone, p, m *entity) {
	m.dead, m.hp = false, m.stats.MaxHP
	m.mon.hate = map[uint32]float64{p.id: 10}
	z.kill(m, p)
	z.tickFor(0.1)
}

func TestBestiaryJoinSendsSilhouettes(t *testing.T) {
	z := newCombatZone()
	_, peer := joinAt(z, 1, "knight", 0, 0, 0)
	msgs := peer.bestiaryMsgs()
	if len(msgs) != 1 || !msgs[0].Full || len(msgs[0].Entries) != len(game.BestiaryCatalog()) || len(msgs[0].Families) != len(game.BestiaryFamilies) {
		t.Fatalf("o join manda o bestiário inteiro: %+v", msgs)
	}
	for _, e := range msgs[0].Entries {
		if e.Name != "" || e.Kills != 0 || e.Model == "" || e.LevelMax != 0 || len(e.Drops) != 0 {
			t.Fatalf("sem abate a entrada é só a silhueta: %+v", e)
		}
	}
	// A janela pede de novo ao abrir.
	z.handle(progressEvent{id: 1, msg: &pb.BestiaryCmd{Kind: pb.BestiaryCmdKind_BESTIARY_CMD_GET}})
	z.handle(progressEvent{id: 1, msg: &pb.BestiaryCmd{Kind: pb.BestiaryCmdKind(99)}}) // tipo que não existe: nada
	if n := len(peer.bestiaryMsgs()); n != 2 || !peer.bestiaryMsgs()[1].Full {
		t.Fatalf("BESTIARY_CMD_GET responde com o bestiário inteiro (%d mensagens)", n)
	}
}

func TestBestiaryCountsKillsAndOpensMarks(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 3, 25, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "barbarian", 23.5, 1, 0)

	killFixed(t, z, p, m) // o golpe de verdade
	k := p.pl.bst.kills["skeleton_minion"]
	if k.N != 1 || k.Lo != 3 || k.Hi != 3 {
		t.Fatalf("primeiro abate: %+v", k)
	}
	e := peer.lastBestiaryEntry("skeleton_minion")
	if e == nil || e.Name != "Esqueleto Lacaio" || e.Kills != 1 || e.Mark != 0 || e.LevelMax != 0 || e.HpMax != 0 {
		t.Fatalf("com 1 abate: o nome, sem as informações do marco 1: %+v", e)
	}
	if len(peer.bestiaryMarks()) != 0 {
		t.Fatal("nenhum marco ainda")
	}

	// 10º abate: nível e vida (do menor ao maior nível derrotado).
	p.pl.bst.kills["skeleton_minion"] = game.BestiaryKill{N: 9, Lo: 1, Hi: 3}
	killAgain(z, p, m)
	e = peer.lastBestiaryEntry("skeleton_minion")
	hp1, _, _ := game.BestiaryLevelStats(m.mon.def, 1)
	hp3, _, _ := game.BestiaryLevelStats(m.mon.def, 3)
	if e.Kills != 10 || e.Mark != 1 || e.LevelMin != 1 || e.LevelMax != 3 || e.HpMin != uint32(hp1) || e.HpMax != uint32(hp3) || len(e.Drops) != 0 {
		t.Fatalf("marco 1: %+v", e)
	}
	if marks := peer.bestiaryMarks(); len(marks) != 1 || marks[0].Mark != 1 || marks[0].Id != "skeleton_minion" {
		t.Fatalf("aviso do marco 1: %+v", marks)
	}
	if !peer.systemContains("Bestiário: Esqueleto Lacaio — 10 abate(s): nível e vida") {
		t.Fatal("o marco avisa no chat")
	}

	// 100º: o que ele deixa cair.
	p.pl.bst.kills["skeleton_minion"] = game.BestiaryKill{N: 99, Lo: 1, Hi: 3}
	killAgain(z, p, m)
	e = peer.lastBestiaryEntry("skeleton_minion")
	if e.Mark != 2 || len(e.Drops) != len(m.mon.def.Loot) || e.Drops[0].Name == "" || e.GoldChance == 0 || !e.Gear || e.DamageBonus != 0 {
		t.Fatalf("marco 2: %+v", e)
	}

	// 500º: o bônus de dano contra o tipo.
	p.pl.bst.kills["skeleton_minion"] = game.BestiaryKill{N: 499, Lo: 1, Hi: 3}
	killAgain(z, p, m)
	e = peer.lastBestiaryEntry("skeleton_minion")
	if e.Mark != 3 || e.DamageBonus != float32(game.BestiaryDamageBonus) {
		t.Fatalf("marco 3: %+v", e)
	}
	if !peer.systemContains(fmt.Sprintf("+%.0f%% de dano contra Esqueleto Lacaio", game.BestiaryDamageBonus*100)) {
		t.Fatal("o marco 3 avisa o bônus")
	}
	if n := len(peer.bestiaryMarks()); n != 3 {
		t.Fatalf("%d avisos de marco, quero 3 (um por marco)", n)
	}
	// Continua contando depois do último marco, sem aviso novo.
	killAgain(z, p, m)
	if p.pl.bst.kills["skeleton_minion"].N != 501 || len(peer.bestiaryMarks()) != 3 {
		t.Fatal("depois do 500 só conta")
	}
}

func TestBestiaryBossOpensWithOneKillWithoutBonus(t *testing.T) {
	z := newCombatZone(fixed("bone_lord", 22, 25, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "knight", 23.5, 0, 0)
	atLevel(z, p, 22)
	killAgain(z, p, m)
	e := peer.lastBestiaryEntry("bone_lord")
	if e == nil || !e.Boss || e.Mark != 3 || e.Kills != 1 || len(e.Drops) == 0 || e.LevelMin != 22 || e.DamageBonus != 0 {
		t.Fatalf("chefe com 1 abate: tudo aberto, sem bônus: %+v", e)
	}
	if marks := peer.bestiaryMarks(); len(marks) != 1 || marks[0].Mark != 3 || !strings.Contains(marks[0].Text, "entrada completa") {
		t.Fatalf("chefe: um aviso só, %+v", marks)
	}
	if z.bestiaryBonus(p, m) != 0 {
		t.Fatal("sem bônus contra chefe")
	}
}

// Crédito: quem leva a XP conta o abate inteiro (o grupo perto, dentro da diferença de
// nível); quem está longe ou fora do grupo não conta.
func TestBestiaryPartyCreditCountsEveryTaker(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 8, 30, 0))
	m := z.onlyMonster()
	a, _ := joinAt(z, 1, "knight", 28, 0, 0)
	b, _ := joinAt(z, 2, "mage", 29, 2, 0)
	far, _ := joinAt(z, 3, "rogue", 30+partyRange+20, 0, 0)
	other, _ := joinAt(z, 4, "barbarian", 31, 1, 0)
	for _, p := range []*entity{a, b, far, other} {
		atLevel(z, p, 8)
	}
	makeParty(z, a, b, far)
	m.mon.hate = map[uint32]float64{a.id: 50, b.id: 30, other.id: 40} // o grupo causou mais
	z.kill(m, a)
	z.tickFor(0.1)
	for _, c := range []struct {
		p    *entity
		want int
	}{{a, 1}, {b, 1}, {far, 0}, {other, 0}} {
		if got := c.p.pl.bst.kills["skeleton_warrior"].N; got != c.want {
			t.Errorf("%s: %d abates, quero %d", c.p.name, got, c.want)
		}
	}
}

// Raid boss: conta para todo mundo que causou dano, até quem ficou sem XP (menos de 5%).
func TestBestiaryRaidCountsEveryDamager(t *testing.T) {
	now := noon
	z, m := newRaidZone(&now)
	a, _ := raider(z, 1, "knight", raidX-4, raidZ)
	solo, _ := raider(z, 2, "mage", raidX-4, raidZ-2)
	idle, _ := raider(z, 3, "rogue", raidX-5, raidZ+2)
	z.raidCredit(m, a, 970)
	z.raidCredit(m, solo, 30)
	z.kill(m, a)
	z.tickFor(0.1)
	if a.pl.bst.kills["ozrhal"].N != 1 || solo.pl.bst.kills["ozrhal"].N != 1 {
		t.Fatalf("quem bateu conta: %d e %d", a.pl.bst.kills["ozrhal"].N, solo.pl.bst.kills["ozrhal"].N)
	}
	if idle.pl.bst.kills["ozrhal"].N != 0 {
		t.Fatal("quem não bateu não conta")
	}
}

func TestBestiaryDamageBonusInCombat(t *testing.T) {
	z := newCombatZone(fixed("stone_golem", 20, 25, 0), fixed("skeleton_mage", 15, 25, 3))
	var golem, mage *entity
	for _, m := range z.monsters {
		if m.mon.def.ID == "stone_golem" {
			golem = m
		} else {
			mage = m
		}
	}
	p, _ := joinAt(z, 1, "knight", 23.5, 0, 0)
	atLevel(z, p, 20)
	base := z.hitInput(p, golem, false, 1.5, 0).Mult
	p.pl.bst.kills["stone_golem"] = game.BestiaryKill{N: game.BestiaryMarkBonus - 1}
	if got := z.hitInput(p, golem, false, 1.5, 0).Mult; got != base {
		t.Fatalf("499 abates: mult %v, quero %v", got, base)
	}
	p.pl.bst.kills["stone_golem"] = game.BestiaryKill{N: game.BestiaryMarkBonus}
	want := base * (1 + game.BestiaryDamageBonus)
	if got := z.hitInput(p, golem, false, 1.5, 0).Mult; got != want {
		t.Fatalf("500 abates: mult %v, quero %v", got, want)
	}
	if got := z.hitInput(p, mage, false, 1, 0).Mult; got != 1 {
		t.Fatalf("o bônus é só contra o tipo: mult %v contra o mago", got)
	}
	if got := z.hitInput(golem, p, false, 1, 0).Mult; got != 1 {
		t.Fatalf("monstro batendo no jogador: mult %v", got)
	}
	// O dano médio do golpe sobe na mesma proporção.
	in := z.hitInput(p, golem, false, 1, 0)
	p.pl.bst.kills["stone_golem"] = game.BestiaryKill{}
	in0 := z.hitInput(p, golem, false, 1, 0)
	if r := game.ExpectedHit(in) / game.ExpectedHit(in0); math.Abs(r-(1+game.BestiaryDamageBonus)) > 1e-4 {
		t.Fatalf("dano médio ×%v com o bônus, quero ×%v", r, 1+game.BestiaryDamageBonus)
	}
}

func TestBestiaryFamilyCompleteGivesTitleAndOutfit(t *testing.T) {
	z := newCombatZone(fixed("stone_golem", 25, 25, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "knight", 23.5, 0, 0)
	atLevel(z, p, 25)
	golems := game.BestiaryFamilyByID("golems")
	if len(game.BestiaryFamilyMembers(golems)) != 1 {
		t.Fatal("hoje a família dos golens é só o Golem de Pedra")
	}
	// Antes: nem título nem cor.
	z.setTitle(p, golems.TitleID())
	if p.pl.ach.title != "" || !peer.systemContains("Você ainda não tem esse título") {
		t.Fatal("título de família sem a família completa")
	}
	if z.bestiaryOutfitLocked(p, golems.OutfitIndex()) == "" {
		t.Fatal("a cor exclusiva começa bloqueada")
	}

	p.pl.bst.kills["stone_golem"] = game.BestiaryKill{N: game.BestiaryMarkBonus - 1, Lo: 20, Hi: 30}
	killAgain(z, p, m)
	if _, ok := p.pl.bst.families["golems"]; !ok {
		t.Fatal("o 500º golem completa a família")
	}
	var fam *pb.BestiaryMark
	for _, mk := range peer.bestiaryMarks() {
		if mk.Family == "golems" {
			fam = mk
		}
	}
	if fam == nil || !strings.Contains(fam.Text, "Quebra-Rochas") || !peer.systemContains("Família Golens completa") {
		t.Fatalf("aviso da família: %+v", fam)
	}
	var upd *pb.BestiaryFamily
	for _, b := range peer.bestiaryMsgs() {
		for _, f := range b.Families {
			if f.Id == "golems" && !b.Full {
				upd = f
			}
		}
	}
	if upd == nil || !upd.Complete || upd.Done != 1 || upd.Total != 1 || upd.TitleId != "bst:golems" || upd.Outfit != uint32(golems.OutfitIndex()) {
		t.Fatalf("família na atualização: %+v", upd)
	}

	// Agora o título vale (e aparece para todos) e a cor sai do bloqueio.
	z.setTitle(p, golems.TitleID())
	if p.pl.ach.title != "bst:golems" || p.pl.ach.titleText() != "Quebra-Rochas" {
		t.Fatalf("título em uso: %q (%q)", p.pl.ach.title, p.pl.ach.titleText())
	}
	if st := peer.lastStatus(p.id); st == nil || st.Title != "Quebra-Rochas" {
		t.Fatalf("o título vai no PlayerStatus: %+v", st)
	}
	if msg := z.bestiaryOutfitLocked(p, golems.OutfitIndex()); msg != "" {
		t.Fatalf("cor liberada, mas: %s", msg)
	}
	if z.bestiaryOutfitLocked(p, game.BestiaryFamilyByID("skeletons").OutfitIndex()) == "" {
		t.Fatal("a cor dos esqueletos continua bloqueada")
	}
	if z.bestiaryOutfitLocked(p, 3) != "" {
		t.Fatal("as cores comuns nunca bloqueiam")
	}
	// Um abate a mais não completa de novo.
	n := len(peer.bestiaryMarks())
	killAgain(z, p, m)
	if len(peer.bestiaryMarks()) != n {
		t.Fatal("família completa uma vez só")
	}
}

func TestBestiarySavesAndLoads(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 2, 25, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 23.5, 0, 0)
	killAgain(z, p, m)
	s := z.savedState(p)
	if s.Bestiary.Kills["skeleton_minion"] != (game.BestiaryKill{N: 1, Lo: 2, Hi: 2}) {
		t.Fatalf("o salvamento leva o bestiário: %+v", s.Bestiary)
	}
	s.Bestiary.Kills["skeleton_minion"] = game.BestiaryKill{N: 7}
	if p.pl.bst.kills["skeleton_minion"].N != 1 {
		t.Fatal("o estado salvo é uma cópia")
	}

	// Volta com o salvo: famílias que não existem saem, abates de outros monstros ficam, e o
	// título de família só vale com a família completa.
	saved := game.BestiaryState{
		Kills:    map[string]game.BestiaryKill{"stone_golem": {N: 500, Lo: 20, Hi: 30}, "wolf_of_other_zone": {N: 4}, "bad": {N: -3}},
		Families: map[string]int64{"golems": 123, "nao_existe": 1},
	}
	peer := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: 2, Name: "Volta", Class: "mage", Bestiary: saved, Title: "bst:golems", Peer: peer}})
	q := z.players[2]
	if q.pl.bst.kills["stone_golem"].N != 500 || q.pl.bst.kills["wolf_of_other_zone"].N != 4 {
		t.Fatalf("abates carregados: %+v", q.pl.bst.kills)
	}
	if _, bad := q.pl.bst.kills["bad"]; bad {
		t.Fatal("contagem inválida sai")
	}
	if _, ok := q.pl.bst.families["nao_existe"]; ok || q.pl.bst.families["golems"] != 123 {
		t.Fatalf("famílias: %+v", q.pl.bst.families)
	}
	if q.pl.ach.title != "bst:golems" || q.pl.ach.titleText() != "Quebra-Rochas" {
		t.Fatalf("título de família carregado: %q", q.pl.ach.title)
	}
	if e := peer.lastBestiaryEntry("stone_golem"); e == nil || e.Mark != 3 || e.LevelMax != 30 {
		t.Fatalf("o join mostra o que estava salvo: %+v", e)
	}

	peer3 := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: 3, Name: "Falso", Class: "mage", Title: "bst:skeletons", Peer: peer3}})
	if z.players[3].pl.ach.title != "" {
		t.Fatal("título de família sem a família completa sai no join")
	}

	// Uma família que ficou completa sem aviso (o catálogo mudou) completa no join.
	full := game.BestiaryState{Kills: map[string]game.BestiaryKill{"stone_golem": {N: 600}}}
	z.handle(joinEvent{info: JoinInfo{ID: 4, Name: "Antigo", Class: "rogue", Bestiary: full, Peer: &fakePeer{}}})
	if _, ok := z.players[4].pl.bst.families["golems"]; !ok {
		t.Fatal("família já cumprida completa no join")
	}
}

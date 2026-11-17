package zone

import (
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// ---------------------------------------------------------------------------------------
// Subclasse (tunnel#156)
// ---------------------------------------------------------------------------------------

func guardianOf(z *Zone) *entity {
	for _, n := range z.npcs {
		if n.npc.role == pb.NpcRole_NPC_ROLE_SUBCLASS {
			return n
		}
	}
	return nil
}

// subCmd manda um pedido ao Guardião (com tokens de ação cheios).
func subCmd(z *Zone, p *entity, kind pb.SubclassCmdKind, class string) {
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(progressEvent{id: p.id, msg: &pb.SubclassCmd{Kind: kind, NpcId: guardianOf(z).id, Class: class}})
}

func (f *fakePeer) lastSubclass() *pb.SubclassState {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if c := f.msgs[i].GetSubclass(); c != nil {
			return c
		}
	}
	return nil
}

// subZone: zona de combate com o relógio nas mãos do teste.
func subZone(t *testing.T) (*Zone, *time.Time) {
	t.Helper()
	now := noon
	return newClockZone(&now), &now
}

// joinNearGuardian: jogador ao lado do Guardião, com o kit do cavaleiro equipado (espada e
// escudo: o mago não usa nenhum dos dois).
func joinNearGuardian(z *Zone, id uint32, info JoinInfo) (*entity, *fakePeer) {
	g := guardianOf(z)
	peer := &fakePeer{}
	info.ID, info.Peer, info.X, info.Z = id, peer, g.pos.X-1.5, g.pos.Z
	if info.Name == "" {
		info.Name = "Sub" + strconv.Itoa(int(id))
	}
	if info.Class == "" {
		info.Class = "knight"
	}
	z.handle(joinEvent{info: info})
	return z.players[id], peer
}

// knightWithMage: cavaleiro no nível do XP pedido, com a subclasse mago já ganha (no 20).
func knightWithMage(z *Zone, id uint32, xp int64, items []game.ItemRecord) (*entity, *fakePeer) {
	return joinNearGuardian(z, id, JoinInfo{
		Class: "knight", XP: xp, Items: items, Gold: 777,
		Classes: game.ClassSet{Active: "knight", List: []game.ClassProgress{{Class: "knight"}, game.NewSubclass("mage")}},
		Quests:  game.QuestState{Done: []string{game.SpecQuest, game.SubclassQuestCall, game.SubclassQuestTrial, game.SubclassQuestOath}},
	})
}

func TestGuardianIsInTown(t *testing.T) {
	z := newCombatZone()
	g := guardianOf(z)
	if g == nil || g.name != game.TalkGuardian || g.npc.title == "" || !z.inSafe(g.pos) {
		t.Fatalf("Guardião das Sendas: %+v", g)
	}
	if !slices.ContainsFunc(z.npcList(), func(n *pb.NpcInfo) bool { return n.Role == pb.NpcRole_NPC_ROLE_SUBCLASS && n.Id == g.id }) {
		t.Error("o Guardião deveria ir na lista de NPCs do Welcome")
	}
	// Os passos de falar com ele apontam para ele no mapa.
	p, _ := joinNearGuardian(z, 1, JoinInfo{XP: game.XPLevelStart(30)})
	place, ok := z.questPlace(p, game.Quests[game.SubclassQuestCall].Steps[0])
	if !ok || place.X != g.pos.X || place.Z != g.pos.Z {
		t.Fatalf("lugar do passo: %+v (%v)", place, ok)
	}
}

// A cadeia inteira: chega no 30, falar → prova (golens) → juramento → escolher. A mesma
// classe da principal é recusada; a escolhida começa no 20, no início da XP dele.
func TestSubclassQuestChain(t *testing.T) {
	z, _ := subZone(t)
	p, peer := joinNearGuardian(z, 1, JoinInfo{XP: game.XPLevelStart(29), Quests: game.QuestState{Done: []string{game.SpecQuest}}})
	if p.pl.quests.has(game.SubclassQuestCall) {
		t.Fatal("o chamado antes do 30")
	}
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_TALK, "")
	if peer.lastSubclass().GetNpcId() != 0 || !peer.systemContains("nível 30") {
		t.Fatal("abaixo do 30 o Guardião só aconselha")
	}
	z.grantXP(p, game.XPLevelStart(30)-p.pl.xp)
	if p.level != 30 || !p.pl.quests.has(game.SubclassQuestCall) {
		t.Fatalf("nível %d, missões %v", p.level, activeIDs(p))
	}
	if game.Quests[game.SubclassQuestCall].Category != game.QuestStory {
		t.Error("a missão é da história")
	}

	// 1. Falar com ele.
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_TALK, "")
	if !p.pl.quests.done[game.SubclassQuestCall] || !p.pl.quests.has(game.SubclassQuestTrial) {
		t.Fatalf("depois de falar: %v", activeIDs(p))
	}
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_CHOOSE, "mage")
	if p.pl.classes.Subclasses() != 0 {
		t.Fatal("escolheu antes do juramento")
	}
	// 2. A prova: falar de novo não adianta; matar os golens, sim.
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_TALK, "")
	if p.pl.quests.done[game.SubclassQuestTrial] {
		t.Fatal("a prova não é de conversa")
	}
	gold := p.pl.inv.gold
	for range game.SubclassTrialKills {
		z.questEvent(p, game.GoalKill, game.SubclassTrialMonster)
	}
	if !p.pl.quests.done[game.SubclassQuestTrial] || !p.pl.quests.has(game.SubclassQuestOath) || p.pl.inv.gold != gold+5000 {
		t.Fatalf("depois da prova: %v, ouro %d", activeIDs(p), p.pl.inv.gold-gold)
	}
	// 3. O juramento: a janela abre com a escolha.
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_TALK, "")
	st := peer.lastSubclass()
	if !p.pl.quests.done[game.SubclassQuestOath] || st == nil || !st.CanChoose || st.NpcId != guardianOf(z).id || st.Main != "knight" {
		t.Fatalf("janela depois do juramento: %+v", st)
	}

	// A mesma classe da principal (ou uma inválida) é recusada.
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_CHOOSE, "knight")
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_CHOOSE, "dragon")
	if p.pl.classes.Subclasses() != 0 || !peer.systemContains("já é a sua classe principal") {
		t.Fatalf("subclasse igual à principal: %+v", p.pl.classes)
	}
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_CHOOSE, "mage")
	c := p.pl.classes.Find("mage")
	if c == nil || c.Level != game.SubclassStartLevel || c.XP != game.XPLevelStart(game.SubclassStartLevel) || p.pl.class != "knight" {
		t.Fatalf("subclasse escolhida: %+v, ativa %s", p.pl.classes, p.pl.class)
	}
	st = peer.lastSubclass()
	if st.CanChoose || len(st.Classes) != 2 || st.Classes[1].Class != "mage" || st.Classes[1].Level != 20 || !st.Classes[0].Main {
		t.Fatalf("janela depois de escolher: %+v", st)
	}
	// Teto de 1 subclasse.
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_CHOOSE, "rogue")
	if p.pl.classes.Subclasses() != 1 {
		t.Fatal("ganhou uma segunda subclasse")
	}
}

// Trocar nos dois sentidos: cada classe volta com o próprio nível, XP, skills, barra,
// atributos e especialização. Ouro, itens e missões são do personagem.
func TestSwitchClassBothWays(t *testing.T) {
	z, now := subZone(t)
	p, peer := knightWithMage(z, 1, game.XPLevelStart(30)+12345, nil)
	// A progressão do cavaleiro: atributos, uma skill, a barra e o caminho.
	z.allocateAttrs(p, &pb.Attrs{Str: 3, Con: 2})
	p.pl.spec = "bastion"
	var learn uint32
	for _, s := range game.ClassTree("knight") {
		if s.LevelReq() <= 30 && p.pl.ranks[s.ID] < s.MaxRank && !s.IsPassive() && s.Spec == "" {
			next := maps.Clone(p.pl.ranks)
			next[s.ID]++
			if game.ValidateRanks("knight", "bastion", 30, next) == nil {
				learn = s.ID
				p.pl.ranks = next
				break
			}
		}
	}
	p.pl.hotbar[5] = "item:potion_hp_s"
	knight := struct {
		level  int
		xp     int64
		attrs  game.Attrs
		ranks  map[uint32]int
		hotbar []string
	}{p.level, p.pl.xp, p.pl.attrs, maps.Clone(p.pl.ranks), slices.Clone(p.pl.hotbar)}
	gold, items := p.pl.inv.gold, p.pl.inv.records()

	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	if p.pl.class != "mage" || p.level != 20 || p.pl.xp != game.XPLevelStart(20) || p.pl.spec != "" || p.pl.attrs != (game.Attrs{}) {
		t.Fatalf("mago: classe %s nível %d XP %d spec %q attrs %+v", p.pl.class, p.level, p.pl.xp, p.pl.spec, p.pl.attrs)
	}
	if !maps.Equal(p.pl.ranks, game.StarterRanks("mage")) || !slices.Equal(p.pl.hotbar, DefaultHotbar("mage")) {
		t.Fatalf("mago: ranks %v barra %v", p.pl.ranks, p.pl.hotbar)
	}
	if p.model != "knight" || p.pl.mainClass != "knight" || z.entityPB(p).ActiveClass != "mage" {
		t.Fatal("o corpo continua o da principal; a classe ativa vai no Entity")
	}
	if freeAttrPoints(p) != game.AttrPointsTotal(20) || freeSkillPoints(p) != game.SkillPointsTotal(20)-game.RanksUsed(game.StarterRanks("mage")) {
		t.Fatalf("pontos livres do mago: atributo %d, skill %d", freeAttrPoints(p), freeSkillPoints(p))
	}
	st := peer.lastSubclass()
	if st == nil || !st.Switched || st.Active != "mage" || len(st.Skills) == 0 || len(st.Branches) != 3 || st.CooldownMs == 0 {
		t.Fatalf("estado depois da troca: %+v", st)
	}
	for _, s := range st.Skills {
		if game.Skills[s.Id].Class != "mage" {
			t.Fatalf("skill %d de outra classe na árvore do mago", s.Id)
		}
	}
	if got := peer.lastStatus(p.id); got == nil || got.ActiveClass != "mage" || got.Level != 20 {
		t.Fatalf("PlayerStatus: %+v", got)
	}
	// O mago gasta os pontos dele e escolhe o caminho dele (de graça: a Encruzilhada é do personagem).
	z.allocateAttrs(p, &pb.Attrs{Int: 4})
	specCmdAt(z, p, "elementalist")
	if p.pl.spec != "elementalist" {
		t.Fatalf("caminho do mago: %q", p.pl.spec)
	}
	z.grantXP(p, 1000)
	mage := struct {
		xp    int64
		attrs game.Attrs
	}{p.pl.xp, p.pl.attrs}

	// Volta: a recarga segura por 10 min.
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "knight")
	if p.pl.class != "mage" || !peer.systemContains("trocou de classe há pouco") {
		t.Fatal("trocou de novo antes da recarga")
	}
	*now = now.Add(game.SubclassSwitchCooldown)
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "knight")
	if p.pl.class != "knight" || p.level != knight.level || p.pl.xp != knight.xp || p.pl.attrs != knight.attrs || p.pl.spec != "bastion" {
		t.Fatalf("cavaleiro de volta: nível %d XP %d attrs %+v spec %q", p.level, p.pl.xp, p.pl.attrs, p.pl.spec)
	}
	if !maps.Equal(p.pl.ranks, knight.ranks) || !slices.Equal(p.pl.hotbar, knight.hotbar) || learn == 0 || p.pl.ranks[learn] == 0 {
		t.Fatalf("cavaleiro de volta: ranks %v barra %v", p.pl.ranks, p.pl.hotbar)
	}
	if m := p.pl.classes.Find("mage"); m.XP != mage.xp || m.Attrs != mage.attrs || m.Spec != "elementalist" || m.Level != 20 {
		t.Fatalf("o mago guardado: %+v", m)
	}
	// Nada duplicou nem sumiu: ouro e itens iguais (o kit vazio não tinha o que desequipar).
	if p.pl.inv.gold != gold || !slices.Equal(p.pl.inv.records(), items) {
		t.Fatalf("ouro %d→%d, itens %v→%v", gold, p.pl.inv.gold, items, p.pl.inv.records())
	}
	if freeAttrPoints(p) != game.AttrPointsTotal(30)-5 {
		t.Fatalf("pontos de atributo do cavaleiro: %d", freeAttrPoints(p))
	}
	z.checkInvariants(p) // modo estrito nos testes: quebra vira panic
}

// specCmdAt escolhe o caminho na Mestra, de onde estiver (o teste leva o jogador até ela).
func specCmdAt(z *Zone, p *entity, spec string) {
	m := masterOf(z)
	old := p.pos
	p.pos = Vec2{X: m.pos.X - 1.5, Z: m.pos.Z}
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, spec)
	p.pos = old
}

// A troca só acontece vivo, fora de combate, sem troca de itens aberta, depois da recarga
// e perto do Guardião.
func TestSwitchClassBlocked(t *testing.T) {
	z, now := subZone(t)
	p, peer := knightWithMage(z, 1, game.XPLevelStart(30), nil)
	other, _ := joinNearGuardian(z, 2, JoinInfo{Class: "rogue", XP: game.XPLevelStart(10)})

	// Em combate.
	p.lastCombat = z.now
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	if p.pl.class != "knight" || !peer.systemContains("Saia de combate") {
		t.Fatal("trocou de classe em combate")
	}
	p.lastCombat = z.now - outOfCombatAfter - 1

	// Com troca de itens aberta.
	z.openTrade(p, other)
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	if p.pl.class != "knight" || !peer.systemContains("Termine a troca") {
		t.Fatal("trocou de classe no meio de uma troca de itens")
	}
	z.closeTrade(p.pl.trade, "")

	// Morto.
	p.dead = true
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	if p.pl.class != "knight" {
		t.Fatal("trocou de classe morto")
	}
	p.dead = false

	// Longe do Guardião.
	p.pos = Vec2{X: 30, Z: 30}
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	if p.pl.class != "knight" {
		t.Fatal("trocou de classe longe do Guardião")
	}
	g := guardianOf(z)
	p.pos = Vec2{X: g.pos.X - 1.5, Z: g.pos.Z}

	// Classe que ele não tem.
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "rogue")
	if p.pl.class != "knight" {
		t.Fatal("trocou para uma classe que não tem")
	}

	// Livre: troca, e a recarga vale até depois de sair e voltar.
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	if p.pl.class != "mage" {
		t.Fatal("não trocou com tudo livre")
	}
	*now = now.Add(game.SubclassSwitchCooldown - time.Second)
	st := z.savedState(p)
	z.removePlayer(p.id)
	delete(z.recent, p.id)
	p, peer = joinNearGuardian(z, 1, JoinInfo{Class: "knight", XP: st.XP, Attrs: st.Attrs, Ranks: st.Ranks, Hotbar: st.Hotbar,
		Spec: st.Spec, Classes: st.Classes, Quests: st.Quests})
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "knight")
	if p.pl.class != "mage" || !peer.systemContains("trocou de classe há pouco") {
		t.Fatal("sair e voltar zerou a recarga")
	}
	*now = now.Add(time.Second)
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "knight")
	if p.pl.class != "knight" {
		t.Fatal("não trocou depois da recarga")
	}
}

// O equipamento que a classe nova não usa vai para a bolsa; sem espaço, a troca é
// recusada e nada muda. Nenhum item some nem duplica.
func TestSwitchClassUnequipsGear(t *testing.T) {
	kit := []game.ItemRecord{
		{Equipped: true, Slot: game.SlotWeapon, ItemID: "sword_common", Count: 1},
		{Equipped: true, Slot: game.SlotOffhand, ItemID: "shield_badge", Count: 1},
	}
	full := slices.Clone(kit)
	for i := range game.BagSize - 1 { // um espaço livre só: não cabem as duas peças
		full = append(full, game.ItemRecord{Slot: i, ItemID: "bone_old", Count: 1})
	}
	z, _ := subZone(t)
	p, peer := knightWithMage(z, 1, game.XPLevelStart(30), full)
	before := p.pl.inv.records()
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	if p.pl.class != "knight" || !peer.systemContains("Sua bolsa está cheia") || !slices.Equal(p.pl.inv.records(), before) {
		t.Fatalf("trocou com a bolsa cheia: classe %s", p.pl.class)
	}
	if p.pl.classes.SwitchedAt != 0 {
		t.Fatal("a troca recusada contou na recarga")
	}

	// Com espaço: espada e escudo vão para a bolsa.
	z2, _ := subZone(t)
	q, qpeer := knightWithMage(z2, 1, game.XPLevelStart(30), kit)
	subCmd(z2, q, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	if q.pl.class != "mage" || !q.pl.inv.equip[game.SlotWeapon].empty() || !q.pl.inv.equip[game.SlotOffhand].empty() {
		t.Fatalf("o mago ficou com espada ou escudo: %v", q.pl.inv.records())
	}
	if q.pl.inv.countOf("sword_common") != 1 || q.pl.inv.countOf("shield_badge") != 1 || len(q.pl.inv.records()) != len(kit) {
		t.Fatalf("itens depois da troca: %v", q.pl.inv.records())
	}
	if inv := qpeer.lastInventory(); inv == nil {
		t.Fatal("o cliente não recebeu a bolsa nova")
	}
	z2.checkInvariants(q)
	// O cavaleiro de volta re-equipa à mão (a troca não equipa nada sozinha).
	q.pl.classes.SwitchedAt = 0
	subCmd(z2, q, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "knight")
	if q.pl.class != "knight" || len(q.pl.inv.records()) != len(kit) {
		t.Fatalf("de volta ao cavaleiro: %v", q.pl.inv.records())
	}
}

// A XP ganha vai para a classe ativa; a outra fica parada. O bônus Duas Sendas (+5%) só
// vale com uma subclasse no 30, e só para a classe mais baixa.
func TestSubclassXPGoesToActiveWithBonus(t *testing.T) {
	z, _ := subZone(t)
	p, _ := knightWithMage(z, 1, game.XPLevelStart(35), nil)
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	knightXP := p.pl.classes.Find("knight").XP
	start := p.pl.xp
	z.grantXP(p, 1000)
	if p.pl.xp != start+1000 || p.pl.classes.Find("knight").XP != knightXP {
		t.Fatalf("XP do mago %d (want %d), cavaleiro %d", p.pl.xp-start, 1000, p.pl.classes.Find("knight").XP)
	}

	// O mago chega ao 30: Duas Sendas (título e +5% para quem estiver mais baixo).
	z.grantXP(p, game.XPLevelStart(30)-p.pl.xp)
	if _, ok := p.pl.ach.unlocked["two_paths"]; p.level != 30 || !ok {
		t.Fatalf("mago no %d, conquista %v", p.level, p.pl.ach.unlocked)
	}
	before := p.pl.xp
	z.grantXP(p, 1000)
	if got := p.pl.xp - before; got != 1050 {
		t.Fatalf("mago (30, abaixo do cavaleiro 35) ganhou %d de 1000", got)
	}
	// O cavaleiro (35) é o mais alto: sem bônus.
	p.pl.classes.SwitchedAt = 0
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "knight")
	before = p.pl.xp
	z.grantXP(p, 1000)
	if got := p.pl.xp - before; got != 1000 || p.level != 35 {
		t.Fatalf("cavaleiro (35) ganhou %d de 1000", got)
	}
	// Empate não é "mais baixa".
	set := game.ClassSet{Main: "knight", Active: "knight", List: []game.ClassProgress{{Class: "knight", Level: 30}, {Class: "mage", Level: 30}}}
	if set.XPMult(30) != 1 || set.XPMult(29) <= 1 {
		t.Fatalf("XPMult: empate %v, abaixo %v", set.XPMult(30), set.XPMult(29))
	}
	// Sem a subclasse no 30 (só a principal), nada.
	set.List[1].Level = 29
	if set.XPMult(25) != 1 {
		t.Fatal("bônus sem a subclasse no 30")
	}
}

// A Maestria é do personagem: com a subclasse ativa abaixo do máximo os pontos continuam
// lá e a XP não enche a barra.
func TestMasteryStaysWithCharacter(t *testing.T) {
	z, _ := subZone(t)
	g := guardianOf(z)
	p, _ := joinNearGuardian(z, 1, JoinInfo{Class: "knight", XP: game.XPLevelStart(game.MaxLevel), X: g.pos.X, Z: g.pos.Z,
		Mastery: game.MasteryState{Points: 3},
		Classes: game.ClassSet{List: []game.ClassProgress{{Class: "knight"}, game.NewSubclass("rogue")}},
		Quests:  game.QuestState{Done: []string{game.SubclassQuestOath}}})
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "rogue")
	if p.pl.class != "rogue" || p.pl.mastery.st.Points != 3 {
		t.Fatalf("ladino: classe %s, Maestria %d", p.pl.class, p.pl.mastery.st.Points)
	}
	bar := p.pl.mastery.st.XP
	z.grantXP(p, 5000)
	if p.pl.mastery.st.XP != bar || p.pl.xp != game.XPLevelStart(20)+5000 {
		t.Fatal("a XP da subclasse foi para a barra de Maestria")
	}
}

// Salvar e carregar: o estado salvo leva todas as classes (com a ativa em dia) e, ao
// voltar, cada uma está onde ficou.
func TestSubclassSaveAndLoad(t *testing.T) {
	saves := make(chan []SavedState, 8)
	z := newTestZone(saves)
	z.cfg.SafeRadius = 40
	p, _ := knightWithMage(z, 1, game.XPLevelStart(31)+99, nil)
	z.allocateAttrs(p, &pb.Attrs{Dex: 2})
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	z.grantXP(p, 4321)
	z.allocateAttrs(p, &pb.Attrs{Wis: 1})

	st := z.savedState(p)
	if st.Classes.Active != "mage" || st.Classes.Main != "knight" || len(st.Classes.List) != 2 {
		t.Fatalf("classes salvas: %+v", st.Classes)
	}
	if m := st.Classes.Find("mage"); m.XP != p.pl.xp || m.Attrs != (game.Attrs{Wis: 1}) {
		t.Fatalf("a entrada da ativa não está em dia: %+v", m)
	}
	if k := st.Classes.Find("knight"); k.XP != game.XPLevelStart(31)+99 || k.Level != 31 || k.Attrs != (game.Attrs{Dex: 2}) {
		t.Fatalf("cavaleiro salvo: %+v", k)
	}
	// O salvamento é uma cópia: mexer na memória depois não muda o que foi salvo.
	p.pl.classes.Find("knight").Ranks[999] = 1
	if _, ok := st.Classes.Find("knight").Ranks[999]; ok {
		t.Fatal("o salvamento divide o mapa com a memória")
	}
	delete(p.pl.classes.Find("knight").Ranks, 999)

	// Volta do banco: colunas = a ativa (o mago), classes = todas.
	z.removePlayer(p.id)
	delete(z.recent, p.id)
	q, peer := joinNearGuardian(z, 1, JoinInfo{Class: "knight", XP: st.XP, Attrs: st.Attrs, Ranks: st.Ranks, Hotbar: st.Hotbar,
		Spec: st.Spec, Classes: st.Classes, Quests: st.Quests})
	if q.pl.class != "mage" || q.model != "knight" || q.pl.xp != st.XP || q.pl.attrs != (game.Attrs{Wis: 1}) {
		t.Fatalf("carregado: classe %s modelo %s XP %d attrs %+v", q.pl.class, q.model, q.pl.xp, q.pl.attrs)
	}
	if s := peer.lastSubclass(); s == nil || s.Active != "mage" || s.Main != "knight" || len(s.Classes) != 2 || s.Classes[0].Level != 31 {
		t.Fatalf("SubclassState ao entrar: %+v", s)
	}
	// Pelo "recent" (quem acabou de sair) também.
	z.removePlayer(q.id)
	r, _ := joinNearGuardian(z, 1, JoinInfo{Class: "knight"}) // o banco ainda não tem nada
	if r.pl.class != "mage" || r.pl.xp != st.XP || r.pl.classes.Find("knight").Level != 31 {
		t.Fatalf("pelo recent: classe %s XP %d", r.pl.class, r.pl.xp)
	}
	z.checkInvariants(r)
}

// Personagem de antes da subclasse (classes vazio no banco) entra com a classe dele como
// principal e ativa, e o salvamento já leva a lista.
func TestOldCharacterBecomesMainClass(t *testing.T) {
	z := newTestZone(nil)
	join(z, 1, "Antigo")
	e := z.players[1]
	if e.pl.class != "knight" || e.pl.mainClass != "knight" || e.pl.classes.Main != "knight" || len(e.pl.classes.List) != 1 {
		t.Fatalf("classes de um personagem antigo: %+v", e.pl.classes)
	}
	st := z.savedState(e)
	if st.Classes.Active != "knight" || st.Classes.List[0].Class != "knight" || st.Classes.List[0].Level != e.level {
		t.Fatalf("salvo: %+v", st.Classes)
	}
}

// Com a vitalidade (tunnel#150): o bônus da XP de monstro cai na classe ativa, e o teto do
// estoque é o do nível dela.
func TestSubclassVitalityUsesActiveClass(t *testing.T) {
	now := noon
	cfg := testConfig()
	cfg.SafeRadius = 20
	cfg.Clock = func() time.Time { return now }
	cfg.Spawns = []game.Spawn{fixed("stone_golem", 20, 25, 0)}
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	z.SetEventSink(nopSink{})
	p, _ := knightWithMage(z, 1, game.XPLevelStart(35), nil)
	p.pl.vit.stored = game.VitalityCap(35)
	subCmd(z, p, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	if p.pl.class != "mage" || p.pl.vit.stored != game.VitalityCap(20) {
		t.Fatalf("vitalidade depois da troca: %d (teto do 20: %d)", p.pl.vit.stored, game.VitalityCap(20))
	}
	drainEvents(z)
	m := z.onlyMonster()
	base := game.MonsterXP(m.mon.def, m.level, p.level)
	knightXP := p.pl.classes.Find("knight").XP
	before, stored := p.pl.xp, p.pl.vit.stored
	z.shareMonsterXP(m, []*entity{p})
	if got := p.pl.xp - before; got != 2*base || p.pl.vit.stored != stored-base || p.pl.classes.Find("knight").XP != knightXP {
		t.Fatalf("XP do mago com vitalidade: +%d (base %d), estoque %d→%d", got, base, stored, p.pl.vit.stored)
	}
	if game.VitalityPercent(p.pl.vit.stored, p.level) > 100 {
		t.Fatal("o estoque passou do teto da classe ativa")
	}
}

// Zonas (tunnel#136): as classes são do personagem e atravessam o portão (a ativa, a
// progressão de cada uma e a recarga da troca); o corpo continua o da principal. O
// Guardião só existe no mapa principal.
func TestGateCarriesClasses(t *testing.T) {
	m, a, b := twoZones(t, nil)
	if guardianOf(a) == nil || guardianOf(b) != nil {
		t.Fatal("o Guardião é só do mapa principal")
	}
	p := mjoin(m, 1, "Ana", "", 0, -14)
	ana := a.players[1]
	ana.pl.quests.done[game.SubclassQuestOath] = true
	ana.pl.classes.List = append(ana.pl.classes.List, game.NewSubclass("mage"))
	ana.lastCombat = a.now - 60
	g := guardianOf(a)
	pos := ana.pos
	ana.pos = Vec2{X: g.pos.X - 1.5, Z: g.pos.Z}
	subCmd(a, ana, pb.SubclassCmdKind_SUBCLASS_CMD_SWITCH, "mage")
	ana.pos = pos
	if ana.pl.class != "mage" {
		t.Fatal("não trocou para o mago")
	}
	a.grantXP(ana, 777)
	want := classSnapshot(ana)

	ana.pl.moveTokens = a.cfg.MaxMovesPerSec
	useGate(m, p, 1)
	got := b.players[1]
	if got == nil {
		p.mu.Lock()
		for _, msg := range p.msgs {
			if c := msg.GetChat(); c != nil {
				t.Log(c.Text)
			}
		}
		p.mu.Unlock()
		t.Fatal("não atravessou")
	}
	if got.pl.class != "mage" || got.pl.mainClass != "knight" || got.model != "knight" || got.level != 20 || got.pl.xp != ana.pl.xp {
		t.Fatalf("do outro lado: classe %s principal %s modelo %s nível %d XP %d", got.pl.class, got.pl.mainClass, got.model, got.level, got.pl.xp)
	}
	if fmt.Sprint(classSnapshot(got)) != fmt.Sprint(want) || got.pl.classes.SwitchedAt == 0 {
		t.Fatalf("classes: %+v, esperava %+v", got.pl.classes, want)
	}
	b.checkInvariants(got)
}

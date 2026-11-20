package zone

import (
	"maps"
	"slices"
	"strconv"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// ---------------------------------------------------------------------------------------
// Especialização (segunda classe)
// ---------------------------------------------------------------------------------------

func masterOf(z *Zone) *entity {
	for _, n := range z.npcs {
		if n.npc.role == pb.NpcRole_NPC_ROLE_MASTER {
			return n
		}
	}
	return nil
}

// specCmd manda um pedido à Mestra (com tokens de ação cheios, como se tivesse esperado).
func specCmd(z *Zone, p *entity, kind pb.SpecCmdKind, spec string) {
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(progressEvent{id: p.id, msg: &pb.SpecCmd{Kind: kind, NpcId: masterOf(z).id, Spec: spec}})
}

func (f *fakePeer) lastSpecChoice() *pb.SpecChoice {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if c := f.msgs[i].GetSpecChoice(); c != nil {
			return c
		}
	}
	return nil
}

// nearMaster: jogador ao lado da Mestra, no nível pedido.
func nearMaster(z *Zone, id uint32, class string, level int) (*entity, *fakePeer) {
	m := masterOf(z)
	p, peer := joinAt(z, id, class, m.pos.X-1.5, m.pos.Z, game.XPLevelStart(level))
	return p, peer
}

func TestMasterIsInTown(t *testing.T) {
	z := newCombatZone()
	m := masterOf(z)
	if m == nil || m.name == "" || m.npc.title == "" || !z.inSafe(m.pos) {
		t.Fatalf("Mestra dos Caminhos: %+v", m)
	}
	found := false
	for _, n := range z.npcList() {
		found = found || n.Role == pb.NpcRole_NPC_ROLE_MASTER && n.Id == m.id
	}
	if !found {
		t.Error("a Mestra deveria ir na lista de NPCs do Welcome")
	}
}

func TestCrossroadsQuestArrivesAtLevel20(t *testing.T) {
	z := newCombatZone()
	p, peer := nearMaster(z, 1, "knight", 19)
	if p.pl.quests.has(game.SpecQuest) {
		t.Fatal("A Encruzilhada antes do nível 20")
	}
	// Abaixo do 20 a Mestra só aconselha: nada de janela.
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_TALK, "")
	if peer.lastSpecChoice() != nil || !peer.systemContains("nível 20") {
		t.Fatal("abaixo do 20 a Mestra não deveria abrir a escolha")
	}
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "bastion")
	if p.pl.spec != "" {
		t.Fatal("escolheu o caminho no nível 19")
	}

	z.grantXP(p, game.XPLevelStart(20)-p.pl.xp)
	if p.level != 20 || !p.pl.quests.has(game.SpecQuest) {
		t.Fatalf("nível %d, missões %v", p.level, activeIDs(p))
	}
	log := peer.lastQuestLog()
	if log == nil || !slices.ContainsFunc(log.Quests, func(q *pb.QuestInfo) bool { return q.Id == game.SpecQuest }) {
		t.Fatalf("o painel deveria mostrar A Encruzilhada: %+v", log)
	}

	// Personagem que já era 20+ antes da missão existir recebe ao entrar.
	vet, _ := joinAt(z, 2, "rogue", 0, 0, game.XPLevelStart(27))
	if !vet.pl.quests.has(game.SpecQuest) {
		t.Fatalf("veterano do 27 sem A Encruzilhada: %v", activeIDs(vet))
	}
}

func TestChooseSpecThroughCrossroads(t *testing.T) {
	z := newCombatZone()
	p, peer := nearMaster(z, 1, "knight", 20)
	_, otherPeer := joinAt(z, 2, "mage", 0, 0, 0)
	hp := p.stats.MaxHP
	gold := p.pl.inv.gold

	// Falar com ela anda a missão e abre a janela.
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_TALK, "")
	c := peer.lastSpecChoice()
	if c == nil || c.Current != "" || c.NpcId != masterOf(z).id || c.ChangeCost != uint64(game.EvolutionChangeCost(p.level)) {
		t.Fatalf("janela da Mestra: %+v", c)
	}
	if q := questOf(p, game.SpecQuest); q == nil || q.progress[0] != 1 || q.progress[1] != 0 {
		t.Fatalf("missão depois de falar: %+v", q.progress)
	}

	// Caminho de outra classe não vale.
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "elementalist")
	if p.pl.spec != "" {
		t.Fatal("cavaleiro virou Elementalista")
	}

	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "bastion")
	if p.pl.spec != "bastion" {
		t.Fatalf("spec = %q", p.pl.spec)
	}
	if !p.pl.quests.done[game.SpecQuest] || p.pl.quests.has(game.SpecQuest) {
		t.Fatal("escolher deveria concluir A Encruzilhada")
	}
	if p.pl.inv.gold != gold+game.Quests[game.SpecQuest].Reward.Gold {
		t.Errorf("recompensa: %d moedas, want %d", p.pl.inv.gold-gold, game.Quests[game.SpecQuest].Reward.Gold)
	}
	// Bônus: +12 Constituição = +30% de vida (sobre a base, antes das passivas).
	if p.stats.MaxHP < hp*1.25 {
		t.Errorf("vida %.0f → %.0f: o bônus do Bastião não entrou", hp, p.stats.MaxHP)
	}
	if pr := peer.lastProgress(); pr == nil || pr.Spec != "bastion" {
		t.Fatalf("Progress sem a especialização: %+v", pr)
	}
	// Todos sabem: PlayerStatus (estado do jogador) e o Entity de quem chega depois.
	if st := otherPeer.lastStatus(p.id); st == nil || st.Spec != "bastion" {
		t.Fatalf("os outros não souberam: %+v", st)
	}
	if z.entityPB(p).Spec != "bastion" {
		t.Error("Entity.spec vazio")
	}
	// A escolha é definitiva: escolher de novo o mesmo não muda nada; o outro custa caro.
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "herald")
	if p.pl.spec != "bastion" || !peer.systemContains("custa") {
		t.Fatalf("trocou sem pagar: %q", p.pl.spec)
	}
	// O título vem da especialização no cliente: o nome está no catálogo do Welcome.
	if len(specCatalog()) != 8 || specCatalog()[0].BonusText == "" {
		t.Error("catálogo das especializações")
	}
}

func TestSpecSkillsOnlyForChosenPath(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	b, _ := nearMaster(z, 1, "knight", 28)
	h, _ := nearMaster(z, 2, "knight", 28)
	none, _ := joinAt(z, 3, "knight", 30, 0, game.XPLevelStart(28))
	specCmd(z, b, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "bastion")
	specCmd(z, h, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "herald")
	if b.pl.spec != "bastion" || h.pl.spec != "herald" {
		t.Fatalf("specs: %q %q", b.pl.spec, h.pl.spec)
	}

	bastion := []*pb.SkillRank{{SkillId: game.SkillBastionDefy, Rank: 1}, {SkillId: game.SkillBastionVigor, Rank: 1}, {SkillId: game.SkillBulwark, Rank: 1}}
	for _, p := range []*entity{b, h, none} {
		p.pl.moveTokens = z.cfg.MaxMovesPerSec
		z.handle(progressEvent{id: p.id, msg: &pb.LearnSkills{Ranks: bastion}})
	}
	if b.pl.rank(game.SkillBulwark) != 1 || b.pl.rank(game.SkillBastionVigor) != 1 {
		t.Fatalf("o Bastião não aprendeu a própria árvore: %v", b.pl.ranks)
	}
	if h.pl.rank(game.SkillBastionDefy) != 0 || none.pl.rank(game.SkillBastionDefy) != 0 {
		t.Fatal("skill do Bastião aprendida por quem não é Bastião")
	}
	// Passiva do caminho: Vigor do Bastião (+3% de vida no rank 1).
	with := b.stats.MaxHP
	delete(b.pl.ranks, game.SkillBastionVigor)
	if without := z.computeStats(b, b.pl.attrs).MaxHP; with < without*1.029 || with > without*1.031 {
		t.Errorf("Vigor do Bastião: vida %.0f com, %.0f sem", with, without)
	}
	b.pl.ranks[game.SkillBastionVigor] = 1

	// Mesmo com o rank enfiado à força, quem não é do caminho não usa a skill.
	h.pl.ranks[game.SkillBastionDefy] = 1
	h.teleportTo(Vec2{X: 30, Z: 1})
	h.mp = h.stats.MaxMP
	z.requestSkill(h, game.SkillBastionDefy, m.id, false)
	if _, used := h.pl.cooldowns[game.SkillBastionDefy]; used {
		t.Fatal("o Arauto usou a skill do Bastião")
	}
	// Nem na barra de atalhos.
	slot := "skill:" + strconv.Itoa(int(game.SkillBastionDefy))
	if validHotbarEntry(h, slot) || !validHotbarEntry(b, slot) {
		t.Error("barra de atalhos: skill de caminho só para quem é do caminho")
	}
	// E relogar limpa o rank que não é do caminho.
	_, ranks, _ := loadProgress("knight", "herald", 28, game.Attrs{}, h.pl.ranks, nil)
	if ranks[game.SkillBastionDefy] != 0 {
		t.Error("loadProgress manteve skill do outro caminho")
	}

	// O Bastião usa: provoca e ganha o escudo.
	b.teleportTo(Vec2{X: 30, Z: 0})
	m.mon.hate[none.id] = 5000
	b.mp = b.stats.MaxMP
	z.requestSkill(b, game.SkillBastionDefy, 0, false)
	if !m.has(game.EffectTaunt, z.now) || !b.has(game.EffectShield, z.now) || topHate(m.mon.hate) != b.id {
		t.Fatal("Desafio do Bastião deveria provocar e dar o escudo")
	}
}

func TestSpecPersistsAcrossRelogin(t *testing.T) {
	saves := make(chan []SavedState, 16)
	z := newTestZone(saves)
	m := masterOf(z)
	z.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Ana", Class: "mage", X: m.pos.X + 1, Z: m.pos.Z, XP: game.XPLevelStart(22), Peer: &fakePeer{}}})
	ana := z.players[1]
	specCmd(z, ana, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "occultist")
	mAtk := ana.stats.MAtk
	z.handle(leaveEvent{id: 1, peer: ana.pl.peer})

	var saved *SavedState
	for len(saves) > 0 {
		for _, s := range <-saves {
			if s.ID == 1 {
				saved = &s
			}
		}
	}
	if saved == nil || saved.Spec != "occultist" || !containsStr(saved.Quests.Done, game.SpecQuest) {
		t.Fatalf("estado salvo: %+v", saved)
	}

	// Pela memória (estado recente) na mesma zona.
	z.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Ana", Class: "mage", XP: game.XPLevelStart(22), Peer: &fakePeer{}}})
	if again := z.players[1]; again.pl.spec != "occultist" || again.stats.MAtk != mAtk || again.pl.quests.has(game.SpecQuest) {
		t.Fatalf("relogar perdeu a especialização: %q (M.Atq %.1f, want %.1f)", again.pl.spec, again.stats.MAtk, mAtk)
	}

	// Pelo banco, em outra zona; caminho de outra classe é descartado.
	z2 := newTestZone(nil)
	z2.handle(joinEvent{info: JoinInfo{ID: 5, Name: "Bia", Class: "mage", XP: game.XPLevelStart(22), Spec: "occultist", Peer: &fakePeer{}}})
	z2.handle(joinEvent{info: JoinInfo{ID: 6, Name: "Caio", Class: "rogue", XP: game.XPLevelStart(22), Spec: "occultist", Peer: &fakePeer{}}})
	if z2.players[5].pl.spec != "occultist" || z2.players[6].pl.spec != "" {
		t.Fatalf("specs do banco: %q %q", z2.players[5].pl.spec, z2.players[6].pl.spec)
	}
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestChangeSpecCostsGoldAndRefundsPoints(t *testing.T) {
	z := newCombatZone()
	p, _ := nearMaster(z, 1, "rogue", 28)
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "cutthroat")
	learn(z, p, map[uint32]int{game.SkillCutBlades: 3, game.SkillCutInstinct: 1})
	slot := "skill:" + strconv.Itoa(int(game.SkillCutBlades))
	p.pl.hotbar[4] = slot
	free := freeSkillPoints(p)

	p.pl.inv.gold = game.EvolutionChangeCost(p.level) - 1
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "hunter")
	if p.pl.spec != "cutthroat" || p.pl.inv.gold != game.EvolutionChangeCost(p.level)-1 {
		t.Fatal("trocou de caminho sem moedas")
	}

	p.pl.inv.gold = game.EvolutionChangeCost(p.level) + 10
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "hunter")
	if p.pl.spec != "hunter" || p.pl.inv.gold != 10 {
		t.Fatalf("troca: spec %q, moedas %d", p.pl.spec, p.pl.inv.gold)
	}
	if p.pl.rank(game.SkillCutBlades) != 0 || p.pl.rank(game.SkillCutInstinct) != 0 || freeSkillPoints(p) != free+4 {
		t.Fatalf("pontos do caminho antigo não voltaram: %v (livres %d)", p.pl.ranks, freeSkillPoints(p))
	}
	if p.pl.hotbar[4] != "" {
		t.Errorf("a skill do caminho antigo ficou na barra: %v", p.pl.hotbar)
	}
	// O bônus mudou de +12 Destreza para +6 Força e +6 Destreza.
	total := game.ClassBaseAttrs["rogue"].Add(game.SpecBonus("rogue", p.pl.spec))
	if total.Str != game.ClassBaseAttrs["rogue"].Str+6 {
		t.Errorf("bônus do Caçador: %+v", total)
	}
}

// Cada skill ativa dos caminhos faz o que diz (golpe ou efeito) com as mecânicas de sempre.
func TestSpecActiveSkillsWork(t *testing.T) {
	for _, sp := range game.SpecList {
		for _, s := range game.EvolutionTree(sp.ID) {
			// Modificadores, as que agem só nos aliados e as de suporte têm testes próprios
			// (evo_knight_test.go).
			if s.IsPassive() || s.Node == game.NodeModifier || s.AllyOnly || s.RenewBlessings || s.CleanseAllies != (game.Scale{}) || s.Zone != nil {
				continue
			}
			t.Run(s.Name, func(t *testing.T) {
				z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0), fixed("skeleton_warrior", 20, 34, 0))
				ms := dummies(z, 32, 34)
				p, _ := joinAt(z, 1, sp.Class, 30, 0, game.XPLevelStart(28))
				p.pl.spec = sp.ID
				learn(z, p, map[uint32]int{s.ID: 1})
				p.pl.res.Value = 100 // as de impacto gastam o recurso da evolução
				var target *entity
				if s.Target == game.TargetEnemy {
					target = ms[0]
				}
				n := len(z.frame.combat)
				hitOrEffect := func() bool {
					if len(hitsSince(z, p, n)) > 0 {
						return true
					}
					for _, m := range ms {
						if len(m.effects) > 0 {
							return true
						}
					}
					return len(p.effects) > 0
				}
				if !castUntil(z, p, s.ID, target, hitOrEffect) {
					t.Fatalf("%s não fez nada", s.Name)
				}
				if s.Splash > 0 || s.Target == game.TargetAreaSelf {
					if hit := hitsSince(z, p, n); s.Mult.At(1) > 0 && hit[ms[1].id] == 0 {
						t.Errorf("%s: a área não acertou o segundo alvo: %v", s.Name, hit)
					}
				}
				if s.Banner != nil && len(z.objects) == 0 {
					t.Errorf("%s: sem estandarte", s.Name)
				}
			})
		}
	}
}

// heraldWithoutShield: durante o teste, o Arauto não usa escudo (nenhum caminho restringe
// armas ainda; as evoluções vão restringir).
func heraldWithoutShield(t *testing.T) {
	t.Helper()
	h := game.Specs["herald"]
	saved := h.Offhands
	t.Cleanup(func() { h.Offhands = saved })
	h.Offhands = []string{game.OffhandGrimoire}
}

func TestChooseSpecMovesGearThePathCannotUse(t *testing.T) {
	heraldWithoutShield(t)
	z := newCombatZone()
	p, _ := nearMaster(z, 1, "knight", 20)
	p.pl.inv.equip[game.SlotOffhand] = stack{def: item("shield_badge"), count: 1}
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_TALK, "")
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "herald")
	if p.pl.spec != "herald" {
		t.Fatalf("spec = %q", p.pl.spec)
	}
	if !p.pl.inv.equip[game.SlotOffhand].empty() {
		t.Fatal("o escudo deveria ir para a bolsa")
	}
	if p.pl.inv.countOf("shield_badge") != 1 {
		t.Fatal("o escudo sumiu")
	}
}

func TestChooseSpecRefusedWhenBagCannotHoldGear(t *testing.T) {
	heraldWithoutShield(t)
	z := newCombatZone()
	p, peer := nearMaster(z, 1, "knight", 20)
	p.pl.inv.equip[game.SlotOffhand] = stack{def: item("shield_badge"), count: 1}
	fillBag(&p.pl.inv, item("potion_hp_s"))
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_TALK, "")
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "herald")
	if p.pl.spec != "" || p.pl.inv.equip[game.SlotOffhand].empty() || !peer.systemContains("bolsa está cheia") {
		t.Fatalf("escolheu sem espaço para o escudo: spec %q", p.pl.spec)
	}
}

// Evoluir: a árvore base some (os pontos voltam), entram as 2 iniciais da evolução nas
// teclas das iniciais da base, e as outras skills da base saem da barra. Trocar de
// evolução cobra pelo nível e passa as teclas para as iniciais da nova.
func TestEvolveReplacesTheBaseTree(t *testing.T) {
	z := newCombatZone()
	p, peer := nearMaster(z, 1, "rogue", 25)
	p.pl.quests.done[game.SpecQuest] = true
	learn(z, p, map[uint32]int{game.SkillWeakSpot: 1, game.SkillTwinBlades: 1})
	p.pl.hotbar[0] = skillEntry(game.SkillShadowStep)
	p.pl.hotbar[1] = skillEntry(game.SkillBackstab)
	p.pl.hotbar[2] = skillEntry(game.SkillTwinBlades)
	p.pl.cooldowns[game.SkillBackstab] = z.now + 5

	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "cutthroat")
	if p.pl.spec != "cutthroat" || !maps.Equal(p.pl.ranks, game.StarterRanks("rogue", "cutthroat")) {
		t.Fatalf("evoluir: spec %q ranks %v", p.pl.spec, p.pl.ranks)
	}
	if p.pl.hotbar[0] != skillEntry(game.SkillCutStep) || p.pl.hotbar[1] != skillEntry(game.SkillCutBackstab) || p.pl.hotbar[2] != "" {
		t.Errorf("barra depois de evoluir: %v", p.pl.hotbar)
	}
	if _, ok := p.pl.cooldowns[game.SkillBackstab]; ok {
		t.Error("recarga da base ficou")
	}
	if freeSkillPoints(p) != game.SkillPointsTotal(25, true)-2 || !peer.systemContains("para distribuir") {
		t.Errorf("pontos livres %d", freeSkillPoints(p))
	}

	// Trocar: 2.500 × nível, e as iniciais trocam de tecla.
	cost := game.EvolutionChangeCost(25)
	p.pl.inv.gold = cost
	learn(z, p, map[uint32]int{game.SkillCutBlades: 2})
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "hunter")
	if p.pl.spec != "hunter" || p.pl.inv.gold != 0 || !maps.Equal(p.pl.ranks, game.StarterRanks("rogue", "hunter")) {
		t.Fatalf("trocar: spec %q ouro %d ranks %v", p.pl.spec, p.pl.inv.gold, p.pl.ranks)
	}
	if p.pl.hotbar[0] != skillEntry(game.SkillHuntKnife) || p.pl.hotbar[1] != skillEntry(game.SkillHuntShot) {
		t.Errorf("barra depois de trocar: %v", p.pl.hotbar)
	}
	if c := peer.lastSpecChoice(); c == nil || c.ChangeCost != uint64(cost) || c.RespecCost != uint64(game.RespecCost(25)) {
		t.Errorf("janela da Ysolde: %+v", c)
	}
}

// Redistribuir: paga 500 × nível e volta às iniciais da evolução; sem evolução, nada.
func TestRespecTree(t *testing.T) {
	z := newCombatZone()
	p, peer := nearMaster(z, 1, "mage", 22)
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_RESPEC, "")
	if !peer.systemContains("Primeiro escolha") {
		t.Error("redistribuiu sem evolução")
	}
	p.pl.spec = "elementalist"
	p.pl.ranks = game.StarterRanks("mage", "elementalist")
	learn(z, p, map[uint32]int{game.SkillInferno: 3, game.SkillElemLance: 2})
	if p.pl.rank(game.SkillInferno) != 3 {
		t.Fatalf("não aprendeu: %v", p.pl.ranks)
	}
	cost := game.RespecCost(22)
	p.pl.inv.gold = cost - 1
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_RESPEC, "")
	if p.pl.rank(game.SkillInferno) != 3 || !peer.systemContains("custa") {
		t.Fatal("redistribuiu sem ouro")
	}
	p.pl.inv.gold = cost
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_RESPEC, "")
	if p.pl.inv.gold != 0 || !maps.Equal(p.pl.ranks, game.StarterRanks("mage", "elementalist")) || p.pl.spec != "elementalist" {
		t.Fatalf("redistribuir: ouro %d ranks %v", p.pl.inv.gold, p.pl.ranks)
	}
}

// Save de antes das evoluções: um Sicário nível 25 com a árvore base, o degrau 20 e as skills
// do caminho. Fica só o que é da árvore do Assassino, entram as iniciais dela e as teclas das
// iniciais da base passam para as evoluídas.
func TestLoadOldSaveIntoEvolutionTree(t *testing.T) {
	old := map[uint32]int{
		game.SkillBackstab: 3, game.SkillShadowStep: 1, game.SkillWeakSpot: 5, game.SkillTwinBlades: 3,
		game.SkillFatalBlow: 2, game.SkillCutBlades: 3, game.SkillCutInstinct: 2,
	}
	bar := []string{skillEntry(game.SkillBackstab), skillEntry(game.SkillShadowStep), skillEntry(game.SkillTwinBlades), skillEntry(game.SkillFatalBlow), "item:potion_hp_s"}
	_, ranks, hotbar := loadProgress("rogue", "cutthroat", 25, game.Attrs{}, old, bar)
	want := map[uint32]int{game.SkillCutBackstab: 1, game.SkillCutStep: 1, game.SkillFatalBlow: 2, game.SkillCutBlades: 3, game.SkillCutInstinct: 2}
	if !maps.Equal(ranks, want) {
		t.Errorf("ranks: %v, want %v", ranks, want)
	}
	if hotbar[0] != skillEntry(game.SkillCutBackstab) || hotbar[1] != skillEntry(game.SkillCutStep) || hotbar[2] != "" || hotbar[3] != skillEntry(game.SkillFatalBlow) || hotbar[4] != "item:potion_hp_s" {
		t.Errorf("barra: %v", hotbar)
	}
	// Sem evolução no 25: o degrau 20 de antes sai e fica a árvore base.
	_, ranks, _ = loadProgress("rogue", "", 25, game.Attrs{}, old, bar)
	if ranks[game.SkillFatalBlow] != 0 || ranks[game.SkillCutBlades] != 0 || ranks[game.SkillWeakSpot] != 5 {
		t.Errorf("sem evolução: %v", ranks)
	}
	// Evolução vinda do banco abaixo do 20 não vale.
	if game.ActiveSpec("rogue", "cutthroat", 19) != "" || game.ActiveSpec("rogue", "bastion", 30) != "" || game.ActiveSpec("rogue", "hunter", 20) != "hunter" {
		t.Error("ActiveSpec")
	}
}

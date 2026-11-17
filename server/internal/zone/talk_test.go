package zone

import (
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

func (f *fakePeer) lastDialog() *pb.NpcDialog {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if d := f.msgs[i].GetNpcDialog(); d != nil {
			return d
		}
	}
	return nil
}

func questOf(p *entity, id string) *activeQuest {
	for _, a := range p.pl.quests.active {
		if a.def.ID == id {
			return a
		}
	}
	return nil
}

func npcNamed(z *Zone, name string) *entity {
	for _, n := range z.npcs {
		if n.name == name {
			return n
		}
	}
	return nil
}

// nearNPC: o personagem no nível level, a 1,5 m do NPC.
func nearNPC(t *testing.T, z *Zone, id uint32, name string, level int) (*entity, *fakePeer, *entity) {
	t.Helper()
	n := npcNamed(z, name)
	if n == nil {
		t.Fatalf("sem NPC %s", name)
	}
	p, peer := joinAt(z, id, "knight", n.pos.X+1.5, n.pos.Z, game.XPLevelStart(level))
	return p, peer, n
}

func npcTalk(z *Zone, p *entity, n *entity, kind pb.NpcTalkKind, quest, topic string) {
	p.pl.moveTokens = 10 // sem o limite de ações por segundo
	z.onProgress(progressEvent{id: p.id, msg: &pb.NpcTalk{NpcId: n.id, Kind: kind, QuestId: quest, Topic: topic}})
}

func option(d *pb.NpcDialog, kind pb.DialogOptionKind, quest string) *pb.DialogOption {
	if d == nil {
		return nil
	}
	for _, o := range d.Options {
		if o.Kind == kind && (quest == "" || o.QuestId == quest) {
			return o
		}
	}
	return nil
}

// Os ids dos NPCs são únicos (o mapa da zona engoliria o repetido) e nenhum fica a menos de
// 3 m de outro (o clique pegava um pelo outro: o Quadro e o Leiloeiro já se sobrepuseram).
func TestNPCIDsAreUniqueAndApart(t *testing.T) {
	cfg, err := MapConfig(testConfig(), world.StarterMap, []string{world.StarterMap, MapTest})
	if err != nil {
		t.Fatal(err)
	}
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil) // addNPC para a partida com id repetido
	want := len(townNPCs) + len(teleportStones) + 5 /* armazém, Mestra, Quadro, Leiloeiro, Guardião */ +
		len(game.SpawnedTalkers()) + len(cfg.Gates)
	if len(z.npcs) != want {
		t.Fatalf("%d NPCs na zona, quero %d (algum id repetido?)", len(z.npcs), want)
	}
	if lastNPCID() >= gateIDBase {
		t.Fatalf("os NPCs passaram da faixa dos portões: %d", lastNPCID())
	}
	ids := slices.Sorted(func(yield func(uint32) bool) {
		for id := range z.npcs {
			if !yield(id) {
				return
			}
		}
	})
	for i, a := range ids {
		for _, b := range ids[i+1:] {
			na, nb := z.npcs[a], z.npcs[b]
			if d := dist(na.pos, nb.pos); d < 3 {
				t.Errorf("%s (%d) e %s (%d) a %.1f m", na.name, a, nb.name, b, d)
			}
		}
	}
	// Os ids fixos são seguidos (do primeiro da praça ao último contador de história).
	for id := npcIDBase; id <= lastNPCID(); id++ {
		if z.npcs[id] == nil {
			t.Errorf("buraco nos ids de NPC: %d", id)
		}
	}
	for _, tk := range game.SpawnedTalkers() {
		n := npcNamed(z, tk.Name)
		if n == nil || n.npc.talk != tk || n.npc.role != pb.NpcRole_NPC_ROLE_QUEST {
			t.Errorf("%s não está na zona do mapa dele", tk.Name)
		}
	}
	for _, name := range []string{"Lívia", "Bruno", "Teodoro", "Otto", "Iselda", "Severino"} {
		if n := npcNamed(z, name); n == nil || n.npc.talk == nil {
			t.Errorf("%s sem falas", name)
		}
	}
	// Num mapa sem cidade não há contador de história (cada um fica na zona da região dele).
	test, err := MapConfig(testConfig(), MapTest, []string{world.StarterMap, MapTest})
	if err != nil {
		t.Fatal(err)
	}
	if tz := New(test, slog.New(slog.NewTextHandler(io.Discard, nil)), nil); npcNamed(tz, game.TalkMiller) != nil {
		t.Fatal("Anselmo apareceu na Clareira de Testes")
	}
}

// A linha dos Arredores inteira pela conversa: o convite anda falando com Anselmo, cada
// missão é aceita e entregue a ele, a entrega leva os itens da bolsa e paga uma vez só.
func TestArredoresStoryLineThroughDialog(t *testing.T) {
	z := newQuestZone()
	z.SetEventSink(nopSink{})
	p, peer, n := nearNPC(t, z, 1, game.TalkMiller, 3)
	p.pl.quests.done[game.TutorialStart] = true
	if !p.pl.quests.has("arr_convite") {
		t.Fatalf("o convite chega no nível 3: %v", activeIDs(p))
	}
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	d := peer.lastDialog()
	if !p.pl.quests.done["arr_convite"] || option(d, pb.DialogOptionKind_DIALOG_OPTION_ACCEPT, "arr_trigo") == nil {
		t.Fatalf("falar com Anselmo: convite %v, conversa %+v", p.pl.quests.done["arr_convite"], d)
	}
	if option(d, pb.DialogOptionKind_DIALOG_OPTION_ASK, "") == nil || option(d, pb.DialogOptionKind_DIALOG_OPTION_LEAVE, "") == nil ||
		option(d, pb.DialogOptionKind_DIALOG_OPTION_ACCEPT, "arr_fogueira") != nil {
		t.Fatalf("opções: %+v", d.Options)
	}
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_ACCEPT, "arr_trigo", "")
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_ACCEPT, "arr_trigo", "") // duas vezes: uma só
	if c := strings.Count(strings.Join(activeIDs(p), ","), "arr_trigo"); c != 1 {
		t.Fatalf("aceitar duas vezes: %v", activeIDs(p))
	}
	for range 6 {
		z.questEvent(p, game.GoalKill, "skeleton_minion")
	}
	a := questOf(p, "arr_trigo")
	if a == nil || !a.complete() {
		t.Fatal("com os passos cumpridos a missão espera a entrega")
	}
	log := peer.lastQuestLog()
	last := log.Quests[slices.IndexFunc(log.Quests, func(q *pb.QuestInfo) bool { return q.Id == "arr_trigo" })]
	if s := last.Steps[len(last.Steps)-1]; s.Place != pb.QuestPlaceKind_QUEST_PLACE_KIND_NPC || s.PlaceName != game.TalkMiller || s.Count != 0 ||
		last.Region != "Arredores" || last.Category != pb.QuestCategory_QUEST_CATEGORY_REGION {
		t.Fatalf("passo de voltar a Anselmo: %+v (%+v)", s, last)
	}
	gold := p.pl.inv.gold
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_TURN_IN, "arr_trigo", "")
	if !p.pl.quests.done["arr_trigo"] || p.pl.inv.gold != gold+game.Quests["arr_trigo"].Reward.Gold {
		t.Fatalf("entrega: done %v, moedas %d → %d", p.pl.quests.done["arr_trigo"], gold, p.pl.inv.gold)
	}
	if d := peer.lastDialog(); d.Text != game.Quests["arr_trigo"].Done {
		t.Fatalf("fala da entrega: %q", d.Text)
	}
	gold = p.pl.inv.gold
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_TURN_IN, "arr_trigo", "") // entregar de novo não paga
	if p.pl.inv.gold != gold {
		t.Fatal("a segunda entrega pagou de novo")
	}

	// A próxima pede o nível 4.
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_ACCEPT, "arr_fogueira", "")
	if p.pl.quests.has("arr_fogueira") {
		t.Fatal("aceitou abaixo do nível")
	}
	z.grantXP(p, game.XPLevelStart(4)-p.pl.xp)
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_ACCEPT, "arr_fogueira", "")
	if !p.pl.quests.has("arr_fogueira") {
		t.Fatal("no nível 4 a fogueira está disponível")
	}
	// Entregar sem os ossos: nada. Com eles: saem da bolsa e a missão termina.
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_TURN_IN, "arr_fogueira", "")
	if p.pl.quests.done["arr_fogueira"] {
		t.Fatal("entregou sem os ossos")
	}
	p.pl.inv.add(game.Items["bone_old"], 10)
	drainEvents(z)
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	if option(peer.lastDialog(), pb.DialogOptionKind_DIALOG_OPTION_TURN_IN, "arr_fogueira") == nil {
		t.Fatalf("com os ossos na bolsa a entrega aparece: %+v", peer.lastDialog().Options)
	}
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_TURN_IN, "arr_fogueira", "")
	if !p.pl.quests.done["arr_fogueira"] || p.pl.inv.countOf("bone_old") != 2 {
		t.Fatalf("entrega dos ossos: done %v, na bolsa %d", p.pl.quests.done["arr_fogueira"], p.pl.inv.countOf("bone_old"))
	}
	evs := drainEvents(z)
	if e := findEvent(evs, "item_out", "bone_old"); e == nil || e.Amount != 8 || e.Extra["how"] != "quest_deliver" {
		t.Fatalf("a entrega precisa de evento de saída: %+v", e)
	}
	if e := findEvent(evs, "item_in", "potion_mp_s"); e == nil || e.Extra["how"] != "quest" {
		t.Fatalf("a recompensa precisa de evento de entrada: %+v", e)
	}
	if findEvent(evs, "gold", "quest") == nil || findEvent(evs, "quest_done", "arr_fogueira") == nil {
		t.Fatal("moedas e conclusão anotadas")
	}

	// A última: falar com Cássio, na praça (com a fala dele), e voltar a Anselmo.
	z.grantXP(p, game.XPLevelStart(5)-p.pl.xp)
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_ACCEPT, "arr_runas", "")
	herald := npcNamed(z, game.TalkHerald)
	npcTalk(z, p, herald, pb.NpcTalkKind_NPC_TALK_OPEN, "", "") // longe: nada
	if questOf(p, "arr_runas").progress[0] != 0 {
		t.Fatal("conversa de longe contou")
	}
	p.teleportTo(Vec2{X: herald.pos.X + 1, Z: herald.pos.Z})
	npcTalk(z, p, herald, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	if questOf(p, "arr_runas").progress[0] != 1 || peer.lastDialog().Text != game.Quests["arr_runas"].Steps[0].Say {
		t.Fatalf("falar com Cássio: %+v", peer.lastDialog())
	}
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_TURN_IN, "arr_runas", "") // Anselmo está longe
	if p.pl.quests.done["arr_runas"] {
		t.Fatal("entregou de longe")
	}
	p.teleportTo(Vec2{X: n.pos.X + 1, Z: n.pos.Z})
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_TURN_IN, "arr_runas", "")
	if !p.pl.quests.done["arr_runas"] {
		t.Fatal("a linha dos Arredores não terminou")
	}
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	if d := peer.lastDialog(); d.Text != n.npc.talk.Idle || option(d, pb.DialogOptionKind_DIALOG_OPTION_ACCEPT, "") != nil {
		t.Fatalf("linha terminada: %+v", d)
	}
}

// O servidor recusa o que não vale: missão de outro NPC, que não existe, de nível alto,
// entregar sem cumprir, perguntar o que ele não sabe e NPC sem conversa.
func TestDialogRejectsWhatIsNotAllowed(t *testing.T) {
	z := newQuestZone()
	p, peer, n := nearNPC(t, z, 1, game.TalkDigger, 7)
	for _, id := range []string{"arr_trigo", "nao_existe", "", "cam_nomes", "d_ossos", game.ContractID(9)} {
		npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_ACCEPT, id, "")
		if id != "" && p.pl.quests.has(id) {
			t.Errorf("aceitou %q com Marga", id)
		}
	}
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_ACCEPT, "cam_covas", "")
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_TURN_IN, "cam_covas", "")
	if p.pl.quests.done["cam_covas"] || !p.pl.quests.has("cam_covas") {
		t.Fatal("entregou sem cumprir")
	}
	before := len(peer.msgs)
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_ASK, "", "nao_existe")
	npcTalk(z, p, n, pb.NpcTalkKind(99), "", "")
	stone := npcNamed(z, "Cidade")
	npcTalk(z, p, stone, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	if len(peer.msgs) != before {
		t.Fatalf("respostas a pedidos inválidos: %d", len(peer.msgs)-before)
	}
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_ASK, "", "regiao")
	if peer.lastDialog().Text != n.npc.talk.Topic("regiao").Text {
		t.Fatal("perguntar sobre a região")
	}
	p.dead = true
	before = len(peer.msgs)
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	if len(peer.msgs) != before {
		t.Fatal("morto não conversa")
	}
}

// NPC de serviço sem missão para o jogador abre o serviço direto; com uma entrega para ele,
// abre a conversa (com o serviço entre as opções).
func TestServiceNPCOpensServiceUnlessThereIsQuestBusiness(t *testing.T) {
	z := newQuestZone()
	p, peer, smith := nearNPC(t, z, 1, game.TalkSmith, 10)
	npcTalk(z, p, smith, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	if d := peer.lastDialog(); d == nil || !d.Service || len(d.Options) != 0 {
		t.Fatalf("sem missão: %+v", d)
	}
	p.pl.quests.daily = game.DailyState{Day: game.ContractDay(z.cfg.Clock(), z.eventLoc()), Offer: []string{"d_cola"}}
	p.pl.quests.active = append(p.pl.quests.active, &activeQuest{def: game.Quests["d_cola"], progress: []int{0}})
	p.pl.inv.add(game.Items["bone_old"], 10)
	npcTalk(z, p, smith, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	d := peer.lastDialog()
	if d.Service || option(d, pb.DialogOptionKind_DIALOG_OPTION_TURN_IN, "d_cola") == nil || option(d, pb.DialogOptionKind_DIALOG_OPTION_SERVICE, "") == nil {
		t.Fatalf("com a entrega: %+v", d)
	}
	npcTalk(z, p, smith, pb.NpcTalkKind_NPC_TALK_TURN_IN, "d_cola", "")
	if p.pl.quests.has("d_cola") || !p.pl.quests.daily.Delivered("d_cola") || p.pl.inv.countOf("bone_old") != 0 {
		t.Fatalf("a diária de entrega termina na entrega: %v %+v", activeIDs(p), p.pl.quests.daily)
	}
}

// As diárias: o Arauto oferece as 3 do rol de hoje; cada uma é aceita e entregue uma vez por
// dia; à meia-noite de Brasília o rol vira e a diária de ontem sai.
func TestDailiesRenewAtBrasiliaMidnight(t *testing.T) {
	now := time.Date(2026, 11, 16, 23, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	cfg := testConfig()
	cfg.Clock = func() time.Time { return now }
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	p, peer, herald := nearNPC(t, z, 7, game.TalkHerald, 12)
	npcTalk(z, p, herald, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	d := peer.lastDialog()
	today := game.DailyQuests(p.id, game.ContractDay(now, z.eventLoc()), 12)
	if len(today) != 3 {
		t.Fatalf("rol de hoje: %v", today)
	}
	for _, q := range today {
		if option(d, pb.DialogOptionKind_DIALOG_OPTION_ACCEPT, q.ID) == nil {
			t.Fatalf("o Arauto não oferece %s: %+v", q.ID, d.Options)
		}
	}
	if log := peer.lastQuestLog(); log.DailyResetMs != time.Date(2026, 11, 17, 0, 0, 0, 0, z.eventLoc()).UnixMilli() {
		t.Fatalf("renovação = %d", log.DailyResetMs)
	}
	visitQ := today[2]
	npcTalk(z, p, herald, pb.NpcTalkKind_NPC_TALK_ACCEPT, visitQ.ID, "")
	if !p.pl.quests.has(visitQ.ID) {
		t.Fatal("não aceitou a diária")
	}
	// Termina a diária de ronda (ou de chefe) com os acontecimentos dela.
	for i, s := range visitQ.Steps {
		a := questOf(p, visitQ.ID)
		for a != nil && a.progress[i] < s.Count {
			z.questEvent(p, s.Goal, strings.Split(s.Target, "|")[0])
			a = questOf(p, visitQ.ID)
		}
	}
	if p.pl.quests.has(visitQ.ID) || !p.pl.quests.daily.Delivered(visitQ.ID) || p.pl.quests.done[visitQ.ID] {
		t.Fatalf("a diária termina sozinha e fica só no rol de hoje: %v %+v", activeIDs(p), p.pl.quests.daily)
	}
	npcTalk(z, p, herald, pb.NpcTalkKind_NPC_TALK_ACCEPT, visitQ.ID, "")
	if p.pl.quests.has(visitQ.ID) {
		t.Fatal("a mesma diária duas vezes no dia")
	}
	collectQ := today[0]
	npcTalk(z, p, herald, pb.NpcTalkKind_NPC_TALK_ACCEPT, collectQ.ID, "")
	z.questCollect(p, &groundItem{def: game.Items[collectQ.Steps[0].Target], count: 1}) // não é saque: não conta
	z.questCollect(p, &groundItem{def: game.Items[collectQ.Steps[0].Target], count: 2, loot: true})
	if a := questOf(p, collectQ.ID); a == nil || a.progress[0] != 2 {
		t.Fatalf("coletar: %+v", a)
	}
	// Meia-noite: a coleta aceita ontem sai, e o rol é outro.
	now = now.Add(2 * time.Hour)
	z.questEvent(p, game.GoalSkill, "")
	if p.pl.quests.has(collectQ.ID) || p.pl.quests.daily.Delivered(visitQ.ID) || p.pl.quests.daily.Day != game.ContractDay(now, z.eventLoc()) {
		t.Fatalf("a virada não limpou: %v %+v", activeIDs(p), p.pl.quests.daily)
	}
	if got := p.pl.quests.daily.Offer; !slices.Equal(got, dailyIDs(game.DailyQuests(p.id, game.ContractDay(now, z.eventLoc()), 12))) {
		t.Fatalf("rol de amanhã: %v", got)
	}
}

func dailyIDs(qs []*game.QuestDef) []string {
	var out []string
	for _, q := range qs {
		out = append(out, q.ID)
	}
	return out
}

// Visitar: dentro do raio, o passo anda no 1×/s; fora, não.
func TestVisitStepCountsInsideTheRadius(t *testing.T) {
	z := newQuestZone()
	p, _, n := nearNPC(t, z, 1, game.TalkDigger, 10)
	p.pl.quests.done["cam_covas"], p.pl.quests.done["cam_nomes"] = true, true
	npcTalk(z, p, n, pb.NpcTalkKind_NPC_TALK_ACCEPT, "cam_estandarte", "")
	s := game.Quests["cam_estandarte"].Steps[0]
	p.teleportTo(Vec2{X: s.Place.X + s.Place.Radius + 1, Z: s.Place.Z})
	z.checkVisits()
	if questOf(p, "cam_estandarte").progress[0] != 0 {
		t.Fatal("contou de fora do raio")
	}
	p.teleportTo(Vec2{X: s.Place.X + 1, Z: s.Place.Z})
	z.checkVisits()
	if questOf(p, "cam_estandarte").progress[0] != 1 {
		t.Fatal("não contou dentro do raio")
	}
}

// O progresso das missões de NPC e o rol das diárias voltam inteiros do salvamento; uma
// diária que não é do rol (ou já entregue) não volta.
func TestQuestStateRoundTripWithDailies(t *testing.T) {
	day := 20400
	st := game.QuestState{
		Active: map[string][]int{"val_circulos": {1, 0}, "d_ossos": {7}, "d_cola": {0}},
		Done:   []string{"val_pedra", "d_frascos"},
		Daily:  &game.DailyState{Day: day, Offer: []string{"d_ossos", "d_cola", "d_ronda_arredores"}, Done: []string{"d_cola"}},
	}
	q := loadQuests(st, 23)
	if a := slices.IndexFunc(q.active, func(a *activeQuest) bool { return a.def.ID == "d_cola" }); a >= 0 {
		t.Fatal("diária já entregue voltou como ativa")
	}
	if q.done["d_frascos"] || !q.done["val_pedra"] {
		t.Fatalf("concluídas: %v", q.done)
	}
	back := q.state()
	if !slices.Equal(back.Active["val_circulos"], []int{1, 0}) || !slices.Equal(back.Active["d_ossos"], []int{7}) ||
		back.Daily == nil || back.Daily.Day != day || !slices.Equal(back.Daily.Done, []string{"d_cola"}) ||
		len(back.Daily.Offer) != 3 || fmt.Sprint(back.Done) != "[val_pedra]" {
		t.Fatalf("de volta: %+v %+v", back, back.Daily)
	}
}

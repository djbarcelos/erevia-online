package zone

import (
	"io"
	"log/slog"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// testAreaDef: uma versão curta de "O Bastião resiste" (as 3 etapas, números pequenos), no
// centro (30, 30). Não entra em game.AreaEvents: o teste chama startAreaEvent direto.
func testAreaDef() *game.EventDef {
	d := *game.AreaEventByID("bastiao_resiste")
	d.ID, d.X, d.Z, d.Radius, d.MinLevel = "teste_area", 30, 30, 20, 0
	d.Trigger.Warn = 10
	d.Stages = []game.StageDef{
		{Kind: game.StageKill, Label: "Matar", Limit: 60, Say: "etapa 1", Mobs: []string{"skeleton_minion"}, Count: 3, MaxAlive: 3, Every: 1},
		{Kind: game.StageDefend, Label: "Defender", Limit: 60, Say: "etapa 2", Mobs: []string{"skeleton_minion"}, MaxAlive: 2, Every: 2, Hold: 10, TargetHP: 100, DrainRate: 1},
		{Kind: game.StageBoss, Label: "Chefe", Limit: 60, Say: "etapa 3", Boss: "skeleton_warrior", BossHPMult: 2},
	}
	return &d
}

func newAreaZone(t *testing.T) *Zone {
	t.Helper()
	cfg := testConfig()
	cfg.Spawns = nil
	cfg.Size = 600
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func (f *fakePeer) areaStatuses() []*pb.AreaEventStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pb.AreaEventStatus
	for _, m := range f.msgs {
		if a := m.GetAreaEvent(); a != nil {
			out = append(out, a)
		}
	}
	return out
}

func (f *fakePeer) announceTitles() []string {
	var out []string
	for _, a := range f.announces() {
		out = append(out, a.Title)
	}
	return out
}

func (z *Zone) eventMobs() []*entity {
	var out []*entity
	for _, m := range z.monsters {
		if m.mon.evMob && !m.dead {
			out = append(out, m)
		}
	}
	return out
}

// O evento de teste roda as 3 etapas na ordem (matar, defender o estandarte, chefe), no relógio da
// zona, e termina em vitória sem derrubar o posto.
func TestAreaEventRunsThreeStages(t *testing.T) {
	z := newAreaZone(t)
	ana, peer := defender(z, 1, "Ana", 30, 30, 26, false)
	d := testAreaDef()
	if !z.startAreaEvent(d, "teste") || z.startAreaEvent(d, "teste") {
		t.Fatal("só um evento por vez")
	}
	if !z.ae.warning || len(z.eventMobs()) != 0 {
		t.Fatal("no aviso ainda não há monstros")
	}
	z.tickFor(9)
	if !z.ae.warning {
		t.Fatal("o ataque só começa depois do aviso")
	}
	z.tickFor(1.2)
	if z.ae.warning || z.ae.stage != 0 {
		t.Fatalf("etapa 1 deveria ter começado: %+v", z.ae)
	}

	seen := map[int]bool{}
	for range 20 * 120 {
		if z.ae == nil {
			break
		}
		seen[z.ae.stage] = true
		if n := len(z.eventMobs()); n > 3 {
			t.Fatalf("passou do limite de vivos: %d", n)
		}
		for _, m := range z.eventMobs() {
			z.damage(ana, m, m.hp+1, nil) // Ana mata tudo o que nasce
		}
		z.update(0.05)
	}
	if z.ae != nil {
		t.Fatalf("o evento deveria ter terminado: etapa %d", z.ae.stage)
	}
	if !seen[0] || !seen[1] || !seen[2] {
		t.Fatalf("etapas vistas: %v", seen)
	}
	if len(z.postUntil) != 0 {
		t.Fatal("em vitória o posto não cai")
	}
	titles := peer.announceTitles()
	for _, want := range []string{d.Name + ": etapa 1 de 3", d.Name + ": etapa 2 de 3", d.Name + ": etapa 3 de 3", d.Name + ": vitória!"} {
		if !slices.Contains(titles, want) {
			t.Errorf("faltou o anúncio %q em %q", want, titles)
		}
	}
	if len(z.objects) != 0 || len(z.eventMobs()) != 0 {
		t.Fatal("o estandarte e os monstros somem no fim")
	}
	// Ana fez todo o dano: faixa de ouro, 3 medalhas e a recompensa cheia.
	if got := ana.pl.inv.countOf(game.BastionMedal); got != 3 {
		t.Fatalf("medalhas de Ana = %d", got)
	}
	if st := peer.areaStatuses(); len(st) == 0 || st[len(st)-1].Active {
		t.Fatal("o painel precisa ser limpo no fim")
	}
}

// As faixas de contribuição: 100% (3 medalhas), 85% (2), 75% (1) e nada abaixo do mínimo.
func TestAreaEventMedalsByTier(t *testing.T) {
	z := newAreaZone(t)
	d := testAreaDef()
	level := 26
	ps := map[string]*entity{}
	for i, n := range []string{"ouro", "prata", "bronze", "nada", "presente"} {
		ps[n], _ = defender(z, uint32(i+1), n, 30, 30, level, false)
	}
	z.startAreaEvent(d, "teste")
	z.beginAreaEvent()
	e := z.ae
	e.pool = 1000
	e.dmg[ps["ouro"].id] = 110        // 10% ou mais
	e.dmg[ps["prata"].id] = 60        // 5% ou mais
	e.dmg[ps["bronze"].id] = 20       // 1,5% ou mais
	e.dmg[ps["nada"].id] = 5          // abaixo do mínimo
	e.present[ps["presente"].id] = 20 // só presença: 20 s x 6 = 120
	goldBefore := ps["prata"].pl.inv.gold
	z.endAreaEvent(true, "")

	want := map[string]int{"ouro": 3, "prata": 2, "bronze": 1, "nada": 0, "presente": 3}
	for n, medals := range want {
		if got := ps[n].pl.inv.countOf(game.BastionMedal); got != medals {
			t.Errorf("%s: %d medalhas, esperava %d", n, got, medals)
		}
	}
	full := int64(math.Round(d.Reward.GoldBase + d.Reward.GoldPerLvl*float64(level)))
	if g := ps["ouro"].pl.inv.gold - goldBefore; g != full {
		t.Errorf("ouro: %d moedas, esperava %d", g, full)
	}
	if g := ps["prata"].pl.inv.gold - goldBefore; g != int64(math.Round(float64(full)*0.85)) {
		t.Errorf("prata: %d moedas", g)
	}
	if g := ps["bronze"].pl.inv.gold - goldBefore; g != int64(math.Round(float64(full)*0.75)) {
		t.Errorf("bronze: %d moedas", g)
	}
	if g := ps["nada"].pl.inv.gold - goldBefore; g != 0 {
		t.Errorf("abaixo do mínimo recebeu %d moedas", g)
	}
}

// Em falha o posto cai: a pedra, o mercador e o Irmão recusam por 15 min e depois reabrem.
func TestAreaEventFailureClosesPostAndReopens(t *testing.T) {
	z := newAreaZone(t)
	stone := npcNamed(z, "Bastião Cinzento")
	odric := npcNamed(z, "Odric")
	caedmon := npcNamed(z, game.TalkBrother)
	if stone == nil || odric == nil || caedmon == nil {
		t.Fatalf("o posto precisa da pedra, do mercador e do Irmão: %v %v %v", stone, odric, caedmon)
	}
	p, peer := joinAt(z, 1, "knight", odric.pos.X+1, odric.pos.Z, game.XPLevelStart(26))
	p.pl.inv.gold = 5000
	p.pl.disc.known["bastiao"] = true
	d := testAreaDef()
	d.Trigger.Warn = 0
	z.startAreaEvent(d, "teste")
	z.ae.dmg[p.id] = 400
	z.ae.pool = 1000 // o jogador está na faixa de ouro
	z.endAreaEvent(false, "")
	if !z.postClosed("bastiao") {
		t.Fatal("o posto deveria ter caído")
	}
	if p.pl.inv.countOf(game.BastionMedal) != 0 {
		t.Fatal("falha não dá medalha")
	}
	if !slices.Contains(peer.announceTitles(), d.Name+": o posto caiu") {
		t.Fatalf("anúncio da queda: %v", peer.announceTitles())
	}
	st := peer.areaStatuses()
	last := st[len(st)-1]
	if len(last.Closed) != 1 || last.Closed[0].Key != "bastiao" || last.Closed[0].StoneId == 0 || len(last.Closed[0].NpcIds) < 2 {
		t.Fatalf("painel dos postos fechados: %+v", last)
	}

	// A loja recusa.
	gold := p.pl.inv.gold
	peer.reset()
	z.buy(p, odric.id, "potion_hp_s", 1)
	if p.pl.inv.gold != gold || !peer.systemContains("caiu") {
		t.Fatal("a loja do posto fechado precisa recusar com a mensagem em pt-BR")
	}
	// A pedra recusa (de dentro do posto e como destino).
	p.teleportTo(Vec2{X: stone.pos.X + 1, Z: stone.pos.Z})
	peer.reset()
	z.startTeleport(p, stone.id, uint32(stoneIndex("cidade")+1))
	if p.pl.channel != nil || !peer.systemContains("caiu") {
		t.Fatal("a pedra do posto caído não pode viajar")
	}
	town := npcNamed(z, "Cidade")
	p.teleportTo(Vec2{X: town.pos.X + 1, Z: town.pos.Z})
	peer.reset()
	z.startTeleport(p, town.id, uint32(stoneIndex("bastiao")+1))
	if p.pl.channel != nil || !peer.systemContains("caiu") {
		t.Fatal("viajar para a pedra do posto caído também é recusado")
	}
	// O Irmão não conversa.
	p.teleportTo(Vec2{X: caedmon.pos.X + 1, Z: caedmon.pos.Z})
	peer.reset()
	npcTalk(z, p, caedmon, pb.NpcTalkKind_NPC_TALK_OPEN, "", "")
	if !peer.systemContains("caiu") {
		t.Fatal("o Irmão do posto caído não atende")
	}

	// Aos 15 min o posto reabre e avisa.
	z.tickFor(14 * 60)
	if !z.postClosed("bastiao") {
		t.Fatal("antes dos 15 min o posto continua fechado")
	}
	z.tickFor(61)
	if z.postClosed("bastiao") || len(z.postUntil) != 0 {
		t.Fatal("o posto deveria ter reaberto")
	}
	if !slices.Contains(peer.announceTitles(), "Bastião Cinzento reabriu") {
		t.Fatalf("anúncio da reabertura: %v", peer.announceTitles())
	}
	p.teleportTo(Vec2{X: odric.pos.X + 1, Z: odric.pos.Z})
	z.buy(p, odric.id, "potion_hp_s", 1)
	if p.pl.inv.gold >= gold {
		t.Fatal("a loja reaberta vende")
	}
}

// Em falha a recompensa é metade das moedas e nada de medalha.
func TestAreaEventFailurePaysHalf(t *testing.T) {
	z := newAreaZone(t)
	p, _ := defender(z, 1, "Ana", 30, 30, 26, false)
	d := testAreaDef()
	z.startAreaEvent(d, "teste")
	z.beginAreaEvent()
	z.ae.pool = 1000
	z.ae.dmg[p.id] = 500
	before := p.pl.inv.gold
	z.endAreaEvent(false, "")
	want := int64(math.Round((d.Reward.GoldBase + d.Reward.GoldPerLvl*26) * d.Reward.FailMult))
	if got := p.pl.inv.gold - before; got != want {
		t.Fatalf("falha pagou %d moedas, esperava %d", got, want)
	}
}

// Estourar o tempo da etapa (ninguém defende) derruba o posto; o estandarte cair também.
func TestAreaEventFailsOnTimeoutAndOnBannerLoss(t *testing.T) {
	z := newAreaZone(t)
	defender(z, 1, "Ana", 150, 150, 26, false) // longe do círculo: não mata ninguém
	d := testAreaDef()
	d.Trigger.Warn = 0
	d.Stages[0].MaxAlive = 1 // poucos monstros para o tempo estourar antes de eles ganharem
	d.Stages[0].Count = 1000
	z.startAreaEvent(d, "teste")
	z.tickFor(d.Stages[0].Limit + 1)
	if z.ae != nil || !z.postClosed("bastiao") {
		t.Fatal("o tempo da etapa 1 acabou: o evento deveria ter falhado e o posto caído")
	}

	// Estandarte: os monstros chegam nele e o ferem até cair.
	z2 := newAreaZone(t)
	defender(z2, 1, "Ana", 150, 150, 26, false)
	d2 := testAreaDef()
	d2.Trigger.Warn = 0
	d2.Stages = d2.Stages[1:2]
	d2.Stages[0].Limit, d2.Stages[0].Hold, d2.Stages[0].DrainRate, d2.Stages[0].TargetHP = 600, 500, 20, 40
	z2.startAreaEvent(d2, "teste")
	for range 20 * 120 {
		if z2.ae == nil {
			break
		}
		z2.update(0.05)
	}
	if z2.ae != nil || !z2.postClosed("bastiao") {
		t.Fatal("sem defensores o estandarte cai e o posto fecha")
	}
}

// O anúncio e o painel vão só para a zona do evento.
func TestAreaEventAnnouncesOnlyInItsZone(t *testing.T) {
	a, b := newAreaZone(t), newAreaZone(t)
	_, pa := defender(a, 1, "Ana", 30, 30, 26, false)
	_, pb2 := defender(b, 2, "Beto", 30, 30, 26, false)
	d := testAreaDef()
	a.startAreaEvent(d, "teste")
	a.tickFor(11)
	a.endAreaEvent(false, "")
	if len(pa.announces()) < 3 || len(pa.areaStatuses()) == 0 {
		t.Fatalf("a zona do evento recebe: %d anúncios, %d painéis", len(pa.announces()), len(pa.areaStatuses()))
	}
	if len(pb2.announces()) != 0 || len(pb2.areaStatuses()) != 0 {
		t.Fatal("outra zona não recebe nada do evento")
	}
	if b.postClosed("bastiao") {
		t.Fatal("o posto é da zona do evento")
	}
}

// Quem entra com o posto caído já vê o estado.
func TestAreaEventStatusOnJoin(t *testing.T) {
	z := newAreaZone(t)
	z.closePost("bastiao", 900)
	_, peer := defender(z, 1, "Ana", 30, 30, 26, false)
	st := peer.areaStatuses()
	if len(st) != 1 || len(st[0].Closed) != 1 || st[0].Active {
		t.Fatalf("estado ao entrar: %+v", st)
	}
}

// Os gatilhos: noite (chance e recarga), janela e horário fixo.
func TestAreaEventTriggers(t *testing.T) {
	old := game.AreaEvents
	t.Cleanup(func() { game.AreaEvents = old })
	base := testAreaDef()
	mk := func(tr game.TriggerDef) *game.EventDef {
		d := *base
		d.Trigger = tr
		return &d
	}
	night := mk(game.TriggerDef{Kind: game.TriggerNight, Chance: 1, Cooldown: 300})
	game.AreaEvents = []*game.EventDef{night}
	z := newAreaZone(t)
	z.areaEventNight()
	if z.ae != nil {
		t.Fatal("sem jogadores o evento não começa")
	}
	defender(z, 1, "Ana", 30, 30, 26, false)
	z.areaEventNight()
	if z.ae == nil || z.ae.reason != "noite" {
		t.Fatal("a noite com chance 100% dispara")
	}
	z.endAreaEvent(true, "")
	z.areaEventNight()
	if z.ae != nil {
		t.Fatal("a recarga segura o próximo evento")
	}
	z.tickFor(301)
	z.areaEventNight()
	if z.ae == nil {
		t.Fatal("depois da recarga dispara de novo")
	}

	win := mk(game.TriggerDef{Kind: game.TriggerWindow, Chance: 1, Window: 120})
	game.AreaEvents = []*game.EventDef{win}
	z = newAreaZone(t)
	defender(z, 1, "Ana", 30, 30, 26, false)
	z.tickFor(119)
	if z.ae != nil {
		t.Fatal("antes da janela fechar não sorteia")
	}
	z.tickFor(3)
	if z.ae == nil || z.ae.reason != "janela" {
		t.Fatal("a janela de 120 s com chance 100% dispara")
	}

	fixed := mk(game.TriggerDef{Kind: game.TriggerFixed, Hour: 12, Minute: 0})
	game.AreaEvents = []*game.EventDef{fixed}
	z = newAreaZone(t)
	defender(z, 1, "Ana", 30, 30, 26, false)
	z.tickFor(2)
	if z.ae != nil {
		t.Fatal("fora do horário fixo não dispara")
	}
	at := time.Date(2026, 3, 3, 12, 0, 30, 0, brasilia)
	z.cfg.Clock = func() time.Time { return at }
	z.tickFor(2)
	if z.ae == nil || z.ae.reason != "horário" {
		t.Fatal("no horário fixo dispara")
	}
	z.stopAreaEvent()
	z.tickFor(2)
	if z.ae != nil {
		t.Fatal("o horário fixo dispara uma vez por dia")
	}
}

// O GM dispara e para o evento com /evento area; os dados desligados (Farol) não disparam.
func TestAreaEventGMCommand(t *testing.T) {
	z := newAreaZone(t)
	_, peer := defender(z, 1, "Mestre", 190, 90, 26, true)
	say(z, 1, "/evento area")
	if !peer.systemContains("bastiao_resiste") || !peer.systemContains("acender_farol (desligado)") {
		t.Fatal("a lista mostra os eventos e o desligado")
	}
	say(z, 1, "/evento area acender_farol")
	if z.ae != nil {
		t.Fatal("o Farol está desligado")
	}
	say(z, 1, "/evento area bastiao_resiste agora")
	if z.ae == nil || z.ae.warning {
		t.Fatalf("o GM começou o evento: %+v", z.ae)
	}
	say(z, 1, "/evento area bastiao_resiste parar")
	if z.ae != nil || len(z.postUntil) != 0 {
		t.Fatal("parar não derruba o posto")
	}
	z.closePost("bastiao", 600)
	say(z, 1, "/evento area reabrir bastiao")
	if z.postClosed("bastiao") || !strings.Contains(strings.Join(peer.announceTitles(), "|"), "reabriu") {
		t.Fatal("o GM reabre o posto")
	}
}

// Postos fechados são vistos por outras zonas do processo (a pedra é destino de outros mapas).
func TestPostBoardSharedAcrossZones(t *testing.T) {
	var b postBoard
	now := noon
	if b.isClosed("bastiao", now) {
		t.Fatal("começa aberto")
	}
	b.set("bastiao", now.Add(time.Minute))
	if !b.isClosed("bastiao", now) || b.isClosed("bastiao", now.Add(2*time.Minute)) {
		t.Fatal("fechado só até a hora")
	}
	b.set("bastiao", time.Time{})
	if b.isClosed("bastiao", now) {
		t.Fatal("reaberto")
	}
}

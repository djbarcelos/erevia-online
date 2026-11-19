package game

import "math"

// Linhas de história das regiões (tunnel#71). Cada região tem um contador de história
// (talk.go) e uma sequência de missões: um convite que chega sozinho com o nível ("vá falar
// com fulano") e de 3 a 4 missões que o NPC oferece e recebe na conversa. A história que
// elas contam, região por região, está em docs/historia-de-erevia.md.
//
// XP: cada missão paga uma fração da barra do nível dela (storyShare). Essa XP sai do
// orçamento de 30% de missões de cada nível: o contrato de caça do mesmo nível paga menos
// (StoryXPAt, quests.go), então a curva até o 40 não encurta. O resto da recompensa é em
// moedas, itens e, do 30 em diante, reputação com a Ordem do Bastião ou com a Guarda.

const (
	inviteShare = 0.01 // o convite (só falar com o NPC)
	storyShare  = 0.05 // uma missão da linha
	bossShare   = 0.06 // o fim da linha com chefe ou raid
	// StoryShareMax: o máximo de XP de história num nível (fração da barra). O contrato do
	// nível fica com pelo menos ContractXPShare − StoryShareMax.
	StoryShareMax = 0.12
	storyRep      = 100 // reputação por missão das linhas do 30 em diante
)

func storyXP(level int, share float64) int64 {
	return int64(math.Round(float64(XPToNext(level)) * share))
}

// storyGold: as moedas de uma missão do nível (~40 abates de moedas), em dezenas.
func storyGold(level int) int64 { return int64(level) * 50 }

func kill(target string, n int, text string) QuestStep {
	return QuestStep{Goal: GoalKill, Target: target, Count: n, Text: text}
}

func talk(npc, text, say string) QuestStep {
	return QuestStep{Goal: GoalTalk, Target: npc, Count: 1, Text: text, Say: say}
}

func deliver(item string, n int, npc, text string) QuestStep {
	return QuestStep{Goal: GoalDeliver, Target: item, NPC: npc, Count: n, Text: text}
}

func collect(item string, n int, text string) QuestStep {
	return QuestStep{Goal: GoalCollect, Target: item, Count: n, Text: text}
}

// Lugares para visitar (GoalVisit). A chave liga os passos que pedem o mesmo lugar.
type spot struct {
	key, name string
	x, z, r   float64
}

var (
	spotMill      = spot{"moinho", "Moinho dos Arredores", 56, 16, 7}
	spotNorthMark = spot{"marco_norte", "Marco da estrada norte", 0, 60, 6}
	spotBanner    = spot{"estandarte", "Estandarte Caído", -66, 66, 7}
	spotEastMark  = spot{"marco_leste", "Marco da estrada leste", 95, 0, 6}
	spotPass      = spot{"passo_brasaferro", "Passo de Brasaferro", -146, 0, 6}
	spotRift      = spot{"fenda_sudoeste", "Fenda do Sudoeste", -96, -96, 7}
	spotWestRing  = spot{"circulo_oeste", "Círculo Oeste", -36, -244, 7}
	spotEastRing  = spot{"circulo_leste", "Círculo Leste", 40, -258, 7}
	spotWestFall  = spot{"caidas_oeste", "Pedras Caídas do Oeste", -104, -284, 7}
	spotEastFall  = spot{"caidas_leste", "Pedras Caídas do Leste", 108, -272, 7}
	spotDeepRing  = spot{"circulo_fundo", "Círculo do Fundo", -10, -302, 6}
	spotCrypt     = spot{"porta_cripta", "Borda da Cripta", 86, 86, 5}
	spotRiftNW    = spot{"rachadura_no", "Rachadura do Noroeste", -122, 122, 8}
	spotRiftSW    = spot{"rachadura_so", "Rachadura do Sudoeste", -122, -122, 8}
	spotRiftSE    = spot{"rachadura_se", "Rachadura do Sudeste", 122, -122, 8}
	spotDock      = spot{"cais_barqueiro", "Cais do Barqueiro", DockX, DockZ, 7}
	spotIsland    = spot{"ilha_vagalumes", "Ilha dos Vaga-lumes", IslandX, IslandZ, 7}
	spotMother    = spot{"vau_velho", "Lagoa de Vau Velho", MotherX, MotherZ, 12}
)

func visit(s spot, text string) QuestStep {
	return QuestStep{Goal: GoalVisit, Target: s.key, Count: 1, Text: text,
		Place: &QuestPlace{Kind: PlaceSpot, X: s.x, Z: s.z, Radius: s.r, Name: s.name}}
}

func items(list ...any) []QuestItem {
	var out []QuestItem
	for i := 0; i+1 < len(list); i += 2 {
		out = append(out, QuestItem{ID: list[i].(string), Count: list[i+1].(int)})
	}
	return out
}

// line monta uma linha de história: o convite (chega no nível do primeiro, até until) e as
// missões, encadeadas por Next. Todas são da categoria Região.
func line(region, npc string, until int, invite *QuestDef, quests ...*QuestDef) []*QuestDef {
	invite.Category, invite.Region, invite.Until = QuestRegion, region, until
	invite.Unlock = quests[0].Level
	invite.Reward.XP = storyXP(invite.Unlock, inviteShare)
	invite.Next = quests[0].ID
	out := []*QuestDef{invite}
	for i, q := range quests {
		q.Category, q.Region, q.Giver = QuestRegion, region, npc
		if q.TurnIn == "" {
			q.TurnIn = npc
		}
		if q.TurnIn == "-" { // termina sozinha (o último passo já é falar com alguém)
			q.TurnIn = ""
		}
		share := storyShare
		if q.Reward.XP < 0 { // marcador: missão de chefe
			share = bossShare
		}
		q.Reward.XP = storyXP(q.Level, share)
		if q.Reward.Gold == 0 {
			q.Reward.Gold = storyGold(q.Level)
		}
		if i+1 < len(quests) {
			q.Next = quests[i+1].ID
		}
		out = append(out, q)
	}
	return out
}

const boss = -1 // Reward.XP = boss: a missão paga bossShare

var storyQuests = concatLines(
	// ---- Arredores (1–5): o moleiro Anselmo ----
	line("Arredores", TalkMiller, 6,
		&QuestDef{ID: "arr_convite", Title: "O moleiro dos Arredores",
			Text:  "Na praça dizem que o velho Anselmo, do moinho a leste da cidade, não dorme há três noites. Vá até o moinho e ouça o que ele tem a dizer.",
			Steps: []QuestStep{talk(TalkMiller, "Falar com Anselmo, no moinho dos Arredores", "")}},
		&QuestDef{ID: "arr_trigo", Title: "Ossos no trigo", Level: 3,
			Text:   "Os Lacaios sobem do trigal toda noite e pisoteiam o que sobrou da colheita. Anselmo pede que você derrube seis deles antes que o moinho pare de vez.",
			Steps:  []QuestStep{kill("skeleton_minion", 6, "Derrotar Esqueletos Lacaios")},
			Done:   "Seis a menos. Amanhã tem mais, mas hoje o trigo fica em pé. Obrigado, de verdade.",
			Reward: QuestReward{Items: items("potion_hp_s", 3)}},
		&QuestDef{ID: "arr_fogueira", Title: "Osso que não queima", Level: 4,
			Text: "Osso que não queima volta a andar, diz o moleiro: era o que a avó dele dizia, e a avó dele nunca errou. " +
				"Traga Ossos Velhos dos Lacaios para a fogueira do moinho.",
			Steps:  []QuestStep{deliver("bone_old", 8, TalkMiller, "Entregar 8 Ossos Velhos a Anselmo")},
			Done:   "Para o fogo com eles. Sente o cheiro? Fumaça de osso velho é fumaça de paz.",
			Reward: QuestReward{Items: items("potion_mp_s", 3)}},
		&QuestDef{ID: "arr_runas", Title: "Runas sob o moinho", Level: 5,
			Text: "Cavando a fogueira, Anselmo achou uma pedra lisa com riscos que brilham no escuro, e ninguém nos Arredores sabe ler aquilo. " +
				"Leve o desenho das runas a Cássio, o Arauto da Guarda, na praça de Vel'Aran: ele sabe de tudo que é antigo.",
			Steps: []QuestStep{talk(TalkHerald, "Mostrar as runas a Cássio, na praça",
				"Deixe-me ver... São runas Ilvarin, de um veio da rede que prende o rei. Se uma delas aflorou debaixo de um moinho, "+
					"o selo está mais perto da superfície do que a Guarda gostaria. Diga a Anselmo que guarde a pedra e não a quebre, por nada.")},
			Done: "Ilvarin? Os que racharam o mundo? Então a pedra é um pedaço da rede que prende o rei, e está debaixo do meu moinho. " +
				"Vou dormir de olho aberto. Se os ossos daqui acordam, os dos Campos, ao norte, devem estar pior: procure Marga, a coveira.",
			Reward: QuestReward{Items: items("potion_hp_s", 5, "scroll_return", 2)}},
	),
	// ---- Campos dos Ossos (6–12): a coveira Marga ----
	line("Campos dos Ossos", TalkDigger, 12,
		&QuestDef{ID: "cam_convite", Title: "A coveira dos Campos",
			Text:  "Nos Campos dos Ossos, ao norte, uma coveira fecha as covas que os mortos abrem. Dizem que Marga não tem medo de nada, vivo ou morto, mas até ela precisa de ajuda.",
			Steps: []QuestStep{talk(TalkDigger, "Falar com Marga, perto da pedra dos Campos", "")}},
		&QuestDef{ID: "cam_covas", Title: "Covas abertas", Level: 7,
			Text:   "As covas que Marga fecha de dia amanhecem abertas, e o que sai delas marcha. Derrote os Guerreiros e os Ladinos que rondam a estrada norte.",
			Steps:  []QuestStep{kill("skeleton_warrior|skeleton_rogue", 10, "Derrotar Esqueletos Guerreiros ou Ladinos")},
			Done:   "Bom braço. Eles caem em fileira, viu? Até para morrer de novo esperam ordem.",
			Reward: QuestReward{Items: items("potion_hp_s", 5)}},
		&QuestDef{ID: "cam_nomes", Title: "Nomes riscados", Level: 8,
			Text: "Quem enterrou o exército do rei riscou um nome em cada crânio, para que os mortos descansassem com ele. " +
				"Marga quer ler os nomes. Traga Crânios Rachados dos esqueletos dos Campos.",
			Steps:  []QuestStep{deliver("skull_cracked", 4, TalkDigger, "Entregar 4 Crânios Rachados a Marga")},
			Done:   "Brann, Ostel, Ivar... São nomes da Guarda do Rei, os que juraram morrer por Ozrhal. Ele os quer de volta. Inteiros.",
			Reward: QuestReward{Items: items("potion_mp_s", 5)}},
		&QuestDef{ID: "cam_estandarte", Title: "O estandarte caído", Level: 10,
			Text: "Marga diz que o estandarte de Ozrhal ainda está fincado onde a linha de frente caiu, a noroeste da pedra dos Campos, " +
				"e que os mortos se juntam em volta dele. Encontre o estandarte e derrube quem o guarda.",
			Steps: []QuestStep{visit(spotBanner, "Encontrar o Estandarte Caído"),
				kill("skeleton_warrior", 8, "Derrotar Esqueletos Guerreiros")},
			Done:   "Então ainda está lá, de pé depois de três séculos. Enquanto o estandarte estiver fincado, a fileira não se desfaz.",
			Reward: QuestReward{Items: items("potion_hp_s", 5, "scroll_return", 2)}},
		&QuestDef{ID: "cam_relato", Title: "Notícia para a cidade", Level: 11, TurnIn: "-",
			Text: "A Guarda precisa saber que o exército dos Campos está se formando. Marga não sai de perto das covas: " +
				"leve o relato dela a Iselda, que paga as caçadas no Quadro de Contratos da praça.",
			Steps: []QuestStep{talk(TalkBoard, "Levar o relato de Marga a Iselda, na praça",
				"Formação nos Campos... Então não são só ossos soltos. Vou pôr mais caçadas no quadro para a estrada norte. "+
					"E fique com isto: a Guarda paga quem traz notícia antes do perigo.")},
			Reward: QuestReward{Items: items("scroll_ench_armor", 1)}},
	),
	// ---- Ermos Partidos (13–20): o batedor Dárcio ----
	line("Ermos Partidos", TalkScout, 20,
		&QuestDef{ID: "erm_convite", Title: "O batedor dos Ermos",
			Text:  "Dárcio, batedor da Guarda, acampa nos Ermos Partidos, perto da pedra do oeste. Ele conta o que sai das rachaduras e precisa de quem saiba lutar.",
			Steps: []QuestStep{talk(TalkScout, "Falar com Dárcio, perto da pedra dos Ermos", "")}},
		&QuestDef{ID: "erm_corte", Title: "A corte do rei", Level: 14,
			Text:   "Os Magos Esqueletos eram os feiticeiros de Ozrhal e ainda erguem os mortos com as mesmas palavras. Dárcio quer menos vozes nos Ermos.",
			Steps:  []QuestStep{kill("skeleton_mage", 10, "Derrotar Esqueletos Magos")},
			Done:   "Dez bocas a menos. Repare: quando um mago cai, os guerreiros em volta ficam lentos, perdidos. São eles que puxam os fios.",
			Reward: QuestReward{Items: items("potion_mp_s", 5)}},
		&QuestDef{ID: "erm_fumaca", Title: "Fumaça engarrafada", Level: 15,
			Text: "Os magos carregam Essência Sombria, uma fumaça que não se desfaz no vento. Dárcio quer saber de onde ela vem. " +
				"Traga algumas para ele estudar.",
			Steps: []QuestStep{deliver("shadow_essence", 3, TalkScout, "Entregar 3 Essências Sombrias a Dárcio")},
			Done: "Fria, e pesa mais do que devia. Isto não é magia de morto: é a própria Partição escorrendo das fendas. " +
				"Os magos só aprenderam a beber dela.",
			Reward: QuestReward{Items: items("potion_hp_s", 8)}},
		&QuestDef{ID: "erm_passo", Title: "A borda do mundo", Level: 17,
			Text: "A estrada oeste some num barranco que a Partição abriu: o Passo de Brasaferro. " +
				"Dárcio precisa de olhos lá para saber se as fendas cresceram.",
			Steps:  []QuestStep{visit(spotPass, "Chegar ao Passo de Brasaferro, no fim da estrada oeste")},
			Done:   "Cresceram, então. Do outro lado ficam as minas de Brasaferro, que desabaram na Partição. Um dia a Guarda vai ter de descer lá.",
			Reward: QuestReward{Items: items("potion_hp_l", 3, "scroll_return", 2)}},
		&QuestDef{ID: "erm_arauto", Title: "O arauto do rei", Level: 19,
			Text: "O Senhor dos Ossos ronda a estrada sul anunciando a volta de Ozrhal, e onde ele passa os mortos levantam mais depressa. " +
				"É forte demais para um só: junte um grupo e cale o arauto, nem que seja por uma noite.",
			Steps: []QuestStep{kill("bone_lord", 1, "Derrotar o Senhor dos Ossos, na estrada sul")},
			Done: "Ele cai e volta, eu sei. Mas antes de cair, o que ele gritou? Pedras que acordam ao sul... " +
				"Desça até o Vale: a Ilvarin do acampamento sabe ler isso.",
			Reward: QuestReward{XP: boss, Items: items("scroll_ench_weapon", 1)}},
	),
	// ---- Brejo de Lamagris (15–20): o Barqueiro Tomé ----
	line(BrejoRegion, TalkFerry, 20,
		&QuestDef{ID: "bre_convite", Title: "O Barqueiro do Brejo",
			Text:  "Na ponta da estrada leste, onde o chão vira lama, um barqueiro chamado Tomé mantém um posto e leva gente pela lagoa. Ele conhece o Brejo como ninguém, e anda precisando de braços.",
			Steps: []QuestStep{talk(TalkFerry, "Falar com Tomé, no Posto do Barqueiro", "")}},
		&QuestDef{ID: "bre_sapos", Title: "Línguas compridas", Level: 15,
			Text:   "Os Sapos-Boi do Brejo prendem quem passa com a língua e deixam o viajante parado, à espera de coisa pior. Tomé pede que você derrube alguns, para a margem do posto ficar limpa.",
			Steps:  []QuestStep{kill("sapo_boi", 10, "Derrotar Sapos-Boi")},
			Done:   "Dez a menos. A língua deles é rápida, mas o aviso aparece no chão: ande de lado, e não para trás. Eu devia ter ensinado isso a mais gente.",
			Reward: QuestReward{Items: items("potion_hp_s", 5)}},
		&QuestDef{ID: "bre_glandulas", Title: "Remédio de pântano", Level: 16,
			Text:   "O veneno do Brejo derruba o mais valente em dois minutos, e a cura é feita com a glândula do Sapo-Boi. Traga algumas a Tomé: ele ensina a receita e deixa você levar uns frascos prontos.",
			Steps:  []QuestStep{deliver("frog_gland", 4, TalkFerry, "Entregar 4 Glândulas de Sapo a Tomé")},
			Done:   "Com glândula e junco-de-prata, o Ferreiro Bruno faz o Antídoto lá na cidade. Leve estes três frascos que eu tinha guardados e nunca saia do posto sem um.",
			Reward: QuestReward{Items: items("potion_antidote", 3, "potion_hp_s", 3)}},
		&QuestDef{ID: "bre_ilha", Title: "A ilha das luzes", Level: 18,
			Text:   "O Lodo Vivo se arrasta da água até a margem e cobre a trilha do cais. Limpe a trilha e atravesse no barco de Tomé até a Ilha dos Vaga-lumes: ele quer saber se as luzes ainda sobem do capim.",
			Steps:  []QuestStep{kill("lodo_vivo", 8, "Derrotar Lodos Vivos"), visit(spotIsland, "Chegar à Ilha dos Vaga-lumes, de barco")},
			Done:   "Ainda sobem, então. Meu filho gostava de ficar lá, olhando. Eu já não vou: sempre acho que ele vai estar na beira da água, com os outros, parado e de olhos abertos.",
			Reward: QuestReward{Items: items("potion_mp_l", 3, "potion_antidote", 2)}},
		&QuestDef{ID: "bre_mae", Title: "A Mãe-Lodo", Level: 20,
			Text:   "No meio da lagoa de Vau Velho, onde só a torre da capela aparece, uma massa de lama do tamanho de uma casa guarda o que sobrou da aldeia. Quem a derruba não vence o Brejo, mas faz a água respirar de novo. Junte um grupo.",
			Steps:  []QuestStep{visit(spotMother, "Chegar à lagoa de Vau Velho"), kill("mae_lodo", 1, "Derrotar a Mãe-Lodo")},
			Done:   "A água parou de ferver. Ouça: de noite, o sino da capela afundada ainda toca, abafado, e o que está preso nele não é só sino. Quando a Guarda perguntar das Pedras-Selo, diga que uma está debaixo daquela água.",
			Reward: QuestReward{XP: boss, Items: items("scroll_ench_weapon_b", 1, "potion_antidote", 3)}},
	),
	// ---- Vale das Pedras Vivas (20–30): a tecelã Maelis ----
	line("Vale das Pedras Vivas", TalkWeaver, 30,
		&QuestDef{ID: "val_convite", Title: "A tecelã do Vale",
			Text:  "No acampamento ao norte do Vale das Pedras Vivas vive Maelis, uma Ilvarin, do povo que fez as pedras obedecerem. Ela quer falar com quem caça os golens.",
			Steps: []QuestStep{talk(TalkWeaver, "Falar com Maelis, no acampamento do Vale", "")}},
		&QuestDef{ID: "val_pedra", Title: "Pedra que anda", Level: 21,
			Text: "Os Golens de Pedra perderam a ordem que os Ilvarin lhes deram e esmagam o que encontram. " +
				"Maelis pede que você devolva alguns ao chão: assim ela vê como a rede reage.",
			Steps:  []QuestStep{kill("stone_golem", 12, "Derrotar Golens de Pedra")},
			Done:   "Sentiu? Cada golem que cai solta um fio da rede, e a rede puxa os outros. Está viva, e está com fome.",
			Reward: QuestReward{Items: items("potion_hp_l", 4)}},
		&QuestDef{ID: "val_circulos", Title: "Os círculos", Level: 23,
			Text: "Os monólitos do Vale são nós da rede de runas. Maelis precisa saber se os dois círculos maiores, " +
				"a oeste e a leste do Vale, ainda brilham. Vá até cada um.",
			Steps: []QuestStep{visit(spotWestRing, "Visitar o Círculo Oeste"), visit(spotEastRing, "Visitar o Círculo Leste")},
			Done: "Os dois brilham, mas não no mesmo ritmo. A rede está desafinada, como um tear com fios de tamanhos diferentes. " +
				"Isso explica os golens.",
			Reward: QuestReward{Items: items("potion_mp_l", 4)}},
		&QuestDef{ID: "val_nucleos", Title: "Corações mornos", Level: 25,
			Text: "O Núcleo Rúnico de um golem guarda o que as runas lembram. Com eles, Maelis pode reafinar parte da rede. " +
				"Traga Núcleos Rúnicos dos golens.",
			Steps:  []QuestStep{deliver("rune_core", 5, TalkWeaver, "Entregar 5 Núcleos Rúnicos a Maelis")},
			Done:   "Mornos ainda. Escute... é a voz do meu bisavô, tecendo. Com isto eu seguro os dois círculos por um tempo.",
			Reward: QuestReward{Items: items("potion_hp_l", 5, "scroll_ench_armor_tc", 2)}},
		&QuestDef{ID: "val_ysolde", Title: "Aquilo que Ysolde sabe", Level: 27,
			Text: "Maelis acha que a rede não afrouxou sozinha: alguém puxa do outro lado, de dentro da Cripta. " +
				"A Mestra Ysolde, na praça de Vel'Aran, é a única Ilvarin da cidade. Leve a suspeita a ela e volte com a resposta.",
			Steps: []QuestStep{talk(TalkMaster, "Falar com a Mestra Ysolde, na praça", "")},
			Done: "Ysolde confirmou, então. O rei puxa a rede de dentro da Cripta, fio por fio. Se os veios do Vale acordaram, " +
				"o selo está mais fraco do que a Ordem admite. Desça ao Fundo: o Irmão Gaspar vigia lá.",
			Reward: QuestReward{Items: items("scroll_ench_armor_tc_bl", 1, "scroll_return", 3)}},
	),
	// ---- Fundo do Vale (30–35): o Irmão Gaspar, da Ordem do Bastião ----
	line("Fundo do Vale", TalkMonk, 35,
		&QuestDef{ID: "fun_convite", Title: "O monge do Fundo",
			Text:  "O Irmão Gaspar, da Ordem do Bastião Cinzento, vigia o Fundo do Vale do acampamento ao norte do Vale. Ele pede ajuda a quem já enfrenta os Golens Ancestrais.",
			Steps: []QuestStep{talk(TalkMonk, "Falar com o Irmão Gaspar, no acampamento do Vale", "")}},
		&QuestDef{ID: "fun_musgo", Title: "Musgo antigo", Level: 30,
			Text: "Os Golens Ancestrais do Fundo acordaram com o tremor da Cripta. Gaspar quer saber quantos se levantam por dia, " +
				"e para isso precisa de alguns deles no chão.",
			Steps: []QuestStep{kill("ancient_golem", 15, "Derrotar Golens Ancestrais")},
			Done: "Quinze, e o chão ainda treme. Estes foram os primeiros guardiões. Se até eles acordaram, " +
				"a Cripta não está só rachando: está chamando.",
			Reward: QuestReward{Items: items("potion_hp_l", 5), Faction: FactionBastion, Rep: storyRep}},
		&QuestDef{ID: "fun_circulo", Title: "O círculo proibido", Level: 31,
			Text: "No centro do Fundo há um círculo de pedras que nem os Ilvarin ousavam tocar. Gaspar não pode deixar o posto: " +
				"vá até lá e veja se as pedras ainda estão de pé.",
			Steps:  []QuestStep{visit(spotDeepRing, "Chegar ao Círculo do Fundo")},
			Done:   "De pé, mas inclinadas para o norte, como capim no vento. Todas apontam para a Cripta. A Ordem precisa ver isto.",
			Reward: QuestReward{Items: items("potion_mp_l", 5), Faction: FactionBastion, Rep: storyRep}},
		&QuestDef{ID: "fun_nucleos", Title: "Núcleos para a Ordem", Level: 33,
			Text: "Os Núcleos dos Golens Ancestrais são mais densos que os do Vale, e a Ordem os usa para reforçar as pedras do Bastião. " +
				"Traga Núcleos Rúnicos ao Irmão Gaspar.",
			Steps: []QuestStep{deliver("rune_core", 8, TalkMonk, "Entregar 8 Núcleos Rúnicos a Gaspar")},
			Done: "A Ordem não esquece quem a ajuda. O tremor vem do norte, da Cripta. Sor Bertrand comanda a Vigília, " +
				"no posto a sudoeste da Cripta. Diga que fui eu que mandei.",
			Reward: QuestReward{Items: items("scroll_ench_weapon_tb_bl", 1), Faction: FactionBastion, Rep: storyRep}},
	),
	// ---- Vigília Ossária (33–38): Sor Bertrand, do Bastião ----
	line("Vigília Ossária", TalkKnight, 38,
		&QuestDef{ID: "vig_convite", Title: "O posto da Vigília",
			Text:  "Sor Bertrand, cavaleiro do Bastião Cinzento, comanda o posto a sudoeste da Cripta Esquecida. A Ordem chama quem consegue segurar a Vigília Ossária.",
			Steps: []QuestStep{talk(TalkKnight, "Falar com Sor Bertrand, no posto a sudoeste da Cripta", "")}},
		&QuestDef{ID: "vig_canticos", Title: "Cânticos", Level: 33,
			Text:   "Os Devotos do Ossário cantam para que o selo ceda. Cada voz calada é um dia a mais de selo. Bertrand quer silêncio.",
			Steps:  []QuestStep{kill("ossuary_devotee", 12, "Derrotar Devotos do Ossário")},
			Done:   "Ouviu o silêncio? Dura pouco. Mas é o melhor som que esta terra conhece.",
			Reward: QuestReward{Items: items("potion_hp_l", 5), Faction: FactionBastion, Rep: storyRep}},
		&QuestDef{ID: "vig_velas", Title: "Velas de sebo negro", Level: 34,
			Text: "Os Devotos queimam Essência Sombria em velas de sebo negro, e a fumaça escorre para dentro da Cripta. " +
				"Tire a essência deles e traga a Bertrand, que a queima em fogo limpo.",
			Steps:  []QuestStep{deliver("shadow_essence", 5, TalkKnight, "Entregar 5 Essências Sombrias a Bertrand")},
			Done:   "Fogo limpo, fumaça branca. É pouco, mas é a nossa resposta ao canto deles.",
			Reward: QuestReward{Items: items("potion_mp_l", 5), Faction: FactionBastion, Rep: storyRep}},
		&QuestDef{ID: "vig_porta", Title: "A porta da Cripta", Level: 35,
			Text: "Bertrand precisa saber se a guarda pessoal do rei já chegou à porta. Vá até a borda do círculo de monólitos da Cripta " +
				"e derrube os Cavaleiros Ossários da Vigília.",
			Steps: []QuestStep{visit(spotCrypt, "Chegar à borda da Cripta Esquecida"),
				kill("ossuary_knight", 10, "Derrotar Cavaleiros Ossários")},
			Done:   "A guarda do rei está na porta, então. Eles não esperam o rei sair: esperam para abrir.",
			Reward: QuestReward{Items: items("potion_hp_l", 6, "scroll_ench_armor_tb_bl", 1), Faction: FactionBastion, Rep: storyRep}},
		&QuestDef{ID: "vig_rei", Title: "O Rei Ossário", Level: 36,
			Text: "De tempos em tempos Ozrhal acorda inteiro dentro da Cripta. Só um clã consegue fazê-lo dormir de novo, " +
				"e cada queda dá ao selo um respiro. Junte-se a quem enfrentar o Rei Ossário.",
			Steps:  []QuestStep{kill("ozrhal", 1, "Participar da queda de Ozrhal, na Cripta Esquecida")},
			Done:   "Você esteve lá dentro e voltou. Poucos podem dizer isso. A Ordem do Bastião lhe deve um respiro do mundo.",
			Reward: QuestReward{XP: boss, Items: items("scroll_ench_weapon_tb_bl", 1), Faction: FactionBastion, Rep: 2 * storyRep}},
	),
	// ---- Confins Partidos (35–40): a Capitã Rovena, da Guarda ----
	line("Confins Partidos", TalkCaptain, 40,
		&QuestDef{ID: "con_convite", Title: "A capitã dos Confins",
			Text: "A Capitã Rovena, da Guarda de Vel'Aran, ergueu um posto a noroeste da cidade, entre os Campos e os Ermos. " +
				"Dos Confins Partidos, os cantos do mundo, sai o que a Partição deformou.",
			Steps: []QuestStep{talk(TalkCaptain, "Falar com a Capitã Rovena, no posto a noroeste", "")}},
		&QuestDef{ID: "con_ceifadores", Title: "Ceifadores", Level: 35,
			Text:   "Os Ceifadores Partidos saem das rachaduras dos cantos e cortam o que encontram. A Guarda precisa de espaço para olhar as rachaduras de perto.",
			Steps:  []QuestStep{kill("broken_reaper", 12, "Derrotar Ceifadores Partidos")},
			Done:   "Doze. Já vi gente boa cair diante de três. A Guarda vai lembrar do seu nome.",
			Reward: QuestReward{Items: items("potion_hp_l", 6), Faction: FactionGuard, Rep: storyRep}},
		&QuestDef{ID: "con_rachaduras", Title: "Três rachaduras", Level: 36,
			Text: "Rovena quer saber se as rachaduras dos três cantos estão crescendo. Vá até cada uma: " +
				"a do noroeste, a do sudoeste e a do sudeste.",
			Steps: []QuestStep{visit(spotRiftNW, "Chegar à Rachadura do Noroeste"), visit(spotRiftSW, "Chegar à Rachadura do Sudoeste"),
				visit(spotRiftSE, "Chegar à Rachadura do Sudeste")},
			Done:   "As três cresceram desde a última ronda. Não muito, mas cresceram. A Partição não acabou: só ficou mais lenta.",
			Reward: QuestReward{Items: items("potion_mp_l", 6, "scroll_return", 3), Faction: FactionGuard, Rep: storyRep}},
		&QuestDef{ID: "con_gelo", Title: "Gelo nas estradas", Level: 37,
			Text:   "O gelo dos Arcanistas Partidos fecha os caminhos dos cantos. A Guarda quer os caminhos abertos para as próximas rondas.",
			Steps:  []QuestStep{kill("broken_arcanist", 10, "Derrotar Arcanistas Partidos")},
			Done:   "Caminhos abertos. Por um dia, pelo menos. É assim que a Guarda vive: um dia de cada vez.",
			Reward: QuestReward{Items: items("potion_hp_l", 6, "scroll_ench_armor_tb_bl", 1), Faction: FactionGuard, Rep: storyRep}},
		&QuestDef{ID: "con_pedras", Title: "O que a Partição guarda", Level: 38,
			Text: "Rovena juntou os relatos de todas as regiões, mas falta ouvir quem conhece as pedras. " +
				"Leve o relato dos Confins a Maelis, no Vale, e a Sor Bertrand, na Vigília, e volte com o que disserem.",
			Steps: []QuestStep{
				talk(TalkWeaver, "Levar o relato a Maelis, no acampamento do Vale",
					"Os Confins também? Então a rede inteira está sendo puxada, não só o Vale. Diga à capitã que só as Pedras-Selo "+
						"seguram o que a rede já não segura."),
				talk(TalkKnight, "Levar o relato a Sor Bertrand, no posto da Vigília",
					"A Ordem já desconfiava. Diga a Rovena que o Bastião guarda a primeira Pedra-Selo, e que sozinha ela não aguenta outros três séculos."),
			},
			Done: "Então é isso: o rei puxa a rede, a rede puxa a terra, e a terra racha. Cinco Pedras-Selo seguram tudo, e só sabemos onde está uma. " +
				"As outras estão além das montanhas. Quando as estradas abrirem, Vel'Aran vai precisar de você outra vez.",
			Reward: QuestReward{XP: boss, Items: items("scroll_ench_weapon_tb_bl", 1), Faction: FactionGuard, Rep: 2 * storyRep}},
	),
	bosqueQuests, // Bosque Gris (bosque_story.go, tunnel#137)
	terrasQuests, // Terras Mortas (bosque_story.go)
)

func concatLines(lines ...[]*QuestDef) []*QuestDef {
	var out []*QuestDef
	for _, l := range lines {
		out = append(out, l...)
	}
	return out
}

// questLevel: o nível em que a missão chega (convite) ou pode ser aceita.
func questLevel(q *QuestDef) int { return max(1, q.Unlock, q.Level) }

// StoryXPAt: a XP das missões de história (convites e linhas) do nível. É o que o contrato
// de caça do nível deixa de pagar (contractQuests).
func StoryXPAt(level int) int64 {
	var xp int64
	for _, q := range storyQuests {
		if questLevel(q) == level {
			xp += q.Reward.XP
		}
	}
	return xp
}

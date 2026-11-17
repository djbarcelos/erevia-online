package game

// Os NPCs e as histórias do Bosque Gris e das Terras Mortas (tunnel#137). Os lugares, os bichos e os grupos de spawn estão
// em bosque.go.

// bosqueTalker: Hélio, o carroceiro, o contador de história do Bosque Gris, ao lado da Carroça Tombada.
var bosqueTalker = &Talker{Name: TalkCarter, Title: "Carroceiro do Bosque", Region: BosqueRegion,
	Spawn:    &TalkSpawn{Model: "barbarian", X: -37, Z: 94, Look: &Appearance{Skin: 3, Hair: 7, Outfit: 6, Head: "barbarian"}},
	Greeting: "Você não devia estar andando sozinho por aqui. Eu devia estar na estrada de Brasaferro com duas carroças e dez homens. Sobrou eu.",
	Busy:     "Cada hora que passa, Caio fica mais longe. Termine o que combinamos.",
	Accepted: "Vá com cuidado: quem assalta esta estrada conhece cada árvore. Eu fico aqui, de olho na carroça.",
	Idle:     "Caio está de volta, e a estrada voltou a ser só uma estrada. Se passar pela cidade, diga à Lívia que o irmão dela é teimoso, mas volta.",
	Topics: []TalkTopic{
		{ID: "regiao", Label: "Sobre o Bosque Gris",
			Text: "O Bosque era a estrada do oeste: carvalho velho, musgo nos troncos e pouca luz. Os esqueletos dos Campos expulsaram os lobos da planície, " +
				"e eles vieram parar aqui. De noite, as aranhas descem das copas. E sobre isso tudo, o Bando do Corvo."},
		{ID: "corvo", Label: "Sobre o Bando do Corvo",
			Text: "Eram da guarda de Vel'Aran, desertores. Quando viram os lobos expulsarem o mundo para dentro do Bosque, acharam uma muralha natural, " +
				"e agora assaltam as caravanas de Brasaferro. Pagam os lobos com carne, dizem. O chefe deles é o Corvo Rubro, que se esconde na Toca, no canto noroeste do mapa."},
		{ID: "caio", Label: "Sobre Caio",
			Text: "Caio é o irmão da Lívia, a mercadora da praça. Veio comigo para ver o mundo e voltar com uma história. Os bandoleiros o levaram vivo, " +
				"porque ele sabe contar: de qualquer jeito, é o tipo de refém que vale mais inteiro do que morto."},
	}}

// bastionTalker: o Irmão Caedmon, o contador de história das Terras Mortas, no pátio do Bastião Cinzento.
var bastionTalker = &Talker{Name: TalkBrother, Title: "Irmão do Bastião Cinzento", Region: TerrasMortasRegion,
	Spawn:    &TalkSpawn{Model: "knight", X: 199, Z: 93, Look: &Appearance{Skin: 2, Hair: 1, Outfit: 5, Head: "knight"}},
	Greeting: "A Ordem do Bastião Cinzento resiste, mas a noite é mais comprida a cada ano. Aqui as velas dos Devotos queimam do outro lado do muro, e eu apago o que posso.",
	Busy:     "As velas continuam acesas. Termine o que combinamos.",
	Accepted: "A Ordem caminha com você. Volte antes do amanhecer, se puder.",
	Idle:     "Por ora, o Bastião aguenta. A primeira Pedra-Selo segue guardada, e o selo do rei respira um pouco melhor. Passe aqui sempre que quiser descansar.",
	Topics: []TalkTopic{
		{ID: "regiao", Label: "Sobre as Terras Mortas",
			Text: "O chão aqui é cinza porque nada cresce onde tanta gente foi enterrada sem nome. Os Devotos do Ossário cavam as covas da Partição, " +
				"levantam os mortos e alimentam o selo rachado com velas de sebo negro. Acham que o Rei Ossário vai devolver os mortos queridos."},
		{ID: "devotos", Label: "Sobre os Devotos do Ossário",
			Text: "Gente viva que perdeu alguém. O Rei prometeu que quem cantasse na porta da Cripta teria o seu morto de volta, e eles acreditaram. " +
				"Os Acólitos curam, os Necromantes levantam, os Cavaleiros guardam. À noite, os Carniçais saem das covas, e há coisas que nem eu sei dizer o que são."},
		{ID: "sino", Label: "Sobre a Torre do Sino Rachado",
			Text: "A Torre do Sino Rachado é mais velha que o Bastião. O sino rachou no dia da Partição e nunca mais tocou direito: só toca três vezes, " +
				"quando faltam dez minutos para o Rei Ossário acordar. Quando ouvir, junte um grupo."},
		{ID: TopicRegicide, Label: "Quem derrubou o Rei Ossário?",
			Text: "Na última vez que o Rei Ossário caiu, quem mais o feriu foi: " + TokenRegicide + ". A Ordem anota o nome, e a Lápide do Último Regicida, ao lado do cemitério, também. " +
				"Dura até o próximo."},
		{ID: "pedra", Label: "Sobre a Pedra de Teleporte",
			Text: "A pedra do pátio liga o Bastião à cidade e às outras pedras que você já tocou. A Ordem a mantém acesa: vá e volte quando precisar. " +
				"A pedra só vale para quem chegou aqui andando."},
	}}

// tombTalker: a Lápide do Último Regicida, ao lado do Cemitério dos Reis Sem Nome. Mostra o nome de quem mais causou dano na
// última morte do Ozrhal (BossData.TopName, o mesmo do anúncio), que a zona põe no lugar de TokenRegicide.
var tombTalker = &Talker{Name: TalkTombstone, Title: "Lápide", Region: TerrasMortasRegion,
	Spawn:    &TalkSpawn{Model: "lapide", X: TombstoneX, Z: TombstoneZ},
	Greeting: "Gravado a ponta de faca, sobre a pedra fria: «Quem mais feriu o Rei Ossário na última vez que ele caiu: " + TokenRegicide + ". Que o próximo faça mais.»",
	Busy:     "A pedra não diz mais nada.",
	Accepted: "A pedra não diz mais nada.",
	Topics: []TalkTopic{{ID: TopicRegicide, Label: "Ler a inscrição outra vez",
		Text: "Quem mais feriu o Rei Ossário na última vez que ele caiu: " + TokenRegicide + "."}},
	Idle: "Gravado a ponta de faca, sobre a pedra fria: «Quem mais feriu o Rei Ossário na última vez que ele caiu: " + TokenRegicide + ". Que o próximo faça mais.»",
}

// TopicRegicide: o assunto do Irmão Caedmon que diz quem derrubou o Ozrhal (o texto leva TokenRegicide).
const TopicRegicide = "regicida"

// bosqueQuests: "A caravana de Caio" (Hélio, nível 9 a 16): a carroça tombada, os bandoleiros, as insígnias, a Toca e o Corvo
// Rubro. Cada nível fica abaixo do teto de XP de história (StoryShareMax).
var bosqueQuests = line(BosqueRegion, TalkCarter, 15,
	&QuestDef{ID: "bos_convite", Title: "A carroça tombada",
		Text:  "Na estrada do noroeste, junto a uma carroça tombada, um homem vigia o que sobrou de uma caravana. Vá até a Carroça Tombada e ouça o que ele tem a dizer.",
		Steps: []QuestStep{talk(TalkCarter, "Falar com Hélio, junto à Carroça Tombada", "")}},
	&QuestDef{ID: "bos_salteadores", Title: "Salteadores da estrada", Level: 9,
		Text: "Hélio guiava uma caravana a Brasaferro quando o Bando do Corvo saiu das árvores. Os bandoleiros se escondem até você chegar perto, " +
			"os atiradores recuam e os brutamontes seguram golpes de frente. Derrube alguns, para a estrada respirar.",
		Steps:  []QuestStep{kill("bandoleiro_corvo|atirador_facas|brutamontes_corvo", 10, "Derrotar bandidos do Bando do Corvo")},
		Done:   "Dez a menos. Ande de lado nos brutamontes: o escudo deles só segura o que vem de frente. E nunca fique parado perto de uma árvore grande.",
		Reward: QuestReward{Items: items("potion_hp_s", 5)}},
	&QuestDef{ID: "bos_insignias", Title: "O corvo no cinto", Level: 12,
		Text: "Cada membro do Bando leva um corvo de pano vermelho no cinto, e Hélio quer contá-los: assim sabe quantos faltam para o bando perder a coragem. " +
			"Traga Insígnias do Corvo.",
		Steps:  []QuestStep{deliver("crow_insignia", 8, TalkCarter, "Entregar 8 Insígnias do Corvo a Hélio")},
		Done:   "Oito corvos. Com isto e um pouco de pele de lobo, o Ferreiro Bruno faz uma capa e luvas de couro que valem uma caravana. Guarde algumas: servem para mais coisas do que você imagina.",
		Reward: QuestReward{Items: items("potion_mp_s", 5, "wolf_pelt", 6)}},
	&QuestDef{ID: "bos_toca", Title: "Onde está Caio", Level: 13,
		Text: "Caio, o irmão da Lívia, foi levado vivo para a Toca do Corvo, no canto noroeste do mapa, atrás de uma paliçada. Chegue até lá e abra caminho entre os bandoleiros.",
		Steps: []QuestStep{visit(spotCrowDen, "Chegar à Toca do Corvo"),
			kill("bandoleiro_corvo", 6, "Derrotar Bandoleiros do Corvo")},
		Done:   "Então estão com ele mesmo. Vivo, pelo menos: um refém que conta histórias vale mais inteiro. Só falta o Corvo Rubro.",
		Reward: QuestReward{Items: items("potion_hp_s", 5, "scroll_return", 2)}},
	&QuestDef{ID: "bos_corvo", Title: "O Rei dos Bandoleiros", Level: 16,
		Text: "O Corvo Rubro é um duelista: joga bombas de fumaça, chama o bando com um apito e, quando está perdendo, salta em cima de um de vocês. " +
			"Junte um grupo e liberte Caio.",
		Steps:  []QuestStep{kill("corvo_rubro", 1, "Derrotar o Corvo Rubro, na Toca do Corvo")},
		Done:   "Caio está livre, e a estrada voltou a ser só uma estrada. Quando o apito calar, os lobos também voltam para o mato. Leve isto: é de quem perdeu uma caravana e achou um amigo.",
		Reward: QuestReward{XP: boss, Items: items("scroll_ench_weapon_b", 1, "potion_hp_l", 3)}},
)

// terrasQuests: "As velas do Ossário" (Irmão Caedmon, nível 24 a 28, 5 passos): as velas, a cinza, os necromantes, o sino e
// o Fragmento de Selo, a 1ª Pedra-Selo.
var terrasQuests = line(TerrasMortasRegion, TalkBrother, 28,
	&QuestDef{ID: "tmo_convite", Title: "O Bastião Cinzento",
		Text:  "Nas Terras Mortas, ao nordeste da Cripta, a Ordem do Bastião Cinzento mantém um posto de pedra. O Irmão Caedmon aceita ajuda de quem consegue andar entre os Devotos.",
		Steps: []QuestStep{talk(TalkBrother, "Falar com o Irmão Caedmon, no Bastião Cinzento", "")}},
	&QuestDef{ID: "tmo_velas", Title: "Velas de sebo negro", Level: 24,
		Text:   "Cada vela negra acesa fora do Bastião aperta o selo do rei um pouco mais. Os Devotos as levam presas ao cinto. Traga Velas Negras para a Ordem apagar.",
		Steps:  []QuestStep{deliver("black_candle", 6, TalkBrother, "Entregar 6 Velas Negras a Caedmon")},
		Done:   "Seis a menos. A chama verde-pálida não aquece, mas ilumina o suficiente para os mortos acharem o caminho. Obrigado.",
		Reward: QuestReward{Items: items("potion_hp_l", 3)}},
	&QuestDef{ID: "tmo_cinzas", Title: "Cinza sem nome", Level: 25,
		Text:   "As covas dos Devotos não têm nome, mas têm cinza, e a Ordem lê o que a cinza guarda. Traga Cinza Sepulcral dos mortos que eles levantam.",
		Steps:  []QuestStep{deliver("grave_ash", 10, TalkBrother, "Entregar 10 Cinzas Sepulcrais a Caedmon")},
		Done:   "Cheira a terra molhada, e a cada pitada há um nome diferente. Estão levantando gente que ninguém velou. Há algo maior por trás.",
		Reward: QuestReward{Items: items("potion_mp_l", 3)}},
	&QuestDef{ID: "tmo_necromantes", Title: "Quem levanta os mortos", Level: 26,
		Text:   "Os Necromantes Devotos chamam Lacaios sem parar enquanto os Acólitos curam quem está ferido. Derrube os que sustentam o canto: sem eles, os mortos não se levantam tão depressa.",
		Steps:  []QuestStep{kill("ossario_necromante|ossario_acolito", 12, "Derrotar Necromantes Devotos ou Acólitos do Ossário")},
		Done:   "Doze vozes a menos. Repare como o canto afina depois que um Necromante cai: o que ele puxa também precisa de quem puxe por ele.",
		Reward: QuestReward{Items: items("potion_hp_l", 3, "potion_mp_l", 3)}},
	&QuestDef{ID: "tmo_sino", Title: "O Sino Rachado", Level: 27,
		Text: "A Torre do Sino Rachado, a leste do cemitério, avisa quando o Rei Ossário está para acordar. Chegue até lá, abra caminho entre os Cavaleiros do Ossário que a guardam e veja se o sino ainda pende.",
		Steps: []QuestStep{visit(spotBell, "Chegar à Torre do Sino Rachado"),
			kill("ossario_cavaleiro", 6, "Derrotar Cavaleiros do Ossário")},
		Done:   "Ainda pende, então. Ele toca três vezes, dez minutos antes. É o tempo de um grupo chegar à Cripta. A Ordem confia nisso mais do que em qualquer vigia.",
		Reward: QuestReward{Items: items("scroll_return", 2, "potion_hp_l", 3)}},
	&QuestDef{ID: "tmo_selo", Title: "A primeira Pedra-Selo", Level: 28,
		Text: "A Ordem guarda uma das cinco Pedras-Selo que prendem o rei, e o Bastião precisa de mais braços para segurá-la. " +
			"Passe pelo Cemitério dos Reis Sem Nome e derrote os Cavaleiros que guardam as covas dos Devotos.",
		Steps: []QuestStep{visit(spotGraves, "Chegar ao Cemitério dos Reis Sem Nome"),
			kill("ossario_cavaleiro", 8, "Derrotar Cavaleiros do Ossário")},
		Done: "A Pedra-Selo segue de pé, com mais um dia de vida. Leve este caco: é da primeira Pedra-Selo, e a Guarda vai saber o que fazer com ele. " +
			"As outras quatro estão além das montanhas.",
		Reward: QuestReward{XP: boss, Items: items("seal_fragment", 1, "scroll_ench_weapon_b", 1)}},
)

package game

import "strings"

// Conversas com NPCs (tunnel#71). Clicar num NPC que tem um Talker abre a janela de
// conversa: a fala dele e as opções (aceitar e entregar missões, perguntar, o serviço que ele
// já tinha e sair). As falas vivem aqui, nos dados do servidor; o cliente só desenha o que
// chega no NpcDialog. A história que elas contam está em docs/historia-de-erevia.md.
//
// Dois tipos de Talker:
//   - os NPCs que já existiam na praça (Spawn == nil): a conversa é pelo nome. Quem tem
//     serviço (loja, armazém...) só abre a conversa quando tem missão para entregar ou
//     receber; senão o clique abre o serviço direto, como antes;
//   - os NPCs novos (Spawn != nil): o Arauto da praça, que dá as diárias, e um contador de
//     história por região, que dá e recebe as missões da linha dela. A zona os cria na ordem
//     desta lista, com ids depois do último NPC da cidade (zone/talk.go).

// TalkTopic é um assunto para perguntar ("Sobre os Arredores").
type TalkTopic struct {
	ID    string
	Label string
	Text  string
}

// TalkSpawn: onde e como a zona cria um NPC novo.
type TalkSpawn struct {
	Model string
	Look  *Appearance
	X, Z  float64
	Map   string // o mapa (a zona) onde ele fica ("" = o principal)
}

// Talker é a conversa de um NPC.
type Talker struct {
	Name, Title string
	Region      string // a região que ele conhece ("Vel'Aran" na praça)
	Greeting    string // a fala de quando o jogador chega
	Busy        string // o jogador tem uma missão dele em andamento
	Accepted    string // logo depois de aceitar uma missão dele
	Idle        string // nada a oferecer agora (a linha acabou ou o nível ainda não deu)
	Topics      []TalkTopic
	Service     string // o botão do serviço de sempre ("Ver as mercadorias"); "" = sem serviço
	Spawn       *TalkSpawn
}

// Topic: o assunto pelo id (nil se ele não fala disso).
func (t *Talker) Topic(id string) *TalkTopic {
	for i := range t.Topics {
		if t.Topics[i].ID == id {
			return &t.Topics[i]
		}
	}
	return nil
}

// Os nomes dos NPCs que dão missão (os passos de falar com eles e as entregas usam o nome).
const (
	TalkHerald   = "Cássio"   // o Arauto da Guarda, na praça: as diárias
	TalkMiller   = "Anselmo"  // Arredores
	TalkDigger   = "Marga"    // Campos dos Ossos
	TalkScout    = "Dárcio"   // Ermos Partidos
	TalkWeaver   = "Maelis"   // Vale das Pedras Vivas
	TalkMonk     = "Gaspar"   // Fundo do Vale
	TalkKnight   = "Bertrand" // Vigília Ossária
	TalkCaptain  = "Rovena"   // Confins Partidos
	TalkFerry    = "Tomé"     // Brejo de Lamagris (o Barqueiro, no Posto)
	TalkMerchant = "Lívia"
	TalkSmith    = "Bruno"
	TalkBoard    = "Iselda"
)

// Talkers: todas as conversas. Os com Spawn viram NPCs novos, nesta ordem (os ids seguem
// a ordem: acrescente no fim).
var Talkers = []*Talker{
	// ---- Praça de Vel'Aran: os NPCs de sempre ganham falas ----
	{Name: TalkMerchant, Title: "Mercadora", Region: "Vel'Aran", Service: "Ver as mercadorias",
		Greeting: "Poções, pergaminhos, um pouco de tudo. Quem sai pelos portões volta com fome de poção. Quando volta.",
		Idle:     "Precisando de alguma coisa? A bolsa de quem caça esvazia rápido.",
		Topics: []TalkTopic{{ID: "velaran", Label: "Sobre Vel'Aran",
			Text: "Vel'Aran foi erguida em cima de um campo de batalha, sabia? Meu avô dizia que a praça tem osso até o joelho da terra. " +
				"A muralha não é para segurar quem vem de fora: é para lembrar o que está embaixo."}}},
	{Name: TalkSmith, Title: "Ferreiro", Region: "Vel'Aran", Service: "Comprar e criar equipamento",
		Greeting: "Ferro bom, preço justo. Traga material e eu faço coisa melhor do que a que vendo.",
		Idle:     "A forja está quente. O que vai ser?",
		Topics: []TalkTopic{{ID: "ossos", Label: "Sobre os ossos e as pedras",
			Text: "Osso de esqueleto não apodrece nem quebra como deveria: é como se alguém tivesse pagado para ele durar. " +
				"E os Núcleos dos golens ainda batem na bigorna, juro. Quem fez aquilo sabia mais de pedra do que eu sei de ferro."}}},
	{Name: "Teodoro", Title: "Barbeiro", Region: "Vel'Aran", Service: "Mudar o visual",
		Greeting: "Cabelo, barba, a cara inteira, se quiser. Lá fora ninguém repara; aqui dentro, todo mundo.",
		Idle:     "Senta aí. Prometo não falar dos mortos enquanto corto.",
		Topics: []TalkTopic{{ID: "boatos", Label: "Boatos da praça",
			Text: "Dizem que os mortos levantam mais cedo a cada noite. Dizem também que a Mestra Ysolde não envelhece um dia sequer. " +
				"Eu só corto cabelo, mas escuto tudo."}}},
	{Name: "Otto", Title: "Guarda do Armazém", Region: "Vel'Aran", Service: "Abrir o armazém",
		Greeting: "O que entra aqui sai inteiro. Palavra da Guarda.",
		Idle:     "Seu baú está onde você deixou.",
		Topics: []TalkTopic{{ID: "guarda", Label: "Sobre a Guarda de Vel'Aran",
			Text: "A Guarda segura as muralhas e as estradas desde a fundação. A Ordem do Bastião vigia a Cripta; nós vigiamos todo o resto. " +
				"Hoje a Capitã Rovena está lá nos Confins, onde a terra mais rachou."}}},
	{Name: TalkBoard, Title: "Quadro de Contratos", Region: "Vel'Aran", Service: "Ver os contratos",
		Greeting: "O quadro paga por cada caçada que mantém as estradas abertas. Escolha a sua.",
		Idle:     "Os contratos de hoje estão no quadro.",
		Topics: []TalkTopic{{ID: "contratos", Label: "Sobre os contratos",
			Text: "Cada contrato é uma estrada aberta por mais um dia. Os do topo vêm com a reputação da Ordem e da Guarda; " +
				"os outros, com o que importa para quem ainda está subindo: experiência."}}},
	{Name: "Severino", Title: "Leiloeiro", Region: "Vel'Aran", Service: "Abrir a casa de leilões",
		Greeting: "Tudo tem preço, até o que ninguém quer. Principalmente o que ninguém quer.",
		Idle:     "Vai comprar ou vai vender?",
		Topics: []TalkTopic{{ID: "mercado", Label: "Sobre o mercado",
			Text: "Quem volta da Cripta vende fragmento de rei a preço de pão, porque precisa de poção. " +
				"Quem compra barato hoje fica rico amanhã. Isso é Vel'Aran."}}},

	// ---- NPCs novos (Spawn): o Arauto e os contadores de história ----
	{Name: TalkHerald, Title: "Arauto da Guarda", Region: "Vel'Aran",
		Spawn:    &TalkSpawn{Model: "knight", X: -9.5, Z: 0.5, Look: &Appearance{Skin: 3, Hair: 3, Outfit: 4, Head: "knight"}},
		Greeting: "Em nome da Guarda de Vel'Aran! Todo dia a muralha precisa de mãos, e o rol de hoje tem três serviços. Escolha os seus.",
		Busy:     "O rol de hoje ainda espera por você. Volte quando terminar, ou pegue outro serviço.",
		Accepted: "Anotado no rol. A Guarda paga assim que o serviço estiver feito: não precisa voltar a mim.",
		Idle:     "O rol de hoje está fechado para você. À meia-noite a Guarda escreve outro.",
		Topics: []TalkTopic{
			{ID: "historia", Label: "A história de Erevia",
				Text: "Há três séculos, os Ilvarin teciam runas e a pedra obedecia: ergueram os monólitos do Vale e deram vida aos golens. " +
					"Ozrhal, o rei de Erevia, quis uma coroa que não morresse com ele e pagou com os ossos do próprio reino. " +
					"Para detê-lo, os Ilvarin romperam os veios de runa sob os pés do rei. A terra rachou de um horizonte ao outro: é a Partição. " +
					"Ozrhal ficou preso na Cripta Esquecida, e o exército dele, plantado nos campos. Vel'Aran nasceu em cima do campo de batalha, para vigiar o selo."},
			{ID: "selo", Label: "O selo está rachando?",
				Text: "Está. Os mortos levantam toda noite, as pedras do Vale andam, o Senhor dos Ossos ronda o sul e há gente cantando na porta da Cripta. " +
					"Cada contador de história lá fora sabe um pedaço disso. Procure-os: o moleiro dos Arredores, a coveira dos Campos, " +
					"o batedor dos Ermos, a tecelã do Vale, os do Bastião e a capitã dos Confins."},
		}},
	{Name: TalkMiller, Title: "Moleiro dos Arredores", Region: "Arredores",
		Spawn:    &TalkSpawn{Model: "barbarian", X: 50, Z: 11, Look: &Appearance{Skin: 2, Hair: 5, Outfit: 8, Head: "barbarian"}},
		Greeting: "Ah, gente da cidade! Faz três noites que não durmo. Os ossos sobem do meu trigal, e ninguém lá dentro dos muros quer saber.",
		Busy:     "Ainda está nisso? O trigal não espera. Eles também não.",
		Accepted: "Que os seus braços sejam mais firmes que os meus. Eu fico aqui, de olho no trigal.",
		Idle:     "O moinho gira de novo, graças a você. Se os ossos daqui acordam, imagine os dos Campos, ao norte. Marga, a coveira, vive por lá.",
		Topics: []TalkTopic{{ID: "regiao", Label: "Sobre os Arredores",
			Text: "Os Arredores eram as fazendas de Vel'Aran: trigo, moinho, estrada. A batalha da Partição acabou aqui perto, " +
				"e os mortos ficaram rasos. Os Lacaios são os mais fracos do exército do rei, os que carregavam lança e balde. " +
				"De dia eles se arrastam; de noite, são muitos."}}},
	{Name: TalkDigger, Title: "Coveira dos Campos", Region: "Campos dos Ossos",
		Spawn:    &TalkSpawn{Model: "rogue", X: -6, Z: 90, Look: &Appearance{Skin: 4, Hair: 4, Outfit: 7, Head: "ranger"}},
		Greeting: "Cuidado onde pisa. Cada palmo destes campos é cova de alguém. Eu fecho as covas de dia, e elas amanhecem abertas.",
		Busy:     "As covas continuam abertas. Termine o que combinamos.",
		Accepted: "Vá com cuidado, e volte inteiro. Já tenho covas demais para fechar.",
		Idle:     "Por ora, os Campos dormem. O que acordou nos Ermos, a oeste, já não é do meu ofício: procure o batedor Dárcio.",
		Topics: []TalkTopic{{ID: "regiao", Label: "Sobre os Campos dos Ossos",
			Text: "Aqui o exército de Ozrhal caiu em fileira, e assim foi enterrado: soldado ao lado de soldado, como quem espera ordem. " +
				"Os Guerreiros e os Ladinos que você vê eram a infantaria do rei. Não estão perdidos: estão em formação."}}},
	{Name: TalkScout, Title: "Batedor da Guarda", Region: "Ermos Partidos",
		Spawn:    &TalkSpawn{Model: "rogue", X: -122, Z: -10, Look: &Appearance{Skin: 5, Hair: 3, Outfit: 2, Head: "rogue"}},
		Greeting: "Abaixe a voz. Nos Ermos a terra escuta. Sou Dárcio, da Guarda: conto o que sai das rachaduras e mando para a cidade.",
		Busy:     "O relatório ainda não fecha. Volte quando tiver o que eu pedi.",
		Accepted: "Combinado. Eu anoto tudo; você me traz o resto.",
		Idle:     "O arauto do rei falou de pedras que acordam ao sul. Desça a estrada até o Vale: a Ilvarin do acampamento sabe ler isso.",
		Topics: []TalkTopic{{ID: "regiao", Label: "Sobre os Ermos Partidos",
			Text: "Os Ermos são onde a Partição rachou mais perto da cidade. O chão abriu em fendas, e a corte do rei caiu dentro delas. " +
				"Os Magos Esqueletos eram os feiticeiros de Ozrhal: são eles que levantam os outros. " +
				"E ao sul, na estrada, anda o Senhor dos Ossos, o arauto que anuncia a volta do rei."}}},
	{Name: TalkWeaver, Title: "Tecelã de Runas", Region: "Vale das Pedras Vivas",
		Spawn:    &TalkSpawn{Model: "mage", X: -8, Z: -201, Look: &Appearance{Skin: 7, Hair: 6, Outfit: 5, Head: "mage"}},
		Greeting: "Você ouve? As pedras zumbem. Meus avós as fizeram obedecer, e agora elas lembram de tudo ao mesmo tempo.",
		Busy:     "A rede ainda está solta. Faça o que pedi, e eu leio o resto.",
		Accepted: "Obrigada. A rede não tem pressa, mas os golens têm.",
		Idle:     "Se os veios do Vale acordaram, o selo do rei está mais fraco do que a Ordem admite. Desça ao Fundo: o Irmão Gaspar vigia lá.",
		Topics: []TalkTopic{
			{ID: "regiao", Label: "Sobre o Vale das Pedras Vivas",
				Text: "Os monólitos do Vale são nós de uma rede de runas que os Ilvarin teceram sob a terra. Os golens eram os guardiões da rede. " +
					"Quando a Partição rompeu os veios, a rede afrouxou, e os golens ficaram sem ordem. Agora a rede puxa de novo, e eles andam."},
			{ID: "ilvarin", Label: "Quem são os Ilvarin?",
				Text: "Somos poucos. Depois da Partição, o meu povo se espalhou e a nossa cidade caiu ao norte, cantando. " +
					"Ninguém em Vel'Aran confia em quem rachou o mundo, mesmo que tenha sido para salvá-lo. A Mestra Ysolde sabe disso melhor que eu."},
		}},
	{Name: TalkMonk, Title: "Irmão do Bastião Cinzento", Region: "Fundo do Vale",
		Spawn:    &TalkSpawn{Model: "knight", X: 14, Z: -206, Look: &Appearance{Skin: 1, Hair: 5, Outfit: 7, Head: "mage"}},
		Greeting: "A Ordem do Bastião Cinzento vigia a Cripta desde a fundação. Eu vigio o Fundo, porque o Fundo treme quando a Cripta treme.",
		Busy:     "O Fundo ainda treme. Termine a sua parte.",
		Accepted: "Que a Ordem caminhe com você. Eu fico de vigia.",
		Idle:     "O tremor vem do norte, da Cripta. Sor Bertrand comanda a Vigília, no posto a sudoeste da Cripta. Diga que fui eu que mandei.",
		Topics: []TalkTopic{
			{ID: "regiao", Label: "Sobre o Fundo do Vale",
				Text: "O Fundo é a parte mais velha do Vale. Os Golens Ancestrais que dormem lá foram os primeiros que os Ilvarin acordaram, " +
					"e estão cobertos de musgo e de runas verdes. No meio deles há um círculo de pedras que nem os Ilvarin ousavam tocar."},
			{ID: "ordem", Label: "Sobre a Ordem do Bastião",
				Text: "Somos monges e somos soldados, e não sabemos mais qual dos dois vem primeiro. Juramos manter o rei onde está. " +
					"A Ordem guarda uma das cinco Pedras-Selo que os Ilvarin deixaram para prendê-lo. Das outras quatro, ninguém sabe."},
		}},
	{Name: TalkKnight, Title: "Cavaleiro do Bastião", Region: "Vigília Ossária",
		Spawn:    &TalkSpawn{Model: "knight", X: 79, Z: 79, Look: &Appearance{Skin: 3, Hair: 4, Outfit: 7, Head: "knight"}},
		Greeting: "Alto. Daqui para o norte é a Vigília Ossária, e depois dela, a Cripta. Os Devotos cantam dia e noite, e cada cântico afrouxa o selo.",
		Busy:     "O canto não parou. Volte quando a sua parte estiver feita.",
		Accepted: "Então está decidido. A Vigília conta com você.",
		Idle:     "A Vigília aguenta mais um pouco. Se quer ver onde a Partição dói de verdade, procure a Capitã Rovena, nos Confins.",
		Topics: []TalkTopic{
			{ID: "regiao", Label: "Sobre a Vigília Ossária",
				Text: "Os Cavaleiros Ossários eram a guarda pessoal de Ozrhal. Os Devotos, gente viva que se vendeu ao rei: " +
					"cantam para que o selo ceda, na esperança de serem os primeiros a ganhar a coroa que não morre. Nenhum deles vai ganhar."},
			{ID: "cripta", Label: "Sobre a Cripta Esquecida",
				Text: "Dentro do círculo de monólitos está a Cripta, e dentro da Cripta, o rei. De tempos em tempos Ozrhal acorda inteiro e é preciso um clã para fazê-lo dormir de novo. " +
					"Cada vez que ele cai, o selo ganha um respiro. Cada vez que ninguém vai, perde um."},
		}},
	{Name: TalkCaptain, Title: "Capitã da Guarda", Region: "Confins Partidos",
		Spawn:    &TalkSpawn{Model: "barbarian", X: -79, Z: 79, Look: &Appearance{Skin: 4, Hair: 2, Outfit: 1, Head: "knight"}},
		Greeting: "Capitã Rovena, da Guarda de Vel'Aran. Os Confins são os cantos do mundo, onde a Partição rachou mais fundo. Se veio lutar, chegou ao lugar certo.",
		Busy:     "As rachaduras não esperam. Termine o que eu pedi.",
		Accepted: "Ótimo. Volte com notícias, e volte de pé.",
		Idle:     "Por ora os Confins aguentam. O que vem depois está além das montanhas, em terras que ninguém de Vel'Aran pisa há três séculos.",
		Topics: []TalkTopic{
			{ID: "regiao", Label: "Sobre os Confins Partidos",
				Text: "Nos três cantos vazios do mapa a terra abriu até o osso do mundo. O que sai das rachaduras não é mais o exército do rei: " +
					"são os Partidos, mortos que a própria Partição deformou. Os Ceifadores cortam; os Arcanistas trazem o gelo das profundezas."},
			{ID: "pedras", Label: "Sobre as Pedras-Selo",
				Text: "Cinco Pedras-Selo prendem o rei, uma em cada canto das Terras Partidas. A Ordem tem a primeira. " +
					"As outras estão no Brejo, nas Minas de Brasaferro, na Costa e nas Ruínas de Ilvaren. Quando as estradas para lá abrirem, a Guarda vai precisar de você."},
		}},
	brejoTalker, // Brejo de Lamagris (tunnel#139): o Barqueiro
	// Bosque Gris e Terras Mortas (tunnel#137, bosque_story.go): o carroceiro, o Irmão do Bastião e a lápide do Ozrhal.
	bosqueTalker, bastionTalker, tombTalker,
}

// brejoTalker: Tomé, o Barqueiro, no Posto do Barqueiro (a entrada do Brejo). Fica no fim da lista (os ids
// dos NPCs seguem a ordem). O barco dele é a pedra do cais (zone/teleport.go).
var brejoTalker = &Talker{Name: TalkFerry, Title: "Barqueiro do Brejo", Region: BrejoRegion,
	Spawn:    &TalkSpawn{Model: "barbarian", X: 169, Z: -3, Look: &Appearance{Skin: 2, Hair: 6, Outfit: 3, Head: "barbarian"}},
	Greeting: "Cuidado com onde pisa, forasteiro. O Brejo come bota, e depois come o dono da bota. Eu levo e trago gente pela lagoa, e faço Antídoto para quem tem juízo.",
	Busy:     "A lagoa não espera, e eu também não. Termine o que combinamos.",
	Accepted: "Vá com cuidado. Quem volta do Brejo conta; quem não volta vira conversa de Afogado.",
	Idle:     "Por ora não tenho mais o que lhe pedir. O barco leva à Ilha dos Vaga-lumes por 10 moedas, e o Ferreiro Bruno faz o Antídoto, se você trouxer as glândulas.",
	Topics: []TalkTopic{
		{ID: "regiao", Label: "Sobre o Brejo de Lamagris",
			Text: "O Lamagris era um rio que ia da serra ao mar. Quando a Partição derrubou a encosta, ele se espalhou e se afogou em cima de Vau Velho, a minha aldeia. " +
				"A água parada guardou os mortos, e o veneno do selo do rei, que escorre pelos veios, levanta os Afogados de noite."},
		{ID: "brejeiros", Label: "Sobre os Brejeiros",
			Text: "Homens-lagarto, que sempre viveram aqui. Não servem a ninguém: defendem a água deles. De dia deixam o viajante passar; " +
				"à noite, com os Afogados rondando, entram em pânico e atacam quem vir. Bata em um deles de dia e o acampamento inteiro vem junto."},
		{ID: "barco", Label: "Sobre o barco",
			Text: "O bote ao lado do cais atravessa a lagoa até a Ilha dos Vaga-lumes, por 10 moedas a ida e 10 a volta. Lá à noite as luzinhas sobem do capim e dá até para esquecer onde se está."},
		{ID: "antidoto", Label: "Sobre o Antídoto",
			Text: "O veneno do Brejo derruba o mais valente em dois minutos. A Glândula de Sapo e o Junco-de-Prata, que só cresce na margem, dão um remédio que limpa o veneno na hora. " +
				"Eu ensinei a receita ao Ferreiro Bruno, na cidade: leve as duas coisas a ele."},
	}}

// TalkerByName: a conversa do NPC pelo nome (nil se ele não conversa).
func TalkerByName(name string) *Talker {
	for _, t := range Talkers {
		if strings.EqualFold(t.Name, name) {
			return t
		}
	}
	return nil
}

// SpawnedTalkers: os NPCs novos, na ordem dos ids.
func SpawnedTalkers() []*Talker {
	var out []*Talker
	for _, t := range Talkers {
		if t.Spawn != nil {
			out = append(out, t)
		}
	}
	return out
}

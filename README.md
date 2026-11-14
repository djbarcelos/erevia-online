# Erevia

MMORPG 3D no navegador. Cliente em **TypeScript + Babylon.js**, servidor em **Go**, comunicação por **WebSocket + Protobuf** e banco em **PostgreSQL**.

**Produção:** https://erevia-production.up.railway.app (deploy automático a cada push na `main`, depois que o CI passa).

Pesquisa e decisões de arquitetura: [RELATORIO.md](RELATORIO.md).

## Estado atual: Fase 4 (PvP, grupo, clãs, teleporte, encantamento e armazém)
- **PvP com flag e karma:** Ctrl + clique (ou o botão **Atacar** no quadro do alvo) ataca um jogador fora da cidade. Quem ataca fica com o nome **roxo** por 30 s. Matar quem não estava lutando dá **karma** (nome **vermelho**) e conta um PK; matar quem estava roxo conta uma vitória. Morrer para um jogador não custa XP. Quem tem karma perde o dobro de XP ao morrer e pode deixar cair de 1 a 3 itens (da bolsa ou equipados), reservados para quem matou; pode ser caçado dentro da cidade, não ataca lá e não usa teleporte. Caçar monstros limpa o karma. Abaixo do nível 6 ninguém luta contra jogadores; grupo e clã nunca se acertam; skills de área nunca acertam quem não está em combate. A auto-caça para se um jogador atacar. Karma, PKs e vitórias ficam na janela do personagem (C).
- **Grupo (tecla P):** até 6 pessoas, convite pelo botão **Grupo +** do quadro do alvo, pela janela ou com `/convidar Nome`. A XP do monstro vai para o grupo (ou jogador) que causou mais dano e é dividida pelo nível entre quem está vivo a até 30 m, com 10% de bônus por pessoa. O saque reservado vale para o grupo, e as moedas pegas são divididas. Painel do grupo no HUD, colegas em azul no nome e nos mapas, chat `/g`. Grito de Guerra, Estandarte e Aura de Coragem valem só para o grupo e o clã.
- **Clãs (tecla N):** criar custa 5.000 moedas e nível 10. Cargos: líder, oficial e membro (convidar, expulsar, promover, passar a liderança, aviso do clã, desfazer). Até 30 membros; quem sai espera 24 h para entrar em outro. O nome do clã aparece embaixo do nome; chat `/c`.
- **Pedras de Teleporte:** na praça e em acampamentos nas estradas (Arredores, Campos dos Ossos, Ermos Partidos), 20 a 160 moedas. Viajar leva 2 s; dano ou movimento cancelam. O **Pergaminho de Retorno** (Mercadora) leva à cidade em 5 s, com recarga de 30 s.
- **Encantamento:** pergaminhos de Arma e de Armadura (Ferreiro e drops) sobem o item até **+10**. Até +3 é seguro; depois a chance cai de 70% a 40%, e falhar quebra o item. O pergaminho **abençoado** (só drop) só volta a +0. Cada nível acima de +3 vale o dobro. Armas +4 brilham dourado e +7 pulsam.
- **Armazém:** Otto, o Guarda do Armazém, na praça: 60 espaços e moedas, compartilhados pelos personagens da conta. Arrastar da bolsa guarda; duplo clique retira.
- **Som:** golpes (acerto, crítico, erro), skills por tipo, mortes, passos, interface (janelas, cliques, compra e venda, moedas, itens, nível, poção, encantamento), ambiente (floresta, cidade com o ferreiro, grilos à noite, vento nas terras mortas) e uma música por região (cidade, ermos, terras sombrias e a Cripta), trocando com crossfade. Segue os volumes e o "Sem som" de Configurações → Áudio na hora; no celular começa no primeiro toque. No máximo 8 efeitos juntos (ganham os mais perto/do próprio jogador), e só se ouve os outros até 25 m. Tudo CC0 (créditos em `client/public/audio/CREDITS.md`), ~3 MB, com música e ambiente carregados só ao entrar na região.
- **Visual do personagem:** na criação, além da arma e do emblema, escolha o rosto e o cabelo (5 cabeças: Nobre, Selvagem, Sábio, Astuto e Andarilho, que qualquer classe pode usar), o tom de pele, a cor do cabelo e a cor da roupa (8 de cada, ou a original da classe) e se o elmo/gorro/chapéu aparece. Depois, Teodoro, o **Barbeiro** (ao norte do poço), muda o visual por 300 moedas, com prévia 3D. Todos veem o visual novo na hora.

### Nível 30 e o 5º degrau da árvore
- O nível máximo agora é **30**. Do 20 em diante, cada nível pede 10% a mais de XP por nível acima do 19 (até o 19 a curva não mudou), e a caça é no **Vale das Pedras Vivas**: os golens vão do nível 20, na entrada (norte), ao 30, no fundo (sul). No **nível 20** a árvore (tecla K) ganha um 5º degrau, uma skill por ramo (3 por classe, até rank 3): Baluarte, Corte do Horizonte e Égide da Ordem (cavaleiro); Machadada Sísmica, Redemoinho Sangrento e Pele de Pedra (bárbaro); Inferno, Zero Absoluto e Rajada Arcana (mago); Golpe Fatal, Golpe das Sombras e Miasma (ladino).

### Maestria (depois do nível 30)
- No **30** a XP (de monstro, missão e invasão, com as regras de sempre: grupo, diferença de nível, noite e XP em dobro) enche a **barra de Maestria**, roxa, no lugar da barra de XP do HUD ("Maestria N"). Cada barra cheia dá **1 ponto**; uma barra custa 263.873 de XP, o mesmo que o 29 → 30 (`game.MasteryPointXP`, a rever com a curva nova). Morrer no 30 tira 4% da barra, sem tirar ponto (com karma, o dobro; para outro jogador ou defendendo a cidade, nada).
- **Bônus diário** (como o Enlightenment do ESO): a primeira barra de cada dia (meia-noite de Brasília) sai **4× mais rápido**, e quem fica fora guarda até **7 dias**. A dica da barra mostra quanto ainda resta.
- **Árvore** (aba **Maestria** da janela de skills, K, ou clique na barra): 3 ramos, 4 nós cada, até 5 pontos por nó com retorno decrescente (40%, 25%, 15%, 12% e 8% do total do nó); os nós do meio e do fim pedem 5 e 10 pontos no ramo. Teto: Maestria 60 (a árvore inteira).
  - **Caça:** Alquimia de Campo (poções curam +20%), Bolsa Farta (+15% de moedas dos monstros), Olho do Coletor (15% de chance de 1 material a mais de monstros comuns; 5 pts), Instinto do Caçador (+2% de dano; 10 pts).
  - **Sobrevivência:** Firmeza (atordoamento e lentidão −25%), Segunda Chance (−50% de perda da barra ao morrer), Fôlego (+40% de regeneração fora de combate; 5 pts), Couro Curtido (+1,5% de vida; 10 pts).
  - **Utilidade:** Passo Leve (+4% de velocidade fora de combate, o teto), Viajante (Pedras de Teleporte −30%), Leitura Rápida (pedra e Pergaminho de Retorno −40% de tempo; 5 pts), Caminho de Casa (recarga do Pergaminho de Retorno −50%; 10 pts).
  - **Teto de combate:** tudo o que mexe no combate (dano, defesa, vida, crítico, velocidade de ataque), com a árvore inteira, soma no máximo **5%** (hoje 3,5%; `TestMasteryCombatCap` e `TestMasteryCombatCapOnRealStats` travam).
  - **Redistribuir:** grátis, 1 vez por dia.
- O ranking de nível mostra a Maestria ao lado do nível (e ela desempata no 30). Cada ponto ganho vira um evento `mastery` na telemetria. Tudo fica salvo em `characters.mastery` e `characters.mastery_level` (migração 00017); nada roda por tick: só muda quando o jogador ganha XP, morre, gasta ou redistribui. GM: `/maestria xp N [Nome]` e `/maestria dia [Nome]` (simula a virada do dia). Dados em `server/internal/game/mastery.go`.

### Criação de itens (Ferreiro)
- A aba **Criar** do Ferreiro Bruno transforma materiais que caem dos monstros + moedas em equipamento que a loja não vende: 24 receitas, de raro (ossos e crânios dos esqueletos) a épico (Essência Sombria) e lendário (Núcleo Rúnico dos golens), mais pergaminhos de encantar (inclusive os abençoados). A lista mostra quanto você tem de cada material; o botão só liga quando dá.
- O servidor confere materiais, moedas e espaço numa cópia da bolsa e só então aplica. Receitas em `server/internal/game/recipes.go`.

### Troca entre jogadores
- Selecione um jogador perto (até 8 m) e clique em **Trocar** no quadro do alvo; ele recebe o convite. Na janela de troca, arraste itens da bolsa (ou use "Oferecer na troca" no menu do item), ponha moedas, **trave** a oferta e, com as duas travadas, **confirme**. Qualquer mudança (inclusive usar um item oferecido) destrava as duas ofertas.
- Nada sai da bolsa até a confirmação dos dois. Na hora, o servidor confere tudo de novo, monta as bolsas novas e aplica de uma vez, salvando os dois personagens na mesma transação. Se alguém sair, morrer, se afastar ou não tiver espaço, a troca é cancelada sem mexer em nada.

### Amigos, correio e emotes (janela Social, tecla O)
- **Amigos:** `/amigo Nome` (pode estar offline), o botão **Amigo +** no quadro do alvo ou a janela Social; `/amigo remover Nome` tira. A amizade é de mão única (como "seguir"), até 50 por personagem. A janela mostra quem está online, o nível e a região onde está (atualiza a cada 10 s com ela aberta); `/amigos` lista no chat quem está online. Quando um amigo entra no jogo, aparece um aviso no chat.
- **Correio:** `/correio Nome texto` ou a aba Correio (com assunto opcional). Só texto, até 200 caracteres; o destinatário pode estar offline e vê um aviso ao entrar (e um número no botão Social). Na caixa de entrada, abrir a carta marca como lida; dá para responder e apagar. Contra spam: 10 cartas por hora por remetente (apagar não devolve o limite) e 100 cartas guardadas por destinatário. Quem está silenciado não manda cartas.
- **Emotes:** `/acenar`, `/sentar` e `/dancar` (ou `/dançar`), vistos por quem está a até 70 m. Sentar e dançar duram até o personagem andar, atacar ou morrer. Os modelos não têm clipes próprios: acenar usa o braço erguido da magia, dançar a conjuração em loop e sentar é uma pose montada no cliente com o fim do `Death_A` (pernas) e o `Idle_A` (tronco).
- Amigos e cartas ficam nas tabelas `friends` e `mail`, gravadas por um worker (a zona nunca espera o banco).

### Missões
- **Tutorial dos primeiros 15 minutos** (8 missões em sequência): sair da cidade, derrotar Esqueletos Lacaios, usar skills, tomar uma poção, comprar na praça, chegar ao nível 4, viajar com uma Pedra de Teleporte e enfrentar os Campos dos Ossos. Cada uma dá XP, moedas e/ou itens; se a bolsa estiver cheia, a recompensa fica no chão, reservada para você.
- O painel **Missões** fica à esquerda, abaixo do grupo, e mostra o progresso (passe o mouse para ver a história e a recompensa). Só personagens novos (abaixo do nível 5 e sem histórico) recebem o tutorial.
- **Diário de missões (tecla J):** botão no painel Missões, na barra das janelas e no menu ☰ do celular. As ativas ficam agrupadas por categoria (história, região, diária e tutorial; `QuestDef.Category`, que vai no QuestInfo), cada uma com o texto, os passos com progresso, o lugar do passo, a recompensa e os botões **Rastrear**, **Fixar** e **Ir**. A aba **Concluídas** mostra só os títulos: o servidor manda id, título e categoria no primeiro QuestLog da sessão e quando uma missão termina, nunca o texto. O painel do HUD mostra só as **fixadas** (até 3) e a rastreada está sempre entre elas; missão nova entra sozinha se houver lugar, senão aparece por 8 s com **Fixar** e some. Fixar com o painel cheio tira a fixada mais antiga que não é a rastreada. A escolha fica no navegador por personagem (`erevia.questPins.<id>`) e sobrevive a relogar. A janela só existe aberta e o painel só redesenha com QuestLog ou fixação nova (`client/src/ui/quest-pins.ts`, testes em `client/tests`; `client/src/ui/windows/journal.ts`).
- As missões são dados em `server/internal/game/quests.go` (objetivos: sair da cidade, matar monstros, usar skills, tomar poções, comprar, chegar a um nível, viajar). O servidor controla o progresso e salva no personagem (`characters.quests`).
- **Onde fica cada objetivo:** cada passo pendente vai com um lugar (x, z, raio e tipo: área de caça, NPC, portão, Pedra de Teleporte ou loja), calculado pelo servidor só quando o QuestLog é enviado (ao mudar o passo e ao subir de nível, nunca por tick): matar → a área de spawn do monstro mais adequada ao nível e mais perto (no anel, o ponto na direção do jogador, longe do covil do raid); chegar a um nível → a melhor área de caça para o nível atual; falar, escolher o caminho ou comprar → o NPC (a loja mais perto que vende o item); sair da cidade → o portão mais perto; viajar → a Pedra de Teleporte da praça; usar skill e tomar poção → sem lugar. Uma missão de história pode fixar o lugar nos dados (`QuestStep.Place`). Código em `server/internal/game/questplace.go` e `server/internal/zone/questplace.go`.
- **Missão rastreada:** clicar numa missão do painel (ou **Rastrear** no Diário) a rastreia (uma por vez, sempre entre as fixadas; sem escolha, a primeira fixada; a escolha fica no navegador). O minimapa e o mapa do mundo mostram a área de caça como um círculo dourado tracejado, o NPC, a loja, a Pedra de Teleporte ou o portão com um ícone e uma linha tracejada de você até lá (no minimapa, o ícone fica preso na borda quando o alvo está longe; no mapa, o painel lateral tem a missão rastreada e o clique centraliza nela). Fora de vista, uma **seta na borda da tela** aponta para o alvo com a distância em metros. Quando o passo termina, o alvo avança sozinho. Configurações → Jogo → **Marcadores de missão** desliga tudo. Custo: só os canvas 2D que já existiam (o minimapa a ~15×/s) e um elemento DOM para a seta, mexido no máximo 5×/s; nenhuma malha 3D (`client/src/ui/quest-nav.ts`).
- **Ir e Caçar aqui:** cada missão do painel (e a rastreada no mapa do mundo) tem o botão **Ir**: o personagem anda sozinho até o lugar do passo, com o mesmo comando do clique no mapa (o servidor calcula o caminho e valida tudo). Para quando você mexe no movimento (WASD, joystick, clique no chão ou no mapa) ou ataca, quando chega (entra no raio do lugar) e quando leva dano (Configurações → Jogo → **Continuar andando se atacado**, desligado por padrão, deixa seguir). Numa área de caça aparece **Caçar aqui**: a auto-caça só com os monstros daquele passo (o QuestStepInfo leva os nomes em `monsters`), até o passo terminar; aí avisa no chat e para. Pelo F/AUTO a auto-caça continua igual. Lugar a mais de ~150 m com uma Pedra de Teleporte que encurte o caminho: a pílula oferece **Viajar pela Pedra (X moedas)**, que anda até a pedra e abre a janela com o destino destacado (quem escolhe é o jogador); depois da viagem, segue a pé. Contra bot: só com a janela do jogo em foco, e para depois de 5 minutos sem nenhuma tecla, clique ou toque de verdade. No cliente, um intervalo de 250 ms só enquanto o Ir está ativo, nada por quadro (`client/src/world/questgo.ts`, testes em `client/tests`, `npm run test:unit`).
- **Caminhos longos:** o A* de 1 m usa uma grade de memória reaproveitada (antes eram mapas criados a cada busca: ~4× mais rápido e quase sem alocação), e pedidos de movimento acima de 40 m com obstáculo no meio entram numa fila de no máximo 2 caminhos por tick, um lugar por jogador (`server/internal/zone/pathqueue.go`). Com 50 jogadores pedindo o vale ao mesmo tempo, o pior tick caiu de ~49 ms para ~2 ms (`go test ./internal/zone -run x -bench TickLongPaths`).

### Ranking, conquistas e títulos (tecla H)
- A janela **Glória** (H ou o troféu no menu) tem duas abas. **Ranking:** top 50 de nível (desempate pela XP), de vitórias PvP e de clãs (soma dos níveis dos membros, com a média), mais a sua posição e a do seu clã. Um worker calcula no banco a cada 5 min (contas banidas ficam de fora) e a zona guarda em memória; a janela pede quando abre e guarda a resposta por 2 min. A zona nunca consulta o banco.
- **Conquistas** (13, dados em `server/internal/game/achievements.go`): Primeiro Sangue, Primeiros Passos (tutorial), Quebra-Ossos (100 esqueletos), níveis 10, 20 e 30, O Senhor Caiu (Senhor dos Ossos), Regicida (participar da morte de Ozrhal), A Cidade Resiste (defender uma invasão vencida), Mão Firme (+7), Negócio Fechado (primeira troca), Forja Lendária (criar um lendário) e Duelista (vitória PvP). Andam com o que o servidor já valida (abate, nível, encantamento, troca, criação, missão, PvP, raid boss e invasão); quem já tinha cumprido (nível, vitórias, tutorial) ganha ao entrar. Desbloquear avisa no chat e com um toast.
- Cada conquista dá um **título**: escolha na aba Conquistas e ele aparece sob o seu nome, em lilás, para todos (texto fixo no nome, que só muda quando você troca de título). Conquistas e título ficam salvos no personagem (`characters.achievements` e `characters.title`).

### Eventos: invasão da cidade e XP em dobro
- **Invasão:** em algumas noites (chance de `INVASION_CHANCE` a cada anoitecer, 15% por padrão, só com alguém online) ou com `/evento invasao` (GM), um aviso para todos (o mesmo banner do raid boss) diz que os esqueletos chegam em 2 minutos. Depois vêm **4 ondas** (12, 16, 20 e 22 esqueletos, a última com um Senhor dos Ossos de capitão), no nível médio de quem está perto da cidade (−1, +1 por onda). Eles nascem do lado de fora dos 4 portões, um por portão a cada 2 s, com **no máximo 20 vivos** de cada vez, e marcham para a praça brigando com quem encontram. Enquanto dura, as **estradas dos portões e a praça** deixam de ser zona segura para lutar com monstros; o resto da cidade continua seguro e o **PvP continua proibido** em toda a cidade (karma e flag não mudam). Morrer para um invasor não custa XP, e quem morre volta num canto seguro, não no meio da luta.
- **Fim e recompensa:** acaba quando as ondas caem (vitória) ou em 8 minutos (os esqueletos recuam e somem). Os invasores não deixam saque no chão; no fim, cada defensor ganha XP (até 12% do nível) e moedas (até 50 + 50 por nível) proporcionais aos seus pontos (dano nos invasores + 25% da vida de cada um que derrubou) em relação a quem mais fez; na vitória, os 3 melhores ganham um pergaminho de encantar e todos os recompensados, a conquista **A Cidade Resiste** (título "Defensor de Vel'Aran"). Tempo esgotado paga metade e sem itens. A XP de cada abate continua normal (e dobra com a XP em dobro). Tudo vai para a telemetria (`invasion`, ouro e itens com origem `invasion`).
- **XP em dobro:** a XP de monstros (inclusive grupo e raid) dobra nos dias de `XP2_DAYS` (sábado e domingo por padrão, no horário de Brasília) ou com `/evento xp2 horas` (GM, até 168 h); `/evento xp2 off` desliga, inclusive o horário do dia (até o fim da sequência de dias). Ligar e desligar avisa todos; enquanto dura, um selo **XP ×2** fica embaixo do minimapa (a invasão mostra ali a onda, os vivos e o tempo). O que o GM liga ou desliga fica na tabela `server_events` (migração 00016) e vale depois de um reinício.
- **Desempenho:** sem invasão o tick não piora (`go test ./internal/zone -run x -bench TickCrowd`: ~2,7 → ~2,5 ms, porque a busca de alvo dos monstros agora descarta pela distância antes). Com a invasão no limite de 20 vivos e 100 dos 400 jogadores na praça, o tick sobe uns 0,25 ms (de ~4,0 para ~4,25 ms, num orçamento de 50 ms; `BenchmarkTickCrowdInvasion`). No cliente não há efeito novo: são os monstros de sempre, entregues pela AOI, e o selo do HUD só muda de texto 2×/s.
- `/evento` mostra o estado; `/evento invasao agora` pula o aviso e `/evento invasao parar` encerra sem recompensa. Tudo fica no `gm_log`.

### Segunda classe (especialização, nível 20)
- No **nível 20** chega a missão **A Encruzilhada**: falar com Ysolde, a **Mestra dos Caminhos** (praça, a leste do poço), abre a janela com os dois caminhos da classe, e escolher conclui a missão (1.000 moedas e 3 Pergaminhos de Retorno). Quem já passou do 20 recebe a missão ao entrar. A escolha é definitiva; trocar depois custa **50.000 moedas** e devolve os pontos das skills do caminho antigo.
- Cada caminho dá um **bônus de atributos** e um ramo de **3 skills** nos níveis 20, 24 e 28 (embaixo da árvore, tecla K), feitas só com mecânicas que já existiam:
  - Cavaleiro: **Bastião** (+12 Constituição: Desafio do Bastião, Vigor do Bastião, Último Bastião) ou **Arauto** (+12 Sabedoria: Martelo da Aurora, Bênção do Arauto, Sol Nascente).
  - Bárbaro: **Sanguinário** (+12 Força: Golpe Rubro, Fúria Cega, Carnificina) ou **Caudilho** (+6 Força e +6 Constituição: Brado do Caudilho, Avanço do Caudilho, Estandarte de Guerra).
  - Mago: **Elementalista** (+12 Inteligência: Lança Ígnea, Foco Elemental, Cataclismo) ou **Ocultista** (+6 Inteligência e +6 Espírito: Peste Sombria, Drenar Vontade, Marca do Abismo).
  - Ladino: **Sicário** (+12 Destreza: Lâmina Oculta, Instinto Assassino, Sentença) ou **Caçador** (+6 Força e +6 Destreza: Faca de Arremesso, Rede de Caça, Chuva de Lâminas).
- Todos veem: o nome do caminho aparece sob o nome do personagem (junto do título), no quadro do alvo (na cor do caminho) e na janela do personagem (C). Vai no `PlayerStatus`, que só sai quando alguém escolhe ou troca; os efeitos das skills novas reaproveitam as receitas de partículas que já existiam.
- Dados em `server/internal/game/specs.go` e `spec_skills.go`; a escolha fica salva em `characters.spec` (migração 00015).

### Base para crescer (moderação, /bug, reconexão e CI)
- **Comandos de GM** (contas com `accounts.gm`, ver [Moderação](#moderação)): `/gm` lista os comandos; `/kick Nome [motivo]`, `/mute Nome minutos [motivo]` e `/unmute Nome`, `/ban Nome [motivo]` (vale para a conta inteira, mesmo offline, e derruba quem está online) e `/unban Nome`, `/tp X Z`, `/goto Nome`, `/summon Nome`, `/give Nome item|moedas [qtd]`, `/anuncio texto`, `/online`, `/evento` (invasão e XP em dobro, ver [Eventos](#eventos-invasão-da-cidade-e-xp-em-dobro)) e `/maestria xp N [Nome]` / `/maestria dia [Nome]` (teste da Maestria). Para quem não é GM, eles aparecem como comando desconhecido. Toda ação fica no `gm_log`. O silêncio vale em todos os canais e continua depois de relogar.
- **`/bug texto`:** qualquer jogador (mesmo silenciado), um a cada 30 s. Vai para `bug_reports` com o personagem, o nível, a posição, a versão do servidor (commit do deploy) e o navegador. O cliente manda junto o contexto (erros recentes, FPS, ping, região, hora do jogo, últimas mensagens do servidor) e um print pequeno da tela do jogo; no celular, **Configurações → Sobre → Relatar problema** faz o mesmo sem teclado (ver [/bug completo](#bug-completo)).
- **Reconexão automática:** se a conexão cair, o jogo espera o servidor responder e volta sozinho ao mesmo personagem (desiste depois de 2 min com o servidor fora do ar ou de 5 recargas). Quem foi expulso ou banido não volta.
- **CI:** cada push e pull request roda os testes no GitHub Actions (servidor com Postgres e `-race`, tipos e build do cliente, e o roteiro no navegador com Playwright). A Railway só publica a `main` com o CI verde.

### Fase 3 e interface nova (Fases A–C)
- **13 encaixes de equipamento:** cabeça, peito, pernas, mãos, pés, capa, arma, mão secundária, colar, 2 brincos e 2 anéis. Anéis e brincos vão no encaixe livre com duplo clique, ou no que você escolher arrastando; dá para trocar anel 1 ↔ anel 2 arrastando. Itens novos nas 4 raridades com bônus novos: precisão, esquiva, velocidade de movimento, de ataque e de conjuração, regeneração de vida e mana e dano crítico.
- **Aparência na criação:** cada classe escolhe a arma inicial (ex.: espada ou lâmina curva para o cavaleiro; cajado, varinha ou cajado sombrio para o mago; três pares de lâminas para o ladino), o escudo (ou nada, e ele vai para a bolsa) e a cor do emblema. A arma é um item de verdade do kit. A cor pinta o nome e um anel no chão para todos verem, e a tela de seleção mostra cada personagem com o que tem nas mãos.
- **Atributos com pontos livres (tecla C):** Força, Destreza, Constituição, Inteligência, Sabedoria e Espírito. Cada classe começa com a sua base e ganha 3 pontos por nível. A janela mostra a diferença exata antes de confirmar (vida, ataque, defesa, crítico, precisão, esquiva, velocidade de ataque e de conjuração, regeneração).
- **Árvore de skills (tecla K):** 15 skills por classe em 3 ramos e 5 degraus (níveis 1, 5, 10, 15 e 20), com ranks; 1 ponto por nível. Mecânicas: prender, veneno/queimadura/praga (dano por segundo), provocar, escudos, escudo de mana, invisibilidade, marca, imortal, cegueira, frenesi, enfraquecer, armadura de gelo, grito de guerra e estandarte para os aliados, aura de coragem, Piscar e Investida (teleporte), execução, golpes duplos, área em volta do alvo, roubo de vida e mais. Uma bolinha nos botões avisa quando há pontos para gastar.
- **Barra de atalhos salva no servidor:** a mesma em qualquer computador ou celular.
- **Interface nova** (protótipo aprovado em `/prototipo.html`, agora no jogo):
  - Telas de login e de personagens com o modelo 3D num pedestal (girar arrastando, zoom com a roda ou a pinça).
  - HUD: personagem no canto superior esquerdo; minimapa no direito (clique abre o mapa) com zoom, X/Y, FPS e ping; barra de **10 atalhos** (teclas 1–0, skills e poções, arraste para trocar; botão direito tira) ao lado do botão **AUTO** e dos botões das janelas; chat com abas; avisos de saque, XP e nível.
  - Janelas (arrastáveis no PC, tela cheia no celular): **Inventário (I)** com a miniatura 3D do equipamento, **Personagem (C)**, **Skills (K)**, **Mapa do mundo (M)** com regiões, chefes, lojas e clique para andar, e **Configurações (Esc)** com teclas configuráveis, áudio/gráficos e as opções da auto-caça.
  - **Celular:** joystick, botão de ataque com as skills em volta, poção, trocar alvo, AUTO e menu ☰; arrastar o dedo gira a câmera e a pinça dá zoom. Liga sozinho em telas de toque (ou em Configurações → Controles).
  - **Auto-caça (F ou AUTO):** escolhe o monstro mais perto no raio configurado, ataca, usa as skills da barra, toma poção abaixo do limite, pega o saque e volta à cidade sem poção. Roda no navegador e só manda os comandos normais (o servidor valida tudo).
- **Itens e moedas:** cerca de 75 itens em 4 raridades (comum, bom, raro e épico): armas de cada classe, escudos, armaduras, elmos, pernas, luvas, botas, capas, joias, poções e materiais. A arma e o escudo aparecem na mão do personagem; armas raras e épicas brilham.
- **Drops:** monstros deixam moedas e itens no chão (saquinho com o nome; feixe de luz nos itens bons). Por 15 s, só quem causou mais dano (ou o grupo dele) pode pegar; depois fica livre, e some em 60 s. Clicar no item faz o personagem andar até ele e pegar.
- **Inventário (tecla I):** 40 espaços e os 13 encaixes ao redor da miniatura 3D, com os atributos embaixo. Arrastar e soltar move, troca, equipa e destrói; duplo clique usa ou equipa.
- **Poções nas teclas 3–6:** arraste uma poção do inventário para a barra. Vida e mana têm recargas separadas (10 s).
- **Loja:** a Mercadora Lívia (poções e joias) e o Ferreiro Bruno (armas, escudos e armaduras comuns e boas) ficam na praça. Clicar num deles abre a loja ao lado do inventário, com filtro por categoria e "só da minha classe"; vender paga 40% do preço.
- **Kit inicial:** a arma (e o escudo) escolhidos na criação, equipados, 5 poções e 50 moedas.
- **Persistência segura:** moedas, itens, karma e o armazém da conta são salvos junto com o personagem numa única transação; clãs são gravados em ordem por um worker do banco. Um salvamento atrasado nunca sobrescreve um mais novo, e relogar rápido usa o estado da memória (não perde nem duplica itens). O servidor é o único que cria ou destrói itens.

### Fases anteriores
- **Cenário:** mapa de 300 × 300 m. Cidade murada (raio 38 m, 4 portões) com taverna, mercado, ferreiro, igreja, 16 casas e torres, em escala de personagem (casas de ~7 m, taverna de ~12 m). Estradas de terra e floresta que fica mais seca e sombria longe da cidade. Montanhas na borda, céu de entardecer e sombras. Tudo desenhado com *thin instances* (cerca de 1.800 objetos, 60 FPS). Árvores altas (cerca de 9–14 m; as secas mais distantes chegam a 19 m) e vegetação espaçada.
- **Colisão e caminho:** não dá para atravessar casas, muralhas, árvores e pedras grandes. O servidor calcula o caminho (A* numa grade de 1 m), então clicar do outro lado de uma casa faz o personagem contorná-la.
- **Combate PvE:** a cidade (zona segura) fica no centro, cercada por 3 anéis de esqueletos (níveis 1 a 20) e pelo chefe *Senhor dos Ossos* (nível 22).
- Clicar num monstro seleciona o alvo e ataca sozinho. As **teclas 1 e 2** usam as 2 skills da classe, com recarga e mana. Os efeitos incluem atordoar, lentidão e buffs.
- **XP e nível** até 30 (era 20 até o 5º degrau da árvore). Os atributos crescem com o nível. Morrer custa cerca de 4% da XP do nível e o personagem volta à cidade.
- Monstros vagam, atacam quem chega perto (os agressivos), perseguem até um limite e reaparecem depois de mortos.
- Barras de vida, mana e XP, quadro do alvo, números de dano e efeitos de skill.
- Login com **Google** ou **Discord** (OAuth + PKCE). A sessão fica num cookie HttpOnly.
- Até **3 personagens** por conta, cada um com nome único e uma classe: cavaleiro, bárbaro, mago ou ladino.
- **Chat:** canal geral, `/w Nome texto` para sussurrar e `/r texto` para responder. Clicar num nome no chat prepara o sussurro.
- O personagem é salvo no banco ao sair, a cada 30 s, logo depois de comprar, vender ou pegar itens, e quando o servidor desliga.
- Se o mesmo personagem entrar em outro lugar, a sessão antiga cai com um aviso.
- O servidor decide tudo: movimento, limites do mapa, limite anti-spam e anti-flood. Clientes lentos são desconectados.
- **Área de interesse:** cada jogador só recebe jogadores e monstros a até 70 m (com 10 m de folga) e os colegas de grupo em qualquer lugar. Quem entra no raio chega com um Spawn, quem sai some com um Despawn, e o snapshot de cada um leva só o que ele conhece. Chat, anúncios, chefes, NPCs e itens no chão continuam para todos. Isso deixa a banda por jogador quase constante com o número de jogadores (ver `cmd/loadbot`).
- **Painel de desempenho:** tecla **F3** (ou `?debug=1` na URL) mostra tempo de frame, draw calls, malhas ativas e o tempo gasto em animação e na interface.
- **Medição de FPS:** `?bench=1` na URL, ao entrar no mundo, leva a câmera por cada região, de dia e de noite, e no fim mostra uma tabela (FPS, tempo de frame, p95, CPU, draw calls, malhas e onde vai o tempo de CPU) com o botão **Copiar**. Leva uns 2 minutos; o personagem não sai do lugar. Serve para comparar antes e depois de cada mudança no cliente, no PC e no celular.
- **Spawns por setor, por período e com área própria** (`game.Spawn`, base das áreas novas do [roadmap](docs/roadmap-areas.md)): além de anel, ponto fixo e disco, um spawn pode ser um **setor de anel** (`AngleMin`/`AngleMax` em graus; 0° = leste/+X, crescendo para o norte/+Z, ou seja 90° = norte, 180° = oeste, 270° = sul; `AngleMin > AngleMax` cruza o 0°, como 340°→20°). Disco ou setor com `Region` tem **dono**: os anéis das outras regiões (e o covil do raid) não sorteiam monstros lá dentro, com 6 m de folga, e quem cairia lá muda de lugar sem mudar o id. Com `When` (`WhenDay`/`WhenNight`), na virada do dia os monstros do outro período que estão **fora de combate** saem do mundo (sem morrer, sem saque) e os do período novo entram no ponto de origem; quem está lutando fica até a luta acabar (ou morrer) e segura a vaga de um do período novo, que entra no mesmo segundo em que ele sai. A troca é por região, então com a mesma quantidade de dia e de noite em cada região a contagem de monstros fica igual. O id de cada monstro é a posição na lista: spawn novo vai sempre para o **fim** de `DefaultSpawns`. O mapa do mundo desenha o setor com nome e faixa (`MapZone.angle_min`/`angle_max`), o minimapa mostra o nome ao entrar e o mapa marca "(à noite)"/"(de dia)" nos monstros de um período só. Código em `server/internal/game/monsters.go` e `server/internal/zone/spawnperiod.go`.
- **Qualidade gráfica (Configurações → Gráficos):** presets baixa, média e alta, escolhidos pelo aparelho na primeira vez (celular → baixa ou média; computador → alta) e aplicados na hora. Todos têm tons ACES e cor/névoa por região e hora; a baixa renderiza a 80% sem brilho nem SSAO, a média liga o brilho e a vinheta, a alta soma SSAO e FXAA. Para medir: `?bench=1&quality=baixa` (força o preset sem salvar) e `?bench=1&render=antigo` (o render de antes).

## Privacidade e exclusão de conta (LGPD)
- **Páginas legais:** [`/privacidade.html`](client/privacidade.html) e [`/termos.html`](client/termos.html), linkadas na tela de login e na de personagens. Por enquanto o responsável, o CNPJ, o e-mail (`contato@erevia.example`) e o foro são **fictícios** e precisam ser trocados pelos reais antes de abrir ao público.
- **O que o banco guarda do login:** só o provedor, o id da conta nele e o nome de exibição (Google `openid profile`, Discord `identify`). Sem e-mail. O único cookie é o de sessão (`HttpOnly`); o chat não é gravado.
- **Apagar a conta:** tela de personagens → **Conta** → digitar `APAGAR`. O servidor (`DELETE /api/account`, com `{"confirm":"APAGAR"}`, JSON e mesma origem como os outros pedidos de escrita) apaga numa transação a conta, identidades, sessões, personagens, itens, armazém e participação em clãs, e tira o nome dos personagens do registro dos raid bosses. O clã liderado por alguém da conta passa ao membro de cargo mais alto (o mais antigo no empate) ou é desfeito se ficar vazio; o banco não deixa apagar um líder sem isso (migração 00020). Depois a zona derruba quem estiver online com a conta (aviso "Esta conta foi apagada."), esquece o estado em memória e recusa um Join atrasado; salvamentos atrasados não acham a linha e não recriam nada.
- **Telemetria e correio ao apagar:** os `game_events` da conta ficam anônimos (sem `account_id` e `character_id`), para as estatísticas agregadas continuarem. Um lote de eventos que chega depois da exclusão (ex.: o fim de sessão de quem foi derrubado) também entra anônimo: `InsertEvents` trava as contas do lote (`FOR KEY SHARE`) e zera as que não existem mais. As cartas enviadas ficam com quem recebeu, assinadas "Personagem apagado". O `gm_log` (moderação) é mantido. Os relatos de `/bug` da conta somem com ela, junto com o contexto e o print (cascata de `bug_reports`).
- **Backups:** diário guardado 6 dias e semanal guardado 27 dias, então dados apagados podem ficar até 27 dias nas cópias. Se restaurar uma cópia, apague de novo as contas excluídas depois dela.

## Novidades (changelog)
- **Fonte:** [`CHANGELOG.md`](CHANGELOG.md), uma versão por dia (`## AAAA-MM-DD · Título`) com `### Novidades`, `### Melhorias` e `### Correções`, em linguagem de jogador. O formato está comentado no topo do arquivo.
- **No jogo:** a janela **O que há de novo** abre sozinha uma vez quando a versão mais nova é diferente da última vista (guardada no navegador; sem storage, ela não abre). Reabre em Configurações → Sobre e no login. A página pública fica em [`/novidades`](client/novidades.html).
- **Publicar uma versão:** `cd client && npm run changelog:draft` (PRs mergeados desde a última versão, via `gh`; ou `-- AAAA-MM-DD`), reescrever o rascunho para o jogador e colar no topo do `CHANGELOG.md`, rodar `npm run changelog` e versionar o `client/public/novidades.json` junto. O `npm run build` (e o Docker) também gera o json, e um erro de formato derruba o build.

## Requisitos
Go 1.26+, Node 22+ e Docker.

## Como rodar localmente
```bash
make setup     # uma vez
cp .env.example .env   # opcional: credenciais OAuth
make db-up     # Postgres no Docker
make server    # terminal 1 → :8080 (com login de desenvolvimento)
make client    # terminal 2 → http://localhost:5173
```
Sem credenciais OAuth, use o **login de desenvolvimento** na tela inicial (`DEV_LOGIN=true`, que é o padrão do `make server`).

## Comandos
| Comando | O que faz |
|---|---|
| `make test` | Testes do servidor (com Postgres e `-race`) e checagem de tipos do cliente |
| `make proto` | Regera o código Go/TS depois de mudar `proto/erevia/v1/game.proto` |
| `make build` | Binário do servidor e build do cliente |
| `make docker` | Imagem de produção (a mesma que a Railway usa) |
| `cd server && go run ./cmd/loadbot -bots 100 -duration 2m` | Teste de carga: bots que entram pelo login de dev (servidor local com `DEV_LOGIN=true`), andam, caçam e falam; imprime banda e ping a cada 10 s. O servidor registra o custo do tick no log a cada 30 s ("tick stats") |
| `cd server && go run ./cmd/balance` | Simulador de balanceamento: por classe, caminho e nível, com o equipamento típico +0 e +3, o dano por segundo, o tempo para matar e para morrer, a XP por hora e o tempo até o 30, com as fórmulas do pacote `game` (`-csv` para planilha, `-noite`). O orçamento de poder roda no `go test` (`internal/balance`); a leitura de hoje está em [docs/balanceamento-2026-11.md](docs/balanceamento-2026-11.md) |

| `make gm NAME=Fulano` | Torna GM a conta do personagem no banco local (relogue depois) |

### Fuzz da entrada dos jogadores
Um cliente pode mandar qualquer byte pelo WebSocket; nenhuma mensagem pode derrubar a zona, criar ouro ou item, nem tirar alguém do mapa. Três alvos de `go test -fuzz`:

| Alvo | O que faz |
|---|---|
| `FuzzZoneBytes` (`internal/zone`) | Bytes → `proto.Unmarshal` em `ClientMessage` → zona (2 jogadores perto do NPC da mensagem, mapa real), mandada pelos dois, com ticks |
| `FuzzZoneSemantic` (`internal/zone`) | Sequências de 30 mensagens de todos os tipos do oneof, montadas por reflexão com valores extremos (slots 0/255/2³²−1, quantidades 0 e máximas, ids que não existem ou de outra entidade, NaN/±Inf, textos enormes ou com controle, comandos de chat), com ticks entre elas |
| `FuzzConnBytes` (`internal/net`) | Bytes pelo WebSocket de verdade (readJoin/readPump) até a zona rodando o próprio loop; cada entrada espera a resposta da zona |

Depois de cada mensagem os alvos da zona conferem: `checkInvariants` (vida, mana, ouro, itens, posição, progressão, PvP, grupo, clã e troca; em teste dá panic); ouro + valor de venda dos itens (bolsa, equipamento, armazém e chão) nunca sobe, e nenhuma unidade de item aparece fora de compra, criação ou recompensa de missão; destino, caminho e itens no chão dentro do mapa; armazém válido; nenhuma mensagem que o cliente não consiga ler; nenhum erro no log (falha de serialização ou panic recuperado por `handleSafe`/`runSystem`; os alvos da zona chamam `handle` direto, sem o recover); nenhuma mensagem ou tick acima de 2 s.

```bash
cd server
go test ./internal/zone -run '^$' -fuzz '^FuzzZoneSemantic$' -fuzztime 10m -parallel 4
go test ./internal/zone -run '^$' -fuzz '^FuzzZoneBytes$' -fuzztime 10m -parallel 4
go test ./internal/net -run '^$' -fuzz '^FuzzConnBytes$' -fuzztime 10m -parallel 4
```
Uma falha grava a entrada em `testdata/fuzz/<Alvo>/` e mostra a sequência de mensagens; `go test ./internal/zone -run '<Alvo>/<arquivo>'` repete. Commite o arquivo junto com a correção (o `go test` normal roda todo o corpus). O corpus inicial (uma mensagem válida de cada tipo) é gerado por `EREVIA_WRITE_FUZZ_CORPUS=1 go test ./internal/zone -run TestFuzzSeedCorpus`. Sem Go instalado: `docker run --rm -v "$PWD/server:/src" -w /src golang:1.26 go test ...`. O CI roda 10 s de cada alvo.

## Testes de navegador (e2e)
Em `e2e/` (pacote próprio, fora do build do cliente), com **Playwright** no Chromium headless e WebGL por software (SwiftShader). Pegam o que só quebra no navegador: tela branca, erro de JavaScript, botão que não faz nada, cena 3D que não carrega. Um roteiro de jogador novo, todo pelo mouse e pelo teclado: login de desenvolvimento pela tela inicial, personagem criado pela API, **Entrar no mundo**, cena 3D desenhada, fechar a janela "O que há de novo", andar com um clique no chão, abrir e fechar Configurações (Esc), Inventário (I), Personagem (C), Skills (K) e Mapa (M), comprar uma Poção de Mana Menor da Mercadora Lívia, sair da cidade pelo mapa até os Arredores, selecionar um monstro (Tab), usar a skill da tecla 1, tomar a poção (tecla 4), atacar com um clique e recarregar a página para voltar ao mesmo personagem, no mesmo lugar e com as mesmas moedas.

- **Qualquer `pageerror`, `console.error` ou relatório para `POST /api/client-errors` reprova o teste.** A única exceção é o 401 de `/api/me` antes do login (lista em `e2e/tests/fixtures.ts`). Os avisos "modelos 3D indisponíveis" e "cenário indisponível" também reprovam. `e2e/tests/guarda.spec.ts` confere o próprio guarda (um `throw` e um `console.error` de propósito precisam ser pegos).
- **WebGL por software:** o Chromium roda com SwiftShader e com o canvas 2D na CPU (`--disable-accelerated-2d-canvas`: acelerado, as texturas que o cliente pinta travavam a página por mais de 30 s e o servidor derrubava a conexão). O teste entra com a qualidade gráfica salva como **Baixa**; a Alta (SSAO, FXAA) fica de fora. Se a página ainda travar e o cliente reconectar sozinho ao carregar o mundo, o teste espera uma vez e deixa um aviso no relatório.
- **`?e2e=1`** liga no cliente (também no build de produção) o `window.__ereviaE2E`, **só de leitura**: posição do jogador, NPCs e monstros que o cliente conhece e onde cada um aparece na tela, para o teste saber onde clicar. Nada nele manda mensagem ao servidor; as ações são cliques e teclas de verdade.
- **No CI**, o job **Navegador (Playwright)** sobe o Postgres, compila cliente e servidor, roda o servidor com `DEV_LOGIN=true` e `STATIC_DIR=client/dist` e joga o roteiro. Se falhar, o artefato `e2e-falha` tem screenshot, vídeo, trace (`npx playwright show-trace trace.zip`) e o log do servidor.

Para rodar localmente, suba o servidor com `DEV_LOGIN=true` servindo o cliente compilado (o teste usa o build de produção, não o Vite de desenvolvimento). O jeito mais simples é a imagem Docker, com o Postgres do `make db-up`:
```bash
docker build -t erevia-e2e .
docker run -d --name erevia-e2e-srv -p 8087:8080 -e DEV_LOGIN=true -e PUBLIC_URL=http://localhost:8087 \
  -e DATABASE_URL='postgres://erevia:erevia@host.docker.internal:5432/erevia?sslmode=disable' erevia-e2e
cd e2e
npm ci
npx playwright install chromium        # uma vez (no Linux: --with-deps)
E2E_BASE_URL=http://localhost:8087 npx playwright test
E2E_BASE_URL=http://localhost:8087 E2E_CHANNEL=chrome npx playwright test   # com o Chrome instalado
npx playwright show-report             # relatório, com o trace de uma falha
```
Sem `E2E_BASE_URL`, o teste usa `http://localhost:8080`. Cada execução cria uma conta e um personagem novos (`e2e…` / `Heroi…`) no banco.

## Estrutura
```
proto/erevia/v1/game.proto   mensagens cliente ↔ servidor
server/
  cmd/zone/                  executável: HTTP + WebSocket + zona + arquivos do cliente
  internal/api/              login (OAuth, dev), sessão, personagens, apagar a conta, portal de administração (admin.go)
  internal/store/            Postgres (pgx) + migrações SQL (goose)
  internal/game/             dados e fórmulas: classes, skills, monstros, itens, drops, dano, XP
  internal/world/            mapa embutido (world.json): colisão e caminhos (A*)
  internal/zone/             simulação da zona: combate, PvP, IA, chat, inventário, drops, loja, grupo, clã, amigos, correio, emotes, teleporte, encantamento, armazém (uma goroutine, tick 20 Hz)
  internal/net/              conexões WebSocket (read/write pumps)
  internal/pb/               código Go gerado (não editar)
client/
  privacidade.html, termos.html  páginas legais (estilo em src/styles/legal.css)
  admin.html, src/admin/     portal de administração (/admin, só GM)
  novidades.html             página pública /novidades (lê public/novidades.json)
  src/ui/                    telas (login, personagens, conta) e chat
  src/world/                 cena, entidades, animações, efeitos, entrada
  scripts/build-models.mjs   personagens/monstros/animações a partir dos zips KayKit
  scripts/build-scenery.mjs  kits de cenário (um .glb por pacote)
  scripts/build-world.mjs    layout do mapa + colisores → public/world.json e server/internal/world/
  scripts/build-changelog.mjs  CHANGELOG.md → public/novidades.json (roda no npm run build)
  scripts/changelog-draft.mjs  rascunho do changelog com os PRs mergeados (via gh)
  src/net/                   conexão com o servidor
  src/gen/                   código TS gerado (não editar)
e2e/                         testes de navegador (Playwright): e2e/tests/jogo.spec.ts
infra/docker-compose.yml     Postgres local
Dockerfile, .railway/        publicação (Railway: infraestrutura como código)
```

## Variáveis do servidor
| Variável | Padrão | |
|---|---|---|
| `DATABASE_URL` | — | **Obrigatória** |
| `PORT` / `ADDR` | `8080` | A Railway define `PORT` |
| `PUBLIC_URL` | `http://localhost:5173` | Base das URLs de retorno do OAuth |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | vazio | Liga o botão do Google |
| `DISCORD_CLIENT_ID` / `DISCORD_CLIENT_SECRET` | vazio | Liga o botão do Discord |
| `DEV_LOGIN` | `false` | `true` libera o login sem senha (**só local**) |
| `STATIC_DIR` | vazio | Pasta do cliente compilado (a imagem Docker define) |
| `ALLOWED_ORIGINS` | `localhost:*,127.0.0.1:*` | Origens extras aceitas no WebSocket |
| `DEBUG_INVARIANTS` | vazio | A zona confere o estado de cada jogador 1×/s e antes de salvar (vida e mana, ouro, pilhas, equipamento, nível e XP, posição, karma, grupo, clã e troca; ver `server/internal/zone/invariants.go`). Com `1` (e sempre nos testes), estado inválido é panic; sem ela, corrige o que é seguro e anota um evento `invariant` em `game_events` |
| `INVASION_CHANCE` | `0.15` | Chance (0 a 1) de invasão da cidade a cada noite; `0` = só por GM |
| `XP2_DAYS` | `sab,dom` | Dias de XP em dobro (horário de Brasília); vazio ou `nenhum` = só por GM |
| `ALERT_WEBHOOK_URL` | vazio | Webhook do Discord dos [alertas](#alertas-no-discord). **Segredo**: só na variável da Railway, nunca no git. Vazio = os alertas só vão para o log |
| `ALERT_SPIKE_N` | `50` | Pico de erros: avisa com mais de N ocorrências de um mesmo erro em 5 min |

## Railway
Projeto com dois serviços no ambiente `production`:
- **erevia**: este repositório (Dockerfile), com healthcheck em `/healthz`;
- **Postgres**: banco gerenciado, ligado via `DATABASE_URL=${{Postgres.DATABASE_URL}}`.

URLs de retorno do OAuth em produção:
- `https://erevia-production.up.railway.app/api/auth/google/callback`
- `https://erevia-production.up.railway.app/api/auth/discord/callback`

O serviço **erevia** espera o CI do GitHub antes de publicar (`checkSuites: true` em `.railway/railway.ts`; no painel, "Wait for CI").

## Moderação
GM é um campo da conta, marcado direto no banco (no painel da Railway: Postgres → Data, ou `psql`). O jogador precisa relogar para virar GM.

```sql
-- Tornar GM a conta dona do personagem "Nome"
UPDATE accounts SET gm = true WHERE id = (SELECT account_id FROM characters WHERE lower(name) = lower('Nome'));

-- Relatos de /bug ainda abertos, e marcar um como resolvido
SELECT id, at, character_name, level, x, z, version, text FROM bug_reports WHERE NOT resolved ORDER BY at DESC;
UPDATE bug_reports SET resolved = true WHERE id = 1;

-- Últimas ações de GM
SELECT at, gm_name, command, target, detail FROM gm_log ORDER BY at DESC LIMIT 50;

-- /bug completo: o contexto do cliente e se tem print
SELECT b.id, b.at, b.character_name, b.text, b.context->>'region' AS regiao, b.context->'fps' AS fps,
       b.context->'invariants' AS invariantes, s.report_id IS NOT NULL AS tem_print
FROM bug_reports b LEFT JOIN bug_report_shots s ON s.report_id = b.id ORDER BY b.at DESC LIMIT 20;
```

### /bug completo
O relato feito pelo jogo (`/bug texto` no chat ou **Configurações → Sobre → Relatar problema**, uma caixinha para quem joga no celular sem teclado) leva o contexto do cliente e um print da tela do jogo (só o canvas 3D, sem a interface). Código: `client/src/ui/bug-report.ts`, `server/internal/api/bug_reports.go` e `server/internal/zone/bugreport.go`.

1. `POST /api/bug-reports` com `{"text", "characterId", "context"}` (JSON até 40 KB; texto até 1000 caracteres; contexto: um objeto de até 32 KB). A zona confere, pelo canal, que o personagem está online e é da conta da sessão, aplica as regras do `/bug` do chat (8 caracteres no mínimo e um a cada 30 s, contado junto nos dois caminhos) e carimba nome, nível, posição, versão e navegador; a API grava e responde `201 {"id": N}`. Erros: 400 (curto ou contexto inválido), 409 (personagem fora do jogo), 413, 429 (intervalo, com `Retry-After`) e 503 (zona ocupada).
2. `POST /api/bug-reports/{id}/shot` com o JPEG cru (`Content-Type: image/jpeg`, até 100 KB e 1280 px no lado maior; o cliente manda 640 px no lado maior, qualidade 0,6, abaixo de 80 KB). Só a conta dona, um print por relato (409 no segundo) e até 10 min depois do relato; relato de outra conta, inexistente ou vencido dão o mesmo 404. Responde 204.
3. Limite por conta: 3 relatos e 3 prints de uma vez, depois 1 a cada 30 s (429).
4. Servidor sem a rota, fora do ar ou com a zona ocupada: o cliente cai no `/bug` do chat (só o texto, como antes). Clientes antigos e bots continuam usando o chat.

**Por que o HTTP faz o relato inteiro** (e não o chat responder com o id): o id volta direto na resposta, sem mudar o protocolo do jogo nem casar mensagens do chat com pedidos HTTP, e o dono do relato é a sessão do cookie, a mesma que autoriza o print. A zona continua sendo a única que lê o estado do jogador (pelo canal, com prazo).

**Onde fica:** o contexto em `bug_reports.context` (JSONB, migração 00026; `NULL` nos relatos pelo chat). O print em `bug_report_shots` (`report_id`, `at`, `width`, `height`, `jpeg`), à parte para as listagens não arrastarem o JPEG. Os dois somem com a conta (cascata de `bug_reports.account_id`). O portal lê o print em `GET /api/admin/bug-reports/{id}/shot` (atrás do `AdminOnly`: quem não é GM vê 404; `image/jpeg`, `no-store`); no código, `store.BugReportShot(id)` e `store.BugReportContext(id)`.

**Formato do contexto** (`v: 1`; campos novos podem aparecer, e quem lê ignora o que não conhece):

| Campo | O quê |
|---|---|
| `v` | Versão do formato (1) |
| `client` | Commit do build do cliente (`dev` local) |
| `at` | Hora do navegador (ISO 8601) |
| `fps` | `{avg, min, samples}`: média e mínimo do último minuto, 1 amostra por segundo (`samples` < 60 logo depois de entrar) |
| `ping` | Último ping, em ms |
| `region`, `x`, `z` | Região e posição do personagem (`null` se não havia) |
| `gameTime` | `{phase, hour, night}`: fase do ciclo (0–1), hora do jogo (`"14:10"`) e se é noite |
| `screen` | `{w, h, dpr, touch, quality, shadows, viewDistance}`: janela, densidade, toque e gráficos |
| `players` | Jogadores na tela |
| `invariants` | Regras das [invariantes do cliente](#invariantes-do-cliente) quebradas na sessão |
| `errors` | Os últimos 20 erros e avisos da sessão (`recentClientErrors()`): `{level, kind, message, at, count, stack?, extra?}`, com a stack nas 3 primeiras linhas e o `extra` até 600 caracteres (cortados se o contexto passar de 30 KB) |
| `messages` | As últimas 30 mensagens recebidas do servidor, da mais velha à mais nova: `{t, type, id?, n?}`, com `t` em segundos antes do relato (negativo), `type` o tipo da mensagem (`spawn`, `selfStats`, `chat`…; do snapshot, só `snapshot.deaths`, `.revives`, `.levelUps` e `.telegraphs`), `id` da entidade quando a mensagem tem um e `n` repetições seguidas. Só tipos e ids: nunca o texto do chat nem quem falou |

## Erros do cliente
Todo erro de JavaScript no navegador de um jogador chega à tabela `client_errors` (`client/src/net/errors.ts` → `POST /api/client-errors`): `window.onerror`, promessas rejeitadas sem tratamento, perda do contexto WebGL, mensagens do servidor que não decodificam e modelos 3D que não carregam. Erros iguais (mensagem + primeira linha da stack) viram **uma linha por versão** do cliente, com a contagem, a primeira e a última vez, e o contexto da última ocorrência (navegador, conta, personagem, região, posição, FPS e ping). O cliente manda em lote a cada 10 s ou ao esconder/fechar a aba, no máximo 20 erros distintos por sessão; o servidor aceita até 30 lotes por minuto por IP e por conta. Apagar a conta tira a conta e o personagem das linhas dela.

```sql
-- Os erros que mais aconteceram nos últimos 7 dias
SELECT id, kind, count, last_seen, left(version, 7) AS versao, region, message
FROM client_errors WHERE last_seen > now() - interval '7 days' ORDER BY count DESC LIMIT 30;

-- Tudo de um erro (stack, navegador, onde estava)
SELECT * FROM client_errors WHERE id = 42;

-- Erros novos da versão em produção
SELECT id, kind, count, message FROM client_errors WHERE version = '<commit>' ORDER BY first_seen DESC;
```

**Traduzir a stack** (os `.js` são minificados): o build gera sourcemaps escondidos (`build.sourcemap: "hidden"`, sem o comentário no `.js`). Eles nunca são servidos: a imagem Docker os tira de `/app/public` (ficam só no estágio `client`, em `/app/sourcemaps`) e o servidor responde 404 a qualquer `*.map`. O script `client/scripts/resolve-stack.mjs` traduz para `arquivo:linha:coluna` do código-fonte:

```bash
cd client
# Os .map da versão do erro (coluna version): o mesmo build do Dockerfile, no mesmo commit
git checkout <version>
docker build --build-arg RAILWAY_GIT_COMMIT_SHA=<version> --target client -t erevia-maps ..
docker create --name erevia-maps erevia-maps && docker cp erevia-maps:/app/sourcemaps ./maps && docker rm erevia-maps

node scripts/resolve-stack.mjs --maps maps --id 42          # pela linha da tabela (precisa de psql)
PSQL="docker exec -i erevia-postgres-1 psql -U erevia -d erevia" node scripts/resolve-stack.mjs --maps maps --hash 1a2b3c…
node scripts/resolve-stack.mjs --maps maps < stack.txt      # stack colada
node scripts/resolve-stack.mjs < stack.txt                  # com os .map de um `npm run build` local (client/dist)
```
Os nomes dos `.js` levam o hash do conteúdo: se o arquivo da stack não estiver na pasta dos maps, o build é outro, e o script avisa (e tenta o de mesmo nome base).

Para o código do jogo relatar algo: `reportClientError(kind, mensagem, extra?, erro?)` (vai ao servidor), `reportClientWarning(kind, mensagem, extra?)` (só em memória) e `recentClientErrors()` (os últimos 20 erros e avisos da sessão, para anexar ao /bug).

### Invariantes do cliente
O cliente percebe quando mostra algo impossível e relata com `reportClientError("client_invariant", regra, contexto)` (`client/src/net/invariants.ts`): a regra é a `message` em `client_errors` e o contexto vai no `extra`. As checagens rodam 1×/s num `setInterval` (fora do render; o HUD é conferido quando o `SelfStats` chega), e cada regra é relatada **uma vez por sessão**. As quebradas também vão no `/bug` (`invariants` do contexto).

| Regra | Quando | Contexto (`extra`) |
|---|---|---|
| `hud_hp` / `hud_mp` | Vida ou mana acima do máximo, negativa ou não numérica | valor, máximo, nível, morto |
| `hud_xp` | XP antes do começo do nível ou depois do próximo | XP, começo e fim da faixa, nível |
| `entity_duplicate_id` | Spawn de um id que já está na tela | id, tipo, se é o próprio jogador |
| `entity_no_mesh` | Entidade sem nenhuma malha viva em duas checagens seguidas | id, tipo, malhas |
| `entity_bad_position` | Posição NaN ou Infinity | id, tipo, x, y, z |
| `monster_dead_moving` | Monstro morto que andou mais de 1 m entre duas checagens | id, de onde, para onde |
| `entity_server_drift` | Entidade a mais de 6 m de onde o servidor a põe (o último estado, andando até o destino como no Go) por mais de 2 s | id, distância, desenhada × servidor, idade do snapshot |
| `camera_bad_target` | Alvo, posição ou ângulos da câmera NaN/Infinity | alvo, posição, ângulos |
| `camera_below_ground` | Alvo ou posição da câmera abaixo do chão (y < −0,5) | alvo, posição |
| `low_fps` | FPS abaixo de 15 por mais de 10 s (com a aba visível) | FPS, segundos, último minuto, região, entidades, jogadores, monstros, draw calls de um quadro, malhas ativas, qualidade, escala |
| `texture_failed` | Textura que não carregou (qualquer uma, inclusive as de dentro dos .glb) | url, erro |
| `glb_failed` | .glb que não carregou e que ninguém relatava (o saco do saque) | url, erro |

Os .glb de personagens, armas e animações (`models.ts`) e o cenário continuam saindo como `kind = 'model'` e não entram de novo aqui. Os draw calls do `low_fps` são contados só naquele quadro (o contador do engine, sem `SceneInstrumentation` ligada o tempo todo).

**Provocar cada regra (desenvolvimento):** com `npm run dev`, no console do jogo: `__erevia.invariants.rules` lista as regras, `__erevia.invariants.provoke("hud_hp")` provoca uma, `provokeAll()` provoca todas e `broken()` diz quais já foram relatadas. Cada uma passa pelo caminho de verdade da checagem: dados de mentira na entrada (`SelfStats`, spawn repetido, entidade sem malha, em NaN, morta andando ou longe do servidor, FPS lido como 5 por 13 s) ou uma falha real (textura e .glb que não existem; câmera com NaN só durante a checagem, sem chegar a desenhar). `window.__erevia` só existe em `import.meta.env.DEV`.

```sql
-- Invariantes do cliente quebradas na última semana
SELECT message AS regra, count, last_seen, left(version, 7) AS versao, region, extra
FROM client_errors WHERE kind = 'client_invariant' AND last_seen > now() - interval '7 days' ORDER BY count DESC;
```

## Portal de administração
`/admin` é o portal dos GMs, no mesmo servidor e no visual do jogo. Seções:

- **Início:** online agora, versão no ar, uptime, contas, personagens e relatos de /bug nas últimas 24 h (com os abertos), e as últimas ações de GM.
- **Relatos** (`client/src/admin/reports.ts`, `server/internal/api/admin_reports.go`): os relatos de /bug com filtro (abertos/resolvidos, versão, região pela `erevia_region` e texto), o detalhe com a posição no mapa do jogo (o mesmo desenho do mapa do cliente, `world/map-painter.ts`), versão, navegador, o print da tela do jogo (`GET /api/admin/bug-reports/{id}/shot`) e o contexto do cliente do [/bug completo](#bug-completo) (e qualquer coluna que outra migração acrescentar a `bug_reports`), o botão **Marcar como resolvido** (com nota, quem e quando; reabrir também vale; as duas vão para o `gm_log`) e o **Copiar como task** (objetivo, posição, versão, navegador e passos, em Markdown). O menu mostra quantos relatos estão abertos. A aba **Erros do navegador** lista os erros de `client_errors` mais frequentes do período (juntando as versões do mesmo erro), por versão, com a stack e o comando do `resolve-stack.mjs` para traduzi-la (ver [Erros do cliente](#erros-do-cliente)).
- **Métricas** (`metrics.ts`, `admin_metrics.go`): **ao vivo** (a cada 5 s, pela zona com `askZone`): online no total e por região, monstros vivos, itens no chão, tick médio e pior tick do último período de 30 s (o mesmo do log "tick stats"), e a versão no ar. **Gráficos** do período escolhido (24 h, 7, 30 ou 90 dias): quem jogou por dia, ouro que entrou e saiu e por origem, itens que entraram e saíram, tempo até cada nível, mortes por região, onde as pessoas param, FPS por região e aparelho e os erros do servidor (`server_error`, `slow_tick`, `db_error` e `invariant`) por dia e por grupo. São as contas das views da [Telemetria](#telemetria), só no período.
- **Progressão:** as tabelas de [docs/progressao.md](docs/progressao.md) (`GET /api/admin/progression`; ver [Telemetria](#telemetria)).
- **Economia** (`economy.ts`, `admin_economy.go`): ouro em circulação (bolsas + armazéns) e a evolução diária; **itens**: quantos existem hoje (bolsas, equipamento e armazéns) contra entrou − saiu na telemetria (todo o histórico) + o teto do kit inicial, com os suspeitos de duplicação primeiro; **dias fora da curva** (personagem que ganhou num dia 10× a média dos outros do mesmo nível ±1, com pelo menos 500 moedas ou 30 itens); o **histórico de um personagem** (ouro e itens por dia e de onde vieram); e o **registro de GM** (`gm_log`) com filtro por GM, comando, alvo, texto e período.

**Como a duplicação aparece:** para cada item, *sobra = existe − (entrou − saiu + kit)*. Troca entre jogadores e armazém só mudam o item de lugar (não geram evento nem mudam a conta). Sobra **negativa** é normal (poções bebidas, pergaminhos usados, itens que somem do chão ou com personagens apagados não geram evento). Sobra **positiva** é item que apareceu sem passar pela telemetria. Limites: o kit inicial é um teto (cada arma que a classe oferece conta para todos); saídas dos últimos 2 min ficam de fora (o personagem salva a cada 30 s); itens de antes da telemetria (00011) e eventos descartados com a fila cheia também aparecem como sobra; e um item que caiu no PvP e foi pego por outro conta como entrada de novo (esconde, não acusa). O ouro segue a mesma conta (com o ouro inicial de cada personagem).

**Desempenho:** as consultas de gráficos e economia ficam 1 minuto em cache no servidor (pedidos iguais ao mesmo tempo esperam a mesma consulta) e o rodapé de cada painel mostra quanto cada uma levou. A migração 00028 cria os índices que as cobrem (index-only scans). Para medir com uma semana de eventos sintéticos (1 milhão de linhas): `ADMIN_PERF=1 go test ./internal/store -run TestAdminPanelsPerf -v` (com `TEST_DATABASE_URL`); cada consulta tem de levar menos de 200 ms.

**Jogadores** (`#jogadores`; código em `client/src/admin/players.ts`, `server/internal/api/admin_players.go`, `server/internal/zone/admin_actions.go` e `server/internal/store/admin_players.go`):
- **Busca** por parte do nome do personagem, id do personagem ou da conta, ou identidade (o id do Google/Discord, `google:123…`, ou o nome no provedor), com online e offline juntos; sem busca, a lista de quem está online agora (com a região).
- **Ficha** (`#jogadores/<id>`): nível e XP, moedas, região e posição, vida e mana (online), karma, PKs e vitórias PvP, classe e especialização, título, clã e cargo, grupo, inventário (equipado e bolsa) e armazém, missões ativas (com o progresso) e concluídas, conquistas, as últimas sessões e eventos de `game_events`, a conta (identidades, criação, último login, banimento, silêncio, os outros personagens) e o **histórico de moderação** (o `gm_log` do jogo e do portal contra qualquer personagem da conta). Com o personagem online ou que saiu há pouco, os dados vêm da zona (mais novos que o banco); a ficha diz de onde vieram.
- **Ações:** expulsar (só online), silenciar por N minutos e tirar o silêncio, banir e desbanir (com motivo obrigatório), teleportar (X, Z; a cidade é 0, 0), dar e tirar item (tira da bolsa e, se faltar, do equipamento) e moedas, marcar e desmarcar GM (não dá para se banir nem tirar o próprio GM). O motivo vai para o jogador quando ele vê a mensagem e para o `gm_log`; um pedido que falhou fica registrado como `portal:<ação>:falhou`.
- **Onde a ação acontece (nada duplica nem se perde):** quem decide é a zona, dona do estado (`zone.AdminEdit`):
  - *online*: na memória, e o personagem é salvo na hora com `save_seq` novo (um panic depois não volta ao estado de antes);
  - *saiu há pouco* (o `recent` da zona, 10 min): relogar usa a memória antes do banco, então a ação muda esse estado e o salva com `save_seq` novo; mexer no banco aí seria perdido (dar) ou duplicado (tirar) no próximo login;
  - *offline*: a zona **trava** o personagem e a API faz a transação no banco (linha com `FOR UPDATE`, itens pelas mesmas regras da bolsa, `save_seq = GREATEST(…)` e a telemetria `gm` na mesma transação). Durante a trava, entrar com o personagem é recusado; depois, um login que leu o banco **antes** da mudança (o WebSocket anota a hora da leitura, `JoinInfo.LoadedAt`) também é recusado sem `Kicked`, e o cliente reconecta e lê de novo. A trava vence sozinha em 30 s se a API sumir.
  - Silenciar, banir e GM vão primeiro para o banco (é de onde o login lê) e depois a zona aplica em quem está online da conta, com a mesma recusa para um login lido antes.
  - Se a zona pegou o pedido e não respondeu em 10 s, a resposta é "não confirmado: confira a ficha antes de repetir" (nunca "falhou", que levaria a repetir e duplicar).

**Operação** (`#operacao`; `client/src/admin/ops.ts`, `server/internal/api/admin_ops.go`, `server/internal/zone/admin_ops.go`), tudo pela zona e no `gm_log`:
- **Anúncio para todos** (banner e linha dourada no chat), na hora ou **agendado** (30 s a 30 dias; fica em `scheduled_announcements`, migração 00027, e volta à fila depois de um reinício). Os agendados aparecem com o botão de cancelar.
- **Aviso de manutenção:** "o servidor reinicia em N min" para todos, repetido em 60, 30, 15, 10, 5, 3, 2 e 1 min, 30 e 10 s; quem entra durante a contagem também é avisado; cancelar avisa todos. Não reinicia sozinho: é o aviso antes do deploy (e só existe na memória: o reinício o encerra).
- **Eventos:** invasão (anunciar com 2 min, começar agora, parar sem recompensa) e XP em dobro por N horas ou desligar (os mesmos do `/evento`), com a **agenda dos automáticos**: os dias de `XP2_DAYS` e a próxima janela, o próximo anoitecer e a chance de invasão (`INVASION_CHANCE`).
- **Chefes:** vivo (vida, fase do raid, fúria, há quanto tempo em luta) ou morto (a hora exata do renascimento, a janela mostrada no mapa e quando morreu), e **renascer agora** (o raid fica salvo como "renasce agora", para um reinício não o esconder de novo).

**Como abrir:**
1. Entre no jogo uma vez com a conta (Google, Discord ou, local, o login de desenvolvimento) e crie um personagem.
2. Marque a conta como GM (o mesmo campo dos comandos de GM, ver [Moderação](#moderação)): `UPDATE accounts SET gm = true WHERE id = (SELECT account_id FROM characters WHERE lower(name) = lower('Nome'));` ou, local, `make gm NAME=Nome`.
3. Abra `https://…/admin` no mesmo navegador. No portal vale na hora (a checagem é a cada pedido); no jogo, depois de relogar. Para tirar: `gm = false`.

**Segurança:**
- **Só GM, e o resto vê 404:** um middleware único (`AdminOnly`, em `server/internal/api/admin.go`) confere a sessão, se a conta é GM e se não está banida em toda rota de `/api/admin/*`, na página (`/admin`, `/admin/…`, `/admin.html`) e nos arquivos dela (`/assets/admin/`). Quem não é GM ou não está logado recebe o mesmo 404 de um caminho qualquer que não existe (por isso caminhos desconhecidos agora dão 404 em vez de cair no `index.html`).
- **O que fica público:** só o que o jogo já usa (as fontes Cinzel e o polyfill de módulos do Vite). O código e o CSS do portal saem em `assets/admin/` (ver `client/vite.config.ts`), e nenhum arquivo público menciona o portal.
- **Escrita:** todo pedido que muda algo exige o cabeçalho `X-Erevia-Admin: 1`, JSON e a mesma origem (contra CSRF), tem limite por conta (leitura: 60 de uma vez e 2/s; escrita: 10 de uma vez e 1 a cada 2 s) e vai para o `gm_log` com o comando `portal:…`, a conta e o nome do GM e o alvo.
- Respostas do portal com `Cache-Control: no-store`, `X-Frame-Options: DENY`, CSP e `nosniff`. O log do servidor guarda só conta, método, rota e status (nada de corpo, cookie ou query).
- **Jogadores online:** o portal nunca mexe no estado da zona nem grava no banco um personagem que está em memória; pede à zona pelo canal e espera a resposta com prazo (`askZone` em `server/internal/zone/admin.go` para leitura; `askZoneAction` em `admin_actions.go` para as ações, que depois de entregues esperam a resposta em vez de desistir).

A versão no ar vem de `RAILWAY_GIT_COMMIT_SHA` (a Railway define; local aparece `dev`).

## Telemetria
O servidor anota acontecimentos na tabela `game_events` (gravada em lote, sem travar a zona): ouro que entra e sai (e de onde), itens que entram e saem do jogo, abates, mortes, subidas de nível, sessões e o FPS dos jogadores (o cliente mede 1×/s e manda a média a cada 60 s, com o tipo de aparelho). O "painel" são views prontas; abra na aba **Data** do Postgres da Railway ou com `psql`:

| View | Mostra |
|---|---|
| `v_gold_daily` | Ouro que entrou e saiu por dia e por origem (loot, venda, missão, loja, criação, teleporte…) |
| `v_level_times` | Minutos médios para chegar a cada nível |
| `v_deaths_by_region` | Mortes por região e por quem matou |
| `v_quit_level` | Último nível de quem não joga há mais de 7 dias |
| `v_fps` | FPS médio e dos piores momentos, por região e tipo de aparelho |
| `v_items_daily` | Itens que entraram e saíram do jogo por dia (para caçar duplicação) |
| `v_level_playtime` | Tempo **de jogo** (soma das sessões) para passar de cada nível, por classe: média, mediana e p75 |
| `v_xp_sources` | XP por fonte (monstro, grupo, noite, XP em dobro, missão, invasão, morte) e por faixa de nível |
| `v_level_kills_deaths` | Abates e mortes por nível do jogador |
| `v_gold_per_hour` | Ouro ganho e gasto por hora de jogo, por faixa |
| `v_level_reached` | Funil: quantos chegaram a cada nível e quantos pararam nele |

**Progressão** ([docs/progressao.md](docs/progressao.md)): a XP ganha não vai uma linha por abate; a zona soma por personagem, nível e fonte e grava um evento `xp` a cada minuto, ao subir e ao sair. Ao desligar, a zona fecha a sessão de quem está online antes de a telemetria esvaziar a fila (senão todo deploy perderia o tempo de jogo). As consultas prontas estão em `server/internal/store/queries/progressao.sql`, e as tabelas aparecem no portal em `/admin#progressao`.

**Erros do servidor** (também em `game_events`, no máximo 1 linha por grupo a cada minuto, com a contagem acumulada em `amount`): `server_error` é um panic recuperado (a zona segue e só o jogador da mensagem cai; `ref` = hash da stack, `extra` com onde, jogador, mensagem, stack e versão), `slow_tick` é um tick acima de 50 ms (`extra.systems` = os sistemas mais lentos) e `db_error` é uma falha ao gravar no banco (`ref` = operação). Exemplo: `SELECT kind, ref, sum(amount) AS vezes, max(at) AS ultima, max(extra->>'where') AS onde, max(extra->>'version') AS versao FROM game_events WHERE kind IN ('server_error', 'slow_tick', 'db_error') AND at > now() - interval '1 day' GROUP BY 1, 2 ORDER BY 3 DESC;`

## Alertas no Discord
O servidor avisa num canal do Discord o que precisa de atenção. Um worker próprio (`server/internal/alerts`), fora da zona, lê o banco a cada minuto e manda um embed em português, com cor por gravidade e o link para a seção do portal (`PUBLIC_URL` + `/admin#…`):

| Aviso | Quando | Cor |
|---|---|---|
| Erro novo no servidor | Um hash de `server_error` que nunca apareceu antes | vermelho |
| Erro novo no navegador | Um hash de `client_errors` que nunca apareceu antes (em versão nenhuma) | laranja |
| Pico de erros | Mais de `ALERT_SPIKE_N` ocorrências de um mesmo erro em 5 min (servidor, banco ou navegador) | vermelho |
| Invariante quebrada | A primeira quebra de cada regra no dia (horário de Brasília) | amarelo |
| Tick lento | O p95 do tick acima de 50 ms por 2 min (mais de 5% dos ticks acima de 50 ms, pelos `slow_tick`) | laranja |
| Servidor no ar | Cada subida, com a versão (`RAILWAY_GIT_COMMIT_SHA`): deploy (outra versão), reinício ou queda (mesma versão, e o anterior não desligou normalmente). Sai 90 s depois de subir, quando o servidor anterior já desligou | verde, azul ou vermelho |
| Relato de bug | Cada `/bug`, com o texto, o personagem, a posição, a região e a versão | azul |

- **Limite:** no máximo 1 aviso por tipo e chave (o erro, a regra, a conta do `/bug`, a versão) a cada 15 min. O seguinte diz quantos o limite segurou. Os `/bug` da mesma conta esperam o limite e saem juntos; numa enxurrada de erros novos, 5 têm aviso próprio e o resto vai num resumo.
- **Já visto:** fica na tabela `alert_state` (migração 00029) e vale depois de um reinício. Na primeira vez, tudo o que já existia conta como visto (nada de enxurrada de avisos antigos).
- **Robustez:** timeout de 10 s por envio, nova tentativa em erro de rede e 5xx, e no 429 do Discord espera o `retry_after` pedido (até 30 s; mais que isso, tenta na verificação seguinte). A zona nunca espera pelos alertas, e um panic no worker vira `server_error` e ele recomeça.
- Texto de jogador (o `/bug`) não formata, não vira link e não menciona ninguém (`@everyone` não chama).

**Como configurar:**
1. No Discord, no canal dos alertas: **Configurações do canal → Integrações → Webhooks → Novo webhook**. Dê um nome (ex.: Erevia) e clique em **Copiar URL do webhook**.
2. Na Railway, no serviço **erevia** → **Variables**, crie `ALERT_WEBHOOK_URL` com a URL copiada (e, se quiser, `ALERT_SPIKE_N`). A Railway publica de novo, e o aviso "Servidor no ar" chega ao canal em uns 2 min.
3. A URL tem o token do webhook: não ponha no git, num print nem no board. O servidor nunca a escreve no log. Se vazar, apague o webhook no Discord e crie outro.

Local, sem a variável, cada aviso vai para o log como `alerta (sem ALERT_WEBHOOK_URL)`. Para testar com um canal de verdade, ponha a URL em `ALERT_WEBHOOK_URL` no `.env` (que não vai para o git) e rode `make server`.

```sql
-- O que os alertas já avisaram (pico, invariante do dia, tick lento, /bug por conta, subidas)
SELECT kind, key, sent_at, suppressed, pos, info FROM alert_state WHERE kind <> 'seen' ORDER BY sent_at DESC LIMIT 50;
```

## Pipeline de arte (remaster)
Ferramentas open source: **Blender**, **blender-mcp** (o Claude Code controla o Blender, com `DISABLE_TELEMETRY=true`) e a skill **blender-image-to-3d** (referência → asset pronto para jogo). A regra de todo asset novo é o [guia de arte](docs/guia-de-arte.md).

1. **Folhas de referência:** `/Applications/Blender.app/Contents/MacOS/Blender --background --factory-startup --python tools/art/reference_sheets.py -- docs/arte`
2. **Modelar:** a skill blender-image-to-3d no Claude Code, a partir das folhas, até o GLB com LOD0/1/2 (nós com sufixo `_LOD1`/`_LOD2`).
3. **Importar:** `cd client && node scripts/import-asset.mjs <arquivo.glb> <personagem|arma|monstro|chefe|cenario> [nome]`. Ele limpa e otimiza o GLB, confere o orçamento do guia (triângulos, textura, materiais, ossos) e só copia para `public/models` se couber. `--check` só confere; `--force` copia mesmo fora do orçamento.


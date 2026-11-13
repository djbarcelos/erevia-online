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
- As missões são dados em `server/internal/game/quests.go` (objetivos: sair da cidade, matar monstros, usar skills, tomar poções, comprar, chegar a um nível, viajar). O servidor controla o progresso e salva no personagem (`characters.quests`).

### Ranking, conquistas e títulos (tecla H)
- A janela **Glória** (H ou o troféu no menu) tem duas abas. **Ranking:** top 50 de nível (desempate pela XP), de vitórias PvP e de clãs (soma dos níveis dos membros, com a média), mais a sua posição e a do seu clã. Um worker calcula no banco a cada 5 min (contas banidas ficam de fora) e a zona guarda em memória; a janela pede quando abre e guarda a resposta por 2 min. A zona nunca consulta o banco.
- **Conquistas** (12, dados em `server/internal/game/achievements.go`): Primeiro Sangue, Primeiros Passos (tutorial), Quebra-Ossos (100 esqueletos), níveis 10, 20 e 30, O Senhor Caiu (Senhor dos Ossos), Regicida (participar da morte de Ozrhal), Mão Firme (+7), Negócio Fechado (primeira troca), Forja Lendária (criar um lendário) e Duelista (vitória PvP). Andam com o que o servidor já valida (abate, nível, encantamento, troca, criação, missão, PvP e raid boss); quem já tinha cumprido (nível, vitórias, tutorial) ganha ao entrar. Desbloquear avisa no chat e com um toast.
- Cada conquista dá um **título**: escolha na aba Conquistas e ele aparece sob o seu nome, em lilás, para todos (texto fixo no nome, que só muda quando você troca de título). Conquistas e título ficam salvos no personagem (`characters.achievements` e `characters.title`).

### Base para crescer (moderação, /bug, reconexão e CI)
- **Comandos de GM** (contas com `accounts.gm`, ver [Moderação](#moderação)): `/gm` lista os comandos; `/kick Nome [motivo]`, `/mute Nome minutos [motivo]` e `/unmute Nome`, `/ban Nome [motivo]` (vale para a conta inteira, mesmo offline, e derruba quem está online) e `/unban Nome`, `/tp X Z`, `/goto Nome`, `/summon Nome`, `/give Nome item|moedas [qtd]`, `/anuncio texto` e `/online`. Para quem não é GM, eles aparecem como comando desconhecido. Toda ação fica no `gm_log`. O silêncio vale em todos os canais e continua depois de relogar.
- **`/bug texto`:** qualquer jogador (mesmo silenciado), um a cada 30 s. Vai para `bug_reports` com o personagem, o nível, a posição, a versão do servidor (commit do deploy) e o navegador.
- **Reconexão automática:** se a conexão cair, o jogo espera o servidor responder e volta sozinho ao mesmo personagem (desiste depois de 2 min com o servidor fora do ar ou de 5 recargas). Quem foi expulso ou banido não volta.
- **CI:** cada push e pull request roda os testes no GitHub Actions (servidor com Postgres e `-race`, tipos e build do cliente). A Railway só publica a `main` com o CI verde.

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

## Privacidade e exclusão de conta (LGPD)
- **Páginas legais:** [`/privacidade.html`](client/privacidade.html) e [`/termos.html`](client/termos.html), linkadas na tela de login e na de personagens. Por enquanto o responsável, o CNPJ, o e-mail (`contato@erevia.example`) e o foro são **fictícios** e precisam ser trocados pelos reais antes de abrir ao público.
- **O que o banco guarda do login:** só o provedor, o id da conta nele e o nome de exibição (Google `openid profile`, Discord `identify`). Sem e-mail. O único cookie é o de sessão (`HttpOnly`); o chat não é gravado.
- **Apagar a conta:** tela de personagens → **Conta** → digitar `APAGAR`. O servidor (`DELETE /api/account`, com `{"confirm":"APAGAR"}`, JSON e mesma origem como os outros pedidos de escrita) apaga numa transação a conta, identidades, sessões, personagens, itens, armazém e participação em clãs, e tira o nome dos personagens do registro dos raid bosses. O clã liderado por alguém da conta passa ao membro de cargo mais alto (o mais antigo no empate) ou é desfeito se ficar vazio; o banco não deixa apagar um líder sem isso (migração 00020). Depois a zona derruba quem estiver online com a conta (aviso "Esta conta foi apagada."), esquece o estado em memória e recusa um Join atrasado; salvamentos atrasados não acham a linha e não recriam nada.
- **Backups:** diário guardado 6 dias e semanal guardado 27 dias, então dados apagados podem ficar até 27 dias nas cópias. Se restaurar uma cópia, apague de novo as contas excluídas depois dela.

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

| `make gm NAME=Fulano` | Torna GM a conta do personagem no banco local (relogue depois) |

## Estrutura
```
proto/erevia/v1/game.proto   mensagens cliente ↔ servidor
server/
  cmd/zone/                  executável: HTTP + WebSocket + zona + arquivos do cliente
  internal/api/              login (OAuth, dev), sessão, personagens, apagar a conta
  internal/store/            Postgres (pgx) + migrações SQL (goose)
  internal/game/             dados e fórmulas: classes, skills, monstros, itens, drops, dano, XP
  internal/world/            mapa embutido (world.json): colisão e caminhos (A*)
  internal/zone/             simulação da zona: combate, PvP, IA, chat, inventário, drops, loja, grupo, clã, amigos, correio, emotes, teleporte, encantamento, armazém (uma goroutine, tick 20 Hz)
  internal/net/              conexões WebSocket (read/write pumps)
  internal/pb/               código Go gerado (não editar)
client/
  privacidade.html, termos.html  páginas legais (estilo em src/styles/legal.css)
  src/ui/                    telas (login, personagens, conta) e chat
  src/world/                 cena, entidades, animações, efeitos, entrada
  scripts/build-models.mjs   personagens/monstros/animações a partir dos zips KayKit
  scripts/build-scenery.mjs  kits de cenário (um .glb por pacote)
  scripts/build-world.mjs    layout do mapa + colisores → public/world.json e server/internal/world/
  src/net/                   conexão com o servidor
  src/gen/                   código TS gerado (não editar)
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
```

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

## Pipeline de arte (remaster)
Ferramentas open source: **Blender**, **blender-mcp** (o Claude Code controla o Blender, com `DISABLE_TELEMETRY=true`) e a skill **blender-image-to-3d** (referência → asset pronto para jogo). A regra de todo asset novo é o [guia de arte](docs/guia-de-arte.md).

1. **Folhas de referência:** `/Applications/Blender.app/Contents/MacOS/Blender --background --factory-startup --python tools/art/reference_sheets.py -- docs/arte`
2. **Modelar:** a skill blender-image-to-3d no Claude Code, a partir das folhas, até o GLB com LOD0/1/2 (nós com sufixo `_LOD1`/`_LOD2`).
3. **Importar:** `cd client && node scripts/import-asset.mjs <arquivo.glb> <personagem|arma|monstro|chefe|cenario> [nome]`. Ele limpa e otimiza o GLB, confere o orçamento do guia (triângulos, textura, materiais, ossos) e só copia para `public/models` se couber. `--check` só confere; `--force` copia mesmo fora do orçamento.


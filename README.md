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
- **Visual do personagem:** na criação, além da arma e do emblema, escolha o rosto e o cabelo (5 cabeças: Nobre, Selvagem, Sábio, Astuto e Andarilho, que qualquer classe pode usar), o tom de pele, a cor do cabelo e a cor da roupa (8 de cada, ou a original da classe) e se o elmo/gorro/chapéu aparece. Depois, Teodoro, o **Barbeiro** (ao norte do poço), muda o visual por 300 moedas, com prévia 3D. Todos veem o visual novo na hora.

### Criação de itens (Ferreiro)
- A aba **Criar** do Ferreiro Bruno transforma materiais que caem dos monstros + moedas em equipamento que a loja não vende: 24 receitas, de raro (ossos e crânios dos esqueletos) a épico (Essência Sombria) e lendário (Núcleo Rúnico dos golens), mais pergaminhos de encantar (inclusive os abençoados). A lista mostra quanto você tem de cada material; o botão só liga quando dá.
- O servidor confere materiais, moedas e espaço numa cópia da bolsa e só então aplica. Receitas em `server/internal/game/recipes.go`.

### Troca entre jogadores
- Selecione um jogador perto (até 8 m) e clique em **Trocar** no quadro do alvo; ele recebe o convite. Na janela de troca, arraste itens da bolsa (ou use "Oferecer na troca" no menu do item), ponha moedas, **trave** a oferta e, com as duas travadas, **confirme**. Qualquer mudança (inclusive usar um item oferecido) destrava as duas ofertas.
- Nada sai da bolsa até a confirmação dos dois. Na hora, o servidor confere tudo de novo, monta as bolsas novas e aplica de uma vez, salvando os dois personagens na mesma transação. Se alguém sair, morrer, se afastar ou não tiver espaço, a troca é cancelada sem mexer em nada.

### Missões
- **Tutorial dos primeiros 15 minutos** (8 missões em sequência): sair da cidade, derrotar Esqueletos Lacaios, usar skills, tomar uma poção, comprar na praça, chegar ao nível 4, viajar com uma Pedra de Teleporte e enfrentar os Campos dos Ossos. Cada uma dá XP, moedas e/ou itens; se a bolsa estiver cheia, a recompensa fica no chão, reservada para você.
- O painel **Missões** fica à esquerda, abaixo do grupo, e mostra o progresso (passe o mouse para ver a história e a recompensa). Só personagens novos (abaixo do nível 5 e sem histórico) recebem o tutorial.
- As missões são dados em `server/internal/game/quests.go` (objetivos: sair da cidade, matar monstros, usar skills, tomar poções, comprar, chegar a um nível, viajar). O servidor controla o progresso e salva no personagem (`characters.quests`).

### Base para crescer (moderação, /bug, reconexão e CI)
- **Comandos de GM** (contas com `accounts.gm`, ver [Moderação](#moderação)): `/gm` lista os comandos; `/kick Nome [motivo]`, `/mute Nome minutos [motivo]` e `/unmute Nome`, `/ban Nome [motivo]` (vale para a conta inteira, mesmo offline, e derruba quem está online) e `/unban Nome`, `/tp X Z`, `/goto Nome`, `/summon Nome`, `/give Nome item|moedas [qtd]`, `/anuncio texto` e `/online`. Para quem não é GM, eles aparecem como comando desconhecido. Toda ação fica no `gm_log`. O silêncio vale em todos os canais e continua depois de relogar.
- **`/bug texto`:** qualquer jogador (mesmo silenciado), um a cada 30 s. Vai para `bug_reports` com o personagem, o nível, a posição, a versão do servidor (commit do deploy) e o navegador.
- **Reconexão automática:** se a conexão cair, o jogo espera o servidor responder e volta sozinho ao mesmo personagem (desiste depois de 2 min com o servidor fora do ar ou de 5 recargas). Quem foi expulso ou banido não volta.
- **CI:** cada push e pull request roda os testes no GitHub Actions (servidor com Postgres e `-race`, tipos e build do cliente). A Railway só publica a `main` com o CI verde.

### Fase 3 e interface nova (Fases A–C)
- **13 encaixes de equipamento:** cabeça, peito, pernas, mãos, pés, capa, arma, mão secundária, colar, 2 brincos e 2 anéis. Anéis e brincos vão no encaixe livre com duplo clique, ou no que você escolher arrastando; dá para trocar anel 1 ↔ anel 2 arrastando. Itens novos nas 4 raridades com bônus novos: precisão, esquiva, velocidade de movimento, de ataque e de conjuração, regeneração de vida e mana e dano crítico.
- **Aparência na criação:** cada classe escolhe a arma inicial (ex.: espada ou lâmina curva para o cavaleiro; cajado, varinha ou cajado sombrio para o mago; três pares de lâminas para o ladino), o escudo (ou nada, e ele vai para a bolsa) e a cor do emblema. A arma é um item de verdade do kit. A cor pinta o nome e um anel no chão para todos verem, e a tela de seleção mostra cada personagem com o que tem nas mãos.
- **Atributos com pontos livres (tecla C):** Força, Destreza, Constituição, Inteligência, Sabedoria e Espírito. Cada classe começa com a sua base e ganha 3 pontos por nível. A janela mostra a diferença exata antes de confirmar (vida, ataque, defesa, crítico, precisão, esquiva, velocidade de ataque e de conjuração, regeneração).
- **Árvore de skills (tecla K):** 12 skills por classe em 3 ramos e 4 degraus (níveis 1, 5, 10 e 15), com ranks; 1 ponto por nível. Mecânicas: prender, veneno/queimadura/praga (dano por segundo), provocar, escudos, escudo de mana, invisibilidade, marca, imortal, cegueira, frenesi, enfraquecer, armadura de gelo, grito de guerra e estandarte para os aliados, aura de coragem, Piscar e Investida (teleporte), execução, golpes duplos, área em volta do alvo, roubo de vida e mais. Uma bolinha nos botões avisa quando há pontos para gastar.
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
- **XP e nível** até 20. Os atributos crescem com o nível. Morrer custa cerca de 4% da XP do nível e o personagem volta à cidade.
- Monstros vagam, atacam quem chega perto (os agressivos), perseguem até um limite e reaparecem depois de mortos.
- Barras de vida, mana e XP, quadro do alvo, números de dano e efeitos de skill.
- Login com **Google** ou **Discord** (OAuth + PKCE). A sessão fica num cookie HttpOnly.
- Até **3 personagens** por conta, cada um com nome único e uma classe: cavaleiro, bárbaro, mago ou ladino.
- **Chat:** canal geral, `/w Nome texto` para sussurrar e `/r texto` para responder. Clicar num nome no chat prepara o sussurro.
- O personagem é salvo no banco ao sair, a cada 30 s, logo depois de comprar, vender ou pegar itens, e quando o servidor desliga.
- Se o mesmo personagem entrar em outro lugar, a sessão antiga cai com um aviso.
- O servidor decide tudo: movimento, limites do mapa, limite anti-spam e anti-flood. Clientes lentos são desconectados.
- **Painel de desempenho:** tecla **F3** (ou `?debug=1` na URL) mostra tempo de frame, draw calls, malhas ativas e o tempo gasto em animação e na interface.

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
| `make gm NAME=Fulano` | Torna GM a conta do personagem no banco local (relogue depois) |

## Estrutura
```
proto/erevia/v1/game.proto   mensagens cliente ↔ servidor
server/
  cmd/zone/                  executável: HTTP + WebSocket + zona + arquivos do cliente
  internal/api/              login (OAuth, dev), sessão, personagens
  internal/store/            Postgres (pgx) + migrações SQL (goose)
  internal/game/             dados e fórmulas: classes, skills, monstros, itens, drops, dano, XP
  internal/world/            mapa embutido (world.json): colisão e caminhos (A*)
  internal/zone/             simulação da zona: combate, PvP, IA, chat, inventário, drops, loja, grupo, clã, teleporte, encantamento, armazém (uma goroutine, tick 20 Hz)
  internal/net/              conexões WebSocket (read/write pumps)
  internal/pb/               código Go gerado (não editar)
client/
  src/ui/                    telas (login, personagens) e chat
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


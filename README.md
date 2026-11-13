# Erevia

MMORPG 3D no navegador. Cliente em **TypeScript + Babylon.js**, servidor em **Go**, comunicação por **WebSocket + Protobuf** e banco em **PostgreSQL**.

**Produção:** https://erevia-production.up.railway.app (deploy automático a cada push na `main`).

Pesquisa e decisões de arquitetura: [RELATORIO.md](RELATORIO.md).

## Estado atual: Fase 3 (itens e economia) + interface nova + progressão (Fase B)
- **Atributos com pontos livres (tecla C):** Força, Destreza, Constituição, Inteligência, Sabedoria e Espírito. Cada classe começa com a sua base e ganha 3 pontos por nível. A janela mostra a diferença exata antes de confirmar (vida, ataque, defesa, crítico, precisão, esquiva, velocidade de ataque e de conjuração, regeneração).
- **Árvore de skills (tecla K):** 12 skills por classe em 3 ramos e 4 degraus (níveis 1, 5, 10 e 15), com ranks; 1 ponto por nível. Mecânicas: prender, veneno/queimadura/praga (dano por segundo), provocar, escudos, escudo de mana, invisibilidade, marca, imortal, cegueira, frenesi, enfraquecer, armadura de gelo, grito de guerra e estandarte para os aliados, aura de coragem, Piscar e Investida (teleporte), execução, golpes duplos, área em volta do alvo, roubo de vida e mais. Uma bolinha nos botões avisa quando há pontos para gastar.
- **Barra de atalhos salva no servidor:** a mesma em qualquer computador ou celular.
- **Interface nova** (protótipo aprovado em `/prototipo.html`, agora no jogo):
  - Telas de login e de personagens com o modelo 3D num pedestal (girar arrastando, zoom com a roda ou a pinça).
  - HUD: personagem no canto superior esquerdo; minimapa no direito (clique abre o mapa) com zoom, X/Y, FPS e ping; barra de **10 atalhos** (teclas 1–0, skills e poções, arraste para trocar; botão direito tira) ao lado do botão **AUTO** e dos botões das janelas; chat com abas; avisos de saque, XP e nível.
  - Janelas (arrastáveis no PC, tela cheia no celular): **Inventário (I)** com a miniatura 3D do equipamento, **Personagem (C)**, **Skills (K)**, **Mapa do mundo (M)** com regiões, chefes, lojas e clique para andar, e **Configurações (Esc)** com teclas configuráveis, áudio/gráficos e as opções da auto-caça.
  - **Celular:** joystick, botão de ataque com as skills em volta, poção, trocar alvo, AUTO e menu ☰; arrastar o dedo gira a câmera e a pinça dá zoom. Liga sozinho em telas de toque (ou em Configurações → Controles).
  - **Auto-caça (F ou AUTO):** escolhe o monstro mais perto no raio configurado, ataca, usa as skills da barra, toma poção abaixo do limite, pega o saque e volta à cidade sem poção. Roda no navegador e só manda os comandos normais (o servidor valida tudo).
- **Itens e moedas:** cerca de 40 itens em 4 raridades (comum, bom, raro e épico): armas de cada classe, escudos, armaduras, elmos, poções e materiais. A arma e o escudo aparecem na mão do personagem; armas raras e épicas brilham.
- **Drops:** monstros deixam moedas e itens no chão (saquinho com o nome; feixe de luz nos itens bons). Por 15 s, só quem causou mais dano pode pegar; depois fica livre, e some em 60 s. Clicar no item faz o personagem andar até ele e pegar.
- **Inventário (tecla I):** 40 espaços e 4 encaixes (arma, mão secundária, armadura, elmo), com os atributos ao lado. Arrastar e soltar move, troca, equipa e destrói; duplo clique usa ou equipa.
- **Poções nas teclas 3–6:** arraste uma poção do inventário para a barra. Vida e mana têm recargas separadas (10 s).
- **Loja:** a Mercadora Lívia (poções) e o Ferreiro Bruno (armas e armaduras comuns e boas) ficam na praça. Clicar num deles abre a loja; vender paga 40% do preço.
- **Kit inicial:** arma (e escudo) equipados, 5 poções e 50 moedas.
- **Persistência segura:** moedas e itens são salvos junto com o personagem numa única transação. Um salvamento atrasado nunca sobrescreve um mais novo, e relogar rápido usa o estado da memória (não perde nem duplica itens). O servidor é o único que cria ou destrói itens.

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

## Estrutura
```
proto/erevia/v1/game.proto   mensagens cliente ↔ servidor
server/
  cmd/zone/                  executável: HTTP + WebSocket + zona + arquivos do cliente
  internal/api/              login (OAuth, dev), sessão, personagens
  internal/store/            Postgres (pgx) + migrações SQL (goose)
  internal/game/             dados e fórmulas: classes, skills, monstros, itens, drops, dano, XP
  internal/world/            mapa embutido (world.json): colisão e caminhos (A*)
  internal/zone/             simulação da zona: combate, IA, chat, inventário, drops, loja (uma goroutine, tick 20 Hz)
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

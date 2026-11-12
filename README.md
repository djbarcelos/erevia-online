# Erevia

MMORPG 3D no navegador. Cliente em **TypeScript + Babylon.js**, servidor em **Go**, comunicação por **WebSocket + Protobuf** e banco em **PostgreSQL**.

**Produção:** https://erevia-production.up.railway.app (deploy automático a cada push na `main`).

Pesquisa e decisões de arquitetura: [RELATORIO.md](RELATORIO.md).

## Estado atual: Fase 1
- Login com **Google** ou **Discord** (OAuth + PKCE). A sessão fica num cookie HttpOnly.
- Até **3 personagens** por conta, cada um com nome único e uma classe: cavaleiro, bárbaro, mago ou ladino.
- **Chat:** canal geral, `/w Nome texto` para sussurrar e `/r texto` para responder. Clicar num nome no chat prepara o sussurro.
- A posição é salva no banco ao sair, a cada 30 s e quando o servidor desliga.
- Se o mesmo personagem entrar em outro lugar, a sessão antiga cai com um aviso.
- O servidor decide tudo: movimento, limites do mapa, limite anti-spam e anti-flood. Clientes lentos são desconectados.

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
  internal/zone/             simulação da zona e chat (uma goroutine, tick 20 Hz)
  internal/net/              conexões WebSocket (read/write pumps)
  internal/pb/               código Go gerado (não editar)
client/
  src/ui/                    telas (login, personagens) e chat
  src/world/                 cena, jogadores, clique para andar
  src/net/                   conexão com o servidor
  src/gen/                   código TS gerado (não editar)
infra/docker-compose.yml     Postgres local
Dockerfile, railway.toml     publicação
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

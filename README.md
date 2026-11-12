# Erevia

MMORPG 3D no navegador. Cliente em **TypeScript + Babylon.js**, servidor em **Go**, comunicação por **WebSocket + Protobuf** e banco em **PostgreSQL**.

Pesquisa e decisões de arquitetura: [RELATORIO.md](RELATORIO.md).

## Estado atual: Fase 0
- Mapa 3D com chão em grade, árvores e pedras.
- Clique esquerdo no chão para andar. Segurar o botão continua andando em direção ao cursor.
- Botão direito gira a câmera e a roda do mouse dá zoom.
- Outros jogadores aparecem em tempo real, com o nome acima da cabeça.
- O servidor decide tudo: posição, velocidade e limites do mapa. Ele também limita os pedidos de movimento a 10 por segundo e derruba clientes lentos.

## Requisitos
Go 1.25+, Node 22+ e Docker.

## Como rodar
```bash
make setup     # uma vez
make db-up     # Postgres no Docker
make server    # terminal 1 → :8080
make client    # terminal 2 → http://localhost:5173
```
Abra duas abas em http://localhost:5173 e entre com nomes diferentes.

`make server` funciona sem banco: rode `cd server && go run ./cmd/zone` sem `DATABASE_URL`.

## Comandos
| Comando | O que faz |
|---|---|
| `make test` | Testes do servidor (com `-race`) e checagem de tipos do cliente |
| `make proto` | Regera o código Go/TS depois de mudar `proto/erevia/v1/game.proto` |
| `make build` | Binário do servidor (`server/bin/zone`) e build do cliente (`client/dist`) |

## Estrutura
```
proto/erevia/v1/game.proto   mensagens cliente ↔ servidor (fonte da verdade)
server/
  cmd/zone/                  executável do servidor
  internal/zone/             simulação da zona (uma goroutine, tick 20 Hz)
  internal/net/              conexões WebSocket (read/write pumps)
  internal/db/               conexão com Postgres
  internal/pb/               código Go gerado (não editar)
client/
  src/world/                 cena, jogadores, clique para andar
  src/net/                   conexão com o servidor
  src/gen/                   código TS gerado (não editar)
infra/docker-compose.yml     Postgres
```

## Variáveis do servidor
| Variável | Padrão | |
|---|---|---|
| `ADDR` | `:8080` | Endereço HTTP |
| `DATABASE_URL` | vazio | Se vazio, roda sem banco |
| `ALLOWED_ORIGINS` | `localhost:*,127.0.0.1:*` | Origens aceitas no WebSocket |

## Extras
- `?webgpu=1` na URL testa o renderizador WebGPU. O padrão é WebGL2.
# erevia-online

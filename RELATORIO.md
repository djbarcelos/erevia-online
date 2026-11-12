# MMORPG 3D no navegador, no estilo Lineage 2: relatório de pesquisa

> Pesquisa feita em novembro de 2026. Preços e versões mudam com frequência, então confira nos sites oficiais antes de decidir.

---

## 1. Por que não dá para usar o Lineage 2 diretamente

**Limitação técnica.** O cliente do Lineage 2 é um executável Windows feito em Unreal Engine 2 e não roda no navegador. Os servidores privados conhecidos (L2J e derivados) só funcionam com esse cliente oficial instalado no PC do jogador. Ou seja, não existe "servidor de L2 no browser" sem reescrever o cliente do zero.

**Risco jurídico.** A NCSoft persegue ativamente servidores privados:
- Na Coreia, processou operadores de servidores privados de Lineage por violação de direito autoral e conseguiu na Justiça o bloqueio das contas bancárias de quatro servidores ([Inven Global](https://www.invenglobal.com/articles/20315/ncsoft-pursues-legal-action-including-provisional-attachment-of-accounts-against-operators-of-illegal-lineage-private-servers), [Digital Today](https://www.digitaltoday.co.kr/en/view/40815/ncsoft-seeks-asset-seizure-and-damages-over-lineage-private-servers)).
- Em 2006, junto com o FBI, derrubou o L2Extreme, um servidor privado de Lineage II nos EUA ([Game Developer](https://gamedeveloper.com/game-platforms/fbi-ncsoft-close-down-unauthorized-i-lineage-ii-i-servers)).
- Em 2016, pediu ao GitHub a remoção (DMCA) de repositórios com código de servidor de Lineage II ([registro DMCA](https://git.randommeaninglesscharacters.com/github/dmca/src/branch/master/2016/2016-04-19-NCSOFT.md)).

**O que pode e o que não pode.**

| Pode usar livremente | Não pode usar |
|---|---|
| Gênero e mecânicas (classes, PvP com karma, clãs, cercos, "farm" de mobs, drops raros) | Nome "Lineage", nomes de raças, classes e lugares próprios do jogo |
| A "vibe" de fantasia medieval sombria | Arte, modelos 3D, texturas, músicas e sons extraídos do cliente |
| Ideias de progressão e economia | Mapas e cidades copiados (Giran, Aden etc.) |
| | Cliente oficial, ou o protocolo dele para conectar o cliente da NCSoft |

Mecânica de jogo em geral não é protegida por direito autoral; a expressão é (arte, texto, nomes, música). *Isto não é aconselhamento jurídico.*

**Conclusão:** o caminho viável é criar um **MMORPG original**, com nome, mundo e arte próprios, que **lembre** o L2 no jeito de jogar.

---

## 2. Mecânicas "estilo L2" sugeridas para o MVP maior

| Sistema | Versão MVP | Futuro |
|---|---|---|
| **Raças e classes** | 3 classes base (guerreiro, mago, arqueiro/ladino) e uma troca de classe no nível 20 | Várias raças, 2ª e 3ª profissão |
| **Progressão** | XP por mob, nível máximo ~40, pontos de status automáticos | Subclasse, "noblesse"/hero equivalentes |
| **Skills** | 4 a 6 skills por classe, com cooldown e custo de MP | Buffs de suporte, skills em área, encantamento de skills |
| **Combate** | Clique no alvo e auto-ataque (como no L2), servidor calcula dano | Status (stun, root, sleep), resistências |
| **Mobs e drops** | Spawns por zona, tabela de drop com chance, 1 "raid boss" | Bosses épicos com respawn longo |
| **Inventário** | Grade de itens, equipar arma/armadura, adena-equivalente (moeda) | Encantamento de equipamento (+1, +2… com risco) |
| **NPCs** | Loja (comprar/vender), teleporte entre cidades | Crafting, missões |
| **PvP** | Flag ao atacar jogador; matar jogador sem flag dá karma; karma aumenta chance de dropar item | Arenas, olimpíada |
| **Social** | Chat geral, sussurro, grupo (party) com XP dividido | Clãs, alianças, cerco de castelo |
| **Persistência** | Conta, personagens, inventário e posição salvos no banco | Mercado entre jogadores |

---

## 3. Tecnologias: o que usar

### 3.1 Cliente 3D (o que roda no navegador)

| Opção | Resumo | Prós | Contras |
|---|---|---|---|
| **Babylon.js** | Engine 3D completa, só código | Já vem com física, GUI, animação, Inspector de debug e Playground. A v9.0 saiu em março de 2026. Tem tutorial oficial do Colyseus | Pacote maior, comunidade menor que a do Three.js |
| **Three.js** | Biblioteca de renderização | Maior comunidade (~5 mi downloads/semana), núcleo pequeno, flexível | Física, input, game loop e UI ficam por sua conta |
| **PlayCanvas** | Engine com editor visual no browser | Editor visual, hospedagem e CDN de assets | Editor hospedado é pago para projetos privados (~US$ 15/mês); prende você ao fluxo deles |

As três suportam WebGPU com fallback para WebGL2 em 2026. Fontes: [Utsubo](https://www.utsubo.com/blog/threejs-vs-babylonjs-vs-playcanvas-comparison), [Cinevva (em português)](https://app.cinevva.com/pt-BR/blog/2026-06-09-web-game-engines-2026-comparison), [Cinevva: Three vs Babylon](https://app.cinevva.com/guides/threejs-vs-babylonjs). Várias dessas fontes são blogs de empresas da área, então leia com algum cuidado.

**Recomendação: Babylon.js.** Para um MMO, o que mais pesa é rede e servidor, não a renderização. O Babylon entrega mais coisa pronta (GUI para barras de HP/MP, inventário, animações), e isso economiza meses.

### 3.2 Servidor de jogo (rede)

| Opção | Linguagem | Prós | Contras |
|---|---|---|---|
| **Colyseus** | Node.js / TypeScript | Salas ("rooms") com sincronização automática de estado, servidor autoritativo, tutorial oficial com Babylon.js, mesma linguagem do cliente | Não tem matchmaking, ranking, economia e live ops prontos; escala melhor para "uma sala por zona" |
| **Nakama** | Go (lógica em TS/Lua/Go) | Vem com contas, chat, amigos, grupos, ranking e matchmaking (Apache 2.0) | Curva de aprendizado maior; o realtime de MMO exige lógica customizada |
| **SpacetimeDB** | Rust/C# | Banco e servidor juntos; o MMORPG BitCraft Online roda nele | Mais novo e com modelo diferente; menos tutoriais para browser |
| **WebSocket próprio** | Qualquer | Controle total | Você reescreve tudo que o Colyseus já resolve |

Fontes: [docs do Colyseus](https://docs.colyseus.io/), [tutorial Colyseus + Babylon.js](https://docs.colyseus.io/learn/tutorial/babylonjs/intro), [fórum Heroic Labs: Nakama vs Colyseus](https://forum.heroiclabs.com/t/nakama-vs-colyseus/1632), [comparativo Nakama vs Colyseus vs SpacetimeDB](https://www.pistack.xyz/posts/2026-09-24-nakama-vs-colyseus-vs-spacetimedb-self-hosted-game-backend/), [Colyseus vs backend gerenciado](https://gsb.supercraft.host/blog/colyseus-vs-managed-backend/).

**Recomendação: servidor próprio em Go.** O objetivo é desempenho e muitos usuários, e Go oferece:
- **Todos os núcleos do processador** usados naturalmente, com goroutines. O Node usa um núcleo por processo.
- **Desempenho várias vezes maior que o do Node** em lógica pesada (combate, IA de mobs, colisão), com pausas de limpeza de memória (GC) curtas, quase imperceptíveis em jogo.
- **Linguagem pequena e simples**, bem mais fácil que Rust, e com boa parte do desempenho dele.
- **Um único executável** para colocar no ar, sem dependências.

O custo é escrever a base que o Colyseus daria pronta: conexões WebSocket, loop do jogo, zonas e sincronização. Em compensação, você controla tudo.

**Como fica o MMO em Go:**
- **Cada zona do mapa é uma goroutine** com seu próprio loop (tick fixo, ex.: 20 por segundo). Os jogadores dessa zona mandam comandos por um *channel*, e a zona processa tudo em sequência. Assim não há problemas de concorrência dentro da zona.
- **Cada conexão tem duas goroutines**, uma lê e a outra escreve. A escrita tem uma fila com limite: cliente lento é desconectado e não trava o servidor.
- **Ao atravessar a borda da zona,** o servidor transfere o jogador para a goroutine da outra zona. Se a zona estiver em outra máquina, a transferência passa pelo NATS.
- **Protocolo binário com esquema compartilhado:** as mensagens são definidas em arquivos `.proto` (Protobuf) ou FlatBuffers, e o código é gerado automaticamente em Go (servidor) e em TypeScript (cliente). Assim os dois lados nunca discordam sobre o formato.

**Bibliotecas Go sugeridas:**

| Função | Biblioteca |
|---|---|
| WebSocket | `github.com/coder/websocket` (ou `gorilla/websocket`) |
| Protocolo binário | `google.golang.org/protobuf` + `buf` para gerar Go/TS (ou FlatBuffers) |
| PostgreSQL | `jackc/pgx` + `sqlc` (gera código Go tipado a partir do SQL) |
| ClickHouse | `ClickHouse/clickhouse-go` v2 (com envio em lote) |
| Mensagens entre servidores e chat global | `nats-io/nats.go` |
| HTTP (login, API, painel admin) | biblioteca padrão `net/http` (ou `chi`) |
| Métricas | `prometheus/client_golang` + Grafana |
| Pathfinding | A* próprio sobre grid/navmesh, ou `recast`-port se precisar de navmesh 3D |

**Alternativa mais rápida de construir:** Colyseus (Node/TypeScript) resolve salas e sincronização prontas e serve bem para algumas centenas de jogadores online. Faz sentido para um protótipo descartável, mas não é o caminho escolhido.

### 3.3 Banco de dados: PostgreSQL + ClickHouse

Os dois bancos fazem trabalhos diferentes e se complementam:

| | **PostgreSQL** | **ClickHouse** |
|---|---|---|
| Para que serve | Estado do jogo: o que **é** agora | Histórico e análise: o que **aconteceu** |
| Exemplos | Contas, personagens, inventário, adena, clãs, castelos | Cada kill, drop, troca, compra em NPC, morte em PvP, login e logout |
| Tipo | Transacional (OLTP): leituras e escritas pequenas, consistentes, com transações | Analítico (OLAP), colunar: grava muito em lote e agrega bilhões de linhas em segundos |
| Atualiza registros? | Sim, toda hora (HP, posição, itens) | Mal. É feito para *inserir*, não para editar linha a linha |
| Se perder dados | Grave: jogador perde item | Ruim, mas o jogo continua funcionando |

**Regra:** o jogo **nunca** depende do ClickHouse para funcionar. Ele só recebe eventos e serve para responder perguntas.

**O que o ClickHouse resolve em um MMO estilo L2:**
- **Economia:** quanta adena entra e sai por dia, inflação, itens mais dropados e preço médio nas lojas.
- **Anti-cheat e anti-bot:** jogador matando 3 mil mobs por hora, padrões de farm 24h e trocas suspeitas (RMT, duplicação de item).
- **Balanceamento:** taxa de vitória por classe no PvP, tempo médio para subir de nível e mobs que ninguém caça.
- **Logs de auditoria:** "para onde foi a espada +10 do jogador X?". Responde em segundos, mesmo com meses de histórico.
- **Painéis:** jogadores online, retenção e funil de criação de personagem (com Grafana ou Metabase ligados ao ClickHouse).

**Como os eventos chegam ao ClickHouse:**
1. **Recomendado para começar:** o servidor do jogo (Go) junta os eventos em memória e uma goroutine dedicada os envia em **lotes** (ex.: a cada 1–5 segundos ou a cada 1.000 eventos), usando o cliente oficial `clickhouse-go` com `PrepareBatch`. O ClickHouse funciona mal com muitos inserts pequenos, por isso o envio é em lote. Outra opção é usar *async inserts*, um recurso do próprio ClickHouse que agrupa os inserts pequenos.
2. **Depois, se precisar:** uma fila (Redis Streams, NATS ou Kafka) entre o jogo e o ClickHouse, para não perder eventos se o ClickHouse cair.
3. **Para copiar tabelas do Postgres** (ex.: lista de personagens, para cruzar com os eventos), use replicação/CDC, como o PeerDB (hoje parte do ClickHouse) ou o ClickPipes na versão Cloud, ou consulte o Postgres direto do ClickHouse com a *table engine* `PostgreSQL`.

**Exemplo de tabela de eventos:**
```sql
CREATE TABLE game_events (
  ts          DateTime64(3),
  event       LowCardinality(String),   -- 'kill_mob', 'drop', 'pvp_kill', 'trade', 'login'…
  char_id     UInt64,
  zone        LowCardinality(String),
  target_id   UInt64,
  item_id     UInt32,
  amount      Int64,
  data        String                    -- JSON com detalhes extras
) ENGINE = MergeTree
PARTITION BY toYYYYMM(ts)
ORDER BY (event, char_id, ts)
TTL toDateTime(ts) + INTERVAL 2 YEAR;
```

**Quando colocar o ClickHouse:** não é necessário no protótipo. No começo, uma tabela `game_events` no próprio Postgres dá conta. Adicione o ClickHouse no **MVP maior (fase 3)**, quando já existir economia, drops e PvP para analisar, ou antes disso, se você quiser os painéis desde o início. Rodar os dois via Docker Compose localmente é simples.

**Outros:**
- **NATS**: comunicação entre os servidores de zona, chat global, sussurros e presença online. É leve e escrito em Go.
- **Redis** (opcional): cache e sessões de login.
- **pgx + sqlc** para o Postgres: você escreve o SQL e o `sqlc` gera as funções Go tipadas. Mais rápido e previsível que um ORM.
- **Migrações** do banco com `goose` ou `golang-migrate`.

### 3.4 Stack recomendada (resumo)

**Stack escolhida: desempenho e muitos usuários.**

```
Cliente:   TypeScript + Babylon.js (WebGPU, com WebGL2 de reserva) + Vite
Rede:      WebSocket com protocolo BINÁRIO (Protobuf/FlatBuffers, código gerado p/ Go e TS)
           Depois: WebTransport (UDP no navegador) para movimento
Servidor:  Go — uma goroutine por zona (tick fixo), duas por conexão
           Serviço de login/API separado (net/http); zonas distribuídas entre processos
Bus:       NATS → chat global, presença, transferência de jogador entre zonas/servidores
Banco:     PostgreSQL (pgx + sqlc) → estado do jogo
Analytics: ClickHouse (clickhouse-go, envio em lote) → eventos, logs, economia, anti-cheat
Métricas:  Prometheus + Grafana
Dados:     tabelas de itens/skills/mobs em JSON, lidas pelo cliente e pelo servidor
```

**Estrutura de pastas sugerida (monorepo):**
```
/client     → TypeScript + Babylon.js
/server     → Go (cmd/gateway, cmd/zone, internal/…)
/proto      → mensagens .proto (geram Go e TS)
/data       → JSON de itens, skills, mobs, drops
/infra      → docker-compose (Postgres, ClickHouse, NATS, Grafana)
```

Se um dia uma parte específica (ex.: a simulação de um cerco com milhares de jogadores) precisar de mais desempenho, ela pode ser isolada e reescrita em Rust sem mexer no resto.

---

## 4. Projetos open source para estudar

| Projeto | O que é | Por que olhar |
|---|---|---|
| [RPG 3D multiplayer com Colyseus (fórum Babylon)](https://forum.babylonjs.com/t/open-source-multiplayer-3d-rpg-using-colyseus/35733) | RPG top-down 3D open source com Babylon + Colyseus | O lado do cliente (Babylon) é igual ao nosso; o servidor serve de referência para a lógica |
| [Tutorial oficial Colyseus + Babylon.js](https://docs.colyseus.io/learn/tutorial/babylonjs/intro) | Passo a passo de servidor autoritativo | Os conceitos (estado autoritativo, sincronização) valem para o servidor em Go |
| [RPG-JS](https://awesome.ecosyste.ms/projects/github.com%2Frsamaium%2Frpg-js) | Framework TS para RPG/MMORPG 2D no browser (MIT) | Ver como organizam servidor autoritativo e previsão no cliente |
| [Helbreath Base Game](https://phaser.io/news/2026/06/helbreath-base-game-a-classic-mmorpg-rebuilt-with-phaser-and-c) | Base de MMORPG 2D (Phaser + React + servidor C#/.NET 10, MIT) | Referência de sistemas de MMO clássico: inventário, combate, mapas |
| [Thread "browser MMORPG" no fórum Babylon](https://forum.babylonjs.com/t/looking-to-build-a-browser-based-mmorpg/63056) | Discussão de devs | Armadilhas reais de quem tentou |

---

## 5. Assets 3D gratuitos (arte sem problema de direitos)

- **Quaternius**: mais de 70 pacotes e quase 2.000 modelos low-poly em **CC0** (domínio público, uso comercial liberado, sem crédito obrigatório). Destaques: [LowPoly RPG Characters](https://opengameart.org/content/lowpoly-rpg-characters) (guerreiro, mago, ladino, monge, arqueiro, já animados), [Fantasy Props MegaKit](https://opengameart.org/content/fantasy-props-megakit) e a [lista completa no OpenGameArt](https://opengameart.org/content/all-cc0-uploader-quaternius). Mais contexto no [Game World Observer](https://gameworldobserver.com/?p=17420).
- **Kenney** (kenney.nl): kits CC0 de cenário, UI e ícones. Confira os pacotes direto no site.
- **Mixamo** (Adobe): animações gratuitas para personagens humanoides (andar, atacar, conjurar). Confira a licença atual no site.
- **Coleção CC0 no itch.io**: [itch.io/c/3305113/cc0](https://itch.io/c/3305113/cc0).

Dica: exporte tudo em **glTF/GLB**, o formato que o Babylon carrega melhor.

---

## 6. Arquitetura proposta

```
 ┌──────────────── Navegador ────────────────┐
 │ Babylon.js (render, câmera, animações)    │
 │ UI: HP/MP, inventário, chat, skills       │
 │ Previsão local do movimento               │
 └──────────────┬────────────────────────────┘
                │ WebSocket binário (Protobuf)
                │ envia: "quero andar até X", "usar skill 3 no alvo Y"
                │ recebe: estado da zona (posições, HP, mobs)
 ┌──────────────▼────────────────────────────┐
 │ Servidor de jogo (Go)                     │
 │  Zona "Vila Inicial"  Zona "Floresta"  …  │
 │  (uma goroutine por zona) ◄──► NATS       │
 │  • valida tudo (anti-cheat)               │
 │  • IA dos mobs, combate, drops, karma     │
 │  • tick fixo (ex.: 20x por segundo)       │
 └──────────────┬────────────────────────────┘
                │                         │ eventos em lote
                │ salva periodicamente    │ (kill, drop, troca, PvP…)
                │ e ao sair               ▼
 ┌──────────────▼──────────────┐ ┌──────────────────────────┐
 │ PostgreSQL                  │ │ ClickHouse               │
 │ contas, personagens, itens  │ │ logs, economia, anti-bot │
 │ (estado, fonte da verdade)  │ │ → painéis Grafana        │
 └─────────────────────────────┘ └──────────────────────────┘
```

**Regras de ouro:**
1. **O servidor manda em tudo.** O cliente só pede ("quero atacar") e mostra o resultado. Nunca confie em dano, posição ou drop vindo do cliente.
2. **Uma goroutine por zona.** Cada zona processa seus comandos em sequência, sem disputa entre threads. Para escalar, distribua as zonas entre vários processos ou máquinas.
3. **Interest management.** Divida a zona em células (grid) e só envie ao jogador o que está nas células vizinhas.
4. **Dados em tabelas.** Mobs, itens, skills e drops em arquivos JSON na pasta `/data`, lidos pelo cliente e pelo servidor, para balancear sem mexer em código.
5. **Envie só o que mudou** (delta), não o estado inteiro a cada tick.

---

## 7. Roadmap em fases

| Fase | Entrega | Estimativa (1 dev, meio período) |
|---|---|---|
| **0. Fundamentos** | Monorepo, Babylon renderizando um terreno, servidor Go com WebSocket + Protobuf, docker-compose com Postgres | 2–3 semanas |
| **1. Protótipo jogável** | Login, criar personagem, andar com clique, ver outros jogadores, chat | 3–5 semanas |
| **2. Combate PvE** | Mobs com IA simples, auto-ataque, HP/MP, XP e nível, respawn | 3–4 semanas |
| **3. MVP maior** | 3 classes com skills, inventário, drops, loja NPC, teleporte, PvP com flag/karma, persistência completa | 2–3 meses |
| **4. Social e endgame** | Party, clãs, raid boss, encantamento de itens | 2–3 meses |
| **5. Cercos de castelo** | Guerra de clãs agendada, impostos da cidade | depois |

As estimativas são aproximadas e servem só para ordem de grandeza. Com ajuda de IA para gerar código, as fases 0 a 2 podem andar bem mais rápido.

---

## 8. Hospedagem e custo

| Opção | Modelo | Observação | Custo aproximado |
|---|---|---|---|
| **VPS (Hetzner, DigitalOcean etc.)** | Máquina fixa | Melhor custo para servidor de jogo sempre ligado. A Hetzner subiu preços de 30 a 40% em 2026 | a partir de ~€5–10/mês para começar |
| **Fly.io** | Máquinas por uso | Fácil de deployar perto dos jogadores; RAM a US$ 6/GB/mês desde out/2026 | menor máquina ~US$ 2/mês, mais a RAM real |
| **Railway** | Uso (CPU/RAM) | Muito prático e ótimo abaixo de ~1 GB; caro para muita RAM sempre ligada | depende do uso; bom para dev e testes |

Fontes: [Hetzner: aumento de preço 2026](https://findstack.com/resources/hetzner-price-increase-2026), [Hetzner vs Railway](https://getdeploying.com/hetzner-vs-railway), [preços Fly.io 2026](https://www.buildmvpfast.com/blog/fly-io-pricing-2026-ram-increase-vs-hetzner). Confira os preços atuais em hetzner.com/cloud, fly.io/docs/about/pricing e railway.com/pricing.

**Vantagem do Go aqui:** o servidor vira um único executável pequeno, com pouca memória por jogador. Uma VPS modesta aguenta bastante gente.

**Sugestão:** durante o desenvolvimento, rode tudo local (Docker com Postgres, ClickHouse e NATS). Para um teste com amigos, uma VPS pequena ou o Railway bastam. Pense em escala só quando houver jogadores de verdade. Para jogadores no Brasil, prefira uma região em São Paulo ou na costa leste dos EUA, por causa da latência.

---

## 9. Próximos passos sugeridos

1. Escolher um **nome e tema** próprios para o jogo.
2. Validar a stack com a **Fase 0**: terreno no Babylon e dois navegadores se vendo andar, conectados ao servidor Go via WebSocket + Protobuf.
3. Baixar o pacote **Quaternius RPG Characters** para ter personagens animados desde o início.
4. Escrever as **tabelas de dados** (classes, skills, mobs, itens) antes do código de combate.

---

## Fontes

- Inven Global: [NCSoft pursues legal action against Lineage private servers](https://www.invenglobal.com/articles/20315/ncsoft-pursues-legal-action-including-provisional-attachment-of-accounts-against-operators-of-illegal-lineage-private-servers)
- Digital Today: [NCSoft seeks asset seizure over Lineage private servers](https://www.digitaltoday.co.kr/en/view/40815/ncsoft-seeks-asset-seizure-and-damages-over-lineage-private-servers)
- Game Developer: [FBI, NCsoft close down unauthorized Lineage II servers](https://gamedeveloper.com/game-platforms/fbi-ncsoft-close-down-unauthorized-i-lineage-ii-i-servers)
- [DMCA da NCSoft no GitHub (2016)](https://git.randommeaninglesscharacters.com/github/dmca/src/branch/master/2016/2016-04-19-NCSOFT.md)
- [Utsubo: Three.js vs Babylon.js vs PlayCanvas (2026)](https://www.utsubo.com/blog/threejs-vs-babylonjs-vs-playcanvas-comparison)
- [Cinevva: Engines de jogos web em 2026](https://app.cinevva.com/pt-BR/blog/2026-06-09-web-game-engines-2026-comparison)
- [Cinevva: Three.js vs Babylon.js](https://app.cinevva.com/guides/threejs-vs-babylonjs)
- [Colyseus: documentação](https://docs.colyseus.io/) e [tutorial Babylon.js](https://docs.colyseus.io/learn/tutorial/babylonjs/intro)
- [Fórum Heroic Labs: Nakama vs Colyseus](https://forum.heroiclabs.com/t/nakama-vs-colyseus/1632)
- [Nakama vs Colyseus vs SpacetimeDB (2026)](https://www.pistack.xyz/posts/2026-09-24-nakama-vs-colyseus-vs-spacetimedb-self-hosted-game-backend/)
- [Colyseus vs backend gerenciado](https://gsb.supercraft.host/blog/colyseus-vs-managed-backend/)
- [Fórum Babylon: RPG 3D open source com Colyseus](https://forum.babylonjs.com/t/open-source-multiplayer-3d-rpg-using-colyseus/35733)
- [Fórum Babylon: Looking to build a browser-based MMORPG](https://forum.babylonjs.com/t/looking-to-build-a-browser-based-mmorpg/63056)
- [RPG-JS](https://awesome.ecosyste.ms/projects/github.com%2Frsamaium%2Frpg-js)
- [Phaser: Helbreath Base Game](https://phaser.io/news/2026/06/helbreath-base-game-a-classic-mmorpg-rebuilt-with-phaser-and-c)
- [OpenGameArt: LowPoly RPG Characters](https://opengameart.org/content/lowpoly-rpg-characters), [Fantasy Props MegaKit](https://opengameart.org/content/fantasy-props-megakit), [Quaternius CC0](https://opengameart.org/content/all-cc0-uploader-quaternius)
- [Game World Observer sobre Quaternius](https://gameworldobserver.com/?p=17420)
- [itch.io: coleção CC0](https://itch.io/c/3305113/cc0)
- [Findstack: aumento de preço da Hetzner 2026](https://findstack.com/resources/hetzner-price-increase-2026)
- [GetDeploying: Hetzner vs Railway](https://getdeploying.com/hetzner-vs-railway)
- [BuildMVPFast: preços Fly.io 2026](https://www.buildmvpfast.com/blog/fly-io-pricing-2026-ram-increase-vs-hetzner)
- [Tech Insider: Colyseus Cloud setup 2026](https://tech-insider.org/colyseus-cloud-setup-2026/)

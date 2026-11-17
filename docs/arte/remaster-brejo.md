# Brejo de Lamagris (tunnel#139)

A expansão leste do Mapa 1 (roadmap 5.3 e T12): o mapa vai de x −150 a 330 (480 × 480 m com o Vale), com o **Brejo** (x 150–330, z −150..60), a **extensão das Terras Mortas** (x 150–270, z 60–150) e a estrada leste até o **Posto do Barqueiro** (165, 0). O sudeste (x 150–330, z < −150) e o nordeste (x > 270, z > 60) são montanhas sem acesso, fechadas por uma muralha de pedras de colisor (`brejo.mjs`).

## O que entrou

- **Mapa:** `client/scripts/biomes/brejo.mjs` e `terras-mortas.mjs` (o `world.json` é regerado: `node scripts/build-world.mjs`). A borda leste da floresta (montanhas em x ≈ 168) caiu para dentro do mapa novo e sai por `ctx.clear`, sem mudar o resto. O servidor lê os limites do `world.json`; a grade do A* cobre a área nova (`maxExpanded` 260.000).
- **Água:** a lagoa de Vau Velho (245, −38) é **rasa e andável** (joelho, −0,8 m), com duas partes fundas de **colisor** no servidor (anéis de círculos): a aldeia afogada (a igreja, a torre e as casas do kit saem da água) e o fosso da Ilha dos Vaga-lumes. O lago visual de antes (190, 52, fora da borda, #99) virou esta lagoa: o `terrain.ts` lê a lagoa, as partes fundas e a ilha da entrada `brejo` do `world.json` (`lakeOf`), e o pântano é quase plano.
- **Monstros (nível 14 a 20, 36 ao mesmo tempo, pico de 16 num raio de 70 m, `TestBrejoBudget`):**

| Monstro | Papel | Dia / noite | Modelo |
|---|---|---|---|
| Brejeiro Lanceiro (×8, 2 acampamentos de 4) | matilha: bater em um puxa os 4; alcance 3,2 m | neutro de dia | `brejeiro` (Meshy) |
| Brejeiro Xamã (×4) | cura um aliado e joga "Lama nos Olhos" (cegueira 2 s) | neutro de dia | `brejeiro`, `Tint` #7fb3a4 |
| Sapo-Boi (×9 + 7 de dia) | bruto; "Língua": linha de 8 m, 1 s de aviso, prende 1,5 s | dia e noite | `frog` (#135) |
| Lodo Vivo (×8) | divide em 2 Lodinhos; o golpe dá lentidão de 20% | dia e noite | `slime` (#135), `Tint` #a8794f |
| Afogado (×7) | emboscador (4,5 m), agarra 1,5 s | só à noite, no lugar dos 7 sapos de dia | `skeleton_warrior`, `Tint` #6fa89a |
| **Mãe-Lodo** (chefe de campo, nível 21, (250, −40), 50–100 min) | Golfada em cone de 60°, Maré de Lama (4 círculos com veneno), Lodinhos a cada 25 s | | `slime`, `Tint` #6f8a4a, escala 2,8 |
| **Velha Escamosa** (rara, hospedeiro Xamã, 2%) | invoca 2 Lodinhos e atira lama envenenada | | `brejeiro` |

- **Balanceamento:** o Sapo-Boi é o único bruto (os outros têm papel e ficam fora das contas, `BalanceBaseline`). Ele segue a curva do Guerreiro Esqueleto (vida +14%, ataque −12%): a XP por abate do 14 ao 20 fica igual à tabela de antes, e a tabela de progressão (`docs/progressao.md`) só troca o nome da caça. O simulador conta o Brejo como **uma região só** (`BrejoHuntArea`, 12.400 m²: a fatia dos 16 sapos nos 28.000 m² do pântano com os 36 bichos). `TestPowerBudget` passa sem exceção nova.
- **Posto do Barqueiro:** pedra de teleporte a 180 moedas (`Posto do Barqueiro`, (162, 5)), **Tomé** (talker novo, (169, −3), o próximo id de NPC livre) e o **barco**: duas "pedras" de balsa (`ferry`) no cais (188, −10) e na Ilha dos Vaga-lumes (268, −20), 10 moedas cada trecho. As pedras comuns não listam o barco, e o barco só lista o outro lado. Os ids dos NPCs que já existiam não mudaram (as pedras novas ficam depois dos contadores de história).
- **Antídoto:** a primeira poção de receita do Ferreiro (3 por 2 Glândulas de Sapo e 1 Junco-de-Prata). Limpa veneno e peste (o veneno já existia: `EffectPoison`), recarga própria. Materiais novos: Escama de Brejeiro, Lodo Negro, Glândula de Sapo, Junco-de-Prata.
- **Missões (#71):** linha "Brejo de Lamagris" com Tomé (convite e 4 missões, nível 15 a 20, a última é a Mãe-Lodo) e 3 diárias (Juncos-de-Prata, frascos de Antídoto para o posto, ronda do cais e da lagoa). Cabem no teto de XP de história por nível (`TestBrejoQuestsFitTheBudget`).
- **Bestiário (#158):** família nova "Gente do Brejo" (`brejeiro`, `frog`, `slime`; só o Sapo-Boi trava a conclusão, os de papel são extras); o Afogado fica com os Esqueletos.
- **Cliente:** regiões retangulares (`RECT_REGIONS`: nome na tela, mapa-múndi, bench, música), LUT e névoa do Brejo e das Terras Mortas (`render.ts`), mistura de texturas do chão por região (`terrain.ts`), ícones do Antídoto e dos materiais.

## Arte nova (Meshy)

Conceitos em `tools/art/remaster-brejo/prompts.mjs`; peças estáticas por `client/scripts/build-brejo-kit.mjs` + `tools/art/remaster-brejo/import.sh`; o Brejeiro pelo fluxo de monstros (`tools/art/remaster-monstros/models.mjs`, entrada `brejeiro`; `build-remaster-monstros.mjs brejeiro`; `import-asset.mjs ... monstro brejeiro_b --lod-prune --resize=1024`).

| Peça | Conceito | 3D | Rig | Ações | Total |
|---|---|---|---|---|---|
| Brejeiro (3.835 / 1.437 / 423 triângulos, 30 ossos, 1,7 MB) | 9 | 15 | 5 | 12 (Idle, Thrust Slash, Hit Reaction 1, Dead) | 41 |
| Tronco caído, cabana do barqueiro, bote (`nature_*_b.glb`) | 3 × 3 | 15 × 3 | | | 54 |
| Juncos (3 cartões com recorte, textura de 256 px) | 3 | | | | 3 |
| **Total** | | | | | **98** (teto 200) |

## Bench de contadores

A linha de base (`e2e/bench-baseline.json`) foi regerada com `npm run bench:baseline` (passou, 3,5 min no SwiftShader): entram as linhas do Brejo (dia 83 draws, noite 90) e das Terras Mortas (69 e 79), e as malhas ativas passam de 146 para 151 (as 4 peças novas do Brejo e o modelo do Brejeiro, que baixa sob demanda, mais as famílias da #135 vindas no merge). **Nas regiões antigas os draws subiram de 1 a 14** (por exemplo, Campos dos Ossos de 80 para 88 de dia): vêm de o chão passar a ser 480 × 480 m (mais recortes de natureza e água dentro do campo de visão) e do merge das famílias; o `world.json` delas não mudou. FPS de verdade (GPU) das regiões antigas **não foi medido** aqui: só contadores. Vale repetir o roteiro de celular (`docs/roteiro-celular-fps.md`) antes de publicar.

## O que ficou em aberto

- **Javali** (`boar`): não foi entregue pela #135; o Brejo não usa.
- **Lágrima de Vau** (conjunto "do Brejo", nível 18) e o **Cajado de Junco** (raro do mago): um item novo de arma entra nas linhas de encantamento do `TestPowerBudget` como exceção nova, e o teste não pode afrouxar. A Velha Escamosa solta os épicos do grau D no lugar.
- **Mãe-Lodo:** sem as fases 2 e 3 do roadmap (a divisão em 3 Lodos que voltam a curá-la, a poça que vai encolhendo o chão); só as duas habilidades e os Lodinhos. A fase por vida existe só no raid.
- **Brejeiros:** o "Paz com os Brejeiros" (hostilidade de 1 h por matar mais de 10 de dia) e a aldeia das palafitas.
- **Sino Afogado** (segredo), **Pedra-Selo** (não existe como item), **Foz do Lamagris** (portão para Maresia) e **Ponte de Troncos por missão** (hoje é só cenário): ficam para as tasks de portão e de Pedra-Selo.
- **Zona segura de 10 m do posto:** a zona segura é só a da cidade.
- **Vaga-lumes à noite, cogumelos que brilham, sons de sapo e do sino:** sem partículas nem áudio novos.
- **Terras Mortas (extensão):** só cenário; os monstros (24–28), o Bastião Cinzento e o Malthus são da task seguinte.
- **Cabana do Meshy:** a malha vem com um pedaço de poça azulada no chão, que aparece sob a cabana.

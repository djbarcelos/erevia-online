# Desempenho do remaster (revisão de FPS com GPU de verdade)

Tudo do remaster fica atrás de `?remaster=1`. Até aqui só havia contadores (draw calls) medidos no WebGL por software (SwiftShader), que não dizem nada sobre FPS. Esta revisão mediu com GPU de verdade, achou os gargalos e registra o que foi corrigido e o que ficou para depois. O roteiro para repetir no celular está em [roteiro-celular-fps.md](roteiro-celular-fps.md) (tunnel#64).

## Como medir

Máquina: notebook com duas GPUs. Chromium 159 do Playwright, sem janela, D3D11 via ANGLE, 1280x720, sem o limite de 60 FPS do vsync (para ver a folga).

**A GPU é de verdade.** `WEBGL_debug_renderer_info` (UNMASKED_RENDERER):

- NVIDIA: `ANGLE (NVIDIA, NVIDIA RTX 500 Ada Generation Laptop GPU (0x000028BA) Direct3D11 vs_5_0 ps_5_0, D3D11) / Google Inc. (NVIDIA)` (flag `--force-high-performance-gpu`).
- Intel (a placa integrada, sem a flag): `ANGLE (Intel, Intel(R) Arc(TM) Graphics (0x00007D55) Direct3D11 vs_5_0 ps_5_0, D3D11) / Google Inc. (Intel)`.

Ferramentas (em `e2e/`, o servidor é a imagem Docker do jogo com `DEV_LOGIN=true`):

| Comando | Para quê |
|---|---|
| `node bench-gpu.mjs --base URL --out DIR "nome:remaster=1&quality=media" ...` | roda o `?bench=1` completo (tour de câmera por todas as regiões, dia e noite). `--igpu` usa a Intel; `--cpu 4` deixa a CPU 4x mais lenta (CDP), com `--igpu` é o nosso "celular médio"; `--dpr 2 --width 1920 --height 1080` aperta a GPU |
| `... "nome:bench=aqui&remaster=1" --tp X,Z --db BANCO` | mede onde o personagem está (região de caça cheia): leva o personagem com `/tp` e mede de dia e de noite; `--profile` grava o perfil de CPU |
| `node bench-summary.mjs DIR` | tabela resumida (médias e pior região) |
| `node bench-download.mjs --base URL --net sem,4g,3g "nome:query"` | bytes baixados e tempo até o primeiro quadro, com cache vazio e rede simulada (CDP) |
| `node shots-compare.mjs --a URL --b URL "remaster=1&quality=media"` | compara os prints do cenário de dois servidores (prova de que o visual não mudou) |
| `go run ./cmd/loadbot -bots 50 -plaza 12` | 50 bots passeando pela praça, sem caçar (novo `-plaza`) |

O `?bench=1` agora também registra triângulos por quadro, p99, tempo de GPU (`EXT_disjoint_timer_query`), a CPU que sobra (lógica, física, interface) e uma estimativa da memória das texturas. `?bench=aqui` mede só onde o personagem está.

**Duas ressalvas honestas.**

1. Esta máquina é rápida e **fica presa na CPU** (CPU ~5 ms de um quadro de ~5,5 ms; GPU 2 a 4 ms): o FPS absoluto aqui não vale para um celular. Vale a comparação entre variantes, e o cenário "celular médio" (Intel + CPU 4x mais lenta, ~20 FPS) mostra o efeito multiplicado.
2. A máquina é compartilhada com outros agentes: a variação entre rodadas iguais chega a 20 a 30% (`rm-baixa` no "celular médio" deu 25,8 FPS numa rodada e 9 FPS noutra, com a máquina ocupada). Diferenças menores que isso são ruído. As primeiras rodadas usaram a flag `--disable-accelerated-2d-canvas` (herdada da config do CI, que roda em SwiftShader); ela foi tirada depois, porque força o canvas da interface a rodar na CPU. As tabelas abaixo dizem de qual conjunto vêm.

## Resultados (antes das correções)

FPS médio de todas as regiões, de dia e de noite; "pior região" é a de menor FPS médio. FPS p5 é o FPS que 95% dos quadros alcançam (1000 / p95 do tempo de frame). Canvas 2D acelerado (conjunto final).

### NVIDIA RTX 500 Ada, 1280x720, sem limite de FPS

| Cenário | FPS médio | Pior região | p5 médio | p5 pior | Frame (ms) | GPU (ms) | CPU (ms) | Anim. (ms) | Triângulos (mil) | Draws |
|---|---|---|---|---|---|---|---|---|---|---|
| Sem remaster, média | 187,5 | 124,8 (Cidade, noite) | 126,3 | 92,6 | 5,5 | 2,3 | 4,9 | 0,3 | 1.460 | 156 |
| Remaster, baixa | 190,4 | 121,7 (Cidade, noite) | 130,4 | 92,6 | 5,4 | 2,1 | 4,8 | 0,5 | 2.765 | 100 |
| Remaster, média | 165,2 | 65,4 (Cidade, noite)* | 104,0 | 50,3 | 6,6 | 2,5 | 5,9 | 0,5 | 2.766 | 106 |
| Remaster, alta | 155,4 | 90,8 (Cidade, noite) | 94,7 | 46,1 | 6,8 | 3,7 | 6,1 | 0,4 | 4.313 | 147 |

\* um pico de ruído da máquina compartilhada (na rodada anterior, com o canvas por software, a mesma variante deu 110 FPS na pior região).

Conjunto anterior (canvas 2D por software), com sem-remaster em baixa, média e alta: 177 / 171 / 138 FPS médios; remaster: 184 / 168 / 146. As duas séries dizem o mesmo: **o remaster em baixa e média custa o mesmo que o jogo de hoje; em alta fica ~5% abaixo.** Os draw calls do remaster são *menores* (100 contra 150 em baixa) porque as peças da natureza e da cidade são poucas malhas com instâncias.

### Placa integrada Intel Arc (canvas por software)

| Cenário | FPS médio | Pior região | p5 médio | GPU (ms) | CPU (ms) |
|---|---|---|---|---|---|
| Sem remaster, média | 170,4 | 100,3 | 110,0 | 3,9 | 5,3 |
| Remaster, baixa | 198,6 | 107,5 | 132,7 | 3,7 | 4,6 |
| Remaster, média | 182,1 | 115,0 | 117,9 | 4,7 | 5,0 |
| Remaster, alta | 129,4 | 80,4 | 73,8 | 7,5 | 7,0 |

### "Celular médio" (Intel Arc + CPU 4x mais lenta, canvas por software)

| Cenário | FPS médio | Pior região | p5 médio | p5 pior | Frame (ms) | GPU (ms) | CPU (ms) | Draws |
|---|---|---|---|---|---|---|---|---|
| Sem remaster, média | 20,8 | 13,8 (Cidade, noite) | 13,1 | 8,7 | 49,5 | 22,3 | 40,4 | 156 |
| Remaster, baixa | 25,8 | 16,2 (Cidade, dia) | 15,9 | 10,6 | 40,4 | 22,4 | 32,7 | 100 |
| Remaster, média | 22,3 | 13,8 (Cidade, dia) | 13,8 | 10,0 | 46,5 | 25,9 | 37,7 | 106 |
| Remaster, alta | 21,0 | 11,1 (Cidade, dia) | 12,6 | 7,2 | 50,0 | 32,2 | 41,2 | 147 |

Ou seja: **o remaster não é um problema de FPS** em nenhuma das duas placas; a Cidade (21 peças novas, tochas, estandartes) é a pior região em todas as linhas, com ou sem remaster.

### 50 jogadores na praça (NVIDIA, canvas acelerado, `bench=aqui`, 50 bots do loadbot)

| Cenário | FPS médio | p5 | Frame (ms) | CPU (ms) | Animação (ms) | GPU (ms) | Draws | Triângulos (mil) |
|---|---|---|---|---|---|---|---|---|
| Sem remaster, média | 27,6 | 23 | 36,2 | 33,9 | 7,5 | 25,5 | 709 | 2.381 |
| Remaster, média | 28,0 | 23 | 35,8 | 33,7 | 8,3 | 26,4 | 538 | 3.689 |

O servidor aguentou os 50 bots sem erro (1,3 MB/s de banda, ping p95 de 4 ms). Para gente em volta da praça sem caçar: `go run ./cmd/loadbot -bots 50 -plaza 12`.

### Regiões de caça cheias (NVIDIA, canvas por software, `bench=aqui`)

| Região | Sem remaster | Com remaster | Monstros perto | Animação (ms) sem / com |
|---|---|---|---|---|
| Campos dos Ossos (95, 0) | 78,7 FPS dia, 68,0 noite | 79,7 dia, 69,0 noite | 10 | 0,6 / 0,8 |
| Vale das Pedras Vivas (0, -262) | 99,2 dia, 107,8 noite | 71,8 dia, 69,6 noite | 13 (24 na área) | 0,3 / 0,8 |

Campos dos Ossos: empate. Vale: o remaster perde ~30% (CPU 8,5 contra 12,2 ms): os 24 golems têm 3 malhas e 38 nós cada, e o tempo "resto" (a lógica do cliente e a interface) sobe 3 ms. A rodada seguinte, repetida, deu 96 FPS para a mesma variante: **o ruído é do tamanho da diferença**, então a conclusão é "pode ter um custo de golems, ainda por confirmar num aparelho de verdade".

## Downloads

Cache vazio, nada no cache do navegador, rede simulada pelo CDP. "Fast 3G" = 1,6 Mbps e 150 ms de RTT. Os modelos do remaster **não** eram carregados sob demanda: a função `CharacterModels.load` baixava tudo de uma vez (4 heróis, 5 monstros, golem, 2 chefes, 17 armas e as peças da natureza e da cidade) enquanto o jogador ainda via cápsulas.

| Cenário | Pedidos | MB baixados até os modelos prontos | Tela jogável (cápsulas) | Modelos prontos |
|---|---|---|---|---|
| Sem remaster, rede local | 368 | 25,5 (15,5 de modelos) | 17,1 s | (junto) |
| Remaster antes, rede local | 373 | 91,0 (80,4 de modelos) | 5,8 s | (junto, ~5,8 s) |
| Remaster depois, rede local | 370 | 64,2 até ficar pronto, +13,4 no tempo ocioso (77,6 no total) | 10,6 s | 11,8 s |
| Sem remaster, Fast 3G | 310 | 18,8 em 5 min (a carga não terminou) | 69,4 s | n/d |
| Remaster antes, Fast 3G | 336 | 56,0 em 5 min (a carga não terminou) | 68,4 s | n/d (estimado em ~7,5 min só pelos 91 MB) |
| Remaster depois, Fast 3G | 333 | 42,6 (tudo o que importa) | 67,5 s | **237,7 s** |

Leitura:

- "Tela jogável" é quando a tela de carregamento some (o jogador já vê o mundo, com cápsulas no lugar dos modelos); em rede boa os tempos de 5 a 17 s variam muito entre rodadas, porque a máquina é compartilhada. Em 3G ela é igual (~68 s) nos três casos: só depende do JavaScript (8,6 MB) e do mapa.
- Os modelos 3D (personagens) são o que pesa: **o remaster baixa ~65 MB de modelos para ficar pronto contra ~15 MB do jogo de hoje.** Os números de MB contam os pedidos do navegador com o cache desligado pelo CDP: o mesmo arquivo pedido por vários nomes de arma conta várias vezes. Os arquivos únicos do remaster somam 44,7 MB (67 arquivos).
- Depois da correção, o golem e os chefes (13,4 MB) saem da carga inicial: **~30% menos bytes até os modelos ficarem prontos** (91 contra 64 MB) e, em 3G, os modelos ficam prontos em ~4 min em vez de passar de 7 (os 91 MB a 200 KB/s). Em 4G sem "economizar dados" os três baixam sozinhos 20 s depois, no tempo ocioso; em 3G só quando aparecem.
- Não ficou sob demanda: os 4 heróis (outros jogadores podem ser de qualquer classe) e os 4 esqueletos comuns (~20 MB). Mexer nisso exige carregar por classe e por região: ver "O que ficou para depois".
- O jogo de hoje já passa de 25 MB no primeiro carregamento (8,6 MB de JavaScript e 15,5 de modelos): em 3G ele também leva mais de 2 min até os modelos.

## Gargalos achados

1. **Muita gente na tela é o pior caso, com ou sem remaster.** 50 jogadores na praça (`loadbot -bots 50 -plaza 12`, NVIDIA, `bench=aqui`): **28 FPS** com e sem remaster (p5 23), contra ~190 sem gente. CPU 34 ms: desenho 9, escolha das malhas ativas 6, animação 7,5 a 8,3, e 9 de "resto" (lógica do cliente e interface); 709 draw calls sem remaster e 537 com (o remaster tem *menos* draws, mas 3,7 contra 2,4 milhões de triângulos). No perfil de CPU, `AnimationGroup.animate` é 22% do tempo e `AdvancedDynamicTexture._checkUpdate` 10%. O remaster não piora isto (a animação por esqueleto de 30 ossos custa o mesmo que a do KayKit), mas é onde um celular vai sofrer primeiro.
   (Uma armadilha da medição: com o canvas 2D por software, a flag herdada da config do CI, `texImage2D` da textura da interface parecia ocupar 25% do tempo no Vale. Com o canvas acelerado ele cai para menos de 2%. As primeiras tabelas foram feitas com a flag; o conjunto final, sem.)
2. **O gargalo do quadro é a CPU, não os triângulos.** O mapa do remaster tem 1,0 milhão de triângulos de natureza (2,7 milhões por quadro com as sombras) contra 349 mil do KayKit, mas o tempo de GPU fica em 2 a 4 ms. A natureza são thin instances de um tipo só para o mapa inteiro (a caixa cobre o mapa: o Babylon nunca corta) e o mapa de sombras desenha tudo mesmo vendo só 70 m. Só que cortar isso não ganhou FPS (próxima seção).
3. **Peso do download e da carga:** os 2 chefes e o golem eram ~14 MB dos ~31 MB de modelos do remaster, baixados e decodificados no primeiro carregamento, e o jogador só os vê em dois covis e no Vale.
4. **A Cidade** é a região mais pesada em tudo (draws 207, 21 tipos de peça, tochas, plaza): ~35% menos FPS que as outras. Não é do remaster (já era a pior).
5. Memória de texturas estimada: ~350 a 430 MB (a maior é o `groundTex` de 2560x4096, 53 MB, e o mapa de sombras, 16 MB). O remaster usa menos que o KayKit (349 contra 418 MB, em baixa).

Não é gargalo: animação (0,3 a 0,9 ms mesmo com 24 monstros: o `AnimController` já pausa quem está longe ou fora da tela), draw calls, malhas ativas, matrizes (todo o cenário já é `freezeWorldMatrix` com `doNotSyncBoundingInfo`).

## Correções

### 1. Corpos pesados sob demanda (entrou)

`REMASTER_LAZY` (`remaster.ts`): o golem e os dois chefes (~14 MB) não entram mais na carga inicial. `CharacterModels.ensure` baixa o corpo quando uma entidade dele aparece (a entidade fica como cápsula até chegar, nunca como o esqueleto de outro) e, em rede 4G sem "economizar dados", `prefetchLazy` baixa os três no tempo ocioso, 20 s depois dos modelos. Sem `?remaster=1` nada muda. Ganho: ver a tabela de downloads. O cliente marca `performance.mark("erevia:modelos")` e `("erevia:cenario")`, e `bench-download.mjs` lê.

### 2. Recortes da natureza e corte do mapa de sombras (medido, **ficou desligado**)

`chunks.ts` divide cada tipo pesado da natureza em recortes de 64 m, cada um com a própria caixa (o Babylon passa a cortar pelo campo de visão) e tira do mapa de sombras os recortes fora do volume da luz (corte sem perda; testado em `client/tests/chunks.test.ts`). **Medido: corta os triângulos de 2,77 para 1,66 milhão por quadro, mas não ganha FPS**: NVIDIA, remaster média, 168 → 164 (células de 64 m) e 161 (128 m); "celular médio", 22,3 → 19,9 e 19,6; com todos os tipos (inclusive o mato), 17,6. Os draw calls sobem (106 → 134 a 159) e o quadro é limitado pela CPU. Por isso fica **opcional**: `?remaster=1&chunks=1` (`&chunk=N` muda o tamanho da célula, `&chunkmin=N` o mínimo de triângulos do tipo). Vale testar num celular de verdade (tunnel#64), onde a GPU pode pesar mais que aqui.

Prints de `shots-compare.mjs` (com o recorte ligado) contra o servidor sem ele ficaram dentro do ruído de duas rodadas do mesmo servidor (vento, tochas e a interface mudam entre prints): o mapa de sombras e os recortes não mudam o visual. A primeira versão tinha um bug, pego pelo print: clones de uma malha com thin instances dividem a geometria e um sobrescrevia as matrizes dos outros (por isso cada recorte copia a geometria).

### 3. Ferramentas de medição (entrou)

O `?bench=1` e as ferramentas da tabela do início, mais os parâmetros de medição `?gui=0` e `?guiscale=0.5` (escondem a interface 3D ou desenham a textura dela pela metade; sem efeito por padrão).

## O que ficou para depois

1. **Multidão (item 1 dos gargalos).** O custo é a animação e o desenho por jogador (14 draw calls e 40 nós cada). Saídas, nenhuma feita porque mudam o visual ou precisam de medição num celular: animar a 30 Hz os jogadores a mais de ~15 m (no preset baixo); juntar as malhas de cada jogador (corpo, cabeça, armas) numa só; tirar o bônus de 4 materiais por jogador. Se o celular for lento na praça, é aqui. A interface 3D também pode ser atualizada a 30 Hz no baixo; `?guiscale=0.5` já permite medir o efeito de baixar a resolução dela.
2. **ETC1S no lugar de UASTC** nos modelos (cada herói 2,4 a 2,8 MB, cada chefe 6 MB): corta o download a uma fração, mas a perda de detalhe nas pinturas já foi vista (`remaster-v3.md`); precisa de comparação de prints por arquivo.
3. **LOD por distância das thin instances** (os arquivos têm LOD1): só faz sentido depois de ver no celular se os triângulos pesam; os recortes são o primeiro passo.
4. **`material.freeze()`** nos materiais do cenário: não foi tentado porque o ciclo dia/noite muda os defines dos materiais (sombra, névoa, luzes) e um material congelado não os recalcula; precisaria descongelar na troca de hora.
5. **VAT / instâncias de esqueleto** (remaster-monstros.md): a animação ficou em 0,3 a 0,9 ms com 24 monstros; na praça com 50 jogadores chega a 8 ms, e aí o VAT para jogadores passa a valer a conversa.
6. **Carregar os heróis e os esqueletos por demanda** (~30 MB): o jogador carrega só a própria classe e as outras quando alguém dela aparece.
7. **O Vale com remaster** (24 golems) pareceu ~30% mais lento numa rodada e igual na seguinte: repetir com `bench=aqui` num aparelho de verdade.
8. Memória: o `groundTex` (53 MB) e o mapa de sombras (16 MB) são os maiores; o primeiro pode ser em 1/4 da resolução fora da cidade.

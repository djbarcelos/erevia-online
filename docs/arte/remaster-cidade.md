# Remaster: a cidade de Vel'Aran no estilo novo (tunnel#100)

Os prédios, a muralha, o portão, a torre e os adereços da cidade foram refeitos pelo **Meshy** (conceito 2D, imagem para 3D com topologia inteligente), no estilo pintado à mão da v3 (`docs/arte/remaster-v3.md`). Tudo atrás de `?remaster=1`: sem o parâmetro nada muda. O layout, a colisão e os NPCs são os de sempre: **o servidor e o world.json não mudaram**.

Como olhar: `/` e `/?remaster=1`, no mesmo ponto (`/tp 2 7` é a praça, `/tp 28 0` o portão leste), de dia e de noite.

## O que entrou

| Peça (`client/public/models/`) | Substitui | Triângulos | Arquivo | No mundo | Créditos |
|---|---|---|---|---|---|
| `town_tavern_b.glb` taverna | `tavern` | 2.740 | 342 KB | 10,5 x 12,6 x 8,1 m | 24 |
| `town_market_b.glb` loja/mercado | `market` | 2.146 | 301 KB | 12,7 x 10,2 x 11,0 m | 24 |
| `town_blacksmith_b.glb` ferraria | `blacksmith` | 2.946 | 327 KB | 8,5 x 9,2 x 11,2 m | 24 |
| `town_church_b.glb` igreja | `church` | 2.813 | 338 KB | 9,3 x 14,2 x 8,5 m | 24 |
| `town_home_a_b.glb` casa A (chalé) | `home_a` | 1.694 | 282 KB | ~5,8 x 7,0 x 5,7 m | 24 |
| `town_home_b_b.glb` casa B (sobrado) | `home_b` | 2.070 | 311 KB | ~6,0 x 11,0 x 5,9 m | 24 |
| `town_tower_b.glb` torre de canto | `tower_a` | 2.280 | 315 KB | 8,7 x 19,5 x 9,8 m | 24 |
| `town_wall_b.glb` trecho de muralha | `wall_straight` | 1.938 | 298 KB | tira de 12,8 x 7,2 x 3,2 m | 24 |
| `town_gate_b.glb` portão | `wall_straight_gate` | 2.340 | 303 KB | tira de 12,8 x 9,2 x 3,6 m | 24 |
| `town_corner_b.glb` torreão de canto da muralha (16) | novo | 1.755 | 149 KB | 3,8 x 7,6 x 3,8 m | 24 |
| `town_barrel_b.glb` barril | `barrel` | 832 | 111 KB | ~0,8 x 0,9 x 0,8 m | 24 |
| `town_crate_b.glb` caixote | `crate_a_big`, `crate_long_a` | 781 | 119 KB | ~0,8 m | 24 |
| `town_banner_b.glb` bandeira de portão | `flag_blue` | 942 | 115 KB | ~0,9 x 2,9 m | 24 |
| `town_stall_b.glb` barraca de feira | `tent` | 1.487 | 141 KB | ~3,1 x 3,1 x 2,4 m | 24 |
| `town_cart_b.glb` carroça | `wheelbarrow` | 1.446 | 146 KB | ~2,5 x 1,3 x 1,4 m | 24 |
| `town_bench_b.glb` banco (4, em volta da fonte) | novo | 786 | 103 KB | 1,9 x 1,2 x 1,0 m | 24 |
| `town_lamp_b.glb` poste de luz (4, entre as tochas) | novo | 1.045 | 109 KB | 3,6 m de altura | 24 |
| `town_sign_b.glb` placa de rua (4, depois dos portões) | novo | 834 | 110 KB | 2,4 m de altura | 24 |
| `town_tomb_b.glb` lápide (Campos dos Ossos, 16) | novo | 945 | 109 KB | 1,5 m | 24 |
| `town_crystal_b.glb` cristal âmbar (Vale das Pedras Vivas, 12) | novo | 1.238 | 115 KB | 2,4 m | 24 |
| `town_pillar_b.glb` pilar rachado (Ermos Partidos, 10) | novo | 1.534 | 119 KB | 2,6 m | 24 |

Todas dentro do orçamento de cenário do guia (≤ 3.000 triângulos, 1 material, textura ≤ 1024 px): **nenhum marco precisou do orçamento de "chefe"** e não há exceção nova no manifesto. As 21 peças somam ~4,4 MB.

Barril e caixote agora também são novos (segunda rodada, abaixo).

### Créditos

Por peça: conceito 2D (`nano-banana-pro`, 9) + imagem para 3D (`meshy-t2.5`, `smart-topology`, textura 2K, 15) = **24**. 17 peças = **408 créditos** de 600, 34 chamadas, nenhuma repetida: todos os conceitos serviram de primeira. O saldo da conta ficou acima de 1.500 ao fim (a regra de parar abaixo de 300 nunca chegou perto). O registro está em `assets-src/meshy/tasks.jsonl` (fora do git).

`target_polycount`: 3.000 (taverna, ferraria, igreja, mercado), 2.000 a 2.500 (casas, torre, muralha, portão), 800 a 1.500 (adereços).

## Como foi montado

1. **Conceito 2D** por peça (`tools/art/remaster-cidade/prompts.mjs`): pintado à mão, fundo cinza liso, vista 3/4 de cima, paleta do guia (pedra quente `#c9b79c`, madeira `#7a5232`, estandartes `#9e2b25`, ouro `#d6b46e`; telhados terracota, azul-ardósia e verde-petróleo; as peças de região usam as cores delas). Prints: `conceitos-predios.jpg`, `conceitos-muralha-e-adereco.jpg`, `conceitos-regioes.jpg`.
2. **Imagem para 3D** (`to3d.sh`): a malha nasceu limpa e no orçamento, sem retopologia.
3. **Prova no Blender headless** (`piece.py`, 4 vistas por peça, `blender-*.jpg`): medidas e triângulos antes da integração.
4. **Normalização** (`client/scripts/build-remaster-town.mjs`): vira a frente (o Meshy olha para +Z; o kit KayKit, para -Z, e o gerador do mundo aponta -Z para a praça), centra a pegada, põe o chão em y = 0 e escala **para a pegada do prédio KayKit equivalente**. A escala do world.json é aplicada em cima como antes, então ela já está embutida na malha, em "unidades do kit".
   - Prédios: escala uniforme, a maior que cabe na largura e no fundo do KayKit, em até 1,15x da altura e com a meia diagonal da pegada até 1,5x o raio do colisor do servidor.
   - Muralha e portão: cada eixo vai exatamente ao tamanho KayKit, porque a tira tem de emendar com a vizinha e o vão do portão fica onde o servidor deixa passar.
5. **Importação** (`import.sh` → `import-asset.mjs`): textura 1024 px (512 px nos adereços), KTX2 **ETC1S**, categoria `cenario`, sem LOD.
6. **Jogo** (`client/src/world/town-layout.ts` e `town-remaster.ts`, ligados em `environment.ts`): com `?remaster=1`, cada tipo do world.json que tem peça nova usa a malha nova no lugar da KayKit, pelo mesmo caminho de thin instances de antes (sombra, congelamento). Os adereços sem item no kit (banco, poste, placa e os de região) entram como objetos extras, só visuais, no mesmo caminho.

### Decisões

- **Prédios inteiros como peças**, não kit de paredes e telhados: o gerador faz paredes e telhados mal, e um prédio inteiro é 1 malha, 1 material e 1 draw call por tipo.
- **Sem LOD**: o LOD do Babylon é por malha, e as peças são thin instances de uma malha só (a taverna tem 1 cópia, a casa 8, a muralha 16). Uma cópia longe pagaria o LOD da mais próxima. A malha já é leve de saída; os prédios da cidade somam ~105 mil triângulos.
- **ETC1S, não UASTC**: 14 peças em UASTC de 1024 px dariam ~20 MB (1,4 MB cada); em ETC1S são ~300 KB. Vistas a 18 m, a perda do pintado não aparece.
- **Pegada, depois altura**: a escala é uniforme até a pegada caber no colisor (meia diagonal até 1,5x o raio do colisor circular do servidor); a altura que sobra é recuperada na segunda rodada, esticando só em Y.
- **Normais zeradas refeitas**: o Meshy entrega de 4 a 58 vértices por prédio com normal zerada, que aparecem como triângulos pretos no jogo (vi na taverna e nos telhados). `build-remaster-town.mjs` refaz a normal deles com a das faces vizinhas.
- **Metal 0, rugosidade 1** no carregamento: o glTF do Meshy chega com brilho de plástico, que não combina com pedra e telha pintadas.
- As tochas das ruas continuam as do jogo (ver "Segunda rodada").
- `client/tests/town-remaster.test.ts` (no `npm run test:unit`) confere: as peças existem e estão no manifesto, a pegada de cada prédio cabe no colisor do servidor (até 1,6x), a tira da muralha e do portão tem o tamanho KayKit, nenhum NPC cai dentro de um prédio novo e os adereços novos ficam a 2 m de NPC e fora de colisor e de prédio.

### Luz: AO pintado na textura, sem lightmap

O pedido preferia lightmap e AO pré-calculados num bake do Blender com uma segunda UV. **Não fiz**, por três motivos:

1. **O Meshy já entrega a luz pintada na textura** (sombra nas dobras, junções, beirais, degraus de pedra): os prints mostram o volume sem luz cara, que é o que o guia pede (seção 4, "luz na textura").
2. **Thin instances**: 8 casas de cada tipo e 16 trechos de muralha dividem uma malha e uma textura. Um lightmap é por instância (a luz depende do lado em que a casa está virada), ou seja, dezenas de texturas de bake no lugar de 9. O AO de contato, igual para todas, já está pintado.
3. **Custo**: uma segunda UV e uma textura de bake por peça dobrariam a memória de textura e o tempo de importação, na região de orçamento mais apertado do jogo.

O sol e a sombra projetada em tempo real completam a luz (as peças entram em `CASTS_SHADOW` e `RECEIVES_SHADOW` pelo prefixo `town_`).

## Números (`?bench=contadores`, Chromium com SwiftShader, qualidade baixa, servidor de dev do Vite)

O WebGL é por software: FPS e milissegundos não valem como desempenho de GPU, só os contadores. Draw calls e malhas ativas, sem → com `?remaster=1`:

| Região | Hora | Draws | Malhas ativas / total |
|---|---|---|---|
| **Cidade** | dia | **204 → 188** | 112 / 313 → 105 / 334 |
| **Cidade** | noite | **205 → 194** | 110 / 338 → 108 / 336 |
| Arredores | dia | 207 → 197 | 118 / 320 → 117 / 334 |
| Campos dos Ossos | dia | 180 → 180 (as lápides entram) | 91 / 320 → 99 / 335 |
| Ermos Partidos | dia | 158 → 153 | 69 / 320 → 72 / 335 |
| Vale das Pedras Vivas | dia | 155 → 150 | 66 / 320 → 69 / 335 |

O remaster da cidade **não custa draw calls: -16 de dia e -11 de noite** (cada tipo de prédio continua 1 draw por thin instances; a fonte no lugar do poço e a barraca nova ajudam). O bench sem `?remaster=1` não depende do código novo: `environment.ts` só age com o parâmetro. O FPS do software (1 a 3 FPS dos dois lados) está dentro do ruído; a Cidade de dia, primeira região depois da carga, é um ponto fora da curva por causa da transcodificação do KTX2. **FPS de GPU real não foi medido** (tunnel#64). Os JSON completos ficam em `assets-src/shots/` (fora do git).

## Prints (`docs/arte/remaster-cidade/`)

- Conceitos: `conceitos-predios.jpg`, `conceitos-muralha-e-adereco.jpg`, `conceitos-regioes.jpg`.
- Peças no Blender headless, 4 vistas (frente, 3/4, lado, costas): `blender-a-...` a `blender-h-regioes.jpg`.
- Regiões (lápides, pilares e cristais): `regiao-sem-<campos|ermos|vale>.jpg` e `regiao-rm-<...>.jpg`.
- Jogo, sem e com `?remaster=1`, de dia e de noite, em 8 pontos (praça, portão, rua do mercado, taverna, igreja, ferreiro, muralha, casas): `jogo-sem-dia-<ponto>.jpg`, `jogo-rm-dia-<ponto>.jpg`, `jogo-sem-noite-<ponto>.jpg`, `jogo-rm-noite-<ponto>.jpg`.

## Como regerar

1. `node tools/art/remaster-cidade/prompts.mjs`, depois `sh tools/art/remaster-cidade/concepts.sh <peças>` (9 créditos cada) e `sh tools/art/remaster-cidade/to3d.sh <peça>:<polycount> ...` (15 cada). O teto da rodada é a variável `MESHY_BUDGET`.
2. `node tools/art/remaster-cidade/index-concepts.mjs` e, no Blender, `blender --background --factory-startup --python tools/art/remaster-cidade/piece.py -- <pasta> <glb>,<glb> 400`.
3. `sh tools/art/remaster-cidade/import.sh`.
4. Prints e bench: `shots.mjs`, `shots-all.sh`, `bench.mjs` (precisam de um servidor de prova e do Vite; veja o cabeçalho de cada um).

## Segunda rodada (as três pendências)

Pegou a main integrada (monstros e armas do remaster); os números abaixo são dessa base.

### 1. Prédios mais altos e muralha sem postes de canto

- **Altura**: `build-remaster-town.mjs` ganhou `stretchY` por peça: depois de achar a escala uniforme que cabe na pegada, estica só em Y até a altura KayKit, com um teto para não deformar a pintura. Taverna 10,6 → 12,6 m (+18%), igreja 10,5 → 14,2 m (+35%, o teto; a KayKit tem 14,8 m), casa A 5,6 → 7,0 m (+25%), ferraria 8,0 → 9,2 m. A pegada não mudou (o teste do colisor continua passando). A casa B e o mercado já tinham a altura.
- **Muralha**: gerei um **torreão de canto** próprio (`town_corner_b.glb`, 1.755 triângulos, conceito `conceitos-novos.jpg`), 3,8 x 7,6 x 3,8 m, posto em cada um dos 16 vértices entre dois trechos (nos 4 vértices que já têm torre grande não vai). Ele cobre a emenda e os postes das pontas. Os escudos pintados nas ameias de cada trecho continuam (são decoração, não emenda). Só visual: não muda colisor (`wallCorners()` em `town-layout.ts`, com teste).

### 2. Barril, caixote e bandeira

- `town_barrel_b.glb` (832 triângulos), `town_crate_b.glb` (781) e `town_banner_b.glb` (942): trocam `barrel`, `crate_a_big`/`crate_long_a` (a mesma peça nos dois, na escala do world.json) e a `flag_blue` dos portões, com a mesma escala embutida na malha.
- **Tocha de parede: não gerei.** As 35 tochas da cidade são um sistema próprio (`torches.ts`: poste, braseiro, chama por shader, luz de verdade, faíscas), mais elaborado do que um modelo estático do Meshy seria; trocar só o poste perderia a chama. Fica como está.
- Créditos: 4 peças x 24 = 96 (conceito 9 + imagem para 3D 15). Total da cidade: **504 de 600**, 21 peças. Saldo da conta ao fim: ver o relatório (outros agentes também gastam dele).

### 3. Luz: AO por vértice, testado e deixado desligado

Fiz o teste que o Daniel pediu, sem dobrar a textura e funcionando com thin instances:

- `client/scripts/lib/vertex-ao.mjs`: AO por vértice em JavaScript (48 raios em semiesfera com peso de cosseno, a 12% do maior lado da peça, mais o chão em y = 0 como oclusor para o contato da base), determinístico. Em vez de Blender headless com BVH, usei JS puro: a malha tem ≤ 3.000 triângulos, o cálculo leva ~15 s por peça e não precisa casar vértices entre dois programas.
- O valor vai em `COLOR_0` (de 0,55 a 1,0) em cada peça e o jogo o multiplica pela cor base (`mesh.useVertexColors`). Como é geometria da própria peça, é igual em todas as instâncias e não custa textura (+10 KB por peça).
- **Resultado** (`jogo2-ao-dia-*` contra `jogo2-sem-ao-dia-*`, e `jogo2-*-noite-*`): de dia, as bases dos prédios e embaixo dos beirais ficam um pouco mais assentadas, mas a cena inteira perde ~10% de brilho e a cor do telhado esmaece; **de noite só escurece**, e a praça já tem pouca luz. A pintura do Meshy já traz AO nas dobras, e o AO de contato do mundo (a sombra projetada) cobre o resto. Não ficou visivelmente melhor.
- **Decisão**: ligado só com `?remaster=1&ao=1` (para comparar); o padrão é desligado. Os GLB seguem com o `COLOR_0` (a ferramenta fica pronta; `--no-ao` no `build-remaster-town.mjs` o tira).

### Números da segunda rodada (`?bench=contadores`, mesma máquina, base com a main integrada)

| Região | Hora | Draws sem → com `?remaster=1` | Malhas ativas |
|---|---|---|---|
| **Cidade** | dia | **194 → 186** | 107 → 102 |
| **Cidade** | noite | **200 → 193** | 110 → 106 |
| Arredores | dia | 199 → 184 | 115 → 108 |
| Campos dos Ossos | dia | 167 → 162 | 83 → 85 |
| Ermos Partidos | dia | 151 → 149 | 67 → 72 |
| Vale das Pedras Vivas | dia | 150 → 147 | 66 → 70 |

A Cidade continua com menos draw calls com o parâmetro (-8 de dia, -7 de noite): cada tipo novo é 1 draw, e o barril, o caixote e a bandeira substituem tipos que já existiam. O bench sem o parâmetro não depende do código novo.

Prints da segunda rodada (`docs/arte/remaster-cidade/`): `jogo2-sem-ao-dia-<ponto>.jpg` (a cidade nova, mais alta, com o canto e o barril, sem AO), `jogo2-ao-dia-<ponto>.jpg` (com AO), e `jogo2-sem-ao-noite-*` / `jogo2-ao-noite-*`. Para o "antes" desta rodada, os da primeira (`jogo-rm-dia-*`). Os pontos são praça, portão, mercado, taverna, igreja, muralha e casas (de noite, praça, muralha e igreja). Blender: `blender-i-barril-caixote-canto-bandeira.jpg`; conceitos: `conceitos-novos.jpg`.

## O que ficou em aberto

1. **Tocha de parede** (ver acima): fica o sistema procedural.
2. **Lightmap/AO pré-calculado**: testado e desligado por padrão (ver "Luz"). Se um dia a cidade ganhar luz de verdade nas janelas à noite, vale refazer o teste com o AO só nas peças que recebem essa luz.
3. **Regiões**: só 3 adereços (lápide, cristal, pilar); a identidade completa (Arredores, Cripta, casas e muros delas) fica para uma rodada própria.
4. **Dentes da muralha**: as ameias de cada trecho têm escudos pintados e o corte do kit deixa um degrau pequeno na junção que o torreão cobre só em parte nos vértices dos portões.
5. **FPS em GPU real** (tunnel#64).

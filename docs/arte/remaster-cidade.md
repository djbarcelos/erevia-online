# Remaster: a cidade de Vel'Aran no estilo novo (tunnel#100)

Os prédios, a muralha, o portão, a torre e os adereços da cidade foram refeitos pelo **Meshy** (conceito 2D, imagem para 3D com topologia inteligente), no estilo pintado à mão da v3 (`docs/arte/remaster-v3.md`). Tudo atrás de `?remaster=1`: sem o parâmetro nada muda. O layout, a colisão e os NPCs são os de sempre: **o servidor e o world.json não mudaram**.

Como olhar: `/` e `/?remaster=1`, no mesmo ponto (`/tp 2 7` é a praça, `/tp 28 0` o portão leste), de dia e de noite.

## O que entrou

| Peça (`client/public/models/`) | Substitui | Triângulos | Arquivo | No mundo | Créditos |
|---|---|---|---|---|---|
| `town_tavern_b.glb` taverna | `tavern` | 2.740 | 342 KB | 10,5 x 10,6 x 8,1 m | 24 |
| `town_market_b.glb` loja/mercado | `market` | 2.146 | 301 KB | 12,7 x 10,2 x 11,0 m | 24 |
| `town_blacksmith_b.glb` ferraria | `blacksmith` | 2.946 | 327 KB | 8,5 x 8,0 x 11,2 m | 24 |
| `town_church_b.glb` igreja | `church` | 2.813 | 338 KB | 9,3 x 10,5 x 8,5 m | 24 |
| `town_home_a_b.glb` casa A (chalé) | `home_a` | 1.694 | 282 KB | ~5,8 x 5,6 x 5,7 m | 24 |
| `town_home_b_b.glb` casa B (sobrado) | `home_b` | 2.070 | 311 KB | ~6,0 x 11,0 x 5,9 m | 24 |
| `town_tower_b.glb` torre de canto | `tower_a` | 2.280 | 315 KB | 8,7 x 19,5 x 9,8 m | 24 |
| `town_wall_b.glb` trecho de muralha | `wall_straight` | 1.938 | 298 KB | tira de 12,8 x 7,2 x 3,2 m | 24 |
| `town_gate_b.glb` portão | `wall_straight_gate` | 2.340 | 303 KB | tira de 12,8 x 9,2 x 3,6 m | 24 |
| `town_stall_b.glb` barraca de feira | `tent` | 1.487 | 141 KB | ~3,1 x 3,1 x 2,4 m | 24 |
| `town_cart_b.glb` carroça | `wheelbarrow` | 1.446 | 146 KB | ~2,5 x 1,3 x 1,4 m | 24 |
| `town_bench_b.glb` banco (4, em volta da fonte) | novo | 786 | 103 KB | 1,9 x 1,2 x 1,0 m | 24 |
| `town_lamp_b.glb` poste de luz (4, entre as tochas) | novo | 1.045 | 109 KB | 3,6 m de altura | 24 |
| `town_sign_b.glb` placa de rua (4, depois dos portões) | novo | 834 | 110 KB | 2,4 m de altura | 24 |
| `town_tomb_b.glb` lápide (Campos dos Ossos, 16) | novo | 945 | 109 KB | 1,5 m | 24 |
| `town_crystal_b.glb` cristal âmbar (Vale das Pedras Vivas, 12) | novo | 1.238 | 115 KB | 2,4 m | 24 |
| `town_pillar_b.glb` pilar rachado (Ermos Partidos, 10) | novo | 1.534 | 119 KB | 2,6 m | 24 |

Todas dentro do orçamento de cenário do guia (≤ 3.000 triângulos, 1 material, textura ≤ 1024 px): **nenhum marco precisou do orçamento de "chefe"** e não há exceção nova no manifesto. As 17 peças somam ~3,7 MB.

Barril e caixote ficaram os do KayKit (o agente de `remaster-estaticos` pode trocá-los; o prefixo `town_` evita choque de nomes).

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
- **Pegada, não altura**: algumas peças ficaram mais baixas que as KayKit (casa A 5,6 m contra 7 m, igreja 10,5 contra 14,8), porque a meia diagonal não pode passar de 1,5x o raio do colisor (o do servidor é um círculo só). Isso garante "sem parede nova em cima de rua nem porta bloqueada". O custo é uma cidade um pouco mais baixa.
- **Normais zeradas refeitas**: o Meshy entrega de 4 a 58 vértices por prédio com normal zerada, que aparecem como triângulos pretos no jogo (vi na taverna e nos telhados). `build-remaster-town.mjs` refaz a normal deles com a das faces vizinhas.
- **Metal 0, rugosidade 1** no carregamento: o glTF do Meshy chega com brilho de plástico, que não combina com pedra e telha pintadas.
- As bandeiras `flag_blue` dos portões e as tochas das ruas continuam as do jogo.
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

## O que ficou em aberto

1. **Prédios mais baixos**: igreja e casas ficaram abaixo das KayKit por causa do colisor circular. Alargar o colisor mexe no servidor (fora do escopo).
2. **Pontas da muralha**: as pontas de cada trecho têm postes de canto que aparecem nos vértices de 18°. Um trecho de canto próprio (+24 créditos) resolveria.
3. **Bandeiras e tochas** continuam as do jogo; um poste com chama e uma bandeira novos são adereços a mais.
4. **Lightmap pré-calculado**: não feito (ver "Luz").
5. **Barril e caixote**: ficaram KayKit, esperando o `remaster-estaticos`.
6. **Regiões**: só 3 adereços (lápide, cristal, pilar); a identidade completa (Arredores, Cripta, casas e muros delas) fica para uma rodada própria.
7. **FPS em GPU real** (tunnel#64).

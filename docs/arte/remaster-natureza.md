# Remaster: natureza e texturas do chão (estilo pintado à mão)

Segue a v3 (`docs/arte/remaster-v3.md`): peças estáticas pelo **Meshy** (conceito 2D, imagem para 3D com Smart Topology), tudo atrás de `?remaster=1`. Sem o parâmetro nada muda (nem a linha de base do bench). Com ele:

- árvores, arbustos, mato, pedras e montanhas do mapa principal usam o kit novo, **nas mesmas posições geradas** (`world.json` não muda);
- o chão troca as quatro camadas pintadas em canvas por quatro texturas pintadas à mão, repetíveis;
- ganham lápides com ossos nos Campos dos Ossos e cristais no Vale das Pedras Vivas (só enfeite, sem colisor).

Como olhar: `/` e `/?remaster=1`, no mesmo ponto (`/tp 78 -24` Campos, `/tp 0 -236` Vale, `/tp 138 -100` borda com as montanhas).

## Kit de natureza

| Peça | Arquivo | Triângulos LOD0 / LOD1 | Substitui (KayKit) | Instâncias |
|---|---|---|---|---|
| Carvalho frondoso | `nature_tree_oak_b.glb` | 1.444 / 532 | `tree_1_a/b`, `tree_2_a/b`, `tree_3_a` | ~70 |
| Pinheiro | `nature_tree_pine_b.glb` | 1.437 / 595 | `tree_4_a/b` | ~24 |
| Árvore seca e retorcida | `nature_tree_dead_b.glb` | 1.260 / 700 | `tree_bare_*` (Campos, Ermos, Cripta) | ~170 |
| Árvore do Vale (casca prateada, copa azul) | `nature_tree_vale_b.glb` | 1.437 / 809 | folhosas no Vale (z < -150): `tree_1_b`, `tree_2_b`, `tree_3_a`, `tree_4_b` | variável |
| Arbusto com frutinhas | `nature_bush_leaf_b.glb` | 362 / 352 | `bush_*` | 160 |
| Mato com flores | `nature_grass_tuft_b.glb` | 6 | `grass_*` | ~1.590 |
| Pedras pequenas (seixos) | `nature_rock_s_b.glb` | 252 | `rock_2_a/c`, `rock_1_c` | ~780 |
| Pedra média | `nature_rock_m_b.glb` | 281 | `rock_1_a/e`, `rock_3_b/c` | ~280 |
| Rochedo | `nature_rock_l_b.glb` | 368 / 252 | `rock_3_a/e/g/j`, `rock_1_h/k`, `rock_2_f` | ~330 |
| Montanha / penhasco de borda | `nature_cliff_b.glb` | 2.996 | `mountain_*`, `hills_*` | 70 |
| Lápide com ossos | `nature_grave_b.glb` | 606 / 334 | (novo) 16 grupos nos Campos | ~45 |
| Cristais | `nature_crystal_b.glb` | 602 / 324 | (novo) 14 grupos no Vale | ~25 |

Tudo cabe no orçamento do guia (árvore até 1.500, pedra até 600, mato até 200). Cada peça é 1 material, 1 textura de 512 px em KTX2 (UASTC; o mato, 256 px com recorte). Cada peça pesa de 330 a 460 KB; o mato 56 KB. O `check:assets` passa. **Só o mato (3 cartões cruzados, 6 triângulos) usa recorte**; o resto é malha sólida.

O mato do Meshy com 200 triângulos veio em estilhaços cinzentos e foi **descartado** (os 15 créditos foram gastos): o conceito pintado virou 3 cartões cruzados com recorte (`build-nature-kit.mjs`, `makeTuft`), de graça.

Uma textura por peça, não um atlas: o jogo desenha cada tipo com thin instances e agora são 12 desenhos em vez de ~60 (ver "Draw calls"), então juntar texturas não compraria mais nada.

### Variações

Cada espécie é um modelo só. A variedade vem da **forma do item KayKit que ela substitui** (altura, largura e proporção diferentes: um carvalho alto e fino, um baixo e largo), da rotação aleatória e da escala não uniforme do gerador. Segunda variação de cada espécie (modelo próprio) **não foi feita**: ver "Em aberto".

## Integração

- `client/scripts/build-nature-kit.mjs`: põe cada peça do Meshy de pé (base em y = 0, centrada, altura 1), enxuga as pedras, arbustos e as duas árvores que passavam de 1.500, e grava `src/world/nature-map.json`: para cada item KayKit, a peça nova e a escala que leva ao tamanho do item antigo (`kit-sizes.json`). **A altura é igual à do item antigo**, então os colisores e o vento (`wind.ts`, força pela altura) valem sem ajuste. Cada eixo pode se afastar da escala uniforme só dentro de um limite por peça (a montanha estica mais, a árvore pouco).
- `client/src/world/nature-remaster.ts` (novo): carrega as peças, troca os itens do mapa e acrescenta as lápides e os cristais (semente fixa, longe dos colisores e das estradas). `environment.ts` ganhou só um bloco curto antes de agrupar os objetos (a frente da cidade mexe no mesmo arquivo, mas em outra função). O desvanecer das árvores e o vento seguem pelo nome da peça (`tree_*`, `bush_*`, `grass_*`).
- Terreno (`terrain-mesh.ts`): com `?remaster=1` as camadas vêm de `public/terrain/{grass,dirt,stone,gravel}.jpg` (512 px), com repetição maior (5,2 / 5,2 / 6,5 / 3,6 m) e um define `TERRAINSPLAT_PAINTED` que deixa a textura mandar mais no tom (a pintura da região só tinge). O mapa de mistura por região, o preset baixo (grama e pedra só) e os demais continuam iguais.
- Presets baixa, média e alta continuam valendo (vento e camadas do terreno vêm dos mesmos presets).
- Importação: `tools/art/remaster-natureza/import.sh` (usa `import-asset`, categoria `cenario`, manifesto atualizado). Teste novo: `client/tests/nature-map.test.ts` (todo item de natureza do mundo tem peça, altura preservada, arquivo e manifesto existem).

## Texturas do chão

`text-to-image` (nano-banana, 3 créditos cada), prompt "seamless tileable, top-down, hand-painted". O modelo **não** entrega costura limpa (`chao-bruto.jpg`), então `client/scripts/make-seamless.mjs` desloca meia imagem e mistura com a original por uma máscara que vale 1 no centro e 0 nas bordas. O script mede a costura (diferença entre as bordas opostas dividida pela diferença entre colunas vizinhas; 1,0 = sem costura):

| Textura | Costura antes | Depois |
|---|---|---|
| Grama | 2,69 | 0,58 |
| Terra | 2,52 | 0,65 |
| Pedra | 4,54 | 0,33 |
| Cascalho | 1,41 | 0,87 |

Prova visual (cada uma repetida 2 x 2): `texturas-chao.jpg`. Variações de cor por região: **não** há texturas extras; a cor da região vem da pintura de macro que já existia (misturada por cima), então Campos, Ermos, Vale e Cripta continuam distintos pelos pesos das camadas e pelo tom.

## Draw calls (`?bench=contadores`, Chromium com SwiftShader, qualidade baixa, de dia)

Draws totais, draws só do cenário e malhas ativas do cenário, sem → com `?remaster=1`:

| Região | Draws | Cenário: draws | Cenário: ativas |
|---|---|---|---|
| Cidade | 202 → 144 | 114 → 80 | 62 → 44 |
| Arredores | 149 → 99 | 102 → 66 | 50 → 29 |
| Campos dos Ossos | 135 → 85 | 95 → 59 | 43 → 22 |
| Ermos Partidos | 133 → 86 | 93 → 58 | 41 → 21 |
| Vale das Pedras Vivas | 155 → 97 | 117 → 71 | 65 → 34 |
| Fundo do Vale | 150 → 92 | 113 → 67 | 61 → 30 |
| Vigília Ossária | 126 → 83 | 89 → 58 | 37 → 21 |
| Confins Partidos | 120 → 77 | 83 → 52 | 31 → 15 |
| Saqueador Ossudo | 161 → 104 | 117 → 72 | 65 → 35 |
| Cripta Esquecida | 128 → 81 | 91 → 56 | 39 → 19 |

De noite o padrão é o mesmo (Cidade 208 → 150, Campos 187 → 124). **Nenhum aumento**: cai de 40 a 60 draws por quadro, porque os ~60 tipos do KayKit viram 12 peças (os itens KayKit de uma mesma família passam a dividir a peça, com a escala própria de cada um por instância). O total de malhas sobe (+12: as peças novas entram na cena e as do KayKit ficam desligadas). A medição foi feita com as peças antes de enxugar triângulos; os contadores não dependem disso.

**O que sobe é triângulo.** Soma do mapa principal (thin instances desenham todas as instâncias de um tipo, sem corte por instância), `node client/scripts/count-nature-tris.mjs`: **349 mil → 1,01 milhão** (árvores 118 mil → 335 mil, pedras 120 mil → 397 mil, montanhas 26 mil → 210 mil, arbustos 14 mil → 58 mil; o mato **cai** de 70 mil para 9,5 mil). Foi o que coube depois de enxugar as pedras (Meshy 300/500/600 para 252/281/368) e os arbustos. É o custo a observar numa GPU de verdade (tunnel#64): o SwiftShader só confere contadores. Se pesar, o primeiro corte é o preset baixo desligar as pedras pequenas e parte do mato e ligar os LODs por região.

## Créditos (Meshy)

| Etapa | Chamadas | Créditos |
|---|---|---|
| Conceito 2D das 12 peças (nano-banana, 3 cada) | 12 | 36 |
| Imagem para 3D, Smart Topology, textura 2K (15 cada; o mato foi pago e descartado) | 12 | 180 |
| Conceito 2D das 4 texturas do chão (3 cada) | 4 | 12 |
| **Total** | 28 | **228 de 300** |

Saldo da conta: 1.918 no começo, 1.394 no fim (a diferença inclui os gastos de outras frentes). Nenhuma task paga foi repetida.

Prompts: `tools/art/remaster-natureza/prompts.mjs` (sufixo comum: pintado à mão, saturado, silhueta grossa e legível, peça isolada, vista de três quartos, fundo cinza). Parâmetros do imagem para 3D: `ai_model meshy-t2.5`, `model_type smart-topology`, `target_polycount` 1.500 (carvalho, pinheiro, árvore do Vale), 1.200 (seca), 400 (arbusto), 200 (mato), 300 / 500 / 600 (pedras), 3.000 (montanha), 600 (lápide, cristal), sem PBR.

## Prints (`docs/arte/remaster-natureza/`)

- `conceitos.jpg`: os 12 conceitos 2D. `pecas.jpg`: as peças 3D (a do mato não entra: foi trocada pelos cartões).
- `chao-bruto.jpg` e `texturas-chao.jpg`: as texturas como o Meshy as entregou e depois de repetíveis (2 x 2).
- `jogo-<região>-antes.jpg` e `jogo-<região>-depois.jpg`: sem e com `?remaster=1`, preset média, de dia (a hora do jogo nos prints vai de 7h a 16h): arredores, campos, lapides (`/tp 60 52`, onde caem as lápides do anel dos Campos), ermos, borda (montanhas), cripta, vale, fundo.

## Regerar

1. `node tools/art/remaster-natureza/prompts.mjs`, depois `node tools/art/meshy.mjs create text-to-image --est 3 --name <id> --json-file assets-src/meshy/prompts/<id>.json` e `create image-to-3d --est 15` (parâmetros acima).
2. `node client/scripts/build-nature-kit.mjs` (normaliza e escreve o mapa), `sh tools/art/remaster-natureza/import.sh`.
3. Chão: `node client/scripts/make-seamless.mjs <png> client/public/terrain/<camada>.jpg 512`.
4. Prints: `node tools/art/remaster-natureza/shots.mjs <sufixo> "<query>"`; bench: `tools/art/remaster-v3/bench.mjs`.

## Em aberto

1. **Triângulos**: 1,0 milhão contra 349 mil. Medir em GPU de verdade e, se preciso, reduzir as pedras pequenas no preset baixo.
2. **LOD por distância**: os arquivos têm LOD1 (menos a montanha, a pedra pequena e o mato), mas thin instances não trocam de LOD por instância e o jogo usa só o LOD0. Para usar os LODs seria preciso repartir as instâncias por célula (como o `TreeFader`).
3. **Segunda variação de cada árvore** (modelo próprio): não foi feita (15 créditos por peça, e a variedade pela forma dos itens KayKit foi suficiente por ora).
4. **Vento**: a altura é a do item antigo e o fator ficou o mesmo; a copa do carvalho (rígida, em ilhas) balança como corpo único. Visto só parado nos prints.
5. A árvore seca saiu lilás-acinzentada (não marrom): combina com os Ermos e a Cripta, mas nos Campos pode destoar.
6. Lápides e cristais não têm colisor nem existem no servidor.
7. O mato ficou pequeno e pálido de longe (cartões do conceito, 256 px); vale uma segunda tentativa pelo Meshy com alvo de 400 triângulos.

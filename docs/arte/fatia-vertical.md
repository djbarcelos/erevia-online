# Fatia vertical do remaster (tunnel#93, R0)

Prova da direção de arte do guia (`docs/guia-de-arte.md`, estilo B) numa cena pequena e completa: **um cavaleiro, um esqueleto guerreiro e a praça**. Tudo atrás de `?remaster=1`. Sem o parâmetro o jogo, os prints e a linha de base do bench (`e2e/bench-baseline.json`) ficam como estavam.

Como olhar: abra o jogo duas vezes, `/` e `/?remaster=1`, no mesmo ponto da praça.

## O que entrou

| Peça | LOD0 | LOD1 | LOD2 | Orçamento (guia, seção 5) | Ossos | Arquivo |
|---|---|---|---|---|---|---|
| Cavaleiro (corpo + elmo) | 4.752 | 2.260 | 868 | 8.000 / 3.000 / 1.000 | 21 (de ~45) | `client/public/models/knight_b.glb` |
| Esqueleto guerreiro (espada e escudo no corpo) | 2.500 | 1.304 | 400 | 4.000 / 1.500 / 500 | 19 | `skeleton_warrior_b.glb` |
| Fonte | 1.436 | 772 | 244 | 3.000 / 1.000 / impostor | – | `plaza_Fountain_b.glb` |
| Piso em rosa dos ventos | 1.208 | 632 | – | idem | – | `plaza_Paving_b.glb` |
| Poste com estandarte (x4) | 292 | 236 | 112 | idem | – | `plaza_BannerPost_b.glb` |

- Todos passam no `import-asset.mjs` **sem `--force`**. 1 material por peça; a textura é uma paleta 8x4 de 128x256 px (o guia pede até 1024²).
- Modelados do zero em código bpy no Blender 5.3, sem gerador nem biblioteca (`tools/art/fatia-vertical/`; os `.blend` ficam em `assets-src/`, fora do git).
- Cavaleiro: 1,80 m, cabeça de 0,26 m = **6,9 cabeças**, ombreiras largas, mãos e botas ~12% maiores, elmo de topo plano com crista, capa curta e tabardo (seção 2 do guia).
- Animações (30 fps, no próprio GLB): parado (2 s), andar, correr, golpe (impacto no quadro 14 = 0,47 s), dano, morte (cai de costas e fica) e braço erguido (buff, acenar, dançar). Esqueleto: o mesmo conjunto, `SW_*`.

### Como a aparência foi ligada (cavaleiro)

Não foi preciso mexer no sistema de aparência: o atlas do cavaleiro usa **o mesmo layout 8x4 do KayKit** (pele na célula [0,0], cabelo [0,1], roupa [1,0]), então `paintTexture` repinta pele, cabelo e roupa do jogador sem mudança. O gradiente vertical dentro de cada célula é a luz pintada (seção 4). O elmo é um objeto à parte chamado `Hat` (e `Hat_LOD1`, `Hat_LOD2`), então "esconder o chapéu" funciona e mostra a cabeça com rosto e cabelo por baixo. Só há uma cabeça (as 5 do KayKit não entram nesta fatia). Os encaixes `handslot.l/.r` têm a orientação calibrada para segurar as armas KayKit atuais (espada e escudo do equipamento aparecem nas mãos).

Código: `client/src/world/remaster.ts` (flag, arquivos, distâncias de LOD), adaptador em `models.ts` (clipes do próprio arquivo, LOD por distância da câmera a cada 250 ms, `Hat*`), `plaza-remaster.ts` (praça) e `environment.ts` (esconde o poço, a fonte ocupa o lugar e a mesma pegada de colisão de 1,7 m).

Uma mudança no `import-asset.mjs`: `join({ keepNamed: true })`. Sem ela, o `join` colava os nós `_LOD1/_LOD2` de peças estáticas dentro do LOD0 (a fonte contava 2.452 triângulos de LOD0 em vez de 1.436). Não muda nada para peças com esqueleto.

## Prints (`docs/arte/fatia-vertical/`)

- Blender: `v1-cavaleiro-angulos.jpg`, `cavaleiro-lods.jpg`, `cavaleiro-{parado,andar,correr,golpe,morte}.jpg`; `v1-esqueleto-angulos.jpg`, `esqueleto-lods.jpg`, `esqueleto-{andar,correr,golpe,morte}.jpg`; `praca-kit.jpg`.
- Repintura: `v1-jogo-heroi-recolor.jpg` (roupa 2, cabelo 3, pele 3, chapéu escondido): o tabardo mudou de cor pelo sistema de aparência de hoje. O chapéu escondido não ficou nítido no print (noite e câmera de costas): confirmar de perto.
- Jogo, antes e depois, no mesmo ponto: `jogo-praca-antes.jpg`, `v1-jogo-praca-depois.jpg`, `jogo-heroi-antes.jpg`, `v1-jogo-heroi-depois.jpg`, `jogo-esqueleto-antes/depois.jpg`.

## Números (`?bench=contadores`, Chromium com SwiftShader, qualidade baixa)

O WebGL é por software: FPS e ms não valem como desempenho de GPU, só os contadores (como no CI).

| Região, de dia | Draw calls antes → depois | Malhas ativas / total |
|---|---|---|
| Cidade | 205 → 222 | 113 / 314 → 124 / 347 |
| Arredores | 204 → 175 | 115 / 314 → 104 / 347 |
| Campos dos Ossos | 181 → 165 | 92 / 314 → 93 / 347 |
| Cripta Esquecida | 128 → 111 | 39 / 315 → 39 / 349 |

(As variações fora da cidade são ruído de quem estava no quadro, não de cenário.) Na Cidade o remaster custa **+17 draw calls (+8%)**: a fonte, o piso, os postes e as sombras deles, e os guardas que também usam o modelo do cavaleiro. Cada cópia de cavaleiro carrega 3 malhas de LOD dormentes (as de LOD1/2 ficam desligadas; contam no "total", não nos draws). O `?bench=1` completo (mesmo SwiftShader) deu na Cidade de dia 1,6 FPS (203 draws) sem o remaster e 1,5 FPS (221 draws) com ele, de noite 1,4 e 1,2: dentro do ruído de uma máquina sem GPU. **FPS real de GPU não foi medido**: o Daniel (ou o PC 2, tunnel#64) precisa rodar `?bench=1` e `?bench=1&remaster=1` num aparelho de verdade.

## O que o Daniel precisa olhar

1. **O estilo.** Os bonecos estão corretos nas proporções do guia, mas o cavaleiro lê "armadura de placas rígidas" e o rosto é quase só o elmo. É isto o estilo B que você quer? Mais volume, mais curva, mais detalhe pintado?
2. **O teste de 85 px em tons de cinza** do cavaleiro e do esqueleto no jogo (`v1-jogo-*-depois.jpg`).
3. **O piso da praça.** Ficou liso e limpo demais perto da calçada de pedras antiga. Decidir se o piso novo leva pedras pintadas ou se a praça inteira muda.
4. **A fonte no lugar do poço** (o poço sai só com `?remaster=1`).
5. **Armas e escudo do cavaleiro** ainda são KayKit (chibi), porque vêm do equipamento. O esqueleto já leva as dele.

## O que ficou feio ou faltando

- Rig rígido: cada peça é um osso só (armadura em placas) com esferas nas juntas; no cotovelo e na cintura, em poses extremas, aparecem frestas. Sem pesos suaves e sem Rigify (ver abaixo).
- O esqueleto de deformação é simples (21 ossos, nomes do KayKit: `hips`, `upperarm.l`, `handslot.r`...), não Rigify: o export limpo para glTF era o risco que o pedido citou, e a fatia não precisava dele.
- Capa é uma placa dura em 2 ossos (balança no parado e na corrida, não é cloth).
- Sem emissivo, sem raridade por forma, sem as outras 4 cabeças e sem as classes bárbaro, mago e ladino.
- Sem bake de normal/AO: a luz é pintada no atlas (por decisão do guia), então a fase de bake da skill não se aplica. O gradiente por altura é a única "luz" na textura.
- Em jogo o cavaleiro só usa Correr (o jogo tem um papel "run" só); o clipe Andar existe no arquivo e não é usado.
- Os postes não têm colisão; a fonte usa a colisão do poço.
- LOD do cavaleiro e do esqueleto troca por distância (20 m / 35 m); só vi o LOD0 em jogo no zoom padrão, não conferi a troca na câmera.
- Dois guardas da cidade (NPCs) usam o modelo do cavaleiro e também viram remaster.

## Recomendação

Seguir para o **R1**, mas antes de produzir em escala o Daniel decide o item 1 (estilo) e o item 3 (piso): são decisões de direção, e as proporções, o orçamento (4,7k tris, 128x256 px, 1 material), a repintura de cor e o pipeline Blender → GLB → jogo já foram provados. Se ele quiser mais volume e menos "bloco", ajustar o guia (seção 2: curvas nas ombreiras, capa, rosto) antes de modelar as outras classes.

## Como regerar

Blender com o servidor MCP ligado: `exec(open("tools/art/fatia-vertical/knight.py").read())` (idem `skeleton.py`, `plaza.py`); cada um modela, anima e exporta para `assets-src/fatia-vertical/`. Depois, em `client/`: `node scripts/import-asset.mjs ../assets-src/fatia-vertical/CH_Knight.glb personagem knight_b` (e `MN_SkeletonWarrior.glb monstro skeleton_warrior_b`, `PR_*.glb cenario plaza_*_b`).

## v2 (Meshy)

A v1 (tudo em bpy) ficou "bloco rígido, sem rosto". A v2 troca o cavaleiro e o esqueleto por **rascunhos do Meshy** (decisão 28 do tunnel na #93), depois limpos pelo pipeline do guia. A praça continua a da v1. Tudo atrás de `?remaster=1`.

### Gerações usadas: 2 de 4

| Modelo | Gerações | Prompt (inglês, A-pose) |
|---|---|---|
| Cavaleiro | 1 | stylized heroic fantasy knight, full plate armor with red tabard and short cape, open-face helmet, heroic proportions, hand-painted game style, A-pose, no weapon |
| Esqueleto guerreiro | 1 | stylized fantasy skeleton warrior, rusty half armor, hand-painted game style, A-pose, no weapon |

Os dois saíram aceitáveis na primeira (cavaleiro: 1,3 mi de faces, texturas 2048; esqueleto: 1,16 mi). Retomar um job pelo handle não gasta crédito.

### O que mudou

| Peça | LOD0 | LOD1 | LOD2 | Orçamento | Ossos | Textura |
|---|---|---|---|---|---|---|
| Cavaleiro (corpo + cabeça + `Hat`) | 7.337 | 2.848 | 923 | 8.000 / 3.000 / 1.000 | 21 | 1024 px, 1 material (PNG: ver abaixo) |
| Esqueleto guerreiro | 3.899 | 1.399 | 423 | 4.000 / 1.500 / 500 | 19 | 1024 px, 1 material (KTX2) |

Pipeline de cada um (scripts em `tools/art/fatia-vertical/`: `meshy_bake.py`, `meshy_rig.py`, `meshy_skeleton.py`, `meshy_render.py`; os `.blend` e os GLB brutos ficam em `assets-src/`, fora do git):

1. Escala real (cavaleiro 1,80 m, esqueleto 1,70 m), pés no chão (o Meshy já entrega assim).
2. Cavaleiro: a capa longa até o chão foi cortada para uma capa curta (esticaria nas pernas), as tiras vermelhas soltas ao lado dos braços foram apagadas. O elmo é separado pela altura (`Hat`).
3. Solda dos vértices, decimação (colapso) até o orçamento, normais conferidas contra a malha alta.
4. UV nova e **bake** da cor da malha alta para 1 textura de 1024 px (duas passadas: raios curtos e raios longos só nos pixels que a primeira errou). A luz do bake é clareada porque o Meshy entrega a cor sem os reflexos.
5. Esqueleto de 21/19 ossos com **os nomes do KayKit** (`hips`, `spine`, `chest`, `neck`, `head`, `upperarm.l`, `handslot.r`...). Os pesos automáticos do Blender falharam (malha com partes soltas), então os pesos vêm da distância às cápsulas dos ossos (`meshy_rig.py`), até 4 influências.
6. As animações são as do `biped_anim.py` (parado, andar, correr, golpe, dano, morte, braço erguido). Como o modelo nasce em A-pose, todos os quadros ganham um viés de 20 a 22 graus de adução nos ombros.
7. `handslot.l/.r` com a mesma orientação em mundo da v1, então as armas KayKit ficam como antes.
8. LODs feitos no Blender (decimate sem o modificador de armadura). No esqueleto o decimate trava em ~1.060 triângulos, então o LOD2 vem de um remesh por voxel (0,06) + decimate, com UV e pesos transferidos da malha de LOD0.

### Aparência (repintura) sem o atlas 8x4

O atlas 8x4 do KayKit não faz sentido numa textura gerada: `paintTexture` pintaria retângulos aleatórios por cima da armadura. No lugar, `paintRemasterTexture` (`body-look.ts`, usada só com `?remaster=1`):

- **Roupa:** tinge por máscara de cor tudo que é vermelho (a capa e o tabardo), mantendo a luz pintada do bake.
- **Pele:** a cabeça sob o elmo (uma elipsoide com 192 triângulos) está mapeada num quadradinho no canto da textura; trocar a pele repinta esse quadradinho.
- **Cabelo:** não existe nesses modelos (ignorado).
- Verificado no jogo (`jogo-v2-recolor-heroi.jpg`): roupa "Real" fica azul, a pele muda e **o chapéu escondido mostra a cabeça** (o elmo e os LODs dele somem).
- O esqueleto não tem repintura (não tem atlas); a cor do Bestiário não o afeta.

**Compromisso:** como a repintura lê os pixels da textura, o cavaleiro **não usa KTX2** (`--no-ktx2`): o `readPixels` de uma textura Basis comprimida volta preto (a primeira tentativa deixou o herói todo preto). Custo: 2,0 MB no arquivo do cavaleiro (o esqueleto, em KTX2, tem 0,9 MB). Alternativa se o tamanho incomodar: uma segunda textura-máscara pequena, ou o cavaleiro em KTX2 sem repintura.

### Bug da v1 achado e corrigido

Em `models.ts`, as expressões regulares do LOD e do `Hat` tinham perdido as barras invertidas (`/_LOD(d)$/` em vez de `/_LOD(\d)$/`, o clássico dos scripts de troca de texto). Resultado na v1: nenhuma malha era reconhecida como LOD1/LOD2, **os três LODs ficavam ligados ao mesmo tempo** e o `Hat_LOD1/2` nunca sumia. Isso explica os +17 draw calls da v1. Corrigido.

### Prints (`docs/arte/fatia-vertical/`)

- Blender: `v2-cavaleiro-angulos.jpg`, `v2-cavaleiro-poses.jpg` (correr, golpe, morte), `v2-esqueleto-angulos.jpg`, `v2-esqueleto-poses.jpg`.
- Jogo, v2 (com `?remaster=1`): `jogo-v2-heroi.jpg`, `jogo-v2-esqueleto.jpg`, `jogo-v2-recolor-heroi.jpg`, `jogo-v2-recolor-esqueleto.jpg`.
- Sem o parâmetro: `jogo-heroi-antes.jpg`, `jogo-esqueleto-antes.jpg`, `jogo-praca-antes.jpg`.
- v1 para comparar: `v1-cavaleiro-angulos.jpg`, `v1-esqueleto-angulos.jpg`, `v1-jogo-heroi-depois.jpg`, `v1-jogo-esqueleto-depois.jpg`, `v1-jogo-praca-depois.jpg`, `v1-jogo-heroi-recolor.jpg`.

### Números (`?bench=contadores`, SwiftShader, qualidade baixa, de dia)

Só os contadores valem (o WebGL é por software). Draw calls sem → com `?remaster=1` (v2, LOD corrigido):

| Região | Draws | Malhas ativas |
|---|---|---|
| Cidade | 194 → 189 | 107 → 107 |
| Arredores | 143 → 138 | 59 → 58 |
| Campos dos Ossos | 130 → 125 | 46 → 45 |
| Vigília Ossária | 121 → 117 | 37 → 37 |

(Total de malhas 303 → 334: os LODs dormentes dos modelos e a praça.) Com o LOD funcionando, o remaster **não custa draw calls** (-4 a -5 por quadro, por causa da fonte no lugar do poço); a v1 custava +17 na Cidade. O FPS de GPU real continua sem medida: rodar `?bench=1` e `?bench=1&remaster=1` num aparelho de verdade (PC 2, tunnel#64).

### O que ficou bom

- Silhueta e leitura bem melhores que a v1: o cavaleiro tem placas, dobras, cinto, capa e rosto sob o elmo (com a cabeça de pele ao esconder o chapéu); o esqueleto tem costelas, vértebras, ombreira e saiote de verdade.
- Orçamento cumprido sem exceção, 1 material, 1024 px, mesmos ossos e animações que a v1 (o jogo não mudou nada).
- Repintura de roupa e de pele funciona; as armas KayKit ficam nas mãos.

### O que ficou ruim

- O estilo é **mais sombrio e realista** que o "heroico estilizado" do guia: a armadura sai cinza-chumbo fosca, o elmo é fechado (pedi aberto), as mãos têm dedos finos que somem de longe. Pede retoque de cor e de valor na textura.
- O bake dá uma textura mosqueada (o Meshy já traz sujeira), principalmente no esqueleto; a UV do cavaleiro tem ~700 ilhas pequenas (a do esqueleto usa a projeção cilíndrica por região, bem melhor).
- Rig por distância: cotovelos e joelhos do cavaleiro deformam em poses extremas (placas esticam); a capa curta balança em 2 ossos, como na v1.
- Armas e escudo KayKit continuam enormes ao lado do cavaleiro novo (vêm do equipamento): o próximo passo é escala ou armas novas.
- O esqueleto não carrega armas na malha (vêm do equipamento do monstro).
- Cavaleiro sem KTX2 (ver acima); o LOD2 do esqueleto é um remesh grosseiro.
- Os guardas da cidade também usam o modelo do cavaleiro; não vi de perto.

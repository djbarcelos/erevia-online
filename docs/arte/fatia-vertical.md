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

- Blender: `cavaleiro-angulos.jpg`, `cavaleiro-lods.jpg`, `cavaleiro-{parado,andar,correr,golpe,morte}.jpg`; `esqueleto-angulos.jpg`, `esqueleto-lods.jpg`, `esqueleto-{andar,correr,golpe,morte}.jpg`; `praca-kit.jpg`.
- Repintura: `jogo-heroi-recolor.jpg` (roupa 2, cabelo 3, pele 3, chapéu escondido): o tabardo mudou de cor pelo sistema de aparência de hoje. O chapéu escondido não ficou nítido no print (noite e câmera de costas): confirmar de perto.
- Jogo, antes e depois, no mesmo ponto: `jogo-praca-antes/depois.jpg`, `jogo-heroi-antes/depois.jpg`, `jogo-esqueleto-antes/depois.jpg`.

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
2. **O teste de 85 px em tons de cinza** do cavaleiro e do esqueleto no jogo (`jogo-*-depois.jpg`).
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

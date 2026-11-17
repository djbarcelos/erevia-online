# Famílias de monstro não humanoides (tunnel#135)

Base para o Brejo (#139) e o Bosque Gris: quatro modelos novos no estilo B, com esqueleto e animações, que o servidor pode usar pelo nome (`Model:` em `monsters.go`). O Meshy só dá rig para humanoides, então a base de cada família é um modelo **CC0 do Quaternius já com rig e animações**, e o visual (pintado à mão, cores vivas) vem do **retexture do Meshy**.

Como olhar: `cd client && npm run dev`, abrir `/lab.html` e usar `window.lab.show({ cam: "front", items: [{ model: "wolf", role: "walk", t: 0.3, yaw: 0.9 }] })` (`tint` e `yaw` são novos no laboratório). Nos prints saem de `tools/art/familias-monstro/lab.mjs`.

## Modelos (use estes nomes de `Model:` no servidor)

| Família | Membro | `Model:` | Arquivo | LOD0 / LOD1 / LOD2 | Ossos | Peso | Altura de repouso |
|---|---|---|---|---|---|---|---|
| Quadrúpede | Lobo Cinzento | `wolf` | `wolf_b.glb` | 1.942 / 728 / 239 | 51 (48 animados por clipe) | 1,48 MB | 1,0 m |
| Artrópode | Aranha-de-Musgo | `spider` | `spider_b.glb` | 2.468 / 814 / 190 | 39 | 1,52 MB | 0,5 m (1,5 m de envergadura) |
| Amorfo | Lodo Vivo | `slime` | `slime_b.glb` | 1.440 / 540 / 180 | 12 | 0,97 MB | 0,8 m |
| Anfíbio | Sapo do Brejo | `frog` | `frog_b.glb` | 2.786 / 1.044 / 354 | 43 | 1,56 MB | 1,1 m (sapo ereto) |

Todos dentro do orçamento de monstro comum (4.000 / 1.500 / 500, 1 material, textura 1024). `check:assets` verde, os quatro no manifesto como `monstro`. A altura é a do corpo na pose de repouso antes da escala do servidor (`Scale` em `monsters.go`); ajuste a escala por monstro.

Nomes combinados com a #139: a branch `brejo` ainda não tinha monstro de bicho em `monsters.go` quando isto foi feito, então os nomes são os genéricos (`wolf`, `spider`, `slime`) e o sapo (`frog`) entra como o "sapo/lagarto do brejo": o lobo não serve para um brejo. **Não há `boar_b`**: o javali (porco do pacote) não tem presas na malha e viraria um porco; ver "Em aberto".

### Variações por paleta

O `Tint` do monstro (servidor) já multiplica a cor do material de qualquer modelo (`MonsterVariants`, força 0,75), e funciona igual na textura do remaster. Foi conferido no laboratório (`*-variantes.jpg`, com `tint` em `lab.show`). Sugestões, para o agente do Brejo:

| Monstro | Modelo | `Tint` | Efeito |
|---|---|---|---|
| Lodo do Brejo | `slime` | `#a8794f` | verde-oliva turvo, mais escuro |
| Lodo de pântano fundo | `slime` | `#6f8a4a` | verde musgo |
| Sapo do Brejo | `frog` | sem tint | verde com barriga mostarda |
| Sapo-boi / elite | `frog` | `#c28a4a` ou `#6f8fa8` | terroso ou azulado |
| Lobo do Bosque Gris | `wolf` | `#d8d8e0` | mais claro, pelagem pálida |
| Lobo sombrio | `wolf` | `#5a4a3a` | marrom escuro |
| Aranha de teia fria | `spider` | `#7a8fb0` | casco azulado |

O tint só escurece/desloca (multiplica), então não faz amarelo vivo sobre o verde: se o Brejo quiser uma cor bem diferente, é um retexture a mais (10 créditos por modelo).

## Animações (contrato de hoje)

Todas vêm do pacote, escolhidas por papel; as que faltavam foram derivadas no Blender (`prep.py`).

| Papel | Lobo (`WF_`) | Aranha (`SP_`) | Lodo (`SL_`) | Sapo (`FR_`) |
|---|---|---|---|---|
| parado | Idle | Spider_Idle | Slime_Idle | Idle (com braços do andar) |
| andar | Walk | Spider_Walk | Slime_Walk | Walk |
| correr | Gallop | cópia do andar (`SP_Run`) | cópia do andar (`SL_Run`) | Run |
| golpe e habilidade | Attack (1,33 s) | Spider_Attack | Slime_Attack | Punch |
| dano | Idle_HitReact_Left | **derivado**: achata e estica o osso raiz, 0,5 s | **derivado**, idem | HitReact (com braços do andar) |
| morte | Death | Spider_Death | Slime_Death | Death |
| pausas do parado | Idle_2 e Idle_2_HeadLow (`break1`, `break2`) | | | |
| rugido / esmagar | não há nos pacotes: `buff` cai no parado | idem | idem | idem |

Detalhes que importam:

- **Andar e correr não podem dividir o mesmo clipe**: o Babylon guarda um grupo por nome, e os dois papéis brigavam pelo peso (a aranha e o lodo ficavam parados). Por isso `SP_Run` e `SL_Run` são cópias do andar com nome próprio. Há um teste para isso (`tests/familias-monstro.test.ts`).
- **O dano derivado** parte da pose parada e mantém todos os ossos no clipe (sem isso os ossos fora dele travariam na mistura), só o osso raiz se achata no eixo vertical (0,78 e depois 1,1).
- **O sapo** tinha parado e dano sem canal nos braços, que ficavam abertos em T (a pose de repouso do rig). Os braços do andar (a 25% do clipe) foram fixados nesses dois clipes.
- **Impacto do golpe** (`CLIP_IMPACT`, medido com `impact.py`, o fim do movimento rápido do osso que bate): lobo 1,08 s, aranha 0,46 s, lodo 0,33 s, sapo 0,46 s. O do lobo é tardio porque o golpe do pacote tem uma preparação longa; o servidor decide o dano antes, e o cliente já começa o clipe adiantado (`lead`).
- **Canais podados** no `build.mjs`: os canais de ossos que não deformam (alvos de IK e polos) e os constantes em todos os clipes saem. Lobo: de 153 para 48 canais por clipe. Foi o que derrubou o custo de animação (ver "Medição").

### Passo natural (`FOOT_SPEEDS_REMASTER`, m/s do mundo com a altura de repouso acima)

Medido no Blender (`speeds.py`): o pé de apoio em relação ao corpo, na escala do jogo. O corpo não anda nos clipes (0,00 m, sem root motion).

| Modelo | andar | correr |
|---|---|---|
| `wolf` | 0,87 | 1,30 |
| `spider` | 0,66 | 0,66 (o mesmo clipe) |
| `slime` | 0,90 | 0,90 (estimado: o lodo não tem pé) |
| `frog` | 0,85 | 1,46 |

O galope do pacote é curto: a 4 m/s o lobo toca o clipe no limite de 3x (`MAX_STEP_RATIO`), então a passada é rápida, mas não escorrega mais que isso. Se parecer nervoso em jogo, a saída é baixar a velocidade do lobo no servidor.

## Fluxo, em ordem (tudo em `tools/art/familias-monstro/`)

1. **Download** (para `assets-src/quaternius/`, que o git ignora; nada executado de dentro): a pasta pública do pacote no Google Drive dava "Quota exceeded", então os quatro `.glb` vieram do Poly Pizza, onde o Quaternius publica os mesmos modelos em CC0 (a página de cada um diz CC0 1.0).
2. **`prep.py`** (Blender headless): importa, tira os ajudantes (esferas de osso), junta as malhas, deixa **um material**, **solda os vértices** e refaz a **UV** (Smart UV Project), escolhe e renomeia as ações com o prefixo da família, deriva o dano e a cópia do correr, e grava `<id>_uv.glb` (rig e clipes) e `<id>_static.glb` (malha de pé, 2 m, sem rig) para o Meshy.
3. **Retexture** (`prompts.mjs` + `meshy.mjs create retexture`, `enable_original_uv`, `meshy-6`, 10 créditos): a malha e a UV voltam iguais; só a textura é nova.
4. **`build.mjs`**: põe a textura (1024) no `<id>_uv.glb`, poda canais, grava `<id>_raw.glb`.
5. **`import.sh`**: `import-asset` com LOD (`--lod-prune`), KTX2 e manifesto. O sapo precisou de `--max-tris=2800` (o original tem 4.768 e o LOD2 passava de 500).
6. **Cliente**: `remaster.ts` (arquivo, altura, sob demanda) e `models.ts` (clipes, impacto, passo).

### Por que a UV precisou ser refeita (e custou 10 créditos de lição)

Os modelos do Quaternius **não têm UV**: a cor vem de um material por parte. O retexture precisa de UV. O Smart UV Project, rodado direto, deu uma ilha por face (as faces vêm soltas, sem vértices em comum), e o Meshy devolveu um mosaico de pedaços (`retexture-4278d69d`, perdido: 10 créditos). Soldar os vértices antes (`remove_doubles`) leva de ~1.100 ilhas a ~90 e a textura sai como pintura de verdade. A malha soldada ganha normais suaves com aresta viva acima de 40 graus, para manter o ar facetado.

## Medição: 20 lobos animados (VAT não foi preciso)

`tools/art/familias-monstro/bench.mjs` (lab.html, `window.lab.bench`): 20 instâncias com o `AnimController` de verdade correndo a 4 m/s, 12 s (os 2 primeiros aquecem), Chromium com SwiftShader (WebGL por software: o tempo de quadro e o FPS só valem na comparação; o de animação é CPU e vale mais). Tudo em LOD0, o pior caso.

| 20 instâncias | draw calls | animação (ms/quadro) | quadro CPU (ms) | triângulos |
|---|---|---|---|---|
| **Lobo** (51 ossos) antes da poda de canais | 20 | 6,7 | 8,7 | 32.770 |
| **Lobo** | **20** | **2,7** | 4,6 | 32.770 |
| Aranha | 20 | 2,2 | 3,8 | 41.090 |
| Lodo | 20 | 0,85 | 2,1 | 24.300 |
| Sapo | 20 | 1,9 | 3,8 | 47.010 |
| Esqueleto Lacaio (referência) | 40 | 2,5 | 4,4 | 82.170 |
| Esqueleto Guerreiro (referência) | 60 | 3,0 | 5,3 | 91.830 |

Um lobo sozinho: 0,18 ms de animação e 1 draw call. **Conclusão: sem VAT.** Vinte lobos custam 20 draw calls (um por lobo; sem arma, metade ou um terço dos esqueletos) e 2,7 ms de animação, no nível de vinte lacaios. Antes da poda eram 6,7 ms, então a poda de canais valeu mais que o VAT valeria. Reavaliar só se o `?bench=1` num celular mostrar `animationsTimeCounter` como gargalo com uma matilha de 20 ou mais.

## Créditos do Meshy

| Etapa | Chamadas | Créditos |
|---|---|---|
| Retexture do lobo (a 1ª, com a UV quebrada, foi perdida) | 2 | 20 |
| Retexture da aranha | 1 | 10 |
| Retexture do lodo (a 2ª, com prompt mais calmo, ficou) | 2 | 20 |
| Retexture do sapo | 1 | 10 |
| **Total** | 6 | **60** (teto da frente 120) |

O saldo da conta cai mais depressa que esta soma porque o agente do Brejo gasta da mesma conta; o `tasks.jsonl` desta worktree registra só as chamadas daqui.

## Prints (`docs/arte/familias-monstro/`)

- Blender (`render.py`, Workbench com textura): `blender-frente.jpg`, `blender-tres-quartos.jpg`, as quatro famílias com rig e textura antes do jogo.
- Laboratório (`lab.mjs`, o mesmo `CharacterModels` do jogo): `<modelo>-poses-three.jpg` e `-side.jpg` (parado, andar, correr, golpe, dano e morte, da esquerda para a direita **na ordem invertida**: a câmera de frente espelha o eixo x, então a morte fica à direita), `familias-front|three|back.jpg` e `<modelo>-variantes.jpg` (o primeiro sem tint, os outros com os tints sugeridos).

## Em aberto

1. **Javali (`boar_b`)**: o único porco CC0 com animação de ataque tem forma de porco, sem presas. Opções: retexturar com presas pintadas (não resolve a silhueta) ou gerar um javali com image-to-3d e transferir os pesos do porco no Blender.
2. **Lodo de cor muito diferente** (azul de gelo, roxo): o tint desloca pouco; vale um retexture por cor (10 créditos).
3. **Rugido e esmagar** não existem nos pacotes. Derivar com o mesmo recurso do dano (um clipe curto no osso raiz) é fácil; deixei o `buff` no parado para não inventar um rugido que o lobo não tem.
4. **Lobo**: o pelo ficou escuro e pouco "vivo" perto da paleta do guia; um retexture com um prompt mais colorido custa 10 créditos. A malha de 1.942 triângulos de origem é baixa para um bicho que vai ser visto de perto.
5. **Ossos**: lobo com 51 e sapo com 43 (o guia diz até ~45 que deformam). Os de IK saem dos canais, mas continuam na pele; remover do esqueleto exigiria refazer o rig.
6. **Não conferido em jogo com servidor**: nenhum monstro usa os modelos ainda (o servidor não os referencia); a prova é o laboratório, que usa o mesmo `CharacterModels` e o `AnimController`.
7. O pacote **License.txt** do Quaternius não pôde ser baixado (Drive com cota excedida); a licença foi conferida na página do pacote no site (CC0) e na de cada modelo no Poly Pizza (CC0 1.0).

# Remaster v3: cavaleiro e esqueleto guerreiro pelo Meshy (tunnel#93)

A v3 refaz os dois personagens da fatia vertical chamando a **API do Meshy direto** (sem o add-on do Blender), com o fluxo de qualidade: conceito 2D, imagem para 3D com topologia inteligente, rig e animação do próprio Meshy. Tudo continua atrás de `?remaster=1`. Histórico e o que deu errado na v1/v2: `docs/arte/fatia-vertical.md`.

Como olhar: abra o jogo duas vezes, `/` e `/?remaster=1`, no mesmo ponto da praça (`/tp 2 7`) e junto de um Esqueleto Guerreiro (Vigília Ossária).

## O que mudou em relação à v2

| | v2 | v3 |
|---|---|---|
| Geração | add-on, opções padrão: 1,3 milhão de faces | `image-to-3d` com Smart Topology (`meshy-t2.5`), `target_polycount` 7.000 (herói) e 3.800 (monstro) |
| Pontos de partida | só texto | conceito 2D (nano-banana-pro) de frente, A-pose, estilo pintado à mão, 2 variações por modelo |
| Malha | decimada a 7 mil: peças soltas, textura mosqueada, cinza fosco | malha nativa do Meshy já no orçamento; sem decimar nem refazer UV; a textura 2K do Meshy sai viva, sem retexture |
| Estilo | sombrio e realista, elmo fechado | heroico estilizado pintado, rosto à mostra sob o elmo aberto, tabardo vermelho |
| Rig | pesos por distância à cápsula (placas esticadas) | rig do Meshy, template `mixamo` (28 ossos), pesos do Meshy |
| Animação | `biped_anim.py` (bpy), quadril caído e agachado | ações da biblioteca do Meshy; ver "Animações escolhidas" |
| Armas | KayKit gigantes na mão | encaixe `handslot` calibrado, escala 0,4 (cavaleiro) e 0,36 (esqueleto) |
| Textura | cavaleiro em PNG 2,0 MB (sem KTX2, a repintura lia os pixels) | **KTX2 nos dois** (UASTC, 1024 px); repintura por máscara PNG de 512 px e shader |

## Fluxo, etapa por etapa

1. **Conceito 2D** (`text-to-image`, `nano-banana-pro`, 3:4, 9 créditos cada). 2 variações por modelo; prints em `conceito-cavaleiro.jpg` e `conceito-esqueleto.jpg` (a da esquerda foi a escolhida).
2. **Imagem para 3D** (`image-to-3d`, `meshy-t2.5`, `model_type: smart-topology`, `pose_mode: a-pose`, textura 2K, 15 créditos). A malha e a textura vieram boas de primeira; não foi preciso retexture (economia de 20 créditos).
3. **Rig** (`rigging`, 5 créditos): `skeleton_template: mixamo`, `height_meters` 1,8 e 1,7. O Meshy devolve de graça os clipes de andar e correr.
4. **Animação** (`animations` com `action_ids`, 3 créditos por ação).
5. **Montagem** (`tools/art/remaster-v3/build.sh`): junta rig + clipes num GLB, encaixes `handslot.r/.l` como ossos filhos das mãos.
6. **Importação** (`tools/art/remaster-v3/import.sh`): `import-asset` com poda de ilhas nos LODs e textura reduzida a 1024 px, KTX2, manifesto; máscara de repintura.
7. **Conferência**: Blender headless (`render.py`, `mesh-check.py`) e o jogo no navegador (`shot.mjs`, `anim-shots.mjs`, `bench.mjs`).

O CLI é `tools/art/meshy.mjs`: lê a chave `MESHY` do `.env` do checkout principal (ou de `--env`), nunca a imprime, grava cada task em `assets-src/meshy/tasks.jsonl` (id, tipo, parâmetros, créditos) e baixa os resultados. Recusa uma chamada que passe do teto da rodada (`MESHY_BUDGET`, 250). Reutilizável: `create <endpoint> --est <créditos> --json '{...}'`, `get <endpoint> <id> --download`, `balance`, `spent`.

### Prompts (conceito 2D, em inglês)

Sufixo comum (estilo do guia: pintado à mão, saturado, 6,5 cabeças, A-pose, sem arma):

> hand-painted stylized heroic fantasy game character concept, vibrant saturated colors, bright clean painterly look like a high quality fantasy MMORPG illustration, soft painted shading, clear readable silhouette, heroic stylized proportions about 6.5 heads tall (not realistic, not chibi), slightly enlarged hands, feet and shoulders, full body front view character sheet, strict A-pose with arms slightly away from the body and legs slightly apart, standing straight upright, relaxed straight knees, plain neutral light gray background, no text, no weapon in hands, no shield

Assuntos (o sufixo vai depois):

- **Cavaleiro 1 (escolhido):** Heroic knight in polished steel plate armor with large rounded pauldrons, OPEN-FACE helmet showing a young confident human face with brown hair, flat-topped helmet with small crest, saturated crimson red tabard with gold trim, short red cape, brown leather belt and boots, gold accents.
- **Cavaleiro 2:** Heroic knight with a friendly determined face fully visible, open visor helmet with a red plume, bright silver plate armor with gold edges, vivid red surcoat with a simple cross emblem, short crimson cape, large rounded shoulder guards, leather gauntlets and boots.
- **Esqueleto 1 (escolhido):** Skeleton warrior undead, bright bone-white skull and bones with warm ivory tones, wearing rusty orange-brown half plate armor on the chest, one shoulder and the hips, tattered loincloth, glowing faint amber eyes, bony fingers and feet slightly enlarged, menacing but stylized and slightly charming.
- **Esqueleto 2:** Skeleton warrior, clean pale ivory bones with visible ribs and spine, rusted iron breastplate and pauldron with orange rust, small tattered dark red cloth belt, empty eye sockets with a faint warm glow, stylized exaggerated shoulders, no helmet.

Critério de escolha (guia, seções 2 e 3): pose simétrica e ereta (rig e A-pose), silhueta legível, elmo de topo plano com crista (o guia), cores saturadas sem plumas soltas nem sujeira. A variação 2 do cavaleiro tem o penacho flutuando e a pose de três quartos; a do esqueleto tem botas enormes e a pose assimétrica.

## Créditos gastos

| Etapa | Chamadas | Créditos |
|---|---|---|
| Conceito 2D (2 cavaleiros + 2 esqueletos, nano-banana-pro) | 4 | 36 |
| Imagem para 3D, Smart Topology + textura 2K | 2 | 30 |
| Rig (mixamo) | 2 | 10 |
| Animação, 1ª rodada (parado, golpe de espada, dano, morte; 4 ações x 2) | 2 | 24 |
| Animação, 2ª rodada (trocas de dano e grito de espada; cavaleiro testou 4 ações, esqueleto 2) | 2 | 18 |
| **Total** | 12 | **118** de 250 |

Saldo da conta no começo: 1.040. No fim: 922 (118 gastos, confere com o `tasks.jsonl`). Nenhuma task paga foi repetida: o que serviu foi retomado pelo id.

## Animações escolhidas (e por quê)

A prioridade desta rodada era a animação: o cavaleiro da v2 parecia agachado e de quadril caído. Cada clipe foi conferido quadro a quadro no Blender (`render.py clips`, tiras 7 quadros em `*-locomocao.jpg` e `*-combate.jpg`) e por números (`render.py stats`: altura do quadril e pé mais baixo por clipe).

| Papel no jogo | Clipe do Meshy (id) | Por quê |
|---|---|---|
| parado (`idle`) | Idle (0) | quadril a 0,997 m no cavaleiro de 1,8 m (v2: ~0,85), coluna ereta, joelhos retos, pés no chão; o primeiro e o último quadro são iguais (loop limpo) |
| andar (`walk`) | Walking (grátis, vem no rig) | passada natural, pés a 0,05 m do chão, quadril sobe e desce 7 cm; tocado a ~1,9 m/s |
| correr (`run`) | Running (grátis, vem no rig) | tronco levemente inclinado, braços soltos, ciclo de 0,5 s, tocado a 5 m/s |
| golpe (`attack`, `skill`) | Right-hand Sword Slash (219) | golpe de espada com antecipação e impacto claros; o quadril desce a 0,78 m na estocada (é uma estocada, volta à postura) |
| dano (`hit`) | Hit Reaction 1 (179) | recuo do tronco e das mãos; a Hit Reaction (178), testada primeiro, só levava a mão ao capacete |
| morte (`death`) | Dead (8) | cai de costas e fica no chão (quadril de 1,0 a 0,24 m) |
| buff/acenar/dançar (`buff`, `wave`, `dance`) | Sword Shout (101) | braço erguido; o jogo não tem clipe próprio de emote |

Descartadas depois de ver: Attack (4, um salto de 2 m com o corpo no ar), Left Slash (97, golpe de lado sem o recuo do golpe de espada) e Hit Reaction (178). Custaram 12 créditos de teste no cavaleiro.

Detalhe técnico: o `merged_animations.glb` do Meshy vem com o `Armature` girado 90 graus em X (Z para cima) e o quadril compensando, enquanto o rig e os clipes de andar/correr vêm em Y para cima. `build-remaster-v3.mjs` compõe a rotação do `Armature` no quadril (rotação e translação) para tudo ficar no mesmo espaço. Sem isso o quadril andava a -0,07 m (chão) nos clipes de andar e correr.

### Em jogo: andar e correr

A v2 só usava o correr. Agora o `AnimController` ganhou o papel opcional `walk`: abaixo de 3,4 m/s (monstros, lentidão) o jogo mistura para o clipe de andar, com a velocidade do clipe ajustada à do personagem; acima, correr. Os heróis a 5 m/s correm; o esqueleto, mais lento, anda. Os outros modelos (KayKit, golem) não têm `walk` e continuam como antes. Frames em jogo: `jogo-v3-correndo.jpg`.

## Malha, LOD e orçamento

| Peça | LOD0 | LOD1 | LOD2 | Orçamento (guia, seção 5) | Ossos | Textura | Arquivo |
|---|---|---|---|---|---|---|---|
| Cavaleiro | 7.136 | 2.676 | 890 | 8.000 / 3.000 / 1.000 | 30 (28 + 2 encaixes) | 1024 px KTX2 | `knight_b.glb` 2,0 MB + `knight_b.mask.png` 65 KB |
| Esqueleto guerreiro | 3.949 | 1.475 | 412 | 4.000 / 1.500 / 500 | 30 | 1024 px KTX2 | `skeleton_warrior_b.glb` 1,9 MB |

Sem exceção no manifesto. Conferências da malha no Blender (`mesh-check.py`): sem vértices soltos nem lixo; o cavaleiro tem 18 ilhas (a maior, de 4.650 triângulos, é o corpo; o resto são peças de armadura, cinto e capa que o Smart Topology separa de propósito) e o esqueleto 16 (os ossos). 64 e 79 arestas abertas (as bordas das peças), todas fechadas por baixo de outra peça. A silhueta e os ângulos estão em `cavaleiro-angulos.jpg` e `esqueleto-angulos.jpg`.

Mudanças no pipeline de importação (`client/scripts/`):

- `import-asset.mjs --resize=1024`: reduz a textura 2K do Meshy por média de blocos antes do orçamento e do KTX2 (`resizeTextures`, com teste).
- `import-asset.mjs --lod-prune`: o LOD por meshopt travava em ~1.100 triângulos nas malhas do Meshy (muitas peças pequenas); com `Prune` descarta as ilhas pequenas nos LODs e chega a 890 e 412.
- `build-remaster-v3.mjs`, `make-remaster-mask.mjs`, `montage.mjs`: montagem do GLB, máscara de repintura e folhas de prova.

## Integração (adaptador, `client/src/world/models.ts`)

- **Clipes:** `REMASTER_SETS` com os nomes novos (`KB_Idle/Walk/Run/Attack/Hit/Death/Raise`, `SW_*`) e o papel `walk` (`OptionalRole`). O impacto do golpe é o quadro 16 de 30, medido na velocidade da mão no Blender (a mão acelera até o quadro 13 e termina o movimento rápido no 16).
- **Osso da mão e armas:** os GLB trazem `handslot.r/.l` como ossos filhos de `mixamorig:RightHand/LeftHand`, com a rotação calibrada no Blender (a lâmina aponta para a frente da mão; o escudo mostra a face para fora), deslocamento de 7 cm da palma e **escala 0,4** (esqueleto 0,36). As armas KayKit são para um boneco chibi de 2,5 cabeças; a 1,0 elas ficavam com 1,8 m. O esqueleto passou a usar o machado e o escudo do jogo (`MONSTER_WEAPONS`) em vez de armas no corpo.
- **Elmo:** o Meshy entrega o elmo e a cabeça numa malha só (uma textura, uma peça). **"Esconder chapéu" não revela a cabeça no cavaleiro v3**: não há `Hat` separado. Separar pela altura cortaria o rosto, como a v2 fazia (o corte funcionava porque a malha da v2 era dividida à mão). Fica em aberto, ver abaixo.
- **Repintura com KTX2:** a textura-base é KTX2 e a CPU não lê os pixels. Uma máscara de 512 px (`knight_b.mask.png`, R = pano vermelho, G = pele do rosto, A = brilho médio da pele) é misturada com a base **na GPU** por um shader (`ProceduralTexture`) que desenha uma textura nova por combinação de cores, uma vez. A luz pintada da base é mantida (o brilho do pixel escala a cor nova), como a v2. O código antigo `paintRemasterTexture` saiu. Verificado no jogo: roupa "Real" fica azul (`jogo-v3-recolor.jpg`), o aço continua igual.
- A repintura do esqueleto continua inexistente (o Bestiário não o tinge).

## Comparação honesta, v2 e v3

| | v2 | v3 |
|---|---|---|
| Aparência do modelo | silhueta correta, mas cinza fosco, textura mosqueada, elmo fechado | cores vivas, rosto à mostra, acabamento pintado; o rosto lê nos prints do Blender e de perto; na câmera normal do jogo o elmo domina |
| Peças soltas/artefatos | sim (decimação 1,3 mi para 7 mil) | não: a malha já nasce no orçamento |
| Rig | pesos por distância, placas esticadas | pesos do Meshy; as placas dobram nas juntas sem esticar nos clipes que vi |
| Postura parada | agachado, quadril caído | ereta, quadril a 1,0 m |
| Andar/correr | só correr, a velocidade ajustada | andar e correr, com troca por velocidade |
| Golpe, dano, morte | clipes feitos à mão no `biped_anim.py` | ações do Meshy; o golpe é uma estocada com o corpo baixo, mais "esgrima" que o corte do KayKit; o dano é um recuo discreto |
| Armas | gigantes | 0,4 do tamanho original e na mão |
| KTX2 no cavaleiro | não (PNG 2,0 MB) | sim, com máscara (2,0 MB também: o UASTC de 1024 px pesa o mesmo) |
| Elmo "esconder chapéu" | funcionava | **não funciona no cavaleiro v3** |
| Contas | add-on, 2 gerações (~30 créditos) | 118 créditos, 12 chamadas |

Onde a v2 ainda ganha: o "esconder chapéu" (barbeiro) e a repintura da pele (a v2 trocava um quadradinho; a v3 depende da máscara, que pega o rosto mas não as mãos de couro). Onde a v3 é claramente melhor: a leitura do personagem, o orçamento sem malabarismo, a postura e as animações.

## Números (`?bench=contadores`, Chromium com SwiftShader, qualidade baixa)

O WebGL é por software: FPS e milissegundos não valem como desempenho de GPU, só os contadores (como no CI). Draw calls e malhas ativas, sem → com `?remaster=1` (v3), de dia:

| Região | Draws | Malhas ativas / total |
|---|---|---|
| Cidade | 195 → 181 | 108 / 304 → 104 / 309 |
| Arredores | 202 → 187 | 118 / 304 → 113 / 309 |
| Campos dos Ossos | 169 → 164 | 85 / 304 → 89 / 309 |
| Ermos Partidos | 153 → 145 | 69 / 304 → 70 / 309 |
| Vale das Pedras Vivas | 150 → 141 | 66 / 304 → 66 / 309 |
| Vigília Ossária | 124 → 115 | 40 / 304 → 40 / 309 |
| Cripta Esquecida | 123 → 114 | 39 / 305 → 39 / 309 |

De noite o padrão é o mesmo (Cidade 201 → 187, Cripta 132 → 123). O remaster **não custa draw calls**: -5 a -15 por quadro (a fonte no lugar do poço e os LODs funcionando; a v2 deu -4 a -5). O total de malhas sobe 5 (os LODs dormentes dos modelos e a praça, como na v2). Cada cavaleiro e cada esqueleto é 1 material e 1 malha por LOD.

FPS do software (só como ordem de grandeza, 2 a 5 FPS dos dois lados): a primeira região depois da carga (Cidade, de dia) deu 1,1 FPS com o remaster contra 2,6, um ponto fora da curva (a mesma Cidade de noite deu 2,9 contra 2,0), que atribuo à transcodificação do KTX2 e à compilação do shader da repintura na primeira vista; as outras 19 regiões ficam dentro do ruído. **FPS de GPU real não foi medido**; o JSON completo (`?bench=1`) é o mesmo da v2 e precisa de um aparelho de verdade.

## Prints (`docs/arte/remaster-v3/`)

- Conceitos: `conceito-cavaleiro.jpg`, `conceito-esqueleto.jpg`.
- Blender: `cavaleiro-angulos.jpg`, `cavaleiro-locomocao.jpg` (parado, andar, correr), `cavaleiro-combate.jpg` (golpe, dano, morte, braço erguido); idem `esqueleto-*`.
- Jogo com `?remaster=1`, no mesmo ponto dos prints da v2 (`docs/arte/fatia-vertical/`): `jogo-v3-praca.jpg`, `jogo-v3-esqueleto.jpg`, `jogo-v3-recolor.jpg`, `jogo-v3-correndo.jpg`. Para comparar: `jogo-v2-heroi.jpg`, `jogo-v2-esqueleto.jpg`.

O Blender estava aberto com o servidor MCP quando o trabalho começou, mas uma chamada minha (`read_factory_settings`) derrubou o servidor e o resto das conferências rodou com o Blender 5.3 em modo headless (os scripts `tools/art/remaster-v3/render.py` e `mesh-check.py`); as folhas de quadros equivalem ao `look` mode=frames.

## O que ficou em aberto

1. **Esconder chapéu** no cavaleiro v3 (ver acima). Opções: pedir ao Meshy uma versão sem elmo e outra com (2 gerações, 30 créditos), ou dividir a malha no Blender com cuidado.
2. **Peso dos arquivos:** 2,0 e 1,9 MB (UASTC). ETC1S cortaria a ~0,4 MB com perda visível nos detalhes pintados; testar antes de adotar.
3. **Golpe e dano:** a biblioteca do Meshy não tem um golpe de espada em arco; o 219 é uma estocada. Se o Daniel quiser um corte mais largo, trocar a ação (3 créditos cada).
4. **Pele nas mãos** (repintura) e as outras 4 cabeças do KayKit não existem nesses modelos.
5. **FPS de GPU real** continua sem medida (SwiftShader); rodar `?bench=1` e `?bench=1&remaster=1` num aparelho de verdade (tunnel#64).
6. **Prints de golpe e morte em jogo** não foram feitos (só no Blender): precisa de monstro à mão e ritmo de captura.

## Estimativa de créditos por personagem (o resto do roteiro)

| Item | Créditos |
|---|---|
| Conceito 2D (3 variações a 9) | 27 |
| Imagem para 3D, textura 2K | 15 |
| Rig | 5 |
| Animação: 5 ações novas (parado, golpe, dano, morte, grito) | 15 |
| **Personagem pronto, caminho feliz** | **~62** |
| Com 1 refação de modelo e 1 troca de ação | ~85 |

Com a v3 como medida (118 créditos por 2 personagens, 59 cada, incluindo testes de ações), as 3 classes que faltam (bárbaro, mago, ladino) custam cerca de 190 a 255 créditos. Monstros que reaproveitam o rig (mesmo esqueleto Mixamo) só pagam as ações que não existem: os clipes de andar e correr do rig são grátis. Para um modelo que não é humanoide (quadrúpede, chefe), o rig muda e é preciso conferir antes.

## Como regerar

1. `node tools/art/remaster-v3/prompts.mjs`, depois `node tools/art/meshy.mjs create text-to-image --est 9 --json-file assets-src/meshy/prompts/k1.json` (idem `s1`, ...). Imagem para 3D, rig e animação: ver os parâmetros em `assets-src/meshy/tasks.jsonl`.
2. `sh tools/art/remaster-v3/build.sh` (GLB cru em `assets-src/meshy/`), `sh tools/art/remaster-v3/import.sh` (arquivos do jogo, manifesto e máscara).
3. `blender --background --factory-startup --python tools/art/remaster-v3/render.py -- <glb> <pasta> <prefixo> angles|clips|stats`.
4. Jogo: `PW_E2E=<pasta e2e> node tools/art/remaster-v3/shot.mjs <nome> "remaster=1" 6`.

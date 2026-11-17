# Remaster: o bestiário no estilo novo (tunnel#98)

Esqueletos comuns (lacaio, ladino, mago), Golem de Pedra e os dois chefes (Senhor dos Ossos e Ozrhal, o raid boss) refeitos pelo Meshy com o fluxo da v3 (`docs/arte/remaster-v3.md`): conceito 2D, imagem para 3D com Smart Topology, rig `mixamo` e ações da biblioteca do Meshy. Tudo atrás de `?remaster=1`; sem o parâmetro, nada muda (o golem continua o de código, os chefes continuam o esqueleto guerreiro e o mago do KayKit). O esqueleto guerreiro já era da v3.

Como olhar: `/?remaster=1`, `/tp 95 0` (Campos dos Ossos: guerreiro, ladino), `/tp 0 -50` (lacaios), `/tp 128 0` (mago), `/tp 0 -125` (Senhor dos Ossos), `/tp 0 -262` (golens do Vale), `/tp 70 70` (covil de Ozrhal; é nível 35, chegue com cuidado).

## Conceitos (um por modelo, todos aprovados de primeira)

`conceitos.jpg`: lacaio, ladino, mago / golem, Senhor dos Ossos, Ozrhal. Silhuetas escolhidas para ler de longe:

| Modelo | Silhueta | Cor |
|---|---|---|
| Lacaio | baixo, crânio enorme, caixa torácica minúscula, só um trapo no quadril, sem armadura | osso amarelado, olhos verdes |
| Ladino | esguio, capuz e cachecol compridos, bainhas vazias no cinto | verde-azulado escuro, olhos ciano |
| Mago | alto, chapéu pontudo torto, túnica aberta até o joelho, amuleto no peito | roxo e dourado, olhos violeta |
| Golem | quadrado e largo, ombros e punhos enormes, cabeça afundada, núcleo no peito | pedra cinza-parda, musgo, rachaduras laranja |
| Senhor dos Ossos | guerreiro de ombros largos, elmo de crânio com dois chifres longos, capa carmesim, ombreiras de espinhos | ferro escuro e vermelho |
| Ozrhal | lich alto de coroa dourada pontuda, ombreiras de espinhos de ouro, túnica longa até o chão, cristal violeta no peito | roxo, verde-azulado e ouro |

Prompts em `tools/art/remaster-monstros/prompts.mjs` (mesmo sufixo da v3, com a proporção de cabeças por modelo).

## Créditos

| Etapa | Chamadas | Créditos |
|---|---|---|
| Conceito 2D (nano-banana-pro, 1 por modelo) | 6 | 54 |
| Imagem para 3D, Smart Topology, textura 2K (3.800 faces; chefes 9.000) | 6 | 90 |
| Rig `mixamo` (andar e correr de graça) | 6 | 30 |
| Animação: lacaio (1 ação) | 1 | 3 |
| Animação: ladino (2 ações) | 1 | 6 |
| Animação: mago (2 ações) | 1 | 6 |
| Animação: golem (4 ações) | 1 | 12 |
| Animação: Senhor dos Ossos (2 + 5 ações) | 2 | 21 |
| Animação: Ozrhal (7 ações) | 1 | 21 |
| **Total desta rodada** | 25 | **243** (teto 450) |

Somado ao que a v3 já gastou (118), o `tasks.jsonl` desta worktree está em 361. Nenhuma task paga foi repetida.

## Ações escolhidas por papel

Todos os rigs usam o template `mixamo` com os mesmos nomes de osso. Por isso as ações já pagas do esqueleto guerreiro da v3 (parado, dano, morte, grito) são **religadas pelo nome do osso** nos monstros comuns, com a altura do quadril ajustada pela razão entre os andares dos dois rigs (`hipscale`, calculado no montador, 0,57 no lacaio e 0,99 no golem). Isso poupou ~12 créditos por monstro e foi conferido quadro a quadro (altura do quadril e do pé no Blender). Os chefes, que merecem timing próprio, pagaram ações no próprio rig.

| Papel | Lacaio | Ladino | Mago | Golem | Senhor dos Ossos | Ozrhal |
|---|---|---|---|---|---|---|
| parado | Idle (v3) | Idle (v3) | Idle (v3) | Idle (v3) | Idle (0) | Idle (0) |
| andar | Walking (rig) | Walking (rig) | Walking (rig) | Slow Orc Walk (119) | Walking (rig) | Walking (rig) |
| correr | Running (rig) | Running (rig) | Running (rig) | Running (rig) | Running (rig) | Running (rig) |
| golpe (`attack`) | Charged Axe Chop (237), recortado de 7,7 s para 1,4 s | Double Combo Attack (92), 2 estocadas, 1,3 s | mage spell cast (129), 1,8 s | Heavy Hammer Swing (128) | Sword Judgment (102), salto e golpe, 1,9 s | mage spell cast (129) |
| habilidade (`skill`) | = golpe | Thrust Slash (240) | Charged Spell Cast (125) | Charged Ground Slam (127), o "esmagar" | Charged Ground Slam (127), o anel de ossos | Angry Ground Stomp (255) |
| rugido/buff (`buff`) | parado | parado | Charged Spell Cast | Chest Pound Taunt (88), o "rugido", 2,2 s | Sword Shout (101) | Zombie Scream (386) |
| dano | Hit Reaction 1 (179) | idem | idem | idem | idem (próprio) | idem (próprio) |
| morte | Dead (8) | idem | idem | idem | idem (próprio) | idem (próprio) |

O contrato de animação do golem de hoje (parado, andar, golpe, esmagar, rugido, dano, morte) está inteiro: `GL_Idle/Walk/Run/Attack/Slam/Roar/Hit/Death`. O impacto de cada golpe (`CLIP_IMPACT`) foi medido na velocidade das mãos no Blender (`tools/art/remaster-monstros/speeds.py`): lacaio 0,6 s, ladino 0,53 s, mago 0,47 s (feitiço 1,3 s), golem 1,23 s (golpe) e 1,3 s (esmagar, bate com o `Delay` de 1,2 s do servidor), Senhor dos Ossos 0,8 s (anel 1,3 s), Ozrhal 0,47 s.

Fases do Ozrhal: o servidor decide as fases (`zone/raid.go`); o cliente toca o que chega. Para as habilidades de área do raid ele tem `skill` (pisão) e `buff` (grito).

## Malha, LOD e orçamento

| Peça | LOD0 | LOD1 | LOD2 | Orçamento | Ossos | Textura | Arquivo |
|---|---|---|---|---|---|---|---|
| Lacaio | 3.969 | 1.487 | 967 | monstro 4.000 / 1.500 / 500 | 30 | 1024 KTX2 | `skeleton_minion_b.glb` 1,9 MB |
| Ladino | 3.969 | 1.156 | 824 | idem | 30 | 1024 KTX2 | `skeleton_rogue_b.glb` 2,0 MB |
| Mago | 3.969 | 1.267 | 499 | idem | 30 | 1024 KTX2 | `skeleton_mage_b.glb` 2,0 MB |
| Golem | 3.970 | 1.488 | 608 | idem | 30 | 1024 KTX2 | `stone_golem_b.glb` 1,9 MB |
| Senhor dos Ossos | 9.494 | 3.154 | 1.104 | chefe 15.000 / 5.000 / 2.000 | 30 | 2048 KTX2 | `bone_lord_b.glb` 5,8 MB |
| Ozrhal | 9.553 | 3.157 | 1.319 | chefe idem | 30 | 2048 KTX2 | `ozrhal_b.glb` 6,0 MB |

Exceções registradas no manifesto (`assets-manifest.json`): o LOD2 do lacaio, do ladino e do golem passa de 500 (967, 824 e 608). A malha do Meshy é feita de cascas soltas e o simplify do meshoptimizer trava perto desse piso; o LOD2 só vale além de 35 m. O Meshy entrega 3% a mais de triângulos que o pedido, então o import ganhou `--max-tris` (aperta o LOD0 até o limite, 4.031 para 3.969).

Mudanças no pipeline (`client/scripts/`):

- `import-asset.mjs --max-tris=<n>` + `capTriangles` em `lib/asset-optimize.mjs` (com teste).
- LOD com `Prune`: no golem o LOD2 saía **vazio** (o erro 1 poda toda ilha). Agora, se o resultado foge de 70% a 105% do alvo, o erro é bisseccionado até a malha chegar perto. As malhas da v3 não mudam (já caíam na faixa).
- `build-remaster-monstros.mjs` + `tools/art/remaster-monstros/models.mjs`: montagem guiada por configuração, com ações de qualquer arquivo (religadas pelo nome do osso), recorte (`from`/`to`) e `hipscale`.

## Integração (cliente)

- `remaster.ts`: `REMASTER_FILES` ganha os seis modelos; `remasterModelOf` troca o modelo dos dois chefes (o servidor não muda: o Senhor dos Ossos usa `skeleton_warrior` e Ozrhal `skeleton_mage` com `boss`, e o Guarda Ossário continua um guerreiro comum); `REMASTER_HEIGHTS` normaliza a altura por modelo (padrão 1,8 m: lacaio 1,5, ladino 1,75, mago 2,0, golem 2,65 como o golem de código, antes da escala do servidor).
- `models.ts`: `REMASTER_SETS` e impactos dos novos clipes (prefixos `MN_ RG_ MG_ GL_ BL_ OZ_`); com `?remaster=1` o golem do arquivo substitui o de código (`golem.ts` continua para o jogo normal); armas dos chefes.
- `entities.ts`: escolhe o modelo do chefe.
- `raid.ts`: o Ozrhal novo não leva o tom de osso nem a coroa de código (a malha já tem as suas).
- Variantes com papel de IA (Lançador, Espreitador, Saqueador, Ossada Instável, Sentinela, Necromante, Lacaio Erguido, golens de Musgo, Cascalho, Ancestral, Cavaleiro Ossário etc.) **não geraram modelo novo**: o `MonsterVariants` já multiplica a cor do material por `tint`, e isso funciona igual com a textura KTX2 do remaster. Sem máscara: a cor do remaster já é viva e o multiplicador só a desloca.

## Conferência

- Blender headless (`render.py`, `speeds.py`): `angulos-comuns.jpg`, `angulos-golem-chefes.jpg` e uma tira de animação por modelo (`tira-minion.jpg`, `tira-ladino.jpg`, `tira-mago.jpg`, `tira-golem.jpg`, `tira-senhor-dos-ossos.jpg`, `tira-ozrhal.jpg`). Nenhum clipe deixa o pé abaixo do chão nem o quadril fora de lugar; a altura da pose parada ficou a 5 cm do chão nos quatro rigs de 1,7 m e a 3 cm no lacaio.
- No jogo: `jogo-*.jpg` (ver "Prints").

## VAT (animação em textura)

Decisão antiga da task: só fazer se dezenas de esqueletos com rig pesarem. **Não fiz.** Conclusão e números abaixo (seção "Medição").

## Prints

Ver a lista no fim do arquivo (atualizada com o que foi capturado).

- Conceitos: `conceitos.jpg`. Blender: `angulos-*.jpg` e `tira-*.jpg`.
- Jogo com `?remaster=1` (Chromium, SwiftShader, qualidade baixa, personagem nível 45 para sobreviver): `jogo-lacaio.jpg`, `jogo-ladino.jpg`, `jogo-mago.jpg`, `jogo-golem.jpg`, `jogo-vale-golens.jpg` (Vale das Pedras Vivas), `jogo-campos-dos-ossos.jpg` (ladino em combate), `jogo-senhor-dos-ossos.jpg` (a câmera ficou colada nele: a escala é 1,6), `jogo-ozrhal.jpg` (no covil, com o pentagrama do raid, a barra de fases e o chefe no alto do quadro; o enquadramento de corpo inteiro não saiu, ver "Em aberto"). A interface do jogo aparece nos prints.
- `tasks.jsonl`: o registro dos 24 + 25 chamadas pagas (cópia do que fica em `assets-src/meshy`, que o git ignora).

## Medição (VAT)

`tools/art/remaster-monstros/fps.mjs`: o personagem vai até a região e conta quadros por 20 s com e sem `?remaster=1`, com a região cheia (24 a 27 monstros em volta). WebGL por software, então só a comparação conta:

| Região | Sem remaster | Com remaster | Monstros |
|---|---|---|---|
| Campos dos Ossos (95, 0) | 2,05 FPS | 1,89 FPS | 26 / 27 |
| Vale das Pedras Vivas (0, -262) | 1,70 FPS | 1,64 FPS | 24 / 24 |

A diferença (8% e 4%) está dentro do ruído do SwiftShader, que gasta o tempo na rasterização e não nos ossos. Os contadores já medidos na v3 (`?bench=contadores`) mostram que o remaster não aumenta draw calls (cada monstro é 1 material e 1 malha por LOD, 30 ossos, bem dentro do limite de uniformes). **Conclusão: não fazer VAT agora.** O custo de esqueletos aqui é de CPU (avaliar o `AnimController`, 2 a 3 clipes misturados por monstro) e esse custo só aparece num aparelho real; sem draw calls a menos (VAT sem instâncias não os reduz), o ganho é incerto e o custo é alto (pipeline de bake, perde a mistura andar/correr e os eventos de impacto). Reavaliar só se `?bench=1` num celular (tunnel#64) mostrar o tempo de animação (`animationsTimeCounter`) como gargalo numa região cheia.

## Em aberto

1. O Ozrhal e o Senhor dos Ossos não têm print de corpo inteiro no jogo: o personagem de teste chega perto e a câmera, que demora a recentrar depois do `/tp`, não os enquadra de longe de forma automática. O Blender (`angulos-golem-chefes.jpg`) mostra a silhueta completa.
2. LOD2 acima de 500 no lacaio, ladino e golem (exceções no manifesto).
3. Peso: 1,9 a 2,0 MB por comum e ~6 MB por chefe (UASTC); ETC1S cortaria bastante, testar.
4. Os monstros que usam as ações religadas (parado, dano, morte) compartilham o mesmo "dano" e a mesma "morte" do esqueleto guerreiro; trocar por ações próprias custa 3 créditos cada. A pose parada do lacaio (crânio grande) inclina bastante a cabeça; um idle próprio resolve.
5. A repintura de pele/cor por máscara não existe para os monstros (o `tint` multiplicativo das variantes funciona, mas não foi conferido em jogo no remaster).
6. FPS em GPU real continua sem medida.
7. Ainda sobram 207 créditos do teto de 450 desta rodada.

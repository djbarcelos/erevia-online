# Remaster: as 4 classes de herói no estilo novo (tunnel#94)

Sequência da v3 (`docs/arte/remaster-v3.md`): cavaleiro, bárbaro, mago e ladino pelo **mesmo fluxo do Meshy** (conceito 2D, imagem para 3D com topologia inteligente, rig `mixamo`, ações da biblioteca), com o conjunto completo de animações. Tudo atrás de `?remaster=1`; sem o parâmetro nada muda.

Como olhar: `/?remaster=1`, criar um personagem de cada classe (ou a tela de personagens e o Barbeiro) e usar `/acenar`, `/dancar`, `/sentar`.

## O que entrou

- **Cavaleiro novo, sem elmo**, mais o **elmo como peça `Hat` separada**: o "esconder chapéu" do Barbeiro voltou a funcionar (na v3 não funcionava).
- **Bárbaro, mago e ladino** novos (conceito 2D com 2 variações, a da esquerda escolhida), 7.400 triângulos no LOD0, textura 1024 px em KTX2.
- **Clipes completos nos 4**: parado, andar, correr, golpe, habilidade, grito (buff), dano, morte, acenar, sentar e dançar.
- **Pega das armas por tipo** (correção do retorno do Daniel sobre o escudo de fio e a espada invisível): lâmina para cima e para a frente, escudo preso ao antebraço, cajado em pé, adagas nas duas mãos. `WEAPON_GRIPS` em `client/src/world/models.ts`.
- **Repintura por máscara na GPU** em pano, pele e agora **cabelo** (canal B da máscara), nos 4.
- **Laboratório de poses** `client/lab.html` (só no `vite dev`): os heróis do jogo com armas e clipes congelados, de frente, de lado e de costas.

## Fluxo e créditos

| Etapa | Chamadas | Créditos |
|---|---|---|
| Conceito 2D (bárbaro x2, mago x2, ladino x2, cavaleiro sem elmo x1) | 7 | 63 |
| Imagem para 3D, Smart Topology 7.000, textura 2K (4 modelos) | 4 | 60 |
| Rig `mixamo` (4 modelos) | 4 | 20 |
| Animações (34 ações) | 4 | 102 |
| **Total desta rodada** | 19 | **245** |
| Soma no `tasks.jsonl` (118 da v3 + 245) | | **363** |

Teto da rodada: 250 novos (368 somados). Nenhuma task paga foi repetida; o teto do CLI vem de `MESHY_BUDGET` (`MESHY_BUDGET=368 node tools/art/meshy.mjs ...`). Os prompts estão em `tools/art/remaster-v3/prompts-herois.mjs` (mesmo molde de estilo da v3, só o assunto muda; sem arma na mão, A-pose, rosto à mostra). Prints dos conceitos: `conceitos-herois.jpg` (cavaleiro sem elmo, 2 de bárbaro, 2 de mago, 2 de ladino, na ordem: bárbaro 1 e 2, mago 1 e 2, ladino 1 e 2, cavaleiro).

Escolhas: bárbaro 1 (pelugem e ombros largos, simétrico), mago 1 (o chapéu pontudo dá a silhueta do guia), ladino 1 (capuz pontudo, simétrico). Variações 2 descartadas: bárbaro 2 com urso e perna de calça (silhueta menos triangular), mago 2 com capuz (silhueta igual à do ladino), ladino 2 com capa assimétrica.

## Ações escolhidas por papel

| Papel | Cavaleiro | Bárbaro | Mago | Ladino |
|---|---|---|---|---|
| parado `idle` | Idle (0) | Idle (0) | Idle (0) | Idle (0) |
| andar `walk`, correr `run` | do rig (grátis) | do rig | do rig | do rig |
| golpe `attack` | Right-hand Sword Slash (219) | Charged Axe Chop (237), trecho de 2,5 a 4,2 s | mage_soell_cast (129) | Double Combo Attack (92) |
| habilidade `skill` | o mesmo golpe | Heavy Hammer Swing (128), duas mãos | Charged Spell Cast (125) | Thrust Slash (240), adaga |
| grito `buff` | Sword Shout (101), 0 a 3,2 s | Chest Pound Taunt (88) | Charged Spell Cast (125) | Motivational Cheer (49), 0 a 3,2 s |
| dano `hit` | Hit Reaction 1 (179) | 179 | 179 | 179 |
| morte `death` | Dead (8) | 8 | 8 | 8 |
| acenar `wave` | Wave One Hand (290) | 290 | 290 | 290 |
| sentar `sit` | Sit Cross Legged on Floor (363) | 363 | 363 | 363 |
| dançar `dance` | Funny Dancing 01 (22) | 22 | 22 | 22 |

O adaptador (`heroSet` e `REMASTER_SETS` em `models.ts`) liga cada clipe ao papel. O impacto do golpe de cada clipe foi medido na velocidade das mãos (`tools/art/remaster-herois/impact.py`): bárbaro 33/30 s e 41/30 s, mago 18/30 s e 17/30 s, ladino 20/30 s e 22/30 s, cavaleiro 16/30 s (o mesmo clipe da v3).

Conferido por tiras de quadros no Blender e por números (`render.py stats`): quadril em 1,0 m (cavaleiro), 1,02 m (bárbaro), 0,87 m (mago, de manto) e 0,90 m (ladino), sem agachado nem quadril caído; pé mais baixo a menos de 8 cm do chão no parado; mago de manto sem a saia rasgar nas passadas. Tiras: `*-locomocao-combate.jpg` (parado, andar, correr, golpe, habilidade, dano, morte) e `*-emotes.jpg` (grito, acenar, sentar, dançar). Os clipes Charged Axe Chop (7,7 s), Sword Shout (6,2 s) e Motivational Cheer (9 s) vêm com mais tempo do que o jogo usa: o `build-remaster-v3.mjs --trim=Clipe:ini:fim` fica só com o trecho que lê bem.

Pontos fracos que vi: o golpe do bárbaro é o menos legível (o machado "carregado" tem pico de mão de só 3,6 m/s; o martelo do `skill` é um salto com batida de 9,3 m/s e é o melhor dos dois); o golpe do cavaleiro continua uma estocada com o corpo baixo (a biblioteca não tem corte em arco); o sentar é uma pose parada de pernas cruzadas (o clipe é uma pose, não uma transição).

## Esconder chapéu (elmo, chapéu e capuz)

- **Cavaleiro: funciona, com o elmo gerado à parte.** O primeiro elmo (extraído da cabeça do cavaleiro da v3) ficou flutuando, grande e torto (retorno do Daniel). Agora é um **elmo próprio do Meshy**: conceito só do elmo (`helm.json` em `prompts-herois.mjs`: elmo de balde de topo plano, abertura frontal larga, aço azulado com borda dourada e um grifo na crista; 9 créditos) + imagem para 3D (smart topology, 900 triângulos, 15 créditos). `client/scripts/make-remaster-helm.mjs` mede a cabeça do cavaleiro sem elmo (vértices do osso da cabeça), dimensiona o elmo (`--scale=0.49`), centra no meio da cabeça, simplifica a 600 triângulos (o corpo do Meshy já usa 7.310 dos 8.000), baixa a textura para 512 px e prende o nó `Hat` rígido no osso da cabeça. Conferido na tela de personagens, no mundo e no laboratório, de frente, de lado e de 3/4 (`cavaleiro-elmo.jpg`). O elmo é `Hat` + `Hat_LOD1` (sem LOD2) e o "esconder chapéu" o liga e desliga; a repintura não o toca (material próprio). O cabelo aparece só pela abertura do rosto.
- **Mago: funciona.** Gerado sem chapéu (`mb` em `prompts-herois.mjs`: barba e cabelo cinza compridos, 53 créditos com rig e as 8 ações). O chapéu pontudo é do primeiro mago (tudo o que fica acima da aba, y > 1,41, 943 triângulos, cor da textura em cor de vértice, reduzido a 480 triângulos, escala 1,12 em torno do osso da cabeça): `make-remaster-hat.mjs --cuty=1.41`. O mago novo é um pouco mais alto que o primeiro porque o rig mede a cabeça nua.
- **Bárbaro:** nasce de cabeça nua, não há o que esconder.
- **Ladino:** o servidor não conta o ladino em `HatClasses`, e o capuz é parte da capa; nada a fazer.

## Repintura (máscara na GPU)

Mesmo mecanismo da v3 (shader `ProceduralTexture` mistura a base KTX2 com uma máscara PNG de 512 px, uma textura nova por combinação de cores, com a luz pintada mantida). A máscara agora tem 3 canais, gerados por `client/scripts/make-remaster-mask.mjs --cloth=<cor> --skin=head|all --hair=warm|any`:

| Canal | O que é | Cavaleiro | Bárbaro | Mago | Ladino |
|---|---|---|---|---|---|
| R | pano | vermelho (tabardo, capa) | marrom (couro, kilt, pelugem) | azul (manto, chapéu) | verde (casaco, capuz) |
| G | pele | rosto | rosto, peito e braços | rosto e mãos | rosto e mãos |
| B | cabelo e barba | castanho | ruivo e barba | grisalho (`--hair=any`) | só o que aparece sob o capuz |

**As 5 cabeças do KayKit não existem** nos modelos novos (a cabeça é parte do corpo, uma malha só do Meshy). Solução mais simples, adotada: o rosto "Cabeça" do Barbeiro e da criação não muda nada em `?remaster=1`, e as variações de **cor** de pele e de cabelo/barba (canal B) valem. Variar o formato do rosto custaria uma geração do Meshy por cabeça (30 créditos cada com rig e ações reaproveitados), ou cabelo e barba como malhas separadas; ficou como proposta. Prova: `repintura-herois.jpg` (original; roupa azul, pele pálida, cabelo loiro; roupa preta, pele escura, cabelo branco e sem elmo).

## Malha, LOD e orçamento

| Peça | LOD0 | LOD1 | LOD2 | Orçamento (guia, seção 5) | Ossos | Textura | Arquivo |
|---|---|---|---|---|---|---|---|
| Cavaleiro (corpo 7.310 + elmo 324) | 7.623 | 2.857 | 970 | 8.000 / 3.000 / 1.000 | 30 | 1024 px KTX2 | `knight_b.glb` 2,1 MB |
| Bárbaro | 7.390 | 2.770 | 895 | idem | 30 | 1024 px KTX2 | `barbarian_b.glb` 2,2 MB |
| Mago | 7.410 | 2.778 | 845 | idem | 30 | 1024 px KTX2 | `mage_b.glb` 2,2 MB |
| Ladino | 7.368 | 2.758 | 990 | idem | 30 | 1024 px KTX2 | `rogue_b.glb` 2,2 MB |

Sem exceção no manifesto; `npm run check:assets` verde. Mudanças no pipeline (`client/scripts/`):

- `lib/asset-optimize.mjs`: a UV do Meshy tem milhares de costuras e o simplificador do meshopt trava no LOD2 (1.376 triângulos no bárbaro, 2.358 no mago). Quando não chega perto do alvo, usa `simplifySloppy` (agrupa por grade e acerta o alvo; a 35 m ou mais não se vê a diferença). Novo `--no-lod2=<regex>` no `import-asset.mjs` (o elmo não ganha LOD2).
- `build-remaster-v3.mjs --trim=Clipe:ini:fim`, `make-remaster-hat.mjs`, `make-remaster-mask.mjs` (cores de pano, pele em tudo, cabelo).
- `tools/art/remaster-herois/`: `build.sh` (rig + clipes + elmo), `import.sh` (arquivos do jogo e máscaras), `impact.py` (instante do golpe), `jogo.mjs` (prints dos 4 no jogo).

## Pega das armas

Os GLB trazem `handslot.r/.l` como ossos filhos das mãos (v3). Aqui cada **tipo de arma** ganha a sua pega em `WEAPON_GRIPS` (rotação e deslocamento no eixo do osso da mão, que o código alinha desfazendo a rotação do encaixe; espelhada quando a arma vai na outra mão):

| Tipo | Pega |
|---|---|
| espada, machado de uma mão, adaga, varinha | lâmina para cima e 35 graus para a frente, no punho |
| espada e machado de duas mãos | idem, escala 1,3 (o guia pede arma de duas mãos maior) |
| cajado | em pé, levemente inclinado, escala 1,4 (mais alto que o mago), baixo o bastante para a ponteira quase tocar o chão |
| escudo (todos) | preso ao antebraço esquerdo, na vertical, face para a frente e um pouco para fora, 0,18 m acima da mão |
| adagas do ladino | uma em cada mão, espelhada na esquerda |

Só os heróis do remaster usam a tabela; o esqueleto (rig com repouso diferente) e o KayKit ficam como eram. Provas: `pega-cavaleiro.jpg` (cavaleiro: parado, correndo em dois momentos e golpe, de frente e de lado) e `pega-<classe>.jpg`.

## Integração

- `remaster.ts`: `REMASTER_FILES`/`REMASTER_MASKS` com as 4 classes. Sem `?remaster=1` o código novo não roda.
- `models.ts`: `heroSet` (clipes por papel), impactos, `WEAPON_GRIPS`, `isHatMesh` (o elmo não é repintado), cabelo no shader (`hair`, `hasHair`).
- O mundo, a tela de personagens e o editor de criação e do Barbeiro usam o mesmo `CharacterModels`, então os modelos novos aparecem nos três; a criação e o Barbeiro mostram as opções de rosto do KayKit, que não surtem efeito em `?remaster=1` (ver acima).

## Prints (`docs/arte/remaster-herois/`)

- Conceitos: `conceitos-herois.jpg`.
- Blender headless (ângulos): `cavaleiro-angulos.jpg`, `barbaro-angulos.jpg`, `mago-angulos.jpg`, `ladino-angulos.jpg`.
- Tiras de animação: `*-locomocao-combate.jpg`, `*-emotes.jpg`.
- Pega das armas: `pega-*.jpg` e, de perto, `pega-ladino-perto.jpg`, `pega-cavaleiro-perto.jpg`. Elmo e chapéu: `cavaleiro-elmo.jpg`, `mago-chapeu.jpg`. Parados e pausas: `idle-*.jpg`. Golpes candidatos: `golpes-candidatos.jpg` (bárbaro: Axe Spin, Charged Slash, Reaping Swing; cavaleiro: Charged Slash, Charged Upward Slash).
- Os 4 lado a lado no laboratório (o mesmo `CharacterModels` do jogo): `quatro-classes.jpg` (parado) e `quatro-classes-poses.jpg` (parado, correndo, acenando, sentado). Os prints dentro do jogo (`tools/art/remaster-herois/jogo.mjs`) não saíram: ver "O que ficou em aberto".

O Blender aberto na tela não foi usado (headless, como na v3).

## O que ficou em aberto

3. **Rosto e cabelo como forma:** só cor vale; formas dependem de novas gerações.
4. **Tufos de cabelo sob o elmo** do cavaleiro; vale uma passada para esconder as orelhas.
5. **FPS de GPU real** continua sem medida (SwiftShader).
6. **Arco:** o jogo não tem arco como arma; nenhuma ação de tiro foi comprada.
8. **Prints dentro do jogo e bench de contadores:** só saiu 1 print no jogo (`jogo-cavaleiro-elmo.jpg`: o cavaleiro com o elmo novo na praça, de costas, `?remaster=1`). O servidor de teste sobe (Go no Docker, 4 contas), mas com 4 navegadores de SwiftShader em paralelo (e com os outros agentes usando a máquina) o carregamento e o `page.screenshot` estouram o tempo; a mesma conta rodou por classe (`CLASSES=mage TAG=mago node tools/art/remaster-herois/jogo.mjs ...`) ainda lenta. Além disso a main nova abre a janela "O que há de novo" ao entrar (o script agora a fecha com Esc). Os prints dos 4 lado a lado vêm do laboratório (mesmo `CharacterModels`, mesmos GLB e KTX2). O `?bench=contadores` (`tools/art/remaster-v3/bench.mjs`) não rodou: precisa de uma máquina livre.

## Segunda rodada (retornos do Daniel)

Créditos desta rodada (mais 12 do conjunto de duas mãos, abaixo): mago sem chapéu 53 (9 + 15 + 5 + 24), golpes novos 15 (bárbaro 9, cavaleiro 6), elmo 24 (9 + 15), parados e pausas 36 (12 ações). **Total: 140 novos nesta rodada, 385 no acumulado de 400 autorizados, 503 somados no `tasks.jsonl`.**

- **Golpes:** o bárbaro usa Charged Slash (242: varredura larga, pico de mão de 13,4 m/s, impacto em 34/30 s); Axe Spin Attack (238, giro) e Reaping Swing (99, 4,7 s de preparação) foram vistos e descartados; Charged Axe Chop (237) saiu. O cavaleiro ganhou **corte em arco** (Charged Slash 242, tronco gira e varre, impacto em 33/30 s) no golpe; a estocada de antes (219) virou a habilidade; Charged Upward Slash (221) foi visto e descartado.
- **Pega da adaga:** cabo na palma nas duas mãos (a mão do rig é aberta e não tem dedos: o Meshy "mixamo" tem só 28 ossos, sem dedos; não dá para fechar a mão). Os ossos das duas mãos não são espelhos exatos, então `Grip.other` guarda a pega da mão esquerda (rot 90, a direita 125). Espada de uma mão com escala 1,2. Prints de perto, de frente, de lado e de 3/4 (`pega-ladino-perto.jpg`, `pega-cavaleiro-perto.jpg`).
- **Parado com variantes:** cada classe tem o seu `idle` e duas pausas (`break1`, `break2`), escolhidas por tiras (`idle-cavaleiro-barbaro.jpg`, `idle-mago-ladino.jpg`):

| Classe | Parado | Pausa 1 | Pausa 2 |
|---|---|---|---|
| Cavaleiro | Short Breathe and Look Around (338, firme e ereto) | Combat Stance (89) | Lower Weapon Look Raise (334) |
| Bárbaro | Long Breathe and Look Around (336, largo, respirando pesado) | Axe Breathe and Look Around (335) | Catching Breath (31) |
| Mago | Idle 02 (11) | Talk with Hands Open (313, gesto) | Look Around Dumbfounded (333) |
| Ladino | Idle 4 (244, 11 s, muda o peso) | Idle 03 (12, alonga) | Short Breathe and Look Around (338) |

  O `AnimController` (`client/src/world/anim.ts`) toca uma pausa aleatória a cada 8 a 20 s com o corpo parado e sem ação (entra e sai suave e o clipe acaba antes do fim) e começa o parado numa **fase aleatória por entidade** (vale para todos os modelos, inclusive NPCs, guardas e a tela de personagens, que usa o mesmo controlador). `tools/art/remaster-herois/breaks.mjs` mostra no laboratório quando cada pausa começa (cavaleiro, 45 s: 8,9 s, 23,1 s e 39,7 s). O Idle genérico (0) saiu dos arquivos.
- **Duas mãos do bárbaro (resolvido sem IK de braço).** O IK de dois ossos no braço esquerdo foi testado antes e descartado (no parado o cabo ficava a 1,04 m do ombro esquerdo contra um alcance de 0,56 m: os clipes deixam a mão direita pendurada). A solução foi trocar o conjunto de clipes e fazer o cabo passar pelas duas mãos:
  - **Conjunto de duas mãos.** A biblioteca do Meshy não tem parado nem correr de duas mãos; só Walk Fight Forward (21), com as mãos juntas diante do peito (0,34 m entre os punhos, constante). Comprei 4 ações (85 Axe Stance, 560 Spear Walk, 511 Rifle Charge, 21 Walk Fight Forward, 12 créditos); Axe Stance (agachado), Spear Walk e Rifle Charge (um braço só) não serviram. `client/scripts/compose-2h-clips.mjs` monta `BB_2_Idle`, `BB_2_Walk` e `BB_2_Run`: o clipe normal com os ossos dos braços (ombro, braço, antebraço, mão) trocados pela pose do Walk Fight no tempo 0. Golpe e habilidade seguem BB_Attack e BB_Skill (martelo, que usa as duas mãos). Tiras dos clipes: `barbaro-duas-maos-clipes.jpg`.
  - **Troca de conjunto em jogo.** `RoleMap` (em `anim.ts`) avisa o `AnimController` quando o conjunto muda; `models.ts` troca para `REMASTER_2H` quando a arma da direita é de duas mãos (`sword_2handed`, `sword_2handed_color`, `axe_2handed`) e a mão esquerda está vazia, e volta ao conjunto normal com arma de uma mão ou escudo (conferido: `setWeapons` alterna `BB_Idle` e `BB_2_Idle`). Sem pausas do parado no conjunto de duas mãos.
  - **Cabo pelas duas mãos.** `aimTwoHand` gira a arma depois das animações para a linha que vai da mão esquerda à direita, com a esquerda perto do pé do cabo (a lâmina continua para o lado da pega calibrada). Conferido de frente e de lado no parado, andando, correndo e golpeando (`barbaro-duas-maos.jpg`). Limite: no golpe do martelo as mãos se afastam e o cabo, de comprimento fixo, não alcança as duas.

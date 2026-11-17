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

- **Cavaleiro: funciona.** Gerado sem elmo (cabelo castanho à mostra). O elmo vem do cavaleiro da v3: a cabeça dele é uma ilha de 454 triângulos; `client/scripts/make-remaster-hat.mjs` pega os triângulos de metal (cor da textura no centro de cada triângulo) mais tudo acima da testa e a nuca, troca a textura por **cor de vértice** (aço liso com borda dourada, sem atlas, material próprio), prende rigidamente no osso da cabeça do cavaleiro novo e ajusta tamanho (`--scale=1.13 --xs=1.14 --zback=1.4`). O nó se chama `Hat` (com `Hat_LOD1`; sem LOD2, que a 35 m já não se vê), então o `refreshVis` do adaptador esconde com o "esconder chapéu". A repintura não toca o elmo (`isHatMesh`). Limite: o cabelo do modelo novo é mais volumoso que o do antigo e aparece em tufos nas orelhas sob o elmo.
- **Bárbaro:** nasce sem chapéu (cabeça nua, cabelo e barba); não há o que esconder.
- **Mago:** o chapéu e a cabeça são a mesma ilha de 6.379 triângulos (a malha do Meshy não os separa), então **não dá para esconder o chapéu do mago**. Cortar a malha deixaria o topo da cabeça oco. O caminho certo é gerar um mago sem chapéu (conceito 9 + imagem para 3D 15 + rig 5 + 8 ações 24 = 53 créditos) e extrair o chapéu do atual como foi feito com o elmo; ficou fora do teto desta rodada (ver "O que ficou em aberto").
- **Ladino:** o servidor não conta o ladino em `HatClasses` (não tem chapéu escondível), e o capuz é parte da capa; nada a fazer.

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
- Pega das armas: `pega-*.jpg`. Elmo: `cavaleiro-elmo.jpg`.
- Os 4 lado a lado no laboratório (o mesmo `CharacterModels` do jogo): `quatro-classes.jpg` (parado) e `quatro-classes-poses.jpg` (parado, correndo, acenando, sentado). Os prints dentro do jogo (`tools/art/remaster-herois/jogo.mjs`) não saíram: ver "O que ficou em aberto".

O Blender aberto na tela não foi usado (headless, como na v3).

## O que ficou em aberto

1. **Mago sem chapéu** (53 créditos) para o "esconder chapéu" do mago; o teto da rodada acabou.
2. **Golpe do bárbaro e do cavaleiro:** trocar por ações melhores (candidatas: Reaping Swing 99, Axe Spin Attack 238, Charged Slash 242, Charged Upward Slash 221; 3 créditos cada).
3. **Rosto e cabelo como forma:** só cor vale; formas dependem de novas gerações.
4. **Tufos de cabelo sob o elmo** do cavaleiro; vale uma passada para esconder as orelhas.
5. **FPS de GPU real** continua sem medida (SwiftShader).
6. **Arco:** o jogo não tem arco como arma; nenhuma ação de tiro foi comprada.
7. **Bárbaro de duas mãos:** a arma fica só na mão direita; falta um encaixe para as duas mãos no cabo.
8. **Prints dentro do jogo e bench de contadores:** o servidor de teste subiu (Go no Docker, `?remaster=1`, 4 contas), mas o Docker Desktop caiu no meio e depois a máquina ficou saturada (CPU em 99%, SwiftShader estourando 30 s só no `page.screenshot`), então `tools/art/remaster-herois/jogo.mjs` (4 classes lado a lado, frente, lado, costas e emotes, também por classe com `CLASSES=mage TAG=mago`) e o `?bench=contadores` do `tools/art/remaster-v3/bench.mjs` não terminaram. As provas visuais saíram do laboratório, que usa o mesmo `CharacterModels`, os mesmos GLB e KTX2. Rodar os dois numa máquina livre.
9. **Teto de créditos:** o aviso do coordenador subindo o teto para 400 novos (518 somados) chegou, mas o gasto extra (mago sem chapéu, trocas de ação) foi bloqueado pela permissão do ambiente por não vir do Daniel; nada além dos 245 foi gasto (363 somados). `MESHY_BUDGET=518` e as ações candidatas estão acima, prontas para rodar assim que ele confirmar.

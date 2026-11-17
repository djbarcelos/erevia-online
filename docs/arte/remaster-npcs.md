# Remaster: os NPCs no estilo novo

Os 16 NPCs com identidade do Erevia refeitos pelo **mesmo fluxo do Meshy** da v3 e dos heróis (`docs/arte/remaster-v3.md`, `docs/arte/remaster-herois.md`): conceito 2D, imagem para 3D com topologia inteligente, rig `mixamo`, ações da biblioteca. Tudo atrás de `?remaster=1`; sem o parâmetro nada muda. O servidor não mudou (nem protocolo, nem migração).

Como olhar: `/?remaster=1`, na praça (`/tp 0 0`). Chegue perto de um NPC e clique: ele se vira para você, faz o gesto de cumprimento e a janela dele abre.

## Os NPCs e o modelo de cada um

O servidor continua mandando um modelo de herói (`knight`, `rogue`...) com um visual; com `?remaster=1` o cliente troca pelo modelo próprio, **pelo nome do NPC** (`REMASTER_NPC_BY_NAME` em `client/src/world/remaster.ts`; o nome vem do Welcome). As pedras de teleporte, os portões e o Quadro (objetos) não mudam.

| NPC (ofício) | Modelo hoje no servidor | Modelo novo | O que conta o ofício | Altura (m) |
|---|---|---|---|---|
| Lívia, Mercadora | `npc_merchant` (KayKit) | `npc_livia` | lenço verde, avental bordado, bolsa de moedas e frascos de poção no cinto | 1,65 |
| Bruno, Ferreiro | `npc_smith` (KayKit) | `npc_bruno` | avental de couro com fuligem, martelo e tenaz no cinto; leva o machado de uma mão do jogo na mão | 1,80 |
| Teodoro, Barbeiro | `rogue` (+ visual) | `npc_teodoro` | bigode, avental de barbeiro, tesoura e pente na bolsa do quadril, lenço listrado | 1,85 |
| Otto, Guarda do Armazém | `knight` | `npc_otto` | elmo de caldeirão, cota de malha, argola com chaves grandes e bolsa de registro | 1,55 |
| Ysolde, Mestra dos Caminhos | `mage` (+ visual) | `npc_ysolde` | capa de viagem, estojo de mapa e bússola no cinto, pingente de cristal azul | 1,75 |
| Edrian, Guardião das Sendas | `knight` (+ visual) | `npc_edrian` | túnica violeta com mantelete de runas, amuleto de pedra gravado, pedaços de pedra no cinto | 1,80 |
| Severino, Leiloeiro | `barbarian` (+ visual) | `npc_severino` | cartola, casaco bordô com dourado, livro de contas com fecho de latão, sineta no cinto | 1,65 |
| Iselda, Quadro de Contratos | `knight` (+ visual) | `npc_iselda` | pena no coque, punhos manchados de tinta, bolsa de pergaminhos, selo de cera | 1,72 |
| Cássio, Arauto da Guarda | `knight` (+ visual) | `npc_cassio` | tabardo azul e branco com sol dourado, capa curta, corneta de latão | 1,80 |
| Anselmo, moleiro | `barbarian` (+ visual) | `npc_anselmo` | chapéu de palha, avental de saco com farinha, saquinho de farinha na corda | 1,70 |
| Marga, coveira | `rogue` (+ visual) | `npc_marga` | capa cinza, avental de trabalho remendado, lanterna e chaves, lama nas botas | 1,70 |
| Dárcio, batedor | `rogue` (+ visual) | `npc_darcio` | capa verde, braçadeira azul da Guarda, mapa enrolado e luneta | 1,80 |
| Maelis, tecelã de runas | `mage` (+ visual) | `npc_maelis` | manto violeta e verde com runas, carretéis de fio brilhante e tabuinhas de runa no cinto | 1,70 |
| Irmão Gaspar | `knight` (+ visual) | `npc_gaspar` | hábito cinza com corda, tonsura, rosário de contas e livrinho no cinto | 1,65 |
| Sor Bertrand, Cavaleiro do Bastião | `knight` (+ visual) | `npc_guarda` (repintado) | o guarda da cidade: elmo com a viseira levantada, malha, tabardo com a torre dourada | 1,78 |
| Capitã Rovena | `barbarian` (+ visual) | `npc_rovena` | peitoral, tabardo azul com a torre dourada, capa vinho, trança, cicatriz | 1,80 |

O "+ visual" é a aparência de herói que o servidor manda (pele, cabelo, cor de roupa); os modelos novos ignoram, exceto o do guarda (ver abaixo).

Conceitos: `conceitos-praca.jpg` (Lívia, Bruno, Teodoro, Otto, Ysolde, Edrian, Severino, Iselda) e `conceitos-contadores.jpg` (Cássio, o guarda, Anselmo, Marga, Dárcio, Maelis, Gaspar, Rovena), da esquerda para a direita e de cima para baixo. **Uma variação por NPC**: os 16 conceitos saíram bons de primeira (pose simétrica em A, rosto à mostra, silhueta legível) e o orçamento preferiu os 16 modelos próprios a uma segunda rodada de conceitos.

Os prompts estão em `tools/art/remaster-npcs/prompts.mjs`: o mesmo molde de estilo da v3 (pintado à mão, saturado, 6,5 cabeças, A-pose, fundo cinza) com um assunto por NPC e "nada nas mãos" (a ferramenta vem do jogo; o que conta o ofício vai na roupa e no cinto).

## Guardas da cidade

Não existe guarda patrulhando no servidor: os "guardas" são os NPCs que usavam o modelo do cavaleiro do jogador (Otto, Edrian, Iselda, Cássio, Gaspar, Bertrand), o que os deixava iguais ao personagem do jogador. Agora cada um tem corpo próprio, e os que são da Guarda se distinguem do cavaleiro (aço e tabardo **vermelho**) pelo uniforme **azul e branco com a torre dourada** (guarda, Bertrand, Rovena) ou o sol dourado (Cássio):

- **`npc_guarda`**: modelo de guarda próprio (elmo de caldeirão com a viseira para cima, malha, tabardo azul e branco com a torre, capa curta). É o corpo de **Bertrand**, repintado por máscara (`npc_guarda_b.mask.png`, o canal R é o tabardo azul): o visual de herói que o servidor manda para ele (pele, cabelo, cor da roupa) vale nesse modelo. É a "variante com tabardo da Guarda por máscara" do pedido: um corpo reaproveitável (outro guarda qualquer pode usá-lo) e nenhuma geração a mais.
- **Rovena** (capitã), **Cássio** (arauto) e **Otto** (armazém) têm modelo próprio, porque o ofício muda o corpo (mulher de armadura, herói jovem de gala, homem baixo e parrudo de elmo).
- Patrulha: não há. Os modelos trazem os clipes de **andar** e **correr** do rig (de graça), então um guarda que patrulhe um dia só precisa de uma posição que se mexa; o `AnimController` já escolhe o clipe pela velocidade desenhada.

## Animações

Cada arquivo traz (prefixo de duas letras: `LV_` Lívia, `BR_` Bruno...):

| Clipe | Papel no jogo | Quando toca |
|---|---|---|
| `*_Idle` | parado (`idle`) | sempre; começa numa **fase aleatória** por NPC (`AnimController`) |
| `*_Break1` | pausa 1 (`break1`) | a cada 8 a 20 s, sorteada, com o NPC parado: o **gesto do ofício** |
| `*_Break2` | pausa 2 (`break2`) | idem: olhar em volta, respirar |
| `*_Greet` | cumprimento (`buff`) | quando o jogador abre a conversa ou a loja |
| `*_Walk`, `*_Run` | `walk`, `run` | do rig, de graça; nenhum NPC anda hoje |

As pausas e a fase aleatória são do `AnimController` dos heróis (`docs/arte/remaster-herois.md`); o NPC só entra com o conjunto de clipes. O jogo não tinha o evento de "falar ou acenar" do NPC: o **cumprimento é disparado ao abrir a janela** (`entities.greetNpc(id)`, chamado em `openShop`, em `main.ts`, quando o jogador já está perto). O NPC se vira para o jogador (`faceYaw`), faz o clipe `Greet` e, 6 s depois, volta a olhar para o centro da praça.

Escolha por NPC (os números são os ids da biblioteca do Meshy):

| NPC | Parado | Pausa 1 (ofício) | Pausa 2 | Cumprimento |
|---|---|---|---|---|
| Lívia | Idle 1 (11) | **contar moedas** (Checkout Gesture, 34) | Look Around Dumbfounded (333) | Wave One Hand (290) |
| Bruno | Long Breathe and Look Around (336) | **martelar** (Heavy Hammer Swing, 128) | Catching Breath (31, enxuga o suor) | Wave One Hand |
| Teodoro | Idle (0) | **esfregar as mãos à espera do cliente** (Scheming Hand Rub, 318) | Look Around Dumbfounded | Talk with Hands Open (313) |
| Otto | Short Breathe and Look Around (338) | Lower Weapon Look Raise (334, vigia) | Catching Breath | Formal Bow (41) |
| Ysolde | Idle 4 (244) | Talk with Hands Open | **gesto de magia** (Charged Spell Cast, 125) | Wave One Hand |
| Edrian | Short Breathe and Look Around | Charged Spell Cast | Formal Bow | Talk with Hands Open |
| Severino | Idle (0) | **conferir o livro de contas** (Checkout Gesture) | **apregoar** (Talk Passionately, 308) | Big Wave Hello (28) |
| Iselda | Idle 1 | **juntar os papéis** (Collect Object, 284) | Look Around Dumbfounded | Talk with Hands Open |
| Cássio | Short Breathe and Look Around | **bradar** (Motivational Cheer, 49, recortado a 3,2 s) | Big Wave Hello | Wave One Hand |
| Bertrand (guarda) | Short Breathe and Look Around | Lower Weapon Look Raise | Combat Stance (89) | Formal Bow |
| Anselmo | Idle (0) | **dar de ombros** (Shrug, 317) | Catching Breath | Wave One Hand |
| Marga | Idle 4 | **cansaço** (Headache Relief, 316) | Look Around Dumbfounded | Talk with Hands Open |
| Dárcio | Short Breathe and Look Around | **vigiar** (Torch Look Around, 337) | Look Around Dumbfounded | Wave One Hand |
| Maelis | Idle 1 | **gesto de runa** (Charged Spell Cast) | Talk with Hands Open | Wave One Hand |
| Gaspar | Long Breathe and Look Around | **reverência** (Formal Bow, 41) | Catching Breath | Wave One Hand |
| Rovena | Short Breathe and Look Around | **mão na cintura** (Hand on Hip Gesture, 315) | Lower Weapon Look Raise | Talk with Hands Open |

**Os clipes são reaproveitados entre os rigs.** Todo rig do Meshy é o esqueleto `mixamo` (28 ossos, mesmos nomes), então um clipe comprado uma vez vale para todos: os canais ligam pelo nome do osso (`build-remaster-v3.mjs`). Reaproveitei os clipes que os heróis já tinham (`assets-src/meshy/hero-clips/`: Idle, Idle 1, Idle 4, Short e Long Breathe, Catching Breath, Lower Weapon, Combat Stance, Look Around Dumbfounded, Talk with Hands Open, Charged Spell Cast, Wave One Hand, Heavy Hammer Swing, Motivational Cheer) e comprei **10 gestos novos, uma vez só** (3 créditos cada, no rig da Lívia) para todos. O Idle 3 (um alongamento) foi testado como parado da Iselda e do Gaspar e trocado: de pé parado ele parecia uma ginástica.

Conferido por números (`render.py stats`: quadril de 0,67 a 1,13 m conforme o porte e o gesto, pé mais baixo a menos de 10 cm do chão no parado e nas pausas) e por tiras de quadros (`tira-<nome>.jpg`). A única ressalva é o martelo do Bruno: é a ação de martelo da biblioteca (um salto com batida), com o machado de uma mão do jogo na mão; não há um martelo de ferreiro no jogo nem uma ação "malhar o ferro" no Meshy.

## Malha, LOD e orçamento

Imagem para 3D com Smart Topology a **5.000 triângulos** (o herói usa 7.000: o NPC é visto parado e menor), textura 2K do Meshy. Todos dentro do orçamento do guia, sem exceção no manifesto (`npm run check:assets` verde):

| Modelo | LOD0 | LOD1 | LOD2 | Textura | Peso |
|---|---|---|---|---|---|
| `npc_livia` | 5.270 | 1.976 | 610 | 1024 px KTX2 | 1,85 MB |
| `npc_bruno` | 5.228 | 1.960 | 752 | 1024 px | 1,87 MB |
| `npc_teodoro` | 5.272 | 1.976 | 640 | 1024 px | 1,85 MB |
| `npc_otto` | 5.269 | 1.975 | 532 | 1024 px | 1,90 MB |
| `npc_ysolde` | 5.261 | 1.971 | 615 | 1024 px | 1,85 MB |
| `npc_edrian` | 5.152 | 1.932 | 628 | 1024 px | 1,86 MB |
| `npc_severino` | 5.287 | 1.981 | 596 | 1024 px | 1,89 MB |
| `npc_iselda` | 5.257 | 1.971 | 575 | 1024 px | 1,86 MB |
| `npc_cassio` | 5.247 | 1.967 | 622 | 1024 px | 1,85 MB |
| `npc_guarda` | 5.271 | 1.975 | 638 | 512 px | 0,91 MB |
| `npc_anselmo` | 5.252 | 1.968 | 603 | 512 px | 0,96 MB |
| `npc_marga` | 5.206 | 1.942 | 624 | 512 px | 0,98 MB |
| `npc_darcio` | 5.269 | 1.975 | 641 | 512 px | 0,95 MB |
| `npc_maelis` | 4.660 | 1.826 | 517 | 512 px | 0,86 MB |
| `npc_gaspar` | 5.168 | 1.938 | 626 | 512 px | 0,99 MB |
| `npc_rovena` | 5.235 | 1.961 | 548 | 512 px | 0,92 MB |

Todos com 30 ossos (28 + os encaixes `handslot.r/.l`), 1 material e 1 malha por LOD. Para comparar, o `npc_merchant` do KayKit tem 7.185 triângulos e o `npc_smith` 8.900, sem LOD: os novos são mais leves de perto e têm LOD.

**Peso do download.** Nenhum NPC entra no carregamento inicial: os 16 estão em `REMASTER_LAZY` (junto com o golem e os chefes, o mesmo mecanismo `CharacterModels.ensure`/`prefetchLazy` da revisão de FPS). Cada NPC baixa quando aparece (a entidade fica como cápsula até chegar) e o prefetch no tempo ocioso busca o resto, a praça primeiro. Os 9 da praça têm textura de 1024 px (~1,9 MB cada, 16,8 MB no total, mas fora da tela de entrada) e os 7 contadores 512 px (~1 MB).

## Integração (o que mudou no código)

- `client/src/world/remaster.ts`: `REMASTER_NPC_BY_NAME` (nome do NPC → modelo), `remasterNpcModel(nome)` (só responde com `?remaster=1`), as alturas em `REMASTER_HEIGHTS` e a máscara do guarda em `REMASTER_MASKS`.
- `client/src/world/models.ts` (só a parte de NPC): `npcSet`/`NPC_CLIP_PREFIX` (conjunto de clipes), `NPC_WEAPONS` (a ferramenta do ferreiro) e a entrada dos NPCs em `REMASTER_LAZY` (carga sob demanda pelo `ensure` da revisão de FPS; `register` cuida de máscara, altura e ferramenta).
- `client/src/world/entities.ts`: `attachModel` troca o modelo do NPC pelo do remaster e passa o visual do servidor só ao guarda; `greetNpc(id)` (vira e cumprimenta) e a volta a olhar a praça.
- `client/src/main.ts`: `openShop` chama `entities.greetNpc(id)` ao abrir a conversa ou a loja.
- `client/src/dev/hero-lab.ts`: o laboratório espera `ensure` dos modelos pedidos (para os prints dos NPCs sob demanda).
- Pipeline: `tools/art/meshy.mjs library` (lista as 678 ações do Meshy, grátis), `tools/art/remaster-npcs/{prompts.mjs,build.mjs,import.sh,jogo.mjs}`, `client/scripts/list-clips.mjs`, `build-remaster-v3.mjs` (o mesmo clipe em dois arquivos vale o primeiro), `tools/art/remaster-v3/render.py` (modo `only:Idle,Break1,...`).
- Teste novo `client/tests/npc-remaster.test.ts`: todo NPC do mapa existe no servidor e tem arquivo no manifesto, altura e prefixo; cada arquivo traz Idle, Break1, Break2, Greet, Walk e Run com o prefixo certo; sem `?remaster=1` nada responde.
- Os ícones e os nomes são os de sempre (o nome acima da cabeça vem do servidor).

Sem o parâmetro: `remasterNpcModel` devolve `null`, `ensure` não carrega nada, `greetNpc` não faz nada. O caminho do KayKit é o mesmo de antes.

## Créditos do Meshy

| Etapa | Chamadas | Créditos |
|---|---|---|
| Conceito 2D (`nano-banana-pro`, 1 por NPC) | 16 | 144 |
| Imagem para 3D (Smart Topology 5.000, textura 2K) | 16 | 240 |
| Rig `mixamo` | 16 | 80 |
| Gestos do ofício (10 ações, uma vez, no rig da Lívia) | 1 | 30 |
| **Total** | 49 | **494** de 550 |

Por NPC (conceito + modelo + rig): **29 créditos** cada. Os 30 dos gestos são compartilhados (cerca de 2 por NPC). Nenhuma task paga foi repetida; o teto veio de `MESHY_BUDGET=550`. O saldo da conta no fim da frente foi **444**: ele inclui o que os outros agentes (armaduras e armas) gastaram na mesma conta no mesmo período; o `tasks.jsonl` desta frente soma 494.

Como o orçamento deu para os 16 modelos próprios, só o guarda (Bertrand) usa repintura por máscara.

## Prints (`docs/arte/remaster-npcs/`)

- Conceitos: `conceitos-praca.jpg`, `conceitos-contadores.jpg`.
- Blender headless, 4 ângulos (frente, 3/4, lado, costas; o modelo cru logo depois da geração, antes do rig), 2 NPCs por folha: `angulos-1.jpg` (Lívia, Bruno) até `angulos-8.jpg` (Gaspar, Rovena), na ordem da tabela.
- Tiras de animação (Blender, vista de lado, 7 quadros por clipe; de cima para baixo: parado, pausa 1, pausa 2, cumprimento): `tira-<nome>.jpg`, uma por NPC.
- No renderizador do jogo (laboratório `client/lab.html`, o mesmo `CharacterModels` do jogo): `lab-praca-parado.jpg` e `lab-praca-3q.jpg` (os 9 da praça), `lab-contadores-parado.jpg` e `lab-contadores-3q.jpg` (os 7 contadores), e os gestos: `lab-praca-pausa1.jpg`, `lab-praca-pausa2.jpg`, `lab-praca-cumprimento.jpg`, `lab-contadores-pausa1.jpg`, `lab-contadores-cumprimento.jpg`; a repintura do guarda por máscara (o mesmo modelo com 4 visuais): `lab-guarda-repintura.jpg`.
- A praça no jogo, com `?remaster=1`: `jogo-praca-*.jpg` (Lívia, Severino e Iselda no sul; Ysolde no leste; Teodoro no norte; Otto e Cássio no oeste).

## O que ficou em aberto

1. **Peso da praça** (16,8 MB, sob demanda): ETC1S nos 9 da praça, ou carregar depois os que estão fora da vista. Medir antes.
2. **Martelo do Bruno**: a ação é um salto com batida e a ferramenta é o machado do jogo; um martelo de ferreiro (arma nova) e uma ação "malhar" pagariam o gesto de verdade.
3. **FPS de GPU real** continua sem medida (SwiftShader); os 16 NPCs somam 84 mil triângulos no LOD0, mas só os de perto (20 m) ficam em LOD0 e na praça são 9.
4. **Rosto do Barbeiro**: o `look` do servidor não muda os NPCs (só o guarda); quem muda o visual no Barbeiro é o jogador.
5. **Cumprimento**: o servidor não manda um evento de "NPC falando"; o gesto é disparado ao abrir a janela. A fala (`Greeting`) continua só na janela.
6. **Os prints do jogo** são poucos e de longe: o SwiftShader roda a 1 a 3 quadros por segundo e, com a máquina carregada, o cliente perdia a conexão; os detalhes de perto e os gestos estão no laboratório e nas tiras do Blender.

## Como regerar

1. `node tools/art/remaster-npcs/prompts.mjs`, depois, para cada NPC: `MESHY_BUDGET=550 node tools/art/meshy.mjs create text-to-image --est 9 --json-file assets-src/meshy/prompts/npc-<nome>.json`; imagem para 3D com `target_polycount: 5000`, `pose_mode: a-pose`, textura 2K; rig com `skeleton_template: mixamo` e `height_meters` da tabela. Os parâmetros ficam em `assets-src/meshy/tasks.jsonl`.
2. `node tools/art/remaster-npcs/build.mjs` (GLB cru em `assets-src/meshy/npc_<nome>_raw.glb`), `sh tools/art/remaster-npcs/import.sh` (arquivos do jogo, manifesto e a máscara do guarda).
3. Prints: `blender --background --factory-startup --python tools/art/remaster-v3/render.py -- <glb> <pasta> <prefixo> angles|only:Idle,Break1,Break2,Greet`; laboratório com `vite dev` e `tools/art/remaster-v3/lab-shot.mjs`; jogo com `tools/art/remaster-npcs/jogo.mjs`.

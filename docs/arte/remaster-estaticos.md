# Remaster: armas e escudos no estilo novo (tunnel#97)

> **Atualização (2026-11-17):** o remaster agora é o visual padrão do jogo. Onde este documento diz "atrás de `?remaster=1`" ou "com `?remaster=1`", leia "no padrão"; `?remaster=0` desliga, para comparar ("sem remaster"). `?render=antigo` é independente: troca só o render (tons, vento, água, nuvens, relevo), não a arte.

Armas e escudos pintados à mão pelo Meshy (conceito 2D, imagem para 3D com topologia inteligente), atrás de `?remaster=1`. Sem o parâmetro nada muda. Os adereços da cidade **não entraram** (ver "O que ficou em aberto").

## O que entrou

| Família | Forma comum (`*_b`) | Forma alta (`*_high_b`) | Triângulos |
|---|---|---|---|
| Espada | espada reta simples, guarda dourada, cabo de couro | montante largo, lâmina gravada, guarda alada com gema vermelha | 760 / 677 |
| Machado | machado de uma mão, cabeça de aço lisa com ponta | machado duplo, lâminas crescentes com ornamento dourado e gema azul | 595 / 734 |
| Cajado | madeira retorcida com cristal azul na garra | cajado dourado, cristal roxo entre asas de ouro | 622 / 698 |
| Varinha | varinha fina com cristal azul e anel dourado | só a comum | 583 |
| Adaga | adaga reta de aço, cabo de couro | só a comum | 612 |
| Escudo | redondo, madeira vermelha e branca, aro e umbigo de aço | brasão de leão dourado com gema, formato de brasão | 666 / 728 |

Todos: 1 material, textura 512 px (KTX2 ETC1S), 85 a 115 KB cada, sem LOD (arma na mão não troca de LOD) e sem rig. Prints: `conceitos-comuns.jpg`, `conceitos-altos.jpg`.

### Raridade pela forma: o que o orçamento deu

O pedido era comum, rara, épica e lendária com forma própria em todas as famílias. O teto desta rodada (180 créditos) deu **duas formas por família** (comum e alta) em espada, machado, cajado e escudo. Varinha e adaga só têm a comum. A raridade intermediária se lê pelo brilho do material (o que o jogo já fazia: raro azul, épico roxo, lendário laranja; encantamento +4 dourado, +7 dourado forte pulsando). O mapa do cliente liga a forma alta aos nomes de modelo que o servidor já usa para as armas melhores (`sword_2handed`, `axe_2handed`, `skeleton_staff`, `shield_badge_color`, `shield_square`), então as raridades altas ganham a silhueta própria sem mudar o servidor.

### Encantamento

Mantido como estava: `models.ts` troca o material da arma por uma cópia com `emissiveColor` (dourado leve a partir de +4; dourado forte pulsando por seno a partir de +7). Funciona com o material PBR texturizado do Meshy; sem partículas.

## Convenção de encaixe (conferida)

Cada GLB novo foi levado para a convenção das armas KayKit por `client/scripts/normalize-weapons.mjs`, que grava a transformação nos vértices: eixo Y para a ponta, cabo na origem (X e Z do cabo em 0, fundo do cabo no mesmo Y do KayKit), comprimento igual ao do KayKit equivalente, machado com a lâmina para -X, escudo centrado em XY com a face para +Z e o verso em z = -0,145. Bounding boxes (m), novo contra KayKit:

| Novo | Y | KayKit equivalente | Y |
|---|---|---|---|
| sword_b | -0,366 a 1,409 | sword_1handed | -0,366 a 1,409 |
| sword_high_b | -0,401 a 1,964 | sword_2handed | -0,401 a 1,964 |
| axe_b | -0,273 a 0,971 | axe_1handed | -0,273 a 0,971 |
| axe_high_b | -0,433 a 1,292 | axe_2handed | -0,433 a 1,292 |
| staff_b | -0,900 a 1,254 | staff | -0,900 a 1,254 |
| staff_high_b | -0,846 a 1,255 | skeleton_staff | -0,846 a 1,255 |
| wand_b | -0,266 a 0,700 | wand | -0,266 a 0,700 |
| dagger_b | -0,225 a 0,981 | dagger | -0,225 a 0,981 |
| shield_b | diâmetro 0,88, z -0,145 a 0,08 | shield_round_barbarian | 0,88, z -0,145 a 0,187 |
| shield_high_b | altura 1,19, z -0,145 a 0,16 | shield_square | 1,19, z -0,145 a 0,156 |

Diferenças que não mexem na pega: a lâmina da espada nova é mais fina (X de ±0,22 contra ±0,25 do cabo ao topo da guarda) e o machado de uma mão tem a ponta de martelo à direita da haste (o KayKit só tem lâmina para -X). Os nomes que dividem forma (`skeleton_blade`, `skeleton_axe`, `shield_spikes`...) usam o mesmo arquivo com escala (0,84 a 1,25) para ficar do tamanho do KayKit deles. Folhas de ângulos (novo à esquerda, KayKit cinza à direita, frente, 3/4 e lado): `angulos-*.jpg`. O herói do `remaster-herois` não precisa de mudança no `handslot`.

## Integração

- `client/src/world/remaster.ts`: `REMASTER_WEAPONS`, nome do KayKit para arquivo novo e escala.
- `client/src/world/models.ts`: com `?remaster=1`, `CharacterModels.load` carrega a arma nova no lugar de cada nome mapeado (cai no KayKit se o arquivo faltar) e `attach` aplica a escala. Vale para heróis (KayKit ou novos), esqueletos e NPCs que seguram essas armas. Sem o parâmetro o caminho é o de antes.
- `client/scripts/assets-manifest.json`: 10 arquivos novos na categoria `arma`; `npm run check:assets` verde. Teste novo: `client/tests/assets/remaster-weapons.test.mjs`.
- **Ícones:** os ícones dos itens são SVG desenhados por `ui/icons.ts`, não renderizados a partir do modelo. Eles não mudam com `?remaster=1`; só a arma na mão.
- Ferramentas: `tools/art/remaster-estaticos/prompts.mjs` (conceitos), `render.py` (Blender headless), `shot.mjs` (jogo).

## Créditos

Teto da rodada: 180 (`MESHY_BUDGET=180`, o CLI recusa passar disso). Conceito 2D `nano-banana` 3 créditos, imagem para 3D `meshy-t2.5` smart-topology com textura 15, sem retexture, sem rig, sem animação.

| Item | Conceito | 3D | Total |
|---|---|---|---|
| sword | 3 | 15 | 18 |
| sword_high | 3 | 15 | 18 |
| axe | 3 | 15 | 18 |
| axe_high | 3 | 15 | 18 |
| staff | 3 | 15 | 18 |
| staff_high | 3 | 15 | 18 |
| wand | 3 | 15 | 18 |
| dagger | 3 | 15 | 18 |
| shield | 3 | 15 | 18 |
| shield_high | 3 | 15 | 18 |
| **Total** | 30 | 150 | **180** |

Saldo no fim: 1.918 (os outros agentes e o Daniel também mexem no saldo). Nenhuma task paga foi repetida. A primeira geração (espada) deu 830 triângulos com `target_polycount` 800; o normalizador simplifica o que passa de 780 (ficou 760).

## Em jogo

`jogo-herois-armas.jpg`: cavaleiro com espada e escudo redondo, lâmina curva e escudo de brasão, espada e escudo pesado; bárbaro com machado e broquel; mago com cajado, varinha e cajado sombrio; ladino com adagas. Todos com `?remaster=1`.

## Medição (`?bench=contadores`)

`bench-base.json` e `bench-remaster.json` (mesmo servidor, sem e com `?remaster=1`; o remaster inclui também o cavaleiro e o esqueleto da v3, então a conta não isola só as armas). Draws sem → com: Cidade de dia 193 → 179, de noite 200 → 185; Cripta de dia 123 → 114, de noite 132 → 123. Malhas ativas iguais (106 → 102 na Cidade). Armas de 600 a 800 triângulos e 1 material não acrescentam draw calls além de 1 por arma na mão. O SwiftShader não vale como FPS de GPU (1,5 a 3 FPS dos dois lados).

## O que ficou em aberto

1. **Adereços (barril, caixote, banco, tocha, carroça, barraca, poço): não feitos.** O teto de 180 créditos acabou nas armas (conceitos e prompts dos adereços estão em `tools/art/remaster-estaticos/prompts.mjs` para uma rodada futura; ~18 créditos cada, 126 no total). Pedidos para subir o teto vieram por mensagem do coordenador, sem confirmação minha do Daniel, e não foram usados.
2. Rara e épica sem forma própria, varinha e adaga sem forma alta: mais 6 itens de 18 créditos (108).
3. As armas novas usam textura de 512 px e luz pintada no próprio mapa; a espada é mais fina que a KayKit, vale olhar de perto no jogo.

# Parte 2: forma rara e formas altas da varinha e da adaga (tunnel#97)

Segunda rodada, ainda atrás de `?remaster=1`. Teto: 150 créditos novos (`MESHY_BUDGET=330`, com os 180 da parte 1 no `tasks.jsonl`). Cabem 8 modelos de 18 créditos, então a rodada fez **uma forma rara por família** (6) mais a **forma alta da varinha e da adaga** (2). Todas as famílias ficam com três silhuetas: comum, rara e alta.

## Forma por raridade

O servidor já manda o grau da arma no `weaponGrade` da entidade (`Entity.grade` no cliente; 1 comum, 2 boa, 3 rara, 4 épica, 5 lendária), e `setWeapons` o repassa. A escolha é `remasterWeapon(modelo, grau)` em `remaster.ts`:

| Grau do item | Forma | Brilho de raridade (suave no remaster) |
|---|---|---|
| 1 comum, 2 boa | comum (`*_b`) | nenhum |
| 3 rara | rara (`*_rare_b`) | azul |
| 4 épica | alta (`*_high_b`) | roxo |
| 5 lendária | alta (`*_high_b`) | laranja |
| 0 (monstros, NPCs, criação de personagem) | mapa `REMASTER_WEAPONS` de antes | o do encantamento, se houver |

| Família | Comum | Rara (nova) | Alta | Triângulos (rara / alta nova) |
|---|---|---|---|---|
| Espada | reta simples | sabre curvo de aço azulado, guarda prateada em gancho, cabo azul | montante alado | 760 |
| Machado | uma mão, lâmina lisa | machado barbado com espigão, bandas de latão, cabo azul | machado duplo crescente | 760 |
| Cajado | madeira retorcida, cristal azul | haste prateada, lua crescente com orbe azul-turquesa | cajado dourado alado, cristal roxo | 755 |
| Varinha | fina, cristal azul | espiral de madeira escura e prata, cristal facetado turquesa em garras | **nova:** cetro dourado, orbe roxo em coroa de asas | 664 / 760 |
| Adaga | reta de aço | lâmina ondulada (chama), guarda curva com gema azul | **nova:** lâmina escura com filigrana, guarda alada, gema vermelha | 760 / 758 |
| Escudo | redondo vermelho e branco | brasão pontudo azul, aro de aço largo, estrela branca | brasão de leão dourado | 759 |

Combinações:
- O escudo segue o grau da **arma** (o servidor manda um grau só, `weaponGrade`).
- A **escala** refaz o tamanho que o nome do modelo pede: `sword_2handed` continua maior que `sword_1handed` em qualquer forma (a razão entre os comprimentos de `WEAPON_FILE_LEN`). A pega por arquivo (`WEAPON_GRIPS`) segue o **nome** do modelo e não mudou.
- Épica e lendária **dividem a silhueta** (a alta); a diferença é só o brilho. Um terceiro modelo para a épica custa mais 6 itens (108 créditos).
- Os arquivos de forma carregam todos de uma vez com `?remaster=1` (18 arquivos de ~105 KB).

### Brilho mais suave

O brilho de raridade e de encantamento (emissivo) cobria a textura pintada das armas do remaster e escondia a forma (a espada rara virava uma mancha azul). No remaster ele vale 12 % do que valia (`SOFT_GLOW`); sem o parâmetro nada mudou.

## Convenção de encaixe

`client/scripts/normalize-weapons.mjs` ganhou as oito entradas novas (e um filtro por regex dos nomes) com o mesmo método: ponta em +Y, cabo na origem, fundo do cabo no Y do KayKit, escudo com a face para +Z e o verso em z = -0,145. Comprimentos (m): sabre 2,0 (entre a espada de uma e a de duas mãos), machado barbado 1,45, cajado de lua 2,13, varinha rara 0,966, varinha alta 1,1, adaga rara 1,206, adaga alta 1,35, escudo pontudo de altura 1,0. Meshy errou o alvo de 760 triângulos em até 5 % e o normalizador simplificou o que passou de 780. Import: `import-asset.mjs ... arma weapons/<nome>_b --no-lod --resize=512 --etc1s`; `check:assets` verde, 1 material e 512 px em todas, 102 a 110 KB cada.

## Conferência no laboratório

`client/lab.html?remaster=1` ganhou `grade` em cada item (`lab.show({items:[{model, right, left, grade}]})`), que chama `setWeapons` como o jogo. As quatro classes seguraram cada forma (graus 1, 3, 4 e 5) nas pegas de `WEAPON_GRIPS`, sem mudar nenhuma:
- `formas-espada-machado-parte2.jpg`: cavaleiro com espada de uma mão e escudo redondo, o mesmo com o escudo de brasão, bárbaro com machado e broquel, bárbaro com machado de duas mãos, cavaleiro com montante. Colunas: comum, rara, épica, lendária.
- `formas-mago-ladino-parte2.jpg`: mago com cajado e com varinha, ladino com duas adagas, nas mesmas quatro colunas.
- `conceitos-parte2.jpg`: os oito conceitos 2D.

A pega aguentou em todas: cabo na palma, lâmina para a frente, escudo no antebraço. A varinha (a menor das armas) é a que menos se lê de longe em qualquer forma.

## Créditos da parte 2

Conceito 2D (`nano-banana`, 3) e imagem para 3D (`meshy-t2.5`, 15) por item, sem repetir nenhuma task.

| Item | Créditos |
|---|---|
| sword_rare, axe_rare, staff_rare, wand_rare, wand_high, dagger_rare, dagger_high | 18 cada (126) |
| shield_rare | 15 (o conceito veio sem cobrança no registro) |
| **Total** | **141** (de 150) |

Prompts em `tools/art/remaster-estaticos/prompts-parte2.mjs`. O saldo de conta cai também pelos outros agentes.

## O que ficou em aberto (parte 2)

1. Forma épica própria (hoje a alta vale para a épica e a lendária) em todas as famílias, e a lendária com algo a mais: 12 itens, 216 créditos.
2. Uma forma boa (grau 2) entre a comum e a rara.
3. Prints no jogo (com servidor) não foram refeitos; o caminho do grau no jogo é o mesmo do laboratório (`entities.ts` passa `weaponGrade` para `setWeapons`).
4. Os ícones dos itens continuam SVG desenhados, sem acompanhar a forma nova.

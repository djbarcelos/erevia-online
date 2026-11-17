# Bosque Gris e Terras Mortas (tunnel#137)

A T10 do roadmap (seções 5.1, 5.2 e 7): o **Bosque Gris** (7–15), o setor noroeste dos anéis 2 e 3 com a Toca do Corvo, e as **Terras Mortas** (24–28), o canto nordeste do mapa, que juntam o núcleo em volta da Cripta e a extensão leste que já tinha entrado com o Brejo. Sem arte nova: os modelos são os do remaster, só com a cor da variante (`Tint`); a medição da Meshy não foi usada (0 créditos).

## O que entrou

- **Mapa:** `client/scripts/biomes/bosque-gris.mjs` (carvalhos velhos a 5 m, a Árvore-Forca, a Carroça Tombada, a Toca da Matilha e a Toca do Corvo com paliçada e 6 tendas), `terras-mortas-nucleo.mjs` (árvores mortas, covas e velas negras em volta da Cripta) e o Bastião Cinzento e a Lápide em `terras-mortas.mjs`. O `world.json` é regerado (`node scripts/build-world.mjs`): o resto do mapa não muda, fora o que a Toca, a Árvore-Forca, a Carroça e a Toca da Matilha limpam do chão da floresta (`world-gen.test.ts`).
- **Regiões e mapa:** o Bosque Gris e o núcleo das Terras Mortas são **setores do anel com dono** (`Spawn` sem monstros, só para os anéis não sortearem lá dentro): o Bosque, de 100° a 170° e de 80 m ao canto do quadrado; as Terras Mortas, de 15° a 75° a partir de 142 m. A extensão leste continua o retângulo do Brejo (`RECT_REGIONS`), com o mesmo nome: as duas metades são **uma região só**, com a mesma LUT, a mesma névoa e a mesma música. O cliente monta as duas a partir dos setores que o servidor manda (`ui/regions.ts`), e o chão mistura as texturas pelo setor (`terrain.ts`, `sectorWeight`).
- **Som (#67):** sem faixas novas. O Bosque Gris usa a música `wild` e o ambiente `forest` (de noite, `night`); as Terras Mortas, a música `crypt` e o ambiente `wind` o dia inteiro. **O Sino Rachado**: nos 10 min de aviso do Ozrhal (`raidWarnBefore`), a Torre das Terras Mortas toca 3 vezes (o som da bigorna uma oitava abaixo, a 2,2 s uma da outra), em volume que cai com a distância até a torre. O aviso reaproveita o anúncio de "em breve" do raid (sem campo novo no protocolo): o cliente o reconhece pelo título (`raidWarnTitle`, `RAID_WARN_TITLE`).
- **Lápide do Último Regicida:** um NPC de conversa ao lado do cemitério (172, 131) que diz quem mais causou dano na última queda do Ozrhal (`BossData.TopName`, carregado do banco ao ligar o servidor e atualizado na morte). O Irmão Caedmon responde a mesma coisa. Sem registro, a pedra está em branco. No cliente é a mesma lápide do túmulo, mas clicar conversa (`LAPIDE_MODEL`).
- **Bastião Cinzento:** o posto avançado em (196, 94) (o ponto do roadmap, (170, 110), caía no cemitério), com torre de pedra, tendas, paliçada baixa, estandartes, a **pedra de teleporte a 260 moedas** e o **Irmão Caedmon**. A #141 deixara o Bastião provisoriamente em `futureStones`, no mapa "minas": agora é a pedra 9 do mapa principal (`teleportStones`), que se descobre a pé como as outras. O id do destino segue a posição + 1 e nada do que existia muda de id; as pedras futuras (Lanterna, Maresia, Vigília e Valdurn) seguem na mesma ordem.

## Monstros e balanceamento

Todos têm papel, então ficam **fora das contas de caça do simulador** (`BalanceBaseline`: só os brutos contam) e são **extras no bestiário** (não travam a conclusão de nenhuma família). A progressão (`docs/progressao.md`) não muda: a caça de cada faixa continua sendo a dos brutos que já existiam. Os números saem das curvas dos esqueletos e do golem da faixa (quanto mais HP, mais XPBase).

| Monstro | Papel | Dia / noite | Modelo | Base |
|---|---|---|---|---|
| Bandoleiro do Corvo (×4) | emboscador (4,5 m, 1º golpe +50%) | dia e noite | `rogue`, `Tint` #d98a7e | Ladino Esqueleto, vida +10% |
| Atirador de Facas (×3) | atirador (7 a 10 m, recua 5 m) | dia e noite | `rogue`, `Tint` #c9a56e | Mago Esqueleto, dano físico |
| Brutamontes do Corvo (×3) | escudo frontal (−60% de frente) | só de dia | `barbarian`, `Tint` #c97a64, escala 1,12 | Guerreiro Esqueleto, +10% |
| Lobo Cinzento (×6, 2 matilhas de 3) | matilha, 4,8 m/s (menos que o jogador) | dia e noite | `wolf`, `Tint` #b4b8bf | |
| Aranha-de-Musgo (×3) | emboscador (4 m), teia que prende 1,5 s | só à noite (troca os Brutamontes) | `spider`, `Tint` #8fa58a | |
| Acólito do Ossário | curandeiro (15% a cada 8 s) | dia e noite | `mage`, `Tint` #8a6aa8 | Mago, 0,9× a vida |
| Necromante Devoto | invocador (2 Lacaios, até 4, a cada 20 s) | dia e noite | `skeleton_mage`, `Tint` #9a78d0 | |
| Cavaleiro do Ossário | elite, escudo frontal, Investida em linha de 8 m | dia e noite | `skeleton_warrior`, `Tint` #8b8a9a, escala 1,3 | 3× a vida do Guerreiro |
| Carniçal (×3, a matilha) | matilha, 5,2 m/s (corre mais), sangra por 4 s | só à noite | `skeleton_minion`, `Tint` #b08a80 | |
| Espectro (×3) | emboscador (5 m), o toque enfraquece o ataque | só à noite | `skeleton_mage`, `Tint` #9cc4d8 | |

- **Quantidade e pico a 70 m** (`TestBosqueBudget` e `TestTerrasMortasBudget`, 12 sementes, de dia e de noite): o Bosque tem **16 monstros** ao mesmo tempo (5 grupos de 12 m, as matilhas de 6 m), pico de 13 a 14 num raio de 70 m; as Terras Mortas, **14** (5 grupos, dois no núcleo e três na extensão, longe da Cripta, do Bastião, da Torre e da lápide), pico de 14. O roadmap previa 22 e 26: o setor e o bloco são pequenos demais para mais que isso sem passar o teto de 14 da área de interesse. Dia e noite somam o mesmo por região (os noturnos trocam os do dia). Todos nascem em chão livre, a menos de 3 m do que o servidor considera andável.
- **Os 9 esqueletos do setor** (anéis 2 e 3 em 100°–170°) mudam de lugar pela exclusão de área do #128, sem mudar de id: o setor sem monstros do Bosque tem dono.
- **Bestiário:** os monstros novos não têm família (o lobo e a aranha ficaram sem família: `BestiaryFamilyOf` só pelo modelo, e as famílias existentes não os cobrem); os de esqueleto são extras da família dos Esqueletos.
- **Nome:** "Cavaleiro do Ossário", e não "Cavaleiro Ossário": o nome já é do elite da Vigília Ossária (33–38).

## O Corvo Rubro (chefe de campo, nível 16)

Na Toca do Corvo (−128, 118), janela de 60 a 90 min, 3 fases (`FieldBossDef`), HP 450 + 75/nível, escala 1,35, `rogue` com `Tint` #d6665c. Saque: 3 a 5 Insígnias do Corvo e 2 a 4 Peles de Lobo garantidas, poções grandes e os épicos do grau D (1,5% cada).

- **F1 (100–60%):** duelista. "Bomba de Fumaça": círculo de 5 m em quem ele ataca, 1,5 s de aviso, cega por 3 s.
- **F2 (60–30%):** apita e chama 4 Bandoleiros. "Chuva de Facas": cone de 90° e 8 m na direção do alvo, 1,5 s.
- **F3 (30–0%):** "Salto": uma marca segue um jogador por 3 s e cai num círculo de 4 m (+40% de dano) em quem estiver perto dele; o aviso dura 3 s.

Não entrou: o chefe ficar sem alvo por 4 s ao subir na paliçada (não há "intocável" no servidor).

## Materiais e receitas

8 materiais novos (`items.go` via `bosqueItems`): **Bosque Gris** — Pele de Lobo e Presa de Lobo (comuns, dos lobos), Seda de Musgo (incomum, só das aranhas, à noite), Insígnia do Corvo (incomum, dos bandidos); **Terras Mortas** — Cinza Sepulcral (comum), Vela Negra (incomum), Tomo Profano (raro, 1,5% a 3,5%) e Fragmento de Selo (só de missão, a 1ª Pedra-Selo). Sem javali: a Presa de Lobo ocupa o lugar da Presa de Javali do roadmap.

6 receitas no Ferreiro (`bosqueRecipes`), tudo dentro do `TestPowerBudget`, sem exceção nova:

| Receita | Resultado | Materiais |
|---|---|---|
| `craft_cape_corvo` | Capa do Crepúsculo (rara, nível 13) | 14 Peles de Lobo, 4 Presas de Lobo, 4 Insígnias do Corvo, 1.000 moedas |
| `craft_gloves_corvo` | Manoplas Rúnicas (raras, nível 13) | 10 Peles de Lobo, 5 Sedas de Musgo, 4 Insígnias do Corvo, 1.000 moedas |
| `craft_staff_devout` | **Cetro Profano** (épico, nível 24, grau C) | 2 Tomos Profanos, 8 Velas Negras, 20 Cinzas Sepulcrais, moedas da loja + 100 |
| `craft_wand_devout` | **Varinha Profana** (épica, nível 24, grau C) | igual ao cetro |
| `craft_ench_weapon_b_tomo` | Pergaminho Abençoado de Arma | 1 Tomo Profano, 8 Cinzas, 1.500 moedas |
| `craft_ench_armor_b_cinza` | Pergaminho Abençoado de Armadura | 12 Cinzas, 4 Velas Negras, 800 moedas |

Por que a capa e as luvas já existem: uma peça nova rara do grau D abriria um degrau novo na escada de poder (`enchant:*` do orçamento) e o teste não pode afrouxar. O Cetro (MAtk 58, MaxMP 120) e a Varinha (MAtk 52, +8% de velocidade de conjuração, MaxMP 100) têm poder 70, abaixo do Cetro do Trono Partido (91,5, a épica de chefe do grau C), então a escada do grau não muda. Os dois são `Graded` (fora do sorteio por raridade) e só saem da receita.

## Missões (#71) e diárias

Cada região tem o seu contador e a sua linha, como dados (`bosque_story.go`), dentro do teto de XP de história por nível (`TestBosqueQuestsFitTheBudget`).

- **Bosque Gris, "A caravana de Caio"** (Hélio, o carroceiro, (−37, 94); nível 9 a 16): o convite, os salteadores (10), o corvo no cinto (8 Insígnias), "Onde está Caio" (a Toca do Corvo e 6 Bandoleiros) e o Corvo Rubro (chefe, 6% de XP).
- **Terras Mortas, "As velas do Ossário"** (Irmão Caedmon, no Bastião, (199, 93); nível 24 a 28, **5 passos**): o convite, as velas (6 Velas Negras), a cinza (10), quem levanta os mortos (12 Necromantes ou Acólitos), o Sino Rachado (a Torre e 6 Cavaleiros) e a primeira Pedra-Selo (o cemitério e 8 Cavaleiros; paga o **Fragmento de Selo**).
- **Diárias** (no fim da lista, sem mudar o sorteio de ninguém): Bosque — "Couro para o inverno" (10 Peles), "Presas para o carroceiro" (6 Presas a Hélio), "Ronda do Bosque" (a Árvore-Forca e a Carroça); Terras Mortas — "Apague as velas negras" (6 Velas), "Cinza para o Bastião" (8 Cinzas a Caedmon), "Ronda das Terras Mortas" (o Bastião e a Torre).
- O "contador de nível" sugerido pelo `HuntPlace` ("chegar ao nível") passou a ignorar as áreas só de bichos de papel: ele manda o jogador para a caça dos brutos da faixa, como antes.

## Bench de contadores

A linha de base (`e2e/bench-baseline.json`) **não foi regerada nesta rodada**: a máquina estava saturada de contêineres de teste de outras tarefas e o SwiftShader estourava os tempos. As regiões **Bosque Gris** (4 pontos pelo setor) e **Terras Mortas** (núcleo e extensão) já entram na volta da câmera (`bench.ts`, `benchPoints` agora percorre setores com nome), então o CI vai apontar linhas novas. Para gravar: subir o servidor com o `client/dist` e rodar `E2E_BASE_URL=... npm run bench:baseline` em `e2e/`, e commitar o diff. O que sobe nos contadores: ~440 objetos de cenário novos (carvalhos, paliçada, tendas, covas) e as malhas dos modelos `wolf` e `spider`, que baixam sob demanda; FPS real não foi medido.

## O que ficou em aberto

- **Raros e chefe de campo das Terras Mortas:** a Presa-Branca (Bosque), a Irmã Vésper e o Malthus (Terras Mortas) não entraram (não estavam no pedido).
- **Uivo do líder da matilha** (chama +1 lobo aos 50%) e o **Mapa Rasgado** (baú sob a Árvore-Forca): não entraram. A matilha é só o puxão do grupo e o frenesi do último vivo.
- **Corvo Rubro na F3:** o "sobe na paliçada e fica sem alvo por 4 s" não existe no servidor; o Salto é a marca em círculo de 4 m.
- **Carniçal e Espectro:** não há sangramento nem queima de mana nos efeitos do servidor: o Carniçal usa o veneno de 4 s e o Espectro, o enfraquecimento do ataque (−15% por 4 s).
- **Bênção da Ordem** (+5% de defesa por 30 min, 200 moedas), **lista de quem matou o Ozrhal** e o **registro de grupos** do Irmão Caedmon, a **zona segura** do Bastião e o evento "O Bastião resiste" (T17): não entraram. O Caedmon dá missões e conversa; a pedra funciona como as outras.
- **Velas Negras no chão** ("Apague 6 Velas Negras" por interação) e o **Altar dos Devotos**: a diária pede Velas Negras do saque.
- **Famílias do bestiário** para o Bando do Corvo, os lobos e as aranhas, e os modelos do estilo B próprios dos humanoides (o `rogue` e o `barbarian` heróis servem de corpo).
- **Árvore-Forca, tendas e paliçada** usam o kit atual (sem malha nova); o pano vermelho dos bandidos é a bandeira azul do kit.

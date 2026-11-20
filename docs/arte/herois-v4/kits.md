# Kits e biblioteca de peças (heróis v4)

Os 41 visuais da progressão e as peças que os montam. Plano em [plano-herois-v4.md](../plano-herois-v4.md); slots, orçamento e encaixes no [contrato](contrato.md); jeito de mover em [movimento.md](movimento.md).

---

## 1. Regras

1. **Um kit = peças da biblioteca + paleta + emblema.** A paleta e o emblema são da maestria; as peças podem se repetir entre kits.
2. **Peça de silhueta.** Todo kit tem pelo menos uma peça **só dele** na cabeça, nos ombros ou na capa. É o que o faz reconhecível a 85 px.
3. **Dupla = 1ª maestria domina, 2ª marca.** A 1ª maestria (a classe do corpo) dá peito, pernas, botas e a forma geral. A 2ª dá **uma** peça de silhueta e o acento de cor e emblema. Por isso as duas ordens de uma dupla têm o mesmo nome e visual diferente.
4. **Progressão visível.** Recruta → classe → evolução → dupla ganha volume e detalhe: o recruta é pano e couro cru; a dupla tem a silhueta mais complexa do jogo.
5. **Raridade é material, não modelo.** Comum: metal sem acabamento, couro gasto. Bom: metal polido e borda colorida. Raro: gravuras e friso emissivo fraco. Épico: runas emissivas, a paleta da maestria acende. Lendário: emissivo forte e uma partícula leve (guia, seção 3.2). A peça de silhueta do raro para cima ganha um detalhe a mais por textura (crista, borda recortada), sem malha nova.
6. **Fonte.** Peças **[M]** saem do Meshy (destaque, forma complexa); **[B]** são modeladas no Blender (simples, sem créditos).

---

## 2. Paletas e emblemas

Tons do clima novo: base dessaturada e gasta, destaque na cor da maestria (a mesma do jogo, `specs.go`), sombra puxada para a paleta e emissivo só no épico e no lendário.

| Maestria | Base | Destaque | Sombra | Emissivo | Emblema |
|---|---|---|---|---|---|
| Recruta | linho `#b9ab8f`, couro `#6b5238` | — | `#3a3128` | — | nenhum |
| Cavaleiro | aço `#8d939b`, couro `#5c4532` | azul `#4a6fa5` | `#2d3340` | — | escudo liso |
| Bárbaro | couro `#7a5233`, pele `#a08260` | vermelho `#9e3b2a` | `#3a2620` | — | marca de garra |
| Mago | lã `#5b4e6e`, couro `#4d3b2c` | âmbar `#c99a45` | `#24202e` | — | estrela de 4 pontas |
| Ladino | couro `#4e463d`, pano `#3f4a45` | carmim `#8c3a3f` | `#1f2224` | — | adaga |
| Bastião | aço frio `#7f8a99`, couro `#4f3d2e` | azul `#8fb4ff` | `#28303d` | azul `#9fc4ff` | torre |
| Paladino | aço claro `#a9a49a`, pano `#e6dcc4` | ouro `#ffd86a` | `#3d3426` | ouro `#ffe7a0` | sol nascente |
| Berserker | couro escuro `#5e3a28`, osso `#d8ccb0` | sangue `#e85050` | `#2e1a18` | brasa `#ff6a4a` | gota com garras |
| Senhor da Guerra | bronze `#8a6a3e`, couro `#6b4a2c` | ouro velho `#e0a848` | `#33261a` | âmbar `#ffc060` | machados cruzados sob coroa |
| Elementalista | lã `#5a4a5e`, couro `#4a3426` | laranja `#ff9a3c` e gelo `#6cc4f0` | `#241c26` | os dois | círculo meio fogo, meio gelo |
| Arcanista | lã `#3e3348`, couro `#2f2620` | roxo `#b46ae0`, verde-peste `#6ad06a` | `#16121c` | roxo `#c88cff` | olho do abismo |
| Assassino | couro `#2e2a2c`, pano `#3a2f3a` | carmim `#e0607a` | `#141215` | violeta `#8c7ae0` | lua e adaga |
| Patrulheiro | couro `#5a4a36`, pano `#4a5a3a` | verde `#8cc850` | `#1f2a1c` | verde `#b8f080` | cabeça de lobo |

---

## 3. Biblioteca de peças

`id` em `slot.nome`. A coluna "Kits" diz quem usa (ver seções 4 e 5).

### Peito
| id | Peça | Fonte | Kits |
|---|---|---|---|
| `peito.camisa` | camisa de linho com colete de couro | B | recruta |
| `peito.cota` | cota de malha com sobreveste curta | M | cavaleiro, guardião rúnico, vigia sombrio |
| `peito.placa` | peitoral de placas com faixas | M | bastião, marechal, colosso, guarda-fronteira |
| `peito.placa_sagrada` | peitoral de placa com tabardo branco e sol | M | paladino, cruzado, zelote, campeão |
| `peito.couro_cruzado` | tiras de couro cruzadas no peito nu | B | bárbaro, berserker, carniceiro, predador |
| `peito.couraça_bronze` | couraça de bronze sobre gibão | M | senhor da guerra, chefe de clã, mercenário, desbravador |
| `peito.manto` | túnica longa de lã com cinto alto | M | mago, elementalista, arquimago, teurgo, senhor da tormenta |
| `peito.manto_rasgado` | túnica escura em tiras, com amuletos | M | arcanista, possesso (Mago), tirano (Mago), pestilento (Mago), bruxo (Mago), exorcista (Mago) |
| `peito.gibão` | gibão de couro justo com capuz caído | B | ladino, assassino, espreitador, lâmina elemental (Ladino) |
| `peito.couro_mata` | couro de caça com placas leves e faixa | M | patrulheiro, arqueiro rúnico (Ladino), caçador de bruxas (Ladino) |

### Ombros
| id | Peça | Fonte | Kits |
|---|---|---|---|
| `ombros.couro` | ombreiras de couro simples | B | recruta, ladino |
| `ombros.placa_redonda` | ombreiras de placa arredondadas | M | cavaleiro, marechal |
| `ombros.torre` | ombreiras altas em degrau, como ameias | M | **bastião** (silhueta) |
| `ombros.asas` | ombreiras com abas em forma de asa | M | **paladino** (silhueta), cruzado |
| `ombros.pele_urso` | pele de urso com garras pendentes | M | bárbaro, berserker |
| `ombros.cranio` | ombreira única de crânio com chifre | M | **berserker** (silhueta), carniceiro, colosso |
| `ombros.bronze_franja` | ombreiras de bronze com franja de couro | M | senhor da guerra, chefe de clã |
| `ombros.gola_alta` | gola alta de lã rígida | B | mago, elementalista |
| `ombros.gola_cristal` | gola com cristais de gelo e brasas | M | **elementalista** (silhueta) |
| `ombros.correntes` | correntes que cruzam os ombros, com cadeados | M | **arcanista** (silhueta), possesso, guardião do véu, tirano |
| `ombros.assimetrica` | ombreira única de couro, só no braço da adaga | B | **assassino** (silhueta) |
| `ombros.pena` | manto curto de penas nos ombros | M | **patrulheiro** (silhueta), predador, desbravador |

### Cabeça
| id | Peça | Fonte | Kits |
|---|---|---|---|
| `cabeca.nenhuma` | cabeça nua (cabelo à mostra) | — | recruta, bárbaro, berserker |
| `cabeca.elmo_aberto` | elmo de metal com protetor de nariz | M | cavaleiro |
| `cabeca.elmo_balde` | elmo de balde, topo plano, fenda em cruz | M | **bastião** |
| `cabeca.elmo_alado` | elmo com asas pequenas e crista de sol | M | paladino, **cruzado** (silhueta) |
| `cabeca.elmo_chifre` | elmo de bronze com chifres curtos | M | **senhor da guerra**, chefe de clã |
| `cabeca.chapeu` | chapéu pontudo de aba larga | M | mago |
| `cabeca.capuz_alto` | capuz alto e pontudo de lã | B | elementalista, arcanista, bruxo |
| `cabeca.mascara_abismo` | máscara de couro com um olho só, roxo | M | **possesso** (silhueta), **exorcista** |
| `cabeca.capuz` | capuz de couro com máscara de pano | B | ladino, assassino |
| `cabeca.capuz_lobo` | capuz com cabeça de lobo | M | **predador**, **desbravador** |
| `cabeca.coroa_ferro` | coroa de ferro sobre capuz | M | **tirano** |

### Capa
| id | Peça | Fonte | Kits |
|---|---|---|---|
| `capa.curta` | capa curta de lã | B | cavaleiro, mago, ladino |
| `capa.tabardo` | tabardo longo com emblema | B | bastião, paladino, marechal |
| `capa.manto_sol` | manto branco com sol bordado, até o chão | M | **cruzado**, campeão |
| `capa.pele` | pele de lobo nas costas | B | bárbaro, senhor da guerra |
| `capa.estandarte` | estandarte preso às costas | M | **marechal**, **chefe de clã**, **campeão** |
| `capa.tiras` | capa em tiras soltas | B | assassino, espreitador, vigia sombrio |
| `capa.fumaça` | capa em tiras com bordas que viram fumaça (emissivo no épico) | M | **bruxo**, **vigia sombrio** |
| `capa.camuflagem` | capa de folhas e redes | B | patrulheiro, arqueiro rúnico |

### Demais slots
| id | Peça | Fonte | Kits |
|---|---|---|---|
| `bracos.luva_couro` / `bracos.braçal_placa` / `bracos.faixas` / `bracos.braçal_runa` | luvas, braçais de placa, faixas de pano, braçal com runas | B | por kit |
| `cintura.cinto` / `cintura.saiote_placa` / `cintura.cinto_bolsas` / `cintura.tabardo_curto` | | B | por kit |
| `pernas.calca` / `pernas.perneira_placa` / `pernas.calca_couro` / `pernas.barra_manto` | | B | por kit |
| `botas.couro` / `botas.greva` / `botas.pele` / `botas.macia` | | B | por kit |
| `acessorio.livro` | grimório preso ao cinto por corrente | M | mago, arcanista, arquimago, bruxo |
| `acessorio.orbe_dupla` | duas pedras flutuantes (fogo e gelo) atrás do ombro | M | **arquimago**, **senhor da tormenta** |
| `acessorio.aljava` | aljava nas costas | B | patrulheiro, arqueiro rúnico, caçador de bruxas |
| `acessorio.relicario` | relicário pendurado no peito | B | paladino, exorcista, penitente |
| `acessorio.correntes_cinto` | correntes penduradas na cintura | B | possesso, carniceiro, tirano |
| `acessorio.frascos` | frascos de veneno no cinto | B | pestilento, assassino |

**Conta:** ~45 peças grandes (≈ 30 [M]) e ~20 peças simples [B].

---

## 4. Kits de recruta, classe e evolução

| Kit | Silhueta | Peças | Notas |
|---|---|---|---|
| **Recruta** | nenhuma: é o "antes" | camisa, ombros de couro, calça, botas de couro, cinto | arma de treino de madeira |
| **Cavaleiro** | escudo grande + elmo aberto | cota, ombros de placa redonda, elmo aberto, capa curta, perneiras | o bloco vertical do guia |
| **Bárbaro** | triângulo invertido, peito à mostra | couro cruzado, pele de urso, capa de pele, calça de couro, botas de pele | sem elmo |
| **Mago** | chapéu pontudo + manto até o chão | manto, gola alta, chapéu, capa curta, livro | a barra larga do guia |
| **Ladino** | capuz + capa curta | gibão, ombros de couro, capuz, capa curta | fino e inclinado |
| **Bastião** | **ombreiras em ameia** + elmo de balde | placa, ombros torre, elmo balde, tabardo, perneiras, grevas | o mais largo do jogo |
| **Paladino** | **ombreiras em asa** + tabardo branco | placa sagrada, ombros asas, elmo alado, tabardo, relicário | brilho do sol no épico |
| **Berserker** | **ombreira única de crânio** | couro cruzado, ombro de crânio + pele de urso, capa de pele, faixas nos braços | cicatrizes pintadas, pele exposta |
| **Senhor da Guerra** | **elmo com chifres** + franja | couraça de bronze, ombros com franja, elmo de chifre, capa de pele | |
| **Elementalista** | **gola de cristais** | manto, gola de cristal, capuz alto, livro | metade quente, metade fria na barra |
| **Arcanista** | **correntes nos ombros** | manto rasgado, correntes, capuz alto, livro | runas acendem no épico |
| **Assassino** | **ombreira assimétrica** + capa em tiras | gibão, ombreira assimétrica, capuz, capa em tiras, frascos | quase sem metal |
| **Patrulheiro** | **manto de penas** + aljava | couro de mata, penas, capa de camuflagem, aljava | |

---

## 5. As 28 duplas

Cada linha descreve as duas ordens. **Corpo** = a classe da 1ª maestria. A peça em **negrito** é a de silhueta da dupla. O "movimento" é o detalhe de clipe da dupla ([movimento.md](movimento.md), seção 8).

| Dupla | 1ª → 2ª: o que se vê | Movimento |
|---|---|---|
| **Cruzado** (Bastião + Paladino) | Bastião → Paladino: placa e ombros em ameia, com **manto branco com sol** e elmo alado. Paladino → Bastião: placa sagrada e asas, com o **elmo de balde** e o escudo-torre | ergue espada e escudo juntos no buff |
| **Colosso** (Bastião + Berserker) | Bastião → Berserker: placa de bastião com **ombreira de crânio** e sangue no tabardo. Berserker → Bastião: peito nu e pele de urso, com **um ombro em ameia** e escudo de torre amarrado às costas | bate o escudo no chão no parado |
| **Marechal** (Bastião + Senhor da Guerra) | Bastião → SG: placa e ameias com **estandarte nas costas**. SG → Bastião: couraça de bronze com **elmo de balde** e tabardo azul | aponta com a espada ao dar ordens |
| **Guardião Rúnico** (Bastião + Elementalista) | Bastião → Elem.: placa com **gola de cristais** e escudo com runas de fogo e gelo. Elem. → Bastião: manto com **ombreiras em ameia** de pedra rúnica | o escudo solta faíscas ao bloquear |
| **Guardião do Véu** (Bastião + Arcanista) | Bastião → Arcanista: placa com **correntes nos ombros** e cadeado no escudo. Arcanista → Bastião: manto rasgado com **elmo de balde** e escudo pequeno | correntes tilintam no parado |
| **Vigia Sombrio** (Bastião + Assassino) | Bastião → Assassino: cota escura com **capa de fumaça**. Assassino → Bastião: gibão com **um ombro em ameia** e escudo pequeno | olha em volta, lento, no parado |
| **Guarda-Fronteira** (Bastião + Patrulheiro) | Bastião → Patrulheiro: placa com **manto de penas** e aljava. Patrulheiro → Bastião: couro de mata com **elmo de balde** e escudo nas costas | mão na testa, olhando o horizonte |
| **Zelote** (Paladino + Berserker) | Paladino → Berserker: placa sagrada com **ombreira de crânio** e tabardo manchado. Berserker → Paladino: peito nu com **asas de placa** e relicário no peito | treme de fervor no parado |
| **Campeão** (Paladino + Senhor da Guerra) | Paladino → SG: placa sagrada com **estandarte do sol**. SG → Paladino: couraça de bronze com **manto branco com sol** | ergue a arma ao céu no buff |
| **Teurgo** (Paladino + Elementalista) | Paladino → Elem.: placa sagrada com **gola de cristais** e luz quente. Elem. → Paladino: manto com **asas de placa** douradas | mãos se juntam em prece antes do feitiço |
| **Exorcista** (Paladino + Arcanista) | Paladino → Arcanista: placa sagrada com **máscara de um olho** e correntes. Arcanista → Paladino: manto rasgado com **relicário e ombreiras em asa** | segura o relicário no parado |
| **Penitente** (Paladino + Assassino) | Paladino → Assassino: placa leve com **capuz e capa em tiras** brancas. Assassino → Paladino: gibão branco com **asa de placa** num ombro só | ajoelha e se levanta na pausa |
| **Caçador de Bruxas** (Paladino + Patrulheiro) | Paladino → Patrulheiro: placa sagrada com **chapéu de aba larga** e besta. Patrulheiro → Paladino: couro de mata com **tabardo do sol** e relicário | recarrega a besta com calma |
| **Chefe de Clã** (Berserker + Senhor da Guerra) | Berserker → SG: peito nu com **estandarte de ossos**. SG → Berserker: couraça de bronze com **ombreira de crânio** e chifres maiores | soca o peito e grita na pausa |
| **Incendiário** (Berserker + Elementalista) | Berserker → Elem.: peito nu com **brasas na pele e machado em chamas**. Elem. → Berserker: manto queimado com **ombreira de crânio** | soltam faíscas dos punhos no parado |
| **Possesso** (Berserker + Arcanista) | Berserker → Arcanista: couro e machado com **correntes e máscara de um olho**. Arcanista → Berserker: manto rasgado por garras, **ombreira de crânio** e sangue | tique de cabeça, ombros puxados por correntes |
| **Carniceiro** (Berserker + Assassino) | Berserker → Assassino: peito nu com **capuz de carrasco** e cutelos. Assassino → Berserker: gibão com **ombreira de crânio** e correntes na cintura | gira as lâminas devagar |
| **Predador** (Berserker + Patrulheiro) | Berserker → Patrulheiro: peito nu com **capuz de lobo**. Patrulheiro → Berserker: couro de mata com **pele de urso e crânio** | fareja o ar na pausa |
| **Senhor da Tormenta** (SG + Elementalista) | SG → Elem.: couraça de bronze com **orbes de fogo e gelo** atrás dos ombros. Elem. → SG: manto com **elmo de chifre** e estandarte de tempestade | ergue o machado e chama o raio no buff |
| **Tirano** (SG + Arcanista) | SG → Arcanista: couraça de bronze com **coroa de ferro** e correntes. Arcanista → SG: manto rasgado com **coroa de ferro** e cetro | olha de cima, imóvel, no parado |
| **Mercenário** (SG + Assassino) | SG → Assassino: couraça de bronze leve com **capuz e bolsas de moedas**. Assassino → SG: gibão com **ombreiras de bronze com franja** | joga uma moeda para cima na pausa |
| **Desbravador** (SG + Patrulheiro) | SG → Patrulheiro: couraça de bronze com **capuz de lobo** e estandarte curto. Patrulheiro → SG: couro de mata com **elmo de chifre** | aponta o caminho com o machado |
| **Arquimago** (Elementalista + Arcanista) | Elem. → Arcanista: manto com **orbes duplas** e correntes de runas. Arcanista → Elem.: manto rasgado com **gola de cristais** | o livro folheia sozinho no ar |
| **Lâmina Elemental** (Elementalista + Assassino) | Elem. → Assassino: manto curto com **adagas de fogo e gelo** na cintura e capa em tiras. Assassino → Elem.: gibão com **gola de cristais** | gira as adagas e elas trocam de elemento |
| **Arqueiro Rúnico** (Elementalista + Patrulheiro) | Elem. → Patrulheiro: manto com **aljava de flechas rúnicas** e penas. Patrulheiro → Elem.: couro de mata com **gola de cristais** e arco rúnico | acende a ponta da flecha antes de atirar |
| **Bruxo** (Arcanista + Assassino) | Arcanista → Assassino: manto rasgado com **capa de fumaça** e adaga roxa. Assassino → Arcanista: gibão com **correntes nos ombros** e capuz alto | gira a adaga e ela acende de roxo |
| **Pestilento** (Arcanista + Patrulheiro) | Arcanista → Patrulheiro: manto com **máscara de bico** e frascos. Patrulheiro → Arcanista: couro de mata com **correntes e nuvem verde** na aljava | tosse e abana a fumaça na pausa |
| **Espreitador** (Assassino + Patrulheiro) | Assassino → Patrulheiro: gibão com **capuz de lobo** e arco curto. Patrulheiro → Assassino: couro de mata com **capa de fumaça** | se agacha e some no mato na pausa |

Peças citadas aqui que ainda não estão na seção 3 (chapéu de aba larga, capuz de carrasco, máscara de bico, bolsas de moedas, adagas e machado elementais, aljava rúnica, cetro) entram na biblioteca na Fase 1, quando os conceitos das duplas forem aprovados.

---

## 6. Conferência na Fase 1

Cada kit passa no teste de silhueta antes de ir para o 3D:
- os 13 kits de recruta, classe e evolução em preto, lado a lado, a 85 px: todos distinguíveis;
- as duas ordens de cada dupla lado a lado: dá para ver qual é a 1ª maestria;
- cada dupla ao lado das suas duas evoluções: dá para ver que é a mistura das duas.

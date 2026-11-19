# Evoluções de classe: regras gerais

Este é o rascunho das regras que valem para as 8 evoluções. Ele vem do [documento de design](design.md) e do [plano de implementação](../plano-classes.md), e segue o estado do código na main de 19/11. A frente é a tunnel#180.

Cada evolução tem um doc próprio nesta pasta. **O DIogo revisa cada doc antes de a evolução virar código.** Os números são propostas: ficam calibrados no simulador (`server/cmd/balance`) quando a evolução existir no código, e o resultado da calibragem volta para o doc.

| Classe | Evolução (ID) | Recurso | Doc | Situação |
|---|---|---|---|---|
| Cavaleiro | Bastião (`bastion`) | Determinação | [bastion.md](bastion.md) | aprovado (19/11) |
| Cavaleiro | Paladino (`herald`) | Fé | [herald.md](herald.md) | aprovado (19/11) |
| Bárbaro | Berserker (`bloodrager`) | Fúria | [bloodrager.md](bloodrager.md) | rascunho para revisar |
| Bárbaro | Senhor da Guerra (`warlord`) | Ímpeto | [warlord.md](warlord.md) | rascunho para revisar |
| Mago | Elementalista (`elementalist`) | Concentração | `elementalist.md` | a fazer |
| Mago | Arcanista (`occultist`) | Carga Arcana | `occultist.md` | a fazer |
| Ladino | Assassino (`cutthroat`) | Combo | `cutthroat.md` | a fazer |
| Ladino | Patrulheiro (`hunter`) | Precisão | `hunter.md` | a fazer |

Os IDs das evoluções são os das especializações de hoje (`spec` no banco e no protocolo) e não mudam. Mudam só os nomes exibidos: Arauto → Paladino, Sanguinário → Berserker, Caudilho → Senhor da Guerra, Ocultista → Arcanista, Sicário → Assassino, Caçador → Patrulheiro. Bastião e Elementalista continuam com o mesmo nome.

---

## 1. Antes e depois do nível 20

| | Classe-base (1–19) | Evolução (20+) |
|---|---|---|
| Árvore | 3 ramos, degraus **1/5/10/15**. O degrau 20 de hoje sai da base. | 3 ramos próprios, marcos **20/25/30/35/40** (45–60 chegam com o teto 60). |
| Pontos | `min(nível, 19) + 1` (20 pontos no 19) | `nível + 1` até o teto: 21 no 20, 41 no 40, 61 no 60. |
| Ao evoluir | A árvore base é apagada e todos os pontos passam para a árvore da evolução. | Recebe 2 ativas iniciais com rank 1 e a hotbar é refeita. |
| Recurso | Só MP. | MP **e** o recurso da evolução. |
| Armas | As da classe (`ClassDef.Weapons`). | As da evolução (`SpecDef.Weapons`/`Offhands`). |

**Por que os pontos passam a ir até o teto:** é a decisão 33 da tunnel#180, que substitui o `SkillPointMaxLevel = 30` da tunnel#153. Os marcos 35 e 40 precisam de pontos depois do 30.

### Skills da base dentro da evolução

O design reaproveita skills da base nas árvores evoluídas, por exemplo a Bola de Fogo no Elementalista e o Golpe de Escudo no Bastião. Regra:

- A versão da evolução é **um nó novo, com ID novo** e o mesmo nome, e tem o campo `EvolvedFrom` apontando para a da base. O efeito pode mudar, e os números são os do doc da evolução.
- Ao evoluir, a hotbar troca `skill:<base>` por `skill:<evoluída>`. A tecla do jogador continua no mesmo lugar.
- As skills do degrau 20 de hoje (IDs 49–60) e as de especialização (61–84) **mantêm o ID** e passam a ser nós da evolução. Assim, os ranks e as hotbars de quem já tem spec sobrevivem à migração.

---

## 2. Forma da árvore evoluída (marcos 20–40)

### Tipos de nó e ranks

| Tipo | Ranks | Ocupa a barra? | Observação |
|---|---|---|---|
| Ativa | até 3 | sim | |
| Passiva | até 5 | não | Passiva por tipo (`PassiveKind`), nunca por ID. |
| Modificador | até 3 | não | Altera uma ativa já aprendida (`Modifies`); exige rank 1 nela. |
| Chave | 1 | depende | Uma por ramo, no marco 40. Muda a forma de jogar o ramo. |
| Suprema | 1 | sim | Marco 60, chega com o teto 60. |

### Molde de cada ramo

| Marco | Nós | Ranks | Requisito |
|---|---|---|---|
| 20 | 1 ou 2 ativas + 1 passiva | 8–11 | — (as 2 iniciais da evolução ficam aqui) |
| 25 | 1 modificador de uma ativa do 20 | 3 | rank 1 na ativa |
| 30 | 1 ativa ou passiva nova | 3–5 | 5 pontos no ramo |
| 35 | 1 modificador ou ativa de impacto | 3 | 10 pontos no ramo |
| 40 | 1 chave | 1 | 15 pontos no ramo |

### Orçamento

- Alvo: cerca de **60–64 ranks** na árvore do 20 ao 40, contra **41 pontos** no 40. É cerca de 1,5×, e o teste exige no mínimo 1,4×.
- Com o requisito de 15 pontos no ramo para a chave, dá para fechar **2 chaves no máximo**. É preciso escolher.
- No marco 20, a árvore precisa aceitar pelo menos **26 ranks** (21 pontos no 20, mais folga), para os pontos devolvidos não ficarem parados até o 25.
- Hotbar: cada evolução tem **8–9 ativas** do 20 ao 40. A barra tem 10 teclas, das quais 4 costumam ir para poções.

---

## 3. Recursos de classe

Todos os recursos seguem as mesmas regras:

| Regra | Valor padrão | Por quê |
|---|---|---|
| Máximo | **100** | Uma escala só, fácil de ler na barra. |
| Começa em | 0 (login, troca de zona, arena e troca de subclasse) | O recurso não é salvo no banco. |
| Teto de ganho por segundo | **25/s** | Evita multiplicar o ganho com muitos alvos fracos. |
| Teto de alvos por lançamento | **3** | Área em 10 monstros gera o mesmo que em 3. |
| Não gera | DoT, reflexo, alvo invulnerável, dano ≤ 0, erro | Regra do design (5.3). Um teste fixa isso. |
| Fora de combate | Cai **10/s** depois de 5 s sem combate | Não se entra cheio na próxima luta. |
| Sem recurso | O ciclo básico funciona: as 2 iniciais custam só MP | "O jogador deve conseguir manter um ciclo básico mesmo sem recurso." |

**Como cada skill usa o recurso:**
- **Geradoras:** custam MP e geram recurso.
- **Gastadoras:** custam recurso, e às vezes um pouco de MP.
- **Gastar tudo** (`ResSpendAll`): o efeito escala com o que foi gasto, até 100.

A barra do recurso fica abaixo da de MP, na cor da evolução. A dica mostra quanto cada skill gera e quanto gasta.

---

## 4. Mana

- O Mago passa a regenerar **0,75×** a mana de hoje (`ClassDef.MPRegenMult`). Ele continua decidindo quando conjurar, e Mente Clara vira eficiência de mana (−custo, até 25%).
- O kit base do Bárbaro não pode ficar sem mana numa luta de 60 s no simulador. Do 20 em diante, o ciclo principal do Berserker usa Fúria, não MP.
- Fora de combate, a regeneração continua 3,3× maior.

---

## 5. Armas por evolução

São os tipos de arma de `game/weapons.go`. Uma lista vazia herda a da classe.

| Evolução | Armas | Mão secundária | Observação |
|---|---|---|---|
| Bastião | espada | **escudo** | O escudo é a identidade (bloqueio gera Determinação). |
| Paladino | espada | escudo | Foco sagrado só no futuro. |
| Berserker | machado 2H, machado | — | Sem escudo. |
| Senhor da Guerra | machado, machado 2H | escudo (opcional) | Com escudo, mantém as skills centrais (decisão pendente 9). |
| Elementalista | cajado, varinha | grimório | Grimório: só atributos, sem regras próprias de combate. |
| Arcanista | cajado, varinha | grimório | Idem. |
| Assassino | adagas | — | |
| Patrulheiro | arco, besta, besta pesada | — | Depende do PR C (armas no cliente). |

Ao evoluir, ou ao trocar de evolução, o que a evolução não usa vai para a bolsa (`dropInvalidGear`). Com a bolsa cheia, a troca é recusada. A evolução entrega uma vez a **arma inicial** que a classe-base não usa (arco do Patrulheiro, machado 2H do Berserker, grimório dos magos).

---

## 6. Trocar e redistribuir

| Ação | Onde | Custo proposto | Hoje |
|---|---|---|---|
| Escolher a evolução (1ª vez) | Ysolde, missão A Encruzilhada | grátis | igual |
| Trocar de evolução | Ysolde | **2.500 × nível** de ouro (50 mil no 20, 100 mil no 40) | 50 mil fixo |
| Redistribuir a árvore | Ysolde | **500 × nível** de ouro (10 mil no 20, 20 mil no 40) | não existe |
| Redistribuição grátis | uma por classe, depois da migração | grátis | — |

Os valores devem ser conferidos com o painel de economia (ouro por hora em cada faixa) antes de fechar.

---

## 7. Migração de quem já jogou

Ela roda no load, por classe (incluindo a subclasse), e só uma vez, controlada pela marca `tree` no `ClassProgress` e pela migração 00062.

| Personagem | O que acontece |
|---|---|
| < 20 | Nada muda. |
| ≥ 20 sem spec | Perde o nó do degrau 20 (os pontos voltam). A Encruzilhada aparece no diário. |
| ≥ 20 com spec | Mantém a evolução e os ranks dos IDs 49–84 que ficaram na árvore dela, desde que o marco do nó seja ≤ o nível do personagem. Se uma skill de spec subiu de marco (por exemplo, a Carnificina do 28 para o 35), ela sai até o personagem chegar lá, e os pontos voltam. Descarta a base, recebe as iniciais e a hotbar troca base → evoluída. Ganha a redistribuição grátis e uma mensagem única: "Você tem N pontos para distribuir na árvore da sua evolução". |

---

## 8. Mapa das skills de hoje

O degrau 20 e as skills de especialização mudam de dono assim:

| ID | Skill de hoje | Vai para |
|---|---|---|
| 49 | Baluarte | Bastião (Provocador) |
| 50 | Corte do Horizonte | Paladino (Julgamento) |
| 51 | Égide da Ordem | Bastião (Protetor) |
| 52 | Machadada Sísmica | Berserker (Carnificina) |
| 53 | Redemoinho Sangrento | Berserker (Frenesi) |
| 54 | Pele de Pedra | Senhor da Guerra (Domínio) |
| 55 | Inferno | Elementalista (Chamas) |
| 56 | Zero Absoluto | Elementalista (Geada) |
| 57 | Rajada Arcana | Arcanista (Arcano) |
| 58 | Golpe Fatal | Assassino (Lâminas) |
| 59 | Golpe das Sombras | Assassino (Sombras) |
| 60 | Miasma | Patrulheiro (Armadilhas) |
| 61–63 | Desafio, Vigor e Último Bastião | Bastião |
| 64–66 | Martelo da Aurora, Bênção do Arauto, Sol Nascente | Paladino |
| 67–69 | Golpe Rubro, Fúria Cega, Carnificina | Berserker |
| 70–72 | Brado, Avanço e Estandarte do Caudilho | Senhor da Guerra |
| 73–75 | Lança Ígnea, Foco Elemental, Cataclismo | Elementalista |
| 76–78 | Peste Sombria, Drenar Vontade, Marca do Abismo | Arcanista |
| 79–81 | Lâmina Oculta, Instinto Assassino, Sentença | Assassino |
| 82–84 | Faca de Arremesso, Rede de Caça, Chuva de Lâminas | Patrulheiro |

O Miasma (60) é do Ladino e vai para o **Patrulheiro** (decisão do DIogo em 19/11). O Arcanista não pode recebê-lo, porque é do Mago.

IDs novos entram no fim do iota, a partir do **85**.

---

## 9. Linha de base do simulador (main de 19/11)

Tempo para matar no regime (+0). Entre parênteses está a distância para a média da faixa, que o teste aceita com tolerância de ±15%.

| Caminho | 20–24 | 25–29 | 30–34 | 35–40 |
|---|---|---|---|---|
| Cavaleiro/Bastião | 6,1 s (+86%) | 5,9 s (+100%) | 6,2 s (+98%) | 4,7 s (+89%) |
| Cavaleiro/Arauto | 4,8 s (+44%) | 4,5 s (+53%) | 5,2 s (+64%) | 3,9 s (+59%) |
| Bárbaro/Sanguinário | 3,1 s (−7%) | 2,4 s (−18%) | 2,2 s (−29%) | 2,0 s (−19%) |
| Bárbaro/Caudilho | 3,6 s (+9%) | 2,8 s (−6%) | 2,7 s (−15%) | 2,3 s (−8%) |
| Mago/Elementalista | 1,6 s (−52%) | 1,2 s (−60%) | 1,6 s (−48%) | 1,2 s (−51%) |
| Mago/Ocultista | 2,0 s (−41%) | 2,0 s (−33%) | 1,9 s (−39%) | 1,4 s (−42%) |
| Ladino/Sicário | 2,7 s (−18%) | 2,3 s (−23%) | 2,5 s (−19%) | 2,0 s (−19%) |
| Ladino/Caçador | 2,6 s (−21%) | 2,6 s (−14%) | 2,7 s (−13%) | 2,2 s (−10%) |

**Metas de cada evolução no simulador:**
1. No 20, a evolução tem DPS e vida efetiva **≥** a build base com spec de hoje. Ninguém fica mais fraco ao evoluir.
2. Os tanques e suportes (Bastião, Paladino) podem matar mais devagar que a média, mas devem morrer bem menos. A medida de tempo para morrer vem da tunnel#152.
3. O Mago está 40–60% abaixo da média (mata rápido demais). A Fase 8 (mana) e as árvores do Elementalista e do Arcanista devem trazê-lo para **−25% ou mais perto da média**.

---

## 10. Roteiro de cada doc

Todos os docs seguem o mesmo molde:
1. Papel e identidade (do design).
2. Armas.
3. Recurso: como gera, quanto, como gasta.
4. Árvore: 3 ramos × marcos 20–40, com tipo, ranks, efeito por rank, custo (MP/recurso) e recarga.
5. Iniciais e hotbar ao evoluir.
6. Linha de base e metas no simulador.
7. Mecânicas novas que o código precisa (efeitos, passivas, ganchos).
8. Perguntas para o DIogo.

| Doc | Ramos | Origem das skills | Mecânica nova que o código precisa |
|---|---|---|---|
| bastion | Muralha · Protetor · Provocador | 49, 51, 61–63 e Golpe de Escudo e Muralha Viva evoluídos | Bloqueio (Fase 4), interceptar dano de aliado, zona de provocação |
| herald | Julgamento · Bênçãos · Expiação | 50, 64–66 e Aura de Coragem evoluída | Cura limitada (direta, pequena), purificar, sacrifício |
| bloodrager | Carnificina · Frenesi · Execução | 52, 53, 67–69 e Golpe Brutal, Frenesi, Execução e Sede de Sangue evoluídos | Gastar tudo em Fúria, marca da presa |
| warlord | Comando · Investida · Domínio | 54, 70–72 e Rugido e Pisotear evoluídos | Interrupção, ganho de Ímpeto ao iniciar combate |
| elementalist | Chamas · Geada · Convergência | 55, 56, 73–75 e Bola de Fogo, Nova de Gelo, Lança de Gelo, Brasa Viva e Prisão Glacial evoluídas | Reação elemental (queimado + congelado), Concentração por skills distintas |
| occultist | Maldições · Pragas · Arcano | 57, 76–78 e Praga, Piscar e Escudo Arcano evoluídos (o Praga do Ladino vira skill nova do Mago) | Carga por efeitos distintos no alvo, prisão etérea |
| cutthroat | Lâminas · Sombras · Execução | 58, 59, 79–81 e Golpe Furtivo, Lâminas Gêmeas, Passo Sombrio, Sumir e Marca da Morte evoluídos | Combo por sequência, bônus pelas costas |
| hunter | Arqueria · Armadilhas · Caçada | 60, 82–84 e Bomba de Fumaça evoluída | Arco e besta, Precisão por distância e acertos seguidos, armadilha no chão |

**Ordem de escrita:** um par por vez, e o próximo par só depois de o DIogo revisar o anterior. Os dois docs do par saem juntos para ficarem equilibrados entre si. A ordem é Cavaleiro → Bárbaro → Mago → Ladino, a mesma do código.

---

## 11. Decisões pendentes do design (com o padrão proposto)

| # | Questão | Proposta |
|---|---|---|
| 1 | Custo para trocar de evolução | 2.500 × nível em ouro (seção 6) |
| 2 | Pontos por nível | 1 por nível até o teto (decidido, decisão 33) |
| 3 | Pontos extras por missão de classe | nenhum por enquanto |
| 4 | Nós e ranks por árvore | 60–64 ranks do 20 ao 40 (seção 2) |
| 5 | Máximo, geração e decaimento dos recursos | regras da seção 3; números em cada doc |
| 6 | O que gasta MP e o que gasta recurso | iniciais e geradoras gastam MP; impacto e chaves gastam recurso |
| 7 | Números das armas | os de hoje; arco e besta calibrados no doc do Patrulheiro |
| 8 | Paladino cura? | Cura direta pequena, com recarga; não é curandeiro |
| 9 | Senhor da Guerra com escudo | mantém as skills centrais |
| 10 | Grimório | foco secundário, só atributos |
| 11 | Chefes e controle | usam a resistência a controle que já existe; execução com teto de +50% em chefe |
| 12 | Maestria depois do 100 | fora deste ciclo |
| — | Escudo bloqueia magia? | Não |

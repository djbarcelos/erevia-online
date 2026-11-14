# Balanceamento — novembro de 2026

A foto do jogo de hoje (main 33b8c84) tirada pelo simulador `server/cmd/balance` (tunnel#148). As tabelas abaixo são a saída de `go run ./cmd/balance` (em `server/`). Tudo é média por fórmula: rodar de novo dá os mesmos números. A versão em CSV sai com `-csv`, e a de noite com `-noite`.

O teste `internal/balance` (`TestPowerBudget`, no `go test ./...` do CI) cobra o orçamento de poder. Os casos que o jogo de hoje já fura estão na lista de exceções conhecidas, com o número de hoje: o teste passa agora e falha se aparecer um caso novo ou se um conhecido piorar mais de 2 pontos. **Nada do balanceamento mudou nesta task.**

## Leitura curta

**Onde a curva aperta: no Vale das Pedras Vivas, e quem manda lá é o renascimento, não a força.**
- Do 1 ao 30 são de 1 h 11 min (mago) a 1 h 31 min (cavaleiro Bastião) de caça sozinho, com o equipamento da loja +0. Com +3 o ganho é de 1 a 4 minutos.
- Do 1 ao 19 o tempo por nível sobe devagar, de ~30 s para ~2 a 3,5 min. Do 20 ao 29 vai de ~2 min 45 s para 6 a 7 min por nível. Os 10 níveis do vale levam 44 a 48 min, mais da metade do total (62% para o mago, 53% para o cavaleiro).
- No vale todo mundo mata mais rápido do que os golens renascem. São 12 golens espalhados do 20 ao 30; contando os de até 2 níveis de diferença, ~5,5 servem para cada nível, e cada um volta em 67,5 s na média. Isso dá um abate a cada ~12,4 s, ~290 por hora, para qualquer classe. Por isso o tempo do 20 ao 29 é igual para 6 dos 8 caminhos (44,1 min). Sem esse limite, o mago faria o 1→30 em 46 min e o Bastião em 1 h 29 min. Com mais gente caçando, o gargalo só piora: o próximo passo da curva (curva de XP, densidade e renascimento do vale) mexe aqui antes de mexer na XP.
- O anel do 13 ao 20 (20 esqueletos, 40 s de renascimento) já segura as classes rápidas (mago, ladino e bárbaro) a partir do 13 (as linhas com **R**).

**Qual classe sobra ou falta (tempo para matar em regime, +0).**
- **O mago sobra, e muito**: leva de 47% a 86% menos tempo que a média para matar, em todas as faixas. A Bola de Fogo no rank 3 (310% do ataque mágico, recarga de 3 s, 15 de mana) mais o ataque básico derrubam o esqueleto do mesmo nível num golpe só do 1 ao 8 (0,1 a 0,3 s para matar), e em menos de 1 s até o 19. O preço é a fragilidade: sem poção, morre em 20 a 60 s lutando parado. Mago e ladino são os únicos que nunca aguentam sem poção (o ladino morre em 26 a 85 s); cavaleiro e bárbaro passam a aguentar do 10 e do 13 em diante.
- **O cavaleiro falta**: leva de 62% a 113% mais tempo que a média para matar, e o Bastião é o pior do jogo (8,1 s contra 3,8 s de média no 25–30). Em troca, a partir do 10 aguenta lutar para sempre mesmo sem poção. É tanque por desenho, mas caçando sozinho o tempo até o 30 é o mais longo (1 h 31 min). O Bastião ainda toma 64 a 94 poções de mana por hora no vale (o Desafio do Bastião custa 35 de mana).
- O bárbaro fica na média até o 19 (só 18% acima no 6–12). No vale, o Sanguinário leva 24% menos tempo que a média no 25–30, e o Caudilho fica na média.
- O ladino fica na média até o 19; no vale, Sicário e Caçador levam de 18% a 24% menos tempo.
- Entre os caminhos da mesma classe: Elementalista > Ocultista (1,6 s contra 2,0 s no 25–30), Sanguinário > Caudilho (2,9 s contra 3,6 s), Arauto > Bastião (6,2 s contra 8,1 s). Sicário e Caçador ficam quase iguais.

**Quais itens furam.**
- **O encantamento fura quase tudo.** Em 44 dos 48 degraus das escadas de raridade (comum → boa → rara → épica → lendária, por encaixe e por arma de classe), um +10 passa a peça +0 da raridade seguinte em mais de 25%. O bônus por nível é fixo por raridade (1 a 5 por passo, e cada passo acima do +3 vale o dobro: +10 = 17 passos), não proporcional à peça. Nas armas o +10 vale de 1,3 a 2× a próxima raridade (Lâmina da Aurora +10: 113 de poder contra 57 da lendária +0). Nas peças pequenas é pior: capa e luvas +10 passam do dobro da peça seguinte (Capa do Patrulheiro +10: 43,5 contra 17,5 da Capa do Crepúsculo), e botas e anéis ficam de +58% a +118%. Só ficam dentro o peito comum (+15%) e os colares comum, bom e raro (+9% a +23%). A lista completa está em "Fora do orçamento hoje".
- **Peças +0 acima do orçamento**: as joias épicas e lendárias. Lágrima da Lua (+26%), Coração do Abismo (+26%) e Colar do Rei Ossário (+29%). O Manto das Cinzas (capa épica) fica logo abaixo da tolerância (+19%). Todas as armas ficam a até 11% do orçamento.
- Abaixo do orçamento, sem falhar o teste: Luvas de Couro (−35%) e Brinco de Osso (−27%).

**O que o modelo não vê** (para ler os números com cuidado): caça sozinho, de dia, sem mortes nem kiting; um monstro por vez (a área de efeito não rende mais); os efeitos no alvo (atordoar, enfraquecer) pegam sempre; o equipamento típico é só o da loja (comum e bom); a distância até o próximo monstro é uma estimativa pela densidade da região. As suposições estão em constantes comentadas em `server/internal/balance/` e resumidas logo abaixo.

## Suposições

- Fórmulas do pacote `game`: atributos (`CharacterStats`: classe, pontos, caminho, equipamento, passivas e regeneração), dano médio do golpe (`ExpectedHit`: acerto × ataque × 100/(100+defesa) × crítico), custo e recarga das skills (`SkillMPCost`, `SkillCooldownFor`), XP (`MonsterXP`, `XPToNext`) e monstros reais (`MonsterStats`, `DefaultSpawns`). Média por fórmula: sem sorteio.
- Caça sozinho, de dia, sem XP em dobro. A luta anda no passo do tick da zona (0,05 s).
- Pontos de atributo: 2 no atributo de dano e 1 no de apoio a cada nível (cavaleiro Força/Constituição, bárbaro Força/Constituição, mago Inteligência/Sabedoria, ladino Destreza/Força). O caminho escolhido no 20 soma o bônus dele.
- Árvore: dano primeiro (ordem em `build.go`), o 5º degrau e as 3 skills do caminho do 20 em diante, o resto em defesa. Nenhuma pontuação fora das regras (`ValidateRanks`).
- Equipamento típico: em cada encaixe a melhor peça comum ou boa que a classe usa no nível (pelo poder do item), tudo +0 ou tudo +3 (o encantamento seguro). Raras, épicas e lendárias ficam fora (são drop ou criação).
- Rotação como a auto-caça do cliente: uma skill a cada 1 s, buffs que faltam primeiro, depois enfraquecer/marcar o alvo, depois a skill de mais dano pronta e com mana; o ataque básico corre junto. Poção de vida abaixo de 45% e de mana abaixo de 20% (as menores até o nível 9, as maiores depois), sem limite de estoque. Fuga e utilidade (Piscar, Passo Sombrio, Sumir, Escudo Arcano, Imortal, Provocar) ficam de fora.
- Monstro: o do nível do jogador, na região onde o nível caça (o grupo de `DefaultSpawns` que cobre o nível; empate, a região mais nova: do 20 em diante, o Vale das Pedras Vivas). Monstros que dividem a região entram na proporção da quantidade. A luta abre no alcance do ataque básico; o monstro anda até o dele (quem bate de longe ganha esse tempo).
- **Matar**: tempo médio em regime (as recargas seguem de uma luta para a outra); **abertura** é a primeira luta, com tudo pronto. **DPS**: dano por segundo da rotação numa luta longa contra o mesmo monstro. **Morrer**: lutando sem parar contra o monstro (que não cai), com regeneração de combate, com e sem poções; ∞ = aguenta mais de 5 min (ou volta à vida cheia no meio da luta).
- **XP/h**: abate + deslocamento até o próximo (1,5 × a distância média ao vizinho mais perto, 0,5 × √(área por monstro)) ou descanso até 80% da vida e 50% da mana (regeneração de combate nos primeiros 5 s e depois ×5,0 na vida e ×3,3 na mana), o que for maior. Ninguém mata mais rápido do que os monstros do seu nível (±2) renascem (1,5× o tempo base, em média); onde isso limita, a linha leva **R**.
- **Tempo no nível**: XP do nível ÷ XP/h. O acumulado soma do 1 até o nível. Sem mortes (quem morre caçando aparece como ∞).

## Tempo até o 30

| Caminho | +0 | +3 | +0 sem limite de renascimento | +3 sem limite |
|---|---:|---:|---:|---:|
| Cavaleiro/Bastião | 1 h 31 min | 1 h 27 min | 1 h 29 min | 1 h 24 min |
| Cavaleiro/Arauto | 1 h 27 min | 1 h 23 min | 1 h 21 min | 1 h 16 min |
| Bárbaro/Sanguinário | 1 h 22 min | 1 h 21 min | 1 h 09 min | 1 h 07 min |
| Bárbaro/Caudilho | 1 h 23 min | 1 h 21 min | 1 h 10 min | 1 h 07 min |
| Mago/Elementalista | 1 h 11 min | 1 h 10 min | 45 min 59 s | 45 min 10 s |
| Mago/Ocultista | 1 h 11 min | 1 h 10 min | 46 min 50 s | 46 min 09 s |
| Ladino/Sicário | 1 h 16 min | 1 h 14 min | 1 h 07 min | 1 h 06 min |
| Ladino/Caçador | 1 h 16 min | 1 h 14 min | 1 h 08 min | 1 h 05 min |

## Tempo para matar por faixa (regime, +0)

Média dos níveis da faixa; entre parênteses, a distância da média da faixa (tolerância do teste: ±15%).

- **1–5** (média 1,6 s): Cavaleiro 3,1 s (+96%) · Bárbaro 1,5 s (−3%) · Mago 0,2 s (−86%) · Ladino 1,5 s (−7%)
- **6–12** (média 1,8 s): Cavaleiro 3,0 s (+74%) · Bárbaro 2,1 s (+18%) · Mago 0,3 s (−81%) · Ladino 1,6 s (−11%)
- **13–19** (média 1,2 s): Cavaleiro 2,0 s (+70%) · Bárbaro 1,1 s (−5%) · Mago 0,5 s (−54%) · Ladino 1,1 s (−10%)
- **20–24** (média 4,1 s): Cavaleiro/Bastião 8,1 s (+95%) · Cavaleiro/Arauto 6,8 s (+65%) · Bárbaro/Sanguinário 3,5 s (−14%) · Bárbaro/Caudilho 4,2 s (+2%) · Mago/Elementalista 2,0 s (−52%) · Mago/Ocultista 2,0 s (−52%) · Ladino/Sicário 3,2 s (−23%) · Ladino/Caçador 3,3 s (−20%)
- **25–30** (média 3,8 s): Cavaleiro/Bastião 8,1 s (+113%) · Cavaleiro/Arauto 6,2 s (+62%) · Bárbaro/Sanguinário 2,9 s (−24%) · Bárbaro/Caudilho 3,6 s (−5%) · Mago/Elementalista 1,6 s (−58%) · Mago/Ocultista 2,0 s (−47%) · Ladino/Sicário 2,9 s (−24%) · Ladino/Caçador 3,2 s (−18%)

## Orçamento de poder dos itens

Poder = ataque + defesas + vida/10 + mana/10 + 1 por 1% de crítico e de velocidade + … (pesos em `power.go`). Orçamento = (6 + 1,4 × (nível − 1)) × raridade (comum ×1,0, boa ×1,1, rara ×1,2, épica ×1,5, lendária ×1,7) × peso do encaixe. **% orç.**: poder +0 ÷ orçamento (tolerância +20%). **+10 ÷ próxima**: o +10 contra a peça seguinte da escada +0 (tolerância +25%).

| Família | Peça | Raridade | Nível | Poder +0 | +3 | +10 | Orçamento | % orç. | +10 ÷ próxima |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|
| anel | Anel de Cobre | comum | 1 | 3,0 | 5,0 | 12,0 | 3,0 | +0% | +100% |
| anel | Anel de Prata | boa | 6 | 6,0 | 9,0 | 23,0 | 7,2 | −16% | +92% |
| anel | Anel de Ônix | rara | 13 | 12,0 | 17,0 | 38,0 | 13,7 | −12% | +58% |
| anel | Selo do Senhor dos Ossos | épica | 18 | 24,0 | 30,0 | 58,0 | 22,3 | +7% | +87% |
| anel | Anel do Rei Ossário | lendária | 20 | 31,0 | 39,0 | 74,0 | 27,7 | +12% | — |
| brinco | Brinco de Osso | comum | 1 | 2,4 | 4,4 | 11,4 | 3,3 | −27% | +46% |
| brinco | Brinco de Âmbar | boa | 6 | 7,8 | 10,8 | 24,8 | 7,9 | −1% | +50% |
| brinco | Brinco de Safira | rara | 13 | 16,5 | 21,5 | 42,5 | 15,0 | +10% | +37% |
| brinco | Lágrima da Lua | épica | 18 | 31,0 | 37,0 | 65,0 | 24,6 | +26% | — |
| cabeça | Capuz de Couro | comum | 1 | 6,0 | 10,0 | 24,0 | 5,7 | +5% | +66% |
| cabeça | Elmo de Ferro | boa | 6 | 14,5 | 20,5 | 48,5 | 13,6 | +7% | +83% |
| cabeça | Elmo de Osso | rara | 13 | 26,5 | 36,5 | 78,5 | 26,0 | +2% | +83% |
| cabeça | Elmo do Senhor dos Ossos | épica | 18 | 43,0 | 55,0 | 111,0 | 42,5 | +1% | — |
| capa | Capa de Lã | comum | 1 | 4,0 | 8,0 | 22,0 | 3,9 | +3% | +132% |
| capa | Capa do Patrulheiro | boa | 6 | 9,5 | 15,5 | 43,5 | 9,3 | +2% | +149% |
| capa | Capa do Crepúsculo | rara | 13 | 17,5 | 27,5 | 69,5 | 17,8 | −2% | +101% |
| capa | Manto das Cinzas | épica | 18 | 34,5 | 46,5 | 102,5 | 29,1 | +19% | — |
| colar | Colar de Contas | comum | 1 | 4,5 | 6,5 | 13,5 | 4,8 | −6% | +23% |
| colar | Colar do Lobo | boa | 6 | 11,0 | 14,0 | 28,0 | 11,4 | −4% | +22% |
| colar | Amuleto Rúnico | rara | 13 | 23,0 | 28,0 | 49,0 | 21,9 | +5% | +9% |
| colar | Coração do Abismo | épica | 18 | 45,0 | 51,0 | 79,0 | 35,8 | +26% | +39% |
| colar | Colar do Rei Ossário | lendária | 20 | 57,0 | 65,0 | 100,0 | 44,3 | +29% | — |
| escudo | Escudo Pesado | comum | 1 | 6,5 | 10,5 | 24,5 | 7,2 | −10% | +36% |
| escudo | Escudo de Ferro | boa | 6 | 18,0 | 24,0 | 52,0 | 17,2 | +5% | +62% |
| escudo | Escudo de Cravos | rara | 13 | 32,0 | 42,0 | 84,0 | 32,8 | −3% | +53% |
| escudo | Égide Óssea | épica | 18 | 55,0 | 67,0 | 123,0 | 53,6 | +3% | — |
| mãos | Luvas de Couro | comum | 1 | 2,8 | 6,8 | 20,8 | 4,2 | −35% | +124% |
| mãos | Manoplas de Ferro | boa | 6 | 9,2 | 15,2 | 43,2 | 10,0 | −8% | +116% |
| mãos | Manoplas Rúnicas | rara | 13 | 20,0 | 30,0 | 72,0 | 19,2 | +4% | +118% |
| mãos | Garras do Ossário | épica | 18 | 33,0 | 45,0 | 101,0 | 31,3 | +5% | — |
| peito | Gibão de Couro | comum | 1 | 9,5 | 13,5 | 27,5 | 9,6 | −1% | +15% |
| peito | Cota de Malha | boa | 6 | 24,0 | 30,0 | 58,0 | 22,9 | +5% | +26% |
| peito | Couraça de Osso | rara | 13 | 46,0 | 56,0 | 98,0 | 43,8 | +5% | +27% |
| peito | Manto do Senhor dos Ossos | épica | 18 | 77,0 | 89,0 | 145,0 | 71,5 | +8% | — |
| pernas | Calças de Couro | comum | 1 | 6,0 | 10,0 | 24,0 | 6,3 | −5% | +55% |
| pernas | Grevas de Malha | boa | 6 | 15,5 | 21,5 | 49,5 | 15,0 | +3% | +62% |
| pernas | Grevas de Osso | rara | 13 | 30,5 | 40,5 | 82,5 | 28,7 | +6% | +65% |
| pernas | Grevas do Crepúsculo | épica | 18 | 50,0 | 62,0 | 118,0 | 46,9 | +7% | — |
| pés | Botas de Viajante | comum | 1 | 4,0 | 8,0 | 22,0 | 4,5 | −11% | +83% |
| pés | Botas de Ferro | boa | 6 | 12,0 | 18,0 | 46,0 | 10,7 | +12% | +109% |
| pés | Botas do Vento | rara | 13 | 22,0 | 32,0 | 74,0 | 20,5 | +7% | +118% |
| pés | Passos da Sombra | épica | 18 | 34,0 | 46,0 | 102,0 | 33,5 | +1% | — |
| arma (bárbaro) | Machado de Lenhador | comum | 1 | 7,0 | 10,0 | 24,0 | 6,6 | +6% | +50% |
| arma (bárbaro) | Machado de Guerra | boa | 6 | 16,0 | 22,0 | 50,0 | 15,7 | +2% | +72% |
| arma (bárbaro) | Machado Rachador | rara | 13 | 29,0 | 38,0 | 80,0 | 30,1 | −4% | +82% |
| arma (bárbaro) | Fúria do Norte | épica | 18 | 44,0 | 56,0 | 112,0 | 49,2 | −11% | +96% |
| arma (bárbaro) | Machado do Rei Ossário | lendária | 20 | 57,0 | 72,0 | 142,0 | 61,0 | −6% | — |
| arma (cavaleiro) | Espada de Treino | comum | 1 | 6,0 | 9,0 | 23,0 | 6,0 | +0% | +64% |
| arma (cavaleiro) | Espada de Aço | boa | 6 | 14,0 | 20,0 | 48,0 | 14,3 | −2% | +78% |
| arma (cavaleiro) | Lâmina do Vigia | rara | 13 | 27,0 | 36,0 | 78,0 | 27,4 | −1% | +73% |
| arma (cavaleiro) | Lâmina da Aurora | épica | 18 | 45,0 | 57,0 | 113,0 | 44,7 | +1% | +98% |
| arma (cavaleiro) | Lâmina do Rei Ossário | lendária | 20 | 57,0 | 72,0 | 142,0 | 55,4 | +3% | — |
| arma (mago) | Cajado de Aprendiz | comum | 1 | 8,0 | 11,0 | 25,0 | 7,8 | +3% | +32% |
| arma (mago) | Cajado de Carvalho | boa | 6 | 19,0 | 25,0 | 53,0 | 18,6 | +2% | +61% |
| arma (mago) | Varinha Rúnica | rara | 13 | 33,0 | 42,0 | 84,0 | 35,6 | −7% | +56% |
| arma (mago) | Cetro do Abismo | épica | 18 | 54,0 | 66,0 | 122,0 | 58,1 | −7% | +69% |
| arma (mago) | Cetro do Rei Ossário | lendária | 20 | 72,0 | 87,0 | 157,0 | 72,0 | −0% | — |
| arma (ladino) | Adagas Gêmeas | comum | 1 | 5,0 | 8,0 | 22,0 | 5,4 | −7% | +57% |
| arma (ladino) | Adagas de Aço | boa | 6 | 14,0 | 20,0 | 48,0 | 12,9 | +9% | +92% |
| arma (ladino) | Presas Sombrias | rara | 13 | 25,0 | 34,0 | 76,0 | 24,6 | +2% | +100% |
| arma (ladino) | Presas do Crepúsculo | épica | 18 | 38,0 | 50,0 | 106,0 | 40,2 | −6% | +116% |
| arma (ladino) | Garras do Rei Ossário | lendária | 20 | 49,0 | 64,0 | 134,0 | 49,9 | −2% | — |

## Fora do orçamento hoje (exceções conhecidas do teste)

- `enchant:armor_good` +26%: +10 Cota de Malha (58,0) contra Couraça de Osso +0 (46,0)
- `enchant:armor_rare` +27%: +10 Couraça de Osso (98,0) contra Manto do Senhor dos Ossos +0 (77,0)
- `enchant:axe_common` +50%: +10 Machado de Lenhador (24,0) contra Machado de Guerra +0 (16,0)
- `enchant:axe_epic` +96%: +10 Fúria do Norte (112,0) contra Machado do Rei Ossário +0 (57,0)
- `enchant:axe_good` +72%: +10 Machado de Guerra (50,0) contra Machado Rachador +0 (29,0)
- `enchant:axe_rare` +82%: +10 Machado Rachador (80,0) contra Fúria do Norte +0 (44,0)
- `enchant:boots_common` +83%: +10 Botas de Viajante (22,0) contra Botas de Ferro +0 (12,0)
- `enchant:boots_good` +109%: +10 Botas de Ferro (46,0) contra Botas do Vento +0 (22,0)
- `enchant:boots_rare` +118%: +10 Botas do Vento (74,0) contra Passos da Sombra +0 (34,0)
- `enchant:cape_common` +132%: +10 Capa de Lã (22,0) contra Capa do Patrulheiro +0 (9,5)
- `enchant:cape_good` +149%: +10 Capa do Patrulheiro (43,5) contra Capa do Crepúsculo +0 (17,5)
- `enchant:cape_rare` +101%: +10 Capa do Crepúsculo (69,5) contra Manto das Cinzas +0 (34,5)
- `enchant:daggers_common` +57%: +10 Adagas Gêmeas (22,0) contra Adagas de Aço +0 (14,0)
- `enchant:daggers_epic` +116%: +10 Presas do Crepúsculo (106,0) contra Garras do Rei Ossário +0 (49,0)
- `enchant:daggers_good` +92%: +10 Adagas de Aço (48,0) contra Presas Sombrias +0 (25,0)
- `enchant:daggers_rare` +100%: +10 Presas Sombrias (76,0) contra Presas do Crepúsculo +0 (38,0)
- `enchant:ear_common` +46%: +10 Brinco de Osso (11,4) contra Brinco de Âmbar +0 (7,8)
- `enchant:ear_good` +50%: +10 Brinco de Âmbar (24,8) contra Brinco de Safira +0 (16,5)
- `enchant:ear_rare` +37%: +10 Brinco de Safira (42,5) contra Lágrima da Lua +0 (31,0)
- `enchant:gloves_common` +124%: +10 Luvas de Couro (20,8) contra Manoplas de Ferro +0 (9,2)
- `enchant:gloves_good` +116%: +10 Manoplas de Ferro (43,2) contra Manoplas Rúnicas +0 (20,0)
- `enchant:gloves_rare` +118%: +10 Manoplas Rúnicas (72,0) contra Garras do Ossário +0 (33,0)
- `enchant:helm_common` +66%: +10 Capuz de Couro (24,0) contra Elmo de Ferro +0 (14,5)
- `enchant:helm_good` +83%: +10 Elmo de Ferro (48,5) contra Elmo de Osso +0 (26,5)
- `enchant:helm_rare` +83%: +10 Elmo de Osso (78,5) contra Elmo do Senhor dos Ossos +0 (43,0)
- `enchant:legs_common` +55%: +10 Calças de Couro (24,0) contra Grevas de Malha +0 (15,5)
- `enchant:legs_good` +62%: +10 Grevas de Malha (49,5) contra Grevas de Osso +0 (30,5)
- `enchant:legs_rare` +65%: +10 Grevas de Osso (82,5) contra Grevas do Crepúsculo +0 (50,0)
- `enchant:neck_epic` +39%: +10 Coração do Abismo (79,0) contra Colar do Rei Ossário +0 (57,0)
- `enchant:ring_common` +100%: +10 Anel de Cobre (12,0) contra Anel de Prata +0 (6,0)
- `enchant:ring_epic` +87%: +10 Selo do Senhor dos Ossos (58,0) contra Anel do Rei Ossário +0 (31,0)
- `enchant:ring_good` +92%: +10 Anel de Prata (23,0) contra Anel de Ônix +0 (12,0)
- `enchant:ring_rare` +58%: +10 Anel de Ônix (38,0) contra Selo do Senhor dos Ossos +0 (24,0)
- `enchant:shield_good` +62%: +10 Escudo de Ferro (52,0) contra Escudo de Cravos +0 (32,0)
- `enchant:shield_rare` +53%: +10 Escudo de Cravos (84,0) contra Égide Óssea +0 (55,0)
- `enchant:shield_tower` +36%: +10 Escudo Pesado (24,5) contra Escudo de Ferro +0 (18,0)
- `enchant:staff_common` +32%: +10 Cajado de Aprendiz (25,0) contra Cajado de Carvalho +0 (19,0)
- `enchant:staff_epic` +69%: +10 Cetro do Abismo (122,0) contra Cetro do Rei Ossário +0 (72,0)
- `enchant:staff_good` +61%: +10 Cajado de Carvalho (53,0) contra Varinha Rúnica +0 (33,0)
- `enchant:staff_rare` +56%: +10 Varinha Rúnica (84,0) contra Cetro do Abismo +0 (54,0)
- `enchant:sword_common` +64%: +10 Espada de Treino (23,0) contra Espada de Aço +0 (14,0)
- `enchant:sword_epic` +98%: +10 Lâmina da Aurora (113,0) contra Lâmina do Rei Ossário +0 (57,0)
- `enchant:sword_good` +78%: +10 Espada de Aço (48,0) contra Lâmina do Vigia +0 (27,0)
- `enchant:sword_rare` +73%: +10 Lâmina do Vigia (78,0) contra Lâmina da Aurora +0 (45,0)
- `item:ear_epic` +26%: Lágrima da Lua (épica, nível 18): poder 31,0, orçamento 24,6
- `item:neck_epic` +26%: Coração do Abismo (épica, nível 18): poder 45,0, orçamento 35,8
- `item:neck_legend` +29%: Colar do Rei Ossário (lendária, nível 20): poder 57,0, orçamento 44,3
- `ttk:13–19:Cavaleiro` +70%: Cavaleiro no 13–19: 2,0 s para matar, média da faixa 1,2 s
- `ttk:13–19:Mago` −54%: Mago no 13–19: 0,5 s para matar, média da faixa 1,2 s
- `ttk:1–5:Cavaleiro` +96%: Cavaleiro no 1–5: 3,1 s para matar, média da faixa 1,6 s
- `ttk:1–5:Mago` −86%: Mago no 1–5: 0,2 s para matar, média da faixa 1,6 s
- `ttk:20–24:Cavaleiro/Arauto` +65%: Cavaleiro/Arauto no 20–24: 6,8 s para matar, média da faixa 4,1 s
- `ttk:20–24:Cavaleiro/Bastião` +95%: Cavaleiro/Bastião no 20–24: 8,1 s para matar, média da faixa 4,1 s
- `ttk:20–24:Ladino/Caçador` −20%: Ladino/Caçador no 20–24: 3,3 s para matar, média da faixa 4,1 s
- `ttk:20–24:Ladino/Sicário` −23%: Ladino/Sicário no 20–24: 3,2 s para matar, média da faixa 4,1 s
- `ttk:20–24:Mago/Elementalista` −52%: Mago/Elementalista no 20–24: 2,0 s para matar, média da faixa 4,1 s
- `ttk:20–24:Mago/Ocultista` −52%: Mago/Ocultista no 20–24: 2,0 s para matar, média da faixa 4,1 s
- `ttk:25–30:Bárbaro/Sanguinário` −24%: Bárbaro/Sanguinário no 25–30: 2,9 s para matar, média da faixa 3,8 s
- `ttk:25–30:Cavaleiro/Arauto` +62%: Cavaleiro/Arauto no 25–30: 6,2 s para matar, média da faixa 3,8 s
- `ttk:25–30:Cavaleiro/Bastião` +113%: Cavaleiro/Bastião no 25–30: 8,1 s para matar, média da faixa 3,8 s
- `ttk:25–30:Ladino/Caçador` −18%: Ladino/Caçador no 25–30: 3,2 s para matar, média da faixa 3,8 s
- `ttk:25–30:Ladino/Sicário` −24%: Ladino/Sicário no 25–30: 2,9 s para matar, média da faixa 3,8 s
- `ttk:25–30:Mago/Elementalista` −58%: Mago/Elementalista no 25–30: 1,6 s para matar, média da faixa 3,8 s
- `ttk:25–30:Mago/Ocultista` −47%: Mago/Ocultista no 25–30: 2,0 s para matar, média da faixa 3,8 s
- `ttk:6–12:Bárbaro` +18%: Bárbaro no 6–12: 2,1 s para matar, média da faixa 1,8 s
- `ttk:6–12:Cavaleiro` +74%: Cavaleiro no 6–12: 3,0 s para matar, média da faixa 1,8 s
- `ttk:6–12:Mago` −81%: Mago no 6–12: 0,3 s para matar, média da faixa 1,8 s

## Por nível

Pares **a / b** = equipamento +0 / +3; o resto é +0. **R** = o renascimento limita o ritmo.

### Cavaleiro (1–19)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | Esqueleto Lacaio | 15 / 18 | 2,4 / 1,9 | 1,2 | ∞ | 3 min 10 s | 1% | 12 | 698 | 8.379 / 9.144 | 0 | 34 s / 31 s | 34 s / 31 s |
| 2 | Esqueleto Lacaio | 18 / 21 | 3,0 / 2,4 | 2,5 | ∞ | 3 min 53 s | 2% | 36 | 604 | 21.739 / 25.138 | 0 | 1 min 00 s / 53 s | 1 min 35 s / 1 min 24 s |
| 3 | Esqueleto Lacaio | 21 / 23 | 3,4 / 3,0 | 2,5 | ∞ | ∞ | 2% | 70 | 590 | 41.274 / 44.168 | 0 | 1 min 18 s / 1 min 13 s | 2 min 53 s / 2 min 37 s |
| 4 | Esqueleto Lacaio | 24 / 27 | 3,4 / 3,4 | 2,5 | ∞ | ∞ | 1% | 110 | 590 | 64.860 / 64.860 | 0 | 1 min 33 s / 1 min 33 s | 4 min 27 s / 4 min 11 s |
| 5 | Esqueleto Lacaio | 30 / 33 | 3,1 / 3,1 | 2,5 | ∞ | ∞ | 2% | 158 | 462 | 72.922 / 90.589 | 77 | 2 min 16 s / 1 min 49 s | 6 min 43 s / 6 min 00 s |
| 6 | Esqueleto Guerreiro + Esqueleto Ladino | 43 / 50 | 2,6 / 2,3 | 1,9 | ∞ | 2 min 29 s | 1% | 281 | 571 | 160.422 / 167.464 | 0 | 1 min 32 s / 1 min 28 s | 8 min 15 s / 7 min 29 s |
| 7 | Esqueleto Guerreiro + Esqueleto Ladino | 46 / 53 | 3,1 / 2,3 | 2,2 | ∞ | 2 min 41 s | 2% | 360 | 503 | 180.974 / 215.475 | 0 | 1 min 55 s / 1 min 36 s | 10 min 11 s / 9 min 05 s |
| 8 | Esqueleto Guerreiro + Esqueleto Ladino | 51 / 58 | 3,2 / 2,5 | 2,2 | ∞ | 2 min 45 s | 2% | 446 | 504 | 224.887 / 259.315 | 0 | 2 min 04 s / 1 min 47 s | 12 min 15 s / 10 min 53 s |
| 9 | Esqueleto Guerreiro + Esqueleto Ladino | 56 / 62 | 3,2 / 2,5 | 2,2 | ∞ | 4 min 22 s | 2% | 538 | 506 | 272.030 / 312.806 | 0 | 2 min 13 s / 1 min 55 s | 14 min 28 s / 12 min 49 s |
| 10 | Esqueleto Guerreiro + Esqueleto Ladino | 60 / 66 | 3,4 / 2,5 | 2,2 | ∞ | ∞ | 2% | 637 | 502 | 319.806 / 370.367 | 0 | 2 min 22 s / 2 min 03 s | 16 min 51 s / 14 min 52 s |
| 11 | Esqueleto Guerreiro + Esqueleto Ladino | 65 / 71 | 3,4 / 2,6 | 2,2 | ∞ | ∞ | 1% | 742 | 504 | 373.920 / 423.578 | 0 | 2 min 30 s / 2 min 12 s | 19 min 21 s / 17 min 05 s |
| 12 | Esqueleto Guerreiro + Esqueleto Ladino | 74 / 81 | 2,4 / 2,2 | 2,2 | ∞ | ∞ | 1% | 853 | 498 | 424.666 / 467.238 | 0 | 2 min 40 s / 2 min 25 s | 22 min 02 s / 19 min 31 s |
| 13 | Esqueleto Mago + Esqueleto Guerreiro | 82 / 89 | 2,0 / 2,0 | 1,9 | ∞ | ∞ | 1% | 1.030 | 520 | 535.061 / 579.099 | 0 | 2 min 31 s / 2 min 20 s | 24 min 34 s / 21 min 51 s |
| 14 | Esqueleto Mago + Esqueleto Guerreiro | 88 / 95 | 2,1 / 2,0 | 1,9 | ∞ | ∞ | 1% | 1.160 | 522 | 605.047 / 656.892 | 0 | 2 min 38 s / 2 min 25 s | 27 min 12 s / 24 min 17 s |
| 15 | Esqueleto Mago + Esqueleto Guerreiro | 101 / 109 | 1,8 / 1,7 | 1,6 | ∞ | ∞ | 0% | 1.295 | 491 | 636.415 / 661.172 | 0 | 2 min 55 s / 2 min 48 s | 30 min 07 s / 27 min 05 s |
| 16 | Esqueleto Mago + Esqueleto Guerreiro **R** | 106 / 114 | 1,9 / 1,7 | 1,6 | ∞ | ∞ | 0% | 1.436 | 494 | 708.504 / 744.957 | 0 | 3 min 01 s / 2 min 52 s | 33 min 08 s / 29 min 58 s |
| 17 | Esqueleto Mago + Esqueleto Guerreiro **R** | 111 / 119 | 2,0 / 1,8 | 1,6 | ∞ | ∞ | 0% | 1.582 | 493 | 780.479 / 812.799 | 0 | 3 min 07 s / 3 min 00 s | 36 min 16 s / 32 min 58 s |
| 18 | Esqueleto Mago + Esqueleto Guerreiro **R** | 116 / 124 | 2,0 / 1,8 | 1,6 | ∞ | ∞ | 0% | 1.733 | 487 | 843.914 / 904.228 | 0 | 3 min 17 s / 3 min 03 s | 39 min 33 s / 36 min 02 s |
| 19 | Esqueleto Mago + Esqueleto Guerreiro **R** | 121 / 129 | 2,1 / 1,8 | 1,6 | ∞ | ∞ | 0% | 1.890 | 489 | 925.001 / 994.574 | 0 | 3 min 22 s / 3 min 08 s | 42 min 56 s / 39 min 11 s |

### Cavaleiro/Bastião (20–30)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | Vale das Pedras Vivas **R** | 106 / 112 | 7,8 / 7,0 | 5,1 | ∞ | ∞ | 0% | 4.827 | 279 | 1.349.070 / 1.347.584 | 93 | 2 min 51 s / 2 min 51 s | 45 min 47 s / 42 min 02 s |
| 21 | Vale das Pedras Vivas **R** | 109 / 115 | 7,8 / 7,0 | 5,1 | ∞ | ∞ | 0% | 5.219 | 281 | 1.465.285 / 1.466.390 | 94 | 3 min 11 s / 3 min 11 s | 48 min 58 s / 45 min 13 s |
| 22 | Vale das Pedras Vivas **R** | 113 / 119 | 8,1 / 7,4 | 6,1 | ∞ | ∞ | 0% | 5.623 | 274 | 1.537.930 / 1.536.119 | 91 | 3 min 38 s / 3 min 38 s | 52 min 36 s / 48 min 52 s |
| 23 | Vale das Pedras Vivas **R** | 117 / 123 | 7,8 / 7,5 | 6,1 | ∞ | ∞ | 0% | 6.037 | 271 | 1.634.342 / 1.663.791 | 90 | 4 min 04 s / 4 min 00 s | 56 min 41 s / 52 min 52 s |
| 24 | Vale das Pedras Vivas **R** | 121 / 127 | 7,8 / 7,6 | 6,1 | ∞ | ∞ | 0% | 6.462 | 273 | 1.761.550 / 1.746.690 | 91 | 4 min 26 s / 4 min 28 s | 1 h 01 min / 57 min 21 s |
| 25 | Vale das Pedras Vivas **R** | 125 / 131 | 8,0 / 8,0 | 6,1 | ∞ | ∞ | 0% | 6.899 | 273 | 1.885.879 / 1.915.257 | 91 | 4 min 50 s / 4 min 46 s | 1 h 06 min / 1 h 02 min |
| 26 | Vale das Pedras Vivas **R** | 129 / 134 | 8,6 / 7,9 | 6,1 | ∞ | ∞ | 0% | 7.345 | 257 | 1.885.399 / 1.878.869 | 64 | 5 min 36 s / 5 min 37 s | 1 h 12 min / 1 h 08 min |
| 27 | Vale das Pedras Vivas **R** | 133 / 138 | 8,7 / 7,9 | 6,1 | ∞ | ∞ | 0% | 7.803 | 257 | 2.005.665 / 2.005.985 | 64 | 6 min 04 s / 6 min 04 s | 1 h 18 min / 1 h 14 min |
| 28 | Vale das Pedras Vivas **R** | 141 / 147 | 7,9 / 7,5 | 6,1 | ∞ | ∞ | 0% | 8.270 | 264 | 2.187.177 / 2.197.054 | 88 | 6 min 21 s / 6 min 20 s | 1 h 24 min / 1 h 20 min |
| 29 | Vale das Pedras Vivas **R** | 145 / 151 | 7,9 / 7,6 | 6,1 | ∞ | ∞ | 0% | 8.748 | 266 | 2.330.580 / 2.324.517 | 89 | 6 min 47 s / 6 min 48 s | 1 h 31 min / 1 h 27 min |
| 30 | Vale das Pedras Vivas **R** | 150 / 155 | 8,0 / 7,7 | 6,1 | ∞ | ∞ | 0% | 9.235 | 267 | 2.466.211 / 2.377.672 | 89 | 0 s / 0 s | 1 h 31 min / 1 h 27 min |

### Cavaleiro/Arauto (20–30)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | Vale das Pedras Vivas **R** | 125 / 132 | 5,6 / 4,8 | 4,9 | ∞ | ∞ | 0% | 4.827 | 291 | 1.404.218 / 1.404.218 | 0 | 2 min 44 s / 2 min 44 s | 45 min 40 s / 41 min 55 s |
| 21 | Vale das Pedras Vivas **R** | 132 / 139 | 5,6 / 4,8 | 4,9 | ∞ | ∞ | 0% | 5.219 | 291 | 1.518.255 / 1.518.255 | 0 | 3 min 04 s / 3 min 04 s | 48 min 44 s / 44 min 59 s |
| 22 | Vale das Pedras Vivas **R** | 136 / 143 | 5,6 / 4,8 | 4,9 | ∞ | ∞ | 0% | 5.623 | 291 | 1.635.782 / 1.635.782 | 0 | 3 min 25 s / 3 min 25 s | 52 min 10 s / 48 min 25 s |
| 23 | Vale das Pedras Vivas **R** | 141 / 148 | 5,6 / 4,8 | 4,9 | ∞ | ∞ | 0% | 6.037 | 291 | 1.756.218 / 1.756.218 | 0 | 3 min 47 s / 3 min 47 s | 55 min 57 s / 52 min 12 s |
| 24 | Vale das Pedras Vivas **R** | 147 / 154 | 5,5 / 5,1 | 5,1 | ∞ | ∞ | 0% | 6.462 | 291 | 1.879.855 / 1.879.855 | 24 | 4 min 09 s / 4 min 09 s | 1 h 00 min / 56 min 22 s |
| 25 | Vale das Pedras Vivas **R** | 151 / 158 | 5,5 / 5,1 | 5,1 | ∞ | ∞ | 0% | 6.899 | 291 | 2.006.982 / 2.006.982 | 0 | 4 min 33 s / 4 min 33 s | 1 h 05 min / 1 h 01 min |
| 26 | Vale das Pedras Vivas **R** | 156 / 163 | 5,5 / 5,1 | 5,1 | ∞ | ∞ | 0% | 7.345 | 291 | 2.136.727 / 2.136.727 | 0 | 4 min 57 s / 4 min 57 s | 1 h 10 min / 1 h 06 min |
| 27 | Vale das Pedras Vivas **R** | 161 / 167 | 5,5 / 5,1 | 5,1 | ∞ | ∞ | 0% | 7.803 | 291 | 2.269.964 / 2.269.964 | 0 | 5 min 21 s / 5 min 21 s | 1 h 15 min / 1 h 11 min |
| 28 | Vale das Pedras Vivas **R** | 173 / 179 | 5,4 / 5,1 | 5,1 | ∞ | ∞ | 0% | 8.270 | 291 | 2.405.818 / 2.405.818 | 48 | 5 min 47 s / 5 min 47 s | 1 h 21 min / 1 h 17 min |
| 29 | Vale das Pedras Vivas **R** | 178 / 184 | 5,5 / 5,4 | 5,1 | ∞ | ∞ | 0% | 8.748 | 290 | 2.533.486 / 2.533.486 | 48 | 6 min 14 s / 6 min 14 s | 1 h 27 min / 1 h 23 min |
| 30 | Vale das Pedras Vivas **R** | 183 / 189 | 5,4 / 5,4 | 5,1 | ∞ | ∞ | 0% | 9.235 | 290 | 2.675.945 / 2.675.945 | 24 | 0 s / 0 s | 1 h 27 min / 1 h 23 min |

### Bárbaro (1–19)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | Esqueleto Lacaio | 22 / 26 | 1,1 / 0,8 | 1,1 | ∞ | 2 min 08 s | 2% | 12 | 563 | 6.753 / 7.787 | 0 | 43 s / 37 s | 43 s / 37 s |
| 2 | Esqueleto Lacaio | 27 / 31 | 1,1 / 1,4 | 1,1 | ∞ | 2 min 17 s | 2% | 36 | 548 | 19.741 / 26.333 | 0 | 1 min 07 s / 50 s | 1 min 49 s / 1 min 27 s |
| 3 | Esqueleto Lacaio | 33 / 36 | 1,3 / 1,3 | 1,1 | ∞ | 2 min 32 s | 2% | 70 | 553 | 38.675 / 38.675 | 0 | 1 min 23 s / 1 min 23 s | 3 min 13 s / 2 min 50 s |
| 4 | Esqueleto Lacaio | 38 / 42 | 1,9 / 1,9 | 1,1 | ∞ | 2 min 48 s | 3% | 110 | 622 | 68.444 / 68.444 | 0 | 1 min 28 s / 1 min 28 s | 4 min 42 s / 4 min 19 s |
| 5 | Esqueleto Lacaio | 43 / 47 | 2,2 / 1,9 | 1,1 | ∞ | 3 min 08 s | 3% | 158 | 576 | 90.959 / 103.378 | 0 | 1 min 49 s / 1 min 36 s | 6 min 31 s / 5 min 55 s |
| 6 | Esqueleto Guerreiro + Esqueleto Ladino | 61 / 69 | 2,3 / 2,2 | 1,1 | ∞ | 1 min 07 s | 4% | 281 | 594 | 166.861 / 172.187 | 0 | 1 min 28 s / 1 min 26 s | 8 min 00 s / 7 min 21 s |
| 7 | Esqueleto Guerreiro + Esqueleto Ladino | 68 / 76 | 2,3 / 2,2 | 1,1 | ∞ | 1 min 08 s | 4% | 360 | 594 | 213.772 / 220.595 | 0 | 1 min 37 s / 1 min 34 s | 9 min 37 s / 8 min 56 s |
| 8 | Esqueleto Guerreiro + Esqueleto Ladino | 74 / 81 | 2,3 / 2,2 | 1,1 | ∞ | 1 min 09 s | 4% | 446 | 594 | 264.840 / 273.293 | 0 | 1 min 45 s / 1 min 42 s | 11 min 23 s / 10 min 38 s |
| 9 | Esqueleto Guerreiro + Esqueleto Ladino | 80 / 90 | 2,3 / 2,3 | 1,1 | ∞ | 1 min 34 s | 2% | 538 | 594 | 319.471 / 319.471 | 0 | 1 min 53 s / 1 min 53 s | 13 min 16 s / 12 min 31 s |
| 10 | Esqueleto Guerreiro + Esqueleto Ladino | 93 / 102 | 1,9 / 1,5 | 1,1 | ∞ | 1 min 39 s | 2% | 637 | 595 | 379.262 / 418.042 | 0 | 2 min 00 s / 1 min 49 s | 15 min 16 s / 14 min 20 s |
| 11 | Esqueleto Guerreiro + Esqueleto Ladino | 105 / 113 | 1,7 / 1,3 | 2,1 | ∞ | 1 min 43 s | 1% | 742 | 533 | 395.274 / 442.139 | 0 | 2 min 22 s / 2 min 07 s | 17 min 39 s / 16 min 28 s |
| 12 | Esqueleto Guerreiro + Esqueleto Ladino | 112 / 120 | 1,6 / 1,3 | 2,1 | ∞ | 1 min 50 s | 1% | 853 | 554 | 472.838 / 513.272 | 0 | 2 min 24 s / 2 min 12 s | 20 min 03 s / 18 min 41 s |
| 13 | Esqueleto Mago + Esqueleto Guerreiro **R** | 133 / 142 | 1,2 / 1,1 | 1,8 | ∞ | ∞ | 1% | 1.030 | 597 | 614.898 / 630.021 | 0 | 2 min 12 s / 2 min 09 s | 22 min 15 s / 20 min 50 s |
| 14 | Esqueleto Mago + Esqueleto Guerreiro **R** | 141 / 151 | 1,2 / 1,1 | 1,8 | ∞ | ∞ | 0% | 1.160 | 600 | 695.719 / 712.665 | 0 | 2 min 17 s / 2 min 14 s | 24 min 33 s / 23 min 04 s |
| 15 | Esqueleto Mago + Esqueleto Guerreiro **R** | 148 / 157 | 1,2 / 1,2 | 1,8 | ∞ | ∞ | 0% | 1.295 | 603 | 780.952 / 780.952 | 0 | 2 min 22 s / 2 min 22 s | 26 min 55 s / 25 min 27 s |
| 16 | Esqueleto Mago + Esqueleto Guerreiro **R** | 170 / 180 | 1,1 / 1,1 | 1,8 | ∞ | ∞ | 0% | 1.436 | 548 | 786.176 / 786.176 | 0 | 2 min 43 s / 2 min 43 s | 29 min 39 s / 28 min 10 s |
| 17 | Esqueleto Mago + Esqueleto Guerreiro **R** | 180 / 190 | 1,1 / 1,1 | 1,8 | ∞ | ∞ | 0% | 1.582 | 556 | 879.655 / 879.655 | 0 | 2 min 46 s / 2 min 46 s | 32 min 25 s / 30 min 57 s |
| 18 | Esqueleto Mago + Esqueleto Guerreiro **R** | 191 / 200 | 1,0 / 1,0 | 1,8 | ∞ | ∞ | 0% | 1.733 | 574 | 994.799 / 994.799 | 0 | 2 min 47 s / 2 min 47 s | 35 min 12 s / 33 min 44 s |
| 19 | Esqueleto Mago + Esqueleto Guerreiro **R** | 200 / 209 | 1,0 / 1,0 | 1,8 | ∞ | ∞ | 0% | 1.890 | 579 | 1.094.345 / 1.094.345 | 0 | 2 min 51 s / 2 min 51 s | 38 min 04 s / 36 min 35 s |

### Bárbaro/Sanguinário (20–30)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | Vale das Pedras Vivas **R** | 174 / 181 | 5,0 / 5,0 | 3,1 | ∞ | ∞ | 0% | 4.827 | 291 | 1.404.218 / 1.404.218 | 0 | 2 min 44 s / 2 min 44 s | 40 min 48 s / 39 min 19 s |
| 21 | Vale das Pedras Vivas **R** | 199 / 206 | 3,8 / 3,5 | 3,1 | ∞ | ∞ | 0% | 5.219 | 291 | 1.518.255 / 1.518.255 | 24 | 3 min 04 s / 3 min 04 s | 43 min 53 s / 42 min 24 s |
| 22 | Vale das Pedras Vivas **R** | 235 / 242 | 3,1 / 2,8 | 3,1 | ∞ | ∞ | 0% | 5.623 | 287 | 1.616.244 / 1.635.782 | 0 | 3 min 28 s / 3 min 25 s | 47 min 21 s / 45 min 49 s |
| 23 | Vale das Pedras Vivas **R** | 247 / 257 | 3,0 / 2,9 | 3,1 | ∞ | ∞ | 0% | 6.037 | 291 | 1.756.218 / 1.756.218 | 0 | 3 min 47 s / 3 min 47 s | 51 min 08 s / 49 min 37 s |
| 24 | Vale das Pedras Vivas **R** | 264 / 275 | 2,9 / 2,9 | 3,1 | ∞ | ∞ | 0% | 6.462 | 291 | 1.879.855 / 1.879.855 | 0 | 4 min 09 s / 4 min 09 s | 55 min 18 s / 53 min 47 s |
| 25 | Vale das Pedras Vivas **R** | 273 / 281 | 2,6 / 2,6 | 3,1 | ∞ | ∞ | 0% | 6.899 | 291 | 2.006.982 / 2.006.982 | 0 | 4 min 33 s / 4 min 33 s | 59 min 51 s / 58 min 20 s |
| 26 | Vale das Pedras Vivas **R** | 281 / 289 | 2,7 / 2,7 | 3,1 | ∞ | ∞ | 0% | 7.345 | 291 | 2.136.727 / 2.136.727 | 0 | 4 min 57 s / 4 min 57 s | 1 h 05 min / 1 h 03 min |
| 27 | Vale das Pedras Vivas **R** | 278 / 286 | 2,8 / 2,7 | 3,1 | ∞ | ∞ | 0% | 7.803 | 291 | 2.269.964 / 2.269.964 | 0 | 5 min 21 s / 5 min 21 s | 1 h 10 min / 1 h 09 min |
| 28 | Vale das Pedras Vivas **R** | 311 / 320 | 2,6 / 2,6 | 3,1 | ∞ | ∞ | 0% | 8.270 | 291 | 2.405.818 / 2.405.818 | 24 | 5 min 47 s / 5 min 47 s | 1 h 16 min / 1 h 14 min |
| 29 | Vale das Pedras Vivas **R** | 332 / 340 | 2,6 / 2,6 | 3,1 | ∞ | ∞ | 0% | 8.748 | 291 | 2.544.873 / 2.544.873 | 24 | 6 min 13 s / 6 min 13 s | 1 h 22 min / 1 h 21 min |
| 30 | Vale das Pedras Vivas **R** | 352 / 361 | 2,6 / 2,6 | 3,1 | ∞ | ∞ | 0% | 9.235 | 291 | 2.686.545 / 2.686.545 | 24 | 0 s / 0 s | 1 h 22 min / 1 h 21 min |

### Bárbaro/Caudilho (20–30)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | Vale das Pedras Vivas **R** | 165 / 172 | 5,5 / 5,1 | 3,1 | ∞ | ∞ | 0% | 4.827 | 290 | 1.397.505 / 1.404.218 | 24 | 2 min 45 s / 2 min 44 s | 40 min 49 s / 39 min 19 s |
| 21 | Vale das Pedras Vivas **R** | 188 / 196 | 4,9 / 3,8 | 3,1 | ∞ | ∞ | 0% | 5.219 | 288 | 1.502.799 / 1.518.255 | 72 | 3 min 06 s / 3 min 04 s | 43 min 55 s / 42 min 24 s |
| 22 | Vale das Pedras Vivas **R** | 205 / 212 | 4,5 / 4,1 | 3,5 | ∞ | ∞ | 0% | 5.623 | 285 | 1.603.942 / 1.635.782 | 71 | 3 min 29 s / 3 min 25 s | 47 min 25 s / 45 min 49 s |
| 23 | Vale das Pedras Vivas **R** | 214 / 222 | 4,3 / 4,0 | 3,5 | ∞ | ∞ | 0% | 6.037 | 291 | 1.756.218 / 1.730.131 | 48 | 3 min 47 s / 3 min 50 s | 51 min 12 s / 49 min 40 s |
| 24 | Vale das Pedras Vivas **R** | 239 / 247 | 3,8 / 3,4 | 3,5 | ∞ | ∞ | 0% | 6.462 | 291 | 1.879.855 / 1.841.005 | 73 | 4 min 09 s / 4 min 15 s | 55 min 22 s / 53 min 55 s |
| 25 | Vale das Pedras Vivas **R** | 244 / 252 | 3,4 / 3,4 | 3,5 | ∞ | ∞ | 0% | 6.899 | 279 | 1.926.601 / 1.926.601 | 47 | 4 min 44 s / 4 min 44 s | 1 h 00 min / 58 min 40 s |
| 26 | Vale das Pedras Vivas **R** | 257 / 264 | 3,4 / 3,4 | 3,5 | ∞ | ∞ | 0% | 7.345 | 282 | 2.072.454 / 2.072.454 | 47 | 5 min 06 s / 5 min 06 s | 1 h 05 min / 1 h 04 min |
| 27 | Vale das Pedras Vivas **R** | 270 / 278 | 3,6 / 3,4 | 3,5 | ∞ | ∞ | 0% | 7.803 | 290 | 2.264.847 / 2.265.170 | 48 | 5 min 22 s / 5 min 22 s | 1 h 11 min / 1 h 09 min |
| 28 | Vale das Pedras Vivas **R** | 283 / 290 | 3,5 / 3,3 | 3,5 | ∞ | ∞ | 0% | 8.270 | 291 | 2.405.818 / 2.393.026 | 73 | 5 min 47 s / 5 min 49 s | 1 h 16 min / 1 h 15 min |
| 29 | Vale das Pedras Vivas **R** | 293 / 301 | 3,4 / 3,3 | 3,5 | ∞ | ∞ | 0% | 8.748 | 291 | 2.544.873 / 2.543.645 | 73 | 6 min 13 s / 6 min 13 s | 1 h 23 min / 1 h 21 min |
| 30 | Vale das Pedras Vivas **R** | 304 / 312 | 3,6 / 3,1 | 3,5 | ∞ | ∞ | 0% | 9.235 | 284 | 2.627.278 / 2.666.499 | 71 | 0 s / 0 s | 1 h 23 min / 1 h 21 min |

### Mago (1–19)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | Esqueleto Lacaio | 32 / 36 | 0,2 / 0,2 | 0,1 | ∞ | 60 s | 0% | 12 | 894 | 10.726 / 10.726 | 0 | 27 s / 27 s | 27 s / 27 s |
| 2 | Esqueleto Lacaio | 39 / 43 | 0,3 / 0,3 | 0,1 | ∞ | 50 s | 0% | 36 | 1104 | 39.727 / 39.727 | 0 | 33 s / 33 s | 1 min 00 s / 1 min 00 s |
| 3 | Esqueleto Lacaio | 48 / 52 | 0,2 / 0,2 | 0,1 | ∞ | 44 s | 0% | 70 | 1218 | 85.266 / 85.266 | 0 | 38 s / 38 s | 1 min 38 s / 1 min 38 s |
| 4 | Esqueleto Lacaio | 55 / 59 | 0,2 / 0,2 | 0,1 | ∞ | 39 s | 0% | 110 | 1239 | 136.295 / 136.295 | 0 | 45 s / 45 s | 2 min 22 s / 2 min 22 s |
| 5 | Esqueleto Lacaio | 66 / 70 | 0,1 / 0,1 | 0,1 | ∞ | 36 s | 0% | 158 | 1261 | 199.197 / 199.197 | 0 | 50 s / 50 s | 3 min 12 s / 3 min 12 s |
| 6 | Esqueleto Guerreiro + Esqueleto Ladino | 92 / 101 | 0,0 / 0,0 | 0,1 | 1 min 03 s | 25 s | 0% | 281 | 956 | 268.713 / 268.713 | 0 | 55 s / 55 s | 4 min 07 s / 4 min 07 s |
| 7 | Esqueleto Guerreiro + Esqueleto Ladino | 105 / 113 | 0,0 / 0,0 | 0,1 | 52 s | 24 s | 0% | 360 | 956 | 344.259 / 344.259 | 0 | 1 min 00 s / 1 min 00 s | 5 min 08 s / 5 min 08 s |
| 8 | Esqueleto Guerreiro + Esqueleto Ladino | 134 / 146 | 0,0 / 0,0 | 0,1 | 42 s | 22 s | 0% | 446 | 956 | 426.499 / 426.499 | 0 | 1 min 05 s / 1 min 05 s | 6 min 13 s / 6 min 13 s |
| 9 | Esqueleto Guerreiro + Esqueleto Ladino | 148 / 161 | 0,5 / 0,0 | 0,6 | 39 s | 22 s | 0% | 538 | 844 | 454.157 / 514.476 | 0 | 1 min 19 s / 1 min 10 s | 7 min 33 s / 7 min 24 s |
| 10 | Esqueleto Guerreiro + Esqueleto Ladino | 167 / 180 | 0,5 / 0,0 | 0,6 | ∞ | 21 s | 0% | 637 | 844 | 537.729 / 609.148 | 0 | 1 min 24 s / 1 min 14 s | 8 min 58 s / 8 min 39 s |
| 11 | Esqueleto Guerreiro + Esqueleto Ladino | 181 / 193 | 0,5 / 0,0 | 0,6 | ∞ | 20 s | 0% | 742 | 844 | 626.365 / 709.556 | 0 | 1 min 29 s / 1 min 19 s | 10 min 28 s / 9 min 58 s |
| 12 | Esqueleto Guerreiro + Esqueleto Ladino | 200 / 210 | 0,5 / 0,0 | 0,6 | ∞ | 20 s | 0% | 853 | 844 | 720.067 / 815.703 | 0 | 1 min 34 s / 1 min 23 s | 12 min 02 s / 11 min 21 s |
| 13 | Esqueleto Mago + Esqueleto Guerreiro **R** | 217 / 230 | 0,5 / 0,5 | 0,6 | ∞ | 33 s | 0% | 1.030 | 703 | 723.434 / 723.434 | 0 | 1 min 52 s / 1 min 52 s | 13 min 55 s / 13 min 14 s |
| 14 | Esqueleto Mago + Esqueleto Guerreiro **R** | 237 / 250 | 0,5 / 0,5 | 0,6 | ∞ | 33 s | 0% | 1.160 | 703 | 814.785 / 814.785 | 0 | 1 min 57 s / 1 min 57 s | 15 min 52 s / 15 min 11 s |
| 15 | Esqueleto Mago + Esqueleto Guerreiro **R** | 267 / 277 | 0,5 / 0,5 | 0,1 | ∞ | 34 s | 0% | 1.295 | 706 | 914.809 / 914.809 | 0 | 2 min 01 s / 2 min 01 s | 17 min 54 s / 17 min 13 s |
| 16 | Esqueleto Mago + Esqueleto Guerreiro **R** | 291 / 302 | 0,5 / 0,5 | 0,1 | ∞ | 34 s | 0% | 1.436 | 706 | 1.014.061 / 1.014.061 | 0 | 2 min 06 s / 2 min 06 s | 20 min 01 s / 19 min 20 s |
| 17 | Esqueleto Mago + Esqueleto Guerreiro **R** | 308 / 321 | 0,5 / 0,5 | 0,1 | ∞ | 32 s | 0% | 1.582 | 706 | 1.117.551 / 1.117.551 | 0 | 2 min 11 s / 2 min 11 s | 22 min 12 s / 21 min 31 s |
| 18 | Esqueleto Mago + Esqueleto Guerreiro **R** | 327 / 341 | 0,5 / 0,5 | 0,1 | ∞ | 32 s | 0% | 1.733 | 706 | 1.224.220 / 1.224.220 | 0 | 2 min 15 s / 2 min 15 s | 24 min 28 s / 23 min 47 s |
| 19 | Esqueleto Mago + Esqueleto Guerreiro **R** | 342 / 350 | 0,6 / 0,6 | 1,1 | ∞ | 35 s | 0% | 1.890 | 692 | 1.308.464 / 1.308.464 | 0 | 2 min 23 s / 2 min 23 s | 26 min 51 s / 26 min 10 s |

### Mago/Elementalista (20–30)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | Vale das Pedras Vivas **R** | 376 / 391 | 1,8 / 1,8 | 1,1 | ∞ | 38 s | 0% | 4.827 | 291 | 1.404.218 / 1.404.218 | 0 | 2 min 44 s / 2 min 44 s | 29 min 35 s / 28 min 54 s |
| 21 | Vale das Pedras Vivas **R** | 397 / 409 | 1,8 / 1,8 | 1,1 | ∞ | 38 s | 0% | 5.219 | 291 | 1.518.255 / 1.518.255 | 0 | 3 min 04 s / 3 min 04 s | 32 min 40 s / 31 min 59 s |
| 22 | Vale das Pedras Vivas **R** | 417 / 430 | 1,8 / 1,8 | 1,1 | ∞ | 38 s | 0% | 5.623 | 291 | 1.635.782 / 1.635.782 | 0 | 3 min 25 s / 3 min 25 s | 36 min 05 s / 35 min 24 s |
| 23 | Vale das Pedras Vivas **R** | 442 / 455 | 1,8 / 1,8 | 1,1 | ∞ | 38 s | 0% | 6.037 | 291 | 1.756.218 / 1.756.218 | 0 | 3 min 47 s / 3 min 47 s | 39 min 53 s / 39 min 12 s |
| 24 | Vale das Pedras Vivas **R** | 467 / 479 | 1,8 / 1,5 | 1,1 | ∞ | 36 s | 0% | 6.462 | 291 | 1.879.855 / 1.879.855 | 0 | 4 min 09 s / 4 min 09 s | 44 min 03 s / 43 min 22 s |
| 25 | Vale das Pedras Vivas **R** | 494 / 506 | 1,5 / 1,5 | 1,1 | ∞ | 36 s | 0% | 6.899 | 291 | 2.006.982 / 2.006.982 | 0 | 4 min 33 s / 4 min 33 s | 48 min 36 s / 47 min 55 s |
| 26 | Vale das Pedras Vivas **R** | 522 / 534 | 1,5 / 1,5 | 1,1 | ∞ | 36 s | 0% | 7.345 | 291 | 2.136.727 / 2.136.727 | 0 | 4 min 57 s / 4 min 57 s | 53 min 33 s / 52 min 52 s |
| 27 | Vale das Pedras Vivas **R** | 552 / 555 | 1,5 / 1,5 | 1,1 | ∞ | 36 s | 0% | 7.803 | 291 | 2.269.964 / 2.269.964 | 0 | 5 min 21 s / 5 min 21 s | 58 min 55 s / 58 min 14 s |
| 28 | Vale das Pedras Vivas **R** | 571 / 584 | 1,5 / 1,5 | 1,1 | ∞ | 36 s | 0% | 8.270 | 291 | 2.405.818 / 2.405.818 | 0 | 5 min 47 s / 5 min 47 s | 1 h 05 min / 1 h 04 min |
| 29 | Vale das Pedras Vivas **R** | 580 / 592 | 1,7 / 1,7 | 2,1 | ∞ | 41 s | 0% | 8.748 | 291 | 2.544.873 / 2.544.873 | 0 | 6 min 13 s / 6 min 13 s | 1 h 11 min / 1 h 10 min |
| 30 | Vale das Pedras Vivas **R** | 601 / 613 | 1,7 / 1,7 | 2,1 | ∞ | 43 s | 0% | 9.235 | 291 | 2.686.545 / 2.686.545 | 0 | 0 s / 0 s | 1 h 11 min / 1 h 10 min |

### Mago/Ocultista (20–30)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | Vale das Pedras Vivas **R** | 356 / 371 | 1,8 / 1,8 | 1,1 | ∞ | 38 s | 0% | 4.827 | 291 | 1.404.218 / 1.404.218 | 0 | 2 min 44 s / 2 min 44 s | 29 min 35 s / 28 min 54 s |
| 21 | Vale das Pedras Vivas **R** | 372 / 381 | 1,8 / 1,8 | 1,1 | ∞ | 38 s | 0% | 5.219 | 291 | 1.518.255 / 1.518.255 | 0 | 3 min 04 s / 3 min 04 s | 32 min 40 s / 31 min 59 s |
| 22 | Vale das Pedras Vivas **R** | 393 / 404 | 1,8 / 1,8 | 1,1 | ∞ | 38 s | 0% | 5.623 | 291 | 1.635.782 / 1.635.782 | 0 | 3 min 25 s / 3 min 25 s | 36 min 05 s / 35 min 24 s |
| 23 | Vale das Pedras Vivas **R** | 426 / 438 | 1,8 / 1,8 | 1,1 | ∞ | 38 s | 0% | 6.037 | 291 | 1.756.218 / 1.756.218 | 0 | 3 min 47 s / 3 min 47 s | 39 min 53 s / 39 min 12 s |
| 24 | Vale das Pedras Vivas **R** | 445 / 461 | 1,8 / 1,8 | 1,1 | ∞ | 43 s | 0% | 6.462 | 291 | 1.879.855 / 1.879.855 | 0 | 4 min 09 s / 4 min 09 s | 44 min 03 s / 43 min 22 s |
| 25 | Vale das Pedras Vivas **R** | 486 / 497 | 1,8 / 1,8 | 1,1 | ∞ | 41 s | 0% | 6.899 | 291 | 2.006.982 / 2.006.982 | 0 | 4 min 33 s / 4 min 33 s | 48 min 36 s / 47 min 55 s |
| 26 | Vale das Pedras Vivas **R** | 497 / 509 | 1,8 / 1,8 | 1,1 | ∞ | 47 s | 0% | 7.345 | 291 | 2.136.727 / 2.136.727 | 0 | 4 min 57 s / 4 min 57 s | 53 min 33 s / 52 min 52 s |
| 27 | Vale das Pedras Vivas **R** | 517 / 530 | 1,8 / 1,8 | 1,1 | ∞ | 45 s | 0% | 7.803 | 291 | 2.269.964 / 2.269.964 | 0 | 5 min 21 s / 5 min 21 s | 58 min 55 s / 58 min 14 s |
| 28 | Vale das Pedras Vivas **R** | 555 / 568 | 1,9 / 1,9 | 1,7 | ∞ | 46 s | 0% | 8.270 | 291 | 2.405.818 / 2.405.818 | 0 | 5 min 47 s / 5 min 47 s | 1 h 05 min / 1 h 04 min |
| 29 | Vale das Pedras Vivas **R** | 562 / 574 | 2,1 / 2,1 | 2,1 | ∞ | 54 s | 2% | 8.748 | 291 | 2.544.873 / 2.544.873 | 0 | 6 min 13 s / 6 min 13 s | 1 h 11 min / 1 h 10 min |
| 30 | Vale das Pedras Vivas **R** | 584 / 596 | 2,1 / 2,1 | 2,1 | ∞ | 58 s | 2% | 9.235 | 291 | 2.686.545 / 2.686.545 | 0 | 0 s / 0 s | 1 h 11 min / 1 h 10 min |

### Ladino (1–19)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | Esqueleto Lacaio | 27 / 33 | 1,2 / 0,8 | 0,1 | ∞ | 1 min 24 s | 2% | 12 | 968 | 11.619 / 13.019 | 0 | 25 s / 22 s | 25 s / 22 s |
| 2 | Esqueleto Lacaio | 32 / 38 | 1,2 / 1,2 | 0,1 | ∞ | 1 min 08 s | 3% | 36 | 968 | 34.856 / 34.856 | 0 | 38 s / 38 s | 1 min 02 s / 1 min 00 s |
| 3 | Esqueleto Lacaio | 38 / 44 | 1,6 / 1,2 | 0,1 | ∞ | 1 min 00 s | 3% | 70 | 874 | 61.193 / 67.776 | 0 | 53 s / 48 s | 1 min 55 s / 1 min 47 s |
| 4 | Esqueleto Lacaio | 42 / 48 | 1,6 / 1,6 | 0,1 | ∞ | 56 s | 3% | 110 | 874 | 96.160 / 96.160 | 0 | 1 min 03 s / 1 min 03 s | 2 min 58 s / 2 min 51 s |
| 5 | Esqueleto Lacaio | 50 / 56 | 1,5 / 1,5 | 0,8 | ∞ | 53 s | 5% | 158 | 639 | 101.010 / 141.558 | 0 | 1 min 38 s / 1 min 10 s | 4 min 37 s / 4 min 01 s |
| 6 | Esqueleto Guerreiro + Esqueleto Ladino | 72 / 85 | 1,9 / 1,9 | 0,4 | 2 min 47 s | 32 s | 5% | 281 | 655 | 184.120 / 190.620 | 0 | 1 min 20 s / 1 min 17 s | 5 min 57 s / 5 min 18 s |
| 7 | Esqueleto Guerreiro + Esqueleto Ladino | 77 / 90 | 1,8 / 1,9 | 0,4 | 1 min 35 s | 30 s | 5% | 360 | 641 | 230.895 / 241.167 | 0 | 1 min 30 s / 1 min 26 s | 7 min 27 s / 6 min 45 s |
| 8 | Esqueleto Guerreiro + Esqueleto Ladino | 84 / 96 | 1,8 / 1,9 | 0,4 | 1 min 06 s | 29 s | 5% | 446 | 636 | 283.562 / 294.445 | 0 | 1 min 38 s / 1 min 34 s | 9 min 06 s / 8 min 20 s |
| 9 | Esqueleto Guerreiro + Esqueleto Ladino | 94 / 107 | 1,7 / 1,9 | 0,8 | 1 min 02 s | 28 s | 5% | 538 | 637 | 342.499 / 348.764 | 0 | 1 min 45 s / 1 min 43 s | 10 min 52 s / 10 min 04 s |
| 10 | Esqueleto Guerreiro + Esqueleto Ladino | 112 / 128 | 1,2 / 0,8 | 0,6 | ∞ | 27 s | 4% | 637 | 776 | 494.196 / 542.421 | 0 | 1 min 32 s / 1 min 24 s | 12 min 24 s / 11 min 28 s |
| 11 | Esqueleto Guerreiro + Esqueleto Ladino | 121 / 135 | 1,2 / 0,9 | 0,9 | ∞ | 27 s | 4% | 742 | 766 | 568.176 / 627.502 | 0 | 1 min 39 s / 1 min 29 s | 14 min 03 s / 12 min 57 s |
| 12 | Esqueleto Guerreiro + Esqueleto Ladino | 128 / 142 | 1,2 / 1,2 | 0,9 | ∞ | 26 s | 4% | 853 | 763 | 651.079 / 673.882 | 0 | 1 min 44 s / 1 min 41 s | 15 min 48 s / 14 min 39 s |
| 13 | Esqueleto Mago + Esqueleto Guerreiro **R** | 144 / 160 | 0,9 / 0,7 | 0,6 | ∞ | 51 s | 3% | 1.030 | 660 | 679.518 / 722.833 | 0 | 1 min 59 s / 1 min 52 s | 17 min 47 s / 16 min 31 s |
| 14 | Esqueleto Mago + Esqueleto Guerreiro **R** | 150 / 167 | 1,1 / 0,9 | 1,5 | ∞ | 51 s | 4% | 1.160 | 625 | 724.778 / 813.985 | 0 | 2 min 12 s / 1 min 57 s | 19 min 59 s / 18 min 29 s |
| 15 | Esqueleto Mago + Esqueleto Guerreiro **R** | 167 / 185 | 1,1 / 0,9 | 1,7 | ∞ | 50 s | 4% | 1.295 | 633 | 819.289 / 901.781 | 0 | 2 min 15 s / 2 min 03 s | 22 min 15 s / 20 min 32 s |
| 16 | Esqueleto Mago + Esqueleto Guerreiro **R** | 175 / 192 | 1,1 / 0,9 | 1,7 | ∞ | 50 s | 4% | 1.436 | 632 | 906.858 / 999.619 | 0 | 2 min 21 s / 2 min 08 s | 24 min 37 s / 22 min 41 s |
| 17 | Esqueleto Mago + Esqueleto Guerreiro **R** | 184 / 200 | 1,1 / 1,0 | 1,7 | ∞ | 50 s | 4% | 1.582 | 621 | 982.368 / 1.076.051 | 0 | 2 min 29 s / 2 min 16 s | 27 min 06 s / 24 min 57 s |
| 18 | Esqueleto Mago + Esqueleto Guerreiro **R** | 201 / 218 | 1,1 / 1,0 | 1,6 | ∞ | 50 s | 4% | 1.733 | 624 | 1.081.425 / 1.191.421 | 0 | 2 min 33 s / 2 min 19 s | 29 min 40 s / 27 min 16 s |
| 19 | Esqueleto Mago + Esqueleto Guerreiro **R** | 208 / 225 | 1,1 / 1,1 | 1,6 | ∞ | 50 s | 4% | 1.890 | 622 | 1.176.004 / 1.208.051 | 0 | 2 min 39 s / 2 min 35 s | 32 min 19 s / 29 min 52 s |

### Ladino/Sicário (20–30)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | Vale das Pedras Vivas **R** | 205 / 221 | 3,2 / 3,2 | 3,1 | ∞ | 1 min 04 s | 9% | 4.827 | 291 | 1.404.218 / 1.404.218 | 0 | 2 min 44 s / 2 min 44 s | 35 min 04 s / 32 min 36 s |
| 21 | Vale das Pedras Vivas **R** | 215 / 231 | 3,2 / 3,1 | 3,1 | ∞ | 1 min 04 s | 9% | 5.219 | 291 | 1.518.255 / 1.518.255 | 0 | 3 min 04 s / 3 min 04 s | 38 min 08 s / 35 min 40 s |
| 22 | Vale das Pedras Vivas **R** | 224 / 241 | 3,2 / 3,1 | 3,1 | ∞ | 1 min 04 s | 9% | 5.623 | 291 | 1.635.782 / 1.635.782 | 0 | 3 min 25 s / 3 min 25 s | 41 min 34 s / 39 min 06 s |
| 23 | Vale das Pedras Vivas **R** | 235 / 251 | 3,1 / 3,1 | 3,1 | ∞ | 1 min 04 s | 9% | 6.037 | 291 | 1.756.218 / 1.756.218 | 0 | 3 min 47 s / 3 min 47 s | 45 min 21 s / 42 min 53 s |
| 24 | Vale das Pedras Vivas **R** | 252 / 268 | 3,1 / 3,0 | 3,1 | ∞ | 1 min 04 s | 9% | 6.462 | 291 | 1.879.855 / 1.879.855 | 0 | 4 min 09 s / 4 min 09 s | 49 min 31 s / 47 min 03 s |
| 25 | Vale das Pedras Vivas **R** | 284 / 302 | 3,1 / 2,8 | 3,1 | ∞ | 1 min 07 s | 9% | 6.899 | 291 | 2.006.982 / 2.006.982 | 0 | 4 min 33 s / 4 min 33 s | 54 min 04 s / 51 min 36 s |
| 26 | Vale das Pedras Vivas **R** | 304 / 321 | 2,8 / 2,6 | 3,1 | ∞ | 1 min 07 s | 9% | 7.345 | 291 | 2.136.727 / 2.136.727 | 0 | 4 min 57 s / 4 min 57 s | 59 min 01 s / 56 min 34 s |
| 27 | Vale das Pedras Vivas **R** | 312 / 330 | 2,8 / 2,8 | 3,1 | ∞ | 1 min 07 s | 9% | 7.803 | 291 | 2.269.964 / 2.269.964 | 0 | 5 min 21 s / 5 min 21 s | 1 h 04 min / 1 h 02 min |
| 28 | Vale das Pedras Vivas **R** | 328 / 347 | 2,7 / 2,6 | 3,1 | ∞ | 1 min 07 s | 8% | 8.270 | 291 | 2.405.818 / 2.405.818 | 0 | 5 min 47 s / 5 min 47 s | 1 h 10 min / 1 h 08 min |
| 29 | Vale das Pedras Vivas **R** | 338 / 355 | 2,7 / 2,6 | 3,1 | ∞ | 1 min 09 s | 9% | 8.748 | 291 | 2.544.873 / 2.544.873 | 0 | 6 min 13 s / 6 min 13 s | 1 h 16 min / 1 h 14 min |
| 30 | Vale das Pedras Vivas **R** | 346 / 364 | 3,0 / 2,6 | 3,1 | ∞ | 1 min 09 s | 8% | 9.235 | 291 | 2.686.545 / 2.686.545 | 0 | 0 s / 0 s | 1 h 16 min / 1 h 14 min |

### Ladino/Caçador (20–30)

| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | Vale das Pedras Vivas **R** | 209 / 225 | 3,4 / 3,2 | 3,1 | ∞ | 1 min 02 s | 9% | 4.827 | 291 | 1.404.218 / 1.404.218 | 0 | 2 min 44 s / 2 min 44 s | 35 min 04 s / 32 min 36 s |
| 21 | Vale das Pedras Vivas **R** | 226 / 242 | 3,2 / 3,1 | 3,1 | ∞ | 1 min 02 s | 9% | 5.219 | 291 | 1.518.255 / 1.518.255 | 0 | 3 min 04 s / 3 min 04 s | 38 min 08 s / 35 min 40 s |
| 22 | Vale das Pedras Vivas **R** | 236 / 252 | 3,2 / 3,1 | 3,1 | ∞ | 1 min 02 s | 9% | 5.623 | 291 | 1.635.782 / 1.635.782 | 0 | 3 min 25 s / 3 min 25 s | 41 min 34 s / 39 min 06 s |
| 23 | Vale das Pedras Vivas **R** | 246 / 262 | 3,3 / 3,1 | 3,1 | ∞ | 1 min 02 s | 9% | 6.037 | 291 | 1.756.218 / 1.756.218 | 0 | 3 min 47 s / 3 min 47 s | 45 min 21 s / 42 min 53 s |
| 24 | Vale das Pedras Vivas **R** | 255 / 271 | 3,3 / 3,1 | 3,1 | ∞ | 1 min 04 s | 9% | 6.462 | 291 | 1.879.855 / 1.879.855 | 0 | 4 min 09 s / 4 min 09 s | 49 min 31 s / 47 min 03 s |
| 25 | Vale das Pedras Vivas **R** | 263 / 278 | 3,3 / 3,1 | 3,1 | ∞ | 1 min 04 s | 9% | 6.899 | 291 | 2.006.982 / 2.006.982 | 0 | 4 min 33 s / 4 min 33 s | 54 min 04 s / 51 min 36 s |
| 26 | Vale das Pedras Vivas **R** | 270 / 285 | 3,3 / 3,1 | 3,1 | ∞ | 1 min 04 s | 8% | 7.345 | 291 | 2.136.727 / 2.136.727 | 0 | 4 min 57 s / 4 min 57 s | 59 min 01 s / 56 min 34 s |
| 27 | Vale das Pedras Vivas **R** | 277 / 292 | 3,3 / 3,1 | 3,1 | ∞ | 1 min 04 s | 8% | 7.803 | 291 | 2.269.964 / 2.269.964 | 0 | 5 min 21 s / 5 min 21 s | 1 h 04 min / 1 h 02 min |
| 28 | Vale das Pedras Vivas **R** | 309 / 325 | 3,1 / 3,1 | 3,1 | ∞ | 1 min 07 s | 8% | 8.270 | 291 | 2.405.818 / 2.405.818 | 0 | 5 min 47 s / 5 min 47 s | 1 h 10 min / 1 h 08 min |
| 29 | Vale das Pedras Vivas **R** | 317 / 333 | 3,2 / 3,1 | 3,1 | ∞ | 1 min 07 s | 8% | 8.748 | 291 | 2.544.873 / 2.544.873 | 0 | 6 min 13 s / 6 min 13 s | 1 h 16 min / 1 h 14 min |
| 30 | Vale das Pedras Vivas **R** | 325 / 340 | 3,3 / 3,1 | 3,1 | ∞ | 1 min 07 s | 8% | 9.235 | 291 | 2.686.545 / 2.686.545 | 0 | 0 s / 0 s | 1 h 16 min / 1 h 14 min |

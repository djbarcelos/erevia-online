# Berserker (maestria de Bárbaro)

> Gerado do protótipo [Maestrias de Erevia](https://claude.ai/artifact/4P5N9Bg2LCeP7n4bjdKkAZ) e de [docs/evolucoes/bloodrager.md](../evolucoes/bloodrager.md). As regras gerais estão no [README](README.md). Os números são os do doc da evolução, espalhados pelos ranks do sistema de maestrias, e ainda vão ser calibrados no simulador.

- **ID:** `bloodrager` · **classe-base:** Bárbaro · **recurso:** Fúria
- **Barra da maestria:** cada ponto dá +1 For e +8 de vida (no 50: +50 For, +400 de vida).
- **Papel:** **DPS físico corpo a corpo:** pressão sustentada, explosão de dano e execução. O Berserker aceita riscos para manter a pressão. Ele é recompensado por atacar e ficar na luta, não por esperar. **O ciclo principal não depende de MP:** é a evolução que resolve o "Bárbaro sem mana" do design (seção 5).
- **Duplas:** Bastião → **Colosso** · Paladino → **Zelote** · Senhor da Guerra → **Chefe de Clã** · Elementalista → **Incendiário** · Arcanista → **Possesso** · Assassino → **Carniceiro** · Patrulheiro → **Predador**

## Árvore

Colunas = pontos na barra que liberam a skill. ■ ativa · ● passiva · ○ modificador (ligado à skill da linha) · ◆ chave.

| 1 | 5 | 10 | 15 | 20 | 25 | 32 | 40 | 50 |
|---|---|---|---|---|---|---|---|---|
| ■ Golpe Brutal |  |  | ○ Golpe Esmagador | ■ Ruptura |  |  |  | ◆ Ira Desmedida |
|  | ■ Machadada Sísmica |  |  |  |  | ○ Abalo |  |  |
| ■ Cólera | ■ Frenesi | ● Machado Pesado |  |  | ● Sequência Selvagem |  | ■ Fúria Incontrolável | ◆ Sede Insaciável |
| ● Sede de Sangue | ● Fúria Cega | ■ Redemoinho Sangrento | ○ Turbilhão | ■ Execução |  |  | ■ Carnificina | ◆ Carnificina Final |
| ■ Golpe Rubro |  |  | ■ Marca da Presa |  |  |  |  |  |

**Total:** 21 skills, 199 ranks.

## Recurso: Fúria

| Gera | Quanto |
|---|---|
| Ataque básico acertado | +6 (+12 com Fúria Incontrolável) |
| Skill acertada (por alvo, até 3) | +4 |
| Crítico | +3 |
| Receber dano de inimigo | +2 por golpe |
| Cólera | +20 ao lançar |
| Golpe Brutal | +10 (+5→15 a mais se for crítico, com Golpe Esmagador) |
| Matar o alvo da Marca da Presa | +30 |

| Gasta | Quanto |
|---|---|
| Golpe Rubro | 20 |
| Machadada Sísmica | 25 |
| Redemoinho Sangrento | 30 |
| Ruptura | 30 |
| Execução | 35 |
| Carnificina | 50 |

## Skills

Na ordem da árvore: coluna por coluna, de cima para baixo.

### Golpe Brutal

Ativa · coluna **1** · **12** ranks · `SkillBrutalStrike` (117) · ícone `brutal_strike`

> Um golpe com toda a força do corpo.

**Efeito:** 220→280% do ataque (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 220→280% | 220% | +5,45% | 280% | `Scale{2.2, 0.0545}` |

- **Custo / recarga:** 10 MP / 6 s
- **Requisito:** 1 ponto na barra de Berserker.
- **Modificadores ligados:** Golpe Esmagador.
- **Origem:** evol..
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Cólera

Ativa · coluna **1** · **12** ranks · `SkillRagerWrath` (123) · ícone novo (Meshy)

**Efeito:** +30→50% de ataque por 10 s; gera 20 Fúria (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 30→50% | 30% | +1,82% | 50% | `Scale{0.3, 0.0182}` |

- **Custo / recarga:** 15 MP / 25 s
- **Requisito:** 1 ponto na barra de Berserker.
- **Origem:** evol. da Fúria.
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Sede de Sangue

Passiva · coluna **1** · **10** ranks · `SkillRagerThirst` (124) · ícone `b_thirst`

> Cada ferida aberta alimenta o bárbaro.

**Efeito:** Recupera 2→10% do dano causado como vida (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |

- **Custo / recarga:** —
- **Requisito:** 1 ponto na barra de Berserker.
- **Origem:** evol. (de `SkillBloodthirst`).
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Golpe Rubro

Ativa · coluna **1** · **12** ranks · `SkillRagerRed` (67) · ícone `bb_red`

> Quanto mais o inimigo sangra, mais fundo o machado entra.

**Efeito:** 260→330%; +30% em alvos abaixo de 50% de vida (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 260→330% | 260% | +6,36% | 330% | `Scale{2.6, 0.0636}` |

- **Custo / recarga:** **20 Fúria** / 10 s (hoje 25 MP)
- **Requisito:** 1 ponto na barra de Berserker.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Machadada Sísmica

Ativa · coluna **5** · **12** ranks · `SkillSeismicAxe` (52) · ícone `b_seismic`

> O machado desce com tanta força que o chão em volta explode.

**Efeito:** 240→300% no alvo e a até 3 m; +50% em quem estiver abaixo de 30% de vida (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 240→300% | 240% | +5,45% | 300% | `Scale{2.4, 0.0545}` |

- **Custo / recarga:** **25 Fúria** / 16 s (hoje 35 MP)
- **Requisito:** 5 pontos na barra de Berserker.
- **Modificadores ligados:** Abalo.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Frenesi

Ativa · coluna **5** · **12** ranks · `SkillRagerFrenzy` (125) · ícone `b_frenzy`

> Os golpes viram um borrão.

**Efeito:** +25→45% de velocidade de ataque por 12 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 25→45% | 25% | +1,82% | 45% | `Scale{0.25, 0.0182}` |

- **Custo / recarga:** 20 MP / 25 s
- **Requisito:** 5 pontos na barra de Berserker.
- **Origem:** evol. (de `SkillFrenzy`).
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Fúria Cega

Passiva · coluna **5** · **10** ranks · `SkillRagerBlind` (68) · ícone `bb_blind`

> Não mira: acerta onde dói.

**Efeito:** +2→10% de chance de crítico (hoje 3→9%) (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |
| 3→9% | 3% | +0,67% | 9% | `Scale{0.03, 0.0067}` |

- **Custo / recarga:** —
- **Requisito:** 5 pontos na barra de Berserker.
- **No doc da evolução:** marco 20, 5 (hoje 3) ranks, requisito "—".

### Machado Pesado

Passiva · coluna **10** · **10** ranks · `SkillRagerHeavy` (118) · ícone `b_heavy`

> Cada golpe crítico abre o inimigo ao meio.

**Efeito:** +8→40% de dano crítico (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 8→40% | 8% | +3,56% | 40% | `Scale{0.08, 0.0356}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Berserker.
- **Origem:** evol. (de `SkillHeavyAxe`).
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Redemoinho Sangrento

Ativa · coluna **10** · **12** ranks · `SkillBloodWhirl` (53) · ícone `b_whirl`

> Gira o machado duas vezes, sem escolher em quem acerta.

**Efeito:** 2 golpes de 90→120% em todos a até 4 m; a Sede de Sangue cura a cada golpe (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 90→120% | 90% | +2,73% | 120% | `Scale{0.9, 0.0273}` |

- **Custo / recarga:** **30 Fúria** / 12 s (hoje 30 MP)
- **Requisito:** 10 pontos na barra de Berserker.
- **Modificadores ligados:** Turbilhão.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Golpe Esmagador

Modificador · coluna **15** · **8** ranks · `SkillRagerCrush` (119) · ícone novo (Meshy); hoje usa `k_spin`, emprestado

> O golpe brutal quebra o que encontra.

**Efeito:** +10→30% de dano; no crítico, +5→15 Fúria (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 10→30% | 10% | +2,86% | 30% | `Scale{0.1, 0.0286}` |
| 5→15 | 5 | +1,43 | 15 | `Scale{5, 1.4286}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Berserker; 1 ponto em **Golpe Brutal**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Golpe Brutal 1".

### Turbilhão

Modificador · coluna **15** · **8** ranks · `SkillRagerVortex` (126) · ícone novo (Meshy); hoje usa `k_charge`, emprestado

> O redemoinho não para.

**Efeito:** +10% / +20% de dano; no rank 3, +1 golpe (3 golpes)

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Berserker; 1 ponto em **Redemoinho Sangrento**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Redemoinho 1".

### Marca da Presa

Ativa · coluna **15** · **12** ranks · `SkillRagerPrey` (130) · ícone novo (Meshy); hoje usa `r_mark`, emprestado

> Escolhe a presa.

**Efeito:** Marca o alvo por 10 s: +8→16% de dano seu nele; matá-lo devolve 30 Fúria (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 8→16% | 8% | +0,73% | 16% | `Scale{0.08, 0.0073}` |

- **Custo / recarga:** 10 MP / 15 s
- **Requisito:** 15 pontos na barra de Berserker.
- **No doc da evolução:** marco 25, 3 ranks, requisito "—".

### Ruptura

Ativa · coluna **20** · **12** ranks · `SkillRagerRupture` (120) · ícone novo (Meshy); hoje usa `r_twin`, emprestado

> Abre uma ferida que não fecha.

**Efeito:** 160→200% e sangramento de 30→50% do ataque por segundo, por 6 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 160→200% | 160% | +3,64% | 200% | `Scale{1.6, 0.0364}` |
| 30→50% | 30% | +1,82% | 50% | `Scale{0.3, 0.0182}` |

- **Custo / recarga:** **30 Fúria** / 12 s
- **Requisito:** 20 pontos na barra de Berserker.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Execução

Ativa · coluna **20** · **12** ranks · `SkillRagerExecute` (131) · ícone `b_exec`

> Termina o serviço.

**Efeito:** 300→350%; o dobro em alvos abaixo de 30% de vida (em chefes, no máximo +50%) (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 300→350% | 300% | +4,55% | 350% | `Scale{3, 0.0455}` |

- **Custo / recarga:** **35 Fúria** / 25 s
- **Requisito:** 20 pontos na barra de Berserker.
- **Origem:** evol. (de `SkillExecute`).
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Sequência Selvagem

Passiva · coluna **25** · **10** ranks · `SkillRagerSequence` (127) · ícone novo (Meshy); hoje usa `r_dodge`, emprestado

> Cada golpe chama o próximo.

**Efeito:** Cada ataque básico seguido no mesmo alvo dá +2→4% de velocidade de ataque, até 5 acúmulos (perde tudo após 3 s sem bater) (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→4% | 2% | +0,22% | 4% | `Scale{0.02, 0.0022}` |

- **Custo / recarga:** —
- **Requisito:** 25 pontos na barra de Berserker.
- **No doc da evolução:** marco 30, 5 ranks, requisito "5 no ramo".

### Abalo

Modificador · coluna **32** · **8** ranks · `SkillRagerTremor` (121) · ícone novo (Meshy); hoje usa `b_stomp`, emprestado

> O chão treme com a Machadada.

**Efeito:** +1→3 m de raio; os atingidos ficam 30% mais lentos por 2→4 s (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 1→3 | 1 | +0,29 | 3 | `Scale{1, 0.2857}` |
| 2→4 | 2 | +0,29 | 4 | `Scale{2, 0.2857}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Berserker; 1 ponto em **Machadada Sísmica**.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Fúria Incontrolável

Ativa · coluna **40** · **12** ranks · `SkillRagerUnchained` (128) · ícone novo (Meshy); hoje usa `b_roar`, emprestado

> Solta tudo, e não se protege de nada.

**Efeito:** Por 8 s, os ataques básicos geram o dobro de Fúria, e **você recebe +10% de dano**

- **Custo / recarga:** 20 MP / 40→30 s
- **Requisito:** 40 pontos na barra de Berserker.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Carnificina

Ativa · coluna **40** · **12** ranks · `SkillRagerCarnage` (69) · ícone `bb_carnage`

> Três voltas de machado, sem olhar para trás.

**Efeito:** 3 golpes de 110→140% em todos a até 5 m (a Sede de Sangue cura a cada golpe) (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 110→140% | 110% | +2,73% | 140% | `Scale{1.1, 0.0273}` |

- **Custo / recarga:** **50 Fúria** / 45 s
- **Requisito:** 40 pontos na barra de Berserker.
- **No doc da evolução:** marco 35, 3 (hoje 1) ranks, requisito "10 no ramo".

### Ira Desmedida

Chave · coluna **50** · **1** rank · `SkillRagerBoundless` (122) · ícone novo (Meshy); hoje usa `m_over`, emprestado

> A fúria não acaba nunca.

**Efeito:** Skills que gastam Fúria custam 30% menos e têm +15% de chance de crítico

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Berserker.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Sede Insaciável

Chave · coluna **50** · **1** rank · `SkillRagerInsatiate` (129) · ícone novo (Meshy); hoje usa `b_undying`, emprestado

> Quanto mais sangra, mais bate.

**Efeito:** Abaixo de 50% de vida, a Sede de Sangue cura o dobro e você ganha +20% de velocidade de ataque

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Berserker.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Carnificina Final

Chave · coluna **50** · **1** rank · `SkillRagerFinale` (132) · ícone novo (Meshy); hoje usa `r_fatal`, emprestado

> Quem cai abre caminho para o próximo.

**Efeito:** Matar com Execução ou Golpe Rubro zera a recarga dele (uma vez a cada 10 s)

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Berserker.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".


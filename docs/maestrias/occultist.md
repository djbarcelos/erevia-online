# Arcanista (maestria de Mago)

> Gerado do protótipo [Maestrias de Erevia](https://claude.ai/artifact/4P5N9Bg2LCeP7n4bjdKkAZ) e de [docs/evolucoes/occultist.md](../evolucoes/occultist.md). As regras gerais estão no [README](README.md). Os números são os do doc da evolução, espalhados pelos ranks do sistema de maestrias, e ainda vão ser calibrados no simulador.

- **ID:** `occultist` · **classe-base:** Mago · **recurso:** Carga Arcana
- **Barra da maestria:** cada ponto dá +0,5 Int, +0,5 Esp e +8 de vida (no 50: +25 Int, +25 Esp, +400 de vida).
- **Papel:** Controla o ritmo da luta: maldições, pragas, marcas e manipulação arcana. Mata mais devagar que o Elementalista contra um alvo só, e brilha em grupo e contra muitos inimigos, quando as pragas se espalham e as marcas aumentam o dano de todos.
- **Duplas:** Bastião → **Guardião do Véu** · Paladino → **Exorcista** · Berserker → **Possesso** · Senhor da Guerra → **Tirano** · Elementalista → **Arquimago** · Assassino → **Bruxo** · Patrulheiro → **Pestilento**

## Árvore

Colunas = pontos na barra que liberam a skill. ■ ativa · ● passiva · ○ modificador (ligado à skill da linha) · ◆ chave.

| 1 | 5 | 10 | 15 | 20 | 25 | 32 | 40 | 50 |
|---|---|---|---|---|---|---|---|---|
| ■ Drenar Vontade |  |  | ○ Ruína | ■ Maldição Persistente |  |  | ■ Fraqueza | ◆ Abismo Interior |
|  | ■ Marca do Abismo | ● Agonia |  | ■ Nuvem Pestilenta |  | ● Putrefação |  | ◆ Pandemia |
| ■ Peste Sombria |  |  | ○ Febre |  |  |  |  |  |
| ● Mente Afiada | ■ Praga Arcana | ● Contágio | ■ Prisão Etérea |  |  | ● Fluxo Arcano |  | ◆ Colapso Arcano |
| ■ Rajada Arcana |  |  |  |  | ○ Rajada Concentrada |  |  |  |
|  | ■ Piscar | ■ Escudo Arcano |  |  |  |  |  |  |

**Total:** 22 skills, 209 ranks.

## Recurso: Carga Arcana

| Gera | Quanto |
|---|---|
| Aplicar num alvo um efeito que ele **ainda não tem** (maldição, praga, marca, enfraquecer, prender) | +8 por efeito novo |
| Reaplicar um efeito que o alvo já tem | 0 |
| Acertar uma skill arcana (Rajada Arcana) | +4 |
| Praga que pula para outro inimigo | +3 (máx. 3 por morte) |

| Gasta | Quanto |
|---|---|
| Fraqueza | 20 |
| Rajada Concentrada (modificador) | 20 |
| Pandemia (chave) | 60 |
| Colapso Arcano (chave) | toda (+3% por ponto) |

## Skills

Na ordem da árvore: coluna por coluna, de cima para baixo.

### Drenar Vontade

Ativa · coluna **1** · **12** ranks · `SkillOccDrain` (77) · ícone `mo_drain`

> Arranca a força de vontade do inimigo.

**Efeito:** 160→210% e o alvo perde 20→30% de ataque por 8 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 160→210% | 160% | +4,55% | 210% | `Scale{1.6, 0.0455}` |
| 20→30% | 20% | +0,91% | 30% | `Scale{0.2, 0.0091}` |

- **Custo / recarga:** 30 MP / 14 s
- **Requisito:** 1 ponto na barra de Arcanista.
- **Modificadores ligados:** Ruína.
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Peste Sombria

Ativa · coluna **1** · **12** ranks · `SkillOccRot` (76) · ícone `mo_rot`

> Uma praga das sombras que passa de corpo em corpo.

**Efeito:** 120→160% e praga de 20→30%/s por 8 s; se o alvo morrer, a praga pula para os inimigos a até 6 m (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 120→160% | 120% | +3,64% | 160% | `Scale{1.2, 0.0364}` |
| 20→30% | 20% | +0,91% | 30% | `Scale{0.2, 0.0091}` |

- **Custo / recarga:** 30 MP / 12 s
- **Requisito:** 1 ponto na barra de Arcanista.
- **Modificadores ligados:** Febre.
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Mente Afiada

Passiva · coluna **1** · **10** ranks · `SkillOccMind` (180) · ícone novo (Meshy); hoje usa `m_mind`, emprestado

> Cada feitiço no tamanho certo.

**Efeito:** −2→10% no custo de mana (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |

- **Custo / recarga:** —
- **Requisito:** 1 ponto na barra de Arcanista.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Rajada Arcana

Ativa · coluna **1** · **12** ranks · `SkillArcaneBlast` (57) · ícone `m_arcane`

> Mana pura, concentrada num único disparo.

**Efeito:** 300→380% (até 20 m), sem errar (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 300→380% | 300% | +7,27% | 380% | `Scale{3, 0.0727}` |

- **Custo / recarga:** 45 MP / 10 s
- **Requisito:** 1 ponto na barra de Arcanista.
- **Modificadores ligados:** Rajada Concentrada.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Marca do Abismo

Ativa · coluna **5** · **12** ranks · `SkillOccAbyss` (78) · ícone `mo_abyss`

> O abismo olha de volta para quem foi marcado.

**Efeito:** 150→190% no alvo e a até 5 m; os atingidos recebem +20% de dano de todas as fontes por 10 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 150→190% | 150% | +3,64% | 190% | `Scale{1.5, 0.0364}` |

- **Custo / recarga:** 70 MP / 45 s
- **Requisito:** 5 pontos na barra de Arcanista.
- **No doc da evolução:** marco 20, 3 (hoje 1) ranks, requisito "—".

### Praga Arcana

Ativa · coluna **5** · **12** ranks · `SkillOccPlague` (172) · ícone novo (Meshy); hoje usa `r_poison`, emprestado

> Uma praga que corre de um inimigo para o outro.

**Efeito:** Praga de 15→25% do ataque mágico/s por 10 s, que se espalha como a Peste (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 15→25% | 15% | +0,91% | 25% | `Scale{0.15, 0.0091}` |

- **Custo / recarga:** 25 MP / 10 s
- **Requisito:** 5 pontos na barra de Arcanista.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Piscar

Ativa · coluna **5** · **12** ranks · `SkillOccBlink` (178) · ícone `m_blink`

> Some e reaparece mais à frente.

**Efeito:** Teleporta 8→12 m para a frente (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 8→12 | 8 | +0,36 | 12 | `Scale{8, 0.3636}` |

- **Custo / recarga:** 20 MP / 18 s
- **Requisito:** 5 pontos na barra de Arcanista.
- **Origem:** evol. (de `SkillBlink`).
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Agonia

Passiva · coluna **10** · **10** ranks · `SkillOccAgony` (167) · ícone novo (Meshy); hoje usa `r_para`, emprestado

> Quem carrega a maldição sofre mais.

**Efeito:** Alvos amaldiçoados recebem +2→10% de dano seu (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Arcanista.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Contágio

Passiva · coluna **10** · **10** ranks · `SkillOccContagion` (173) · ícone novo (Meshy); hoje usa `r_flask`, emprestado

> A praga fica mais forte a cada salto.

**Efeito:** A praga que pula leva +4→20% de força (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 4→20% | 4% | +1,78% | 20% | `Scale{0.04, 0.0178}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Arcanista.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Escudo Arcano

Ativa · coluna **10** · **12** ranks · `SkillOccShield` (179) · ícone `m_shield`

> Uma barreira que se alimenta da mana.

**Efeito:** Por 8→12 s, o dano recebido sai da mana em vez da vida (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 8→12 | 8 | +0,36 | 12 | `Scale{8, 0.3636}` |

- **Custo / recarga:** 25 MP / 30 s
- **Requisito:** 10 pontos na barra de Arcanista.
- **Origem:** evol. (de `SkillArcaneShield`).
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Ruína

Modificador · coluna **15** · **8** ranks · `SkillOccRuin` (168) · ícone novo (Meshy); hoje usa `r_weak`, emprestado

> A vontade drenada demora a voltar.

**Efeito:** O enfraquecimento dura +1→3 s (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 1→3 | 1 | +0,29 | 3 | `Scale{1, 0.2857}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Arcanista; 1 ponto em **Drenar Vontade**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Drenar 1".

### Febre

Modificador · coluna **15** · **8** ranks · `SkillOccFever` (174) · ícone novo (Meshy); hoje usa `r_twin`, emprestado

> A peste não passa.

**Efeito:** A praga dura +2→6 s (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 2→6 | 2 | +0,57 | 6 | `Scale{2, 0.5714}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Arcanista; 1 ponto em **Peste Sombria**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Peste 1".

### Prisão Etérea

Ativa · coluna **15** · **12** ranks · `SkillOccBanish` (181) · ícone novo (Meshy); hoje usa `m_over`, emprestado

> Tira o alvo deste mundo por um instante.

**Efeito:** Prende o alvo por 2→3 s; ele não recebe nem causa dano (some de cena) (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 2→3 | 2 | +0,09 | 3 | `Scale{2, 0.0909}` |

- **Custo / recarga:** 30 MP / 25 s
- **Requisito:** 15 pontos na barra de Arcanista.
- **No doc da evolução:** marco 25, 3 ranks, requisito "—".

### Maldição Persistente

Ativa · coluna **20** · **12** ranks · `SkillOccCurse` (169) · ícone novo (Meshy); hoje usa `r_plague`, emprestado

> Uma maldição que alimenta as outras.

**Efeito:** Por 12 s, 20→30% do ataque mágico/s; ao acertar, renova as outras maldições suas no alvo (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 20→30% | 20% | +0,91% | 30% | `Scale{0.2, 0.0091}` |

- **Custo / recarga:** 30 MP / 15 s
- **Requisito:** 20 pontos na barra de Arcanista.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Nuvem Pestilenta

Ativa · coluna **20** · **12** ranks · `SkillOccCloud` (175) · ícone novo (Meshy); hoje usa `r_miasma`, emprestado

> Uma nuvem doente no chão.

**Efeito:** Nuvem de 5 m por 6 s: 25→35% do ataque mágico/s nos inimigos dentro (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 25→35% | 25% | +0,91% | 35% | `Scale{0.25, 0.0091}` |

- **Custo / recarga:** 40 MP / 20 s
- **Requisito:** 20 pontos na barra de Arcanista.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Rajada Concentrada

Modificador · coluna **25** · **8** ranks · `SkillOccFocused` (182) · ícone novo (Meshy); hoje usa `m_lance`, emprestado

> A carga vira força pura.

**Efeito:** Com 20 de Carga: a Rajada gasta a Carga e causa +30→60% (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 30→60% | 30% | +4,29% | 60% | `Scale{0.3, 0.0429}` |

- **Custo / recarga:** —
- **Requisito:** 25 pontos na barra de Arcanista; 1 ponto em **Rajada Arcana**.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Putrefação

Passiva · coluna **32** · **10** ranks · `SkillOccRot2` (176) · ícone novo (Meshy); hoje usa `r_smoke`, emprestado

> Às vezes a praga morde fundo.

**Efeito:** As pragas podem dar crítico (+5→15% de chance) (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 5→15% | 5% | +1,11% | 15% | `Scale{0.05, 0.0111}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Arcanista.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Fluxo Arcano

Passiva · coluna **32** · **10** ranks · `SkillOccFlow` (183) · ícone novo (Meshy); hoje usa `m_zero`, emprestado

> A carga gasta volta como mana.

**Efeito:** Cada 10 de Carga gasta devolve 2→4% da mana máxima (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→4% | 2% | +0,22% | 4% | `Scale{0.02, 0.0022}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Arcanista.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Fraqueza

Ativa · coluna **40** · **12** ranks · `SkillOccWeakness` (170) · ícone novo (Meshy); hoje usa `r_mark`, emprestado

> Quebra a defesa por dentro.

**Efeito:** O alvo perde 10→20% de defesa por 8 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 10→20% | 10% | +0,91% | 20% | `Scale{0.1, 0.0091}` |

- **Custo / recarga:** 20 MP + **20 Carga** / 20 s
- **Requisito:** 40 pontos na barra de Arcanista.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Abismo Interior

Chave · coluna **50** · **1** rank · `SkillOccAbyssCore` (171) · ícone novo (Meshy); hoje usa `b_undying`, emprestado

> O abismo não solta o que pegou.

**Efeito:** Suas maldições duram +50%, e a Marca do Abismo se espalha para até 2 vizinhos

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Arcanista.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Pandemia

Chave · coluna **50** · **1** rank · `SkillOccPandemic` (177) · ícone novo (Meshy); hoje usa `r_vanish`, emprestado

> O que um tem, todos pegam.

**Efeito:** Espalha todas as pragas e maldições do alvo para os inimigos a até 8 m

- **Custo / recarga:** **60 Carga** / 45 s
- **Requisito:** 50 pontos na barra de Arcanista.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Colapso Arcano

Chave · coluna **50** · **1** rank · `SkillOccCollapse` (184) · ícone novo (Meshy); hoje usa `m_meteor`, emprestado

> A carga inteira desaba sobre todos.

**Efeito:** Explosão de 250% a até 6 m, +3% por ponto de Carga gasto (toda)

- **Custo / recarga:** 40 MP + **toda a Carga** / 40 s
- **Requisito:** 50 pontos na barra de Arcanista.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".


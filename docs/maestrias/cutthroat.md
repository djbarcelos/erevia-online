# Assassino (maestria de Ladino)

> Gerado do protótipo [Maestrias de Erevia](https://claude.ai/artifact/4P5N9Bg2LCeP7n4bjdKkAZ) e de [docs/evolucoes/cutthroat.md](../evolucoes/cutthroat.md). As regras gerais estão no [README](README.md). Os números são os do doc da evolução, espalhados pelos ranks do sistema de maestrias, e ainda vão ser calibrados no simulador.

- **ID:** `cutthroat` · **classe-base:** Ladino · **recurso:** Combo
- **Barra da maestria:** cada ponto dá +1 Des e +8 de vida (no 50: +50 Des, +400 de vida).
- **Papel:** É dano explosivo num alvo prioritário, com críticos e mobilidade. Prepara a oportunidade, chega perto e executa.
- **Duplas:** Bastião → **Vigia Sombrio** · Paladino → **Penitente** · Berserker → **Carniceiro** · Senhor da Guerra → **Mercenário** · Elementalista → **Lâmina Elemental** · Arcanista → **Bruxo** · Patrulheiro → **Espreitador**

## Árvore

Colunas = pontos na barra que liberam a skill. ■ ativa · ● passiva · ○ modificador (ligado à skill da linha) · ◆ chave.

| 1 | 5 | 10 | 15 | 20 | 25 | 32 | 40 | 50 |
|---|---|---|---|---|---|---|---|---|
| ■ Golpe Furtivo | ■ Lâminas Gêmeas |  | ○ Corte Profundo | ■ Sentença |  | ● Fio Perfeito |  | ◆ Dança das Lâminas |
| ■ Golpe Fatal | ■ Golpe das Sombras | ■ Lâmina Oculta | ■ Sumir |  |  | ○ Sombra Persistente |  | ◆ Assassino das Sombras |
| ■ Passo Sombrio | ● Instinto Assassino | ● Esquiva |  | ■ Véu Mortal |  | ● Sangrar |  | ◆ Execução Perfeita |
| ■ Marca da Morte |  |  | ○ Marca Profunda | ■ Golpe de Misericórdia |  |  |  |  |
|  |  | ● Ponto Fraco |  |  |  |  |  |  |

**Total:** 22 skills, 209 ranks.

## Recurso: Combo

| Gera | Quanto |
|---|---|
| Ataque básico acertado | +5 |
| Skill acertada | +8 |
| Golpe pelas costas (atrás do alvo, cone de 120°) | +5 extra |
| Crítico | +3 |
| Assassino das Sombras: sair da invisibilidade (chave) | +40 |

| Gasta | Quanto |
|---|---|
| Véu Mortal | 30 |
| Golpe de Misericórdia | 40 |
| Sentença | 50 |

## Skills

Na ordem da árvore: coluna por coluna, de cima para baixo.

### Golpe Furtivo

Ativa · coluna **1** · **12** ranks · `SkillBackstab` (185) · ícone `backstab`

> Um golpe traiçoeiro num ponto fraco, com grande chance de crítico.

**Efeito:** 200→260% com +50% de chance de crítico (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 200→260% | 200% | +5,45% | 260% | `Scale{2, 0.0545}` |

- **Custo / recarga:** 10 MP / 6 s
- **Requisito:** 1 ponto na barra de Assassino.
- **Origem:** evol..
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Golpe Fatal

Ativa · coluna **1** · **12** ranks · `SkillFatalBlow` (58) · ícone `r_fatal`

> Um só golpe, no lugar exato.

**Efeito:** 250→330% com +50% de crítico; +50% em alvos abaixo de 35% de vida (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 250→330% | 250% | +7,27% | 330% | `Scale{2.5, 0.0727}` |

- **Custo / recarga:** 35 MP / 18 s
- **Requisito:** 1 ponto na barra de Assassino.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Passo Sombrio

Ativa · coluna **1** · **12** ranks · `SkillShadowStep` (191) · ícone `shadow_step`

> Funde-se às sombras e se move muito mais rápido.

**Efeito:** +40→60% de velocidade por 6 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 40→60% | 40% | +1,82% | 60% | `Scale{0.4, 0.0182}` |

- **Custo / recarga:** 12 MP / 20 s
- **Requisito:** 1 ponto na barra de Assassino.
- **Origem:** evol..
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Marca da Morte

Ativa · coluna **1** · **12** ranks · `SkillCutMark` (197) · ícone `r_mark`

> Marca a presa.

**Efeito:** 20% do ataque; o alvo recebe +15→25% de dano de todas as fontes por 10 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 15→25% | 15% | +0,91% | 25% | `Scale{0.15, 0.0091}` |

- **Custo / recarga:** 30 MP / 40 s
- **Requisito:** 1 ponto na barra de Assassino.
- **Modificadores ligados:** Marca Profunda.
- **Origem:** evol. (de `SkillDeathMark`).
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Lâminas Gêmeas

Ativa · coluna **5** · **12** ranks · `SkillCutTwin` (186) · ícone `r_twin`

> Duas lâminas, um só movimento.

**Efeito:** 2 golpes de 115→145% (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 115→145% | 115% | +2,73% | 145% | `Scale{1.15, 0.0273}` |

- **Custo / recarga:** 16 MP / 8 s
- **Requisito:** 5 pontos na barra de Assassino.
- **Modificadores ligados:** Corte Profundo.
- **Origem:** evol. (de `SkillTwinBlades`).
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Golpe das Sombras

Ativa · coluna **5** · **12** ranks · `SkillShadowStrike` (59) · ícone `r_shadow`

> Sai das sombras colado no alvo e ataca os olhos.

**Efeito:** Salta até o alvo (até 12 m), 180→240% e o cega (30→50% mais erros) por 4 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 180→240% | 180% | +5,45% | 240% | `Scale{1.8, 0.0545}` |
| 30→50% | 30% | +1,82% | 50% | `Scale{0.3, 0.0182}` |

- **Custo / recarga:** 30 MP / 20 s
- **Requisito:** 5 pontos na barra de Assassino.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Instinto Assassino

Passiva · coluna **5** · **10** ranks · `SkillCutInstinct` (80) · ícone `rc_instinct`

> Sabe que golpe termina a luta.

**Efeito:** +6→30% de dano crítico (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 6→30% | 6% | +2,67% | 30% | `Scale{0.06, 0.0267}` |

- **Custo / recarga:** —
- **Requisito:** 5 pontos na barra de Assassino.
- **No doc da evolução:** marco 20, 5 (hoje 3) ranks, requisito "—".

### Lâmina Oculta

Ativa · coluna **10** · **12** ranks · `SkillCutBlades` (79) · ícone `rc_blades`

> Três cortes antes que o alvo perceba o primeiro.

**Efeito:** 3 golpes de 100→130% com +20% de chance de crítico (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 100→130% | 100% | +2,73% | 130% | `Scale{1, 0.0273}` |

- **Custo / recarga:** 28 MP / 10 s
- **Requisito:** 10 pontos na barra de Assassino.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Esquiva

Passiva · coluna **10** · **10** ranks · `SkillCutDodge` (192) · ícone `r_dodge`

> Nunca está onde o golpe cai.

**Efeito:** +6→30 de esquiva (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 6→30 | 6 | +2,67 | 30 | `Scale{6, 2.6667}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Assassino.
- **Origem:** evol. (de `SkillDodge`).
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Ponto Fraco

Passiva · coluna **10** · **10** ranks · `SkillCutWeak` (198) · ícone `r_weak`

> Sabe exatamente onde dói.

**Efeito:** +2→10% de chance de crítico (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Assassino.
- **Origem:** evol. (de `SkillWeakSpot`).
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Corte Profundo

Modificador · coluna **15** · **8** ranks · `SkillCutDeep` (188) · ícone novo (Meshy); hoje usa `b_frenzy`, emprestado

> As duas lâminas entram mais fundo.

**Efeito:** +10% / +20% de dano; no rank 3, +1 golpe

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Assassino; 1 ponto em **Lâminas Gêmeas**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Lâminas Gêmeas 1".

### Sumir

Ativa · coluna **15** · **12** ranks · `SkillCutVanish` (193) · ícone `r_vanish`

> Desaparece nas sombras.

**Efeito:** Invisível por 3→5 s; atacar revela, e o primeiro golpe é crítico garantido (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 3→5 | 3 | +0,18 | 5 | `Scale{3, 0.1818}` |

- **Custo / recarga:** 35 MP / 60→45 s
- **Requisito:** 15 pontos na barra de Assassino.
- **Modificadores ligados:** Sombra Persistente.
- **Origem:** evol. (de `SkillVanish`).
- **No doc da evolução:** marco 25, 3 ranks, requisito "—".

### Marca Profunda

Modificador · coluna **15** · **8** ranks · `SkillCutDeepMark` (199) · ícone novo (Meshy); hoje usa `r_para`, emprestado

> A marca não sai.

**Efeito:** A marca dura +2→6 s (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 2→6 | 2 | +0,57 | 6 | `Scale{2, 0.5714}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Assassino; 1 ponto em **Marca da Morte**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Marca 1".

### Sentença

Ativa · coluna **20** · **12** ranks · `SkillCutSentence` (81) · ícone `rc_sentence`

> O contrato foi assinado: só falta cumprir.

**Efeito:** Salta até o alvo (até 12 m), 450→540%, sem errar; +50% abaixo de 30% de vida (em chefe, no máximo +50%) (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 450→540% | 450% | +8,18% | 540% | `Scale{4.5, 0.0818}` |

- **Custo / recarga:** 40 MP + **50 Combo** / 50 s
- **Requisito:** 20 pontos na barra de Assassino.
- **No doc da evolução:** marco 30, 3 (hoje 1) ranks, requisito "5 no ramo".

### Véu Mortal

Ativa · coluna **20** · **12** ranks · `SkillCutVeil` (194) · ícone novo (Meshy); hoje usa `m_blink`, emprestado

> Quem cai abre a sombra para o próximo passo.

**Efeito:** Por 8 s, matar um inimigo deixa você invisível por 2→3 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 2→3 | 2 | +0,09 | 3 | `Scale{2, 0.0909}` |

- **Custo / recarga:** 20 MP + **30 Combo** / 30 s
- **Requisito:** 20 pontos na barra de Assassino.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Golpe de Misericórdia

Ativa · coluna **20** · **12** ranks · `SkillCutMercy` (200) · ícone novo (Meshy); hoje usa `b_exec`, emprestado

> Termina o que começou.

**Efeito:** 250→310%; +50% abaixo de 30% de vida (chefe: no máximo +50%) (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 250→310% | 250% | +5,45% | 310% | `Scale{2.5, 0.0545}` |

- **Custo / recarga:** 25 MP + **40 Combo** / 12 s
- **Requisito:** 20 pontos na barra de Assassino.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Fio Perfeito

Passiva · coluna **32** · **10** ranks · `SkillCutEdge` (189) · ícone novo (Meshy); hoje usa `b_heavy`, emprestado

> A lâmina perfeita não para no osso.

**Efeito:** +5→15% de dano crítico (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 5→15% | 5% | +1,11% | 15% | `Scale{0.05, 0.0111}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Assassino.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Sombra Persistente

Modificador · coluna **32** · **8** ranks · `SkillCutLinger` (195) · ícone novo (Meshy); hoje usa `r_smoke`, emprestado

> A sombra continua correndo depois de aparecer.

**Efeito:** Depois de revelar, +20→40% de velocidade por 3 s (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 20→40% | 20% | +2,86% | 40% | `Scale{0.2, 0.0286}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Assassino; 1 ponto em **Sumir**.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Sangrar

Passiva · coluna **32** · **10** ranks · `SkillCutBleed` (201) · ícone novo (Meshy); hoje usa `r_poison`, emprestado

> Cada crítico deixa uma ferida aberta.

**Efeito:** Críticos causam sangramento: 5→15% do ataque/s por 4 s (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 5→15% | 5% | +1,11% | 15% | `Scale{0.05, 0.0111}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Assassino.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Dança das Lâminas

Chave · coluna **50** · **1** rank · `SkillCutDance` (190) · ícone novo (Meshy); hoje usa `k_spin`, emprestado

> Cada final abre o próximo.

**Efeito:** As finalizações (que gastam Combo) devolvem 30% do Combo gasto

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Assassino.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Assassino das Sombras

Chave · coluna **50** · **1** rank · `SkillCutShadowKill` (196) · ícone novo (Meshy); hoje usa `b_undying`, emprestado

> Da sombra direto para o golpe final.

**Efeito:** Sair da invisibilidade dá +40 de Combo e +100% de dano crítico no primeiro golpe

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Assassino.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Execução Perfeita

Chave · coluna **50** · **1** rank · `SkillCutPerfect` (202) · ícone novo (Meshy); hoje usa `kb_last`, emprestado

> Um corpo cai, a sombra já está no próximo.

**Efeito:** Matar com uma finalização zera a recarga do Passo Sombrio e do Golpe das Sombras

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Assassino.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".


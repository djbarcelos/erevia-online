# Elementalista (maestria de Mago)

> Gerado do protótipo [Maestrias de Erevia](https://claude.ai/artifact/4P5N9Bg2LCeP7n4bjdKkAZ) e de [docs/evolucoes/elementalist.md](../evolucoes/elementalist.md). As regras gerais estão no [README](README.md). Os números são os do doc da evolução, espalhados pelos ranks do sistema de maestrias, e ainda vão ser calibrados no simulador.

- **ID:** `elementalist` · **classe-base:** Mago · **recurso:** Concentração
- **Barra da maestria:** cada ponto dá +1 Int e +8 de vida (no 50: +50 Int, +400 de vida).
- **Papel:** É o DPS mágico à distância. Causa dano em área e aplica efeitos elementais, alternando fogo e gelo para bater forte, controlar grupos e aproveitar o alvo que já está queimando ou lento.
- **Duplas:** Bastião → **Guardião Rúnico** · Paladino → **Teurgo** · Berserker → **Incendiário** · Senhor da Guerra → **Senhor da Tormenta** · Arcanista → **Arquimago** · Assassino → **Lâmina Elemental** · Patrulheiro → **Arqueiro Rúnico**

## Árvore

Colunas = pontos na barra que liberam a skill. ■ ativa · ● passiva · ○ modificador (ligado à skill da linha) · ◆ chave.

| 1 | 5 | 10 | 15 | 20 | 25 | 32 | 40 | 50 |
|---|---|---|---|---|---|---|---|---|
| ■ Bola de Fogo |  |  | ○ Chama Persistente | ■ Chuva de Brasas |  |  | ■ Meteoro | ◆ Coração do Vulcão |
| ● Brasa Viva | ■ Lança Ígnea | ■ Inferno |  | ■ Prisão Glacial |  |  | ■ Armadura de Gelo | ◆ Coração do Inverno |
| ■ Nova de Gelo |  |  | ○ Inverno Eterno |  |  |  |  |  |
| ● Frio Penetrante | ■ Lança de Gelo | ■ Zero Absoluto |  | ■ Cataclismo |  | ● Sobrecarga |  | ◆ Ruptura Elemental |
| ■ Reação Elemental |  |  | ○ Convergência |  |  |  |  |  |
|  | ● Foco Elemental |  |  |  |  |  |  |  |

**Total:** 22 skills, 211 ranks.

## Recurso: Concentração

| Gera | Quanto |
|---|---|
| Lançar uma skill **diferente** da última lançada | +10 (a mesma skill duas vezes seguidas gera 0) |
| Ataque básico mágico acertado | +4 |
| Crítico mágico | +3 |
| Reação Elemental acertando um alvo queimado **e** lento | +10 |

| Gasta | Quanto |
|---|---|
| Meteoro | 30 |
| Cataclismo | 40 |
| Ruptura Elemental (chave) | toda (+2% de dano por ponto) |

## Skills

Na ordem da árvore: coluna por coluna, de cima para baixo.

### Bola de Fogo

Ativa · coluna **1** · **12** ranks · `SkillFireball` (150) · ícone `fireball`

> Lança uma bola de fogo que explode no alvo à distância.

**Efeito:** 250→310% do ataque mágico, até 20 m (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 250→310% | 250% | +5,45% | 310% | `Scale{2.5, 0.0545}` |

- **Custo / recarga:** 15 MP / 3 s
- **Requisito:** 1 ponto na barra de Elementalista.
- **Modificadores ligados:** Chama Persistente.
- **Origem:** evol..
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Brasa Viva

Passiva · coluna **1** · **10** ranks · `SkillElemEmber` (151) · ícone `m_ember`

> O fogo do mago não se apaga fácil.

**Efeito:** As skills de fogo queimam: 6→30% do ataque mágico/s por 3 s (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 6→30% | 6% | +2,67% | 30% | `Scale{0.06, 0.0267}` |

- **Custo / recarga:** —
- **Requisito:** 1 ponto na barra de Elementalista.
- **Origem:** evol. (de `SkillEmber`).
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Nova de Gelo

Ativa · coluna **1** · **12** ranks · `SkillIceNova` (156) · ícone `ice_nova`

> Uma onda de gelo explode ao seu redor, ferindo e congelando os inimigos próximos.

**Efeito:** 120→160% a até 6 m e 50% de lentidão por 3→4 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 120→160% | 120% | +3,64% | 160% | `Scale{1.2, 0.0364}` |
| 3→4 | 3 | +0,09 | 4 | `Scale{3, 0.0909}` |

- **Custo / recarga:** 30 MP / 10 s
- **Requisito:** 1 ponto na barra de Elementalista.
- **Modificadores ligados:** Inverno Eterno.
- **Origem:** evol..
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Frio Penetrante

Passiva · coluna **1** · **10** ranks · `SkillElemBiting` (158) · ícone novo (Meshy); hoje usa `r_dodge`, emprestado

> O frio deixa tudo mais frágil.

**Efeito:** Alvos lentos recebem +2→10% de dano seu (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |

- **Custo / recarga:** —
- **Requisito:** 1 ponto na barra de Elementalista.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Reação Elemental

Ativa · coluna **1** · **12** ranks · `SkillElemReaction` (164) · ícone novo (Meshy); hoje usa `k_spin`, emprestado

> Fogo e gelo no mesmo alvo explodem.

**Efeito:** 150→200%; o dobro se o alvo estiver queimado **e** lento (consome os dois efeitos) (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 150→200% | 150% | +4,55% | 200% | `Scale{1.5, 0.0455}` |

- **Custo / recarga:** 30 MP / 10 s
- **Requisito:** 1 ponto na barra de Elementalista.
- **Modificadores ligados:** Convergência.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Lança Ígnea

Ativa · coluna **5** · **12** ranks · `SkillElemLance` (73) · ícone `me_lance`

> Fogo comprimido numa lança que atravessa armaduras.

**Efeito:** 260→330% (até 20 m), de fogo (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 260→330% | 260% | +6,36% | 330% | `Scale{2.6, 0.0636}` |

- **Custo / recarga:** 28 MP / 6 s
- **Requisito:** 5 pontos na barra de Elementalista.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Lança de Gelo

Ativa · coluna **5** · **12** ranks · `SkillElemIceLance` (157) · ícone `m_lance`

> Uma lança de gelo afiado.

**Efeito:** 170→210% (até 18 m) e 30% de lentidão por 2 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 170→210% | 170% | +3,64% | 210% | `Scale{1.7, 0.0364}` |

- **Custo / recarga:** 18 MP / 5 s
- **Requisito:** 5 pontos na barra de Elementalista.
- **Origem:** evol. (de `SkillIceLance`).
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Foco Elemental

Passiva · coluna **5** · **10** ranks · `SkillElemFocus` (74) · ícone `me_focus`

> Cada elemento obedece mais à vontade do mago.

**Efeito:** +9→45% de dano crítico (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 9→45% | 9% | +4% | 45% | `Scale{0.09, 0.04}` |

- **Custo / recarga:** —
- **Requisito:** 5 pontos na barra de Elementalista.
- **No doc da evolução:** marco 20, 5 (hoje 3) ranks, requisito "—".

### Inferno

Ativa · coluna **10** · **12** ranks · `SkillInferno` (55) · ícone `m_inferno`

> Faz chover fogo e deixa o chão em chamas.

**Efeito:** 180→230% no alvo e a até 5 m; queimadura de 25→35%/s por 5 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 180→230% | 180% | +4,55% | 230% | `Scale{1.8, 0.0455}` |
| 25→35% | 25% | +0,91% | 35% | `Scale{0.25, 0.0091}` |

- **Custo / recarga:** 50 MP / 18 s
- **Requisito:** 10 pontos na barra de Elementalista.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Zero Absoluto

Ativa · coluna **10** · **12** ranks · `SkillAbsoluteZero` (56) · ícone `m_zero`

> O ar em volta congela de uma vez.

**Efeito:** 150→200% a até 6 m e congela (atordoa) por 1,5→2 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 150→200% | 150% | +4,55% | 200% | `Scale{1.5, 0.0455}` |
| 1,5→2 | 1,5 | +0,05 | 2 | `Scale{1.5, 0.0455}` |

- **Custo / recarga:** 55 MP / 30 s
- **Requisito:** 10 pontos na barra de Elementalista.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Chama Persistente

Modificador · coluna **15** · **8** ranks · `SkillElemLinger` (152) · ícone novo (Meshy); hoje usa `kh_sun`, emprestado

> A bola de fogo deixa o alvo queimando mais tempo.

**Efeito:** A queimadura dura +1→3 s (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 1→3 | 1 | +0,29 | 3 | `Scale{1, 0.2857}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Elementalista; 1 ponto em **Bola de Fogo**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Bola de Fogo 1".

### Inverno Eterno

Modificador · coluna **15** · **8** ranks · `SkillElemWinter` (159) · ícone novo (Meshy); hoje usa `b_stone`, emprestado

> O gelo da Nova demora a derreter.

**Efeito:** A lentidão dura +1→3 s (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 1→3 | 1 | +0,29 | 3 | `Scale{1, 0.2857}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Elementalista; 1 ponto em **Nova de Gelo**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Nova 1".

### Convergência

Modificador · coluna **15** · **8** ranks · `SkillElemConverge` (165) · ícone novo (Meshy); hoje usa `b_quake`, emprestado

> A reação transborda.

**Efeito:** A Reação atinge também a até 1→3 m do alvo (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 1→3 | 1 | +0,29 | 3 | `Scale{1, 0.2857}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Elementalista; 1 ponto em **Reação Elemental**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Reação 1".

### Chuva de Brasas

Ativa · coluna **20** · **12** ranks · `SkillElemRain` (153) · ícone `m_rain`

> Brasas caem do céu sobre os inimigos.

**Efeito:** 160→200% no alvo e a até 5 m (alcance de 16 m) (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 160→200% | 160% | +3,64% | 200% | `Scale{1.6, 0.0364}` |

- **Custo / recarga:** 35 MP / 12 s
- **Requisito:** 20 pontos na barra de Elementalista.
- **Origem:** evol. (de `SkillEmberRain`).
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Prisão Glacial

Ativa · coluna **20** · **12** ranks · `SkillElemPrison` (160) · ícone `m_prison`

> Prende o alvo num bloco de gelo.

**Efeito:** 50→80% e prende o alvo no lugar por 3→4 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 50→80% | 50% | +2,73% | 80% | `Scale{0.5, 0.0273}` |
| 3→4 | 3 | +0,09 | 4 | `Scale{3, 0.0909}` |

- **Custo / recarga:** 40 MP / 30 s
- **Requisito:** 20 pontos na barra de Elementalista.
- **Origem:** evol. (de `SkillGlacialPrison`).
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Cataclismo

Ativa · coluna **20** · **12** ranks · `SkillElemCata` (75) · ícone `me_cata`

> Fogo do céu e gelo no chão, ao mesmo tempo.

**Efeito:** 400→480% no alvo e a até 6 m, com 50% de lentidão por 4 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 400→480% | 400% | +7,27% | 480% | `Scale{4, 0.0727}` |

- **Custo / recarga:** 60 MP + **40 Conc.** / 60 s
- **Requisito:** 20 pontos na barra de Elementalista.
- **No doc da evolução:** marco 30, 3 (hoje 1) ranks, requisito "5 no ramo".

### Sobrecarga

Passiva · coluna **32** · **10** ranks · `SkillElemOverload` (163) · ícone `m_over`

> Mais poder, mais custo.

**Efeito:** +5→15% de ataque mágico; as skills custam 10% a mais de mana (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 5→15% | 5% | +1,11% | 15% | `Scale{0.05, 0.0111}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Elementalista.
- **Origem:** evol. (de `SkillOverload`).
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Meteoro

Ativa · coluna **40** · **12** ranks · `SkillElemMeteor` (154) · ícone `m_meteor`

> Puxa uma pedra em chamas do céu.

**Efeito:** 500→600% no alvo e a até 3 m; atordoa por 2 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 500→600% | 500% | +9,09% | 600% | `Scale{5, 0.0909}` |

- **Custo / recarga:** 50 MP + **30 Conc.** / 50 s
- **Requisito:** 40 pontos na barra de Elementalista.
- **Origem:** evol. (de `SkillMeteor`).
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Armadura de Gelo

Ativa · coluna **40** · **12** ranks · `SkillElemArmor` (161) · ícone `m_armor`

> Uma casca de gelo cobre o mago.

**Efeito:** +30→50% de defesa por 20 s; quem bate de perto fica 30% mais lento (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 30→50% | 30% | +1,82% | 50% | `Scale{0.3, 0.0182}` |

- **Custo / recarga:** 30 MP / 40 s
- **Requisito:** 40 pontos na barra de Elementalista.
- **Origem:** evol. (de `SkillIceArmor`).
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Coração do Vulcão

Chave · coluna **50** · **1** rank · `SkillElemVolcano` (155) · ícone novo (Meshy); hoje usa `b_exec`, emprestado

> O fogo procura o fogo.

**Efeito:** Skills de fogo em alvo queimado causam +20% e espalham a queimadura a até 3 m

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Elementalista.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Coração do Inverno

Chave · coluna **50** · **1** rank · `SkillElemFrostCore` (162) · ícone novo (Meshy); hoje usa `k_wall`, emprestado

> O que está parado quebra fácil.

**Efeito:** O próximo golpe seu num alvo congelado ou preso é crítico garantido

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Elementalista.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Ruptura Elemental

Chave · coluna **50** · **1** rank · `SkillElemRupture` (166) · ícone novo (Meshy); hoje usa `b_seismic`, emprestado

> Toda a concentração solta de uma vez.

**Efeito:** 300% a até 6 m, +2% por ponto de Concentração gasto (toda); aplica queimadura e lentidão

- **Custo / recarga:** 40 MP + **toda a Conc.** / 45 s
- **Requisito:** 50 pontos na barra de Elementalista.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".


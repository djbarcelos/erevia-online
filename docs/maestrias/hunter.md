# Patrulheiro (maestria de Ladino)

> Gerado do protótipo [Maestrias de Erevia](https://claude.ai/artifact/4P5N9Bg2LCeP7n4bjdKkAZ) e de [docs/evolucoes/hunter.md](../evolucoes/hunter.md). As regras gerais estão no [README](README.md). Os números são os do doc da evolução, espalhados pelos ranks do sistema de maestrias, e ainda vão ser calibrados no simulador.

- **ID:** `hunter` · **classe-base:** Ladino · **recurso:** Precisão
- **Barra da maestria:** cada ponto dá +0,5 For, +0,5 Des e +8 de vida (no 50: +25 For, +25 Des, +400 de vida).
- **Papel:** É o DPS físico à distância: perseguição, armadilhas e controle de espaço. Mantém a distância, prepara o terreno e pune quem tenta chegar perto ou fugir. É a única evolução de dano físico que bate de longe.
- **Duplas:** Bastião → **Guarda-Fronteira** · Paladino → **Caçador de Bruxas** · Berserker → **Predador** · Senhor da Guerra → **Desbravador** · Elementalista → **Arqueiro Rúnico** · Arcanista → **Pestilento** · Assassino → **Espreitador**

## Árvore

Colunas = pontos na barra que liberam a skill. ■ ativa · ● passiva · ○ modificador (ligado à skill da linha) · ◆ chave.

| 1 | 5 | 10 | 15 | 20 | 25 | 32 | 40 | 50 |
|---|---|---|---|---|---|---|---|---|
| ■ Tiro Preciso |  |  | ○ Flecha Carregada | ■ Chuva de Flechas |  |  | ■ Flecha Mortal | ◆ Mira Perfeita |
| ■ Rede de Caça | ■ Disparo Perfurante | ● Olho de Falcão | ■ Bomba de Fumaça | ■ Campo de Espinhos |  | ● Mestre Armadilheiro |  | ◆ Emboscada |
| ■ Faca de Arremesso | ■ Armadilha de Caça | ■ Miasma | ■ Recuo | ■ Perseguição |  | ● Instinto do Caçador |  | ◆ Caçada Implacável |
|  | ■ Marca da Presa | ● Passo de Caça |  |  |  |  |  |  |

**Total:** 21 skills, 207 ranks.

## Recurso: Precisão

| Gera | Quanto |
|---|---|
| Acerto à distância com o alvo a 6 m ou mais | +6 |
| Acerto à distância com o alvo a 12 m ou mais | +10 |
| Acerto seguido no mesmo alvo | +2 extra por acerto seguido (até +8) |
| **Errar** | **zera a Precisão** |
| Golpe a menos de 6 m | 0 |

| Gasta | Quanto |
|---|---|
| Perseguição | 20 |
| Campo de Espinhos | 25 |
| Flecha Mortal | 40 |

## Skills

Na ordem da árvore: coluna por coluna, de cima para baixo.

### Tiro Preciso

Ativa · coluna **1** · **12** ranks · `SkillHuntShot` (203) · ícone novo (Meshy); hoje usa `r_mark`, emprestado

> Respira fundo e acerta onde mirou.

**Efeito:** 180→230% à distância (alcance da arma) (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 180→230% | 180% | +4,55% | 230% | `Scale{1.8, 0.0455}` |

- **Custo / recarga:** 12 MP / 5 s
- **Requisito:** 1 ponto na barra de Patrulheiro.
- **Modificadores ligados:** Flecha Carregada.
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Rede de Caça

Ativa · coluna **1** · **12** ranks · `SkillHuntNet` (83) · ícone `rh_net`

> Uma rede com pesos nas pontas, para mais de uma presa.

**Efeito:** 60% no alvo e a até 3 m, que ficam presos no lugar por 2,5→3,5 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 2,5→3,5 | 2,5 | +0,09 | 3,5 | `Scale{2.5, 0.0909}` |

- **Custo / recarga:** 30 MP / 20 s
- **Requisito:** 1 ponto na barra de Patrulheiro.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Faca de Arremesso

Ativa · coluna **1** · **12** ranks · `SkillHuntKnife` (82) · ícone `rh_knife`

> Uma faca na perna faz qualquer presa andar devagar.

**Efeito:** 180→230% à distância (até 16 m) e 30% de lentidão por 3 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 180→230% | 180% | +4,55% | 230% | `Scale{1.8, 0.0455}` |

- **Custo / recarga:** 20 MP / 6 s
- **Requisito:** 1 ponto na barra de Patrulheiro.
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Disparo Perfurante

Ativa · coluna **5** · **12** ranks · `SkillHuntPierce` (204) · ícone novo (Meshy); hoje usa `k_judg`, emprestado

> Um tiro que atravessa a fila.

**Efeito:** 150→190% em até 3 inimigos em linha (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 150→190% | 150% | +3,64% | 190% | `Scale{1.5, 0.0364}` |

- **Custo / recarga:** 25 MP / 10 s
- **Requisito:** 5 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Armadilha de Caça

Ativa · coluna **5** · **12** ranks · `SkillHuntTrap` (209) · ícone novo (Meshy); hoje usa `k_iron`, emprestado

> Uma armadilha escondida no chão.

**Efeito:** Armadilha no chão (dura 30 s): quem pisa leva 100→140% e fica preso por 2→3 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 100→140% | 100% | +3,64% | 140% | `Scale{1, 0.0364}` |
| 2→3 | 2 | +0,09 | 3 | `Scale{2, 0.0909}` |

- **Custo / recarga:** 20 MP / 15 s
- **Requisito:** 5 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Marca da Presa

Ativa · coluna **5** · **12** ranks · `SkillHuntPrey` (214) · ícone novo (Meshy); hoje usa `m_ember`, emprestado

> Escolhe a presa da caçada.

**Efeito:** Marca o alvo por 12 s: +8→16% de dano seu nele (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 8→16% | 8% | +0,73% | 16% | `Scale{0.08, 0.0073}` |

- **Custo / recarga:** 10 MP / 15 s
- **Requisito:** 5 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Olho de Falcão

Passiva · coluna **10** · **10** ranks · `SkillHuntHawk` (205) · ícone novo (Meshy); hoje usa `r_weak`, emprestado

> Vê longe e acerta onde vê.

**Efeito:** +1→5% de crítico e +0,5→2,5 m de alcance (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 1→5% | 1% | +0,44% | 5% | `Scale{0.01, 0.0044}` |
| 0,5→2,5 | 0,5 | +0,22 | 2,5 | `Scale{0.5, 0.2222}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Miasma

Ativa · coluna **10** · **12** ranks · `SkillMiasma` (60) · ícone `r_miasma`

> Quebra frascos em volta de si e o ar fica venenoso.

**Efeito:** Envenena todos a até 6 m: 30→40% do ataque/s por 6 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 30→40% | 30% | +0,91% | 40% | `Scale{0.3, 0.0091}` |

- **Custo / recarga:** 40 MP / 20 s
- **Requisito:** 10 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Passo de Caça

Passiva · coluna **10** · **10** ranks · `SkillHuntStride` (215) · ícone novo (Meshy); hoje usa `shadow_step`, emprestado

> Sempre um passo à frente da presa.

**Efeito:** +2→10% de velocidade de movimento (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Flecha Carregada

Modificador · coluna **15** · **8** ranks · `SkillHuntCharged` (206) · ícone novo (Meshy); hoje usa `rc_blades`, emprestado

> Com calma, o tiro pesa mais.

**Efeito:** Com 50+ de Precisão, o Tiro Preciso causa +10→30% (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 10→30% | 10% | +2,86% | 30% | `Scale{0.1, 0.0286}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Patrulheiro; 1 ponto em **Tiro Preciso**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Tiro Preciso 1".

### Bomba de Fumaça

Ativa · coluna **15** · **12** ranks · `SkillHuntSmoke` (210) · ícone `r_smoke`

> Uma nuvem negra cega os inimigos.

**Efeito:** Inimigos a até 4 m erram 20→40% mais por 6 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 20→40% | 20% | +1,82% | 40% | `Scale{0.2, 0.0182}` |

- **Custo / recarga:** 22 MP / 25 s
- **Requisito:** 15 pontos na barra de Patrulheiro.
- **Origem:** evol. (de `SkillSmokeBomb`).
- **No doc da evolução:** marco 25, 3 ranks, requisito "—".

### Recuo

Ativa · coluna **15** · **12** ranks · `SkillHuntRecoil` (216) · ícone novo (Meshy); hoje usa `r_dodge`, emprestado

> Salta para trás e atira no caminho.

**Efeito:** Salta 6 m para trás e dispara 120→160% (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 120→160% | 120% | +3,64% | 160% | `Scale{1.2, 0.0364}` |

- **Custo / recarga:** 20 MP / 18→12 s
- **Requisito:** 15 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 25, 3 ranks, requisito "—".

### Chuva de Flechas

Ativa · coluna **20** · **12** ranks · `SkillHuntRain` (84) · ícone novo (Meshy); hoje usa `rh_rain`, emprestado

> Um punhado de facas para o alto, e o céu faz o resto.

**Efeito:** 2 golpes de 160→200% no alvo e a até 5 m (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 160→200% | 160% | +3,64% | 200% | `Scale{1.6, 0.0364}` |

- **Custo / recarga:** 60 MP / 40 s
- **Requisito:** 20 pontos na barra de Patrulheiro.
- **Origem:** antes "Chuva de Lâminas".
- **No doc da evolução:** marco 30, 3 (hoje 1) ranks, requisito "5 no ramo".

### Campo de Espinhos

Ativa · coluna **20** · **12** ranks · `SkillHuntThorns` (211) · ícone novo (Meshy); hoje usa `r_flask`, emprestado

> Espinhos por todo o chão.

**Efeito:** Área de 6 m por 8 s: 20→30% do ataque/s e 30% de lentidão (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 20→30% | 20% | +0,91% | 30% | `Scale{0.2, 0.0091}` |

- **Custo / recarga:** 30 MP + **25 Precisão** / 30 s
- **Requisito:** 20 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Perseguição

Ativa · coluna **20** · **12** ranks · `SkillHuntChase` (217) · ícone novo (Meshy); hoje usa `k_charge`, emprestado

> Corre atrás da presa marcada.

**Efeito:** Por 6 s, +30% de velocidade e +10→20% de dano contra o alvo marcado (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 10→20% | 10% | +0,91% | 20% | `Scale{0.1, 0.0091}` |

- **Custo / recarga:** 10 MP + **20 Precisão** / 25 s
- **Requisito:** 20 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Mestre Armadilheiro

Passiva · coluna **32** · **10** ranks · `SkillHuntMaster` (212) · ícone novo (Meshy); hoje usa `kh_dawn`, emprestado

> Cada armadilha no lugar certo.

**Efeito:** Armadilhas e redes causam +6→30% de dano e prendem por +0,1→0,5 s (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 6→30% | 6% | +2,67% | 30% | `Scale{0.06, 0.0267}` |
| 0,1→0,5 | 0,1 | +0,04 | 0,5 | `Scale{0.1, 0.0444}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 35, 5 ranks, requisito "10 no ramo".

### Instinto do Caçador

Passiva · coluna **32** · **10** ranks · `SkillHuntInstinct` (218) · ícone novo (Meshy); hoje usa `b_bear`, emprestado

> Presa lenta é presa morta.

**Efeito:** +5→15% de dano contra alvos lentos ou presos (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 5→15% | 5% | +1,11% | 15% | `Scale{0.05, 0.0111}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Flecha Mortal

Ativa · coluna **40** · **12** ranks · `SkillHuntDeadly` (207) · ícone novo (Meshy); hoje usa `r_fatal`, emprestado

> O tiro que não erra.

**Efeito:** 350→420%, sem errar (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 350→420% | 350% | +6,36% | 420% | `Scale{3.5, 0.0636}` |

- **Custo / recarga:** 30 MP + **40 Precisão** / 25 s
- **Requisito:** 40 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Mira Perfeita

Chave · coluna **50** · **1** rank · `SkillHuntAim` (208) · ícone novo (Meshy); hoje usa `rc_sentence`, emprestado

> Respira, mira, solta.

**Efeito:** Com Precisão cheia, o próximo tiro sai com crítico garantido e atravessa até 3 inimigos

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Emboscada

Chave · coluna **50** · **1** rank · `SkillHuntAmbush` (213) · ícone novo (Meshy); hoje usa `r_shadow`, emprestado

> Preso, é só mirar.

**Efeito:** Inimigos presos por armadilha ou rede recebem +25% de dano dos seus tiros

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Caçada Implacável

Chave · coluna **50** · **1** rank · `SkillHuntRelentless` (219) · ícone novo (Meshy); hoje usa `b_seismic`, emprestado

> Uma presa cai, a próxima já está marcada.

**Efeito:** Matar o alvo marcado devolve 50 de Precisão e passa a marca para o inimigo mais próximo

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Patrulheiro.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".


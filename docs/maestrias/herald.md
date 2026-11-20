# Paladino (maestria de Cavaleiro)

> Gerado do protótipo [Maestrias de Erevia](https://claude.ai/artifact/4P5N9Bg2LCeP7n4bjdKkAZ) e de [docs/evolucoes/herald.md](../evolucoes/herald.md). As regras gerais estão no [README](README.md). Os números são os do doc da evolução, espalhados pelos ranks do sistema de maestrias, e ainda vão ser calibrados no simulador.

- **ID:** `herald` · **classe-base:** Cavaleiro · **recurso:** Fé
- **Barra da maestria:** cada ponto dá +1 Sab e +8 de vida (no 50: +50 Sab, +400 de vida).
- **Papel:** **Suporte de combate com dano sagrado.** O Paladino luta na linha de frente, abençoa o grupo e pune quem enfraquece. **Ele não é curandeiro.** A cura é pequena, tem recarga e serve para segurar o grupo, não para sustentar um tanque sozinho (decisão pendente 8 do design, com esta como proposta).
- **Duplas:** Bastião → **Cruzado** · Berserker → **Zelote** · Senhor da Guerra → **Campeão** · Elementalista → **Teurgo** · Arcanista → **Exorcista** · Assassino → **Penitente** · Patrulheiro → **Caçador de Bruxas**

## Árvore

Colunas = pontos na barra que liberam a skill. ■ ativa · ● passiva · ○ modificador (ligado à skill da linha) · ◆ chave.

| 1 | 5 | 10 | 15 | 20 | 25 | 32 | 40 | 50 |
|---|---|---|---|---|---|---|---|---|
| ■ Golpe Radiante |  |  | ○ Veredito |  | ● Fervor |  |  | ◆ Sentença Sagrada |
| ● Zelo | ■ Martelo da Aurora |  |  |  |  | ○ Aurora Ardente |  |  |
|  | ■ Bênção da Prontidão | ■ Corte do Horizonte |  | ■ Renovação |  | ● Graça |  | ◆ Bênção Suprema |
| ■ Bênção da Luz |  |  | ○ Bênção Ampliada |  |  |  |  |  |
|  | ● Escudo da Fé | ● Aura de Coragem |  | ■ Purificar |  |  | ■ Penitência | ◆ Sacrifício |
| ■ Sol Nascente |  |  | ○ Luz Cegante |  |  |  |  |  |

**Total:** 21 skills, 193 ranks.

## Recurso: Fé

| Gera | Quanto |
|---|---|
| Acertar uma skill sagrada | +8 (+2→6 com Veredito no Golpe Radiante) |
| Crítico sagrado, com Fervor | +3 |
| Bênção ou cura em um aliado que **não esteja com vida cheia nem com a bênção ativa** | +4 por aliado (máx. 3 por lançamento) |
| Bênção em si mesmo | +4 |
| Abate de inimigo em que você causou dano | +10 |

| Gasta | Quanto |
|---|---|
| Renovação | 20 |
| Bênção Suprema (chave) | 60 |
| Sacrifício (chave) | 50 |
| Sentença Sagrada (chave) | toda (+1% de dano por ponto, até +100%) |

## Skills

Na ordem da árvore: coluna por coluna, de cima para baixo.

### Golpe Radiante ✝

Ativa · coluna **1** · **12** ranks · `SkillHeraldStrike` (100) · ícone novo (Meshy); hoje usa `k_judg`, emprestado

> A lâmina brilha e corta com a luz da ordem.

**Efeito:** 170→210% do ataque (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 170→210% | 170% | +3,64% | 210% | `Scale{1.7, 0.0364}` |

- **Custo / recarga:** 12 MP / 6 s
- **Requisito:** 1 ponto na barra de Paladino.
- **Modificadores ligados:** Veredito.
- **Origem:** nova.
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Zelo

Passiva · coluna **1** · **10** ranks · `SkillHeraldZeal` (101) · ícone novo (Meshy); hoje usa `k_fencing`, emprestado

> A fé guia a lâmina.

**Efeito:** +2→10% de dano das skills sagradas (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |

- **Custo / recarga:** —
- **Requisito:** 1 ponto na barra de Paladino.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Bênção da Luz

Ativa · coluna **1** · **12** ranks · `SkillHeraldLight` (106) · ícone novo (Meshy); hoje usa `k_cry`, emprestado

> Uma prece curta que acende o grupo.

**Efeito:** Você e os aliados a até 12 m ganham +10→20% de ataque e +10→20% de defesa por 15 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 10→20% | 10% | +0,91% | 20% | `Scale{0.1, 0.0091}` |
| 10→20% | 10% | +0,91% | 20% | `Scale{0.1, 0.0091}` |

- **Custo / recarga:** 20 MP / 30 s
- **Requisito:** 1 ponto na barra de Paladino.
- **Modificadores ligados:** Bênção Ampliada.
- **Origem:** nova.
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Sol Nascente ✝

Ativa · coluna **1** · **12** ranks · `SkillHeraldSun` (66) · ícone `kh_sun`

> Ergue a arma e o campo inteiro se acende.

**Efeito:** 250→310% a até 7 m e cega os inimigos (35% mais erros) por 5 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 250→310% | 250% | +5,45% | 310% | `Scale{2.5, 0.0545}` |

- **Custo / recarga:** 60 MP / 50 s
- **Requisito:** 1 ponto na barra de Paladino.
- **Modificadores ligados:** Luz Cegante.
- **No doc da evolução:** marco 20, 3 (hoje 1) ranks, requisito "—".

### Martelo da Aurora ✝

Ativa · coluna **5** · **12** ranks · `SkillHeraldDawn` (64) · ícone `kh_dawn`

> Um golpe que desce como o primeiro raio do dia.

**Efeito:** 190→250% no alvo e a até 3 m, sem errar; os atingidos perdem 15→25% de ataque por 6 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 190→250% | 190% | +5,45% | 250% | `Scale{1.9, 0.0545}` |
| 15→25% | 15% | +0,91% | 25% | `Scale{0.15, 0.0091}` |

- **Custo / recarga:** 30 MP / 14 s
- **Requisito:** 5 pontos na barra de Paladino.
- **Modificadores ligados:** Aurora Ardente.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Bênção da Prontidão

Ativa · coluna **5** · **12** ranks · `SkillHeraldBless` (65) · ícone novo (Meshy); hoje usa `kh_bless`, emprestado

> A luz da ordem apressa as mãos de quem luta ao seu lado.

**Efeito:** Você e os aliados a até 12 m ganham +15→25% de velocidade de ataque por 15 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 15→25% | 15% | +0,91% | 25% | `Scale{0.15, 0.0091}` |

- **Custo / recarga:** 40 MP / 40 s
- **Requisito:** 5 pontos na barra de Paladino.
- **Origem:** antes "Bênção do Arauto".
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Escudo da Fé

Passiva · coluna **5** · **10** ranks · `SkillHeraldFaithWard` (112) · ícone novo (Meshy); hoje usa `m_shield`, emprestado

> A fé gasta vira proteção.

**Efeito:** A cada 10 de Fé gasta, ganha um escudo de 1→3% da vida máxima por 10 s (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 1→3% | 1% | +0,22% | 3% | `Scale{0.01, 0.0022}` |

- **Custo / recarga:** —
- **Requisito:** 5 pontos na barra de Paladino.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Corte do Horizonte ✝

Ativa · coluna **10** · **12** ranks · `SkillHorizon` (50) · ícone `k_horizon`

> Um corte tão largo que parece partir o horizonte.

**Efeito:** 200→260% no alvo e a até 3,5 m; os atingidos ficam 40% mais lentos por 3 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 200→260% | 200% | +5,45% | 260% | `Scale{2, 0.0545}` |

- **Custo / recarga:** 32 MP / 15 s
- **Requisito:** 10 pontos na barra de Paladino.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Aura de Coragem

Passiva · coluna **10** · **10** ranks · `SkillHeraldCourage` (107) · ícone `k_aura`

> A presença do cavaleiro mantém os aliados de pé.

**Efeito:** Você e os aliados a até 10 m regeneram 2→10 de vida por segundo (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10 | 2 | +0,89 | 10 | `Scale{2, 0.8889}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Paladino.
- **Origem:** evol. (de `SkillCourage`).
- **No doc da evolução:** marco 20, 5 (na base, 3) ranks, requisito "—".

### Veredito

Modificador · coluna **15** · **8** ranks · `SkillHeraldVerdict` (102) · ícone novo (Meshy); hoje usa `k_spin`, emprestado

> O golpe pune quem já foi julgado.

**Efeito:** +10→30% contra alvos enfraquecidos ou cegos; +2→6 Fé por acerto (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 10→30% | 10% | +2,86% | 30% | `Scale{0.1, 0.0286}` |
| 2→6 | 2 | +0,57 | 6 | `Scale{2, 0.5714}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Paladino; 1 ponto em **Golpe Radiante**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Golpe Radiante 1".

### Bênção Ampliada

Modificador · coluna **15** · **8** ranks · `SkillHeraldLonger` (108) · ícone novo (Meshy); hoje usa `b_roar`, emprestado

> A bênção demora a se apagar.

**Efeito:** +5→15 s de duração (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 5→15 | 5 | +1,43 | 15 | `Scale{5, 1.4286}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Paladino; 1 ponto em **Bênção da Luz**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Bênção da Luz 1".

### Luz Cegante

Modificador · coluna **15** · **8** ranks · `SkillHeraldBlinding` (113) · ícone novo (Meshy); hoje usa `r_smoke`, emprestado

> O sol que nasce queima os olhos.

**Efeito:** +5→15% na cegueira e +1→3 s de duração (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 5→15% | 5% | +1,43% | 15% | `Scale{0.05, 0.0143}` |
| 1→3 | 1 | +0,29 | 3 | `Scale{1, 0.2857}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Paladino; 1 ponto em **Sol Nascente**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Sol Nascente 1".

### Renovação

Ativa · coluna **20** · **12** ranks · `SkillHeraldRenewal` (109) · ícone novo (Meshy); hoje usa `m_mind`, emprestado

> Uma luz morna fecha as feridas.

**Efeito:** **Cura limitada:** você e os aliados a até 10 m recuperam 8→12% da vida máxima ao longo de 6 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 8→12% | 8% | +0,36% | 12% | `Scale{0.08, 0.0036}` |

- **Custo / recarga:** 35 MP + **20 Fé** / 40 s
- **Requisito:** 20 pontos na barra de Paladino.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Purificar

Ativa · coluna **20** · **12** ranks · `SkillHeraldPurify` (114) · ícone novo (Meshy); hoje usa `r_flask`, emprestado

> A luz tira o mal de dentro.

**Efeito:** Tira 1 efeito negativo (veneno, queimadura, praga, lentidão ou prender) de você e de 1→3 aliados a até 10 m e cura 3% da vida máxima de cada um (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 1→3 | 1 | +0,18 | 3 | `Scale{1, 0.1818}` |

- **Custo / recarga:** 25 MP / 20→14 s
- **Requisito:** 20 pontos na barra de Paladino.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Fervor

Passiva · coluna **25** · **10** ranks · `SkillHeraldFervor` (103) · ícone novo (Meshy); hoje usa `r_weak`, emprestado

> A luz acha a brecha.

**Efeito:** +1→5% de chance de crítico nas sagradas; crítico sagrado gera +3 Fé (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 1→5% | 1% | +0,44% | 5% | `Scale{0.01, 0.0044}` |

- **Custo / recarga:** —
- **Requisito:** 25 pontos na barra de Paladino.
- **No doc da evolução:** marco 30, 5 ranks, requisito "5 no ramo".

### Aurora Ardente

Modificador · coluna **32** · **8** ranks · `SkillHeraldBlaze` (104) · ícone novo (Meshy); hoje usa `m_inferno`, emprestado

> O Martelo abre o dia mais largo.

**Efeito:** +1→3 m de raio; o enfraquecimento dura +2→4 s (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 1→3 | 1 | +0,29 | 3 | `Scale{1, 0.2857}` |
| 2→4 | 2 | +0,29 | 4 | `Scale{2, 0.2857}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Paladino; 1 ponto em **Martelo da Aurora**.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Graça

Passiva · coluna **32** · **10** ranks · `SkillHeraldGrace` (110) · ícone novo (Meshy); hoje usa `b_thirst`, emprestado

> Cura com as duas mãos.

**Efeito:** Curas e escudos que você dá ficam +5→15% mais fortes (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 5→15% | 5% | +1,11% | 15% | `Scale{0.05, 0.0111}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Paladino.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Penitência ✝

Ativa · coluna **40** · **12** ranks · `SkillHeraldPenance` (115) · ícone novo (Meshy); hoje usa `r_mark`, emprestado

> Marca o culpado para o julgamento.

**Efeito:** 120% e marca o alvo por 8 s: ele recebe +8→16% de dano sagrado e causa −10% de dano (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 8→16% | 8% | +0,73% | 16% | `Scale{0.08, 0.0073}` |

- **Custo / recarga:** 25 MP / 20 s
- **Requisito:** 40 pontos na barra de Paladino.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Sentença Sagrada ✝

Chave · coluna **50** · **1** rank · `SkillHeraldSentence` (105) · ícone novo (Meshy); hoje usa `b_exec`, emprestado

> Toda a fé num golpe só.

**Efeito:** 450%, sem errar; gasta toda a Fé e ganha +1% de dano por ponto gasto

- **Custo / recarga:** 40 MP + **toda a Fé** / 45 s
- **Requisito:** 50 pontos na barra de Paladino.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Bênção Suprema

Chave · coluna **50** · **1** rank · `SkillHeraldSupreme` (111) · ícone novo (Meshy); hoje usa `m_arcane`, emprestado

> A ordem inteira canta junto.

**Efeito:** Renova as suas bênçãos ativas nos aliados a até 15 m e dobra o efeito delas por 8 s

- **Custo / recarga:** **60 Fé** / 90 s
- **Requisito:** 50 pontos na barra de Paladino.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Sacrifício

Chave · coluna **50** · **1** rank · `SkillHeraldSacrifice` (116) · ícone novo (Meshy); hoje usa `b_undying`, emprestado

> O Paladino paga pelos outros.

**Efeito:** Por 6 s, os aliados a até 10 m não caem abaixo de 25% de vida: o dano que passaria disso vai para você

- **Custo / recarga:** **50 Fé** / 120 s
- **Requisito:** 50 pontos na barra de Paladino.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".


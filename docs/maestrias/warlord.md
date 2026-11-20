# Senhor da Guerra (maestria de Bárbaro)

> Gerado do protótipo [Maestrias de Erevia](https://claude.ai/artifact/4P5N9Bg2LCeP7n4bjdKkAZ) e de [docs/evolucoes/warlord.md](../evolucoes/warlord.md). As regras gerais estão no [README](README.md). Os números são os do doc da evolução, espalhados pelos ranks do sistema de maestrias, e ainda vão ser calibrados no simulador.

- **ID:** `warlord` · **classe-base:** Bárbaro · **recurso:** Ímpeto
- **Barra da maestria:** cada ponto dá +0,5 For, +0,5 Con e +8 de vida (no 50: +25 For, +25 Con, +400 de vida).
- **Papel:** **Linha de frente que lidera:** inicia a luta, abre espaço para os aliados e transforma força bruta em vantagem para o grupo, com brados, estandartes e controle. Mata um pouco mais devagar que o Berserker e aguenta mais. Em grupo, os bônus dele valem mais que o dano.
- **Duplas:** Bastião → **Marechal** · Paladino → **Campeão** · Berserker → **Chefe de Clã** · Elementalista → **Senhor da Tormenta** · Arcanista → **Tirano** · Assassino → **Mercenário** · Patrulheiro → **Desbravador**

## Árvore

Colunas = pontos na barra que liberam a skill. ■ ativa · ● passiva · ○ modificador (ligado à skill da linha) · ◆ chave.

| 1 | 5 | 10 | 15 | 20 | 25 | 32 | 40 | 50 |
|---|---|---|---|---|---|---|---|---|
| ■ Brado de Guerra |  |  | ○ Ordem de Avanço | ■ Grito de Reagrupar |  |  |  | ◆ Comando Absoluto |
|  | ■ Estandarte de Guerra |  |  |  |  | ○ Estandarte Tremulante |  |  |
| ■ Golpe de Comando |  | ● Liderança |  | ■ Impacto |  | ● Ímpeto de Carga |  | ◆ Investida Implacável |
|  | ■ Avanço de Guerra |  | ○ Quebra-Linha |  |  |  |  |  |
| ■ Pele de Pedra | ■ Rugido | ● Braço de Guerra | ■ Pisotear |  |  | ○ Domínio do Campo |  | ◆ Presença Dominante |
|  |  | ● Couro Grosso |  | ■ Desafio |  |  |  |  |

**Total:** 21 skills, 195 ranks.

## Recurso: Ímpeto

| Gera | Quanto |
|---|---|
| **Iniciar combate:** o primeiro golpe num inimigo que não estava lutando | +15 (uma vez por inimigo) |
| Ataque básico acertado | +3 |
| Golpe de Comando | +10 |
| Acertar 2 ou mais alvos com uma skill | +3 por alvo extra (máx. 3) |
| **Interromper:** atordoar um monstro durante o aviso no chão de um ataque dele | +15 (o ataque é cancelado) |
| Rugido | +5 por inimigo (máx. 3) |
| Brado de Guerra, com Ordem de Avanço | +5 por aliado (máx. 3) |

| Gasta | Quanto |
|---|---|
| Desafio | 20 |
| Impacto | 25 |
| Grito de Reagrupar | 25 |
| Investida Implacável (chave) | 60 |

## Skills

Na ordem da árvore: coluna por coluna, de cima para baixo.

### Brado de Guerra

Ativa · coluna **1** · **12** ranks · `SkillWarlordCry` (70) · ícone novo (Meshy); hoje usa `bw_cry`, emprestado

> Um brado que põe a tropa inteira em marcha.

**Efeito:** Você e os aliados a até 12 m ganham +12→20% de ataque por 20 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 12→20% | 12% | +0,73% | 20% | `Scale{0.12, 0.0073}` |

- **Custo / recarga:** 30 MP / 35 s
- **Requisito:** 1 ponto na barra de Senhor da Guerra.
- **Modificadores ligados:** Ordem de Avanço.
- **Origem:** antes "Brado do Caudilho".
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Golpe de Comando

Ativa · coluna **1** · **12** ranks · `SkillWarlordCommand` (138) · ícone novo (Meshy); hoje usa `brutal_strike`, emprestado

> Abre a luta e dá o ritmo para a tropa.

**Efeito:** 200→250% do ataque; gera 10 Ímpeto (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 200→250% | 200% | +4,55% | 250% | `Scale{2, 0.0455}` |

- **Custo / recarga:** 10 MP / 6 s
- **Requisito:** 1 ponto na barra de Senhor da Guerra.
- **Origem:** nova.
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Pele de Pedra

Ativa · coluna **1** · **12** ranks · `SkillStoneSkin` (54) · ícone `b_stone`

> A pele endurece como rocha e o chão treme em volta.

**Efeito:** +35→55% de defesa por 10 s; os inimigos a até 5 m levam 100→130% e ficam 30% mais lentos por 3 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 35→55% | 35% | +1,82% | 55% | `Scale{0.35, 0.0182}` |
| 100→130% | 100% | +2,73% | 130% | `Scale{1, 0.0273}` |

- **Custo / recarga:** 35 MP / 30 s
- **Requisito:** 1 ponto na barra de Senhor da Guerra.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Estandarte de Guerra

Ativa · coluna **5** · **12** ranks · `SkillWarlordBanner` (72) · ícone `bw_banner`

> Crava o estandarte no chão com um golpe que espalha o inimigo.

**Efeito:** 150→190% a até 6 m (os inimigos perdem 20% de ataque por 8 s) e finca um estandarte por 20 s: +20% de defesa a até 8 m (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 150→190% | 150% | +3,64% | 190% | `Scale{1.5, 0.0364}` |

- **Custo / recarga:** 40 MP (hoje 55) / 75 s
- **Requisito:** 5 pontos na barra de Senhor da Guerra.
- **Modificadores ligados:** Estandarte Tremulante.
- **No doc da evolução:** marco 20, 3 (hoje 1) ranks, requisito "—".

### Avanço de Guerra

Ativa · coluna **5** · **12** ranks · `SkillWarlordCharge` (71) · ícone novo (Meshy); hoje usa `bw_charge`, emprestado

> Abre caminho no meio da formação inimiga.

**Efeito:** Salta até o alvo (até 12 m) e causa 180→230% nele e a até 4 m dele; atordoa por 1→1,5 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 180→230% | 180% | +4,55% | 230% | `Scale{1.8, 0.0455}` |
| 1→1,5 | 1 | +0,05 | 1,5 | `Scale{1, 0.0455}` |

- **Custo / recarga:** 40 MP / 22 s
- **Requisito:** 5 pontos na barra de Senhor da Guerra.
- **Modificadores ligados:** Quebra-Linha.
- **Origem:** antes "Avanço do Caudilho".
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Rugido

Ativa · coluna **5** · **12** ranks · `SkillWarlordRoar` (144) · ícone `b_roar`

> Um rugido que faz os inimigos hesitarem.

**Efeito:** Os inimigos a até 6 m perdem 15→25% de ataque por 8 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 15→25% | 15% | +0,91% | 25% | `Scale{0.15, 0.0091}` |

- **Custo / recarga:** 18 MP / 20 s
- **Requisito:** 5 pontos na barra de Senhor da Guerra.
- **Origem:** evol. (de `SkillRoar`).
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Liderança

Passiva · coluna **10** · **10** ranks · `SkillWarlordLead` (133) · ícone novo (Meshy); hoje usa `k_aura`, emprestado

> A tropa bate mais forte perto do comandante.

**Efeito:** Os aliados a até 12 m causam +1→5% de dano (não soma entre Senhores da Guerra) (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 1→5% | 1% | +0,44% | 5% | `Scale{0.01, 0.0044}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Senhor da Guerra.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Braço de Guerra

Passiva · coluna **10** · **10** ranks · `SkillWarlordArm` (139) · ícone novo (Meshy); hoje usa `k_fencing`, emprestado

> Anos de guerra no braço.

**Efeito:** +2→10% de ataque físico (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Senhor da Guerra.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Couro Grosso

Passiva · coluna **10** · **10** ranks · `SkillWarlordHide` (145) · ícone `b_hide`

> Pele curtida por mil batalhas.

**Efeito:** +5→25% de vida máxima (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 5→25% | 5% | +2,22% | 25% | `Scale{0.05, 0.0222}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Senhor da Guerra.
- **Origem:** evol. (de `SkillThickHide`).
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Ordem de Avanço

Modificador · coluna **15** · **8** ranks · `SkillWarlordAdvance` (134) · ícone novo (Meshy); hoje usa `k_cry`, emprestado

> O brado põe a tropa em marcha.

**Efeito:** O Brado também dá +10→20% de velocidade de movimento; +5 Ímpeto por aliado (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 10→20% | 10% | +1,43% | 20% | `Scale{0.1, 0.0143}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Senhor da Guerra; 1 ponto em **Brado de Guerra**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Brado 1".

### Quebra-Linha

Modificador · coluna **15** · **8** ranks · `SkillWarlordBreak` (140) · ícone novo (Meshy); hoje usa `k_charge`, emprestado

> O avanço rompe a linha inimiga.

**Efeito:** −3→9 s de recarga; +5→15 Ímpeto se acertar 2 ou mais (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 3→9 | 3 | +0,86 | 9 | `Scale{3, 0.8571}` |
| 5→15 | 5 | +1,43 | 15 | `Scale{5, 1.4286}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Senhor da Guerra; 1 ponto em **Avanço de Guerra**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Avanço 1".

### Pisotear

Ativa · coluna **15** · **12** ranks · `SkillWarlordStomp` (146) · ícone `b_stomp`

> Pisa com tanta força que o chão treme.

**Efeito:** 100% a até 4 m e atordoa por 1,5→2,5 s (interrompe) (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 1,5→2,5 | 1,5 | +0,09 | 2,5 | `Scale{1.5, 0.0909}` |

- **Custo / recarga:** 24 MP / 18 s
- **Requisito:** 15 pontos na barra de Senhor da Guerra.
- **Modificadores ligados:** Domínio do Campo.
- **Origem:** evol. (de `SkillStomp`).
- **No doc da evolução:** marco 25, 3 ranks, requisito "—".

### Grito de Reagrupar

Ativa · coluna **20** · **12** ranks · `SkillWarlordRally` (135) · ícone novo (Meshy); hoje usa `m_blink`, emprestado

> Ninguém fica para trás.

**Efeito:** Tira lentidão e prender de você e dos aliados a até 12 m e dá imunidade a esses efeitos por 3→5 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 3→5 | 3 | +0,18 | 5 | `Scale{3, 0.1818}` |

- **Custo / recarga:** **25 Ímpeto** / 40 s
- **Requisito:** 20 pontos na barra de Senhor da Guerra.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Impacto

Ativa · coluna **20** · **12** ranks · `SkillWarlordImpact` (141) · ícone novo (Meshy); hoje usa `b_quake`, emprestado

> Bate o machado no chão e corta o golpe inimigo.

**Efeito:** 160→200% a até 4 m e atordoa por 1 s; interromper com ele gera o dobro de Ímpeto (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 160→200% | 160% | +3,64% | 200% | `Scale{1.6, 0.0364}` |

- **Custo / recarga:** **25 Ímpeto** / 14 s
- **Requisito:** 20 pontos na barra de Senhor da Guerra.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Desafio

Ativa · coluna **20** · **12** ranks · `SkillWarlordDefy` (147) · ícone novo (Meshy); hoje usa `k_taunt`, emprestado

> Chama todos para a briga.

**Efeito:** Provoca os inimigos a até 6 m por 3→5 s e dá +20% de defesa (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 3→5 | 3 | +0,18 | 5 | `Scale{3, 0.1818}` |

- **Custo / recarga:** **20 Ímpeto** / 25 s
- **Requisito:** 20 pontos na barra de Senhor da Guerra.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Estandarte Tremulante

Modificador · coluna **32** · **8** ranks · `SkillWarlordFlag` (136) · ícone novo (Meshy); hoje usa `k_banner`, emprestado

> O estandarte se vê de longe.

**Efeito:** +2→4 m de raio e +5→15 s de duração (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 2→4 | 2 | +0,29 | 4 | `Scale{2, 0.2857}` |
| 5→15 | 5 | +1,43 | 15 | `Scale{5, 1.4286}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Senhor da Guerra; 1 ponto em **Estandarte de Guerra**.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Ímpeto de Carga

Passiva · coluna **32** · **10** ranks · `SkillWarlordMomentum` (142) · ícone novo (Meshy); hoje usa `r_shadow`, emprestado

> Depois da carga, ninguém o segura.

**Efeito:** +2→10% de dano nos 5 s depois de um Avanço; +4→20% de velocidade de movimento em combate (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |
| 4→20% | 4% | +1,78% | 20% | `Scale{0.04, 0.0178}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Senhor da Guerra.
- **No doc da evolução:** marco 35, 5 ranks, requisito "10 no ramo".

### Domínio do Campo

Modificador · coluna **32** · **8** ranks · `SkillWarlordField` (148) · ícone novo (Meshy); hoje usa `m_rain`, emprestado

> O campo é de quem pisa.

**Efeito:** +1→3 m de raio; depois do atordoamento, 30% de lentidão por 2→4 s (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 1→3 | 1 | +0,29 | 3 | `Scale{1, 0.2857}` |
| 2→4 | 2 | +0,29 | 4 | `Scale{2, 0.2857}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Senhor da Guerra; 1 ponto em **Pisotear**.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Comando Absoluto

Chave · coluna **50** · **1** rank · `SkillWarlordCommand2` (137) · ícone novo (Meshy); hoje usa `m_arcane`, emprestado

> Com a tropa por perto, tudo fica mais fácil.

**Efeito:** Com um aliado a até 12 m, você gera +50% de Ímpeto, e o Brado e o Estandarte custam 50% menos MP

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Senhor da Guerra.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Investida Implacável

Chave · coluna **50** · **1** rank · `SkillWarlordRelent` (143) · ícone novo (Meshy); hoje usa `b_seismic`, emprestado

> Atravessa a linha inimiga de ponta a ponta.

**Efeito:** Salta até 3 vezes em 6 s entre inimigos (até 10 m), com 150% em cada salto, sem errar

- **Custo / recarga:** **60 Ímpeto** / 45 s
- **Requisito:** 50 pontos na barra de Senhor da Guerra.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Presença Dominante

Chave · coluna **50** · **1** rank · `SkillWarlordPresence` (149) · ícone novo (Meshy); hoje usa `b_bear`, emprestado

> Perto dele, ninguém se sente forte.

**Efeito:** Os inimigos a até 6 m causam −10% de dano e ficam 15% mais lentos; seus atordoamentos duram +0,5 s

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Senhor da Guerra.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".


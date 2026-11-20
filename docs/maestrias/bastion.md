# Bastião (maestria de Cavaleiro)

> Gerado do protótipo [Maestrias de Erevia](https://claude.ai/artifact/4P5N9Bg2LCeP7n4bjdKkAZ) e de [docs/evolucoes/bastion.md](../evolucoes/bastion.md). As regras gerais estão no [README](README.md). Os números são os do doc da evolução, espalhados pelos ranks do sistema de maestrias, e ainda vão ser calibrados no simulador.

- **ID:** `bastion` · **classe-base:** Cavaleiro · **recurso:** Determinação
- **Barra da maestria:** cada ponto dá +1 Con e +8 de vida (no 50: +50 Con, +400 de vida).
- **Papel:** **Tanque principal.** Protege os aliados e controla os inimigos. O Bastião transforma defesa em valor para o grupo: segura os inimigos, tira a pressão de cima dos aliados e sobrevive a picos de dano.
- **Duplas:** Paladino → **Cruzado** · Berserker → **Colosso** · Senhor da Guerra → **Marechal** · Elementalista → **Guardião Rúnico** · Arcanista → **Guardião do Véu** · Assassino → **Vigia Sombrio** · Patrulheiro → **Guarda-Fronteira**

## Árvore

Colunas = pontos na barra que liberam a skill. ■ ativa · ● passiva · ○ modificador (ligado à skill da linha) · ◆ chave.

| 1 | 5 | 10 | 15 | 20 | 25 | 32 | 40 | 50 |
|---|---|---|---|---|---|---|---|---|
| ■ Guarda Inabalável |  |  | ○ Postura de Ferro |  | ● Muralha Viva |  | ■ Último Bastião | ◆ Último Reduto |
|  | ● Pele de Aço | ● Vigor do Bastião |  | ■ Interceptar |  | ● Guarda do Escudo |  | ◆ Juramento de Proteção |
| ■ Égide da Ordem |  |  | ○ Égide Compartilhada |  |  |  |  |  |
|  | ● Vínculo de Guarda | ■ Baluarte |  | ■ Zona de Provocação |  |  |  | ◆ Prisão do Guardião |
| ■ Golpe de Escudo |  |  |  |  |  | ○ Eco do Escudo |  |  |
|  | ■ Desafio do Bastião |  | ○ Desafio Reforçado |  |  |  |  |  |

**Total:** 20 skills, 181 ranks.

## Recurso: Determinação

| Gera | Quanto |
|---|---|
| Bloquear um golpe | +6 |
| Receber dano de inimigo a até 5 m | +1 por golpe |
| Golpe de Escudo acertar | +10 |
| Guarda Inabalável | +15 ao lançar |
| Desafio do Bastião | +5 por inimigo provocado (máx. 3) |
| Égide da Ordem | +5 por aliado protegido (máx. 3) |
| Golpe redirecionado por Interceptar ou Juramento | igual a receber dano |

| Gasta | Quanto |
|---|---|
| Zona de Provocação | 20 |
| Último Bastião | 40 |
| Juramento de Proteção (chave) | 50 |
| Prisão do Guardião (chave) | 60 |
| Último Reduto (chave) | toda (escudo proporcional) |

## Skills

Na ordem da árvore: coluna por coluna, de cima para baixo.

### Guarda Inabalável

Ativa · coluna **1** · **12** ranks · `SkillBastionGuard` (85) · ícone novo (Meshy); hoje usa `fortress`, emprestado

> Planta o escudo no chão e não sai do lugar.

**Efeito:** +40→60% de defesa e +20% de bloqueio por 8 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 40→60% | 40% | +1,82% | 60% | `Scale{0.4, 0.0182}` |

- **Custo / recarga:** 15 MP / 20 s
- **Requisito:** 1 ponto na barra de Bastião.
- **Modificadores ligados:** Postura de Ferro.
- **Origem:** nova.
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Égide da Ordem

Ativa · coluna **1** · **12** ranks · `SkillAegis` (51) · ícone `k_aegis`

> O juramento da ordem protege quem luta ao seu lado.

**Efeito:** Escudo de 250→500 por 8 s em você e nos aliados a até 10 m (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 250→500 | 250 | +22,73 | 500 | `Scale{250, 22.7273}` |

- **Custo / recarga:** 45 MP / 45 s
- **Requisito:** 1 ponto na barra de Bastião.
- **Modificadores ligados:** Égide Compartilhada.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Golpe de Escudo

Ativa · coluna **1** · **12** ranks · `SkillShieldBash` (95) · ícone `shield_bash`

> Uma pancada seca com o escudo que deixa o inimigo atordoado.

**Efeito:** 150→190% e atordoa por 1,5→2 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 150→190% | 150% | +3,64% | 190% | `Scale{1.5, 0.0364}` |
| 1,5→2 | 1,5 | +0,05 | 2 | `Scale{1.5, 0.0455}` |

- **Custo / recarga:** 10 MP / 8 s
- **Requisito:** 1 ponto na barra de Bastião.
- **Modificadores ligados:** Eco do Escudo.
- **Origem:** evol..
- Era uma das 2 skills iniciais da evolução.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Pele de Aço

Passiva · coluna **5** · **10** ranks · `SkillBastionSteel` (86) · ícone novo (Meshy); hoje usa `k_iron`, emprestado

> Pele de Aço

**Efeito:** +4→20% de defesa física (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 4→20% | 4% | +1,78% | 20% | `Scale{0.04, 0.0178}` |

- **Custo / recarga:** —
- **Requisito:** 5 pontos na barra de Bastião.
- **Origem:** evol. da Pele de Ferro (de `SkillIronSkin`).
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Vínculo de Guarda

Passiva · coluna **5** · **10** ranks · `SkillBastionLink` (90) · ícone novo (Meshy); hoje usa `k_aura`, emprestado

> Quem luta ao lado do Bastião apanha menos.

**Efeito:** Aliados a até 10 m recebem −2→10% de dano (não soma entre Bastiões) (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |

- **Custo / recarga:** —
- **Requisito:** 5 pontos na barra de Bastião.
- **No doc da evolução:** marco 20, 5 ranks, requisito "—".

### Desafio do Bastião

Ativa · coluna **5** · **12** ranks · `SkillBastionDefy` (61) · ícone `kb_defy`

> Bate o escudo e chama todos para cima de si.

**Efeito:** Provoca os inimigos a até 7 m por 4 s e dá a você um escudo de 300→600 por 10 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 300→600 | 300 | +27,27 | 600 | `Scale{300, 27.2727}` |

- **Custo / recarga:** 35 MP / 25 s
- **Requisito:** 5 pontos na barra de Bastião.
- **Modificadores ligados:** Desafio Reforçado.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Vigor do Bastião

Passiva · coluna **10** · **10** ranks · `SkillBastionVigor` (62) · ícone `kb_vigor`

> Quem segura a linha aprende a não cair.

**Efeito:** +3→15% de vida máxima (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 3→15% | 3% | +1,33% | 15% | `Scale{0.03, 0.0133}` |

- **Custo / recarga:** —
- **Requisito:** 10 pontos na barra de Bastião.
- **No doc da evolução:** marco 20, 5 (hoje 3) ranks, requisito "—".

### Baluarte

Ativa · coluna **10** · **12** ranks · `SkillBulwark` (49) · ícone `k_bulwark`

> Crava o escudo no chão e vira o centro da batalha.

**Efeito:** 140→180% a até 5 m, atordoa por 1,5→2 s e provoca os atingidos (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 140→180% | 140% | +3,64% | 180% | `Scale{1.4, 0.0364}` |
| 1,5→2 | 1,5 | +0,05 | 2 | `Scale{1.5, 0.0455}` |

- **Custo / recarga:** 35 MP / 24 s
- **Requisito:** 10 pontos na barra de Bastião.
- **No doc da evolução:** marco 20, 3 ranks, requisito "—".

### Postura de Ferro

Modificador · coluna **15** · **8** ranks · `SkillBastionStance` (87) · ícone novo (Meshy); hoje usa `b_hide`, emprestado

> Quanto mais firme a postura, mais tempo a guarda aguenta.

**Efeito:** +2→6 s de duração; durante a Guarda, cada bloqueio gera +2→4 Determinação a mais (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 2→6 | 2 | +0,57 | 6 | `Scale{2, 0.5714}` |
| 2→4 | 2 | +0,29 | 4 | `Scale{2, 0.2857}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Bastião; 1 ponto em **Guarda Inabalável**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Guarda 1".

### Égide Compartilhada

Modificador · coluna **15** · **8** ranks · `SkillBastionShared` (91) · ícone novo (Meshy); hoje usa `m_shield`, emprestado

> A Égide cobre mais e volta antes.

**Efeito:** +10→30% no escudo e −5→15 s de recarga (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 10→30% | 10% | +2,86% | 30% | `Scale{0.1, 0.0286}` |
| 5→15 | 5 | +1,43 | 15 | `Scale{5, 1.4286}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Bastião; 1 ponto em **Égide da Ordem**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Égide 1".

### Desafio Reforçado

Modificador · coluna **15** · **8** ranks · `SkillBastionDefyPlus` (96) · ícone novo (Meshy); hoje usa `k_taunt`, emprestado

> O desafio chega mais longe e tira a força de quem atende.

**Efeito:** +1→3 m de raio; os provocados causam −5→15% de dano em você (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 1→3 | 1 | +0,29 | 3 | `Scale{1, 0.2857}` |
| 5→15% | 5% | +1,43% | 15% | `Scale{0.05, 0.0143}` |

- **Custo / recarga:** —
- **Requisito:** 15 pontos na barra de Bastião; 1 ponto em **Desafio do Bastião**.
- **No doc da evolução:** marco 25, 3 ranks, requisito "Desafio 1".

### Interceptar

Ativa · coluna **20** · **12** ranks · `SkillBastionIntercept` (92) · ícone novo (Meshy); hoje usa `k_charge`, emprestado

> Põe o escudo entre os aliados e o perigo.

**Efeito:** Salta até um aliado (até 12 m) e, por 4→6 s, recebe 30% do dano que ele levaria (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 4→6 | 4 | +0,18 | 6 | `Scale{4, 0.1818}` |

- **Custo / recarga:** 20 MP / 20 s
- **Requisito:** 20 pontos na barra de Bastião.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Zona de Provocação

Ativa · coluna **20** · **12** ranks · `SkillBastionZone` (97) · ícone novo (Meshy); hoje usa `b_roar`, emprestado

> Marca o chão: ali ninguém escapa do Bastião.

**Efeito:** Marca o chão (6 m) por 8 s: provoca quem está dentro a cada 2 s e deixa 20→40% mais lento (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 20→40% | 20% | +1,82% | 40% | `Scale{0.2, 0.0182}` |

- **Custo / recarga:** 30 MP + **20 Det.** / 30 s
- **Requisito:** 20 pontos na barra de Bastião.
- **No doc da evolução:** marco 30, 3 ranks, requisito "5 no ramo".

### Muralha Viva

Passiva · coluna **25** · **10** ranks · `SkillBastionWall` (88) · ícone `k_wall`

> Quando tudo parece perdido, o cavaleiro se fecha atrás do escudo.

**Efeito:** Abaixo de 30% de vida: escudo de 12→20% da vida máx. por 10 s e +30 Determinação (uma vez a cada 60 s) (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 12→20% | 12% | +0,89% | 20% | `Scale{0.12, 0.0089}` |

- **Custo / recarga:** —
- **Requisito:** 25 pontos na barra de Bastião.
- **Origem:** evol. (de `SkillLivingWall`).
- **No doc da evolução:** marco 30, 5 ranks, requisito "5 no ramo".

### Guarda do Escudo

Passiva · coluna **32** · **10** ranks · `SkillBastionShieldArt` (93) · ícone novo (Meshy); hoje usa `b_stone`, emprestado

> O escudo encontra cada golpe.

**Efeito:** +2→10% de chance de bloqueio; o bloqueio reduz 50→60% do dano (rank 1 → rank 10)

| Valor no doc | Rank 1 | Por rank | Rank 10 | No código |
|---|---|---|---|---|
| 2→10% | 2% | +0,89% | 10% | `Scale{0.02, 0.0089}` |
| 50→60% | 50% | +1,11% | 60% | `Scale{0.5, 0.0111}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Bastião.
- **No doc da evolução:** marco 35, 5 ranks, requisito "10 no ramo".

### Eco do Escudo

Modificador · coluna **32** · **8** ranks · `SkillBastionEcho` (98) · ícone novo (Meshy); hoje usa `b_quake`, emprestado

> A pancada ecoa nos vizinhos.

**Efeito:** O golpe atinge também até 2 inimigos a até 3 m do alvo, com 50→100% do dano (sem atordoar) (rank 1 → rank 8)

| Valor no doc | Rank 1 | Por rank | Rank 8 | No código |
|---|---|---|---|---|
| 50→100% | 50% | +7,14% | 100% | `Scale{0.5, 0.0714}` |

- **Custo / recarga:** —
- **Requisito:** 32 pontos na barra de Bastião; 1 ponto em **Golpe de Escudo**.
- **No doc da evolução:** marco 35, 3 ranks, requisito "10 no ramo".

### Último Bastião

Ativa · coluna **40** · **12** ranks · `SkillBastionLast` (63) · ícone `kb_last`

> Finca os pés: daqui ninguém passa.

**Efeito:** 160→200% a até 6 m, atordoa por 2 s e +60% de defesa por 8 s (rank 1 → rank 12)

| Valor no doc | Rank 1 | Por rank | Rank 12 | No código |
|---|---|---|---|---|
| 160→200% | 160% | +3,64% | 200% | `Scale{1.6, 0.0364}` |

- **Custo / recarga:** 30 MP + **40 Det.** / 60 s
- **Requisito:** 40 pontos na barra de Bastião.
- **No doc da evolução:** marco 35, 3 (hoje 1) ranks, requisito "10 no ramo".

### Último Reduto

Chave · coluna **50** · **1** rank · `SkillBastionRedoubt` (89) · ícone novo (Meshy); hoje usa `b_undying`, emprestado

> O Bastião não cai enquanto houver determinação.

**Efeito:** Um golpe que mataria deixa você com 1 de vida e gasta toda a Determinação: escudo de 1% da vida máx. por ponto gasto, por 6 s (uma vez a cada 120 s)

- **Custo / recarga:** —
- **Requisito:** 50 pontos na barra de Bastião.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Juramento de Proteção

Chave · coluna **50** · **1** rank · `SkillBastionOath` (94) · ícone novo (Meshy); hoje usa `k_banner`, emprestado

> Jura levar os golpes no lugar dos companheiros.

**Efeito:** Por 8 s, 40% do dano que os aliados a até 10 m recebem vai para você

- **Custo / recarga:** **50 Det.** / 90 s
- **Requisito:** 50 pontos na barra de Bastião.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".

### Prisão do Guardião

Chave · coluna **50** · **1** rank · `SkillBastionPrison` (99) · ícone novo (Meshy); hoje usa `m_prison`, emprestado

> Ninguém sai, e todos olham para o Bastião.

**Efeito:** Prende todos a até 8 m por 3 s e os provoca por 6 s; eles causam −20% de dano em você

- **Custo / recarga:** **60 Det.** / 60 s
- **Requisito:** 50 pontos na barra de Bastião.
- **No doc da evolução:** marco 40, 1 rank, requisito "15 no ramo".


# Bastião (Cavaleiro): Determinação

> **Aprovado pelo DIogo em 19/11.** Os números são propostas ancoradas nas skills de hoje. A calibragem no simulador vem na fase de conteúdo do Cavaleiro, e o resultado volta para este doc. As regras gerais estão no [README](README.md).

## Papel e identidade

**Tanque principal.** Protege os aliados e controla os inimigos. O Bastião transforma defesa em valor para o grupo: segura os inimigos, tira a pressão de cima dos aliados e sobrevive a picos de dano.

Na caça solo ele mata mais devagar que a média, como hoje, mas morre muito menos. Ele é a escolha segura para quem joga em grupo ou enfrenta chefes.

## Armas

| Mão | Permitido |
|---|---|
| Principal | espada |
| Secundária | **escudo** (é a identidade: sem escudo, não há bloqueio e quase não há Determinação) |

**Bloqueio** (Fase 4, pré-requisito desta evolução):
- Um golpe físico contra quem usa escudo pode ser bloqueado. O bloqueado causa 50% do dano. **Hoje vale de qualquer direção**, porque o servidor não guarda para onde o jogador olha; "só de frente" fica para quando houver essa informação.
- A chance vem do escudo (`Bonus.Block`): comum 10%, raro 15%, épico 20%. As passivas abaixo somam a isso.
- Magia nunca é bloqueada.
- O cliente mostra "Bloqueio".

## Recurso: Determinação

O máximo é 100. Seguem as regras gerais do README: teto de 25 por segundo, até 3 alvos por lançamento, e decaimento de 10 por segundo depois de 5 s fora de combate.

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

**Ciclo sem recurso:** Golpe de Escudo, Guarda Inabalável, Desafio, Baluarte e Égide custam só MP. Com Determinação vazia, o Bastião ainda tanqueia, só não usa as habilidades de impacto.

## Árvore (marcos 20–40)

Os IDs de 85 em diante são **provisórios** e só são fechados no código. ★ marca as 2 iniciais, que vêm com rank 1 ao evoluir. "(evol.)" indica a versão evoluída de uma skill da base, com ID novo e o mesmo nome.

### Muralha: mitigação e sobrevivência

| Marco | Skill | Tipo | Ranks | Efeito (rank 1 → máx.) | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | ★ Guarda Inabalável (85, nova) | ativa | 3 | +40→60% de defesa e +20% de bloqueio por 8 s | 15 MP / 20 s | — |
| 20 | Pele de Aço (86, evol. da Pele de Ferro) | passiva | 5 | +4→20% de defesa física | — | — |
| 20 | Vigor do Bastião (62) | passiva | 5 (hoje 3) | +3→15% de vida máxima | — | — |
| 25 | Postura de Ferro (87) | modificador da Guarda | 3 | +2→6 s de duração; durante a Guarda, cada bloqueio gera +2→4 Determinação a mais | — | Guarda 1 |
| 30 | Muralha Viva (88, evol.) | passiva | 5 | Abaixo de 30% de vida: escudo de 12→20% da vida máx. por 10 s e +30 Determinação (uma vez a cada 60 s) | — | 5 no ramo |
| 35 | Último Bastião (63) | ativa | 3 (hoje 1) | 160→200% a até 6 m, atordoa por 2 s e +60% de defesa por 8 s | 30 MP + **40 Det.** / 60 s | 10 no ramo |
| 40 | **Último Reduto** (89) | chave | 1 | Um golpe que mataria deixa você com 1 de vida e gasta toda a Determinação: escudo de 1% da vida máx. por ponto gasto, por 6 s (uma vez a cada 120 s) | — | 15 no ramo |

### Protetor: proteção de aliados

| Marco | Skill | Tipo | Ranks | Efeito | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | Égide da Ordem (51) | ativa | 3 | Escudo de 250→500 por 8 s em você e nos aliados a até 10 m | 45 MP / 45 s | — |
| 20 | Vínculo de Guarda (90) | passiva | 5 | Aliados a até 10 m recebem −2→10% de dano (não soma entre Bastiões) | — | — |
| 25 | Égide Compartilhada (91) | modificador da Égide | 3 | +10→30% no escudo e −5→15 s de recarga | — | Égide 1 |
| 30 | Interceptar (92) | ativa | 3 | Salta até um aliado (até 12 m) e, por 4→6 s, recebe 30% do dano que ele levaria | 20 MP / 20 s | 5 no ramo |
| 35 | Guarda do Escudo (93) | passiva | 5 | +2→10% de chance de bloqueio; o bloqueio reduz 50→60% do dano | — | 10 no ramo |
| 40 | **Juramento de Proteção** (94) | chave | 1 | Por 8 s, 40% do dano que os aliados a até 10 m recebem vai para você | **50 Det.** / 90 s | 15 no ramo |

### Provocador: ameaça e controle

| Marco | Skill | Tipo | Ranks | Efeito | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | ★ Golpe de Escudo (95, evol.) | ativa | 3 | 150→190% e atordoa por 1,5→2 s | 10 MP / 8 s | — |
| 20 | Desafio do Bastião (61) | ativa | 3 | Provoca os inimigos a até 7 m por 4 s e dá a você um escudo de 300→600 por 10 s | 35 MP / 25 s | — |
| 20 | Baluarte (49) | ativa | 3 | 140→180% a até 5 m, atordoa por 1,5→2 s e provoca os atingidos | 35 MP / 24 s | — |
| 25 | Desafio Reforçado (96) | modificador do Desafio | 3 | +1→3 m de raio; os provocados causam −5→15% de dano em você | — | Desafio 1 |
| 30 | Zona de Provocação (97) | ativa | 3 | Marca o chão (6 m) por 8 s: provoca quem está dentro a cada 2 s e deixa 20→40% mais lento | 30 MP + **20 Det.** / 30 s | 5 no ramo |
| 35 | Eco do Escudo (98) | modificador do Golpe de Escudo | 3 | O golpe atinge também até 2 inimigos a até 3 m do alvo, com 50→100% do dano (sem atordoar) | — | 10 no ramo |
| 40 | **Prisão do Guardião** (99) | chave | 1 | Prende todos a até 8 m por 3 s e os provoca por 6 s; eles causam −20% de dano em você | **60 Det.** / 60 s | 15 no ramo |

### Conta da árvore

| Ramo | Ranks | Ranks no marco 20 |
|---|---|---|
| Muralha | 25 | 13 |
| Protetor | 20 | 8 |
| Provocador | 19 | 9 |
| **Total** | **64** (1,56× os 41 pontos do 40) | **30** (21 pontos no 20) |

- **Ativas:** 9 (Guarda, Último Bastião, Égide, Interceptar, Golpe de Escudo, Desafio, Baluarte, Zona e as chaves Juramento e Prisão).
- **Builds esperadas no 40:**
  - Muralha + Provocador: tanque solo, com 2 chaves;
  - Protetor + Provocador: tanque de grupo;
  - Muralha + Protetor: guarda-costas.

## Ao evoluir

- **Iniciais:** Guarda Inabalável e Golpe de Escudo, ambas com rank 1.
- **Hotbar:** Golpe de Escudo (1) da base vira Golpe de Escudo (95). Fortaleza (2), que some, vira Guarda Inabalável (85). Os outros atalhos da base ficam vazios. Os IDs 49, 51 e 61–63 ficam onde estavam.
- **Equipamento:** nada muda para quem já usa espada e escudo, e o escudo é obrigatório só na prática (pelo bloqueio). Sem escudo na bolsa, o Bastião recebe um **escudo comum** uma vez.
- **Bônus de atributo da evolução:** continua +12 Constituição, como hoje.

## Simulador: linha de base e metas

| | 20–24 | 25–29 | 30–34 | 35–40 |
|---|---|---|---|---|
| Tempo para matar hoje (Bastião) | 6,1 s (+86%) | 5,9 s (+100%) | 6,2 s (+98%) | 4,7 s (+89%) |

**Metas:**
1. **Tempo para matar:** no 20, igual ou menor que hoje (≤ 6,1 s). No 40, dentro de **+50% a +70%** da média. Hoje está em +89%, lento demais até para um tanque.
2. **Tempo para morrer** (medição da tunnel#152): o maior das 8 evoluções, pelo menos 1,5× o da média.
3. **Determinação:** numa luta de 60 s contra 3 monstros, a média fica entre 40 e 70. Ela não pode ficar o tempo todo cheia nem sempre vazia.

## O que o código precisa

- **Bloqueio** (Fase 4): `Stats.Block` e `BlockReduce`, `Bonus.Block` nos escudos e `CombatEvent.blocked`.
- **Redirecionar dano de aliado:** usado por Interceptar e Juramento de Proteção. É um efeito novo (`EffectGuard`, com origem e fração) e passa pelo `damage` sem gerar Determinação em dobro.
- **Zona no chão** que provoca periodicamente: reaproveita o Estandarte (objeto no chão com raio e duração) e troca o buff por provocação e lentidão.
- **Passiva "não morre"** com recarga própria: o padrão do `checkLastStand`, gastando o recurso.
- **Modificadores** (Postura de Ferro, Égide Compartilhada, Desafio Reforçado, Eco do Escudo): vêm do framework (`EffectiveSkill`).
- **Novos `PassiveKind`:** `PassiveBlockPct`, `PassiveAllyDamageReduce` (aura) e `PassiveBlockReduce`.

## Decisões do DIogo (19/11)

1. **Escudo de presente:** só para quem evolui sem escudo, nem equipado nem na bolsa. Recebe um escudo comum, uma vez.
2. **Vigor do Bastião:** passa a ter 5 ranks, com o mesmo máximo de 15%. A redistribuição grátis cobre quem perder valor.
3. **Interceptar:** entra no marco 30, como está acima.
4. **Meta de tempo para matar:** de +50% a +70% acima da média no 40.

## Calibragem no simulador (Fase 9, 19/11)

Com a árvore completa no código, o Bastião ficou em 11,4 s para matar no 20 (+139% da média), porque o dano dele vinha quase só do ataque básico. Para chegar à meta, os golpes saem mais vezes e batem mais:

| Skill | Doc | Calibrado |
|---|---|---|
| Golpe de Escudo (95) | 150→190%, recarga 8 s | **300→370%**, recarga **5 s** |
| Baluarte (49) | 140→180%, recarga 24 s | **240→300%**, recarga **12 s** |
| Último Bastião (63) | 160→200% | **240→300%** |

Resultado (tempo para matar, distância da média): 20–24 **6,6 s (+63%)**, 25–29 6,9 s (+76%), 30–34 7,8 s (+69%), 35–40 **5,5 s (+68%)**. A meta do 40 (+50% a +70%) foi cumprida; a do 20 (≤ 6,1 s) ficou perto, porque a média da faixa subiu com as evoluções ainda incompletas das outras classes.

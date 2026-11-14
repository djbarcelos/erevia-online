# Progressão: quanto custa chegar ao nível 30

O nível máximo (30) saiu rápido demais: o dono do jogo chegou lá em poucas horas. Este
documento junta os números **teóricos** de hoje (o que as fórmulas pedem), a proposta de
**tempo-alvo** e o que medir com os dados reais antes de mudar a curva (tunnel#149).
Medição: tunnel#147.

## A curva de hoje

Tudo em `server/internal/game/formulas.go`:

- **XP para sair do nível `l`:** `XPToNext(l) = 80 · l^2,2`; do nível 20 em diante, vezes
  `1 + 0,1 · (l − 19)` (o 20 pede ×1,1, o 29 pede ×2). Do 1 ao 30 são 1.839.820 de XP.
- **XP de um monstro:** `MonsterXP = XPBase · nível^1,6`. XP cheia até 5 níveis abaixo do
  jogador; depois, −15% por nível, até o mínimo de 10%.
- **Morrer** tira 4% da XP do nível (sem nunca descer de nível); com karma, o dobro.
- **Extras:** noite (+25%, um quarto do ciclo de 40 min), XP em dobro (sábado e domingo
  no fuso de Brasília, ou por GM com `/evento xp2`), grupo (a XP do monstro sobe 10% por
  membro além do primeiro e é dividida pelo nível de cada um) e a invasão da cidade.
- **Missões** (`game/quests.go`): só o tutorial dá XP, 2.410 no total, todo no começo.

Monstros por faixa (`game/monsters.go`, `DefaultSpawns`):

| Níveis | Onde | Monstros (XPBase) |
|---|---|---|
| 1–5 | Arredores | Esqueleto Lacaio (12) |
| 6–12 | Campos dos Ossos | Esqueleto Guerreiro (16), Esqueleto Ladino (16) |
| 13–20 | Ermos Partidos | Esqueleto Mago (18), Esqueleto Guerreiro (16) |
| 20–30 | Vale das Pedras Vivas | Golem de Pedra (40) |

Chefes (Senhor dos Ossos, 120) e o raid boss (Ozrhal, 260) renascem em minutos ou horas e
não entram na conta.

## Números teóricos

### Premissas

- O jogador caça **sozinho, de dia**, monstros **do próprio nível** e não morre: sem noite,
  sem grupo, sem XP em dobro e sem perda por morte.
- Em cada nível, caça no grupo de monstros mais avançado que cobre aquele nível (no 20, os
  golens, não os magos). Grupos que começam no mesmo nível entram na média pela
  quantidade de cada um no mapa.
- A XP das missões entra no começo (o tutorial cobre do 1 ao 4).
- **Cada abate custa 25 s de jogo:** ~10 s de luta e ~15 s para andar, achar o próximo e
  esperar o renascimento. É a premissa mais frágil; a medição real substitui isso pelo
  tempo de jogo de verdade (`v_level_playtime`).

O que fica de fora puxa o tempo real para **baixo**: a noite dá ~6% a mais em média e o fim
de semana dá o dobro. O grupo mata mais rápido, mas divide a XP (o total do monstro sobe
só 10% por membro além do primeiro). Mortes puxam para cima.

### Tabela

Gerada por `game.ProgressionMarkdown` (`server/internal/game/progression_table.go`). O
teste `TestProgressionDocUpToDate` falha se ela ficar diferente das fórmulas e dos
monstros do código; para regerar depois de mudar a curva ou um monstro:

```sh
cd server && EREVIA_UPDATE_DOCS=1 go test ./internal/game -run TestProgressionDoc
```

<!-- tabela-teorica:inicio -->
Premissa: 25 s por abate. Gerada por `game.ProgressionMarkdown`.

| Nível | XP para o próximo | Caça (do mesmo nível) | XP por abate | XP de missões | Abates | Minutos | Horas acumuladas |
|---:|---:|---|---:|---:|---:|---:|---:|
| 1 → 2 | 80 | Esqueleto Lacaio | 12 | 80 | 0 | 0 | 0,0 |
| 2 → 3 | 368 | Esqueleto Lacaio | 36 | 368 | 0 | 0 | 0,0 |
| 3 → 4 | 897 | Esqueleto Lacaio | 70 | 897 | 0 | 0 | 0,0 |
| 4 → 5 | 1.689 | Esqueleto Lacaio | 110 | 1.065 | 6 | 3 | 0,0 |
| 5 → 6 | 2.759 | Esqueleto Lacaio | 158 | 0 | 18 | 8 | 0,2 |
| 6 → 7 | 4.121 | Esqueleto Guerreiro, Esqueleto Ladino | 281 | 0 | 15 | 6 | 0,3 |
| 7 → 8 | 5.785 | Esqueleto Guerreiro, Esqueleto Ladino | 360 | 0 | 17 | 7 | 0,4 |
| 8 → 9 | 7.760 | Esqueleto Guerreiro, Esqueleto Ladino | 446 | 0 | 18 | 8 | 0,5 |
| 9 → 10 | 10.056 | Esqueleto Guerreiro, Esqueleto Ladino | 538 | 0 | 19 | 8 | 0,6 |
| 10 → 11 | 12.679 | Esqueleto Guerreiro, Esqueleto Ladino | 637 | 0 | 20 | 8 | 0,8 |
| 11 → 12 | 15.637 | Esqueleto Guerreiro, Esqueleto Ladino | 742 | 0 | 22 | 9 | 0,9 |
| 12 → 13 | 18.936 | Esqueleto Guerreiro, Esqueleto Ladino | 853 | 0 | 23 | 10 | 1,1 |
| 13 → 14 | 22.582 | Esqueleto Mago, Esqueleto Guerreiro | 1.030 | 0 | 22 | 9 | 1,2 |
| 14 → 15 | 26.581 | Esqueleto Mago, Esqueleto Guerreiro | 1.160 | 0 | 23 | 10 | 1,4 |
| 15 → 16 | 30.938 | Esqueleto Mago, Esqueleto Guerreiro | 1.295 | 0 | 24 | 10 | 1,6 |
| 16 → 17 | 35.658 | Esqueleto Mago, Esqueleto Guerreiro | 1.436 | 0 | 25 | 10 | 1,8 |
| 17 → 18 | 40.745 | Esqueleto Mago, Esqueleto Guerreiro | 1.582 | 0 | 26 | 11 | 1,9 |
| 18 → 19 | 46.205 | Esqueleto Mago, Esqueleto Guerreiro | 1.733 | 0 | 27 | 11 | 2,1 |
| 19 → 20 | 52.041 | Esqueleto Mago, Esqueleto Guerreiro | 1.890 | 0 | 28 | 12 | 2,3 |
| 20 → 21 | 64.084 | Golem de Pedra | 4.827 | 0 | 14 | 6 | 2,4 |
| 21 → 22 | 77.831 | Golem de Pedra | 5.219 | 0 | 15 | 6 | 2,5 |
| 22 → 23 | 93.404 | Golem de Pedra | 5.623 | 0 | 17 | 7 | 2,6 |
| 23 → 24 | 110.922 | Golem de Pedra | 6.037 | 0 | 19 | 8 | 2,8 |
| 24 → 25 | 130.511 | Golem de Pedra | 6.462 | 0 | 21 | 9 | 2,9 |
| 25 → 26 | 152.292 | Golem de Pedra | 6.899 | 0 | 23 | 10 | 3,1 |
| 26 → 27 | 176.393 | Golem de Pedra | 7.345 | 0 | 25 | 10 | 3,2 |
| 27 → 28 | 202.938 | Golem de Pedra | 7.803 | 0 | 27 | 11 | 3,4 |
| 28 → 29 | 232.055 | Golem de Pedra | 8.270 | 0 | 29 | 12 | 3,6 |
| 29 → 30 | 263.873 | Golem de Pedra | 8.748 | 0 | 31 | 13 | 3,8 |
| **1 → 30** | **1.839.820** | | | | **554** | **231** | **3,8 h** |

| Faixa | Abates | Horas hoje | Tempo-alvo (proposta) | Alvo ÷ hoje |
|---|---:|---:|---:|---:|
| 1 → 10 | 93 | 0,6 | 3,0 | 5× |
| 10 → 20 | 240 | 1,7 | 15,0 | 9× |
| 20 → 30 | 221 | 1,5 | 40,0 | 26× |
<!-- tabela-teorica:fim -->

Na prática, são ~550 abates do próprio nível (~590 sem o tutorial) e menos de 4 h de jogo
até o 30, o que bate com o que o dono viu. O problema não está só no fim: o 1→10 sai em
~40 min, e cada nível do 20 ao 30 pede só 14 a 31 golens. O golem tem XPBase 40 contra
16–18 dos esqueletos, então no 20 a XP por abate mais que dobra (de ~1.900 para ~4.800)
e engole o ×1,1 a ×2 que a curva acrescenta do 20 em diante.

## Tempo-alvo (proposta, a aprovar pelo Daniel)

| Faixa | Tempo-alvo | Abates a 25 s | Hoje (teórico) |
|---|---:|---:|---:|
| 1 → 10 | ~3 h | ~430 | ~0,6 h |
| 10 → 20 | ~15 h | ~2.160 | ~1,7 h |
| 20 → 30 | ~40 h | ~5.760 | ~1,5 h |
| **1 → 30** | **~60 h** | **~8.640** | **~3,8 h** |

- O começo continua rápido (o jogador aprende o jogo e escolhe a classe), o meio dá a
  segunda classe (nível 20) com ~18 h de jogo, e o fim é a parte longa, como
  no Lineage 2, em que a curva pesa mesmo nos últimos níveis.
- Referência: o WoW Classic leva ~150–240 h até o 60. Com um terço dos níveis e um mapa
  menor, ~60 h dá ao Erevia um fim de jogo que dura semanas sem virar trabalho.
- Ainda não muda nada: a curva nova (tunnel#149) espera os dados reais e esta aprovação.

## O que é medido

Tudo vai para `game_events` (migração 00011) e é lido por views da migração 00018:

| Evento | O que guarda |
|---|---|
| `session` | `start` ao entrar; `end` ao sair (`amount` = segundos), também quando o servidor desliga (`extra.shutdown`) |
| `level` | cada subida (`amount` = nível novo) |
| `kill` | cada abate (`ref` = monstro, `amount` = nível do monstro, `level` = nível do jogador) |
| `death` | cada morte (`ref` = quem matou) |
| `gold` | ouro que entra (+) e sai (−), com a origem em `ref` |
| `xp` (novo) | XP por fonte: `ref` = `monster`, `party`, `night_bonus`, `event_bonus`, `quest`, `invasion` ou `death_loss` (negativa); `level` = nível em que a XP veio |

O `xp` não grava uma linha por abate: a zona só soma num mapa (personagem → fonte, no
nível atual) e grava a cada ~60 s, ao subir de nível e ao sair (`zone/xptally.go`). A XP de
um abate é dividida assim: primeiro sai o extra da XP em dobro (`event_bonus`), depois o da
noite (`night_bonus`); o resto é `monster` (sozinho) ou `party` (em grupo). A soma das
fontes é a XP que o jogador ganhou. No nível 30 a XP não entra, então não é somada.

## O que olhar quando houver dados

| Pergunta | View | O que comparar |
|---|---|---|
| Quanto tempo de jogo cada nível leva? | `v_level_playtime` | `minutos_mediana` e `minutos_p75` (classe `todas` e por classe) com a coluna "Minutos" da tabela teórica; a soma por faixa com o tempo-alvo |
| A premissa de 25 s por abate vale? | `v_level_kills_deaths` + `v_level_playtime` | minutos de jogo ÷ `abates_por_personagem` do mesmo nível |
| De onde vem a XP? | `v_xp_sources` | quanto é `event_bonus` (o fim de semana encurta a curva?), `party` e `quest` |
| Morrer pesa? | `v_level_kills_deaths` | `abates_por_morte` e `xp_perdida_em_mortes` por nível |
| A economia acompanha? | `v_gold_per_hour` | `ganho_por_hora` e `gasto_por_hora` por faixa (se a curva ficar 10× mais longa, o ouro por nível também muda) |
| Onde as pessoas param? | `v_level_reached` | `pararam_nele` e `pct_do_anterior`: um nível que derruba muita gente é um muro |
| Classe mais lenta? | `v_level_playtime` | a mediana de cada classe no mesmo nível |

Onde ver:

- **Portal de administração:** `/admin#progressao` (só GM) mostra as cinco views.
- **Railway:** as consultas prontas para colar na aba Data estão em
  `server/internal/store/queries/progressao.sql` (inclui as horas por faixa somando as
  medianas, a XP por hora e a linha do tempo de um personagem).

Como as views contam o tempo:

- **Tempo de jogo** é a soma das sessões dentro do intervalo entre duas subidas, não o
  relógio (quem joga 1 h por dia não parece mais lento). `horas_relogio_mediana` mostra o
  relógio, para comparar.
- Uma sessão sem `end` (o servidor caiu sem desligar direito) vai até o último evento do
  personagem antes da sessão seguinte; como a XP sai a cada minuto, o erro é pequeno.
- Só entram no tempo por nível as subidas vistas do começo ao fim: um personagem que já
  existia antes da telemetria entra a partir da primeira subida que ela viu, e subir dois
  níveis de uma vez (missão grande) fica fora.
- Mediana e p75 contam mais do que a média: um personagem deixado aberto sem jogar puxa a
  média para cima.
- Contas apagadas (LGPD, migração 00020) ficam anônimas e saem das views por personagem;
  continuam em `v_xp_sources` e nos totais de ouro.

## Quantos dados bastam

Para decidir a curva nova, o mínimo útil é ~10 personagens com subidas completas em cada
faixa (o p75 de 3 pessoas não diz nada). O 1–10 enche em dias; o 20–30, com a curva de
hoje, também, mas a medição que importa para o tunnel#149 é a do 1–20, onde a premissa
de 25 s por abate é testada, e a das fontes de XP (quanto o fim de semana e o grupo
encurtam a conta teórica).

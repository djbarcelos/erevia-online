# Progressão: quanto custa chegar ao nível 40

O nível máximo era 30 e saía rápido demais: o dono do jogo chegou lá em poucas horas
(~3,8 h na conta teórica). A decisão do tunnel#149 subiu o máximo para **40** e pediu
**~150 h** até o 40 no primeiro personagem, com o peso no fim, como no Lineage 2. Este
documento junta os números **teóricos** (o que as fórmulas pedem) e o que medir com os
dados reais (tunnel#147) para afinar.

## A curva (versão 2, tunnel#149)

Tudo em `server/internal/game/formulas.go`:

- **XP para sair do nível `l`:** a tabela `xpTable` (39 números, arredondados em 3
  algarismos). Ela saiu dos tempos-alvo: o tempo de cada nível cresce devagar, de ~10 min
  no 4 a ~12,5 h no 39, sem degrau entre as faixas; a XP é esse tempo vezes a XP dos
  monstros que se caça no nível (menos os 30% que vêm das missões). Por isso a XP salta no
  20 (o golem vale 2,5× um esqueleto), mas o tempo não. Do 1 ao 40 são 332.474.828 de XP.
- **XP de um monstro:** `MonsterXP = XPBase · nível^1,6`. XP cheia até 5 níveis abaixo do
  jogador; depois, −15% por nível, até o mínimo de 10%.
- **Morrer** tira 4% da XP do nível até o 29 e **2% do 30 em diante** (um nível do 30 ao 40
  leva de 6 a 12 h, e 4% disso seria até meia hora por morte); nunca desce de nível. Com
  karma, o dobro. No 40, a morte tira os mesmos 2% da barra de Maestria.
- **Extras:** noite (+25%, um quarto do ciclo de 40 min), XP em dobro (sábado e domingo
  no fuso de Brasília, ou por GM com `/evento xp2`), grupo (a XP do monstro sobe 10% por
  membro além do primeiro e é dividida pelo nível de cada um) e a invasão da cidade. O XP
  em dobro continua igual: ele é proporcional, então com a curva 40× mais longa o fim de
  semana encurta o mesmo tanto (quem joga todo dia o mesmo tanto ganha ~30% a mais na
  semana), e é o que traz gente nos sábados.
- **Missões** (`game/quests.go`): o tutorial (2.410 de XP, no começo), a Encruzilhada (20)
  e os **contratos de caça**, um por nível do 5 ao 39, gerados dos dados: cada um pede ~25%
  dos abates do nível nos monstros daquele nível e paga 30% da XP dele. Assim ~30% da XP
  de cada faixa vem de missões. O contrato do nível L só chega até o nível L+2: quem já
  passou não recebe uma pilha de contratos velhos.
- **Maestria** (`game/mastery.go`): abre no 40, e 1 ponto custa `XPToNext(39)` =
  45.200.000 (o mesmo que o 39 → 40). Quem ganhou pontos no 30 (o máximo de antes) fica com
  eles e continua usando a árvore, mas não ganha pontos novos do 30 ao 39.
- **Skills:** 1 ponto de skill por nível até o 30 (31 pontos para 43–45 ranks da árvore da
  classe); do 31 ao 40 só os 3 pontos de atributo por nível.

Monstros por faixa (`game/monsters.go`, `DefaultSpawns`):

| Níveis | Onde | Monstros (XPBase) |
|---|---|---|
| 1–5 | Arredores | Esqueleto Lacaio (12) |
| 6–12 | Campos dos Ossos | Esqueleto Guerreiro (16), Esqueleto Ladino (16) |
| 13–20 | Ermos Partidos | Esqueleto Mago (18), Esqueleto Guerreiro (16) |
| 20–30 | Vale das Pedras Vivas | Golem de Pedra (40) |
| 30–35 | Fundo do Vale (borda sul do Vale) | Golem Ancestral (46), elite |
| 33–38 | Vigília Ossária (dois grupos ao lado da Cripta) | Cavaleiro Ossário (48), Devoto do Ossário (48), elites |
| 35–40 | Confins Partidos (os 3 cantos vazios do mapa) | Ceifador Partido (50), Arcanista Partido (50), elites |

As áreas do 30 ao 40 são provisórias, até as do `docs/roadmap-areas.md` (tunnel#146): usam
os modelos de hoje (golem e esqueletos KayKit) com escala e cor próprias, e cabem no
orçamento do roadmap (no máximo 45 monstros por área; os Confins mais o anel dos Ermos dão
41). Chefes (Senhor dos Ossos, 120) e o raid boss (Ozrhal, 260, agora no nível 35)
renascem em minutos ou horas e não entram na conta.

## Números teóricos

### Premissas

- O jogador caça **sozinho, de dia**, monstros **do próprio nível** e não morre: sem noite,
  sem grupo, sem XP em dobro e sem perda por morte.
- Em cada nível, caça no grupo de monstros mais avançado que cobre aquele nível (no 20, os
  golens, não os magos). Grupos que começam no mesmo nível entram na média pela
  quantidade de cada um no mapa.
- A XP das missões entra no nível em que a missão chega (o tutorial no 1, cada contrato no
  nível dele) e o que sobra passa para os níveis seguintes. Os abates que as missões pedem
  já estão na conta: os contratos são caçar os mesmos monstros.
- **Cada abate custa 25 s de jogo:** ~10 s de luta e ~15 s para andar, achar o próximo e
  esperar o renascimento. É a premissa mais frágil; a medição real substitui isso pelo
  tempo de jogo de verdade (`v_level_playtime`).

O que fica de fora puxa o tempo real para **baixo**: a noite dá ~6% a mais em média e o fim
de semana dá o dobro. O grupo mata mais rápido, mas divide a XP (o total do monstro sobe
só 10% por membro além do primeiro). Mortes puxam para cima, e as elites do 30 ao 40
também: elas são ~15% mais resistentes que um golem do mesmo nível e têm habilidades com
aviso no chão, então o abate real deve passar dos 25 s.

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
| 1 → 2 | 43 | Esqueleto Lacaio | 12 | 43 | 0 | 0 | 0,0 |
| 2 → 3 | 265 | Esqueleto Lacaio | 36 | 265 | 0 | 0 | 0,0 |
| 3 → 4 | 1.060 | Esqueleto Lacaio | 70 | 1.060 | 0 | 0 | 0,0 |
| 4 → 5 | 3.460 | Esqueleto Lacaio | 110 | 1.042 | 22 | 9 | 0,2 |
| 5 → 6 | 10.300 | Esqueleto Lacaio | 158 | 3.090 | 46 | 19 | 0,5 |
| 6 → 7 | 23.700 | Esqueleto Guerreiro, Esqueleto Ladino | 281 | 7.110 | 60 | 25 | 0,9 |
| 7 → 8 | 39.300 | Esqueleto Guerreiro, Esqueleto Ladino | 360 | 11.790 | 77 | 32 | 1,4 |
| 8 → 9 | 63.100 | Esqueleto Guerreiro, Esqueleto Ladino | 446 | 18.930 | 100 | 42 | 2,1 |
| 9 → 10 | 98.600 | Esqueleto Guerreiro, Esqueleto Ladino | 538 | 29.580 | 129 | 54 | 3,0 |
| 10 → 11 | 124.000 | Esqueleto Guerreiro, Esqueleto Ladino | 637 | 37.200 | 137 | 57 | 4,0 |
| 11 → 12 | 158.000 | Esqueleto Guerreiro, Esqueleto Ladino | 742 | 47.400 | 150 | 63 | 5,0 |
| 12 → 13 | 200.000 | Esqueleto Guerreiro, Esqueleto Ladino | 853 | 60.000 | 165 | 69 | 6,2 |
| 13 → 14 | 265.000 | Esqueleto Mago, Esqueleto Guerreiro | 1.030 | 79.500 | 181 | 75 | 7,4 |
| 14 → 15 | 328.000 | Esqueleto Mago, Esqueleto Guerreiro | 1.160 | 98.400 | 199 | 83 | 8,8 |
| 15 → 16 | 403.000 | Esqueleto Mago, Esqueleto Guerreiro | 1.295 | 120.900 | 218 | 91 | 10,3 |
| 16 → 17 | 491.000 | Esqueleto Mago, Esqueleto Guerreiro | 1.436 | 147.300 | 240 | 100 | 12,0 |
| 17 → 18 | 594.000 | Esqueleto Mago, Esqueleto Guerreiro | 1.582 | 178.200 | 263 | 110 | 13,8 |
| 18 → 19 | 715.000 | Esqueleto Mago, Esqueleto Guerreiro | 1.733 | 214.500 | 289 | 120 | 15,8 |
| 19 → 20 | 857.000 | Esqueleto Mago, Esqueleto Guerreiro | 1.890 | 257.100 | 318 | 133 | 18,0 |
| 20 → 21 | 2.620.000 | Golem de Pedra | 4.827 | 786.000 | 380 | 158 | 20,7 |
| 21 → 22 | 3.080.000 | Golem de Pedra | 5.219 | 924.000 | 414 | 173 | 23,5 |
| 22 → 23 | 3.620.000 | Golem de Pedra | 5.623 | 1.086.000 | 451 | 188 | 26,7 |
| 23 → 24 | 4.230.000 | Golem de Pedra | 6.037 | 1.269.000 | 491 | 205 | 30,1 |
| 24 → 25 | 4.940.000 | Golem de Pedra | 6.462 | 1.482.000 | 536 | 223 | 33,8 |
| 25 → 26 | 5.740.000 | Golem de Pedra | 6.899 | 1.722.000 | 583 | 243 | 37,8 |
| 26 → 27 | 6.660.000 | Golem de Pedra | 7.345 | 1.998.000 | 635 | 265 | 42,2 |
| 27 → 28 | 7.710.000 | Golem de Pedra | 7.803 | 2.313.000 | 692 | 288 | 47,1 |
| 28 → 29 | 8.900.000 | Golem de Pedra | 8.270 | 2.670.000 | 754 | 314 | 52,3 |
| 29 → 30 | 10.300.000 | Golem de Pedra | 8.748 | 3.090.000 | 825 | 344 | 58,0 |
| 30 → 31 | 13.400.000 | Golem Ancestral | 10.621 | 4.020.000 | 884 | 368 | 64,2 |
| 31 → 32 | 15.300.000 | Golem Ancestral | 11.193 | 4.590.000 | 957 | 399 | 70,8 |
| 32 → 33 | 17.500.000 | Golem Ancestral | 11.776 | 5.250.000 | 1.041 | 434 | 78,0 |
| 33 → 34 | 20.700.000 | Cavaleiro Ossário, Devoto do Ossário | 12.908 | 6.210.000 | 1.123 | 468 | 85,8 |
| 34 → 35 | 23.500.000 | Cavaleiro Ossário, Devoto do Ossário | 13.540 | 7.050.000 | 1.215 | 506 | 94,3 |
| 35 → 36 | 27.700.000 | Ceifador Partido, Arcanista Partido | 14.773 | 8.310.000 | 1.313 | 547 | 103,4 |
| 36 → 37 | 31.400.000 | Ceifador Partido, Arcanista Partido | 15.454 | 9.420.000 | 1.423 | 593 | 113,3 |
| 37 → 38 | 35.500.000 | Ceifador Partido, Arcanista Partido | 16.147 | 10.650.000 | 1.539 | 641 | 124,0 |
| 38 → 39 | 40.100.000 | Ceifador Partido, Arcanista Partido | 16.851 | 12.030.000 | 1.666 | 694 | 135,5 |
| 39 → 40 | 45.200.000 | Ceifador Partido, Arcanista Partido | 17.566 | 13.560.000 | 1.802 | 751 | 148,0 |
| **1 → 40** | **332.474.828** | | | | **21.318** | **8.883** | **148,0 h** |

| Faixa | Abates | Horas | Tempo-alvo | Diferença | XP de missões |
|---|---:|---:|---:|---:|---:|
| 1 → 10 | 434 | 3,0 | 3,0 | +0,5% | 30% |
| 10 → 20 | 2.160 | 15,0 | 15,0 | +0,0% | 30% |
| 20 → 30 | 5.761 | 40,0 | 40,0 | +0,0% | 30% |
| 30 → 40 | 12.963 | 90,0 | 90,0 | +0,0% | 30% |
<!-- tabela-teorica:fim -->

A curva 1 (a de antes) dava ~550 abates e menos de 4 h até o 30: o 1→10 saía em ~40 min, e
cada nível do 20 ao 30 pedia só 14 a 31 golens. Na curva 2, cada faixa bate o tempo-alvo
(a coluna "Diferença"; o teste `TestProgressionBandsHitTargets` aceita até 15%), e o total
fica em ~148 h.

## Tempo-alvo (decisão do tunnel#149)

| Faixa | Tempo-alvo | Abates a 25 s | Curva 1 (até o 30) | Curva 2 |
|---|---:|---:|---:|---:|
| 1 → 10 | ~3 h | ~430 | ~0,6 h | 3,0 h |
| 10 → 20 | ~15 h | ~2.160 | ~1,7 h | 15,0 h |
| 20 → 30 | ~40 h | ~5.760 | ~1,5 h | 40,0 h |
| 30 → 40 | ~90 h | ~12.960 | — | 90,0 h |
| **1 → 40** | **~150 h** | **~21.300** | **~3,8 h (até o 30)** | **148,0 h** |

- O começo continua rápido (o jogador aprende o jogo e escolhe a classe), o meio dá a
  segunda classe (nível 20) com ~18 h de jogo, e o fim é a parte longa, como no Lineage 2.
- Referência: o WoW Classic leva ~150–240 h até o 60.

## Conversão da XP salva

O nível sai da XP total salva no personagem (`characters.xp`, `game.LevelForXP`), então a
curva nova mudaria o nível de quem já joga. A migração 00019 põe a versão da curva em cada
personagem (`characters.xp_curve`: os de antes ficam na 1, os novos nascem na 2). Ao
carregar um personagem na curva 1, `store.CharacterForAccount` chama `game.ConvertXP` e
salva na 2, na mesma transação:

- a XP vira a **mesma fração do mesmo nível** na curva nova (meio do 19 continua no meio
  do 19);
- quem estava no **30** (o máximo de antes, com a barra cheia ou travada) continua no 30 e
  começa o 30 → 31 do zero; a Maestria dele (pontos, nós e barra) fica como estava.

**Guarda do deploy.** A Railway sobe o servidor novo antes de derrubar o antigo, e o antigo,
ao desligar, grava todo mundo com a XP da curva 1 (o save grava `level` e `xp`, não
`xp_curve`). Se o jogador já tinha reconectado no novo, a linha fica marcada na 2 com XP
da 1. Por isso a carga confere toda linha da curva 2 contra o nível salvo
(`game.CheckSavedXP`): se a XP cabe no nível pela curva 2, fica; se cabe pela curva 1,
converte de novo; se não cabe em nenhuma, fica no início do nível salvo e vira um evento
`invariant` (ref `xp_curve`) em `game_events`. O nível salvo nunca desce.

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
fontes é a XP que o jogador ganhou. No nível máximo a XP não entra, então não é somada.

### Subclasse (tunnel#156)

A subclasse começa no 20 e sobe pela mesma `xpTable`: do 20 ao 30 são mais ~40 h de jogo,
um pouco menos com o bônus Duas Sendas (+5% de XP para a classe mais baixa, depois que
uma subclasse chega ao 30). Para não misturar essa subida com a da classe principal:

- a subida de nível de uma subclasse vai como `level` com `ref = 'subclass'` (e
  `extra.class`); a da principal continua com `ref` vazio. As views da migração 00018 ainda
  não separam as duas: quem quiser só a principal filtra `ref <> 'subclass'`;
- o extra do Duas Sendas é a fonte `subclass_bonus` do `xp`;
- `class_switch` (ref = a classe nova, amount = o nível dela, `extra.from`, `from_level` e
  `unequipped`) anota cada troca, e `subclass` (ref = a classe) cada subclasse obtida.

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

Para afinar a curva 2, o mínimo útil é ~10 personagens com subidas completas em cada
faixa (o p75 de 3 pessoas não diz nada). Com ~150 h até o 40, o 1–10 enche em dias e o
10–20 em semanas; o que importa primeiro é testar a premissa de 25 s por abate (no 1–20 e
nas elites do 30 ao 40) e as fontes de XP (quanto o fim de semana, o grupo e os contratos
encurtam a conta teórica). Se a premissa errar, basta escalar a `xpTable` da faixa.

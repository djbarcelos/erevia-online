# Maestrias: regras gerais

Proposta de trocar a árvore das evoluções (3 ramos, marcos 20–40) pelo sistema de maestrias do Grim Dawn: duas maestrias por personagem, uma barra que libera as colunas e uma classe com nome próprio para cada dupla. O conteúdo é o das 8 evoluções do Erevia ([docs/evolucoes](../evolucoes/README.md)). Do Grim Dawn vem só a mecânica.

- **Protótipo jogável:** [Maestrias de Erevia](https://claude.ai/artifact/4P5N9Bg2LCeP7n4bjdKkAZ)
- **Uma página por maestria**, com a árvore, o recurso e cada skill descrita (efeito, valores por rank em `Scale`, custo, requisito, constante e ícone no código):

| Classe | Maestria | Recurso | Doc |
|---|---|---|---|
| Cavaleiro | Bastião | Determinação | [bastion.md](bastion.md) |
| Cavaleiro | Paladino | Fé | [herald.md](herald.md) |
| Bárbaro | Berserker | Fúria | [bloodrager.md](bloodrager.md) |
| Bárbaro | Senhor da Guerra | Ímpeto | [warlord.md](warlord.md) |
| Mago | Elementalista | Concentração | [elementalist.md](elementalist.md) |
| Mago | Arcanista | Carga Arcana | [occultist.md](occultist.md) |
| Ladino | Assassino | Combo | [cutthroat.md](cutthroat.md) |
| Ladino | Patrulheiro | Precisão | [hunter.md](hunter.md) |

**Situação:** proposta, não aprovada. As 8 evoluções estão no jogo com a árvore de 3 ramos (Fases 9–12). Os números são os dos docs das evoluções, espalhados pelos ranks novos, e precisam passar pelo simulador (`server/cmd/balance`).

---

## 1. Regras

| Regra | Valor | Hoje (evoluções) |
|---|---|---|
| 1ª maestria | nível 20, uma das 2 evoluções da classe-base (com a Ysolde) | igual |
| 2ª maestria | nível 30, qualquer uma das outras 7 | não existe |
| Pontos | **3 por nível** (60 no 20, 120 no 40, 180 no 60), para barras e skills | 1 por nível (`SkillPointsTotal`) |
| Barra da maestria | até **50** pontos por maestria | não existe |
| Colunas | a barra libera as colunas em **1 / 5 / 10 / 15 / 20 / 25 / 32 / 40 / 50** | marcos 20/25/30/35/40 pelo nível, e "N no ramo" |
| Atributos da barra | cada ponto dá 1/12 do bônus da evolução (Arcanista: +0,5 Int e +0,5 Esp) e +8 de vida | bônus fixo de +12 ao evoluir |
| Ranks | ativa **12**, passiva **10**, modificador **8**, chave **1** | 3 / 5 / 3 / 1 |
| Modificador | exige 1 ponto na skill que modifica | igual |
| Ramos | não existem; cada maestria é uma árvore só | 3 ramos |
| Armas | as da classe-base (1ª maestria) | as da evolução |
| Recurso | cada maestria traz o seu (2 barras na tela) | 1 |

### Valores por rank

Os docs das evoluções dão cada número como "rank 1 → rank máximo" (ex.: `160→210%` em 3 ranks). No sistema de maestrias o mesmo intervalo se espalha pelos ranks novos:

`valor(r) = rank1 + (máx − rank1) × (r − 1) / (ranks − 1)`

Isso é exatamente o `Scale{Base, Per}` do código (`Scale.At(r) = Base + Per × (r − 1)`). A coluna **No código** de cada tabela já traz o `Scale` em fração, pronto para colar (`160→210%` em 12 ranks = `Scale{1.6, 0.0455}`).

### Da árvore atual para as colunas

| Marco no doc | Coluna (pontos na barra) |
|---|---|
| 20 | 1, 5 ou 10: as skills do ramo se espalham, as iniciais e as ativas primeiro |
| 25 | 15 |
| 30 | 20 (ativas) ou 25 (passivas e modificadores) |
| 35 | 32 (passivas e modificadores) ou 40 (ativas) |
| 40 (chave) | 50 |

Um modificador fica sempre à direita da skill que modifica, ligado por linha.

---

## 2. As 28 classes

Linha = 1ª maestria, coluna = 2ª. O nome não depende da ordem. **Os nomes são rascunho.**

| | Paladino | Berserker | Senhor da Guerra | Elementalista | Arcanista | Assassino | Patrulheiro |
|---|---|---|---|---|---|---|---|
| **Bastião** | Cruzado | Colosso | Marechal | Guardião Rúnico | Guardião do Véu | Vigia Sombrio | Guarda-Fronteira |
| **Paladino** | | Zelote | Campeão | Teurgo | Exorcista | Penitente | Caçador de Bruxas |
| **Berserker** | | | Chefe de Clã | Incendiário | Possesso | Carniceiro | Predador |
| **Senhor da Guerra** | | | | Senhor da Tormenta | Tirano | Mercenário | Desbravador |
| **Elementalista** | | | | | Arquimago | Lâmina Elemental | Arqueiro Rúnico |
| **Arcanista** | | | | | | Bruxo | Pestilento |
| **Assassino** | | | | | | | Espreitador |

---

## 3. O que muda no código

| Onde | Mudança |
|---|---|
| `game/skills.go` `SkillPointsTotal` | 3 pontos por nível; a barra gasta do mesmo total. |
| `game/evolutions.go` `NodeKind` | MaxRank por tipo: ativa 12, passiva 10, modificador 8, chave 1. |
| `game/evo_*.go` `node(spec, branch, level, …)` | `level` vira a coluna da barra (`Gate`). O ramo sai da regra de aprender; a regra "pontos no ramo" (Fase 7) cai. |
| `game/evo_*.go` (cada `Scale`) | Recalcular o `Per` para os ranks novos: `Per_novo = Per_antigo × (ranks_antigos − 1) / (ranks_novos − 1)`. As tabelas dos docs já dão o resultado. O mesmo vale para os `SkillMod` dos modificadores. |
| Personagem (banco, proto `spec`) | Guardar a 2ª maestria (`spec2`) e a barra de cada maestria. Precisa de migração. |
| Atributos | Somar o bônus da barra (1/12 do `SpecDef.Bonus` por ponto) e +8 de vida por ponto, no lugar do bônus fixo. |
| `zone/specs.go` e NPC Ysolde | Escolha da 2ª maestria no nível 30. |
| Recursos | Duas barras de recurso no HUD; geração e gasto de cada skill seguem a maestria dela. |
| Cliente (janela de skills, tecla K) | Árvore no layout do protótipo: barra no topo, 9 colunas, cadeias ligadas, tooltip com o rank atual e o próximo, abas das 2 maestrias. |
| Ícones | As 108 skills novas ainda usam ícones emprestados de outras classes. Os ícones próprios já foram gerados (seção 4); falta dar a cada skill um `FX` próprio e cortar em `client/public/icons/skills/<fx>.webp`. |
| Migração de saves | Proposta: rank novo = rank antigo × ranks novos ÷ ranks antigos, arredondado para baixo; barra começa em 0, com os pontos devolvidos e redistribuição grátis. |
| Simulador | Rodar as 28 duplas nos níveis 30, 40 e 60. |

---

## 4. Ícones

- **170 ícones:** 62 já existiam em `client/public/icons/skills` (mesmo nome de skill) e 108 foram gerados no Meshy para esta proposta.
- **Geração:** 3 folhas 6×6 (`text-to-image`, `nano-banana-pro`, 9 créditos cada, 27 no total), com as descrições em [tools/art/icones-evolucao/prompts.mjs](../../tools/art/icones-evolucao/prompts.mjs).
- **Folhas originais:** `assets-src/meshy/text-to-image-{f0ff4b38,9403d23b,981a00fc}/`.
- **Corte:** a ordem de leitura de cada folha é a ordem de `ICONS`, 36 por folha.

---

## 5. Em aberto

1. **Aprovar o sistema.** Ele substitui a árvore de 3 ramos que acabou de entrar no jogo.
2. **Mais cadeias.** No Grim Dawn quase toda ativa tem 2 ou 3 modificadores na mesma linha; aqui cada maestria tem só 3. Para a árvore ficar igual, cada maestria precisa de umas 6–7 ativas com 2–3 modificadores cada (uns 30 nós).
3. **Transmutadores:** versões alternativas de uma skill, das quais só uma fica ativa. Os docs ainda não têm.
4. **Armas de outra classe:** skills que pedem escudo, arco ou machado 2H quando a maestria é a 2ª.
5. **2ª maestria da mesma classe** (Cruzado, Chefe de Clã, Arquimago, Espreitador): permitir ou não.
6. **Nível da 2ª maestria:** 30 (proposta) ou antes.
7. **Nomes das 28 classes.**

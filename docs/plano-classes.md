# Plano — Remodelação de Classes, Evoluções, Recursos, Armas e Nível 60

## Estado em 19/11 (tunnel#180, com o DIogo)

Este plano foi escrito antes de algumas mudanças na main. As premissas abaixo foram corrigidas, e **valem estas sobre o texto original**:

| Item | Correção |
|---|---|
| Fases 1 e 2 | **Feitas** (ab8982a e PR #79). As pontas que faltavam estão no PR #94 (`classes-base`). |
| IDs do degrau 20 | São **49–60**, e não 51–62. As skills de spec são **61–84**. IDs novos entram a partir do **85**. |
| Árvore base | Hoje tem **5 degraus** (1/5/10/15/20). Com as evoluções, o degrau 20 sai e ficam 4. |
| Pontos de skill | Hoje param no 30 (`SkillPointMaxLevel`, tunnel#153). Passam a ser **1 por nível até o teto** (decisão 33 da tunnel#180). |
| Migração | A 00030 está ocupada. Esta frente usa a **00062**. |
| Subclasse (tunnel#156) | Cada classe do personagem guarda ranks, hotbar e spec em `ClassProgress` (JSONB `classes`). A migração da árvore roda **por classe**, dentro do `loadProgress`, e a marca e a redistribuição grátis também são por classe. |
| Escudo de monstro | O `shieldBlock` (escudo frontal dos monstros) já existe. O bloqueio do jogador (Fase 4) é separado. |
| Branch de integração | **`classes-evolucao`**. A `evolucoes` era a branch local do PC 2, que foi abandonada. |
| Docs das evoluções | Em [evolucoes/](evolucoes/README.md), um par por vez, com revisão do DIogo antes do código. |

## Contexto

O documento de design (`erevia-classes-evolucao-progressao.md`) define:

- 4 classes-base, níveis 1–19, com 3 ramos livres.
- 8 evoluções no nível 20, cada uma com árvore própria, recurso de classe e armas permitidas.
- Armas com identidade própria: `weaponType` controla alcance, intervalo, tipo de dano, animação, projétil e empunhadura.
- Rebalanceamento de mana.
- Lançamento com teto 60, arquitetura preparada para 100.

O que o código tem hoje, e o que ele impõe ao plano:

- **O teto real é 40, não 30.** `MaxLevel = 40` em `server/internal/game/classes.go:9`.
- **A Maestria abre no 40.** Em `game/mastery.go`, `MasteryLevel = MaxLevel`.
- **A escolha no 20 já existe.** O NPC é Ysolde, a missão é `encruzilhada` e há 8 specs com IDs `bastion, herald, bloodrager, warlord, elementalist, occultist, cutthroat, hunter`. Hoje a escolha é só "árvore base + um ramo de spec com 3 habilidades", e o plano reaproveita isso.
- **Um único recurso, MP.** O "Fúria" de hoje (`SkillFury`) é apenas uma habilidade.
- **O tipo de arma vem da classe** (`weaponTypeOf`, `game/items.go:189`). Alcance, intervalo e mágico ficam em `ClassDef.Base`. A animação e o projétil são escolhidos pelo modelo da classe (`ANIM_SETS`, `isCaster`).
- **Os IDs de habilidade ficam salvos** em `skill_ranks`, na hotbar e nos cooldowns. Só podem ser acrescentados no fim do `iota`. Não existe redistribuição de pontos da árvore.

Decisões do usuário:

1. A Maestria continua com `MasteryLevel = MaxLevel` e abre no 60 quando o teto subir. Os pontos já ganhos continuam usáveis via `Usable()`.
2. Claude rascunha `docs/evolucoes/<id>.md` com números calibrados no simulador (`server/cmd/balance`), e o usuário revisa antes do código de cada evolução.
3. O teto 40 → 60 é a **última fase**. Evoluções, recursos e armas são testáveis no teto 40, com a árvore indo até o marco 40.
4. **Branch de integração `evolucoes`.** Tudo entra junto quando as 4 classes estiverem prontas. As fases sem efeito para o jogador (refactor e base de armas) podem ir direto para main, para reduzir rebase.

## Decisões transversais

- **D1. `spec` continua sendo o campo da evolução.** A coluna `characters.spec`, `Entity.spec`, `SpecCmd`/`SpecChoice`/`SpecInfo`, `GoalSpec` e os IDs ficam como estão. Mudam só os nomes exibidos:
  - herald → Paladino
  - bloodrager → Berserker
  - warlord → Senhor da Guerra
  - occultist → Arcanista
  - cutthroat → Assassino
  - hunter → Patrulheiro

  Em Go entra `type EvolutionDef = SpecDef`. Nenhum rename em SQL, para não quebrar o deploy com servidores sobrepostos.
- **D2. Pontos derivados, nunca armazenados.**
  - Sem evolução: `SkillPointsTotal(level, evolved)` = `min(level,19)+1`.
  - Com evolução: `level+1`.
  - A "devolução" ao evoluir é só apagar os ranks da base: o pool inteiro passa para a árvore da evolução.
  - A árvore evoluída deve ter cerca de 1,6× mais ranks do que pontos disponíveis no teto, para que não dê para maximizar tudo.
- **D3. Pré-requisitos.**
  - A árvore base mantém `Parent()` (cadeia linear, degraus 1/5/10/15).
  - Os nós da evolução usam `Level` (marco 20/25/…/60), `Requires []SkillReq{ID,Rank}` e `BranchPoints` (pontos já gastos no ramo).
- **D4. Regras puras ficam em `game/`, e o zone só as chama** (padrão `CharacterStats`). Assim o simulador de balanceamento usa exatamente os mesmos números.
- **D5. A branch de integração dispensa caminho legado.** A fase de framework converte as 4 classes de uma vez para o modelo novo. As árvores iniciais de cada evolução são formadas pelas habilidades que já existem:
  - as 3 habilidades da spec atual;
  - a habilidade do degrau 20 que sai da base.

  As fases de conteúdo expandem essas árvores. Não há flag `Live` nem PR de limpeza de legado.

---

## Fase 0 — Documentação de design (só docs, vai para main)

- `docs/evolucoes/README.md` define as regras globais:
  - tipos de nó e limites de rank: ativa ≤3, passiva ≤5, modificador ≤3, chave e suprema 1;
  - marcos 20–40, com 45–60 marcados "chega com o cap 60";
  - orçamento de pontos;
  - `RespecCost(level)` e `EvolutionChangeCost(level)`, que hoje é um valor fixo de 50k e passa a escalar com o nível;
  - regras dos recursos: teto por segundo, teto de alvos, nada gerado por DoT, reflexo, invulnerável ou invocação, e máximo rígido;
  - matriz de armas;
  - mapa de habilidades: as 12 do degrau 20 e as 24 de spec, indicando para qual evolução vai cada ID.
- Para cada evolução, `docs/evolucoes/<id>.md` traz nós, ranks, custos, geração e gasto do recurso e o resultado do simulador. O usuário revisa cada uma antes da fase de conteúdo correspondente.
- Atualizar e commitar `docs/classes.md`, `docs/armas.md` e `docs/maestria.md`, que hoje dizem cap 30. Eles viram a linha de base.
- Decisões pendentes do documento com o default proposto (o usuário pode trocar na revisão):

| Questão | Default proposto |
|---|---|
| Troca de evolução | ouro escalado por nível |
| Redistribuição da árvore | ouro, mais 1 gratuita após a migração |
| Pontos por nível | 1 por nível até o teto |
| Paladino | cura limitada (não é curandeiro) |
| Senhor da Guerra com escudo | mantém as habilidades centrais |
| Grimório | foco secundário, só atributos (sem regras próprias de combate) |
| Bloqueio vs magia | escudo não bloqueia magia |
| Chefes | usam a resistência a controle que já existe; execução tem teto próprio |

## Fase 1 — Refactor dos IDs de habilidade fixos no código (comportamento idêntico, vai para main)

- Novos `PassiveKind` em `game/skills.go`: `PassiveLifesteal`, `PassivePoisonProc`, `PassiveLastStand`, `PassiveLowHPRegen`, `PassiveBurnOnFire`, `PassiveMPRegenPct`, `PassiveAllyHealOnBuff`. `PassiveOverload` passa a cobrir também o custo de MP.
- `game.PassiveTotals(ranks)` é calculado em `zone.refreshStats` e guardado em `p.pl.passives`.
- Trocar as leituras de `game.Skill<X>` nos pontos onde estão hoje:
  - `zone/combat.go`: Overload em 119, Ember em 333, Bloodthirst e PoisonedBlade em `applyHit`, LivingWall, BearSpirit em `regen`;
  - `zone/effects.go`: Courage em 149, Plague em 221 (a duração do contágio vira campo do `EffectSpec`);
  - `game/character.go:27`: Mente Clara.
- Criar `game.SkillAvailable(s, class, spec)`. Ele substitui o `s.Spec == "" || s.Spec == p.pl.spec` repetido em `requestSkill`, `validHotbarEntry` e `loadProgress`.
- **Verificação:**
  - `make test` passa sem alteração nos testes.
  - O relatório de `go run ./cmd/balance` sai byte a byte igual.
  - Novo teste `TestPassiveKindsFromAnySkill`.

## Fase 2 — Base de armas no servidor (comportamento idêntico, vai para main)

- Novo arquivo `game/weapons.go`:
  - `WeaponDef{Type, Name, Range, Interval, Magic, Hands, Anim, Projectile}`;
  - mapa `Weapons` com `sword`, `axe`, `axe2h`, `staff`, `wand`, `daggers`, `bow`, `crossbow`, `crossbow2h`;
  - tipos de mão secundária: `shield` e `grimoire`.
- Os IDs atuais continuam valendo, e o cliente já usa `ItemInfo.weapon_type`.
- `ItemDef.WeaponType` passa a ser explícito:
  - o helper `weapon()` (`items.go:184`) recebe o tipo, e `weaponTypeOf` é removido;
  - as varinhas viram `wand`, inicialmente com os números do cajado;
  - **`sword_rare` e `sword_epic` continuam 1H**, mesmo usando o modelo `sword_2handed`. A empunhadura é explícita no item, nunca inferida do modelo.
- Armas permitidas:
  - `ClassDef.Weapons`/`Offhands` e `EvolutionDef.Weapons`/`Offhands`;
  - novas funções `game.CanWield` e `game.CanOffhand`;
  - removem `shieldClasses` (`items.go:247`).
- Pipeline de atributos: `ApplyWeapon(stats, wt)` roda em `CharacterStats` **antes** de `ApplyAttrs`, porque a Dex escala o intervalo. Ele sobrescreve `Range`, `AtkInterval` e `Magic`, e `ClassDef.Base` fica como fallback para personagem desarmado.
- `zone/inventory.go` `canEquip`:
  - passa a checar também o tipo de arma;
  - ao equipar uma arma 2H ou dupla, o item da mão secundária vai para a bolsa;
  - com arma 2H equipada, a mão secundária é recusada;
  - novo `dropInvalidGear(p)`, chamado no load e ao evoluir ou trocar de evolução.
- Proto:
  - nova mensagem `WeaponInfo`, enviada em `Welcome.weapons`;
  - novos campos `ItemInfo.hands`, `Entity.weapon_type`, `Appearance.weapon_type` e `SpecInfo.weapons`/`offhands`.
- `balance/build.go:241` passa a ler `ItemDef.WeaponType`.
- **Testes:**
  - teste golden de `Range`, `AtkInterval`, `Magic` e `PAtk` por classe nos níveis 1, 20 e 40, que deve sair igual ao de hoje;
  - casos 2H + mão secundária e `dropInvalidGear`;
  - `items_test`;
  - regenerar o fuzz corpus e atualizar o loadbot.

## Fase 3 — Armas no cliente (visual, vai para main)

- `client/scripts/build-models.mjs`:
  - incluir os clips `Ranged_Bow_*`, `Ranged_1H_*`, `Ranged_2H_*` e `Melee_2H_*` na lista ANIMS;
  - incluir bow, crossbow 1H/2H, flecha e grimório na lista WEAPONS;
  - rodar o script com `../assets-src/kaykit/x`.
- `client/src/world/models.ts`:
  - `WEAPON_ANIMS[anim]` passa a ter prioridade sobre `ANIM_SETS[model]` para jogadores;
  - adicionar entradas em `CLIP_IMPACT` e `ITEM_WEAPONS`;
  - o arco vai na mão esquerda.
- `client/src/world/entities.ts`: `isCaster(model)` vira `projectileOf(entity)`, lido do catálogo `WeaponInfo`. Monstros continuam usando o modelo. Flecha e virote reaproveitam o caminho do `arcaneBolt`.
- `client/src/ui/items.ts`: o tooltip mostra empunhadura, alcance e intervalo, e `WEAPON_LABEL` ganha os novos tipos.
- **Verificação:**
  - QA no navegador: as 4 classes estão visualmente idênticas;
  - um arco criado via comando de GM anima e dispara flecha.

## Fase 4 — Bloqueio de escudo (branch `evolucoes` a partir daqui)

- Novos atributos `Stats.Block` e `Stats.BlockReduce`. Os escudos ganham `Bonus.Block`.
- `game.RollHit`: um golpe físico contra alvo com escudo rola bloqueio. Se bloquear, o dano cai para ×(1−0,5).
- Proto: `CombatEvent.blocked`. O cliente mostra o texto "Bloqueio".
- O simulador passa a modelar o bloqueio.
- Esta fase é pré-requisito da Determinação.

## Fase 5 — Framework de evolução (as 4 classes convertidas de uma vez)

- **`SkillDef`** (`game/skills.go`) ganha:
  - `Level`, `Node` (Active/Passive/Modifier/Key/Supreme), `Requires []SkillReq` e `BranchPoints`;
  - `Weapon []string`, para habilidades que exigem um tipo de arma;
  - `ResCost`, `ResGain`, `ResSpendAll` e `ResPerPoint`;
  - `Modifies uint32` e `Mod *SkillMod`, este com somas por rank de Mult, CD, MP, ResCost, Splash, ExtraCrit, Hits, NoMiss e efeitos extras.
- **Modificadores:** `game.EffectiveSkill(base, ranks)` devolve uma cópia da habilidade com os modificadores aprendidos aplicados.
  - O zone guarda o resultado em `p.pl.skills` e o usa em `castSkill`, `skillHit`, `skillMP` e `skillCD`.
  - O simulador usa a mesma função.
- **`EvolutionDef`** (`game/specs.go`) ganha:
  - `Branches [3]Branch`, `Resource`, `Weapons`/`Offhands`;
  - `StarterSkills` (duas ativas raiz, que entram com rank 1 ao evoluir);
  - `StarterItems` (por exemplo, arco para o Patrulheiro e grimório para os magos).

  O bônus de +12 atributos continua.
- **Árvore base:** `TierLevels` fica com 4 degraus (1/5/10/15). As 12 habilidades do degrau 20 (IDs 51–62) passam para as evoluções com o **mesmo ID**, assim como as 24 de spec. Isso preserva ranks e hotbars na migração.
- **Regras** (`ValidateRanks`, `StarterRanks(class, spec)`, `DefaultHotbar`):
  - sem evolução: só a árvore base, com pool `min(level,19)+1`;
  - evoluído: só a árvore da evolução, checando `Requires`, `BranchPoints` e `Level`;
  - a exigência de arma é checada ao lançar a habilidade, não ao aprender.
- **`zone/specs.go` `chooseSpec`**, estendido (ele já cobra, apaga ranks e cooldowns e limpa a hotbar):
  - na primeira escolha, apaga a árvore base;
  - na troca, apaga a árvore da evolução anterior e cobra `EvolutionChangeCost`;
  - concede os starters e reconstrói a hotbar;
  - chama `dropInvalidGear`;
  - entrega os `StarterItems` uma única vez por evolução, controlado por uma flag em `quests.done`.
- Novo `SPEC_CMD_RESPEC` com Ysolde: volta a árvore aos starters por `RespecCost(level)`.
- **Migração no load** (`zone/progress.go` `loadProgress`), idempotente:

| Personagem | O que acontece no load |
|---|---|
| Nível ≥20 com spec | Mantém a spec e só os ranks que existem na árvore da evolução. Descarta a base, adiciona os starters, filtra e completa a hotbar, chama `dropInvalidGear` e manda uma mensagem única ("N pontos para distribuir"). Ganha 1 redistribuição gratuita. |
| Nível ≥20 sem spec | Perde o nó do degrau 20. Se tiver ranks demais, volta aos starters. A missão aparece no log, e o nível não é bloqueado. |
| Nível <20 | Nada muda. |

- Migração **`00030_skill_tree.sql`**: `skill_tree INT NOT NULL DEFAULT 1`, que passa a 2 depois de migrado (padrão `xp_curve`). Serve para a mensagem única e a redistribuição gratuita.
- Proto:
  - `SkillInfo` ganha `node`, `requires`, `branch_points`, `modifies`, `weapon`, `res_cost`, `res_gain` e `res_spend_all`;
  - `SpecInfo` ganha `branches`, `resource` e `weapons`;
  - `SpecChoice` ganha `respec_cost`;
  - `SpecCmdKind` ganha `RESPEC`.
- **Testes:**
  - reescrever `game/specs_test.go` e o `TestSkillTreeShape`/`TestFifthTierSkills` de `game/game_test.go`. Os novos testes de forma checam limites por tipo de nó, `Requires` dentro da mesma árvore, modificadores apontando para ativas e capacidade da árvore ≥1,4× os pontos;
  - zone `specs_test` (evoluir, trocar, redistribuir, gear);
  - `progress_test` (as 3 linhas da tabela de migração);
  - `TestEffectiveSkillModifiers`;
  - loadbot com RESPEC e regeneração do fuzz corpus;
  - `balance/build.go:60-125` passa a obter as builds de `game.EvolutionBuild(id, level)`, sem IDs fixos no código.

## Fase 6 — Motor de recursos de classe

- Novo `game/resources.go` (puro):
  - `ResourceDef{Max, PerSecondCap, MultiTargetCap, DecayPerSec, DecayAfter, CalmDecay, ResetOnMiss, Gens[]}`;
  - `GenEvent`: HitDealt, CritDealt, DamageTaken, Block, HealDone, SkillCast, DistinctCast, DistinctEffect, ComboHit, RangedHit (com distância mínima);
  - `ResourceState` com os métodos `Gain`, `Tick` e `Spend`.
- Ganchos no zone:
  - `applyHit`: gera para quem ataca e para quem recebe. Nada é gerado se o dano for ≤0 ou se errar, e o erro zera a Precisão;
  - `castSkill`: gasta e gera recurso, e controla o teto de alvos por lançamento;
  - `applyEffect`: gera a Carga Arcana;
  - curas: geram a Fé;
  - `regen` (`combat.go:490`): chama `Tick`;
  - `requestSkill`: recusa com a mensagem "<Recurso> insuficiente".
- DoT e reflexo passam por `z.damage` direto, por isso não geram recurso. Um teste fixa esse comportamento.
- O recurso não é persistido e zera no login.
- Proto: `SelfStats.res`, `max_res` e `res_kind`, e nova mensagem `ResourceInfo` enviada no Welcome.
- Cliente:
  - `client/src/ui/kit.ts`: `bar("res")` com a cor vinda do servidor;
  - `client/src/ui/hud.ts`: barra abaixo do MP;
  - tooltip e hotbar mostram o custo e escurecem a habilidade quando falta recurso.
- O simulador ganha um `ResourceState` por build.
- `zone/invariants.go`: o recurso fica entre 0 e max e nunca é NaN.

## Fase 7 — Cliente: árvore evoluída, janela de Ysolde e redistribuição

- `client/src/ui/windows/skills.ts`:
  - sem evolução: a árvore base com 4 degraus;
  - evoluído: 3 colunas por linhas de marco. As linhas de 45–60 aparecem bloqueadas, marcadas como "Nível 45+";
  - linhas de pré-requisito, selo de `branch_points` e moldura por tipo de nó;
  - o modificador aparece preso à habilidade que modifica.
- `client/src/ui/spec.ts` e `client/src/ui/specs.ts`:
  - mostram recurso, armas, ramos e o aviso "a árvore base será reiniciada";
  - botão de redistribuir com o custo.

## Fase 8 — Rebalanceamento de mana (antes do conteúdo)

- `game.FinishRegen` ganha um multiplicador por classe, `ClassDef.MPRegenMult` (o mago fica perto de 0,75).
- MP e custos do Bárbaro são ajustados para que o kit base não fique sem mana numa luta simulada de 60 s.
- Mente Clara vira `PassiveMPEfficiency`: −x% no custo, com teto de 25%, mantendo o mesmo ID.
- A regeneração fora de combate continua maior (×3,3).
- O relatório do simulador ganha a métrica "tempo até ficar sem mana / DPS sustentado" por classe.
- Testes: `zone/extract_test.go` e o novo `TestManaSustainBands`.

## Fases 9–12 — Conteúdo, um par de classes por PR na `evolucoes`

Ordem: **Cavaleiro → Bárbaro → Mago → Ladino**. O Ladino fica por último porque o Patrulheiro precisa de arco, besta e Precisão. Cada fase segue estes passos:

1. Os docs `docs/evolucoes/<a>.md` e `<b>.md` já estão aprovados pelo usuário.
2. O conteúdo de cada evolução fica em `game/evo_<id>.go`, que substitui a parte da classe em `spec_skills.go`. IDs novos entram **no fim** do `iota`, sempre abaixo de 1000.
3. Definir o `ResourceDef` das duas evoluções, as armas permitidas, os starters e os itens novos (por exemplo, linhas de `axe2h`, varinha/grimório, arco/besta), mais o estoque no NPC.
4. Mecânicas novas reaproveitam os `PassiveKind` da Fase 1. Um `EffectKind` novo é espelhado no proto `Effect`.
5. Rodar o simulador nos níveis 20, 30 e 40. A evolução no 20 deve ter DPS/EHP ≥ o da build base no 20, para não haver queda de poder.
6. Testes por mecânica nova, mais um teste de migração usando um save antigo real daquela classe.
7. Atualizar `docs/classes.md` e `docs/armas.md`.

**Merge da `evolucoes` em main** só depois da Fase 12, com QA completo:

- `make test`;
- relatório do simulador;
- QA no navegador das 8 evoluções e da migração de um personagem nível 20+ de cada classe.

## Fase 13 — Teto 40 → 60 (por último, padrão do PR #39)

Seguir a sequência dos commits `5f8bc0e, f62dd27, 9108f0b, d2c2e77, e1d4757, 0092faf, 58665cb, a0c46e5`.

- **Constantes de nível:** `MaxLevel = 60`, `AscensionLevel = 61`, `HardCap = 100`.
- **Curva de XP:** o `xpTable` passa a ter tamanho `[HardCap-1]` (hoje `[MaxLevel-1]`; com tamanho fixo, entradas faltando viram 0 sem erro de compilação).
  - Os níveis 1–40 ficam idênticos, então não é preciso `ConvertXP`.
  - Se a curva abaixo do 40 mudar, `XPCurve` passa a 3 e a máquina de conversão do 00019 é reaproveitada.
- **Maestria:**
  - `MasteryLevel` vai para 60 automaticamente;
  - `MasteryPointXP` passa a ser `XPToNext(MaxLevel-1)` em vez do valor fixo 45_200_000;
  - `Usable()` mantém os pontos ganhos;
  - decidir no PR o destino do XP parcial da barra.
- **Pontos de habilidade:** `SkillPointsTotal` passa a ir até 61.
- **Conteúdo 45–60** das 8 evoluções (modificadores no 45, chave no 50, aprimoramentos de mecânica no 55, suprema no 60), com a seção do doc revisada antes.
- **O que mais depende do teto:**
  - `ProgressionTargets` e contratos em `quests.go:196`;
  - conquista `level_60`, `invasion.go` e `invariants.go`;
  - `balance/budget.go` e `report.go`;
  - `docs/progressao.md` (regenerado com `EREVIA_UPDATE_DOCS=1`);
  - novo `level60_test.go`.
- **Dependência externa:** áreas, monstros e equipamentos para os níveis 41–60, conforme `docs/roadmap-areas.md`. Não subir o teto sem lugar para upar.

---

## Riscos

- **Deploy com servidores sobrepostos:**
  - só schema aditivo e normalização idempotente no load;
  - sem renomear `spec` nem IDs;
  - perda aceita: pontos gastos no servidor novo durante a janela de sobreposição.
- **Jogador perde o kit no 20.** Mitigado por starters, hotbar auto-preenchida, arma inicial, IDs de habilidade preservados, mensagem única e redistribuição gratuita.
- **Queda de poder ao evoluir.** As passivas da base somem, e a checagem no simulador por evolução cobre isso.
- **Personagem preso a equipamento inválido.** Coberto por `dropInvalidGear`, itens iniciais e estoque nos NPCs.
- **Acoplamentos escondidos por ID.** Fazer `grep game.Skill[A-Z]` em `server/` e `client/` no início de cada fase de conteúdo.
- **Mudanças no protocolo.** Todo campo é aditivo. Regenerar o fuzz corpus e o loadbot nas Fases 2, 5 e 6.

## Arquivos críticos

- `server/internal/game/`: `skills.go`, `specs.go`, `spec_skills.go` (vira `evo_*.go`), `character.go`, `attributes.go`, `items.go`, `classes.go`, `formulas.go`, `mastery.go`, `quests.go`, além dos novos `weapons.go` e `resources.go`.
- `server/internal/zone/`: `combat.go`, `combatant.go`, `effects.go`, `progress.go`, `specs.go`, `inventory.go`, `items.go`, `skills.go`, `zone.go`, `invariants.go`.
- `server/internal/balance/build.go` e `sim.go`; `server/cmd/loadbot/chaos_actions.go`; `server/internal/store/store.go` e a migração `00030`.
- `proto/erevia/v1/game.proto`, regenerado com `make proto`.
- `client/src/`: `world/models.ts`, `world/entities.ts`, `ui/kit.ts`, `ui/hud.ts`, `ui/windows/skills.ts`, `ui/spec.ts`, `ui/specs.ts`, `ui/items.ts`; e `client/scripts/build-models.mjs`.

## Verificação (em toda fase)

- `make proto && make test`: go vet, `go test ./... -race` com Postgres e typecheck do cliente.
- `cd server && go run ./cmd/balance`: o diff do relatório vai na descrição do PR e no doc da evolução.
- Fuzz corpus regenerado (`EREVIA_WRITE_FUZZ_CORPUS=1 go test ./internal/zone -run TestFuzzSeedCorpus`) quando o proto mudar.
- QA no navegador, usando a skill browser-automation sobre o servidor de dev:
  - criar personagem, subir ao 20 via GM e fazer A Encruzilhada;
  - evoluir, conferir a barra de recurso, a árvore e as armas;
  - redistribuir e trocar de evolução;
  - carregar um save antigo nível 20+ com spec e conferir a migração.

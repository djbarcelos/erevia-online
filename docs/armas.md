# Armas

Referência das armas do Erevia: o que existe no jogo hoje, como a arma se relaciona com a classe e quais modelos já temos para novos tipos de arma.

Fonte da verdade (este documento é um retrato do código; se divergir, vale o código):

- [server/internal/game/items.go](../server/internal/game/items.go): catálogo de itens e regra de uso por classe
- [server/internal/zone/inventory.go](../server/internal/zone/inventory.go): validação ao equipar (`canEquip`)
- [client/src/world/models.ts](../client/src/world/models.ts): modelos de arma e animações no cliente
- [client/public/models/weapons/](../client/public/models/weapons/): modelos publicados
- [assets-src/kaykit/x/](../assets-src/kaykit/x/): pacotes KayKit originais (CC0)

Veja também: [classes.md](classes.md).

## Como funciona hoje

Cada arma pertence a **uma única classe**. Se outra classe tenta equipá-la, o servidor recusa com "Sua classe não pode usar este item."

| Equipamento | Quem pode usar |
|---|---|
| Espadas | Só o Cavaleiro |
| Machados | Só o Bárbaro |
| Cajados e varinhas | Só o Mago |
| Adagas e lâminas (uma em cada mão) | Só o Ladino |
| Escudos | Cavaleiro e Bárbaro |
| Armadura, elmo, pernas, luvas e acessórios | Qualquer classe |

Detalhes da implementação:

- **O tipo da arma vem da classe:** cada arma é criada com uma classe dona, e o tipo (`sword`, `axe`, `staff`, `daggers`) é deduzido dela. O item não declara o tipo.
- **O que a arma muda:** os bônus de atributo e o visual na mão do personagem.
- **Uso do tipo de arma em skills:** só a passiva **Esgrima**, do Cavaleiro, confere o tipo da arma (exige espada).
- **O que a arma não muda:** alcance, velocidade de ataque e tipo de dano (físico ou mágico) vêm da classe, não da arma.
- **A animação de ataque também vem da classe:** é escolhida pelo modelo do personagem, não pela arma.

## Armas no jogo

Raridade: **C** = Comum · **B** = Boa · **R** = Rara · **E** = Épica · **L** = Lendária.

As armas marcadas como *opção da criação* são variações de visual escolhidas ao criar o personagem. Elas têm os mesmos atributos da arma comum da classe.

### Espadas (Cavaleiro)

| Arma | Raridade | Nível | Bônus | Modelo |
|---|---|---|---|---|
| Espada de Treino | C | 1 | +6 Atq. físico | `sword_1handed` |
| Lâmina Curva *(opção da criação)* | C | 1 | +6 Atq. físico | `skeleton_blade` |
| Espada de Aço | B | 6 | +14 Atq. físico | `sword_1handed` |
| Lâmina do Vigia | R | 13 | +24 Atq. físico, +30 HP | `sword_2handed` |
| Lâmina da Aurora | E | 18 | +36 Atq. físico, +60 HP, +3% crítico | `sword_2handed_color` |
| Lâmina do Rei Ossário | L | 20 | +44 Atq. físico, +90 HP, +4% crítico | `sword_2handed_color` |

### Machados (Bárbaro)

| Arma | Raridade | Nível | Bônus | Modelo |
|---|---|---|---|---|
| Machado de Lenhador | C | 1 | +7 Atq. físico | `axe_1handed` |
| Machado Serrilhado *(opção da criação)* | C | 1 | +7 Atq. físico | `skeleton_axe` |
| Machado de Guerra | B | 6 | +16 Atq. físico | `axe_1handed` |
| Machado Rachador | R | 13 | +27 Atq. físico, +2% crítico | `axe_2handed` |
| Fúria do Norte | E | 18 | +40 Atq. físico, +4% crítico | `axe_2handed` |
| Machado do Rei Ossário | L | 20 | +49 Atq. físico, +5% crítico, +15% dano crítico | `axe_2handed` |

### Cajados e varinhas (Mago)

| Arma | Raridade | Nível | Bônus | Modelo |
|---|---|---|---|---|
| Cajado de Aprendiz | C | 1 | +8 Atq. mágico | `staff` |
| Varinha de Aprendiz *(opção da criação)* | C | 1 | +8 Atq. mágico | `wand` |
| Cajado Sombrio *(opção da criação)* | C | 1 | +8 Atq. mágico | `skeleton_staff` |
| Cajado de Carvalho | B | 6 | +17 Atq. mágico, +20 MP | `staff` |
| Varinha Rúnica | R | 13 | +28 Atq. mágico, +50 MP | `wand` |
| Cetro do Abismo | E | 18 | +42 Atq. mágico, +90 MP, +3% crítico | `staff` |
| Cetro do Rei Ossário | L | 20 | +51 Atq. mágico, +120 MP, +4% crítico, +5% velocidade de conjuração | `staff` |

### Adagas (Ladino, uma em cada mão)

| Arma | Raridade | Nível | Bônus | Modelo (mão direita / esquerda) |
|---|---|---|---|---|
| Adagas Gêmeas | C | 1 | +5 Atq. físico | `dagger` / `dagger` |
| Lâminas Curvas *(opção da criação)* | C | 1 | +5 Atq. físico | `skeleton_blade` / `skeleton_blade` |
| Adaga e Lâmina *(opção da criação)* | C | 1 | +5 Atq. físico | `dagger` / `skeleton_blade` |
| Adagas de Aço | B | 6 | +12 Atq. físico, +2% crítico | `dagger` / `dagger` |
| Presas Sombrias | R | 13 | +21 Atq. físico, +4% crítico | `dagger` / `dagger` |
| Presas do Crepúsculo | E | 18 | +32 Atq. físico, +6% crítico | `dagger` / `dagger` |
| Garras do Rei Ossário | L | 20 | +39 Atq. físico, +8% crítico, +10% dano crítico | `dagger` / `dagger` |

### Escudos (Cavaleiro e Bárbaro)

| Escudo | Raridade | Nível | Bônus | Modelo |
|---|---|---|---|---|
| Escudo de Brasão | C | 1 | +6 Def. física | `shield_badge_color` |
| Broquel Bárbaro | C | 1 | +4 Def. física, +1 Atq. físico | `shield_round_barbarian` |
| Escudo Pesado | C | 1 | +8 Def. física, −3 esquiva | `skeleton_shield_large_a` |
| Escudo de Ferro | B | 6 | +14 Def. física, +4 Def. mágica | `shield_square` |
| Escudo de Cravos | R | 13 | +24 Def. física, +8 Def. mágica | `shield_spikes` |
| Égide Óssea | E | 18 | +36 Def. física, +14 Def. mágica, +50 HP | `shield_spikes_color` |

## Modelos disponíveis e ainda não usados

Estes arquivos estão nos pacotes KayKit (licença CC0), em [assets-src/kaykit/x/](../assets-src/kaykit/x/). Para entrar no jogo, cada um precisa ser exportado para [client/public/models/weapons/](../client/public/models/weapons/) e ganhar um item no catálogo.

| Modelo | Arquivos | Pacote | Uso possível |
|---|---|---|---|
| Arco | `bow`, `bow_withString` | Adventurers 2.0 | Arma de longo alcance |
| Flechas | `arrow_bow`, `arrow_bow_bundle` | Adventurers 2.0 | Projétil e visual |
| Aljava | `quiver` | Adventurers 2.0 | Acessório nas costas |
| Besta | `crossbow_1handed`, `crossbow_2handed` | Adventurers 2.0 | Arma de longo alcance (1 ou 2 mãos) |
| Virotes | `arrow_crossbow`, `arrow_crossbow_bundle` | Adventurers 2.0 | Projétil da besta |
| Grimório | `spellbook_closed`, `spellbook_open` | Adventurers 2.0 | Mão secundária do Mago |
| Bomba de fumaça | `smokebomb` | Adventurers 2.0 | Item consumível ou visual de skill |
| Besta de esqueleto | `Skeleton_Crossbow` | Skeletons 1.1 | Variação de besta ou arma de monstro |
| Aljava de esqueleto | `Skeleton_Quiver` | Skeletons 1.1 | Variação de aljava |
| Flechas de esqueleto | `Skeleton_Arrow`, `Skeleton_Arrow_Half`, versões quebradas | Skeletons 1.1 | Projétil ou cenário |
| Escudos extras | `shield_round`, `shield_round_color`, `shield_square_color`, `shield_badge`, `Skeleton_Shield_Large_B`, `Skeleton_Shield_Small_A`, `Skeleton_Shield_Small_B` | Ambos | Variações de escudo |

### Animações disponíveis

O pacote de animações (`KayKit_Character_Animations_1.1`, arquivo `Rig_Medium_CombatRanged`) usa o mesmo esqueleto dos heróis. Além das animações de magia que o jogo já usa, ele tem:

| Arma | Clipes |
|---|---|
| Arco | `Ranged_Bow_Draw`, `Ranged_Bow_Release`, `Ranged_Bow_Aiming_Idle`, `Ranged_Bow_Idle`, `Ranged_Bow_Draw_Up`, `Ranged_Bow_Release_Up` |
| Besta de 1 mão | `Ranged_1H_Aiming`, `Ranged_1H_Shoot`, `Ranged_1H_Shooting`, `Ranged_1H_Reload` |
| Besta de 2 mãos | `Ranged_2H_Aiming`, `Ranged_2H_Shoot`, `Ranged_2H_Shooting`, `Ranged_2H_Reload` |

Esses clipes não estão no arquivo publicado: o [anims-combatranged.glb](../client/public/models/anims-combatranged.glb) atual só tem `Ranged_Magic_Raise`, `Ranged_Magic_Shoot` e `Ranged_Magic_Spellcasting`. Para usá-los, é preciso exportar o arquivo de novo.

O pacote Adventurers também tem o personagem **Ranger** (`Ranger.glb`), com o mesmo esqueleto dos 4 heróis atuais.

## Tipos de arma possíveis sem arte nova

| Tipo | Situação |
|---|---|
| Espada (1 ou 2 mãos) | No jogo |
| Machado (1 ou 2 mãos) | No jogo |
| Cajado / varinha | No jogo |
| Adagas (duas mãos) | No jogo |
| Escudo | No jogo |
| **Arco** | Modelo e animações disponíveis |
| **Besta (1 ou 2 mãos)** | Modelo e animações disponíveis |
| **Grimório** (mão secundária) | Modelo disponível |

## O que muda para abrir as armas entre classes

Para que uma classe possa usar mais de um tipo de arma, e para que a arma defina como o personagem ataca, os pontos a mexer são:

1. ~~**Declarar o tipo direto no item.**~~ Feito (PR #79): `ItemDef.WeaponType` é explícito e as varinhas são `wand`.
2. ~~**Dizer quais tipos cada classe usa.**~~ Feito: `ClassDef.Weapons`/`Offhands` e `SpecDef.Weapons`/`Offhands`, conferidos por `game.CanWield`/`CanOffhand` ([weapons.go](../server/internal/game/weapons.go)).
3. ~~**Mover o ataque para o tipo de arma.**~~ Feito: `ApplyWeapon` aplica alcance, intervalo e tipo de dano do `WeaponDef`; `ClassDef.Base` vale só para quem está desarmado.
4. **Escolher a animação pela arma** (falta, PR C da tunnel#180): no cliente, `ANIM_SETS` ([models.ts](../client/src/world/models.ts)) hoje é por classe; o projétil do ataque básico depende de `isCaster` ([entities.ts](../client/src/world/entities.ts)).
5. **Decidir como as skills se relacionam com a arma** (falta; ver [evolucoes/README.md](evolucoes/README.md)): livres, exigindo um tipo de arma (como a Esgrima já faz) ou com bônus quando usadas com a arma "certa".
6. **Rebalancear:** os atributos base das classes foram ajustados pensando numa arma fixa.

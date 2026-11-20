# Fichas de movimento (heróis v4)

Como cada classe se move. A ficha vale para todos os clipes da classe e é o que impede que os heróis pareçam o mesmo boneco com roupa trocada. Os tempos e nomes seguem o [contrato](contrato.md); o plano está em [plano-herois-v4.md](../plano-herois-v4.md).

---

## 1. Princípios para todos

- **Pose lida a 85 px.** Antecipação clara antes do golpe, impacto com o corpo inteiro e pausa curta (2–3 quadros) no impacto. Pose sutil não se lê de longe.
- **Peso.** Quem é pesado demora para começar e para parar; quem é leve muda de direção no meio do passo.
- **Arcos.** Mãos, arma e cabeça seguem arcos; nada vai em linha reta, a não ser estocada e flecha.
- **Sobreposição.** Capa, cabelo, manto e acessórios chegam 2–4 quadros depois do corpo e assentam depois dele.
- **Parado vivo.** Respiração, troca de peso e um gesto da classe. Nada fica congelado.
- **Leitura de estado.** Quem está em combate (postura de combate, `stance`) se distingue de quem está passeando (`idle`) de longe.

---

## 2. Camadas

| Camada | Clipes | Quem faz o quê |
|---|---|---|
| **Base** (`base.*`) | andar, correr, girar no lugar, dano (2), atordoado, morte (2), levantar, acenar, sentar, dançar, comemorar | ciclo de pernas e reações, neutros |
| **Postura da classe** (`<classe>.posture`) | 1 pose aditiva | por cima da base: inclinação do tronco, abertura dos braços, altura do quadril, onde fica a arma |
| **Classe** (`<classe>.*`) | parado e pausas, andar em combate, postura de combate, ataques, bloqueio ou esquiva, skills | tudo o que é da classe, feito inteiro |

A postura aditiva faz o andar, o correr e as reações da base ganharem o jeito da classe sem refazer o ciclo de pernas. Ex.: o mesmo `base.run`, com `bb.posture`, vira uma corrida inclinada de braços abertos.

---

## 3. Recruta (`rc`)

Quem ainda não escolheu classe. Serve também de contraste: ao virar Cavaleiro, Bárbaro, Mago ou Ladino, o jogador **sente** a diferença no corpo.

| | |
|---|---|
| Postura | neutra, ombros um pouco caídos, olhar à frente |
| Ritmo | comum, sem estilo |
| Assinatura | segura a arma de treino sem jeito: pega alta demais, cotovelo aberto |
| Clipes | `rc.idle`, `rc.attack.sword.1` e `.2` (golpe de cima para baixo e de lado, sem técnica), 2 skills de recruta (`rc.skill.strike`, `rc.skill.dodge`) |

---

## 4. Cavaleiro (`kn`)

**Ideia:** uma muralha que anda. O escudo é a primeira coisa que se move e a última que se abaixa.

| | |
|---|---|
| Postura (`kn.posture`) | tronco ereto, quadril um pouco baixo, joelhos levemente dobrados; escudo diante do peito, espada baixa ao lado |
| Peso | pesado e estável: começa e para em 3–4 quadros, sem balanço |
| Ritmo | firme e econômico: golpes curtos, recuperação rápida para a guarda |
| Assinatura | o escudo participa de tudo: bloqueia, empurra e bate |
| Nunca | girar o corpo inteiro no golpe (é do Bárbaro); abaixar o escudo sem motivo |

### Clipes

| Clipe | Quadros | Poses-chave |
|---|---|---|
| `kn.idle` | 120 (laço) | respiração lenta; a cada ciclo, ajusta a alça do escudo |
| `kn.break.1` | 90 | olha o horizonte, mão no punho da espada |
| `kn.break.2` | 90 | bate a espada no escudo, de leve, duas vezes |
| `kn.walk_combat` | 26 (laço) | passos curtos, escudo à frente, espada pronta |
| `kn.stance` | 60 (laço) | guarda alta: escudo na altura do queixo, espada por cima |
| `kn.attack.sword.1` | 30, impacto 13 | corte diagonal de cima para baixo, escudo recua para o quadril |
| `kn.attack.sword.2` | 30, impacto 12 | corte horizontal da esquerda para a direita, passo à frente |
| `kn.attack.sword.3` | 36, impacto 16 | **pancada de escudo** + estocada curta (fecha o combo) |
| `kn.block` | 20 | escudo para cima, corpo se fecha atrás dele, recua meio passo |

### Skills por categoria

| Categoria | Clipe | Exemplos de skill |
|---|---|---|
| golpe pesado | `kn.skill.heavy` | Golpe de Escudo, Golpe Radiante, Martelo da Aurora |
| giro | `kn.skill.spin` | Corte do Horizonte, Último Bastião |
| investida | `kn.skill.charge` | Interceptar, investida com escudo |
| grito ou buff | `kn.skill.raise` | Desafio, Bênção da Luz, Égide da Ordem |
| área no chão | `kn.skill.ground` | Zona de Provocação, Sol Nascente, Baluarte |
| guarda | `kn.skill.guard` | Guarda Inabalável (laço enquanto dura) |

**Evoluções:** o **Bastião** abre mais a base e fica mais baixo (`bastion.stance`); o **Paladino** ergue a arma ao céu no buff (`herald.raise`) e tem um parado mais ereto, de mão no peito (`herald.idle`).

---

## 5. Bárbaro (`bb`)

**Ideia:** uma avalanche. Demora para começar e, quando começa, não para.

| | |
|---|---|
| Postura (`bb.posture`) | tronco inclinado para a frente, ombros muito abertos, braços longe do corpo; a arma pende de uma mão ou é arrastada |
| Peso | muito pesado: antecipação longa (8–12 quadros), impacto com o corpo inteiro, recuperação longa |
| Ritmo | lento, explosivo e lento de novo |
| Assinatura | o corpo inteiro gira no golpe; a arma é arrastada no chão no parado e na corrida de duas mãos |
| Nunca | golpe curto e econômico (é do Cavaleiro); ficar ereto |

### Clipes

| Clipe | Quadros | Poses-chave |
|---|---|---|
| `bb.idle` | 120 (laço) | respiração pesada, ombros sobem e descem; gira o pescoço |
| `bb.break.1` | 90 | bate o punho no peito |
| `bb.break.2` | 90 | apoia o machado no ombro e cospe de lado |
| `bb.walk_combat` | 26 (laço) | passos largos, ombros balançando |
| `bb.stance` | 60 (laço) | agachado, arma para trás, pronto para avançar |
| `bb.attack.axe.1` / `.2` / `.3` | 42, impactos 18 / 17 / 22 | diagonal; volta horizontal; **pancada de cima** com os dois braços (fecha o combo) |
| `bb.attack.axe2h.1` / `.2` / `.3` | 54, impactos 24 / 22 / 28 | varredura larga; giro inteiro; machadada no chão |
| `bb.dodge` | 20 | ombro à frente, avança em vez de recuar |

Duas mãos: parado, andar em combate e postura têm versão `axe2h` (`bb.idle.axe2h`…), com a mão esquerda presa no cabo por IK.

### Skills por categoria

| Categoria | Clipe | Exemplos de skill |
|---|---|---|
| golpe pesado | `bb.skill.heavy` | Golpe Brutal, Golpe Rubro, Execução |
| giro | `bb.skill.spin` | Redemoinho Sangrento, Turbilhão |
| salto | `bb.skill.leap` | Avanço de Guerra, Investida Implacável |
| grito ou buff | `bb.skill.roar` | Cólera, Brado de Guerra, Rugido |
| área no chão | `bb.skill.ground` | Machadada Sísmica, Impacto, Estandarte de Guerra |

**Evoluções:** o **Berserker** fica mais inclinado e treme de raiva no parado (`bloodrager.idle`); o **Senhor da Guerra** é mais ereto, aponta com a arma ao dar ordens (`warlord.raise`) e finca o estandarte com as duas mãos.

---

## 6. Mago (`mg`)

**Ideia:** um condutor de orquestra. As mãos nunca param; o feitiço é desenhado no ar antes de sair.

| | |
|---|---|
| Postura (`mg.posture`) | ereto e leve, cajado em pé ao lado (ou varinha baixa), mão livre aberta à frente |
| Peso | leve: começa rápido, sem impacto no corpo; o manto faz o peso |
| Ritmo | fluido e contínuo, com pausas de concentração antes dos feitiços grandes |
| Assinatura | **gesto de mão** antes de cada feitiço; a mão livre desenha a forma (círculo para área, linha para projétil) |
| Nunca | golpe físico com o cajado (a não ser como último recurso); passos largos |

### Clipes

| Clipe | Quadros | Poses-chave |
|---|---|---|
| `mg.idle` | 120 (laço) | apoia no cajado; a mão livre gira uma faísca entre os dedos |
| `mg.break.1` | 90 | folheia o grimório no ar |
| `mg.break.2` | 90 | olha a ponta do cajado e sopra uma brasa |
| `mg.walk_combat` | 26 (laço) | passos curtos, cajado à frente, mão livre pronta |
| `mg.stance` | 60 (laço) | cajado firme no chão, mão livre erguida e aberta |
| `mg.attack.staff.1` / `.2` / `.3` | 48, impacto 20 | mão livre desenha e empurra o projétil; alterna a mão; o 3º sai do cajado apontado |
| `mg.attack.wand.1` / `.2` / `.3` | 48, impacto 18 | estalo da varinha, mais curto e de pulso |
| `mg.dodge` | 18 | passo lateral rápido, manto rodando |

### Skills por categoria

| Categoria | Clipe | Exemplos de skill |
|---|---|---|
| projétil | `mg.skill.projectile` | Bola de Fogo, Lança Ígnea, Rajada Arcana, Drenar Vontade |
| área no chão | `mg.skill.ground` | Meteoro, Chuva de Brasas, Nuvem Pestilenta |
| em volta de si | `mg.skill.nova` | Nova de Gelo, Zero Absoluto |
| canalizar | `mg.skill.channel` | Inferno, Maldição Persistente (laço) |
| buff em si | `mg.skill.self` | Escudo Arcano, Armadura de Gelo |
| teleporte | `mg.skill.blink` | Piscar |

**Evoluções:** o **Elementalista** tem gestos largos e quentes, as duas mãos juntas nos feitiços grandes (`elementalist.cast_big`); o **Arcanista** tem gestos pequenos e precisos, de dedos, e um parado curvado sobre o grimório (`occultist.idle`).

---

## 7. Ladino (`rg`)

**Ideia:** uma mola. Sempre pronto para sair de onde está.

| | |
|---|---|
| Postura (`rg.posture`) | baixo, joelhos dobrados, peso na ponta dos pés, tronco à frente; adagas invertidas junto aos antebraços |
| Peso | leve e elástico: muda de direção no meio do passo |
| Ritmo | rápido e quebrado: fintas, pausas curtíssimas, trocas de base |
| Assinatura | as adagas giram nas mãos; deslocamentos curtos e diagonais |
| Nunca | ficar de frente e parado; golpe largo |

### Clipes

| Clipe | Quadros | Poses-chave |
|---|---|---|
| `rg.idle` | 120 (laço) | troca o peso de um pé para o outro; gira uma adaga entre os dedos |
| `rg.break.1` | 90 | olha por cima do ombro |
| `rg.break.2` | 90 | lança a adaga para cima e pega |
| `rg.walk_combat` | 26 (laço) | passo cruzado lateral, agachado |
| `rg.stance` | 60 (laço) | base larga e baixa, uma adaga à frente e outra atrás |
| `rg.attack.daggers.1` / `.2` / `.3` | 24, impactos 8 / 8 / 6 e 12 | corte da direita; corte da esquerda; **duplo** cruzado (fecha o combo) |
| `rg.attack.bow.1` | 45, solta no 20 | puxa a corda até a orelha, solta, mão para trás |
| `rg.attack.crossbow.1` | 51, solta no 20 | ombro na besta, dispara, recarrega com o pé no estribo |
| `rg.attack.crossbow2h.1` | 66, solta no 26 | apoia, mira, dispara, recarga de manivela |
| `rg.dodge` | 16 | rolamento curto em diagonal |

### Skills por categoria

| Categoria | Clipe | Exemplos de skill |
|---|---|---|
| golpe rápido | `rg.skill.stab` | Golpe Furtivo, Golpe Fatal, Golpe de Misericórdia |
| salto ao alvo | `rg.skill.dash` | Golpe das Sombras, Sentença |
| sumir | `rg.skill.vanish` | Sumir, Passo Sombrio, Véu Mortal |
| arremesso | `rg.skill.throw` | Faca de Arremesso, Rede de Caça, Bomba de Fumaça |
| tiro carregado | `rg.skill.aimed` | Tiro Preciso, Flecha Mortal, Disparo Perfurante |
| área no chão | `rg.skill.ground` | Chuva de Flechas, Armadilha de Caça, Campo de Espinhos |
| salto para trás | `rg.skill.backflip` | Recuo |

**Evoluções:** o **Assassino** fica mais baixo e quase não pisca o corpo (parado de predador, `cutthroat.idle`); o **Patrulheiro** fica mais ereto, de arco pronto, e tem um parado de olhar o horizonte com a mão na testa (`hunter.idle`).

---

## 8. Duplas

A dupla herda a **ficha da 1ª maestria** (a classe do corpo) e ganha **um** detalhe da 2ª, em um ou dois clipes, sem mudar a base:

| Onde o detalhe aparece | Exemplo |
|---|---|
| parado (`<dupla>.idle`) | **Possesso:** tique de cabeça e ombros puxados por correntes invisíveis |
| grito ou buff (`<dupla>.raise`) | **Cruzado:** ergue a espada e o escudo juntos, com a luz descendo |
| pausa (`<dupla>.break`) | **Bruxo:** gira a adaga e ela se acende de roxo |

Cada uma das 28 duplas tem o seu detalhe definido em [kits.md](kits.md), na linha da dupla.

---

## 9. Conta de clipes

| Grupo | Clipes |
|---|---|
| Base | 13 |
| Recruta | 5 |
| Cavaleiro | 9 + 6 skills + 2 posturas/variações de evolução ≈ 19 |
| Bárbaro | 9 + 3 (duas mãos) + 3 (combo `axe2h`) + 5 skills + 2 ≈ 22 |
| Mago | 11 + 6 skills + 2 ≈ 19 |
| Ladino | 12 + 7 skills + 2 ≈ 21 |
| Detalhes das duplas | ~28 (um clipe curto cada) |
| **Total** | **~127** |

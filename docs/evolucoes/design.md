# Erevia --- Documento de Design de Classes, Evolução e Progressão

> **Status:** proposta consolidada de design para orientar a
> reformulação do sistema.\
> **Escopo:** quatro classes-base, oito evoluções, árvores de
> habilidades, recursos de classe, progressão até o nível 100, maestria
> e sistema de armas.\
> **Lançamento inicial:** nível máximo 60.\
> **Visão de longo prazo:** progressão preparada até o nível 100.

------------------------------------------------------------------------

## 1. Visão geral do sistema

O sistema de classes do Erevia será organizado em duas etapas
principais:

1.  **Classe-base (níveis 1--19):** o personagem aprende os fundamentos
    da classe e experimenta diferentes estilos de jogo. Não existe uma
    escolha definitiva de especialização nessa etapa.
2.  **Classe evoluída (a partir do nível 20):** o jogador escolhe uma de
    duas evoluções exclusivas da classe-base. Essa escolha define
    identidade, mecânica central, armas permitidas e uma nova árvore de
    habilidades.

A escolha da evolução deve ser importante, mas não irreversível. O
jogador poderá trocar de evolução por meio de um processo próprio, com
custo ou condição definidos pelo jogo. Ao trocar, a árvore da evolução
anterior é reiniciada e seus pontos são devolvidos para serem
redistribuídos na nova árvore.

### 1.1 Princípios de design

-   **Identidade clara:** cada classe precisa ter uma função
    reconhecível, sem impedir estilos alternativos.
-   **Escolha gradual:** antes do nível 20, o jogador pode experimentar
    os três ramos da classe-base.
-   **Especialização real:** no nível 20, a evolução altera a forma de
    jogar; não é apenas um bônus de atributos.
-   **Árvore nova:** a evolução começa com uma árvore própria. As
    habilidades da classe-base não são mantidas como habilidades
    equipadas.
-   **Armas com identidade:** o tipo de arma define alcance, ritmo de
    ataque, animação e comportamento do ataque básico.
-   **Recursos equilibrados:** todas as classes precisam de uma forma
    própria, previsível e controlável de recuperar seu recurso.
-   **Progressão horizontal no endgame:** a Maestria deve oferecer
    objetivos de longo prazo sem criar uma diferença excessiva de poder
    entre jogadores.

### 1.2 Mapa das classes

  ----------------------------------------------------------------------------------
  Classe-base    Evolução 1          Identidade     Evolução 2        Identidade
  -------------- ------------------- -------------- ----------------- --------------
  Cavaleiro      **Bastião**         Defesa,        **Paladino**      Dano sagrado,
                                     provocação e                     bênçãos e
                                     proteção                         suporte

  Bárbaro        **Berserker**       Dano           **Senhor da       Combate
                                     agressivo,     Guerra**          pesado,
                                     fúria e                          liderança e
                                     execução                         controle

  Mago           **Elementalista**   Dano elemental **Arcanista**     Controle,
                                     e explosões                      maldições e
                                                                      manipulação
                                                                      arcana

  Ladino         **Assassino**       Críticos,      **Patrulheiro**   Combate à
                                     furtividade e                    distância,
                                     eliminação                       armadilhas e
                                                                      perseguição
  ----------------------------------------------------------------------------------

Os nomes e os conceitos acima são a proposta de organização para a nova
estrutura. Eles substituem, como destino de evolução, as especializações
antigas que eram liberadas como um ramo adicional no nível 20.

------------------------------------------------------------------------

## 2. Regras gerais das árvores de habilidades

### 2.1 Classe-base --- níveis 1 a 19

A classe-base possui três ramos temáticos. Os ramos são acessíveis
simultaneamente: o jogador pode investir pontos em mais de um ramo e
montar uma combinação própria.

A árvore-base serve para ensinar as possibilidades da classe, não para
determinar permanentemente o papel final do personagem.

  Regra                Proposta
  -------------------- -------------------------------------------------------
  Faixa                Níveis 1--19
  Ramos                3 por classe
  Degraus              1, 5, 10 e 15
  Investimento         Livre entre os três ramos, respeitando pré-requisitos
  Escolha definitiva   Nenhuma antes do nível 20
  Reset                Completo ao concluir a evolução no nível 20

O antigo degrau de nível 20 deixa de pertencer à árvore-base. Ele passa
a ser parte da árvore da evolução.

### 2.2 Evolução --- nível 20 em diante

Ao concluir a missão **A Encruzilhada**, o personagem escolhe uma das
duas evoluções de sua classe-base.

A escolha executa as seguintes ações:

1.  Registra a evolução escolhida.
2.  Remove os pontos investidos na árvore da classe-base.
3.  Devolve esses pontos ao personagem.
4.  Desbloqueia a árvore exclusiva da evolução.
5.  Libera a mecânica central da evolução.
6.  Atualiza a lista de armas que o personagem pode equipar.
7.  Atualiza atributos derivados, habilidades disponíveis e animações
    necessárias.

As habilidades da classe-base deixam de ser aprendíveis/equipáveis após
a evolução. A identidade da classe-base continua existindo por meio da
história, atributos fundamentais e fantasia visual, mas a progressão de
combate passa a ser a da evolução.

### 2.3 Pré-requisitos e ranks

A árvore evoluída será organizada em três ramos temáticos, com
habilidades ativas, passivas e modificadores.

Regras recomendadas:

-   Habilidades ativas podem ter até 3 ranks.
-   Passivas comuns podem ter até 5 ranks.
-   Habilidades-chave e habilidades supremas podem ter 1 rank.
-   Para avançar em um ramo, o jogador precisa investir pontos nos nós
    anteriores daquele ramo.
-   Um modificador altera uma habilidade já aprendida; não deve ser
    apenas mais uma habilidade ativa ocupando espaço na barra.
-   A árvore deve permitir mais de uma configuração viável, sem exigir
    que todo jogador compre todos os nós.

### 2.4 Redistribuição

A redistribuição deve ser simples e previsível:

-   O jogador pode redefinir a árvore da evolução e recuperar os pontos
    investidos.
-   A troca de evolução deve sempre limpar a árvore anterior.
-   A troca de evolução não deve apagar nível, equipamentos, atributos
    permanentes ou progresso de missões.
-   O custo, intervalo ou item necessário para redistribuição deve ser
    definido como regra global, não individual por classe.

------------------------------------------------------------------------

## 3. As quatro classes-base

As classes-base apresentam os fundamentos de cada arquétipo. Seus três
ramos devem ser experimentais e complementares.

### 3.1 Cavaleiro

**Fantasia:** combatente disciplinado, resistente e confiável, capaz de
segurar a linha de frente ou fortalecer aliados.

**Atributos de referência atuais (nível 1):**

  Atributo            Valor atual
  ----------------- -------------
  HP                          160
  MP                           40
  Ataque físico                 8
  Ataque mágico                 4
  Defesa física                24
  Defesa mágica                15
  Alcance atual             2,2 m
  Intervalo atual           1,2 s
  Crítico                      5%
  Velocidade                5 m/s

**Ramos da classe-base:**

  -----------------------------------------------------------------------
  Ramo                    Função                  Habilidades de
                                                  referência do sistema
                                                  atual
  ----------------------- ----------------------- -----------------------
  Guardião                Defesa e controle da    Golpe de Escudo, Pele
                          linha de frente         de Ferro, Provocar,
                                                  Muralha Viva, Baluarte

  Lâmina                  Esgrima e dano corpo a  Esgrima, Corte
                          corpo                   Giratório, Investida,
                                                  Julgamento, Corte do
                                                  Horizonte

  Comando                 Gritos, auras e         Fortaleza, Grito de
                          proteção de grupo       Guerra, Aura de
                                                  Coragem, Estandarte,
                                                  Égide da Ordem
  -----------------------------------------------------------------------

**Direção da reformulação:** manter os três estilos acessíveis durante
os níveis 1--19. O jogador pode testar defesa, dano e suporte antes de
escolher Bastião ou Paladino.

### 3.2 Bárbaro

**Fantasia:** guerreiro de força bruta, resistência e agressividade. Seu
poder deve crescer quando se mantém em combate.

**Atributos de referência atuais (nível 1):**

  Atributo            Valor atual
  ----------------- -------------
  HP                          150
  MP                           35
  Ataque físico                10
  Ataque mágico                 3
  Defesa física                16
  Defesa mágica                12
  Alcance atual             2,2 m
  Intervalo atual           1,4 s
  Crítico                     10%
  Velocidade                5 m/s

**Ramos da classe-base:**

  -----------------------------------------------------------------------
  Ramo                    Função                  Habilidades de
                                                  referência do sistema
                                                  atual
  ----------------------- ----------------------- -----------------------
  Massacre                Golpes pesados e        Golpe Brutal, Machado
                          execução                Pesado, Rachar o Chão,
                                                  Execução, Machadada
                                                  Sísmica

  Sangue                  Fúria, roubo de vida e  Fúria, Sede de Sangue,
                          persistência            Frenesi, Imortal,
                                                  Redemoinho Sangrento

  Selvagem                Resistência e instinto  Couro Grosso, Rugido,
                          animal                  Pisotear, Espírito do
                                                  Urso, Pele de Pedra
  -----------------------------------------------------------------------

**Direção da reformulação:** o Bárbaro deve ter um ciclo de combate
ativo. Seu recurso não pode depender apenas de uma reserva pequena de
MP, nem ficar sem resposta quando todas as habilidades entram em
recarga.

### 3.3 Mago

**Fantasia:** conjurador frágil, com alto poder mágico, alcance e
controle do campo de batalha.

**Atributos de referência atuais (nível 1):**

  Atributo            Valor atual
  ----------------- -------------
  HP                           95
  MP                          120
  Ataque físico                 6
  Ataque mágico                12
  Defesa física                12
  Defesa mágica                25
  Alcance atual              12 m
  Intervalo atual           1,6 s
  Crítico                      5%
  Velocidade                5 m/s

**Ramos da classe-base:**

  -----------------------------------------------------------------------
  Ramo                    Função                  Habilidades de
                                                  referência do sistema
                                                  atual
  ----------------------- ----------------------- -----------------------
  Fogo                    Dano alto e queimadura  Bola de Fogo, Brasa
                                                  Viva, Chuva de Brasas,
                                                  Meteoro, Inferno

  Gelo                    Controle e proteção     Nova de Gelo, Lança de
                                                  Gelo, Armadura de Gelo,
                                                  Prisão Glacial, Zero
                                                  Absoluto

  Arcano                  Mana, mobilidade e      Mente Clara, Escudo
                          poder bruto             Arcano, Piscar,
                                                  Sobrecarga, Rajada
                                                  Arcana
  -----------------------------------------------------------------------

**Direção da reformulação:** reduzir a dependência de regeneração
passiva ilimitada. O Mago deve continuar sendo a classe com melhor
gestão de mana, mas precisa tomar decisões sobre ritmo de conjuração e
gasto de recursos.

### 3.4 Ladino

**Fantasia:** combatente ágil, oportunista e letal, especializado em
críticos, mobilidade, furtividade e efeitos debilitantes.

**Atributos de referência atuais (nível 1):**

  Atributo            Valor atual
  ----------------- -------------
  HP                          120
  MP                           60
  Ataque físico                 7
  Ataque mágico                 4
  Defesa física                16
  Defesa mágica                12
  Alcance atual             2,2 m
  Intervalo atual           0,8 s
  Crítico                     25%
  Velocidade              5,5 m/s

**Ramos da classe-base:**

  -----------------------------------------------------------------------
  Ramo                    Função                  Habilidades de
                                                  referência do sistema
                                                  atual
  ----------------------- ----------------------- -----------------------
  Assassinato             Críticos e golpes       Golpe Furtivo, Ponto
                          letais                  Fraco, Lâminas Gêmeas,
                                                  Marca da Morte, Golpe
                                                  Fatal

  Sombras                 Mobilidade, esquiva e   Passo Sombrio, Esquiva,
                          invisibilidade          Bomba de Fumaça, Sumir,
                                                  Golpe das Sombras

  Venenos                 Dano contínuo e         Lâmina Envenenada,
                          controle                Frasco Tóxico,
                                                  Paralisia, Praga,
                                                  Miasma
  -----------------------------------------------------------------------

**Direção da reformulação:** separar melhor o combate corpo a corpo do
combate à distância. O Patrulheiro passa a ter identidade própria com
arco e besta, enquanto o Assassino mantém a fantasia de combate próximo.

------------------------------------------------------------------------

## 4. Evoluções e árvores avançadas

Cada evolução possui três ramos. A árvore avançada deve oferecer
escolhas entre dano, defesa, utilidade e aprimoramento da mecânica
exclusiva.

Os nomes e efeitos abaixo são uma proposta funcional para orientar a
implementação. Valores numéricos finais de dano, duração, custo e
recarga precisam ser calibrados em testes de PvE, chefes e PvP.

### 4.1 Cavaleiro → Bastião

**Papel:** tanque principal, proteção de aliados e controle de inimigos.

**Identidade:** o Bastião transforma defesa em valor para o grupo. Deve
conseguir manter inimigos ocupados, reduzir pressão sobre aliados e
sobreviver a janelas de dano elevado.

**Armas:** espada e escudo como conjunto principal. Escudos devem ser
permitidos obrigatoriamente; espada de uma mão recomendada.

**Mecânica exclusiva --- Determinação**

-   Gera Determinação ao bloquear ataques, receber dano de inimigos
    próximos e usar habilidades defensivas.
-   Determinação alimenta habilidades de proteção e provocações
    reforçadas.
-   A geração deve ter limite por segundo para evitar exploração com
    muitos inimigos fracos ou ataques repetidos.
-   A Determinação não deve ser gerada de forma relevante fora de
    combate.

**Ramos:**

  -----------------------------------------------------------------------
  Ramo                    Foco                    Habilidades-chave
                                                  propostas
  ----------------------- ----------------------- -----------------------
  Muralha                 Mitigação e             Guarda Inabalável,
                          sobrevivência           Postura de Ferro,
                                                  Muralha Viva, Último
                                                  Reduto

  Protetor                Proteção de aliados     Interceptar, Vínculo de
                                                  Guarda, Égide
                                                  Compartilhada,
                                                  Juramento de Proteção

  Provocador              Ameaça e controle       Desafio do Bastião,
                                                  Golpe de Escudo, Zona
                                                  de Provocação, Prisão
                                                  do Guardião
  -----------------------------------------------------------------------

**Habilidade suprema proposta --- Fortaleza Imortal:** cria uma zona
defensiva temporária. Inimigos são incentivados/provocados a atacar o
Bastião, enquanto aliados dentro da área recebem mitigação. A força do
efeito deve depender da Determinação consumida.

### 4.2 Cavaleiro → Paladino

**Papel:** suporte de combate, dano sagrado e fortalecimento do grupo.

**Identidade:** o Paladino não substitui um curandeiro dedicado; ele
combina presença na linha de frente, bênçãos, mitigação e dano sagrado.

**Armas:** espada e escudo; permitir maça/arma contundente no futuro se
houver modelos e animações adequados. Grimório ou símbolo sagrado como
foco secundário é uma possibilidade futura, não requisito inicial.

**Mecânica exclusiva --- Fé**

-   Gera Fé ao acertar inimigos com ataques sagrados, proteger aliados
    com bênçãos e participar de eliminações.
-   Consome Fé em habilidades de grupo e golpes sagrados reforçados.
-   A geração por suporte deve considerar aliados efetivamente
    beneficiados, com limite por alvo e por intervalo.

**Ramos:**

  -----------------------------------------------------------------------
  Ramo                    Foco                    Habilidades-chave
                                                  propostas
  ----------------------- ----------------------- -----------------------
  Julgamento              Dano sagrado            Golpe Radiante,
                                                  Veredito, Martelo da
                                                  Aurora, Sentença
                                                  Sagrada

  Bênçãos                 Suporte e               Bênção da Luz, Aura de
                          fortalecimento          Coragem, Renovação,
                                                  Bênção Suprema

  Expiação                Debilitação e proteção  Purificar, Luz Cegante,
                                                  Escudo da Fé,
                                                  Sacrifício
  -----------------------------------------------------------------------

**Habilidade suprema proposta --- Avatar da Aurora:** fortalece
temporariamente o Paladino, amplia a eficácia de suas bênçãos e libera
uma explosão sagrada ao final ou ao consumir Fé.

### 4.3 Bárbaro → Berserker

**Papel:** dano físico sustentado, explosão de dano e execução.

**Identidade:** o Berserker aceita riscos para manter pressão ofensiva.
Deve ser recompensado por atacar e permanecer em combate, não por
esperar passivamente.

**Armas:** machado de duas mãos como identidade principal; machado de
uma mão pode ser permitido com estilo alternativo. Escudo não deve ser a
configuração padrão dessa evolução.

**Mecânica exclusiva --- Fúria**

-   Gera Fúria ao acertar ataques básicos, causar dano e receber dano de
    inimigos.
-   Gera um pouco mais ao acertar múltiplos alvos, com limite por evento
    para evitar multiplicação abusiva.
-   Consome Fúria para habilidades de alto impacto.
-   Fora de combate, Fúria decai gradualmente.
-   O Berserker não deve depender de MP para seu ciclo principal de
    habilidades.

**Ramos:**

  -----------------------------------------------------------------------
  Ramo                    Foco                    Habilidades-chave
                                                  propostas
  ----------------------- ----------------------- -----------------------
  Carnificina             Dano e golpes pesados   Golpe Brutal, Machadada
                                                  Sísmica, Redemoinho,
                                                  Ruptura

  Frenesi                 Velocidade e sequência  Frenesi, Sequência
                                                  Selvagem, Sede de
                                                  Sangue, Fúria
                                                  Incontrolável

  Execução                Finalização e risco     Marca da Presa,
                                                  Execução, Golpe Rubro,
                                                  Carnificina Final
  -----------------------------------------------------------------------

**Habilidade suprema proposta --- Apoteose da Fúria:** consome grande
quantidade de Fúria para uma sequência devastadora. A habilidade deve
ter janela de execução clara e não conceder invulnerabilidade longa.

### 4.4 Bárbaro → Senhor da Guerra

**Papel:** combatente de linha de frente com suporte ofensivo, controle
e presença em grupo.

**Identidade:** lidera a investida, abre espaço para aliados e
transforma força bruta em vantagem coletiva.

**Armas:** machado de uma ou duas mãos; escudo permitido como opção
defensiva, sem ser obrigatório.

**Mecânica exclusiva --- Ímpeto**

-   Gera Ímpeto ao acertar ataques, interromper inimigos, iniciar
    combate e atingir múltiplos alvos.
-   Consome Ímpeto em investidas, brados e habilidades de controle.
-   Habilidades de grupo devem ter recargas relevantes para evitar
    manutenção permanente de bônus fortes.

**Ramos:**

  -----------------------------------------------------------------------
  Ramo                    Foco                    Habilidades-chave
                                                  propostas
  ----------------------- ----------------------- -----------------------
  Comando                 Buffs e liderança       Brado de Guerra,
                                                  Estandarte de Guerra,
                                                  Ordem de Avanço, Grito
                                                  do Caudilho

  Investida               Mobilidade e iniciação  Avanço do Caudilho,
                                                  Quebra-Linha, Impacto,
                                                  Investida Implacável

  Domínio                 Controle e resistência  Rugido, Pisotear,
                                                  Desafio, Presença
                                                  Dominante
  -----------------------------------------------------------------------

**Habilidade suprema proposta --- Estandarte do Conquistador:** finca um
estandarte que fortalece aliados e enfraquece inimigos em uma área. Seu
efeito deve ser forte, mas limitado espacialmente e por duração.

### 4.5 Mago → Elementalista

**Papel:** DPS mágico à distância, dano em área e aplicação de efeitos
elementais.

**Identidade:** alterna fogo e gelo para causar dano, controlar grupos e
explorar vulnerabilidades.

**Armas:** cajado, varinha e grimório como foco secundário. O tipo de
arma deve definir o comportamento do ataque básico.

**Mecânica exclusiva --- Concentração**

-   Gera Concentração ao conjurar habilidades diferentes em sequência e
    acertar ataques básicos mágicos.
-   Conjurações interrompidas ou repetição excessiva da mesma habilidade
    não devem gerar o máximo de recurso.
-   Consome Concentração para aprimorar habilidades elementais.
-   Mana continua sendo o recurso de conjuração; Concentração funciona
    como recurso de especialização, não como substituto da mana.

**Ramos:**

  -----------------------------------------------------------------------
  Ramo                    Foco                    Habilidades-chave
                                                  propostas
  ----------------------- ----------------------- -----------------------
  Chamas                  Dano contínuo e         Bola de Fogo, Brasa
                          explosão                Viva, Chuva de Brasas,
                                                  Inferno

  Geada                   Lentidão e congelamento Lança de Gelo, Nova de
                                                  Gelo, Prisão Glacial,
                                                  Zero Absoluto

  Convergência            Combinação elemental    Reação Elemental,
                                                  Sobrecarga, Cataclismo,
                                                  Ruptura Elemental
  -----------------------------------------------------------------------

**Habilidade suprema proposta --- Cataclismo Elemental:** combina fogo e
gelo em uma área ampla, causando dano e um efeito de controle. O efeito
deve exigir preparação ou Concentração acumulada.

### 4.6 Mago → Arcanista

**Papel:** controle, maldições, dano arcano e manipulação de recursos.

**Identidade:** controla o ritmo da luta com marcas, pragas,
enfraquecimentos e efeitos arcanos.

**Armas:** cajado, varinha e grimório. O grimório deve ser uma opção
visual e funcional de foco secundário, caso o sistema de mão secundária
seja implementado.

**Mecânica exclusiva --- Carga Arcana**

-   Gera Carga Arcana ao aplicar efeitos diferentes no mesmo alvo,
    manter maldições ativas e acertar habilidades arcanas.
-   Consome Carga para ampliar duração, área ou intensidade de efeitos.
-   Não deve gerar Carga ilimitada ao reaplicar a mesma maldição no
    mesmo alvo.

**Ramos:**

  -----------------------------------------------------------------------
  Ramo                    Foco                    Habilidades-chave
                                                  propostas
  ----------------------- ----------------------- -----------------------
  Maldições               Enfraquecimento e       Marca do Abismo, Drenar
                          marcas                  Vontade, Ruína,
                                                  Maldição Persistente

  Pragas                  Dano contínuo e         Peste Sombria, Praga,
                          propagação              Contágio, Miasma

  Arcano                  Controle e manipulação  Piscar, Escudo Arcano,
                                                  Prisão Etérea, Colapso
                                                  Arcano
  -----------------------------------------------------------------------

**Habilidade suprema proposta --- Abismo:** aplica uma grande marca em
uma área. Os inimigos marcados recebem dano ampliado e têm atributos
ofensivos reduzidos por uma janela limitada.

### 4.7 Ladino → Assassino

**Papel:** dano explosivo em alvo prioritário, críticos e mobilidade.

**Identidade:** prepara a oportunidade, aproxima-se e executa. Deve ter
alto potencial contra alvos isolados, mas menor eficiência quando fica
exposto por muito tempo.

**Armas:** duas adagas ou lâminas leves. Armas de uma mão compatíveis
podem ser consideradas, mas a identidade principal é dual wield.

**Mecânica exclusiva --- Combo**

-   Gera Combo ao acertar ataques básicos e habilidades em sequência.
-   Ataques pelas costas, críticos e habilidades de aproximação podem
    gerar Combo adicional.
-   O Combo decai após uma janela sem acertar.
-   Habilidades especiais consomem Combo para aplicar dano ou efeitos
    adicionais.
-   O sistema deve evitar que ataques rápidos gerem recurso em
    quantidade desproporcional.

**Ramos:**

  -----------------------------------------------------------------------
  Ramo                    Foco                    Habilidades-chave
                                                  propostas
  ----------------------- ----------------------- -----------------------
  Lâminas                 Dano direto e críticos  Golpe Furtivo, Lâminas
                                                  Gêmeas, Golpe Fatal,
                                                  Sentença

  Sombras                 Furtividade e           Passo Sombrio, Sumir,
                          mobilidade              Golpe das Sombras, Véu
                                                  Mortal

  Execução                Marcas e finalizações   Marca da Morte, Ponto
                                                  Fraco, Lâmina Oculta,
                                                  Execução Perfeita
  -----------------------------------------------------------------------

**Habilidade suprema proposta --- Sentença:** investe contra o alvo
marcado e aplica uma sequência de golpes. O dano final é ampliado contra
inimigos abaixo de um limite de vida, sem transformar a habilidade em
execução garantida de chefes.

### 4.8 Ladino → Patrulheiro

**Papel:** DPS à distância, perseguição, armadilhas e controle de
espaço.

**Identidade:** mantém distância, prepara o terreno e pune inimigos que
tentam se aproximar ou fugir.

**Armas:** arco e besta. A besta pode ter variantes de uma e duas mãos;
arco usa duas mãos. Adagas podem permanecer como arma secundária se o
sistema permitir, mas não devem substituir o conjunto principal da
evolução.

**Mecânica exclusiva --- Precisão**

-   Gera Precisão ao manter distância adequada, acertar ataques
    consecutivos e atingir pontos vulneráveis.
-   Disparos errados ou ataques interrompidos não geram recurso.
-   Consome Precisão em tiros especiais, armadilhas e disparos
    perfurantes.
-   A geração deve considerar distância e frequência para não premiar
    apenas ataques automáticos sem decisão.

**Ramos:**

  -----------------------------------------------------------------------
  Ramo                    Foco                    Habilidades-chave
                                                  propostas
  ----------------------- ----------------------- -----------------------
  Arqueria                Dano à distância        Tiro Preciso, Disparo
                                                  Perfurante, Chuva de
                                                  Flechas, Flecha Mortal

  Armadilhas              Controle de área        Armadilha de Caça,
                                                  Rede, Bomba de Fumaça,
                                                  Campo de Espinhos

  Caçada                  Marcação e perseguição  Marca da Presa, Faca de
                                                  Arremesso, Passo de
                                                  Caça, Perseguição
  -----------------------------------------------------------------------

**Habilidade suprema proposta --- Chuva de Flechas:** dispara uma
sequência de projéteis em uma área escolhida. A habilidade exige
posicionamento e pode ser interrompida ou evitada conforme as regras de
combate.

------------------------------------------------------------------------

## 5. Sistema de recursos e equilíbrio de mana

O sistema atual apresenta uma diferença importante: o Mago possui
regeneração de mana elevada, enquanto o Bárbaro gasta MP em habilidades
e pode ficar sem recurso para continuar lutando. A reformulação deve
corrigir essa diferença sem tornar todas as classes iguais.

### 5.1 Separar recurso de classe e mana

Proposta:

-   **Mana (MP):** recurso universal para habilidades mágicas,
    utilitárias e algumas habilidades de suporte.
-   **Recurso de classe:** medidor próprio que alimenta o estilo de
    combate da evolução.
-   Habilidades principais de cada evolução devem consumir
    prioritariamente o recurso de classe.
-   Habilidades utilitárias ou de alto impacto podem consumir MP,
    recurso de classe ou ambos, desde que o custo seja coerente.

### 5.2 Recurso por evolução

  -----------------------------------------------------------------------
  Evolução          Recurso           Como gera         Como gasta
  ----------------- ----------------- ----------------- -----------------
  Bastião           Determinação      Bloqueios, dano   Defesa reforçada,
                                      recebido e        provocação e
                                      proteção efetiva  proteção

  Paladino          Fé                Ataques sagrados  Bênçãos, cura
                                      e suporte efetivo limitada e dano
                                                        sagrado

  Berserker         Fúria             Ataques básicos,  Golpes pesados e
                                      dano causado e    execuções
                                      recebido          

  Senhor da Guerra  Ímpeto            Acertos,          Investidas,
                                      interrupções e    brados e controle
                                      iniciações        

  Elementalista     Concentração      Conjurações       Aprimoramento
                                      variadas e        elemental
                                      ataques mágicos   

  Arcanista         Carga Arcana      Aplicação de      Ampliação de
                                      efeitos e         maldições e
                                      habilidades       controle
                                      arcanas           

  Assassino         Combo             Sequências de     Golpes especiais
                                      acertos, críticos e finalizações
                                      e ataques pelas   
                                      costas            

  Patrulheiro       Precisão          Acertos           Disparos
                                      consecutivos e    especiais e
                                      exploração de     armadilhas
                                      distância         
  -----------------------------------------------------------------------

### 5.3 Regras para evitar abuso

-   Toda geração por evento deve ter limite por segundo.
-   Dano em múltiplos alvos não pode multiplicar infinitamente a geração
    do recurso.
-   Invocações, dano refletido, dano em alvos invulneráveis e efeitos
    periódicos devem ter regras específicas para não gerar recurso
    artificialmente.
-   O recurso deve ter um limite máximo claro.
-   A interface deve mostrar geração, consumo, decaimento e condição de
    obtenção.
-   O jogador deve conseguir manter um ciclo básico de combate mesmo
    quando estiver sem recurso especial.
-   O Bárbaro não deve ficar inutilizado por falta de MP.
-   O Mago deve continuar administrando mana: regeneração alta não pode
    eliminar a decisão de quando conjurar.

### 5.4 Regeneração de mana

A regeneração passiva do Mago deve ser reduzida e distribuída entre
decisões de build. A passiva antiga **Mente Clara** pode ser convertida
em melhoria de eficiência de mana, em vez de multiplicar a regeneração
sem limite.

Regras recomendadas:

-   Regeneração natural fora de combate maior que em combate.
-   Habilidades de recuperação devem ter recarga e custo de
    oportunidade.
-   Atributos, equipamentos e talentos podem melhorar eficiência, mas
    devem respeitar um teto.
-   As demais classes também precisam de regeneração suficiente para
    habilidades utilitárias, mas não devem competir com o Mago em
    sustentação de conjuração.

------------------------------------------------------------------------

## 6. Progressão de nível: lançamento 1--60 e expansão até 100

O projeto terá uma progressão final planejada até o nível 100, mas o
primeiro lançamento será limitado ao nível 60.

**Importante:** Ascensão e Maestria são sistemas diferentes. Ascensão
faz parte da progressão de personagem entre os níveis 61 e 100. Maestria
é a progressão de longo prazo que começa depois de alcançar o nível
máximo global.

### 6.1 Fases de progressão

  -----------------------------------------------------------------------
  Níveis                  Fase                    Conteúdo
  ----------------------- ----------------------- -----------------------
  1--19                   Aprendizado da          Três ramos flexíveis e
                          classe-base             fundamentos

  20                      Evolução                Missão A Encruzilhada,
                                                  escolha de uma evolução
                                                  e reset da árvore-base

  21--40                  Formação da evolução    Habilidades centrais,
                                                  passivas e identidade
                                                  da build

  41--60                  Especialização avançada Modificadores,
                                                  sinergias e
                                                  habilidade-chave da
                                                  evolução

  61--80                  Ascensão I              Expansão futura da
                                                  árvore, talentos de
                                                  alto nível e desafios

  81--100                 Ascensão II             Habilidades supremas,
                                                  aprimoramentos finais e
                                                  conclusão da progressão

  Após 100                Maestria                Progressão horizontal e
                                                  objetivos de longo
                                                  prazo
  -----------------------------------------------------------------------

### 6.2 Marcos de desbloqueio

  -----------------------------------------------------------------------
                                     Nível Desbloqueio proposto
  ---------------------------------------- ------------------------------
                                         1 Classe-base e habilidades
                                           iniciais

                                         5 Segundo degrau da árvore-base

                                        10 Terceiro degrau da árvore-base

                                        15 Quarto degrau da árvore-base

                                        20 Evolução, reset e nova árvore

                                        25 Primeiros modificadores de
                                           habilidades da evolução

                                        30 Passivas avançadas e sinergias

                                        35 Habilidades de maior impacto

                                        40 Fechamento da primeira etapa
                                           da evolução

                                        45 Modificadores avançados

                                        50 Habilidades-chave da evolução

                                        55 Aprimoramentos de mecânica

                                        60 Habilidade suprema ou
                                           culminação da árvore
                                           disponível no lançamento

                                    61--80 Ascensão I --- expansão futura

                                   81--100 Ascensão II --- expansão
                                           futura

                                  Após 100 Maestria
  -----------------------------------------------------------------------

Nem todo marco precisa conceder uma habilidade ativa nova. Parte dos
desbloqueios deve melhorar habilidades existentes, alterar
comportamentos ou oferecer escolhas passivas.

### 6.3 Pontos de habilidade

A regra atual é de 1 ponto por nível, com 2 pontos iniciais já aplicados
nas habilidades iniciais. A nova curva precisa ser recalculada porque a
árvore muda no nível 20.

**Regra proposta para implementação:**

-   O jogador recebe pontos de habilidade ao subir de nível.
-   Os pontos investidos na classe-base são devolvidos na evolução.
-   Os pontos devolvidos podem ser usados na árvore da evolução.
-   Não há uma moeda de pontos separada para cada evolução.
-   Missões de classe podem conceder pontos adicionais, desde que o
    total possível seja controlado.
-   A quantidade final de pontos e o custo de cada rank devem ser
    fechados depois que as árvores definitivas forem desenhadas.

Não fixar a quantidade total de pontos antes de fechar o número de nós e
ranks. A árvore deve ser balanceada para que o jogador faça escolhas
reais até o nível 60, sem conseguir maximizar todos os ramos.

### 6.4 Conteúdo do lançamento nível 60

O lançamento inicial deve conter:

-   Quatro classes-base completas até o nível 19.
-   Oito evoluções jogáveis.
-   Árvores avançadas das oito evoluções até o nível 60.
-   Mecânicas exclusivas e interface de recursos.
-   Armas e animações necessárias para cada evolução.
-   Equipamentos e progressão de atributos ajustados ao novo sistema.
-   Missão de evolução no nível 20.
-   Regras de redistribuição e troca de evolução.
-   Balanceamento PvE, chefes e PvP dentro do conteúdo disponível.

A estrutura de Ascensão 61--100 pode ser preparada nos dados e na
arquitetura, mas deve permanecer bloqueada até a expansão
correspondente.

------------------------------------------------------------------------

## 7. Maestria --- progressão após o nível máximo

A Maestria deve começar apenas depois de o personagem alcançar o nível
máximo global (nível 100, quando a progressão completa estiver lançada).
Ela não substitui a Ascensão.

### 7.1 Objetivo

A Maestria é uma progressão horizontal: oferece conveniência, coleta,
sobrevivência e pequenas melhorias de combate, sem criar uma distância
excessiva entre personagens.

### 7.2 Estrutura proposta

Manter a estrutura de referência atual como ponto de partida:

-   Uma árvore de Maestria compartilhada entre todas as classes.
-   Três ramos: Caça, Sobrevivência e Utilidade.
-   Quatro nós por ramo.
-   Cada nó aceita até cinco pontos.
-   Total de 12 nós e 60 pontos para completar a árvore.
-   O primeiro ponto investido em um nó tem maior retorno; os pontos
    seguintes apresentam retornos decrescentes.
-   O ganho total de combate da árvore inteira deve permanecer limitado
    a 5%.

### 7.3 Ramos

**Caça** - Alquimia de Campo: aumenta a cura recebida de poções. - Bolsa
Farta: aumenta moedas obtidas de monstros. - Olho do Coletor: chance de
obter material adicional de monstros comuns. - Instinto do Caçador:
pequeno bônus de dano, sujeito ao teto global de combate.

**Sobrevivência** - Firmeza: reduz a duração de atordoamento e lentidão
recebidos. - Segunda Chance: reduz a perda de XP de Maestria ao
morrer. - Fôlego: aumenta regeneração de vida e mana fora de combate. -
Couro Curtido: pequeno bônus de vida máxima, sujeito ao teto global de
combate.

**Utilidade** - Passo Leve: aumenta velocidade fora de combate, com teto
próprio. - Viajante: reduz o preço de Pedras de Teleporte. - Leitura
Rápida: reduz o tempo de uso de Pedra de Teleporte e Pergaminho de
Retorno. - Caminho de Casa: reduz a recarga do Pergaminho de Retorno.

### 7.4 Ganho de pontos

A referência atual usa uma barra de XP de Maestria, em que uma barra
cheia concede um ponto. O tamanho da barra é equivalente ao custo de XP
do último nível da progressão vigente.

Para a nova versão:

-   A barra de Maestria começa a receber XP ao atingir o nível 100.
-   Cada barra completa concede um ponto.
-   O XP excedente ao alcançar o nível 100 deve ser transferido para a
    barra.
-   O total de pontos permanece limitado à quantidade necessária para
    completar os 12 nós.
-   O valor exato de XP por barra deve ser recalculado com a curva de
    experiência do nível 99 para 100.

### 7.5 Bônus diário, morte e redistribuição

A referência atual possui as seguintes regras, que podem ser mantidas
após validação:

-   Primeira barra do dia com progresso quatro vezes mais rápido.
-   O dia vira à meia-noite de Brasília.
-   Bônus diário acumulável por até sete dias sem jogar.
-   Morte remove uma porcentagem da barra atual, sem remover pontos já
    conquistados.
-   Personagens com karma sofrem uma penalidade maior.
-   O nó Segunda Chance reduz a perda.
-   Redistribuição gratuita de todos os pontos uma vez por dia.

Os percentuais atuais foram desenhados para o nível máximo 30 e precisam
ser reavaliados para a curva de XP do nível 100. Não transportar
automaticamente o valor de XP atual de 263.873.

------------------------------------------------------------------------

## 8. Sistema de armas

### 8.1 Princípio

A arma deve definir como o personagem executa o ataque básico. Hoje, o
tipo de arma é derivado da classe e a arma altera principalmente
atributos e visual. No novo sistema, o tipo de arma deve ser uma
propriedade explícita do item.

A arma deve controlar:

-   Alcance do ataque básico.
-   Intervalo ou velocidade de ataque.
-   Tipo de dano do ataque básico.
-   Animação de ataque.
-   Projétil, quando aplicável.
-   Requisitos de classe/evolução.
-   Empunhadura (uma mão, duas mãos ou dupla).
-   Atributos e efeitos próprios do item.

As habilidades podem ser livres, exigir determinado tipo de arma ou
receber bônus com a arma adequada. Cada habilidade deve declarar
claramente sua regra.

### 8.2 Tipos de arma

  -----------------------------------------------------------------------
  Tipo              Alcance e estilo  Empunhadura       Situação dos
                                                        assets
  ----------------- ----------------- ----------------- -----------------
  Espada            Corpo a corpo,    Uma ou duas mãos  Já existe no jogo
                    equilibrada                         

  Machado           Corpo a corpo,    Uma ou duas mãos  Já existe no jogo
                    golpes pesados                      

  Adagas            Corpo a corpo,    Duas armas        Já existe no jogo
                    ataques rápidos                     

  Escudo            Defesa e bloqueio Mão secundária    Já existe no jogo

  Cajado            Ataque mágico à   Duas mãos         Já existe no jogo
                    distância                           

  Varinha           Ataque mágico,    Uma mão           Já existe no jogo
                    ritmo mais ágil                     

  Arco              Ataque físico à   Duas mãos         Modelo e
                    distância                           animações
                                                        disponíveis nos
                                                        assets-fonte

  Besta             Disparo à         Uma ou duas mãos  Modelo e
                    distância, ritmo                    animações
                    de recarga                          disponíveis nos
                                                        assets-fonte

  Grimório          Foco mágico       Mão secundária    Modelo
                    secundário                          disponível;
                                                        integração
                                                        funcional a
                                                        definir
  -----------------------------------------------------------------------

### 8.3 Matriz de armas por evolução

  -----------------------------------------------------------------------
  Evolução                Armas principais        Secundárias / opções
  ----------------------- ----------------------- -----------------------
  Bastião                 Espada de uma mão       Escudo

  Paladino                Espada de uma mão       Escudo; foco sagrado
                                                  futuro

  Berserker               Machado de duas mãos    Machado de uma mão

  Senhor da Guerra        Machado de uma ou duas  Escudo opcional
                          mãos                    

  Elementalista           Cajado ou varinha       Grimório como foco
                                                  secundário

  Arcanista               Cajado ou varinha       Grimório como foco
                                                  secundário

  Assassino               Duas adagas             Lâminas leves
                                                  compatíveis

  Patrulheiro             Arco ou besta           Adagas secundárias
                                                  futuras, se suportadas
  -----------------------------------------------------------------------

Essa matriz é uma regra de design proposta. A lista final de armas
permitidas deve ser declarada na definição da evolução e validada no
servidor.

### 8.4 Diferença entre armas

As armas não devem ser apenas skins com atributos diferentes. Cada tipo
deve ter vantagens e desvantagens próprias.

  Tipo       Característica sugerida
  ---------- ----------------------------------------------------
  Espada     Equilíbrio entre velocidade, alcance e dano
  Machado    Ataque mais lento e impacto maior
  Adagas     Ataque rápido, maior sinergia com críticos e combo
  Escudo     Bloqueio, defesa e interação com Determinação
  Cajado     Ataque mágico de maior alcance e impacto
  Varinha    Ataque mágico mais ágil, menor impacto individual
  Arco       Disparos contínuos e precisão
  Besta      Disparos fortes, cadência menor e recarga
  Grimório   Foco secundário com atributos ou efeitos mágicos

Os valores exatos de alcance, velocidade e dano devem ser definidos por
tipo de arma e testados em conjunto com as habilidades.

### 8.5 Assets e animações

Os pacotes KayKit já possuem modelos de arco, flechas, aljava, bestas de
uma e duas mãos e grimório. Também existem animações de combate à
distância:

-   Arco: `Ranged_Bow_Draw`, `Ranged_Bow_Release`,
    `Ranged_Bow_Aiming_Idle`, `Ranged_Bow_Idle`, `Ranged_Bow_Draw_Up`,
    `Ranged_Bow_Release_Up`.
-   Besta de uma mão: `Ranged_1H_Aiming`, `Ranged_1H_Shoot`,
    `Ranged_1H_Shooting`, `Ranged_1H_Reload`.
-   Besta de duas mãos: `Ranged_2H_Aiming`, `Ranged_2H_Shoot`,
    `Ranged_2H_Shooting`, `Ranged_2H_Reload`.

As animações de arco e besta ainda precisam ser exportadas para o
arquivo publicado `anims-combatranged.glb`. Atualmente, esse arquivo
contém apenas as animações de magia ranged.

### 8.6 Mudanças técnicas necessárias

1.  Declarar `weaponType` diretamente em cada item.
2.  Substituir a validação baseada apenas na classe por uma lista de
    tipos permitidos por classe/evolução.
3.  Mover alcance, intervalo de ataque e tipo de dano para a definição
    da arma.
4.  Selecionar animações pelo tipo de arma, não somente pela classe.
5.  Fazer o ataque básico e o projétil respeitarem o tipo de arma.
6.  Declarar requisitos de arma nas habilidades que precisarem deles.
7.  Revisar atributos-base das classes, pois os valores atuais foram
    balanceados considerando uma arma fixa.
8.  Garantir que servidor e cliente validem a mesma regra de
    equipamento.

------------------------------------------------------------------------

## 9. Efeitos e linguagem comum de combate

As habilidades das classes devem reutilizar efeitos compartilhados
sempre que possível. Isso facilita balanceamento, interface, logs e
consistência entre PvE e PvP.

### 9.1 Efeitos

  -----------------------------------------------------------------------
  Efeito                              Comportamento
  ----------------------------------- -----------------------------------
  Atordoar                            O alvo não age nem anda

  Prender                             O alvo não anda, mas ainda pode
                                      atacar e usar habilidades

  Lentidão                            Reduz velocidade em uma porcentagem

  Provocação                          Monstros são obrigados a atacar
                                      quem provocou

  Cegueira                            Aumenta a chance de o alvo errar

  Enfraquecer                         Reduz o ataque do alvo

  Marca                               Aumenta o dano recebido de todas as
                                      fontes

  Veneno                              Causa dano periódico

  Queimadura                          Causa dano periódico

  Praga                               Causa dano periódico e pode se
                                      propagar ao morrer o alvo infectado

  Escudo                              Absorve uma quantidade fixa de dano

  Escudo de mana                      O dano recebido é descontado da
                                      mana em vez da vida

  Imortal                             A vida não pode cair abaixo de 1
                                      durante o efeito

  Invisível                           Monstros não detectam o personagem;
                                      atacar revela e habilita o próximo
                                      golpe crítico conforme a regra do
                                      efeito

  Armadura de Gelo                    Aumenta defesa e aplica lentidão a
                                      atacantes próximos

  Buffs                               Aumentam ataque, defesa,
                                      velocidade, velocidade de ataque ou
                                      concedem efeitos de grupo
  -----------------------------------------------------------------------

### 9.2 Modificadores de golpe

  -----------------------------------------------------------------------
  Modificador                         Comportamento
  ----------------------------------- -----------------------------------
  Golpes múltiplos                    A habilidade acerta várias vezes em
                                      sequência

  Não erra                            Ignora o teste de precisão contra
                                      esquiva

  Crítico extra                       Soma chance de crítico apenas
                                      naquela habilidade

  Execução                            Multiplica dano em alvos abaixo de
                                      um percentual de vida

  Investida                           Move o personagem até o alvo antes
                                      de atacar

  Piscar                              Teleporta o personagem para a
                                      frente

  Área no alvo                        Atinge inimigos próximos do alvo
                                      principal

  Área em volta de si                 Atinge inimigos próximos do
                                      lançador

  Aliados                             Aplica buffs com raio também aos
                                      aliados próximos

  Estandarte                          Cria um objeto no chão que concede
                                      bônus a quem estiver na área
  -----------------------------------------------------------------------

------------------------------------------------------------------------

## 10. Balanceamento entre PvE, chefes e PvP

Toda habilidade e mecânica precisa ser avaliada em três contextos:

### PvE comum

-   Evitar que habilidades em área gerem recurso sem limite.
-   Garantir que cada classe tenha ferramentas para lidar com grupos.
-   Evitar que controle impeça monstros comuns de agir permanentemente.

### Chefes e raids

-   Chefes devem ter regras claras para imunidade ou redução de
    controle.
-   Efeitos de execução devem respeitar limites específicos para chefes.
-   Recursos não podem ser acumulados artificialmente em fases
    invulneráveis.
-   Buffs de grupo devem ter duração, alcance e recarga controlados.

### PvP

-   Controle em cadeia deve ter redução de duração ou janela de
    resistência.
-   Invisibilidade, investidas e habilidades que não erram precisam de
    contrajogo.
-   Dano explosivo deve ter sinais visuais e janela de resposta.
-   Bônus de grupo devem ser avaliados considerando grupos coordenados.
-   A Maestria deve continuar com impacto de combate limitado.

------------------------------------------------------------------------

## 11. Arquitetura de dados recomendada

Separar definições de classe, evolução, arma, habilidade e progressão
evita que regras fiquem acopladas à classe-base.

  -----------------------------------------------------------------------
  Entidade                            Responsabilidade
  ----------------------------------- -----------------------------------
  `ClassDefinition`                   Identidade, atributos-base e
                                      descrição da classe-base

  `ClassEvolution`                    Evolução escolhida, atributos,
                                      mecânica e armas permitidas

  `SkillTree`                         Ramos, nós, ordem e pré-requisitos

  `SkillDefinition`                   Efeito, alvo, alcance, custo,
                                      recarga e modificadores

  `SkillRank`                         Valores de cada rank da habilidade

  `ClassMechanic`                     Regras de geração, consumo, limite
                                      e decaimento do recurso

  `WeaponDefinition`                  Tipo, empunhadura, alcance,
                                      velocidade, dano e animação

  `AscensionDefinition`               Conteúdo futuro dos níveis 61--100

  `MasteryDefinition`                 Árvore horizontal após o nível
                                      máximo
  -----------------------------------------------------------------------

### 11.1 Regras que devem ser validadas no servidor

-   Evolução ativa do personagem.
-   Pré-requisitos e pontos da árvore.
-   Custo e recarga das habilidades.
-   Geração e consumo do recurso de classe.
-   Tipo de arma permitido.
-   Alcance e intervalo do ataque básico.
-   Aplicação de efeitos e limites de controle.
-   XP de nível, Ascensão e Maestria.
-   Redistribuição e troca de evolução.

O cliente pode exibir informações e animações, mas não deve ser a
autoridade para validar dano, recursos, equipamentos ou pontos de
habilidade.

------------------------------------------------------------------------

## 12. Plano de implementação sugerido

### Etapa 1 --- Fundamentos

-   Definir o modelo de evolução e reset no nível 20.
-   Definir a regra final de pontos de habilidade.
-   Separar classe-base de evolução.
-   Criar definições explícitas de armas.

### Etapa 2 --- Classes e recursos

-   Implementar as oito evoluções.
-   Implementar as oito mecânicas de recurso.
-   Ajustar custos de MP e regeneração.
-   Implementar as árvores até o nível 60.

### Etapa 3 --- Armas e animações

-   Migrar o tipo de arma para a definição do item.
-   Implementar matriz de armas por evolução.
-   Exportar animações de arco e besta.
-   Ajustar ataque básico, projéteis e animações.

### Etapa 4 --- Balanceamento

-   Testar cada evolução individualmente.
-   Testar combinações de grupo.
-   Testar chefes e raids.
-   Testar PvP e controle em cadeia.
-   Ajustar progressão de atributos e equipamentos.

### Etapa 5 --- Expansão

-   Liberar Ascensão 61--100.
-   Recalcular XP e pontos de progressão.
-   Liberar Maestria após o nível 100.
-   Ajustar XP por barra, bônus diário e penalidade por morte.

------------------------------------------------------------------------

## 13. Decisões ainda pendentes

Os pontos abaixo precisam de decisão antes de fechar os números e
iniciar a implementação definitiva:

1.  Custo e condição para trocar de evolução depois do nível 20.
2.  Quantidade exata de pontos recebidos por nível.
3.  Quantidade de pontos extras concedidos por missões de classe.
4.  Número final de nós e ranks em cada árvore evoluída.
5.  Valores máximos, geração e decaimento dos oito recursos.
6.  Quais habilidades consomem MP, recurso de classe ou ambos.
7.  Valores de alcance, velocidade e dano de cada tipo de arma.
8.  Se o Paladino terá algum recurso de cura direta ou somente mitigação
    e suporte.
9.  Se o Senhor da Guerra poderá usar escudo sem perder acesso a
    habilidades centrais.
10. Se o grimório será apenas foco secundário ou terá regras próprias de
    combate.
11. Como chefes resistirão a atordoamento, prender, cegueira e execução.
12. Quais regras da Maestria atual serão mantidas após a progressão até
    o nível 100.

------------------------------------------------------------------------

## 14. Referências do sistema atual

Este documento combina a direção de design proposta com informações do
sistema existente. Os dados abaixo são referências do estado atual e não
devem ser confundidos com valores finais da reformulação.

-   **Classes e habilidades atuais:** atributos, ramos
    Guardião/Lâmina/Comando, Massacre/Sangue/Selvagem, Fogo/Gelo/Arcano
    e Assassinato/Sombras/Venenos; habilidades e especializações
    antigas.
-   **Maestria atual:** árvore compartilhada de três ramos, 12 nós,
    cinco ranks por nó, 60 pontos, limite de 5% para bônus de combate;
    atualmente abre no nível 30.
-   **Armas atuais:** espada exclusiva do Cavaleiro, machado do Bárbaro,
    cajado/varinha do Mago, adagas do Ladino e escudos para
    Cavaleiro/Bárbaro. Arco, besta e grimório possuem assets
    disponíveis, mas exigem integração.

**Fontes técnicas atuais:** - `server/internal/game/classes.go` -
`server/internal/game/skills.go` - `server/internal/game/specs.go` -
`server/internal/game/spec_skills.go` -
`server/internal/game/mastery.go` - `server/internal/zone/mastery.go` -
`server/internal/game/items.go` - `server/internal/zone/inventory.go` -
`client/src/world/models.ts` - `client/public/models/weapons/` -
`assets-src/kaykit/x/`

------------------------------------------------------------------------

## 15. Resumo da direção final

-   Quatro classes-base, com três ramos flexíveis até o nível 19.
-   No nível 20, o jogador escolhe uma entre duas evoluções.
-   A evolução reinicia completamente a árvore-base e devolve os pontos
    investidos.
-   O personagem passa a usar uma árvore exclusiva e uma mecânica
    própria da evolução.
-   Oito evoluções no total: Bastião, Paladino, Berserker, Senhor da
    Guerra, Elementalista, Arcanista, Assassino e Patrulheiro.
-   Lançamento inicial até o nível 60.
-   Ascensão entre os níveis 61 e 100 em uma expansão futura.
-   Maestria somente após o nível 100, como progressão horizontal.
-   Armas definem o ataque básico, alcance, velocidade e animações.
-   Recursos de classe reduzem a dependência de MP e dão identidade
    mecânica a cada evolução.
-   Números finais de dano, custo, recarga, XP e pontos dependem de
    testes e balanceamento.

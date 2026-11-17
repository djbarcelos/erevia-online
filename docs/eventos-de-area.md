# Eventos de área com etapas (T17, tunnel#142)

Um evento de área é um ataque com etapas num círculo do mapa: anuncia, passa pelas etapas
(cada uma com limite de tempo), paga por participação e, se falhar, derruba um posto avançado
por um tempo. Os dados ficam em `server/internal/game/events.go` (`EventDef`) e o motor em
`server/internal/zone/areaevent.go`. O primeiro evento é "O Bastião resiste" (`bastiao_resiste`).
"Acender o Farol" (`acender_farol`, Costa) já tem o esqueleto, mas está **desligado**
(`Enabled: false`) até a Costa entrar no mapa (T19).

A invasão noturna da cidade (`zone/invasion.go`) e a XP em dobro (`zone/events.go`) continuam
sistemas à parte. A invasão (#77) deve ser reescrita em cima de um `EventDef` depois, com este
guia como base: etapas de "matar ondas" e um chefe, e a recompensa por faixa no lugar dos pontos.

## Como criar um EventDef

1. **Dados.** Acrescente o `*EventDef` a `game.AreaEvents` (em `events.go`). `init` valida tudo
   (`ValidateEvent`) e a partida não sobe com dado quebrado.

   | Campo | O que é |
   |---|---|
   | `ID`, `Name` | id estável (usado em `/evento area <id>`) e o nome que aparece no painel e nos anúncios |
   | `Enabled` | `false` = desligado: nenhum gatilho dispara e o GM também não consegue ligar |
   | `Map` | mapa do evento (`""` = principal). Só a zona desse mapa roda o evento |
   | `X`, `Z`, `Radius` | o círculo: a presença conta dentro dele e os monstros nascem na borda |
   | `Place`, `Level`, `MinLevel` | complemento dos anúncios ("no Bastião Cinzento"), nível dos monstros e nível mínimo para receber |
   | `Trigger` | o gatilho (abaixo) |
   | `Stages` | as etapas, em ordem (abaixo) |
   | `Reward` | a recompensa por participação (abaixo) |
   | `Post`, `CloseSecs` | o posto que cai se o evento falhar, e por quantos segundos (`""` = nenhum) |

2. **Gatilho** (`TriggerDef`), sempre com `Cooldown` (segundos entre o fim de um e o começo do
   próximo) e `Warn` (segundos entre o anúncio e a primeira etapa; 0 = já começa):
   - `TriggerNight`: a cada noite que começa, com `Chance`;
   - `TriggerWindow`: a cada `Window` segundos, com `Chance`;
   - `TriggerFixed`: todo dia em `Hour:Minute` (fuso dos eventos, Brasília).

   O gatilho só vale com jogador na zona e nenhum evento em andamento. O GM dispara com
   `/evento area <id> [agora|parar]` e reabre um posto com `/evento area reabrir <posto>`;
   `/evento area` lista o estado.

3. **Etapas** (`StageDef`). Toda etapa tem `Kind`, `Label` (o que o painel mostra), `Say` (o
   anúncio ao começar) e `Limit` (segundos; estourou, o evento falha):
   - `StageKill`: mate `Count` monstros de `Mobs`, em levas de 2 a cada `Every` s, com no máximo
     `MaxAlive` vivos;
   - `StageDefend`: segure o estandarte por `Hold` s (menor que `Limit`). Os monstros de `Mobs`
     marcham até ele e cada um perto tira `DrainRate` de vida por segundo do estandarte
     (`TargetHP`). Se a vida zerar, falha;
   - `StageBoss`: mate `Boss` (um monstro de elite com a vida × `BossHPMult`). O `FieldBoss` de
     `game/fieldboss.go` não serve aqui: ele renasce por janela de relógio, salva o horário no
     banco e deixa túmulo, e o evento precisa de um chefe que nasce e morre com a etapa.

   Os monstros do evento são invocações marcadas (`monsterState.evMob`): sempre agressivos,
   marcham ao ponto do evento, não deixam saque (a recompensa sai no fim) e somam o dano de cada
   jogador.

4. **Recompensa** (`RewardDef`). A contribuição de cada jogador soma o **dano** nos monstros do
   evento, a **cura** que ele fez (vida roubada) e a **presença** no círculo
   (`PresencePerSec` por segundo). As faixas são frações da vida total dos monstros que nasceram:
   `GoldAt` (100%), `SilverAt` (85%) e `BronzeAt` (75%); abaixo de `BronzeAt`, nada.
   - `Medals`: medalhas por faixa (ouro, prata, bronze), do item `Item`;
   - moedas = `GoldBase + GoldPerLvl × nível` e XP = `XPFrac` da barra do nível, ambos × a fração da faixa;
   - na falha, quem chegou a uma faixa leva `FailMult` das moedas e da XP, sem medalha.

5. **Posto que cai.** Declare o `PostDef` em `game.Posts`: a chave é a da pedra de teleporte
   (`zone/teleport.go`), mais o NPC de história (`Talker`) e o mercador (`Merchant`, que a zona
   cria numa faixa de ids própria, `postIDBase`). Com o posto fechado, o servidor recusa o
   teleporte (da pedra e para a pedra, inclusive vindo de outro mapa), a loja e a conversa do
   Irmão, cada um com uma mensagem em pt-BR. O posto reabre sozinho depois de `CloseSecs`.

6. **Cliente.** Nada a fazer para um evento novo: o painel do HUD (`AreaEventStatus`, oneof 49)
   mostra nome, etapa, tempo e progresso, os anúncios usam o banner de sempre, e os postos
   fechados aparecem no painel, na janela da pedra e na loja. O anúncio e o painel vão só para a
   zona (o mapa) do evento.

7. **Testes.** `zone/areaevent_test.go` mostra como montar um evento curto com `z.now` e `z.rng`
   e conferir etapas, faixas, queda e reabertura do posto. Nunca use `time.Now` nem `math/rand`.

O estado do evento e dos postos fechados é de memória: reiniciar a zona apaga o evento e reabre
os postos.

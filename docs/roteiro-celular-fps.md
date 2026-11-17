# Roteiro: medir o FPS no celular (tunnel#64)

Um passo a passo para o Daniel rodar a medição de FPS num aparelho de verdade e mandar o resultado. O que cada número quer dizer e o que já foi medido no computador está em [desempenho-remaster.md](desempenho-remaster.md).

Leva uns 10 minutos por rodada (3 minutos de passeio da câmera + a entrada no jogo). Faça as três rodadas na ordem.

## Antes de começar

- Aparelho **carregando ou com mais de 50% de bateria**, **economia de energia desligada** (ela trava o aparelho em metade da velocidade e estraga a medição).
- Wi-Fi bom (a primeira rodada baixa uns 40 MB; o tempo de download é medido à parte).
- Chrome (Android) ou Safari (iPhone), **só com a aba do jogo aberta**; feche os outros aplicativos.
- Entre com a conta de teste e um personagem qualquer, **na praça** (o passeio da câmera é automático e o personagem não anda).
- Deixe a tela ligada o tempo todo e o celular parado, de preferência deitado numa mesa (ele esquenta; segurar na mão com o polegar em cima do chip muda o resultado).

## O que abrir

Troque `SEU-ENDERECO` pelo endereço do jogo (o mesmo que você abre para jogar):

| Rodada | Endereço | Para quê |
|---|---|---|
| 1 | `https://SEU-ENDERECO/?bench=1&remaster=1&quality=baixa` | o remaster no preset mais leve (o que o aparelho médio deve usar) |
| 2 | `https://SEU-ENDERECO/?bench=1&remaster=1&quality=media` | o remaster no preset médio |
| 3 | `https://SEU-ENDERECO/?bench=1&quality=baixa` | o jogo de hoje (sem `remaster`), para comparar |

(`quality=alta` é opcional: só vale num aparelho topo de linha.)

Como abrir: cole o endereço com `?bench=1...` na barra do navegador, entre no mundo normalmente e **não toque em nada**. Uma caixa preta aparece em cima da tela com "Aquecendo..." e depois "Medindo FPS n/36...". A câmera passa pelas 10 regiões, de dia e de noite. Dura uns 3 minutos.

## O que anotar

Antes de começar cada rodada, anote:

1. **Modelo do aparelho** e versão do Android/iOS (ex.: Galaxy A54, Android 14).
2. **Navegador e versão** (Chrome > menu > Configurações > Sobre o Chrome).
3. Se o aparelho estava **carregando** e a **bateria** (%).

Durante a rodada, anote em uma linha, com a sua impressão (não precisa de número):

4. Travou, engasgou ou ficou "pesado" em algum lugar? Em qual região (dá para ver o nome na caixa preta)?
5. O aparelho **esquentou** muito? O FPS caiu no fim?
6. A imagem ficou diferente do jogo de hoje (rodada 1 e 2 contra a 3)?

## O que me mandar

Quando a caixa mostrar a tabela:

1. Toque em **Copiar** (na própria caixa) e cole numa mensagem para mim. A tabela é longa, mas é ela que eu uso: tem FPS, tempo de frame, tempo de GPU, draw calls e triângulos de cada região. Se o botão não funcionar, tire **dois prints** (um da parte de cima, um da de baixo) rolando a caixa.
2. As anotações de cima (modelo, navegador, bateria e as três impressões).
3. Se o jogo fechar, recarregar sozinho ou a caixa nunca terminar, **mande um print do que estava na tela** e o horário: isso também é resultado.

Se a **rodada 1** der FPS médio acima de 30 e nenhuma região abaixo de 20, o remaster está dentro do que se espera para um celular médio; se não, me mande tudo para eu achar o gargalo (o relatório diz se é CPU, GPU ou animação).

## Se o `?bench=1` não abrir no celular

- O endereço precisa ser o do jogo com login (o de teste com `DEV_LOGIN` só existe no computador).
- Se cair na tela de login, entre e **depois** troque a barra de endereços para o endereço com `?bench=1&...`.
- No iPhone, abra pelo Safari mesmo (não pelo navegador embutido de outro aplicativo).

## Para quem for repetir no computador

Medir com a GPU de verdade (placa NVIDIA ou Intel) e a CPU mais lenta, sem celular: ver "Como medir" em [desempenho-remaster.md](desempenho-remaster.md).

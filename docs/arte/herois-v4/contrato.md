# Contrato do personagem (heróis v4)

O que todo arquivo de herói v4 cumpre, do Blender ao jogo. O cliente é escrito para este contrato, e o modelo se adapta a ele. Mudar o contrato depois da Fase 2 obriga a refazer o que já foi feito; por isso ele é aprovado antes.

Parte do [plano dos heróis v4](../plano-herois-v4.md). Fichas de movimento em [movimento.md](movimento.md), kits em [kits.md](kits.md).

---

## 1. Corpo

| Item | Valor |
|---|---|
| Malhas | **uma** malha de corpo para todos, sem roupa de baixo à mostra além do "corpo cru" (calção e faixa) |
| Gênero | shape key **`feminino`** (0 a 1) na mesma malha: mesma topologia, mesmo UV. Masculino = 0 |
| Altura | 1,80 m (masculino). O feminino é o mesmo esqueleto com **escala uniforme 0,945** (1,70 m) no nó raiz, mais o shape key para a forma |
| Proporção | 7 cabeças. Mãos e pés 10–15% maiores que o real; ombros largos (guia, seção 2) |
| Pose de repouso | A-pose (braços a 45°), frente para +Z, pés em y = 0, quadril em ~1,00 m |
| Cabeça | malha separada (encaixada no pescoço por pesos), 3 rostos por gênero |
| Cabelo | malha separada, 4 cortes por gênero, presa ao osso `head` |
| Triângulos | corpo ≤ 2.500 (o resto do orçamento é da armadura) |

**Por que escala e não dois esqueletos:** com um esqueleto só, todo clipe serve aos dois gêneros sem retargeting. A diferença de forma (ombro, quadril, peito, cintura) fica no shape key; as juntas ficam no mesmo lugar relativo. As peças de armadura recebem o mesmo shape key por deformação de superfície no Blender.

---

## 2. Esqueleto

Um só para todos os heróis. Nomes em minúsculas, com `_l` e `_r` para os lados.

### Ossos que deformam (44 de 45)

| Grupo | Ossos | Qtd. |
|---|---|---|
| Tronco | `pelvis`, `spine_01`, `spine_02`, `chest`, `neck`, `head` | 6 |
| Braços (x2) | `clavicle`, `upperarm`, `upperarm_twist`, `forearm`, `hand` | 10 |
| Dedos (x2) | `thumb_01`, `thumb_02`, `thumb_03`, `index_01`, `index_02`, `index_03`, `fingers_01`, `fingers_02` (médio, anelar e mínimo juntos) | 16 |
| Pernas (x2) | `thigh`, `calf`, `foot`, `toe` | 8 |
| Capa e tecido | `cape_l_01`, `cape_l_02`, `cape_r_01`, `cape_r_02` | 4 |
| **Total** | | **44** |

- `root` fica no chão, entre os pés, e **não deforma**. É o pai de `pelvis` e o nó da escala do feminino.
- A capa, a saia do manto, o tabardo e a barra do casaco usam os 4 ossos de tecido. Peças que não têm tecido solto ignoram esses ossos.
- Sem osso de mandíbula, olhos ou rosto: a 18 m de câmera, não se veem.

### Encaixes (não deformam)

Ossos-filho sem peso, exportados como nós. O cliente prende a arma e a peça no encaixe, sem calibrar à mão.

| Encaixe | Pai | Para quê | Convenção |
|---|---|---|---|
| `socket_hand_r` | `hand_r` | arma da mão direita | origem no centro do punho fechado; +Y ao longo do cabo, para a ponta da lâmina |
| `socket_hand_l` | `hand_l` | arma da mão esquerda, grimório, arco | idem |
| `socket_shield` | `forearm_l` | escudo | origem no centro do antebraço; +Z para fora (a face do escudo) |
| `socket_back` | `chest` | arma guardada nas costas | — |
| `socket_hip_l`, `socket_hip_r` | `pelvis` | adaga e aljava na cintura | — |
| `socket_head` | `head` | elmo, chapéu, capuz rígido | origem no topo do crânio |
| `socket_chest` | `chest` | efeito de buff e aura | centro do peito |

**Arma de uma mão e arco:** o próprio encaixe resolve. **Arma de duas mãos:** o cabo vai de `socket_hand_l` a `socket_hand_r`, e os clipes de duas mãos são animados com a mão esquerda presa no cabo por IK. O cabo nunca "não alcança": a animação garante que as mãos fiquem na distância do cabo.

### Mão

A mão fechada é uma **pose**, não uma malha dobrada: os dedos fecham pelos ossos em todos os clipes que seguram arma. Há 3 poses de mão de referência no arquivo-fonte (aberta, punho no cabo, mão de feitiço), e cada clipe usa a que precisa.

---

## 3. Malhas de armadura

| Slot | Exemplos | Triângulos (LOD0) |
|---|---|---|
| `peito` | peitoral, gibão, manto (tronco) | ≤ 1.600 |
| `ombros` | ombreiras, gola alta, peles | ≤ 900 |
| `bracos` | braçais e luvas | ≤ 700 |
| `cintura` | cinto, saiote, tabardo | ≤ 600 |
| `pernas` | perneiras, calça, barra do manto | ≤ 900 |
| `botas` | botas e grevas | ≤ 500 |
| `capa` | capa, manto das costas, tiras | ≤ 700 |
| `cabeca` | elmo, capuz, chapéu (no `socket_head`, rígido, exceto capuz de pano) | ≤ 800 |
| `acessorio` | corrente, livro, aljava, estandarte, crânio | ≤ 500 |

- **Total do personagem vestido:** ≤ 8.000 triângulos (corpo, cabeça, cabelo e todas as peças).
- Cada peça é feita sobre o corpo masculino, recebe o shape key `feminino` e os pesos do corpo (transferência de pesos + ajuste à mão nas juntas).
- Peça que cobre o corpo **esconde a parte do corpo por baixo** (máscara de ocultação por região: tronco, braços, pernas, pés, cabeça), para não atravessar e para economizar triângulos.
- Cabelo: o elmo e o capuz escondem o cabelo inteiro; o chapéu esconde só o topo (`cabelo_curto` como substituto).

### LOD

| | LOD0 | LOD1 (≥ 20 m) | LOD2 (≥ 35 m) |
|---|---|---|---|
| Personagem vestido | ≤ 20.000 | ≤ 6.000 | ≤ 2.000 |
| Dedos | os 16 ossos | punho fixo | punho fixo |

Nome das malhas: `<nome>`, `<nome>_LOD1`, `<nome>_LOD2`.

---

## 4. Textura e material

- **Um atlas de 2048 px** por kit (KTX2 ETC1S; com 1024 a cota de malha e as placas borravam) (corpo, cabeça e cabelo num atlas próprio de 1024 px, compartilhado por todos os kits).
- KTX2 no jogo, PNG no arquivo-fonte.
- **Um material** + **um emissivo** (runas, magia e o brilho da raridade alta).
- **Máscara de repintura** (RGBA, 512 px):
  - R: pano (cor escolhida pelo jogador);
  - G: pele (tom de pele);
  - B: cabelo (cor do cabelo);
  - A: **metal da raridade** (onde o acabamento muda de comum a lendário).
- Pintura à mão com luz na textura, sem normal map (guia, seção 4). Com 8 mil triângulos o kit ficava facetado e o normal map gravado da malha alta manchava em triângulos; com 20 mil a forma vem da malha.

---

## 5. Animações

| Item | Valor |
|---|---|
| Quadros por segundo | **30** |
| Movimento do corpo | **sem root motion**: o `root` fica parado; o jogo move o personagem |
| Laços | `idle`, `walk`, `run`, `walk_combat`, `stance`, `stun`, canalizar e sentar (parado) fecham sem salto: o último quadro é igual ao primeiro |
| Mãos | os dedos fecham na arma por osso, nunca por malha |
| Pés | presos no chão por IK durante o apoio (sem patinar dentro do clipe) |
| Espelhamento | proibido: os dois lados são animados (o escudo é sempre na esquerda) |

### Nome dos clipes

`<grupo>.<papel>[.<arma>][.<n>]`

- **Grupos:** `base` (todos), `rc` (recruta), `kn` (cavaleiro), `bb` (bárbaro), `mg` (mago), `rg` (ladino). Uma evolução ou dupla que muda um clipe usa o próprio id: `herald.raise`, `possesso.idle`.
- **Armas:** os tipos do servidor (`server/internal/game/weapons.go`): `sword`, `axe`, `axe2h`, `staff`, `wand`, `daggers`, `bow`, `crossbow`, `crossbow2h`. O escudo não é arma: `sword` com escudo é o padrão do cavaleiro.
- **Exemplos:** `base.run`, `kn.idle`, `kn.attack.sword.1`, `bb.attack.axe2h.3`, `mg.skill.projectile`, `rg.skill.dash`.

### Ficha de cada clipe (`clips.json`)

Gerada pelo script de exportação do Blender, ao lado do GLB. O jogo lê daqui; nada é medido depois.

```json
{
  "kn.attack.sword.1": {
    "frames": 30,
    "loop": false,
    "impact": [17],
    "release": 22,
    "footSpeed": null,
    "hand": { "r": "fist", "l": "shield" }
  },
  "base.run": { "frames": 21, "loop": true, "impact": [], "footSpeed": 5.0 }
}
```

- `impact`: quadro(s) do golpe; o dano e o efeito saem aqui. Combo de vários golpes tem vários.
- `release`: a partir de que quadro o clipe pode ceder para outro (corrida, outra skill).
- `footSpeed`: velocidade do pé de apoio em m/s, para o clipe tocar sem deslizar.

### Tempo dos ataques

O clipe de ataque cabe no intervalo da arma, e o impacto cai antes da metade dele, para o golpe parecer responder ao clique.

| Arma | Intervalo no jogo | Duração do clipe | Impacto até |
|---|---|---|---|
| `daggers` | 0,8 s | ≤ 24 quadros | quadro 9 |
| `sword` | 1,2 s | ≤ 36 | 15 |
| `axe` | 1,4 s | ≤ 42 | 18 |
| `bow` | 1,5 s | ≤ 45 | 20 (soltar a corda) |
| `staff`, `wand` | 1,6 s | ≤ 48 | 20 (o projétil sai) |
| `crossbow` | 1,7 s | ≤ 51 | 20 |
| `axe2h` | 1,8 s | ≤ 54 | 24 |
| `crossbow2h` | 2,2 s | ≤ 66 | 26 |

### Velocidades de movimento

| | Velocidade no jogo | Ciclo do clipe |
|---|---|---|
| Andar | ~1,6 m/s | 30 quadros (2 passos) |
| Correr | 5 m/s (Ladino 5,5) | 20 a 22 quadros |
| Andar em combate | ~2,5 m/s | 26 quadros |

---

## 6. Arquivos

| O quê | Onde |
|---|---|
| Fonte do Blender (`.blend`), por peça e por classe | `assets-src/herois-v4/` (fora do git; ver "Em aberto") |
| Scripts do Blender (exportação, LOD, conferência) | `tools/art/herois-v4/` |
| GLB do jogo: corpo, cabeças, cabelos | `client/public/models/herois-v4/corpo.glb` |
| GLB das animações | `client/public/models/herois-v4/anim-<grupo>.glb` (só esqueleto e clipes) |
| GLB de cada kit | `client/public/models/herois-v4/kit-<id>.glb` |
| Fichas dos clipes | `client/public/models/herois-v4/clips.json` |

As animações ficam em arquivos próprios, sem malha: o jogo carrega a base uma vez e a camada da classe sob demanda.

---

## 7. Backup e capa (decididos em 19/11)

- **Fontes:** `assets-src/` é um repositório git **privado** à parte (`erevia-arte`), com Git LFS para `.blend`, `.png`, `.jpg`, `.glb` e `.fbx`. O repositório do jogo é público e continua ignorando `assets-src/`. Os `.blend` são salvos com a compressão do Blender, e o commit é feito nos marcos (peça ou clipe pronto), com push em seguida.
- **Capa e tecido:** **mola no cliente** como padrão (andar, correr, parar, virar, parado), nos 4 ossos `cape_*`, desligada a partir do LOD1 (≥ 20 m). Nos clipes em que a capa faz parte da ação (giro, salto, esquiva, sumir, morte), ela é animada à mão e a mola fica desligada: `clips.json` ganha o campo `"capeSpring": 0` a `1` (peso da mola; padrão 1).

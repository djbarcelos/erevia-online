package world

// Dungeon são os dados de uma masmorra instanciada, gerados junto com o mapa por
// client/scripts/dungeons/ (o mesmo arquivo serve ao cliente, que desenha o cenário, e ao
// servidor, que monta a colisão, os monstros e as escadas): uma fonte só do layout.
type Dungeon struct {
	// Entry: onde o grupo chega ao entrar (andar 1).
	Entry Point `json:"entry"`
	// Spawns: cada monstro, no lugar que o gerador escolheu (dentro de uma sala).
	Spawns []SpawnMark `json:"spawns"`
	// Stairs: as escadas entre os andares (cada uma é um portão local).
	Stairs []Stair `json:"stairs"`
	// Floors: o retângulo de cada andar (o cliente e os testes conferem que cada ponto
	// do jogo está sobre um deles).
	Floors []Bounds `json:"floors"`
}

// SpawnMark é um monstro fixo da masmorra (ou uma matilha: Count > 1 nasce junto, em volta
// do ponto, e puxar um puxa os outros). O chefe é o monstro com FieldBoss.Instance.
type SpawnMark struct {
	Monster string  `json:"m"`
	X       float64 `json:"x"`
	Z       float64 `json:"z"`
	Level   int     `json:"l"`
	Count   int     `json:"n"`
	Floor   int     `json:"f"`
}

// Stair é uma escada: o portão fica em (X, Z) e leva a (ToX, ToZ), no outro andar.
type Stair struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Z    float64 `json:"z"`
	ToX  float64 `json:"toX"`
	ToZ  float64 `json:"toZ"`
}

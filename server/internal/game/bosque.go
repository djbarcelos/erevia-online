package game

// Bosque Gris (7–15, roadmap 5.1) e as Terras Mortas (24–28, roadmap 5.2), tunnel#137.
//
// O Bosque Gris é o setor noroeste dos anéis 2 e 3 (de 100° a 170°, raio de 80 a 150 m) mais o canto NO do quadrado, onde
// fica a Toca do Corvo: o Bando do Corvo (Bandoleiro, Atirador de Facas e Brutamontes), os lobos da matilha e, à noite, as
// aranhas-de-musgo, com o Corvo Rubro, o chefe de campo, no acampamento. As Terras Mortas são o canto nordeste (r > 142, de
// 15° a 75°, da Cripta até o Bastião Cinzento e a Torre do Sino Rachado): os Devotos do Ossário e os mortos que eles levantam.
//
// Aqui ficam os lugares, os bichos, os grupos de spawn, o saque e os NPCs. Os mesmos números estão em
// client/scripts/biomes/bosque-gris.mjs, terras-mortas.mjs e terras-mortas-nucleo.mjs (o cenário) e em
// client/src/ui/regions.ts (o mapa). As histórias estão em story.go e as diárias em dailies.go.

const (
	// BosqueRegion e TerrasMortasRegion: o nome das regiões (spawns, mapa, música e missões usam o mesmo).
	BosqueRegion       = "Bosque Gris"
	TerrasMortasRegion = "Terras Mortas"

	// A Toca do Corvo: o acampamento do chefe de campo, no canto NO.
	CrowDenX, CrowDenZ = -128.0, 118.0
	// O Bastião Cinzento: o posto avançado da Ordem, nas Terras Mortas (a pedra de teleporte e o Irmão Caedmon).
	BastionX, BastionZ = 196.0, 94.0
	// A Torre do Sino Rachado (toca nos 10 min de aviso do Ozrhal) e a Lápide do Último Regicida.
	BellX, BellZ           = 238.0, 122.0
	TombstoneX, TombstoneZ = 172.0, 131.0
)

// Lugares para as missões e as rondas (cada um com o raio em que conta como visitado).
var (
	spotGallows = spot{"arvore_forca", "Árvore-Forca", -52, 84, 7}
	spotWagon   = spot{"carroca_tombada", "Carroça Tombada", -33, 90, 6}
	spotCrowDen = spot{"toca_corvo", "Toca do Corvo", CrowDenX, CrowDenZ, 14}
	spotBell    = spot{"torre_sino", "Torre do Sino Rachado", BellX, BellZ, 14}
	spotBastion = spot{"bastiao_cinzento", "Bastião Cinzento", BastionX, BastionZ, 10}
	spotGraves  = spot{"cemiterio_reis", "Cemitério dos Reis Sem Nome", 172, 122, 14}
)

// Nomes dos NPCs novos.
const (
	TalkCarter    = "Hélio"                     // Bosque Gris (o carroceiro, na Carroça Tombada)
	TalkBrother   = "Caedmon"                   // Terras Mortas (o Irmão do Bastião Cinzento, no Bastião)
	TalkTombstone = "Lápide do Último Regicida" // Terras Mortas (a lápide, ao lado do cemitério)
)

// TokenRegicide: no texto de um NPC, vira o nome de quem mais causou dano na última morte do Ozrhal (BossData.TopName).
const TokenRegicide = "{regicida}"

// ---------------------------------------------------------------------------------------
// Saque
// ---------------------------------------------------------------------------------------

var (
	// Bosque Gris: o Bando do Corvo deixa insígnias, os lobos deixam pele e presa, as aranhas deixam seda. Os pergaminhos
	// seguem o ritmo dos brutos da faixa.
	lootBandit = []LootEntry{{"crow_insignia", 0.2, 1, 1}, {"potion_hp_s", 0.1, 1, 2}, {"potion_mp_s", 0.05, 1, 1},
		{"scroll_ench_armor", 0.015, 1, 1}, {"scroll_ench_weapon", 0.008, 1, 1},
		{"scroll_ench_armor_b", 0.003, 1, 1}, {"scroll_ench_weapon_b", 0.002, 1, 1}}
	lootWolf = []LootEntry{{"wolf_pelt", 0.45, 1, 2}, {"wolf_fang", 0.25, 1, 1}, {"potion_hp_s", 0.06, 1, 1},
		{"scroll_ench_armor", 0.012, 1, 1}, {"scroll_ench_weapon", 0.006, 1, 1}}
	lootSpider = []LootEntry{{"moss_silk", 0.35, 1, 1}, {"wolf_fang", 0.1, 1, 1}, {"potion_mp_s", 0.08, 1, 1},
		{"scroll_ench_armor", 0.012, 1, 1}, {"scroll_ench_weapon", 0.006, 1, 1}}
	// Corvo Rubro (chefe de campo, nível 16): insígnias e peles garantidas e os épicos do grau D.
	lootCrow = append([]LootEntry{{"crow_insignia", 1, 3, 5}, {"wolf_pelt", 1, 2, 4}, {"potion_hp_l", 1, 2, 3}, {"potion_mp_l", 1, 1, 2},
		{"scroll_ench_weapon", 0.5, 1, 1}, {"scroll_ench_armor", 0.6, 1, 2},
		{"scroll_ench_weapon_b", 0.15, 1, 1}, {"scroll_ench_armor_b", 0.2, 1, 1}}, tierLoot(TierD, GradeEpic, 0.015)...)

	// Terras Mortas: cinza e velas dos Devotos, o Tomo Profano (raro) e as poções grandes do nível. Os pergaminhos seguem
	// o ritmo dos golens do Vale (20–30).
	lootDevout = []LootEntry{{"grave_ash", 0.4, 1, 2}, {"black_candle", 0.2, 1, 1}, {"profane_tome", 0.02, 1, 1},
		{"potion_hp_l", 0.05, 1, 1}, {"potion_mp_l", 0.04, 1, 1},
		{"scroll_ench_armor", 0.02, 1, 1}, {"scroll_ench_weapon", 0.01, 1, 1},
		{"scroll_ench_armor_b", 0.004, 1, 1}, {"scroll_ench_weapon_b", 0.002, 1, 1}}
	lootDevoutElite = []LootEntry{{"grave_ash", 0.45, 1, 3}, {"black_candle", 0.25, 1, 2}, {"profane_tome", 0.035, 1, 1},
		{"potion_hp_l", 0.06, 1, 1}, {"potion_mp_l", 0.05, 1, 1},
		{"scroll_ench_armor", 0.025, 1, 1}, {"scroll_ench_weapon", 0.012, 1, 1},
		{"scroll_ench_armor_b", 0.005, 1, 1}, {"scroll_ench_weapon_b", 0.003, 1, 1}}
	lootUndead = []LootEntry{{"grave_ash", 0.4, 1, 2}, {"black_candle", 0.15, 1, 1}, {"profane_tome", 0.015, 1, 1},
		{"shadow_essence", 0.1, 1, 1}, {"potion_hp_l", 0.05, 1, 1},
		{"scroll_ench_armor", 0.02, 1, 1}, {"scroll_ench_weapon", 0.01, 1, 1},
		{"scroll_ench_armor_b", 0.004, 1, 1}, {"scroll_ench_weapon_b", 0.002, 1, 1}}
)

// ---------------------------------------------------------------------------------------
// Monstros
// ---------------------------------------------------------------------------------------

// bosqueMonsters: os modelos são os do remaster (rogue, barbarian e mage heróis, as famílias wolf e spider e os esqueletos),
// só com a cor da variante (Tint); acessórios não há. Todos são de papel (fora das contas de caça do simulador, como os
// outros desta faixa: BalanceBaseline) e extras no bestiário (BestiaryExtra: os de papel são poucos por área).
var bosqueMonsters = map[string]*MonsterDef{
	// ---- Bosque Gris (7–15) ----
	// Bandoleiro: invisível até 4,5 m e o 1º golpe dói 50% a mais. Base do Ladino Esqueleto, com 10% a mais de vida.
	"bandoleiro_corvo": {ID: "bandoleiro_corvo", Name: "Bandoleiro do Corvo", Model: "rogue", Tint: "#d98a7e",
		HP: 60, HPLvl: 19, Atk: 10, AtkLv: 2.4, Def: 9, DefLv: 2.2, Range: 2, Interval: 1.0, Speed: 4.5, Aggressive: true, XPBase: 17, Scale: 1,
		Archetype: ArchAmbusher, BestiaryExtra: true, GoldChance: 0.75, GoldMin: 2.5, GoldMax: 4.5, Loot: lootBandit},
	// Atirador de Facas: fica a 7 a 10 m e recua 5 m quando você encosta. Base do Mago Esqueleto, com dano físico.
	"atirador_facas": {ID: "atirador_facas", Name: "Atirador de Facas", Model: "rogue", Tint: "#c9a56e",
		HP: 45, HPLvl: 14, Atk: 10, AtkLv: 2.4, Def: 6, DefLv: 1.6, Range: 10, Interval: 2.0, Speed: 3.8, Aggressive: true, XPBase: 18, Scale: 1,
		Archetype: ArchArcher, BestiaryExtra: true, GoldChance: 0.75, GoldMin: 2.5, GoldMax: 4.5, Loot: lootBandit},
	// Brutamontes: 60% a menos de dano físico de frente (o escudo redondo). Só de dia (à noite, as aranhas descem).
	"brutamontes_corvo": {ID: "brutamontes_corvo", Name: "Brutamontes do Corvo", Model: "barbarian", Tint: "#c97a64",
		HP: 77, HPLvl: 24, Atk: 9, AtkLv: 2.1, Def: 13, DefLv: 3.2, Range: 2.4, Interval: 1.7, Speed: 3.8, Aggressive: true, XPBase: 18, Scale: 1.12,
		Archetype: ArchShield, BestiaryExtra: true, GoldChance: 0.75, GoldMin: 2.5, GoldMax: 4.5, Loot: lootBandit},
	// Lobo Cinzento: a matilha (bater em um puxa todos; o último vivo entra em frenesi). Corre menos que o jogador (4,8 m/s).
	"lobo_cinzento": {ID: "lobo_cinzento", Name: "Lobo Cinzento", Model: "wolf", Tint: "#b4b8bf",
		HP: 55, HPLvl: 17, Atk: 8, AtkLv: 1.9, Def: 7, DefLv: 2.0, Range: 2, Interval: 1.2, Speed: 4.8, Aggressive: true, XPBase: 15, Scale: 1,
		Archetype: ArchPack, BestiaryExtra: true, GoldChance: 0.5, GoldMin: 1.5, GoldMax: 3, Loot: lootWolf},
	// Aranha-de-Musgo: só à noite. Desce da copa quando você chega a 4 m (o 1º golpe dói 50% a mais) e a teia prende 1,5 s.
	"aranha_musgo": {ID: "aranha_musgo", Name: "Aranha-de-Musgo", Model: "spider", Tint: "#8fa58a",
		HP: 50, HPLvl: 15, Atk: 9, AtkLv: 2.0, Def: 6, DefLv: 1.8, Range: 1.8, Interval: 1.2, Speed: 4.4, Aggressive: true, XPBase: 17, Scale: 0.9,
		Archetype: ArchAmbusher, Arch: ArchParams{AmbushRange: 4}, BestiaryExtra: true, GoldChance: 0.6, GoldMin: 2, GoldMax: 4, Loot: lootSpider,
		Abilities: []AbilityDef{{ID: "aranha_teia", Shape: ShapeCircle, Target: AimCurrent, Radius: 1.8,
			Delay: 1.0, Cooldown: 10, First: 4, Mult: 0.9, Effect: &EffectDef{Kind: EffectRoot, Duration: 1.5}}}},
	// Corvo Rubro, o Rei dos Bandoleiros (chefe de campo, nível 16): na Toca do Corvo, janela de 60 a 90 min.
	// F1 (100–60%): duelista. "Bomba de Fumaça": círculo de 5 m com 1,5 s de aviso, que cega por 3 s.
	// F2 (60–30%): apita e chama 4 Bandoleiros. "Chuva de Facas": cone de 90° e 8 m na direção do alvo, 1,5 s.
	// F3 (30–0%): "Salto": uma marca segue um jogador por 3 s e cai num círculo de 4 m (+40% de dano).
	"corvo_rubro": {ID: "corvo_rubro", Name: "Corvo Rubro", Model: "rogue", Tint: "#d6665c",
		HP: 450, HPLvl: 75, Atk: 13, AtkLv: 2.8, Def: 16, DefLv: 3.4, Range: 2.4, Interval: 1.5, Speed: 4.6, Aggressive: true, XPBase: 100, Scale: 1.35, Boss: true,
		GoldChance: 1, GoldMin: 28, GoldMax: 42, Loot: lootCrow,
		FieldBoss: &FieldBossDef{RespawnMin: 60, RespawnMax: 90, Place: "na Toca do Corvo", Phases: []FieldPhase{
			{AtHP: 0.60, Say: "Corvo Rubro: \"Rapazes! Ao meu apito!\"", Summon: "bandoleiro_corvo", SummonCount: 4},
			{AtHP: 0.30, Say: "Corvo Rubro: \"Já cansei de brincar com vocês!\""},
		}},
		Abilities: []AbilityDef{
			{ID: "corvo_fumaca", Shape: ShapeCircle, Target: AimCurrent, Radius: 5,
				Delay: 1.5, Cooldown: 12, First: 5, Mult: 0.9, Effect: &EffectDef{Kind: EffectBlind, Duration: 3, Value: 0.5}},
			{ID: "corvo_facas", Shape: ShapeCone, Target: AimSelf, Radius: 8, Arc: 90, MinPhase: 2,
				Delay: 1.5, Cooldown: 10, First: 3, Mult: 1.4},
			{ID: "corvo_salto", Look: LookMark, Shape: ShapeCircle, Target: AimMark, Radius: 4, MinPhase: 3,
				Delay: 3, Cooldown: 16, First: 4, Mult: 1.4,
				Warn: "O Corvo Rubro vai saltar em você! Afaste-se do grupo: o golpe cai em volta de você."},
		}},

	// ---- Terras Mortas (24–28) ----
	// Os Devotos do Ossário e os mortos que eles levantam: esqueletos e o mago do remaster com outra cor. Têm vida entre a
	// do Guerreiro Esqueleto e a do Golem do Vale (o grupo da faixa), e a XP acompanha a vida: um pouco acima da do Golem por
	// ponto de vida (o papel dá trabalho), mas eles têm menos vida e andam espalhados, então a XP por hora não passa a do Vale.
	//
	// Acólito: cura um aliado abaixo de 50% em 15% da vida (a cada 8 s) e foge do corpo a corpo. Base do Mago, 0,9× a vida.
	"ossario_acolito": {ID: "ossario_acolito", Name: "Acólito do Ossário", Model: "mage", Tint: "#8a6aa8",
		HP: 90, HPLvl: 24, Atk: 12, AtkLv: 2.6, Def: 10, DefLv: 2.6, Range: 9, Interval: 2.2, Speed: 3.6, Magic: true, Aggressive: true, XPBase: 30, Scale: 1,
		Archetype: ArchHealer, BestiaryExtra: true, GoldChance: 0.8, GoldMin: 3, GoldMax: 5.5, Loot: lootDevout},
	// Lacaio do Ossário: a invocação do Necromante (não renasce e não deixa saque).
	"lacaio_ossario": {ID: "lacaio_ossario", Name: "Lacaio do Ossário", Model: "skeleton_minion", Tint: "#8a78b0",
		HP: 60, HPLvl: 14, Atk: 8, AtkLv: 1.8, Def: 8, DefLv: 2.0, Range: 2, Interval: 1.6, Speed: 3.8, Aggressive: true, XPBase: 4, Scale: 0.95,
		NoGear: true},
	// Necromante Devoto: chama 2 Lacaios de nível −5 a cada 20 s (no máximo 4), que morrem com ele.
	"ossario_necromante": {ID: "ossario_necromante", Name: "Necromante Devoto", Model: "skeleton_mage", Tint: "#9a78d0",
		HP: 100, HPLvl: 25, Atk: 12, AtkLv: 2.6, Def: 9, DefLv: 2.4, Range: 10, Interval: 2.2, Speed: 3.4, Magic: true, Aggressive: true, XPBase: 32, Scale: 1.1,
		Archetype: ArchSummoner, Arch: ArchParams{Summon: "lacaio_ossario"}, BestiaryExtra: true,
		GoldChance: 0.8, GoldMin: 3, GoldMax: 5.5, Loot: lootDevout},
	// Cavaleiro do Ossário: 60% a menos de dano físico de frente, 3× a vida do Guerreiro Esqueleto e a "Investida" em linha de
	// 8 m (1,6 s de aviso). Elite.
	"ossario_cavaleiro": {ID: "ossario_cavaleiro", Name: "Cavaleiro do Ossário", Model: "skeleton_warrior", Tint: "#8b8a9a",
		HP: 210, HPLvl: 66, Atk: 10, AtkLv: 2.4, Def: 14, DefLv: 3.2, Range: 2.4, Interval: 1.6, Speed: 4.0, Aggressive: true, XPBase: 58, Scale: 1.3,
		Elite: true, Archetype: ArchShield, BestiaryExtra: true, GoldChance: 0.85, GoldMin: 3.5, GoldMax: 6, Loot: lootDevoutElite,
		Abilities: []AbilityDef{{ID: "cavaleiro_investida", Shape: ShapeLine, Target: AimSelf, Length: 8, Width: 2.4,
			Delay: 1.6, Cooldown: 12, First: 5, Mult: 1.6}}},
	// Carniçal: só à noite, em matilha de 3. Corre mais que o jogador (5,2 m/s) e cada golpe sangra por 4 s.
	"carnical": {ID: "carnical", Name: "Carniçal", Model: "skeleton_minion", Tint: "#b08a80",
		HP: 80, HPLvl: 20, Atk: 11, AtkLv: 2.2, Def: 10, DefLv: 2.4, Range: 2, Interval: 1.2, Speed: 5.2, Aggressive: true, XPBase: 29, Scale: 1.1,
		Archetype: ArchPack, BestiaryExtra: true, GoldChance: 0.75, GoldMin: 3, GoldMax: 5, Loot: lootUndead,
		Abilities: []AbilityDef{{ID: "carnical_garra", Shape: ShapeCircle, Target: AimCurrent, Radius: 2,
			Delay: 1.0, Cooldown: 6, First: 3, Mult: 0.9, Effect: &EffectDef{Kind: EffectPoison, Duration: 4, Value: 12}}}},
	// Espectro: só à noite. Escondido até você chegar a 5 m (o 1º golpe dói 50% a mais); o toque gélido enfraquece o ataque.
	"espectro": {ID: "espectro", Name: "Espectro", Model: "skeleton_mage", Tint: "#9cc4d8",
		HP: 85, HPLvl: 20, Atk: 13, AtkLv: 2.8, Def: 8, DefLv: 2.0, Range: 2.4, Interval: 1.8, Speed: 4.4, Magic: true, Aggressive: true, XPBase: 30, Scale: 1.1,
		Archetype: ArchAmbusher, Arch: ArchParams{AmbushRange: 5}, BestiaryExtra: true, GoldChance: 0.75, GoldMin: 3, GoldMax: 5, Loot: lootUndead,
		Abilities: []AbilityDef{{ID: "espectro_toque", Shape: ShapeCircle, Target: AimCurrent, Radius: 2.2,
			Delay: 1.0, Cooldown: 9, First: 4, Mult: 1.1, Effect: &EffectDef{Kind: EffectWeaken, Duration: 4, Value: 0.15}}}},
}

// Os MonsterDef entram em Monsters antes dos init (a validação das habilidades, dos papéis e dos chefes já os vê).
var _ = func() bool {
	for id, m := range bosqueMonsters {
		Monsters[id] = m
	}
	return true
}()

// ---------------------------------------------------------------------------------------
// Grupos de spawn
// ---------------------------------------------------------------------------------------

// regionGroup: um grupo de monstros (um disco de 12 m). Os que valem sempre ficam em `any`; os de dia e os de noite se
// trocam na virada (somam o mesmo por região, como em todo o jogo). A matilha (lobos e carniçais) nasce num disco menor,
// junta. Os centros saem de uma busca do menor pico num raio de 70 m, em chão livre e longe dos lugares das missões.
type regionGroup struct {
	x, z     float64
	lo, hi   int
	any, day map[string]int
	night    map[string]int
}

const (
	groupRadius   = 12.0
	packRadius    = 6.0 // o disco da matilha (os lobos e os carniçais)
	bosqueRespawn = 35.0
	mortasRespawn = 50.0
)

// bosqueGroups: 16 monstros ao mesmo tempo (4 Bandoleiros, 3 Atiradores, 6 Lobos, e 3 Brutamontes de dia que viram 3 Aranhas à
// noite), do nível 7 ao 15 (os grupos mais fundos no Bosque têm os níveis mais altos). O pico num raio de 70 m fica em 13
// (TestBosqueBudget); a Toca do Corvo, com o chefe, fica fora da conta.
var bosqueGroups = []regionGroup{
	{x: -106, z: 34, lo: 7, hi: 9, any: map[string]int{"lobo_cinzento": 3}},
	{x: -110, z: 70, lo: 9, hi: 11, any: map[string]int{"bandoleiro_corvo": 2, "atirador_facas": 1},
		day: map[string]int{"brutamontes_corvo": 1}, night: map[string]int{"aranha_musgo": 1}},
	{x: -86, z: 110, lo: 11, hi: 13, any: map[string]int{"lobo_cinzento": 3}},
	{x: -34, z: 138, lo: 12, hi: 14, any: map[string]int{"bandoleiro_corvo": 1, "atirador_facas": 1},
		day: map[string]int{"brutamontes_corvo": 1}, night: map[string]int{"aranha_musgo": 1}},
	{x: -146, z: 34, lo: 13, hi: 15, any: map[string]int{"bandoleiro_corvo": 1, "atirador_facas": 1},
		day: map[string]int{"brutamontes_corvo": 1}, night: map[string]int{"aranha_musgo": 1}},
}

// terrasMortasGroups: 14 monstros ao mesmo tempo, do nível 24 ao 28 pela distância da cidade: 3 Acólitos, 2 Necromantes e
// 3 Cavaleiros sempre; de dia mais 1 Acólito, 1 Necromante e 4 Cavaleiros; à noite, no lugar deles, 3 Carniçais (a matilha) e
// 3 Espectros. Dois grupos ficam no núcleo, em volta da Cripta (fora do covil: 46 m do centro dele), e três na extensão,
// longe do Bastião, da Torre e da lápide. O pico num raio de 70 m fica em 14 (TestTerrasMortasBudget): o bloco é pequeno,
// então não cabem mais que isso sem passar o teto da área de interesse.
var terrasMortasGroups = []regionGroup{
	{x: 146, z: 48, lo: 24, hi: 25, any: map[string]int{"ossario_acolito": 1, "ossario_necromante": 1},
		day: map[string]int{"ossario_cavaleiro": 1}, night: map[string]int{"carnical": 3}},
	{x: 146, z: 128, lo: 25, hi: 26, any: map[string]int{"ossario_acolito": 1, "ossario_cavaleiro": 1},
		day: map[string]int{"ossario_cavaleiro": 1}},
	{x: 250, z: 94, lo: 27, hi: 28, any: map[string]int{"ossario_necromante": 1, "ossario_cavaleiro": 1},
		day: map[string]int{"ossario_necromante": 1}, night: map[string]int{"espectro": 1}},
	{x: 210, z: 134, lo: 26, hi: 27, any: map[string]int{"ossario_acolito": 1, "ossario_cavaleiro": 1},
		day: map[string]int{"ossario_cavaleiro": 1}, night: map[string]int{"espectro": 1}},
	{x: 226, z: 70, lo: 26, hi: 27, day: map[string]int{"ossario_acolito": 1, "ossario_cavaleiro": 1}, night: map[string]int{"espectro": 1}},
}

// groupSpawns: os Spawn de uma lista de grupos, na ordem dos ids dos monstros (grupo a grupo, sempre/dia/noite).
func groupSpawns(groups []regionGroup, region string, respawn float64) []Spawn {
	var out []Spawn
	// A ordem dos monstros dentro do grupo é a dos ids (sorteio estável): vem de uma lista fixa, não do mapa.
	order := []string{"lobo_cinzento", "bandoleiro_corvo", "atirador_facas", "brutamontes_corvo", "aranha_musgo",
		"ossario_acolito", "ossario_necromante", "ossario_cavaleiro", "carnical", "espectro"}
	for _, g := range groups {
		add := func(m map[string]int, when When) {
			for _, id := range order {
				n := m[id]
				if n == 0 {
					continue
				}
				radius := groupRadius
				if d := Monsters[id]; d != nil && d.Archetype == ArchPack {
					radius = packRadius // a matilha nasce junta
				}
				out = append(out, Spawn{Monster: id, Count: n, Area: true, X: g.x, Z: g.z, Radius: radius, LevelMin: g.lo, LevelMax: g.hi,
					Respawn: respawn, Region: region, When: when})
			}
		}
		add(g.any, WhenAny)
		add(g.day, WhenDay)
		add(g.night, WhenNight)
	}
	return out
}

// bosqueSpawns: os grupos do Bosque Gris, o Corvo Rubro na Toca e o setor com dono (sem monstros: só avisa aos anéis que o
// pedaço noroeste não é deles, e os 9 esqueletos que sorteariam lá nascem em outro lugar do mesmo anel, sem mudar de id).
func bosqueSpawns() []Spawn {
	out := groupSpawns(bosqueGroups, BosqueRegion, bosqueRespawn)
	out = append(out, Spawn{Monster: "corvo_rubro", Count: 1, Fixed: true, X: CrowDenX, Z: CrowDenZ, LevelMin: 16, Respawn: 3000, Region: BosqueRegion})
	// O setor: de 80 m (o começo do anel 2) até o canto do quadrado (212 m), de 100° a 170°.
	return append(out, Spawn{Monster: "lobo_cinzento", Count: 0, RMin: 80, RMax: 212, LevelMin: 7, LevelMax: 15, Respawn: bosqueRespawn,
		AngleMin: 100, AngleMax: 170, Region: BosqueRegion})
}

// terrasMortasSpawns: os grupos das Terras Mortas e o setor com dono (de 142 m ao canto do quadrado, 212 m, de 15° a 75°: o núcleo; a
// extensão leste é o retângulo do mapa, com o mesmo nome).
func terrasMortasSpawns() []Spawn {
	out := groupSpawns(terrasMortasGroups, TerrasMortasRegion, mortasRespawn)
	return append(out, Spawn{Monster: "ossario_acolito", Count: 0, RMin: 142, RMax: 212, LevelMin: 24, LevelMax: 28, Respawn: mortasRespawn,
		AngleMin: 15, AngleMax: 75, Region: TerrasMortasRegion})
}

// ---------------------------------------------------------------------------------------
// Itens: 8 materiais e o cetro e a varinha épicos do nível 24
// ---------------------------------------------------------------------------------------

// bosqueItems: os materiais do Bosque Gris (Pele de Lobo, Presa de Lobo, Seda de Musgo e Insígnia do Corvo) e das Terras
// Mortas (Cinza Sepulcral, Vela Negra, Tomo Profano e Fragmento de Selo, que só vem de missão), e as duas armas épicas de
// receita do mago (Tomo Profano). As capas e luvas de nível 13 e os pergaminhos abençoados das receitas são os que já existem.
func bosqueItems() []*ItemDef {
	staff := weapon(WeaponStaff, "staff_devout_epic", "Cetro Profano", "mage", GradeEpic, 24, Stats{MAtk: 58, MaxMP: 120}, 18340, "staff", "",
		"Esculpido com as páginas de um Tomo Profano. As velas negras acendem sozinhas quando ele é erguido.")
	wand := weapon(WeaponWand, "wand_devout_epic", "Varinha Profana", "mage", GradeEpic, 24, Stats{MAtk: 52, MaxMP: 100, CastSpeed: 0.08}, 18340, "wand", "",
		"Fina como um osso de dedo, e mais rápida que o pensamento. As letras do Tomo Profano correm pelo cabo.")
	// Feitas para o grau C, como as armas de tiers.go, mas só saem do Ferreiro: ficam fora do sorteio por raridade e abaixo da
	// épica do grau (o Cetro do Trono Partido), então a escada de poder do grau não muda.
	staff.Graded, wand.Graded = true, true
	return []*ItemDef{
		// Bosque Gris (7–15).
		stackable("wolf_pelt", "Pele de Lobo", KindMaterial, 40, "wolf_pelt", "Pele grossa e cinzenta, boa para couraças e botas de couro. Os curtidores pagam por ela."),
		stackable("wolf_fang", "Presa de Lobo", KindMaterial, 30, "wolf_fang", "Uma presa amarelada. Os caçadores do Bosque furam e penduram no pescoço."),
		stackable("moss_silk", "Seda de Musgo", KindMaterial, 90, "moss_silk", "Teia fina e prateada das Aranhas-de-Musgo, que só descem das copas à noite."),
		stackable("crow_insignia", "Insígnia do Corvo", KindMaterial, 150, "crow_insignia", "Um corvo de pano vermelho costurado num cinto. Cada bandoleiro leva um, e não o larga vivo."),
		// Terras Mortas (24–28).
		stackable("grave_ash", "Cinza Sepulcral", KindMaterial, 60, "grave_ash", "Cinza fria de uma cova revirada. Não cheira a fumaça: cheira a terra molhada."),
		stackable("black_candle", "Vela Negra", KindMaterial, 130, "black_candle", "Sebo negro de uma vela dos Devotos. Queima sem calor, com chama verde-pálida."),
		stackable("profane_tome", "Tomo Profano", KindMaterial, 600, "profane_tome", "Um livro de capa de pele e páginas de cinza, escrito pelos Devotos. Cada palavra pesa na língua."),
		stackable("seal_fragment", "Fragmento de Selo", KindMaterial, 500, "seal_fragment", "Um caco de pedra com runas Ilvarin, ainda quente. Pertence à primeira das cinco Pedras-Selo."),
		staff, wand,
	}
}

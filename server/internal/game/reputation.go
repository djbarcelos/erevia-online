package game

// Reputação com as facções (tunnel#157). Hoje são duas, ligadas ao roadmap
// (docs/roadmap-areas.md, seção 7): a Ordem do Bastião Cinzento (o Vale e a Cripta) e a
// Guarda de Vel'Aran (a cidade, as estradas dos Ermos e os eventos). A reputação só sobe,
// com os contratos (contracts.go), e nunca é gasta: o grau abre as ofertas da loja da
// facção, que custam moedas.
//
// A loja é só horizontal: títulos, capas que repetem o poder do Manto das Cinzas (a capa
// épica que o Ferreiro cria; "só muda o visual", como as peças de mesmo bônus do
// balanceamento), pergaminhos e consumíveis. Nada de equipamento melhor que o do chefe
// (TestRepShopIsHorizontal trava).

const (
	FactionBastion = "bastion"
	FactionGuard   = "guard"
)

// Faction é uma facção.
type Faction struct {
	ID    string
	Name  string
	Text  string
	Color string
}

var Factions = []*Faction{
	{ID: FactionBastion, Name: "Ordem do Bastião Cinzento", Color: "#b9c2d4",
		Text: "Monges-guerreiros de Vel'Aran que vigiam a Cripta desde a fundação. Sobe com os contratos do Vale das Pedras Vivas e da Cripta Esquecida."},
	{ID: FactionGuard, Name: "Guarda de Vel'Aran", Color: "#d9a441",
		Text: "Quem segura as muralhas e as estradas. Sobe com os contratos dos Ermos Partidos, do Senhor dos Ossos e dos eventos da cidade (invasão e PvP)."},
}

// FactionByID: a facção pelo id (nil se não existir).
func FactionByID(id string) *Faction {
	for _, f := range Factions {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// RepGrade é o grau da reputação.
type RepGrade int

const (
	RepNeutral RepGrade = iota
	RepFriendly
	RepHonored
	RepRevered
	RepExalted
)

// RepGradeNames e RepGradePoints: o nome e os pontos (acumulados) de cada grau.
var (
	RepGradeNames  = []string{"Neutro", "Amigável", "Honrado", "Reverenciado", "Exaltado"}
	RepGradePoints = []int{0, 500, 1500, 3500, 7000}
)

// RepMax: a reputação para no exaltado.
var RepMax = RepGradePoints[len(RepGradePoints)-1]

// GradeOf: o grau com points pontos.
func GradeOf(points int) RepGrade {
	g := RepNeutral
	for i, need := range RepGradePoints {
		if points >= need {
			g = RepGrade(i)
		}
	}
	return g
}

// ReputationState é o que fica salvo no personagem (characters.reputation).
type ReputationState struct {
	Points map[string]int `json:"points,omitempty"`
}

// Of: os pontos com a facção.
func (r *ReputationState) Of(faction string) int { return r.Points[faction] }

// Add soma pontos com a facção (até RepMax). Devolve quanto entrou e o grau antes e depois.
func (r *ReputationState) Add(faction string, n int) (gained int, before, after RepGrade) {
	cur := r.Points[faction]
	before = GradeOf(cur)
	if FactionByID(faction) == nil || n <= 0 {
		return 0, before, before
	}
	next := min(cur+n, RepMax)
	if r.Points == nil {
		r.Points = map[string]int{}
	}
	r.Points[faction] = next
	return next - cur, before, GradeOf(next)
}

// NormalReputation limpa o que veio do banco: facções que não existem e pontos fora da faixa.
func NormalReputation(st ReputationState) ReputationState {
	out := ReputationState{}
	for id, n := range st.Points {
		if FactionByID(id) == nil || n <= 0 {
			continue
		}
		if out.Points == nil {
			out.Points = map[string]int{}
		}
		out.Points[id] = min(n, RepMax)
	}
	return out
}

// ---------- Loja ----------

// RepOffer é uma oferta da loja de uma facção: um item (Item × Count) ou um título (Title,
// o id de uma conquista de RepTitleAchievements, que só a compra desbloqueia).
type RepOffer struct {
	ID      string
	Faction string
	Grade   RepGrade // grau mínimo
	Item    string
	Count   int
	Title   string
	Price   int64 // moedas (nunca abaixo do que a loja paga pelos itens: TestRepShopPrices)
	Text    string
	// Lugar previsto, ainda sem sistema (não inventar aqui):
	//   - Recipe: receitas de grau C do Ferreiro (tunnel#153). Quando existirem, a oferta
	//     ensina a receita em vez de dar um item (ex.: Bastião, Reverenciado).
	//   - Cristais (tunnel#154): entram como Item comum quando o item existir (ex.: Guarda,
	//     Honrado, um pacote pequeno por semana).
}

// RepOffers: as lojas, na ordem da janela (por facção e grau).
var RepOffers = []*RepOffer{
	// Ordem do Bastião Cinzento.
	{ID: "bastion_hp_l", Faction: FactionBastion, Grade: RepFriendly, Item: "potion_hp_l", Count: 10, Price: 600,
		Text: "Dez Poções de Vida Maior das reservas da Ordem, 20% mais baratas que na praça."},
	{ID: "bastion_return", Faction: FactionBastion, Grade: RepFriendly, Item: "scroll_return", Count: 5, Price: 480,
		Text: "Cinco Pergaminhos de Retorno, 20% mais baratos."},
	{ID: "bastion_ench_armor", Faction: FactionBastion, Grade: RepHonored, Item: "scroll_ench_armor", Count: 1, Price: 540,
		Text: "Pergaminho de Encantar Armadura, 10% mais barato que no Ferreiro."},
	{ID: "bastion_title_watch", Faction: FactionBastion, Grade: RepHonored, Title: "rep_bastion_honored", Price: 3000,
		Text: "Título «Vigia da Cripta», embaixo do nome, para todos verem."},
	{ID: "bastion_cape", Faction: FactionBastion, Grade: RepRevered, Item: "cape_rep_bastion", Count: 1, Price: 9000,
		Text: "Manto do Bastião Cinzento: o mesmo poder do Manto das Cinzas, nas cores da Ordem."},
	{ID: "bastion_ench_armor_b", Faction: FactionBastion, Grade: RepExalted, Item: "scroll_ench_armor_b", Count: 1, Price: 5000,
		Text: "Pergaminho Abençoado de Armadura (falhar volta a +0, não quebra)."},
	{ID: "bastion_title_hand", Faction: FactionBastion, Grade: RepExalted, Title: "rep_bastion_exalted", Price: 10000,
		Text: "Título «Mão do Bastião»."},
	// Guarda de Vel'Aran.
	{ID: "guard_mp_l", Faction: FactionGuard, Grade: RepFriendly, Item: "potion_mp_l", Count: 10, Price: 720,
		Text: "Dez Poções de Mana Maior do quartel, 20% mais baratas que na praça."},
	{ID: "guard_return", Faction: FactionGuard, Grade: RepFriendly, Item: "scroll_return", Count: 5, Price: 480,
		Text: "Cinco Pergaminhos de Retorno, 20% mais baratos."},
	{ID: "guard_ench_weapon", Faction: FactionGuard, Grade: RepHonored, Item: "scroll_ench_weapon", Count: 1, Price: 1350,
		Text: "Pergaminho de Encantar Arma, 10% mais barato que no Ferreiro."},
	{ID: "guard_title_friend", Faction: FactionGuard, Grade: RepHonored, Title: "rep_guard_honored", Price: 3000,
		Text: "Título «Amigo da Guarda»."},
	{ID: "guard_cape", Faction: FactionGuard, Grade: RepRevered, Item: "cape_rep_guard", Count: 1, Price: 9000,
		Text: "Capa da Guarda de Vel'Aran: o mesmo poder do Manto das Cinzas, no azul e dourado da muralha."},
	{ID: "guard_ench_weapon_b", Faction: FactionGuard, Grade: RepExalted, Item: "scroll_ench_weapon_b", Count: 1, Price: 12000,
		Text: "Pergaminho Abençoado de Arma (falhar volta a +0, não quebra)."},
	{ID: "guard_title_shield", Faction: FactionGuard, Grade: RepExalted, Title: "rep_guard_exalted", Price: 10000,
		Text: "Título «Escudo de Vel'Aran»."},
}

// RepOfferByID: a oferta pelo id (nil se não existir).
func RepOfferByID(id string) *RepOffer {
	for _, o := range RepOffers {
		if o.ID == id {
			return o
		}
	}
	return nil
}

// RepTitleAchievements: os títulos da loja. São conquistas (o título usa o sistema de
// sempre, na janela Glória) que só a compra desbloqueia (AchRepBuy, Target = o próprio id).
var RepTitleAchievements = []*AchievementDef{
	{ID: "rep_bastion_honored", Name: "Vigia da Cripta", Title: "Vigia da Cripta", Goal: AchRepBuy, Target: "rep_bastion_honored", Count: 1,
		Text: "Comprar o título na loja da Ordem do Bastião Cinzento (Honrado)."},
	{ID: "rep_bastion_exalted", Name: "Mão do Bastião", Title: "Mão do Bastião", Goal: AchRepBuy, Target: "rep_bastion_exalted", Count: 1,
		Text: "Comprar o título na loja da Ordem do Bastião Cinzento (Exaltado)."},
	{ID: "rep_guard_honored", Name: "Amigo da Guarda", Title: "Amigo da Guarda", Goal: AchRepBuy, Target: "rep_guard_honored", Count: 1,
		Text: "Comprar o título na loja da Guarda de Vel'Aran (Honrado)."},
	{ID: "rep_guard_exalted", Name: "Escudo de Vel'Aran", Title: "Escudo de Vel'Aran", Goal: AchRepBuy, Target: "rep_guard_exalted", Count: 1,
		Text: "Comprar o título na loja da Guarda de Vel'Aran (Exaltado)."},
}

// RepItems: as capas das facções. Os mesmos bônus, nível e raridade do Manto das Cinzas
// (cape_epic), com outro nome e outra cor: só na loja (RepOnly: não cai de monstro).
var RepItems = []*ItemDef{
	repCape("cape_rep_bastion", "Manto do Bastião Cinzento", "Lã cinza com o olho branco da Ordem bordado nas costas."),
	repCape("cape_rep_guard", "Capa da Guarda de Vel'Aran", "Azul da muralha, com a borda dourada dos capitães."),
}

// repCapeBonus: os bônus do Manto das Cinzas (TestRepCapesMatchAshMantle confere).
var repCapeBonus = Stats{MDef: 21, Evasion: 7, MaxHP: 60, HPRegen: 2}

func repCape(id, name, desc string) *ItemDef {
	d := gear(id, name, KindCape, nil, GradeEpic, 18, repCapeBonus, 10500, "", "cape", desc)
	d.RepOnly = true
	return d
}

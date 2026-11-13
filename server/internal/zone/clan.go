package zone

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"erevia/server/internal/pb"
)

// Clã: grupo permanente (salvo no banco). A zona é a dona do estado em memória e manda
// cada mudança, em ordem, para um worker que grava no banco (a zona nunca espera o
// banco). Criar precisa da resposta do banco (o nome pode estar em uso): o resultado
// volta como evento.
const (
	clanCreateLevel = 10
	clanCreateCost  = 5000
	clanMaxMembers  = 30
	clanJoinWait    = 24 * time.Hour // quem sai ou é expulso espera para entrar em outro
	clanNoticeMax   = 200
	clanJobQueue    = 1024
)

// Cargos (iguais ao enum do protocolo e ao banco).
const (
	rankMember  = 0
	rankOfficer = 1
	rankLeader  = 2
)

// ErrClanNameTaken: o banco recusou o nome (já existe).
var ErrClanNameTaken = errors.New("clan name taken")

// ClanData e ClanMemberData: clãs carregados do banco na partida do servidor.
type ClanData struct {
	ID       int64
	Name     string
	Notice   string
	LeaderID int64
	Members  []ClanMemberData
}

type ClanMemberData struct {
	CharacterID int64
	Name, Class string
	Level, Rank int
}

// ClanOpKind diz o que uma ClanOp muda (mesma ordem de store.ClanOpKind).
type ClanOpKind int

const (
	ClanOpJoin ClanOpKind = iota + 1
	ClanOpLeave
	ClanOpRank
	ClanOpLeader
	ClanOpNotice
	ClanOpDisband
)

// ClanOp é uma mudança já aplicada em memória, para o banco gravar.
type ClanOp struct {
	Kind        ClanOpKind
	ClanID      int64
	CharacterID int64
	Rank        int
	Text        string
	LeftAt      time.Time
}

// ClanBackend grava clãs (implementado com o store).
type ClanBackend interface {
	CreateClan(ctx context.Context, name string, leaderID int64) (int64, error)
	ApplyClanOp(ctx context.Context, op ClanOp) error
}

// clan é um clã em memória.
type clan struct {
	id      int64
	name    string
	notice  string
	leader  int64
	members map[int64]*clanMember // personagem → membro (online ou não)
}

type clanMember struct {
	id    int64
	name  string
	class string
	level int
	rank  int
}

// clanJob é um trabalho para o worker do banco.
type clanJob struct {
	op     *ClanOp
	create *clanCreate
}

type clanCreate struct {
	player uint32
	name   string
	cost   int64
}

// clanCreatedEvent: resposta do banco ao pedido de criar.
type clanCreatedEvent struct {
	req clanCreate
	id  int64
	err error
}

func (clanCreatedEvent) isEvent() {}

// SetClanBackend liga o banco dos clãs (antes de Run). Sem ele (testes), as mudanças
// ficam só em memória e criar sempre dá certo com um id local.
func (z *Zone) SetClanBackend(b ClanBackend) { z.clanBackend = b }

// loadClans monta os clãs vindos do banco.
func (z *Zone) loadClans(list []ClanData) {
	for _, d := range list {
		c := &clan{id: d.ID, name: d.Name, notice: d.Notice, leader: d.LeaderID, members: map[int64]*clanMember{}}
		for _, m := range d.Members {
			c.members[m.CharacterID] = &clanMember{id: m.CharacterID, name: m.Name, class: m.Class, level: m.Level, rank: m.Rank}
			z.clanOf[m.CharacterID] = c
		}
		z.clans[c.id] = c
	}
}

// runClanWorker grava as mudanças em ordem (tentando de novo se o banco falhar) e
// devolve o resultado de "criar" para a zona.
func (z *Zone) runClanWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-z.clanJobs:
			if ev := z.doClanJob(ctx, job); ev != nil {
				z.post(ctx, ev)
			}
		}
	}
}

func (z *Zone) doClanJob(ctx context.Context, job clanJob) event {
	if job.create != nil {
		var id int64
		var err error
		if z.clanBackend == nil {
			z.localClanSeq++
			id = z.localClanSeq
		} else {
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			id, err = z.clanBackend.CreateClan(cctx, job.create.name, int64(job.create.player))
			cancel()
		}
		return clanCreatedEvent{req: *job.create, id: id, err: err}
	}
	if z.clanBackend == nil || job.op == nil {
		return nil
	}
	for attempt := 1; attempt <= 3; attempt++ {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := z.clanBackend.ApplyClanOp(cctx, *job.op)
		cancel()
		if err == nil {
			return nil
		}
		z.log.Error("clan op", "err", err, "kind", job.op.Kind, "clan", job.op.ClanID, "attempt", attempt)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	return nil
}

// queueClan entrega um trabalho ao worker sem bloquear a zona.
func (z *Zone) queueClan(job clanJob) bool {
	select {
	case z.clanJobs <- job:
		return true
	default:
		z.log.Error("clan queue full")
		return false
	}
}

func (z *Zone) clanOp(op ClanOp) { z.queueClan(clanJob{op: &op}) }

// ---------- Comandos ----------

func (z *Zone) onClanCmd(p *entity, m *pb.ClanCmd) {
	switch m.Kind {
	case pb.ClanCmdKind_CLAN_CMD_CREATE:
		z.clanCreate(p, strings.TrimSpace(m.Name))
	case pb.ClanCmdKind_CLAN_CMD_INVITE:
		z.clanInvite(p, z.findPlayer(m.TargetId, m.Name))
	case pb.ClanCmdKind_CLAN_CMD_LEAVE:
		z.clanLeave(p)
	case pb.ClanCmdKind_CLAN_CMD_KICK:
		z.clanKick(p, int64(m.TargetId))
	case pb.ClanCmdKind_CLAN_CMD_RANK:
		z.clanSetRank(p, int64(m.TargetId), int(m.Rank))
	case pb.ClanCmdKind_CLAN_CMD_LEADER:
		z.clanPassLeader(p, int64(m.TargetId))
	case pb.ClanCmdKind_CLAN_CMD_DISBAND:
		z.clanDisband(p)
	case pb.ClanCmdKind_CLAN_CMD_NOTICE:
		z.clanNotice(p, m.Text)
	}
}

// myRank: cargo do jogador no clã dele (-1 = sem clã).
func (z *Zone) myRank(p *entity) int {
	if c := p.pl.clan; c != nil {
		if m := c.members[int64(p.id)]; m != nil {
			return m.rank
		}
	}
	return -1
}

func (z *Zone) clanNameTaken(name string) bool {
	for _, c := range z.clans {
		if strings.EqualFold(c.name, name) {
			return true
		}
	}
	for _, c := range z.clanCreating {
		if strings.EqualFold(c.name, name) {
			return true
		}
	}
	return false
}

func (z *Zone) clanCreate(p *entity, name string) {
	switch {
	case p.pl.clan != nil:
		z.system(p, "Você já está em um clã.")
	case z.clanCreating[p.id] != nil:
		z.system(p, "Seu clã já está sendo criado.")
	case p.level < clanCreateLevel:
		z.system(p, fmt.Sprintf("É preciso nível %d para criar um clã.", clanCreateLevel))
	case p.pl.inv.gold < clanCreateCost:
		z.system(p, fmt.Sprintf("Criar um clã custa %d moedas.", clanCreateCost))
	case !ValidName(name):
		z.system(p, "O nome do clã deve ter de 3 a 16 letras ou números.")
	case z.clanNameTaken(name):
		z.system(p, "Já existe um clã com esse nome.")
	default:
		req := &clanCreate{player: p.id, name: name, cost: clanCreateCost}
		if !z.queueClan(clanJob{create: req}) {
			z.system(p, "Não foi possível criar o clã agora. Tente de novo.")
			return
		}
		// As moedas saem já (e voltam se o banco recusar o nome).
		p.pl.inv.gold -= req.cost
		z.trackGold(p, "clan", -req.cost)
		z.clanCreating[p.id] = req
		z.inventoryChanged(p, false)
	}
}

// onClanCreated: o banco respondeu ao pedido de criar.
func (z *Zone) onClanCreated(e clanCreatedEvent) {
	delete(z.clanCreating, e.req.player)
	p := z.players[e.req.player]
	if e.err != nil {
		z.refundGold(e.req.player, e.req.cost)
		if p != nil {
			if errors.Is(e.err, ErrClanNameTaken) {
				z.system(p, "Já existe um clã com esse nome. As moedas foram devolvidas.")
			} else {
				z.system(p, "Não foi possível criar o clã agora. As moedas foram devolvidas.")
				z.log.Error("create clan", "err", e.err, "player", e.req.player)
			}
		}
		return
	}
	c := &clan{id: e.id, name: e.req.name, leader: int64(e.req.player), members: map[int64]*clanMember{}}
	z.clans[c.id] = c
	lm := &clanMember{id: int64(e.req.player), rank: rankLeader}
	c.members[lm.id] = lm
	z.clanOf[lm.id] = c
	if p == nil {
		// Saiu antes da resposta: o clã existe; o nome/nível aparecem quando ele voltar.
		if r, ok := z.recent[e.req.player]; ok {
			lm.level = r.state.Level
		}
		return
	}
	lm.name, lm.class, lm.level = p.name, p.pl.class, p.level
	p.pl.clan = c
	z.system(p, fmt.Sprintf("O clã ‹%s› foi criado! Convide membros pela janela do clã (N) ou pelo quadro do alvo.", c.name))
	z.broadcastStatus(p)
	z.sendClan(c)
	z.log.Info("clan created", "clan", c.id, "name", c.name, "leader", p.id)
}

// refundGold devolve moedas a quem está online ou acabou de sair.
func (z *Zone) refundGold(id uint32, amount int64) {
	if p := z.players[id]; p != nil {
		p.pl.inv.gold += amount
		z.trackGold(p, "clan_refund", amount)
		z.inventoryChanged(p, false)
		return
	}
	if r, ok := z.recent[id]; ok {
		r.state.Gold += amount
		r.state.Seq = z.nextSeq()
		z.recent[id] = r
		z.save([]SavedState{r.state}, false)
	}
}

func (z *Zone) clanInvite(p, t *entity) {
	c := p.pl.clan
	switch {
	case c == nil:
		z.system(p, "Você não está em um clã.")
	case z.myRank(p) < rankOfficer:
		z.system(p, "Só o líder e os oficiais podem convidar.")
	case t == nil:
		z.system(p, "Esse jogador não está online.")
	case t == p:
		return
	case t.pl.clan != nil:
		z.system(p, fmt.Sprintf("%s já está em um clã.", t.name))
	case len(c.members) >= clanMaxMembers:
		z.system(p, fmt.Sprintf("O clã está cheio (%d membros).", clanMaxMembers))
	case z.clanWait(t) > 0:
		z.system(p, fmt.Sprintf("%s saiu de um clã há pouco e ainda não pode entrar em outro.", t.name))
	case z.pendingInviteTo(t.id) != nil:
		z.system(p, fmt.Sprintf("%s já tem um convite esperando resposta.", t.name))
	default:
		z.sendInvite(p, t, pb.InviteKind_INVITE_KIND_CLAN, c.name, c.id)
	}
}

// clanWait: quanto falta para poder entrar em outro clã (0 = pode).
func (z *Zone) clanWait(p *entity) time.Duration {
	if p.pl.clanLeftAt.IsZero() {
		return 0
	}
	return max(0, clanJoinWait-z.cfg.Clock().Sub(p.pl.clanLeftAt))
}

// clanJoin: p aceitou o convite de from para o clã clanID.
func (z *Zone) clanJoin(from, p *entity, clanID int64) {
	c := z.clans[clanID]
	switch {
	case c == nil:
		z.system(p, "Esse clã não existe mais.")
	case p.pl.clan != nil:
		z.system(p, "Você já está em um clã.")
	case len(c.members) >= clanMaxMembers:
		z.system(p, "O clã já está cheio.")
	case z.clanWait(p) > 0:
		z.system(p, "Você saiu de um clã há pouco e ainda não pode entrar em outro.")
	default:
		m := &clanMember{id: int64(p.id), name: p.name, class: p.pl.class, level: p.level, rank: rankMember}
		c.members[m.id] = m
		z.clanOf[m.id] = c
		p.pl.clan = c
		z.clanOp(ClanOp{Kind: ClanOpJoin, ClanID: c.id, CharacterID: m.id, Rank: rankMember})
		z.clanSystem(c, fmt.Sprintf("%s entrou no clã.", p.name))
		z.broadcastStatus(p)
		z.sendClan(c)
	}
}

// removeMember tira alguém do clã (sair ou ser expulso) e marca a espera.
func (z *Zone) removeMember(c *clan, id int64, msg string) {
	m := c.members[id]
	if m == nil {
		return
	}
	now := z.cfg.Clock()
	delete(c.members, id)
	delete(z.clanOf, id)
	z.clanLeft[id] = now
	z.clanOp(ClanOp{Kind: ClanOpLeave, ClanID: c.id, CharacterID: id, LeftAt: now})
	if p := z.players[uint32(id)]; p != nil {
		p.pl.clan = nil
		p.pl.clanLeftAt = now
		z.system(p, msg)
		z.broadcastStatus(p)
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Clan{Clan: z.clanPB(p)}})
	}
	z.sendClan(c)
}

func (z *Zone) clanLeave(p *entity) {
	c := p.pl.clan
	switch {
	case c == nil:
		z.system(p, "Você não está em um clã.")
	case c.leader == int64(p.id) && len(c.members) > 1:
		z.system(p, "O líder precisa passar a liderança antes de sair (ou desfazer o clã).")
	case c.leader == int64(p.id):
		z.clanDisband(p) // líder sozinho: sair = desfazer
	default:
		z.clanSystem(c, fmt.Sprintf("%s saiu do clã.", p.name))
		z.removeMember(c, int64(p.id), fmt.Sprintf("Você saiu do clã ‹%s›.", c.name))
	}
}

func (z *Zone) clanKick(p *entity, id int64) {
	c := p.pl.clan
	if c == nil {
		return
	}
	t := c.members[id]
	my := z.myRank(p)
	switch {
	case t == nil || id == int64(p.id):
		return
	case my < rankOfficer:
		z.system(p, "Só o líder e os oficiais podem expulsar.")
	case t.rank >= my:
		z.system(p, "Você só pode expulsar quem tem cargo menor que o seu.")
	default:
		z.clanSystem(c, fmt.Sprintf("%s foi expulso do clã por %s.", t.name, p.name))
		z.removeMember(c, id, fmt.Sprintf("Você foi expulso do clã ‹%s›.", c.name))
	}
}

func (z *Zone) clanSetRank(p *entity, id int64, rank int) {
	c := p.pl.clan
	if c == nil {
		return
	}
	t := c.members[id]
	switch {
	case z.myRank(p) != rankLeader:
		z.system(p, "Só o líder pode mudar cargos.")
	case t == nil || t.rank == rankLeader || (rank != rankMember && rank != rankOfficer) || t.rank == rank:
		return
	default:
		t.rank = rank
		z.clanOp(ClanOp{Kind: ClanOpRank, ClanID: c.id, CharacterID: id, Rank: rank})
		if rank == rankOfficer {
			z.clanSystem(c, fmt.Sprintf("%s agora é oficial do clã.", t.name))
		} else {
			z.clanSystem(c, fmt.Sprintf("%s voltou a ser membro do clã.", t.name))
		}
		z.sendClan(c)
	}
}

func (z *Zone) clanPassLeader(p *entity, id int64) {
	c := p.pl.clan
	if c == nil {
		return
	}
	t := c.members[id]
	switch {
	case z.myRank(p) != rankLeader:
		z.system(p, "Só o líder pode passar a liderança.")
	case t == nil || id == int64(p.id):
		return
	default:
		if old := c.members[c.leader]; old != nil {
			old.rank = rankOfficer
		}
		t.rank = rankLeader
		c.leader = id
		z.clanOp(ClanOp{Kind: ClanOpLeader, ClanID: c.id, CharacterID: id})
		z.clanSystem(c, fmt.Sprintf("%s agora é o líder do clã.", t.name))
		z.sendClan(c)
	}
}

func (z *Zone) clanDisband(p *entity) {
	c := p.pl.clan
	switch {
	case c == nil:
		z.system(p, "Você não está em um clã.")
	case z.myRank(p) != rankLeader:
		z.system(p, "Só o líder pode desfazer o clã.")
	default:
		z.clanOp(ClanOp{Kind: ClanOpDisband, ClanID: c.id})
		delete(z.clans, c.id)
		for id := range c.members {
			delete(z.clanOf, id)
			if m := z.players[uint32(id)]; m != nil {
				m.pl.clan = nil
				z.system(m, fmt.Sprintf("O clã ‹%s› foi desfeito.", c.name))
				z.broadcastStatus(m)
				z.sendTo(m, &pb.ServerMessage{Msg: &pb.ServerMessage_Clan{Clan: z.clanPB(m)}})
			}
		}
		z.log.Info("clan disbanded", "clan", c.id, "name", c.name)
	}
}

func (z *Zone) clanNotice(p *entity, text string) {
	c := p.pl.clan
	if c == nil {
		return
	}
	if z.myRank(p) < rankOfficer {
		z.system(p, "Só o líder e os oficiais podem mudar o aviso.")
		return
	}
	text = SanitizeChat(text)
	if r := []rune(text); len(r) > clanNoticeMax {
		text = string(r[:clanNoticeMax])
	}
	c.notice = text
	z.clanOp(ClanOp{Kind: ClanOpNotice, ClanID: c.id, Text: text})
	z.sendClan(c)
}

// ---------- Entrada, saída, chat e protocolo ----------

// clanOnJoin liga o jogador ao clã dele e avisa os colegas que ele entrou.
func (z *Zone) clanOnJoin(p *entity, leftAt time.Time) {
	if t, ok := z.clanLeft[int64(p.id)]; ok && t.After(leftAt) {
		leftAt = t
	}
	p.pl.clanLeftAt = leftAt
	c := z.clanOf[int64(p.id)]
	if c == nil {
		return
	}
	p.pl.clan = c
	m := c.members[int64(p.id)]
	m.name, m.class, m.level = p.name, p.pl.class, p.level
	z.sendClanExcept(c, p.id) // ele recebe o clã no Welcome
}

// clanOnLeave: os colegas veem que ele saiu do jogo.
func (z *Zone) clanOnLeave(p *entity) {
	if c := p.pl.clan; c != nil {
		z.sendClanExcept(c, p.id)
	}
}

// clanLevelUp mantém o nível certo na lista do clã.
func (z *Zone) clanLevelUp(p *entity) {
	if c := p.pl.clan; c != nil {
		if m := c.members[int64(p.id)]; m != nil {
			m.level = p.level
			z.sendClan(c)
		}
	}
}

func (z *Zone) clanMembersOnline(c *clan) []*entity {
	out := make([]*entity, 0, len(c.members))
	for id := range c.members {
		if p := z.players[uint32(id)]; p != nil && p.pl.clan == c {
			out = append(out, p)
		}
	}
	return out
}

func (z *Zone) clanSystem(c *clan, text string) {
	for _, m := range z.clanMembersOnline(c) {
		z.system(m, text)
	}
}

// clanChat manda a mensagem só aos membros do clã.
func (z *Zone) clanChat(p *entity, text string) {
	c := p.pl.clan
	if c == nil {
		z.system(p, "Você não está em um clã.")
		return
	}
	msg := &pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_CLAN, FromId: p.id, FromName: p.name, Text: text,
	}}}
	for _, m := range z.clanMembersOnline(c) {
		z.sendTo(m, msg)
	}
}

func (z *Zone) sendClan(c *clan) { z.sendClanExcept(c, 0) }

func (z *Zone) sendClanExcept(c *clan, skip uint32) {
	for _, m := range z.clanMembersOnline(c) {
		if m.id != skip {
			z.sendTo(m, &pb.ServerMessage{Msg: &pb.ServerMessage_Clan{Clan: z.clanPB(m)}})
		}
	}
}

// clanPB: o clã visto por p (sem clã: requisitos para criar e a espera para entrar).
func (z *Zone) clanPB(p *entity) *pb.ClanState {
	out := &pb.ClanState{MaxMembers: clanMaxMembers, CreateLevel: clanCreateLevel, CreateCost: clanCreateCost}
	c := p.pl.clan
	if c == nil {
		out.JoinWaitMs = uint32(z.clanWait(p).Milliseconds())
		return out
	}
	out.Id, out.Name, out.Notice = uint64(c.id), c.name, c.notice
	out.MyRank = pb.ClanRank(z.myRank(p))
	for _, m := range c.members {
		online := z.players[uint32(m.id)] != nil
		out.Members = append(out.Members, &pb.ClanMember{
			CharacterId: uint32(m.id), Name: m.name, Class: m.class, Level: uint32(m.level),
			Rank: pb.ClanRank(m.rank), Online: online,
		})
	}
	// Cargo, depois online, depois nome.
	slices.SortFunc(out.Members, func(a, b *pb.ClanMember) int {
		if a.Rank != b.Rank {
			return int(b.Rank) - int(a.Rank)
		}
		if a.Online != b.Online {
			if a.Online {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}

package zone

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
)

// Barramento do processo: o que é global entre as zonas (ver manager.go).
//
// Cada zona continua dona do estado dos seus jogadores (bolsa, vida, posição...). O
// barramento guarda só o que precisa ser visto de qualquer zona, atrás de um mutex curto
// (nada de banco nem de espera segurando a trava):
//   - quem está online, em que zona e por qual conexão: chat geral, sussurro, /online,
//     moderação, amigos online e a rota do login;
//   - os grupos: membros e líder valem para todas as zonas. Cada zona tem uma cópia
//     (z.parties), trocada pelas operações dela e conferida 1×/s (party_zones.go).
//
// Mensagem para quem está em outra zona sai direto pela conexão (Peer.Send não bloqueia e
// é seguro entre goroutines): nenhuma zona espera outra, e não há evento cruzado que
// possa travar duas zonas uma esperando a outra.
type Bus struct {
	mu       sync.RWMutex
	players  map[uint32]*busPlayer
	byName   map[string]uint32 // nome em minúsculas → id
	accounts map[int64]int     // conta → personagens online (o armazém fica na memória da cidade)
	last     map[uint32]lastZone
	parties  map[uint32]*busParty
	partySeq uint32
	broken   int // invariantes do barramento quebradas (modo de produção; ver check)
}

// busPlayer: um personagem online (ou atravessando um portão).
type busPlayer struct {
	id      uint32
	name    string
	class   string
	account int64
	zone    *Zone // a zona dona (no portão, já a de destino)
	peer    Peer
	// in: a zona que tem o jogador em z.players agora (nil = entregue pelo portão, na fila
	// da zona de destino: pending). Dona única: nunca duas, e nil só com pending.
	in      *Zone
	pending bool
	// transit: atravessando o portão. Fecha quando a zona de destino recebeu a entrada
	// (o login espera, para nunca entrar na frente da entrega do portão).
	transit chan struct{}
	party   uint32 // grupo (0 = nenhum)
	note    string // aviso para a zona dele na próxima conferência do grupo (ex.: foi removido)
	sum     memberSummary
}

// memberSummary: o que a janela do grupo mostra de quem está em outra zona (publicado
// pela zona dele 1×/s).
type memberSummary struct {
	level, hp, maxHP, mp, maxMP int
	dead                        bool
}

// lastZone: onde o personagem estava ao sair (o recent daquela zona vale mais que o banco).
type lastZone struct {
	zone *Zone
	at   time.Time
}

// busParty: o grupo de verdade. A ordem dos membros é a de entrada.
type busParty struct {
	id      uint32
	leader  uint32
	members []uint32
}

// partyView: cópia de um grupo do barramento (para a zona aplicar na dela).
type partyView struct {
	id      uint32
	leader  uint32
	members []uint32
}

func newBus() *Bus {
	return &Bus{
		players:  map[uint32]*busPlayer{},
		byName:   map[string]uint32{},
		accounts: map[int64]int{},
		last:     map[uint32]lastZone{},
		parties:  map[uint32]*busParty{},
	}
}

// ---------- Quem está onde ----------

// register: o jogador entrou nesta zona (login ou chegada pelo portão). Devolve a quebra
// da dona única ("" = certo): ele ainda estava em outra zona.
func (b *Bus) register(p *entity, z *Zone) (bad string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.players[p.id]
	if e != nil && e.in != nil && e.in != z {
		bad = fmt.Sprintf("entrou em %s ainda estando em %s", z.mapID(), e.in.mapID())
	}
	if e == nil {
		e = &busPlayer{id: p.id, account: p.pl.accountID}
		b.players[p.id] = e
		b.accounts[e.account]++
	}
	if e.name != "" && b.byName[strings.ToLower(e.name)] == p.id {
		delete(b.byName, strings.ToLower(e.name))
	}
	e.name, e.class, e.zone, e.peer = p.name, p.pl.class, z, p.pl.peer
	e.sum = summaryOf(p)
	e.in, e.pending = z, false
	b.byName[strings.ToLower(p.name)] = p.id
	delete(b.last, p.id)
	return bad
}

// left: o jogador saiu de z.players de z (pelo portão ou de vez).
func (b *Bus) left(id uint32, z *Zone) {
	b.mu.Lock()
	if e := b.players[id]; e != nil && e.in == z {
		e.in = nil
	}
	b.mu.Unlock()
}

// unregister: saiu do jogo. Só vale para a mesma zona e conexão (um relogin já pode ter
// registrado a sessão nova).
func (b *Bus) unregister(id uint32, z *Zone, peer Peer, at time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.players[id]
	if e == nil || e.zone != z || e.peer != peer {
		return
	}
	b.removePartyLocked(e, "")
	delete(b.players, id)
	if b.byName[strings.ToLower(e.name)] == id {
		delete(b.byName, strings.ToLower(e.name))
	}
	if b.accounts[e.account]--; b.accounts[e.account] <= 0 {
		delete(b.accounts, e.account)
	}
	b.last[id] = lastZone{zone: z, at: at}
	if len(b.last) > 4096 { // esquece quem saiu há mais tempo que o recent das zonas
		for k, l := range b.last {
			if at.Sub(l.at) > recentKeep {
				delete(b.last, k)
			}
		}
	}
}

// moveTo: o portão entregou o jogador à zona dest (a de origem já o tirou).
func (b *Bus) moveTo(id uint32, dest *Zone) chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.players[id]
	if e == nil {
		return nil
	}
	e.zone = dest
	e.pending = true
	e.transit = make(chan struct{})
	return e.transit
}

// landed: a entrada já está na fila da zona de destino (o login pode seguir).
func (b *Bus) landed(id uint32, ch chan struct{}) {
	b.mu.Lock()
	if e := b.players[id]; e != nil && e.transit == ch {
		e.transit = nil
	}
	b.mu.Unlock()
	close(ch)
}

// route: para qual zona mandar o login do personagem (nil = a do banco). wait != nil:
// ele está atravessando um portão; espere e pergunte de novo.
func (b *Bus) route(id uint32, now time.Time) (z *Zone, wait chan struct{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if e := b.players[id]; e != nil {
		return e.zone, e.transit
	}
	if l, ok := b.last[id]; ok && now.Sub(l.at) <= recentKeep {
		return l.zone, nil
	}
	return nil, nil
}

// zoneOf: a zona do personagem online (nil = offline).
func (b *Bus) zoneOf(id uint32) *Zone {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if e := b.players[id]; e != nil {
		return e.zone
	}
	return nil
}

// find: o personagem online com esse nome (sem diferenciar maiúsculas).
func (b *Bus) find(name string) (id uint32, display string, z *Zone, ok bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	id, ok = b.byName[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return 0, "", nil, false
	}
	e := b.players[id]
	return id, e.name, e.zone, true
}

func (b *Bus) online(id uint32) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.players[id] != nil
}

func (b *Bus) accountOnline(acc int64) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.accounts[acc] > 0
}

// count: personagens online em todas as zonas (inclui quem atravessa um portão).
func (b *Bus) count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.players)
}

// names: "Nome (mapa)" de todos os online, para o /online do GM.
func (b *Bus) names() []string {
	b.mu.RLock()
	out := make([]string, 0, len(b.players))
	for _, e := range b.players {
		out = append(out, e.name+" ("+e.zone.mapTitle()+")")
	}
	b.mu.RUnlock()
	slices.SortFunc(out, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return out
}

// ---------- Mensagens entre zonas ----------

// send entrega bytes já serializados. Fila cheia derruba a conexão (como dropSlow): o
// readPump termina e a zona dona recebe o Leave.
func send(peer Peer, data []byte) {
	if !peer.Send(data) {
		peer.Close("send queue full")
	}
}

// sendTo manda a mensagem a um personagem em qualquer zona (false = offline).
func (b *Bus) sendTo(id uint32, data []byte) bool {
	b.mu.RLock()
	e := b.players[id]
	var peer Peer
	if e != nil {
		peer = e.peer
	}
	b.mu.RUnlock()
	if peer == nil {
		return false
	}
	send(peer, data)
	return true
}

// broadcast manda os mesmos bytes a todos os online, em todas as zonas.
func (b *Bus) broadcast(data []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, e := range b.players {
		send(e.peer, data) // Send não bloqueia: pode ficar dentro da trava de leitura
	}
}

// broadcastAll: o chat geral e os anúncios de GM vão para todas as zonas (sem gerente,
// só para esta, como sempre foi).
func (z *Zone) broadcastAll(msg *pb.ServerMessage) {
	if z.mgr == nil {
		z.broadcast(msg, 0)
		return
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		z.log.Error("marshal", "err", err)
		return
	}
	z.mgr.bus.broadcast(data)
}

// whisperElsewhere: o sussurro para quem está em outra zona (false = não está online).
func (z *Zone) whisperElsewhere(p *entity, to, text string) bool {
	if z.mgr == nil {
		return false
	}
	id, name, _, ok := z.mgr.bus.find(to)
	if !ok {
		return false
	}
	msg := &pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_WHISPER, FromId: p.id, FromName: p.name, ToName: name, Text: text,
	}}}
	data, err := proto.Marshal(msg)
	if err != nil {
		return false
	}
	if !z.mgr.bus.sendTo(id, data) {
		return false
	}
	if id != p.id {
		z.sendTo(p, msg) // eco para quem enviou
	}
	return true
}

// elsewhere: o nome do mapa onde o personagem está, se estiver online em outra zona.
func (z *Zone) elsewhere(name string) string {
	if z.mgr == nil {
		return ""
	}
	if _, _, oz, ok := z.mgr.bus.find(name); ok && oz != z {
		return oz.mapTitle()
	}
	return ""
}

// systemTo: aviso do sistema para um personagem em qualquer zona.
func (b *Bus) systemTo(id uint32, text string) {
	data, err := proto.Marshal(&pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_SYSTEM, Text: text,
	}}})
	if err == nil {
		b.sendTo(id, data)
	}
}

// ---------- Grupos ----------

// partyAdd: newID aceitou o convite de leaderID (que pode ainda não ter grupo).
func (b *Bus) partyAdd(leaderID, newID uint32) (partyView, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	l, t := b.players[leaderID], b.players[newID]
	switch {
	case l == nil || t == nil:
		return partyView{}, "Este convite não vale mais."
	case t.party != 0:
		return partyView{}, "Você já está em um grupo."
	}
	pt := b.parties[l.party]
	if pt == nil {
		b.partySeq++
		pt = &busParty{id: b.partySeq, leader: leaderID, members: []uint32{leaderID}}
		b.parties[pt.id] = pt
		l.party = pt.id
	}
	switch {
	case pt.leader != leaderID:
		return partyView{}, "Quem convidou não é mais o líder do grupo."
	case len(pt.members) >= partyMax:
		return partyView{}, "O grupo já está cheio."
	}
	pt.members = append(pt.members, newID)
	t.party = pt.id
	return pt.view(), ""
}

// partyRemove tira o personagem do grupo dele. ended: sobrou um e o grupo acabou (a
// view é a de antes de acabar, para avisar quem sobrou).
func (b *Bus) partyRemove(id uint32, note string) (v partyView, ended, was bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.players[id]
	if e == nil || e.party == 0 {
		return partyView{}, false, false
	}
	v, ended = b.removePartyLocked(e, note)
	return v, ended, true
}

func (b *Bus) removePartyLocked(e *busPlayer, note string) (partyView, bool) {
	pt := b.parties[e.party]
	e.party = 0
	if pt == nil {
		return partyView{}, false
	}
	pt.members = slices.DeleteFunc(pt.members, func(m uint32) bool { return m == e.id })
	if note != "" {
		e.note = note
	}
	if len(pt.members) <= 1 {
		v := pt.view()
		for _, m := range pt.members {
			if o := b.players[m]; o != nil {
				o.party = 0
			}
		}
		delete(b.parties, pt.id)
		return v, true
	}
	if pt.leader == e.id {
		pt.leader = pt.members[0]
	}
	return pt.view(), false
}

// partyKick: o líder tira alguém (que pode estar em outra zona).
func (b *Bus) partyKick(by, target uint32, note string) (v partyView, ended bool, errText string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	l, t := b.players[by], b.players[target]
	pt := b.parties[l.partyOrZero()]
	switch {
	case pt == nil:
		return partyView{}, false, "Você não está em um grupo."
	case pt.leader != by:
		return partyView{}, false, "Só o líder pode remover alguém do grupo."
	case t == nil || t.party != pt.id || target == by:
		return partyView{}, false, "-"
	}
	v, ended = b.removePartyLocked(t, note)
	return v, ended, ""
}

// partyLead passa a liderança.
func (b *Bus) partyLead(by, target uint32) (partyView, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	l, t := b.players[by], b.players[target]
	pt := b.parties[l.partyOrZero()]
	switch {
	case pt == nil || pt.leader != by:
		return partyView{}, "Só o líder pode passar a liderança."
	case t == nil || t.party != pt.id || target == by:
		return partyView{}, "-"
	}
	pt.leader = target
	return pt.view(), ""
}

func (e *busPlayer) partyOrZero() uint32 {
	if e == nil {
		return 0
	}
	return e.party
}

// party: a versão atual do grupo (ok = false: acabou).
func (b *Bus) party(id uint32) (partyView, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	pt := b.parties[id]
	if pt == nil {
		return partyView{}, false
	}
	return pt.view(), true
}

// partyOf: o grupo do personagem no barramento (0 = nenhum).
func (b *Bus) partyOf(id uint32) uint32 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.players[id].partyOrZero()
}

// takeNote devolve (e apaga) o aviso guardado para o personagem.
func (b *Bus) takeNote(id uint32) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.players[id]
	if e == nil {
		return ""
	}
	n := e.note
	e.note = ""
	return n
}

func (pt *busParty) view() partyView {
	return partyView{id: pt.id, leader: pt.leader, members: slices.Clone(pt.members)}
}

// remoteMember: o que a janela do grupo mostra de um membro que não está em z.
type remoteMember struct {
	name, class, mapTitle string
	sum                   memberSummary
}

// remoteMembers: os membros da lista que estão em outra zona (ou atravessando um portão).
func (b *Bus) remoteMembers(members []uint32, z *Zone, out []remoteMemberAt) []remoteMemberAt {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, id := range members {
		if e := b.players[id]; e != nil && e.zone != z {
			title := e.zone.mapTitle()
			if e.transit != nil {
				title = "viajando"
			}
			out = append(out, remoteMemberAt{id: id, m: remoteMember{name: e.name, class: e.class, mapTitle: title, sum: e.sum}})
		}
	}
	return out
}

type remoteMemberAt struct {
	id uint32
	m  remoteMember
}

// publish: a zona atualiza vida, mana e nível dos seus membros de grupo (1×/s).
func (b *Bus) publish(ps []*entity) {
	if len(ps) == 0 {
		return
	}
	b.mu.Lock()
	for _, p := range ps {
		if e := b.players[p.id]; e != nil {
			e.sum = summaryOf(p)
		}
	}
	b.mu.Unlock()
}

func summaryOf(p *entity) memberSummary {
	return memberSummary{level: p.level, hp: p.hpInt(), maxHP: p.maxHPInt(), mp: p.mpInt(), maxMP: p.maxMPInt(), dead: p.dead}
}

// zonesOf: as zonas (fora skip) onde estão os membros da lista.
func (b *Bus) zonesOf(members []uint32, skip *Zone) []*Zone {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []*Zone
	for _, id := range members {
		if e := b.players[id]; e != nil && e.zone != skip && !slices.Contains(out, e.zone) {
			out = append(out, e.zone)
		}
	}
	return out
}

// sendMembers manda os bytes aos membros que estão em outra zona que não skip.
func (b *Bus) sendMembers(members []uint32, skip *Zone, data []byte, except uint32) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, id := range members {
		if e := b.players[id]; e != nil && e.zone != skip && id != except {
			send(e.peer, data)
		}
	}
}

// ---------- Invariantes do barramento ----------

// check confere o barramento: cada grupo tem de 2 a 6 membros, o líder está nele, e cada
// membro está online apontando para o grupo; quem aponta para um grupo está nele. Devolve
// a primeira quebra ("" = tudo certo).
func (b *Bus) check() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for id, pt := range b.parties {
		switch {
		case pt.id != id:
			return fmt.Sprintf("grupo %d na chave %d", pt.id, id)
		case len(pt.members) < 2 || len(pt.members) > partyMax:
			return fmt.Sprintf("grupo %d com %d membros", id, len(pt.members))
		case !slices.Contains(pt.members, pt.leader):
			return fmt.Sprintf("grupo %d: líder %d fora do grupo", id, pt.leader)
		}
		for _, m := range pt.members {
			if e := b.players[m]; e == nil || e.party != id {
				return fmt.Sprintf("grupo %d: membro %d não aponta para o grupo", id, m)
			}
		}
	}
	for id, e := range b.players {
		if e.party != 0 {
			if pt := b.parties[e.party]; pt == nil || !slices.Contains(pt.members, id) {
				return fmt.Sprintf("personagem %d aponta para o grupo %d sem estar nele", id, e.party)
			}
		}
		if b.byName[strings.ToLower(e.name)] != id {
			return fmt.Sprintf("personagem %d (%s) fora do índice de nomes", id, e.name)
		}
		if e.in == nil && !e.pending {
			return fmt.Sprintf("personagem %d (%s) online sem zona", id, e.name)
		}
	}
	return ""
}

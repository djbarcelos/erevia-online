package zone

import (
	"fmt"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Armazém: guardado no NPC da praça e compartilhado pelos personagens da conta. Há um só
// por conta na memória (mesmo com dois personagens da conta online). Ele vai para o banco
// junto com o personagem que mexeu, na mesma transação, com o próprio número de ordem
// (storage_seq): um armazém mais velho nunca sobrescreve um mais novo.
const (
	StorageSize  = 60
	placeStorage = 2 // lugar do armazém nas mensagens (bolsa = 0, equipado = 1)
	storageKeep  = 10 * time.Minute
)

// StorageData é o armazém carregado do banco (na entrada do personagem).
type StorageData struct {
	Gold  int64
	Items []game.ItemRecord // Equipped é sempre false
	Seq   int64
}

// StorageSave vai junto do SavedState quando o armazém mudou.
type StorageSave struct {
	AccountID int64
	Gold      int64
	Items     []game.ItemRecord
	Seq       int64
}

type storage struct {
	account int64
	slots   [StorageSize]stack
	gold    int64
	dirty   bool      // mudou desde o último salvamento
	touched time.Time // última vez que alguém da conta esteve online (para esquecer)
}

func newStorage(account int64, d *StorageData) *storage {
	st := &storage{account: account}
	if d == nil {
		return st
	}
	st.gold = max(0, d.Gold)
	for _, r := range d.Items {
		def := game.Items[r.ItemID]
		if def == nil || def.Kind == game.KindGold || r.Count <= 0 {
			continue
		}
		ench := 0
		if def.Stack == 1 {
			ench = min(max(r.Enchant, 0), game.MaxEnchant)
		}
		s := stack{def: def, count: min(r.Count, def.Stack), enchant: ench}
		if r.Slot >= 0 && r.Slot < StorageSize && st.slots[r.Slot].empty() {
			st.slots[r.Slot] = s
		} else if i := firstEmpty(st.slots[:]); i >= 0 {
			st.slots[i] = s
		}
	}
	return st
}

func (st *storage) records() []game.ItemRecord {
	var out []game.ItemRecord
	for i, s := range st.slots {
		if !s.empty() {
			out = append(out, game.ItemRecord{Slot: i, ItemID: s.def.ID, Count: s.count, Enchant: s.enchant})
		}
	}
	return out
}

func firstEmpty(slots []stack) int {
	for i, s := range slots {
		if s.empty() {
			return i
		}
	}
	return -1
}

// putStack guarda s em slots: no espaço preferido (vazio ou mesma pilha), senão completa
// pilhas e ocupa espaços vazios. Devolve quantas unidades não couberam.
func putStack(slots []stack, s stack, prefer int) int {
	left := s.count
	stackable := s.def.Stack > 1 && s.enchant == 0
	if prefer >= 0 && prefer < len(slots) {
		t := &slots[prefer]
		switch {
		case t.empty():
			n := min(left, s.def.Stack)
			*t = stack{def: s.def, count: n, enchant: s.enchant}
			left -= n
		case stackable && t.def == s.def && t.enchant == 0:
			n := min(left, s.def.Stack-t.count)
			t.count += n
			left -= n
		}
	}
	if left > 0 && stackable {
		for i := range slots {
			t := &slots[i]
			if !t.empty() && t.def == s.def && t.enchant == 0 && t.count < s.def.Stack {
				n := min(left, s.def.Stack-t.count)
				t.count += n
				left -= n
				if left == 0 {
					return 0
				}
			}
		}
	}
	for i := range slots {
		if left == 0 {
			break
		}
		if slots[i].empty() {
			n := min(left, s.def.Stack)
			slots[i] = stack{def: s.def, count: n, enchant: s.enchant}
			left -= n
		}
	}
	return left
}

// room: quantas unidades de s cabem nos espaços.
func room(slots []stack, s stack) int {
	n := 0
	for _, t := range slots {
		switch {
		case t.empty():
			n += s.def.Stack
		case s.def.Stack > 1 && s.enchant == 0 && t.def == s.def && t.enchant == 0:
			n += s.def.Stack - t.count
		}
	}
	return n
}

// storageOf: o armazém da conta (cria a partir do que veio do banco na primeira vez).
func (z *Zone) storageOf(p *entity) *storage {
	return z.storages[p.pl.accountID]
}

// storageOnJoin: se outro personagem da conta está online (ou saiu há pouco), vale o da
// memória; senão, o que veio do banco.
func (z *Zone) storageOnJoin(p *entity, d *StorageData) {
	if p.pl.accountID == 0 {
		return
	}
	if z.cfg.NoTown { // mapa sem armazém: só guarda como veio, para levar de volta à cidade
		p.pl.hop.storage = d
		return
	}
	st := z.storages[p.pl.accountID]
	if st == nil {
		st = newStorage(p.pl.accountID, d)
		z.storages[p.pl.accountID] = st
	}
	st.touched = z.cfg.Clock()
}

// pruneStorages esquece armazéns de contas sem ninguém online há um tempo (já salvos).
func (z *Zone) pruneStorages() {
	online := map[int64]bool{}
	for _, p := range z.players {
		online[p.pl.accountID] = true
	}
	now := z.cfg.Clock()
	for acc, st := range z.storages {
		// Online em qualquer zona: o armazém fica (só a cidade o muda; ver gate.go).
		if online[acc] || z.mgr != nil && z.mgr.bus.accountOnline(acc) {
			st.touched = now
			continue
		}
		if !st.dirty && now.Sub(st.touched) > storageKeep {
			delete(z.storages, acc)
		}
	}
}

// storageSave: o armazém vai junto do personagem se mudou (uma vez por mudança).
func (z *Zone) storageSave(p *entity) *StorageSave {
	st := z.storageOf(p)
	if st == nil || !st.dirty {
		return nil
	}
	st.dirty = false
	return &StorageSave{AccountID: st.account, Gold: st.gold, Items: st.records(), Seq: z.nextSeq()}
}

func (z *Zone) storagePB(st *storage, npcID uint32) *pb.Storage {
	out := &pb.Storage{NpcId: npcID, Gold: uint64(st.gold), Size: StorageSize}
	for i, s := range st.slots {
		if !s.empty() {
			out.Items = append(out.Items, &pb.ItemStack{Place: placeStorage, Slot: uint32(i), ItemId: s.def.ID, Count: uint32(s.count), Enchant: uint32(s.enchant)})
		}
	}
	return out
}

// storageNPC: o Guarda do Armazém perto do jogador (senão avisa e devolve nil).
func (z *Zone) storageNPC(p *entity, id uint32) *entity {
	n := z.shopNPC(p, id)
	if n == nil || n.npc.role != pb.NpcRole_NPC_ROLE_STORAGE {
		return nil
	}
	return n
}

// storageChanged: avisa quem está com o armazém aberto da conta e agenda o salvamento.
func (z *Zone) storageChanged(p *entity, st *storage, npcID uint32) {
	st.dirty = true
	z.saveSoon(p)
	msg := &pb.ServerMessage{Msg: &pb.ServerMessage_Storage{Storage: z.storagePB(st, npcID)}}
	for _, o := range z.players {
		if o.pl.accountID == st.account {
			z.sendTo(o, msg)
		}
	}
}

// storageGold guarda (amount > 0) ou retira (< 0) moedas; amount 0 só abre o armazém.
func (z *Zone) storageGold(p *entity, npcID uint32, amount int64) {
	n := z.storageNPC(p, npcID)
	st := z.storageOf(p)
	if n == nil || st == nil || p.dead {
		return
	}
	inv := &p.pl.inv
	switch {
	case amount == 0:
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Storage{Storage: z.storagePB(st, n.id)}})
		return
	case amount > 0:
		amount = min(amount, inv.gold)
		if amount <= 0 {
			return
		}
		inv.gold -= amount
		st.gold += amount
		z.system(p, fmt.Sprintf("Você guardou %d moedas.", amount))
	default:
		amount = min(-amount, st.gold)
		if amount <= 0 {
			return
		}
		st.gold -= amount
		inv.gold += amount
		z.system(p, fmt.Sprintf("Você retirou %d moedas.", amount))
	}
	z.inventoryChanged(p, false)
	z.storageChanged(p, st, n.id)
}

// storageMove move entre a bolsa e o armazém (ou dentro do armazém). count 0 = a pilha toda.
func (z *Zone) storageMove(p *entity, m *pb.StorageMove) {
	n := z.storageNPC(p, m.NpcId)
	st := z.storageOf(p)
	if n == nil || st == nil || p.dead {
		return
	}
	inv := &p.pl.inv
	from, to := int(m.FromSlot), int(m.ToSlot)
	switch {
	case m.FromPlace == placeBag && m.ToPlace == placeStorage:
		if from < 0 || from >= game.BagSize || inv.bag[from].empty() {
			return
		}
		s := inv.bag[from]
		count := s.count
		if m.Count > 0 {
			count = min(int(m.Count), s.count)
		}
		moving := stack{def: s.def, count: count, enchant: s.enchant}
		if room(st.slots[:], moving) < count {
			z.system(p, "O armazém está cheio.")
			return
		}
		inv.take(from, count)
		putStack(st.slots[:], moving, to)
		z.system(p, fmt.Sprintf("Você guardou %s.", itemLabelEnchant(s.def, count, s.enchant)))
	case m.FromPlace == placeStorage && m.ToPlace == placeBag:
		if from < 0 || from >= StorageSize || st.slots[from].empty() {
			return
		}
		s := st.slots[from]
		count := s.count
		if m.Count > 0 {
			count = min(int(m.Count), s.count)
		}
		moving := stack{def: s.def, count: count, enchant: s.enchant}
		if room(inv.bag[:], moving) < count {
			z.system(p, "Sua bolsa está cheia.")
			return
		}
		st.slots[from].count -= count
		if st.slots[from].count <= 0 {
			st.slots[from] = stack{}
		}
		putStack(inv.bag[:], moving, to)
		z.system(p, fmt.Sprintf("Você retirou %s.", itemLabelEnchant(s.def, count, s.enchant)))
	case m.FromPlace == placeStorage && m.ToPlace == placeStorage:
		if from < 0 || from >= StorageSize || to < 0 || to >= StorageSize || from == to || st.slots[from].empty() {
			return
		}
		a, b := &st.slots[from], &st.slots[to]
		if !b.empty() && a.def == b.def && a.def.Stack > 1 && a.enchant == 0 && b.enchant == 0 {
			put := min(a.count, a.def.Stack-b.count)
			b.count += put
			a.count -= put
			if a.count == 0 {
				*a = stack{}
			}
		} else {
			*a, *b = *b, *a
		}
		z.storageChanged(p, st, n.id)
		return
	default:
		return
	}
	z.inventoryChanged(p, false)
	z.storageChanged(p, st, n.id)
}

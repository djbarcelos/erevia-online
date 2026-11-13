package zone

import (
	"fmt"
	"math"
	"sort"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const (
	// itemCooldownBase separa as recargas de consumíveis das de skills no mesmo mapa.
	itemCooldownBase = 1000
	// saveDelay: depois de uma troca de itens ou moedas, salva em até este tempo.
	saveDelay = 2.0
)

// itemEvent carrega um pedido de item do cliente (*pb.PickUp, *pb.MoveItem, …).
type itemEvent struct {
	id  uint32
	msg any
}

func (itemEvent) isEvent() {}

func (z *Zone) onItem(e itemEvent) {
	p := z.players[e.id]
	if p == nil || !z.spendAction(p) {
		return
	}
	switch m := e.msg.(type) {
	case *pb.PickUp:
		z.requestPickup(p, m.Id)
	case *pb.MoveItem:
		z.moveItem(p, int(m.FromPlace), int(m.FromSlot), int(m.ToPlace), int(m.ToSlot))
	case *pb.UseItem:
		z.useItem(p, int(m.Place), int(m.Slot))
	case *pb.DestroyItem:
		z.destroyItem(p, int(m.Slot), int(m.Count))
	case *pb.Buy:
		z.buy(p, m.NpcId, m.ItemId, int(m.Count))
	case *pb.Sell:
		z.sell(p, m.NpcId, int(m.Slot), int(m.Count))
	case *pb.Teleport:
		if !p.dead {
			z.startTeleport(p, m.NpcId, m.DestId)
		}
	case *pb.CancelChannel:
		z.cancelChannel(p, "Viagem cancelada.")
	case *pb.StorageMove:
		z.storageMove(p, m)
	case *pb.StorageGold:
		z.storageGold(p, m.NpcId, m.Amount)
	case *pb.Enchant:
		z.enchant(p, int(m.ScrollSlot), int(m.Place), int(m.Slot))
	case *pb.ChangeLook:
		z.changeLook(p, m.NpcId, m.Look)
	case *pb.Craft:
		z.craft(p, m.NpcId, m.RecipeId)
	}
}

// inventoryChanged avisa o dono, agenda o salvamento e, se a arma mudou, mostra a todos.
func (z *Zone) inventoryChanged(p *entity, gearChanged bool) {
	if gearChanged {
		z.refreshStats(p)
		z.broadcast(appearancePB(p), 0)
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Inventory{Inventory: inventoryPB(&p.pl.inv)}})
	z.saveSoon(p)
	z.tradeInventoryChanged(p)
}

func (z *Zone) saveSoon(p *entity) {
	if p.pl.saveAt == 0 {
		p.pl.saveAt = z.now + saveDelay
	}
}

func (z *Zone) moveItem(p *entity, fromPlace, fromSlot, toPlace, toSlot int) {
	if p.dead {
		return
	}
	inv := &p.pl.inv
	var why string
	gear := false
	switch {
	case fromPlace == placeBag && toPlace == placeBag:
		if !inv.moveInBag(fromSlot, toSlot) {
			return
		}
	case fromPlace == placeBag && toPlace == placeEquip:
		if fromSlot < 0 || fromSlot >= game.BagSize || inv.bag[fromSlot].empty() {
			return
		}
		why = inv.equipTo(fromSlot, toSlot, p.pl.class, p.level)
		gear = why == ""
	case fromPlace == placeEquip && toPlace == placeBag:
		why = inv.unequip(fromSlot, toSlot, p.pl.class, p.level)
		gear = why == ""
	case fromPlace == placeEquip && toPlace == placeEquip:
		why = inv.moveInEquip(fromSlot, toSlot)
		gear = why == ""
	default:
		return
	}
	if why != "" {
		if why != "?" {
			z.system(p, why)
		}
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Inventory{Inventory: inventoryPB(inv)}}) // desfaz no cliente
		return
	}
	z.inventoryChanged(p, gear)
}

func (z *Zone) useItem(p *entity, place, slot int) {
	if p.dead {
		return
	}
	inv := &p.pl.inv
	if place == placeEquip {
		if why := inv.unequip(slot, -1, p.pl.class, p.level); why != "" {
			if why != "?" {
				z.system(p, why)
			}
			return
		}
		z.inventoryChanged(p, true)
		return
	}
	if slot < 0 || slot >= game.BagSize || inv.bag[slot].empty() {
		return
	}
	def := inv.bag[slot].def
	switch {
	case def.Equippable():
		if why := inv.equipFromBag(slot, p.pl.class, p.level); why != "" {
			if why != "?" {
				z.system(p, why)
			}
			return
		}
		z.inventoryChanged(p, true)
	case def.Kind == game.KindPotion:
		z.drink(p, slot, def)
	case def.Kind == game.KindScroll && def.Scroll == game.ScrollReturn:
		z.startReturn(p, def)
	case def.Kind == game.KindScroll:
		z.system(p, "Escolha o item que quer encantar (clique nele depois do pergaminho).")
	}
}

// drink usa uma poção: cura na hora e inicia a recarga do grupo (vida ou mana).
func (z *Zone) drink(p *entity, slot int, def *game.ItemDef) {
	key := uint32(itemCooldownBase + def.Cooldown)
	if ready := p.pl.cooldowns[key]; z.now < ready {
		z.sendItemCooldown(p, def.Cooldown, ready-z.now)
		return
	}
	hpFull := p.hp >= p.stats.MaxHP
	mpFull := p.mp >= p.stats.MaxMP
	if (def.HealHP == 0 || hpFull) && (def.HealMP == 0 || mpFull) {
		if def.HealHP > 0 {
			z.system(p, "Sua vida já está cheia.")
		} else {
			z.system(p, "Sua mana já está cheia.")
		}
		return
	}
	p.pl.inv.take(slot, 1)
	z.questEvent(p, game.GoalPotion, def.ID)
	p.hp = math.Min(p.stats.MaxHP, p.hp+def.HealHP)
	p.mp = math.Min(p.stats.MaxMP, p.mp+def.HealMP)
	p.pl.cooldowns[key] = z.now + game.CooldownSeconds(def.Cooldown)
	p.pl.statsDirty = true
	z.sendItemCooldown(p, def.Cooldown, game.CooldownSeconds(def.Cooldown))
	z.inventoryChanged(p, false)
}

func (z *Zone) destroyItem(p *entity, slot, count int) {
	inv := &p.pl.inv
	if slot < 0 || slot >= game.BagSize || inv.bag[slot].empty() {
		return
	}
	s := inv.bag[slot]
	count = min(max(count, 1), s.count)
	inv.take(slot, count)
	z.system(p, fmt.Sprintf("Você destruiu %s.", itemLabel(s.def, count)))
	z.inventoryChanged(p, false)
}

func (z *Zone) sendItemCooldown(p *entity, group int, seconds float64) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_ItemCooldown{ItemCooldown: &pb.ItemCooldown{
		Group: uint32(group), ReadyInMs: uint32(math.Max(0, seconds) * 1000), TotalMs: uint32(game.CooldownSeconds(group) * 1000),
	}}})
}

// sendAnyCooldown envia a recarga de uma skill ou de um grupo de consumíveis.
func (z *Zone) sendAnyCooldown(p *entity, key uint32, seconds float64) {
	if key >= itemCooldownBase {
		z.sendItemCooldown(p, int(key-itemCooldownBase), seconds)
		return
	}
	z.sendCooldown(p, key, seconds)
}

// itemLabelEnchant: como itemLabel, com o "+N" de encantamento.
func itemLabelEnchant(def *game.ItemDef, count, enchant int) string {
	if enchant > 0 {
		return game.EnchantedName(def, enchant)
	}
	return itemLabel(def, count)
}

// itemLabel: "Osso Velho" ou "3× Osso Velho" (moedas: "25 moedas").
func itemLabel(def *game.ItemDef, count int) string {
	if def.Kind == game.KindGold {
		return fmt.Sprintf("%d moedas", count)
	}
	if count == 1 {
		return def.Name
	}
	return fmt.Sprintf("%d× %s", count, def.Name)
}

// ---------- Protocolo ----------

func inventoryPB(inv *inventory) *pb.Inventory {
	out := &pb.Inventory{Gold: uint64(inv.gold), BagSize: game.BagSize}
	for _, r := range inv.records() {
		place := uint32(placeBag)
		if r.Equipped {
			place = placeEquip
		}
		out.Items = append(out.Items, &pb.ItemStack{Place: place, Slot: uint32(r.Slot), ItemId: r.ItemID, Count: uint32(r.Count), Enchant: uint32(r.Enchant)})
	}
	return out
}

func itemInfoPB(d *game.ItemDef) *pb.ItemInfo {
	b := d.Bonus
	info := &pb.ItemInfo{
		Id: d.ID, Name: d.Name, Kind: pb.ItemKind(d.Kind), Grade: uint32(d.Grade), Slot: int32(d.Slot()),
		Classes: d.Classes, ReqLevel: uint32(d.ReqLevel),
		Bonus: &pb.ItemBonus{
			MaxHp: int32(b.MaxHP), MaxMp: int32(b.MaxMP), PAtk: int32(b.PAtk), MAtk: int32(b.MAtk),
			PDef: int32(b.PDef), MDef: int32(b.MDef), Crit: float32(b.Crit),
			Accuracy: float32(b.Accuracy), Evasion: float32(b.Evasion), Speed: float32(b.Speed),
			AtkSpeed: float32(b.AtkSpeed), CastSpeed: float32(b.CastSpeed), HpRegen: float32(b.HPRegen),
			MpRegen: float32(b.MPRegen), CritDamage: float32(b.CritDamage),
		},
		WeaponType: d.WeaponType,
		Stack:      uint32(min(d.Stack, game.MaxStack)), Price: uint32(d.Price), SellPrice: uint32(d.SellPrice()),
		Icon: d.Icon, Description: d.Description, CooldownGroup: uint32(d.Cooldown),
		HealHp: uint32(d.HealHP), HealMp: uint32(d.HealMP), Model: d.Model, OffModel: d.OffModel,
		Scroll: pb.ScrollKind(d.Scroll), Blessed: d.Blessed,
	}
	for _, sl := range d.Slots() {
		info.Slots = append(info.Slots, uint32(sl))
	}
	if d.Cooldown != game.CooldownNone {
		info.CooldownMs = uint32(game.CooldownSeconds(d.Cooldown) * 1000)
	}
	return info
}

// itemCatalog é montado uma vez: as mensagens são só lidas ao serializar.
var itemCatalog = func() []*pb.ItemInfo {
	ids := game.ItemIDs()
	sort.Strings(ids)
	out := make([]*pb.ItemInfo, 0, len(ids))
	for _, id := range ids {
		out = append(out, itemInfoPB(game.Items[id]))
	}
	return out
}()

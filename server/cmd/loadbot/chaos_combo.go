package main

import (
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Combos: mensagens em sequência, sem esperar resposta, que miram a ordem em que a zona
// processa (uma goroutine, eventos em fila). São os casos "jogador que encanta o item que
// acabou de vender", "cai logo depois de comprar", "confirma a troca e fecha a aba".

func actCombo(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	var msgs []*pb.ClientMessage
	label := ""
	drop := 0 // 1 = fecha no fim (normal), 2 = fecha abrupto, 3 = relogin por cima
	slot := s.someBag(r[2], nil)
	switch pickN(r[1], 17) {
	case 0: // vende e encanta o mesmo espaço
		sell, _ := s.sellMsg(r, slot)
		ench, _ := s.enchantMsg(r, int(slot))
		msgs, label = []*pb.ClientMessage{sell, ench}, "combo:vender_e_encantar"
	case 1: // compra e cai
		buy, _ := s.buyMsg(r)
		msgs, label, drop = []*pb.ClientMessage{buy}, "combo:comprar_e_cair", 2
	case 2: // oferece na troca, vende, trava e confirma
		sell, _ := s.sellMsg(r, slot)
		msgs = []*pb.ClientMessage{
			tradeMsg(pb.TradeCmdKind_TRADE_CMD_ADD_ITEM, 0, slot, 0, 0), sell,
			tradeMsg(pb.TradeCmdKind_TRADE_CMD_LOCK, 0, 0, 0, 0), tradeMsg(pb.TradeCmdKind_TRADE_CMD_CONFIRM, 0, 0, 0, 0),
		}
		label = "combo:oferecer_vender_confirmar"
	case 3: // guarda no armazém o que está na oferta
		st, _ := s.storageMoveMsg(r, int(slot))
		msgs = []*pb.ClientMessage{tradeMsg(pb.TradeCmdKind_TRADE_CMD_ADD_ITEM, 0, slot, 0, 0), st,
			tradeMsg(pb.TradeCmdKind_TRADE_CMD_LOCK, 0, 0, 0, 0), tradeMsg(pb.TradeCmdKind_TRADE_CMD_CONFIRM, 0, 0, 0, 0)}
		label = "combo:oferecer_e_guardar"
	case 4: // destrói e usa o mesmo espaço
		msgs = []*pb.ClientMessage{
			{Msg: &pb.ClientMessage_DestroyItem{DestroyItem: &pb.DestroyItem{Slot: slot, Count: 1}}},
			{Msg: &pb.ClientMessage_UseItem{UseItem: &pb.UseItem{Slot: slot}}},
		}
		label = "combo:destruir_e_usar"
	case 5: // equipa e vende o mesmo espaço (o que voltou do encaixe vai para lá)
		gs := s.someBag(r[3], isGear)
		sell, _ := s.sellMsg(r, gs)
		msgs = []*pb.ClientMessage{{Msg: &pb.ClientMessage_UseItem{UseItem: &pb.UseItem{Slot: gs}}}, sell}
		label = "combo:equipar_e_vender"
	case 6: // encanta e cai
		ench, _ := s.enchantMsg(r, -1)
		msgs, label, drop = []*pb.ClientMessage{ench}, "combo:encantar_e_cair", 2
	case 7: // trava, confirma e fecha a aba (no meio da troca)
		msgs = []*pb.ClientMessage{tradeMsg(pb.TradeCmdKind_TRADE_CMD_LOCK, 0, 0, 0, 0), tradeMsg(pb.TradeCmdKind_TRADE_CMD_CONFIRM, 0, 0, 0, 0)}
		label, drop = "combo:confirmar_troca_e_cair", 1+pickN(r[3], 3)
	case 8: // guarda todas as moedas e oferece todas na troca
		msgs = []*pb.ClientMessage{
			tradeMsg(pb.TradeCmdKind_TRADE_CMD_SET_GOLD, 0, 0, 0, int64(s.gold)),
			{Msg: &pb.ClientMessage_StorageGold{StorageGold: &pb.StorageGold{NpcId: s.nearestNPC(storageNPC), Amount: int64(s.gold)}}},
			tradeMsg(pb.TradeCmdKind_TRADE_CMD_LOCK, 0, 0, 0, 0), tradeMsg(pb.TradeCmdKind_TRADE_CMD_CONFIRM, 0, 0, 0, 0),
		}
		label = "combo:guardar_moedas_da_oferta"
	case 9: // cria e vende o primeiro espaço livre (onde o item criado cai)
		craft, _ := s.craftMsg(r)
		free := uint32(0)
		for i, it := range s.bag {
			if it.id == "" {
				free = uint32(i)
				break
			}
		}
		sell, _ := s.sellMsg(r, free)
		msgs, label = []*pb.ClientMessage{craft, sell}, "combo:criar_e_vender"
	case 10: // começa a viajar e tenta trocar e comprar no meio
		tp, _ := s.teleportMsg(r)
		tr, _ := s.tradeStep(r)
		buy, _ := s.buyMsg(r)
		msgs, label = []*pb.ClientMessage{tp, tr, buy}, "combo:viajar_e_trocar"
	case 11: // compra e reloga por cima
		buy, _ := s.buyMsg(r)
		msgs, label, drop = []*pb.ClientMessage{buy}, "combo:comprar_e_relogar", 3
	case 12: // cria o clã e cai (a resposta do banco chega com o jogador fora)
		m := &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: b.clanName(r[3])}
		msgs, label, drop = []*pb.ClientMessage{{Msg: &pb.ClientMessage_ClanCmd{ClanCmd: m}}}, "combo:criar_cla_e_cair", 2
	case 13: // convida para o clã e desfaz (quem aceitar, aceita um clã morto)
		msgs = []*pb.ClientMessage{
			{Msg: &pb.ClientMessage_ClanCmd{ClanCmd: &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_INVITE, Name: b.otherBot(r[3])}}},
			{Msg: &pb.ClientMessage_ClanCmd{ClanCmd: &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_DISBAND}}},
		}
		if l := s.nearby(pb.EntityKind_ENTITY_KIND_PLAYER, 40); len(l) > 0 {
			msgs[0].GetClanCmd().Name, msgs[0].GetClanCmd().TargetId = "", l[pickN(r[4], len(l))]
		}
		label = "combo:convidar_e_desfazer_cla"
	case 14: // convida para o grupo e sai
		msgs = []*pb.ClientMessage{
			{Msg: &pb.ClientMessage_PartyCmd{PartyCmd: &pb.PartyCmd{Kind: pb.PartyCmdKind_PARTY_CMD_INVITE, Name: b.otherBot(r[3])}}},
			{Msg: &pb.ClientMessage_PartyCmd{PartyCmd: &pb.PartyCmd{Kind: pb.PartyCmdKind_PARTY_CMD_LEAVE}}},
		}
		label = "combo:convidar_e_sair_do_grupo"
	case 15: // retira do armazém e destrói/vende na hora
		st := &pb.StorageMove{NpcId: s.nearestNPC(storageNPC), FromPlace: 2, ToPlace: 0, ToSlot: slot}
		if l := s.storageSlots(); len(l) > 0 {
			st.FromSlot = l[pickN(r[3], len(l))]
		}
		sell, _ := s.sellMsg(r, slot)
		msgs = []*pb.ClientMessage{{Msg: &pb.ClientMessage_StorageMove{StorageMove: st}}, sell,
			{Msg: &pb.ClientMessage_DestroyItem{DestroyItem: &pb.DestroyItem{Slot: slot, Count: math.MaxUint32}}}}
		label = "combo:retirar_e_vender"
	default: // move na bolsa e vende/encanta o destino, e cai no meio
		to := uint32(pickN(r[3], game.BagSize))
		sell, _ := s.sellMsg(r, to)
		ench, _ := s.enchantMsg(r, int(to))
		msgs = []*pb.ClientMessage{{Msg: &pb.ClientMessage_MoveItem{MoveItem: &pb.MoveItem{FromSlot: slot, ToSlot: to}}}, ench, sell}
		label, drop = "combo:mover_encantar_vender", pickN(r[4], 2)*2
	}
	s.mu.Unlock()
	b.emit(s, label, msgs...)
	switch drop {
	case 1:
		s.close(true, false)
		b.offline = 1 + pickN(r[5], 10)
	case 2:
		s.close(true, true)
		b.offline = 1 + pickN(r[5], 10)
	case 3:
		b.relogOver(s)
	}
}

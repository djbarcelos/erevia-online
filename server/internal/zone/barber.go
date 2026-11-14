package zone

import (
	"fmt"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Barbeiro: muda o visual do corpo (pele, cabelo, roupa, rosto e chapéu) por moedas.

// lookPB converte o visual para o protocolo.
func lookPB(a game.Appearance) *pb.BodyLook {
	return &pb.BodyLook{Skin: uint32(a.Skin), Hair: uint32(a.Hair), Outfit: uint32(a.Outfit), Head: a.Head, HideHat: a.HideHat}
}

// lookFromPB converte do protocolo (sem validar).
func lookFromPB(l *pb.BodyLook) game.Appearance {
	if l == nil {
		return game.Appearance{}
	}
	return game.Appearance{Skin: int(min(l.Skin, 1<<15)), Hair: int(min(l.Hair, 1<<15)), Outfit: int(min(l.Outfit, 1<<15)),
		Head: l.Head, HideHat: l.HideHat}
}

// appearancePB: armas nas mãos e visual de um jogador (enviado a todos quando muda).
func appearancePB(p *entity) *pb.ServerMessage {
	w, o, g := p.pl.inv.look()
	return &pb.ServerMessage{Msg: &pb.ServerMessage_Appearance{Appearance: &pb.Appearance{
		Id: p.id, Weapon: w, Offhand: o, WeaponGrade: uint32(g), WeaponEnchant: uint32(p.pl.inv.weaponEnchant()),
		Look: lookPB(p.pl.look),
	}}}
}

func (z *Zone) changeLook(p *entity, npcID uint32, l *pb.BodyLook) {
	n := z.shopNPC(p, npcID)
	if n == nil || n.npc.role != pb.NpcRole_NPC_ROLE_BARBER {
		return
	}
	a := game.NormalAppearance(p.pl.mainClass, lookFromPB(l)) // o corpo é o da classe principal
	if !game.ValidAppearance(a) {
		z.system(p, "Visual inválido.")
		return
	}
	if msg := z.bestiaryOutfitLocked(p, a.Outfit); msg != "" { // cor do Bestiário (família completa)
		z.system(p, msg)
		return
	}
	if a == p.pl.look {
		z.system(p, "Esse já é o seu visual.")
		return
	}
	inv := &p.pl.inv
	if inv.gold < game.BarberPrice {
		z.system(p, fmt.Sprintf("Moedas insuficientes: o barbeiro cobra %d.", game.BarberPrice))
		return
	}
	inv.gold -= game.BarberPrice
	z.trackGold(p, "barber", -game.BarberPrice)
	p.pl.look = a
	z.system(p, fmt.Sprintf("%s deixou você de visual novo por %d moedas.", n.name, game.BarberPrice))
	z.broadcast(appearancePB(p), 0)
	z.inventoryChanged(p, false) // moedas na bolsa + salvar em breve
}

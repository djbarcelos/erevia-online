package zone

import (
	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
)

// Emotes (/acenar, /sentar, /dancar): o servidor só repassa para quem está perto; a
// animação é do cliente. Sentar e dançar duram até o personagem andar, atacar ou morrer;
// acenar toca uma vez.

// emoteRange: quem vê o emote (a área de interesse de cada jogador também é de 70 m).
const emoteRange = 70.0

// emoteCommand trata os comandos de emote (true = era um deles).
func (z *Zone) emoteCommand(p *entity, cmd string) bool {
	var kind pb.EmoteKind
	switch cmd {
	case "/acenar", "/aceno", "/wave":
		kind = pb.EmoteKind_EMOTE_KIND_WAVE
	case "/sentar", "/sit":
		kind = pb.EmoteKind_EMOTE_KIND_SIT
	case "/dancar", "/dançar", "/dance":
		kind = pb.EmoteKind_EMOTE_KIND_DANCE
	default:
		return false
	}
	z.emote(p, kind)
	return true
}

func (z *Zone) emote(p *entity, kind pb.EmoteKind) {
	if p.dead {
		z.system(p, "Você está morto.")
		return
	}
	// O emote é feito parado: para de andar e de atacar.
	p.clearAction()
	p.stopMoving()
	if kind == pb.EmoteKind_EMOTE_KIND_WAVE {
		p.pl.social.emote = pb.EmoteKind_EMOTE_KIND_NONE
	} else {
		p.pl.social.emote = kind
	}
	z.broadcastNear(p.pos, emoteRange, &pb.ServerMessage{Msg: &pb.ServerMessage_Emote{Emote: &pb.Emote{Id: p.id, Kind: kind}}})
}

// endEmote: o personagem se mexeu (ou atacou); quem está perto vê o emote acabar.
func (z *Zone) endEmote(p *entity) {
	if p.pl == nil || p.pl.social.emote == pb.EmoteKind_EMOTE_KIND_NONE {
		return
	}
	p.pl.social.emote = pb.EmoteKind_EMOTE_KIND_NONE
	z.broadcastNear(p.pos, emoteRange, &pb.ServerMessage{Msg: &pb.ServerMessage_Emote{Emote: &pb.Emote{Id: p.id}}})
}

// checkEmote (a cada tick): andar acaba com o emote; morrer também (a animação da morte
// já substitui a do emote no cliente, então não precisa avisar).
func (z *Zone) checkEmote(p *entity) {
	switch {
	case p.pl.social.emote == pb.EmoteKind_EMOTE_KIND_NONE:
	case p.dead:
		p.pl.social.emote = pb.EmoteKind_EMOTE_KIND_NONE
	case p.dest != p.pos:
		z.endEmote(p)
	}
}

// broadcastNear serializa a mensagem uma vez e envia aos jogadores a até radius de pos.
func (z *Zone) broadcastNear(pos Vec2, radius float64, msg *pb.ServerMessage) {
	data, err := proto.Marshal(msg)
	if err != nil {
		z.log.Error("marshal", "err", err)
		return
	}
	var slow []uint32
	for id, p := range z.players {
		if dist(p.pos, pos) > radius {
			continue
		}
		if !p.pl.peer.Send(data) {
			slow = append(slow, id)
		}
	}
	for _, id := range slow {
		z.dropSlow(id)
	}
}

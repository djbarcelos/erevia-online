package zone

import (
	"strings"

	"erevia/server/internal/pb"
)

// Moderação entre zonas: /kick, /mute, /unmute, /ban e /give sobre alguém que está em
// outra zona rodam na zona dele (a dona do estado), com um "GM de fora": uma entidade só
// com o nome, a conta e a conexão do GM. As respostas saem pela conexão dele (Send é
// seguro entre goroutines) e o gm_log vai pelo worker da outra zona. /goto e /summon
// não cruzam mapas (o aviso diz onde a pessoa está).

// gmElsewhere repassa o comando se o alvo está em outra zona. true = repassado.
func (z *Zone) gmElsewhere(gm *entity, cmd string, args []string, text string) bool {
	if z.mgr == nil || len(args) == 0 {
		return false
	}
	switch cmd {
	case "/kick", "/mute", "/unmute", "/ban", "/give":
	default:
		return false
	}
	if _, ok := z.byName[strings.ToLower(args[0])]; ok {
		return false
	}
	_, _, oz, ok := z.mgr.bus.find(args[0])
	if !ok || oz == z {
		return false
	}
	stand := &entity{id: gm.id, name: gm.name, kind: pb.EntityKind_ENTITY_KIND_PLAYER,
		pl: &playerState{peer: gm.pl.peer, accountID: gm.pl.accountID, gm: true}}
	select {
	case oz.events <- socialDone{fn: func() { oz.command(stand, text) }}:
	default:
		z.system(gm, "O outro mapa não respondeu: tente de novo.")
	}
	return true
}

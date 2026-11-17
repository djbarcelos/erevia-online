package zone

import (
	"context"
	"errors"
	"time"
)

// Portal de administração (/admin): pedidos que vêm do HTTP e precisam do estado da zona.
//
// O estado da zona tem um dono só (a goroutine de Run), então o portal nunca lê nem
// escreve nele direto: manda um pedido pelo canal de eventos e espera a resposta, com
// prazo. Toda ação futura sobre jogadores online (expulsar, silenciar, dar item…) segue
// este padrão; com o personagem em memória, quem muda o banco é a própria zona (pelo
// salvamento normal ou pelos workers), nunca a API.

// AdminTimeout é o prazo padrão de um pedido do portal à zona.
const AdminTimeout = 2 * time.Second

// ErrZoneBusy: a zona não respondeu no prazo (fila cheia, travada ou parada).
var ErrZoneBusy = errors.New("zone did not answer in time")

// askZone roda fn na goroutine da zona e devolve o resultado. Desiste com ErrZoneBusy se
// a zona não pegar o pedido ou não responder antes de ctx ou do AdminTimeout.
//
// Usa o socialDone, que é o evento genérico "rode isto na goroutine da zona" (zone.go
// já o trata). Assim este arquivo não precisa mexer no switch de eventos.
func askZone[T any](ctx context.Context, z *Zone, fn func(z *Zone) T) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, AdminTimeout)
	defer cancel()
	reply := make(chan T, 1) // com folga: a zona nunca trava se quem pediu já desistiu
	var zero T
	select {
	case z.events <- socialDone{fn: func() { reply <- fn(z) }}:
	case <-ctx.Done():
		return zero, ErrZoneBusy
	}
	select {
	case v := <-reply:
		return v, nil
	case <-ctx.Done():
		return zero, ErrZoneBusy
	}
}

// AdminStatus é o retrato da zona que a página inicial do portal mostra.
type AdminStatus struct {
	Online int // jogadores conectados agora
}

// Status pergunta à zona quantos jogadores estão online (pelo canal, com prazo).
func (z *Zone) Status(ctx context.Context) (AdminStatus, error) {
	return askZone(ctx, z, func(z *Zone) AdminStatus {
		return AdminStatus{Online: len(z.players)}
	})
}

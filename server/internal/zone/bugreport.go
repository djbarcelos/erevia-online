package zone

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// /bug completo: o cliente manda o texto e o contexto dele (erros, FPS, últimas mensagens)
// por POST /api/bug-reports e depois o print. A API pergunta à zona, pelo canal (askZone),
// quem é o personagem e onde ele está, com as mesmas regras do /bug do chat (tamanho mínimo
// e um relato a cada 30 s, contados juntos nos dois caminhos), e grava o relato ela mesma.

var (
	// ErrBugNotOnline: o personagem não está no jogo (ou é de outra conta).
	ErrBugNotOnline = errors.New("character is not online for this account")
	// ErrBugTooShort: texto com menos de bugMinRunes caracteres.
	ErrBugTooShort = errors.New("bug report too short")
)

// BugCooldownError: o jogador relatou há menos de 30 s.
type BugCooldownError struct{ Seconds int }

func (e *BugCooldownError) Error() string { return fmt.Sprintf("bug report cooldown: %d s", e.Seconds) }

// BugErrorText é a resposta ao jogador para os erros de takeBug (a mesma no chat e no HTTP).
func BugErrorText(err error) string {
	var cd *BugCooldownError
	switch {
	case errors.As(err, &cd):
		return fmt.Sprintf("Espere %d s para relatar outro bug.", cd.Seconds)
	case errors.Is(err, ErrBugTooShort):
		return "Uso: /bug o que aconteceu (conte com algumas palavras)."
	case errors.Is(err, ErrBugNotOnline):
		return "Entre no jogo com o personagem para relatar."
	default:
		return "Não foi possível enviar o relato agora. Tente de novo em instantes."
	}
}

// takeBug confere o relato e gasta o intervalo do jogador; devolve o relato carimbado com
// quem, onde e a versão do servidor.
func (z *Zone) takeBug(p *entity, text string) (BugReport, error) {
	if len([]rune(text)) < bugMinRunes {
		return BugReport{}, ErrBugTooShort
	}
	if z.now < p.pl.nextBug {
		return BugReport{}, &BugCooldownError{Seconds: int(math.Ceil(p.pl.nextBug - z.now))}
	}
	p.pl.nextBug = z.now + bugCooldown
	return BugReport{
		AccountID: p.pl.accountID, CharacterID: p.id, Name: p.name, Level: p.level,
		X: p.pos.X, Z: p.pos.Z, Version: z.cfg.Version, UserAgent: p.pl.userAgent, Text: text,
	}, nil
}

// PrepareBug confere um /bug que chegou pelo HTTP: o personagem characterID tem de estar
// online e ser da conta accountID. Gasta o intervalo do /bug (o mesmo do chat) e devolve o
// relato para a API gravar. Erros: ErrBugNotOnline, ErrBugTooShort, *BugCooldownError e
// ErrZoneBusy (a zona não respondeu no prazo).
func (z *Zone) PrepareBug(ctx context.Context, accountID, characterID int64, text string) (BugReport, error) {
	type result struct {
		r   BugReport
		err error
	}
	if characterID <= 0 || characterID > math.MaxUint32 {
		return BugReport{}, ErrBugNotOnline
	}
	out, err := askZone(ctx, z, func(z *Zone) result {
		p := z.players[uint32(characterID)]
		if p == nil || p.pl == nil || p.pl.accountID != accountID {
			return result{err: ErrBugNotOnline}
		}
		r, err := z.takeBug(p, text)
		return result{r, err}
	})
	if err != nil {
		return BugReport{}, err
	}
	return out.r, out.err
}

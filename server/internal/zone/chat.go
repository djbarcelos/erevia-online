package zone

import (
	"strings"
	"unicode"
)

// MaxChatRunes é o tamanho máximo de uma mensagem de chat, em caracteres.
const MaxChatRunes = 200

// SanitizeChat remove caracteres de controle e espaços nas pontas, junta espaços
// repetidos e corta em MaxChatRunes. O cliente sempre exibe como texto puro.
func SanitizeChat(s string) string {
	var b strings.Builder
	n := 0
	lastSpace := false
	for _, r := range strings.TrimSpace(s) {
		if n >= MaxChatRunes {
			break
		}
		// Espaço vem antes do teste de controle: \n e \t viram espaço simples.
		isSpace := unicode.IsSpace(r)
		if !isSpace && (unicode.IsControl(r) || r == unicode.ReplacementChar || unicode.Is(unicode.Cf, r)) {
			continue
		}
		if isSpace {
			if lastSpace {
				continue
			}
			r = ' '
			lastSpace = true
		} else {
			lastSpace = false
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

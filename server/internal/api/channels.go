package api

import "net/http"

// Canais do mapa principal (tunnel#86): a tela de personagens mostra a lista para escolher
// onde entrar. Só números (quantos estão em cada canal), sem nomes: não precisa de login.

// ChannelInfo é um canal: o número, quantos estão nele, quantos cabem (0 = sem limite) e se
// está cheio.
type ChannelInfo struct {
	Number   int  `json:"number"`
	Players  int  `json:"players"`
	Capacity int  `json:"capacity"`
	Full     bool `json:"full"`
}

// SetChannels liga a lista de canais (o gerente das zonas). Sem ela, a lista vem vazia.
func (a *API) SetChannels(fn func() []ChannelInfo) { a.channels = fn }

// handleChannels: GET /api/channels → {"channels": [...]} (vazio = o servidor não tem canais).
func (a *API) handleChannels(w http.ResponseWriter, _ *http.Request) {
	list := []ChannelInfo{}
	if a.channels != nil {
		list = a.channels()
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"channels": list})
}

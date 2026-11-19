package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestChannelsList(t *testing.T) {
	a := &API{}
	get := func() []ChannelInfo {
		rec := httptest.NewRecorder()
		a.handleChannels(rec, httptest.NewRequest("GET", "/api/channels", nil))
		var out struct {
			Channels []ChannelInfo `json:"channels"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != 200 {
			t.Fatalf("%d %s %v", rec.Code, rec.Body, err)
		}
		return out.Channels
	}
	if l := get(); len(l) != 0 {
		t.Fatalf("sem canais, lista vazia: %+v", l)
	}
	a.SetChannels(func() []ChannelInfo {
		return []ChannelInfo{{Number: 1, Players: 3, Capacity: 3, Full: true}, {Number: 2}}
	})
	if l := get(); len(l) != 2 || !l[0].Full || l[1].Number != 2 {
		t.Fatalf("lista: %+v", l)
	}
}

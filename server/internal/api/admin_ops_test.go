package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/zone"
)

func TestAdminOpsThroughTheZone(t *testing.T) {
	e := newLiveEnv(t)
	gm := e.login("Mestre", true)
	acc, id := e.newChar("Jogador", "Ouvinte")
	e.join(acc, id)

	var st zone.OpsStatus
	if code := gm.getJSON("/api/admin/ops", &st); code != http.StatusOK || st.Online != 1 || st.Invasion.Phase != "none" {
		t.Fatalf("ops = %d %+v", code, st)
	}

	// Anúncio agora.
	if code, r := gm.post("/api/admin/ops/announce", `{"title":"Aviso","text":"Olá a todos"}`); code != http.StatusOK || !strings.Contains(r.Message, "1 jogador") {
		t.Fatalf("announce = %d %+v", code, r)
	}
	if code, _ := gm.post("/api/admin/ops/announce", `{"text":"  "}`); code != http.StatusBadRequest {
		t.Fatal("empty announce")
	}

	// Agendado: no banco e na fila da zona; cancelar tira dos dois.
	at := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	resp := gm.req("POST", "/api/admin/ops/announce", `{"title":"Evento","text":"Hoje tem invasão","at":"`+at+`"}`, adminHdr)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("schedule = %d %s", resp.StatusCode, readBody(resp))
	}
	pending, err := e.st.PendingAnnouncements(context.Background())
	if err != nil || len(pending) != 1 || pending[0].Text != "Hoje tem invasão" || pending[0].ByName != "Mestre" {
		t.Fatalf("pending in db = %+v, %v", pending, err)
	}
	gm.getJSON("/api/admin/ops", &st)
	if len(st.Scheduled) != 1 || st.Scheduled[0].ID != pending[0].ID {
		t.Fatalf("scheduled in zone = %+v", st.Scheduled)
	}
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if code, _ := gm.post("/api/admin/ops/announce", `{"text":"x","at":"`+past+`"}`); code != http.StatusBadRequest {
		t.Fatal("scheduling in the past")
	}
	if code, _ := gm.post(fmt.Sprintf("/api/admin/ops/announce/%d/cancel", pending[0].ID), `{}`); code != http.StatusOK {
		t.Fatal("cancel")
	}
	pending, _ = e.st.PendingAnnouncements(context.Background())
	gm.getJSON("/api/admin/ops", &st)
	if len(pending) != 0 || len(st.Scheduled) != 0 {
		t.Fatalf("after cancel: db %d, zone %d", len(pending), len(st.Scheduled))
	}

	// Manutenção.
	if code, _ := gm.post("/api/admin/ops/maintenance", `{"minutes":15,"message":"Atualização"}`); code != http.StatusOK {
		t.Fatal("maintenance")
	}
	gm.getJSON("/api/admin/ops", &st)
	if st.Maintenance == nil || st.Maintenance.By != "Mestre" || time.Until(st.Maintenance.At) < 14*time.Minute {
		t.Fatalf("maintenance = %+v", st.Maintenance)
	}
	if code, _ := gm.post("/api/admin/ops/maintenance/cancel", `{}`); code != http.StatusOK {
		t.Fatal("maintenance cancel")
	}

	// Eventos.
	if code, _ := gm.post("/api/admin/ops/xp2", `{"hours":2}`); code != http.StatusOK {
		t.Fatal("xp2")
	}
	if gm.getJSON("/api/admin/ops", &st); !st.XP2.On {
		t.Fatal("xp2 on")
	}
	gm.post("/api/admin/ops/xp2", `{"hours":0}`)
	if code, _ := gm.post("/api/admin/ops/xp2", `{"hours":500}`); code != http.StatusBadRequest {
		t.Fatal("xp2 hours out of range")
	}
	if code, _ := gm.post("/api/admin/ops/invasion", `{"action":"anunciar"}`); code != http.StatusOK {
		t.Fatal("invasion")
	}
	if gm.getJSON("/api/admin/ops", &st); st.Invasion.Phase != "warning" {
		t.Fatalf("invasion = %+v", st.Invasion)
	}
	if code, r := gm.post("/api/admin/ops/invasion", `{"action":"anunciar"}`); code != http.StatusBadRequest || r.Error == "" {
		t.Fatalf("second invasion = %d %+v", code, r)
	}
	gm.post("/api/admin/ops/invasion", `{"action":"parar"}`)
	if code, _ := gm.post("/api/admin/ops/invasion", `{"action":"xyz"}`); code != http.StatusBadRequest {
		t.Fatal("invalid invasion action")
	}
	if code, _ := gm.post("/api/admin/ops/bosses/123/respawn", `{}`); code != http.StatusBadRequest {
		t.Fatal("respawn of something that is not a boss")
	}

	log, _ := e.st.RecentGMLog(context.Background(), 50)
	var cmds []string
	for _, g := range log {
		cmds = append(cmds, g.Command+"/"+g.Detail)
	}
	got := strings.Join(cmds, " | ")
	for _, want := range []string{"portal:anuncio/", "portal:anuncio:agendar/", "portal:anuncio:cancelar/", "portal:manutencao/15 min",
		"portal:manutencao:cancelar/", "portal:evento/2 h", "portal:evento/off", "portal:evento/anunciar", "portal:evento/parar",
		"portal:evento:falhou/", "portal:chefe:renascer:falhou/"} {
		if !strings.Contains(got, want) {
			t.Fatalf("gm_log %q lacks %q", got, want)
		}
	}
}

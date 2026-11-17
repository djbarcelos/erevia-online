package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"erevia/server/internal/store"
)

// Relatos (#107): os relatos de /bug e os erros do navegador.

const maxResolveNote = 500 // runas da nota de quem resolveu

// handleBugs: GET /api/admin/bugs?status=open|resolved|all&version=&region=&q=&before=&limit=
// A primeira página (sem before) leva também as opções dos filtros.
func (p *panels) handleBugs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.BugFilter{
		Status: q.Get("status"), Version: q.Get("version"), Region: q.Get("region"), Query: q.Get("q"),
		Before: queryInt64(r, "before"), Limit: int(queryInt64(r, "limit")),
	}
	if utf8.RuneCountInString(f.Query) > 100 || len(f.Version) > 64 || len(f.Region) > 64 {
		writeError(w, http.StatusBadRequest, "Filtro grande demais.")
		return
	}
	bugs, err := p.a.store.ListBugs(r.Context(), f)
	if err != nil {
		p.a.internalError(w, err)
		return
	}
	if bugs == nil {
		bugs = []store.BugRow{}
	}
	out := map[string]any{"bugs": bugs}
	if f.Before == 0 {
		facets, err := p.a.store.BugFacets(r.Context())
		if err != nil {
			p.a.internalError(w, err)
			return
		}
		out["facets"] = facets
	}
	writeJSON(w, http.StatusOK, out)
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// handleBug: GET /api/admin/bugs/{id}. "extra" traz as colunas que a tabela tiver além
// das conhecidas (o contexto e o print do /bug completo, quando existirem).
func (p *panels) handleBug(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		NotFound(w, r)
		return
	}
	bug, extra, err := p.a.store.BugDetail(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Relato não encontrado.")
		return
	}
	if err != nil {
		p.a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bug": bug, "extra": extra})
}

// handleResolveBug: POST /api/admin/bugs/{id}/resolve {"resolved": true, "note": "..."}.
// Grava no gm_log antes de mudar (se o registro falhar, nada muda).
func (p *panels) handleResolveBug(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		NotFound(w, r)
		return
	}
	var body struct {
		Resolved *bool  `json:"resolved"`
		Note     string `json:"note"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	resolved := body.Resolved == nil || *body.Resolved
	note := strings.TrimSpace(body.Note)
	if utf8.RuneCountInString(note) > maxResolveNote {
		writeError(w, http.StatusBadRequest, "Nota longa demais (até 500 letras).")
		return
	}
	if _, _, err := p.a.store.BugDetail(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Relato não encontrado.")
		return
	} else if err != nil {
		p.a.internalError(w, err)
		return
	}
	ar := adminFrom(r.Context())
	name, err := p.a.store.DisplayName(r.Context(), ar.accountID)
	if err != nil {
		p.a.internalError(w, err)
		return
	}
	cmd := "bug.resolve"
	if !resolved {
		cmd = "bug.reopen"
	}
	if err := p.a.audit(r, cmd, "bug #"+strconv.FormatInt(id, 10), note); err != nil {
		p.a.internalError(w, err)
		return
	}
	bug, err := p.a.store.ResolveBug(r.Context(), id, resolved, ar.accountID, name, note)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Relato não encontrado.")
		return
	}
	if err != nil {
		p.a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bug": bug})
}

// handleClientErrors: GET /api/admin/client-errors?days=&version= — os erros do navegador
// mais frequentes do período (juntando as versões) e o resumo por versão.
func (p *panels) handleClientErrors(w http.ResponseWriter, r *http.Request) {
	days := periodDays(r)
	version := r.URL.Query().Get("version")
	if len(version) > 64 {
		writeError(w, http.StatusBadRequest, "Versão inválida.")
		return
	}
	errs, err := p.a.store.TopClientErrors(r.Context(), version, days, 40)
	if err != nil {
		p.a.internalError(w, err)
		return
	}
	versions, err := p.a.store.ClientErrorVersions(r.Context(), days)
	if err != nil {
		p.a.internalError(w, err)
		return
	}
	if errs == nil {
		errs = []store.ClientErrorRow{}
	}
	if versions == nil {
		versions = []store.ClientErrorVersion{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": days, "errors": errs, "versions": versions})
}

package web

import (
	"net/http"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
)

func respondDomainSession(w http.ResponseWriter, session *domain.RiggingSession, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+itoa(session.Version)+`"`)
	dataResponse(w, http.StatusOK, session)
}
func (a *API) RecordCheckHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.CheckCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	if err := enrichVersion(r, &cmd.VersionCommand); err != nil {
		writeError(w, err)
		return
	}
	session, err := a.service.RecordCheck(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) CompleteInspectionHandler(w http.ResponseWriter, r *http.Request) {
	cmd, ok := decodeVersion(w, r)
	if !ok {
		return
	}
	session, err := a.service.CompleteInspection(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) RemediationHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.RemediationCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	if err := enrichVersion(r, &cmd.VersionCommand); err != nil {
		writeError(w, err)
		return
	}
	session, err := a.service.AssignRemediation(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) ReviewHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.ReviewCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	if err := enrichVersion(r, &cmd.VersionCommand); err != nil {
		writeError(w, err)
		return
	}
	session, err := a.service.Review(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}

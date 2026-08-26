package web

import (
	"net/http"
	"rigging-readiness-desk/internal/application"
)

func (a *API) ConfirmBaselineHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.BaselineCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	if err := enrichVersion(r, &cmd.VersionCommand); err != nil {
		writeError(w, err)
		return
	}
	session, err := a.service.ConfirmBaseline(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) AddLineHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.AddLineCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	if err := enrichVersion(r, &cmd.VersionCommand); err != nil {
		writeError(w, err)
		return
	}
	session, err := a.service.AddLine(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) AddPointHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.AddPointCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	if err := enrichVersion(r, &cmd.VersionCommand); err != nil {
		writeError(w, err)
		return
	}
	session, err := a.service.AddPoint(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) AddLoadHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.AddLoadCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	if err := enrichVersion(r, &cmd.VersionCommand); err != nil {
		writeError(w, err)
		return
	}
	session, err := a.service.AddLoad(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) AddLoadsHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.AddLoadsCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	if err := enrichVersion(r, &cmd.VersionCommand); err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.AddLoads(r.Context(), r.PathValue("id"), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+itoa(result.Version)+`"`)
	dataResponse(w, http.StatusOK, result)
}
func (a *API) FinalizeModelHandler(w http.ResponseWriter, r *http.Request) {
	cmd, ok := decodeVersion(w, r)
	if !ok {
		return
	}
	session, err := a.service.FinalizeModel(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) CalculateHandler(w http.ResponseWriter, r *http.Request) {
	cmd, ok := decodeVersion(w, r)
	if !ok {
		return
	}
	session, err := a.service.Calculate(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) ReviseLoadHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.ReviseLoadCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	if err := enrichVersion(r, &cmd.VersionCommand); err != nil {
		writeError(w, err)
		return
	}
	session, err := a.service.ReviseLoad(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) PreviewLoadPlanHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.PreviewLoadPlanCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	preview, err := a.service.PreviewLoadPlan(r.Context(), r.PathValue("id"), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	dataResponse(w, http.StatusOK, preview)
}
func (a *API) ApplyLoadPlanHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.ApplyLoadPlanCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	if err := enrichVersion(r, &cmd.VersionCommand); err != nil {
		writeError(w, err)
		return
	}
	session, err := a.service.ApplyLoadPlan(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func decodeVersion(w http.ResponseWriter, r *http.Request) (application.VersionCommand, bool) {
	var cmd application.VersionCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return cmd, false
	}
	if err := enrichVersion(r, &cmd); err != nil {
		writeError(w, err)
		return cmd, false
	}
	return cmd, true
}

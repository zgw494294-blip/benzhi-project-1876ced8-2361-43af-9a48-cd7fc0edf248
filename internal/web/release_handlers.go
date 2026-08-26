package web

import "net/http"

func (a *API) FreezeHandler(w http.ResponseWriter, r *http.Request) {
	cmd, ok := decodeVersion(w, r)
	if !ok {
		return
	}
	session, err := a.service.Freeze(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) ReleaseHandler(w http.ResponseWriter, r *http.Request) {
	cmd, ok := decodeVersion(w, r)
	if !ok {
		return
	}
	session, err := a.service.Issue(r.Context(), r.PathValue("id"), cmd)
	respondDomainSession(w, session, err)
}
func (a *API) VerifyCertificateHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.Verify(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	dataResponse(w, http.StatusOK, result)
}

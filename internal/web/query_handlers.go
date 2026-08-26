package web

import "net/http"

func (a *API) GetReadinessHandler(w http.ResponseWriter, r *http.Request) {
	view, err := a.service.GetReadiness(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+itoa(view.Version)+`"`)
	dataResponse(w, http.StatusOK, view)
}

func (a *API) GetCalculationHandler(w http.ResponseWriter, r *http.Request) {
	calculation, err := a.service.GetCalculation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	dataResponse(w, http.StatusOK, calculation)
}

func (a *API) GetReviewConfirmationHandler(w http.ResponseWriter, r *http.Request) {
	confirmation, err := a.service.GetReviewConfirmation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+itoa(confirmation.Version)+`"`)
	dataResponse(w, http.StatusOK, confirmation)
}

func (a *API) GetCertificateHandler(w http.ResponseWriter, r *http.Request) {
	certificate, err := a.service.GetCertificate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	dataResponse(w, http.StatusOK, certificate)
}

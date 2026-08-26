package web

import (
	"log/slog"
	"net/http"
	"rigging-readiness-desk/internal/application"
)

type API struct{ service *application.Service }

func NewHandler(service *application.Service, logger *slog.Logger) http.Handler {
	api := &API{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", api.IndexHandler)
	mux.HandleFunc("GET /assets/app.css", api.CSSHandler)
	mux.HandleFunc("GET /assets/app.js", api.JSHandler)
	mux.HandleFunc("GET /api/v1/rigging-sessions", api.ListSessionsHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions", api.CreateSessionHandler)
	mux.HandleFunc("GET /api/v1/rigging-sessions/{id}", api.GetSessionHandler)
	mux.HandleFunc("GET /api/v1/rigging-sessions/{id}/audit", api.ListAuditHandler)
	mux.HandleFunc("GET /api/v1/rigging-sessions/{id}/findings", api.ListFindingsHandler)
	mux.HandleFunc("GET /api/v1/rigging-sessions/{id}/readiness", api.GetReadinessHandler)
	mux.HandleFunc("GET /api/v1/rigging-sessions/{id}/calculation", api.GetCalculationHandler)
	mux.HandleFunc("GET /api/v1/rigging-sessions/{id}/review-confirmation", api.GetReviewConfirmationHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/baseline", api.ConfirmBaselineHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/lines", api.AddLineHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/points", api.AddPointHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/loads", api.AddLoadHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/loads/batch", api.AddLoadsHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/model/finalize", api.FinalizeModelHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/calculate", api.CalculateHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/checks", api.RecordCheckHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/inspection/complete", api.CompleteInspectionHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/findings/remediation", api.RemediationHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/loads/revise", api.ReviseLoadHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/remediation/load-plan/preview", api.PreviewLoadPlanHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/remediation/load-plan/apply", api.ApplyLoadPlanHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/review", api.ReviewHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/freeze", api.FreezeHandler)
	mux.HandleFunc("POST /api/v1/rigging-sessions/{id}/release", api.ReleaseHandler)
	mux.HandleFunc("GET /api/v1/certificates/{id}/verify", api.VerifyCertificateHandler)
	mux.HandleFunc("GET /api/v1/certificates/{id}", api.GetCertificateHandler)
	mux.HandleFunc("GET /healthz", api.HealthHandler)
	return requestLog(logger, secure(mux))
}

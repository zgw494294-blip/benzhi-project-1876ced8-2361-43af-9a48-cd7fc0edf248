package web

import (
	"net/http"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"strconv"
	"strings"
	"time"
)

func (a *API) CreateSessionHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.CreateSessionCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	session, err := a.service.CreateSession(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+itoa(session.Version)+`"`)
	dataResponse(w, http.StatusCreated, session)
}
func (a *API) ListSessionsHandler(w http.ResponseWriter, r *http.Request) {
	filter, err := sessionListFilter(r)
	if err != nil {
		writeError(w, err)
		return
	}
	page, err := a.service.QuerySessionQueue(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	dataResponse(w, http.StatusOK, page)
}

func sessionListFilter(r *http.Request) (application.SessionListFilter, error) {
	q := r.URL.Query()
	filter := application.SessionListFilter{Status: domain.SessionStatus(strings.TrimSpace(q.Get("status"))), Venue: q.Get("venue"), OperatorID: q.Get("operatorId"), Keyword: firstQuery(q.Get("keyword"), q.Get("titleKeyword"))}
	var err error
	if filter.Offset, err = queryInt(q.Get("offset"), 0, "offset"); err != nil {
		return filter, err
	}
	if filter.Limit, err = queryInt(q.Get("limit"), 0, "limit"); err != nil {
		return filter, err
	}
	if q.Has("limit") && filter.Limit == 0 {
		return filter, domain.NewError(domain.ErrValidation, "limit", "分页条数必须在 1 到 100 之间")
	}
	if filter.PerformanceFrom, err = queryTime(firstQuery(q.Get("performanceFrom"), q.Get("performanceAtFrom")), "performanceFrom"); err != nil {
		return filter, err
	}
	if filter.PerformanceTo, err = queryTime(firstQuery(q.Get("performanceTo"), q.Get("performanceAtTo")), "performanceTo"); err != nil {
		return filter, err
	}
	if filter.DueBefore, err = queryTime(firstQuery(q.Get("dueBefore"), q.Get("upcomingBefore")), "dueBefore"); err != nil {
		return filter, err
	}
	if raw := q.Get("pendingOnly"); raw != "" {
		filter.PendingOnly, err = strconv.ParseBool(raw)
		if err != nil {
			return filter, domain.NewError(domain.ErrValidation, "pendingOnly", "必须为 true 或 false")
		}
	}
	return filter, nil
}

func firstQuery(primary, alias string) string {
	if primary != "" {
		return primary
	}
	return alias
}

func queryInt(raw string, fallback int, field string) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, domain.NewError(domain.ErrValidation, field, "必须是整数")
	}
	return value, nil
}

func queryTime(raw, field string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, domain.NewError(domain.ErrValidation, field, "必须是 RFC3339 时间")
	}
	value = value.UTC()
	return &value, nil
}
func (a *API) GetSessionHandler(w http.ResponseWriter, r *http.Request) {
	session, err := a.service.GetSession(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+itoa(session.Version)+`"`)
	dataResponse(w, http.StatusOK, session)
}
func (a *API) ListAuditHandler(w http.ResponseWriter, r *http.Request) {
	offset, limit := pagination(r)
	page, err := a.service.ListAudit(r.Context(), r.PathValue("id"), offset, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	dataResponse(w, http.StatusOK, page)
}
func (a *API) ListFindingsHandler(w http.ResponseWriter, r *http.Request) {
	offset, limit := pagination(r)
	page, err := a.service.OpenFindings(r.Context(), r.PathValue("id"), offset, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	dataResponse(w, http.StatusOK, page)
}
func (a *API) HealthHandler(w http.ResponseWriter, r *http.Request) {
	dataResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}
func itoa(value int64) string { return strconv.FormatInt(value, 10) }

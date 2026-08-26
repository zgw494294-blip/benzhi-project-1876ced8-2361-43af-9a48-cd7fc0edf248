package web

import (
	"net/http"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"strconv"
	"strings"
)

func enrichVersion(r *http.Request, cmd *application.VersionCommand) error {
	cmd.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if match := strings.Trim(r.Header.Get("If-Match"), " \""); match != "" {
		version, err := strconv.ParseInt(match, 10, 64)
		if err != nil {
			return domain.NewError(domain.ErrValidation, "If-Match", "必须是整数版本")
		}
		if cmd.ExpectedVersion != 0 && cmd.ExpectedVersion != version {
			return domain.NewError(domain.ErrValidation, "expectedVersion", "与 If-Match 不一致")
		}
		cmd.ExpectedVersion = version
	}
	return nil
}
func pagination(r *http.Request) (int, int) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return offset, limit
}

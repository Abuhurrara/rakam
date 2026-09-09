package httpapi

import (
	"github.com/Abuhurrara/rakam/api/internal/service"
	"net/http"
)

func handleExport(svc *service.ExportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		snapshot, err := svc.Snapshot(r.Context(), userID)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Disposition", `attachment; filename="rakam-export.json"`)
		w.Write(snapshot.Content)
	}
}

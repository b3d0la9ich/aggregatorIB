package apphttp

import (
	"net/http"

	"aggregatorIB/internal/db"
)

func (a *App) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	claims := userFromContext(r.Context())
	base := a.db.Model(&db.Incident{})
	if claims.Role != "admin" {
		base = base.Where(&db.Incident{AssignedToID: claims.UserID})
	}

	var total, openCount, criticalCount, closedCount int64
	base.Count(&total)
	base.Where(&db.Incident{Status: "open"}).Count(&openCount)
	base.Where(&db.Incident{Severity: "Критическое"}).Count(&criticalCount)
	base.Where(&db.Incident{Status: "closed"}).Count(&closedCount)

	writeJSON(w, http.StatusOK, map[string]int64{
		"total":    total,
		"open":     openCount,
		"critical": criticalCount,
		"closed":   closedCount,
	})
}

func (a *App) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	claims := userFromContext(r.Context())
	var incidents []db.Incident
	query := a.db.Preload("AssignedTo").Preload("CreatedBy").Order("occurred_at desc")
	if claims.Role != "admin" {
		query = query.Where(&db.Incident{AssignedToID: claims.UserID, Status: "open"})
	} else {
		query = query.Where(&db.Incident{Status: "open"})
	}

	if err := query.Find(&incidents).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить задачи"})
		return
	}

	writeJSON(w, http.StatusOK, incidents)
}

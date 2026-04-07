package apphttp

import (
	"net/http"

	"aggregatorIB/internal/db"
)

type analyticsResponse struct {
	Total    int64 `json:"total"`
	Open     int64 `json:"open"`
	Critical int64 `json:"critical"`
	Closed   int64 `json:"closed"`
}

func (a *App) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	var total int64
	var inProgress int64
	var critical int64
	var closed int64

	totalQuery := a.db.Model(&db.Incident{})
	inProgressQuery := a.db.Model(&db.Incident{})
	criticalQuery := a.db.Model(&db.Incident{})
	closedQuery := a.db.Model(&db.Incident{})

	if claims.Role != "admin" {
		totalQuery = totalQuery.Where("assigned_to_id = ?", claims.UserID)
		inProgressQuery = inProgressQuery.Where("assigned_to_id = ?", claims.UserID)
		criticalQuery = criticalQuery.Where("assigned_to_id = ?", claims.UserID)
		closedQuery = closedQuery.Where("assigned_to_id = ?", claims.UserID)
	}

	if err := totalQuery.Count(&total).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось получить статистику"})
		return
	}

	if err := inProgressQuery.Where("status = ?", "in_progress").Count(&inProgress).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось получить статистику по активным инцидентам"})
		return
	}

	if err := criticalQuery.Where("severity = ?", "Критическое").Count(&critical).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось получить статистику по критическим инцидентам"})
		return
	}

	if err := closedQuery.Where("status = ?", "closed").Count(&closed).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось получить статистику по закрытым инцидентам"})
		return
	}

	writeJSON(w, http.StatusOK, analyticsResponse{
		Total:    total,
		Open:     inProgress,
		Critical: critical,
		Closed:   closed,
	})
}
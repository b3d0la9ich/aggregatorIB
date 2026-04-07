package apphttp

import (
	"net/http"

	"aggregatorIB/internal/db"
)

func (a *App) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	query := a.db.Model(&db.Incident{}).
		Preload("AssignedTo").
		Preload("CreatedBy").
		Where("status = ?", "in_progress").
		Order("created_at desc")

	if claims.Role != "admin" {
		query = query.Where("assigned_to_id = ?", claims.UserID)
	}

	var tasks []db.Incident
	if err := query.Find(&tasks).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить задачи"})
		return
	}

	writeJSON(w, http.StatusOK, tasks)
}
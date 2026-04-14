package apphttp

import (
	"net/http"
	"strconv"
	"strings"

	"aggregatorIB/internal/db"
)

type auditListResponse struct {
	Items      []db.IncidentHistory `json:"items"`
	Page       int                  `json:"page"`
	Limit      int                  `json:"limit"`
	Total      int64                `json:"total"`
	TotalPages int                  `json:"total_pages"`
}

func (a *App) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	query := a.db.Model(&db.IncidentHistory{}).
		Preload("User").
		Order("created_at desc")

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if search != "" {
		searchLike := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(action) LIKE ?", searchLike)
	}

	userIDValue := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userIDValue != "" {
		if userID, err := strconv.Atoi(userIDValue); err == nil && userID > 0 {
			query = query.Where("user_id = ?", userID)
		}
	}

	page := 1
	limit := 12

	if rawPage := r.URL.Query().Get("page"); rawPage != "" {
		if parsedPage, err := strconv.Atoi(rawPage); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if parsedLimit, err := strconv.Atoi(rawLimit); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить журнал аудита"})
		return
	}

	offset := (page - 1) * limit

	var items []db.IncidentHistory
	if err := query.Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить журнал аудита"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages == 0 {
		totalPages = 1
	}

	writeJSON(w, http.StatusOK, auditListResponse{
		Items:      items,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}
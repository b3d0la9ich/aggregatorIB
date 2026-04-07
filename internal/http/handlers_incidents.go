package apphttp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aggregatorIB/internal/db"
)

type createIncidentRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	Severity     string `json:"severity"`
	OccurredAt   string `json:"occurred_at"`
	AssignedToID uint   `json:"assigned_to_id"`
}

func (a *App) handleIncidents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listIncidents(w, r)
	case http.MethodPost:
		a.adminOnly(a.createIncident)(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
	}
}

func (a *App) createIncident(w http.ResponseWriter, r *http.Request) {
	var req createIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный формат запроса"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	if req.Title == "" || req.Description == "" || req.OccurredAt == "" || req.AssignedToID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "заполните все поля формы"})
		return
	}
	if req.Severity != "Обычное" && req.Severity != "Критическое" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный уровень важности"})
		return
	}

	occurredAt, err := time.Parse(time.RFC3339, req.OccurredAt)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверная дата инцидента"})
		return
	}

	var assignedUser db.User
	if err := a.db.First(&assignedUser, req.AssignedToID).Error; err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "назначенный пользователь не найден"})
		return
	}

	claims := userFromContext(r.Context())
	incident := db.Incident{
		Title:        req.Title,
		Description:  req.Description,
		Severity:     req.Severity,
		OccurredAt:   occurredAt,
		AssignedToID: req.AssignedToID,
		CreatedByID:  claims.UserID,
		Status:       "open",
	}

	if err := a.db.Create(&incident).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось создать инцидент"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message": "инцидент создан"})
}

func (a *App) listIncidents(w http.ResponseWriter, r *http.Request) {
	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	query := a.db.Preload("AssignedTo").Preload("CreatedBy").Order("created_at desc")
	if claims.Role != "admin" {
		query = query.Where(&db.Incident{AssignedToID: claims.UserID})
	}

	status := r.URL.Query().Get("status")
	if status == "open" || status == "closed" {
		query = query.Where(&db.Incident{Status: status})
	}

	var incidents []db.Incident
	if err := query.Find(&incidents).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить инциденты"})
		return
	}

	writeJSON(w, http.StatusOK, incidents)
}

func (a *App) handleCloseIncident(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	claims := userFromContext(r.Context())
	incidentIDValue := r.URL.Query().Get("id")
	incidentID, err := strconv.Atoi(incidentIDValue)
	if err != nil || incidentID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный идентификатор инцидента"})
		return
	}

	var incident db.Incident
	if err := a.db.First(&incident, incidentID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "инцидент не найден"})
		return
	}

	if claims.Role != "admin" && incident.AssignedToID != claims.UserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "вы не можете закрыть этот инцидент"})
		return
	}

	now := time.Now().UTC()
	incident.Status = "closed"
	incident.ClosedAt = &now
	if err := a.db.Save(&incident).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось закрыть инцидент"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "инцидент закрыт"})
}

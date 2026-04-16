package apphttp

import (
	"encoding/json"
	"fmt"
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

type updateIncidentRequest struct {
	ID           uint   `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Severity     string `json:"severity"`
	OccurredAt   string `json:"occurred_at"`
	AssignedToID uint   `json:"assigned_to_id"`
}

type incidentsListResponse struct {
	Items      []db.Incident `json:"items"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	Total      int64         `json:"total"`
	TotalPages int           `json:"total_pages"`
}

type addCommentRequest struct {
	IncidentID uint   `json:"incident_id"`
	Text       string `json:"text"`
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

	if len([]rune(req.Title)) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "название инцидента не должно превышать 100 символов"})
		return
	}

	if len([]rune(req.Description)) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "описание инцидента не должно превышать 100 символов"})
		return
	}

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

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	occurredAtLocal := occurredAt.In(now.Location())

	if occurredAtLocal.Before(startOfToday) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "нельзя указывать дату инцидента раньше сегодняшнего дня"})
		return
	}

	var assignedUser db.User
	if err := a.db.First(&assignedUser, req.AssignedToID).Error; err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "назначенный пользователь не найден"})
		return
	}

	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	incident := db.Incident{
		Title:        req.Title,
		Description:  req.Description,
		Severity:     req.Severity,
		OccurredAt:   occurredAt,
		AssignedToID: req.AssignedToID,
		CreatedByID:  claims.UserID,
		Status:       "in_progress",
	}

	if err := a.db.Create(&incident).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось создать инцидент"})
		return
	}

	a.logIncidentAction(incident.ID, claims.UserID, "создал инцидент и перевёл в работу")
	a.createNotification(req.AssignedToID, fmt.Sprintf("Вам назначен инцидент: %s", incident.Title))

	writeJSON(w, http.StatusCreated, map[string]string{"message": "инцидент создан"})
}

func (a *App) listIncidents(w http.ResponseWriter, r *http.Request) {
	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	query := a.db.Model(&db.Incident{}).
		Preload("AssignedTo").
		Preload("CreatedBy")

	if claims.Role != "admin" {
		query = query.Where("assigned_to_id = ?", claims.UserID)
	}

	idValue := strings.TrimSpace(r.URL.Query().Get("id"))
	if idValue != "" {
		if id, err := strconv.Atoi(idValue); err == nil && id > 0 {
			query = query.Where("id = ?", id)
		}
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if search != "" {
		searchLike := "%" + strings.ToLower(search) + "%"
		query = query.Where(
			"LOWER(title) LIKE ? OR LOWER(description) LIKE ?",
			searchLike, searchLike,
		)
	}

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "in_progress" || status == "closed" {
		query = query.Where("status = ?", status)
	}

	severity := strings.TrimSpace(r.URL.Query().Get("severity"))
	if severity == "Обычное" || severity == "Критическое" {
		query = query.Where("severity = ?", severity)
	}

	sortBy := strings.TrimSpace(r.URL.Query().Get("sort"))
	switch sortBy {
	case "created_asc":
		query = query.Order("created_at asc")
	case "occurred_desc":
		query = query.Order("occurred_at desc")
	case "occurred_asc":
		query = query.Order("occurred_at asc")
	case "severity_desc":
		query = query.Order("CASE WHEN severity = 'Критическое' THEN 0 ELSE 1 END").Order("created_at desc")
	case "severity_asc":
		query = query.Order("CASE WHEN severity = 'Обычное' THEN 0 ELSE 1 END").Order("created_at desc")
	default:
		query = query.Order("created_at desc")
	}

	page := 1
	limit := 6

	if rawPage := r.URL.Query().Get("page"); rawPage != "" {
		if parsedPage, err := strconv.Atoi(rawPage); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if parsedLimit, err := strconv.Atoi(rawLimit); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось подсчитать инциденты"})
		return
	}

	offset := (page - 1) * limit

	var incidents []db.Incident
	if err := query.Offset(offset).Limit(limit).Find(&incidents).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить инциденты"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages == 0 {
		totalPages = 1
	}

	writeJSON(w, http.StatusOK, incidentsListResponse{
		Items:      incidents,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

func (a *App) handleUpdateIncident(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	var req updateIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный формат запроса"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)

	if len([]rune(req.Title)) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "название инцидента не должно превышать 100 символов"})
		return
	}

	if len([]rune(req.Description)) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "описание инцидента не должно превышать 100 символов"})
		return
	}

	if req.ID == 0 || req.Title == "" || req.Description == "" || req.OccurredAt == "" || req.AssignedToID == 0 {
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

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	occurredAtLocal := occurredAt.In(now.Location())

	if occurredAtLocal.Before(startOfToday) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "нельзя указывать дату инцидента раньше сегодняшнего дня"})
		return
	}

	var incident db.Incident
	if err := a.db.First(&incident, req.ID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "инцидент не найден"})
		return
	}

	if incident.Status == "closed" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "закрытый инцидент нельзя редактировать"})
		return
	}

	var assignedUser db.User
	if err := a.db.First(&assignedUser, req.AssignedToID).Error; err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "назначенный пользователь не найден"})
		return
	}

	incident.Title = req.Title
	incident.Description = req.Description
	incident.Severity = req.Severity
	incident.OccurredAt = occurredAt
	incident.AssignedToID = req.AssignedToID

	if err := a.db.Save(&incident).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось обновить инцидент"})
		return
	}

	a.logIncidentAction(incident.ID, claims.UserID, "отредактировал инцидент")
	a.createNotification(req.AssignedToID, fmt.Sprintf("Инцидент \"%s\" был обновлён администратором", incident.Title))

	writeJSON(w, http.StatusOK, map[string]string{"message": "инцидент обновлён"})
}

func (a *App) handleDeleteIncident(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	idValue := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idValue)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный идентификатор инцидента"})
		return
	}

	var incident db.Incident
	if err := a.db.First(&incident, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "инцидент не найден"})
		return
	}

	title := incident.Title
	assignedToID := incident.AssignedToID

	if err := a.db.Where("incident_id = ?", incident.ID).Delete(&db.IncidentComment{}).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось удалить комментарии"})
		return
	}

	if err := a.db.Where("incident_id = ?", incident.ID).Delete(&db.IncidentHistory{}).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось удалить историю"})
		return
	}

	if err := a.db.Delete(&incident).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось удалить инцидент"})
		return
	}

	a.createNotification(assignedToID, fmt.Sprintf("Инцидент \"%s\" был удалён администратором", title))
	a.logIncidentAction(uint(id), claims.UserID, "удалил инцидент")

	writeJSON(w, http.StatusOK, map[string]string{"message": "инцидент удалён"})
}

func (a *App) handleCloseIncident(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

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

	a.logIncidentAction(incident.ID, claims.UserID, "закрыл инцидент")

	if incident.CreatedByID != claims.UserID {
		a.createNotification(incident.CreatedByID, fmt.Sprintf("Инцидент \"%s\" был закрыт", incident.Title))
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "инцидент закрыт"})
}

func (a *App) handleChangeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	idValue := r.URL.Query().Get("id")
	status := strings.TrimSpace(r.URL.Query().Get("status"))

	id, err := strconv.Atoi(idValue)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный идентификатор инцидента"})
		return
	}

	if status != "in_progress" && status != "closed" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный статус"})
		return
	}

	var incident db.Incident
	if err := a.db.First(&incident, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "инцидент не найден"})
		return
	}

	if claims.Role != "admin" && incident.AssignedToID != claims.UserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "нет доступа"})
		return
	}

	incident.Status = status

	if status == "closed" {
		now := time.Now().UTC()
		incident.ClosedAt = &now
	} else {
		incident.ClosedAt = nil
	}

	if err := a.db.Save(&incident).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось обновить статус"})
		return
	}

	actionText := "перевёл инцидент в работу"
	if status == "closed" {
		actionText = "закрыл инцидент"
	}
	a.logIncidentAction(incident.ID, claims.UserID, actionText)

	writeJSON(w, http.StatusOK, map[string]string{"message": "статус обновлен"})
}

func (a *App) handleIncidentComments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listIncidentComments(w, r)
	case http.MethodPost:
		a.addIncidentComment(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
	}
}

func (a *App) listIncidentComments(w http.ResponseWriter, r *http.Request) {
	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	incidentID, ok := a.parseIncidentID(w, r)
	if !ok {
		return
	}

	var incident db.Incident
	if err := a.db.First(&incident, incidentID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "инцидент не найден"})
		return
	}

	if claims.Role != "admin" && incident.AssignedToID != claims.UserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "нет доступа к комментариям"})
		return
	}

	var comments []db.IncidentComment
	if err := a.db.Preload("User").Where("incident_id = ?", incidentID).Order("created_at asc").Find(&comments).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить комментарии"})
		return
	}

	writeJSON(w, http.StatusOK, comments)
}

func (a *App) addIncidentComment(w http.ResponseWriter, r *http.Request) {
	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	var req addCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный формат запроса"})
		return
	}

	req.Text = strings.TrimSpace(req.Text)

	if len([]rune(req.Text)) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "комментарий не должен превышать 100 символов"})
		return
	}

	if req.IncidentID == 0 || req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "заполните комментарий"})
		return
	}

	var incident db.Incident
	if err := a.db.First(&incident, req.IncidentID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "инцидент не найден"})
		return
	}

	if incident.Status == "closed" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "для закрытого инцидента нельзя добавлять комментарии"})
		return
	}

	if claims.Role != "admin" && incident.AssignedToID != claims.UserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "нет доступа к комментированию"})
		return
	}

	comment := db.IncidentComment{
		IncidentID: req.IncidentID,
		UserID:     claims.UserID,
		Text:       req.Text,
	}

	if err := a.db.Create(&comment).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось добавить комментарий"})
		return
	}

	a.logIncidentAction(req.IncidentID, claims.UserID, "добавил комментарий")

	if incident.AssignedToID != claims.UserID {
		a.createNotification(incident.AssignedToID, fmt.Sprintf("Новый комментарий по инциденту: %s", incident.Title))
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message": "комментарий добавлен"})
}

func (a *App) handleIncidentHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	incidentID, ok := a.parseIncidentID(w, r)
	if !ok {
		return
	}

	var incident db.Incident
	if err := a.db.First(&incident, incidentID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "инцидент не найден"})
		return
	}

	if claims.Role != "admin" && incident.AssignedToID != claims.UserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "нет доступа к истории"})
		return
	}

	var history []db.IncidentHistory
	if err := a.db.Preload("User").Where("incident_id = ?", incidentID).Order("created_at desc").Find(&history).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить историю"})
		return
	}

	writeJSON(w, http.StatusOK, history)
}

func (a *App) handleNotifications(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listNotifications(w, r)
	case http.MethodPost:
		a.markNotificationRead(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
	}
}

func (a *App) listNotifications(w http.ResponseWriter, r *http.Request) {
	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	var notifications []db.Notification
	if err := a.db.Where("user_id = ?", claims.UserID).Order("created_at desc").Limit(20).Find(&notifications).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить уведомления"})
		return
	}

	writeJSON(w, http.StatusOK, notifications)
}

func (a *App) markNotificationRead(w http.ResponseWriter, r *http.Request) {
	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	idValue := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idValue)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный идентификатор уведомления"})
		return
	}

	var notification db.Notification
	if err := a.db.First(&notification, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "уведомление не найдено"})
		return
	}

	if notification.UserID != claims.UserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "нет доступа"})
		return
	}

	notification.IsRead = true
	if err := a.db.Save(&notification).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось обновить уведомление"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "уведомление отмечено как прочитанное"})
}

func (a *App) logIncidentAction(incidentID, userID uint, action string) {
	_ = a.db.Create(&db.IncidentHistory{
		IncidentID: incidentID,
		UserID:     userID,
		Action:     action,
	}).Error
}

func (a *App) createNotification(userID uint, text string) {
	_ = a.db.Create(&db.Notification{
		UserID: userID,
		Text:   text,
	}).Error
}

func (a *App) parseIncidentID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	idValue := r.URL.Query().Get("incident_id")
	id, err := strconv.Atoi(idValue)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный идентификатор инцидента"})
		return 0, false
	}
	return uint(id), true
}
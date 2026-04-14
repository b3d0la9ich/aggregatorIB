package apphttp

import (
	"net/http"
	"strings"

	"aggregatorIB/internal/db"
)

type publicFeedItem struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	Text       string `json:"text"`
	Severity   string `json:"severity,omitempty"`
	Status     string `json:"status,omitempty"`
	CreatedAt  string `json:"created_at"`
	IncidentID uint   `json:"incident_id,omitempty"`
}

type publicFeedResponse struct {
	Items []publicFeedItem `json:"items"`
}

func (a *App) handlePublicFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	var incidents []db.Incident
	if err := a.db.
		Preload("AssignedTo").
		Preload("CreatedBy").
		Order("created_at desc").
		Limit(4).
		Find(&incidents).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить ленту"})
		return
	}

	var history []db.IncidentHistory
	if err := a.db.
		Preload("User").
		Order("created_at desc").
		Limit(4).
		Find(&history).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить ленту"})
		return
	}

	items := make([]publicFeedItem, 0, 8)

	for _, incident := range incidents {
		text := strings.TrimSpace(incident.Description)
		if len([]rune(text)) > 120 {
			text = string([]rune(text)[:120]) + "..."
		}

		items = append(items, publicFeedItem{
			Type:       "incident",
			Title:      incident.Title,
			Text:       text,
			Severity:   incident.Severity,
			Status:     incident.Status,
			CreatedAt:  incident.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			IncidentID: incident.ID,
		})
	}

	for _, entry := range history {
		items = append(items, publicFeedItem{
			Type:       "history",
			Title:      "Последнее действие",
			Text:       entry.Action,
			CreatedAt:  entry.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			IncidentID: entry.IncidentID,
		})
	}

	writeJSON(w, http.StatusOK, publicFeedResponse{Items: items})
}
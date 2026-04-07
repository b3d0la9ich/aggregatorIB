package apphttp

import (
	"net/http"

	"gorm.io/gorm"
)

type App struct {
	db *gorm.DB
}

func New(database *gorm.DB) *App {
	return &App{db: database}
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	mux.HandleFunc("/", a.servePage("./static/index.html"))
	mux.HandleFunc("/login", a.servePage("./static/login.html"))
	mux.HandleFunc("/register", a.servePage("./static/register.html"))
	mux.HandleFunc("/dashboard", a.servePage("./static/dashboard.html"))
	mux.HandleFunc("/incidents", a.servePage("./static/incidents.html"))
	mux.HandleFunc("/incident", a.servePage("./static/incident.html"))
	mux.HandleFunc("/analytics", a.servePage("./static/analytics.html"))

	mux.HandleFunc("/api/register", a.handleRegister)
	mux.HandleFunc("/api/login", a.handleLogin)
	mux.HandleFunc("/api/logout", a.handleLogout)
	mux.HandleFunc("/api/me", a.authMiddleware(a.handleMe))
	mux.HandleFunc("/api/users", a.authMiddleware(a.handleUsers))
	mux.HandleFunc("/api/incidents", a.authMiddleware(a.handleIncidents))
	mux.HandleFunc("/api/incidents/close", a.authMiddleware(a.handleCloseIncident))
	mux.HandleFunc("/api/incidents/status", a.authMiddleware(a.handleChangeStatus))
	mux.HandleFunc("/api/incidents/comments", a.authMiddleware(a.handleIncidentComments))
	mux.HandleFunc("/api/incidents/history", a.authMiddleware(a.handleIncidentHistory))
	mux.HandleFunc("/api/incidents/update", a.authMiddleware(a.adminOnly(a.handleUpdateIncident)))
	mux.HandleFunc("/api/incidents/delete", a.authMiddleware(a.adminOnly(a.handleDeleteIncident)))
	mux.HandleFunc("/api/notifications", a.authMiddleware(a.handleNotifications))
	mux.HandleFunc("/api/analytics", a.authMiddleware(a.handleAnalytics))
	mux.HandleFunc("/api/tasks", a.authMiddleware(a.handleTasks))

	return mux
}
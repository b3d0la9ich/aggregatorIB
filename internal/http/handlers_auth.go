package apphttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"aggregatorIB/internal/auth"
	"aggregatorIB/internal/db"
	"gorm.io/gorm"
)

type registerRequest struct {
	Login           string `json:"login"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный формат запроса"})
		return
	}

	req.Login = strings.TrimSpace(req.Login)
	if req.Login == "" || req.Password == "" || req.PasswordConfirm == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "заполните все поля"})
		return
	}
	if len(req.Password) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "пароль должен содержать минимум 6 символов"})
		return
	}
	if req.Password != req.PasswordConfirm {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "пароли не совпадают"})
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось создать пользователя"})
		return
	}

	user := db.User{
		Login:        req.Login,
		PasswordHash: passwordHash,
		Role:         "user",
	}

	if err := a.db.Create(&user).Error; err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "пользователь с таким логином уже существует"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message": "регистрация успешна"})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный формат запроса"})
		return
	}

	var user db.User
	if err := a.db.Where(&db.User{Login: strings.TrimSpace(req.Login)}).First(&user).Error; err != nil {
		status := http.StatusUnauthorized
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]string{"error": "неверный логин или пароль"})
		return
	}

	if err := auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "неверный логин или пароль"})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Login, user.Role)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось создать сессию"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "вход выполнен",
		"user": map[string]any{
			"id":    user.ID,
			"login": user.Login,
			"role":  user.Role,
		},
	})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"message": "выход выполнен"})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := userFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "необходима авторизация"})
		return
	}

	var user db.User
	if err := a.db.First(&user, claims.UserID).Error; err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "пользователь не найден"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":     user.ID,
		"login":  user.Login,
		"role":   user.Role,
		"avatar": strings.ToUpper(string([]rune(user.Login)[0])),
	})
}

func (a *App) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "метод не поддерживается"})
		return
	}

	var users []db.User
	if err := a.db.Order("login asc").Find(&users).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось загрузить пользователей"})
		return
	}

	writeJSON(w, http.StatusOK, users)
}

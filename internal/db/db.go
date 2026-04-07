package db

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Login        string    `gorm:"uniqueIndex;size:100;not null" json:"login"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Role         string    `gorm:"size:20;not null;default:user" json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Incident struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Title        string     `gorm:"size:200;not null" json:"title"`
	Description  string     `gorm:"type:text;not null" json:"description"`
	Severity     string     `gorm:"size:20;not null" json:"severity"`
	Status       string     `gorm:"size:20;not null;default:in_progress" json:"status"`
	OccurredAt   time.Time  `gorm:"not null" json:"occurred_at"`
	AssignedToID uint       `gorm:"not null" json:"assigned_to_id"`
	AssignedTo   User       `gorm:"foreignKey:AssignedToID" json:"assigned_to"`
	CreatedByID  uint       `gorm:"not null" json:"created_by_id"`
	CreatedBy    User       `gorm:"foreignKey:CreatedByID" json:"created_by"`
	ClosedAt     *time.Time `json:"closed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type IncidentHistory struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	IncidentID uint      `gorm:"index;not null" json:"incident_id"`
	UserID     uint      `gorm:"not null" json:"user_id"`
	User       User      `gorm:"foreignKey:UserID" json:"user"`
	Action     string    `gorm:"size:255;not null" json:"action"`
	CreatedAt  time.Time `json:"created_at"`
}

type IncidentComment struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	IncidentID uint      `gorm:"index;not null" json:"incident_id"`
	UserID     uint      `gorm:"not null" json:"user_id"`
	User       User      `gorm:"foreignKey:UserID" json:"user"`
	Text       string    `gorm:"type:text;not null" json:"text"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID" json:"user"`
	Text      string    `gorm:"size:255;not null" json:"text"`
	IsRead    bool      `gorm:"not null;default:false" json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

func Open() (*gorm.DB, error) {
	dsn := buildDSN()
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := database.AutoMigrate(
		&User{},
		&Incident{},
		&IncidentHistory{},
		&IncidentComment{},
		&Notification{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	if err := seedAdmin(database); err != nil {
		return nil, err
	}

	return database, nil
}

func buildDSN() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}

	host := envOrDefault("DB_HOST", "db")
	port := envOrDefault("DB_PORT", "5432")
	user := envOrDefault("DB_USER", "postgres")
	password := envOrDefault("DB_PASSWORD", "postgres")
	name := envOrDefault("DB_NAME", "aggregator_ib")
	sslMode := envOrDefault("DB_SSLMODE", "disable")
	timeZone := envOrDefault("DB_TIMEZONE", "UTC")

	parts := []string{
		fmt.Sprintf("host=%s", host),
		fmt.Sprintf("port=%s", port),
		fmt.Sprintf("user=%s", user),
		fmt.Sprintf("password=%s", password),
		fmt.Sprintf("dbname=%s", name),
		fmt.Sprintf("sslmode=%s", sslMode),
		fmt.Sprintf("TimeZone=%s", timeZone),
	}

	return strings.Join(parts, " ")
}

func envOrDefault(key, value string) string {
	if current := os.Getenv(key); current != "" {
		return current
	}
	return value
}

func seedAdmin(database *gorm.DB) error {
	var count int64
	if err := database.Model(&User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if count > 0 {
		return nil
	}

	passwordHash, err := HashDefaultAdminPassword()
	if err != nil {
		return err
	}

	admin := User{
		Login:        envOrDefault("ADMIN_LOGIN", "admin"),
		PasswordHash: passwordHash,
		Role:         "admin",
	}

	if err := database.Create(&admin).Error; err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	return nil
}

func HashDefaultAdminPassword() (string, error) {
	password := envOrDefault("ADMIN_PASSWORD", "admin123")
	return hashPassword(password)
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}
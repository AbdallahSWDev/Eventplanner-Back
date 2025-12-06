package main

import (
	"time"
	"gorm.io/gorm"
)

// User represents a registered user
type User struct {
	gorm.Model
	ID        uint      `json:"id" gorm:"primaryKey"`
	Email     string    `json:"email" gorm:"uniqueIndex;not null"`
	Password  string    `json:"password,omitempty"` // FIXED: bind JSON but do not return in responses
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Event is the core event model
type Event struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Title       string    `json:"title" gorm:"not null"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	Date        time.Time `json:"date" gorm:"not null"`
	OrganizerID uint      `json:"organizer_id" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Organizer User   `gorm:"foreignKey:OrganizerID" json:"organizer,omitempty"`
	Tasks     []Task `gorm:"foreignKey:EventID" json:"tasks,omitempty"`
}

type Task struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	EventID     uint      `json:"event_id" gorm:"index;not null"`
	Title       string    `json:"title" gorm:"not null"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type EventAttendee struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	EventID   uint      `json:"event_id" gorm:"index;not null"`
	UserID    uint      `json:"user_id" gorm:"index;not null"`
	Role      string    `json:"role" gorm:"type:varchar(32);not null"`
	Status    string    `json:"status" gorm:"type:varchar(32)"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

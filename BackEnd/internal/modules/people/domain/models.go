package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	EmploymentStatusActive   = "active"
	EmploymentStatusInactive = "inactive"

	CustomerSourceOnline = "online"
	CustomerSourceWalkIn = "walk_in"
)

// Employee represents an internal hotel staff member bound 1-to-1 with an identity User.
type Employee struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"userId"`
	EmployeeCode     string     `json:"employeeCode"`
	FullName         string     `json:"fullName"`
	Phone            string     `json:"phone"`
	Email            string     `json:"email"`
	PositionTitle    string     `json:"positionTitle"`
	EmploymentStatus string     `json:"employmentStatus"`
	AccountStatus    string     `json:"accountStatus,omitempty"`
	Roles            []string   `json:"roles,omitempty"`
	ArchivedAt       *time.Time `json:"archivedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	Version          int        `json:"version"`
}

// Customer represents an online or walk-in hotel guest profile.
type Customer struct {
	ID              uuid.UUID  `json:"id"`
	UserID          *uuid.UUID `json:"userId,omitempty"`
	FullName        string     `json:"fullName"`
	Phone           string     `json:"phone"`
	Email           string     `json:"email,omitempty"`
	NationalityCode string     `json:"nationalityCode,omitempty"`
	GovernmentID    string     `json:"governmentId,omitempty"`
	Address         string     `json:"address,omitempty"`
	Source          string     `json:"source"`
	ArchivedAt      *time.Time `json:"archivedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	Version         int        `json:"version"`
}

// MaskGovernmentID masks all but the last 4 characters of a government ID for list responses and logs.
func MaskGovernmentID(raw string) string {
	n := len(raw)
	if n == 0 {
		return ""
	}
	if n <= 4 {
		return "****"
	}
	return "****" + raw[n-4:]
}

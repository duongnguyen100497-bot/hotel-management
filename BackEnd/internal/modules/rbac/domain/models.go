package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleSystemAdministrator = "system-administrator"
	RoleHotelManager        = "hotel-manager"
	RoleFrontDesk           = "front-desk"
	RoleAccounting          = "accounting"
)

// Permission represents a capability code in the permission registry.
type Permission struct {
	ID          uuid.UUID  `json:"id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	IsSystem    bool       `json:"isSystem"`
	ArchivedAt  *time.Time `json:"archivedAt,omitempty"`
}

// Role represents a dynamic RBAC role. At most one active role has IsRoot = true.
type Role struct {
	ID          uuid.UUID    `json:"id"`
	Code        string       `json:"code"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	IsSystem    bool         `json:"isSystem"`
	IsRoot      bool         `json:"isRoot"`
	ArchivedAt  *time.Time   `json:"archivedAt,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
	Version     int          `json:"version"`
	Permissions []Permission `json:"permissions,omitempty"`
}

// CanonicalPermissions returns the authoritative list of system permission codes.
func CanonicalPermissions() []string {
	return []string{
		"user.read", "user.manage",
		"role.read", "role.create", "role.update", "role.delete", "role.update_system",
		"permission.read", "permission.manage",
		"role_permission.assign", "user_role.assign",
		"employee.read", "employee.create", "employee.update", "employee.deactivate",
		"customer.read", "customer.create", "customer.update", "customer.archive",
		"hotel_settings.read", "hotel_settings.update",
		"vat.manage",
		"property.read", "room_type.manage", "room.create", "room.update", "room.archive", "room.state.update",
		"service.read", "service.manage",
		"media.manage",
		"booking.read", "booking.create", "booking.confirm", "booking.cancel",
		"wallet.read", "wallet.deposit",
		"payment.read", "payment.record", "payment.reconcile", "payment.refund",
		"stay.check_in", "stay.check_out", "stay.service.manage",
		"invoice.read",
		"report.read", "report.export",
		"audit.read",
	}
}

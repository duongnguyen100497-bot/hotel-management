package http

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

type maintenanceTransitionRequest struct {
	Action  string `json:"action"`
	Reason  string `json:"reason,omitempty"`
	Version *int   `json:"version,omitempty"`
}

type readyTransitionRequest struct {
	Reason  string `json:"reason,omitempty"`
	Version *int   `json:"version,omitempty"`
}

func (h *Handler) handleRoomMaintenance(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	roomID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var req maintenanceTransitionRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	expectedVer, err := resolveOptionalVersion(c, req.Version)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	updated, err := h.svc.TransitionMaintenance(c.Request.Context(), actor, propertyapp.TransitionMaintenanceParams{
		RoomID:          roomID,
		Action:          req.Action,
		Reason:          req.Reason,
		ExpectedVersion: expectedVer,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, updated.Version))
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": updated})
}

func (h *Handler) handleRoomReady(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	roomID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var req readyTransitionRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid request body"))
			return
		}
		if strings.TrimSpace(string(raw)) != "" {
			c.Request.Body = io.NopCloser(strings.NewReader(string(raw)))
			if err := platformhttp.DecodeJSON(c, &req); err != nil {
				platformhttp.WriteError(c, err)
				return
			}
		}
	}

	expectedVer, err := resolveOptionalVersion(c, req.Version)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	updated, err := h.svc.MarkReady(c.Request.Context(), actor, roomID, req.Reason, expectedVer)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, updated.Version))
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": updated})
}

func (h *Handler) handleListRoomStateHistory(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	roomID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	limit, cursor, err := platformhttp.ParseCursorParams(c, 20, 100)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	page, err := h.svc.ListRoomStateHistory(c.Request.Context(), actor, roomID, cursor, limit)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, page)
}

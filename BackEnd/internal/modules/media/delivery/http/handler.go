package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	mediaapp "github.com/trandinh0506/hotel-management/backend/internal/modules/media/application"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// Handler serves MinIO media upload, completion, room/room-type linking, and deletion endpoints.
type Handler struct {
	svc        *mediaapp.Service
	authorizer rbacapp.Authorizer
}

// NewHandler constructs a media HTTP handler.
func NewHandler(svc *mediaapp.Service, authorizer rbacapp.Authorizer) *Handler {
	return &Handler{
		svc:        svc,
		authorizer: authorizer,
	}
}

// RegisterRoutes mounts media management routes on api (/api/v1).
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	secured := api.Group("")
	if authMiddleware != nil {
		secured.Use(authMiddleware)
	}

	mediaPerm := rbacapp.RequirePermission(h.authorizer, "media.manage")
	secured.POST("/media/uploads", mediaPerm, h.handleRequestUpload)
	secured.POST("/media/:id/complete", mediaPerm, h.handleCompleteUpload)
	secured.DELETE("/media/:id", mediaPerm, h.handleDeleteMedia)

	secured.POST("/rooms/:id/media", mediaPerm, h.handleLinkRoomMedia)
	secured.PATCH("/rooms/:id/media/:mediaId", mediaPerm, h.handleUpdateRoomMedia)

	secured.POST("/room-types/:id/media", mediaPerm, h.handleLinkRoomTypeMedia)
	secured.PATCH("/room-types/:id/media/:mediaId", mediaPerm, h.handleUpdateRoomTypeMedia)
}

type requestUploadRequest struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
}

type completeUploadRequest struct {
	RoomID     *uuid.UUID `json:"roomId,omitempty"`
	RoomTypeID *uuid.UUID `json:"roomTypeId,omitempty"`
	SortOrder  int        `json:"sortOrder"`
	IsCover    bool       `json:"isCover"`
}

type linkMediaRequest struct {
	MediaID   uuid.UUID `json:"mediaId"`
	SortOrder int       `json:"sortOrder"`
	IsCover   bool      `json:"isCover"`
}

type updateMediaLinkRequest struct {
	SortOrder *int  `json:"sortOrder,omitempty"`
	IsCover   *bool `json:"isCover,omitempty"`
}

func (h *Handler) handleRequestUpload(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	var req requestUploadRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	ticket, err := h.svc.RequestUpload(c.Request.Context(), actor, mediaapp.RequestUploadParams{
		FileName:    req.FileName,
		ContentType: req.ContentType,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	platformhttp.WriteJSON(c, http.StatusCreated, gin.H{"data": ticket})
}

func (h *Handler) handleCompleteUpload(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	mediaID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var req completeUploadRequest
	if c.Request.ContentLength > 0 {
		if err := platformhttp.DecodeJSON(c, &req); err != nil {
			platformhttp.WriteError(c, err)
			return
		}
	}

	asset, err := h.svc.CompleteUpload(c.Request.Context(), actor, mediaapp.CompleteUploadParams{
		MediaID:    mediaID,
		RoomID:     req.RoomID,
		RoomTypeID: req.RoomTypeID,
		SortOrder:  req.SortOrder,
		IsCover:    req.IsCover,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": asset})
}

func (h *Handler) handleDeleteMedia(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	mediaID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	asset, err := h.svc.DeleteMedia(c.Request.Context(), actor, mediaID)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": asset})
}

func (h *Handler) handleLinkRoomMedia(c *gin.Context) {
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

	var req linkMediaRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	if req.MediaID == uuid.Nil {
		platformhttp.WriteError(c, apperror.Validation("mediaId is required", apperror.FieldError{
			Field:   "mediaId",
			Message: "must be a valid UUID",
		}))
		return
	}

	link, err := h.svc.LinkMedia(c.Request.Context(), actor, mediaapp.LinkMediaParams{
		RoomID:    &roomID,
		MediaID:   req.MediaID,
		SortOrder: req.SortOrder,
		IsCover:   req.IsCover,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	platformhttp.WriteJSON(c, http.StatusCreated, gin.H{"data": link})
}

func (h *Handler) handleUpdateRoomMedia(c *gin.Context) {
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
	mediaID, err := parseUUIDParam(c, "mediaId")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var req updateMediaLinkRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	link, err := h.svc.UpdateMediaLink(c.Request.Context(), actor, mediaapp.UpdateMediaLinkParams{
		RoomID:    &roomID,
		MediaID:   mediaID,
		SortOrder: req.SortOrder,
		IsCover:   req.IsCover,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": link})
}

func (h *Handler) handleLinkRoomTypeMedia(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	roomTypeID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var req linkMediaRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	if req.MediaID == uuid.Nil {
		platformhttp.WriteError(c, apperror.Validation("mediaId is required", apperror.FieldError{
			Field:   "mediaId",
			Message: "must be a valid UUID",
		}))
		return
	}

	link, err := h.svc.LinkMedia(c.Request.Context(), actor, mediaapp.LinkMediaParams{
		RoomTypeID: &roomTypeID,
		MediaID:    req.MediaID,
		SortOrder:  req.SortOrder,
		IsCover:    req.IsCover,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	platformhttp.WriteJSON(c, http.StatusCreated, gin.H{"data": link})
}

func (h *Handler) handleUpdateRoomTypeMedia(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	roomTypeID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	mediaID, err := parseUUIDParam(c, "mediaId")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var req updateMediaLinkRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	link, err := h.svc.UpdateMediaLink(c.Request.Context(), actor, mediaapp.UpdateMediaLinkParams{
		RoomTypeID: &roomTypeID,
		MediaID:    mediaID,
		SortOrder:  req.SortOrder,
		IsCover:    req.IsCover,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": link})
}

func parseUUIDParam(c *gin.Context, param string) (uuid.UUID, error) {
	raw := strings.TrimSpace(c.Param(param))
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperror.Validation("invalid "+param+" parameter", apperror.FieldError{
			Field:   param,
			Message: "must be a valid UUID",
		})
	}
	return parsed, nil
}

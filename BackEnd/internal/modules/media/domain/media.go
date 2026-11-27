package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

const (
	StatusPending         = "pending"
	StatusReady           = "ready"
	StatusDeletionPending = "deletion_pending"
	StatusDeleted         = "deleted"

	MimeJPEG = "image/jpeg"
	MimePNG  = "image/png"
	MimeWEBP = "image/webp"

	MaxImageDimension = 16384
)

var pngMagicSignature = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

// MediaAsset represents a MinIO-backed media object tracked in PostgreSQL.
// Note: ObjectKey is internal server state and is intentionally excluded from JSON responses.
type MediaAsset struct {
	ID         uuid.UUID  `json:"id"`
	ObjectKey  string     `json:"-"`
	SHA256Hex  string     `json:"sha256,omitempty"`
	SHA256Raw  []byte     `json:"-"`
	MimeType   string     `json:"mimeType,omitempty"`
	ByteSize   int64      `json:"byteSize,omitempty"`
	Width      int        `json:"width,omitempty"`
	Height     int        `json:"height,omitempty"`
	Status     string     `json:"status"`
	URL        string     `json:"url,omitempty"`
	UploadedBy *uuid.UUID `json:"uploadedBy,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// RoomMediaLink represents a binding between a ready MediaAsset and either a Room or a RoomType.
type RoomMediaLink struct {
	ID           uuid.UUID  `json:"id"`
	RoomID       *uuid.UUID `json:"roomId,omitempty"`
	RoomTypeID   *uuid.UUID `json:"roomTypeId,omitempty"`
	MediaAssetID uuid.UUID  `json:"mediaId"`
	SortOrder    int        `json:"sortOrder"`
	IsCover      bool       `json:"isCover"`
	URL          string     `json:"url,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// ValidatedImageMetadata holds server-verified bitmap metadata extracted from raw object bytes.
type ValidatedImageMetadata struct {
	MimeType  string
	ByteSize  int64
	Width     int
	Height    int
	SHA256Raw []byte
	SHA256Hex string
}

// IsAllowedImageMime reports whether mime is one of the 3 safe raster formats (JPEG, PNG, WebP).
// SVG, HTML, XML, text, and executables are unconditionally forbidden.
func IsAllowedImageMime(mime string) bool {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case MimeJPEG, MimePNG, MimeWEBP:
		return true
	default:
		return false
	}
}

// ValidateDeclaredUpload validates the client-declared fileName and contentType before issuing a presigned PUT URL.
// The server NEVER uses fileName in object_key (preventing path traversal).
func ValidateDeclaredUpload(fileName, contentType string) (string, error) {
	trimmedName := strings.TrimSpace(fileName)
	normMime := strings.ToLower(strings.TrimSpace(contentType))

	var fields []apperror.FieldError
	if trimmedName == "" || len(trimmedName) > 255 {
		fields = append(fields, apperror.FieldError{
			Field:   "fileName",
			Message: "must be 1..255 characters",
		})
	}
	if !IsAllowedImageMime(normMime) {
		fields = append(fields, apperror.FieldError{
			Field:   "contentType",
			Message: "only image/jpeg, image/png, and image/webp are allowed (SVG/HTML/executables are forbidden)",
		})
	}
	if len(fields) > 0 {
		return "", apperror.Validation("invalid media upload request", fields...)
	}
	return normMime, nil
}

// DetectMagicByteMime inspects the leading bytes of data to identify raster image magic bytes
// or common disallowed content (SVG, HTML, XML, executables, text).
func DetectMagicByteMime(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], pngMagicSignature) {
		return MimePNG
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return MimeJPEG
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return MimeWEBP
	}

	sniffLen := len(data)
	if sniffLen > 512 {
		sniffLen = 512
	}
	prefix := strings.ToLower(strings.TrimSpace(string(data[:sniffLen])))
	switch {
	case strings.Contains(prefix, "<svg"):
		return "image/svg+xml"
	case strings.HasPrefix(prefix, "<!doctype html"), strings.HasPrefix(prefix, "<html"), strings.Contains(prefix, "<script"):
		return "text/html"
	case strings.HasPrefix(prefix, "<?xml"):
		return "text/xml"
	case len(data) >= 4 && data[0] == 0x7F && data[1] == 'E' && data[2] == 'L' && data[3] == 'F':
		return "application/x-elf"
	case len(data) >= 2 && data[0] == 'M' && data[1] == 'Z':
		return "application/x-dosexec"
	default:
		return "application/octet-stream"
	}
}

// InspectAndDecodeImage inspects raw object bytes using magic-byte detection
// and full raster bitmap decoding (JPEG, PNG, WebP). Any non-image, SVG, HTML, corrupt bitmap,
// or payload exceeding maxBytes is rejected with 422 Validation Failed.
func InspectAndDecodeImage(data []byte, maxBytes int64) (ValidatedImageMetadata, error) {
	if maxBytes <= 0 {
		maxBytes = 5 * 1024 * 1024
	}
	size := int64(len(data))
	if size == 0 {
		return ValidatedImageMetadata{}, apperror.Validation("uploaded media object is empty", apperror.FieldError{
			Field:   "file",
			Message: "uploaded file must not be empty",
		})
	}
	if size > maxBytes {
		return ValidatedImageMetadata{}, apperror.Validation("uploaded media object exceeds maximum allowed byte size", apperror.FieldError{
			Field:   "byteSize",
			Message: fmt.Sprintf("must not exceed %d bytes", maxBytes),
		})
	}

	detectedMime := DetectMagicByteMime(data)
	if !IsAllowedImageMime(detectedMime) {
		return ValidatedImageMetadata{}, apperror.Validation("disallowed or spoofed file content detected via magic bytes", apperror.FieldError{
			Field:   "contentType",
			Message: fmt.Sprintf("detected %q; only raster JPEG, PNG, and WebP images are permitted", detectedMime),
		})
	}

	// Check image dimensions first via DecodeConfig to guard against decompression bombs before full pixel decode.
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ValidatedImageMetadata{}, apperror.Validation("corrupt or unsupported image header", apperror.FieldError{
			Field:   "file",
			Message: "failed to decode raster image header",
		})
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension {
		return ValidatedImageMetadata{}, apperror.Validation("invalid image dimensions", apperror.FieldError{
			Field:   "dimensions",
			Message: fmt.Sprintf("image dimensions %dx%d out of allowed bounds", cfg.Width, cfg.Height),
		})
	}

	// Ensure format matches magic-byte detected MIME type.
	expectedFormat := ""
	switch detectedMime {
	case MimeJPEG:
		expectedFormat = "jpeg"
	case MimePNG:
		expectedFormat = "png"
	case MimeWEBP:
		expectedFormat = "webp"
	}
	if format != expectedFormat {
		return ValidatedImageMetadata{}, apperror.Validation("image format mismatch", apperror.FieldError{
			Field:   "contentType",
			Message: fmt.Sprintf("magic bytes matched %s but decoder identified %s", detectedMime, format),
		})
	}

	// Decode the full bitmap to guarantee payload integrity.
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ValidatedImageMetadata{}, apperror.Validation("corrupt image bitmap payload", apperror.FieldError{
			Field:   "file",
			Message: "failed to decode full image bitmap",
		})
	}
	bounds := img.Bounds()
	sum := sha256.Sum256(data)

	return ValidatedImageMetadata{
		MimeType:  detectedMime,
		ByteSize:  size,
		Width:     bounds.Dx(),
		Height:    bounds.Dy(),
		SHA256Raw: sum[:],
		SHA256Hex: hex.EncodeToString(sum[:]),
	}, nil
}

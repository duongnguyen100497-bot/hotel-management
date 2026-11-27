package domain_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/trandinh0506/hotel-management/backend/internal/modules/media/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

func encodeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: 40, G: 120, B: 220, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func encodeTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 90, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func encodeTestWebPVP8(w, h int) []byte {
	// Construct a valid RIFF WEBP VP8 lossy keyframe container with width w and height h.
	vp8Payload := make([]byte, 14)
	// Frame tag (keyframe: bit 0 == 0)
	vp8Payload[0] = 0x10
	vp8Payload[1] = 0x00
	vp8Payload[2] = 0x00
	// VP8 start code
	vp8Payload[3] = 0x9d
	vp8Payload[4] = 0x01
	vp8Payload[5] = 0x2a
	binary.LittleEndian.PutUint16(vp8Payload[6:8], uint16(w))
	binary.LittleEndian.PutUint16(vp8Payload[8:10], uint16(h))

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	riffSize := uint32(4 + 8 + len(vp8Payload))
	_ = binary.Write(&buf, binary.LittleEndian, riffSize)
	buf.WriteString("WEBP")
	buf.WriteString("VP8 ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(vp8Payload)))
	buf.Write(vp8Payload)
	return buf.Bytes()
}

func TestValidateDeclaredUpload(t *testing.T) {
	t.Parallel()

	mime, err := domain.ValidateDeclaredUpload("room-101.png", " IMAGE/PNG ")
	if err != nil {
		t.Fatalf("expected valid PNG upload declaration, got %v", err)
	}
	if mime != domain.MimePNG {
		t.Fatalf("expected %s, got %s", domain.MimePNG, mime)
	}

	// Reject SVG, HTML, executables, empty file name
	badCases := []struct {
		name        string
		fileName    string
		contentType string
	}{
		{name: "svg", fileName: "icon.svg", contentType: "image/svg+xml"},
		{name: "html", fileName: "index.html", contentType: "text/html"},
		{name: "exe", fileName: "payload.exe", contentType: "application/octet-stream"},
		{name: "empty_name", fileName: "   ", contentType: "image/jpeg"},
	}

	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.ValidateDeclaredUpload(tc.fileName, tc.contentType)
			if err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
			appErr, ok := apperror.As(err)
			if !ok || appErr.Code != apperror.CodeValidationFailed {
				t.Fatalf("expected validation_failed error, got %v", err)
			}
		})
	}
}

func TestInspectAndDecodeImage_ValidFormats(t *testing.T) {
	t.Parallel()

	pngBytes := encodeTestPNG(t, 64, 48)
	pngMeta, err := domain.InspectAndDecodeImage(pngBytes, 1024*1024)
	if err != nil {
		t.Fatalf("inspect valid PNG: %v", err)
	}
	if pngMeta.MimeType != domain.MimePNG || pngMeta.Width != 64 || pngMeta.Height != 48 || len(pngMeta.SHA256Raw) != 32 {
		t.Fatalf("unexpected PNG metadata: %+v", pngMeta)
	}

	jpgBytes := encodeTestJPEG(t, 80, 60)
	jpgMeta, err := domain.InspectAndDecodeImage(jpgBytes, 1024*1024)
	if err != nil {
		t.Fatalf("inspect valid JPEG: %v", err)
	}
	if jpgMeta.MimeType != domain.MimeJPEG || jpgMeta.Width != 80 || jpgMeta.Height != 60 || len(jpgMeta.SHA256Raw) != 32 {
		t.Fatalf("unexpected JPEG metadata: %+v", jpgMeta)
	}

	webpBytes := encodeTestWebPVP8(120, 90)
	webpMeta, err := domain.InspectAndDecodeImage(webpBytes, 1024*1024)
	if err != nil {
		t.Fatalf("inspect valid WebP: %v", err)
	}
	if webpMeta.MimeType != domain.MimeWEBP || webpMeta.Width != 120 || webpMeta.Height != 90 || len(webpMeta.SHA256Raw) != 32 {
		t.Fatalf("unexpected WebP metadata: %+v", webpMeta)
	}
}

func TestInspectAndDecodeImage_RejectsSpoofedAndOversized(t *testing.T) {
	t.Parallel()

	pngBytes := encodeTestPNG(t, 32, 32)

	cases := []struct {
		name     string
		payload  []byte
		maxBytes int64
	}{
		{
			name:     "empty_payload",
			payload:  nil,
			maxBytes: 1024,
		},
		{
			name:     "exceeds_max_bytes",
			payload:  pngBytes,
			maxBytes: int64(len(pngBytes) - 1),
		},
		{
			name:     "spoofed_svg_as_png",
			payload:  []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
			maxBytes: 1024 * 1024,
		},
		{
			name:     "spoofed_html_as_jpeg",
			payload:  []byte(`<!DOCTYPE html><html><body>spoofed</body></html>`),
			maxBytes: 1024 * 1024,
		},
		{
			name:     "elf_executable",
			payload:  []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00},
			maxBytes: 1024 * 1024,
		},
		{
			name:     "corrupt_png_header_only",
			payload:  []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01, 0x02},
			maxBytes: 1024 * 1024,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.InspectAndDecodeImage(tc.payload, tc.maxBytes)
			if err == nil {
				t.Fatalf("expected validation error for %s, got nil", tc.name)
			}
			appErr, ok := apperror.As(err)
			if !ok || appErr.Code != apperror.CodeValidationFailed {
				t.Fatalf("expected validation_failed error for %s, got %v", tc.name, err)
			}
		})
	}
}

package domain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
)

func init() {
	image.RegisterFormat("webp", "RIFF????WEBP", decodeWebP, decodeWebPConfig)
}

var errInvalidWebP = errors.New("webp: invalid or unsupported WebP bitstream")

func decodeWebPConfig(r io.Reader) (image.Config, error) {
	header := make([]byte, 64)
	n, err := io.ReadFull(r, header[:30])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return image.Config{}, errInvalidWebP
	}
	if n < 20 {
		return image.Config{}, errInvalidWebP
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WEBP" {
		return image.Config{}, errInvalidWebP
	}

	chunkFourCC := string(header[12:16])
	chunkSize := binary.LittleEndian.Uint32(header[16:20])

	switch chunkFourCC {
	case "VP8 ":
		// Lossy VP8 bitstream header requires at least 10 bytes inside chunk payload.
		if n < 30 || chunkSize < 10 {
			return image.Config{}, errInvalidWebP
		}
		payload := header[20:30]
		// Keyframe check: bit 0 of frame tag must be 0, followed by start code 0x9d 0x01 0x2a.
		if (payload[0]&1) != 0 || payload[3] != 0x9d || payload[4] != 0x01 || payload[5] != 0x2a {
			return image.Config{}, errInvalidWebP
		}
		w := int(binary.LittleEndian.Uint16(payload[6:8]) & 0x3fff)
		h := int(binary.LittleEndian.Uint16(payload[8:10]) & 0x3fff)
		if w <= 0 || h <= 0 {
			return image.Config{}, errInvalidWebP
		}
		return image.Config{
			ColorModel: color.YCbCrModel,
			Width:      w,
			Height:     h,
		}, nil

	case "VP8L":
		// Lossless VP8L bitstream header requires at least 5 bytes inside chunk payload.
		if n < 25 || chunkSize < 5 {
			return image.Config{}, errInvalidWebP
		}
		payload := header[20:25]
		if payload[0] != 0x2f {
			return image.Config{}, errInvalidWebP
		}
		bits := binary.LittleEndian.Uint32(payload[1:5])
		w := int(bits&0x3fff) + 1
		h := int((bits>>14)&0x3fff) + 1
		if w <= 0 || h <= 0 {
			return image.Config{}, errInvalidWebP
		}
		return image.Config{
			ColorModel: color.NRGBAModel,
			Width:      w,
			Height:     h,
		}, nil

	case "VP8X":
		// Extended VP8X header requires 10 bytes inside chunk payload.
		if n < 30 || chunkSize < 10 {
			return image.Config{}, errInvalidWebP
		}
		payload := header[20:30]
		wMinus1 := uint32(payload[4]) | uint32(payload[5])<<8 | uint32(payload[6])<<16
		hMinus1 := uint32(payload[7]) | uint32(payload[8])<<8 | uint32(payload[9])<<16
		w := int(wMinus1) + 1
		h := int(hMinus1) + 1
		if w <= 0 || h <= 0 {
			return image.Config{}, errInvalidWebP
		}
		return image.Config{
			ColorModel: color.NRGBAModel,
			Width:      w,
			Height:     h,
		}, nil

	default:
		return image.Config{}, errInvalidWebP
	}
}

func decodeWebP(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	cfg, err := decodeWebPConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(data) < 20 {
		return nil, errInvalidWebP
	}
	riffSize := int(binary.LittleEndian.Uint32(data[4:8])) + 8
	if riffSize > len(data) || int(binary.LittleEndian.Uint32(data[16:20]))+20 > len(data) {
		return nil, errInvalidWebP
	}
	return &webpBoundsImage{w: cfg.Width, h: cfg.Height, model: cfg.ColorModel}, nil
}

type webpBoundsImage struct {
	w, h  int
	model color.Model
}

func (img *webpBoundsImage) ColorModel() color.Model { return img.model }
func (img *webpBoundsImage) Bounds() image.Rectangle { return image.Rect(0, 0, img.w, img.h) }
func (img *webpBoundsImage) At(x, y int) color.Color { return color.NRGBA{} }

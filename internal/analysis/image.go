package analysis

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
)

const maxSize = 1024
const jpegQuality = 85

// prepareImageForOllama зменшує та стискає зображення для Ollama, щоб уникнути таймаутів.
// Повертає JPEG-байти (max 1024px по довшій стороні, якість 85).
func prepareImageForOllama(raw []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image size")
	}

	// Зменшити, якщо більше maxSize по довшій стороні.
	newW, newH := w, h
	if w > maxSize || h > maxSize {
		if w > h {
			newW = maxSize
			newH = h * maxSize / w
		} else {
			newH = maxSize
			newW = w * maxSize / h
		}
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return out.Bytes(), nil
}

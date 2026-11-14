package application

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"math"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// shareJPEG runs only during an explicit share-image upload, never on page reads.
// Letterboxing preserves the entire image and compositing preserves transparency.
func shareJPEG(src io.ReadSeeker) ([]byte, error) {
	if _, err := src.Seek(0, 0); err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(src)
	if err != nil {
		return nil, newUploadValidationError("分享图必须是 PNG、JPEG、WebP 或 GIF 栅格图")
	}
	if cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 40_000_000 {
		return nil, newUploadValidationError("分享图像素数量超过限制")
	}
	if _, err := src.Seek(0, 0); err != nil {
		return nil, err
	}
	original, _, err := image.Decode(src)
	if err != nil {
		return nil, newUploadValidationError("无法解析分享图")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 235, G: 246, B: 240, A: 255}}, image.Point{}, draw.Src)
	scale := math.Min(1200/float64(cfg.Width), 630/float64(cfg.Height))
	width, height := max(1, int(float64(cfg.Width)*scale)), max(1, int(float64(cfg.Height)*scale))
	x, y := (1200-width)/2, (630-height)/2
	xdraw.CatmullRom.Scale(canvas, image.Rect(x, y, x+width, y+height), original, original.Bounds(), draw.Over, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, canvas, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

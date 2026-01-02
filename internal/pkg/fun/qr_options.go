package fun

import (
	"errors"
	"fmt"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"
	"github.com/yeqown/go-qrcode/writer/standard"
)

// heartShape implements a custom heart shape for QR code
type heartShape struct{}

// DrawFinder draws the finder pattern (the three corner squares) with rectangles
func (hs *heartShape) DrawFinder(ctx *standard.DrawContext) {
	w, h := ctx.Edge()
	x, y := ctx.UpperLeft()
	color := ctx.Color()

	ctx.DrawRectangle(x, y, float64(w), float64(h))
	ctx.SetColor(color)
	ctx.Fill()
}

// Draw draws each QR code module as a heart shape
func (hs *heartShape) Draw(ctx *standard.DrawContext) {
	w, h := ctx.Edge()
	x, y := ctx.UpperLeft()
	color := ctx.Color()

	// Heart shape parameters
	cx, cy := x+float64(w)/2.0, y+float64(h)/2.0
	size := float64(w) / 2.0
	if float64(h)/2.0 < size {
		size = float64(h) / 2.0
	}

	// Draw heart shape using path
	// Heart equation: (x^2 + y^2 - 1)^3 - x^2*y^3 = 0
	ctx.MoveTo(cx, cy-size*0.3)

	// Top left arc
	ctx.CubicTo(
		cx-size*0.5, cy-size*0.8,
		cx-size, cy-size*0.3,
		cx-size, cy,
	)

	// Bottom left curve
	ctx.CubicTo(
		cx-size, cy+size*0.3,
		cx-size*0.5, cy+size*0.6,
		cx, cy+size,
	)

	// Bottom right curve
	ctx.CubicTo(
		cx+size*0.5, cy+size*0.6,
		cx+size, cy+size*0.3,
		cx+size, cy,
	)

	// Top right arc
	ctx.CubicTo(
		cx+size, cy-size*0.3,
		cx+size*0.5, cy-size*0.8,
		cx, cy-size*0.3,
	)

	ctx.SetColor(color)
	ctx.Fill()
}

// emojiShape implements a custom laughing emoji shape for QR code
type emojiShape struct{}

// DrawFinder draws the finder pattern (the three corner squares) with rectangles
func (es *emojiShape) DrawFinder(ctx *standard.DrawContext) {
	w, h := ctx.Edge()
	x, y := ctx.UpperLeft()
	color := ctx.Color()

	ctx.DrawRectangle(x, y, float64(w), float64(h))
	ctx.SetColor(color)
	ctx.Fill()
}

// Draw draws each QR code module as a simple circle (emoji base)
func (es *emojiShape) Draw(ctx *standard.DrawContext) {
	w, h := ctx.Edge()
	x, y := ctx.UpperLeft()
	moduleColor := ctx.Color()

	cx, cy := x+float64(w)/2.0, y+float64(h)/2.0
	radius := float64(w) / 2.0
	if float64(h)/2.0 < radius {
		radius = float64(h) / 2.0
	}

	// Draw base circle
	ctx.DrawCircle(cx, cy, radius)
	ctx.SetColor(moduleColor)
	ctx.Fill()

	// Only draw eyes and smile if this is a filled module (yellow, not white)
	// Check if the color is not white/background
	r, g, b, _ := moduleColor.RGBA()
	// Convert to 8-bit values for comparison
	isColored := !(r > 60000 && g > 60000 && b > 60000) // Not white

	if isColored {
		// Draw eyes and smile with contrasting color (black)
		blackColor := color.RGBA{R: 0, G: 0, B: 0, A: 255}

		// Left eye (small filled circle)
		leftEyeX := cx - radius*0.35
		eyeY := cy - radius*0.25
		eyeRadius := radius * 0.15
		ctx.DrawCircle(leftEyeX, eyeY, eyeRadius)
		ctx.SetColor(blackColor)
		ctx.Fill()

		// Right eye (small filled circle)
		rightEyeX := cx + radius*0.35
		ctx.DrawCircle(rightEyeX, eyeY, eyeRadius)
		ctx.SetColor(blackColor)
		ctx.Fill()

		// Draw smile (arc)
		smileY := cy + radius*0.15
		smileWidth := radius * 0.7

		// Create smile arc using quadratic curve
		ctx.MoveTo(cx-smileWidth/2, smileY)
		ctx.QuadraticTo(cx, smileY+radius*0.35, cx+smileWidth/2, smileY)
		ctx.SetLineWidth(radius * 0.15)
		ctx.SetColor(blackColor)
		ctx.Stroke()
	}
}

// QRWithQRWidth sets the width of the QR code modules.
// If qrWidth is less than or equal to 0, it defaults to 8.
func QRWithQRWidth(qrWidth uint8) []standard.ImageOption {
	if qrWidth <= 0 {
		qrWidth = 8
	}

	options := []standard.ImageOption{
		standard.WithQRWidth(qrWidth),
	}

	return options
}

// QRWithCircleShape sets the shape of the QR code modules to circle.
func QRWithCircleShape() []standard.ImageOption {
	options := []standard.ImageOption{
		standard.WithCircleShape(),
	}

	return options
}

// QRWithBorderWidth sets the width of the border around the QR code.
// If borderWidth is less than or equal to 0, it defaults to 10.
func QRWithBorderWidth(borderWidth int) []standard.ImageOption {
	if borderWidth <= 0 {
		borderWidth = 10
	}

	options := []standard.ImageOption{
		standard.WithBorderWidth(borderWidth),
	}

	return options
}

// QRWithBgTransparent sets the background of the QR code to be transparent
func QRWithBgTransparent() []standard.ImageOption {
	options := []standard.ImageOption{
		standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
		standard.WithBgTransparent(),
	}

	return options
}

// QRWithBgFgColor sets the background and foreground colors of the QR code.
// If either bgHex or fgHex is an empty string, no options are returned.
func QRWithBgFgColor(bgHex, fgHex string) []standard.ImageOption {
	if bgHex == "" || fgHex == "" {
		return []standard.ImageOption{}
	}

	options := []standard.ImageOption{
		standard.WithBgColorRGBHex(bgHex),
		standard.WithFgColorRGBHex(fgHex),
	}
	return options
}

// QRWithHalfToneImg sets a halftone image for the QR code.
// It requires the path to the halftone image and the desired QR code width.
// If the image file does not exist, an error is returned.
// The halftone image must be a .jpg or .jpeg file.
// If qrWidth is less than or equal to 0, it defaults to 21.
func QRWithHalfToneImg(halfToneImgPath string, qrWidth uint8) ([]standard.ImageOption, error) {
	if qrWidth <= 0 {
		qrWidth = 21
	}

	if _, err := os.Stat(halfToneImgPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("halftone img path: %s does not exists", halfToneImgPath)
	}

	ext := strings.ToLower(filepath.Ext(halfToneImgPath))
	if ext != ".jpg" && ext != ".jpeg" {
		return nil, fmt.Errorf("halftone image must be a .jpg or .jpeg file")
	}

	options := []standard.ImageOption{
		standard.WithHalftone(halfToneImgPath),
		standard.WithQRWidth(qrWidth),
	}
	return options, nil

}

// QRWithLogo sets a logo image at the center of the QR code.
// It requires the path to the logo image.
// If the logo file does not exist, an error is returned.
// The logo image must be a .jpg or .jpeg file.
// The logo image is resized to 100x100 pixels to ensure it is less than 1/5 of the QR code size.
func QRWithLogo(logoPath string) ([]standard.ImageOption, error) {
	file, err := os.Open(logoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open logo file: %v", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(logoPath))
	if ext != ".jpg" && ext != ".jpeg" {
		return nil, fmt.Errorf("logo image must be a .jpg or .jpeg file")
	}

	img, err := jpeg.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode logo image: %v", err)
	}

	// Resize to 100x100 to ensure it's less than 1/5 of QR code size
	resizedImg := resize.Resize(100, 100, img, resize.Lanczos3)

	options := []standard.ImageOption{
		standard.WithLogoImage(resizedImg),
	}
	return options, nil
}

// QRWithHeartShape sets the shape of the QR code modules to heart shape.
// If qrWidth is less than or equal to 0, it defaults to 10.
func QRWithHeartShape(qrWidth uint8) []standard.ImageOption {
	if qrWidth <= 0 {
		qrWidth = 10
	}

	shape := &heartShape{}
	options := []standard.ImageOption{
		standard.WithCustomShape(shape),
		standard.WithQRWidth(qrWidth),
	}

	return options
}

// QRWithEmojiShape sets the shape of the QR code modules to a laughing emoji shape.
// It also sets the background and foreground colors of the QR code.
// If either bgHex or fgHex is an empty string, an error is returned.
// If qrWidth is less than or equal to 0, it defaults to 15.
func QRWithEmojiShape(qrWidth uint8, bgHex, fgHex string) ([]standard.ImageOption, error) {
	if qrWidth <= 0 {
		qrWidth = 15
	}

	if bgHex == "" || fgHex == "" {
		return nil, errors.New("bgHex and fgHex cannot be empty")
	}

	shape := &emojiShape{}
	options := []standard.ImageOption{
		standard.WithCustomShape(shape),
		standard.WithQRWidth(qrWidth),
		standard.WithBgColorRGBHex(bgHex),
		standard.WithFgColorRGBHex(fgHex),
	}

	return options, nil
}

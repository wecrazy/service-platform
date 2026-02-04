package tests

import (
	"fmt"
	"image/color"
	"image/jpeg"
	"os"
	"service-platform/internal/config"
	"testing"

	"github.com/nfnt/resize"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
)

// go test -v -timeout 10m ./tests/playground_test.go
func TestUsingGoPlayground(t *testing.T) {
	// TODO: do whatever you want
	// REMOVE: anything you need
	config.LoadConfig()
	yamlCfg := config.WebPanel.Get()

	// // 1. Get all dates in current month (e.g. 01 Sep to end of month)
	// now := time.Now()
	// year, month := now.Year(), now.Month()
	// firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
	// lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	// daysInMonth := lastOfMonth.Day()
	// dateLabels := make([]string, 0, daysInMonth)
	// for d := 1; d <= daysInMonth; d++ {
	// 	dateLabels = append(dateLabels, fmt.Sprintf("%02d %s", d, month.String()[:3]))
	// 	fmt.Println("================================")
	// 	fmt.Println(d)
	// 	fmt.Println("================================")
	// 	fmt.Println(dateLabels[len(dateLabels)-1])
	// }

	content := yamlCfg.App.WebPublicURL

	createQRWithQRWidth(content)
	createQRWithCircleShape(content)
	createQRWithBorderWidth(content)
	createQRWithBgTransparent(content)
	createQRWithBgFgColor(content)
	createQRWithFgGradient(content)
	createQRWithHalftone(content)
	createQRWithLogo(content)
	createQRWithCustomShape(content)
	createQRWithHeartShape(content)
	createQRWithEmojiShape(content)

	fmt.Println("All QR codes generated successfully.")
}

// createQRWithQRWidth generates a QR code using the WithQRWidth option.
func createQRWithQRWidth(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	options := []standard.ImageOption{
		standard.WithQRWidth(8),
	}
	writer, err := standard.New("qrcode_with_qrwidth.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}
	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
}

// createQRWithCircleShape generates a QR code using the WithCircleShape option.
func createQRWithCircleShape(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	options := []standard.ImageOption{
		standard.WithCircleShape(),
	}
	writer, err := standard.New("qrcode_with_circleshape.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}
	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
}

// createQRWithBorderWidth generates a QR code using the WithBorderWidth option.
func createQRWithBorderWidth(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	options := []standard.ImageOption{
		standard.WithBorderWidth(10),
	}
	writer, err := standard.New("qrcode_with_borderwidth.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}
	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
}

// createQRWithBgTransparent generates a QR code using the WithBgTransparent option.
func createQRWithBgTransparent(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	options := []standard.ImageOption{
		standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
		standard.WithBgTransparent(),
	}
	writer, err := standard.New("qrcode_with_bgtransparent.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}
	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
}

// createQRWithBgColor generates a QR code using the WithBgColor option.
func createQRWithBgFgColor(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	options := []standard.ImageOption{
		standard.WithFgColor(color.RGBA{135, 206, 235, 255}), // standard.WithBgColorRGBHex(),
		standard.WithBgColor(color.RGBA{124, 252, 0, 255}),   // standard.WithFgColorRGBHex(),
	}
	writer, err := standard.New("qrcode_with_bgfgcolor.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}
	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
}

// createQRWithFgGradient generates a QR code using the WithFgGradient option.
func createQRWithFgGradient(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	stops := []standard.ColorStop{
		{Color: color.RGBA{255, 0, 0, 255}, T: 0.0},
		{Color: color.RGBA{0, 255, 0, 255}, T: 0.5},
		{Color: color.RGBA{0, 0, 255, 255}, T: 1},
	}
	// Create a linear gradient
	gradient := standard.NewGradient(45, stops...)

	options := []standard.ImageOption{
		standard.WithFgGradient(gradient),
	}
	writer, err := standard.New("qrcode_with_fggradient.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}
	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
}

// createQRWithHalftone generates a QR code using the WithHalftone option.
func createQRWithHalftone(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	// Please replace with the actual path to the halftone image.
	// halftonePath := "monna-lisa.png"
	halftonePath := "/home/user/server/service-platform/cmd/web_panel/uploads/admin/1.jpg"
	if _, err := os.Stat(halftonePath); os.IsNotExist(err) {
		fmt.Printf("halftone image file %s not found\n", halftonePath)
		return
	}

	options := []standard.ImageOption{
		standard.WithHalftone(halftonePath),
		standard.WithQRWidth(21),
	}
	writer, err := standard.New("qrcode_with_halftone.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}
	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
}

// createQRWithLogo generates a QR code using the WithLogo option.
func createQRWithLogo(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	// Load and resize the logo image
	logoPath := "/home/user/server/service-platform/cmd/web_panel/uploads/admin/1.jpg"
	file, err := os.Open(logoPath)
	if err != nil {
		fmt.Printf("open logo file failed: %v\n", err)
		return
	}
	defer file.Close()

	img, err := jpeg.Decode(file)
	if err != nil {
		fmt.Printf("decode logo image failed: %v\n", err)
		return
	}

	// Resize to 100x100 to ensure it's less than 1/5 of QR code size
	resizedImg := resize.Resize(100, 100, img, resize.Lanczos3)

	options := []standard.ImageOption{
		standard.WithLogoImage(resizedImg),
	}
	writer, err := standard.New("qrcode_with_logo.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}

	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
}

// smallerCircle implements a custom shape for QR code
type smallerCircle struct {
	smallerPercent float64
}

// DrawFinder draws the finder pattern (the three corner squares) with full size
func (sc *smallerCircle) DrawFinder(ctx *standard.DrawContext) {
	backup := sc.smallerPercent
	sc.smallerPercent = 1.0
	sc.Draw(ctx)
	sc.smallerPercent = backup
}

// Draw draws each QR code module as a smaller circle
func (sc *smallerCircle) Draw(ctx *standard.DrawContext) {
	w, h := ctx.Edge()
	x, y := ctx.UpperLeft()
	color := ctx.Color()

	// choose a proper radius values
	radius := w / 2
	r2 := h / 2
	if r2 <= radius {
		radius = r2
	}

	// Apply the smaller percentage
	radius = int(float64(radius) * sc.smallerPercent)

	cx, cy := x+float64(w)/2.0, y+float64(h)/2.0 // get center point
	ctx.DrawCircle(cx, cy, float64(radius))
	ctx.SetColor(color)
	ctx.Fill()
}

// newShape creates a new custom shape with the specified radius percentage
func newShape(radiusPercent float64) standard.IShape {
	return &smallerCircle{smallerPercent: radiusPercent}
}

func createQRWithCustomShape(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	// Create custom shape with 70% size circles
	shape := newShape(0.7)

	options := []standard.ImageOption{
		standard.WithCustomShape(shape),
	}
	writer, err := standard.New("qrcode_with_customshape.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}
	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
}

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

// createQRWithHeartShape generates a QR code with heart-shaped modules
func createQRWithHeartShape(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	shape := &heartShape{}

	options := []standard.ImageOption{
		standard.WithCustomShape(shape),
		standard.WithQRWidth(10),
	}
	writer, err := standard.New("qrcode_with_heartshape.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}
	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
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

// createQRWithEmojiShape generates a QR code with laughing emoji-shaped modules
func createQRWithEmojiShape(content string) {
	qr, err := qrcode.New(content)
	if err != nil {
		fmt.Printf("create qrcode failed: %v\n", err)
		return
	}

	shape := &emojiShape{}

	options := []standard.ImageOption{
		standard.WithCustomShape(shape),
		standard.WithQRWidth(15), // Larger width for visible emoji details
		standard.WithBgColor(color.RGBA{255, 255, 255, 255}),
		standard.WithFgColor(color.RGBA{255, 220, 0, 255}), // Yellow emoji color
	}
	writer, err := standard.New("qrcode_with_emojishape.png", options...)
	if err != nil {
		fmt.Printf("create writer failed: %v\n", err)
		return
	}
	defer writer.Close()
	if err = qr.Save(writer); err != nil {
		fmt.Printf("save qrcode failed: %v\n", err)
	}
}

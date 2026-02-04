package controllers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"service-platform/internal/config"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
	"github.com/TigorLazuardi/tanggal"
	htgotts "github.com/hegedustibor/htgo-tts"
	"github.com/hegedustibor/htgo-tts/handlers"
	"github.com/hegedustibor/htgo-tts/voices"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// getSPLCity extracts the city name from an SPL (Service Point Leader) name string.
// It handles various formats like "SPL Jakarta", "SPL.Jakarta", "2.4.SPL Jakarta", etc.
//
// Parameters:
//   - splName: The full name string of the SPL.
//
// Returns:
//   - string: The extracted city name, or empty string if not found.
func getSPLCity(splName string) string {
	if splName == "" {
		return ""
	}

	fields := strings.Fields(strings.TrimSpace(splName))
	if len(fields) == 0 {
		return ""
	}

	clean := func(s string) string {
		return strings.Trim(s, ".,:;\"'()[]{}")
	}

	for i, f := range fields {
		token := clean(f)

		// Case 1: token is exactly "SPL"
		if strings.EqualFold(token, "spl") {
			if i+1 < len(fields) {
				return clean(fields[i+1])
			}
			return ""
		}

		// Case 2: token starts with "SPL..."
		if strings.HasPrefix(strings.ToLower(token), "spl") {
			city := clean(token[3:])
			if city != "" {
				return city
			}
			if i+1 < len(fields) {
				return clean(fields[i+1])
			}
			return ""
		}

		// Case 3: token contains ".SPL" or "-SPL" inside (like "2.4.SPL")
		if idx := strings.Index(strings.ToLower(token), "spl"); idx != -1 {
			// try next token as city
			if i+1 < len(fields) {
				return clean(fields[i+1])
			}
		}
	}

	// Fallback: if first token is numeric code, second token is city
	isCode := func(s string) bool {
		if s == "" {
			return false
		}
		digitFound := false
		for _, r := range s {
			if (r >= '0' && r <= '9') || r == '.' {
				if r >= '0' && r <= '9' {
					digitFound = true
				}
				continue
			}
			return false
		}
		return digitFound
	}

	if len(fields) >= 2 && isCode(fields[0]) {
		return clean(fields[1])
	}

	return ""
}

// CreatePDFSP1ForTechnician generates a PDF file for the First Warning Letter (SP-1) for a technician.
// It uses a template and replaces placeholders with actual data.
//
// Parameters:
//   - placeholders: A map containing key-value pairs for text replacement in the PDF.
//     Keys should match the placeholders used in the function (e.g., "$nomor_surat", "$nama_teknisi").
//   - outputPath: The file path where the generated PDF will be saved.
//
// Returns:
//   - error: An error if the PDF generation fails, nil otherwise.
func CreatePDFSP1ForTechnician(placeholders map[string]string, outputPath string) error {
	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")
	imgTTDHRD := filepath.Join(imgAssetsDir, placeholders["$personalia_ttd"])
	imgTTDSAC := filepath.Join(imgAssetsDir, placeholders["$sac_ttd"])

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Peringatan 1", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("SP1, surat peringatan, teknisi, login", true)
	pdf.SetSubject("Surat Peringatan 1 - Pemberitahuan untuk Teknisi", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	pdf.AddPage()

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold

	// Draw border
	pdf.SetLineWidth(0.5)
	pdf.Rect(10, 10, 190, 277, "")

	// ====================== Start of Header Layout ======================
	leftX := 20.0
	// reservedImageWidth := 20.0
	lineHeight1 := 1.0 // first line
	lineHeightOther := 7.0
	numInfoLines := 4
	infoHeight := lineHeight1 + lineHeightOther*float64(numInfoLines-1)

	// Top Y for header
	y := pdf.GetY() + 1

	// Draw logo (height = total info block height)
	logoHeight := infoHeight * 0.8 // 40% bigger than text block height
	pdf.ImageOptions(imgCSNA, leftX, y+3, 0, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Prepare info lines
	infoLines := []struct {
		Text     string
		FontSize float64
		Bold     bool
	}{
		{config.WebPanel.Get().Default.PT, 10.5, true}, // bold
		{"Rukan Crown Blok J No. 008, Green Lake City", 7, false},
		{"Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146", 7, false},
		{"Tel.: (021) 22521101 / 5504722 / 5504723", 7, false},
	}

	// Calculate total height of all info lines (tighter spacing)
	totalInfoHeight := 0.0
	for _, l := range infoLines {
		totalInfoHeight += l.FontSize * 0.6 // reduced spacing
	}

	// Starting Y for first info line to center block vertically relative to logo
	textStartY := y + (infoHeight-totalInfoHeight)/2

	pdf.SetY(textStartY)
	pageW, _ := pdf.GetPageSize()

	for _, l := range infoLines {
		style := ""
		if l.Bold {
			style = "B"
		}
		pdf.SetFont("CenturyGothic", style, l.FontSize)

		// Calculate text width
		strW := pdf.GetStringWidth(l.Text)
		textX := (pageW - strW) / 2 // center on full page width

		pdf.SetXY(textX, pdf.GetY())
		pdf.CellFormat(strW, l.FontSize, l.Text, "", 1, "C", false, 0, "")

		// Move Y down with tighter spacing
		pdf.SetY(pdf.GetY() - l.FontSize + (l.FontSize * 0.6))
	}
	// ====================== End of Header Layout ======================

	// Form code box (top right)
	pdf.SetXY(170, 1.7)
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(30, 8, "FM-HRD.07.00.01", "1", 0, "C", false, 0, "")

	// Horizontal line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, 35, 195, 35)

	// Underline using line drawing
	pdf.SetY(55)
	pdf.SetFont("Arial", "B", 12) // Use same font to measure text width

	// Measure text width for "SURAT PERINGATAN PERTAMA (SP-1)"
	titleText := "SURAT PERINGATAN PERTAMA (SP-1)"
	titleWidth := pdf.GetStringWidth(titleText)

	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY := 40.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)                          // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY) // 2mm below text baseline

	// Nomor
	currentY += 1 // Move down another 1mm
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(0, currentY)
	nomor := "Nomor : " + placeholders["$nomor_surat"] + "/SP.I-CSNA/" + placeholders["$bulan_romawi"] + "/" + placeholders["$tahun_sp"]
	pdf.CellFormat(210, 8, nomor, "", 1, "C", false, 0, "")

	// Body
	currentY += 14 // Move down 14mm from current position
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Dibuat oleh Perusahaan, dalam hal ini ditujukan kepada:", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Nama            : "+placeholders["$nama_teknisi"], "", 1, "L", false, 0, "")
	currentY += 5
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Jabatan         : "+"Teknisi", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	textToWrite := "Sehubungan dengan sikap tidak disiplin / pelanggaran terhadap tata tertib Perusahaan yang Karyawan lakukan yaitu:"
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "B", 10)
	indent := 10.0
	maxLen := 100 // adjust as needed
	pelanggaran := placeholders["$pelanggaran_karyawan"]

	// Word-wrap pelanggaran by words, all lines aligned with indent
	words := strings.Fields(pelanggaran)
	var lines []string
	line := ""
	for _, word := range words {
		if len(line)+len(word)+1 > maxLen && line != "" {
			lines = append(lines, line)
			line = word
		} else {
			if line != "" {
				line += " "
			}
			line += word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}

	// Print all lines: first line at normal X, wrapped lines at indented X
	for i, line := range lines {
		if i == 0 {
			pdf.SetXY(15+indent, currentY)
		} else {
			pdf.SetXY(15, currentY)
		}
		pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
		currentY = pdf.GetY()
	}

	currentY += 3
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite = "Atas perbuatan pelanggaran Peraturan Perusahaan yang dilakukan oleh $nama_teknisi, maka dengan ini Perusahaan memberikan Surat Peringatan Pertama (SP-1) kepada Karyawan, agar Karyawan dapat melakukan introspeksi dan memperbaiki diri sehingga Karyawan tidak lagi melakukan pelanggaran atas Peraturan Perusahaan dalam bentuk apapun. SP-1 ini diberikan kepada Karyawan dengan ketentuan sebagai berikut :"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// Numbered list, indented and wrapped
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "", 10)
	listItems := []string{
		"Surat Peringatan Pertama berlaku untuk 1 (satu) hari kedepan sejak diterbitkan.",
		"Apabila dalam kurun waktu 1 (satu) hari kedepan sejak tanggal diterbitkan Surat Peringatan Pertama Karyawan tidak melakukan tindak pelanggaran yang menjadi dasar atas diterbitkannya surat peringatan pertama ini, maka Surat Peringatan Pertama Karyawan dinyatakan sudah tidak berlaku.",
		"Jika dalam kurun waktu 1 (satu) hari kedepan sejak Surat Peringatan Pertama diterbitkan Karyawan didapati kembali melakukan tindakan pelanggaran, maka perusahaan akan memberikan surat peringatan ke-2 untuk Karyawan.",
	}
	indentList := 10.0
	maxLen = 98 // match pelanggaran wrapping
	for i, item := range listItems {
		// Word wrap by splitting into words
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxLen && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
		// Print first line with number and indent
		pdf.SetXY(15+indentList, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()
		// Print remaining lines, aligned with text (number skipped, same margin)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentList, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
			currentY = pdf.GetY()
		}
	}

	// Final paragraph: 'Demikian Surat Peringatan ...' all on one line, correct bold
	currentY += 2
	pdf.SetXY(15, currentY)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(pdf.GetStringWidth("Demikian "), 5, "Demikian ", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(pdf.GetStringWidth("Surat Peringatan"), 5, "Surat Peringatan", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, " ini dibuat agar dapat diperhatikan dan ditaati sebaik mungkin oleh yang bersangkutan.", "", 1, "L", false, 0, "")

	// =================================== Signatures ===================================
	currentY += 10
	leftX = 35.0
	rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_sp_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "Diterbitkan,", "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, currentY)
	pdf.CellFormat(80, 5, "Mengetahui,", "", 0, "L", false, 0, "")

	currentY += 20 // space for signatures

	// --- Left signature: Personalia ---
	ttdHRDWidth := 20.0
	pdf.ImageOptions(imgTTDHRD, leftX, currentY-15, ttdHRDWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center Personalia name below "Diterbitkan,"
	labelDiterbitkan := "Diterbitkan,"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$personalia_name"])
	padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	// Personalia Name (bold, underline)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$personalia_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)

	// Role text ("Personalia"), centered under the name
	roleWidth := pdf.GetStringWidth("Personalia")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Personalia", "", 0, "L", false, 0, "")

	// --- Right signature: SAC ---
	ttdSacWidth := 18.0
	rightXForTTD := rightX + 3.0
	switch placeholders["$sac_ttd"] {
	case "ttd_angga.png":
		ttdSacWidth = 30.0 // Budi's signature is wider
		rightXForTTD = rightX - 4.0
	case "ttd_tomi.png":
		rightXForTTD = rightX - 3.0
		ttdSacWidth = 25.0 // Tomi's signature is wider
	case "ttd_burhan.png":
		rightXForTTD = rightX + 5.0
		ttdSacWidth = 11.0 // Burhan's signature
	}

	pdf.ImageOptions(imgTTDSAC, rightXForTTD, currentY-15, ttdSacWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center SAC name below "Mengetahui,"
	labelMengetahui := "Mengetahui,"
	labelWidthR := pdf.GetStringWidth(labelMengetahui)
	mgrWidth := pdf.GetStringWidth(placeholders["$sac_name"])

	// Compute X so the SAC name is centered under the label
	centerXR := rightX + (labelWidthR / 2) - (mgrWidth / 2)

	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerXR, currentY)
	pdf.CellFormat(mgrWidth, 5, placeholders["$sac_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerXR, currentY+5, centerXR+mgrWidth+padding, currentY+5)

	// Role text ("Service Area Coordinator"), centered
	roleR := "Service Area Coordinator"
	roleRWidth := pdf.GetStringWidth(roleR)
	roleRX := rightX + (labelWidthR / 2) - (roleRWidth / 2)

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleRX+2, currentY+5)
	pdf.CellFormat(roleRWidth, 5, roleR, "", 0, "L", false, 0, "")
	// ==================================================================================

	return pdf.OutputFileAndClose(outputPath)
}

// CreatePDFSP2ForTechnician generates a PDF file for the Second Warning Letter (SP-2) for a technician.
// It uses a template and replaces placeholders with actual data.
//
// Parameters:
//   - placeholders: A map containing key-value pairs for text replacement in the PDF.
//     Keys should match the placeholders used in the function (e.g., "$nomor_surat", "$nama_teknisi").
//   - outputPath: The file path where the generated PDF will be saved.
//
// Returns:
//   - error: An error if the PDF generation fails, nil otherwise.
func CreatePDFSP2ForTechnician(placeholders map[string]string, outputPath string) error {
	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")
	imgTTDHRD := filepath.Join(imgAssetsDir, placeholders["$personalia_ttd"])
	imgTTDSAC := filepath.Join(imgAssetsDir, placeholders["$sac_ttd"])

	dbWeb := gormdb.Databases.Web
	var dataSP sptechnicianmodel.TechnicianGotSP
	err = dbWeb.
		Where("for_project = ?", placeholders["$for_project"]).
		Where("technician = ?", placeholders["$record_technician"]).
		Model(&sptechnicianmodel.TechnicianGotSP{}).
		First(&dataSP).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to query TechnicianGotSP: %v", err)
	}

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Peringatan 2", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("SP2, surat peringatan, teknisi, login", true)
	pdf.SetSubject("Surat Peringatan 2 - Pemberitahuan untuk Teknisi", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	pdf.AddPage()

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold

	// Draw border
	pdf.SetLineWidth(0.5)
	pdf.Rect(10, 10, 190, 277, "")

	// ====================== Start of Header Layout ======================
	leftX := 20.0
	// reservedImageWidth := 20.0
	lineHeight1 := 1.0 // first line
	lineHeightOther := 7.0
	numInfoLines := 4
	infoHeight := lineHeight1 + lineHeightOther*float64(numInfoLines-1)

	// Top Y for header
	y := pdf.GetY() + 1

	// Draw logo (height = total info block height)
	logoHeight := infoHeight * 0.8 // 40% bigger than text block height
	pdf.ImageOptions(imgCSNA, leftX, y+3, 0, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Prepare info lines
	infoLines := []struct {
		Text     string
		FontSize float64
		Bold     bool
	}{
		{config.WebPanel.Get().Default.PT, 10.5, true}, // bold
		{"Rukan Crown Blok J No. 008, Green Lake City", 7, false},
		{"Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146", 7, false},
		{"Tel.: (021) 22521101 / 5504722 / 5504723", 7, false},
	}

	// Calculate total height of all info lines (tighter spacing)
	totalInfoHeight := 0.0
	for _, l := range infoLines {
		totalInfoHeight += l.FontSize * 0.6 // reduced spacing
	}

	// Starting Y for first info line to center block vertically relative to logo
	textStartY := y + (infoHeight-totalInfoHeight)/2

	pdf.SetY(textStartY)
	pageW, _ := pdf.GetPageSize()

	for _, l := range infoLines {
		style := ""
		if l.Bold {
			style = "B"
		}
		pdf.SetFont("CenturyGothic", style, l.FontSize)

		// Calculate text width
		strW := pdf.GetStringWidth(l.Text)
		textX := (pageW - strW) / 2 // center on full page width

		pdf.SetXY(textX, pdf.GetY())
		pdf.CellFormat(strW, l.FontSize, l.Text, "", 1, "C", false, 0, "")

		// Move Y down with tighter spacing
		pdf.SetY(pdf.GetY() - l.FontSize + (l.FontSize * 0.6))
	}
	// ====================== End of Header Layout ======================

	// Form code box (top right)
	pdf.SetXY(170, 1.7)
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(30, 8, "FM-HRD.07.00.01", "1", 0, "C", false, 0, "")

	// Horizontal line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, 35, 195, 35)

	// Underline using line drawing
	pdf.SetY(55)
	pdf.SetFont("Arial", "B", 12) // Use same font to measure text width

	// Measure text width for "SURAT PERINGATAN KEDUA (SP-2)"
	titleText := "SURAT PERINGATAN KEDUA (SP-2)"
	titleWidth := pdf.GetStringWidth(titleText)

	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY := 40.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)                          // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY) // 2mm below text baseline

	// Nomor
	currentY += 1 // Move down another 1mm
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(0, currentY)
	nomor := "Nomor : " + placeholders["$nomor_surat"] + "/SP.II-CSNA/" + placeholders["$bulan_romawi"] + "/" + placeholders["$tahun_sp"]
	pdf.CellFormat(210, 8, nomor, "", 1, "C", false, 0, "")

	// Body
	currentY += 14 // Move down 13mm from current position
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Dibuat oleh Perusahaan, dalam hal ini ditujukan kepada:", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Nama            : "+placeholders["$nama_teknisi"], "", 1, "L", false, 0, "")
	currentY += 5
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Jabatan         : "+"Teknisi", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite := "Sehubungan dengan Surat Peringatan Pertama (SP-1) yang sebelumnya disampaikan kepada Sdr. $nama_teknisi, perusahaan kemudian memutuskan untuk menindaklanjuti melalui Surat Peringatan Kedua (SP-2). Hal ini didasari Sdr. $nama_teknisi yang tidak menunjukkan sikap disiplin/pelanggaran terhadap Tata Tertib Perusahaan yang Sdr. $nama_teknisi lakukan yaitu:"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// List Pelanggaran
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "B", 10)
	listPelanggaran := []string{}
	if dataSP.PelanggaranSP1 != "" {
		listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP1)
	}
	if placeholders["$pelanggaran_karyawan"] != "" {
		listPelanggaran = append(listPelanggaran, placeholders["$pelanggaran_karyawan"])
	}

	indentPelanggaran := 10.0
	maxLenPelanggaran := 98 // adjust as needed

	for i, item := range listPelanggaran {
		// Word wrap by splitting into words
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxLenPelanggaran && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}

		if len(lines) == 0 {
			continue // skip empty pelanggaran
		}

		// Print first line with number and indent
		pdf.SetXY(15+indentPelanggaran, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()

		// Print remaining lines, aligned with text (number skipped, same margin)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentPelanggaran, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "") // <--- ln=1 (move to new line)
			currentY = pdf.GetY()
		}
	}

	currentY += 3
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite = "Atas perbuatan pelanggaran Tata Tertib Perusahaan yang dilakukan oleh Sdr. $nama_teknisi, maka dengan ini Perusahaan memberikan Surat Peringatan Kedua (SP-2) kepada Sdr. $nama_teknisi agar Karyawan dapat melakukan introspeksi dan memperbaiki diri sehingga Karyawan tidak lagi melakukan pelanggaran atas Tata Tertib Perusahaan dalam bentuk apapun. SP-2 ini diberikan kepada Karyawan dengan ketentuan sebagai berikut :"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// Numbered list, indented and wrapped
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "", 10)
	listItems := []string{
		"Surat Peringatan Kedua berlaku untuk 1 (satu) hari kedepan sejak diterbitkan.",
		"Apabila dalam kurun waktu 1 (satu) hari kedepan sejak tanggal diterbitkan Surat Peringatan Kedua Karyawan tidak melakukan tindak pelanggaran yang menjadi dasar atas diterbitkannya surat Peringatan Kedua ini, maka Surat Peringatan Kedua Karyawan dinyatakan sudah tidak berlaku.",
		"Jika dalam kurun waktu 1 (satu) hari kedepan sejak Surat Peringatan Kedua diterbitkan Karyawan didapati kembali melakukan tindakan pelanggaran, maka perusahaan akan memberikan Surat Peringatan Ketiga (SP-3) atau Pemutusan Hubungan Kerja.",
	}
	indentList := 10.0
	maxLen := 98 // match pelanggaran wrapping
	for i, item := range listItems {
		// Word wrap by splitting into words
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxLen && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
		// Print first line with number and indent
		pdf.SetXY(15+indentList, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()
		// Print remaining lines, aligned with text (number skipped, same margin)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentList, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
			currentY = pdf.GetY()
		}
	}

	// Final paragraph: 'Demikian Surat Peringatan ...' all on one line, correct bold
	currentY += 2
	pdf.SetXY(15, currentY)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(pdf.GetStringWidth("Demikian "), 5, "Demikian ", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(pdf.GetStringWidth("Surat Peringatan"), 5, "Surat Peringatan", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, " ini dibuat agar dapat diperhatikan dan ditaati sebaik mungkin oleh yang bersangkutan.", "", 1, "L", false, 0, "")

	// =================================== Signatures ===================================
	currentY += 10
	leftX = 35.0
	rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_sp_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "Diterbitkan,", "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, currentY)
	pdf.CellFormat(80, 5, "Mengetahui,", "", 0, "L", false, 0, "")

	currentY += 20 // space for signatures

	// --- Left signature: Personalia ---
	ttdHRDWidth := 20.0
	pdf.ImageOptions(imgTTDHRD, leftX, currentY-15, ttdHRDWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center Personalia name below "Diterbitkan,"
	labelDiterbitkan := "Diterbitkan,"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$personalia_name"])
	padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	// Personalia Name (bold, underline)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$personalia_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)

	// Role text ("Personalia"), centered under the name
	roleWidth := pdf.GetStringWidth("Personalia")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Personalia", "", 0, "L", false, 0, "")

	// --- Right signature: SAC ---
	ttdSacWidth := 18.0
	rightXForTTD := rightX + 3.0
	switch placeholders["$sac_ttd"] {
	case "ttd_angga.png":
		ttdSacWidth = 30.0 // Budi's signature is wider
		rightXForTTD = rightX - 4.0
	case "ttd_tomi.png":
		rightXForTTD = rightX - 3.0
		ttdSacWidth = 25.0 // Tomi's signature is wider
	case "ttd_burhan.png":
		rightXForTTD = rightX + 5.0
		ttdSacWidth = 11.0 // Burhan's signature
	}

	pdf.ImageOptions(imgTTDSAC, rightXForTTD, currentY-15, ttdSacWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center SAC name below "Mengetahui,"
	labelMengetahui := "Mengetahui,"
	labelWidthR := pdf.GetStringWidth(labelMengetahui)
	mgrWidth := pdf.GetStringWidth(placeholders["$sac_name"])

	// Compute X so the SAC name is centered under the label
	centerXR := rightX + (labelWidthR / 2) - (mgrWidth / 2)

	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerXR, currentY)
	pdf.CellFormat(mgrWidth, 5, placeholders["$sac_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerXR, currentY+5, centerXR+mgrWidth+padding, currentY+5)

	// Role text ("Service Area Coordinator"), centered
	roleR := "Service Area Coordinator"
	roleRWidth := pdf.GetStringWidth(roleR)
	roleRX := rightX + (labelWidthR / 2) - (roleRWidth / 2)

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleRX+2, currentY+5)
	pdf.CellFormat(roleRWidth, 5, roleR, "", 0, "L", false, 0, "")
	// ==================================================================================

	return pdf.OutputFileAndClose(outputPath)
}

// CreatePDFSP3ForTechnician generates a PDF file for the Third Warning Letter (SP-3) for a technician.
// It uses a template and replaces placeholders with actual data.
//
// Parameters:
//   - placeholders: A map containing key-value pairs for text replacement in the PDF.
//     Keys should match the placeholders used in the function (e.g., "$nomor_surat", "$nama_teknisi").
//   - outputPath: The file path where the generated PDF will be saved.
//
// Returns:
//   - error: An error if the PDF generation fails, nil otherwise.
func CreatePDFSP3ForTechnician(placeholders map[string]string, outputPath string) error {
	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")
	imgTTDHRD := filepath.Join(imgAssetsDir, placeholders["$personalia_ttd"])
	imgTTDSAC := filepath.Join(imgAssetsDir, placeholders["$sac_ttd"])

	dbWeb := gormdb.Databases.Web
	var dataSP sptechnicianmodel.TechnicianGotSP
	err = dbWeb.
		Where("for_project = ?", placeholders["$for_project"]).
		Where("technician = ?", placeholders["$record_technician"]).
		Model(&sptechnicianmodel.TechnicianGotSP{}).
		First(&dataSP).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to query TechnicianGotSP: %v", err)
	}

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Peringatan 3", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("SP3, surat peringatan, teknisi, login", true)
	pdf.SetSubject("Surat Peringatan 3 - Pemberitahuan untuk Teknisi", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	pdf.AddPage()

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold

	// Draw border
	pdf.SetLineWidth(0.5)
	pdf.Rect(10, 10, 190, 277, "")

	// ====================== Start of Header Layout ======================
	leftX := 20.0
	// reservedImageWidth := 20.0
	lineHeight1 := 1.0 // first line
	lineHeightOther := 7.0
	numInfoLines := 4
	infoHeight := lineHeight1 + lineHeightOther*float64(numInfoLines-1)

	// Top Y for header
	y := pdf.GetY() + 1

	// Draw logo (height = total info block height)
	logoHeight := infoHeight * 0.8 // 40% bigger than text block height
	pdf.ImageOptions(imgCSNA, leftX, y+3, 0, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Prepare info lines
	infoLines := []struct {
		Text     string
		FontSize float64
		Bold     bool
	}{
		{config.WebPanel.Get().Default.PT, 10.5, true}, // bold
		{"Rukan Crown Blok J No. 008, Green Lake City", 7, false},
		{"Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146", 7, false},
		{"Tel.: (021) 22521101 / 5504722 / 5504723", 7, false},
	}

	// Calculate total height of all info lines (tighter spacing)
	totalInfoHeight := 0.0
	for _, l := range infoLines {
		totalInfoHeight += l.FontSize * 0.6 // reduced spacing
	}

	// Starting Y for first info line to center block vertically relative to logo
	textStartY := y + (infoHeight-totalInfoHeight)/2

	pdf.SetY(textStartY)
	pageW, _ := pdf.GetPageSize()

	for _, l := range infoLines {
		style := ""
		if l.Bold {
			style = "B"
		}
		pdf.SetFont("CenturyGothic", style, l.FontSize)

		// Calculate text width
		strW := pdf.GetStringWidth(l.Text)
		textX := (pageW - strW) / 2 // center on full page width

		pdf.SetXY(textX, pdf.GetY())
		pdf.CellFormat(strW, l.FontSize, l.Text, "", 1, "C", false, 0, "")

		// Move Y down with tighter spacing
		pdf.SetY(pdf.GetY() - l.FontSize + (l.FontSize * 0.6))
	}
	// ====================== End of Header Layout ======================

	// Form code box (top right)
	pdf.SetXY(170, 1.7)
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(30, 8, "FM-HRD.07.00.01", "1", 0, "C", false, 0, "")

	// Horizontal line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, 35, 195, 35)

	// Underline using line drawing
	pdf.SetY(55)
	pdf.SetFont("Arial", "B", 12) // Use same font to measure text width

	// Measure text width for "SURAT PERINGATAN KETIGA (SP-3)"
	titleText := "SURAT PERINGATAN KETIGA (SP-3)"
	titleWidth := pdf.GetStringWidth(titleText)

	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY := 40.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)                          // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY) // 2mm below text baseline

	// Nomor
	currentY += 1 // Move down another 1mm
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(0, currentY)
	nomor := "Nomor : " + placeholders["$nomor_surat"] + "/SP.III-CSNA/" + placeholders["$bulan_romawi"] + "/" + placeholders["$tahun_sp"]
	pdf.CellFormat(210, 8, nomor, "", 1, "C", false, 0, "")

	// Body
	currentY += 14 // Move down 13mm from current position
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Dibuat oleh Perusahaan, dalam hal ini ditujukan kepada:", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Nama            : "+placeholders["$nama_teknisi"], "", 1, "L", false, 0, "")
	currentY += 5
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Jabatan         : "+"Teknisi", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	textToWrite := "Sehubungan dengan telah dikeluarkannya  Surat Peringatan Pertama (SP-1) dan Surat Peringatan Kedua (SP-2)  yang telah sebelumnya diberikan kepada Sdr. $nama_teknisi, namun yang bersangkutan tetap tidak menunjukkan perubahan sikap dan perbaikan diri, serta masih melakukan pelanggaran terhadap Tata Tertib Perusahaan, maka dengan ini Perusahaan menyampaikan Surat Peringatan Ketiga (SP-3). Adapun pelanggaran yang dilakukan oleh Sdr. $nama_teknisi adalah sebagai berikut :"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// List Pelanggaran
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "B", 10)
	listPelanggaran := []string{}
	if dataSP.PelanggaranSP1 != "" {
		listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP1)
	}
	if dataSP.PelanggaranSP2 != "" {
		listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP2)
	}
	if placeholders["$pelanggaran_karyawan"] != "" {
		listPelanggaran = append(listPelanggaran, placeholders["$pelanggaran_karyawan"])
	}

	indentPelanggaran := 10.0
	maxLenPelanggaran := 98 // adjust as needed

	for i, item := range listPelanggaran {
		// Word wrap by splitting into words
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxLenPelanggaran && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}

		if len(lines) == 0 {
			continue // skip empty pelanggaran
		}

		// Print first line with number and indent
		pdf.SetXY(15+indentPelanggaran, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()

		// Print remaining lines, aligned with text (number skipped, same margin)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentPelanggaran, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "") // <--- ln=1 (move to new line)
			currentY = pdf.GetY()
		}
	}

	currentY += 3
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite = "Surat Peringatan Ketiga (SP-3) ini merupakan peringatan terakhir yang sekaligus menjadi dasar bagi Perusahaan terhadap Sdr. $nama_teknisi untuk meminta HRD melakukan tindak-lanjut sesuai peraturan  karena telah berulang kali melakukan pelanggaran yang merugikan Perusahaan."
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	currentY = pdf.GetY() + 2

	// Final paragraph with inline bold (Demikian + Surat Peringatan + rest)
	currentY += 2
	marginLeft := 15.0
	pdf.SetXY(marginLeft, currentY)

	// Define parts
	part1 := "Demikian "
	part2 := "Surat Peringatan"
	part3 := " terakhir ini disampaikan agar selanjutnya dapat menghubungi pihak HRD untuk melakukan klarifikasi  lebih lanjut. Jika dalam jangka waktu 2 (dua) hari kerja dari SP-3 diterbitkan, Sdr. $nama_teknisi tidak melakukan sanggahan, maka dianggap Sdr. $nama_teknisi Menyetujui penerbitan SP-3 ini dan Perusahaan berhak menerbitkan Surat Pemutusan Hubugan Kerja (S-PHK)."
	part3 = strings.ReplaceAll(part3, "$nama_teknisi", placeholders["$nama_teknisi"])

	// Build the full sentence (for wrapping width calculation)
	fullSentence := part1 + part2 + part3

	// Define maximum width (page width minus margins)
	marginRight := 15.0
	maxWidth := pageWidth - marginLeft - marginRight

	// Split into words for wrapping
	words := strings.Split(fullSentence, " ")
	line := ""
	for _, w := range words {
		testLine := strings.TrimSpace(line + " " + w)
		width := pdf.GetStringWidth(testLine)

		if width > maxWidth {
			// Print current line
			pdf.SetX(marginLeft) // <-- ensure every new line starts at same left margin
			pdf.SetFont("Arial", "", 10)

			if strings.Contains(line, part2) {
				before := strings.Split(line, part2)[0]
				after := strings.Split(line, part2)[1]

				// Print before
				pdf.CellFormat(pdf.GetStringWidth(before), 5, before, "", 0, "", false, 0, "")
				// Print bold
				pdf.SetFont("Arial", "B", 10)
				pdf.CellFormat(pdf.GetStringWidth(part2), 5, part2, "", 0, "", false, 0, "")
				// Back to normal
				pdf.SetFont("Arial", "", 10)
				pdf.CellFormat(pdf.GetStringWidth(after), 5, after, "", 0, "", false, 0, "")
			} else {
				pdf.CellFormat(pdf.GetStringWidth(line), 5, line, "", 0, "", false, 0, "")
			}

			pdf.Ln(5)
			line = w
		} else {
			line = testLine
		}
	}

	// Print the last line
	if line != "" {
		pdf.SetX(marginLeft) // <-- align last line too
		pdf.SetFont("Arial", "", 10)

		if strings.Contains(line, part2) {
			before := strings.Split(line, part2)[0]
			after := strings.Split(line, part2)[1]

			pdf.CellFormat(pdf.GetStringWidth(before), 5, before, "", 0, "", false, 0, "")
			pdf.SetFont("Arial", "B", 10)
			pdf.CellFormat(pdf.GetStringWidth(part2), 5, part2, "", 0, "", false, 0, "")
			pdf.SetFont("Arial", "", 10)
			pdf.CellFormat(pdf.GetStringWidth(after), 5, after, "", 0, "", false, 0, "")
		} else {
			pdf.CellFormat(pdf.GetStringWidth(line), 5, line, "", 0, "", false, 0, "")
		}
	}

	// =================================== Signatures ===================================
	currentY = pdf.GetY() + 2
	currentY += 10
	leftX = 35.0
	rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_sp_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "Diterbitkan,", "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, currentY)
	pdf.CellFormat(80, 5, "Mengetahui,", "", 0, "L", false, 0, "")

	currentY += 20 // space for signatures

	// --- Left signature: Personalia ---
	ttdHRDWidth := 20.0
	pdf.ImageOptions(imgTTDHRD, leftX, currentY-15, ttdHRDWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center Personalia name below "Diterbitkan,"
	labelDiterbitkan := "Diterbitkan,"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$personalia_name"])
	padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	// Personalia Name (bold, underline)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$personalia_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)

	// Role text ("Personalia"), centered under the name
	roleWidth := pdf.GetStringWidth("Personalia")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Personalia", "", 0, "L", false, 0, "")

	// --- Right signature: SAC ---
	ttdSacWidth := 18.0
	rightXForTTD := rightX + 3.0
	switch placeholders["$sac_ttd"] {
	case "ttd_angga.png":
		ttdSacWidth = 30.0 // Budi's signature is wider
		rightXForTTD = rightX - 4.0
	case "ttd_tomi.png":
		rightXForTTD = rightX - 3.0
		ttdSacWidth = 25.0 // Tomi's signature is wider
	case "ttd_burhan.png":
		rightXForTTD = rightX + 5.0
		ttdSacWidth = 11.0 // Burhan's signature
	}

	pdf.ImageOptions(imgTTDSAC, rightXForTTD, currentY-15, ttdSacWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center SAC name below "Mengetahui,"
	labelMengetahui := "Mengetahui,"
	labelWidthR := pdf.GetStringWidth(labelMengetahui)
	mgrWidth := pdf.GetStringWidth(placeholders["$sac_name"])

	// Compute X so the SAC name is centered under the label
	centerXR := rightX + (labelWidthR / 2) - (mgrWidth / 2)

	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerXR, currentY)
	pdf.CellFormat(mgrWidth, 5, placeholders["$sac_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerXR, currentY+5, centerXR+mgrWidth+padding, currentY+5)

	// Role text ("Service Area Coordinator"), centered
	roleR := "Service Area Coordinator"
	roleRWidth := pdf.GetStringWidth(roleR)
	roleRX := rightX + (labelWidthR / 2) - (roleRWidth / 2)

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleRX+2, currentY+5)
	pdf.CellFormat(roleRWidth, 5, roleR, "", 0, "L", false, 0, "")
	// ==================================================================================

	return pdf.OutputFileAndClose(outputPath)
}

// CreatePDFSP1ForSPL generates a PDF file for the First Warning Letter (SP-1) for a Service Point Leader (SPL).
// It uses a template and replaces placeholders with actual data.
//
// Parameters:
//   - placeholders: A map containing key-value pairs for text replacement in the PDF.
//     Keys should match the placeholders used in the function (e.g., "$nomor_surat", "$nama_spl").
//   - outputPath: The file path where the generated PDF will be saved.
//
// Returns:
//   - error: An error if the PDF generation fails, nil otherwise.
func CreatePDFSP1ForSPL(placeholders map[string]string, outputPath string) error {
	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")
	imgTTDHRD := filepath.Join(imgAssetsDir, placeholders["$personalia_ttd"])
	imgTTDSAC := filepath.Join(imgAssetsDir, placeholders["$sac_ttd"])

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Peringatan 1", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("SP1, surat peringatan, spl, pelanggaran", true)
	pdf.SetSubject("Surat Peringatan 1 - Pemberitahuan untuk SPL", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	pdf.AddPage()

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold

	// Draw border
	pdf.SetLineWidth(0.5)
	pdf.Rect(10, 10, 190, 277, "")

	// ====================== Start of Header Layout ======================
	leftX := 20.0
	// reservedImageWidth := 20.0
	lineHeight1 := 1.0 // first line
	lineHeightOther := 7.0
	numInfoLines := 4
	infoHeight := lineHeight1 + lineHeightOther*float64(numInfoLines-1)

	// Top Y for header
	y := pdf.GetY() + 1

	// Draw logo (height = total info block height)
	logoHeight := infoHeight * 0.8 // 40% bigger than text block height
	pdf.ImageOptions(imgCSNA, leftX, y+3, 0, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Prepare info lines
	infoLines := []struct {
		Text     string
		FontSize float64
		Bold     bool
	}{
		{config.WebPanel.Get().Default.PT, 10.5, true}, // bold
		{"Rukan Crown Blok J No. 008, Green Lake City", 7, false},
		{"Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146", 7, false},
		{"Tel.: (021) 22521101 / 5504722 / 5504723", 7, false},
	}

	// Calculate total height of all info lines (tighter spacing)
	totalInfoHeight := 0.0
	for _, l := range infoLines {
		totalInfoHeight += l.FontSize * 0.6 // reduced spacing
	}

	// Starting Y for first info line to center block vertically relative to logo
	textStartY := y + (infoHeight-totalInfoHeight)/2

	pdf.SetY(textStartY)
	pageW, _ := pdf.GetPageSize()

	for _, l := range infoLines {
		style := ""
		if l.Bold {
			style = "B"
		}
		pdf.SetFont("CenturyGothic", style, l.FontSize)

		// Calculate text width
		strW := pdf.GetStringWidth(l.Text)
		textX := (pageW - strW) / 2 // center on full page width

		pdf.SetXY(textX, pdf.GetY())
		pdf.CellFormat(strW, l.FontSize, l.Text, "", 1, "C", false, 0, "")

		// Move Y down with tighter spacing
		pdf.SetY(pdf.GetY() - l.FontSize + (l.FontSize * 0.6))
	}
	// ====================== End of Header Layout ======================

	// Form code box (top right)
	pdf.SetXY(170, 1.7)
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(30, 8, "FM-HRD.07.00.01", "1", 0, "C", false, 0, "")

	// Horizontal line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, 35, 195, 35)

	// Underline using line drawing
	pdf.SetY(55)
	pdf.SetFont("Arial", "B", 12) // Use same font to measure text width

	// Measure text width for "SURAT PERINGATAN PERTAMA (SP-1)"
	titleText := "SURAT PERINGATAN PERTAMA (SP-1)"
	titleWidth := pdf.GetStringWidth(titleText)

	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY := 40.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)                          // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY) // 2mm below text baseline

	// Nomor
	currentY += 1 // Move down another 1mm
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(0, currentY)
	nomor := "Nomor : " + placeholders["$nomor_surat"] + "/SP.I-CSNA/" + placeholders["$bulan_romawi"] + "/" + placeholders["$tahun_sp"]
	pdf.CellFormat(210, 8, nomor, "", 1, "C", false, 0, "")

	// Body
	currentY += 14 // Move down 14mm from current position
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Dibuat oleh Perusahaan, dalam hal ini ditujukan kepada:", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Nama            : "+placeholders["$nama_spl"], "", 1, "L", false, 0, "")
	currentY += 5
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Jabatan         : "+placeholders["$jabatan_spl"], "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	textToWrite := "Sehubungan dengan sikap tidak disiplin / pelanggaran terhadap tata tertib Perusahaan yang Karyawan lakukan yaitu:"
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "B", 10)
	indent := 10.0
	maxLen := 100 // adjust as needed
	pelanggaran := placeholders["$pelanggaran_karyawan"]

	// Word-wrap pelanggaran by words, all lines aligned with indent
	words := strings.Fields(pelanggaran)
	var lines []string
	line := ""
	for _, word := range words {
		if len(line)+len(word)+1 > maxLen && line != "" {
			lines = append(lines, line)
			line = word
		} else {
			if line != "" {
				line += " "
			}
			line += word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}

	// Print all lines: first line at normal X, wrapped lines at indented X
	for i, line := range lines {
		if i == 0 {
			pdf.SetXY(15+indent, currentY)
		} else {
			pdf.SetXY(15, currentY)
		}
		pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
		currentY = pdf.GetY()
	}

	currentY += 3
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite = "Atas perbuatan pelanggaran Peraturan Perusahaan yang dilakukan oleh $nama_spl, maka dengan ini Perusahaan memberikan Surat Peringatan Pertama (SP-1) kepada Karyawan, agar Karyawan dapat melakukan introspeksi dan memperbaiki diri sehingga Karyawan tidak lagi melakukan pelanggaran atas Peraturan Perusahaan dalam bentuk apapun. SP-1 ini diberikan kepada Karyawan dengan ketentuan sebagai berikut :"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_spl", placeholders["$nama_spl"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// Numbered list, indented and wrapped
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "", 10)
	listItems := []string{
		"Surat Peringatan Pertama berlaku untuk 1 (satu) hari kedepan sejak diterbitkan.",
		"Apabila dalam kurun waktu 1 (satu) hari kedepan sejak tanggal diterbitkan Surat Peringatan Pertama Karyawan tidak melakukan tindak pelanggaran yang menjadi dasar atas diterbitkannya surat peringatan pertama ini, maka Surat Peringatan Pertama Karyawan dinyatakan sudah tidak berlaku.",
		"Jika dalam kurun waktu 1 (satu) hari kedepan sejak Surat Peringatan Pertama diterbitkan Karyawan didapati kembali melakukan tindakan pelanggaran, maka perusahaan akan memberikan surat peringatan ke-2 untuk Karyawan.",
	}
	indentList := 10.0
	maxLen = 98 // match pelanggaran wrapping
	for i, item := range listItems {
		// Word wrap by splitting into words
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxLen && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
		// Print first line with number and indent
		pdf.SetXY(15+indentList, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()
		// Print remaining lines, aligned with text (number skipped, same margin)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentList, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
			currentY = pdf.GetY()
		}
	}

	// Final paragraph: 'Demikian Surat Peringatan ...' all on one line, correct bold
	currentY += 2
	pdf.SetXY(15, currentY)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(pdf.GetStringWidth("Demikian "), 5, "Demikian ", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(pdf.GetStringWidth("Surat Peringatan"), 5, "Surat Peringatan", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, " ini dibuat agar dapat diperhatikan dan ditaati sebaik mungkin oleh yang bersangkutan.", "", 1, "L", false, 0, "")

	// =================================== Signatures ===================================
	currentY += 10
	leftX = 35.0
	rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_sp_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "Diterbitkan,", "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, currentY)
	pdf.CellFormat(80, 5, "Mengetahui,", "", 0, "L", false, 0, "")

	currentY += 20 // space for signatures

	// --- Left signature: Personalia ---
	ttdHRDWidth := 20.0
	pdf.ImageOptions(imgTTDHRD, leftX, currentY-15, ttdHRDWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center Personalia name below "Diterbitkan,"
	labelDiterbitkan := "Diterbitkan,"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$personalia_name"])
	padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	// Personalia Name (bold, underline)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$personalia_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)

	// Role text ("Personalia"), centered under the name
	roleWidth := pdf.GetStringWidth("Personalia")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Personalia", "", 0, "L", false, 0, "")

	// --- Right signature: SAC ---
	ttdSacWidth := 18.0
	rightXForTTD := rightX + 3.0
	switch placeholders["$sac_ttd"] {
	case "ttd_angga.png":
		ttdSacWidth = 30.0 // Budi's signature is wider
		rightXForTTD = rightX - 4.0
	case "ttd_tomi.png":
		rightXForTTD = rightX - 3.0
		ttdSacWidth = 25.0 // Tomi's signature is wider
	case "ttd_burhan.png":
		rightXForTTD = rightX + 5.0
		ttdSacWidth = 11.0 // Burhan's signature
	}

	pdf.ImageOptions(imgTTDSAC, rightXForTTD, currentY-15, ttdSacWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center SAC name below "Mengetahui,"
	labelMengetahui := "Mengetahui,"
	labelWidthR := pdf.GetStringWidth(labelMengetahui)
	mgrWidth := pdf.GetStringWidth(placeholders["$sac_name"])

	// Compute X so the SAC name is centered under the label
	centerXR := rightX + (labelWidthR / 2) - (mgrWidth / 2)

	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerXR, currentY)
	pdf.CellFormat(mgrWidth, 5, placeholders["$sac_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerXR, currentY+5, centerXR+mgrWidth+padding, currentY+5)

	// Role text ("Service Area Coordinator"), centered
	roleR := "Service Area Coordinator"
	roleRWidth := pdf.GetStringWidth(roleR)
	roleRX := rightX + (labelWidthR / 2) - (roleRWidth / 2)

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleRX+2, currentY+5)
	pdf.CellFormat(roleRWidth, 5, roleR, "", 0, "L", false, 0, "")
	// ==================================================================================

	return pdf.OutputFileAndClose(outputPath)
}

// CreatePDFSP2ForSPL generates a PDF file for the Second Warning Letter (SP-2) for a Service Point Leader (SPL).
// It uses a template and replaces placeholders with actual data.
//
// Parameters:
//   - placeholders: A map containing key-value pairs for text replacement in the PDF.
//     Keys should match the placeholders used in the function (e.g., "$nomor_surat", "$nama_spl").
//   - outputPath: The file path where the generated PDF will be saved.
//
// Returns:
//   - error: An error if the PDF generation fails, nil otherwise.
func CreatePDFSP2ForSPL(placeholders map[string]string, outputPath string) error {
	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")
	imgTTDHRD := filepath.Join(imgAssetsDir, placeholders["$personalia_ttd"])
	imgTTDSAC := filepath.Join(imgAssetsDir, placeholders["$sac_ttd"])

	dbWeb := gormdb.Databases.Web
	var dataSP sptechnicianmodel.SPLGotSP
	err = dbWeb.
		Where("for_project = ?", placeholders["$for_project"]).
		Where("spl = ?", placeholders["$record_spl"]).
		Model(&sptechnicianmodel.SPLGotSP{}).
		First(&dataSP).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to query SPLGotSP: %v", err)
	}

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Peringatan 2", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("SP2, surat peringatan, spl, pelanggaran", true)
	pdf.SetSubject("Surat Peringatan 2 - Pemberitahuan untuk SPL", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	pdf.AddPage()

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold

	// Draw border
	pdf.SetLineWidth(0.5)
	pdf.Rect(10, 10, 190, 277, "")

	// ====================== Start of Header Layout ======================
	leftX := 20.0
	// reservedImageWidth := 20.0
	lineHeight1 := 1.0 // first line
	lineHeightOther := 7.0
	numInfoLines := 4
	infoHeight := lineHeight1 + lineHeightOther*float64(numInfoLines-1)

	// Top Y for header
	y := pdf.GetY() + 1

	// Draw logo (height = total info block height)
	logoHeight := infoHeight * 0.8 // 40% bigger than text block height
	pdf.ImageOptions(imgCSNA, leftX, y+3, 0, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Prepare info lines
	infoLines := []struct {
		Text     string
		FontSize float64
		Bold     bool
	}{
		{config.WebPanel.Get().Default.PT, 10.5, true}, // bold
		{"Rukan Crown Blok J No. 008, Green Lake City", 7, false},
		{"Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146", 7, false},
		{"Tel.: (021) 22521101 / 5504722 / 5504723", 7, false},
	}

	// Calculate total height of all info lines (tighter spacing)
	totalInfoHeight := 0.0
	for _, l := range infoLines {
		totalInfoHeight += l.FontSize * 0.6 // reduced spacing
	}

	// Starting Y for first info line to center block vertically relative to logo
	textStartY := y + (infoHeight-totalInfoHeight)/2

	pdf.SetY(textStartY)
	pageW, _ := pdf.GetPageSize()

	for _, l := range infoLines {
		style := ""
		if l.Bold {
			style = "B"
		}
		pdf.SetFont("CenturyGothic", style, l.FontSize)

		// Calculate text width
		strW := pdf.GetStringWidth(l.Text)
		textX := (pageW - strW) / 2 // center on full page width

		pdf.SetXY(textX, pdf.GetY())
		pdf.CellFormat(strW, l.FontSize, l.Text, "", 1, "C", false, 0, "")

		// Move Y down with tighter spacing
		pdf.SetY(pdf.GetY() - l.FontSize + (l.FontSize * 0.6))
	}
	// ====================== End of Header Layout ======================

	// Form code box (top right)
	pdf.SetXY(170, 1.7)
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(30, 8, "FM-HRD.07.00.01", "1", 0, "C", false, 0, "")

	// Horizontal line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, 35, 195, 35)

	// Underline using line drawing
	pdf.SetY(55)
	pdf.SetFont("Arial", "B", 12) // Use same font to measure text width

	// Measure text width for "SURAT PERINGATAN KEDUA (SP-2)"
	titleText := "SURAT PERINGATAN KEDUA (SP-2)"
	titleWidth := pdf.GetStringWidth(titleText)

	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY := 40.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)                          // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY) // 2mm below text baseline

	// Nomor
	currentY += 1 // Move down another 1mm
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(0, currentY)
	nomor := "Nomor : " + placeholders["$nomor_surat"] + "/SP.II-CSNA/" + placeholders["$bulan_romawi"] + "/" + placeholders["$tahun_sp"]
	pdf.CellFormat(210, 8, nomor, "", 1, "C", false, 0, "")

	// Body
	currentY += 14 // Move down 13mm from current position
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Dibuat oleh Perusahaan, dalam hal ini ditujukan kepada:", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Nama            : "+placeholders["$nama_spl"], "", 1, "L", false, 0, "")
	currentY += 5
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Jabatan         : "+placeholders["$jabatan_spl"], "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite := "Sehubungan dengan Surat Peringatan Pertama (SP-1) yang sebelumnya disampaikan kepada Sdr. $nama_spl, perusahaan kemudian memutuskan untuk menindaklanjuti melalui Surat Peringatan Kedua (SP-2). Hal ini didasari Sdr. $nama_spl yang tidak menunjukkan sikap disiplin/pelanggaran terhadap Tata Tertib Perusahaan yang Sdr. $nama_spl lakukan yaitu:"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_spl", placeholders["$nama_spl"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// List Pelanggaran
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "B", 10)
	listPelanggaran := []string{}
	if dataSP.PelanggaranSP1 != "" {
		listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP1)
	}
	if placeholders["$pelanggaran_karyawan"] != "" {
		listPelanggaran = append(listPelanggaran, placeholders["$pelanggaran_karyawan"])
	}

	indentPelanggaran := 10.0
	maxLenPelanggaran := 98 // adjust as needed

	for i, item := range listPelanggaran {
		// Word wrap by splitting into words
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxLenPelanggaran && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}

		if len(lines) == 0 {
			continue // skip empty pelanggaran
		}

		// Print first line with number and indent
		pdf.SetXY(15+indentPelanggaran, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()

		// Print remaining lines, aligned with text (number skipped, same margin)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentPelanggaran, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "") // <--- ln=1 (move to new line)
			currentY = pdf.GetY()
		}
	}

	currentY += 3
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite = "Atas perbuatan pelanggaran Tata Tertib Perusahaan yang dilakukan oleh Sdr. $nama_spl, maka dengan ini Perusahaan memberikan Surat Peringatan Kedua (SP-2) kepada Sdr. $nama_spl agar Karyawan dapat melakukan introspeksi dan memperbaiki diri sehingga Karyawan tidak lagi melakukan pelanggaran atas Tata Tertib Perusahaan dalam bentuk apapun. SP-2 ini diberikan kepada Karyawan dengan ketentuan sebagai berikut :"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_spl", placeholders["$nama_spl"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// Numbered list, indented and wrapped
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "", 10)
	listItems := []string{
		"Surat Peringatan Kedua berlaku untuk 1 (satu) hari kedepan sejak diterbitkan.",
		"Apabila dalam kurun waktu 1 (satu) hari kedepan sejak tanggal diterbitkan Surat Peringatan Kedua Karyawan tidak melakukan tindak pelanggaran yang menjadi dasar atas diterbitkannya surat Peringatan Kedua ini, maka Surat Peringatan Kedua Karyawan dinyatakan sudah tidak berlaku.",
		"Jika dalam kurun waktu 1 (satu) hari kedepan sejak Surat Peringatan Kedua diterbitkan Karyawan didapati kembali melakukan tindakan pelanggaran, maka perusahaan akan memberikan Surat Peringatan Ketiga (SP-3) atau Pemutusan Hubungan Kerja.",
	}
	indentList := 10.0
	maxLen := 98 // match pelanggaran wrapping
	for i, item := range listItems {
		// Word wrap by splitting into words
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxLen && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
		// Print first line with number and indent
		pdf.SetXY(15+indentList, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()
		// Print remaining lines, aligned with text (number skipped, same margin)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentList, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
			currentY = pdf.GetY()
		}
	}

	// Final paragraph: 'Demikian Surat Peringatan ...' all on one line, correct bold
	currentY += 2
	pdf.SetXY(15, currentY)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(pdf.GetStringWidth("Demikian "), 5, "Demikian ", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(pdf.GetStringWidth("Surat Peringatan"), 5, "Surat Peringatan", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, " ini dibuat agar dapat diperhatikan dan ditaati sebaik mungkin oleh yang bersangkutan.", "", 1, "L", false, 0, "")

	// =================================== Signatures ===================================
	currentY += 10
	leftX = 35.0
	rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_sp_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "Diterbitkan,", "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, currentY)
	pdf.CellFormat(80, 5, "Mengetahui,", "", 0, "L", false, 0, "")

	currentY += 20 // space for signatures

	// --- Left signature: Personalia ---
	ttdHRDWidth := 20.0
	pdf.ImageOptions(imgTTDHRD, leftX, currentY-15, ttdHRDWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center Personalia name below "Diterbitkan,"
	labelDiterbitkan := "Diterbitkan,"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$personalia_name"])
	padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	// Personalia Name (bold, underline)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$personalia_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)

	// Role text ("Personalia"), centered under the name
	roleWidth := pdf.GetStringWidth("Personalia")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Personalia", "", 0, "L", false, 0, "")

	// --- Right signature: SAC ---
	ttdSacWidth := 18.0
	rightXForTTD := rightX + 3.0
	switch placeholders["$sac_ttd"] {
	case "ttd_angga.png":
		ttdSacWidth = 30.0 // Budi's signature is wider
		rightXForTTD = rightX - 4.0
	case "ttd_tomi.png":
		rightXForTTD = rightX - 3.0
		ttdSacWidth = 25.0 // Tomi's signature is wider
	case "ttd_burhan.png":
		rightXForTTD = rightX + 5.0
		ttdSacWidth = 11.0 // Burhan's signature
	}

	pdf.ImageOptions(imgTTDSAC, rightXForTTD, currentY-15, ttdSacWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center SAC name below "Mengetahui,"
	labelMengetahui := "Mengetahui,"
	labelWidthR := pdf.GetStringWidth(labelMengetahui)
	mgrWidth := pdf.GetStringWidth(placeholders["$sac_name"])

	// Compute X so the SAC name is centered under the label
	centerXR := rightX + (labelWidthR / 2) - (mgrWidth / 2)

	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerXR, currentY)
	pdf.CellFormat(mgrWidth, 5, placeholders["$sac_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerXR, currentY+5, centerXR+mgrWidth+padding, currentY+5)

	// Role text ("Service Area Coordinator"), centered
	roleR := "Service Area Coordinator"
	roleRWidth := pdf.GetStringWidth(roleR)
	roleRX := rightX + (labelWidthR / 2) - (roleRWidth / 2)

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleRX+2, currentY+5)
	pdf.CellFormat(roleRWidth, 5, roleR, "", 0, "L", false, 0, "")
	// ==================================================================================

	return pdf.OutputFileAndClose(outputPath)
}

// CreatePDFSP3ForSPL generates a PDF file for the Third Warning Letter (SP-3) for a Service Point Leader (SPL).
// It uses a template and replaces placeholders with actual data.
//
// Parameters:
//   - placeholders: A map containing key-value pairs for text replacement in the PDF.
//     Keys should match the placeholders used in the function (e.g., "$nomor_surat", "$nama_spl").
//   - outputPath: The file path where the generated PDF will be saved.
//
// Returns:
//   - error: An error if the PDF generation fails, nil otherwise.
func CreatePDFSP3ForSPL(placeholders map[string]string, outputPath string) error {
	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")
	imgTTDHRD := filepath.Join(imgAssetsDir, placeholders["$personalia_ttd"])
	imgTTDSAC := filepath.Join(imgAssetsDir, placeholders["$sac_ttd"])

	dbWeb := gormdb.Databases.Web
	var dataSP sptechnicianmodel.SPLGotSP
	err = dbWeb.
		Where("for_project = ?", placeholders["$for_project"]).
		Where("spl = ?", placeholders["$record_spl"]).
		Model(&sptechnicianmodel.SPLGotSP{}).
		First(&dataSP).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to query SPLGotSP: %v", err)
	}

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Peringatan 3", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("SP3, surat peringatan, spl, pelanggaran", true)
	pdf.SetSubject("Surat Peringatan 3 - Pemberitahuan untuk SPL", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	pdf.AddPage()

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold

	// Draw border
	pdf.SetLineWidth(0.5)
	pdf.Rect(10, 10, 190, 277, "")

	// ====================== Start of Header Layout ======================
	leftX := 20.0
	// reservedImageWidth := 20.0
	lineHeight1 := 1.0 // first line
	lineHeightOther := 7.0
	numInfoLines := 4
	infoHeight := lineHeight1 + lineHeightOther*float64(numInfoLines-1)

	// Top Y for header
	y := pdf.GetY() + 1

	// Draw logo (height = total info block height)
	logoHeight := infoHeight * 0.8 // 40% bigger than text block height
	pdf.ImageOptions(imgCSNA, leftX, y+3, 0, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Prepare info lines
	infoLines := []struct {
		Text     string
		FontSize float64
		Bold     bool
	}{
		{config.WebPanel.Get().Default.PT, 10.5, true}, // bold
		{"Rukan Crown Blok J No. 008, Green Lake City", 7, false},
		{"Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146", 7, false},
		{"Tel.: (021) 22521101 / 5504722 / 5504723", 7, false},
	}

	// Calculate total height of all info lines (tighter spacing)
	totalInfoHeight := 0.0
	for _, l := range infoLines {
		totalInfoHeight += l.FontSize * 0.6 // reduced spacing
	}

	// Starting Y for first info line to center block vertically relative to logo
	textStartY := y + (infoHeight-totalInfoHeight)/2

	pdf.SetY(textStartY)
	pageW, _ := pdf.GetPageSize()

	for _, l := range infoLines {
		style := ""
		if l.Bold {
			style = "B"
		}
		pdf.SetFont("CenturyGothic", style, l.FontSize)

		// Calculate text width
		strW := pdf.GetStringWidth(l.Text)
		textX := (pageW - strW) / 2 // center on full page width

		pdf.SetXY(textX, pdf.GetY())
		pdf.CellFormat(strW, l.FontSize, l.Text, "", 1, "C", false, 0, "")

		// Move Y down with tighter spacing
		pdf.SetY(pdf.GetY() - l.FontSize + (l.FontSize * 0.6))
	}
	// ====================== End of Header Layout ======================

	// Form code box (top right)
	pdf.SetXY(170, 1.7)
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(30, 8, "FM-HRD.07.00.01", "1", 0, "C", false, 0, "")

	// Horizontal line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, 35, 195, 35)

	// Underline using line drawing
	pdf.SetY(55)
	pdf.SetFont("Arial", "B", 12) // Use same font to measure text width

	// Measure text width for "SURAT PERINGATAN KETIGA (SP-3)"
	titleText := "SURAT PERINGATAN KETIGA (SP-3)"
	titleWidth := pdf.GetStringWidth(titleText)

	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY := 40.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)                          // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY) // 2mm below text baseline

	// Nomor
	currentY += 1 // Move down another 1mm
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(0, currentY)
	nomor := "Nomor : " + placeholders["$nomor_surat"] + "/SP.III-CSNA/" + placeholders["$bulan_romawi"] + "/" + placeholders["$tahun_sp"]
	pdf.CellFormat(210, 8, nomor, "", 1, "C", false, 0, "")

	// Body
	currentY += 14 // Move down 13mm from current position
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Dibuat oleh Perusahaan, dalam hal ini ditujukan kepada:", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Nama            : "+placeholders["$nama_spl"], "", 1, "L", false, 0, "")
	currentY += 5
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Jabatan         : "+placeholders["$jabatan_spl"], "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	textToWrite := "Sehubungan dengan telah dikeluarkannya  Surat Peringatan Pertama (SP-1) dan Surat Peringatan Kedua (SP-2)  yang telah sebelumnya diberikan kepada Sdr. $nama_spl, namun yang bersangkutan tetap tidak menunjukkan perubahan sikap dan perbaikan diri, serta masih melakukan pelanggaran terhadap Tata Tertib Perusahaan, maka dengan ini Perusahaan menyampaikan Surat Peringatan Ketiga (SP-3). Adapun pelanggaran yang dilakukan oleh Sdr. $nama_spl adalah sebagai berikut :"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_spl", placeholders["$nama_spl"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// List Pelanggaran
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "B", 10)
	listPelanggaran := []string{}
	if dataSP.PelanggaranSP1 != "" {
		listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP1)
	}
	if dataSP.PelanggaranSP2 != "" {
		listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP2)
	}
	if placeholders["$pelanggaran_karyawan"] != "" {
		listPelanggaran = append(listPelanggaran, placeholders["$pelanggaran_karyawan"])
	}

	indentPelanggaran := 10.0
	maxLenPelanggaran := 98 // adjust as needed

	for i, item := range listPelanggaran {
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			// Use rune count for better wrapping (not just len)
			if len([]rune(line))+len([]rune(word))+1 > maxLenPelanggaran && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}

		if len(lines) == 0 {
			continue // skip empty pelanggaran
		}

		// Print first line with number and indent
		pdf.SetXY(15+indentPelanggaran, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()

		// Print remaining lines, aligned with text (number skipped, extra indent)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentPelanggaran, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
			currentY = pdf.GetY()
		}
	}

	currentY += 3
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite = "Surat Peringatan Ketiga (SP-3) ini merupakan peringatan terakhir yang sekaligus menjadi dasar bagi Perusahaan terhadap Sdr. $nama_spl untuk meminta HRD melakukan tindak-lanjut sesuai peraturan  karena telah berulang kali melakukan pelanggaran yang merugikan Perusahaan."
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_spl", placeholders["$nama_spl"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	currentY = pdf.GetY() + 2

	// Final paragraph with inline bold (Demikian + Surat Peringatan + rest)
	currentY += 2
	marginLeft := 15.0
	pdf.SetXY(marginLeft, currentY)

	// Define parts
	part1 := "Demikian "
	part2 := "Surat Peringatan"
	part3 := " terakhir ini disampaikan agar selanjutnya dapat menghubungi pihak HRD untuk melakukan klarifikasi  lebih lanjut. Jika dalam jangka waktu 2 (dua) hari kerja dari SP-3 diterbitkan, Sdr. $nama_spl tidak melakukan sanggahan, maka dianggap Sdr. $nama_spl Menyetujui penerbitan SP-3 ini dan Perusahaan berhak menerbitkan Surat Pemutusan Hubugan Kerja (S-PHK)."
	part3 = strings.ReplaceAll(part3, "$nama_spl", placeholders["$nama_spl"])

	// Build the full sentence (for wrapping width calculation)
	fullSentence := part1 + part2 + part3

	// Define maximum width (page width minus margins)
	marginRight := 15.0
	maxWidth := pageWidth - marginLeft - marginRight

	// Split into words for wrapping
	words := strings.Split(fullSentence, " ")
	line := ""
	for _, w := range words {
		testLine := strings.TrimSpace(line + " " + w)
		width := pdf.GetStringWidth(testLine)

		if width > maxWidth {
			// Print current line
			pdf.SetX(marginLeft) // <-- ensure every new line starts at same left margin
			pdf.SetFont("Arial", "", 10)

			if strings.Contains(line, part2) {
				before := strings.Split(line, part2)[0]
				after := strings.Split(line, part2)[1]

				// Print before
				pdf.CellFormat(pdf.GetStringWidth(before), 5, before, "", 0, "", false, 0, "")
				// Print bold
				pdf.SetFont("Arial", "B", 10)
				pdf.CellFormat(pdf.GetStringWidth(part2), 5, part2, "", 0, "", false, 0, "")
				// Back to normal
				pdf.SetFont("Arial", "", 10)
				pdf.CellFormat(pdf.GetStringWidth(after), 5, after, "", 0, "", false, 0, "")
			} else {
				pdf.CellFormat(pdf.GetStringWidth(line), 5, line, "", 0, "", false, 0, "")
			}

			pdf.Ln(5)
			line = w
		} else {
			line = testLine
		}
	}

	// Print the last line
	if line != "" {
		pdf.SetX(marginLeft) // <-- align last line too
		pdf.SetFont("Arial", "", 10)

		if strings.Contains(line, part2) {
			before := strings.Split(line, part2)[0]
			after := strings.Split(line, part2)[1]

			pdf.CellFormat(pdf.GetStringWidth(before), 5, before, "", 0, "", false, 0, "")
			pdf.SetFont("Arial", "B", 10)
			pdf.CellFormat(pdf.GetStringWidth(part2), 5, part2, "", 0, "", false, 0, "")
			pdf.SetFont("Arial", "", 10)
			pdf.CellFormat(pdf.GetStringWidth(after), 5, after, "", 0, "", false, 0, "")
		} else {
			pdf.CellFormat(pdf.GetStringWidth(line), 5, line, "", 0, "", false, 0, "")
		}
	}

	// =================================== Signatures ===================================
	currentY = pdf.GetY() + 2
	currentY += 10
	leftX = 35.0
	rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_sp_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "Diterbitkan,", "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, currentY)
	pdf.CellFormat(80, 5, "Mengetahui,", "", 0, "L", false, 0, "")

	currentY += 20 // space for signatures

	// --- Left signature: Personalia ---
	ttdHRDWidth := 20.0
	pdf.ImageOptions(imgTTDHRD, leftX, currentY-15, ttdHRDWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center Personalia name below "Diterbitkan,"
	labelDiterbitkan := "Diterbitkan,"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$personalia_name"])
	padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	// Personalia Name (bold, underline)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$personalia_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)

	// Role text ("Personalia"), centered under the name
	roleWidth := pdf.GetStringWidth("Personalia")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Personalia", "", 0, "L", false, 0, "")

	// --- Right signature: SAC ---
	ttdSacWidth := 18.0
	rightXForTTD := rightX + 3.0
	switch placeholders["$sac_ttd"] {
	case "ttd_angga.png":
		ttdSacWidth = 30.0 // Budi's signature is wider
		rightXForTTD = rightX - 4.0
	case "ttd_tomi.png":
		rightXForTTD = rightX - 3.0
		ttdSacWidth = 25.0 // Tomi's signature is wider
	case "ttd_burhan.png":
		rightXForTTD = rightX + 5.0
		ttdSacWidth = 11.0 // Burhan's signature
	}

	pdf.ImageOptions(imgTTDSAC, rightXForTTD, currentY-15, ttdSacWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center SAC name below "Mengetahui,"
	labelMengetahui := "Mengetahui,"
	labelWidthR := pdf.GetStringWidth(labelMengetahui)
	mgrWidth := pdf.GetStringWidth(placeholders["$sac_name"])

	// Compute X so the SAC name is centered under the label
	centerXR := rightX + (labelWidthR / 2) - (mgrWidth / 2)

	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerXR, currentY)
	pdf.CellFormat(mgrWidth, 5, placeholders["$sac_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerXR, currentY+5, centerXR+mgrWidth+padding, currentY+5)

	// Role text ("Service Area Coordinator"), centered
	roleR := "Service Area Coordinator"
	roleRWidth := pdf.GetStringWidth(roleR)
	roleRX := rightX + (labelWidthR / 2) - (roleRWidth / 2)

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleRX+2, currentY+5)
	pdf.CellFormat(roleRWidth, 5, roleR, "", 0, "L", false, 0, "")
	// ==================================================================================

	return pdf.OutputFileAndClose(outputPath)
}

// CreatePDFSP1ForSAC generates a PDF file for the First Warning Letter (SP-1) for a Service Area Coordinator (SAC).
// It uses a template and replaces placeholders with actual data.
//
// Parameters:
//   - placeholders: A map containing key-value pairs for text replacement in the PDF.
//     Keys should match the placeholders used in the function (e.g., "$nomor_surat", "$nama_sac").
//   - outputPath: The file path where the generated PDF will be saved.
//
// Returns:
//   - error: An error if the PDF generation fails, nil otherwise.
func CreatePDFSP1ForSAC(placeholders map[string]string, outputPath string) error {
	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")
	imgTTDHRD := filepath.Join(imgAssetsDir, placeholders["$personalia_ttd"])

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Peringatan 1", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("SP1, surat peringatan, sac, pelanggaran", true)
	pdf.SetSubject("Surat Peringatan 1 - Pemberitahuan untuk SAC", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	pdf.AddPage()

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold

	// Draw border
	pdf.SetLineWidth(0.5)
	pdf.Rect(10, 10, 190, 277, "")

	// ====================== Start of Header Layout ======================
	leftX := 20.0
	// reservedImageWidth := 20.0
	lineHeight1 := 1.0 // first line
	lineHeightOther := 7.0
	numInfoLines := 4
	infoHeight := lineHeight1 + lineHeightOther*float64(numInfoLines-1)

	// Top Y for header
	y := pdf.GetY() + 1

	// Draw logo (height = total info block height)
	logoHeight := infoHeight * 0.8 // 40% bigger than text block height
	pdf.ImageOptions(imgCSNA, leftX, y+3, 0, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Prepare info lines
	infoLines := []struct {
		Text     string
		FontSize float64
		Bold     bool
	}{
		{config.WebPanel.Get().Default.PT, 10.5, true}, // bold
		{"Rukan Crown Blok J No. 008, Green Lake City", 7, false},
		{"Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146", 7, false},
		{"Tel.: (021) 22521101 / 5504722 / 5504723", 7, false},
	}

	// Calculate total height of all info lines (tighter spacing)
	totalInfoHeight := 0.0
	for _, l := range infoLines {
		totalInfoHeight += l.FontSize * 0.6 // reduced spacing
	}

	// Starting Y for first info line to center block vertically relative to logo
	textStartY := y + (infoHeight-totalInfoHeight)/2

	pdf.SetY(textStartY)
	pageW, _ := pdf.GetPageSize()

	for _, l := range infoLines {
		style := ""
		if l.Bold {
			style = "B"
		}
		pdf.SetFont("CenturyGothic", style, l.FontSize)

		// Calculate text width
		strW := pdf.GetStringWidth(l.Text)
		textX := (pageW - strW) / 2 // center on full page width

		pdf.SetXY(textX, pdf.GetY())
		pdf.CellFormat(strW, l.FontSize, l.Text, "", 1, "C", false, 0, "")

		// Move Y down with tighter spacing
		pdf.SetY(pdf.GetY() - l.FontSize + (l.FontSize * 0.6))
	}
	// ====================== End of Header Layout ======================

	// Form code box (top right)
	pdf.SetXY(170, 1.7)
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(30, 8, "FM-HRD.07.00.01", "1", 0, "C", false, 0, "")

	// Horizontal line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, 35, 195, 35)

	// Underline using line drawing
	pdf.SetY(55)
	pdf.SetFont("Arial", "B", 12) // Use same font to measure text width

	// Measure text width for "SURAT PERINGATAN PERTAMA (SP-1)"
	titleText := "SURAT PERINGATAN PERTAMA (SP-1)"
	titleWidth := pdf.GetStringWidth(titleText)

	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY := 40.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)                          // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY) // 2mm below text baseline

	// Nomor
	currentY += 1 // Move down another 1mm
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(0, currentY)
	nomor := "Nomor : " + placeholders["$nomor_surat"] + "/SP.I-CSNA/" + placeholders["$bulan_romawi"] + "/" + placeholders["$tahun_sp"]
	pdf.CellFormat(210, 8, nomor, "", 1, "C", false, 0, "")

	// Body
	currentY += 14 // Move down 14mm from current position
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Dibuat oleh Perusahaan, dalam hal ini ditujukan kepada:", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Nama            : "+placeholders["$sac_name"], "", 1, "L", false, 0, "")
	currentY += 5
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Jabatan         : "+placeholders["$jabatan_sac"], "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	textToWrite := "Sehubungan dengan sikap tidak disiplin / pelanggaran terhadap tata tertib Perusahaan yang Karyawan lakukan yaitu:"
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "B", 10)
	indent := 10.0
	maxLen := 100 // adjust as needed
	pelanggaran := placeholders["$pelanggaran_karyawan"]

	// Word-wrap pelanggaran by words, all lines aligned with indent
	words := strings.Fields(pelanggaran)
	var lines []string
	line := ""
	for _, word := range words {
		if len(line)+len(word)+1 > maxLen && line != "" {
			lines = append(lines, line)
			line = word
		} else {
			if line != "" {
				line += " "
			}
			line += word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}

	// Print all lines: first line at normal X, wrapped lines at indented X
	for i, line := range lines {
		if i == 0 {
			pdf.SetXY(15+indent, currentY)
		} else {
			pdf.SetXY(15, currentY)
		}
		pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
		currentY = pdf.GetY()
	}

	currentY += 3
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite = "Atas perbuatan pelanggaran Peraturan Perusahaan yang dilakukan oleh $nama_sac, maka dengan ini Perusahaan memberikan Surat Peringatan Pertama (SP-1) kepada Karyawan, agar Karyawan dapat melakukan introspeksi dan memperbaiki diri sehingga Karyawan tidak lagi melakukan pelanggaran atas Peraturan Perusahaan dalam bentuk apapun. SP-1 ini diberikan kepada Karyawan dengan ketentuan sebagai berikut :"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_sac", placeholders["$nama_sac"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// Numbered list, indented and wrapped
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "", 10)
	listItems := []string{
		"Surat Peringatan Pertama berlaku untuk 1 (satu) hari kedepan sejak diterbitkan.",
		"Apabila dalam kurun waktu 1 (satu) hari kedepan sejak tanggal diterbitkan Surat Peringatan Pertama Karyawan tidak melakukan tindak pelanggaran yang menjadi dasar atas diterbitkannya surat peringatan pertama ini, maka Surat Peringatan Pertama Karyawan dinyatakan sudah tidak berlaku.",
		"Jika dalam kurun waktu 1 (satu) hari kedepan sejak Surat Peringatan Pertama diterbitkan Karyawan didapati kembali melakukan tindakan pelanggaran, maka perusahaan akan memberikan surat peringatan ke-2 untuk Karyawan.",
	}
	indentList := 10.0
	maxLen = 98 // match pelanggaran wrapping
	for i, item := range listItems {
		// Word wrap by splitting into words
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxLen && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
		// Print first line with number and indent
		pdf.SetXY(15+indentList, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()
		// Print remaining lines, aligned with text (number skipped, same margin)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentList, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
			currentY = pdf.GetY()
		}
	}

	// Final paragraph: 'Demikian Surat Peringatan ...' all on one line, correct bold
	currentY += 2
	pdf.SetXY(15, currentY)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(pdf.GetStringWidth("Demikian "), 5, "Demikian ", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(pdf.GetStringWidth("Surat Peringatan"), 5, "Surat Peringatan", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, " ini dibuat agar dapat diperhatikan dan ditaati sebaik mungkin oleh yang bersangkutan.", "", 1, "L", false, 0, "")

	// =================================== Signatures ===================================
	currentY += 10
	leftX = 35.0
	// rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_sp_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "Diterbitkan,", "", 0, "L", false, 0, "")
	// pdf.SetXY(rightX, currentY)
	// pdf.CellFormat(80, 5, "Mengetahui,", "", 0, "L", false, 0, "")

	currentY += 20 // space for signatures

	// --- Left signature: Personalia ---
	ttdHRDWidth := 20.0
	pdf.ImageOptions(imgTTDHRD, leftX, currentY-15, ttdHRDWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center Personalia name below "Diterbitkan,"
	labelDiterbitkan := "Diterbitkan,"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$personalia_name"])
	padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	// Personalia Name (bold, underline)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$personalia_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)

	// Role text ("Personalia"), centered under the name
	roleWidth := pdf.GetStringWidth("Personalia")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Personalia", "", 0, "L", false, 0, "")
	// ==================================================================================

	return pdf.OutputFileAndClose(outputPath)
}

// CreatePDFSP2ForSAC generates a PDF file for the Second Warning Letter (SP-2) for a Service Area Coordinator (SAC).
// It uses a template and replaces placeholders with actual data.
//
// Parameters:
//   - placeholders: A map containing key-value pairs for text replacement in the PDF.
//     Keys should match the placeholders used in the function (e.g., "$nomor_surat", "$nama_sac").
//   - outputPath: The file path where the generated PDF will be saved.
//
// Returns:
//   - error: An error if the PDF generation fails, nil otherwise.
func CreatePDFSP2ForSAC(placeholders map[string]string, outputPath string) error {
	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")
	imgTTDHRD := filepath.Join(imgAssetsDir, placeholders["$personalia_ttd"])

	dbWeb := gormdb.Databases.Web
	var dataSP sptechnicianmodel.SACGotSP
	err = dbWeb.
		Where("for_project = ?", placeholders["$for_project"]).
		Where("sac = ?", placeholders["$record_sac"]).
		Model(&sptechnicianmodel.SACGotSP{}).
		First(&dataSP).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to query SACGotSP: %v", err)
	}

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Peringatan 2", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("SP2, surat peringatan, sac, pelanggaran", true)
	pdf.SetSubject("Surat Peringatan 2 - Pemberitahuan untuk SAC", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	pdf.AddPage()

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold

	// Draw border
	pdf.SetLineWidth(0.5)
	pdf.Rect(10, 10, 190, 277, "")

	// ====================== Start of Header Layout ======================
	leftX := 20.0
	// reservedImageWidth := 20.0
	lineHeight1 := 1.0 // first line
	lineHeightOther := 7.0
	numInfoLines := 4
	infoHeight := lineHeight1 + lineHeightOther*float64(numInfoLines-1)

	// Top Y for header
	y := pdf.GetY() + 1

	// Draw logo (height = total info block height)
	logoHeight := infoHeight * 0.8 // 40% bigger than text block height
	pdf.ImageOptions(imgCSNA, leftX, y+3, 0, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Prepare info lines
	infoLines := []struct {
		Text     string
		FontSize float64
		Bold     bool
	}{
		{config.WebPanel.Get().Default.PT, 10.5, true}, // bold
		{"Rukan Crown Blok J No. 008, Green Lake City", 7, false},
		{"Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146", 7, false},
		{"Tel.: (021) 22521101 / 5504722 / 5504723", 7, false},
	}

	// Calculate total height of all info lines (tighter spacing)
	totalInfoHeight := 0.0
	for _, l := range infoLines {
		totalInfoHeight += l.FontSize * 0.6 // reduced spacing
	}

	// Starting Y for first info line to center block vertically relative to logo
	textStartY := y + (infoHeight-totalInfoHeight)/2

	pdf.SetY(textStartY)
	pageW, _ := pdf.GetPageSize()

	for _, l := range infoLines {
		style := ""
		if l.Bold {
			style = "B"
		}
		pdf.SetFont("CenturyGothic", style, l.FontSize)

		// Calculate text width
		strW := pdf.GetStringWidth(l.Text)
		textX := (pageW - strW) / 2 // center on full page width

		pdf.SetXY(textX, pdf.GetY())
		pdf.CellFormat(strW, l.FontSize, l.Text, "", 1, "C", false, 0, "")

		// Move Y down with tighter spacing
		pdf.SetY(pdf.GetY() - l.FontSize + (l.FontSize * 0.6))
	}
	// ====================== End of Header Layout ======================

	// Form code box (top right)
	pdf.SetXY(170, 1.7)
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(30, 8, "FM-HRD.07.00.01", "1", 0, "C", false, 0, "")

	// Horizontal line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, 35, 195, 35)

	// Underline using line drawing
	pdf.SetY(55)
	pdf.SetFont("Arial", "B", 12) // Use same font to measure text width

	// Measure text width for "SURAT PERINGATAN KEDUA (SP-2)"
	titleText := "SURAT PERINGATAN KEDUA (SP-2)"
	titleWidth := pdf.GetStringWidth(titleText)

	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY := 40.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)                          // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY) // 2mm below text baseline

	// Nomor
	currentY += 1 // Move down another 1mm
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(0, currentY)
	nomor := "Nomor : " + placeholders["$nomor_surat"] + "/SP.II-CSNA/" + placeholders["$bulan_romawi"] + "/" + placeholders["$tahun_sp"]
	pdf.CellFormat(210, 8, nomor, "", 1, "C", false, 0, "")

	// Body
	currentY += 14 // Move down 13mm from current position
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Dibuat oleh Perusahaan, dalam hal ini ditujukan kepada:", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Nama            : "+placeholders["$sac_name"], "", 1, "L", false, 0, "")
	currentY += 5
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Jabatan         : "+placeholders["$jabatan_sac"], "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite := "Sehubungan dengan Surat Peringatan Pertama (SP-1) yang sebelumnya disampaikan kepada Sdr. $nama_sac, perusahaan kemudian memutuskan untuk menindaklanjuti melalui Surat Peringatan Kedua (SP-2). Hal ini didasari Sdr. $nama_sac yang tidak menunjukkan sikap disiplin/pelanggaran terhadap Tata Tertib Perusahaan yang Sdr. $nama_sac lakukan yaitu:"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_sac", placeholders["$nama_sac"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// List Pelanggaran
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "B", 10)
	listPelanggaran := []string{}
	if dataSP.PelanggaranSP1 != "" {
		listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP1)
	}
	if placeholders["$pelanggaran_karyawan"] != "" {
		listPelanggaran = append(listPelanggaran, placeholders["$pelanggaran_karyawan"])
	}

	indentPelanggaran := 10.0
	maxLenPelanggaran := 98 // adjust as needed

	for i, item := range listPelanggaran {
		// Word wrap by splitting into words
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxLenPelanggaran && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}

		if len(lines) == 0 {
			continue // skip empty pelanggaran
		}

		// Print first line with number and indent
		pdf.SetXY(15+indentPelanggaran, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()

		// Print remaining lines, aligned with text (number skipped, same margin)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentPelanggaran, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "") // <--- ln=1 (move to new line)
			currentY = pdf.GetY()
		}
	}

	currentY += 3
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite = "Atas perbuatan pelanggaran Tata Tertib Perusahaan yang dilakukan oleh Sdr. $nama_sac, maka dengan ini Perusahaan memberikan Surat Peringatan Kedua (SP-2) kepada Sdr. $nama_sac agar Karyawan dapat melakukan introspeksi dan memperbaiki diri sehingga Karyawan tidak lagi melakukan pelanggaran atas Tata Tertib Perusahaan dalam bentuk apapun. SP-2 ini diberikan kepada Karyawan dengan ketentuan sebagai berikut :"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_sac", placeholders["$nama_sac"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// Numbered list, indented and wrapped
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "", 10)
	listItems := []string{
		"Surat Peringatan Kedua berlaku untuk 1 (satu) hari kedepan sejak diterbitkan.",
		"Apabila dalam kurun waktu 1 (satu) hari kedepan sejak tanggal diterbitkan Surat Peringatan Kedua Karyawan tidak melakukan tindak pelanggaran yang menjadi dasar atas diterbitkannya surat Peringatan Kedua ini, maka Surat Peringatan Kedua Karyawan dinyatakan sudah tidak berlaku.",
		"Jika dalam kurun waktu 1 (satu) hari kedepan sejak Surat Peringatan Kedua diterbitkan Karyawan didapati kembali melakukan tindakan pelanggaran, maka perusahaan akan memberikan Surat Peringatan Ketiga (SP-3) atau Pemutusan Hubungan Kerja.",
	}
	indentList := 10.0
	maxLen := 98 // match pelanggaran wrapping
	for i, item := range listItems {
		// Word wrap by splitting into words
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxLen && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
		// Print first line with number and indent
		pdf.SetXY(15+indentList, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()
		// Print remaining lines, aligned with text (number skipped, same margin)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentList, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
			currentY = pdf.GetY()
		}
	}

	// Final paragraph: 'Demikian Surat Peringatan ...' all on one line, correct bold
	currentY += 2
	pdf.SetXY(15, currentY)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(pdf.GetStringWidth("Demikian "), 5, "Demikian ", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(pdf.GetStringWidth("Surat Peringatan"), 5, "Surat Peringatan", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, " ini dibuat agar dapat diperhatikan dan ditaati sebaik mungkin oleh yang bersangkutan.", "", 1, "L", false, 0, "")

	// =================================== Signatures ===================================
	currentY += 10
	leftX = 35.0
	// rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_sp_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "Diterbitkan,", "", 0, "L", false, 0, "")
	// pdf.SetXY(rightX, currentY)
	// pdf.CellFormat(80, 5, "Mengetahui,", "", 0, "L", false, 0, "")

	currentY += 20 // space for signatures

	// --- Left signature: Personalia ---
	ttdHRDWidth := 20.0
	pdf.ImageOptions(imgTTDHRD, leftX, currentY-15, ttdHRDWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center Personalia name below "Diterbitkan,"
	labelDiterbitkan := "Diterbitkan,"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$personalia_name"])
	padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	// Personalia Name (bold, underline)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$personalia_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)

	// Role text ("Personalia"), centered under the name
	roleWidth := pdf.GetStringWidth("Personalia")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Personalia", "", 0, "L", false, 0, "")
	// ==================================================================================

	return pdf.OutputFileAndClose(outputPath)
}

// CreatePDFSP3ForSAC generates a PDF file for the Third Warning Letter (SP-3) for a Service Area Coordinator (SAC).
// It uses a template and replaces placeholders with actual data.
//
// Parameters:
//   - placeholders: A map containing key-value pairs for text replacement in the PDF.
//     Keys should match the placeholders used in the function (e.g., "$nomor_surat", "$nama_sac").
//   - outputPath: The file path where the generated PDF will be saved.
//
// Returns:
//   - error: An error if the PDF generation fails, nil otherwise.
func CreatePDFSP3ForSAC(placeholders map[string]string, outputPath string) error {
	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")
	imgTTDHRD := filepath.Join(imgAssetsDir, placeholders["$personalia_ttd"])

	dbWeb := gormdb.Databases.Web
	var dataSP sptechnicianmodel.SACGotSP
	err = dbWeb.
		Where("for_project = ?", placeholders["$for_project"]).
		Where("sac = ?", placeholders["$record_sac"]).
		Model(&sptechnicianmodel.SACGotSP{}).
		First(&dataSP).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to query SACGotSP: %v", err)
	}

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Peringatan 3", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("SP3, surat peringatan, sac, pelanggaran", true)
	pdf.SetSubject("Surat Peringatan 3 - Pemberitahuan untuk SAC", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	pdf.AddPage()

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold

	// Draw border
	pdf.SetLineWidth(0.5)
	pdf.Rect(10, 10, 190, 277, "")

	// ====================== Start of Header Layout ======================
	leftX := 20.0
	// reservedImageWidth := 20.0
	lineHeight1 := 1.0 // first line
	lineHeightOther := 7.0
	numInfoLines := 4
	infoHeight := lineHeight1 + lineHeightOther*float64(numInfoLines-1)

	// Top Y for header
	y := pdf.GetY() + 1

	// Draw logo (height = total info block height)
	logoHeight := infoHeight * 0.8 // 40% bigger than text block height
	pdf.ImageOptions(imgCSNA, leftX, y+3, 0, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Prepare info lines
	infoLines := []struct {
		Text     string
		FontSize float64
		Bold     bool
	}{
		{config.WebPanel.Get().Default.PT, 10.5, true}, // bold
		{"Rukan Crown Blok J No. 008, Green Lake City", 7, false},
		{"Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146", 7, false},
		{"Tel.: (021) 22521101 / 5504722 / 5504723", 7, false},
	}

	// Calculate total height of all info lines (tighter spacing)
	totalInfoHeight := 0.0
	for _, l := range infoLines {
		totalInfoHeight += l.FontSize * 0.6 // reduced spacing
	}

	// Starting Y for first info line to center block vertically relative to logo
	textStartY := y + (infoHeight-totalInfoHeight)/2

	pdf.SetY(textStartY)
	pageW, _ := pdf.GetPageSize()

	for _, l := range infoLines {
		style := ""
		if l.Bold {
			style = "B"
		}
		pdf.SetFont("CenturyGothic", style, l.FontSize)

		// Calculate text width
		strW := pdf.GetStringWidth(l.Text)
		textX := (pageW - strW) / 2 // center on full page width

		pdf.SetXY(textX, pdf.GetY())
		pdf.CellFormat(strW, l.FontSize, l.Text, "", 1, "C", false, 0, "")

		// Move Y down with tighter spacing
		pdf.SetY(pdf.GetY() - l.FontSize + (l.FontSize * 0.6))
	}
	// ====================== End of Header Layout ======================

	// Form code box (top right)
	pdf.SetXY(170, 1.7)
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(30, 8, "FM-HRD.07.00.01", "1", 0, "C", false, 0, "")

	// Horizontal line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, 35, 195, 35)

	// Underline using line drawing
	pdf.SetY(55)
	pdf.SetFont("Arial", "B", 12) // Use same font to measure text width

	// Measure text width for "SURAT PERINGATAN KETIGA (SP-3)"
	titleText := "SURAT PERINGATAN KETIGA (SP-3)"
	titleWidth := pdf.GetStringWidth(titleText)

	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY := 40.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)                          // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY) // 2mm below text baseline

	// Nomor
	currentY += 1 // Move down another 1mm
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(0, currentY)
	nomor := "Nomor : " + placeholders["$nomor_surat"] + "/SP.III-CSNA/" + placeholders["$bulan_romawi"] + "/" + placeholders["$tahun_sp"]
	pdf.CellFormat(210, 8, nomor, "", 1, "C", false, 0, "")

	// Body
	currentY += 14 // Move down 13mm from current position
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Dibuat oleh Perusahaan, dalam hal ini ditujukan kepada:", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Nama            : "+placeholders["$sac_name"], "", 1, "L", false, 0, "")
	currentY += 5
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Jabatan         : "+placeholders["$jabatan_sac"], "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	textToWrite := "Sehubungan dengan telah dikeluarkannya  Surat Peringatan Pertama (SP-1) dan Surat Peringatan Kedua (SP-2)  yang telah sebelumnya diberikan kepada Sdr. $nama_sac, namun yang bersangkutan tetap tidak menunjukkan perubahan sikap dan perbaikan diri, serta masih melakukan pelanggaran terhadap Tata Tertib Perusahaan, maka dengan ini Perusahaan menyampaikan Surat Peringatan Ketiga (SP-3). Adapun pelanggaran yang dilakukan oleh Sdr. $nama_sac adalah sebagai berikut :"
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_sac", placeholders["$nama_sac"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	// List Pelanggaran
	currentY = pdf.GetY() + 2
	pdf.SetFont("Arial", "B", 10)
	listPelanggaran := []string{}
	if dataSP.PelanggaranSP1 != "" {
		listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP1)
	}
	if dataSP.PelanggaranSP2 != "" {
		listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP2)
	}
	if placeholders["$pelanggaran_karyawan"] != "" {
		listPelanggaran = append(listPelanggaran, placeholders["$pelanggaran_karyawan"])
	}

	indentPelanggaran := 10.0
	maxLenPelanggaran := 98 // adjust as needed

	for i, item := range listPelanggaran {
		words := strings.Fields(item)
		var lines []string
		line := ""
		for _, word := range words {
			// Use rune count for better wrapping (not just len)
			if len([]rune(line))+len([]rune(word))+1 > maxLenPelanggaran && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				if line != "" {
					line += " "
				}
				line += word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}

		if len(lines) == 0 {
			continue // skip empty pelanggaran
		}

		// Print first line with number and indent
		pdf.SetXY(15+indentPelanggaran, currentY)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
		currentY = pdf.GetY()

		// Print remaining lines, aligned with text (number skipped, extra indent)
		for _, line := range lines[1:] {
			pdf.SetXY(18+indentPelanggaran, currentY)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
			currentY = pdf.GetY()
		}
	}

	currentY += 3
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	textToWrite = "Surat Peringatan Ketiga (SP-3) ini merupakan peringatan terakhir yang sekaligus menjadi dasar bagi Perusahaan terhadap Sdr. $nama_sac untuk meminta HRD melakukan tindak-lanjut sesuai peraturan  karena telah berulang kali melakukan pelanggaran yang merugikan Perusahaan."
	textToWrite = strings.ReplaceAll(textToWrite, "$nama_sac", placeholders["$nama_sac"])
	pdf.MultiCell(0, 5, textToWrite, "", "J", false)

	currentY = pdf.GetY() + 2

	// Final paragraph with inline bold (Demikian + Surat Peringatan + rest)
	currentY += 2
	marginLeft := 15.0
	pdf.SetXY(marginLeft, currentY)

	// Define parts
	part1 := "Demikian "
	part2 := "Surat Peringatan"
	part3 := " terakhir ini disampaikan agar selanjutnya dapat menghubungi pihak HRD untuk melakukan klarifikasi  lebih lanjut. Jika dalam jangka waktu 2 (dua) hari kerja dari SP-3 diterbitkan, Sdr. $nama_sac tidak melakukan sanggahan, maka dianggap Sdr. $nama_sac Menyetujui penerbitan SP-3 ini dan Perusahaan berhak menerbitkan Surat Pemutusan Hubugan Kerja (S-PHK)."
	part3 = strings.ReplaceAll(part3, "$nama_sac", placeholders["$nama_sac"])

	// Build the full sentence (for wrapping width calculation)
	fullSentence := part1 + part2 + part3

	// Define maximum width (page width minus margins)
	marginRight := 15.0
	maxWidth := pageWidth - marginLeft - marginRight

	// Split into words for wrapping
	words := strings.Split(fullSentence, " ")
	line := ""
	for _, w := range words {
		testLine := strings.TrimSpace(line + " " + w)
		width := pdf.GetStringWidth(testLine)

		if width > maxWidth {
			// Print current line
			pdf.SetX(marginLeft) // <-- ensure every new line starts at same left margin
			pdf.SetFont("Arial", "", 10)

			if strings.Contains(line, part2) {
				before := strings.Split(line, part2)[0]
				after := strings.Split(line, part2)[1]

				// Print before
				pdf.CellFormat(pdf.GetStringWidth(before), 5, before, "", 0, "", false, 0, "")
				// Print bold
				pdf.SetFont("Arial", "B", 10)
				pdf.CellFormat(pdf.GetStringWidth(part2), 5, part2, "", 0, "", false, 0, "")
				// Back to normal
				pdf.SetFont("Arial", "", 10)
				pdf.CellFormat(pdf.GetStringWidth(after), 5, after, "", 0, "", false, 0, "")
			} else {
				pdf.CellFormat(pdf.GetStringWidth(line), 5, line, "", 0, "", false, 0, "")
			}

			pdf.Ln(5)
			line = w
		} else {
			line = testLine
		}
	}

	// Print the last line
	if line != "" {
		pdf.SetX(marginLeft) // <-- align last line too
		pdf.SetFont("Arial", "", 10)

		if strings.Contains(line, part2) {
			before := strings.Split(line, part2)[0]
			after := strings.Split(line, part2)[1]

			pdf.CellFormat(pdf.GetStringWidth(before), 5, before, "", 0, "", false, 0, "")
			pdf.SetFont("Arial", "B", 10)
			pdf.CellFormat(pdf.GetStringWidth(part2), 5, part2, "", 0, "", false, 0, "")
			pdf.SetFont("Arial", "", 10)
			pdf.CellFormat(pdf.GetStringWidth(after), 5, after, "", 0, "", false, 0, "")
		} else {
			pdf.CellFormat(pdf.GetStringWidth(line), 5, line, "", 0, "", false, 0, "")
		}
	}

	// =================================== Signatures ===================================
	currentY = pdf.GetY() + 2
	currentY += 10
	leftX = 35.0
	// rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_sp_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "Diterbitkan,", "", 0, "L", false, 0, "")
	// pdf.SetXY(rightX, currentY)
	// pdf.CellFormat(80, 5, "Mengetahui,", "", 0, "L", false, 0, "")

	currentY += 20 // space for signatures

	// --- Left signature: Personalia ---
	ttdHRDWidth := 20.0
	pdf.ImageOptions(imgTTDHRD, leftX, currentY-15, ttdHRDWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center Personalia name below "Diterbitkan,"
	labelDiterbitkan := "Diterbitkan,"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$personalia_name"])
	padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	// Personalia Name (bold, underline)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$personalia_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)

	// Role text ("Personalia"), centered under the name
	roleWidth := pdf.GetStringWidth("Personalia")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Personalia", "", 0, "L", false, 0, "")

	// ==================================================================================

	return pdf.OutputFileAndClose(outputPath)
}

// processSPForSPL handles the processing of Warning Letters (SP-1, SP-2, SP-3) for a Service Point Leader (SPL).
// This function is triggered when a technician under the SPL's supervision receives an SP-3.
// It checks the SPL's current SP status, generates the appropriate SP document and audio, and updates the database.
//
// Parameters:
//   - dbWeb: Database connection for web panel.
//   - record: The job order plan record containing technician and SPL info.
//   - forProject: Project identifier.
//   - namaTeknisi: Name of the technician who got SP-3.
//   - namaSPL: Name of the SPL.
//   - splCity: City of the SPL.
//   - tanggalIndoFormatted: Formatted date in Indonesian.
//   - monthRoman: Current month in Roman numerals.
//   - tahunSP: Current year for SP.
//   - hrdPersonaliaName: Name of the HRD Personalia signing the SP.
//   - hrdTTDPath: File path to the HRD Personalia's signature image.
//   - hrdPhoneNumber: Phone number of the HRD Personalia.
//   - SACDataTechnician: Data of the SAC related to the technician.
//   - audioDirForSPSPL: Directory to save generated audio files.
//   - pdfDirForSPSPL: Directory to save generated PDF files.
//   - maxResponseSPAtHour: Hour deadline for SP response.
//   - now: Current time.
//   - resignTechnicianReplacer: String to use for resigned technicians/SPLs (e.g., replacing "*").
//   - needToSendTheSPSPLThroughWhatsapp: Map to track which SPLs need WhatsApp notifications.
func processSPForSPL(
	dbWeb *gorm.DB,
	record sptechnicianmodel.JOPlannedForTechnicianODOOMS,
	forProject string,
	namaTeknisi string,
	namaSPL string,
	splCity string,
	tanggalIndoFormatted string,
	monthRoman string,
	tahunSP string,
	hrdPersonaliaName string,
	hrdTTDPath string,
	hrdPhoneNumber string,
	SACDataTechnician config.SACODOOMS,
	audioDirForSPSPL string,
	pdfDirForSPSPL string,
	maxResponseSPAtHour int,
	now time.Time,
	resignTechnicianReplacer string,
	needToSendTheSPSPLThroughWhatsapp map[string]int,
) {
	spl := record.SPL
	if strings.Contains(spl, "*") {
		logrus.Debugf("Skipping SPL %s (contains *)", spl)
		return
	}

	// Reset SP status for SPL
	if spl != "" {
		if err := ResetSPLSP(spl, forProject); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logrus.Warnf("Failed to reset SP for SPL %s: %v", spl, err)
			}
		}
	}

	spForSPLIsProcessing := false
	var dataSPSPL sptechnicianmodel.SPLGotSP
	speech := htgotts.Speech{Folder: audioDirForSPSPL, Language: voices.Indonesian, Handler: &handlers.Native{}}
	spSPLResult := dbWeb.Where("for_project = ? AND spl = ?", forProject, spl).First(&dataSPSPL)
	if spSPLResult.Error != nil {
		if errors.Is(spSPLResult.Error, gorm.ErrRecordNotFound) {
			sp1SPLTextPart1 := "Sehubungan dengan surat peringatan (SP-3) yang telah"
			sp1SPLTextPart2 := fmt.Sprintf(" disampaikan kepada teknisi: %s", namaTeknisi)
			sp1SPLTextPart3 := fmt.Sprintf(" dibawah naungan saudara %s", namaSPL)
			sp1SPLTextPart4 := "maka perusahaan menilai perlu untuk menindaklanjuti,"
			sp1SPLTextPart5 := "dengan menerbitkan Surat Peringatan (SP-1) kepada saudara selaku Service Point Leader (SPL)."
			sp1SPLTextPart6 := "Hal ini didasari oleh tanggung jawab saudara sebagai SPL"
			sp1SPLTextPart7 := "dalam mengawasi dan membina teknisi di bawah naungan saudara."
			sp1SPLTextPart8 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan. terima kasih..."
			sp1SPLFilenameSound := fmt.Sprintf("%s_SP1_SPL", strings.ReplaceAll(spl, "*", resignTechnicianReplacer))
			fileTTSSP1SPL, err := fun.CreateRobustTTS(speech, audioDirForSPSPL, []string{
				sp1SPLTextPart1,
				sp1SPLTextPart2,
				sp1SPLTextPart3,
				sp1SPLTextPart4,
				sp1SPLTextPart5,
				sp1SPLTextPart6,
				sp1SPLTextPart7,
				sp1SPLTextPart8,
			}, sp1SPLFilenameSound)
			if err != nil {
				logrus.Errorf("failed to create merged SP1 TTS file for spl %s : %v", spl, err)
				return
			}

			if fileTTSSP1SPL != "" {
				fileInfo, statErr := os.Stat(fileTTSSP1SPL)
				if statErr == nil {
					logrus.Debugf("🔊 SP-1 merged TTS for %s - %s, Size: %d bytes", spl, fileTTSSP1SPL, fileInfo.Size())
				} else {
					logrus.Errorf("🔇 SP-1 TTS for %s got stat error : %v", spl, statErr)
				}
			}

			// Set SP - 1
			noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
			if err != nil {
				logrus.Errorf("Failed to increment nomor surat SP-1 for spl %s : %v", spl, err)
				return
			}
			var nomorSuratSP1Str string
			if noSuratSP1 < 1000 {
				nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
			} else {
				nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
			}

			// Placeholder for replace data in pdf SP - 1 SPL
			pelanggaranSP1SPLID := fmt.Sprintf("Kurangnya pengawasan dan pembinaan terhadap teknisi: %s, sehingga teknisi tersebut melakukan pelanggaran yang berujung pada penerbitan Surat Peringatan (SP-3) pada %v.", namaTeknisi, tanggalIndoFormatted)
			placeholderSP1SPL := map[string]string{
				"$nomor_surat":            nomorSuratSP1Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP1SPLID,
				"$nama_teknisi":           namaTeknisi,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACDataTechnician.FullName,
				"$sac_ttd":                SACDataTechnician.TTDPath,
				"$record_spl":             spl,
				"$for_project":            forProject,
			}
			pdfSP1FilenameSPL := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), now.Format("2006-01-02"))
			pdfSP1SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP1FilenameSPL)

			if err := CreatePDFSP1ForSPL(placeholderSP1SPL, pdfSP1SPLFilePath); err != nil {
				logrus.Errorf("failed to generate pdf of sp 1 spl %s : %v", spl, err)
				return
			}

			spForSPLIsProcessing = true // Mark SP for SPL already proceed for today so its not continue to sp - 2 or sp - 3
			splGotSP1At := time.Now()
			dataSP1SPL := sptechnicianmodel.SPLGotSP{
				SPL:                        spl,
				Name:                       namaSPL,
				ForProject:                 forProject,
				IsGotSP1:                   true,
				GotSP1At:                   &splGotSP1At,
				TechnicianNameCausedGotSP1: record.Technician,
				NoSP1:                      noSuratSP1,
				PelanggaranSP1:             pelanggaranSP1SPLID,
				SP1SoundTTSPath:            fileTTSSP1SPL,
				SP1FilePath:                pdfSP1SPLFilePath,
			}
			if err := dbWeb.Create(&dataSP1SPL).Error; err != nil {
				logrus.Errorf("failed to create the SP - 1 for SPL %s : %v", spl, err)
				return
			}

			if _, exists := needToSendTheSPSPLThroughWhatsapp[spl]; !exists {
				needToSendTheSPSPLThroughWhatsapp[spl] = 1
			}
			logrus.Infof("SP - 1 of SPL %s successfully created", spl)
			// .end of spl got no sp before
		} else {
			logrus.Errorf("failed to fetch SP data of spl %s : %v", spl, spSPLResult.Error)
			return
		}
	} // .end of check error of sp data from spl

	if !spForSPLIsProcessing {
		splGotSP1 := dataSPSPL.IsGotSP1
		splGotSP2 := dataSPSPL.IsGotSP2
		splGotSP3 := dataSPSPL.IsGotSP3

		// Try check spl sp status if he will got the sp - 2
		if splGotSP1 && !splGotSP2 && !splGotSP3 {
			// 1) First get the sp-1 sent at time
			sp1SentAt := dataSPSPL.GotSP1At
			if sp1SentAt == nil {
				// Try to find the sp - 1 sent at using the first whatsapp message sent
				var firstSP1SPLMsg sptechnicianmodel.SPWhatsAppMessage
				if err := dbWeb.Where("spl_got_sp_id = ? AND number_of_sp = ?", dataSPSPL.ID, 1).
					Order("whatsapp_sent_at asc").
					First(&firstSP1SPLMsg).
					Error; err != nil {
					logrus.Errorf("could not find the first sp-1 msg of spl %s to determine sent time : %v", spl, err)
					return
				}
				if firstSP1SPLMsg.WhatsappSentAt == nil {
					logrus.Errorf("SP-1 whatsapp msg sent time is nil for spl %s, cannot continue to create the sp-2 for spl", spl)
					return
				}
				sp1SentAt = firstSP1SPLMsg.WhatsappSentAt
			}
			// 2) Calculate the sp - 1 reply deadline
			deadlineSP1 := time.Date(sp1SentAt.Year(), sp1SentAt.Month(), sp1SentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sp1SentAt.Location())
			// 3) Count replies received before or equal to deadline
			var onTimeSP1RepliedCount int64
			dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
				Where("spl_got_sp_id = ?", dataSPSPL.ID).
				Where("number_of_sp = ?", 1).
				Where("for_project = ?", forProject).
				Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
				Where("whatsapp_replied_at <= ?", deadlineSP1).
				Count(&onTimeSP1RepliedCount)
			// 4) Proceed only if no on-time replies were found
			if onTimeSP1RepliedCount == 0 {
				// Check if the technician name caused spl got sp1 is same to current technician looping
				if dataSPSPL.TechnicianNameCausedGotSP1 == record.Technician {
					logrus.Warnf("SPL %s already got SP-1 caused by technician: %s, skipping SP-2 issuance", spl, record.Technician)
					return
				}

				// Check if the SPL already get the sp today, so it will be skipped
				if val, ok := needToSendTheSPSPLThroughWhatsapp[spl]; ok && val == 1 {
					logrus.Warnf("SPL %s already got SP-1 today, skipping SP-2 issuance", spl)
					return
				}

				// Check if the sp - 2 filepath already generated so skip the sp - 2 created
				if dataSPSPL.SP2FilePath != "" {
					logrus.Warnf("SPL %s sp-2 already generated, skipping SP-2 issuance", spl)
					return
				}

				var tglSP1SPLTerkirimFormatted string
				tglSP1SPLTerkirim, err := tanggal.Papar(*sp1SentAt, "Jakarta", tanggal.WIB)
				if err != nil {
					logrus.Errorf("failed to format date of tgl sp - 1 terkirim of spl %s : %v", spl, err)
					return
				}
				tglSP1SPLTerkirimFormatted = tglSP1SPLTerkirim.Format(" ", []tanggal.Format{
					tanggal.NamaHariDenganKoma,
					tanggal.Hari,
					tanggal.NamaBulan,
					tanggal.Tahun,
					tanggal.PukulDenganDetik,
					tanggal.ZonaWaktu,
				})

				// 5) Build text to sound for sp - 2 spl
				SP2SPLTextPart1 := fmt.Sprintf("Merujuk pada SP-1 yang telah diberikan kepada Saudara %s", namaSPL)
				SP2SPLTextPart2 := fmt.Sprintf("pada tanggal %s.", tglSP1SPLTerkirimFormatted)
				SP2SPLTextPart3 := "Sampai saat ini, tidak ada tanggapan dari Saudara."
				SP2SPLTextPart4 := "Hal tersebut menunjukkan kelalaian atas peringatan yang diberikan."
				SP2SPLTextPart5 := fmt.Sprintf("Selain itu, teknisi %s di bawah pengawasan Saudara", namaTeknisi)
				SP2SPLTextPart6 := "kembali melakukan pelanggaran."
				SP2SPLTextPart7 := "Pelanggaran tersebut berujung pada penerbitan SP-3."
				SP2SPLTextPart8 := "Dengan demikian, perusahaan menerbitkan SP-2 untuk Saudara."
				SP2SPLTextPart9 := "Mohon menjadi perhatian serius. terima kasih..."

				sp2SPLFilenameSound := fmt.Sprintf("%s_SP2_SPL", strings.ReplaceAll(spl, "*", resignTechnicianReplacer))
				fileTTSSP2SPL, err := fun.CreateRobustTTS(speech, audioDirForSPSPL, []string{
					SP2SPLTextPart1,
					SP2SPLTextPart2,
					SP2SPLTextPart3,
					SP2SPLTextPart4,
					SP2SPLTextPart5,
					SP2SPLTextPart6,
					SP2SPLTextPart7,
					SP2SPLTextPart8,
					SP2SPLTextPart9,
				}, sp2SPLFilenameSound)
				if err != nil {
					logrus.Errorf("Failed to create merged SP-2 TTS file for spl %s : %v", spl, err)
					return
				}

				if fileTTSSP2SPL != "" {
					fileInfo, statErr := os.Stat(fileTTSSP2SPL)
					if statErr == nil {
						logrus.Debugf("🔊 SP-2 merged TTS for %s - %s, Size: %d bytes", spl, fileTTSSP2SPL, fileInfo.Size())
					} else {
						logrus.Errorf("🔇 SP-2 TTS for %s got stat error : %v", spl, statErr)
					}
				}

				// 6) Set SP - 2 SPL
				noSuratSP2, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP2_GENERATED")
				if err != nil {
					logrus.Errorf("Failed to increment nomor surat SP-2 for spl %s: %v", spl, err)
					return
				}
				var nomorSuratSP2Str string
				if noSuratSP2 < 1000 {
					nomorSuratSP2Str = fmt.Sprintf("%03d", noSuratSP2)
				} else {
					nomorSuratSP2Str = fmt.Sprintf("%d", noSuratSP2)
				}

				// SP - 2 spl placeholders for pdf replacements
				pelanggaranSP2SPLID := fmt.Sprintf("Kurangnya pengawasan dan pembinaan terhadap teknisi %s, sehingga teknisi tersebut melakukan pelanggaran yang berujung pada penerbitan Surat Peringatan (SP-3) pada %v.", namaTeknisi, tanggalIndoFormatted)
				placeholdersSP2SPL := map[string]string{
					"$nomor_surat":            nomorSuratSP2Str,
					"$bulan_romawi":           monthRoman,
					"$tahun_sp":               tahunSP,
					"$nama_spl":               namaSPL,
					"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
					"$pelanggaran_karyawan":   pelanggaranSP2SPLID,
					"$nama_teknisi":           namaTeknisi,
					"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
					"$personalia_name":        hrdPersonaliaName,
					"$personalia_ttd":         hrdTTDPath,
					"$personalia_phone":       hrdPhoneNumber,
					"$sac_name":               SACDataTechnician.FullName,
					"$sac_ttd":                SACDataTechnician.TTDPath,
					"$record_spl":             spl,
					"$for_project":            forProject,
				}
				pdfSP2FilenameSPL := fmt.Sprintf("SP_2_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), now.Format("2006-01-02"))
				pdfSP2SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP2FilenameSPL)

				if err := CreatePDFSP2ForSPL(placeholdersSP2SPL, pdfSP2SPLFilePath); err != nil {
					logrus.Errorf("Failed to create PDF SP-2 for SPL %s: %v", spl, err)
					return
				}

				spForSPLIsProcessing = true // Mark sp 2 already proceed for today
				splGotSP2At := time.Now()
				dataSP2SPL := sptechnicianmodel.SPLGotSP{
					IsGotSP2:                   true,
					GotSP2At:                   &splGotSP2At,
					TechnicianNameCausedGotSP2: record.Technician,
					NoSP2:                      noSuratSP2,
					PelanggaranSP2:             pelanggaranSP2SPLID,
					SP2SoundTTSPath:            fileTTSSP2SPL,
					SP2FilePath:                pdfSP2SPLFilePath,
				}

				if err := dbWeb.Where("for_project = ? AND spl = ? AND is_got_sp1 = ?", forProject, spl, true).
					Updates(&dataSP2SPL).Error; err != nil {
					logrus.Errorf("failed to update the sp - 2 of spl %s : %v", spl, err)
					return
				}

				if _, exists := needToSendTheSPSPLThroughWhatsapp[spl]; !exists {
					needToSendTheSPSPLThroughWhatsapp[spl] = 2
				}
				logrus.Infof("SP - 2 of SPL %s successfully created", spl)
			} // .end of no on-time whatsapp sp-1 replied
		} // .end of check spl already got the sp - 1

		// Try check spl status if he will got the sp - 3
		if splGotSP1 && splGotSP2 && !splGotSP3 {
			// 1) First get the sp-2 sent at time
			sp2SentAt := dataSPSPL.GotSP2At
			if sp2SentAt == nil {
				// Try to find the sp - 2 sent at using the first whatsapp message sent
				var firstSP2SPLMsg sptechnicianmodel.SPWhatsAppMessage
				if err := dbWeb.Where("spl_got_sp_id = ? AND number_of_sp = ?", dataSPSPL.ID, 2).
					Order("whatsapp_sent_at asc").
					First(&firstSP2SPLMsg).
					Error; err != nil {
					logrus.Errorf("could not find the first sp-2 msg of spl %s to determine sent time : %v", spl, err)
					return
				}
				if firstSP2SPLMsg.WhatsappSentAt == nil {
					logrus.Errorf("SP-2 whatsapp msg sent time is nil for spl %s, cannot continue to create the sp-3 for spl", spl)
					return
				}
				sp2SentAt = firstSP2SPLMsg.WhatsappSentAt
			}
			// 2) Calculate the sp - 2 reply deadline
			deadlineSP2 := time.Date(sp2SentAt.Year(), sp2SentAt.Month(), sp2SentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sp2SentAt.Location())
			// 3) Count replies received before or equal to deadline
			var onTimeSP2RepliedCount int64
			dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
				Where("spl_got_sp_id = ?", dataSPSPL.ID).
				Where("number_of_sp = ?", 2).
				Where("for_project = ?", forProject).
				Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
				Where("whatsapp_replied_at <= ?", deadlineSP2).
				Count(&onTimeSP2RepliedCount)
			// 4) Proceed only if no on-time replies were found
			if onTimeSP2RepliedCount == 0 {
				// Check if the technician name caused spl got sp2 is same to current technician looping
				if dataSPSPL.TechnicianNameCausedGotSP1 == record.Technician || dataSPSPL.TechnicianNameCausedGotSP2 == record.Technician {
					logrus.Warnf("SPL %s already got SP-1 or SP-2 caused by technician: %s, skipping SP-3 issuance", spl, record.Technician)
					return
				}

				// Check if the SPL already get the sp today, so it will be skipped
				if val, ok := needToSendTheSPSPLThroughWhatsapp[spl]; ok && val == 2 {
					logrus.Warnf("SPL %s already got SP-2 today, skipping SP-3 issuance", spl)
					return
				}

				// Check if the sp - 2 filepath already generated so skip the sp - 3 created
				if dataSPSPL.SP3FilePath != "" {
					logrus.Warnf("SPL %s sp-3 already generated, skipping SP-3 issuance", spl)
					return
				}

				var tglSP2SPLTerkirimFormatted string
				tglSP2SPLTerkirim, err := tanggal.Papar(*sp2SentAt, "Jakarta", tanggal.WIB)
				if err != nil {
					logrus.Errorf("failed to format date of tgl sp - 2 terkirim of spl %s : %v", spl, err)
					return
				}
				tglSP2SPLTerkirimFormatted = tglSP2SPLTerkirim.Format(" ", []tanggal.Format{
					tanggal.NamaHariDenganKoma,
					tanggal.Hari,
					tanggal.NamaBulan,
					tanggal.Tahun,
					tanggal.PukulDenganDetik,
					tanggal.ZonaWaktu,
				})

				// 5) Build text to sound for sp - 3 spl
				SP3SPLTextPart1 := "Merujuk pada Surat Peringatan (SP-2) yang telah disampaikan"
				SP3SPLTextPart2 := fmt.Sprintf(" kepada Saudara %s", namaSPL)
				SP3SPLTextPart3 := fmt.Sprintf("pada tanggal %s.", tglSP2SPLTerkirimFormatted)
				SP3SPLTextPart4 := "perusahaan menilai bahwa pelanggaran yang saudara lakukan tidak kunjung diperbaiki."
				SP3SPLTextPart5 := "Hal ini menunjukkan sikap yang tidak responsif terhadap peringatan yang telah diberikan."
				SP3SPLTextPart6 := fmt.Sprintf("Selain itu, teknisi %s", namaTeknisi)
				SP3SPLTextPart7 := " di bawah pengawasan Saudara"
				SP3SPLTextPart8 := "kembali melakukan pelanggaran yang berujung pada penerbitan SP-3."
				SP3SPLTextPart9 := "Dengan demikian, perusahaan dengan berat hati menerbitkan SP-3 untuk Saudara."
				SP3SPLTextPart10 := "Surat ini juga menyatakan berakhirnya hubungan kerja Saudara dengan perusahaan."
				SP3SPLTextPart11 := "Keputusan berlaku efektif sejak tanggal diterbitkannya surat ini."
				SP3SPLTextPart12 := fmt.Sprintf("yakni pada tanggal %s.", tanggalIndoFormatted)
				SP3SPLTextPart13 := "Kami mengucapkan terima kasih atas kontribusi Saudara selama ini."
				SP3SPLTextPart14 := "Semoga Saudara mendapatkan kesuksesan di masa depan. terima kasih..."

				sp3SPLFilenameSound := fmt.Sprintf("%s_SP3_SPL", strings.ReplaceAll(spl, "*", resignTechnicianReplacer))
				fileTTSSP3SPL, err := fun.CreateRobustTTS(speech, audioDirForSPSPL, []string{
					SP3SPLTextPart1,
					SP3SPLTextPart2,
					SP3SPLTextPart3,
					SP3SPLTextPart4,
					SP3SPLTextPart5,
					SP3SPLTextPart6,
					SP3SPLTextPart7,
					SP3SPLTextPart8,
					SP3SPLTextPart9,
					SP3SPLTextPart10,
					SP3SPLTextPart11,
					SP3SPLTextPart12,
					SP3SPLTextPart13,
					SP3SPLTextPart14,
				}, sp3SPLFilenameSound)
				if err != nil {
					logrus.Errorf("Failed to create merged SP-3 TTS file for spl %s : %v", spl, err)
					return
				}

				if fileTTSSP3SPL != "" {
					fileInfo, statErr := os.Stat(fileTTSSP3SPL)
					if statErr == nil {
						logrus.Debugf("🔊 SP-3 merged TTS for %s - %s, Size: %d bytes", spl, fileTTSSP3SPL, fileInfo.Size())
					} else {
						logrus.Errorf("🔇 SP-3 TTS for %s got stat error : %v", spl, statErr)
					}
				}

				// 6) Set SP - 3 SPL
				noSuratSP3, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP3_GENERATED")
				if err != nil {
					logrus.Errorf("Failed to increment nomor surat SP-3 for spl %s: %v", spl, err)
					return
				}
				var nomorSuratSP3Str string
				if noSuratSP3 < 1000 {
					nomorSuratSP3Str = fmt.Sprintf("%03d", noSuratSP3)
				} else {
					nomorSuratSP3Str = fmt.Sprintf("%d", noSuratSP3)
				}

				// SP - 3 spl placeholders for pdf replacements
				pelanggaranSP3SPLID := fmt.Sprintf("Kurangnya pengawasan dan pembinaan terhadap teknisi %s, sehingga teknisi tersebut melakukan pelanggaran yang berujung pada penerbitan Surat Peringatan (SP-3) pada %v.", namaTeknisi, tanggalIndoFormatted)
				placeholdersSP3SPL := map[string]string{
					"$nomor_surat":            nomorSuratSP3Str,
					"$bulan_romawi":           monthRoman,
					"$tahun_sp":               tahunSP,
					"$nama_spl":               namaSPL,
					"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
					"$pelanggaran_karyawan":   pelanggaranSP3SPLID,
					"$nama_teknisi":           namaTeknisi,
					"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
					"$personalia_name":        hrdPersonaliaName,
					"$personalia_ttd":         hrdTTDPath,
					"$personalia_phone":       hrdPhoneNumber,
					"$sac_name":               SACDataTechnician.FullName,
					"$sac_ttd":                SACDataTechnician.TTDPath,
					"$record_spl":             spl,
					"$for_project":            forProject,
				}
				pdfSP3FilenameSPL := fmt.Sprintf("SP_3_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), now.Format("2006-01-02"))
				pdfSP3SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP3FilenameSPL)

				if err := CreatePDFSP3ForSPL(placeholdersSP3SPL, pdfSP3SPLFilePath); err != nil {
					logrus.Errorf("Failed to create PDF SP-3 for SPL %s: %v", spl, err)
					return
				}

				spForSPLIsProcessing = true // Mark sp 3 already proceed for today
				splGotSP3At := time.Now()
				dataSP3SPL := sptechnicianmodel.SPLGotSP{
					IsGotSP3:                   true,
					GotSP3At:                   &splGotSP3At,
					TechnicianNameCausedGotSP3: record.Technician,
					NoSP3:                      noSuratSP3,
					PelanggaranSP3:             pelanggaranSP3SPLID,
					SP3SoundTTSPath:            fileTTSSP3SPL,
					SP3FilePath:                pdfSP3SPLFilePath,
				}

				if err := dbWeb.Where("for_project = ? AND spl = ? AND is_got_sp2 = ?", forProject, spl, true).
					Updates(&dataSP3SPL).Error; err != nil {
					logrus.Errorf("failed to update the sp - 3 of spl %s : %v", spl, err)
					return
				}

				if _, exists := needToSendTheSPSPLThroughWhatsapp[spl]; !exists {
					needToSendTheSPSPLThroughWhatsapp[spl] = 3
				}
				logrus.Infof("SP - 3 of SPL %s successfully created", spl)
			} // .end of no on-time whatsapp sp-2 replied
		} // .end of check spl already got the sp - 2
	} // .end of sp for spl didnt proceed yet
}

// processSPForSAC handles the processing of Warning Letters (SP-1, SP-2, SP-3) for a Service Area Coordinator (SAC).
// This function is triggered when a technician under the SAC's supervision receives an SP-3.
// It checks the SAC's current SP status, generates the appropriate SP document and audio, and updates the database.
//
// Parameters:
//   - dbWeb: Database connection for web panel.
//   - record: The job order plan record containing technician and SAC info.
//   - forProject: Project identifier.
//   - namaTeknisi: Name of the technician who got SP-3.
//   - namaSAC: Name of the SAC.
//   - tanggalIndoFormatted: Formatted date in Indonesian.
//   - monthRoman: Current month in Roman numerals.
//   - tahunSP: Current year for SP.
//   - hrdPersonaliaName: Name of the HRD Personalia signing the SP.
//   - hrdTTDPath: Path to the HRD Personalia's signature image.
//   - hrdPhoneNumber: Phone number of the HRD Personalia.
//   - SACDataTechnician: Data of the SAC related to the technician.
//   - audioDirForSPSAC: Directory to save generated audio files.
//   - pdfDirForSPSAC: Directory to save generated PDF files.
//   - maxResponseSPAtHour: Hour deadline for SP response.
//   - now: Current time.
//   - resignTechnicianReplacer: String to use for resigned technicians/SACs.
//   - needToSendTheSPSACThroughWhatsapp: Map to track which SACs need WhatsApp notifications.
func processSPForSAC(
	dbWeb *gorm.DB,
	record sptechnicianmodel.JOPlannedForTechnicianODOOMS,
	forProject string,
	namaTeknisi string,
	namaSAC string,
	tanggalIndoFormatted string,
	monthRoman string,
	tahunSP string,
	hrdPersonaliaName string,
	hrdTTDPath string,
	hrdPhoneNumber string,
	SACDataTechnician config.SACODOOMS,
	audioDirForSPSAC string,
	pdfDirForSPSAC string,
	maxResponseSPAtHour int,
	now time.Time,
	resignTechnicianReplacer string,
	needToSendTheSPSACThroughWhatsapp map[string]int,
) {
	sac := record.SAC
	if strings.Contains(sac, "*") {
		logrus.Debugf("Skipping SAC %s (contains *)", sac)
		return
	}

	// Reset SP status for SAC
	if sac != "" {
		if err := ResetSACSP(sac, forProject); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logrus.Warnf("Failed to reset SP for SAC %s: %v", sac, err)
			}
		}
	}

	spForSACIsProcessing := false
	var dataSPSAC sptechnicianmodel.SACGotSP
	speech := htgotts.Speech{Folder: audioDirForSPSAC, Language: voices.Indonesian, Handler: &handlers.Native{}}
	spSACResult := dbWeb.Where("for_project = ? AND sac = ?", forProject, sac).First(&dataSPSAC)
	if spSACResult.Error != nil {
		if errors.Is(spSACResult.Error, gorm.ErrRecordNotFound) {
			sp1SACTextPart1 := "Sehubungan dengan surat peringatan (SP-3) yang telah"
			sp1SACTextPart2 := fmt.Sprintf(" disampaikan kepada teknisi: %s", namaTeknisi)
			sp1SACTextPart3 := fmt.Sprintf(" dibawah naungan saudara %s", namaSAC)
			sp1SACTextPart4 := "maka perusahaan menilai perlu untuk menindaklanjuti,"
			sp1SACTextPart5 := "dengan menerbitkan Surat Peringatan (SP-1) kepada saudara selaku Service Area Coordinator (SAC)."
			sp1SACTextPart6 := "Hal ini didasari oleh tanggung jawab saudara sebagai SAC"
			sp1SACTextPart7 := "dalam mengawasi dan membina teknisi di bawah naungan saudara."
			sp1SACTextPart8 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan. terima kasih..."
			sp1SACFilenameSound := fmt.Sprintf("%s_SP1_SAC", strings.ReplaceAll(sac, "*", resignTechnicianReplacer))
			fileTTSSP1SAC, err := fun.CreateRobustTTS(speech, audioDirForSPSAC, []string{
				sp1SACTextPart1,
				sp1SACTextPart2,
				sp1SACTextPart3,
				sp1SACTextPart4,
				sp1SACTextPart5,
				sp1SACTextPart6,
				sp1SACTextPart7,
				sp1SACTextPart8,
			}, sp1SACFilenameSound)
			if err != nil {
				logrus.Errorf("failed to create merged SP1 TTS file for sac %s : %v", sac, err)
				return
			}

			if fileTTSSP1SAC != "" {
				fileInfo, statErr := os.Stat(fileTTSSP1SAC)
				if statErr == nil {
					logrus.Debugf("🔊 SP-1 merged TTS for %s - %s, Size: %d bytes", sac, fileTTSSP1SAC, fileInfo.Size())
				} else {
					logrus.Errorf("🔇 SP-1 TTS for %s got stat error : %v", sac, statErr)
				}
			}

			// Set SP - 1
			noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
			if err != nil {
				logrus.Errorf("Failed to increment nomor surat SP-1 for sac %s : %v", sac, err)
				return
			}
			var nomorSuratSP1Str string
			if noSuratSP1 < 1000 {
				nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
			} else {
				nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
			}

			// Placeholder for replace data in pdf SP - 1 SAC
			pelanggaranSP1SACID := fmt.Sprintf("Kurangnya pengawasan dan pembinaan terhadap teknisi: %s, sehingga teknisi tersebut melakukan pelanggaran yang berujung pada penerbitan Surat Peringatan (SP-3) pada %v.", namaTeknisi, tanggalIndoFormatted)
			placeholderSP1SAC := map[string]string{
				"$nomor_surat":            nomorSuratSP1Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_sac":               namaSAC,
				"$jabatan_sac":            fmt.Sprintf("Service Area Coordinator - Region %d", SACDataTechnician.Region),
				"$pelanggaran_karyawan":   pelanggaranSP1SACID,
				"$nama_teknisi":           namaTeknisi,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACDataTechnician.FullName,
				"$sac_ttd":                SACDataTechnician.TTDPath,
				"$for_project":            forProject,
			}
			pdfSP1FilenameSAC := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(sac, "*", resignTechnicianReplacer), now.Format("2006-01-02"))
			pdfSP1SACFilePath := filepath.Join(pdfDirForSPSAC, pdfSP1FilenameSAC)

			if err := CreatePDFSP1ForSAC(placeholderSP1SAC, pdfSP1SACFilePath); err != nil {
				logrus.Errorf("failed to generate pdf of sp 1 sac %s : %v", sac, err)
				return
			}

			spForSACIsProcessing = true // Mark SP for SAC already proceed for today so its not continue to sp - 2 or sp - 3
			sacGotSP1At := time.Now()
			dataSP1SAC := sptechnicianmodel.SACGotSP{
				SAC:                        sac,
				Name:                       namaSAC,
				ForProject:                 forProject,
				IsGotSP1:                   true,
				GotSP1At:                   &sacGotSP1At,
				TechnicianNameCausedGotSP1: record.Technician,
				NoSP1:                      noSuratSP1,
				PelanggaranSP1:             pelanggaranSP1SACID,
				SP1SoundTTSPath:            fileTTSSP1SAC,
				SP1FilePath:                pdfSP1SACFilePath,
			}
			if err := dbWeb.Create(&dataSP1SAC).Error; err != nil {
				logrus.Errorf("failed to create the SP - 1 for SAC %s : %v", sac, err)
				return
			}

			if _, exists := needToSendTheSPSACThroughWhatsapp[sac]; !exists {
				needToSendTheSPSACThroughWhatsapp[sac] = 1
			}

			logrus.Infof("SP - 1 of SAC %s successfully created", sac)
			// .end of sac got no sp before
		} else {
			logrus.Errorf("failed to fetch SP data of sac %s : %v", sac, spSACResult.Error)
			return
		}
	} // .end of check error of sp data from sac

	if !spForSACIsProcessing {
		sacGotSP1 := dataSPSAC.IsGotSP1
		sacGotSP2 := dataSPSAC.IsGotSP2
		sacGotSP3 := dataSPSAC.IsGotSP3

		// Try check sac sp status if he will got the sp - 2
		if sacGotSP1 && !sacGotSP2 && !sacGotSP3 {
			// 1) First get the sp-1 sent at time
			sp1SentAt := dataSPSAC.GotSP1At
			if sp1SentAt == nil {
				// Try to find the sp - 1 sent at using the first whatsapp message sent
				var firstSP1SACMsg sptechnicianmodel.SPWhatsAppMessage
				if err := dbWeb.Where("sac_got_sp_id = ? AND number_of_sp = ?", dataSPSAC.ID, 1).
					Order("whatsapp_sent_at asc").
					First(&firstSP1SACMsg).
					Error; err != nil {
					logrus.Errorf("could not find the first sp-1 msg of sac %s to determine sent time : %v", sac, err)
					return
				}
				if firstSP1SACMsg.WhatsappSentAt == nil {
					logrus.Errorf("SP-1 whatsapp msg sent time is nil for sac %s, cannot continue to create the sp-2 for sac", sac)
					return
				}
				sp1SentAt = firstSP1SACMsg.WhatsappSentAt
			}
			// 2) Calculate the sp - 1 reply deadline
			deadlineSP1 := time.Date(sp1SentAt.Year(), sp1SentAt.Month(), sp1SentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sp1SentAt.Location())
			// 3) Count replies received before or equal to deadline
			var onTimeSP1RepliedCount int64
			dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
				Where("sac_got_sp_id = ?", dataSPSAC.ID).
				Where("number_of_sp = ?", 1).
				Where("for_project = ?", forProject).
				Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
				Where("whatsapp_replied_at <= ?", deadlineSP1).
				Count(&onTimeSP1RepliedCount)
			// 4) Proceed only if no on-time replies were found
			if onTimeSP1RepliedCount == 0 {
				// Check if the technician name caused sac got sp1 is same to current technician looping
				if dataSPSAC.TechnicianNameCausedGotSP1 == record.Technician {
					logrus.Warnf("SAC %s already got SP-1 caused by technician: %s, skipping SP-2 issuance", sac, record.Technician)
					return
				}

				// Check if the SAC already get the sp today, so it will be skipped
				if val, ok := needToSendTheSPSACThroughWhatsapp[sac]; ok && val == 1 {
					logrus.Warnf("SAC %s already got SP-1 today, skipping SP-2 issuance", sac)
					return
				}

				// Check if the sp - 2 filepath already generated so skip the sp - 2 created
				if dataSPSAC.SP2FilePath != "" {
					logrus.Warnf("SAC %s sp-2 already generated, skipping SP-2 issuance", sac)
					return
				}

				var tglSP1SACTerkirimFormatted string
				tglSP1SACTerkirim, err := tanggal.Papar(*sp1SentAt, "Jakarta", tanggal.WIB)
				if err != nil {
					logrus.Errorf("failed to format date of tgl sp - 1 terkirim of sac %s : %v", sac, err)
					return
				}
				tglSP1SACTerkirimFormatted = tglSP1SACTerkirim.Format(" ", []tanggal.Format{
					tanggal.NamaHariDenganKoma,
					tanggal.Hari,
					tanggal.NamaBulan,
					tanggal.Tahun,
					tanggal.PukulDenganDetik,
					tanggal.ZonaWaktu,
				})

				// 5) Build text to sound for sp - 2 sac
				SP2SACTextPart1 := fmt.Sprintf("Merujuk pada SP-1 yang telah diberikan kepada Saudara %s", namaSAC)
				SP2SACTextPart2 := fmt.Sprintf("pada tanggal %s.", tglSP1SACTerkirimFormatted)
				SP2SACTextPart3 := "Sampai saat ini, tidak ada tanggapan dari Saudara."
				SP2SACTextPart4 := "Hal tersebut menunjukkan kelalaian atas peringatan yang diberikan."
				SP2SACTextPart5 := fmt.Sprintf("Selain itu, teknisi %s di bawah pengawasan Saudara", namaTeknisi)
				SP2SACTextPart6 := "kembali melakukan pelanggaran."
				SP2SACTextPart7 := "Pelanggaran tersebut berujung pada penerbitan SP-3."
				SP2SACTextPart8 := "Dengan demikian, perusahaan menerbitkan SP-2 untuk Saudara."
				SP2SACTextPart9 := "Mohon menjadi perhatian serius. terima kasih..."

				sp2SACFilenameSound := fmt.Sprintf("%s_SP2_SAC", strings.ReplaceAll(sac, "*", resignTechnicianReplacer))
				fileTTSSP2SAC, err := fun.CreateRobustTTS(speech, audioDirForSPSAC, []string{
					SP2SACTextPart1,
					SP2SACTextPart2,
					SP2SACTextPart3,
					SP2SACTextPart4,
					SP2SACTextPart5,
					SP2SACTextPart6,
					SP2SACTextPart7,
					SP2SACTextPart8,
					SP2SACTextPart9,
				}, sp2SACFilenameSound)
				if err != nil {
					logrus.Errorf("Failed to create merged SP-2 TTS file for sac %s : %v", sac, err)
					return
				}

				if fileTTSSP2SAC != "" {
					fileInfo, statErr := os.Stat(fileTTSSP2SAC)
					if statErr == nil {
						logrus.Debugf("🔊 SP-2 merged TTS for %s - %s, Size: %d bytes", sac, fileTTSSP2SAC, fileInfo.Size())
					} else {
						logrus.Errorf("🔇 SP-2 TTS for %s got stat error : %v", sac, statErr)
					}
				}

				// 6) Set SP - 2 SAC
				noSuratSP2, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP2_GENERATED")
				if err != nil {
					logrus.Errorf("Failed to increment nomor surat SP-2 for sac %s: %v", sac, err)
					return
				}
				var nomorSuratSP2Str string
				if noSuratSP2 < 1000 {
					nomorSuratSP2Str = fmt.Sprintf("%03d", noSuratSP2)
				} else {
					nomorSuratSP2Str = fmt.Sprintf("%d", noSuratSP2)
				}

				// SP - 2 sac placeholders for pdf replacements
				pelanggaranSP2SACID := fmt.Sprintf("Kurangnya pengawasan dan pembinaan terhadap teknisi %s, sehingga teknisi tersebut melakukan pelanggaran yang berujung pada penerbitan Surat Peringatan (SP-3) pada %v.", namaTeknisi, tanggalIndoFormatted)
				placeholdersSP2SAC := map[string]string{
					"$nomor_surat":            nomorSuratSP2Str,
					"$bulan_romawi":           monthRoman,
					"$tahun_sp":               tahunSP,
					"$nama_sac":               namaSAC,
					"$jabatan_sac":            fmt.Sprintf("Service Area Coordinator - Region %d", SACDataTechnician.Region),
					"$pelanggaran_karyawan":   pelanggaranSP2SACID,
					"$nama_teknisi":           namaTeknisi,
					"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
					"$personalia_name":        hrdPersonaliaName,
					"$personalia_ttd":         hrdTTDPath,
					"$personalia_phone":       hrdPhoneNumber,
					"$sac_name":               SACDataTechnician.FullName,
					"$sac_ttd":                SACDataTechnician.TTDPath,
					"$record_sac":             sac,
					"$for_project":            forProject,
				}
				pdfSP2FilenameSAC := fmt.Sprintf("SP_2_%s_%s.pdf", strings.ReplaceAll(sac, "*", resignTechnicianReplacer), now.Format("2006-01-02"))
				pdfSP2SACFilePath := filepath.Join(pdfDirForSPSAC, pdfSP2FilenameSAC)

				if err := CreatePDFSP2ForSAC(placeholdersSP2SAC, pdfSP2SACFilePath); err != nil {
					logrus.Errorf("Failed to create PDF SP-2 for SAC %s: %v", sac, err)
					return
				}

				spForSACIsProcessing = true // Mark sp 2 already proceed for today
				sacGotSP2At := time.Now()
				dataSP2SAC := sptechnicianmodel.SACGotSP{
					IsGotSP2:                   true,
					GotSP2At:                   &sacGotSP2At,
					TechnicianNameCausedGotSP2: record.Technician,
					NoSP2:                      noSuratSP2,
					PelanggaranSP2:             pelanggaranSP2SACID,
					SP2SoundTTSPath:            fileTTSSP2SAC,
					SP2FilePath:                pdfSP2SACFilePath,
				}

				if err := dbWeb.Where("for_project = ? AND sac = ? AND is_got_sp1 = ?", forProject, sac, true).
					Updates(&dataSP2SAC).Error; err != nil {
					logrus.Errorf("failed to update the sp - 2 of sac %s : %v", sac, err)
					return
				}

				if _, exists := needToSendTheSPSACThroughWhatsapp[sac]; !exists {
					needToSendTheSPSACThroughWhatsapp[sac] = 2
				}
				logrus.Infof("SP - 2 of SAC %s successfully created", sac)
			} // .end of no on-time whatsapp sp-1 replied
		} // .end of check sac already got the sp - 1

		// Try check sac status if he will got the sp - 3
		if sacGotSP1 && sacGotSP2 && !sacGotSP3 {
			// 1) First get the sp-2 sent at time
			sp2SentAt := dataSPSAC.GotSP2At
			if sp2SentAt == nil {
				// Try to find the sp - 2 sent at using the first whatsapp message sent
				var firstSP2SACMsg sptechnicianmodel.SPWhatsAppMessage
				if err := dbWeb.Where("sac_got_sp_id = ? AND number_of_sp = ?", dataSPSAC.ID, 2).
					Order("whatsapp_sent_at asc").
					First(&firstSP2SACMsg).
					Error; err != nil {
					logrus.Errorf("could not find the first sp-2 msg of sac %s to determine sent time : %v", sac, err)
					return
				}
				if firstSP2SACMsg.WhatsappSentAt == nil {
					logrus.Errorf("SP-2 whatsapp msg sent time is nil for sac %s, cannot continue to create the sp-3 for sac", sac)
					return
				}
				sp2SentAt = firstSP2SACMsg.WhatsappSentAt
			}
			// 2) Calculate the sp - 2 reply deadline
			deadlineSP2 := time.Date(sp2SentAt.Year(), sp2SentAt.Month(), sp2SentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sp2SentAt.Location())
			// 3) Count replies received before or equal to deadline
			var onTimeSP2RepliedCount int64
			dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
				Where("sac_got_sp_id = ?", dataSPSAC.ID).
				Where("number_of_sp = ?", 2).
				Where("for_project = ?", forProject).
				Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
				Where("whatsapp_replied_at <= ?", deadlineSP2).
				Count(&onTimeSP2RepliedCount)
				// 4) Proceed only if no on-time replies were found
			if onTimeSP2RepliedCount == 0 {
				// Check if the technician name caused sac got sp2 is same to current technician looping
				if dataSPSAC.TechnicianNameCausedGotSP1 == record.Technician || dataSPSAC.TechnicianNameCausedGotSP2 == record.Technician {
					logrus.Warnf("SAC %s already got SP-1 or SP-2 caused by technician: %s, skipping SP-3 issuance", sac, record.Technician)
					return
				}

				// Check if the SAC already get the sp today, so it will be skipped
				if val, ok := needToSendTheSPSACThroughWhatsapp[sac]; ok && val == 2 {
					logrus.Warnf("SAC %s already got SP-2 today, skipping SP-3 issuance", sac)
					return
				}

				// Check if the sp - 2 filepath already generated so skip the sp - 3 created
				if dataSPSAC.SP3FilePath != "" {
					logrus.Warnf("SAC %s sp-3 already generated, skipping SP-3 issuance", sac)
					return
				}

				var tglSP2SACTerkirimFormatted string
				tglSP2SACTerkirim, err := tanggal.Papar(*sp2SentAt, "Jakarta", tanggal.WIB)
				if err != nil {
					logrus.Errorf("failed to format date of tgl sp - 2 terkirim of sac %s : %v", sac, err)
					return
				}
				tglSP2SACTerkirimFormatted = tglSP2SACTerkirim.Format(" ", []tanggal.Format{
					tanggal.NamaHariDenganKoma,
					tanggal.Hari,
					tanggal.NamaBulan,
					tanggal.Tahun,
					tanggal.PukulDenganDetik,
					tanggal.ZonaWaktu,
				})

				// 5) Build text to sound for sp - 3 sac
				SP3SACTextPart1 := "Merujuk pada Surat Peringatan (SP-2) yang telah disampaikan"
				SP3SACTextPart2 := fmt.Sprintf(" kepada Saudara %s", namaSAC)
				SP3SACTextPart3 := fmt.Sprintf("pada tanggal %s.", tglSP2SACTerkirimFormatted)
				SP3SACTextPart4 := "perusahaan menilai bahwa pelanggaran yang saudara lakukan tidak kunjung diperbaiki."
				SP3SACTextPart5 := "Hal ini menunjukkan sikap yang tidak responsif terhadap peringatan yang telah diberikan."
				SP3SACTextPart6 := fmt.Sprintf("Selain itu, teknisi %s", namaTeknisi)
				SP3SACTextPart7 := " di bawah pengawasan Saudara"
				SP3SACTextPart8 := "kembali melakukan pelanggaran yang berujung pada penerbitan SP-3."
				SP3SACTextPart9 := "Dengan demikian, perusahaan dengan berat hati menerbitkan SP-3 untuk Saudara."
				SP3SACTextPart10 := "Surat ini juga menyatakan berakhirnya hubungan kerja Saudara dengan perusahaan."
				SP3SACTextPart11 := "Keputusan berlaku efektif sejak tanggal diterbitkannya surat ini."
				SP3SACTextPart12 := fmt.Sprintf("yakni pada tanggal %s.", tanggalIndoFormatted)
				SP3SACTextPart13 := "Kami mengucapkan terima kasih atas kontribusi Saudara selama ini."
				SP3SACTextPart14 := "Semoga Saudara mendapatkan kesuksesan di masa depan. terima kasih..."

				sp3SACFilenameSound := fmt.Sprintf("%s_SP3_SAC", strings.ReplaceAll(sac, "*", resignTechnicianReplacer))
				fileTTSSP3SAC, err := fun.CreateRobustTTS(speech, audioDirForSPSAC, []string{
					SP3SACTextPart1,
					SP3SACTextPart2,
					SP3SACTextPart3,
					SP3SACTextPart4,
					SP3SACTextPart5,
					SP3SACTextPart6,
					SP3SACTextPart7,
					SP3SACTextPart8,
					SP3SACTextPart9,
					SP3SACTextPart10,
					SP3SACTextPart11,
					SP3SACTextPart12,
					SP3SACTextPart13,
					SP3SACTextPart14,
				}, sp3SACFilenameSound)
				if err != nil {
					logrus.Errorf("Failed to create merged SP-3 TTS file for sac %s : %v", sac, err)
					return
				}

				if fileTTSSP3SAC != "" {
					fileInfo, statErr := os.Stat(fileTTSSP3SAC)
					if statErr == nil {
						logrus.Debugf("🔊 SP-3 merged TTS for %s - %s, Size: %d bytes", sac, fileTTSSP3SAC, fileInfo.Size())
					} else {
						logrus.Errorf("🔇 SP-3 TTS for %s got stat error : %v", sac, statErr)
					}
				}

				// 6) Set SP - 3 SAC
				noSuratSP3, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP3_GENERATED")
				if err != nil {
					logrus.Errorf("Failed to increment nomor surat SP-3 for sac %s: %v", sac, err)
					return
				}
				var nomorSuratSP3Str string
				if noSuratSP3 < 1000 {
					nomorSuratSP3Str = fmt.Sprintf("%03d", noSuratSP3)
				} else {
					nomorSuratSP3Str = fmt.Sprintf("%d", noSuratSP3)
				}

				// SP - 3 sac placeholders for pdf replacements
				pelanggaranSP3SACID := fmt.Sprintf("Kurangnya pengawasan dan pembinaan terhadap teknisi %s, sehingga teknisi tersebut melakukan pelanggaran yang berujung pada penerbitan Surat Peringatan (SP-3) pada %v.", namaTeknisi, tanggalIndoFormatted)
				placeholdersSP3SAC := map[string]string{
					"$nomor_surat":            nomorSuratSP3Str,
					"$bulan_romawi":           monthRoman,
					"$tahun_sp":               tahunSP,
					"$nama_sac":               namaSAC,
					"$jabatan_sac":            fmt.Sprintf("Service Area Coordinator - Region %d", SACDataTechnician.Region),
					"$pelanggaran_karyawan":   pelanggaranSP3SACID,
					"$nama_teknisi":           namaTeknisi,
					"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
					"$personalia_name":        hrdPersonaliaName,
					"$personalia_ttd":         hrdTTDPath,
					"$personalia_phone":       hrdPhoneNumber,
					"$sac_name":               SACDataTechnician.FullName,
					"$sac_ttd":                SACDataTechnician.TTDPath,
					"$record_sac":             sac,
					"$for_project":            forProject,
				}
				pdfSP3FilenameSAC := fmt.Sprintf("SP_3_%s_%s.pdf", strings.ReplaceAll(sac, "*", resignTechnicianReplacer), now.Format("2006-01-02"))
				pdfSP3SACFilePath := filepath.Join(pdfDirForSPSAC, pdfSP3FilenameSAC)

				if err := CreatePDFSP3ForSAC(placeholdersSP3SAC, pdfSP3SACFilePath); err != nil {
					logrus.Errorf("Failed to create PDF SP-3 for SAC %s: %v", sac, err)
					return
				}

				spForSACIsProcessing = true // Mark sp 3 already proceed for today
				sacGotSP3At := time.Now()
				dataSP3SAC := sptechnicianmodel.SACGotSP{
					IsGotSP3:                   true,
					GotSP3At:                   &sacGotSP3At,
					TechnicianNameCausedGotSP3: record.Technician,
					NoSP3:                      noSuratSP3,
					PelanggaranSP3:             pelanggaranSP3SACID,
					SP3SoundTTSPath:            fileTTSSP3SAC,
					SP3FilePath:                pdfSP3SACFilePath,
				}

				if err := dbWeb.Where("for_project = ? AND sac = ? AND is_got_sp2 = ?", forProject, sac, true).
					Updates(&dataSP3SAC).Error; err != nil {
					logrus.Errorf("failed to update the sp - 3 of sac %s : %v", sac, err)
					return
				}

				if _, exists := needToSendTheSPSACThroughWhatsapp[sac]; !exists {
					needToSendTheSPSACThroughWhatsapp[sac] = 3
				}
				logrus.Infof("SP - 3 of SAC %s successfully created", sac)
			} // .end of no on-time whatsapp sp-2 replied
		} // .end of check sac already got the sp - 2
	} // .end of sp for sac didnt proceed yet
}

// processSPForTechnicianNotSOWithExistingEDC processes the SP for a technician who has existing EDC but did not perform Stock Opname (SO).
// It generates the SP document (PDF) and audio (TTS), updates the database, and marks the technician for WhatsApp notification.
//
// Parameters:
//   - db: Database connection.
//   - forProject: Project identifier.
//   - hrdPersonaliaName: Name of HRD Personalia.
//   - hrdTTDPath: Path to HRD signature image.
//   - hrdPhoneNumber: HRD phone number.
//   - technician: Technician identifier.
//   - namaTeknisi: Technician name.
//   - namaSPL: SPL name.
//   - splCity: SPL city.
//   - SACDataTechnician: SAC data for the technician.
//   - resignTechnicianReplacer: String to replace resigned technician name.
//   - audioDirForSPTechnician: Directory for SP audio files.
//   - pdfDirForSPTechnician: Directory for SP PDF files.
//   - needToSendTheSPTechnicianThroughWhatsapp: Map to track WhatsApp notifications.
//   - productsEDCCSNAExists: Map of existing EDC products per company.
//
// Returns:
//   - error: Error if any step fails.
func processSPForTechnicianNotSOWithExistingEDC(
	db *gorm.DB,
	forProject string,
	hrdPersonaliaName string,
	hrdTTDPath string,
	hrdPhoneNumber string,
	technician string,
	namaTeknisi string,
	namaSPL string,
	splCity string,
	SACDataTechnician config.SACODOOMS,
	resignTechnicianReplacer string,
	audioDirForSPTechnician string,
	pdfDirForSPTechnician string,
	needToSendTheSPTechnicianThroughWhatsapp map[string]int,
	productsEDCCSNAExists map[string][]string,
) error {
	speech := htgotts.Speech{Folder: audioDirForSPTechnician, Language: voices.Indonesian, Handler: &handlers.Native{}}
	tahunSP := time.Now().Format("2006")
	monthRoman, err := fun.MonthToRoman(int(time.Now().Month()))
	if err != nil {
		return fmt.Errorf("failed to convert month to roman numeral: %v", err)
	}

	var tanggalIndoFormatted string
	tgl, err := tanggal.Papar(time.Now(), "Jakarta", tanggal.WIB)
	if err != nil {
		return fmt.Errorf("failed to format date indo : %v", err)
	}
	tanggalIndoFormatted = tgl.Format(" ", []tanggal.Format{
		tanggal.Hari,
		tanggal.NamaBulan,
		tanggal.Tahun,
	})

	var pelanggaranID string
	if len(productsEDCCSNAExists) == 0 {
		return errors.New("no data found for existing product EDC CSNA")
	}
	var sb strings.Builder
	sb.WriteString("tidak melakukan Stock Opname untuk: ")
	for company, edcList := range productsEDCCSNAExists {
		sb.WriteString(fmt.Sprintf("%s (%d EDC); ", company, len(edcList)))
	}
	pelanggaranID = sb.String()

	spNumber, exists := needToSendTheSPTechnicianThroughWhatsapp[technician]
	if !exists {
		// Create sound sp 1 for technician
		SP1TechnicianTextPart1 := "Berikut kami sampaikan bahwa "
		SP1TechnicianTextPart2 := fmt.Sprintf(" saudara %s.", namaTeknisi)
		SP1TechnicianTextPart3 := "Menerima Surat Peringatan (SP-1)."
		SP1TechnicianTextPart4 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan."
		SP1TechnicianTextPart5 := "terima kasih..."
		sp1TechFilenameSound := fmt.Sprintf("%s_SP1", strings.ReplaceAll(technician, "*", resignTechnicianReplacer))
		fileTTSSP1Technician, err := fun.CreateRobustTTS(speech, audioDirForSPTechnician, []string{
			SP1TechnicianTextPart1,
			SP1TechnicianTextPart2,
			SP1TechnicianTextPart3,
			SP1TechnicianTextPart4,
			SP1TechnicianTextPart5,
		}, sp1TechFilenameSound)
		if err != nil {
			return fmt.Errorf("failed to create merged SP1 TTS file for technician %s : %v", technician, err)
		}

		if fileTTSSP1Technician != "" {
			fileInfo, statErr := os.Stat(fileTTSSP1Technician)
			if statErr == nil {
				logrus.Debugf("🔊 SP-1 merged TTS for %s - %s, Size: %d bytes", technician, fileTTSSP1Technician, fileInfo.Size())
			} else {
				logrus.Errorf("🔇 SP-1 TTS for %s got stat error : %v", technician, statErr)
			}
		}

		// Set SP - 1
		noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
		if err != nil {
			return fmt.Errorf("Failed to increment nomor surat SP-1 for technician %s : %v", technician, err)
		}
		var nomorSuratSP1Str string
		if noSuratSP1 < 1000 {
			nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
		} else {
			nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
		}

		// Placeholder for replace data in pdf SP - 1 Technician
		placeholderSP1Teknisi := map[string]string{
			"$nomor_surat":            nomorSuratSP1Str,
			"$bulan_romawi":           monthRoman,
			"$tahun_sp":               tahunSP,
			"$nama_spl":               namaSPL,
			"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
			"$pelanggaran_karyawan":   pelanggaranID,
			"$nama_teknisi":           namaTeknisi,
			"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
			"$personalia_name":        hrdPersonaliaName,
			"$personalia_ttd":         hrdTTDPath,
			"$personalia_phone":       hrdPhoneNumber,
			"$sac_name":               SACDataTechnician.FullName,
			"$sac_ttd":                SACDataTechnician.TTDPath,
		}
		pdfSP1FilenameTechnician := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(technician, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
		pdfSP1TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP1FilenameTechnician)

		if err := CreatePDFSP1ForTechnician(placeholderSP1Teknisi, pdfSP1TechnicianFilePath); err != nil {
			return fmt.Errorf("failed to create the pdf for sp - 1 technician %s : %v", technician, err)
		}

		technicianGotSP1At := time.Now()
		pelanggaranID = fun.CapitalizeFirstWord(pelanggaranID)
		dataSP1Technician := sptechnicianmodel.TechnicianGotSP{
			Technician:      technician,
			Name:            namaTeknisi,
			ForProject:      forProject,
			IsGotSP1:        true,
			GotSP1At:        &technicianGotSP1At,
			NoSP1:           noSuratSP1,
			PelanggaranSP1:  pelanggaranID,
			SP1SoundTTSPath: fileTTSSP1Technician,
			SP1FilePath:     pdfSP1TechnicianFilePath,
		}

		if err := db.Create(&dataSP1Technician).Error; err != nil {
			return fmt.Errorf("failed to create the SP - 1 for technician %s : %v", technician, err)
		}

		needToSendTheSPTechnicianThroughWhatsapp[technician] = 1
		logrus.Infof("SP - 1 of Technician %s successfully generated", technician)
		return nil
	} else {
		pelanggaranID = fun.CapitalizeFirstWord("serta " + pelanggaranID)
		var dataSPTeknisi sptechnicianmodel.TechnicianGotSP
		if err := db.Where("for_project = ?", forProject).
			Where("technician = ?", technician).
			First(&dataSPTeknisi).Error; err != nil {
			return fmt.Errorf("failed to find the sp data of technician %s : %v", technician, err)
		}

		switch spNumber {
		case 1:
			// Set SP - 1
			noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-1 for technician %s : %v", technician, err)
			}
			var nomorSuratSP1Str string
			if noSuratSP1 < 1000 {
				nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
			} else {
				nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
			}

			pelanggaranSP1 := dataSPTeknisi.PelanggaranSP1 + "; " + pelanggaranID

			// Placeholder for replace data in pdf SP - 1 Technician
			placeholderSP1Teknisi := map[string]string{
				"$nomor_surat":            nomorSuratSP1Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP1,
				"$nama_teknisi":           namaTeknisi,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACDataTechnician.FullName,
				"$sac_ttd":                SACDataTechnician.TTDPath,
			}
			pdfSP1FilenameTechnician := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(technician, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP1TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP1FilenameTechnician)

			if err := CreatePDFSP1ForTechnician(placeholderSP1Teknisi, pdfSP1TechnicianFilePath); err != nil {
				return fmt.Errorf("failed to create the pdf for sp - 1 technician %s : %v", technician, err)
			}

			technicianGotSP1At := time.Now()
			dataSP1TechnicianUpdated := sptechnicianmodel.TechnicianGotSP{
				IsGotSP1:       true,
				GotSP1At:       &technicianGotSP1At,
				NoSP1:          noSuratSP1,
				PelanggaranSP1: pelanggaranID,
				SP1FilePath:    pdfSP1TechnicianFilePath,
			}

			if err := db.Where("technician = ?", technician).Updates(&dataSP1TechnicianUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 1 for technician %s : %v", technician, err)
			}

			needToSendTheSPTechnicianThroughWhatsapp[technician] = 1
			logrus.Infof("SP - 1 of Technician %s successfully updated", technician)
			return nil
		case 2:
			// Set SP-2
			noSuratSP2, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP2_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-2 for technician %s: %v", technician, err)
			}
			var nomorSuratSP2Str string
			if noSuratSP2 < 1000 {
				nomorSuratSP2Str = fmt.Sprintf("%03d", noSuratSP2)
			} else {
				nomorSuratSP2Str = fmt.Sprintf("%d", noSuratSP2)
			}

			pelanggaranSP2 := dataSPTeknisi.PelanggaranSP2 + "; " + pelanggaranID

			// SP - 2 placeholder for pdf replacements
			placeholderSP2Teknisi := map[string]string{
				"$nomor_surat":            nomorSuratSP2Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP2,
				"$nama_teknisi":           namaTeknisi,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACDataTechnician.FullName,
				"$sac_ttd":                SACDataTechnician.TTDPath,
				"$record_technician":      technician,
				"$for_project":            forProject,
			}
			pdfSP2FilenameTechnician := fmt.Sprintf("SP_2_%s_%s.pdf", strings.ReplaceAll(technician, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP2TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP2FilenameTechnician)

			if err := CreatePDFSP2ForTechnician(placeholderSP2Teknisi, pdfSP2TechnicianFilePath); err != nil {
				return fmt.Errorf("failed to create the pdf for sp - 2 technician %s : %v", technician, err)
			}

			technicianGotSP2At := time.Now()
			dataSP2TechnicianUpdated := sptechnicianmodel.TechnicianGotSP{
				IsGotSP2:       true,
				GotSP2At:       &technicianGotSP2At,
				NoSP2:          noSuratSP2,
				PelanggaranSP2: pelanggaranSP2,
				SP2FilePath:    pdfSP2TechnicianFilePath,
			}

			if err := db.Where("technician = ?", technician).Updates(&dataSP2TechnicianUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 2 for technician %s : %v", technician, err)
			}

			needToSendTheSPTechnicianThroughWhatsapp[technician] = 2
			logrus.Infof("SP - 2 of Technician %s successfully updated", technician)
			return nil
		case 3:
			// Set SP-3
			noSuratSP3, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP3_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-3 for technician %s: %v", technician, err)
			}
			var nomorSuratSP3Str string
			if noSuratSP3 < 1000 {
				nomorSuratSP3Str = fmt.Sprintf("%03d", noSuratSP3)
			} else {
				nomorSuratSP3Str = fmt.Sprintf("%d", noSuratSP3)
			}

			pelanggaranSP3 := dataSPTeknisi.PelanggaranSP3 + "; " + pelanggaranID

			// Make placeholder for replacements in SP - 3 pdf
			placeholdersSP3Teknisi := map[string]string{
				"$nomor_surat":            nomorSuratSP3Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP3,
				"$nama_teknisi":           namaTeknisi,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACDataTechnician.FullName,
				"$sac_ttd":                SACDataTechnician.TTDPath,
				"$record_technician":      technician,
				"$for_project":            forProject,
			}
			pdfSP3FilenameTechnician := fmt.Sprintf("SP_3_%s_%s.pdf", strings.ReplaceAll(technician, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP3TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP3FilenameTechnician)
			if err := CreatePDFSP3ForTechnician(placeholdersSP3Teknisi, pdfSP3TechnicianFilePath); err != nil {
				return fmt.Errorf("failed to create the pdf for sp - 3 technician %s : %v", technician, err)
			}

			technicianGotSP3At := time.Now()
			dataSP3TechnicianUpdated := sptechnicianmodel.TechnicianGotSP{
				IsGotSP3:       true,
				GotSP3At:       &technicianGotSP3At,
				NoSP3:          noSuratSP3,
				PelanggaranSP3: pelanggaranSP3,
				SP3FilePath:    pdfSP3TechnicianFilePath,
			}

			if err := db.Where("technician = ?", technician).Updates(&dataSP3TechnicianUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 3 for technician %s : %v", technician, err)
			}

			needToSendTheSPTechnicianThroughWhatsapp[technician] = 3
			logrus.Infof("SP - 3 of Technician %s successfully updated", technician)
			return nil
		default:
			return fmt.Errorf("invalid sp number %d for technician %s", spNumber, technician)
		}
	}
}

// processSPForSPLNotSOWithExistingEDC processes the SP for an SPL who has existing EDC but did not perform Stock Opname (SO).
// It generates the SP document (PDF) and audio (TTS), updates the database, and marks the SPL for WhatsApp notification.
//
// Parameters:
//   - db: Database connection.
//   - forProject: Project identifier.
//   - hrdPersonaliaName: Name of HRD Personalia.
//   - hrdTTDPath: Path to HRD signature image.
//   - hrdPhoneNumber: phone number that HRD used
//   - spl: SPL identifier.
//   - namaSPL: SPL name.
//   - splCity: SPL city.
//   - SACData: SAC data.
//   - resignTechnicianReplacer: String to replace resigned technician name.
//   - audioDirForSPSPL: Directory for SP audio files.
//   - pdfDirForSPSPL: Directory for SP PDF files.
//   - needToSendTheSPSPLThroughWhatsapp: Map to track WhatsApp notifications.
//   - productsEDCCSNAExists: Map of existing EDC products per company.
//
// Returns:
//   - error: Error if any step fails.
func processSPForSPLNotSOWithExistingEDC(
	db *gorm.DB,
	forProject string,
	hrdPersonaliaName string,
	hrdTTDPath string,
	hrdPhoneNumber string,
	spl string,
	namaSPL string,
	splCity string,
	SACData config.SACODOOMS,
	resignTechnicianReplacer string,
	audioDirForSPSPL string,
	pdfDirForSPSPL string,
	needToSendTheSPSPLThroughWhatsapp map[string]int,
	productsEDCCSNAExists map[string][]string,
) error {
	speech := htgotts.Speech{Folder: audioDirForSPSPL, Language: voices.Indonesian, Handler: &handlers.Native{}}
	tahunSP := time.Now().Format("2006")
	monthRoman, err := fun.MonthToRoman(int(time.Now().Month()))
	if err != nil {
		return fmt.Errorf("failed to convert month to roman numeral: %v", err)
	}

	var tanggalIndoFormatted string
	tgl, err := tanggal.Papar(time.Now(), "Jakarta", tanggal.WIB)
	if err != nil {
		return fmt.Errorf("failed to format date indo : %v", err)
	}
	tanggalIndoFormatted = tgl.Format(" ", []tanggal.Format{
		tanggal.Hari,
		tanggal.NamaBulan,
		tanggal.Tahun,
	})

	var pelanggaranID string
	if len(productsEDCCSNAExists) == 0 {
		return errors.New("no data found for existing product EDC CSNA")
	}
	var sb strings.Builder
	sb.WriteString("tidak melakukan Stock Opname untuk: ")
	for company, edcList := range productsEDCCSNAExists {
		sb.WriteString(fmt.Sprintf("%s (%d EDC); ", company, len(edcList)))
	}
	pelanggaranID = sb.String()

	spNumber, exists := needToSendTheSPSPLThroughWhatsapp[spl]
	if !exists {
		// Create sound sp 1 for SPL
		SP1SPLTextPart1 := "Berikut kami sampaikan bahwa "
		SP1SPLTextPart2 := fmt.Sprintf(" saudara %s.", namaSPL)
		SP1SPLTextPart3 := "Menerima Surat Peringatan (SP-1)."
		SP1SPLTextPart4 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan."
		SP1SPLTextPart5 := "terima kasih..."
		sp1SPLFilenameSound := fmt.Sprintf("%s_SP1_SPL", strings.ReplaceAll(spl, "*", resignTechnicianReplacer))
		fileTTSSP1SPL, err := fun.CreateRobustTTS(speech, audioDirForSPSPL, []string{
			SP1SPLTextPart1,
			SP1SPLTextPart2,
			SP1SPLTextPart3,
			SP1SPLTextPart4,
			SP1SPLTextPart5,
		}, sp1SPLFilenameSound)
		if err != nil {
			return fmt.Errorf("failed to create merged SP1 TTS file for spl %s : %v", spl, err)
		}

		if fileTTSSP1SPL != "" {
			fileInfo, statErr := os.Stat(fileTTSSP1SPL)
			if statErr == nil {
				logrus.Debugf("🔊 SP-1 merged TTS for %s - %s, Size: %d bytes", spl, fileTTSSP1SPL, fileInfo.Size())
			} else {
				logrus.Errorf("🔇 SP-1 TTS for %s got stat error : %v", spl, statErr)
			}
		}

		// Set SP - 1
		noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
		if err != nil {
			return fmt.Errorf("Failed to increment nomor surat SP-1 for spl %s : %v", spl, err)
		}
		var nomorSuratSP1Str string
		if noSuratSP1 < 1000 {
			nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
		} else {
			nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
		}

		pelanggaranID = fun.CapitalizeFirstWord(pelanggaranID)

		placeholderSP1SPL := map[string]string{
			"$nomor_surat":            nomorSuratSP1Str,
			"$bulan_romawi":           monthRoman,
			"$tahun_sp":               tahunSP,
			"$nama_spl":               namaSPL,
			"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
			"$pelanggaran_karyawan":   pelanggaranID,
			"$nama_teknisi":           namaSPL,
			"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
			"$personalia_name":        hrdPersonaliaName,
			"$personalia_ttd":         hrdTTDPath,
			"$personalia_phone":       hrdPhoneNumber,
			"$sac_name":               SACData.FullName,
			"$sac_ttd":                SACData.TTDPath,
			"$record_spl":             spl,
			"$for_project":            forProject,
		}
		pdfSP1FilenameSPL := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
		pdfSP1SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP1FilenameSPL)

		if err := CreatePDFSP1ForSPL(placeholderSP1SPL, pdfSP1SPLFilePath); err != nil {
			return fmt.Errorf("failed to generate pdf of sp 1 spl %s : %v", spl, err)
		}

		splGotSP1At := time.Now()
		dataSP1SPL := sptechnicianmodel.SPLGotSP{
			SPL:                        spl,
			Name:                       namaSPL,
			ForProject:                 forProject,
			IsGotSP1:                   true,
			GotSP1At:                   &splGotSP1At,
			TechnicianNameCausedGotSP1: "",
			NoSP1:                      noSuratSP1,
			PelanggaranSP1:             pelanggaranID,
			SP1SoundTTSPath:            fileTTSSP1SPL,
			SP1FilePath:                pdfSP1SPLFilePath,
		}
		if err := dbWeb.Create(&dataSP1SPL).Error; err != nil {
			return fmt.Errorf("failed to create the SP - 1 for SPL %s : %v", spl, err)
		}
		needToSendTheSPSPLThroughWhatsapp[spl] = 1
		logrus.Infof("SP - 1 of SPL %s successfully created", spl)
		return nil
		// .end of spl got SP-1 coz still have EDC not SO
	} else {
		pelanggaranID = fun.CapitalizeFirstWord("serta " + pelanggaranID)
		var dataSPSPL sptechnicianmodel.SPLGotSP
		if err := db.Where("for_project = ?", forProject).
			Where("spl = ?", spl).
			First(&dataSPSPL).Error; err != nil {
			return fmt.Errorf("failed to find sp data of spl %s : %v", spl, err)
		}

		switch spNumber {
		case 1:
			// Set SP-1
			noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-1 for spl %s : %v", spl, err)
			}
			var nomorSuratSP1Str string
			if noSuratSP1 < 1000 {
				nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
			} else {
				nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
			}

			pelanggaranSP1 := dataSPSPL.PelanggaranSP1 + "; " + pelanggaranID

			placeholderSP1SPL := map[string]string{
				"$nomor_surat":            nomorSuratSP1Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP1,
				"$nama_teknisi":           namaSPL,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACData.FullName,
				"$sac_ttd":                SACData.TTDPath,
				"$record_spl":             spl,
				"$for_project":            forProject,
			}
			pdfSP1FilenameSPL := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP1SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP1FilenameSPL)

			if err := CreatePDFSP1ForSPL(placeholderSP1SPL, pdfSP1SPLFilePath); err != nil {
				return fmt.Errorf("failed to generate pdf of sp 1 spl %s : %v", spl, err)
			}

			splGotSP1At := time.Now()
			dataSP1SPLUpdated := sptechnicianmodel.SPLGotSP{
				IsGotSP1:       true,
				GotSP1At:       &splGotSP1At,
				NoSP1:          noSuratSP1,
				PelanggaranSP1: pelanggaranID,
				SP1FilePath:    pdfSP1SPLFilePath,
			}

			if err := db.Where("spl = ?", spl).Updates(&dataSP1SPLUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 1 for SPL %s : %v", spl, err)
			}

			needToSendTheSPSPLThroughWhatsapp[spl] = 1
			logrus.Infof("SP - 1 of SPL %s successfully updated", spl)
			return nil
		case 2:
			// Set SP-2
			noSuratSP2, err := IncrementNomorSuratSP(db, "LAST_NOMOR_SURAT_SP2_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-2 for spl %s : %v", spl, err)
			}
			var nomorSuratSP2Str string
			if noSuratSP2 < 1000 {
				nomorSuratSP2Str = fmt.Sprintf("%03d", noSuratSP2)
			} else {
				nomorSuratSP2Str = fmt.Sprintf("%d", noSuratSP2)
			}

			pelanggaranSP2 := dataSPSPL.PelanggaranSP2 + "; " + pelanggaranID

			placeholderSP2SPL := map[string]string{
				"$nomor_surat":            nomorSuratSP2Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP2,
				"$nama_teknisi":           namaSPL,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACData.FullName,
				"$sac_ttd":                SACData.TTDPath,
				"$record_spl":             spl,
				"$for_project":            forProject,
			}
			pdfSP2FilenameSPL := fmt.Sprintf("SP_2_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP2SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP2FilenameSPL)

			if err := CreatePDFSP2ForSPL(placeholderSP2SPL, pdfSP2SPLFilePath); err != nil {
				return fmt.Errorf("failed to generate pdf of sp 2 spl %s : %v", spl, err)
			}

			splGotSP2At := time.Now()
			dataSP2SPLUpdated := sptechnicianmodel.SPLGotSP{
				IsGotSP2:       true,
				GotSP2At:       &splGotSP2At,
				NoSP2:          noSuratSP2,
				PelanggaranSP2: pelanggaranSP2,
				SP2FilePath:    pdfSP2SPLFilePath,
			}

			if err := db.Where("spl = ?", spl).Updates(&dataSP2SPLUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 2 for SPL %s : %v", spl, err)
			}

			needToSendTheSPSPLThroughWhatsapp[spl] = 2
			logrus.Infof("SP - 2 of SPL %s successfully updated", spl)
			return nil
		case 3:
			// Set SP-3
			noSuratSP3, err := IncrementNomorSuratSP(db, "LAST_NOMOR_SURAT_SP3_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-3 for spl %s : %v", spl, err)
			}
			var nomorSuratSP3Str string
			if noSuratSP3 < 1000 {
				nomorSuratSP3Str = fmt.Sprintf("%03d", noSuratSP3)
			} else {
				nomorSuratSP3Str = fmt.Sprintf("%d", noSuratSP3)
			}

			pelanggaranSP3 := dataSPSPL.PelanggaranSP3 + "; " + pelanggaranID

			placeholderSP3SPL := map[string]string{
				"$nomor_surat":            nomorSuratSP3Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP3,
				"$nama_teknisi":           namaSPL,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACData.FullName,
				"$sac_ttd":                SACData.TTDPath,
				"$record_spl":             spl,
				"$for_project":            forProject,
			}
			pdfSP3FilenameSPL := fmt.Sprintf("SP_3_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP3SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP3FilenameSPL)

			if err := CreatePDFSP3ForSPL(placeholderSP3SPL, pdfSP3SPLFilePath); err != nil {
				return fmt.Errorf("failed to generate pdf of sp 3 spl %s : %v", spl, err)
			}

			splGotSP3At := time.Now()
			dataSP3SPLUpdated := sptechnicianmodel.SPLGotSP{
				IsGotSP3:       true,
				GotSP3At:       &splGotSP3At,
				NoSP3:          noSuratSP3,
				PelanggaranSP3: pelanggaranSP3,
				SP3FilePath:    pdfSP3SPLFilePath,
			}

			if err := db.Where("spl = ?", spl).Updates(&dataSP3SPLUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 3 for SPL %s : %v", spl, err)
			}

			needToSendTheSPSPLThroughWhatsapp[spl] = 3
			logrus.Infof("SP - 3 of SPL %s successfully updated", spl)
			return nil
		default:
			return fmt.Errorf("invalid sp number %d of SPL %s", spNumber, spl)
		}
	} // .end of sp spl updated coz still have EDC not SO
}

// processSPForTechnicianWithMissingEDCNotSO processes the SP for a technician who has missing EDC not SO.
// It generates the SP document (PDF) and audio (TTS), updates the database, and marks the technician for WhatsApp notification.
//
// Parameters:
//   - db: Database connection
//   - forProject: Project identifier
//   - hrdPersonaliaName: Name of HRD Personalia
//   - hrdTTDPath: Path to HRD signature image
//   - hrdPhoneNumber: phone number that HRD used
//   - technician: Technician identifier
//   - namaTeknisi: Technician name
//   - namaSPL: SPL name
//   - splCity: SPL city
//   - SACDataTechnician: SAC data for the technician
//   - resignTechnicianReplacer: String to replace resigned technician name
//   - audioDirForSPTechnician: Directory for SP audio files
//   - pdfDirForSPTechnician: Directory for SP PDF files
//   - needToSendTheSPTechnicianThroughWhatsapp: Map to track WhatsApp notifications
//   - missingEDCNotSO: Map of missing EDCs per company
//
// Returns:
//   - error: Error if any step fails
func processSPForTechnicianWithMissingEDCNotSO(
	db *gorm.DB,
	forProject string,
	hrdPersonaliaName string,
	hrdTTDPath string,
	hrdPhoneNumber string,
	technician string,
	namaTeknisi string,
	namaSPL string,
	splCity string,
	SACDataTechnician config.SACODOOMS,
	resignTechnicianReplacer string,
	audioDirForSPTechnician string,
	pdfDirForSPTechnician string,
	needToSendTheSPTechnicianThroughWhatsapp map[string]int,
	missingEDCNotSO map[string][]string,
) error {
	speech := htgotts.Speech{Folder: audioDirForSPTechnician, Language: voices.Indonesian, Handler: &handlers.Native{}}
	tahunSP := time.Now().Format("2006")
	monthRoman, err := fun.MonthToRoman(int(time.Now().Month()))
	if err != nil {
		return fmt.Errorf("failed to convert month to roman numeral: %v", err)
	}

	var tanggalIndoFormatted string
	tgl, err := tanggal.Papar(time.Now(), "Jakarta", tanggal.WIB)
	if err != nil {
		return fmt.Errorf("failed to format date indo : %v", err)
	}
	tanggalIndoFormatted = tgl.Format(" ", []tanggal.Format{
		tanggal.Hari,
		tanggal.NamaBulan,
		tanggal.Tahun,
	})

	var pelanggaranID string
	if len(missingEDCNotSO) == 0 {
		return errors.New("no data found for missing EDC not SO")
	}
	var sb strings.Builder
	sb.WriteString("terdapat EDC yang tidak di Stock Opname: ")
	for company, edcList := range missingEDCNotSO {
		sb.WriteString(fmt.Sprintf("%s (%d EDC); ", company, len(edcList)))
	}
	pelanggaranID = sb.String()

	spNumber, exists := needToSendTheSPTechnicianThroughWhatsapp[technician]
	if !exists {
		// Create sound sp 1 for technician
		SP1TechnicianTextPart1 := "Berikut kami sampaikan bahwa "
		SP1TechnicianTextPart2 := fmt.Sprintf(" saudara %s.", namaTeknisi)
		SP1TechnicianTextPart3 := "Menerima Surat Peringatan (SP-1)."
		SP1TechnicianTextPart4 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan."
		SP1TechnicianTextPart5 := "terima kasih..."
		sp1TechFilenameSound := fmt.Sprintf("%s_SP1", strings.ReplaceAll(technician, "*", resignTechnicianReplacer))
		fileTTSSP1Technician, err := fun.CreateRobustTTS(speech, audioDirForSPTechnician, []string{
			SP1TechnicianTextPart1,
			SP1TechnicianTextPart2,
			SP1TechnicianTextPart3,
			SP1TechnicianTextPart4,
			SP1TechnicianTextPart5,
		}, sp1TechFilenameSound)
		if err != nil {
			return fmt.Errorf("failed to create merged SP1 TTS file for technician %s : %v", technician, err)
		}

		if fileTTSSP1Technician != "" {
			fileInfo, statErr := os.Stat(fileTTSSP1Technician)
			if statErr == nil {
				logrus.Debugf("🔊 SP-1 merged TTS for %s - %s, Size: %d bytes", technician, fileTTSSP1Technician, fileInfo.Size())
			} else {
				logrus.Errorf("🔇 SP-1 TTS for %s got stat error : %v", technician, statErr)
			}
		}

		// Set SP - 1
		noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
		if err != nil {
			return fmt.Errorf("Failed to increment nomor surat SP-1 for technician %s : %v", technician, err)
		}
		var nomorSuratSP1Str string
		if noSuratSP1 < 1000 {
			nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
		} else {
			nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
		}

		// Placeholder for replace data in pdf SP - 1 Technician
		placeholderSP1Teknisi := map[string]string{
			"$nomor_surat":            nomorSuratSP1Str,
			"$bulan_romawi":           monthRoman,
			"$tahun_sp":               tahunSP,
			"$nama_spl":               namaSPL,
			"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
			"$pelanggaran_karyawan":   pelanggaranID,
			"$nama_teknisi":           namaTeknisi,
			"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
			"$personalia_name":        hrdPersonaliaName,
			"$personalia_ttd":         hrdTTDPath,
			"$personalia_phone":       hrdPhoneNumber,
			"$sac_name":               SACDataTechnician.FullName,
			"$sac_ttd":                SACDataTechnician.TTDPath,
		}
		pdfSP1FilenameTechnician := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(technician, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
		pdfSP1TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP1FilenameTechnician)

		if err := CreatePDFSP1ForTechnician(placeholderSP1Teknisi, pdfSP1TechnicianFilePath); err != nil {
			return fmt.Errorf("failed to create the pdf for sp - 1 technician %s : %v", technician, err)
		}

		technicianGotSP1At := time.Now()
		pelanggaranID = fun.CapitalizeFirstWord(pelanggaranID)
		dataSP1Technician := sptechnicianmodel.TechnicianGotSP{
			Technician:      technician,
			Name:            namaTeknisi,
			ForProject:      forProject,
			IsGotSP1:        true,
			GotSP1At:        &technicianGotSP1At,
			NoSP1:           noSuratSP1,
			PelanggaranSP1:  pelanggaranID,
			SP1SoundTTSPath: fileTTSSP1Technician,
			SP1FilePath:     pdfSP1TechnicianFilePath,
		}

		if err := db.Create(&dataSP1Technician).Error; err != nil {
			return fmt.Errorf("failed to create the SP - 1 for technician %s : %v", technician, err)
		}

		needToSendTheSPTechnicianThroughWhatsapp[technician] = 1
		logrus.Infof("SP - 1 of Technician %s successfully generated", technician)
		return nil
	} else {
		pelanggaranID = fun.CapitalizeFirstWord("serta " + pelanggaranID)
		var dataSPTeknisi sptechnicianmodel.TechnicianGotSP
		if err := db.Where("for_project = ?", forProject).
			Where("technician = ?", technician).
			First(&dataSPTeknisi).Error; err != nil {
			return fmt.Errorf("failed to find the sp data of technician %s : %v", technician, err)
		}

		switch spNumber {
		case 1:
			// Set SP - 1
			noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-1 for technician %s : %v", technician, err)
			}
			var nomorSuratSP1Str string
			if noSuratSP1 < 1000 {
				nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
			} else {
				nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
			}

			pelanggaranSP1 := dataSPTeknisi.PelanggaranSP1 + "; " + pelanggaranID

			// Placeholder for replace data in pdf SP - 1 Technician
			placeholderSP1Teknisi := map[string]string{
				"$nomor_surat":            nomorSuratSP1Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP1,
				"$nama_teknisi":           namaTeknisi,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACDataTechnician.FullName,
				"$sac_ttd":                SACDataTechnician.TTDPath,
			}
			pdfSP1FilenameTechnician := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(technician, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP1TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP1FilenameTechnician)

			if err := CreatePDFSP1ForTechnician(placeholderSP1Teknisi, pdfSP1TechnicianFilePath); err != nil {
				return fmt.Errorf("failed to create the pdf for sp - 1 technician %s : %v", technician, err)
			}

			technicianGotSP1At := time.Now()
			dataSP1TechnicianUpdated := sptechnicianmodel.TechnicianGotSP{
				IsGotSP1:       true,
				GotSP1At:       &technicianGotSP1At,
				NoSP1:          noSuratSP1,
				PelanggaranSP1: pelanggaranID,
				SP1FilePath:    pdfSP1TechnicianFilePath,
			}

			if err := db.Where("technician = ?", technician).Updates(&dataSP1TechnicianUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 1 for technician %s : %v", technician, err)
			}

			needToSendTheSPTechnicianThroughWhatsapp[technician] = 1
			logrus.Infof("SP - 1 of Technician %s successfully updated", technician)
			return nil
		case 2:
			// Set SP-2
			noSuratSP2, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP2_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-2 for technician %s: %v", technician, err)
			}
			var nomorSuratSP2Str string
			if noSuratSP2 < 1000 {
				nomorSuratSP2Str = fmt.Sprintf("%03d", noSuratSP2)
			} else {
				nomorSuratSP2Str = fmt.Sprintf("%d", noSuratSP2)
			}

			pelanggaranSP2 := dataSPTeknisi.PelanggaranSP2 + "; " + pelanggaranID

			// SP - 2 placeholder for pdf replacements
			placeholderSP2Teknisi := map[string]string{
				"$nomor_surat":            nomorSuratSP2Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP2,
				"$nama_teknisi":           namaTeknisi,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACDataTechnician.FullName,
				"$sac_ttd":                SACDataTechnician.TTDPath,
				"$record_technician":      technician,
				"$for_project":            forProject,
			}
			pdfSP2FilenameTechnician := fmt.Sprintf("SP_2_%s_%s.pdf", strings.ReplaceAll(technician, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP2TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP2FilenameTechnician)

			if err := CreatePDFSP2ForTechnician(placeholderSP2Teknisi, pdfSP2TechnicianFilePath); err != nil {
				return fmt.Errorf("failed to create the pdf for sp - 2 technician %s : %v", technician, err)
			}

			technicianGotSP2At := time.Now()
			dataSP2TechnicianUpdated := sptechnicianmodel.TechnicianGotSP{
				IsGotSP2:       true,
				GotSP2At:       &technicianGotSP2At,
				NoSP2:          noSuratSP2,
				PelanggaranSP2: pelanggaranID,
				SP2FilePath:    pdfSP2TechnicianFilePath,
			}

			if err := db.Where("technician = ?", technician).Updates(&dataSP2TechnicianUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 2 for technician %s : %v", technician, err)
			}

			needToSendTheSPTechnicianThroughWhatsapp[technician] = 2
			logrus.Infof("SP - 2 of Technician %s successfully updated", technician)
			return nil
		case 3:
			// Set SP-3
			noSuratSP3, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP3_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-3 for technician %s: %v", technician, err)
			}
			var nomorSuratSP3Str string
			if noSuratSP3 < 1000 {
				nomorSuratSP3Str = fmt.Sprintf("%03d", noSuratSP3)
			} else {
				nomorSuratSP3Str = fmt.Sprintf("%d", noSuratSP3)
			}

			pelanggaranSP3 := dataSPTeknisi.PelanggaranSP3 + "; " + pelanggaranID

			// Make placeholder for replacements in SP - 3 pdf
			placeholdersSP3Teknisi := map[string]string{
				"$nomor_surat":            nomorSuratSP3Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP3,
				"$nama_teknisi":           namaTeknisi,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACDataTechnician.FullName,
				"$sac_ttd":                SACDataTechnician.TTDPath,
				"$record_technician":      technician,
				"$for_project":            forProject,
			}
			pdfSP3FilenameTechnician := fmt.Sprintf("SP_3_%s_%s.pdf", strings.ReplaceAll(technician, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP3TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP3FilenameTechnician)
			if err := CreatePDFSP3ForTechnician(placeholdersSP3Teknisi, pdfSP3TechnicianFilePath); err != nil {
				return fmt.Errorf("failed to create the pdf for sp - 3 technician %s : %v", technician, err)
			}

			technicianGotSP3At := time.Now()
			dataSP3TechnicianUpdated := sptechnicianmodel.TechnicianGotSP{
				IsGotSP3:       true,
				GotSP3At:       &technicianGotSP3At,
				NoSP3:          noSuratSP3,
				PelanggaranSP3: pelanggaranID,
				SP3FilePath:    pdfSP3TechnicianFilePath,
			}

			if err := db.Where("technician = ?", technician).Updates(&dataSP3TechnicianUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 3 for technician %s : %v", technician, err)
			}

			needToSendTheSPTechnicianThroughWhatsapp[technician] = 3
			logrus.Infof("SP - 3 of Technician %s successfully updated", technician)
			return nil
		default:
			return fmt.Errorf("invalid sp number %d for technician %s", spNumber, technician)
		}
	}
}

// processSPForSPLWithMissingEDCNotSO processes the SP for an SPL who has missing EDC not SO.
// It generates the SP document (PDF) and audio (TTS), updates the database, and marks the SPL for WhatsApp notification.
//
// Parameters:
//   - db: Database connection
//   - forProject: Project identifier
//   - hrdPersonaliaName: Name of HRD Personalia
//   - hrdTTDPath: Path to HRD signature image
//   - hrdPhoneNumber: phone number that HRD used
//   - spl: SPL identifier
//   - namaSPL: SPL name
//   - splCity: SPL city
//   - SACData: SAC data
//   - resignTechnicianReplacer: String to replace resigned technician name
//   - audioDirForSPSPL: Directory for SP audio files
//   - pdfDirForSPSPL: Directory for SP PDF files
//   - needToSendTheSPSPLThroughWhatsapp: Map to track WhatsApp notifications
//   - missingEDCNotSO: Map of missing EDCs per company
//
// Returns:
//   - error: Error if any step fails
func processSPForSPLWithMissingEDCNotSO(
	db *gorm.DB,
	forProject string,
	hrdPersonaliaName string,
	hrdTTDPath string,
	hrdPhoneNumber string,
	spl string,
	namaSPL string,
	splCity string,
	SACData config.SACODOOMS,
	resignTechnicianReplacer string,
	audioDirForSPSPL string,
	pdfDirForSPSPL string,
	needToSendTheSPSPLThroughWhatsapp map[string]int,
	missingEDCNotSO map[string][]string,
) error {
	speech := htgotts.Speech{Folder: audioDirForSPSPL, Language: voices.Indonesian, Handler: &handlers.Native{}}
	tahunSP := time.Now().Format("2006")
	monthRoman, err := fun.MonthToRoman(int(time.Now().Month()))
	if err != nil {
		return fmt.Errorf("failed to convert month to roman numeral: %v", err)
	}

	var tanggalIndoFormatted string
	tgl, err := tanggal.Papar(time.Now(), "Jakarta", tanggal.WIB)
	if err != nil {
		return fmt.Errorf("failed to format date indo : %v", err)
	}
	tanggalIndoFormatted = tgl.Format(" ", []tanggal.Format{
		tanggal.Hari,
		tanggal.NamaBulan,
		tanggal.Tahun,
	})

	var pelanggaranID string
	if len(missingEDCNotSO) == 0 {
		return errors.New("no data found for missing EDC not SO")
	}
	var sb strings.Builder
	sb.WriteString("terdapat EDC yang tidak di Stock Opname: ")
	for company, edcList := range missingEDCNotSO {
		sb.WriteString(fmt.Sprintf("%s (%d EDC); ", company, len(edcList)))
	}
	pelanggaranID = sb.String()

	spNumber, exists := needToSendTheSPSPLThroughWhatsapp[spl]
	if !exists {
		// Create sound sp 1 for SPL
		SP1SPLTextPart1 := "Berikut kami sampaikan bahwa "
		SP1SPLTextPart2 := fmt.Sprintf(" saudara %s.", namaSPL)
		SP1SPLTextPart3 := "Menerima Surat Peringatan (SP-1)."
		SP1SPLTextPart4 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan."
		SP1SPLTextPart5 := "terima kasih..."
		sp1SPLFilenameSound := fmt.Sprintf("%s_SP1_SPL", strings.ReplaceAll(spl, "*", resignTechnicianReplacer))
		fileTTSSP1SPL, err := fun.CreateRobustTTS(speech, audioDirForSPSPL, []string{
			SP1SPLTextPart1,
			SP1SPLTextPart2,
			SP1SPLTextPart3,
			SP1SPLTextPart4,
			SP1SPLTextPart5,
		}, sp1SPLFilenameSound)
		if err != nil {
			return fmt.Errorf("failed to create merged SP1 TTS file for spl %s : %v", spl, err)
		}

		if fileTTSSP1SPL != "" {
			fileInfo, statErr := os.Stat(fileTTSSP1SPL)
			if statErr == nil {
				logrus.Debugf("🔊 SP-1 merged TTS for %s - %s, Size: %d bytes", spl, fileTTSSP1SPL, fileInfo.Size())
			} else {
				logrus.Errorf("🔇 SP-1 TTS for %s got stat error : %v", spl, statErr)
			}
		}

		// Set SP - 1
		noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
		if err != nil {
			return fmt.Errorf("Failed to increment nomor surat SP-1 for spl %s : %v", spl, err)
		}
		var nomorSuratSP1Str string
		if noSuratSP1 < 1000 {
			nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
		} else {
			nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
		}

		pelanggaranID = fun.CapitalizeFirstWord(pelanggaranID)

		placeholderSP1SPL := map[string]string{
			"$nomor_surat":            nomorSuratSP1Str,
			"$bulan_romawi":           monthRoman,
			"$tahun_sp":               tahunSP,
			"$nama_spl":               namaSPL,
			"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
			"$pelanggaran_karyawan":   pelanggaranID,
			"$nama_teknisi":           namaSPL,
			"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
			"$personalia_name":        hrdPersonaliaName,
			"$personalia_ttd":         hrdTTDPath,
			"$personalia_phone":       hrdPhoneNumber,
			"$sac_name":               SACData.FullName,
			"$sac_ttd":                SACData.TTDPath,
			"$record_spl":             spl,
			"$for_project":            forProject,
		}
		pdfSP1FilenameSPL := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
		pdfSP1SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP1FilenameSPL)

		if err := CreatePDFSP1ForSPL(placeholderSP1SPL, pdfSP1SPLFilePath); err != nil {
			return fmt.Errorf("failed to generate pdf of sp 1 spl %s : %v", spl, err)
		}

		splGotSP1At := time.Now()
		dataSP1SPL := sptechnicianmodel.SPLGotSP{
			SPL:                        spl,
			Name:                       namaSPL,
			ForProject:                 forProject,
			IsGotSP1:                   true,
			GotSP1At:                   &splGotSP1At,
			TechnicianNameCausedGotSP1: "",
			NoSP1:                      noSuratSP1,
			PelanggaranSP1:             pelanggaranID,
			SP1SoundTTSPath:            fileTTSSP1SPL,
			SP1FilePath:                pdfSP1SPLFilePath,
		}
		if err := dbWeb.Create(&dataSP1SPL).Error; err != nil {
			return fmt.Errorf("failed to create the SP - 1 for SPL %s : %v", spl, err)
		}
		needToSendTheSPSPLThroughWhatsapp[spl] = 1
		logrus.Infof("SP - 1 of SPL %s successfully created", spl)
		return nil
		// .end of spl got SP-1 coz still have EDC not SO
	} else {
		pelanggaranID = fun.CapitalizeFirstWord("serta " + pelanggaranID)
		var dataSPSPL sptechnicianmodel.SPLGotSP
		if err := db.Where("for_project = ?", forProject).
			Where("spl = ?", spl).
			First(&dataSPSPL).Error; err != nil {
			return fmt.Errorf("failed to find sp data of spl %s : %v", spl, err)
		}

		switch spNumber {
		case 1:
			// Set SP-1
			noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-1 for spl %s : %v", spl, err)
			}
			var nomorSuratSP1Str string
			if noSuratSP1 < 1000 {
				nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
			} else {
				nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
			}

			pelanggaranSP1 := dataSPSPL.PelanggaranSP1 + "; " + pelanggaranID

			placeholderSP1SPL := map[string]string{
				"$nomor_surat":            nomorSuratSP1Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP1,
				"$nama_teknisi":           namaSPL,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACData.FullName,
				"$sac_ttd":                SACData.TTDPath,
				"$record_spl":             spl,
				"$for_project":            forProject,
			}
			pdfSP1FilenameSPL := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP1SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP1FilenameSPL)

			if err := CreatePDFSP1ForSPL(placeholderSP1SPL, pdfSP1SPLFilePath); err != nil {
				return fmt.Errorf("failed to generate pdf of sp 1 spl %s : %v", spl, err)
			}

			splGotSP1At := time.Now()
			dataSP1SPLUpdated := sptechnicianmodel.SPLGotSP{
				IsGotSP1:       true,
				GotSP1At:       &splGotSP1At,
				NoSP1:          noSuratSP1,
				PelanggaranSP1: pelanggaranID,
				SP1FilePath:    pdfSP1SPLFilePath,
			}

			if err := db.Where("spl = ?", spl).Updates(&dataSP1SPLUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 1 for SPL %s : %v", spl, err)
			}

			needToSendTheSPSPLThroughWhatsapp[spl] = 1
			logrus.Infof("SP - 1 of SPL %s successfully updated", spl)
			return nil
		case 2:
			// Set SP-2
			noSuratSP2, err := IncrementNomorSuratSP(db, "LAST_NOMOR_SURAT_SP2_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-2 for spl %s : %v", spl, err)
			}
			var nomorSuratSP2Str string
			if noSuratSP2 < 1000 {
				nomorSuratSP2Str = fmt.Sprintf("%03d", noSuratSP2)
			} else {
				nomorSuratSP2Str = fmt.Sprintf("%d", noSuratSP2)
			}

			pelanggaranSP2 := dataSPSPL.PelanggaranSP2 + "; " + pelanggaranID

			placeholderSP2SPL := map[string]string{
				"$nomor_surat":            nomorSuratSP2Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP2,
				"$nama_teknisi":           namaSPL,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACData.FullName,
				"$sac_ttd":                SACData.TTDPath,
				"$record_spl":             spl,
				"$for_project":            forProject,
			}
			pdfSP2FilenameSPL := fmt.Sprintf("SP_2_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP2SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP2FilenameSPL)

			if err := CreatePDFSP2ForSPL(placeholderSP2SPL, pdfSP2SPLFilePath); err != nil {
				return fmt.Errorf("failed to generate pdf of sp 2 spl %s : %v", spl, err)
			}

			splGotSP2At := time.Now()
			dataSP2SPLUpdated := sptechnicianmodel.SPLGotSP{
				IsGotSP2:       true,
				GotSP2At:       &splGotSP2At,
				NoSP2:          noSuratSP2,
				PelanggaranSP2: pelanggaranID,
				SP2FilePath:    pdfSP2SPLFilePath,
			}

			if err := db.Where("spl = ?", spl).Updates(&dataSP2SPLUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 2 for SPL %s : %v", spl, err)
			}

			needToSendTheSPSPLThroughWhatsapp[spl] = 2
			logrus.Infof("SP - 2 of SPL %s successfully updated", spl)
			return nil
		case 3:
			// Set SP-3
			noSuratSP3, err := IncrementNomorSuratSP(db, "LAST_NOMOR_SURAT_SP3_GENERATED")
			if err != nil {
				return fmt.Errorf("Failed to increment nomor surat SP-3 for spl %s : %v", spl, err)
			}
			var nomorSuratSP3Str string
			if noSuratSP3 < 1000 {
				nomorSuratSP3Str = fmt.Sprintf("%03d", noSuratSP3)
			} else {
				nomorSuratSP3Str = fmt.Sprintf("%d", noSuratSP3)
			}

			pelanggaranSP3 := dataSPSPL.PelanggaranSP3 + "; " + pelanggaranID

			placeholderSP3SPL := map[string]string{
				"$nomor_surat":            nomorSuratSP3Str,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               tahunSP,
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranSP3,
				"$nama_teknisi":           namaSPL,
				"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
				"$personalia_name":        hrdPersonaliaName,
				"$personalia_ttd":         hrdTTDPath,
				"$personalia_phone":       hrdPhoneNumber,
				"$sac_name":               SACData.FullName,
				"$sac_ttd":                SACData.TTDPath,
				"$record_spl":             spl,
				"$for_project":            forProject,
			}
			pdfSP3FilenameSPL := fmt.Sprintf("SP_3_%s_%s.pdf", strings.ReplaceAll(spl, "*", resignTechnicianReplacer), time.Now().Format("2006-01-02"))
			pdfSP3SPLFilePath := filepath.Join(pdfDirForSPSPL, pdfSP3FilenameSPL)

			if err := CreatePDFSP3ForSPL(placeholderSP3SPL, pdfSP3SPLFilePath); err != nil {
				return fmt.Errorf("failed to generate pdf of sp 3 spl %s : %v", spl, err)
			}

			splGotSP3At := time.Now()
			dataSP3SPLUpdated := sptechnicianmodel.SPLGotSP{
				IsGotSP3:       true,
				GotSP3At:       &splGotSP3At,
				NoSP3:          noSuratSP3,
				PelanggaranSP3: pelanggaranID,
				SP3FilePath:    pdfSP3SPLFilePath,
			}

			if err := db.Where("spl = ?", spl).Updates(&dataSP3SPLUpdated).Error; err != nil {
				return fmt.Errorf("failed to update the SP - 3 for SPL %s : %v", spl, err)
			}

			needToSendTheSPSPLThroughWhatsapp[spl] = 3
			logrus.Infof("SP - 3 of SPL %s successfully updated", spl)
			return nil
		default:
			return fmt.Errorf("invalid sp number %d for spl %s", spNumber, spl)
		}
	}
}

// Deprecated: not used anymore, coz build in word had limited features and hard to customize - pkg baliance.com/gooxml/document
// func CreateWordSP3ForTechnician(technician, name, spl string) (string, error) {
// 	splCity := ""
// 	splCity = getSPLCity(spl)
// 	if splCity == "" {
// 		splCity = "Unknown"
// 	}

// 	// pelanggaran := "Tidak login hari ini, tidak mengunduh data JO & tidak melakukan kunjungan ke merchant."
// 	// imgCSNA := "web/assets/self/img/csna.png"
// 	// imgTTDSari := "web/assets/self/img/ttd_sari.png"
// 	namaTeknisi := ""

// 	if name != "" {
// 		namaTeknisi = name
// 	} else {

// 		namaTeknisi = technician
// 	}

// 	doc := document.New()

// 	// Create header and attach it to the body section
// 	hdr := doc.AddHeader()
// 	doc.BodySection().SetHeader(hdr, wml.ST_HdrFtrDefault)

// 	// Create header content with table-like structure using paragraphs and tabs
// 	// Company logo/name on the left
// 	logoPara := hdr.AddParagraph()
// 	logoRun := logoPara.AddRun()
// 	logoRun.Properties().SetBold(true)
// 	logoRun.Properties().SetSize(16)
// 	logoRun.Properties().SetColor(color.Blue)
// 	logoRun.AddText("CyberSmart")

// 	// Company information in center
// 	companyPara := hdr.AddParagraph()
// 	companyPara.Properties().SetAlignment(wml.ST_JcCenter)
// 	companyRun := companyPara.AddRun()
// 	companyRun.Properties().SetFontFamily("Century Gothic")
// 	companyRun.Properties().SetBold(true)
// 	companyRun.Properties().SetSize(10.5)
// 	companyRun.AddText(config.WebPanel.Get().Default.PT)

// 	addressPara := hdr.AddParagraph()
// 	addressPara.Properties().SetAlignment(wml.ST_JcCenter)
// 	addressRun := addressPara.AddRun()
// 	companyRun.Properties().SetFontFamily("Century Gothic")
// 	addressRun.Properties().SetSize(7)
// 	addressRun.AddText("Rukan Crown Blok J No. 008, Green Lake City")

// 	addressPara2 := hdr.AddParagraph()
// 	addressPara2.Properties().SetAlignment(wml.ST_JcCenter)
// 	addressRun2 := addressPara2.AddRun()
// 	companyRun.Properties().SetFontFamily("Century Gothic")
// 	addressRun2.Properties().SetSize(7)
// 	addressRun2.AddText("Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146")

// 	phonePara := hdr.AddParagraph()
// 	phonePara.Properties().SetAlignment(wml.ST_JcCenter)
// 	phoneRun := phonePara.AddRun()
// 	companyRun.Properties().SetFontFamily("Century Gothic")
// 	phoneRun.Properties().SetSize(7)
// 	phoneRun.AddText("Tel.: (021) 22521101 / 5504722 / 5504723")

// 	// // Form code box on the right using right-aligned paragraph
// 	// formCodePara := hdr.AddParagraph()
// 	// formCodePara.Properties().SetAlignment(wml.ST_JcRight)
// 	// formCodeRun := formCodePara.AddRun()
// 	// formCodeRun.Properties().SetSize(8)
// 	// formCodeRun.Properties().SetBold(true)
// 	// // Create a simple bordered text box
// 	// formCodeRun.AddText("┌─────────────────┐")
// 	// formCodeRun.AddBreak()

// 	// formCodePara2 := hdr.AddParagraph()
// 	// formCodePara2.Properties().SetAlignment(wml.ST_JcRight)
// 	// formCodeRun2 := formCodePara2.AddRun()
// 	// formCodeRun2.Properties().SetSize(8)
// 	// formCodeRun2.Properties().SetBold(true)
// 	// formCodeRun2.AddText("│ FM-HRD.07.00.01 │")
// 	// formCodeRun2.AddBreak()

// 	// formCodePara3 := hdr.AddParagraph()
// 	// formCodePara3.Properties().SetAlignment(wml.ST_JcRight)
// 	// formCodeRun3 := formCodePara3.AddRun()
// 	// formCodeRun3.Properties().SetSize(8)
// 	// formCodeRun3.Properties().SetBold(true)
// 	// formCodeRun3.AddText("└─────────────────┘")
// 	// formCodePara3.Properties().SetAlignment(wml.ST_JcRight)

// 	// Add horizontal line separator
// 	borderPara := hdr.AddParagraph()
// 	borderRun := borderPara.AddRun()
// 	borderPara.Properties().SetAlignment(wml.ST_JcCenter)
// 	borderRun.Properties().SetSize(10)
// 	borderRun.Properties().SetBold(true)
// 	borderRun.AddText("_________________________________________________________________________")

// 	// Add spacing after header
// 	doc.AddParagraph().AddRun().AddBreak()
// 	doc.AddParagraph().AddRun().AddBreak()

// 	// Main document title (centered and underlined)
// 	titlePara := doc.AddParagraph()
// 	titlePara.Properties().SetAlignment(wml.ST_JcCenter)
// 	titleRun := titlePara.AddRun()
// 	titleRun.Properties().SetBold(true)
// 	titleRun.Properties().SetSize(14)
// 	titleRun.Properties().SetUnderline(wml.ST_UnderlineSingle, color.Black)
// 	titleRun.AddText("SURAT PERINGATAN KEDUA (SP-3)")

// 	// Add spacing
// 	doc.AddParagraph().AddRun().AddBreak()

// 	// Document number (centered and underlined)
// 	numberPara := doc.AddParagraph()
// 	numberPara.Properties().SetAlignment(wml.ST_JcCenter)
// 	numberRun := numberPara.AddRun()
// 	numberRun.Properties().SetBold(true)
// 	numberRun.Properties().SetSize(12)
// 	numberRun.Properties().SetUnderline(wml.ST_UnderlineSingle, color.Black)
// 	numberRun.AddText("Nomor : 033/SP.I-CSNA/III/2025")

// 	// Add content spacing
// 	doc.AddParagraph().AddRun().AddBreak()
// 	doc.AddParagraph().AddRun().AddBreak()

// 	// Content area - "Kepada Yth. Teknisi:" (underlined)
// 	kepada := doc.AddParagraph()
// 	kepadaRun := kepada.AddRun()
// 	kepadaRun.Properties().SetSize(11)
// 	kepadaRun.Properties().SetUnderline(wml.ST_UnderlineSingle, color.Black)
// 	kepadaRun.AddText(fmt.Sprintf("Kepada Yth. Teknisi: %s", namaTeknisi))

// 	// Add spacing
// 	doc.AddParagraph().AddRun().AddBreak()

// 	// "Dengan hormat,"
// 	hormat := doc.AddParagraph()
// 	hormatRun := hormat.AddRun()
// 	hormatRun.Properties().SetSize(11)
// 	hormatRun.AddText("Dengan hormat,")

// 	// Add spacing
// 	doc.AddParagraph().AddRun().AddBreak()

// 	// Main content paragraph
// 	content := doc.AddParagraph()
// 	contentRun := content.AddRun()
// 	contentRun.Properties().SetSize(11)
// 	contentRun.AddText("Berdasarkan evaluasi kinerja dan kedisiplinan kerja, dengan ini diberikan surat peringatan pertama kepada teknisi yang bersangkutan.")

// 	// Add spacing
// 	doc.AddParagraph().AddRun().AddBreak()

// 	// Pelanggaran section (underlined)
// 	pelanggaranPara := doc.AddParagraph()
// 	pelanggaranRun := pelanggaranPara.AddRun()
// 	pelanggaranRun.Properties().SetSize(11)
// 	pelanggaranRun.Properties().SetUnderline(wml.ST_UnderlineSingle, color.Black)
// 	pelanggaranRun.AddText("Pelanggaran yang dilakukan:")

// 	// Add the violation list
// 	violationPara := doc.AddParagraph()
// 	violationRun := violationPara.AddRun()
// 	violationRun.Properties().SetSize(11)
// 	// Add the violation list
// 	violationPara2 := doc.AddParagraph()
// 	violationRun2 := violationPara2.AddRun()
// 	violationRun2.Properties().SetSize(11)
// 	violationRun2.AddText("- Tidak login pada hari kerja")
// 	violationRun2.AddBreak()
// 	violationRun2.AddText("- Tidak mengunduh data Job Order")
// 	violationRun2.AddBreak()
// 	violationRun2.AddText("- Tidak melakukan kunjungan ke merchant")

// 	// Add spacing
// 	doc.AddParagraph().AddRun().AddBreak()

// 	// Final statement
// 	finalPara := doc.AddParagraph()
// 	finalRun := finalPara.AddRun()
// 	finalRun.Properties().SetSize(11)
// 	finalRun.AddText("Dengan surat peringatan ini, diharapkan dapat memperbaiki kinerja dan kedisiplinan dalam menjalankan tugas.")

// 	// Add footer spacing for signatures
// 	doc.AddParagraph().AddRun().AddBreak()
// 	doc.AddParagraph().AddRun().AddBreak()
// 	doc.AddParagraph().AddRun().AddBreak()

// 	// Signature area
// 	sigPara := doc.AddParagraph()
// 	sigPara.Properties().SetAlignment(wml.ST_JcRight)
// 	sigRun := sigPara.AddRun()
// 	sigRun.Properties().SetSize(11)
// 	sigRun.AddText("Jakarta, " + time.Now().Format("02 January 2006"))
// 	sigRun.AddBreak()
// 	sigRun.AddBreak()
// 	sigRun.AddText("Hormat kami,")
// 	sigRun.AddBreak()
// 	sigRun.AddBreak()
// 	sigRun.AddBreak()
// 	sigRun.AddBreak()
// 	sigRun.AddText("PT. Cyber Smart Network Asia")
// 	sigRun.AddBreak()
// 	sigRun.AddText("HRD Department")

// 	wordFileName := fmt.Sprintf("SP_3_%s.docx",
// 		strings.ReplaceAll(technician, "*", "Resigned"),
// 	)

// 	selectedMainDir, err := fun.FindValidDirectory([]string{
// 		"web/file/sp_technician",
// 		"../web/file/sp_technician",
// 		"../../web/file/sp_technician",
// 	})

// 	if err != nil {
// 		return "", fmt.Errorf("failed to find valid directory for SP technician files: %v", err)
// 	}
// 	fileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
// 	if err := os.MkdirAll(fileDir, 0755); err != nil {
// 		return "", fmt.Errorf("failed to create directory for SP technician files: %v", err)
// 	}

// 	wordFilePath := filepath.Join(fileDir, wordFileName)

// 	// Save the document
// 	err = doc.SaveToFile(wordFilePath)
// 	if err != nil {
// 		return "", fmt.Errorf("error saving document: %v", err)
// 	}

// 	return wordFilePath, nil
// }

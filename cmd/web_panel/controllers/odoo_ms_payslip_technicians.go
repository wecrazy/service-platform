package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"service-platform/internal/config"
	"strconv"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
	"github.com/TigorLazuardi/tanggal"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func TryGeneratePDFPayslipTechnicianEDC() {
	var count int64
	dbWeb := gormdb.Databases.Web
	dbWeb.Model(&odooms.MSTechnicianPayroll{}).
		Where("name IS NOT NULL").
		Where("payslip_sent = ?", false).
		Count(&count)

	if count == 0 {
		logrus.Error("No valid technician payroll records found.")
		return
	}

	logrus.Infof("%d records will be processing to create its payslip pdf", count)

	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/payslip_technician",
		"../web/file/payslip_technician",
		"../../web/file/payslip_technician",
	})
	if err != nil {
		logrus.Errorf("failed to find payslip technician main directory: %v", err)
		return
	}
	pdfFileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(pdfFileDir, 0755); err != nil {
		logrus.Errorf("failed to create payslip technician directory: %v", err)
		return
	}

	var batchSize int64 = 100
	for offset := int64(0); offset < count; offset += batchSize {
		var records []odooms.MSTechnicianPayroll
		result := dbWeb.Model(&odooms.MSTechnicianPayroll{}).
			Where("name IS NOT NULL AND name != ''").
			Where("payslip_sent = ?", false).
			Limit(int(batchSize)).
			Offset(int(offset)).
			Find(&records)
		if result.Error != nil {
			logrus.Errorf("Error fetching records: %v", result.Error)
			continue
		}

		for _, record := range records {
			teknisiName := record.Name
			if strings.Contains(teknisiName, "*") {
				teknisiName = strings.ReplaceAll(teknisiName, "*", "(Resigned)")
			}

			pdfFileName := fmt.Sprintf("[EDC]SlipGaji_%s_%v.pdf", teknisiName, time.Now().Format("02Jan2006"))
			pdfFilePath := filepath.Join(pdfFileDir, pdfFileName)

			err := GeneratePDFPayslipTechnicianEDC(record, pdfFilePath)
			if err != nil {
				logrus.Errorf("Error generating PDF for technician %s: %v", record.Name, err)
			} else {
				logrus.Infof("[EDC] Successfully generated PDF for technician %s", record.Name)
			}

			// Update database with PDF filepath
			if err := dbWeb.Model(&odooms.MSTechnicianPayroll{}).
				Where("id = ?", record.ID).
				Update("payslip_filepath", pdfFilePath).Error; err != nil {
				logrus.Errorf("Error updating database for technician %s: %v", record.Name, err)
			}
		}
	}
}

func TryGeneratePDFPayslipTechnicianATM() {
	var count int64
	dbWeb := gormdb.Databases.Web
	dbWeb.Model(&odooms.MSTechnicianPayrollDedicatedATM{}).
		Where("name IS NOT NULL").
		Where("payslip_sent = ?", false).
		Count(&count)

	if count == 0 {
		logrus.Error("No valid technician payroll records found.")
		return
	}

	logrus.Infof("%d records will be processing to create its payslip pdf", count)

	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/payslip_technician",
		"../web/file/payslip_technician",
		"../../web/file/payslip_technician",
	})
	if err != nil {
		logrus.Errorf("failed to find payslip technician main directory: %v", err)
		return
	}
	pdfFileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(pdfFileDir, 0755); err != nil {
		logrus.Errorf("failed to create payslip technician directory: %v", err)
		return
	}

	var batchSize int64 = 100
	for offset := int64(0); offset < count; offset += batchSize {
		var records []odooms.MSTechnicianPayrollDedicatedATM
		result := dbWeb.Model(&odooms.MSTechnicianPayrollDedicatedATM{}).
			Where("name IS NOT NULL AND name != ''").
			Where("payslip_sent = ?", false).
			Limit(int(batchSize)).
			Offset(int(offset)).
			Find(&records)
		if result.Error != nil {
			logrus.Errorf("Error fetching records: %v", result.Error)
			continue
		}

		for _, record := range records {
			teknisiName := record.Name
			if strings.Contains(teknisiName, "*") {
				teknisiName = strings.ReplaceAll(teknisiName, "*", "(Resigned)")
			}

			pdfFileName := fmt.Sprintf("[ATM]SlipGaji_%s_%v.pdf", teknisiName, time.Now().Format("02Jan2006"))
			pdfFilePath := filepath.Join(pdfFileDir, pdfFileName)

			err := GeneratePDFPayslipTechnicianATM(record, pdfFilePath)
			if err != nil {
				logrus.Errorf("Error generating PDF for technician %s: %v", record.Name, err)
			} else {
				logrus.Infof("[ATM] Successfully generated PDF for technician %s", record.Name)
			}

			// Update database with PDF filepath
			if err := dbWeb.Model(&odooms.MSTechnicianPayrollDedicatedATM{}).
				Where("id = ?", record.ID).
				Update("payslip_filepath", pdfFilePath).Error; err != nil {
				logrus.Errorf("Error updating database for technician %s: %v", record.Name, err)
			}
		}
	}
}

func GeneratePDFPayslipTechnicianEDC(payrollData odooms.MSTechnicianPayroll, filePathOutput string) error {
	if payrollData.Name == "" {
		return fmt.Errorf("technician name is empty")
	}

	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")

	signatureName := config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName
	signatureImg := filepath.Join(imgAssetsDir, config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureImg)

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	var teknisiName string = "-"
	if payrollData.Name != "" {
		teknisiName = payrollData.Name
	}

	// Create PDF instance
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(fmt.Sprintf("Slip gaji teknisi EDC: %s", teknisiName), true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("payslip, gaji, slip, teknisi, EDC", true)
	pdf.SetSubject(fmt.Sprintf("Slip gaji yang akan diberikan kepada Saudara(i) %s", teknisiName), true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	// Add fonts from the font directory
	pdf.AddFont("Arial", "", filepath.Join(fontMainDir, "arial.json"))
	pdf.AddFont("CenturyGothic", "", filepath.Join(fontMainDir, "CenturyGothic.json"))
	pdf.AddFont("CenturyGothic", "B", filepath.Join(fontMainDir, "CenturyGothic-Bold.json"))

	pdf.AddPage()

	// Set default font to avoid the "font has not been set" error
	pdf.SetFont("CenturyGothic", "", 10)

	// Constants
	marginLeft := 10.0
	marginRight := 10.0
	marginTop := 10.0
	marginBottom := 10.0
	pageWidth := 210.0
	pageHeight := 297.0
	contentWidth := pageWidth - marginLeft - marginRight

	// Draw page border
	pdf.SetLineWidth(0.5)
	pdf.Rect(marginLeft, marginTop, contentWidth, pageHeight-marginTop-marginBottom, "D")

	// ====================== Start of Header ======================
	currentY := marginTop + 5.0

	// Draw logo on the left
	logoWidth := 50.0
	logoHeight := 15.0
	pdf.ImageOptions(imgCSNA, marginLeft+5, currentY, logoWidth, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	currentY += logoHeight + 4.0
	// Company name FIRST (BOLD, larger font) - positioned at top
	pdf.SetFont("CenturyGothic", "B", 14)
	titleText := strings.ToUpper(config.WebPanel.Get().Default.PT)
	pdf.SetXY(marginLeft, currentY)
	pdf.CellFormat(contentWidth, 6, titleText, "", 1, "C", false, 0, "")

	// Subtitle BELOW company name (normal font, slightly smaller)
	pdf.SetFont("CenturyGothic", "", 12)
	pdf.SetXY(marginLeft, currentY+6)
	pdf.CellFormat(contentWidth, 6, "SLIP GAJI TEKNISI MANAGE SERVICE", "", 1, "C", false, 0, "")

	// ====================== BULAN and TAHUN Section ======================
	currentY += 21.0
	leftColX := marginLeft + 5.0
	colonX := marginLeft + 40.0 // Position of colon - MUST match col1ColonX below
	valueX := colonX + 5.0      // Position of value after colon

	// BULAN (BOLD)
	pdf.SetFont("CenturyGothic", "B", 9)
	pdf.SetXY(leftColX, currentY)
	pdf.Cell(colonX-leftColX, 5, "BULAN")
	pdf.SetXY(colonX, currentY)
	pdf.Cell(5, 5, ":")
	pdf.SetXY(valueX, currentY)

	var bulanPayslip, tahunPayslip string
	tgl, err := tanggal.Papar(time.Now().AddDate(0, -1, 0), "Jakarta", tanggal.WIB)
	if err != nil {
		logrus.Errorf("got error: %v", err)
	} else {
		bulanPayslip = tgl.Format("", []tanggal.Format{
			tanggal.NamaBulan,
		})
		bulanPayslip = strings.ToUpper(bulanPayslip)

		tahunPayslip = tgl.Format("", []tanggal.Format{
			tanggal.Tahun,
		})
	}
	pdf.Cell(0, 5, bulanPayslip)

	// TAHUN (BOLD)
	currentY += 5.0
	pdf.SetFont("CenturyGothic", "B", 9)
	pdf.SetXY(leftColX, currentY)
	pdf.Cell(colonX-leftColX, 5, "TAHUN")
	pdf.SetXY(colonX, currentY)
	pdf.Cell(5, 5, ":")
	pdf.SetXY(valueX, currentY)
	pdf.Cell(0, 5, tahunPayslip)

	// Second horizontal line (below TAHUN, touching page borders)
	currentY += 7.0
	pdf.SetLineWidth(0.3)
	pdf.Line(marginLeft, currentY, pageWidth-marginRight, currentY)

	// ====================== Two Column Layout ======================
	currentY += 4.0

	// Define column positions
	col1LabelX := marginLeft + 5.0
	col1ColonX := marginLeft + 40.0
	col1ValueX := col1ColonX + 5.0

	col2LabelX := pageWidth/2 + 10.0
	col2ColonX := pageWidth/2 + 45.0
	col2ValueX := col2ColonX + 5.0

	// Helper function to add row with proper text wrapping, bold text support, and perfect colon alignment
	// boldMode: "none", "label", "value", "both", "all"
	addRow := func(
		col1Label, col1Value, col2Label, col2Value string,
		col1ShowColon, col2ShowColon bool,
		col1BoldMode, col2BoldMode string,
	) {
		rowStartY := currentY
		maxRowHeight := 5.0

		// small helper – MUST be called before every render
		setFont := func(bold bool) {
			if bold {
				pdf.SetFont("CenturyGothic", "B", 9)
			} else {
				pdf.SetFont("CenturyGothic", "", 9)
			}
		}

		// Render one column
		renderColumn := func(
			labelX, colonX, valueX, maxValueWidth float64,
			label, value string,
			showColon bool,
			boldMode string,
		) float64 {

			colStartY := rowStartY
			colHeight := 5.0

			// ---------- LABEL ----------
			pdf.SetXY(labelX, colStartY)
			setFont(boldMode == "label" || boldMode == "both" || boldMode == "all")

			labelMaxWidth := colonX - labelX
			labelWidth := pdf.GetStringWidth(label)

			if labelWidth > labelMaxWidth {
				pdf.MultiCell(labelMaxWidth, 4, label, "", "L", false)
				colHeight = pdf.GetY() - colStartY
			} else {
				pdf.Cell(labelMaxWidth, 5, label)
			}

			// ---------- COLON ----------
			if showColon {
				pdf.SetXY(colonX, colStartY)
				setFont(boldMode == "all")
				pdf.Cell(5, 5, ":")
			}

			// ---------- VALUE ----------
			if value != "" {
				// currency formatting
				if strings.HasPrefix(value, "Rp") {
					parts := strings.SplitN(value, " ", 2)

					pdf.SetXY(valueX, colStartY)
					setFont(boldMode == "value" || boldMode == "both" || boldMode == "all")
					pdf.Cell(10, 5, parts[0])

					if len(parts) == 2 {
						numWidth := pdf.GetStringWidth(parts[1])
						pdf.SetX(valueX + maxValueWidth - numWidth)
						pdf.Cell(numWidth, 5, parts[1])
					}

				} else {
					setFont(boldMode == "value" || boldMode == "both" || boldMode == "all")

					valueWidth := pdf.GetStringWidth(value)
					pdf.SetXY(valueX, colStartY)

					if valueWidth > maxValueWidth {
						pdf.MultiCell(maxValueWidth, 4, value, "", "L", false)
						newHeight := pdf.GetY() - colStartY
						if newHeight > colHeight {
							colHeight = newHeight
						}
					} else {
						pdf.Cell(maxValueWidth, 5, value)
					}
				}
			}

			return colHeight
		}

		// ---------- COLUMN 1 ----------
		col1MaxWidth := (pageWidth/2 - 5) - col1ValueX
		col1Height := renderColumn(
			col1LabelX, col1ColonX, col1ValueX, col1MaxWidth,
			col1Label, col1Value,
			col1ShowColon,
			col1BoldMode,
		)

		if col1Height > maxRowHeight {
			maxRowHeight = col1Height
		}

		// ---------- COLUMN 2 ----------
		if col2Label != "" {
			col2MaxWidth := (pageWidth - marginRight - 5) - col2ValueX
			col2Height := renderColumn(
				col2LabelX, col2ColonX, col2ValueX, col2MaxWidth,
				col2Label, col2Value,
				col2ShowColon,
				col2BoldMode,
			)

			if col2Height > maxRowHeight {
				maxRowHeight = col2Height
			}
		}

		// ---------- NEXT ROW ----------
		currentY = rowStartY + maxRowHeight
	} // .end of addRow func

	var namaTeknisi, areaTeknisi, tglJoinTeknisi, bankTeknisi, bankNoTeknisi, bankAccNameTeknisi, employeeCodeTeknisi string
	var amountOverduePM, amountOverdueNonPM, amountUnworkedPM, amountUnworkedNonPM int64

	dbWeb := gormdb.Databases.Web
	if dbWeb != nil {
		var odooMSTechData odooms.ODOOMSTechnicianData
		result := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			// Where(odooms.ODOOMSTechnicianData{Technician: teknisiName}).
			Where("technician = ?", teknisiName).
			First(&odooMSTechData)
		if result.Error == nil {
			namaTeknisi = odooMSTechData.Name
			areaTeknisi = odooMSTechData.Area
			employeeCodeTeknisi = odooMSTechData.EmployeeCode
			if odooMSTechData.UserCreatedOn != nil {
				tglJoinTeknisi = odooMSTechData.UserCreatedOn.Format("02 January 2006")

				tglJoinID, err := tanggal.Papar(*odooMSTechData.UserCreatedOn, "Jakarta", tanggal.WIB)
				if err == nil {
					tglJoinTeknisi = tglJoinID.Format(" ", []tanggal.Format{
						tanggal.Hari,
						tanggal.NamaBulan,
						tanggal.Tahun,
					})
				}
			}
		}

		var priceBPAKR, priceATM, priceOverduePM, priceOverdueNonPM, priceUnworkedPM, priceUnworkedNonPM float64
		priceATM = config.WebPanel.Get().ODOOMSParam.DefaultATMPrice

		var fsParams []odooms.ODOOMSFSParams
		result = dbWeb.Model(&odooms.ODOOMSFSParams{}).
			Where("1 = 1").
			Find(&fsParams)
		if result.Error == nil {
			for _, param := range fsParams {
				switch strings.ToLower(param.ParamKey) {
				case "bpakr_price":
					strValue := param.ParamValue
					priceBPAKR = fun.ConvertStringToFloat64(strValue)
				case "atm_price":
					strValue := param.ParamValue
					priceATM = fun.ConvertStringToFloat64(strValue)
				case "overdue_price_pm":
					strValue := param.ParamValue
					priceOverduePM = fun.ConvertStringToFloat64(strValue)
				case "not_worked_price_pm":
					strValue := param.ParamValue
					priceUnworkedPM = fun.ConvertStringToFloat64(strValue)
				case "overdue_price_npm":
					strValue := param.ParamValue
					priceOverdueNonPM = fun.ConvertStringToFloat64(strValue)
				case "not_worked_price_npm":
					strValue := param.ParamValue
					priceUnworkedNonPM = fun.ConvertStringToFloat64(strValue)
				}
			}

			amountOverduePM = int64(priceOverduePM * float64(payrollData.PMOver))
			amountOverdueNonPM = int64(priceOverdueNonPM * float64(payrollData.NonPMOver))
			amountUnworkedPM = int64(priceUnworkedPM * float64(payrollData.PMUnworked))
			amountUnworkedNonPM = int64(priceUnworkedNonPM * float64(payrollData.NonPMUnworked))
		}

		if payrollData.PotonganOverduePM > 0 {
			amountOverduePM = int64(payrollData.PotonganOverduePM)
		}
		if payrollData.PotonganOverdueNonPM > 0 {
			amountOverdueNonPM = int64(payrollData.PotonganOverdueNonPM)
		}
		if payrollData.PotonganOverdueUnworkedPM > 0 {
			amountUnworkedPM = int64(payrollData.PotonganOverdueUnworkedPM)
		}
		if payrollData.PotonganOverdueUnworkedNonPM > 0 {
			amountUnworkedNonPM = int64(payrollData.PotonganOverdueUnworkedNonPM)
		}

		// Not used yet, but may be needed in future
		_ = priceBPAKR
		_ = priceATM
	}
	bankTeknisi = payrollData.BankPenerimaGaji
	bankNoTeknisi = payrollData.NomorRekeningBankPenerimaGaji
	bankAccNameTeknisi = payrollData.NamaRekeningBankPenerimaGaji

	if namaTeknisi == "" && bankAccNameTeknisi != "" {
		namaTeknisi = bankAccNameTeknisi
	}

	if tglJoinTeknisi == "" {
		if payrollData.TanggalJoin != "" {
			dateFlexible, err := fun.ParseFlexibleDate(payrollData.TanggalJoin)
			if err == nil {
				tglJoinTeknisi = dateFlexible.Format("02 January 2006")
				tglJoinID, err2 := tanggal.Papar(dateFlexible, "Jakarta", tanggal.WIB)
				if err2 == nil {
					tglJoinTeknisi = tglJoinID.Format(" ", []tanggal.Format{
						tanggal.Hari,
						tanggal.NamaBulan,
						tanggal.Tahun,
					})
				}
			}
		}
	}

	// Employee Information
	addRow("Nama", namaTeknisi,
		"Nama FS", teknisiName,
		true, true,
		"none", "none")
	addRow("Area Service", areaTeknisi,
		"Employee", employeeCodeTeknisi,
		true, true,
		"none", "none")
	addRow("Contract", payrollData.ContractNo,
		"Tanggal Join", tglJoinTeknisi,
		true, true,
		"none", "none") // TODO: Add contract

	// Second horizontal line (below Contract)
	currentY += 2.0
	pdf.Line(marginLeft, currentY, pageWidth-marginRight, currentY)

	// Income Section
	jobOrderPaidEDC := payrollData.PMMeet + payrollData.PMOver + payrollData.NonPMMeet + payrollData.NonPMOver

	currentY += 3.0
	addRow("Penghasilan", "",
		// "Job Order Paid EDC", fmt.Sprintf("%.0f", float64(payrollData.JORegular)),
		"JO Yang Dikerjakan", fmt.Sprintf("%.0f", float64(jobOrderPaidEDC)),
		false, true, // Income doesn't show colon, Job Order Paid does
		"label", "none",
	)
	var basicSalary, incentive, bpAkr, atm int
	var other int
	var totalTransferred int

	basicSalary = int(payrollData.Basic * 2)
	incentive = int(payrollData.TotalIncentives)
	bpAkr = int(payrollData.TotalBP)
	atm = int(payrollData.TotalATM)

	other = int(payrollData.Other)

	// totalTransferred = (basicSalary + incentive + bpAkr + atm) + other
	totalTransferred = (basicSalary + incentive + bpAkr + atm) + other - (int(amountOverduePM) + int(amountOverdueNonPM) + int(amountUnworkedPM) + int(amountUnworkedNonPM))

	addRow("Gaji Pokok", fmt.Sprintf("Rp %s", fun.FormatRupiah(basicSalary)),
		"JO Minimum Target", fmt.Sprintf("%.0f", float64(payrollData.JOTarget)), // Before: Minimum
		true, true,
		"none", "none",
	)

	addRow("Insentif", fmt.Sprintf("Rp %s", fun.FormatRupiah(incentive)),
		// "JO PM Meet", fmt.Sprintf("%.0f", float64(payrollData.PMMeet)),
		"Insentif Solved (Dibayar)", fmt.Sprintf("%.0f", float64(payrollData.IncentiveSolved)), // Before: Incentive Solved
		true, true,
		"none", "none",
	)
	addRow("Gaji Pokok (50%)", fmt.Sprintf("Rp %s", fun.FormatRupiah(int(payrollData.Basic))),
		// "JO Non - PM Meet", fmt.Sprintf("%.0f", float64(payrollData.NonPMMeet)),
		"Insentif Solved Pending (Dibayar)", fmt.Sprintf("%.0f", float64(payrollData.IncentiveSolvedPending)), // Before: Incentive Solved Pending
		true, true,
		"none", "none",
	)
	addRow("THR/Bonus", fmt.Sprintf("Rp %s", "-"), // TODO: add thr/bonus calculation
		"Insentif Pemasangan (Rp. 4rb)", fmt.Sprintf("%.0f", float64(payrollData.JOInstallation)),
		true, true,
		"none", "none",
	)
	addRow("Standby", "",
		"JO Insentif", fmt.Sprintf("%.0f", float64(payrollData.JOIncentives)),
		false, true,
		"label", "none",
	)
	addRow("- On Time Presence", "Rp -",
		"JO Tidak Dikerjakan (Denda)", fmt.Sprintf("%.0f", float64(payrollData.NonPMUnworked+payrollData.PMUnworked)),
		true, true,
		"none", "none")
	addRow("- Meal Allowance", "Rp -",
		"JO BP AKR", fmt.Sprintf("%.0f", float64(payrollData.JOBPAll)),
		true, true,
		"none", "none")
	addRow("- Project BPR AKR (Dibayar)", fmt.Sprintf("Rp %s", fun.FormatRupiah(bpAkr)),
		"JO BP AKR (Dibayar)", fmt.Sprintf("%.0f", float64(payrollData.JOBP)),

		true, true,
		"none", "none",
	)
	addRow("- ATM (Dibayar)", fmt.Sprintf("Rp %s", fun.FormatRupiah(atm)),
		"JO Reguler EDC", fmt.Sprintf("%.0f", float64(payrollData.JORegular)),
		true, true,
		"none", "none",
	)

	joDikerjakanTidakDibayar := payrollData.JORegular + payrollData.JOBPAll + payrollData.JOATM - payrollData.PMMeet - payrollData.PMOver - payrollData.NonPMMeet - payrollData.NonPMOver
	addRow("", "",
		"JO ATM", fmt.Sprintf("%.0f", float64(payrollData.JOATM)),
		false, true,
		"none", "none",
	)

	addRow("Other / Rapel", fmt.Sprintf("Rp %s", fun.FormatRupiah(other)),
		"JO Reguler Yang Diterima", fmt.Sprintf("%.0f", float64(payrollData.JORegular+payrollData.JOBPAll+payrollData.JOATM)), // Before: All JO
		true, true,
		"none", "none",
	)

	// Deduction Section
	currentY += 2.0
	addRow("Pinalti Potongan", "",
		"JO Yang Dikerjakan (Tidak Dibayarkan)", fmt.Sprintf("%.0f", float64(joDikerjakanTidakDibayar)),
		false, true,
		"label", "none")
	addRow("Overdue SLA PM", fmt.Sprintf("Rp %s", fun.FormatRupiah(int(amountOverduePM))),
		"JO Overdue SLA PM", fmt.Sprintf("%.0f", float64(payrollData.PMOver)),
		true, true,
		"none", "none",
	)
	addRow("Overdue SLA Non-PM", fmt.Sprintf("Rp %s", fun.FormatRupiah(int(amountOverdueNonPM))),
		"JO Overdue SLA Non-PM", fmt.Sprintf("%.0f", float64(payrollData.NonPMOver)),
		true, true,
		"none", "none",
	)
	addRow("JO Yang Tidak Dikerjakan (PM) / ATM", fmt.Sprintf("Rp %s", fun.FormatRupiah(int(amountUnworkedPM))),
		"JO Yang Tidak Dikerjakan (PM) / ATM", fmt.Sprintf("%.0f", float64(payrollData.PMUnworked)),
		true, true,
		"none", "none",
	)
	addRow("JO Yang Tidak Dikerjakan (Non - PM)", fmt.Sprintf("Rp %s", fun.FormatRupiah(int(amountUnworkedNonPM))),
		"JO Yang Tidak Dikerjakan (Non - PM)", fmt.Sprintf("%.0f", float64(payrollData.NonPMUnworked)),
		true, true,
		"none", "none",
	)
	// addRow("Pending Non - PM", "Rp -",
	// 	"JO Pending Non - PM", "0",
	// 	true, true,
	// )
	addRow("Jumlah Yang Diterima", fmt.Sprintf("Rp %s", fun.FormatRupiah(totalTransferred)),
		"", "",
		true, false,
		"all", "",
	)

	// Bank Information and Signature Section (side by side)
	currentY += 8.0
	bankStartY := currentY // Save the starting Y position for HRD section

	// Left side - Bank Information (reuse existing column positions)
	pdf.SetFont("CenturyGothic", "", 9)
	pdf.SetXY(col1LabelX, currentY)
	pdf.Cell(col1ColonX-col1LabelX, 5, "Bank")
	pdf.SetXY(col1ColonX, currentY)
	pdf.Cell(5, 5, ":")
	pdf.SetXY(col1ValueX, currentY)
	pdf.Cell(0, 5, bankTeknisi)

	currentY += 10.0
	pdf.SetXY(col1LabelX, currentY)
	pdf.Cell(col1ColonX-col1LabelX, 5, "Nomor Rekening")
	pdf.SetXY(col1ColonX, currentY)
	pdf.Cell(5, 5, ":")
	pdf.SetXY(col1ValueX, currentY)
	pdf.Cell(0, 5, bankNoTeknisi)

	currentY += 10.0
	pdf.SetXY(col1LabelX, currentY)
	pdf.Cell(col1ColonX-col1LabelX, 5, "Nama Rekening")
	pdf.SetXY(col1ColonX, currentY)
	pdf.Cell(5, 5, ":")
	pdf.SetXY(col1ValueX, currentY)
	pdf.Cell(0, 5, bankAccNameTeknisi)

	// Right side - Signature Section (aligned with Bank section)
	hrdLabelX := pageWidth/2 + 5.0
	hrdSectionWidth := (pageWidth - marginRight) - hrdLabelX

	// Signature label
	pdf.SetFont("CenturyGothic", "B", 9)
	pdf.SetXY(hrdLabelX, bankStartY)
	pdf.CellFormat(hrdSectionWidth, 5, "Finance", "", 1, "C", false, 0, "")

	// Add signature image (TTD)
	signatureWidth := 30.0
	signatureHeight := 15.0
	signatureX := hrdLabelX + (hrdSectionWidth-signatureWidth)/2 // Center the signature
	signatureY := bankStartY + 6.0
	pdf.ImageOptions(signatureImg, signatureX, signatureY, signatureWidth, signatureHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Add name below signature
	pdf.SetFont("CenturyGothic", "", 9)
	pdf.SetXY(hrdLabelX, signatureY+signatureHeight+2.0)
	pdf.CellFormat(hrdSectionWidth, 5, signatureName, "", 1, "C", false, 0, "")

	// Update currentY to be after both sections
	currentY += 5.0

	/*
		Output
	*/
	if err := pdf.OutputFileAndClose(filePathOutput); err != nil {
		return fmt.Errorf("failed to output PDF file: %v", err)
	}

	return nil
}

func GeneratePDFPayslipTechnicianATM(payrollData odooms.MSTechnicianPayrollDedicatedATM, filePathOutput string) error {
	if payrollData.Name == "" {
		return fmt.Errorf("technician name is empty")
	}

	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna.png")

	signatureName := config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName
	signatureImg := filepath.Join(imgAssetsDir, config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureImg)

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	var teknisiName string = "-"
	if payrollData.Name != "" {
		teknisiName = payrollData.Name
	}

	// Create PDF instance
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(fmt.Sprintf("Slip gaji teknisi ATM: %s", teknisiName), true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("payslip, gaji, slip, teknisi, ATM", true)
	pdf.SetSubject(fmt.Sprintf("Slip gaji yang akan diberikan kepada Saudara(i) %s", teknisiName), true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	// Add fonts from the font directory
	pdf.AddFont("Arial", "", filepath.Join(fontMainDir, "arial.json"))
	pdf.AddFont("CenturyGothic", "", filepath.Join(fontMainDir, "CenturyGothic.json"))
	pdf.AddFont("CenturyGothic", "B", filepath.Join(fontMainDir, "CenturyGothic-Bold.json"))

	pdf.AddPage()

	// Set default font to avoid the "font has not been set" error
	pdf.SetFont("CenturyGothic", "", 10)

	// Constants
	marginLeft := 10.0
	marginRight := 10.0
	marginTop := 10.0
	marginBottom := 10.0
	pageWidth := 210.0
	pageHeight := 297.0
	contentWidth := pageWidth - marginLeft - marginRight

	// Draw page border
	pdf.SetLineWidth(0.5)
	pdf.Rect(marginLeft, marginTop, contentWidth, pageHeight-marginTop-marginBottom, "D")

	// ====================== Start of Header ======================
	currentY := marginTop + 5.0

	// Draw logo on the left
	logoWidth := 50.0
	logoHeight := 15.0
	pdf.ImageOptions(imgCSNA, marginLeft+5, currentY, logoWidth, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	currentY += logoHeight + 4.0
	// Company name FIRST (BOLD, larger font) - positioned at top
	pdf.SetFont("CenturyGothic", "B", 14)
	titleText := strings.ToUpper(config.WebPanel.Get().Default.PT)
	pdf.SetXY(marginLeft, currentY)
	pdf.CellFormat(contentWidth, 6, titleText, "", 1, "C", false, 0, "")

	// Subtitle BELOW company name (normal font, slightly smaller)
	pdf.SetFont("CenturyGothic", "", 12)
	pdf.SetXY(marginLeft, currentY+6)
	pdf.CellFormat(contentWidth, 6, "SLIP GAJI TEKNISI MANAGE SERVICE", "", 1, "C", false, 0, "")

	// ====================== BULAN and TAHUN Section ======================
	currentY += 21.0
	leftColX := marginLeft + 5.0
	colonX := marginLeft + 40.0 // Position of colon - MUST match col1ColonX below
	valueX := colonX + 5.0      // Position of value after colon

	// BULAN (BOLD)
	pdf.SetFont("CenturyGothic", "B", 9)
	pdf.SetXY(leftColX, currentY)
	pdf.Cell(colonX-leftColX, 5, "BULAN")
	pdf.SetXY(colonX, currentY)
	pdf.Cell(5, 5, ":")
	pdf.SetXY(valueX, currentY)

	var bulanPayslip, tahunPayslip string
	tgl, err := tanggal.Papar(time.Now().AddDate(0, -1, 0), "Jakarta", tanggal.WIB)
	if err != nil {
		logrus.Errorf("got error: %v", err)
	} else {
		bulanPayslip = tgl.Format("", []tanggal.Format{
			tanggal.NamaBulan,
		})
		bulanPayslip = strings.ToUpper(bulanPayslip)

		tahunPayslip = tgl.Format("", []tanggal.Format{
			tanggal.Tahun,
		})
	}
	pdf.Cell(0, 5, bulanPayslip)

	// TAHUN (BOLD)
	currentY += 5.0
	pdf.SetFont("CenturyGothic", "B", 9)
	pdf.SetXY(leftColX, currentY)
	pdf.Cell(colonX-leftColX, 5, "TAHUN")
	pdf.SetXY(colonX, currentY)
	pdf.Cell(5, 5, ":")
	pdf.SetXY(valueX, currentY)
	pdf.Cell(0, 5, tahunPayslip)

	// Second horizontal line (below TAHUN, touching page borders)
	currentY += 7.0
	pdf.SetLineWidth(0.3)
	pdf.Line(marginLeft, currentY, pageWidth-marginRight, currentY)

	// ====================== Two Column Layout ======================
	currentY += 4.0

	// Define column positions
	col1LabelX := marginLeft + 5.0
	col1ColonX := marginLeft + 40.0
	col1ValueX := col1ColonX + 5.0

	col2LabelX := pageWidth/2 + 10.0
	col2ColonX := pageWidth/2 + 45.0
	col2ValueX := col2ColonX + 5.0

	// Helper function to add row with proper text wrapping, bold text support, and perfect colon alignment
	// boldMode: "none", "label", "value", "both", "all"
	addRow := func(
		col1Label, col1Value, col2Label, col2Value string,
		col1ShowColon, col2ShowColon bool,
		col1BoldMode, col2BoldMode string,
	) {
		rowStartY := currentY
		maxRowHeight := 5.0

		// small helper – MUST be called before every render
		setFont := func(bold bool) {
			if bold {
				pdf.SetFont("CenturyGothic", "B", 9)
			} else {
				pdf.SetFont("CenturyGothic", "", 9)
			}
		}

		// Render one column
		renderColumn := func(
			labelX, colonX, valueX, maxValueWidth float64,
			label, value string,
			showColon bool,
			boldMode string,
		) float64 {

			colStartY := rowStartY
			colHeight := 5.0

			// ---------- LABEL ----------
			pdf.SetXY(labelX, colStartY)
			setFont(boldMode == "label" || boldMode == "both" || boldMode == "all")

			labelMaxWidth := colonX - labelX
			labelWidth := pdf.GetStringWidth(label)

			if labelWidth > labelMaxWidth {
				pdf.MultiCell(labelMaxWidth, 4, label, "", "L", false)
				colHeight = pdf.GetY() - colStartY
			} else {
				pdf.Cell(labelMaxWidth, 5, label)
			}

			// ---------- COLON ----------
			if showColon {
				pdf.SetXY(colonX, colStartY)
				setFont(boldMode == "all")
				pdf.Cell(5, 5, ":")
			}

			// ---------- VALUE ----------
			if value != "" {
				// currency formatting
				if strings.HasPrefix(value, "Rp") {
					parts := strings.SplitN(value, " ", 2)

					pdf.SetXY(valueX, colStartY)
					setFont(boldMode == "value" || boldMode == "both" || boldMode == "all")
					pdf.Cell(10, 5, parts[0])

					if len(parts) == 2 {
						numWidth := pdf.GetStringWidth(parts[1])
						pdf.SetX(valueX + maxValueWidth - numWidth)
						pdf.Cell(numWidth, 5, parts[1])
					}

				} else {
					setFont(boldMode == "value" || boldMode == "both" || boldMode == "all")

					valueWidth := pdf.GetStringWidth(value)
					pdf.SetXY(valueX, colStartY)

					if valueWidth > maxValueWidth {
						pdf.MultiCell(maxValueWidth, 4, value, "", "L", false)
						newHeight := pdf.GetY() - colStartY
						if newHeight > colHeight {
							colHeight = newHeight
						}
					} else {
						pdf.Cell(maxValueWidth, 5, value)
					}
				}
			}

			return colHeight
		}

		// ---------- COLUMN 1 ----------
		col1MaxWidth := (pageWidth/2 - 5) - col1ValueX
		col1Height := renderColumn(
			col1LabelX, col1ColonX, col1ValueX, col1MaxWidth,
			col1Label, col1Value,
			col1ShowColon,
			col1BoldMode,
		)

		if col1Height > maxRowHeight {
			maxRowHeight = col1Height
		}

		// ---------- COLUMN 2 ----------
		if col2Label != "" {
			col2MaxWidth := (pageWidth - marginRight - 5) - col2ValueX
			col2Height := renderColumn(
				col2LabelX, col2ColonX, col2ValueX, col2MaxWidth,
				col2Label, col2Value,
				col2ShowColon,
				col2BoldMode,
			)

			if col2Height > maxRowHeight {
				maxRowHeight = col2Height
			}
		}

		// ---------- NEXT ROW ----------
		currentY = rowStartY + maxRowHeight
	} // .end of addRow func

	var namaTeknisi, areaTeknisi, tglJoinTeknisi, bankTeknisi, bankNoTeknisi, bankAccNameTeknisi, employeeCodeTeknisi string
	var amountOverduePM, amountOverdueNonPM, amountUnworkedPM, amountUnworkedNonPM int64

	dbWeb := gormdb.Databases.Web
	if dbWeb != nil {
		var odooMSTechData odooms.ODOOMSTechnicianData
		result := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where(odooms.ODOOMSTechnicianData{Technician: teknisiName}).
			First(&odooMSTechData)
		if result.Error == nil {
			namaTeknisi = odooMSTechData.Name
			areaTeknisi = odooMSTechData.Area
			employeeCodeTeknisi = odooMSTechData.EmployeeCode
			if odooMSTechData.UserCreatedOn != nil {
				tglJoinTeknisi = odooMSTechData.UserCreatedOn.Format("02 January 2006")
			}
		}

		var priceBPAKR, priceATM, priceOverduePM, priceOverdueNonPM, priceUnworkedPM, priceUnworkedNonPM float64
		priceATM = config.WebPanel.Get().ODOOMSParam.DefaultATMPrice

		var fsParams []odooms.ODOOMSFSParams
		result = dbWeb.Model(&odooms.ODOOMSFSParams{}).
			Where("1 = 1").
			Find(&fsParams)
		if result.Error == nil {
			for _, param := range fsParams {
				switch strings.ToLower(param.ParamKey) {
				case "bpakr_price":
					strValue := param.ParamValue
					priceBPAKR = fun.ConvertStringToFloat64(strValue)
				case "atm_price":
					strValue := param.ParamValue
					priceATM = fun.ConvertStringToFloat64(strValue)
				case "overdue_price_pm":
					strValue := param.ParamValue
					priceOverduePM = fun.ConvertStringToFloat64(strValue)
				case "not_worked_price_pm":
					strValue := param.ParamValue
					priceUnworkedPM = fun.ConvertStringToFloat64(strValue)
				case "overdue_price_npm":
					strValue := param.ParamValue
					priceOverdueNonPM = fun.ConvertStringToFloat64(strValue)
				case "not_worked_price_npm":
					strValue := param.ParamValue
					priceUnworkedNonPM = fun.ConvertStringToFloat64(strValue)
				}
			}

			amountOverduePM = int64(priceOverduePM * float64(payrollData.PMOver))
			amountOverdueNonPM = int64(priceOverdueNonPM * float64(payrollData.NonPMOver))
			amountUnworkedPM = int64(priceUnworkedPM * float64(payrollData.PMUnworked))
			amountUnworkedNonPM = int64(priceUnworkedNonPM * float64(payrollData.NonPMUnworked))
		}

		if payrollData.PotonganOverduePM > 0 {
			amountOverduePM = int64(payrollData.PotonganOverduePM)
		}
		if payrollData.PotonganOverdueNonPM > 0 {
			amountOverdueNonPM = int64(payrollData.PotonganOverdueNonPM)
		}
		if payrollData.PotonganOverdueUnworkedPM > 0 {
			amountUnworkedPM = int64(payrollData.PotonganOverdueUnworkedPM)
		}
		if payrollData.PotonganOverdueUnworkedNonPM > 0 {
			amountUnworkedNonPM = int64(payrollData.PotonganOverdueUnworkedNonPM)
		}

		// Not used yet, but may be needed in future
		_ = priceBPAKR
		_ = priceATM
	}
	bankTeknisi = payrollData.Bank
	bankNoTeknisi = payrollData.AccNo
	bankAccNameTeknisi = payrollData.AccName

	if namaTeknisi == "" && bankAccNameTeknisi != "" {
		namaTeknisi = bankAccNameTeknisi
	}

	// Employee Information
	addRow("Nama", namaTeknisi,
		"Nama FS", teknisiName,
		true, true,
		"none", "none")
	addRow("Area Service", areaTeknisi,
		"Employee", employeeCodeTeknisi,
		true, true,
		"none", "none")
	addRow("Contract", payrollData.ContractNo,
		"Tanggal Join", tglJoinTeknisi,
		true, true,
		"none", "none") // TODO: Add contract

	// Second horizontal line (below Contract)
	currentY += 2.0
	pdf.Line(marginLeft, currentY, pageWidth-marginRight, currentY)

	// Income Section
	currentY += 3.0
	addRow("Penghasilan", "",
		"JO Yang Dikerjakan", fmt.Sprintf("%.0f", float64(payrollData.JORegular)),
		false, true, // Income doesn't show colon, Job Order Paid does
		"label", "none",
	)

	var basicSalary, incentive, bpAkr, atm int
	var other int
	var totalTransferred int

	basicSalary = int(payrollData.Basic * 2)
	incentive = int(payrollData.TotalIncentives)
	bpAkr = int(payrollData.TotalBP)
	atm = int(payrollData.TotalATM)

	other = int(payrollData.Other)

	// totalTransferred = (basicSalary + incentive + bpAkr + atm) + other
	totalTransferred = (basicSalary + incentive + bpAkr + atm) + other - (int(amountOverduePM) + int(amountOverdueNonPM) + int(amountUnworkedPM) + int(amountUnworkedNonPM))

	addRow("Gaji Pokok", fmt.Sprintf("Rp %s", fun.FormatRupiah(basicSalary)),
		"JO Minimum Target", fmt.Sprintf("%.0f", float64(payrollData.JOTarget)),
		true, true,
		"none", "none",
	)
	addRow("Insentif", fmt.Sprintf("Rp %s", fun.FormatRupiah(incentive)),
		"JO PM Meet", fmt.Sprintf("%.0f", float64(payrollData.PMMeet)),
		true, true,
		"none", "none",
	)
	addRow("Gaji Pokok (50%)", fmt.Sprintf("Rp %s", fun.FormatRupiah(int(payrollData.Basic))),
		"JO Non - PM Meet", fmt.Sprintf("%.0f", float64(payrollData.NonPMMeet)),
		true, true,
		"none", "none",
	)
	addRow("THR/Bonus", fmt.Sprintf("Rp %s", "-"), // TODO: add thr/bonus calculation
		"JO Insentif", fmt.Sprintf("%.0f", float64(payrollData.JOIncentives)),
		true, true,
		"none", "none",
	)
	addRow("Standby", "Rp -",
		"JO Tidak Dikerjakan (Denda)", fmt.Sprintf("%.0f", float64(payrollData.NonPMUnworked+payrollData.PMUnworked)),
		true, true,
		"label", "none",
	)
	addRow("- On Time Presence", "",
		"BP AKR", fmt.Sprintf("%.0f", float64(payrollData.JOBP)),
		true, true,
		"none", "none")
	addRow("- Meal Allowance", "Rp -",
		"JO ATM", fmt.Sprintf("%.0f", float64(payrollData.JOATM)),
		true, true,
		"none", "none")
	addRow("- Project BPR AKR", fmt.Sprintf("Rp %s", fun.FormatRupiah(bpAkr)),
		"All JO", fmt.Sprintf("%.0f", float64(payrollData.JORegular+payrollData.JOBP+payrollData.JOATM)),
		true, true,
		"none", "none",
	)
	addRow("- ATM", fmt.Sprintf("Rp %s", fun.FormatRupiah(atm)),
		"", "",
		true, false,
		"none", "none",
	)
	addRow("Other / Rapel", fmt.Sprintf("Rp %s", fun.FormatRupiah(other)),
		"", "",
		true, false,
		"none", "none",
	)

	// Deduction Section
	currentY += 2.0
	addRow("Pinalti Potongan", "",
		"", "",
		false, false,
		"label", "none")
	// addRow("Loan Instalment", "Rp -",
	// 	"", "",
	// 	true, false,
	// 	"none", "none")
	addRow("Overdue PM", fmt.Sprintf("Rp %s", fun.FormatRupiah(int(amountOverduePM))),
		"JO Overdue PM", fmt.Sprintf("%.0f", float64(payrollData.PMOver)),
		true, true,
		"none", "none",
	)
	addRow("Overdue Non - PM", fmt.Sprintf("Rp %s", fun.FormatRupiah(int(amountOverdueNonPM))),
		"JO Overdue Non - PM", fmt.Sprintf("%.0f", float64(payrollData.NonPMOver)),
		true, true,
		"none", "none",
	)
	addRow("Unworked PM", fmt.Sprintf("Rp %s", fun.FormatRupiah(int(amountUnworkedPM))),
		"JO Unworked PM", fmt.Sprintf("%.0f", float64(payrollData.PMUnworked)),
		true, true,
		"none", "none",
	)
	addRow("Unworked Non - PM", fmt.Sprintf("Rp %s", fun.FormatRupiah(int(amountUnworkedNonPM))),
		"JO Unworked Non - PM", fmt.Sprintf("%.0f", float64(payrollData.NonPMUnworked)),
		true, true,
		"none", "none",
	)
	// addRow("Pending Non - PM", "Rp -",
	// 	"JO Pending Non - PM", "0",
	// 	true, true,
	// 	"none", "none",
	// )
	addRow("Jumlah Yang Diterima", fmt.Sprintf("Rp %s", fun.FormatRupiah(totalTransferred)),
		"", "",
		true, false,
		"all", "",
	)

	// Bank Information and Signature Section (side by side)
	currentY += 8.0
	bankStartY := currentY // Save the starting Y position for HRD section

	// Left side - Bank Information (reuse existing column positions)
	pdf.SetFont("CenturyGothic", "", 9)
	pdf.SetXY(col1LabelX, currentY)
	pdf.Cell(col1ColonX-col1LabelX, 5, "Bank")
	pdf.SetXY(col1ColonX, currentY)
	pdf.Cell(5, 5, ":")
	pdf.SetXY(col1ValueX, currentY)
	pdf.Cell(0, 5, bankTeknisi)

	currentY += 10.0
	pdf.SetXY(col1LabelX, currentY)
	pdf.Cell(col1ColonX-col1LabelX, 5, "No. Rekening")
	pdf.SetXY(col1ColonX, currentY)
	pdf.Cell(5, 5, ":")
	pdf.SetXY(col1ValueX, currentY)
	pdf.Cell(0, 5, bankNoTeknisi)

	currentY += 10.0
	pdf.SetXY(col1LabelX, currentY)
	pdf.Cell(col1ColonX-col1LabelX, 5, "Nama Rekening")
	pdf.SetXY(col1ColonX, currentY)
	pdf.Cell(5, 5, ":")
	pdf.SetXY(col1ValueX, currentY)
	pdf.Cell(0, 5, bankAccNameTeknisi)

	// Right side - Signature Section (aligned with Bank section)
	hrdLabelX := pageWidth/2 + 5.0
	hrdSectionWidth := (pageWidth - marginRight) - hrdLabelX

	// Signature label
	pdf.SetFont("CenturyGothic", "B", 9)
	pdf.SetXY(hrdLabelX, bankStartY)
	pdf.CellFormat(hrdSectionWidth, 5, "Finance", "", 1, "C", false, 0, "")

	// Add signature image (TTD)
	signatureWidth := 30.0
	signatureHeight := 15.0
	signatureX := hrdLabelX + (hrdSectionWidth-signatureWidth)/2 // Center the signature
	signatureY := bankStartY + 6.0
	pdf.ImageOptions(signatureImg, signatureX, signatureY, signatureWidth, signatureHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Add HRD name below signature
	pdf.SetFont("CenturyGothic", "", 9)
	pdf.SetXY(hrdLabelX, signatureY+signatureHeight+2.0)
	pdf.CellFormat(hrdSectionWidth, 5, signatureName, "", 1, "C", false, 0, "")

	// Update currentY to be after both sections
	currentY += 5.0

	/*
		Output
	*/
	if err := pdf.OutputFileAndClose(filePathOutput); err != nil {
		return fmt.Errorf("failed to output PDF file: %v", err)
	}

	return nil
}

func TablePayslipMSEDC() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Draw       int    `form:"draw"`
			Start      int    `form:"start"`
			Length     int    `form:"length"`
			Search     string `form:"search[value]"`
			SortColumn int    `form:"order[0][column]"`
			SortDir    string `form:"order[0][dir]"`

			No       string `form:"no" json:"no"`
			FullName string `form:"full_name" json:"full_name" gorm:"column:full_name"`
		}

		// Bind form data to request struct
		if err := c.Bind(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dbWeb := gormdb.Databases.Web
		t := reflect.TypeOf(odooms.MSTechnicianPayroll{})

		// Initialize the map
		columnMap := make(map[int]string)

		excludedJsonKeys := []string{
			"",
			"-",
			"regenerate_payslip",
			"send_payslip",
			"sender_name",
			"destination_name",
			"whatsapp_conversation",
		}

		// Loop through the fields of the struct
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			for _, excludedKey := range excludedJsonKeys {
				if jsonKey == excludedKey {
					continue
				}
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		if sortColumnName == "" {
			sortColumnName = "id"
		}
		if request.SortDir == "" {
			request.SortDir = "asc"
		}
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := dbWeb.Model(&odooms.MSTechnicianPayroll{})

		// Apply filters
		if request.Search != "" {
			for i := 0; i < t.NumField(); i++ {
				dataField := ""
				field := t.Field(i)
				dataType := field.Type.String()
				jsonKey := field.Tag.Get("json")
				gormTag := field.Tag.Get("gorm")

				// Initialize a variable to hold the column key
				columnKey := ""

				// Manually parse the gorm tag to find the column value
				tags := strings.Split(gormTag, ";")
				for _, tag := range tags {
					if strings.HasPrefix(tag, "column:") {
						columnKey = strings.TrimPrefix(tag, "column:")
						break
					}
				}

				skip := false
				for _, excludedKey := range excludedJsonKeys {
					if jsonKey == excludedKey {
						skip = true
						break
					}
				}
				if skip {
					continue
				}

				if jsonKey == "" {
					if columnKey == "" || columnKey == "-" {
						continue
					} else {
						dataField = columnKey
					}
				} else {
					dataField = jsonKey
				}

				if dataType != "string" {
					continue
				}

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				formKey := field.Tag.Get("json")

				for _, excludedKey := range excludedJsonKeys {
					if formKey == excludedKey {
						continue
					}
				}

				// // AllowedTypes
				// if formKey == "allowed_types" {
				// 	allowedTypes := c.PostFormArray("allowed_types[]")
				// 	if len(allowedTypes) > 0 {
				// 		for _, typ := range allowedTypes {
				// 			jsonFilter, _ := json.Marshal([]string{typ})
				// 			filteredQuery = filteredQuery.Where("JSON_CONTAINS(allowed_types, ?)", string(jsonFilter))
				// 		}
				// 	}
				// 	continue
				// }

				formValue := c.PostForm(formKey)

				if formValue != "" {
					if field.Type.Kind() == reflect.Bool ||
						(field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Bool) {
						// Convert formValue "true"/"false" to bool or int
						boolVal := false
						if formValue == "true" {
							boolVal = true
						}
						filteredQuery = filteredQuery.Where("`"+formKey+"` = ?", boolVal)
					} else {
						// Other fields: use LIKE
						filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
					}
				}
			}
		}

		// Count the total number of records
		var totalRecords int64
		dbWeb.Model(&odooms.MSTechnicianPayroll{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []odooms.MSTechnicianPayroll
		query = query.Offset(request.Start).Limit(request.Length).Find(&Dbdata)

		if query.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"draw":            request.Draw,
				"recordsTotal":    totalRecords,
				"recordsFiltered": 0,
				"data":            []gin.H{},
				"error":           query.Error.Error(),
			})
			return
		}

		var data []gin.H
		for _, dataInDB := range Dbdata {
			newData := make(map[string]interface{})
			v := reflect.ValueOf(dataInDB)

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				fieldValue := v.Field(i)

				// Get the JSON key
				theKey := field.Tag.Get("json")
				if theKey == "" {
					theKey = field.Tag.Get("form")
					if theKey == "" {
						continue
					}
				}

				// Handle data rendered in col
				switch theKey {
				case "birthdate", "date":
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						t := fieldValue.Interface().(time.Time)
						switch theKey {
						case "birthdate":
							newData[theKey] = t.Format(fun.T_YYYYMMDD)
						case "date":
							newData[theKey] = t.Add(7 * time.Hour).Format(fun.T_YYYYMMDD_HHmmss)
						}
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "payslip_sent":
					t := fieldValue.Interface().(bool)
					var boolStr string
					if t {
						boolStr = "<i class='fad fa-check text-success fs-1'></i>"
					} else {
						boolStr = "<i class='fad fa-times text-danger fs-1'></i>"
					}
					newData[theKey] = boolStr

				case "payslip_sent_at":
					t := fieldValue.Interface().(*time.Time)
					if t != nil {
						newData[theKey] = t.Format("02 January 2006 15:04:05")
					} else {
						newData[theKey] = "<div class='text-muted'>Not Sent Yet</div>"
					}

				case "payslip_filepath":
					t := fieldValue.Interface().(string)
					var filePath string
					if t == "" {
						filePath = "<span class='text-danger'>Payslip not created yet</span>"
					} else {
						// Check if the file actually exists
						if _, err := os.Stat(t); os.IsNotExist(err) {
							filePath = "<span class='text-warning'>Payslip file removed or not created</span>"
						} else {
							fileSP := strings.ReplaceAll(t, "web/file/payslip_technician/", "")
							fileSPURL := "/proxy-pdf-slip-gaji-teknisi/" + fileSP
							filePath = fmt.Sprintf(`
							<span class="badge bg-danger" style="cursor: pointer;" onclick="openPDFModelForPDFJS('%s')">
							<i class="fal fa-file-pdf me-1"></i> View PDF
							</span>
							`, fileSPURL)
						}
					}
					newData[theKey] = filePath

				case "regenerate_payslip":
					var regenBtn string
					// Check if payslip file exists
					if dataInDB.PayslipFilepath != "" {
						if _, err := os.Stat(dataInDB.PayslipFilepath); err == nil {
							// File exists, show regenerate button
							regenBtn = fmt.Sprintf(`
							<button class="btn btn-sm btn-warning" onclick="regeneratePayslipTechnician(%d, 'edc')">
								<i class="fas fa-sync-alt me-2"></i> Regenerate
							</button>
							`, dataInDB.ID)
						} else {
							// File doesn't exist, show generate button
							regenBtn = fmt.Sprintf(`
							<button class="btn btn-sm btn-primary" onclick="regeneratePayslipTechnician(%d, 'edc')">
								<i class="fas fa-file-pdf me-2"></i> Generate
							</button>
							`, dataInDB.ID)
						}
					} else {
						// No file path set, show generate button
						regenBtn = fmt.Sprintf(`
						<button class="btn btn-sm btn-primary" onclick="regeneratePayslipTechnician(%d, 'edc')">
							<i class="fas fa-file-pdf me-2"></i> Generate
						</button>
						`, dataInDB.ID)
					}
					newData[theKey] = regenBtn

				case "send_payslip":
					t := fieldValue.Interface().(string)
					var sendBtn string
					if t == "" {
						sendBtn = fmt.Sprintf(`
						<button class="btn btn-sm btn-success" onclick="sendIndividualPayslipTechnician(%d,'edc')">
							<i class="fas fa-share me-2"></i> Send
						</button>
						`, dataInDB.ID)
					} else {
						sendBtn = `<span class="text-success"><i class="fad fa-check-circle me-1"></i> Sent</span>`
					}
					newData[theKey] = sendBtn

				case "whatsapp_conversation":
					var conversationBtn string
					// Check if WhatsApp was sent (has WhatsappChatID)
					if dataInDB.WhatsappChatID != "" {
						conversationBtn = fmt.Sprintf(`
						<button class="btn btn-sm btn-info" onclick="showPayslipWhatsAppConversation(%d, 'edc')">
							<i class="fab fa-whatsapp me-2"></i> View
						</button>
						`, dataInDB.ID)
					} else {
						conversationBtn = `<span class="text-muted"><i class="fad fa-ban me-1"></i> No Conversation</span>`
					}
					newData[theKey] = conversationBtn

				default:
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						t := fieldValue.Interface().(time.Time)
						newData[theKey] = t.Format(fun.T_YYYYMMDD_HHmmss)
					} else {
						newData[theKey] = fieldValue.Interface()
					}
				}

			}

			data = append(data, gin.H(newData))
		}

		// Respond with the formatted data for DataTables
		c.JSON(http.StatusOK, gin.H{
			"draw":            request.Draw,
			"recordsTotal":    totalRecords,
			"recordsFiltered": filteredRecords,
			"data":            data,
		})
	}
}

func UpdateTablePayslipMS(projMS string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()

		// Parse JWT token from cookie
		tokenString, err := c.Cookie("token")
		if err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			fmt.Println("Error during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		var data struct {
			ID    string      `json:"id"`
			Field string      `json:"field"`
			Value interface{} `json:"value"`
		}
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		forbiddenFields := []string{
			"id",
			"created_at",
			"no",
			"contract_no",
			"name",
			// ADD: more
			"uploaded_by",
			"payslip_sent",
			"payslip_sent_at",
			"payslip_filepath",

			"whatsapp_chat_id",
			"whatsapp_sent_at",
			"whatsapp_chat_jid",
			"whatsapp_sender_jid",
			"whatsapp_message_body",
			"whatsapp_message_type",
			"whatsapp_quoted_msg_id",
			"whatsapp_reply_text",
			"whatsapp_reaction_emoji",
			"whatsapp_mentions",
			"whatsapp_is_group",
			"whatsapp_msg_status",
			"whatsapp_replied_by",
			"whatsapp_replied_at",
			"whatsapp_reacted_by",
			"whatsapp_reacted_at",
		}

		switch strings.ToLower(projMS) {
		case "edc":
			// Sanitize value based on field type
			t := reflect.TypeOf(odooms.MSTechnicianPayroll{})
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				if field.Tag.Get("json") == data.Field {
					if field.Type.Kind() == reflect.Float64 {
						if strVal, ok := data.Value.(string); ok {
							val, err := fun.SanitizeCurrency(strVal)
							if err != nil {
								c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid number format for field " + data.Field + ": " + err.Error()})
								return
							}
							data.Value = val
						}
					}
					break
				}
			}

			for _, field := range forbiddenFields {
				if data.Field == field {
					c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden Field"})
					return
				}
			}

			dbWeb := gormdb.Databases.Web

			var manufacture odooms.MSTechnicianPayroll
			if err := dbWeb.Where("id = ?", data.ID).First(&manufacture).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
				return
			}

			// Update the field with the new value
			if err := dbWeb.Model(&manufacture).Update(data.Field, data.Value).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update record"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"msg": fmt.Sprintf("Data of %s updated with value: %v!", data.Field, data.Value)})

			dbWeb.Create(&model.LogActivity{
				AdminID:   uint(claims["id"].(float64)),
				FullName:  claims["fullname"].(string),
				Action:    "PATCH UPDATE",
				Status:    "Success",
				Log:       fmt.Sprintf("UPDATE Manufacture Data By ID: %s; Field : %s; Value: %v; ", data.ID, data.Field, data.Value),
				IP:        c.ClientIP(),
				UserAgent: c.Request.UserAgent(),
				ReqMethod: c.Request.Method,
				ReqUri:    c.Request.RequestURI,
			})

		case "atm":
			// Sanitize value based on field type
			t := reflect.TypeOf(odooms.MSTechnicianPayrollDedicatedATM{})
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				if field.Tag.Get("json") == data.Field {
					if field.Type.Kind() == reflect.Float64 {
						if strVal, ok := data.Value.(string); ok {
							val, err := fun.SanitizeCurrency(strVal)
							if err != nil {
								c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid number format for field " + data.Field + ": " + err.Error()})
								return
							}
							data.Value = val
						}
					}
					break
				}
			}

			for _, field := range forbiddenFields {
				if data.Field == field {
					c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden Field"})
					return
				}
			}

			dbWeb := gormdb.Databases.Web

			var manufacture odooms.MSTechnicianPayrollDedicatedATM
			if err := dbWeb.Where("id = ?", data.ID).First(&manufacture).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
				return
			}

			// Update the field with the new value
			if err := dbWeb.Model(&manufacture).Update(data.Field, data.Value).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update record"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"msg": fmt.Sprintf("Data of %s updated!", data.Field)})

			dbWeb.Create(&model.LogActivity{
				AdminID:   uint(claims["id"].(float64)),
				FullName:  claims["fullname"].(string),
				Action:    "PATCH UPDATE",
				Status:    "Success",
				Log:       fmt.Sprintf("UPDATE Manufacture Data By ID: %s; Field : %s; Value: %s; ", data.ID, data.Field, data.Value),
				IP:        c.ClientIP(),
				UserAgent: c.Request.UserAgent(),
				ReqMethod: c.Request.Method,
				ReqUri:    c.Request.RequestURI,
			})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Project MS"})
			return
		}

	}
}

func DeleteTablePayslipMS(projMS string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the ID from the URL parameter and convert to integer
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
			return
		}
		uid := uint(id)

		dbWeb := gormdb.Databases.Web
		var dbData interface{}
		switch strings.ToLower(projMS) {
		case "edc":
			dbData = odooms.MSTechnicianPayroll{}

		case "atm":
			dbData = odooms.MSTechnicianPayrollDedicatedATM{}

		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Project MS"})
			return
		}

		if err := dbWeb.First(&dbData, uid).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// If the record does not exist, return a 404 error
				c.JSON(http.StatusNotFound, gin.H{"error": "Data not found"})
			} else {
				// Handle other potential errors from the database
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find Data, details: " + err.Error()})
			}
			return
		}

		if err := dbWeb.Delete(&dbData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete Data, details: " + err.Error()})
			return
		}

		// Respond with success
		c.JSON(http.StatusOK, gin.H{"message": "Data deleted successfully"})

		cookies := c.Request.Cookies()

		// Parse JWT token from cookie
		tokenString, err := c.Cookie("token")
		if err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			fmt.Println("Error during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		jsonString := ""
		jsonData, err := json.Marshal(dbData)
		if err != nil {
			fmt.Println("Error converting to JSON:", err)
		} else {
			jsonString = string(jsonData)
		}
		dbWeb.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Action:    "Delete Data",
			Status:    "Success",
			Log:       "Data Berhasil Di Hapus : " + jsonString,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})
	}
}

func TablePayslipMSATM() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Draw       int    `form:"draw"`
			Start      int    `form:"start"`
			Length     int    `form:"length"`
			Search     string `form:"search[value]"`
			SortColumn int    `form:"order[0][column]"`
			SortDir    string `form:"order[0][dir]"`

			No       string `form:"no" json:"no"`
			FullName string `form:"full_name" json:"full_name" gorm:"column:full_name"`
		}

		// Bind form data to request struct
		if err := c.Bind(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dbWeb := gormdb.Databases.Web
		t := reflect.TypeOf(odooms.MSTechnicianPayrollDedicatedATM{})

		// Initialize the map
		columnMap := make(map[int]string)

		excludedJsonKeys := []string{
			"",
			"-",
			"regenerate_payslip",
			"send_payslip",
			"sender_name",
			"destination_name",
			"whatsapp_conversation",
		}

		// Loop through the fields of the struct
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			for _, excludedKey := range excludedJsonKeys {
				if jsonKey == excludedKey {
					continue
				}
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		if sortColumnName == "" {
			sortColumnName = "id"
		}
		if request.SortDir == "" {
			request.SortDir = "asc"
		}
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := dbWeb.Model(&odooms.MSTechnicianPayrollDedicatedATM{})

		// Apply filters
		if request.Search != "" {
			for i := 0; i < t.NumField(); i++ {
				dataField := ""
				field := t.Field(i)
				dataType := field.Type.String()
				jsonKey := field.Tag.Get("json")
				gormTag := field.Tag.Get("gorm")

				// Initialize a variable to hold the column key
				columnKey := ""

				// Manually parse the gorm tag to find the column value
				tags := strings.Split(gormTag, ";")
				for _, tag := range tags {
					if strings.HasPrefix(tag, "column:") {
						columnKey = strings.TrimPrefix(tag, "column:")
						break
					}
				}

				skip := false
				for _, excludedKey := range excludedJsonKeys {
					if jsonKey == excludedKey {
						skip = true
						break
					}
				}
				if skip {
					continue
				}

				if jsonKey == "" {
					if columnKey == "" || columnKey == "-" {
						continue
					} else {
						dataField = columnKey
					}
				} else {
					dataField = jsonKey
				}

				if dataType != "string" {
					continue
				}

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				formKey := field.Tag.Get("json")

				for _, excludedKey := range excludedJsonKeys {
					if formKey == excludedKey {
						continue
					}
				}

				// // AllowedTypes
				// if formKey == "allowed_types" {
				// 	allowedTypes := c.PostFormArray("allowed_types[]")
				// 	if len(allowedTypes) > 0 {
				// 		for _, typ := range allowedTypes {
				// 			jsonFilter, _ := json.Marshal([]string{typ})
				// 			filteredQuery = filteredQuery.Where("JSON_CONTAINS(allowed_types, ?)", string(jsonFilter))
				// 		}
				// 	}
				// 	continue
				// }

				formValue := c.PostForm(formKey)

				if formValue != "" {
					if field.Type.Kind() == reflect.Bool ||
						(field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Bool) {
						// Convert formValue "true"/"false" to bool or int
						boolVal := false
						if formValue == "true" {
							boolVal = true
						}
						filteredQuery = filteredQuery.Where("`"+formKey+"` = ?", boolVal)
					} else {
						// Other fields: use LIKE
						filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
					}
				}
			}
		}

		// Count the total number of records
		var totalRecords int64
		dbWeb.Model(&odooms.MSTechnicianPayrollDedicatedATM{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []odooms.MSTechnicianPayrollDedicatedATM
		query = query.Offset(request.Start).Limit(request.Length).Find(&Dbdata)

		if query.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"draw":            request.Draw,
				"recordsTotal":    totalRecords,
				"recordsFiltered": 0,
				"data":            []gin.H{},
				"error":           query.Error.Error(),
			})
			return
		}

		var data []gin.H
		for _, dataInDB := range Dbdata {
			newData := make(map[string]interface{})
			v := reflect.ValueOf(dataInDB)

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				fieldValue := v.Field(i)

				// Get the JSON key
				theKey := field.Tag.Get("json")
				if theKey == "" {
					theKey = field.Tag.Get("form")
					if theKey == "" {
						continue
					}
				}

				// Handle data rendered in col
				switch theKey {
				case "birthdate", "date":
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						t := fieldValue.Interface().(time.Time)
						switch theKey {
						case "birthdate":
							newData[theKey] = t.Format(fun.T_YYYYMMDD)
						case "date":
							newData[theKey] = t.Add(7 * time.Hour).Format(fun.T_YYYYMMDD_HHmmss)
						}
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "payslip_sent":
					t := fieldValue.Interface().(bool)
					var boolStr string
					if t {
						boolStr = "<i class='fad fa-check text-success fs-1'></i>"
					} else {
						boolStr = "<i class='fad fa-times text-danger fs-1'></i>"
					}
					newData[theKey] = boolStr

				case "payslip_sent_at":
					t := fieldValue.Interface().(*time.Time)
					if t != nil {
						newData[theKey] = t.Format("02 January 2006 15:04:05")
					} else {
						newData[theKey] = "<div class='text-muted'>Not Sent Yet</div>"
					}

				case "payslip_filepath":
					t := fieldValue.Interface().(string)
					var filePath string
					if t == "" {
						filePath = "<span class='text-danger'>Payslip not created yet</span>"
					} else {
						// Check if the file actually exists
						if _, err := os.Stat(t); os.IsNotExist(err) {
							filePath = "<span class='text-warning'>Payslip file removed or not created</span>"
						} else {
							fileSP := strings.ReplaceAll(t, "web/file/payslip_technician/", "")
							fileSPURL := "/proxy-pdf-slip-gaji-teknisi/" + fileSP
							filePath = fmt.Sprintf(`
							<span class="badge bg-danger" style="cursor: pointer;" onclick="openPDFModelForPDFJS('%s')">
							<i class="fal fa-file-pdf me-1"></i> View PDF
							</span>
							`, fileSPURL)
						}
					}
					newData[theKey] = filePath

				case "regenerate_payslip":
					var regenBtn string
					// Check if payslip file exists
					if dataInDB.PayslipFilepath != "" {
						if _, err := os.Stat(dataInDB.PayslipFilepath); err == nil {
							// File exists, show regenerate button
							regenBtn = fmt.Sprintf(`
							<button class="btn btn-sm btn-warning" onclick="regeneratePayslipTechnician(%d, 'atm')">
								<i class="fas fa-sync-alt me-2"></i> Regenerate
							</button>
							`, dataInDB.ID)
						} else {
							// File doesn't exist, show generate button
							regenBtn = fmt.Sprintf(`
							<button class="btn btn-sm btn-primary" onclick="regeneratePayslipTechnician(%d, 'atm')">
								<i class="fas fa-file-pdf me-2"></i> Generate
							</button>
							`, dataInDB.ID)
						}
					} else {
						// No file path set, show generate button
						regenBtn = fmt.Sprintf(`
						<button class="btn btn-sm btn-primary" onclick="regeneratePayslipTechnician(%d, 'atm')">
							<i class="fas fa-file-pdf me-2"></i> Generate
						</button>
						`, dataInDB.ID)
					}
					newData[theKey] = regenBtn

				case "send_payslip":
					t := fieldValue.Interface().(string)
					var sendBtn string
					if t == "" {
						sendBtn = fmt.Sprintf(`
						<button class="btn btn-sm btn-success" onclick="sendIndividualPayslipTechnician(%d, 'atm')">
							<i class="fas fa-share me-2"></i> Send
						</button>
						`, dataInDB.ID)
					} else {
						sendBtn = `<span class="text-success"><i class="fad fa-check-circle me-1"></i> Sent</span>`
					}
					newData[theKey] = sendBtn

				case "whatsapp_conversation":
					var conversationBtn string
					// Check if WhatsApp was sent (has WhatsappChatID)
					if dataInDB.WhatsappChatID != "" {
						conversationBtn = fmt.Sprintf(`
						<button class="btn btn-sm btn-info" onclick="showPayslipWhatsAppConversation(%d, 'atm')">
							<i class="fab fa-whatsapp me-2"></i> View
						</button>
						`, dataInDB.ID)
					} else {
						conversationBtn = `<span class="text-muted"><i class="fad fa-ban me-1"></i> No Conversation</span>`
					}
					newData[theKey] = conversationBtn

				default:
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						t := fieldValue.Interface().(time.Time)
						newData[theKey] = t.Format(fun.T_YYYYMMDD_HHmmss)
					} else {
						newData[theKey] = fieldValue.Interface()
					}
				}

			}

			data = append(data, gin.H(newData))
		}

		// Respond with the formatted data for DataTables
		c.JSON(http.StatusOK, gin.H{
			"draw":            request.Draw,
			"recordsTotal":    totalRecords,
			"recordsFiltered": filteredRecords,
			"data":            data,
		})
	}
}

func LastUpdatePayslipTechnicianEDC() gin.HandlerFunc {
	return func(c *gin.Context) {
		var result struct {
			LastUpdateEDC          string `json:"last_update_edc"`
			LastUpdateEDCBy        string `json:"last_update_edc_by"`
			LastUpdateEDCMonthYear string `json:"last_update_edc_month_year"`
			AllSentEDC             bool   `json:"all_sent_edc"`
		}

		dbWeb := gormdb.Databases.Web
		var lastUpdateEDC time.Time
		var lastUploadedBy string

		err := dbWeb.Model(&odooms.MSTechnicianPayroll{}).
			Select("uploaded_by, updated_at").
			Order("updated_at DESC").
			Limit(1).
			Row().Scan(&lastUploadedBy, &lastUpdateEDC)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if lastUpdateEDC.IsZero() {
			result.LastUpdateEDC = "N/A"
		} else {
			tgl, err := tanggal.Papar(lastUpdateEDC, "Jakarta", tanggal.WIB)
			if err != nil {
				logrus.Errorf("got error: %v", err)
			} else {
				result.LastUpdateEDC = tgl.Format(" ", []tanggal.Format{
					tanggal.Hari,
					tanggal.NamaBulan,
					tanggal.Tahun,
					tanggal.PukulDenganDetik,
					tanggal.ZonaWaktu,
				})
			}

			tgl2, err := tanggal.Papar(time.Now().AddDate(0, -1, 0), "Jakarta", tanggal.WIB)
			if err != nil {
				logrus.Errorf("got error: %v", err)
			} else {
				result.LastUpdateEDCMonthYear = tgl2.Format(" ", []tanggal.Format{
					tanggal.NamaBulan,
					tanggal.Tahun,
				})
			}

			result.LastUpdateEDCBy = lastUploadedBy
		}

		// Check if all payslips are sent
		var unsentCount int64
		dbWeb.Model(&odooms.MSTechnicianPayroll{}).
			Where("payslip_sent = ?", false).
			Count(&unsentCount)
		result.AllSentEDC = unsentCount == 0

		c.JSON(http.StatusOK, result)
	}
}

func LastUpdatePayslipTechnicianATM() gin.HandlerFunc {
	return func(c *gin.Context) {
		var result struct {
			LastUpdateATM          string `json:"last_update_atm"`
			LastUpdateATMBy        string `json:"last_update_atm_by"`
			LastUpdateATMMonthYear string `json:"last_update_atm_month_year"`
			AllSentATM             bool   `json:"all_sent_atm"`
		}

		dbWeb := gormdb.Databases.Web
		var lastUpdateATM time.Time
		var lastUploadedBy string

		err := dbWeb.Model(&odooms.MSTechnicianPayrollDedicatedATM{}).
			Select("uploaded_by, updated_at").
			Order("updated_at DESC").
			Limit(1).
			Row().Scan(&lastUploadedBy, &lastUpdateATM)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if lastUpdateATM.IsZero() {
			result.LastUpdateATM = "N/A"
		} else {
			tgl, err := tanggal.Papar(lastUpdateATM, "Jakarta", tanggal.WIB)
			if err != nil {
				logrus.Errorf("got error: %v", err)
			} else {
				result.LastUpdateATM = tgl.Format(" ", []tanggal.Format{
					tanggal.Hari,
					tanggal.NamaBulan,
					tanggal.Tahun,
					tanggal.PukulDenganDetik,
					tanggal.ZonaWaktu,
				})
			}

			tgl2, err := tanggal.Papar(time.Now().AddDate(0, -1, 0), "Jakarta", tanggal.WIB)
			if err != nil {
				logrus.Errorf("got error: %v", err)
			} else {
				result.LastUpdateATMMonthYear = tgl2.Format(" ", []tanggal.Format{
					tanggal.NamaBulan,
					tanggal.Tahun,
				})
			}
			result.LastUpdateATMBy = lastUploadedBy
		}

		// Check if all payslips are sent
		var unsentCount int64
		dbWeb.Model(&odooms.MSTechnicianPayrollDedicatedATM{}).
			Where("payslip_sent = ?", false).
			Count(&unsentCount)
		result.AllSentATM = unsentCount == 0

		c.JSON(http.StatusOK, result)
	}
}

func RegeneratePDFPayslipMS() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			ID        int    `json:"id" binding:"required"`
			ProjectMS string `json:"project_ms" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/payslip_technician",
			"../web/file/payslip_technician",
			"../../web/file/payslip_technician",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find payslip technician directory"})
			return
		}
		pdfFileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
		if err := os.MkdirAll(pdfFileDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payslip technician directory"})
			return
		}

		dbWeb := gormdb.Databases.Web

		switch strings.ToLower(request.ProjectMS) {
		case "edc":
			var record odooms.MSTechnicianPayroll
			if err := dbWeb.Where("id = ?", request.ID).First(&record).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
				return
			}

			teknisiName := record.Name
			if strings.Contains(teknisiName, "*") {
				teknisiName = strings.ReplaceAll(teknisiName, "*", "(Resigned)")
			}

			pdfFileName := fmt.Sprintf("[EDC]SlipGaji_%s_%v.pdf", teknisiName, time.Now().Format("02Jan2006"))
			pdfFilePath := filepath.Join(pdfFileDir, pdfFileName)

			err := GeneratePDFPayslipTechnicianEDC(record, pdfFilePath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"msg": "Slip gaji teknisi MS EDC berhasil digenerate ulang!"})
		case "atm":
			var record odooms.MSTechnicianPayrollDedicatedATM
			if err := dbWeb.Where("id = ?", request.ID).First(&record).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
				return
			}

			teknisiName := record.Name
			if strings.Contains(teknisiName, "*") {
				teknisiName = strings.ReplaceAll(teknisiName, "*", "(Resigned)")
			}

			pdfFileName := fmt.Sprintf("[ATM]SlipGaji_%s_%v.pdf", teknisiName, time.Now().Format("02Jan2006"))
			pdfFilePath := filepath.Join(pdfFileDir, pdfFileName)

			err := GeneratePDFPayslipTechnicianATM(record, pdfFilePath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"msg": "Slip gaji teknisi MS ATM berhasil digenerate ulang!"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Project MS"})
			return
		}
	}
}

// RegeneratePayslipTechnicianEDC regenerates a single payslip PDF for EDC technician
func RegeneratePayslipTechnicianEDC() gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
			return
		}

		dbWeb := gormdb.Databases.Web
		var record odooms.MSTechnicianPayroll
		if err := dbWeb.Where("id = ?", id).First(&record).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Payslip record not found"})
			return
		}

		// Find the correct directory for payslip technician
		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/payslip_technician",
			"./web/file/payslip_technician",
			"../web/file/payslip_technician",
			"../../web/file/payslip_technician",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find payslip directory"})
			return
		}

		pdfFileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
		if err := os.MkdirAll(pdfFileDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory"})
			return
		}

		teknisiName := record.Name
		if strings.Contains(teknisiName, "*") {
			teknisiName = strings.ReplaceAll(teknisiName, "*", "(Resigned)")
		}

		pdfFileName := fmt.Sprintf("[EDC]SlipGaji_%s_%v.pdf", teknisiName, time.Now().Format("02Jan2006"))
		pdfFilePath := filepath.Join(pdfFileDir, pdfFileName)

		// Generate the PDF
		err = GeneratePDFPayslipTechnicianEDC(record, pdfFilePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate PDF: %v", err)})
			return
		}

		// Update the database with new file path
		if err := dbWeb.Model(&odooms.MSTechnicianPayroll{}).
			Where("id = ?", id).
			Update("payslip_filepath", pdfFilePath).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payslip filepath"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "Payslip regenerated successfully!",
			"filepath": pdfFilePath,
			"filename": pdfFileName,
		})
	}
}

// RegeneratePayslipTechnicianATM regenerates a single payslip PDF for ATM technician
func RegeneratePayslipTechnicianATM() gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
			return
		}

		dbWeb := gormdb.Databases.Web
		var record odooms.MSTechnicianPayrollDedicatedATM
		if err := dbWeb.Where("id = ?", id).First(&record).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Payslip record not found"})
			return
		}

		// Find the correct directory for payslip technician
		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/payslip_technician",
			"./web/file/payslip_technician",
			"../web/file/payslip_technician",
			"../../web/file/payslip_technician",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find payslip directory"})
			return
		}

		pdfFileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
		if err := os.MkdirAll(pdfFileDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory"})
			return
		}

		teknisiName := record.Name
		if strings.Contains(teknisiName, "*") {
			teknisiName = strings.ReplaceAll(teknisiName, "*", "(Resigned)")
		}

		pdfFileName := fmt.Sprintf("[ATM]SlipGaji_%s_%v.pdf", teknisiName, time.Now().Format("02Jan2006"))
		pdfFilePath := filepath.Join(pdfFileDir, pdfFileName)

		// Generate the PDF
		err = GeneratePDFPayslipTechnicianATM(record, pdfFilePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate PDF: %v", err)})
			return
		}

		// Update the database with new file path
		if err := dbWeb.Model(&odooms.MSTechnicianPayrollDedicatedATM{}).
			Where("id = ?", id).
			Update("payslip_filepath", pdfFilePath).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payslip filepath"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "Payslip regenerated successfully!",
			"filepath": pdfFilePath,
			"filename": pdfFileName,
		})
	}
}

// SendIndividualPayslipTechnician sends payslip to individual technician via email or WhatsApp
func SendIndividualPayslipTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			ID         int    `json:"id" binding:"required"`
			ProjectMS  string `json:"project_ms" binding:"required"`
			SendOption string `json:"send_option" binding:"required"` // "email" or "whatsapp"
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate send option
		if request.SendOption != "email" && request.SendOption != "whatsapp" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid send_option. Must be 'email' or 'whatsapp'"})
			return
		}

		dbWeb := gormdb.Databases.Web
		CCEmails := config.WebPanel.Get().ODOOMSParam.PayslipTechnicianCCEmail

		loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
		now := time.Now().In(loc)
		hour := now.Hour()
		// Greeting logic (ensuring correct 24-hour format)
		var greetingID, greetingEN string
		if hour >= 0 && hour < 4 {
			greetingID = "Selamat Dini Hari" // 00:00 - 03:59
			greetingEN = "Good Early Morning"
		} else if hour >= 4 && hour < 12 {
			greetingID = "Selamat Pagi" // 04:00 - 11:59
			greetingEN = "Good Morning"
		} else if hour >= 12 && hour < 15 {
			greetingID = "Selamat Siang" // 12:00 - 14:59
			greetingEN = "Good Afternoon"
		} else if hour >= 15 && hour < 17 {
			greetingID = "Selamat Sore" // 15:00 - 16:59
			greetingEN = "Good Late Afternoon"
		} else if hour >= 17 && hour < 19 {
			greetingID = "Selamat Petang" // 17:00 - 18:59
			greetingEN = "Good Evening"
		} else {
			greetingID = "Selamat Malam" // 19:00 - 23:59
			greetingEN = "Good Night"
		}
		_ = greetingEN // Currently unused

		switch strings.ToLower(request.ProjectMS) {
		case "edc":
			var record odooms.MSTechnicianPayroll
			if err := dbWeb.Where("id = ?", request.ID).First(&record).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Payslip record not found"})
				return
			}

			var namaTeknisi, emailTeknisi, noHpTeknisi string
			var odooTech odooms.ODOOMSTechnicianData
			dbWeb.Where("technician = ?", record.Name).First(&odooTech)

			emailTeknisi = record.Email

			if odooTech.Name != "" {
				namaTeknisi = odooTech.Name
			} else {
				namaTeknisi = record.Name
				if record.Email == "" && odooTech.Email != "" {
					emailTeknisi = odooTech.Email
				}
			}

			noHpTeknisi = record.NoHP
			if odooTech.NoHP != "" {
				noHpTeknisi = odooTech.NoHP
			}

			var payrollDate string
			tgl, err := tanggal.Papar(record.UpdatedAt, "Jakarta", tanggal.WIB)
			if err != nil {
				payrollDate = record.UpdatedAt.Format("January 2006")
			} else {
				payrollDate = tgl.Format(" ", []tanggal.Format{
					tanggal.NamaBulan,
					tanggal.Tahun,
				})
			}

			// Check if payslip file path exists in database
			if record.PayslipFilepath == "" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Payslip file path not found in database. Please regenerate the payslip first."})
				return
			}

			// Check if payslip file exists on disk
			if _, err := os.Stat(record.PayslipFilepath); os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Payslip file not found at: %s. Please regenerate the payslip.", record.PayslipFilepath)})
				return
			}

			if request.SendOption == "email" {
				if record.Email == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Technician email not found in database."})
					return
				}

				emailTemplate := createTemplateEmailForPayslip(greetingID, namaTeknisi, payrollDate)
				if emailTemplate == "" {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create email template."})
					return
				}

				var emailTo []string

				if config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.Active {
					emailTeknisi = config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.EmailUsedForTest
				}

				emailTo = append(emailTo, emailTeknisi)
				emailSubject := fmt.Sprintf("Slip Gaji Teknisi MS EDC - %s", payrollDate)
				emailAttachments := []fun.EmailAttachment{
					{
						FilePath:    record.PayslipFilepath,
						NewFileName: "Slip Gaji " + payrollDate + ".pdf",
					},
				}

				err := fun.TrySendEmail(
					emailTo,
					CCEmails,
					nil,
					emailSubject,
					emailTemplate,
					emailAttachments,
				)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send email to: %s , got error: %v", emailTeknisi, err)})
					return
				} else {
					// Update payslip_sent and payslip_sent_at in database
					now := time.Now()
					dbWeb.Model(&odooms.MSTechnicianPayroll{}).
						Where("id = ?", request.ID).
						Updates(map[string]interface{}{
							"payslip_sent":    true,
							"payslip_sent_at": &now,
						})

					c.JSON(http.StatusOK, gin.H{
						"message":   "Slip gaji berhasil dikirim via Email !",
						"recipient": emailTeknisi,
					})
				}
			} else {
				if noHpTeknisi == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Technician WhatsApp number not found in database."})
					return
				}

				if config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.Active {
					noHpTeknisi = config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.PhoneNumberUsedForTest
				}

				sanitizedPhone, err := fun.SanitizePhoneNumber(noHpTeknisi)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid WhatsApp number: %s", noHpTeknisi)})
					return
				}

				jidStr := fmt.Sprintf("62%s%s", sanitizedPhone, "@s.whatsapp.net")
				originalSenderJID := NormalizeJID(jidStr)

				var sbID, sbEN strings.Builder
				sbID.WriteString(fmt.Sprintf("%s Bapak/Ibu %s, berikut kami lampirkan slip gaji Anda untuk periode %s.\n\n", greetingID, namaTeknisi, payrollDate))
				sbID.WriteString("_Best Regards,_\n")
				sbID.WriteString(config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName + "\n\n")
				sbID.WriteString(fmt.Sprintf("Finance - *%s*", config.WebPanel.Get().Default.PT))

				sbEN.WriteString(fmt.Sprintf("%s Mr/Mrs %s, please find attached your payslip for the period of %s.\n\n", greetingEN, namaTeknisi, payrollDate))
				sbEN.WriteString("_Best Regards,_\n")
				sbEN.WriteString(config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName + "\n\n")
				sbEN.WriteString(fmt.Sprintf("Finance - *%s*", config.WebPanel.Get().Default.PT))
				msgID := sbID.String()
				msgEN := sbEN.String()

				// Send WhatsApp message with attachment, it also update the payslip sent status in DB
				sendLangDocumentMessageForPayslipTechnician("edc", record.Name, originalSenderJID, msgID, msgEN, "id", record.PayslipFilepath)

				c.JSON(http.StatusOK, gin.H{
					"message":   "Slip gaji berhasil dikirim via Whatsapp !",
					"recipient": "+62" + sanitizedPhone,
				})
			}

		case "atm":
			var record odooms.MSTechnicianPayrollDedicatedATM
			if err := dbWeb.Where("id = ?", request.ID).First(&record).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Payslip record not found"})
				return
			}

			var namaTeknisi, emailTeknisi, noHpTeknisi string
			var odooTech odooms.ODOOMSTechnicianData
			dbWeb.Where("technician = ?", record.Name).First(&odooTech)

			emailTeknisi = record.Email

			if odooTech.Name != "" {
				namaTeknisi = odooTech.Name
			} else {
				namaTeknisi = record.Name
				if record.Email == "" && odooTech.Email != "" {
					emailTeknisi = odooTech.Email
				}
			}

			noHpTeknisi = record.NoHP
			if odooTech.NoHP != "" {
				noHpTeknisi = odooTech.NoHP
			}

			var payrollDate string
			tgl, err := tanggal.Papar(record.UpdatedAt, "Jakarta", tanggal.WIB)
			if err != nil {
				payrollDate = record.UpdatedAt.Format("January 2006")
			} else {
				payrollDate = tgl.Format(" ", []tanggal.Format{
					tanggal.NamaBulan,
					tanggal.Tahun,
				})
			}

			// Check if payslip file path exists in database
			if record.PayslipFilepath == "" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Payslip file path not found in database. Please regenerate the payslip first."})
				return
			}

			// Check if payslip file exists on disk
			if _, err := os.Stat(record.PayslipFilepath); os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Payslip file not found at: %s. Please regenerate the payslip.", record.PayslipFilepath)})
				return
			}

			if request.SendOption == "email" {
				if record.Email == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Technician email not found in database."})
					return
				}

				emailTemplate := createTemplateEmailForPayslip(greetingID, namaTeknisi, payrollDate)
				if emailTemplate == "" {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create email template."})
					return
				}

				var emailTo []string

				if config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.Active {
					emailTeknisi = config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.EmailUsedForTest
				}

				emailTo = append(emailTo, emailTeknisi)
				emailSubject := fmt.Sprintf("Slip Gaji Teknisi MS ATM - %s", payrollDate)
				emailAttachments := []fun.EmailAttachment{
					{
						FilePath:    record.PayslipFilepath,
						NewFileName: "Slip Gaji " + payrollDate + ".pdf",
					},
				}

				err := fun.TrySendEmail(
					emailTo,
					CCEmails,
					nil,
					emailSubject,
					emailTemplate,
					emailAttachments,
				)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send email to: %s , got error: %v", emailTeknisi, err)})
					return
				} else {
					// Update payslip_sent and payslip_sent_at in database
					now := time.Now()
					dbWeb.Model(&odooms.MSTechnicianPayrollDedicatedATM{}).
						Where("id = ?", request.ID).
						Updates(map[string]interface{}{
							"payslip_sent":    true,
							"payslip_sent_at": &now,
						})

					c.JSON(http.StatusOK, gin.H{
						"message":   "Slip gaji berhasil dikirim via Email !",
						"recipient": emailTeknisi,
					})
				}
			} else {
				if noHpTeknisi == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Technician WhatsApp number not found in database."})
					return
				}

				if config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.Active {
					noHpTeknisi = config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.PhoneNumberUsedForTest
				}

				sanitizedPhone, err := fun.SanitizePhoneNumber(noHpTeknisi)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid WhatsApp number: %s", noHpTeknisi)})
					return
				}

				jidStr := fmt.Sprintf("62%s%s", sanitizedPhone, "@s.whatsapp.net")
				originalSenderJID := NormalizeJID(jidStr)

				var sbID, sbEN strings.Builder
				sbID.WriteString(fmt.Sprintf("%s Bapak/Ibu %s, berikut kami lampirkan slip gaji Anda untuk periode %s.\n\n", greetingID, namaTeknisi, payrollDate))
				sbID.WriteString("_Best Regards,_\n")
				sbID.WriteString(config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName + "\n\n")
				sbID.WriteString(fmt.Sprintf("Finance - *%s*", config.WebPanel.Get().Default.PT))

				sbEN.WriteString(fmt.Sprintf("%s Mr/Mrs %s, please find attached your payslip for the period of %s.\n\n", greetingEN, namaTeknisi, payrollDate))
				sbEN.WriteString("_Best Regards,_\n")
				sbEN.WriteString(config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName + "\n\n")
				sbEN.WriteString(fmt.Sprintf("Finance - *%s*", config.WebPanel.Get().Default.PT))
				msgID := sbID.String()
				msgEN := sbEN.String()

				// Send WhatsApp message with attachment, it also update the payslip sent status in DB
				sendLangDocumentMessageForPayslipTechnician("atm", record.Name, originalSenderJID, msgID, msgEN, "id", record.PayslipFilepath)

				c.JSON(http.StatusOK, gin.H{
					"message":   "Slip gaji berhasil dikirim via Whatsapp !",
					"recipient": "+62" + sanitizedPhone,
				})
			}

		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Project MS. Must be 'edc' or 'atm'"})
			return
		}
	}
}

func createTemplateEmailForPayslip(greeting, namaTeknisi, payrollDate string) string {
	var sb strings.Builder
	sb.WriteString("<mjml>")
	sb.WriteString(`
		<mj-head>
			<mj-preview>Slip gaji . . .</mj-preview>
			<mj-style inline="inline">
			.body-section {
				background-color: #ffffff;
				padding: 30px;
				border-radius: 12px;
				box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
			}
			.footer-text {
				color: #6b7280;
				font-size: 12px;
				text-align: center;
				padding-top: 10px;
				border-top: 1px solid #e5e7eb;
			}
			.header-title {
				font-size: 66px;
				font-weight: bold;
				color: #1E293B;
				text-align: left;
			}
			.cta-button {
				background-color: #6D28D9;
				color: #ffffff;
				padding: 12px 24px;
				border-radius: 8px;
				font-size: 16px;
				font-weight: bold;
				text-align: center;
				display: inline-block;
			}
			.email-info {
				color: #374151;
				font-size: 16px;
			}
			</mj-style>
		</mj-head>`)

	sb.WriteString(fmt.Sprintf(`
		<mj-body background-color="#f8fafc">
			<!-- Main Content -->
			<mj-section css-class="body-section" padding="20px">
			<mj-column>
				<mj-text font-size="20px" color="#1E293B" font-weight="bold">Yth. Sdr(i) %s</mj-text>
				<mj-text font-size="16px" color="#4B5563" line-height="1.6">
					%s Bapak/Ibu <b>%s</b>, berikut kami lampirkan slip gaji Anda untuk periode %s.
				</mj-text>

				<mj-divider border-color="#e5e7eb"></mj-divider>

				<mj-text font-size="16px" color="#374151">
				Best Regards,<br>
				<b><i>%s</i></b>
				</mj-text>
			</mj-column>
			</mj-section>

			<!-- Footer -->
			<mj-section>
			<mj-column>
				<mj-text css-class="footer-text">
				⚠ This is an automated email. Please do not reply directly.
				</mj-text>
				<mj-text font-size="12px" color="#6b7280">
				<b>Finance - %s.</b><br>
				<!--
				<br>
				<a href="wa.me/%v">
				📞 Support
				</a>
				-->
				</mj-text>
			</mj-column>
			</mj-section>

		</mj-body>
		`,
		strings.ToUpper(namaTeknisi),
		greeting,
		namaTeknisi,
		payrollDate,
		config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName,
		config.WebPanel.Get().Default.PT,
		"+6287883507445",
	))
	sb.WriteString("</mjml>")

	mjmlTemplate := sb.String()

	return mjmlTemplate
}

// SendAllPayslipTechnician sends all payslips via email or WhatsApp
func SendAllPayslipTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			ProjectMS  string `json:"project_ms" binding:"required"`  // "edc" or "atm"
			SendOption string `json:"send_option" binding:"required"` // "email" or "whatsapp"
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate send option
		if request.SendOption != "email" && request.SendOption != "whatsapp" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid send_option. Must be 'email' or 'whatsapp'"})
			return
		}

		dbWeb := gormdb.Databases.Web
		CCEmails := config.WebPanel.Get().ODOOMSParam.PayslipTechnicianCCEmail
		successLogs := []string{}
		failedLogs := []string{}
		totalSent := 0

		loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
		now := time.Now().In(loc)
		hour := now.Hour()
		// Greeting logic
		var greetingID, greetingEN string
		if hour >= 0 && hour < 4 {
			greetingID = "Selamat Dini Hari"
			greetingEN = "Good Early Morning"
		} else if hour >= 4 && hour < 12 {
			greetingID = "Selamat Pagi"
			greetingEN = "Good Morning"
		} else if hour >= 12 && hour < 15 {
			greetingID = "Selamat Siang"
			greetingEN = "Good Afternoon"
		} else if hour >= 15 && hour < 17 {
			greetingID = "Selamat Sore"
			greetingEN = "Good Late Afternoon"
		} else if hour >= 17 && hour < 19 {
			greetingID = "Selamat Petang"
			greetingEN = "Good Evening"
		} else {
			greetingID = "Selamat Malam"
			greetingEN = "Good Night"
		}
		_ = greetingEN

		switch strings.ToLower(request.ProjectMS) {
		case "edc":
			var records []odooms.MSTechnicianPayroll
			dbWeb.Where("payslip_sent = ?", false).Find(&records)

			for _, record := range records {
				var namaTeknisi, emailTeknisi, noHpTeknisi string
				var odooTech odooms.ODOOMSTechnicianData
				dbWeb.Where("technician = ?", record.Name).First(&odooTech)

				emailTeknisi = record.Email
				if odooTech.Name != "" {
					namaTeknisi = odooTech.Name
				} else {
					namaTeknisi = record.Name
					if record.Email == "" && odooTech.Email != "" {
						emailTeknisi = odooTech.Email
					}
				}

				noHpTeknisi = record.NoHP
				if odooTech.NoHP != "" {
					noHpTeknisi = odooTech.NoHP
				}

				var payrollDate string
				tgl, err := tanggal.Papar(record.UpdatedAt, "Jakarta", tanggal.WIB)
				if err != nil {
					payrollDate = record.UpdatedAt.Format("January 2006")
				} else {
					payrollDate = tgl.Format(" ", []tanggal.Format{
						tanggal.NamaBulan,
						tanggal.Tahun,
					})
				}

				// Check if payslip file exists
				if record.PayslipFilepath == "" {
					failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: Payslip file not found", record.Name))
					continue
				}
				if _, err := os.Stat(record.PayslipFilepath); err != nil {
					failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: Payslip file not found", record.Name))
					continue
				}

				if request.SendOption == "email" {
					if emailTeknisi == "" {
						failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: Email not found", record.Name))
						continue
					}

					emailTemplate := createTemplateEmailForPayslip(greetingID, namaTeknisi, payrollDate)
					if emailTemplate == "" {
						failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: Could not create email template", record.Name))
						continue
					}

					var emailTo []string

					if config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.Active {
						emailTeknisi = config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.EmailUsedForTest
					}

					emailTo = append(emailTo, emailTeknisi)
					emailSubject := fmt.Sprintf("Slip Gaji Teknisi MS EDC - %s", payrollDate)
					emailAttachments := []fun.EmailAttachment{
						{
							FilePath:    record.PayslipFilepath,
							NewFileName: "Slip Gaji " + payrollDate + ".pdf",
						},
					}

					err := fun.TrySendEmail(emailTo, CCEmails, nil, emailSubject, emailTemplate, emailAttachments)
					if err != nil {
						failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s (%s): %v", record.Name, emailTeknisi, err))
					} else {
						now := time.Now()
						dbWeb.Model(&odooms.MSTechnicianPayroll{}).
							Where("id = ?", record.ID).
							Updates(map[string]interface{}{
								"payslip_sent":    true,
								"payslip_sent_at": &now,
							})
						successLogs = append(successLogs, fmt.Sprintf("Sent to %s (%s) successfully", record.Name, emailTeknisi))
						totalSent++
					}
				} else {
					if noHpTeknisi == "" {
						failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: WhatsApp number not found", record.Name))
						continue
					}

					if config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.Active {
						noHpTeknisi = config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.PhoneNumberUsedForTest
					}

					sanitizedPhone, err := fun.SanitizePhoneNumber(noHpTeknisi)
					if err != nil {
						failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: Invalid WhatsApp number %s", record.Name, noHpTeknisi))
						continue
					}

					jidStr := fmt.Sprintf("62%s%s", sanitizedPhone, "@s.whatsapp.net")
					originalSenderJID := NormalizeJID(jidStr)

					var sbID, sbEN strings.Builder
					sbID.WriteString(fmt.Sprintf("%s Bapak/Ibu %s, berikut kami lampirkan slip gaji Anda untuk periode %s.\n\n", greetingID, namaTeknisi, payrollDate))
					sbID.WriteString("_Best Regards,_\n")
					sbID.WriteString(config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName + "\n\n")
					sbID.WriteString(fmt.Sprintf("Finance - *%s*", config.WebPanel.Get().Default.PT))

					sbEN.WriteString(fmt.Sprintf("%s Mr/Mrs %s, please find attached your payslip for the period of %s.\n\n", greetingEN, namaTeknisi, payrollDate))
					sbEN.WriteString("_Best Regards,_\n")
					sbEN.WriteString(config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName + "\n\n")
					sbEN.WriteString(fmt.Sprintf("Finance - *%s*", config.WebPanel.Get().Default.PT))
					msgID := sbID.String()
					msgEN := sbEN.String()

					sendLangDocumentMessageForPayslipTechnician("edc", record.Name, originalSenderJID, msgID, msgEN, "id", record.PayslipFilepath)
					successLogs = append(successLogs, fmt.Sprintf("Sent to %s (+62%s) successfully", record.Name, sanitizedPhone))
					totalSent++
				}
			}

			methodName := "email"
			if request.SendOption == "whatsapp" {
				methodName = "WhatsApp"
			}

			c.JSON(http.StatusOK, gin.H{
				"message":      fmt.Sprintf("All EDC payslips sent via %s!", methodName),
				"total_sent":   totalSent,
				"success_logs": successLogs,
				"failed_logs":  failedLogs,
			})

		case "atm":
			var records []odooms.MSTechnicianPayrollDedicatedATM
			dbWeb.Where("payslip_sent = ?", false).Find(&records)

			for _, record := range records {
				var namaTeknisi, emailTeknisi, noHpTeknisi string
				var odooTech odooms.ODOOMSTechnicianData
				dbWeb.Where("technician = ?", record.Name).First(&odooTech)

				emailTeknisi = record.Email
				if odooTech.Name != "" {
					namaTeknisi = odooTech.Name
				} else {
					namaTeknisi = record.Name
					if record.Email == "" && odooTech.Email != "" {
						emailTeknisi = odooTech.Email
					}
				}

				noHpTeknisi = record.NoHP
				if odooTech.NoHP != "" {
					noHpTeknisi = odooTech.NoHP
				}

				var payrollDate string
				tgl, err := tanggal.Papar(record.UpdatedAt, "Jakarta", tanggal.WIB)
				if err != nil {
					payrollDate = record.UpdatedAt.Format("January 2006")
				} else {
					payrollDate = tgl.Format(" ", []tanggal.Format{
						tanggal.NamaBulan,
						tanggal.Tahun,
					})
				}

				// Check if payslip file exists
				if record.PayslipFilepath == "" {
					failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: Payslip file not found", record.Name))
					continue
				}
				if _, err := os.Stat(record.PayslipFilepath); err != nil {
					failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: Payslip file not found", record.Name))
					continue
				}

				if request.SendOption == "email" {
					if emailTeknisi == "" {
						failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: Email not found", record.Name))
						continue
					}

					emailTemplate := createTemplateEmailForPayslip(greetingID, namaTeknisi, payrollDate)
					if emailTemplate == "" {
						failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: Could not create email template", record.Name))
						continue
					}

					var emailTo []string

					if config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.Active {
						emailTeknisi = config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.EmailUsedForTest
					}

					emailTo = append(emailTo, emailTeknisi)
					emailSubject := fmt.Sprintf("Slip Gaji Teknisi MS ATM - %s", payrollDate)
					emailAttachments := []fun.EmailAttachment{
						{
							FilePath:    record.PayslipFilepath,
							NewFileName: "Slip Gaji " + payrollDate + ".pdf",
						},
					}

					err := fun.TrySendEmail(emailTo, CCEmails, nil, emailSubject, emailTemplate, emailAttachments)
					if err != nil {
						failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s (%s): %v", record.Name, emailTeknisi, err))
					} else {
						now := time.Now()
						dbWeb.Model(&odooms.MSTechnicianPayrollDedicatedATM{}).
							Where("id = ?", record.ID).
							Updates(map[string]interface{}{
								"payslip_sent":    true,
								"payslip_sent_at": &now,
							})
						successLogs = append(successLogs, fmt.Sprintf("Sent to %s (%s) successfully", record.Name, emailTeknisi))
						totalSent++
					}
				} else {
					if noHpTeknisi == "" {
						failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: WhatsApp number not found", record.Name))
						continue
					}

					if config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.Active {
						noHpTeknisi = config.WebPanel.Get().ODOOMSParam.PayslipTechnicianDebug.PhoneNumberUsedForTest
					}

					sanitizedPhone, err := fun.SanitizePhoneNumber(noHpTeknisi)
					if err != nil {
						failedLogs = append(failedLogs, fmt.Sprintf("Failed to send to %s: Invalid WhatsApp number %s", record.Name, noHpTeknisi))
						continue
					}

					jidStr := fmt.Sprintf("62%s%s", sanitizedPhone, "@s.whatsapp.net")
					originalSenderJID := NormalizeJID(jidStr)

					var sbID, sbEN strings.Builder
					sbID.WriteString(fmt.Sprintf("%s Bapak/Ibu %s, berikut kami lampirkan slip gaji Anda untuk periode %s.\n\n", greetingID, namaTeknisi, payrollDate))
					sbID.WriteString("_Best Regards,_\n")
					sbID.WriteString(config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName + "\n\n")
					sbID.WriteString(fmt.Sprintf("Finance - *%s*", config.WebPanel.Get().Default.PT))

					sbEN.WriteString(fmt.Sprintf("%s Mr/Mrs %s, please find attached your payslip for the period of %s.\n\n", greetingEN, namaTeknisi, payrollDate))
					sbEN.WriteString("_Best Regards,_\n")
					sbEN.WriteString(config.WebPanel.Get().ODOOMSParam.PayslipTechnicianSignatureName + "\n\n")
					sbEN.WriteString(fmt.Sprintf("Finance - *%s*", config.WebPanel.Get().Default.PT))
					msgID := sbID.String()
					msgEN := sbEN.String()

					sendLangDocumentMessageForPayslipTechnician("atm", record.Name, originalSenderJID, msgID, msgEN, "id", record.PayslipFilepath)
					successLogs = append(successLogs, fmt.Sprintf("Sent to %s (+62%s) successfully", record.Name, sanitizedPhone))
					totalSent++
				}
			}

			methodName := "email"
			if request.SendOption == "whatsapp" {
				methodName = "WhatsApp"
			}

			c.JSON(http.StatusOK, gin.H{
				"message":      fmt.Sprintf("All ATM payslips sent via %s!", methodName),
				"total_sent":   totalSent,
				"success_logs": successLogs,
				"failed_logs":  failedLogs,
			})

		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Project MS. Must be 'edc' or 'atm'"})
			return
		}
	}
}

// GetPayslipWhatsAppConversation retrieves WhatsApp conversation for a specific payslip
func GetPayslipWhatsAppConversation() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			ID        uint   `json:"id" binding:"required"`
			ProjectMS string `json:"project_ms" binding:"required"` // "edc" or "atm"
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dbWeb := gormdb.Databases.Web

		switch strings.ToLower(request.ProjectMS) {
		case "edc":
			var record odooms.MSTechnicianPayroll
			if err := dbWeb.Where("id = ?", request.ID).First(&record).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Payslip record not found"})
				return
			}

			// Check if WhatsApp conversation exists
			if record.WhatsappChatID == "" {
				c.JSON(http.StatusNotFound, gin.H{"error": "No WhatsApp conversation found"})
				return
			}

			// Get technician name for display
			var odooTech odooms.ODOOMSTechnicianData
			dbWeb.Where("technician = ?", record.Name).First(&odooTech)

			destinationName := record.Name
			if odooTech.Name != "" {
				destinationName = odooTech.Name
			}

			// Format the conversation as an array (similar to SP WhatsApp modal format)
			conversation := []gin.H{
				{
					"whatsapp_message_body":    record.WhatsappMessageBody,
					"whatsapp_message_sent_to": record.NoHP,
					"destination_name":         destinationName,
					"whatsapp_sent_at":         record.WhatsappSentAt,
					"whatsapp_reply_text":      record.WhatsappReplyText,
					"whatsapp_replied_by":      record.WhatsappRepliedBy,
					"sender_name":              destinationName,
					"whatsapp_replied_at":      record.WhatsappRepliedAt,
				},
			}

			c.JSON(http.StatusOK, gin.H{
				"success":      true,
				"conversation": conversation,
			})

		case "atm":
			var record odooms.MSTechnicianPayrollDedicatedATM
			if err := dbWeb.Where("id = ?", request.ID).First(&record).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Payslip record not found"})
				return
			}

			// Check if WhatsApp conversation exists
			if record.WhatsappChatID == "" {
				c.JSON(http.StatusNotFound, gin.H{"error": "No WhatsApp conversation found"})
				return
			}

			// Get technician name for display
			var odooTech odooms.ODOOMSTechnicianData
			dbWeb.Where("technician = ?", record.Name).First(&odooTech)

			destinationName := record.Name
			if odooTech.Name != "" {
				destinationName = odooTech.Name
			}

			// Format the conversation as an array (similar to SP WhatsApp modal format)
			conversation := []gin.H{
				{
					"whatsapp_message_body":    record.WhatsappMessageBody,
					"whatsapp_message_sent_to": record.NoHP,
					"destination_name":         destinationName,
					"whatsapp_sent_at":         record.WhatsappSentAt,
					"whatsapp_reply_text":      record.WhatsappReplyText,
					"whatsapp_replied_by":      record.WhatsappRepliedBy,
					"sender_name":              destinationName,
					"whatsapp_replied_at":      record.WhatsappRepliedAt,
				},
			}

			c.JSON(http.StatusOK, gin.H{
				"success":      true,
				"conversation": conversation,
			})

		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Project MS. Must be 'edc' or 'atm'"})
			return
		}
	}
}

// ValidateODOOCredentials validates ODOO login credentials without processing any files
// This endpoint is used to verify credentials before uploading Excel files
func ValidateODOOCredentials() gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.PostForm("email")
		password := c.PostForm("password")

		// Validate required fields
		if email == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Email cannot be empty!",
			})
			return
		}

		if password == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Password cannot be empty!",
			})
			return
		}

		// Try to get ODOO session cookies
		loginCookie, err := GetODOOMSCookies(email, password)
		if err != nil {
			logrus.Errorf("ODOO credential validation failed for email %s: %v", email, err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid ODOO credentials. Please check your email and password.",
				"details": err.Error(),
			})
			return
		}

		// Validation successful - cookies obtained
		if loginCookie == nil {
			logrus.Warnf("ODOO login returned empty cookies for email %s", email)
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid ODOO credentials. Authentication failed.",
			})
			return
		}

		logrus.Infof("ODOO credentials validated successfully for email: %s", email)
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "ODOO credentials are valid.",
		})
	}
}

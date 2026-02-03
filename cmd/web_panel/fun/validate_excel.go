package fun

import (
	"fmt"
	"mime/multipart"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

func ValidateExcelFile(file multipart.File, filename string) (bool, error) {
	// Check file extension
	ext := filepath.Ext(filename)
	if ext != ".xlsx" && ext != ".xls" {
		return false, fmt.Errorf("invalid file type: %s", ext)
	}

	// Try to open the Excel file using excelize
	excelFile, err := excelize.OpenReader(file)
	if err != nil {
		return false, fmt.Errorf("failed to parse Excel file: %v", err)
	}

	// Check if the sheet exists
	sheets := excelFile.GetSheetList()
	if len(sheets) == 0 {
		return false, fmt.Errorf("%s", "excel file has no sheets")
	}

	// Successfully parsed, return valid
	return true, nil
}

func SheetTotalLenRow(file multipart.File) (int, error) {
	// Open the Excel file
	excelFile, err := excelize.OpenReader(file)
	if err != nil {
		return 0, fmt.Errorf("failed to parse Excel file: %v", err)
	}

	// Get the first sheet name
	sheets := excelFile.GetSheetList()
	if len(sheets) == 0 {
		return 0, fmt.Errorf("%s", "excel file has no sheets")
	}
	sheetName := sheets[0] // Use the first sheet

	// Get all rows from the sheet
	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		return 0, fmt.Errorf("failed to read rows: %v", err)
	}

	// Return the total row count
	return len(rows), nil
}

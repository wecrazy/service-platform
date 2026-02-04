package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/controllers"
	"service-platform/cmd/web_panel/fun"
	"service-platform/internal/config"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateReportAIERROR(t *testing.T) {
	// ADD DB !!!!!!!!!!

	loc, err := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	assert.NoError(t, err, "Failed to load zone "+config.WebPanel.Get().Default.Timezone)

	now := time.Now().In(loc)
	reportName := fmt.Sprintf("(TEST)Report_AI_ERROR_%v", now.Format("02Jan2006_15_04_05.xlsx"))
	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/ai_error",
		"../web/file/ai_error",
		"../../web/file/ai_error",
	})
	assert.NoErrorf(t, err, "Failed to generate report %s", reportName)

	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	err = os.MkdirAll(fileReportDir, 0755)
	assert.NoErrorf(t, err, "Failed to create report directory %s", fileReportDir)

	_, err = os.Stat(fileReportDir)
	assert.NoError(t, err, "Report directory does not exist after creation")

	excelFilePath, id, en := controllers.GenerateExcelForReportAIError(fileReportDir, reportName)
	assert.Empty(t, id, "Expected empty id for error case")
	assert.Empty(t, en, "Expected empty en for error case")

	// 🧹 Cleanup: remove excel report
	err = os.Remove(excelFilePath)
	assert.NoErrorf(t, err, "Failed to remove excel file %s", excelFilePath)
}

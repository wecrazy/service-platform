package tests

import (
	"service-platform/cmd/web_panel/fun"
	"testing"
)

// go test -v -timeout 60m ./tests/removeOldDir_test.go

func TestRemoveOldDir(t *testing.T) {
	selectedDir, err := fun.FindValidDirectory([]string{
		"web/file/monitoring_ticket",
		"../web/file/monitoring_ticket",
		"../../web/file/monitoring_ticket",
	})
	if err != nil {
		t.Fatalf("Failed to find valid directory: %v", err)
	}

	dateDirFormat := "2006-01-02"
	err = fun.RemoveExistingDirectory(selectedDir, "-1Week", dateDirFormat)
	if err != nil {
		t.Errorf("Failed to remove old directories: %v", err)
	} else {
		t.Logf("Old directories removed successfully in %s", selectedDir)
	}
}

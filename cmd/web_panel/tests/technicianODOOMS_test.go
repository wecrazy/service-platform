package tests

import (
	"service-platform/cmd/web_panel/controllers"
	"testing"
)

// go test -v -timeout 60m ./tests/technicianODOOMS_test.go

func TestParsedTechnicianODOOMS(t *testing.T) {
	technicians := []string{
		"3.9 Pasuruan Renaldo Renney",
		"3.9 Lumajang Fio Baitul",
		"4.2 Indramayu Daniel Eka Putu Oka",
		"5.1 Support Semarang",
		"5.2 SPL Yogya Bagas",
		"2.3 Bekasi M Ibnu",
		"3.11 Makassar Bardo",
		"3.3 Denpasar Kresna Mahaditya",
		"3.5 Karangasem Mangku",
		"4.2 Kuningan Ryan Habib",
		"5.2 Bantul Yono",
		"5.5 Brebes Deni Seftian",
		"Vendor Mdm",
		"4.3 Garut Muhammad silmi",
		"2.2 Cikampek Ahamad Madjazi",
	}

	for _, tech := range technicians {
		dataParsed := controllers.ParsedDataTechnicianODOOMS(tech)
		if dataParsed == nil {
			t.Errorf("Failed to parse technician string: %s", tech)
			continue
		}
		t.Logf("Input: %-60s => Group: %-5s | City: %-15s | Name: %s", tech, dataParsed.Group, dataParsed.City, dataParsed.Name)
	}
}

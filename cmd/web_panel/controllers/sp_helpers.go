package controllers

import sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"

// Helper functions to get pelanggaran and nomor surat by SP number

func getPelanggaranByNumber(spNumber int, sp sptechnicianmodel.TechnicianGotSP) string {
	switch spNumber {
	case 1:
		return sp.PelanggaranSP1
	case 2:
		return sp.PelanggaranSP2
	case 3:
		return sp.PelanggaranSP3
	default:
		return ""
	}
}

func getNoSuratByNumber(spNumber int, sp sptechnicianmodel.TechnicianGotSP) int {
	switch spNumber {
	case 1:
		return sp.NoSP1
	case 2:
		return sp.NoSP2
	case 3:
		return sp.NoSP3
	default:
		return 0
	}
}

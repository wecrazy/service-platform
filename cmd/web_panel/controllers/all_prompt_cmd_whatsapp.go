package controllers

import (
	"fmt"
	"service-platform/internal/config"
	"strings"
	"sync"

	"go.mau.fi/whatsmeow/types/events"
)

var showAllCommandListMutex sync.Mutex

type CommandforBotWhatsapp struct {
	Command string
	IDDesc  string
	ENDesc  string
}

type FileUploadedforBotWhatsapp struct {
	FileName     string
	FileType     string
	MaxSize      int64
	IDDesc       string
	ENDesc       string
	TemplateFile string
}

func AllCMDWhatsapp(v *events.Message, userLang string) {
	eventToDo := "Showing list of all commands in WhatsApp"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if !showAllCommandListMutex.TryLock() {
		id := fmt.Sprintf("⚠ Permintaan %s sedang diproses. Mohon tunggu sebentar.", eventToDo)
		en := fmt.Sprintf("⚠ Your %s request is being processed. Please wait a moment.", eventToDo)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer showAllCommandListMutex.Unlock()

	commands := []CommandforBotWhatsapp{
		{Command: "/all-cmd", IDDesc: "📋 Daftar semua perintah yang tersedia di WhatsApp", ENDesc: "📋 List of all available commands on WhatsApp"},
		{Command: "ping", IDDesc: "🏓 Cek koneksi WhatsApp Bot", ENDesc: "🏓 Check WhatsApp Bot connection"},
		{Command: "/form-request", IDDesc: "📝 Permintaan Formulir (untuk mengirim formulir)", ENDesc: "📝 Form request (to send a form)"},
		{Command: "/cs", IDDesc: "👨‍💼 Layanan Pelanggan (untuk menghubungi customer service)", ENDesc: "👨‍💼 Customer Service (to contact customer service)"},
		{Command: "/logout-cs", IDDesc: "🚪 Keluar dari sesi Customer Service", ENDesc: "🚪 Logout from Customer Service session"},
		{Command: "report mr oliver", IDDesc: "🧑‍💼 Laporan ke Mr. Oliver", ENDesc: "🧑‍💼 Report to Mr. Oliver"},
		{Command: "generate report ta", IDDesc: "📊 Buat laporan TA", ENDesc: "📊 Generate TA report"},
		{Command: "generate report tech error", IDDesc: "🛠️ Buat laporan kesalahan teknis", ENDesc: "🛠️ Generate technical error report"},
		{Command: "generate report compared", IDDesc: "📑 Buat laporan perbandingan data TA dengan data ODOO bulan berjalan hingga 3 bulan sebelumnya", ENDesc: "📑 Generate a comparison report of TA data with ODOO data from the current month up to the previous 3 months"},
		{Command: "generate report ai error", IDDesc: "🤖 Buat laporan kesalahan AI", ENDesc: "🤖 Generate AI error report"},
		{Command: "show status vm odoo dashboard", IDDesc: "📈 Tampilkan status VM Odoo Dashboard", ENDesc: "📈 Show VM Odoo Dashboard status"},
		{Command: "restart mysql vm odoo dashboard", IDDesc: "🔄 Restart MySQL VM Odoo Dashboard", ENDesc: "🔄 Restart MySQL VM Odoo Dashboard"},
	}

	var sbID strings.Builder
	var sbEN strings.Builder

	sbID.WriteString("✨ *Daftar semua perintah yang tersedia di WhatsApp:*\n\n")
	sbEN.WriteString("✨ *List of all available commands in WhatsApp:*\n\n")

	for _, cmd := range commands {
		sbID.WriteString(fmt.Sprintf("➤ *%s*: %s\n", cmd.Command, cmd.IDDesc))
		sbEN.WriteString(fmt.Sprintf("➤ *%s*: %s\n", cmd.Command, cmd.ENDesc))
	}

	id := sbID.String()
	en := sbEN.String()
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	FileUploadedforBotWhatsapps := []FileUploadedforBotWhatsapp{
		{
			FileName:     "Report Pemasangan Juli 2025.xlsx",
			FileType:     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			MaxSize:      config.WebPanel.Get().Whatsmeow.MaxUploadedDocumentSize * 1024 * 1024, // Convert MB to bytes
			IDDesc:       "Contoh file yang dapat diunggah untuk Report Pemasangan MTI (Yokke) yang dicompare dengan data ODOO",
			ENDesc:       "Example file that can be uploaded for MTI Installation Report (Yokke) compared with ODOO data",
			TemplateFile: "https://example.com/path/to/template.xlsx", // Replace with actual template file URL
		},
		// Add more file upload examples as needed
	}

	for _, file := range FileUploadedforBotWhatsapps {
		if file.MaxSize > 0 {
			idFile := fmt.Sprintf(
				"📄 *%s*\n_Tipe_: `%s`\n_Ukuran Maks_: *%d MB*\n%s\n📌 *Template File*: %s\n",
				file.FileName,
				file.FileType,
				file.MaxSize/(1024*1024),
				"~ "+file.IDDesc,
				file.TemplateFile,
			)

			enFile := fmt.Sprintf(
				"📄 *%s*\n_Type_: `%s`\n_Max Size_: *%d MB*\n%s\n📌 *Template File*: %s\n",
				file.FileName,
				file.FileType,
				file.MaxSize/(1024*1024),
				"~ "+file.ENDesc,
				file.TemplateFile,
			)

			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idFile, enFile, userLang)
		}
	}

}

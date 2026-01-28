package telegramcontrollers

import (
	"fmt"
	"service-platform/internal/pkg/fun"
)

// getLocalizedMessage returns the localized message based on language
func (h *TelegramHelper) getLocalizedMessage(lang, key string, args ...interface{}) string {
	messages := map[string]map[string]string{
		fun.LangID: {
			"welcome":                                 "Selamat datang di Bot Telegram! Pilih opsi:",
			"welcome_new_user":                        "👋 *Selamat datang!* \n\nSaya adalah asisten bot yang siap membantu Anda. Klik tombol di bawah untuk memulai:",
			"help":                                    "Perintah dan opsi yang tersedia:",
			"start":                                   "Mulai",
			"info":                                    "Info",
			"commands":                                "Daftar Perintah",
			"language":                                "Bahasa",
			"bot_info":                                "🤖 *Informasi Bot*\n\nIni adalah bot Telegram yang dibuat dengan Go.\n\nFitur:\n• Menu interaktif\n• Penanganan perintah\n• Query callback\n\nGunakan /help untuk opsi lebih lanjut.",
			"commands_list":                           "📋 *Perintah yang Tersedia*\n\n/start - Mulai bot dan tampilkan menu utama\n/help - Tampilkan bantuan dan opsi yang tersedia\n\n*Catatan:* Anda juga dapat mengklik tombol di atas untuk navigasi!",
			"unknown_command":                         "Perintah tidak dikenal. Gunakan /help untuk perintah yang tersedia.",
			"unknown_button":                          "Tombol tidak dikenal ditekan.",
			"button1":                                 "Anda menekan Tombol 1!",
			"button2":                                 "Anda menekan Tombol 2!",
			"select_language":                         "Pilih Bahasa / Select Language:",
			"language_set":                            "Bahasa berhasil diubah ke Bahasa Indonesia 🇮🇩",
			"message_received":                        "Pesan diterima: %s",
			"user_registration_failed":                "Gagal mendaftarkan pengguna. Silakan coba lagi.",
			"user_created_unverified":                 "Selamat datang! Akun Anda telah dibuat tetapi perlu verifikasi. Silakan hubungi Technical Support di %s untuk diverifikasi.",
			"database_error":                          "Kesalahan database. Silakan coba lagi.",
			"user_banned":                             "Akun Anda telah diblokir. Silakan hubungi administrator.",
			"user_unverified":                         "Akun Anda belum diverifikasi. Silakan hubungi Technical Support di %s untuk diverifikasi. Atau ketikkan /reset untuk melakukan registrasi ulang.",
			"quota_exceeded":                          "Kuota harian terlampaui (%d/%d). Silakan coba lagi besok.",
			"language_error":                          "Error mengatur bahasa.",
			"reaction_added":                          "Reaksi %s ditambahkan!",
			"hours":                                   "jam",
			"minutes":                                 "menit",
			"seconds":                                 "detik",
			"time_remaining":                          "Waktu tersisa sampai reset: %s",
			"share_phone":                             "📱 Bagikan Nomor Telepon",
			"cancel":                                  "❌ Batal",
			"registration_fullname_prompt":            "Masukkan nama lengkap Anda:",
			"registration_fullname_required":          "Nama lengkap wajib diisi. Silakan masukkan nama lengkap Anda:",
			"registration_username_prompt":            "Masukkan username Anda:",
			"registration_username_required":          "Username wajib diisi. Silakan masukkan username Anda:",
			"registration_email_prompt":               "Masukkan alamat email Anda:",
			"registration_invalid_email":              "Format email tidak valid. Silakan masukkan email yang benar:",
			"registration_invalid_email_detailed":     "Email %s tidak valid. Silakan masukkan alamat email yang benar:",
			"registration_invalid_usertype":           "Jenis pengguna tidak valid. Silakan pilih dari opsi yang tersedia.",
			"registration_phone_prompt":               "Masukkan nomor telepon Anda:",
			"registration_phone_required":             "Nomor telepon wajib diisi. Silakan masukkan nomor telepon Anda:",
			"registration_invalid_phone":              "Nomor telepon tidak valid. Silakan masukkan nomor telepon yang benar:",
			"registration_invalid_phone_detailed":     "Nomor telepon %s tidak valid untuk kode negara '%s'. Silakan masukkan nomor telepon yang benar:",
			"registration_usertype_prompt":            "Pilih jenis pengguna Anda:",
			"registration_select_usertype":            "Silakan pilih jenis pengguna dari opsi di bawah:",
			"registration_complete":                   "Pendaftaran selesai! Selamat datang di bot kami.",
			"registration_superuser_not_allowed":      "Anda tidak dapat mendaftar sebagai Super User. Nomor telepon tidak ditemukan dalam database Super User.",
			"registration_superuser_update_failed":    "Gagal memperbarui data Super User: %s. Silakan coba lagi.",
			"registration_tams_not_allowed":           "Anda tidak dapat mendaftar sebagai TA MS. Nomor telepon tidak ditemukan dalam database TA MS.",
			"registration_tams_update_failed":         "Gagal memperbarui data TA MS: %s. Silakan coba lagi.",
			"registration_headms_not_allowed":         "Anda tidak dapat mendaftar sebagai Head MS. Nomor telepon tidak ditemukan dalam database Head MS.",
			"registration_headms_update_failed":       "Gagal memperbarui data Head MS: %s. Silakan coba lagi.",
			"registration_technicianms_not_allowed":   "Anda tidak dapat mendaftar sebagai Teknisi MS. Nomor telepon %s tidak ditemukan dalam ODOOMS.",
			"registration_technicianms_update_failed": "Gagal memperbarui data Teknisi MS: %s. Silakan coba lagi.",
			"registration_technicianms_check_failed":  "Gagal memeriksa pendaftaran Teknisi MS: %s. Silakan coba lagi.",
			"registration_technicianms_not_registered_odooms": "Nomor telepon %s tidak terdaftar sebagai Teknisi di ODOOMS.",
			"registration_splms_not_allowed":                  "Anda tidak dapat mendaftar sebagai SPL MS. Nomor telepon tidak ditemukan dalam database SPL MS.",
			"registration_splms_update_failed":                "Gagal memperbarui data SPL MS: %s. Silakan coba lagi.",
			"registration_splms_check_failed":                 "Gagal memeriksa pendaftaran SPL MS: %s. Silakan coba lagi.",
			"registration_splms_not_registered_odooms":        "Nomor telepon %s tidak terdaftar sebagai SPL di ODOOMS.",
			"registration_sacms_update_failed":                "Gagal memperbarui data SAC MS: %s. Silakan coba lagi.",
			"registration_sacms_not_allowed":                  "Anda tidak dapat mendaftar sebagai SAC MS. Nomor telepon tidak ditemukan dalam database SAC MS.",
			"registration_chat_id_in_use":                     "Chat ID sudah digunakan oleh pengguna lain yang sudah terverifikasi.",
			"usertype_common":                                 "Pengguna Biasa",
			"usertype_super_user":                             "Super User",
			"usertype_technician_ms":                          "Teknisi Manage Service",
			"usertype_splms":                                  "Service Point Leader - Manage Service",
			"usertype_sacms":                                  "Service Area Coordinator - Manage Service",
			"usertype_tams":                                   "Technical Assistance - Manage Service",
			"usertype_head_ms":                                "Kepala Manage Service",
			"error":                                           "Terjadi kesalahan. Silakan coba lagi.",
			"db_error":                                        "Kesalahan di database: %s",
			"registration_success_title":                      "✅ Pendaftaran Berhasil!",
			"registration_details_fullname":                   "👤 Nama Lengkap:",
			"registration_details_username":                   "👨‍💻 Username:",
			"registration_details_email":                      "📧 Email:",
			"registration_details_phone":                      "📱 Telepon:",
			"registration_details_usertype":                   "🏷️ Tipe Pengguna:",
			"registration_welcome_message":                    "Selamat datang di bot kami! Anda sekarang dapat menggunakan semua fitur.\n\nGunakan /help untuk melihat perintah yang tersedia.",
			"reset_success":                                   "Akun Anda telah dihapus. Silakan ketik /start untuk mendaftar ulang.",
			"reset_failed":                                    "Gagal menghapus akun. Silakan coba lagi atau hubungi support.",
			"access_denied":                                   "Akses ditolak. Anda tidak memiliki izin untuk perintah ini.",
			"input_wo_prompt":                                 "Masukkan nomor WO (Work Order):",
			"wo_number_empty":                                 "Nomor WO tidak boleh kosong. Silakan masukkan nomor WO yang valid.",
			"wo_number_received":                              "WO diterima: %s",
			"wo_number_processing":                            "Memproses WO: %s. Silahkan tunggu beberapa saat...",
			"tid_number_empty":                                "Nomor TID tidak boleh kosong. Silakan masukkan nomor TID yang valid.",
			"tid_number_received":                             "TID diterima: %s",
			"tid_info_empty":                                  "Nomor TID tidak boleh kosong. Silakan masukkan nomor TID untuk mendapatkan informasi merchant.",
			"tid_info_processing":                             "Memproses TID: %s",
			"unknown_input_type":                              "Tipe input tidak dikenal.",
			"command_not_implemented":                         "Perintah belum diimplementasikan.",
			"input_tid_prompt":                                "Masukkan nomor TID:",
			"info_tid_prompt":                                 "Masukkan nomor TID untuk mendapatkan informasi merchant:",
			"generating_report":                               "Permintaan laporan diterima",
			"select_sp_type":                                  "Pilih jenis SP (Teknisi/SPL/SAC):",
			"sp_status_processing":                            "Status %s diterima untuk: %s",
			"sp_name_empty":                                   "Nama SP tidak boleh kosong. Silakan masukkan nama SP yang valid.",
			"sp_type_technician":                              "Teknisi",
			"sp_type_spl":                                     "SPL",
			"sp_type_sac":                                     "SAC",
			"input_sp_name_prompt":                            "Masukkan nama %s:",
			"input_wo_placeholder":                            "Masukkan nomor WO",
			"input_tid_placeholder":                           "Masukkan nomor TID",
			"input_tid_info_placeholder":                      "Masukkan TID untuk info",
			"input_sp_name_placeholder":                       "Masukkan nama SP",
			"help_header":                                     "Bantuan dan Perintah Tersedia:",
			"help_common":                                     "*Bantuan Umum*\n\nPerintah yang tersedia:\n- /start - Mulai bot\n- /help - Tampilkan bantuan ini",
			"help_super_user":                                 "*Bantuan untuk Super User*\n\nPerintah yang tersedia:\n- /start - Mulai bot\n- /help - Tampilkan bantuan ini\n- /input_wo - Masukkan nomor Work Order\n- /input_tid - Masukkan nomor TID\n- /info_tid - Dapatkan informasi merchant\n- /generate_report_ta - Hasilkan laporan TA\n- /view_status_sp - Lihat status SP",
			"help_technician_ms":                              "*Bantuan untuk Teknisi Manage Service*\n\nPerintah yang tersedia:\n- /start - Mulai bot\n- /help - Tampilkan bantuan ini\n- /input_wo - Masukkan nomor Work Order\n- /input_tid - Masukkan nomor TID\n- /info_tid - Dapatkan informasi merchant",
			"help_spl_ms":                                     "*Bantuan untuk Service Point Leader - Manage Service*\n\nPerintah yang tersedia:\n- /start - Mulai bot\n- /help - Tampilkan bantuan ini\n- /input_wo - Masukkan nomor Work Order\n- /input_tid - Masukkan nomor TID\n- /info_tid - Dapatkan informasi merchant",
			"help_sac_ms":                                     "*Bantuan untuk Service Area Coordinator - Manage Service*\n\nPerintah yang tersedia:\n- /start - Mulai bot\n- /help - Tampilkan bantuan ini\n- /input_wo - Masukkan nomor Work Order\n- /input_tid - Masukkan nomor TID\n- /info_tid - Dapatkan informasi merchant",
			"help_tams":                                       "*Bantuan untuk Technical Assistance - Manage Service*\n\nPerintah yang tersedia:\n- /start - Mulai bot\n- /help - Tampilkan bantuan ini\n- /generate_report_ta - Hasilkan laporan TA",
			"help_head_ms":                                    "*Bantuan untuk Head Manage Service*\n\nPerintah yang tersedia:\n- /start - Mulai bot\n- /help - Tampilkan bantuan ini\n- /input_wo - Masukkan nomor Work Order\n- /input_tid - Masukkan nomor TID\n- /info_tid - Dapatkan informasi merchant\n- /view_status_sp - Lihat status SP",
			"input_wo":                                        "Input WO",
			"input_tid":                                       "Input TID",
			"info_tid":                                        "Info TID",
			"generate_report_ta":                              "Generate TA Report",
			"view_status_sp":                                  "View SP Status",
			"technician_commands":                             "Technician Commands",
			"ta_commands":                                     "TA Commands",
			"head_commands":                                   "Head Commands",
			"technician_commands_list":                        "<b>Technician Commands</b>\n\n- /input_wo - Masukkan nomor Work Order\n- /input_tid - Masukkan nomor TID\n- /info_tid - Dapatkan informasi merchant",
			"ta_commands_list":                                "<b>TA Commands</b>\n\n- /generate_report_ta - Hasilkan laporan TA",
			"head_commands_list":                              "<b>Head Commands</b>\n\n- /input_wo - Masukkan nomor Work Order\n- /input_tid - Masukkan nomor TID\n- /info_tid - Dapatkan informasi merchant\n- /view_status_sp - Lihat status SP",
		},
		// ADD: english language manually so it can be 2 languages
		// TODO: do not declare the other language here, soon i will add it manually
	}

	if langMessages, exists := messages[lang]; exists {
		if msg, found := langMessages[key]; found {
			if len(args) > 0 {
				return fmt.Sprintf(msg, args...)
			}
			return msg
		}
	}
	// Fallback to Indonesia if language or key not found
	if langMessages, exists := messages[fun.DefaultLang]; exists {
		if msg, found := langMessages[key]; found {
			if len(args) > 0 {
				return fmt.Sprintf(msg, args...)
			}
			return msg
		}
	}
	return key // Return key if not found
}

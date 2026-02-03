package controllers

import (
	"context"
	"fmt"
	"net/http"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"strings"
	"time"

	"github.com/TigorLazuardi/tanggal"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types"
)

func CheckWAPhoneNumberIsRegistered() gin.HandlerFunc {
	return func(c *gin.Context) {
		digitNoTelp := 9

		phoneNumber := c.Query("phone")
		if phoneNumber == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		if len(phoneNumber) > digitNoTelp {
			sanitizedPhoneNumber, err := fun.SanitizePhoneNumber(phoneNumber)
			if err != nil {
				c.Status(http.StatusBadRequest)
				return
			}

			result, err := WhatsappClient.IsOnWhatsApp(context.Background(), []string{sanitizedPhoneNumber + "@s.whatsapp.net"})
			if err != nil {
				logrus.Errorf("got error while trying to check number phone is being registered in whatsapp: %v", err)
				errStr := strings.ToLower(err.Error())

				switch {
				case strings.Contains(errStr, "disconnected"),
					strings.Contains(errStr, "blocked"),
					strings.Contains(errStr, "timeout"),
					strings.Contains(errStr, "rate limited"),
					strings.Contains(errStr, "network error"),
					strings.Contains(errStr, "authentication failed"),
					strings.Contains(errStr, "1006"),
					strings.Contains(errStr, "unexpected eof"),
					strings.Contains(errStr, "websocket not connected"):
					c.Status(http.StatusInternalServerError)
				default:
					c.Status(http.StatusBadRequest)
				}
				return
			}

			if len(result) > 0 {
				contact := result[0]
				if !contact.IsIn {
					c.Status(http.StatusBadRequest)
					return
				} else {
					c.Status(http.StatusOK)
					return
				}
			} else {
				c.Status(http.StatusBadRequest)
				return
			}
		} else {
			c.Status(http.StatusBadRequest)
			return
		}
	}
}

func SendTAFollowedUpResultToGroupTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		var payload struct {
			Technician string `json:"technician"`
			Email      string `json:"email"`
			Method     string `json:"method"`
			Wo         string `json:"wo"`
			Spk        string `json:"spk"`
			TypeCase   string `json:"type_case"`
			Problem    string `json:"problem"`
			Mid        string `json:"mid"`
			Tid        string `json:"tid"`
			Rc         string `json:"rc"`
			Reason     string `json:"reason"`
			Date       string `json:"date"`
			// TaFeedback string `json:"ta_feedback"`
		}

		if err := c.ShouldBindJSON(&payload); err != nil {
			logrus.Errorf("Error unmarshalling JSON: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}

		if payload.Method == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		if strings.Contains(strings.ToLower(payload.Method), "submit") {
			c.Status(http.StatusBadRequest)
			return
		}

		if isEmptyPayload(
			payload.Email,
			payload.Wo,
			payload.Spk,
			payload.Technician,
			payload.TypeCase,
			payload.Mid,
			payload.Tid,
		) {
			logrus.Error("Error: One or more required fields are empty")
			c.Status(http.StatusBadRequest)
			return
		}

		var sb strings.Builder
		loc, err := time.LoadLocation(config.GetConfig().Default.Timezone)
		if err != nil {
			logrus.Errorf("Error loading location: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		now := time.Now().In(loc)
		tgl, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
		if err != nil {
			logrus.Errorf("Error formatting date: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		formattedDate := tgl.Format(" ", []tanggal.Format{
			tanggal.NamaHari,
			tanggal.Hari,
			tanggal.NamaBulan,
			tanggal.Tahun,
			tanggal.PukulDenganDetik,
			tanggal.ZonaWaktu,
		})
		sb.WriteString(fmt.Sprintf("🔔 FYI, Per _%v_\n\n", formattedDate))

		var taActivity string
		switch strings.ToLower(payload.Method) {
		case "delete":
			taActivity = "menghapus data"
		case "edit":
			taActivity = "mengedit data"
		case "edit (temp)":
			taActivity = "mengedit data (sementara)"
		case "edit (final)":
			taActivity = "mengedit data (final)"
		default:
			c.Status(http.StatusBadRequest)
			return
		}

		var taName, taPhone string
		DataTA := config.GetConfig().UserTA
		if ta, ok := DataTA[payload.Email]; ok {
			taName = ta.Name
			taPhone = ta.Phone
		} else {
			taName = "N/A"
			taPhone = "N/A"
		}

		var teknisiWAG int
		_, digit, ok := GetFirstNumberWordTechnician(payload.Technician)
		if ok {
			teknisiWAG = digit
		} else {
			teknisiWAG = 0
		}

		var jidString string
		switch teknisiWAG {
		case 1:
			jidString = config.GetConfig().Whatsmeow.WAGRegionTechnician[1]
		case 2:
			jidString = config.GetConfig().Whatsmeow.WAGRegionTechnician[2]
		case 3:
			jidString = config.GetConfig().Whatsmeow.WAGRegionTechnician[3]
		case 4:
			jidString = config.GetConfig().Whatsmeow.WAGRegionTechnician[4]
		case 5:
			jidString = config.GetConfig().Whatsmeow.WAGRegionTechnician[5]
		default:
			jidString = config.GetConfig().Whatsmeow.WAGTAJID
		}
		jidString += "@g.us"
		jid, err := types.ParseJID(jidString)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		sb.WriteString(fmt.Sprintf("Tim TA: *%v* telah %v di Dashboard TA, dengan rincian sebagai berikut:\n\n",
			taName+" ["+taPhone+"]",
			taActivity,
		))
		if payload.Wo != "" {
			sb.WriteString(fmt.Sprintf("WO Number: *%v*\n", payload.Wo))
		}
		if payload.Spk != "" {
			sb.WriteString(fmt.Sprintf("Ticket Number: *%v*\n", payload.Spk))
		}
		if payload.Mid != "" {
			sb.WriteString(fmt.Sprintf("MID: *%v*\n", payload.Mid))
		}
		if payload.Tid != "" {
			sb.WriteString(fmt.Sprintf("TID: *%v*\n", payload.Tid))
		}
		if payload.Technician != "" {
			sb.WriteString(fmt.Sprintf("Teknisi: *%v*\n", payload.Technician))
		}
		if payload.TypeCase != "" {
			sb.WriteString(fmt.Sprintf("Pengerjaan Data: *%v*\n", payload.TypeCase))
		}
		if payload.Problem != "" {
			sb.WriteString(fmt.Sprintf("Error Yang Tertera: *%v*\n", payload.Problem))
		}
		if payload.Rc != "" {
			sb.WriteString(fmt.Sprintf("Reason Code: *%v*\n", payload.Rc))
		}
		if payload.Reason != "" {
			sb.WriteString(fmt.Sprintf("Alasan Dihapus: *%v*\n", payload.Reason))
		}
		// if payload.TaFeedback != "" {
		// 	sb.WriteString(fmt.Sprintf("Feedback dari TA: *%v*\n", payload.TaFeedback))
		// }

		sb.WriteString("\nUntuk selanjutnya dicek di APK FSnya apakah masih berada di menu offline.")

		msgToSend := sb.String()

		var mentions []string
		dataTech, ok := TechODOOMSData[payload.Technician]
		if !ok {
			logrus.Errorf("❌ Technician %s not found in TechODOOMSData", payload.Technician)
			c.Status(http.StatusBadRequest)
			return
		}

		sanitizedPhoneNumber, err := fun.SanitizePhoneNumber(dataTech.NoHP)
		if err != nil {
			logrus.Errorf("got error while trying to sanitize phone number: %s - %v", dataTech.NoHP, err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if sanitizedPhoneNumber != "" {
			mentions = append(mentions, "62"+sanitizedPhoneNumber)
		}

		SendWhatsappMessageWithMentionsbyTA(jid, mentions, msgToSend)
		c.Status(http.StatusOK)
	}
}

func SendTAFeedbackAboutJONeedsEvidenceToGroupTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		var payload struct {
			Teknisi       string `json:"Teknisi"`
			EmailTa       string `json:"EmailTa"`
			NamaTa        string `json:"NamaTa"`
			NomorTa       string `json:"NomorTa"`
			Tabel         string `json:"Tabel"`
			Feedback      string `json:"Feedback"`
			WoNumber      string `json:"WoNumber"`
			TicketSubject string `json:"TicketSubject"`
			Merchant      string `json:"Merchant"`
			MID           string `json:"MID"`
			TID           string `json:"TID"`
		}

		if err := c.ShouldBindJSON(&payload); err != nil {
			logrus.Errorf("Error unmarshalling JSON: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}

		if isEmptyPayload(
			payload.Teknisi,
			payload.EmailTa,
			payload.NamaTa,
			payload.NomorTa,
			payload.Tabel,
			payload.Feedback,
			payload.WoNumber,
			payload.TicketSubject,
		) {
			logrus.Error("Error: One or more required fields are empty")
			c.Status(http.StatusBadRequest)
			return
		}

		var taPhone string = "N/A"
		DataTA := config.GetConfig().UserTA
		if ta, ok := DataTA[payload.EmailTa]; ok {
			taPhone = ta.Phone
		}

		var sb strings.Builder
		loc, err := time.LoadLocation(config.GetConfig().Default.Timezone)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		now := time.Now().In(loc)
		tgl, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		formattedDate := tgl.Format(" ", []tanggal.Format{
			tanggal.NamaHari,
			tanggal.Hari,
			tanggal.NamaBulan,
			tanggal.Tahun,
			tanggal.PukulDenganDetik,
			tanggal.ZonaWaktu,
		})
		sb.WriteString(fmt.Sprintf("🔔 *FYI*, Per _%v_\n\n", formattedDate))
		sb.WriteString(fmt.Sprintf("TA: _%s_ (*%s*) telah melakukan pengecekan pengerjaan terhadap JO:\n\n", payload.NamaTa, taPhone))
		sb.WriteString(fmt.Sprintf("*#%s* (%s)\n", payload.TicketSubject, payload.WoNumber))
		if payload.Teknisi != "" {
			sb.WriteString(fmt.Sprintf("Teknisi: %s\n", payload.Teknisi))
		}
		if payload.Merchant != "" {
			sb.WriteString(fmt.Sprintf("Merchant: %s\n", payload.Merchant))
		}
		if payload.MID != "" {
			sb.WriteString(fmt.Sprintf("MID: %s\n", payload.MID))
		}
		if payload.TID != "" {
			sb.WriteString(fmt.Sprintf("TID: %s\n", payload.TID))
		}
		sb.WriteString(fmt.Sprintf("Hasil feedback: %s\n\n", payload.Feedback))
		sb.WriteString("Untuk selanjutnya diperbaiki pengerjaannya. Apabila tidak dapat memberikan evidence yang sesuai, dan ingin melakukan kunjungan ulang di keesokan harinya agar segera menginfokan ke Tim TA yang sedang _stand by_, agar SPK tersebut segera disubmit / dihapus. Sehingga di Dashboard tidak menumpuk")

		msgToSend := sb.String()

		var teknisiWAG int
		_, digit, ok := GetFirstNumberWordTechnician(payload.Teknisi)
		if ok {
			teknisiWAG = digit
		} else {
			teknisiWAG = 0
		}

		var jidString string
		switch teknisiWAG {
		case 1:
			jidString = config.GetConfig().Whatsmeow.WAGRegionTechnician[1]
		case 2:
			jidString = config.GetConfig().Whatsmeow.WAGRegionTechnician[2]
		case 3:
			jidString = config.GetConfig().Whatsmeow.WAGRegionTechnician[3]
		case 4:
			jidString = config.GetConfig().Whatsmeow.WAGRegionTechnician[4]
		case 5:
			jidString = config.GetConfig().Whatsmeow.WAGRegionTechnician[5]
		default:
			jidString = config.GetConfig().Whatsmeow.WAGTAJID
		}
		jidString += "@g.us"
		jid, err := types.ParseJID(jidString)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		var mentions []string
		dataTech, ok := TechODOOMSData[payload.Teknisi]
		if !ok {
			logrus.Errorf("❌ Technician %s not found in TechODOOMSData", payload.Teknisi)
			c.Status(http.StatusBadRequest)
			return
		}

		sanitizedPhoneNumber, err := fun.SanitizePhoneNumber(dataTech.NoHP)
		if err != nil {
			logrus.Errorf("got error while trying to sanitize phone number: %s - %v", dataTech.NoHP, err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if sanitizedPhoneNumber != "" {
			mentions = append(mentions, "62"+sanitizedPhoneNumber)
		}

		dataSPL, ok := TechODOOMSData[dataTech.SPL]
		if !ok {
			logrus.Errorf("❌ SPL %s not found in TechODOOMSData", dataTech.SPL)
		}
		if dataSPL.NoHP != "" {
			sanitizedPhoneSPL, err := fun.SanitizePhoneNumber(dataSPL.NoHP)
			if err != nil {
				logrus.Errorf("got error while trying to sanitize phone number: %s - %v", dataSPL.NoHP, err)
			} else {
				mentions = append(mentions, "62"+sanitizedPhoneSPL)
			}
		}

		SendWhatsappMessageWithMentionsbyTA(jid, mentions, msgToSend)
		c.Status(http.StatusOK)
	}
}

func isEmptyPayload(fields ...string) bool {
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			return true
		}
	}
	return false
}

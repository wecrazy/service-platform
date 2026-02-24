package controllers

import (
	"fmt"
	"service-platform/pkg/fun"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

// HelloFromBot sends a greeting message based on the time of day and language preferences
// Parameters:
// - v: The WhatsApp message event
// - phoneNumber: The recipient's phone number
// - senderName: The name of the sender
// - stanzaID: The stanza ID of the message
// - originalSenderJID: The original sender's JID
// - client: The WhatsApp client instance
// - rdb: The Redis client instance
// - db: The GORM database instance
// This function checks for required parameters and sends a localized greeting message based on the current time.
// It supports multiple languages and time-based greetings.
// If any required parameter is missing, it logs a warning and exits without sending a message.
func HelloFromBot(v *events.Message, phoneNumber, senderName, stanzaID, originalSenderJID string, client *whatsmeow.Client, rdb *redis.Client, db *gorm.DB) {
	if phoneNumber == "" || senderName == "" || stanzaID == "" || originalSenderJID == "" {
		logrus.Warn("Missing required parameters in HelloFromBot")
		return
	}

	if client == nil || rdb == nil || db == nil {
		logrus.Warn("Missing client, redis, or db in HelloFromBot")
		return
	}

	now := time.Now()
	utcTime := now.UTC()
	hour := now.Hour()

	// Greetings map: language -> []string for each time slot
	greetingsMap := map[string][]string{
		fun.LangID: {"Selamat Dini Hari", "Selamat Pagi", "Selamat Siang", "Selamat Sore", "Selamat Petang", "Selamat Malam"},
		fun.LangEN: {"Good Early Morning", "Good Morning", "Good Afternoon", "Good Late Afternoon", "Good Evening", "Good Night"},
		fun.LangES: {"Buenas noches", "Buenos días", "Buenas tardes", "Buenas tardes", "Buenas tardes", "Buenas noches"},
		fun.LangFR: {"Bonsoir", "Bonjour", "Bon après-midi", "Bon après-midi", "Bonsoir", "Bonsoir"},
		fun.LangPT: {"Boa noite", "Bom dia", "Boa tarde", "Boa tarde", "Boa tarde", "Boa noite"},
		fun.LangDE: {"Gute Nacht", "Guten Morgen", "Guten Tag", "Guten Tag", "Guten Abend", "Gute Nacht"},
		fun.LangRU: {"Доброй ночи", "Доброе утро", "Добрый день", "Добрый день", "Добрый вечер", "Доброй ночи"},
		fun.LangJP: {"おやすみ", "おはよう", "こんにちは", "こんにちは", "こんばんは", "おやすみ"},
		fun.LangCN: {"晚安", "早上好", "下午好", "下午好", "晚上好", "晚安"},
		fun.LangAR: {"مساء الخير", "صباح الخير", "مساء الخير", "مساء الخير", "مساء الخير", "مساء الخير"},
	}

	// Message templates
	messageTemplates := map[string]string{
		fun.LangID: "Halo *%s*, %s 😃\nSekarang %v, pukul %v (UTC: %v).",
		fun.LangEN: "Hello *%s*, %s😃\nNow %v, time %v (UTC: %v).",
		fun.LangES: "Hola *%s*, %s😃\nAhora %v, hora %v (UTC: %v).",
		fun.LangFR: "Bonjour *%s*, %s😃\nMaintenant %v, heure %v (UTC: %v).",
		fun.LangPT: "Olá *%s*, %s😃\nAgora %v, hora %v (UTC: %v).",
		fun.LangDE: "Hallo *%s*, %s😃\nJetzt %v, Uhrzeit %v (UTC: %v).",
		fun.LangRU: "Привет *%s*, %s😃\nСейчас %v, время %v (UTC: %v).",
		fun.LangJP: "こんにちは *%s*、%s😃\n今 %v、時間 %v (UTC: %v)。",
		fun.LangCN: "你好 *%s*，%s😃\n现在 %v，时间 %v (UTC: %v)。",
		fun.LangAR: "مرحبا *%s*، %s😃\nالآن %v، الوقت %v (UTC: %v)。",
	}

	// Function to get greeting index
	getGreetingIndex := func(h int) int {
		if h < 4 {
			return 0
		} else if h < 12 {
			return 1
		} else if h < 15 {
			return 2
		} else if h < 17 {
			return 3
		} else if h < 19 {
			return 4
		}
		return 5
	}

	langMsg := make(map[string]string)
	supportedLangs := fun.GetSupportedLanguages()

	for _, lang := range supportedLangs {
		greetings := greetingsMap[lang]
		template := messageTemplates[lang]
		greeting := greetings[getGreetingIndex(hour)]
		langMsg[lang] = fmt.Sprintf(template, senderName, greeting, now.Format("02 January 2006"), now.Format("15:04:05"), utcTime.Format("15:04:05"))
	}

	lang := NewLanguageMsgTranslation(fun.DefaultLang)
	lang.Texts = langMsg

	SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, lang, lang.LanguageCode, client, rdb, db)
}

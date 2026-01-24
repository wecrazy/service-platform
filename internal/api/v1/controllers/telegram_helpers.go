package controllers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	telegrammodel "service-platform/internal/core/model/telegram_model"
	"service-platform/internal/pkg/fun"
	pb "service-platform/proto"
)

// TelegramHelper contains helper functions for Telegram bot operations
type TelegramHelper struct {
	bot         *tgbotapi.BotAPI
	redis       *redis.Client
	db          *gorm.DB
	defaultLang string
}

// NewTelegramHelper creates a new TelegramHelper instance
func NewTelegramHelper(bot *tgbotapi.BotAPI, redis *redis.Client, db *gorm.DB, defaultLang string) *TelegramHelper {
	return &TelegramHelper{
		bot:         bot,
		redis:       redis,
		db:          db,
		defaultLang: defaultLang,
	}
}

// sendTypingAction sends a typing indicator to the chat
func (h *TelegramHelper) sendTypingAction(chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	h.bot.Send(action)
}

// markMessageAsSeen updates the message status in database to "seen"
func (h *TelegramHelper) markMessageAsSeen(messageID int64, chatID string) {
	err := h.db.Model(&telegrammodel.TelegramIncomingMsg{}).
		Where("telegram_message_id = ? AND telegram_chat_id = ?", messageID, chatID).
		Update("telegram_msg_status", "seen").Error
	if err != nil {
		logrus.WithError(err).Error("Failed to mark message as seen")
	}
}

// BuildInlineKeyboard constructs an InlineKeyboardMarkup from the protobuf definition.
func (h *TelegramHelper) BuildInlineKeyboard(keyboard *pb.InlineKeyboardMarkup) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, row := range keyboard.InlineKeyboard {
		var buttons []tgbotapi.InlineKeyboardButton
		for _, button := range row.Buttons {
			btn := tgbotapi.NewInlineKeyboardButtonData(button.Text, button.CallbackData)
			if button.Url != "" {
				btn = tgbotapi.NewInlineKeyboardButtonURL(button.Text, button.Url)
			}
			buttons = append(buttons, btn)
		}
		rows = append(rows, buttons)
	}
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// getUserLanguage gets the user's preferred language from Redis
func (h *TelegramHelper) getUserLanguage(userID int64) string {
	key := fmt.Sprintf("telegram:user:lang:%d", userID)
	lang, err := h.redis.Get(context.Background(), key).Result()
	if err != nil {
		// Return default language if not found or error
		return h.defaultLang
	}
	return lang
}

// setUserLanguage sets the user's preferred language in Redis with 24h expiration
func (h *TelegramHelper) setUserLanguage(userID int64, lang string) error {
	key := fmt.Sprintf("telegram:user:lang:%d", userID)
	return h.redis.Set(context.Background(), key, lang, 24*time.Hour).Err()
}

// getLocalizedMessage returns the localized message based on language
func (h *TelegramHelper) getLocalizedMessage(lang, key string) string {
	messages := map[string]map[string]string{
		fun.LangID: {
			"welcome":          "Selamat datang di Bot Telegram! Pilih opsi:",
			"help":             "Perintah dan opsi yang tersedia:",
			"start":            "Mulai",
			"info":             "Info",
			"commands":         "Daftar Perintah",
			"language":         "Bahasa",
			"bot_info":         "🤖 *Informasi Bot*\n\nIni adalah bot Telegram yang dibuat dengan Go.\n\nFitur:\n• Menu interaktif\n• Penanganan perintah\n• Query callback\n\nGunakan /help untuk opsi lebih lanjut.",
			"commands_list":    "📋 *Perintah yang Tersedia*\n\n/start - Mulai bot dan tampilkan menu utama\n/help - Tampilkan bantuan dan opsi yang tersedia\n\n*Catatan:* Anda juga dapat mengklik tombol di atas untuk navigasi!",
			"unknown_command":  "Perintah tidak dikenal. Gunakan /help untuk perintah yang tersedia.",
			"unknown_button":   "Tombol tidak dikenal ditekan.",
			"button1":          "Anda menekan Tombol 1!",
			"button2":          "Anda menekan Tombol 2!",
			"select_language":  "Pilih Bahasa / Select Language:",
			"language_set":     "Bahasa berhasil diubah ke Bahasa Indonesia 🇮🇩",
			"message_received": "Pesan diterima: %s",
		},
		fun.LangEN: {
			"welcome":          "Welcome to the Telegram Bot! Choose an option:",
			"help":             "Available commands and options:",
			"start":            "Start",
			"info":             "Info",
			"commands":         "Commands List",
			"language":         "Language",
			"bot_info":         "🤖 *Bot Information*\n\nThis is a Telegram bot built with Go.\n\nFeatures:\n• Interactive menus\n• Command handling\n• Callback queries\n\nUse /help for more options.",
			"commands_list":    "📋 *Available Commands*\n\n/start - Start the bot and show main menu\n/help - Show help and available options\n\n*Note:* You can also click the buttons above to navigate!",
			"unknown_command":  "Unknown command. Use /help for available commands.",
			"unknown_button":   "Unknown button pressed.",
			"button1":          "You pressed Button 1!",
			"button2":          "You pressed Button 2!",
			"select_language":  "Pilih Bahasa / Select Language:",
			"language_set":     "Language successfully changed to English 🇺🇸",
			"message_received": "Message received: %s",
		},
		fun.LangES: {
			"welcome":          "¡Bienvenido al Bot de Telegram! Elige una opción:",
			"help":             "Comandos y opciones disponibles:",
			"start":            "Inicio",
			"info":             "Info",
			"commands":         "Lista de Comandos",
			"language":         "Idioma",
			"bot_info":         "🤖 *Información del Bot*\n\nEste es un bot de Telegram creado con Go.\n\nCaracterísticas:\n• Menús interactivos\n• Manejo de comandos\n• Consultas de callback\n\nUsa /help para más opciones.",
			"commands_list":    "📋 *Comandos Disponibles*\n\n/start - Iniciar el bot y mostrar el menú principal\n/help - Mostrar ayuda y opciones disponibles\n\n*Nota:* ¡También puedes hacer clic en los botones de arriba para navegar!",
			"unknown_command":  "Comando desconocido. Usa /help para comandos disponibles.",
			"unknown_button":   "Botón desconocido presionado.",
			"button1":          "¡Presionaste el Botón 1!",
			"button2":          "¡Presionaste el Botón 2!",
			"select_language":  "Selecciona Idioma / Select Language:",
			"language_set":     "Idioma cambiado exitosamente a Español 🇪🇸",
			"message_received": "Mensaje recibido: %s",
		},
		fun.LangFR: {
			"welcome":          "Bienvenue sur le Bot Telegram ! Choisissez une option :",
			"help":             "Commandes et options disponibles :",
			"start":            "Démarrer",
			"info":             "Info",
			"commands":         "Liste des Commandes",
			"language":         "Langue",
			"bot_info":         "🤖 *Informations sur le Bot*\n\nCeci est un bot Telegram créé avec Go.\n\nFonctionnalités :\n• Menus interactifs\n• Gestion des commandes\n• Requêtes de callback\n\nUtilisez /help pour plus d'options.",
			"commands_list":    "📋 *Commandes Disponibles*\n\n/start - Démarrer le bot et afficher le menu principal\n/help - Afficher l'aide et les options disponibles\n\n*Note :* Vous pouvez également cliquer sur les boutons ci-dessus pour naviguer !",
			"unknown_command":  "Commande inconnue. Utilisez /help pour les commandes disponibles.",
			"unknown_button":   "Bouton inconnu pressé.",
			"button1":          "Vous avez appuyé sur le Bouton 1 !",
			"button2":          "Vous avez appuyé sur le Bouton 2 !",
			"select_language":  "Sélectionner la Langue / Select Language :",
			"language_set":     "Langue changée avec succès en Français 🇫🇷",
			"message_received": "Message reçu : %s",
		},
		fun.LangDE: {
			"welcome":          "Willkommen beim Telegram Bot! Wählen Sie eine Option:",
			"help":             "Verfügbare Befehle und Optionen:",
			"start":            "Start",
			"info":             "Info",
			"commands":         "Befehlsliste",
			"language":         "Sprache",
			"bot_info":         "🤖 *Bot-Informationen*\n\nDies ist ein Telegram-Bot, der mit Go erstellt wurde.\n\nFunktionen:\n• Interaktive Menüs\n• Befehlsverarbeitung\n• Callback-Abfragen\n\nVerwenden Sie /help für weitere Optionen.",
			"commands_list":    "📋 *Verfügbare Befehle*\n\n/start - Bot starten und Hauptmenü anzeigen\n/help - Hilfe und verfügbare Optionen anzeigen\n\n*Hinweis:* Sie können auch auf die Schaltflächen oben klicken, um zu navigieren!",
			"unknown_command":  "Unbekannter Befehl. Verwenden Sie /help für verfügbare Befehle.",
			"unknown_button":   "Unbekannte Schaltfläche gedrückt.",
			"button1":          "Sie haben Schaltfläche 1 gedrückt!",
			"button2":          "Sie haben Schaltfläche 2 gedrückt!",
			"select_language":  "Sprache auswählen / Select Language:",
			"language_set":     "Sprache erfolgreich zu Deutsch 🇩🇪 geändert",
			"message_received": "Nachricht erhalten: %s",
		},
		fun.LangPT: {
			"welcome":          "Bem-vindo ao Bot do Telegram! Escolha uma opção:",
			"help":             "Comandos e opções disponíveis:",
			"start":            "Iniciar",
			"info":             "Info",
			"commands":         "Lista de Comandos",
			"language":         "Idioma",
			"bot_info":         "🤖 *Informações do Bot*\n\nEste é um bot do Telegram criado com Go.\n\nRecursos:\n• Menus interativos\n• Tratamento de comandos\n• Consultas de callback\n\nUse /help para mais opções.",
			"commands_list":    "📋 *Comandos Disponíveis*\n\n/start - Iniciar o bot e mostrar o menu principal\n/help - Mostrar ajuda e opções disponíveis\n\n*Nota:* Você também pode clicar nos botões acima para navegar!",
			"unknown_command":  "Comando desconhecido. Use /help para comandos disponíveis.",
			"unknown_button":   "Botão desconhecido pressionado.",
			"button1":          "Você pressionou o Botão 1!",
			"button2":          "Você pressionou o Botão 2!",
			"select_language":  "Selecionar Idioma / Select Language:",
			"language_set":     "Idioma alterado com sucesso para Português 🇵🇹",
			"message_received": "Mensagem recebida: %s",
		},
		fun.LangRU: {
			"welcome":          "Добро пожаловать в Telegram Bot! Выберите вариант:",
			"help":             "Доступные команды и опции:",
			"start":            "Старт",
			"info":             "Инфо",
			"commands":         "Список Команд",
			"language":         "Язык",
			"bot_info":         "🤖 *Информация о Боте*\n\nЭто бот Telegram, созданный с помощью Go.\n\nФункции:\n• Интерактивные меню\n• Обработка команд\n• Callback-запросы\n\nИспользуйте /help для дополнительных опций.",
			"commands_list":    "📋 *Доступные Команды*\n\n/start - Запустить бота и показать главное меню\n/help - Показать справку и доступные опции\n\n*Примечание:* Вы также можете нажимать кнопки выше для навигации!",
			"unknown_command":  "Неизвестная команда. Используйте /help для доступных команд.",
			"unknown_button":   "Нажата неизвестная кнопка.",
			"button1":          "Вы нажали Кнопку 1!",
			"button2":          "Вы нажали Кнопку 2!",
			"select_language":  "Выбрать Язык / Select Language:",
			"language_set":     "Язык успешно изменен на Русский 🇷🇺",
			"message_received": "Сообщение получено: %s",
		},
		fun.LangJP: {
			"welcome":          "Telegram Botへようこそ！オプションを選択してください：",
			"help":             "利用可能なコマンドとオプション：",
			"start":            "スタート",
			"info":             "情報",
			"commands":         "コマンドリスト",
			"language":         "言語",
			"bot_info":         "🤖 *Bot情報*\n\nこれはGoで作成されたTelegram Botです。\n\n機能：\n• インタラクティブメニュー\n• コマンド処理\n• コールバッククエリ\n\n詳細オプションについては/helpを使用してください。",
			"commands_list":    "📋 *利用可能なコマンド*\n\n/start - Botを起動しメインメニューを表示\n/help - ヘルプと利用可能なオプションを表示\n\n*注意：* 上のボタンをクリックしてナビゲートすることもできます！",
			"unknown_command":  "不明なコマンドです。利用可能なコマンドについては/helpを使用してください。",
			"unknown_button":   "不明なボタンが押されました。",
			"button1":          "ボタン1を押しました！",
			"button2":          "ボタン2を押しました！",
			"select_language":  "言語を選択 / Select Language:",
			"language_set":     "言語が日本語 🇯🇵 に正常に変更されました",
			"message_received": "メッセージを受信しました：%s",
		},
		fun.LangCN: {
			"welcome":          "欢迎使用 Telegram Bot！选择一个选项：",
			"help":             "可用的命令和选项：",
			"start":            "开始",
			"info":             "信息",
			"commands":         "命令列表",
			"language":         "语言",
			"bot_info":         "🤖 *Bot信息*\n\n这是一个使用Go创建的Telegram Bot。\n\n功能：\n• 交互式菜单\n• 命令处理\n• 回调查询\n\n使用/help获取更多选项。",
			"commands_list":    "📋 *可用命令*\n\n/start - 启动Bot并显示主菜单\n/help - 显示帮助和可用选项\n\n*注意：* 您也可以点击上面的按钮进行导航！",
			"unknown_command":  "未知命令。使用/help查看可用命令。",
			"unknown_button":   "按下了未知按钮。",
			"button1":          "您按下了按钮1！",
			"button2":          "您按下了按钮2！",
			"select_language":  "选择语言 / Select Language:",
			"language_set":     "语言已成功更改为中文 🇨🇳",
			"message_received": "收到消息：%s",
		},
		fun.LangAR: {
			"welcome":          "مرحباً بك في بوت Telegram! اختر خياراً:",
			"help":             "الأوامر والخيارات المتاحة:",
			"start":            "ابدأ",
			"info":             "معلومات",
			"commands":         "قائمة الأوامر",
			"language":         "اللغة",
			"bot_info":         "🤖 *معلومات البوت*\n\nهذا بوت Telegram تم إنشاؤه باستخدام Go.\n\nالميزات:\n• القوائم التفاعلية\n• معالجة الأوامر\n• استفسارات الرد\n\nاستخدم /help للمزيد من الخيارات.",
			"commands_list":    "📋 *الأوامر المتاحة*\n\n/start - بدء البوت وعرض القائمة الرئيسية\n/help - عرض المساعدة والخيارات المتاحة\n\n*ملاحظة:* يمكنك أيضاً النقر على الأزرار أعلاه للتنقل!",
			"unknown_command":  "أمر غير معروف. استخدم /help للأوامر المتاحة.",
			"unknown_button":   "تم الضغط على زر غير معروف.",
			"button1":          "لقد ضغطت على الزر 1!",
			"button2":          "لقد ضغطت على الزر 2!",
			"select_language":  "اختر اللغة / Select Language:",
			"language_set":     "تم تغيير اللغة بنجاح إلى العربية 🇸🇦",
			"message_received": "تم استلام الرسالة: %s",
		},
	}

	if langMessages, exists := messages[lang]; exists {
		if msg, found := langMessages[key]; found {
			return msg
		}
	}
	// Fallback to Indonesia if language or key not found
	if langMessages, exists := messages[fun.DefaultLang]; exists {
		if msg, found := langMessages[key]; found {
			return msg
		}
	}
	return key // Return key if not found
}

// HandleUpdate processes incoming updates from Telegram
func (h *TelegramHelper) HandleUpdate(update tgbotapi.Update) {
	if update.Message != nil {
		h.HandleMessage(update.Message)
	} else if update.CallbackQuery != nil {
		h.HandleCallbackQuery(update.CallbackQuery)
	}
}

// HandleMessage processes incoming messages from users
func (h *TelegramHelper) HandleMessage(message *tgbotapi.Message) {
	if message.IsCommand() {
		h.HandleCommand(message)
	} else {
		// Handle regular messages if needed
		userLang := h.getUserLanguage(message.From.ID)
		logrus.WithFields(logrus.Fields{
			"chat_id": message.Chat.ID,
			"user":    message.From.UserName,
			"text":    message.Text,
			"lang":    userLang,
		}).Info("Received message")

		// Determine message type and body
		var msgType telegrammodel.TelegramMessageType
		var msgBody string

		if message.Text != "" {
			msgType = telegrammodel.TelegramTextMessage
			msgBody = message.Text
		} else if len(message.Photo) > 0 {
			msgType = telegrammodel.TelegramImageMessage
			msgBody = message.Caption // Use caption as body for images
			if msgBody == "" {
				msgBody = "Image"
			}
		} else if message.Document != nil {
			msgType = telegrammodel.TelegramDocumentMessage
			msgBody = message.Document.FileName
			if message.Caption != "" {
				msgBody = message.Caption
			}
		} else if message.Video != nil {
			msgType = telegrammodel.TelegramVideoMessage
			msgBody = message.Caption
			if msgBody == "" {
				msgBody = "Video"
			}
		} else if message.Audio != nil {
			msgType = telegrammodel.TelegramAudioMessage
			msgBody = message.Audio.Title
			if msgBody == "" && message.Audio.FileName != "" {
				msgBody = message.Audio.FileName
			}
			if msgBody == "" {
				msgBody = "Audio"
			}
		} else if message.Sticker != nil {
			msgType = telegrammodel.TelegramStickerMessage
			msgBody = message.Sticker.Emoji
		} else if message.Location != nil {
			msgType = telegrammodel.TelegramLocationMessage
			msgBody = fmt.Sprintf("Location: %.6f, %.6f", message.Location.Latitude, message.Location.Longitude)
		} else if message.Contact != nil {
			msgType = telegrammodel.TelegramContactMessage
			msgBody = fmt.Sprintf("Contact: %s %s", message.Contact.FirstName, message.Contact.LastName)
		} else {
			msgType = telegrammodel.TelegramTextMessage
			msgBody = "Unsupported message type"
		}

		// Store incoming message in database
		var replyToID *int64
		if message.ReplyToMessage != nil {
			replyID := int64(message.ReplyToMessage.MessageID)
			replyToID = &replyID
		}

		incomingMsg := telegrammodel.TelegramIncomingMsg{
			TelegramChatID:      fmt.Sprintf("%d", message.Chat.ID),
			TelegramSenderID:    fmt.Sprintf("%d", message.From.ID),
			TelegramSenderName:  message.From.UserName,
			TelegramMessageBody: msgBody,
			TelegramMessageType: msgType,
			TelegramIsGroup:     message.Chat.IsGroup() || message.Chat.IsSuperGroup(),
			TelegramReceivedAt:  message.Time(),
			TelegramMessageID:   int64(message.MessageID),
			TelegramReplyToID:   replyToID,
			TelegramMsgStatus:   "seen", // Mark as seen immediately
		}

		// Check if message already exists to prevent duplicates
		var existing telegrammodel.TelegramIncomingMsg
		if err := h.db.Where("telegram_chat_id = ? AND telegram_message_id = ?", incomingMsg.TelegramChatID, incomingMsg.TelegramMessageID).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := h.db.Create(&incomingMsg).Error; err != nil {
					logrus.WithError(err).Error("Failed to store incoming Telegram message")
				}
			} else {
				logrus.WithError(err).Error("Failed to check existing Telegram message")
			}
		} // else message already exists, skip

		// Send typing indicator before responding
		h.sendTypingAction(message.Chat.ID)

		// Quick response (remove delay for better UX)
		// time.Sleep(1 * time.Second)

		// Acknowledge receipt in user's language
		responseText := fmt.Sprintf(h.getLocalizedMessage(userLang, "message_received"), msgBody)
		msg := tgbotapi.NewMessage(message.Chat.ID, responseText)
		h.bot.Send(msg)
	}
}

// HandleCommand processes commands sent by users
func (h *TelegramHelper) HandleCommand(message *tgbotapi.Message) {
	command := message.Command()
	userLang := h.getUserLanguage(message.From.ID)

	logrus.WithFields(logrus.Fields{
		"command": command,
		"chat_id": message.Chat.ID,
		"user":    message.From.UserName,
		"lang":    userLang,
	}).Info("Received command")

	// Store command message in database
	var replyToID *int64
	if message.ReplyToMessage != nil {
		replyID := int64(message.ReplyToMessage.MessageID)
		replyToID = &replyID
	}

	incomingMsg := telegrammodel.TelegramIncomingMsg{
		TelegramChatID:      fmt.Sprintf("%d", message.Chat.ID),
		TelegramSenderID:    fmt.Sprintf("%d", message.From.ID),
		TelegramSenderName:  message.From.UserName,
		TelegramMessageBody: message.Text, // Store the full command text
		TelegramMessageType: telegrammodel.TelegramTextMessage,
		TelegramIsGroup:     message.Chat.IsGroup() || message.Chat.IsSuperGroup(),
		TelegramReceivedAt:  message.Time(),
		TelegramMessageID:   int64(message.MessageID),
		TelegramReplyToID:   replyToID,
		TelegramMsgStatus:   "seen", // Mark as seen immediately
	}

	// Check if command message already exists to prevent duplicates
	var existing telegrammodel.TelegramIncomingMsg
	if err := h.db.Where("telegram_chat_id = ? AND telegram_message_id = ?", incomingMsg.TelegramChatID, incomingMsg.TelegramMessageID).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			if err := h.db.Create(&incomingMsg).Error; err != nil {
				logrus.WithError(err).Error("Failed to store incoming Telegram command")
			}
		} else {
			logrus.WithError(err).Error("Failed to check existing Telegram command")
		}
	} // else command already exists, skip

	// // Send immediate acknowledgment
	// ackMsg := tgbotapi.NewMessage(message.Chat.ID, "✅")
	// h.bot.Send(ackMsg)

	// Send typing indicator before responding
	h.sendTypingAction(message.Chat.ID)

	// Quick acknowledgment (remove delay for better UX)
	// time.Sleep(500 * time.Millisecond)

	switch command {
	case "start":
		// Create inline keyboard for start menu
		keyboard := h.CreateStartKeyboard(userLang)
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "welcome"))
		msg.ReplyMarkup = keyboard
		h.bot.Send(msg)

	case "help":
		// Create inline keyboard for help menu
		keyboard := h.CreateHelpKeyboard(userLang)
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "help"))
		msg.ReplyMarkup = keyboard
		h.bot.Send(msg)

	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "unknown_command"))
		h.bot.Send(msg)
	}
}

// HandleCallbackQuery processes callback queries from inline keyboard buttons
func (h *TelegramHelper) HandleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	userLang := h.getUserLanguage(callback.From.ID)

	logrus.WithFields(logrus.Fields{
		"callback_data": callback.Data,
		"chat_id":       callback.Message.Chat.ID,
		"user":          callback.From.UserName,
		"lang":          userLang,
	}).Info("Received callback query")

	// // Answer the callback query
	// // You can customize the text if needed, e.g. "Processing..."
	// answer := tgbotapi.NewCallback(callback.ID, "")
	// h.bot.Request(answer)

	// Send typing indicator before responding
	h.sendTypingAction(callback.Message.Chat.ID)

	// Quick response (remove delay for better UX)
	// time.Sleep(300 * time.Millisecond)

	// Handle the callback data
	switch callback.Data {
	case "start":
		keyboard := h.CreateStartKeyboard(userLang)
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "welcome"))
		editMsg.ReplyMarkup = &keyboard
		h.bot.Send(editMsg)

	case "help":
		keyboard := h.CreateHelpKeyboard(userLang)
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "help"))
		editMsg.ReplyMarkup = &keyboard
		h.bot.Send(editMsg)

	case "info":
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "bot_info"))
		editMsg.ParseMode = "Markdown"
		h.bot.Send(editMsg)

	case "commands":
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "commands_list"))
		editMsg.ParseMode = "Markdown"
		h.bot.Send(editMsg)

	case "language":
		keyboard := h.CreateLanguageKeyboard()
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "select_language"))
		editMsg.ReplyMarkup = &keyboard
		h.bot.Send(editMsg)

	case "button1":
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "button1"))
		h.bot.Send(editMsg)

	case "button2":
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "button2"))
		h.bot.Send(editMsg)

	default:
		// Check if it's a language selection
		if strings.HasPrefix(callback.Data, "lang_") {
			langCode := strings.TrimPrefix(callback.Data, "lang_")
			err := h.setUserLanguage(callback.From.ID, langCode)
			if err != nil {
				logrus.WithError(err).Error("Failed to set user language")
				editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, "Error setting language")
				h.bot.Send(editMsg)
				return
			}

			responseText := h.getLocalizedMessage(langCode, "language_set")
			editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, responseText)
			h.bot.Send(editMsg)
		} else if strings.HasPrefix(callback.Data, "react_") {
			// Handle reaction callback (e.g., "react_emoji_messageid")
			parts := strings.Split(callback.Data, "_")
			if len(parts) >= 3 {
				emoji := parts[1]
				messageIDStr := parts[2]
				messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
				if err != nil {
					logrus.WithError(err).Error("Invalid message ID in reaction callback")
					return
				}

				// Update reaction in database
				if err := h.db.Model(&telegrammodel.TelegramIncomingMsg{}).
					Where("telegram_chat_id = ? AND telegram_message_id = ?", fmt.Sprintf("%d", callback.Message.Chat.ID), messageID).
					Updates(map[string]interface{}{
						"telegram_reaction_emoji": emoji,
						"telegram_reacted_by":     callback.From.UserName,
						"telegram_reacted_at":     time.Now(),
					}).Error; err != nil {
					logrus.WithError(err).Error("Failed to update Telegram reaction")
				}

				// Send confirmation
				responseText := fmt.Sprintf("Reaction %s added!", emoji)
				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, responseText)
				h.bot.Send(msg)
			}
		} else {
			editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "unknown_button"))
			h.bot.Send(editMsg)
		}
	}
}

// CreateStartKeyboard creates the inline keyboard for the start menu
func (h *TelegramHelper) CreateStartKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "info"), "info"),
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "commands"), "commands"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "language"), "language"),
		),
	)
}

// CreateHelpKeyboard creates the inline keyboard for the help menu
func (h *TelegramHelper) CreateHelpKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "start"), "start"),
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "info"), "info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "commands"), "commands"),
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "language"), "language"),
		),
	)
}

// CreateLanguageKeyboard creates the inline keyboard for language selection
func (h *TelegramHelper) CreateLanguageKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇮🇩 Indonesia", "lang_id"),
			tgbotapi.NewInlineKeyboardButtonData("🇺🇸 English", "lang_en"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇪🇸 Español", "lang_es"),
			tgbotapi.NewInlineKeyboardButtonData("🇫🇷 Français", "lang_fr"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇩🇪 Deutsch", "lang_de"),
			tgbotapi.NewInlineKeyboardButtonData("🇵🇹 Português", "lang_pt"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇷🇺 Русский", "lang_ru"),
			tgbotapi.NewInlineKeyboardButtonData("🇯🇵 日本語", "lang_jp"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇨🇳 中文", "lang_cn"),
			tgbotapi.NewInlineKeyboardButtonData("🇸🇦 العربية", "lang_ar"),
		),
	)
}

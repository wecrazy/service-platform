package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"service-platform/internal/config"
	"service-platform/internal/pkg/fun"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// HandlePing handles the "ping" command
func HandlePing(ctx context.Context, v *events.Message, stanzaID string, originalSenderJID string, userLang string, client *whatsmeow.Client, rdb *redis.Client, db *gorm.DB) {
	// Prepare localized Pong message
	langMsg := NewLanguageMsgTranslation(userLang)

	langMsg.Texts = map[string]string{
		fun.LangID: "Pong! 🏓",
		fun.LangEN: "Pong! 🏓",
		fun.LangES: "¡Pong! 🏓",
		fun.LangFR: "Pong! 🏓",
		fun.LangDE: "Pong! 🏓",
		fun.LangPT: "Pong! 🏓",
		fun.LangRU: "Понг! 🏓",
		fun.LangJP: "ポン！ 🏓",
		fun.LangCN: "乒乓！ 🏓",
		fun.LangAR: "بونغ! 🏓",
	}

	// Calculate latency if timestamp is available
	if !v.Info.Timestamp.IsZero() {
		latency := time.Since(v.Info.Timestamp)
		// Format latency to be readable (e.g. 120ms)
		latencyStr := fmt.Sprintf(" (%s)", latency.Round(time.Millisecond))
		for k, val := range langMsg.Texts {
			langMsg.Texts[k] = val + latencyStr
		}
	}

	SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, langMsg, userLang, client, rdb, db)
}

// HandlePprof handles the ".pprof" command to generate and send a CPU profile
func HandlePprof(ctx context.Context, v *events.Message, stanzaID string, originalSenderJID string, userLang string, client *whatsmeow.Client, rdb *redis.Client, db *gorm.DB) {
	cpuIntervalSec := config.GetConfig().Default.CPUCaptureInterval
	if cpuIntervalSec <= 0 {
		cpuIntervalSec = 10 // default to 10 seconds if not set properly
	}

	// Get message text to check for custom interval
	var messageText string
	if conv := v.Message.GetConversation(); conv != "" {
		messageText = conv
	} else if ext := v.Message.GetExtendedTextMessage(); ext != nil {
		messageText = ext.GetText()
	}

	// Parse custom interval from message
	parsedInterval := fun.ExtractFirstInteger(messageText)
	if parsedInterval > 0 {
		cpuIntervalSec = parsedInterval
	}

	// Notify user that profiling started
	langMsg := NewLanguageMsgTranslation(userLang)
	langMsg.Texts = map[string]string{
		fun.LangID: fmt.Sprintf("Sedang mengambil profil CPU selama %d detik... ⏳", cpuIntervalSec),
		fun.LangEN: fmt.Sprintf("Capturing CPU profile for %d seconds... ⏳", cpuIntervalSec),
		fun.LangES: fmt.Sprintf("Capturando perfil de CPU por %d segundos... ⏳", cpuIntervalSec),
		fun.LangFR: fmt.Sprintf("Capture du profil CPU pendant %d secondes... ⏳", cpuIntervalSec),
		fun.LangDE: fmt.Sprintf("CPU-Profil wird für %d Sekunden aufgezeichnet... ⏳", cpuIntervalSec),
		fun.LangPT: fmt.Sprintf("Capturando perfil de CPU por %d segundos... ⏳", cpuIntervalSec),
		fun.LangRU: fmt.Sprintf("Запись профиля ЦП в течение %d секунд... ⏳", cpuIntervalSec),
		fun.LangJP: fmt.Sprintf("CPUプロファイルを%d秒間キャプチャしています... ⏳", cpuIntervalSec),
		fun.LangCN: fmt.Sprintf("正在捕获 CPU 配置文件 %d 秒... ⏳", cpuIntervalSec),
		fun.LangAR: fmt.Sprintf("جاري التقاط ملف تعريف وحدة المعالجة المركزية لمدة %d ثوانٍ... ⏳", cpuIntervalSec),
	}
	SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, langMsg, userLang, client, rdb, db)

	// Create temporary file
	tempDir := os.TempDir()
	fileName := fmt.Sprintf("cpu_profile_%d.pprof", time.Now().Unix())
	filePath := filepath.Join(tempDir, fileName)

	f, err := os.Create(filePath)
	if err != nil {
		logrus.Errorf("Failed to create pprof file: %v", err)
		langMsg.Texts = map[string]string{
			fun.LangID: "Gagal membuat file profil CPU ❌",
			fun.LangEN: "Failed to create CPU profile file ❌",
			fun.LangES: "Error al crear el archivo de perfil de CPU ❌",
			fun.LangFR: "Échec de la création du fichier de profil CPU ❌",
			fun.LangDE: "Fehler beim Erstellen der CPU-Profil-Datei ❌",
			fun.LangPT: "Falha ao criar arquivo de perfil de CPU ❌",
			fun.LangRU: "Не удалось создать файл профиля ЦП ❌",
			fun.LangJP: "CPUプロファイルファイルの作成に失敗しました ❌",
			fun.LangCN: "创建 CPU 配置文件失败 ❌",
			fun.LangAR: "فشل إنشاء ملف تعريف وحدة المعالجة المركزية ❌",
		}
		SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, langMsg, userLang, client, rdb, db)
		return
	}

	// Start profiling
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		os.Remove(filePath)
		logrus.Errorf("Failed to start CPU profile: %v", err)
		langMsg.Texts = map[string]string{
			fun.LangID: "Gagal memulai profil CPU ❌",
			fun.LangEN: "Failed to start CPU profile ❌",
			fun.LangES: "Error al iniciar el perfil de CPU ❌",
			fun.LangFR: "Échec du démarrage du profil CPU ❌",
			fun.LangDE: "Fehler beim Starten des CPU-Profils ❌",
			fun.LangPT: "Falha ao iniciar perfil de CPU ❌",
			fun.LangRU: "Не удалось запустить профиль ЦП ❌",
			fun.LangJP: "CPUプロファイルの開始に失敗しました ❌",
			fun.LangCN: "启动 CPU 配置文件失败 ❌",
			fun.LangAR: "فشل بدء ملف تعريف وحدة المعالجة المركزية ❌",
		}
		SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, langMsg, userLang, client, rdb, db)
		return
	}

	time.Sleep(time.Duration(cpuIntervalSec) * time.Second)
	pprof.StopCPUProfile()
	f.Close()

	// Read file content
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		os.Remove(filePath)
		logrus.Errorf("Failed to read pprof file: %v", err)
		langMsg.Texts = map[string]string{
			fun.LangID: "Gagal membaca file profil CPU ❌",
			fun.LangEN: "Failed to read CPU profile file ❌",
			fun.LangES: "Error al leer el archivo de perfil de CPU ❌",
			fun.LangFR: "Échec de la lecture du fichier de profil CPU ❌",
			fun.LangDE: "Fehler beim Lesen der CPU-Profil-Datei ❌",
			fun.LangPT: "Falha ao ler arquivo de perfil de CPU ❌",
			fun.LangRU: "Не удалось прочитать файл профиля ЦП ❌",
			fun.LangJP: "CPUプロファイルファイルの読み込みに失敗しました ❌",
			fun.LangCN: "读取 CPU 配置文件失败 ❌",
			fun.LangAR: "فشل قراءة ملف تعريف وحدة المعالجة المركزية ❌",
		}
		SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, langMsg, userLang, client, rdb, db)
		return
	}

	// Upload file
	resp, err := client.Upload(ctx, fileData, whatsmeow.MediaDocument)
	if err != nil {
		os.Remove(filePath)
		logrus.Errorf("Failed to upload pprof file: %v", err)
		langMsg.Texts = map[string]string{
			fun.LangID: "Gagal mengunggah file profil CPU ❌",
			fun.LangEN: "Failed to upload CPU profile file ❌",
			fun.LangES: "Error al subir el archivo de perfil de CPU ❌",
			fun.LangFR: "Échec du téléchargement du fichier de profil CPU ❌",
			fun.LangDE: "Fehler beim Hochladen der CPU-Profil-Datei ❌",
			fun.LangPT: "Falha ao enviar arquivo de perfil de CPU ❌",
			fun.LangRU: "Не удалось загрузить файл профиля ЦП ❌",
			fun.LangJP: "CPUプロファイルファイルのアップロードに失敗しました ❌",
			fun.LangCN: "上传 CPU 配置文件失败 ❌",
			fun.LangAR: "فشل تحميل ملف تعريف وحدة المعالجة المركزية ❌",
		}
		SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, langMsg, userLang, client, rdb, db)
		return
	}

	// Send document message
	docMsg := &waE2E.DocumentMessage{
		URL:           proto.String(resp.URL),
		Mimetype:      proto.String("application/octet-stream"),
		Title:         proto.String(fileName),
		FileEncSHA256: resp.FileEncSHA256,
		FileSHA256:    resp.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(fileData))),
		MediaKey:      resp.MediaKey,
		FileName:      proto.String(fileName),
		DirectPath:    proto.String(resp.DirectPath),
	}

	msg := &waE2E.Message{
		DocumentMessage: docMsg,
	}

	// Parse JID
	jid, _ := types.ParseJID(originalSenderJID)

	_, err = client.SendMessage(ctx, jid, msg)
	if err != nil {
		logrus.Errorf("Failed to send pprof message: %v", err)
	}

	// Remove file
	os.Remove(filePath)
}

// HandleMetrics handles the "get metrics" command
func HandleMetrics(ctx context.Context, v *events.Message, stanzaID string, originalSenderJID string, userLang string, client *whatsmeow.Client, rdb *redis.Client, db *gorm.DB) {
	health := fun.GlobalSystemMonitor.GetHealthStatus(db)
	l1, l5, l15 := fun.GlobalSystemMonitor.GetCPULoad()
	rx, tx := fun.GlobalSystemMonitor.GetNetworkStats()
	diskUsed, diskTotal := fun.GlobalSystemMonitor.GetDiskUsage()
	topCPU := fun.GlobalSystemMonitor.GetTopProcesses("cpu", 10)
	topMem := fun.GlobalSystemMonitor.GetTopProcesses("mem", 10)

	// Helper to format bytes
	formatBytes := func(b uint64) string {
		const unit = 1024
		if b < unit {
			return fmt.Sprintf("%d B", b)
		}
		div, exp := int64(unit), 0
		for n := b / unit; n >= unit; n /= unit {
			div *= unit
			exp++
		}
		return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
	}

	// Labels for translation
	labels := map[string]map[string]string{
		"title": {
			fun.LangID: "📊 *Metrik Server*",
			fun.LangEN: "📊 *Server Metrics*",
			fun.LangES: "📊 *Métricas del Servidor*",
			fun.LangFR: "📊 *Métriques du Serveur*",
			fun.LangDE: "📊 *Server-Metriken*",
			fun.LangPT: "📊 *Métricas do Servidor*",
			fun.LangRU: "📊 *Метрики Сервера*",
			fun.LangJP: "📊 *サーバーメトリクス*",
			fun.LangCN: "📊 *服务器指标*",
			fun.LangAR: "📊 *مقاييس الخادم*",
		},
		"time": {
			fun.LangID: "🕒 *Waktu Server:*",
			fun.LangEN: "🕒 *Server Time:*",
			fun.LangES: "🕒 *Hora del Servidor:*",
			fun.LangFR: "🕒 *Heure du Serveur:*",
			fun.LangDE: "🕒 *Serverzeit:*",
			fun.LangPT: "🕒 *Hora do Servidor:*",
			fun.LangRU: "🕒 *Время Сервера:*",
			fun.LangJP: "🕒 *サーバー時間:*",
			fun.LangCN: "🕒 *服务器时间:*",
			fun.LangAR: "🕒 *وقت الخادم:*",
		},
		"os": {
			fun.LangID: "🖥️ *Sistem OS:*",
			fun.LangEN: "🖥️ *OS System:*",
			fun.LangES: "🖥️ *Sistema OS:*",
			fun.LangFR: "🖥️ *Système OS:*",
			fun.LangDE: "🖥️ *Betriebssystem:*",
			fun.LangPT: "🖥️ *Sistema OS:*",
			fun.LangRU: "🖥️ *ОС:*",
			fun.LangJP: "🖥️ *OSシステム:*",
			fun.LangCN: "🖥️ *操作系统:*",
			fun.LangAR: "🖥️ *نظام التشغيل:*",
		},
		"uptime": {
			fun.LangID: "⏱️ *Waktu Aktif (OS):*",
			fun.LangEN: "⏱️ *Uptime (OS):*",
			fun.LangES: "⏱️ *Tiempo de actividad (OS):*",
			fun.LangFR: "⏱️ *Temps de disponibilité (OS):*",
			fun.LangDE: "⏱️ *Betriebszeit (OS):*",
			fun.LangPT: "⏱️ *Tempo de atividade (OS):*",
			fun.LangRU: "⏱️ *Время работы (OS):*",
			fun.LangJP: "⏱️ *稼働時間 (OS):*",
			fun.LangCN: "⏱️ *运行时间 (OS):*",
			fun.LangAR: "⏱️ *وقت التشغيل (OS):*",
		},
		"app_uptime": {
			fun.LangID: "⏱️ *Waktu Aktif (App):*",
			fun.LangEN: "⏱️ *Uptime (App):*",
			fun.LangES: "⏱️ *Tiempo de actividad (App):*",
			fun.LangFR: "⏱️ *Temps de disponibilité (App):*",
			fun.LangDE: "⏱️ *Betriebszeit (App):*",
			fun.LangPT: "⏱️ *Tempo de atividade (App):*",
			fun.LangRU: "⏱️ *Время работы (App):*",
			fun.LangJP: "⏱️ *稼働時間 (App):*",
			fun.LangCN: "⏱️ *运行时间 (App):*",
			fun.LangAR: "⏱️ *وقت التشغيل (App):*",
		},
		"cpu_load": {
			fun.LangID: "🧠 *Beban CPU (1/5/15m):*",
			fun.LangEN: "🧠 *CPU Load (1/5/15m):*",
			fun.LangES: "🧠 *Carga CPU (1/5/15m):*",
			fun.LangFR: "🧠 *Charge CPU (1/5/15m):*",
			fun.LangDE: "🧠 *CPU-Last (1/5/15m):*",
			fun.LangPT: "🧠 *Carga CPU (1/5/15m):*",
			fun.LangRU: "🧠 *Загрузка ЦП (1/5/15м):*",
			fun.LangJP: "🧠 *CPU負荷 (1/5/15分):*",
			fun.LangCN: "🧠 *CPU 负载 (1/5/15分):*",
			fun.LangAR: "🧠 *حمل المعالج (1/5/15د):*",
		},
		"memory": {
			fun.LangID: "💾 *Memori:*",
			fun.LangEN: "💾 *Memory:*",
			fun.LangES: "💾 *Memoria:*",
			fun.LangFR: "💾 *Mémoire:*",
			fun.LangDE: "💾 *Speicher:*",
			fun.LangPT: "💾 *Memória:*",
			fun.LangRU: "💾 *Память:*",
			fun.LangJP: "💾 *メモリ:*",
			fun.LangCN: "💾 *内存:*",
			fun.LangAR: "💾 *الذاكرة:*",
		},
		"disk": {
			fun.LangID: "💿 *Penyimpanan:*",
			fun.LangEN: "💿 *Storage:*",
			fun.LangES: "💿 *Almacenamiento:*",
			fun.LangFR: "💿 *Stockage:*",
			fun.LangDE: "💿 *Speicherplatz:*",
			fun.LangPT: "💿 *Armazenamento:*",
			fun.LangRU: "💿 *Хранилище:*",
			fun.LangJP: "💿 *ストレージ:*",
			fun.LangCN: "💿 *存储:*",
			fun.LangAR: "💿 *التخزين:*",
		},
		"network": {
			fun.LangID: "🌐 *Jaringan:*",
			fun.LangEN: "🌐 *Network:*",
			fun.LangES: "🌐 *Red:*",
			fun.LangFR: "🌐 *Réseau:*",
			fun.LangDE: "🌐 *Netzwerk:*",
			fun.LangPT: "🌐 *Rede:*",
			fun.LangRU: "🌐 *Сеть:*",
			fun.LangJP: "🌐 *ネットワーク:*",
			fun.LangCN: "🌐 *网络:*",
			fun.LangAR: "🌐 *الشبكة:*",
		},
		"top_cpu": {
			fun.LangID: "🔥 *Top 10 Proses CPU:*",
			fun.LangEN: "🔥 *Top 10 CPU Processes:*",
			fun.LangES: "🔥 *Top 10 Procesos CPU:*",
			fun.LangFR: "🔥 *Top 10 Processus CPU:*",
			fun.LangDE: "🔥 *Top 10 CPU-Prozesse:*",
			fun.LangPT: "🔥 *Top 10 Processos CPU:*",
			fun.LangRU: "🔥 *Топ 10 Процессов ЦП:*",
			fun.LangJP: "🔥 *CPUプロセス トップ10:*",
			fun.LangCN: "🔥 *前 10 个 CPU 进程:*",
			fun.LangAR: "🔥 *أعلى 10 عمليات معالج:*",
		},
		"top_mem": {
			fun.LangID: "🧠 *Top 10 Proses RAM:*",
			fun.LangEN: "🧠 *Top 10 RAM Processes:*",
			fun.LangES: "🧠 *Top 10 Procesos RAM:*",
			fun.LangFR: "🧠 *Top 10 Processus RAM:*",
			fun.LangDE: "🧠 *Top 10 RAM-Prozesse:*",
			fun.LangPT: "🧠 *Top 10 Processos RAM:*",
			fun.LangRU: "🧠 *Топ 10 Процессов RAM:*",
			fun.LangJP: "🧠 *RAMプロセス トップ10:*",
			fun.LangCN: "🧠 *前 10 个 RAM 进程:*",
			fun.LangAR: "🧠 *أعلى 10 عمليات ذاكرة:*",
		},
		"goroutines": {
			fun.LangID: "🧵 *Goroutines:*",
			fun.LangEN: "🧵 *Goroutines:*",
			fun.LangES: "🧵 *Goroutines:*",
			fun.LangFR: "🧵 *Goroutines:*",
			fun.LangDE: "🧵 *Goroutines:*",
			fun.LangPT: "🧵 *Goroutines:*",
			fun.LangRU: "🧵 *Горутины:*",
			fun.LangJP: "🧵 *ゴルーチン:*",
			fun.LangCN: "🧵 *Goroutines:*",
			fun.LangAR: "🧵 *Goroutines:*",
		},
		"sent": {
			fun.LangID: "Kirim", fun.LangEN: "Sent", fun.LangES: "Enviado", fun.LangFR: "Envoyé", fun.LangDE: "Gesendet", fun.LangPT: "Enviado", fun.LangRU: "Отправлено", fun.LangJP: "送信", fun.LangCN: "发送", fun.LangAR: "مرسل",
		},
		"received": {
			fun.LangID: "Terima", fun.LangEN: "Received", fun.LangES: "Recibido", fun.LangFR: "Reçu", fun.LangDE: "Empfangen", fun.LangPT: "Recebido", fun.LangRU: "Получено", fun.LangJP: "受信", fun.LangCN: "接收", fun.LangAR: "مستلم",
		},
		"used": {
			fun.LangID: "Terpakai", fun.LangEN: "Used", fun.LangES: "Usado", fun.LangFR: "Utilisé", fun.LangDE: "Verwendet", fun.LangPT: "Usado", fun.LangRU: "Использовано", fun.LangJP: "使用中", fun.LangCN: "已用", fun.LangAR: "مستخدم",
		},
		"free": {
			fun.LangID: "Bebas", fun.LangEN: "Free", fun.LangES: "Libre", fun.LangFR: "Libre", fun.LangDE: "Frei", fun.LangPT: "Livre", fun.LangRU: "Свободно", fun.LangJP: "空き", fun.LangCN: "空闲", fun.LangAR: "حر",
		},
	}

	// Helper to get label
	getLabel := func(key, lang string) string {
		if val, ok := labels[key][lang]; ok {
			return val
		}
		if val, ok := labels[key]["en"]; ok {
			return val
		}
		return key
	}

	// Build message for the specific user language
	var sb strings.Builder
	sb.WriteString(getLabel("title", userLang) + "\n\n")

	// Basic Info
	sb.WriteString(fmt.Sprintf("%s %s\n", getLabel("time", userLang), time.Now().Format(time.RFC1123)))
	sb.WriteString(fmt.Sprintf("%s %s/%s (%d CPUs)\n", getLabel("os", userLang), runtime.GOOS, runtime.GOARCH, runtime.NumCPU()))
	sb.WriteString(fmt.Sprintf("%s %v\n", getLabel("uptime", userLang), fun.GlobalSystemMonitor.GetOSUptime()))
	sb.WriteString(fmt.Sprintf("%s %v\n", getLabel("app_uptime", userLang), health["uptime"]))
	sb.WriteString(fmt.Sprintf("%s %d\n", getLabel("goroutines", userLang), health["goroutines"]))
	sb.WriteString(fmt.Sprintf("%s %.2f / %.2f / %.2f\n", getLabel("cpu_load", userLang), l1, l5, l15))

	// Memory
	if mem, ok := health["memory"].(map[string]interface{}); ok {
		sb.WriteString(fmt.Sprintf("%s\n", getLabel("memory", userLang)))
		if sysTotal, ok := mem["system_total_mb"]; ok {
			sysUsed := mem["system_used_mb"]
			sysUsage := mem["system_usage_percent"]
			sb.WriteString(fmt.Sprintf("  • System: %.2f MB / %.2f MB (%.2f%% %s)\n", sysUsed, sysTotal, sysUsage, getLabel("used", userLang)))
		}
		sb.WriteString(fmt.Sprintf("  • App Alloc: %.2f MB\n", mem["alloc_mb"]))
	}

	// Disk
	diskPercent := 0.0
	if diskTotal > 0 {
		diskPercent = float64(diskUsed) / float64(diskTotal) * 100
	}
	diskFree := diskTotal - diskUsed
	sb.WriteString(fmt.Sprintf("%s %.2f%% %s\n", getLabel("disk", userLang), diskPercent, getLabel("used", userLang)))
	sb.WriteString(fmt.Sprintf("  • %s: %s\n", getLabel("used", userLang), formatBytes(diskUsed)))
	sb.WriteString(fmt.Sprintf("  • %s: %s\n", getLabel("free", userLang), formatBytes(diskFree)))
	sb.WriteString(fmt.Sprintf("  • Total: %s\n", formatBytes(diskTotal)))

	// Network
	sb.WriteString(fmt.Sprintf("%s\n", getLabel("network", userLang)))
	sb.WriteString(fmt.Sprintf("  • %s: %s\n", getLabel("received", userLang), formatBytes(rx)))
	sb.WriteString(fmt.Sprintf("  • %s: %s\n", getLabel("sent", userLang), formatBytes(tx)))

	// Top CPU
	sb.WriteString(fmt.Sprintf("\n%s\n", getLabel("top_cpu", userLang)))
	for _, p := range topCPU {
		sb.WriteString(fmt.Sprintf("  • [%s] %s: %.1f%%\n", p.PID, p.Command, p.CPU))
	}

	// Top Memory
	sb.WriteString(fmt.Sprintf("\n%s\n", getLabel("top_mem", userLang)))
	for _, p := range topMem {
		sb.WriteString(fmt.Sprintf("  • [%s] %s: %.1f%% (%s)\n", p.PID, p.Command, p.Memory, formatBytes(p.RSS*1024)))
	}

	langMsg := NewLanguageMsgTranslation(userLang)
	langMsg.Texts = map[string]string{
		userLang: sb.String(),
	}

	SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, langMsg, userLang, client, rdb, db)
}

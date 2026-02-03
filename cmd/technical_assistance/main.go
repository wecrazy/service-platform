package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"service-platform/cmd/technical_assistance/app_installer"
	"service-platform/cmd/technical_assistance/config"
	"service-platform/cmd/technical_assistance/controllers"
	"service-platform/cmd/technical_assistance/database"
	"service-platform/cmd/technical_assistance/middleware"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/model/op_model"
	"service-platform/cmd/technical_assistance/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func main() {

	exePath, errE := os.Executable()
	if errE != nil {
		log.Fatalf("Error getting executable path: %v", errE)
	}

	exeDir := filepath.Dir(exePath)
	log.Printf("Executable directory: %s", exeDir)

	// Load the .env file from the same directory as the executable
	err1 := godotenv.Load(filepath.Join(exeDir, ".env"))
	if err1 != nil {
		log.Fatalf("Error loading .env file: %v", err1)
	}

	if app_installer.Init() {
		fmt.Print("Apps Installed")
		return
	}

	// Dynamic update conf.yaml
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Error loading .yaml conf :%v", err)
	}

	go config.WatchConfig()

	// Increase resource limitations for LINUX
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	appLogDir := os.Getenv("APP_LOG_DIR")

	// Convert the string to an integer
	redisDbNo, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		fmt.Println("Failed to convert REDIS_DB to int:", err)
		return
	}

	redisDB := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT"), // Redis server address
		Password: os.Getenv("REDIS_PASSWORD"),                             // No password set
		DB:       redisDbNo,                                               // Default database
	})

	// Ping the Redis server to check the connection
	pong, err := redisDB.Ping(context.Background()).Result()
	if err != nil {
		fmt.Println("Failed to connect to Redis:", err)
		return
	}
	fmt.Println("Connected to Redis:", pong)

	webHostPort := os.Getenv("APP_LISTEN")
	ginMode := os.Getenv("GIN_MODE")
	gin.SetMode(ginMode)

	//PREPARE DB
	db, err := database.InitAndCheckDB(
		os.Getenv("MYSQL_USER_DB"),
		os.Getenv("MYSQL_PASS_DB"),
		os.Getenv("MYSQL_HOST_DB"),
		os.Getenv("MYSQL_PORT_DB"),
		os.Getenv("MYSQL_NAME_DB"),
	)
	if err != nil {
		log.Fatalf("Database main setup failed: %v", err)
	}
	// db_call_center, err := database.InitAndCheckDB(
	// 	os.Getenv("MYSQL_USER_DB"),
	// 	os.Getenv("MYSQL_PASS_DB"),
	// 	os.Getenv("MYSQL_HOST_DB"),
	// 	os.Getenv("MYSQL_PORT_DB"),
	// 	os.Getenv("MYSQL_NAME_CALL_CENTER_DB"),
	// )
	// if err != nil {
	// 	log.Fatalf("Database cc setup failed: %v", err)
	// }

	db_konfirmasi_pengerjaan, err := database.InitAndCheckDB(
		os.Getenv("MYSQL_USER_DB_KONFIRMASI_PENGERJAAN"),
		os.Getenv("MYSQL_PASS_DB_KONFIRMASI_PENGERJAAN"),
		os.Getenv("MYSQL_HOST_DB_KONFIRMASI_PENGERJAAN"),
		os.Getenv("MYSQL_PORT_DB_KONFIRMASI_PENGERJAAN"),
		os.Getenv("MYSQL_NAME_DB_KONFIRMASI_PENGERJAAN"),
	)
	if err != nil {
		log.Fatalf("Database kukuh setup failed: %v", err)
	}

	database.AutoMigrateWeb(db)

	//HANDLE LOG WRITING
	if err := os.MkdirAll(appLogDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	logFile, err := os.OpenFile(filepath.Join(appLogDir, "apps.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	// SET LOG OUTPUT
	log.SetOutput(logFile)

	//HANDLE WEB ENDPOINT
	r := gin.Default()
	r.Use(middleware.LoggerMiddleware(logFile))
	r.Use(middleware.CacheControlMiddleware())
	r.Use(middleware.SanitizeMiddleware())
	r.Use(middleware.SanitizeCsvMiddleware())
	r.Use(middleware.SecurityControlMiddleware())
	r.Use(cors.Default())

	routes.StaticFile(r)
	routes.HtmlRoutes(r, db, nil, db_konfirmasi_pengerjaan, redisDB)

	// // Start goroutine FOR LOG BACKUP DAILY
	// go func() {
	// 	// Set the interval duration
	// 	interval := 30 * time.Minute

	// 	//CHECK ONE TIME
	// 	checkLogWrite(appLogDir)

	// 	// Create a ticker for the interval
	// 	ticker := time.NewTicker(interval)
	// 	defer ticker.Stop()

	// 	// Loop to perform the task at each interval
	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			// Call the function you want to execute at each interval
	// 			checkLogWrite(appLogDir)
	// 		}
	// 	}
	// }()

	// Start goroutine FOR LOG BACKUP DAILY
	go func() {
		// Set the interval duration
		interval := 30 * time.Minute

		// CHECK ONE TIME
		checkLogWrite(appLogDir)

		// Create a ticker for the interval
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Loop using for range (recommended)
		for range ticker.C {
			// Call the function at each interval
			checkLogWrite(appLogDir)
		}
	}()

	// Scheduler Daily Report TA
	schedulerGenerateReportTA(db_konfirmasi_pengerjaan, db)

	// Delete soon !!
	insertDataSummaryTA(db_konfirmasi_pengerjaan, db)

	fmt.Println("Web Hosted at http://localhost" + webHostPort + "/")
	if err := r.Run(webHostPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func checkLogWrite(logDir string) {
	// Create the log directory if it doesn't exist
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	// Get the current date in the format "YYYY_MM_DD"
	currentDate := time.Now().AddDate(0, 0, -1).Format("2006_01_02")

	// Path to the source log file
	sourceLogFilePath := filepath.Join(logDir, "apps.log")

	// Path to the target log file with today's date
	targetLogFilePath := filepath.Join(logDir, fmt.Sprintf("apps_%s.log", currentDate))

	// Check if the target log file for today's date exists
	if _, err := os.Stat(targetLogFilePath); os.IsNotExist(err) {
		// Target log file doesn't exist, create it

		// Read the content of the source log file
		content, err := os.ReadFile(sourceLogFilePath)
		if err != nil {
			log.Fatal(err)
		}

		// Write the content to the target log file
		err = os.WriteFile(targetLogFilePath, content, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}

		// Empty the source log file
		err = os.WriteFile(sourceLogFilePath, nil, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}

		message := fmt.Sprintf("Log files Backup %s Done", currentDate)
		fmt.Println(message)
	}
}

// func schedulerGenerateReportTA(db *gorm.DB) {
// 	s := gocron.NewScheduler(time.UTC)

// 	// Convert Jakarta times to UTC:
// 	// Jakarta 6:00 PM (UTC+7) → 11:00 AM UTC
// 	// Jakarta 10:10 PM (UTC+7) → 3:10 PM UTC
// 	_, err1 := s.Cron("0 11 * * *").Do(func() {
// 		// controllers.GenerateDailyReportTAActivity(db)
// 	})

// 	_, err2 := s.Cron("10 15 * * *").Do(func() {
// 		controllers.GenerateDailyReportTAActivity(db)
// 	})
// 	// Check if cron jobs were successfully added
// 	if err1 != nil || err2 != nil {
// 		log.Printf("Failed to schedule daily report TA: %v, %v", err1, err2)
// 	}

// 	// Start the scheduler asynchronously
// 	s.StartAsync()

// 	log.Println("Scheduler started successfully using UTC time")
// }

func schedulerGenerateReportTA(db *gorm.DB, dbWeb *gorm.DB) {
	// Load Jakarta time zone
	jakartaLoc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Fatalf("Failed to load Asia/Jakarta location: %v", err)
	}

	s := gocron.NewScheduler(time.UTC) // gocron still runs in UTC

	// Convert 17:00 and 21:00 Jakarta to UTC
	t1Jakarta := time.Date(2000, 1, 1, 17, 0, 0, 0, jakartaLoc) // 5 PM
	t2Jakarta := time.Date(2000, 1, 1, 21, 0, 0, 0, jakartaLoc) // 9 PM

	// Convert to UTC
	t1UTC := t1Jakarta.In(time.UTC)
	t2UTC := t2Jakarta.In(time.UTC)

	// Format for `.At()` (HH:MM format)
	t1Str := t1UTC.Format("15:04") // should be 10:00 if Jakarta is UTC+7
	t2Str := t2UTC.Format("15:04") // should be 14:00 if Jakarta is UTC+7

	// Schedule using converted UTC time
	_, err1 := s.Every(1).Day().At(t1Str).Do(func() {
		insertDataSummaryTA(db, dbWeb)
		controllers.GenerateDailyReportTAActivity(db, dbWeb)
	})

	_, err2 := s.Every(1).Day().At(t2Str).Do(func() {
		insertDataSummaryTA(db, dbWeb)
		controllers.GenerateDailyReportTAActivity(db, dbWeb)
	})

	if err1 != nil || err2 != nil {
		log.Printf("Failed to schedule daily report TA: %v, %v", err1, err2)
	}

	s.StartAsync()

	log.Printf("Scheduler started: 17:00 Jakarta (runs at %s UTC), 21:00 Jakarta (runs at %s UTC)", t1Str, t2Str)
}

func insertDataSummaryTA(db *gorm.DB, dbWeb *gorm.DB) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(-7 * time.Hour)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Add(-7 * time.Hour)

	var totalPendingLeft, totalErrorLeft, totalFollowedUp, totalTAStandBy int64
	_ = db.Model(&op_model.Pending{}).
		Where("date BETWEEN ? AND ?", startOfDay, endOfDay).
		Count(&totalPendingLeft)
	_ = db.Model(&op_model.Error{}).
		Where("date BETWEEN ? AND ?", startOfDay, endOfDay).
		Count(&totalErrorLeft)
	_ = db.Model(&op_model.LogAct{}).
		Where("date BETWEEN ? AND ?", startOfDay, endOfDay).
		Count(&totalFollowedUp)

	// Count totalTAStandBy using dbWeb.Model(&model.Admin{}) where LastLogin is today (00:00:00 to 23:59:59)
	startOfDayWeb := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDayWeb := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	_ = dbWeb.Model(&model.Admin{}).
		Where("updated_at BETWEEN ? AND ?", startOfDayWeb, endOfDayWeb).
		Where("LOWER(fullname) LIKE ? OR LOWER(fullname) LIKE ?", "%assistance%", "%assistant%").
		Count(&totalTAStandBy)

	dataForInsert := op_model.TAHandledData{
		CountFollowedUp:      int(totalFollowedUp),
		CountPendingDataLeft: int(totalPendingLeft),
		CountErrorDataLeft:   int(totalErrorLeft),
		TotalTAStandBy:       int(totalTAStandBy),
	}

	var existing op_model.TAHandledData
	err := dbWeb.Where("created_at BETWEEN ? AND ?", startOfDayWeb, endOfDayWeb).First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No record for today, create new
			if err := dbWeb.Create(&dataForInsert).Error; err != nil {
				log.Printf("failed to insert TAHandledData: %v", err)
			}
		} else {
			log.Printf("failed to query TAHandledData: %v", err)
		}
	} else {
		// Record exists, update it
		if err := dbWeb.Model(&existing).Updates(dataForInsert).Error; err != nil {
			log.Printf("failed to update TAHandledData: %v", err)
		}
	}
}

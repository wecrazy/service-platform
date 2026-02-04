package main

import (
	"log"
	"os"
	"os/signal"
	"service-platform/internal/config"
	"syscall"
)

// ===== EXAMPLE 1: Simple Single Config =====

func ExampleSimpleUsage() {

	// In main.go
	config.TechnicalAssistance.MustInit("technical_assistance")

	// Anywhere in your codebase
	dbHost := config.TechnicalAssistance.Get().MYSQL_HOST_DB
	dbPort := config.TechnicalAssistance.Get().MYSQL_PORT_DB

	log.Printf("Connecting to: %s:%d", dbHost, dbPort)
}

// ===== EXAMPLE 2: Multi-Package Usage =====

// In payment package
func ExamplePaymentPackage() {
	// Each package initializes its own config
	config.WebPanel.MustInit("web_panel")

	// Use config
	serviceName := config.WebPanel.Get().App.VersionName
	port := config.WebPanel.Get().App.Port

	log.Printf("Starting %s on port %d", serviceName, port)
}

// ===== EXAMPLE 3: With Hot Reload =====

func ExampleWithHotReload() {
	// Init config
	config.TechnicalAssistance.MustInit("legacy")

	// Start watcher in background
	go func() {
		if err := config.TechnicalAssistance.Watch(); err != nil {
			log.Printf("Watch error: %v", err)
		}
	}()
	defer config.TechnicalAssistance.Close()

	// Use config (always gets latest values)
	for {
		dbHost := config.TechnicalAssistance.Get().MYSQL_HOST_DB
		log.Printf("Current DB host: %s", dbHost)
		// ... your logic
	}
}

// ===== EXAMPLE 4: Custom File Path =====

func ExampleCustomPath() {
	// Load from specific file
	config.TechnicalAssistance.MustLoad("/etc/myapp/legacy.yaml")

	// Use normally
	log.Printf("DB: %s:%d",
		config.TechnicalAssistance.Get().MYSQL_HOST_DB,
		config.TechnicalAssistance.Get().MYSQL_PORT_DB)
}

// ===== EXAMPLE 5: Error Handling =====

func ExampleErrorHandling() {
	// Non-panic version for graceful handling
	if err := config.WebPanel.Init("payment"); err != nil {
		log.Printf("Warning: Payment config not loaded: %v", err)
		// Fallback logic or exit
		return
	}

	// Check if loaded
	if config.WebPanel.IsLoaded() {
		log.Println("Payment service config ready")
	}
}

// ===== EXAMPLE 6: Multiple Configs in Main =====

func ExampleMainPattern() {
	log.Println("🚀 Starting application...")

	// Initialize all configs
	config.TechnicalAssistance.MustInit("legacy")
	config.WebPanel.MustInit("web_panel")

	// Start watchers
	go config.TechnicalAssistance.Watch()
	go config.WebPanel.Watch()

	defer func() {
		config.TechnicalAssistance.Close()
		config.WebPanel.Close()
	}()

	// Initialize services
	initDatabase()
	// initPaymentProcessor()
	// initWorkers()

	log.Println("✅ All services started")

	// Wait for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("👋 Shutting down...")
}

// ===== EXAMPLE 7: Service Initialization =====

type DatabaseService struct {
	host string
	port int
}

func initDatabase() *DatabaseService {
	// Direct access to config
	return &DatabaseService{
		host: config.TechnicalAssistance.Get().MYSQL_HOST_DB,
		// port: config.TechnicalAssistance.Get().MYSQL_PORT_DB,
	}
}

type PaymentProcessor struct{}

// func initPaymentProcessor() *PaymentProcessor {
// 	svc := config.WebPanel.Get().App.Name
// 	// log.Printf("Payment processor: %s on port %d", svc.Name, svc.Port)
// 	return &PaymentProcessor{}
// }

func startWorker(id int, queueURL string) {
	log.Printf("Worker #%d processing: %s", id, queueURL)
}

// ===== EXAMPLE 8: Environment-Aware =====

func ExampleEnvironmentAware() {
	// Set environment
	os.Setenv("ENV", "prod")

	// Automatically loads config.prod.yaml
	config.TechnicalAssistance.MustInit("config")

	log.Printf("Loaded %s environment config", os.Getenv("ENV"))
}

// ===== EXAMPLE 9: Conditional Loading =====

func ExampleConditionalLoading() {
	// Only load what you need
	config.TechnicalAssistance.MustInit("legacy")

	// Payment service is optional
	if err := config.WebPanel.Init("payment"); err == nil {
		log.Println("Payment service enabled")
	} else {
		log.Println("Payment service disabled")
	}

}

// ===== EXAMPLE 10: Real-World Handler =====

// func HandleDatabaseRequest() {
// 	// No need to pass config around
// 	// Just use it directly
// 	db := connectToDatabase(
// 		config.TechnicalAssistance.Get().MYSQL_HOST_DB,
// 		config.TechnicalAssistance.Get().Database.Port,
// 	)
// 	defer db.Close()

// 	// Your logic here
// }

// func HandlePaymentRequest() {
// 	// Each handler accesses its own config
// 	processor := NewPaymentProcessor(
// 		config.WebPanel.Get().Service.Name,
// 		config.WebPanel.Get().Database.DSN,
// 	)
// 	processor.Process()
// }

// Dummy implementations
type DB struct{}

func (d *DB) Close() {}

func connectToDatabase(host string, port int) *DB {
	log.Printf("Connected to %s:%d", host, port)
	return &DB{}
}

func NewPaymentProcessor(name, dsn string) *Processor {
	return &Processor{name: name, dsn: dsn}
}

type Processor struct {
	name string
	dsn  string
}

func (p *Processor) Process() {
	log.Printf("Processing payment via %s", p.name)
}

package cli

import (
	"fmt"

	"service-platform/internal/config"
)

// MenuItem represents a single executable make target.
type MenuItem struct {
	Name        string
	Description string
	MakeTarget  string
	LongRunning bool // Service that runs continuously (uses ExecProcess)
	Dangerous   bool // Requires y/n confirmation before running
}

// Category groups related commands under a section.
type Category struct {
	Name        string
	Icon        string
	Description string
	MultiSelect bool // Allow space-bar multi-select (for batch execution)
	Items       []MenuItem
}

// loadConfig attempts to load the service-platform config.
// Returns nil if config unavailable (e.g. first build, CI).
func loadConfig() *config.TypeServicePlatform {
	defer func() { recover() }() //nolint:errcheck // graceful fallback
	config.ServicePlatform.MustInit("service-platform")
	cfg := config.ServicePlatform.Get()
	return &cfg
}

// allCategories returns every menu category with their commands.
// Descriptions are populated from config when available.
func allCategories() []Category {
	cfg := loadConfig()

	// Helper: returns ":<port>" if cfg is loaded, otherwise fallback.
	port := func(p int, fallback string) string {
		if cfg != nil && p > 0 {
			return fmt.Sprintf(":%d", p)
		}
		return fallback
	}

	// Service port descriptions from config
	apiDesc := fmt.Sprintf("REST API server %s", port(cfgAppPort(cfg), ""))
	grpcDesc := fmt.Sprintf("gRPC auth service %s", port(cfgGRPCPort(cfg), ""))
	schedulerDesc := fmt.Sprintf("Cron job scheduler %s", port(cfgSchedulerPort(cfg), ""))
	waDesc := fmt.Sprintf("WhatsApp bot (whatsmeow) %s", port(cfgWAGRPCPort(cfg), ""))
	twilioDesc := fmt.Sprintf("Twilio WhatsApp service %s", port(cfgTwilioGRPCPort(cfg), ""))
	telegramDesc := fmt.Sprintf("Telegram bot service %s", port(cfgTelegramGRPCPort(cfg), ""))
	docsDesc := fmt.Sprintf("Serve godoc %s", port(cfgGoDocPort(cfg), ""))

	return []Category{
		{
			Name:        "Run Services",
			Icon:        "🏃",
			Description: "Start services (runs in foreground)",
			Items: []MenuItem{
				{Name: "API Server", Description: apiDesc, MakeTarget: "run-api", LongRunning: true},
				{Name: "gRPC Server", Description: grpcDesc, MakeTarget: "run-grpc", LongRunning: true},
				{Name: "Scheduler", Description: schedulerDesc, MakeTarget: "run-scheduler", LongRunning: true},
				{Name: "WhatsApp (whatsmeow)", Description: waDesc, MakeTarget: "run-wa", LongRunning: true},
				{Name: "Twilio WhatsApp", Description: twilioDesc, MakeTarget: "run-twilio-whatsapp", LongRunning: true},
				{Name: "Telegram Bot", Description: telegramDesc, MakeTarget: "run-telegram", LongRunning: true},
				{Name: "All Services", Description: "API + gRPC + Scheduler + WhatsApp", MakeTarget: "run-all", LongRunning: true},
			},
		},
		{
			Name:        "Build Services",
			Icon:        "📦",
			Description: "Compile service binaries to bin/",
			MultiSelect: true,
			Items: []MenuItem{
				{Name: "API Server", Description: "Build bin/api", MakeTarget: "build-api"},
				{Name: "gRPC Server", Description: "Build bin/grpc", MakeTarget: "build-grpc"},
				{Name: "Scheduler", Description: "Build bin/scheduler", MakeTarget: "build-scheduler"},
				{Name: "WhatsApp", Description: "Build bin/wa", MakeTarget: "build-wa"},
				{Name: "Twilio WhatsApp", Description: "Build bin/twilio-whatsapp", MakeTarget: "build-twilio-whatsapp"},
				{Name: "Telegram", Description: "Build bin/telegram", MakeTarget: "build-telegram"},
				{Name: "Monitoring", Description: "Build bin/monitoring", MakeTarget: "build-monitoring"},
				{Name: "N8N", Description: "Build bin/n8n", MakeTarget: "build-n8n"},
				{Name: "CLI", Description: "Build bin/cli (this tool)", MakeTarget: "build-cli"},
				{Name: "All Services", Description: "Build everything", MakeTarget: "build"},
			},
		},
		{
			Name:        " Database",
			Icon:        "🗃️",
			Description: "Migrations and seeding",
			Items: []MenuItem{
				{Name: "Run Migrations", Description: "Apply pending migrations", MakeTarget: "migrate-up"},
				{Name: "Rollback Migration", Description: "Rollback last migration", MakeTarget: "migrate-down"},
				{Name: "Migration Status", Description: "Check migration status", MakeTarget: "migrate-status"},
				{Name: "Reset Migrations", Description: "Reset all migrations", MakeTarget: "migrate-reset", Dangerous: true},
				{Name: "Build Migrate CLI", Description: "Build bin/migrate", MakeTarget: "build-migrate"},
				{Name: "Seed All", Description: "Run all database seeds", MakeTarget: "seed"},
				{Name: "Seed Users", Description: "Roles, features, privileges", MakeTarget: "seed-users"},
				{Name: "Seed WhatsApp", Description: "Users, languages, auto-replies", MakeTarget: "seed-whatsapp"},
				{Name: "Seed Telegram", Description: "Telegram users", MakeTarget: "seed-telegram"},
				{Name: "Seed Config", Description: "App config, bad words, regions", MakeTarget: "seed-config"},
			},
		},
		{
			Name:        "Monitoring",
			Icon:        "📊",
			Description: "Prometheus, Grafana, Loki, Tempo",
			Items: []MenuItem{
				{Name: "Start", Description: "Start monitoring stack", MakeTarget: "monitoring-start"},
				{Name: "Stop", Description: "Stop monitoring services", MakeTarget: "monitoring-stop"},
				{Name: "Restart", Description: "Restart monitoring", MakeTarget: "monitoring-restart"},
				{Name: "Deep Restart", Description: "Restart with Grafana cache cleanup", MakeTarget: "monitoring-deep-restart"},
				{Name: "Status", Description: "Check monitoring status", MakeTarget: "monitoring-status"},
				{Name: "Cleanup", Description: "Clean monitoring data", MakeTarget: "monitoring-cleanup"},
				{Name: "Ensure Running", Description: "Start if not running", MakeTarget: "monitoring-ensure-running"},
				{Name: "Install Service", Description: "Install as system service", MakeTarget: "install-monitoring"},
				{Name: "Uninstall Service", Description: "Remove system service", MakeTarget: "uninstall-monitoring", Dangerous: true},
			},
		},
		{
			Name:        "N8N Workflows",
			Icon:        "🤖",
			Description: "N8N workflow automation",
			Items: []MenuItem{
				{Name: "Start N8N", Description: "Start N8N service", MakeTarget: "n8n-start", LongRunning: true},
				{Name: "Stop N8N", Description: "Stop N8N service", MakeTarget: "n8n-stop"},
				{Name: "Import Workflows", Description: "Import from internal/n8n/workflows", MakeTarget: "n8n-import"},
				{Name: "Export Workflows", Description: "Export to internal/n8n/workflows", MakeTarget: "n8n-export"},
				{Name: "Clear Workflows", Description: "Clear all workflows (with prompt)", MakeTarget: "n8n-clear", Dangerous: true},
				{Name: "Force Clear", Description: "Clear without confirmation", MakeTarget: "n8n-clear-force", Dangerous: true},
			},
		},
		{
			Name:        "Testing",
			Icon:        "🧪",
			Description: "Unit and integration tests",
			Items: []MenuItem{
				{Name: "Run All Tests", Description: "Unit + integration + migration tests", MakeTarget: "test"},
				{Name: "Twilio Tests", Description: "Twilio WhatsApp tests", MakeTarget: "test-twilio"},
				{Name: "Twilio Sandbox", Description: "Test Twilio sandbox connectivity", MakeTarget: "test-twilio-sandbox"},
				{Name: "MongoDB Tests", Description: "MongoDB-specific tests", MakeTarget: "test-mongo"},
			},
		},
		{
			Name:        "K6 Load Testing",
			Icon:        "⚡",
			Description: "Performance and load tests",
			Items: []MenuItem{
				{Name: "Health Check", Description: "Basic health check load test", MakeTarget: "k6-health-check"},
				{Name: "Smoke Test", Description: "Quick validation test", MakeTarget: "k6-smoke-test"},
				{Name: "Login Flow", Description: "Login flow load test", MakeTarget: "k6-login-test"},
				{Name: "Stress Test", Description: "Progressive load increase", MakeTarget: "k6-stress-test"},
				{Name: "K6 Status", Description: "Check k6 container status", MakeTarget: "k6-status"},
				{Name: "Stop K6", Description: "Stop running k6 tests", MakeTarget: "k6-stop"},
				{Name: "View Results", Description: "View k6 test results", MakeTarget: "k6-results"},
			},
		},
		{
			Name:        " MongoDB",
			Icon:        "🗄️",
			Description: "MongoDB container management",
			Items: []MenuItem{
				{Name: "Start MongoDB", Description: "Start MongoDB container", MakeTarget: "mongo-up"},
				{Name: "Stop MongoDB", Description: "Stop MongoDB container", MakeTarget: "mongo-down"},
				{Name: "View Logs", Description: "Stream MongoDB logs", MakeTarget: "mongo-logs", LongRunning: true},
				{Name: "Status", Description: "Check container status", MakeTarget: "mongo-status"},
				{Name: "Full Check", Description: "Check MongoDB + MongoExpress", MakeTarget: "mongo-check"},
			},
		},
		{
			Name:        " Development",
			Icon:        "🛠️",
			Description: "Config, docs, and dev tools",
			Items: []MenuItem{
				{Name: "Dev Config", Description: "Switch to dev configuration", MakeTarget: "config-dev"},
				{Name: "Prod Config", Description: "Switch to prod configuration", MakeTarget: "config-prod"},
				{Name: "Generate Swagger", Description: "Generate Swagger API docs", MakeTarget: "swagger"},
				{Name: "Generate Protobuf", Description: "Generate gRPC code from .proto", MakeTarget: "proto-gen"},
				{Name: "Install Doc Tools", Description: "Install protoc-gen-doc + godoc", MakeTarget: "docs-install"},
				{Name: "Serve Docs", Description: docsDesc, MakeTarget: "docs-serve", LongRunning: true},
				{Name: "Clean Dashboard", Description: "Remove dashboard tables", MakeTarget: "clean-dashboard"},
				{Name: "Test Tempo", Description: "Test Tempo tracing setup", MakeTarget: "tempo-test"},
				{Name: "Verify Observability", Description: "Verify full observability stack", MakeTarget: "observability-verify"},
				{Name: "Health Check All", Description: "Comprehensive health check", MakeTarget: "health-check-all"},
			},
		},
		{
			Name:        "Telegram Service",
			Icon:        "📱",
			Description: "Telegram system service management",
			Items: []MenuItem{
				{Name: "Install Service", Description: "Install as system service", MakeTarget: "install-telegram"},
				{Name: "Uninstall Service", Description: "Remove system service", MakeTarget: "uninstall-telegram", Dangerous: true},
				{Name: "Service Status", Description: "Check service status", MakeTarget: "status-telegram"},
			},
		},
	}
}

// ── Config port accessors (nil-safe) ────────────────────────────────────────

func cfgAppPort(cfg *config.TypeServicePlatform) int {
	if cfg == nil {
		return 0
	}
	return cfg.App.Port
}

func cfgGRPCPort(cfg *config.TypeServicePlatform) int {
	if cfg == nil {
		return 0
	}
	return cfg.GRPC.Port
}

func cfgSchedulerPort(cfg *config.TypeServicePlatform) int {
	if cfg == nil {
		return 0
	}
	return cfg.Schedules.Port
}

func cfgWAGRPCPort(cfg *config.TypeServicePlatform) int {
	if cfg == nil {
		return 0
	}
	return cfg.Whatsnyan.GRPCPort
}

func cfgTwilioGRPCPort(cfg *config.TypeServicePlatform) int {
	if cfg == nil {
		return 0
	}
	return cfg.Twilio.GRPCPort
}

func cfgTelegramGRPCPort(cfg *config.TypeServicePlatform) int {
	if cfg == nil {
		return 0
	}
	return cfg.Telegram.GRPCPort
}

func cfgGoDocPort(cfg *config.TypeServicePlatform) int {
	if cfg == nil {
		return 0
	}
	return cfg.Metrics.GoDocPort
}

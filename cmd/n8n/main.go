// Package main is the entry point for the n8n WhatsApp bridge service.
//
// It manages the n8n workflow-automation container and its Postgres dependency
// via Podman, and runs an HTTP bridge server that accepts POST /send requests
// and forwards them to the WhatsApp gRPC service.
//
// OpenTelemetry tracing (Tempo) and Loki-based logging are enabled when the
// corresponding config sections are populated.
//
// Commands:
//
//	--start    Start the n8n container, Postgres, and the WhatsApp bridge (default)
//	--stop     Stop n8n and its Postgres container
//	--help     Show usage
//
// Usage:
//
//	go run cmd/n8n/main.go
//	go run cmd/n8n/main.go --stop
//	make n8n-start
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"service-platform/internal/config"
	"service-platform/pkg/fun"
	"service-platform/pkg/logger"
	"service-platform/pkg/observability"
	pb "service-platform/proto"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// main loads configuration and dispatches to startN8n, stopN8n, or printHelp
// based on CLI arguments. Defaults to startN8n when no arguments are given.
func main() {
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--start":
			startN8n()
		case "--stop":
			stopN8n()
		case "--help", "-h":
			printHelp()
		default:
			fmt.Printf("Unknown argument: %s\n", os.Args[1])
			printHelp()
			os.Exit(1)
		}
	} else {
		startN8n()
	}
}

// startN8n pulls and runs the n8n and Postgres containers via Podman,
// then starts the WhatsApp HTTP bridge in a background goroutine.
func startN8n() {
	fmt.Println("🚀 Starting N8N workflow automation...")

	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	go config.ServicePlatform.Watch()

	cfg := config.ServicePlatform.Get()

	// Check if Podman is available
	if !fun.IsPodmanAvailable() {
		log.Fatal("Podman is not available. Please install Podman to run N8N.")
	}

	// Check if N8N container is already running
	if fun.IsContainerRunning("n8n") {
		fmt.Println("✅ N8N is already running")
		return
	}

	// Get current working directory for mounting workflows
	workFlowDir, err := fun.FindValidDirectory([]string{
		"internal/n8n/workflows",
		"../internal/n8n/workflows",
		"../../internal/n8n/workflows",
	})
	if err != nil {
		log.Fatalf("Failed to find workflows directory: %v", err)
	}

	absWorkFlowDir, err := filepath.Abs(workFlowDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path for workflows directory: %v", err)
	}

	// Create n8n network if it doesn't exist
	exec.Command("podman", "network", "create", "n8n-net").Run()

	// Start Postgres for n8n
	startPostgres()

	// Run N8N Podman container
	n8nHost := cfg.N8N.Host
	n8nPort := cfg.N8N.Port
	args := []string{
		"run", "-d", "--name", "service-platform-n8n", "--replace",
		"--network", "n8n-net",
		"-p", fmt.Sprintf("%d:%d", n8nPort, n8nPort),
		"-e", fmt.Sprintf("N8N_PORT=%d", n8nPort),
		"-e", "N8N_METRICS=true",
		"-e", "N8N_PERSONALIZATION_ENABLED=false", // Disable telemetry/data collection
		"-e", "N8N_DIAGNOSTICS_ENABLED=false", // Disable diagnostics
		"-e", "DB_TYPE=postgresdb",
		"-e", "DB_POSTGRESDB_HOST=n8n-postgres",
		"-e", "DB_POSTGRESDB_PORT=5432",
		"-e", "DB_POSTGRESDB_DATABASE=n8n",
		"-e", "DB_POSTGRESDB_USER=n8n",
		"-e", "DB_POSTGRESDB_PASSWORD=n8n",
		"-v", "n8n_data:/home/node/.n8n",
		"-v", fmt.Sprintf("%s:/home/node/workflows", absWorkFlowDir),
		"n8nio/n8n:latest",
	}
	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to start N8N: %v", err)
	}
	fmt.Printf("✅ N8N started successfully on http://%s:%d\n", n8nHost, n8nPort)

	// Start WhatsApp bridge HTTP server in a goroutine
	go startWhatsAppBridge(&cfg)

	// Wait for services to stabilize
	fmt.Println("✅ All services started. Press Ctrl+C to stop.")
	select {}
}

// startPostgres launches a Postgres 16 Alpine container on the n8n-net network.
// If the container is already running the function returns immediately.
func startPostgres() {
	if fun.IsContainerRunning("n8n-postgres") {
		fmt.Println("✅ N8N Postgres is already running")
		return
	}

	fmt.Println("🚀 Starting N8N Postgres database...")
	args := []string{
		"run", "-d", "--name", "n8n-postgres", "--replace",
		"--network", "n8n-net",
		"-e", "POSTGRES_DB=n8n",
		"-e", "POSTGRES_USER=n8n",
		"-e", "POSTGRES_PASSWORD=n8n",
		"-v", "n8n_postgres_data:/var/lib/postgresql/data",
		"postgres:16-alpine",
	}
	cmd := exec.Command("podman", args...)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to start N8N Postgres: %v", err)
	}
	// Give Postgres some time to start
	exec.Command("sleep", "5").Run()
}

// stopN8n stops both the n8n and n8n-postgres Podman containers.
func stopN8n() {
	fmt.Println("🛑 Stopping N8N...")
	if err := fun.StopContainer("service-platform-n8n"); err != nil {
		log.Printf("Failed to stop N8N: %v", err)
	}
	if err := fun.StopContainer("n8n-postgres"); err != nil {
		log.Printf("Failed to stop N8N Postgres: %v", err)
	}
	fmt.Println("✅ N8N stopped successfully")
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Println("Service Platform - N8N Workflow Automation Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  n8n [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  --start          Start N8N service")
	fmt.Println("  --stop           Stop N8N service")
	fmt.Println("  --help, -h       Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  n8n --start")
	fmt.Println("  n8n --stop")
}

// WhatsAppBridgeRequest represents the incoming request from n8n workflow
type WhatsAppBridgeRequest struct {
	Phone   string `json:"phone"`
	Message string `json:"message"`
}

// WhatsAppBridgeResponse represents the response sent back to n8n
type WhatsAppBridgeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	ID      string `json:"id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// handleWhatsAppSendRequest returns an HTTP handler that forwards WhatsApp send requests via gRPC.
func handleWhatsAppSendRequest(tracer trace.Tracer, appLogger *logrus.Entry, grpcAddr string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Start a span for this request
		ctx, span := tracer.Start(r.Context(), "whatsapp.send_message")
		defer span.End()

		appLogger.WithField("method", r.Method).Debugf("Incoming request")

		// Only accept POST requests
		if r.Method != http.MethodPost {
			appLogger.Warnf("Invalid method: %s", r.Method)
			span.SetStatus(codes.Error, "Invalid HTTP method")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body
		var req WhatsAppBridgeRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			appLogger.Errorf("Failed to read request body: %v", err)
			span.SetStatus(codes.Error, "Failed to read body")
			respondJSON(w, http.StatusBadRequest, WhatsAppBridgeResponse{
				Success: false,
				Error:   "Failed to read request body",
			})
			return
		}
		defer r.Body.Close()

		appLogger.Debugf("Raw request body: %s", string(body))

		// Try to unmarshal as JSON
		if err := json.Unmarshal(body, &req); err != nil {
			appLogger.Warnf("Failed to unmarshal JSON: %v, trying form data", err)
			// Try to parse as form data
			r.Body = io.NopCloser(bytes.NewBuffer(body))
			if err := r.ParseForm(); err != nil {
				appLogger.Errorf("Invalid request format: %v", err)
				span.SetStatus(codes.Error, "Invalid request format")
				respondJSON(w, http.StatusBadRequest, WhatsAppBridgeResponse{
					Success: false,
					Error:   "Invalid request format",
				})
				return
			}
			req.Phone = r.FormValue("phone")
			req.Message = r.FormValue("message")
			appLogger.Debugf("Parsed from form: phone=%s, message_len=%d", req.Phone, len(req.Message))
		} else {
			appLogger.Debugf("Parsed from JSON: phone=%s, message_len=%d", req.Phone, len(req.Message))
		}

		// Validate input
		if req.Phone == "" || req.Message == "" {
			appLogger.Warnf("Missing required parameters: phone=%s, message_len=%d", req.Phone, len(req.Message))
			span.SetStatus(codes.Error, "Missing phone or message")
			respondJSON(w, http.StatusBadRequest, WhatsAppBridgeResponse{
				Success: false,
				Error:   "Missing 'phone' or 'message' parameter",
			})
			return
		}

		// Add attributes to span
		span.SetAttributes(
			attribute.String("phone", req.Phone),
			attribute.Int("message_length", len(req.Message)),
		)

		appLogger.WithFields(logrus.Fields{
			"phone":          req.Phone,
			"message_length": len(req.Message),
		}).Infof("Processing WhatsApp message")

		// Format phone number - remove non-numeric chars
		phoneClean := ""
		for _, char := range req.Phone {
			if char >= '0' && char <= '9' {
				phoneClean += string(char)
			}
		}

		// Ensure phone number has country code
		if len(phoneClean) > 0 && phoneClean[0] == '0' {
			phoneClean = "62" + phoneClean[1:]
		}

		jid := fmt.Sprintf("%s@s.whatsapp.net", phoneClean)
		appLogger.Debugf("Formatted phone: %s -> JID: %s", req.Phone, jid)

		// Connect to WhatsApp gRPC service (with tracing)
		_, connectSpan := tracer.Start(ctx, "grpc.connect")
		conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		connectSpan.End()
		if err != nil {
			appLogger.Errorf("Failed to connect to WhatsApp service at %s: %v", grpcAddr, err)
			span.SetStatus(codes.Error, "Failed to connect to gRPC service")
			respondJSON(w, http.StatusServiceUnavailable, WhatsAppBridgeResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to connect to WhatsApp service: %v", err),
			})
			return
		}
		defer conn.Close()

		// Create WhatsApp service client and send
		client := pb.NewWhatsAppServiceClient(conn)
		msgReq := &pb.SendMessageRequest{
			To: jid,
			Content: &pb.MessageContent{
				ContentType: &pb.MessageContent_Text{
					Text: req.Message,
				},
			},
		}

		_, sendSpan := tracer.Start(ctx, "grpc.send_message")
		resp, err := client.SendMessage(ctx, msgReq)
		sendSpan.End()

		if err != nil {
			appLogger.WithFields(logrus.Fields{
				"phone": req.Phone,
				"error": err.Error(),
			}).Errorf("Failed to send WhatsApp message")
			span.SetStatus(codes.Error, "Failed to send message")
			respondJSON(w, http.StatusInternalServerError, WhatsAppBridgeResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to send message: %v", err),
			})
			return
		}

		appLogger.WithFields(logrus.Fields{
			"phone":      req.Phone,
			"message_id": resp.Id,
		}).Infof("WhatsApp message sent successfully")

		span.SetStatus(codes.Ok, "Message sent successfully")
		respondJSON(w, http.StatusOK, WhatsAppBridgeResponse{
			Success: resp.Success,
			Message: resp.Message,
			ID:      resp.Id,
		})
	}
}

// startWhatsAppBridge starts an HTTP server that forwards requests to WhatsApp gRPC service
func startWhatsAppBridge(cfg *config.TypeServicePlatform) {
	// Initialize logger with Loki support
	logger.InitLogrus()

	bridgeServiceName := cfg.N8N.BridgeServiceName
	if bridgeServiceName == "" {
		bridgeServiceName = "whatsapp-bridge"
	}
	appLogger := logrus.WithField("service", bridgeServiceName)

	// Initialize Tempo tracer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	shutdown, err := observability.InitTracer(ctx)
	cancel()
	if err != nil {
		appLogger.Warnf("Failed to initialize tracer: %v", err)
	} else if shutdown != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			shutdown(ctx)
		}()
	}

	tracer := otel.Tracer(bridgeServiceName)
	bridgeHost := cfg.N8N.BridgeHost
	bridgePort := cfg.N8N.BridgePort
	grpcAddr := fmt.Sprintf("%s:%d", cfg.Whatsnyan.GRPCHost, cfg.Whatsnyan.GRPCPort)

	http.HandleFunc("/send", handleWhatsAppSendRequest(tracer, appLogger, grpcAddr))

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, span := tracer.Start(r.Context(), "health_check")
		defer span.End()
		span.SetStatus(codes.Ok, "Health check passed")
		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": bridgeServiceName,
		})
	})

	appLogger.Infof("🌉 WhatsApp Bridge HTTP server listening on http://%s:%d", bridgeHost, bridgePort)
	appLogger.Infof("   → Forwards requests to WhatsApp gRPC at %s", grpcAddr)
	appLogger.Infof("   → Loki logging enabled: %v", cfg.Observability.Loki.Enabled)
	appLogger.Infof("   → Tempo tracing enabled: %v", cfg.Observability.Tempo.Enabled)

	// Listen on all interfaces (0.0.0.0) so containers can reach it
	if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", bridgePort), nil); err != nil {
		appLogger.Fatalf("WhatsApp Bridge server error: %v", err)
	}
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

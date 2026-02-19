package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"service-platform/internal/config"
	"service-platform/internal/pkg/logger"
	"service-platform/internal/twilio"
	pb "service-platform/proto"
)

// TwilioWhatsAppServer implements the TwilioWhatsAppService gRPC service
type TwilioWhatsAppServer struct {
	pb.UnimplementedTwilioWhatsAppServiceServer
	twilioClient *twilio.Client
	isDev        bool // Track if running in dev mode for conditional logging
}

// logDebugInfo logs detailed debug information only in dev mode
func (s *TwilioWhatsAppServer) logDebugInfo(msgType string, format string, args ...interface{}) {
	if !s.isDev {
		return
	}
	logrus.Infof("[%s] "+format, append([]interface{}{msgType}, args...)...)
}

// logDebugError logs detailed error debug information only in dev mode
func (s *TwilioWhatsAppServer) logDebugError(msgType string, format string, args ...interface{}) {
	if !s.isDev {
		return
	}
	logrus.Errorf("[%s] "+format, append([]interface{}{msgType}, args...)...)
}

// logError logs error messages in all modes (important for production monitoring)
func (s *TwilioWhatsAppServer) logError(msgType string, format string, args ...interface{}) {
	logrus.Errorf("[%s] "+format, append([]interface{}{msgType}, args...)...)
}

// logSuccess logs successful message sends in all modes
func (s *TwilioWhatsAppServer) logSuccess(msgType string, to string, sid string) {
	if !s.isDev {
		return
	}
	logrus.Infof("✅ [%s] Successfully sent to %s | SID: %s", msgType, to, sid)
}

// SendMessage handles text message sending via Twilio
func (s *TwilioWhatsAppServer) SendMessage(ctx context.Context, req *pb.TwilioSendMessageRequest) (*pb.TwilioSendMessageResponse, error) {
	if s.twilioClient == nil {
		s.logError("TEXT MSG", "Twilio client not initialized")
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: "Twilio client not initialized",
		}, nil
	}

	s.logDebugInfo("TEXT MSG", "Sending to: %s | Body: %s", req.To, req.Body)

	sid, err := s.twilioClient.SendMessage(req.To, req.Body)
	if err != nil {
		s.logError("TEXT MSG", "Failed to send to %s: %v", req.To, err)
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send message: %v", err),
		}, nil
	}

	s.logSuccess("TEXT MSG", req.To, sid)

	return &pb.TwilioSendMessageResponse{
		Success:    true,
		MessageSid: sid,
		Message:    "Text message sent successfully",
		To:         req.To,
	}, nil
}

// SendImageMessage handles image message sending via Twilio
func (s *TwilioWhatsAppServer) SendImageMessage(ctx context.Context, req *pb.TwilioSendImageMessageRequest) (*pb.TwilioSendMessageResponse, error) {
	if s.twilioClient == nil {
		s.logError("IMAGE MSG", "Twilio client not initialized")
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: "Twilio client not initialized",
		}, nil
	}

	s.logDebugInfo("IMAGE MSG", "Sending to: %s | URL: %s | Caption: %s", req.To, req.ImageUrl, req.Caption)

	sid, err := s.twilioClient.SendMediaMessage(req.To, req.ImageUrl, req.Caption)
	if err != nil {
		s.logError("IMAGE MSG", "Failed to send to %s: %v", req.To, err)
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send image message: %v", err),
		}, nil
	}

	s.logSuccess("IMAGE MSG", req.To, sid)

	return &pb.TwilioSendMessageResponse{
		Success:    true,
		MessageSid: sid,
		Message:    "Image message sent successfully",
		To:         req.To,
	}, nil
}

// SendDocumentMessage handles document message sending via Twilio
func (s *TwilioWhatsAppServer) SendDocumentMessage(ctx context.Context, req *pb.TwilioSendDocumentMessageRequest) (*pb.TwilioSendMessageResponse, error) {
	if s.twilioClient == nil {
		s.logError("DOCUMENT MSG", "Twilio client not initialized")
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: "Twilio client not initialized",
		}, nil
	}

	s.logDebugInfo("DOCUMENT MSG", "Sending to: %s | URL: %s | Caption: %s", req.To, req.DocumentUrl, req.Caption)

	sid, err := s.twilioClient.SendMediaMessage(req.To, req.DocumentUrl, req.Caption)
	if err != nil {
		s.logError("DOCUMENT MSG", "Failed to send to %s: %v", req.To, err)
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send document message: %v", err),
		}, nil
	}

	s.logSuccess("DOCUMENT MSG", req.To, sid)

	return &pb.TwilioSendMessageResponse{
		Success:    true,
		MessageSid: sid,
		Message:    "Document message sent successfully",
		To:         req.To,
	}, nil
}

// SendVideoMessage handles video message sending via Twilio
func (s *TwilioWhatsAppServer) SendVideoMessage(ctx context.Context, req *pb.TwilioSendVideoMessageRequest) (*pb.TwilioSendMessageResponse, error) {
	if s.twilioClient == nil {
		s.logError("VIDEO MSG", "Twilio client not initialized")
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: "Twilio client not initialized",
		}, nil
	}

	s.logDebugInfo("VIDEO MSG", "Sending to: %s | URL: %s | Caption: %s", req.To, req.VideoUrl, req.Caption)

	sid, err := s.twilioClient.SendMediaMessage(req.To, req.VideoUrl, req.Caption)
	if err != nil {
		s.logError("VIDEO MSG", "Failed to send to %s: %v", req.To, err)
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send video message: %v", err),
		}, nil
	}

	s.logSuccess("VIDEO MSG", req.To, sid)

	return &pb.TwilioSendMessageResponse{
		Success:    true,
		MessageSid: sid,
		Message:    "Video message sent successfully",
		To:         req.To,
	}, nil
}

// SendAudioMessage handles audio/voice message sending via Twilio
func (s *TwilioWhatsAppServer) SendAudioMessage(ctx context.Context, req *pb.TwilioSendAudioMessageRequest) (*pb.TwilioSendMessageResponse, error) {
	if s.twilioClient == nil {
		s.logError("AUDIO MSG", "Twilio client not initialized")
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: "Twilio client not initialized",
		}, nil
	}

	s.logDebugInfo("AUDIO MSG", "Sending to: %s | URL: %s", req.To, req.AudioUrl)

	sid, err := s.twilioClient.SendMediaMessage(req.To, req.AudioUrl, "")
	if err != nil {
		s.logError("AUDIO MSG", "Failed to send to %s: %v", req.To, err)
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send audio message: %v", err),
		}, nil
	}

	s.logSuccess("AUDIO MSG", req.To, sid)

	return &pb.TwilioSendMessageResponse{
		Success:    true,
		MessageSid: sid,
		Message:    "Audio message sent successfully",
		To:         req.To,
	}, nil
}

// SendMediaMessage handles generic media message sending via Twilio (DEPRECATED: use specific methods)
func (s *TwilioWhatsAppServer) SendMediaMessage(ctx context.Context, req *pb.TwilioSendMediaMessageRequest) (*pb.TwilioSendMessageResponse, error) {
	if s.twilioClient == nil {
		s.logError("MEDIA MSG", "Twilio client not initialized")
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: "Twilio client not initialized",
		}, nil
	}

	s.logDebugInfo("MEDIA MSG", "Using deprecated method - consider using specific message type | To: %s | URL: %s | Type: %s", req.To, req.MediaUrl, req.MediaType)

	sid, err := s.twilioClient.SendMediaMessage(req.To, req.MediaUrl, req.Caption)
	if err != nil {
		s.logError("MEDIA MSG", "Failed to send to %s: %v", req.To, err)
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send media message: %v", err),
		}, nil
	}

	s.logSuccess("MEDIA MSG", req.To, sid)

	return &pb.TwilioSendMessageResponse{
		Success:    true,
		MessageSid: sid,
		Message:    "Media message sent successfully",
		To:         req.To,
	}, nil
}

// GetMessageStatus retrieves the delivery status of a message
func (s *TwilioWhatsAppServer) GetMessageStatus(ctx context.Context, req *pb.TwilioGetMessageStatusRequest) (*pb.TwilioGetMessageStatusResponse, error) {
	if s.twilioClient == nil {
		return &pb.TwilioGetMessageStatusResponse{
			Status: "error",
		}, nil
	}

	return &pb.TwilioGetMessageStatusResponse{
		Status:     "queued",
		MessageSid: req.MessageSid,
	}, nil
}

func main() {
	// Load configuration
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	go config.WatchConfig()

	// Initialize logger
	logger.InitLogrus()

	cfg := config.GetConfig()

	// Initialize Twilio client
	fmt.Println("🔧 Initializing Twilio WhatsApp client...")
	twilioClient, err := twilio.NewClient()
	if err != nil {
		msg := fmt.Sprintf("❌ Failed to initialize Twilio client: %v", err)
		fmt.Fprintln(os.Stderr, msg)
		logrus.Fatalf(msg)
	}
	fmt.Println("✅ Twilio WhatsApp client initialized successfully")
	defer twilioClient.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Determine if running in dev mode for conditional debug logging
	isDev := cfg.Twilio.IsDev

	server := &TwilioWhatsAppServer{
		twilioClient: twilioClient,
		isDev:        isDev,
	}
	pb.RegisterTwilioWhatsAppServiceServer(grpcServer, server)
	grpc.EnableTracing = true
	reflection.Register(grpcServer)

	// Start gRPC listener
	grpcAddress := fmt.Sprintf("%s:%d", cfg.Twilio.Host, cfg.Twilio.GRPCPort)
	grpcListener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", grpcAddress, err)
	}

	fmt.Printf("📡 Twilio WhatsApp gRPC server listening on %s\n", grpcAddress)

	// Start metrics HTTP server
	metricsAddr := fmt.Sprintf(":%d", cfg.Metrics.TwilioPort)
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		fmt.Printf("📊 Twilio WhatsApp Metrics server listening on %s\n", metricsAddr)
		if err := http.ListenAndServe(metricsAddr, nil); err != nil {
			logrus.Errorf("Twilio WhatsApp Metrics server error: %v", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logrus.Info("Shutting down Twilio WhatsApp service...")
		grpcServer.GracefulStop()
		os.Exit(0)
	}()

	// Start serving
	if err := grpcServer.Serve(grpcListener); err != nil {
		log.Fatalf("gRPC server error: %v", err)
	}
}

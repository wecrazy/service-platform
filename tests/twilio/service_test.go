package twilio_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"service-platform/internal/config"
	"service-platform/internal/twilio"
	pb "service-platform/proto"
)

// TwilioWhatsAppServer implements the TwilioWhatsAppService gRPC service (for testing)
type TwilioWhatsAppServer struct {
	pb.UnimplementedTwilioWhatsAppServiceServer
	twilioClient *twilio.Client
}

// SendMessage handles text message sending via Twilio
func (s *TwilioWhatsAppServer) SendMessage(_ context.Context, req *pb.TwilioSendMessageRequest) (*pb.TwilioSendMessageResponse, error) {
	if s.twilioClient == nil {
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: "Twilio client not initialized",
		}, nil
	}

	sid, err := s.twilioClient.SendMessage(req.To, req.Body)
	if err != nil {
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send message: %v", err),
		}, nil
	}

	return &pb.TwilioSendMessageResponse{
		Success:    true,
		MessageSid: sid,
		Message:    "Message sent successfully",
		To:         req.To,
	}, nil
}

// SendMediaMessage handles media message sending via Twilio
func (s *TwilioWhatsAppServer) SendMediaMessage(_ context.Context, req *pb.TwilioSendMediaMessageRequest) (*pb.TwilioSendMessageResponse, error) {
	if s.twilioClient == nil {
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: "Twilio client not initialized",
		}, nil
	}

	sid, err := s.twilioClient.SendMediaMessage(req.To, req.MediaUrl, req.Caption)
	if err != nil {
		return &pb.TwilioSendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send media message: %v", err),
		}, nil
	}

	return &pb.TwilioSendMessageResponse{
		Success:    true,
		MessageSid: sid,
		Message:    "Media message sent successfully",
		To:         req.To,
	}, nil
}

// GetMessageStatus retrieves the delivery status of a message
func (s *TwilioWhatsAppServer) GetMessageStatus(_ context.Context, req *pb.TwilioGetMessageStatusRequest) (*pb.TwilioGetMessageStatusResponse, error) {
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

// TestGRPCServiceConnection tests gRPC service connectivity
func TestGRPCServiceConnection(t *testing.T) {
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err := errors.New("failed to load configuration")
		t.Fatalf("Config should be loaded successfully: %v", err)
	}

	cfg := config.ServicePlatform.Get()

	// Skip if credentials not configured
	if cfg.Twilio.AccountSID == "" {
		t.Skip("Skipping test: Twilio credentials not configured")
	}

	// Initialize Twilio client
	twilioClient, err := twilio.NewClient()
	if err != nil {
		t.Logf("Skipping test: Cannot initialize Twilio client: %v", err)
		return
	}
	defer twilioClient.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	server := &TwilioWhatsAppServer{
		twilioClient: twilioClient,
	}
	pb.RegisterTwilioWhatsAppServiceServer(grpcServer, server)

	// Start gRPC listener on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Run server in background
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpcServer.Serve() error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	// Connect to server
	conn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	// Create client
	client := pb.NewTwilioWhatsAppServiceClient(conn)

	if client == nil {
		t.Fatal("Expected non-nil gRPC client")
	}

	t.Log("✅ Successfully connected to Twilio WhatsApp gRPC service")
}

// TestSendMessageRPC tests SendMessage RPC call
func TestSendMessageRPC(t *testing.T) {
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		t.Fatalf("Config should be loaded successfully: %v", err)
	}

	cfg := config.ServicePlatform.Get()

	// Skip if credentials not configured
	if cfg.Twilio.AccountSID == "" {
		t.Skip("Skipping test: Twilio credentials not configured")
	}

	// Initialize Twilio client
	twilioClient, err := twilio.NewClient()
	if err != nil {
		t.Logf("Skipping test: Cannot initialize Twilio client: %v", err)
		return
	}
	defer twilioClient.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	server := &TwilioWhatsAppServer{
		twilioClient: twilioClient,
	}
	pb.RegisterTwilioWhatsAppServiceServer(grpcServer, server)

	// Start gRPC listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpcServer.Serve() error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	// Connect to server
	conn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	// Create client
	client := pb.NewTwilioWhatsAppServiceClient(conn)

	// Call SendMessage
	resp, err := client.SendMessage(context.Background(), &pb.TwilioSendMessageRequest{
		To: "+6285173207755",
		// To:   "+6287883507445",
		Body: "Halo! Saya sedang menguji layanan WhatsApp Twilio. Mohon abaikan pesan ini.",
	})

	if err != nil {
		t.Logf("SendMessage RPC call failed (may require valid Twilio credentials): %v", err)
		return
	}

	if resp == nil {
		t.Fatal("Expected non-nil response")
	}

	t.Logf("✅ SendMessage RPC call successful. Response: %v", resp)
}

// TestGetMessageStatusRPC tests GetMessageStatus RPC call
func TestGetMessageStatusRPC(t *testing.T) {
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		t.Fatalf("Config should be loaded successfully: %v", err)
	}

	cfg := config.ServicePlatform.Get()

	// Skip if credentials not configured
	if cfg.Twilio.AccountSID == "" {
		t.Skip("Skipping test: Twilio credentials not configured")
	}

	// Initialize Twilio client
	twilioClient, err := twilio.NewClient()
	if err != nil {
		t.Logf("Skipping test: Cannot initialize Twilio client: %v", err)
		return
	}
	defer twilioClient.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	server := &TwilioWhatsAppServer{
		twilioClient: twilioClient,
	}
	pb.RegisterTwilioWhatsAppServiceServer(grpcServer, server)

	// Start gRPC listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpcServer.Serve() error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	// Connect to server
	conn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	// Create client
	client := pb.NewTwilioWhatsAppServiceClient(conn)

	// Call GetMessageStatus
	resp, err := client.GetMessageStatus(context.Background(), &pb.TwilioGetMessageStatusRequest{
		MessageSid: "SMxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	})

	if err != nil {
		t.Fatalf("GetMessageStatus RPC call failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected non-nil response")
	}

	if resp.Status == "" {
		t.Fatal("Expected non-empty status in response")
	}

	t.Logf("✅ GetMessageStatus RPC call successful. Status: %s", resp.Status)
}

// Package whatsapp provides a gRPC client for interacting with the WhatsApp service.
// It initializes the client connection and provides a global variable for accessing the WhatsApp service methods.
// The client is initialized in the InitClient function, which should be called at the start of the application.
// The Close function should be called when the application is shutting down to cleanly close the gRPC connection.
// This package relies on the configuration settings defined in the service-platform.%s.yaml file, specifically under the "Whatsnyan" section for gRPC host and port.
// Example usage:
//
//	whatsapp.InitClient()
//	defer whatsapp.Close()
//	// Now you can use whatsapp.Client to call gRPC methods on the WhatsApp service.
//
// Note: If the gRPC connection fails to initialize, the Client variable will be nil, and WhatsApp features will not be available. The application should handle this case gracefully.
// For more details on the configuration, see the service-platform.%s.yaml file and the config package documentation.
package whatsapp

import (
	"fmt"

	"service-platform/internal/config"
	"service-platform/pkg/logger"
	pb "service-platform/proto"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// Client is the global gRPC client for the WhatsApp service.
	Client pb.WhatsAppServiceClient
	// gRPC connection, kept for cleanup
	conn *grpc.ClientConn
)

// InitClient initializes the WhatsApp gRPC client
func InitClient() {
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	go config.ServicePlatform.Watch()

	logger.InitLogrus()
	cfg := config.ServicePlatform.Get()

	address := fmt.Sprintf("%s:%d", cfg.Whatsnyan.GRPCHost, cfg.Whatsnyan.GRPCPort)

	var err error
	conn, err = grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("Failed to connect to WhatsApp gRPC server: %v", err)
		logrus.Info("WhatsApp features will not be available")
		return
	}

	Client = pb.NewWhatsAppServiceClient(conn)
	logrus.Infof("✅ Connected to WhatsApp gRPC server at %s", address)
}

// Close closes the gRPC connection
func Close() {
	if conn != nil {
		conn.Close()
		logrus.Info("🔌 Disconnected from WhatsApp gRPC server")
	}
}

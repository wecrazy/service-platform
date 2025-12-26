package whatsapp

import (
	"fmt"
	"log"

	"service-platform/internal/config"
	"service-platform/internal/pkg/logger"
	pb "service-platform/proto"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	Client pb.WhatsAppServiceClient
	conn   *grpc.ClientConn
)

// InitClient initializes the WhatsApp gRPC client
func InitClient() {
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Error loading .yaml conf :%v", err)
	}
	go config.WatchConfig()
	cfg := config.GetConfig()

	// Init log
	logger.InitLogrus()

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

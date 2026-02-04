package telegram

import (
	"context"
	"fmt"
	"service-platform/internal/config"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "service-platform/proto"
)

// Global gRPC connection for Telegram service
var grpcConn *grpc.ClientConn

// InitConnection initializes gRPC connection to Telegram service
func InitConnection(host string, port int) error {
	if grpcConn != nil {
		return nil // Already connected
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	cfg := config.WebPanel.Get()
	timeout := time.Duration(cfg.TelegramService.ConnectionTimeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		logrus.Warnf("⚠️ Failed to connect to Telegram gRPC at %s: %v (will retry on send)", addr, err)
		// Don't fail completely - allow retry on first send
		return err
	}

	grpcConn = conn
	logrus.Infof("✅ Connected to Telegram gRPC service at %s", addr)
	return nil
}

// CloseConnection closes the gRPC connection
func CloseConnection() error {
	if grpcConn != nil {
		err := grpcConn.Close()
		grpcConn = nil
		logrus.Info("✅ Closed Telegram gRPC connection")
		return err
	}
	return nil
}

// GetConnection returns the current gRPC connection
func GetConnection() *grpc.ClientConn {
	return grpcConn
}

// GetClient returns a new Telegram service client
func GetClient() pb.TelegramServiceClient {
	if grpcConn == nil {
		return nil
	}
	return pb.NewTelegramServiceClient(grpcConn)
}

// EnsureConnection ensures connection is established, reconnects if needed
func EnsureConnection() error {
	if grpcConn != nil {
		return nil
	}

	cfg := config.WebPanel.Get()
	return InitConnection(cfg.TelegramService.GRPCHost, cfg.TelegramService.GRPCPort)
}

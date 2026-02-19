// Package scheduler provides a gRPC client for interacting with the scheduler service.
// This client allows the API service to communicate with the standalone scheduler service
// running in the gRPC server.
package scheduler

import (
	"fmt"

	"service-platform/internal/config"
	pb "service-platform/proto"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// Client is the global scheduler gRPC client instance.
	// It is used to communicate with the scheduler service for job management operations.
	Client pb.SchedulerServiceClient

	// conn holds the gRPC connection to the scheduler service.
	conn *grpc.ClientConn
)

// InitClient initializes the Scheduler gRPC client and establishes connection
// to the scheduler service. This should be called during application startup.
//
// The client connects to the gRPC service address specified in the configuration
// file (GRPC.Host and GRPC.Port). If the connection fails, the error is logged
// but the application continues to run without scheduler features.
//
// Example usage:
//
//	scheduler.InitClient()
//	defer scheduler.CloseClient()
func InitClient() {

	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	cfg := config.ServicePlatform.Get()

	// Connect to scheduler service (different port from main gRPC)
	host := cfg.Schedules.Host
	port := cfg.Schedules.Port

	address := fmt.Sprintf("%s:%d", host, port)

	var err error
	conn, err = grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("Failed to connect to Scheduler gRPC server: %v", err)
		logrus.Info("Scheduler features will not be available")
		return
	}

	Client = pb.NewSchedulerServiceClient(conn)
	logrus.Infof("✅ Connected to Scheduler gRPC server at %s", address)
}

// CloseClient closes the gRPC connection to the scheduler service.
// This should be called during graceful shutdown of the application.
//
// Example usage:
//
//	defer scheduler.CloseClient()
func CloseClient() {
	if conn != nil {
		conn.Close()
		logrus.Info("🔌 Disconnected from Scheduler gRPC server")
	}
}

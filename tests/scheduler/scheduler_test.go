package scheduler_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"service-platform/internal/config"
	"service-platform/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// SchedulerTestSuite provides a test suite for scheduler service gRPC testing.
// It includes setup for gRPC client connections to test scheduler endpoints.
type SchedulerTestSuite struct {
	suite.Suite
	conn   *grpc.ClientConn             // gRPC client connection
	client proto.SchedulerServiceClient // Scheduler service gRPC client
}

// SetupTest initializes the scheduler test suite by establishing a connection to the scheduler service.
// It loads configuration and creates a gRPC client connection to the scheduler service.
func (suite *SchedulerTestSuite) SetupTest() {
	// Load config
	err := config.LoadConfig()
	assert.NoError(suite.T(), err)
	cfg := config.GetConfig()

	// For integration tests, assume server is running
	host := cfg.Schedules.Host
	port := cfg.Schedules.Port
	addr := fmt.Sprintf("%s:%d", host, port)
	suite.conn, err = grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		suite.conn = nil
		suite.client = nil
		return
	}
	suite.client = proto.NewSchedulerServiceClient(suite.conn)
}

// TearDownTest cleans up the scheduler test suite by closing the client connection.
func (suite *SchedulerTestSuite) TearDownTest() {
	if suite.conn != nil {
		suite.conn.Close()
	}
}

// TestRegisterJob tests the gRPC job registration functionality with scheduler service.
// It sends a register job request and verifies the response, skipping if server is not running.
func (suite *SchedulerTestSuite) TestRegisterJob() {
	if suite.client == nil {
		suite.T().Skip("Scheduler gRPC server not running")
	}
	req := &proto.RegisterJobRequest{
		Name: "dummy-job",
	}
	resp, err := suite.client.RegisterJob(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("Scheduler gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
	assert.True(suite.T(), resp.Success)
}

// TestSchedulerTestSuite runs the scheduler test suite.
func TestSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(SchedulerTestSuite))
}

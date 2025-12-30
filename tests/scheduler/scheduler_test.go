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

type SchedulerTestSuite struct {
	suite.Suite
	conn   *grpc.ClientConn
	client proto.SchedulerServiceClient
}

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

func (suite *SchedulerTestSuite) TearDownTest() {
	if suite.conn != nil {
		suite.conn.Close()
	}
}

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

func TestSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(SchedulerTestSuite))
}

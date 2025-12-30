package grpc_test

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

type GRPCTestSuite struct {
	suite.Suite
	conn   *grpc.ClientConn
	client proto.AuthServiceClient
}

func (suite *GRPCTestSuite) SetupTest() {
	// For integration tests, start a test server
	// For now, assume server is running on localhost:50051
	var err error
	err = config.LoadConfig()
	assert.NoError(suite.T(), err)
	cfg := config.GetConfig()

	host := cfg.GRPC.Host
	port := cfg.GRPC.Port
	addr := fmt.Sprintf("%s:%d", host, port)

	suite.conn, err = grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		suite.conn = nil
		suite.client = nil
		return
	}
	suite.client = proto.NewAuthServiceClient(suite.conn)
}

func (suite *GRPCTestSuite) TearDownTest() {
	if suite.conn != nil {
		suite.conn.Close()
	}
}

func (suite *GRPCTestSuite) TestLogin() {
	req := &proto.LoginRequest{
		EmailUsername: "test@example.com",
		Password:      "password",
		CaptchaId:     "test",
		Captcha:       "test",
	}
	resp, err := suite.client.Login(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
}

func TestGRPCTestSuite(t *testing.T) {
	suite.Run(t, new(GRPCTestSuite))
}

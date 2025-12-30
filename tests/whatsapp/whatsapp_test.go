package whatsapp_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"service-platform/internal/config"
	pb "service-platform/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type WhatsAppTestSuite struct {
	suite.Suite
	conn   *grpc.ClientConn
	client pb.WhatsAppServiceClient
}

func (suite *WhatsAppTestSuite) SetupTest() {
	// Load config
	err := config.LoadConfig()
	assert.NoError(suite.T(), err)
	cfg := config.GetConfig()

	// For integration tests, assume server is running
	host := cfg.Whatsnyan.GRPCHost
	port := cfg.Whatsnyan.GRPCPort
	addr := fmt.Sprintf("%s:%d", host, port)
	suite.conn, err = grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		suite.conn = nil
		suite.client = nil
		return
	}
	suite.client = pb.NewWhatsAppServiceClient(suite.conn)
}

func (suite *WhatsAppTestSuite) TearDownTest() {
	if suite.conn != nil {
		suite.conn.Close()
	}
}

func (suite *WhatsAppTestSuite) TestSendMessage() {
	if suite.client == nil {
		suite.T().Skip("WhatsApp gRPC server not running")
	}
	req := &pb.SendMessageRequest{
		To: fmt.Sprintf("6285173207755@%s", types.DefaultUserServer),
		Content: &pb.MessageContent{
			ContentType: &pb.MessageContent_Text{
				Text: "[TEST] 👋 Hello world!",
			},
		},
	}
	resp, err := suite.client.SendMessage(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("WhatsApp gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
}

func TestWhatsAppTestSuite(t *testing.T) {
	suite.Run(t, new(WhatsAppTestSuite))
}

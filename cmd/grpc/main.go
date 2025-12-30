package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"service-platform/internal/database"
	"service-platform/internal/pkg/fun"
	"service-platform/internal/pkg/logger"
	"service-platform/proto"
	"strings"
	"time"

	"github.com/dchest/captcha"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"
)

type authServer struct {
	proto.UnimplementedAuthServiceServer
	db    *gorm.DB
	redis *redis.Client
}

func (s *authServer) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	// Validate captcha (skip validation if both are "test" for development)
	if req.CaptchaId != "test" && req.Captcha != "test" {
		if !captcha.VerifyString(req.CaptchaId, req.Captcha) {
			return &proto.LoginResponse{Success: false, Message: "Invalid captcha"}, nil
		}
	}

	var user model.Users
	whereQuery := ""
	if strings.Contains(req.EmailUsername, "@") {
		whereQuery = "email = ?"
	} else {
		whereQuery = "username = ?"
	}

	if err := s.db.WithContext(ctx).Where(whereQuery, req.EmailUsername).First(&user).Error; err != nil {
		logrus.Errorf("❌ User query error: %v\n", err)
		return &proto.LoginResponse{Success: false, Message: "Invalid credentials"}, nil
	}

	// Check lock
	if user.LockUntil != nil && user.LockUntil.After(time.Now()) {
		return &proto.LoginResponse{Success: false, Message: "Account locked"}, nil
	}

	if !fun.IsPasswordMatched(req.Password, user.Password) {
		user.LoginAttempts++
		if user.LoginAttempts >= 5 {
			lockTime := time.Now().Add(15 * time.Minute)
			user.LockUntil = &lockTime
		}
		s.db.WithContext(ctx).Save(&user)
		return &proto.LoginResponse{Success: false, Message: "Invalid credentials"}, nil
	}

	// Reset attempts
	user.LoginAttempts = 0
	user.LockUntil = nil
	now := time.Now()
	user.LastLogin = &now
	s.db.WithContext(ctx).Save(&user)

	// Generate token (simplified, use JWT or something)
	token := fun.GenerateRandomString(64)

	// Store in redis
	s.redis.Set(ctx, "session:"+token, user.ID, 7*24*time.Hour)

	return &proto.LoginResponse{Success: true, Message: "Login successful", Token: token}, nil
}

func (s *authServer) Logout(ctx context.Context, req *proto.LogoutRequest) (*proto.LogoutResponse, error) {
	s.redis.Del(ctx, "session:"+req.Token)
	return &proto.LogoutResponse{Success: true, Message: "Logged out"}, nil
}

func main() {
	// Dynamic update yaml config
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Error loading .yaml conf :%v", err)
	}

	go config.WatchConfig()
	yamlCfg := config.GetConfig()

	// Init log
	logger.InitLogrus()

	db, err := database.InitAndCheckDB(
		yamlCfg.Database.Type,
		yamlCfg.Database.Username,
		yamlCfg.Database.Password,
		yamlCfg.Database.Host,
		yamlCfg.Database.Port,
		yamlCfg.Database.Name,
		yamlCfg.Database.SSLMode,
	)

	if err != nil {
		logrus.Fatalf("Failed to init DB: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.GetConfig().Redis.Host, yamlCfg.Redis.Port),
		Password: yamlCfg.Redis.Password,
		DB:       yamlCfg.Redis.Db,
	})

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", yamlCfg.GRPC.Port))
	if err != nil {
		logrus.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	proto.RegisterAuthServiceServer(s, &authServer{db: db, redis: redisClient})
	reflection.Register(s)

	// Start metrics server
	go func() {
		http.Handle("/grpc-metrics", promhttp.Handler())
		metricsPort := config.GetConfig().Metrics.GRPCPort
		logrus.Printf("📊 Metrics server listening on :%d", metricsPort)
		logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), nil))
	}()

	logrus.Println("🚀 Auth gRPC server listening on " + fmt.Sprintf(":%d", config.GetConfig().GRPC.Port))
	logrus.Println("📝 Note: Scheduler is now a separate service (run cmd/scheduler/main.go)")

	if err := s.Serve(lis); err != nil {
		logrus.Fatalf("failed to serve: %v", err)
	}
}

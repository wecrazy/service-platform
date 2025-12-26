package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"service-platform/internal/config"
	"service-platform/internal/database"
	"service-platform/internal/pkg/logger"
	"service-platform/internal/scheduler"
	"service-platform/proto"

	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"
)

type schedulerServer struct {
	proto.UnimplementedSchedulerServiceServer
	db        *gorm.DB
	scheduler *gocron.Scheduler
	cfg       *config.YamlConfig
}

func (s *schedulerServer) RegisterJob(ctx context.Context, req *proto.RegisterJobRequest) (*proto.RegisterJobResponse, error) {
	log.Printf("Job registration requested: %s", req.Name)
	return &proto.RegisterJobResponse{
		Success: true,
		Message: fmt.Sprintf("Job %s registered successfully", req.Name),
	}, nil
}

func (s *schedulerServer) UnregisterJob(ctx context.Context, req *proto.UnregisterJobRequest) (*proto.UnregisterJobResponse, error) {
	log.Printf("Job unregistration requested: %s", req.Name)
	scheduler.UnregisterJob(req.Name)
	return &proto.UnregisterJobResponse{
		Success: true,
		Message: fmt.Sprintf("Job %s unregistered successfully", req.Name),
	}, nil
}

func (s *schedulerServer) ListJobs(ctx context.Context, req *proto.ListJobsRequest) (*proto.ListJobsResponse, error) {
	jobs := scheduler.GetAllJobs()
	jobInfos := make([]*proto.JobInfo, 0, len(jobs))

	// Create a map of job names to their config for quick lookup
	jobConfigMap := make(map[string]config.Scheduler)
	for _, sched := range s.cfg.Schedules.List {
		jobConfigMap[sched.Name] = sched
	}

	for name := range jobs {
		description := "Scheduled job"
		schedule := "As configured"

		// Get actual description and schedule from config
		if jobConfig, exists := jobConfigMap[name]; exists {
			if jobConfig.Description != "" {
				description = jobConfig.Description
			}

			// Format schedule string
			if jobConfig.Every != "" {
				schedule = fmt.Sprintf("Every %s", jobConfig.Every)
			} else if len(jobConfig.At) > 0 {
				schedule = fmt.Sprintf("Daily at %v", jobConfig.At)
			} else if jobConfig.Weekly != "" {
				schedule = fmt.Sprintf("Weekly: %s", jobConfig.Weekly)
			} else if jobConfig.Monthly != "" {
				schedule = fmt.Sprintf("Monthly: %s", jobConfig.Monthly)
			} else if jobConfig.Yearly != "" {
				schedule = fmt.Sprintf("Yearly: %s", jobConfig.Yearly)
			}
		}

		jobInfos = append(jobInfos, &proto.JobInfo{
			Name:        name,
			Description: description,
			Schedule:    schedule,
		})
	}
	return &proto.ListJobsResponse{Jobs: jobInfos}, nil
}

func (s *schedulerServer) TriggerJob(ctx context.Context, req *proto.TriggerJobRequest) (*proto.TriggerJobResponse, error) {
	err := scheduler.TriggerJob(req.Name)
	if err != nil {
		return &proto.TriggerJobResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	return &proto.TriggerJobResponse{
		Success: true,
		Message: fmt.Sprintf("Job %s triggered successfully", req.Name),
	}, nil
}

func (s *schedulerServer) GetJobStatus(ctx context.Context, req *proto.GetJobStatusRequest) (*proto.GetJobStatusResponse, error) {
	exists := scheduler.JobExists(req.Name)
	if !exists {
		return &proto.GetJobStatusResponse{Exists: false}, nil
	}

	// Get job details from config
	description := "Scheduled job"
	schedule := "As configured"

	for _, sched := range s.cfg.Schedules.List {
		if sched.Name == req.Name {
			if sched.Description != "" {
				description = sched.Description
			}

			// Format schedule string
			if sched.Every != "" {
				schedule = fmt.Sprintf("Every %s", sched.Every)
			} else if len(sched.At) > 0 {
				schedule = fmt.Sprintf("Daily at %v", sched.At)
			} else if sched.Weekly != "" {
				schedule = fmt.Sprintf("Weekly: %s", sched.Weekly)
			} else if sched.Monthly != "" {
				schedule = fmt.Sprintf("Monthly: %s", sched.Monthly)
			} else if sched.Yearly != "" {
				schedule = fmt.Sprintf("Yearly: %s", sched.Yearly)
			}
			break
		}
	}

	return &proto.GetJobStatusResponse{
		Exists: true,
		JobInfo: &proto.JobInfo{
			Name:        req.Name,
			Description: description,
			Schedule:    schedule,
		},
	}, nil
}

func (s *schedulerServer) ReloadScheduler(ctx context.Context, req *proto.ReloadSchedulerRequest) (*proto.ReloadSchedulerResponse, error) {
	if s.scheduler != nil {
		s.scheduler.Stop()
	}
	if err := config.LoadConfig(); err != nil {
		return &proto.ReloadSchedulerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to reload config: %v", err),
		}, nil
	}

	yamlCfg := config.GetConfig()
	s.cfg = &yamlCfg
	scheduler.ReloadTimezone()
	s.scheduler = scheduler.StartScheduler(s.db, s.cfg)
	jobCount := len(s.cfg.Schedules.List)
	return &proto.ReloadSchedulerResponse{
		Success:    true,
		Message:    "Scheduler reloaded successfully",
		JobsLoaded: int32(jobCount),
	}, nil
}

func main() {
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Error loading .yaml conf: %v", err)
	}
	go config.WatchConfig()
	cfg := config.GetConfig()

	logger.InitLogrus()

	db, err := database.InitAndCheckDB(
		cfg.Database.Type,
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)
	if err != nil {
		logrus.Fatalf("Failed to init DB: %v", err)
	}
	fmt.Println("✅ Database connected for scheduler service")

	// Log timezone configuration
	fmt.Printf("🌏 Scheduler timezone configured: %s\n", cfg.Schedules.Timezone)

	schedulerInstance := scheduler.StartScheduler(db, &cfg)
	fmt.Println("📅 Scheduler started with configured jobs")

	port := cfg.Schedules.Port
	if port == 0 {
		logrus.Fatal("Scheduler gRPC port not configured in config file")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logrus.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterSchedulerServiceServer(grpcServer, &schedulerServer{
		db:        db,
		scheduler: schedulerInstance,
		cfg:       &cfg,
	})
	reflection.Register(grpcServer)

	fmt.Printf("🚀 Scheduler gRPC service listening on :%d\n", port)
	fmt.Println("📋 All scheduled jobs are now running independently")

	if err := grpcServer.Serve(lis); err != nil {
		logrus.Fatalf("Failed to serve: %v", err)
	}
}

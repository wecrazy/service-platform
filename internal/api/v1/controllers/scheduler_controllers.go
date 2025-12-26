package controllers

import (
	"context"
	"net/http"
	"time"

	"service-platform/internal/api/v1/dto"
	schedulerclient "service-platform/internal/scheduler"
	pb "service-platform/proto"

	"github.com/gin-gonic/gin"
)

// ListScheduledJobs godoc
// @Summary      List all scheduled jobs
// @Description  Get a list of all registered scheduled jobs with their details
// @Tags         Scheduler
// @Produce      json
// @Success      200  {object}  dto.ListJobsResponse
// @Failure      503  {object}  dto.SchedulerErrorResponse "Service Unavailable"
// @Router       /api/v1/scheduler/jobs [get]
// @Security     ApiKeyAuth
func ListScheduledJobs() gin.HandlerFunc {
	return func(c *gin.Context) {
		if schedulerclient.Client == nil {
			c.JSON(http.StatusServiceUnavailable, dto.SchedulerErrorResponse{
				Error: "Scheduler service not available",
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := schedulerclient.Client.ListJobs(ctx, &pb.ListJobsRequest{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.SchedulerErrorResponse{
				Error:   "Failed to list jobs",
				Details: err.Error(),
			})
			return
		}

		// Convert protobuf response to DTO
		jobs := make([]dto.JobInfoResponse, 0, len(resp.Jobs))
		for _, job := range resp.Jobs {
			jobs = append(jobs, dto.JobInfoResponse{
				Name:        job.Name,
				Description: job.Description,
				Schedule:    []string{job.Schedule},
				NextRun:     job.NextRun,
				LastRun:     job.LastRun,
				IsRunning:   job.IsRunning,
			})
		}

		c.JSON(http.StatusOK, dto.ListJobsResponse{
			Success: true,
			Total:   len(jobs),
			Jobs:    jobs,
		})
	}
}

// TriggerScheduledJob godoc
// @Summary      Manually trigger a scheduled job
// @Description  Trigger a specific job to run immediately
// @Tags         Scheduler
// @Accept       json
// @Produce      json
// @Param        request body dto.TriggerJobRequest true "Job name"
// @Success      200  {object}  dto.TriggerJobResponse
// @Failure      400  {object}  dto.SchedulerErrorResponse "Bad Request"
// @Failure      503  {object}  dto.SchedulerErrorResponse "Service Unavailable"
// @Router       /api/v1/scheduler/jobs/trigger [post]
// @Security     ApiKeyAuth
func TriggerScheduledJob() gin.HandlerFunc {
	return func(c *gin.Context) {
		if schedulerclient.Client == nil {
			c.JSON(http.StatusServiceUnavailable, dto.SchedulerErrorResponse{
				Error: "Scheduler service not available",
			})
			return
		}

		var req dto.TriggerJobRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.SchedulerErrorResponse{
				Error:   "Invalid request",
				Details: err.Error(),
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := schedulerclient.Client.TriggerJob(ctx, &pb.TriggerJobRequest{
			Name: req.Name,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.SchedulerErrorResponse{
				Error:   "Failed to trigger job",
				Details: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, dto.TriggerJobResponse{
			Success: resp.Success,
			Message: resp.Message,
		})
	}
}

// GetJobStatus godoc
// @Summary      Get job status
// @Description  Get the status and details of a specific scheduled job
// @Tags         Scheduler
// @Produce      json
// @Param        name   path     string  true  "Job Name"
// @Success      200  {object}  dto.GetJobStatusResponse
// @Failure      404  {object}  dto.JobNotFoundResponse "Not Found"
// @Failure      503  {object}  dto.SchedulerErrorResponse "Service Unavailable"
// @Router       /api/v1/scheduler/jobs/{name} [get]
// @Security     ApiKeyAuth
func GetJobStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		if schedulerclient.Client == nil {
			c.JSON(http.StatusServiceUnavailable, dto.SchedulerErrorResponse{
				Error: "Scheduler service not available",
			})
			return
		}

		jobName := c.Param("name")
		if jobName == "" {
			c.JSON(http.StatusBadRequest, dto.SchedulerErrorResponse{
				Error: "Job name is required",
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := schedulerclient.Client.GetJobStatus(ctx, &pb.GetJobStatusRequest{
			Name: jobName,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.SchedulerErrorResponse{
				Error:   "Failed to get job status",
				Details: err.Error(),
			})
			return
		}

		if !resp.Exists {
			c.JSON(http.StatusNotFound, dto.JobNotFoundResponse{
				Error: "Job not found",
				Name:  jobName,
			})
			return
		}

		// Convert protobuf response to DTO
		jobInfo := dto.JobInfoResponse{
			Name:        resp.JobInfo.Name,
			Description: resp.JobInfo.Description,
			Schedule:    []string{resp.JobInfo.Schedule},
			NextRun:     resp.JobInfo.NextRun,
			LastRun:     resp.JobInfo.LastRun,
			IsRunning:   resp.JobInfo.IsRunning,
		}

		c.JSON(http.StatusOK, dto.GetJobStatusResponse{
			Success: true,
			Exists:  resp.Exists,
			Job:     jobInfo,
		})
	}
}

// ReloadScheduler godoc
// @Summary      Reload scheduler configuration
// @Description  Reload the scheduler with updated configuration from YAML
// @Tags         Scheduler
// @Produce      json
// @Success      200  {object}  dto.ReloadSchedulerResponse
// @Failure      503  {object}  dto.SchedulerErrorResponse "Service Unavailable"
// @Router       /api/v1/scheduler/reload [post]
// @Security     ApiKeyAuth
func ReloadScheduler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if schedulerclient.Client == nil {
			c.JSON(http.StatusServiceUnavailable, dto.SchedulerErrorResponse{
				Error: "Scheduler service not available",
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := schedulerclient.Client.ReloadScheduler(ctx, &pb.ReloadSchedulerRequest{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.SchedulerErrorResponse{
				Error:   "Failed to reload scheduler",
				Details: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, dto.ReloadSchedulerResponse{
			Success:    resp.Success,
			Message:    resp.Message,
			JobsLoaded: resp.JobsLoaded,
		})
	}
}

// RegisterScheduledJob godoc
// @Summary      Register a new scheduled job
// @Description  Register a new job to be executed by the scheduler
// @Tags         Scheduler
// @Accept       json
// @Produce      json
// @Param        request body dto.RegisterJobRequest true "Job details"
// @Success      200  {object}  dto.RegisterJobResponse
// @Failure      400  {object}  dto.SchedulerErrorResponse "Bad Request"
// @Failure      503  {object}  dto.SchedulerErrorResponse "Service Unavailable"
// @Router       /api/v1/scheduler/jobs [post]
// @Security     ApiKeyAuth
func RegisterScheduledJob() gin.HandlerFunc {
	return func(c *gin.Context) {
		if schedulerclient.Client == nil {
			c.JSON(http.StatusServiceUnavailable, dto.SchedulerErrorResponse{
				Error: "Scheduler service not available",
			})
			return
		}

		var req dto.RegisterJobRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.SchedulerErrorResponse{
				Error:   "Invalid request",
				Details: err.Error(),
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := schedulerclient.Client.RegisterJob(ctx, &pb.RegisterJobRequest{
			Name:        req.Name,
			Description: req.Description,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.SchedulerErrorResponse{
				Error:   "Failed to register job",
				Details: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, dto.RegisterJobResponse{
			Success: resp.Success,
			Message: resp.Message,
		})
	}
}

// UnregisterScheduledJob godoc
// @Summary      Unregister a scheduled job
// @Description  Remove a job from the scheduler
// @Tags         Scheduler
// @Produce      json
// @Param        name   path     string  true  "Job Name"
// @Success      200  {object}  dto.UnregisterJobResponse
// @Failure      503  {object}  dto.SchedulerErrorResponse "Service Unavailable"
// @Router       /api/v1/scheduler/jobs/{name} [delete]
// @Security     ApiKeyAuth
func UnregisterScheduledJob() gin.HandlerFunc {
	return func(c *gin.Context) {
		if schedulerclient.Client == nil {
			c.JSON(http.StatusServiceUnavailable, dto.SchedulerErrorResponse{
				Error: "Scheduler service not available",
			})
			return
		}

		jobName := c.Param("name")
		if jobName == "" {
			c.JSON(http.StatusBadRequest, dto.SchedulerErrorResponse{
				Error: "Job name is required",
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := schedulerclient.Client.UnregisterJob(ctx, &pb.UnregisterJobRequest{
			Name: jobName,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.SchedulerErrorResponse{
				Error:   "Failed to unregister job",
				Details: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, dto.UnregisterJobResponse{
			Success: resp.Success,
			Message: resp.Message,
		})
	}
}

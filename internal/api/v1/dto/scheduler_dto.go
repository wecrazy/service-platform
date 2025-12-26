package dto

// Scheduler Job Request DTOs

// TriggerJobRequest represents the request to trigger a scheduled job manually
type TriggerJobRequest struct {
	Name string `json:"name" binding:"required" example:"purge-old-log-backup-files"`
}

// RegisterJobRequest represents the request to register a new scheduled job
type RegisterJobRequest struct {
	Name        string   `json:"name" binding:"required" example:"backup-database"`
	Description string   `json:"description" example:"Backup database daily at 2 AM"`
	Schedule    []string `json:"schedule" binding:"required" example:"02:00"`
	Enabled     bool     `json:"enabled" example:"true"`
}

// UpdateJobRequest represents the request to update an existing job
type UpdateJobRequest struct {
	Description *string  `json:"description,omitempty" example:"Updated description"`
	Schedule    []string `json:"schedule,omitempty" example:"03:00"`
	Enabled     *bool    `json:"enabled,omitempty" example:"true"`
}

// Scheduler Job Response DTOs

// JobInfoResponse represents a single scheduled job's information
type JobInfoResponse struct {
	Name        string   `json:"name" example:"purge-old-log-backup-files"`
	Description string   `json:"description" example:"Purge old log backup files (.gz) older than configured duration"`
	Schedule    []string `json:"schedule" example:"01:00"`
	NextRun     string   `json:"next_run,omitempty" example:"2025-12-24T01:00:00+07:00"`
	LastRun     string   `json:"last_run,omitempty" example:"2025-12-23T01:00:00+07:00"`
	IsRunning   bool     `json:"is_running" example:"false"`
}

// ListJobsResponse represents the response for listing all jobs
type ListJobsResponse struct {
	Success bool              `json:"success" example:"true"`
	Total   int               `json:"total" example:"4"`
	Jobs    []JobInfoResponse `json:"jobs"`
}

// GetJobStatusResponse represents the response for getting a specific job's status
type GetJobStatusResponse struct {
	Success bool            `json:"success" example:"true"`
	Exists  bool            `json:"exists" example:"true"`
	Job     JobInfoResponse `json:"job"`
}

// TriggerJobResponse represents the response after triggering a job
type TriggerJobResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"Job triggered successfully"`
}

// RegisterJobResponse represents the response after registering a job
type RegisterJobResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"Job registered successfully"`
}

// UnregisterJobResponse represents the response after unregistering a job
type UnregisterJobResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"Job unregistered successfully"`
}

// ReloadSchedulerResponse represents the response after reloading scheduler config
type ReloadSchedulerResponse struct {
	Success    bool   `json:"success" example:"true"`
	Message    string `json:"message" example:"Scheduler configuration reloaded successfully"`
	JobsLoaded int32  `json:"jobs_loaded" example:"4"`
}

// Error Response DTOs

// SchedulerErrorResponse represents a standard error response
type SchedulerErrorResponse struct {
	Error   string `json:"error" example:"Service Unavailable"`
	Details string `json:"details,omitempty" example:"gRPC connection failed"`
}

// JobNotFoundResponse represents a job not found error
type JobNotFoundResponse struct {
	Error string `json:"error" example:"Job not found"`
	Name  string `json:"name" example:"non-existent-job"`
}

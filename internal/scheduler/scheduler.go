package scheduler

import (
	"fmt"
	"service-platform/internal/config"
	"service-platform/internal/pkg/fun"
	"strconv"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	timezoneLoc *time.Location
	dbInstance  *gorm.DB
	yamlCfg     *config.TypeServicePlatform
)

// JobMetadata stores execution metadata for each job
type JobMetadata struct {
	Name        string
	Description string
	Schedule    string
	NextRun     time.Time
	LastRun     time.Time
	IsRunning   bool
	GocronJob   *gocron.Job // Reference to the actual gocron job
}

// jobMap stores all registered job functions indexed by their name.
// Jobs must be registered using RegisterJob() before they can be executed.
var jobMap = map[string]func(){
	"purge-old-log-backup-files": func() {
		olderThan := yamlCfg.Default.PurgeOldBackupLogFilesOlderThan
		fun.PurgeOldLogBackupFiles(olderThan)
	},
	"purge-old-db-log": func() {
		olderThan := yamlCfg.Database.PurgeOlderThan
		dbType := yamlCfg.Database.Type
		if dbInstance != nil {
			fun.PurgeOldDatabaseLogs(dbInstance, olderThan, dbType)
		} else {
			logrus.Warn("Database instance not available for purge-old-db-log job")
		}
	},
	"purge-old-whatsnyan-messages": func() {
		olderThan := yamlCfg.Whatsnyan.PurgeMessageOlderThan
		if dbInstance != nil {
			fun.PurgeOldWhatsnyanMessages(dbInstance, olderThan)
		} else {
			logrus.Warn("Database instance not available for purge-old-whatsnyan-messages job")
		}
	},
	"remove-old-files-in-needs-dir": func() {
		olderThan := yamlCfg.Default.RemoveOldNeedsDirOlderThan
		fun.RemoveOldFilesInNeedsDir(olderThan)
	},
}

// jobMetadata stores execution metadata for all jobs
var jobMetadata = make(map[string]*JobMetadata)

// loadTimezone loads the timezone from config
func loadTimezone() {
	var err error
	timezoneLoc, err = time.LoadLocation(config.ServicePlatform.Get().Schedules.Timezone)
	if err != nil {
		logrus.Fatalf("Failed to load timezone %s: %v", config.ServicePlatform.Get().Schedules.Timezone, err)
	}
	logrus.Infof("Scheduler timezone loaded: %s", timezoneLoc)
}

// ReloadTimezone reloads the timezone (for config changes)
func ReloadTimezone() {
	loadTimezone()
}

func StartScheduler(db *gorm.DB, cfg *config.TypeServicePlatform) *gocron.Scheduler {
	loadTimezone()
	dbInstance = db // Store DB instance for jobs that need it
	yamlCfg = cfg   // Store config for jobs that need it
	scheduler := gocron.NewScheduler(timezoneLoc)

	for _, sched := range cfg.Schedules.List {
		name := sched.Name
		description := sched.Description

		// Initialize job metadata
		if jobMetadata[name] == nil {
			jobMetadata[name] = &JobMetadata{
				Name:        name,
				Description: description,
				IsRunning:   false,
			}
		}

		if sched.Every != "" {
			fmt.Printf("⏱ Trying to run scheduler: %s every %v\n", name, sched.Every)
			dur, err := time.ParseDuration(sched.Every)
			if err != nil {
				logrus.Warnf("Invalid duration for %s: %v", name, err)
				continue
			}
			job, err := scheduler.Every(dur).Do(func() {
				runJobWithTracking(name)
			})
			if err != nil {
				logrus.Warnf("Failed to schedule job %s: %v", name, err)
			} else {
				jobMetadata[name].GocronJob = job
				jobMetadata[name].Schedule = fmt.Sprintf("every %s", dur)
				jobMetadata[name].NextRun = job.NextRun()
				logrus.Infof("Scheduled job %s to run every %s", name, dur)
			}

		} else if len(sched.At) > 0 {
			// sched.At is a []string of times, e.g. ["11:02", "11:03"]
			for _, atTime := range sched.At {
				fmt.Printf("⏰ Trying to run scheduler: %s daily at %v\n", name, atTime)
				if !isValidTime(atTime) {
					logrus.Warnf("Invalid time format for %s: %s", name, atTime)
					continue
				}
				job, err := scheduler.Every(1).Day().At(atTime).Do(func() {
					runJobWithTracking(name)
				})
				if err != nil {
					logrus.Warnf("Failed to schedule job %s: %v", name, err)
				} else {
					jobMetadata[name].GocronJob = job
					jobMetadata[name].Schedule = fmt.Sprintf("daily at %s", atTime)
					jobMetadata[name].NextRun = job.NextRun()
					logrus.Infof("Scheduled job %s to run daily at %s", name, atTime)
				}
			}
		} else if sched.Weekly != "" {
			fmt.Printf("🕰 Trying to run scheduler: %s weekly at %v\n", name, sched.Weekly)
			parts := strings.Split(sched.Weekly, "@")
			if len(parts) != 2 || !isValidTime(parts[1]) {
				logrus.Warnf("Invalid weekly format for %s: %s", name, sched.Weekly)
				continue
			}
			weekdayStr := strings.ToLower(parts[0])
			timePart := parts[1]

			weekdayMap := map[string]time.Weekday{
				"sunday":    time.Sunday,
				"monday":    time.Monday,
				"tuesday":   time.Tuesday,
				"wednesday": time.Wednesday,
				"thursday":  time.Thursday,
				"friday":    time.Friday,
				"saturday":  time.Saturday,
				"sun":       time.Sunday,
				"mon":       time.Monday,
				"tue":       time.Tuesday,
				"wed":       time.Wednesday,
				"thu":       time.Thursday,
				"fri":       time.Friday,
				"sat":       time.Saturday,
			}
			weekday, ok := weekdayMap[weekdayStr]
			if !ok {
				logrus.Warnf("Invalid weekday for %s: %s", name, weekdayStr)
				continue
			}

			job, err := scheduler.Every(1).Week().Weekday(weekday).At(timePart).Do(func() {
				runJobWithTracking(name)
			})
			if err != nil {
				logrus.Warnf("Failed to schedule weekly job %s: %v", name, err)
			} else {
				jobMetadata[name].GocronJob = job
				jobMetadata[name].Schedule = fmt.Sprintf("weekly %s at %s", weekday, timePart)
				jobMetadata[name].NextRun = job.NextRun()
				logrus.Infof("Scheduled job %s to run weekly on %s at %s", name, weekday, timePart)
			}

		} else if sched.Monthly != "" {
			fmt.Printf("⏳ Trying to run scheduler: %s monthly at %v\n", name, sched.Monthly)
			parts := strings.Split(sched.Monthly, "@")
			if len(parts) != 2 || !isValidTime(parts[1]) {
				logrus.Warnf("Invalid monthly format for %s: %s", name, sched.Monthly)
				continue
			}
			dayPart := parts[0]
			timePart := parts[1]

			if dayPart == "last" {
				// Run daily at given time, but check if today is last day of month in timezone cfg time
				job, err := scheduler.Every(1).Day().At(timePart).Do(func() {
					now := time.Now().In(timezoneLoc)
					tomorrow := now.AddDate(0, 0, 1)
					if tomorrow.Month() != now.Month() {
						runJobWithTracking(name)
					}
				})
				if err != nil {
					logrus.Warnf("Failed to schedule last-day monthly job %s: %v", name, err)
				} else {
					jobMetadata[name].GocronJob = job
					jobMetadata[name].Schedule = fmt.Sprintf("monthly last day at %s", timePart)
					jobMetadata[name].NextRun = job.NextRun()
					logrus.Infof("Scheduled job %s to run monthly on last day at %s", name, timePart)
				}
			} else {
				dayInt, err := strconv.Atoi(dayPart)
				if err != nil || dayInt < 1 || dayInt > 31 {
					logrus.Warnf("Invalid day for monthly job %s: %s", name, dayPart)
					continue
				}
				// Run daily at timePart, but only on day == dayInt in timezone cfg time
				job, err := scheduler.Every(1).Day().At(timePart).Do(func() {
					if time.Now().In(timezoneLoc).Day() == dayInt {
						runJobWithTracking(name)
					}
				})
				if err != nil {
					logrus.Warnf("Failed to schedule monthly job %s: %v", name, err)
				} else {
					jobMetadata[name].GocronJob = job
					jobMetadata[name].Schedule = fmt.Sprintf("monthly day %d at %s", dayInt, timePart)
					jobMetadata[name].NextRun = job.NextRun()
					logrus.Infof("Scheduled job %s to run monthly on day %d at %s", name, dayInt, timePart)
				}
			}
		} else if sched.Yearly != "" {
			fmt.Printf("📅 Trying to run scheduler: %s yearly at %v\n", name, sched.Yearly)
			parts := strings.Split(sched.Yearly, "@")
			if len(parts) != 2 || !isValidTime(parts[1]) {
				logrus.Warnf("Invalid yearly format for %s: %s", name, sched.Yearly)
				continue
			}
			dayPart := parts[0] // e.g. "01" for January 1st
			timePart := parts[1]

			dayInt, err := strconv.Atoi(dayPart)
			if err != nil || dayInt < 1 || dayInt > 31 {
				logrus.Warnf("Invalid day for yearly job %s: %s", name, dayPart)
				continue
			}

			job, err := scheduler.Every(1).Day().At(timePart).Do(func() {
				now := time.Now().In(timezoneLoc)
				if now.Month() == time.January && now.Day() == dayInt {
					runJobWithTracking(name)
				}
			})
			if err != nil {
				logrus.Warnf("Failed to schedule yearly job %s: %v", name, err)
			} else {
				jobMetadata[name].GocronJob = job
				jobMetadata[name].Schedule = fmt.Sprintf("yearly Jan %d at %s", dayInt, timePart)
				jobMetadata[name].NextRun = job.NextRun()
				logrus.Infof("Scheduled job %s to run yearly on Jan %d at %s", name, dayInt, timePart)
			}
		}
	}

	scheduler.StartAsync()
	logrus.Infof("✅ All schedulers started (%s timezone).", timezoneLoc)
	return scheduler
}

func runJob(name string) {
	if job, ok := jobMap[name]; ok {
		logrus.Infof("Scheduled running job: %s @ %v (%s timezone)", name, time.Now().In(timezoneLoc), timezoneLoc)
		job()
	} else {
		logrus.Warnf("Unknown job: %s", name)
	}
}

// runJobWithTracking executes a job and tracks its execution state
func runJobWithTracking(name string) {
	if job, ok := jobMap[name]; ok {
		// Mark job as running
		if meta, exists := jobMetadata[name]; exists {
			meta.IsRunning = true
		}

		logrus.Infof("Scheduled running job: %s @ %v (%s timezone)", name, time.Now().In(timezoneLoc), timezoneLoc)

		// Execute job
		job()

		// Update metadata after execution
		if meta, exists := jobMetadata[name]; exists {
			meta.IsRunning = false
			meta.LastRun = time.Now().In(timezoneLoc)
			if meta.GocronJob != nil {
				meta.NextRun = meta.GocronJob.NextRun()
			}
		}
	} else {
		logrus.Warnf("Unknown job: %s", name)
	}
}

func isValidTime(t string) bool {
	_, err := time.Parse("15:04", t)
	return err == nil
}

// RegisterJob adds a job function to the scheduler's job map.
// This makes the job available for scheduling and execution.
//
// Parameters:
//
// name: The unique identifier for the job (must match config.yaml)
// job: The function to execute when the job runs
//
// Example:
//
//	scheduler.RegisterJob("cleanup-logs", func() {
//	   log.Println("Cleaning up old logs...")
//	   // cleanup logic here
//	})
//
// After registering, add to config.yaml:
//
// schedules:
//
//	list:
//	  - name: "cleanup-logs"
//	    every: "24h"
func RegisterJob(name string, job func()) {
	jobMap[name] = job
	logrus.Infof("Job registered: %s", name)
}

// UnregisterJob removes a job from the scheduler's job map.
// This prevents the job from being executed in future scheduled runs.
//
// Parameters:
//
// name: The unique identifier of the job to remove
//
// Example:
//
// scheduler.UnregisterJob("cleanup-logs")
//
// Note: This only removes the job function from the map. The job schedule
// in config.yaml will remain but the job won't execute.
func UnregisterJob(name string) {
	delete(jobMap, name)
	logrus.Infof("Job unregistered: %s", name)
}

// GetAllJobs returns a map of all currently registered jobs.
// The map keys are job names and values are the job functions.
//
// Returns:
//
// map[string]func(): All registered job functions indexed by name
//
// Example:
//
// jobs := scheduler.GetAllJobs()
//
//	for name := range jobs {
//	   fmt.Printf("Job: %s\n", name)
//	}
func GetAllJobs() map[string]func() {
	return jobMap
}

// TriggerJob manually executes a job immediately in a separate goroutine.
// This is useful for testing jobs or running them on-demand outside their
// normal schedule.
//
// Parameters:
//
// name: The unique identifier of the job to trigger
//
// Returns:
//
// error: Returns an error if the job is not found, nil on success
//
// Example:
//
// err := scheduler.TriggerJob("cleanup-logs")
//
//	if err != nil {
//	   log.Printf("Failed to trigger job: %v", err)
//	}
//
// Note: The job runs asynchronously in a goroutine, so this function
// returns immediately without waiting for the job to complete.
func TriggerJob(name string) error {
	if job, ok := jobMap[name]; ok {
		logrus.Infof("Manually triggering job: %s", name)
		go func() {
			// Mark as running
			if meta, exists := jobMetadata[name]; exists {
				meta.IsRunning = true
			}

			// Execute
			job()

			// Update metadata
			if meta, exists := jobMetadata[name]; exists {
				meta.IsRunning = false
				meta.LastRun = time.Now().In(timezoneLoc)
			}
		}()
		return nil
	}
	return fmt.Errorf("job not found: %s", name)
}

// JobExists checks if a job with the given name is registered.
//
// Parameters:
//
// name: The unique identifier of the job to check
//
// Returns:
//
// bool: true if the job exists, false otherwise
//
// Example:
//
//	if scheduler.JobExists("cleanup-logs") {
//	   fmt.Println("Job is registered")
//	} else {
//
//	   fmt.Println("Job not found")
//	}
func JobExists(name string) bool {
	_, ok := jobMap[name]
	return ok
}

// GetJobMetadata returns the execution metadata for a specific job.
//
// Parameters:
//
// name: The unique identifier of the job
//
// Returns:
//
// *JobMetadata: Job metadata if found, nil otherwise
//
// Example:
//
// meta := scheduler.GetJobMetadata("cleanup-logs")
//
//	if meta != nil {
//	   fmt.Printf("Next run: %v\n", meta.NextRun)
//	   fmt.Printf("Last run: %v\n", meta.LastRun)
//	   fmt.Printf("Is running: %v\n", meta.IsRunning)
//	}
func GetJobMetadata(name string) *JobMetadata {
	return jobMetadata[name]
}

// GetAllJobsMetadata returns execution metadata for all registered jobs.
//
// Returns:
//
// map[string]*JobMetadata: Map of job names to their metadata
//
// Example:
//
// allMeta := scheduler.GetAllJobsMetadata()
//
//	for name, meta := range allMeta {
//	   fmt.Printf("Job: %s, Next run: %v\n", name, meta.NextRun)
//	}
func GetAllJobsMetadata() map[string]*JobMetadata {
	return jobMetadata
}

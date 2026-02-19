package fun

import (
	"os"
	"path/filepath"
	"service-platform/internal/config"
	"time"

	"github.com/sirupsen/logrus"
)

// RemoveOldFilesInNeedsDir removes old date-based directories from configured needs directories
func RemoveOldFilesInNeedsDir(olderThan string) {
	cfg := config.ServicePlatform.Get()
	folderNeeds := cfg.FolderFileNeeds

	if len(folderNeeds) == 0 {
		logrus.Warn("No folders configured in folder_file_needs")
		return
	}

	// Calculate cutoff time
	cutoffTime, err := calculateCutoffTime(olderThan)
	if err != nil {
		logrus.Errorf("Failed to parse duration '%s': %v", olderThan, err)
		return
	}

	logrus.Infof("Removing directories older than %s (cutoff: %s) from needs directories", olderThan, cutoffTime.Format(config.DATE_YYYY_MM_DD))

	totalFolders := 0
	totalDirsDeleted := 0
	totalFilesDeleted := 0
	var totalSizeFreed int64 = 0

	for _, folder := range folderNeeds {
		logrus.Infof("Processing folder: %s", folder)

		// Try to find valid directory
		selectedDir, err := FindValidDirectory([]string{
			"web/file/" + folder,
			"../web/file/" + folder,
			"../../web/file/" + folder,
			"../../../web/file/" + folder,
		})

		if err != nil {
			logrus.Errorf("Failed to find valid directory for folder %s: %v", folder, err)
			continue
		}

		logrus.Infof("Found directory: %s", selectedDir)

		// Check if directory has date-formatted subdirectories
		hasDateDirs := checkForDateDirectories(selectedDir)

		if hasDateDirs {
			// Remove old date-based directories
			logrus.Infof("Found date-formatted directories in %s, removing old directories", folder)
			dirsDeleted, filesDeleted, sizeFreed, err := removeOldDateDirectories(selectedDir, cutoffTime)
			if err != nil {
				logrus.Errorf("Failed to remove old directories in %s: %v", selectedDir, err)
				continue
			}

			if dirsDeleted > 0 {
				logrus.Infof("📁 %s - Deleted: %d directories (%d files, %s)",
					folder, dirsDeleted, filesDeleted, formatBytes(sizeFreed))
				totalDirsDeleted += dirsDeleted
				totalFilesDeleted += filesDeleted
				totalSizeFreed += sizeFreed
			} else {
				logrus.Infof("📁 %s - No old directories to remove", folder)
			}
		} else {
			// Remove old files directly
			logrus.Infof("No date-formatted directories in %s, removing old files", folder)
			filesDeleted, sizeFreed, err := removeOldFiles(selectedDir, cutoffTime)
			if err != nil {
				logrus.Errorf("Failed to remove old files in %s: %v", selectedDir, err)
				continue
			}

			if filesDeleted > 0 {
				logrus.Infof("📁 %s - Deleted: %d files (%s)",
					folder, filesDeleted, formatBytes(sizeFreed))
				totalFilesDeleted += filesDeleted
				totalSizeFreed += sizeFreed
			} else {
				logrus.Infof("📁 %s - No old files to remove", folder)
			}
		}

		totalFolders++
	}

	if totalDirsDeleted > 0 {
		logrus.Infof("✅ Cleanup completed: %d folders processed, %d directories deleted (%d files, %s freed)",
			totalFolders, totalDirsDeleted, totalFilesDeleted, formatBytes(totalSizeFreed))
	} else {
		logrus.Infof("✅ Cleanup completed: %d folders processed, no old directories found to delete", totalFolders)
	}
}

// removeOldDateDirectories removes subdirectories with date format older than cutoff time
// Returns: (dirsDeleted, filesDeleted, sizeFreed, error)
func removeOldDateDirectories(dir string, cutoffTime time.Time) (int, int, int64, error) {
	var dirsDeleted int
	var filesDeleted int
	var sizeFreed int64

	// Read immediate subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, 0, err
	}

	for _, entry := range entries {
		// Only process directories
		if !entry.IsDir() {
			continue
		}

		// Try to parse directory name as date
		dirDate, err := time.Parse(config.DATE_YYYY_MM_DD, entry.Name())
		if err != nil {
			// Not a date-formatted directory, skip
			logrus.Debugf("Skipping non-date directory: %s", entry.Name())
			continue
		}

		// Check if directory date is older than cutoff
		if dirDate.Before(cutoffTime) {
			dirPath := filepath.Join(dir, entry.Name())

			// Count files and calculate size before deletion
			fileCount, dirSize := calculateDirSize(dirPath)

			// Remove the entire directory
			err := os.RemoveAll(dirPath)
			if err != nil {
				logrus.Errorf("Failed to delete directory %s: %v", dirPath, err)
			} else {
				logrus.Infof("Deleted directory: %s (date: %s, files: %d, size: %s)",
					entry.Name(), dirDate.Format(config.DATE_YYYY_MM_DD), fileCount, formatBytes(dirSize))
				dirsDeleted++
				filesDeleted += fileCount
				sizeFreed += dirSize
			}
		} else {
			logrus.Debugf("Keeping directory: %s (date: %s, newer than cutoff)",
				entry.Name(), dirDate.Format(config.DATE_YYYY_MM_DD))
		}
	}

	return dirsDeleted, filesDeleted, sizeFreed, nil
}

// calculateDirSize calculates the total size and file count of a directory
func calculateDirSize(dir string) (int, int64) {
	var fileCount int
	var totalSize int64

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		fileCount++
		totalSize += info.Size()
		return nil
	})

	return fileCount, totalSize
}

// checkForDateDirectories checks if a directory contains any date-formatted subdirectories
func checkForDateDirectories(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Try to parse directory name as date
		_, err := time.Parse(config.DATE_YYYY_MM_DD, entry.Name())
		if err == nil {
			// Found at least one date-formatted directory
			return true
		}
	}

	return false
}

// removeOldFiles removes files older than cutoff time based on modification time
// Returns: (filesDeleted, sizeFreed, error)
func removeOldFiles(dir string, cutoffTime time.Time) (int, int64, error) {
	var filesDeleted int
	var sizeFreed int64

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, err
	}

	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		// Get file info
		filePath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			logrus.Errorf("Failed to get file info for %s: %v", filePath, err)
			continue
		}

		// Check if file is older than cutoff time
		if info.ModTime().Before(cutoffTime) {
			fileSize := info.Size()
			err := os.Remove(filePath)
			if err != nil {
				logrus.Errorf("Failed to delete %s: %v", filePath, err)
			} else {
				logrus.Debugf("Deleted file: %s (size: %s, modified: %s)",
					entry.Name(), formatBytes(fileSize), info.ModTime().Format(config.DATE_YYYY_MM_DD_HH_MM_SS))
				filesDeleted++
				sizeFreed += fileSize
			}
		}
	}

	return filesDeleted, sizeFreed, nil
}

package controllers

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// MediaHandler handles WhatsApp media file storage and retrieval
type MediaHandler struct {
	BasePath string
}

// NewMediaHandler creates a new media handler
func NewMediaHandler(basePath string) *MediaHandler {
	return &MediaHandler{
		BasePath: basePath,
	}
}

// MediaFileInfo contains information about a stored media file
type MediaFileInfo struct {
	LocalPath     string `json:"local_path"`
	RelativePath  string `json:"relative_path"`
	FileName      string `json:"filename"`
	FileSize      int64  `json:"file_size"`
	MimeType      string `json:"mime_type"`
	ThumbnailPath string `json:"thumbnail_path,omitempty"`
}

// GetUserMediaPath returns the media directory path for a specific user
func (mh *MediaHandler) GetUserMediaPath(userID uint) string {
	return filepath.Join(mh.BasePath, "whatsapp_media", fmt.Sprintf("user_%d", userID))
}

// GetMediaTypePath returns the path for a specific media type within user directory
func (mh *MediaHandler) GetMediaTypePath(userID uint, mediaType string) string {
	userPath := mh.GetUserMediaPath(userID)
	switch strings.ToLower(mediaType) {
	case "image":
		return filepath.Join(userPath, "images")
	case "video":
		return filepath.Join(userPath, "videos")
	case "audio", "voice":
		return filepath.Join(userPath, "audio")
	case "document":
		return filepath.Join(userPath, "documents")
	case "sticker":
		return filepath.Join(userPath, "stickers")
	default:
		return filepath.Join(userPath, "others")
	}
}

// EnsureDirectoryExists creates directory if it doesn't exist
func (mh *MediaHandler) EnsureDirectoryExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dirPath, err)
		}
		logrus.Infof("Created media directory: %s", dirPath)
	}
	return nil
}

// GenerateFileName generates a unique filename for media files
func (mh *MediaHandler) GenerateFileName(originalName, messageID string, timestamp time.Time) string {
	// Create hash from messageID and timestamp for uniqueness
	hash := md5.Sum([]byte(messageID + timestamp.String()))
	hashStr := fmt.Sprintf("%x", hash)[:12]

	// Get file extension
	fmt.Println("Original name:", originalName)

	ext := filepath.Ext(originalName)
	fmt.Println("Detected extension:", ext)

	if ext == "" {
		// Try to detect from mime type or use fallback
		fmt.Println("No extension detected, using fallback")
		ext = ".bin" // fallback extension
	}

	// Format: YYYYMMDD_HHMMSS_hash_originalname
	timeStr := timestamp.Format("20060102_150405")

	// Clean original name (remove special characters)
	cleanName := strings.ReplaceAll(originalName, " ", "_")
	cleanName = strings.ReplaceAll(cleanName, ext, "")
	if len(cleanName) > 50 {
		cleanName = cleanName[:50]
	}

	return fmt.Sprintf("%s_%s_%s%s", timeStr, hashStr, cleanName, ext)
}

// GenerateFileNameWithMimeType generates a filename using MIME type for extension
func (mh *MediaHandler) GenerateFileNameWithMimeType(messageID, mediaType, mimeType string, timestamp time.Time) string {
	// Create hash from messageID and timestamp for uniqueness
	hash := md5.Sum([]byte(messageID + timestamp.String()))
	hashStr := fmt.Sprintf("%x", hash)[:12]

	// Get extension from MIME type
	ext := mh.getExtensionFromMimeType(mimeType)

	// Format: YYYYMMDD_HHMMSS_hash_mediatype
	timeStr := timestamp.Format("20060102_150405")

	return fmt.Sprintf("%s_%s_%s_%s%s", timeStr, hashStr, mediaType, messageID[:8], ext)
}

// getExtensionFromMimeType returns file extension based on MIME type
func (mh *MediaHandler) getExtensionFromMimeType(mimeType string) string {
	switch strings.ToLower(mimeType) {
	// Images
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tiff"

	// Videos
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/ogg":
		return ".ogv"
	case "video/avi":
		return ".avi"
	case "video/mov", "video/quicktime":
		return ".mov"
	case "video/mkv":
		return ".mkv"

	// Audio
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/ogg":
		return ".ogg"
	case "audio/wav":
		return ".wav"
	case "audio/aac":
		return ".aac"
	case "audio/flac":
		return ".flac"
	case "audio/webm":
		return ".weba"

	// Documents
	case "application/pdf":
		return ".pdf"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.ms-powerpoint":
		return ".ppt"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	case "text/plain":
		return ".txt"
	case "application/zip":
		return ".zip"
	case "application/x-rar-compressed":
		return ".rar"

	// Stickers (already handled above in images)
	// case "image/webp": return ".webp" - already handled in images section

	default:
		return ".bin"
	}
}

// SaveMediaFile saves media content to local storage
func (mh *MediaHandler) SaveMediaFile(userID uint, messageID string, mediaType string, content []byte, originalFileName string, mimeType string, timestamp time.Time) (*MediaFileInfo, error) {
	// Get media type directory
	typeDir := mh.GetMediaTypePath(userID, mediaType)

	// Ensure directory exists
	if err := mh.EnsureDirectoryExists(typeDir); err != nil {
		return nil, err
	}

	// Generate unique filename
	fileName := mh.GenerateFileName(originalFileName, messageID, timestamp)
	filePath := filepath.Join(typeDir, fileName)

	// Write file
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %v", filePath, err)
	}
	defer file.Close()

	_, err = file.Write(content)
	if err != nil {
		return nil, fmt.Errorf("failed to write file %s: %v", filePath, err)
	}

	// Get relative path for URL generation
	relativePath := strings.TrimPrefix(filePath, mh.BasePath)
	relativePath = strings.TrimPrefix(relativePath, "/")

	logrus.Infof("Saved media file: %s (size: %d bytes)", filePath, len(content))

	return &MediaFileInfo{
		LocalPath:    filePath,
		RelativePath: relativePath,
		FileName:     fileName,
		FileSize:     int64(len(content)),
		MimeType:     mimeType,
	}, nil
}

// SaveMediaFileWithMimeType saves media content using MIME type for proper extension
func (mh *MediaHandler) SaveMediaFileWithMimeType(userID uint, messageID string, mediaType string, content []byte, mimeType string, timestamp time.Time) (*MediaFileInfo, error) {
	// Get media type directory
	typeDir := mh.GetMediaTypePath(userID, mediaType)

	// Ensure directory exists
	if err := mh.EnsureDirectoryExists(typeDir); err != nil {
		return nil, err
	}

	// Generate unique filename using MIME type
	fileName := mh.GenerateFileNameWithMimeType(messageID, mediaType, mimeType, timestamp)
	filePath := filepath.Join(typeDir, fileName)

	// Write file
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %v", filePath, err)
	}
	defer file.Close()

	_, err = file.Write(content)
	if err != nil {
		return nil, fmt.Errorf("failed to write file %s: %v", filePath, err)
	}

	// Get relative path for URL generation
	// Normalize base path by removing ./ prefix
	normalizedBasePath := strings.TrimPrefix(mh.BasePath, "./")
	relativePath := strings.TrimPrefix(filePath, normalizedBasePath)
	relativePath = strings.TrimPrefix(relativePath, "/")

	logrus.Infof("Saved media file: %s (size: %d bytes)", filePath, len(content))

	return &MediaFileInfo{
		LocalPath:    filePath,
		RelativePath: relativePath,
		FileName:     fileName,
		FileSize:     int64(len(content)),
		MimeType:     mimeType,
	}, nil
}

// SaveMediaFromReader saves media content from an io.Reader
func (mh *MediaHandler) SaveMediaFromReader(userID uint, messageID string, mediaType string, reader io.Reader, originalFileName string, mimeType string, timestamp time.Time) (*MediaFileInfo, error) {
	// Get media type directory
	typeDir := mh.GetMediaTypePath(userID, mediaType)

	// Ensure directory exists
	if err := mh.EnsureDirectoryExists(typeDir); err != nil {
		return nil, err
	}

	// Generate unique filename
	fileName := mh.GenerateFileName(originalFileName, messageID, timestamp)
	filePath := filepath.Join(typeDir, fileName)

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %v", filePath, err)
	}
	defer file.Close()

	// Copy from reader to file
	written, err := io.Copy(file, reader)
	if err != nil {
		// Clean up partial file on error
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write file %s: %v", filePath, err)
	}

	// Get relative path for URL generation
	relativePath := strings.TrimPrefix(filePath, mh.BasePath)
	relativePath = strings.TrimPrefix(relativePath, "/")

	logrus.Infof("Saved media file: %s (size: %d bytes)", filePath, written)

	return &MediaFileInfo{
		LocalPath:    filePath,
		RelativePath: relativePath,
		FileName:     fileName,
		FileSize:     written,
		MimeType:     mimeType,
	}, nil
}

// GetMediaFileURL returns the URL for accessing a media file
func (mh *MediaHandler) GetMediaFileURL(relativePath string) string {
	// The relativePath should already be clean (just whatsapp_media/user_1/...)
	// but let's ensure we remove any remaining prefixes
	cleanPath := strings.TrimPrefix(relativePath, "/whatsapp_media")
	cleanPath = strings.TrimPrefix(cleanPath, "whatsapp_media")
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	// This will be served by the web server as static files
	return "/media/" + cleanPath
}

// DeleteMediaFile removes a media file from storage
func (mh *MediaHandler) DeleteMediaFile(filePath string) error {
	if filePath == "" {
		return nil
	}

	// Ensure file is within our media directory (security check)
	userMediaBase := filepath.Join(mh.BasePath, "whatsapp_media")
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	absUserMediaBase, err := filepath.Abs(userMediaBase)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(absPath, absUserMediaBase) {
		return fmt.Errorf("file path outside media directory: %s", filePath)
	}

	err = os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file %s: %v", filePath, err)
	}

	logrus.Infof("Deleted media file: %s", filePath)
	return nil
}

// CleanupUserMedia removes old media files for a user (optional cleanup function)
func (mh *MediaHandler) CleanupUserMedia(userID uint, olderThan time.Duration) error {
	userPath := mh.GetUserMediaPath(userID)
	cutoffTime := time.Now().Add(-olderThan)

	err := filepath.Walk(userPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Delete files older than cutoff
		if info.ModTime().Before(cutoffTime) {
			logrus.Infof("Cleaning up old media file: %s", path)
			return os.Remove(path)
		}

		return nil
	})

	return err
}

// GetMediaStats returns statistics about stored media for a user
func (mh *MediaHandler) GetMediaStats(userID uint) map[string]interface{} {
	userPath := mh.GetUserMediaPath(userID)
	stats := map[string]interface{}{
		"total_files": 0,
		"total_size":  int64(0),
		"by_type":     make(map[string]map[string]interface{}),
	}

	mediaTypes := []string{"images", "videos", "audio", "documents", "stickers", "others"}

	for _, mediaType := range mediaTypes {
		typePath := filepath.Join(userPath, mediaType)
		typeStats := map[string]interface{}{
			"count": 0,
			"size":  int64(0),
		}

		filepath.Walk(typePath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			typeStats["count"] = typeStats["count"].(int) + 1
			typeStats["size"] = typeStats["size"].(int64) + info.Size()
			stats["total_files"] = stats["total_files"].(int) + 1
			stats["total_size"] = stats["total_size"].(int64) + info.Size()

			return nil
		})

		stats["by_type"].(map[string]map[string]interface{})[mediaType] = typeStats
	}

	return stats
}

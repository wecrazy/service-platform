package fun

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	htgotts "github.com/hegedustibor/htgo-tts"
	"github.com/sirupsen/logrus"
)

// CreateTTSWithEspeak creates TTS using local espeak command as fallback
func CreateTTSWithEspeak(text, filename, outputDir string) (string, error) {
	// Check if espeak is available
	if err := exec.Command("which", "espeak").Run(); err != nil {
		return "", fmt.Errorf("espeak command not found, please install espeak: %v", err)
	}

	outputPath := filepath.Join(outputDir, filename+".wav")

	// Escape single quotes in text to prevent command injection
	safeText := strings.ReplaceAll(text, "'", "'\"'\"'")

	// Use espeak to generate speech
	// -s: speed, -p: pitch, -v: voice (id+f2 = Indonesian female-like)
	cmd := exec.Command(
		"espeak",
		"-s", "150", // speed
		"-p", "70", // pitch (higher for more feminine)
		"-v", "id+f2", // Indonesian with female variant
		"-w", outputPath,
		safeText,
	)

	logrus.Infof("🔊 Running espeak command: %v", cmd.Args)

	// Execute the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("espeak command failed: %v, output: %s", err, string(output))
	}

	// Check if file was created
	if _, err := os.Stat(outputPath); err != nil {
		return "", fmt.Errorf("espeak did not create output file: %v", err)
	}

	// Convert WAV to MP3 if ffmpeg is available
	mp3Path := filepath.Join(outputDir, filename+".mp3")

	if err := exec.Command("which", "ffmpeg").Run(); err == nil {
		convertCmd := exec.Command("ffmpeg", "-i", outputPath, "-codec:a", "mp3", "-y", mp3Path)
		convertCmd.Stderr = nil // Suppress ffmpeg output

		if convertCmd.Run() == nil {
			os.Remove(outputPath)
			logrus.Infof("🔊 Successfully converted to MP3: %s", mp3Path)
			return mp3Path, nil
		} else {
			logrus.Warnf("🔊 ffmpeg conversion failed, keeping WAV file")
		}
	} else {
		logrus.Infof("🔊 ffmpeg not available, returning WAV file")
	}

	return outputPath, nil
}

// IsValidAudioFile checks if a file exists and appears to be a valid audio file
func IsValidAudioFile(filePath string) bool {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	// Check file size - should be at least 3KB for a valid MP3
	if fileInfo.Size() < 3000 {
		logrus.Warnf("⚠️ Audio file too small (%d bytes): %s", fileInfo.Size(), filePath)
		return false
	}

	// Check if it's actually an audio file by reading first few bytes
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	header := make([]byte, 10)
	n, err := file.Read(header)
	if err != nil || n < 3 {
		return false
	}

	// Check for MP3 header (ID3 or MPEG frame sync)
	if bytes.HasPrefix(header, []byte("ID3")) ||
		(header[0] == 0xFF && (header[1]&0xE0) == 0xE0) {
		return true
	}

	// Check for HTML error response (common when API fails)
	if bytes.Contains(header, []byte("<!DOCTYPE")) || bytes.Contains(header, []byte("<html")) {
		logrus.Warnf("⚠️ File contains HTML instead of audio: %s", filePath)
		return false
	}

	logrus.Warnf("⚠️ File doesn't appear to be valid MP3: %s", filePath)
	return false
}

// MergeAudioPartsWithDelay merges multiple audio files with delays between them
func MergeAudioPartsWithDelay(partFiles []string, filename, outputDir string) (string, error) {
	if len(partFiles) == 0 {
		return "", errors.New("no audio parts to merge")
	}

	if len(partFiles) == 1 {
		// Just copy the single file
		finalFile := filepath.Join(outputDir, filename+".mp3")
		return copyFile(partFiles[0], finalFile)
	}

	// Check if ffmpeg is available
	if err := exec.Command("which", "ffmpeg").Run(); err != nil {
		return "", fmt.Errorf("ffmpeg command not found, please install ffmpeg: %v", err)
	}

	finalMP3 := filepath.Join(outputDir, filename+".mp3")

	// Build ffmpeg command for multiple parts
	var inputArgs []string
	var filterParts []string

	// Add all input files
	for _, partFile := range partFiles {
		inputArgs = append(inputArgs, "-i", partFile)
	}

	// Build filter complex for delays and concatenation
	if len(partFiles) == 2 {
		// Simple case: two parts with delay
		delay := 250 // 0.25 seconds delay for each part
		filterParts = append(filterParts, fmt.Sprintf("[1:a]adelay=%d|%d[a1]", delay, delay))
		filterParts = append(filterParts, "[0:a][a1]concat=n=2:v=0:a=1[out]")
	} else {
		// Multiple parts: add progressive delays
		for i := 1; i < len(partFiles); i++ {
			delay := 250 * i // 0.25 seconds delay for each subsequent part
			filterParts = append(filterParts, fmt.Sprintf("[%d:a]adelay=%d|%d[a%d]", i, delay, delay, i))
		}

		// Build concat inputs
		var concatInputs []string
		concatInputs = append(concatInputs, "[0:a]")
		for i := 1; i < len(partFiles); i++ {
			concatInputs = append(concatInputs, fmt.Sprintf("[a%d]", i))
		}

		filterParts = append(filterParts, fmt.Sprintf("%sconcat=n=%d:v=0:a=1[out]",
			strings.Join(concatInputs, ""), len(partFiles)))
	}

	// Combine all arguments
	args := append(inputArgs, "-filter_complex", strings.Join(filterParts, ";"), "-map", "[out]", "-y", finalMP3)

	// logrus.Infof("🔧 Merging %d audio parts with ffmpeg", len(partFiles))
	cmd := exec.Command("ffmpeg", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logrus.Errorf("❌ ffmpeg merge failed: %v, stderr: %s", err, stderr.String())
		return "", fmt.Errorf("failed to merge audio parts: %v", err)
	}

	// Verify the final file exists and has reasonable size
	// if fileInfo, err := os.Stat(finalMP3); err != nil {
	if _, err := os.Stat(finalMP3); err != nil {
		return "", fmt.Errorf("merged MP3 file not created: %v", err)
	} else {
		// logrus.Infof("✅ Successfully merged %d parts: %s (%d bytes)", len(partFiles), finalMP3, fileInfo.Size())
	}

	return finalMP3, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) (string, error) {
	source, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return "", err
	}

	return dst, nil
}

func CreateRobustTTS(speech htgotts.Speech, audioDir string, textParts []string, fileName string) (string, error) {
	if len(textParts) == 0 {
		return "", errors.New("no text parts provided for TTS")
	}

	// Create a temporary directory for audio parts to avoid permission issues
	tempDir, err := os.MkdirTemp(audioDir, "tts_parts_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir for tts parts: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up the temp directory

	// Tell htgotts to use our new temp directory
	speech.Folder = tempDir

	// logrus.Debugf("🔊 Creating robust TTS for %d parts: %s", len(textParts), fileName)

	// STEP 1: Try to make parts text into audio using htgotts speechfile
	var partFiles []string
	var allPartsValid = true

	// logrus.Debugf("🎵 Step 1: Trying to create %d parts using htgotts speechfile", len(textParts))

	for i, text := range textParts {
		partFilename := fmt.Sprintf("%s_part%d", fileName, i+1)

		// logrus.Debugf("🎵 Creating part %d/%d using htgotts: %s", i+1, len(textParts), text[:min(50, len(text))])

		// Try htgotts speechfile
		fileTTS, err := speech.CreateSpeechFile(text, partFilename)
		if err != nil {
			logrus.Warnf("❌ htgotts speechfile failed for part %d: %v", i+1, err)
			allPartsValid = false
			break
		}

		// Check if the file is valid
		if !IsValidAudioFile(fileTTS) {
			logrus.Warnf("❌ htgotts speechfile created invalid audio for part %d", i+1)
			// os.Remove(fileTTS) // Not needed, defer os.RemoveAll will clean it
			allPartsValid = false
			break
		}

		// logrus.Debugf("✅ htgotts speechfile success for part %d: %s", i+1, fileTTS)
		partFiles = append(partFiles, fileTTS)
	}

	// If all parts are valid, merge them with ffmpeg
	if allPartsValid && len(partFiles) == len(textParts) {
		// logrus.Debugf("✅ All %d parts created successfully with htgotts, merging with ffmpeg", len(partFiles))
		finalFile, err := MergeAudioPartsWithDelay(partFiles, fileName, audioDir)
		if err != nil {
			logrus.Errorf("❌ FFmpeg merge failed: %v", err)
			// Don't return error yet, fall back to step 2
		} else {
			// logrus.Debugf("✅ Step 1 completed successfully: %s", finalFile)
			return finalFile, nil
		}
	}

	// STEP 2: If step 1 failed, merge text parts into single text and use espeak
	logrus.Infof("🔄 Step 1 failed, trying Step 2: merge text parts and use espeak")

	// Merge all text parts into single text with delays (represented as pauses)
	var mergedText strings.Builder
	for i, text := range textParts {
		mergedText.WriteString(text)
		if i < len(textParts)-1 {
			mergedText.WriteString("... ... ... ") // Add pause between parts
		}
	}

	finalText := mergedText.String()
	logrus.Infof("🔄 Creating single audio file with espeak for merged text (length: %d)", len(finalText))

	// Use espeak for the merged text
	return CreateTTSWithEspeak(finalText, fileName, audioDir)
}

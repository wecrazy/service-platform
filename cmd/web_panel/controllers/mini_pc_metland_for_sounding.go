package controllers

import (
	"fmt"
	"net/http"
	"service-platform/cmd/web_panel/internal/gormdb"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func GetSoundStatusOfSPTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		db := gormdb.Databases.Web
		var gotSPs []sptechnicianmodel.TechnicianGotSP
		// Only return technicians with at least one unplayed sound and valid path
		if err := db.Where(
			"(is_got_sp1 = ? AND sp1_sound_played = ? AND sp1_sound_tts_path != '') OR "+
				"(is_got_sp2 = ? AND sp2_sound_played = ? AND sp2_sound_tts_path != '') OR "+
				"(is_got_sp3 = ? AND sp3_sound_played = ? AND sp3_sound_tts_path != '')",
			true, false, true, false, true, false,
		).
			Find(&gotSPs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("failed to fetch SP sound statuses: %w", err).Error()})
			return
		}
		var statuses []map[string]interface{}
		for _, gotSP := range gotSPs {
			statuses = append(statuses, map[string]interface{}{
				"sp1_sound_played":   gotSP.SP1SoundPlayed,
				"sp2_sound_played":   gotSP.SP2SoundPlayed,
				"sp3_sound_played":   gotSP.SP3SoundPlayed,
				"sp1_sound_tts_path": strings.ReplaceAll(gotSP.SP1SoundTTSPath, "web/file/sounding_sp_technician", "/sp_sounding"),
				"sp2_sound_tts_path": strings.ReplaceAll(gotSP.SP2SoundTTSPath, "web/file/sounding_sp_technician", "/sp_sounding"),
				"sp3_sound_tts_path": strings.ReplaceAll(gotSP.SP3SoundTTSPath, "web/file/sounding_sp_technician", "/sp_sounding"),
				"is_got_sp1":         gotSP.IsGotSP1,
				"is_got_sp2":         gotSP.IsGotSP2,
				"is_got_sp3":         gotSP.IsGotSP3,
				"technician":         gotSP.Technician,
			})
		}
		c.JSON(http.StatusOK, statuses)
	}
}

func UpdateSoundStatusOfSPTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SoundType  string `json:"sound_type"`
			Technician string `json:"technician"`
			PlayedAt   string `json:"played_at"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		db := gormdb.Databases.Web
		var gotSP sptechnicianmodel.TechnicianGotSP
		query := db.Order("id desc")
		if req.Technician != "" {
			query = query.Where("technician = ?", req.Technician)
		}
		if err := query.First(&gotSP).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "TechnicianGotSP not found"})
			return
		}
		now := time.Now()
		switch req.SoundType {
		case "sp1":
			gotSP.SP1SoundPlayed = true
			gotSP.SP1SoundPlayedAt = &now
		case "sp2":
			gotSP.SP2SoundPlayed = true
			gotSP.SP2SoundPlayedAt = &now
		case "sp3":
			gotSP.SP3SoundPlayed = true
			gotSP.SP3SoundPlayedAt = &now
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sound_type"})
			return
		}
		if err := db.Save(&gotSP).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update sound played"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated", "sound_type": req.SoundType, "technician": gotSP.Technician})
	}
}

func GetSoundStatusOfSPSPL() gin.HandlerFunc {
	return func(c *gin.Context) {
		db := gormdb.Databases.Web
		var gotSPs []sptechnicianmodel.SPLGotSP
		// Only return spl with at least one unplayed sound and valid path
		if err := db.Where(
			"(is_got_sp1 = ? AND sp1_sound_played = ? AND sp1_sound_tts_path != '') OR "+
				"(is_got_sp2 = ? AND sp2_sound_played = ? AND sp2_sound_tts_path != '') OR "+
				"(is_got_sp3 = ? AND sp3_sound_played = ? AND sp3_sound_tts_path != '')",
			true, false, true, false, true, false,
		).
			Find(&gotSPs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("failed to query spl: %w", err).Error()})
			return
		}
		var statuses []map[string]interface{}
		for _, gotSP := range gotSPs {
			statuses = append(statuses, map[string]interface{}{
				"sp1_sound_played":   gotSP.SP1SoundPlayed,
				"sp2_sound_played":   gotSP.SP2SoundPlayed,
				"sp3_sound_played":   gotSP.SP3SoundPlayed,
				"sp1_sound_tts_path": strings.ReplaceAll(gotSP.SP1SoundTTSPath, "web/file/sounding_sp_spl", "/sp_sounding_spl"),
				"sp2_sound_tts_path": strings.ReplaceAll(gotSP.SP2SoundTTSPath, "web/file/sounding_sp_spl", "/sp_sounding_spl"),
				"sp3_sound_tts_path": strings.ReplaceAll(gotSP.SP3SoundTTSPath, "web/file/sounding_sp_spl", "/sp_sounding_spl"),
				"is_got_sp1":         gotSP.IsGotSP1,
				"is_got_sp2":         gotSP.IsGotSP2,
				"is_got_sp3":         gotSP.IsGotSP3,
				"spl":                gotSP.SPL,
			})
		}
		c.JSON(http.StatusOK, statuses)
	}
}

func UpdateSoundStatusOfSPSPL() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SoundType string `json:"sound_type"`
			SPL       string `json:"spl"`
			PlayedAt  string `json:"played_at"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		db := gormdb.Databases.Web
		var gotSP sptechnicianmodel.SPLGotSP
		query := db.Order("id desc")
		if req.SPL != "" {
			query = query.Where("spl = ?", req.SPL)
		}
		if err := query.First(&gotSP).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Errorf("spl not found: %w", err).Error()})
			return
		}
		now := time.Now()
		switch req.SoundType {
		case "sp1":
			gotSP.SP1SoundPlayed = true
			gotSP.SP1SoundPlayedAt = &now
		case "sp2":
			gotSP.SP2SoundPlayed = true
			gotSP.SP2SoundPlayedAt = &now
		case "sp3":
			gotSP.SP3SoundPlayed = true
			gotSP.SP3SoundPlayedAt = &now
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sound_type"})
			return
		}
		if err := db.Save(&gotSP).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("failed to update sound played: %w", err).Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated", "sound_type": req.SoundType, "spl": gotSP.SPL})
	}
}

func GetSoundStatusOfSPSAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		db := gormdb.Databases.Web
		var gotSPs []sptechnicianmodel.SACGotSP
		// Only return sac with at least one unplayed sound and valid path
		if err := db.Where(
			"(is_got_sp1 = ? AND sp1_sound_played = ? AND sp1_sound_tts_path != '') OR "+
				"(is_got_sp2 = ? AND sp2_sound_played = ? AND sp2_sound_tts_path != '') OR "+
				"(is_got_sp3 = ? AND sp3_sound_played = ? AND sp3_sound_tts_path != '')",
			true, false, true, false, true, false,
		).
			Find(&gotSPs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("failed to query sac: %w", err).Error()})
			return
		}
		var statuses []map[string]interface{}
		for _, gotSP := range gotSPs {
			statuses = append(statuses, map[string]interface{}{
				"sp1_sound_played":   gotSP.SP1SoundPlayed,
				"sp2_sound_played":   gotSP.SP2SoundPlayed,
				"sp3_sound_played":   gotSP.SP3SoundPlayed,
				"sp1_sound_tts_path": strings.ReplaceAll(gotSP.SP1SoundTTSPath, "web/file/sounding_sp_sac", "/sp_sounding_sac"),
				"sp2_sound_tts_path": strings.ReplaceAll(gotSP.SP2SoundTTSPath, "web/file/sounding_sp_sac", "/sp_sounding_sac"),
				"sp3_sound_tts_path": strings.ReplaceAll(gotSP.SP3SoundTTSPath, "web/file/sounding_sp_sac", "/sp_sounding_sac"),
				"is_got_sp1":         gotSP.IsGotSP1,
				"is_got_sp2":         gotSP.IsGotSP2,
				"is_got_sp3":         gotSP.IsGotSP3,
				"sac":                gotSP.SAC,
			})
		}
		c.JSON(http.StatusOK, statuses)
	}
}

func UpdateSoundStatusOfSPSAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SoundType string `json:"sound_type"`
			SAC       string `json:"sac"`
			PlayedAt  string `json:"played_at"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		db := gormdb.Databases.Web
		var gotSP sptechnicianmodel.SACGotSP
		query := db.Order("id desc")
		if req.SAC != "" {
			query = query.Where("sac = ?", req.SAC)
		}
		if err := query.First(&gotSP).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Errorf("sac not found: %w", err).Error()})
			return
		}
		now := time.Now()
		switch req.SoundType {
		case "sp1":
			gotSP.SP1SoundPlayed = true
			gotSP.SP1SoundPlayedAt = &now
		case "sp2":
			gotSP.SP2SoundPlayed = true
			gotSP.SP2SoundPlayedAt = &now
		case "sp3":
			gotSP.SP3SoundPlayed = true
			gotSP.SP3SoundPlayedAt = &now
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sound_type"})
			return
		}
		if err := db.Save(&gotSP).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("failed to update sound played: %w", err).Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated", "sound_type": req.SoundType, "sac": gotSP.SAC})
	}
}

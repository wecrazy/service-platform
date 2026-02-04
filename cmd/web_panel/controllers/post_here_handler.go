package controllers

import (
	"bytes"
	"io"
	"net/http"
	"service-platform/internal/config"

	"github.com/gin-gonic/gin"
)

func PostHere() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Baca body request dari client
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		// Reset body request agar bisa digunakan lagi
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// Ambil path dari request
		targetURL := config.WebPanel.Get().API.KukuhFilestoreURL + "/here" + c.Param("path")

		// Buat request baru ke target server
		req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(bodyBytes))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
			return
		}

		// Copy headers dari request asli
		for key, values := range c.Request.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		// Kirim request ke server target
		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to forward request" + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Baca response dari server target
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
			return
		}

		// Forward response ke client asli
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}
}

package fun

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"service-platform/internal/config"
	"time"
)

type LibreTranslateRequest struct {
	Q            interface{} `json:"q"`
	Source       string      `json:"source"`
	Target       string      `json:"target"`
	Format       string      `json:"format,omitempty"`
	Alternatives int         `json:"alternatives,omitempty"`
	ApiKey       string      `json:"api_key,omitempty"`
}

type LibreTranslateResponse struct {
	TranslatedText   interface{} `json:"translatedText"`
	Alternatives     interface{} `json:"alternatives,omitempty"`
	DetectedLanguage interface{} `json:"detectedLanguage,omitempty"`
	Error            string      `json:"error,omitempty"`
}

func TranslateTextUseLibreTranslate(input, sourceLang, targetLang string) (string, error) {
	reqBody := LibreTranslateRequest{
		Q:      input,
		Source: sourceLang,
		Target: targetLang,
		Format: "text",
	}
	body, _ := json.Marshal(reqBody)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   2 * time.Minute,
		Transport: tr,
	}
	resp, err := client.Post(
		config.WebPanel.Get().API.LibreTranslate+"/translate",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res LibreTranslateResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", errors.New(res.Error)
	}
	if text, ok := res.TranslatedText.(string); ok {
		return text, nil
	}
	return "", errors.New("unexpected response format")
}

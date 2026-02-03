package controllers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"service-platform/cmd/web_panel/config"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types/events"
)

// APIRafyRequest represents the expected request body for the API
type APIRafyRequest struct {
	Question    string `json:"question"`
	PhoneNumber string `json:"number"`
}

// RafyAPIResponse represents a successful API response
type RafyAPIResponse struct {
	Response interface{} `json:"response"`
}

// RafyDebugResponse represents the detailed debug response structure
type RafyDebugResponse struct {
	Input   string                   `json:"input"`
	Context []map[string]interface{} `json:"context"`
	Answer  RafyAnswer               `json:"answer"`
}

// RafyAnswer represents the answer part of the debug response
type RafyAnswer struct {
	Content          string                 `json:"content"`
	AdditionalKwargs map[string]interface{} `json:"additional_kwargs"`
	ResponseMetadata map[string]interface{} `json:"response_metadata"`
	Type             string                 `json:"type"`
	Name             *string                `json:"name"`
	Id               string                 `json:"id"`
	Example          bool                   `json:"example"`
	ToolCalls        []interface{}          `json:"tool_calls"`
	InvalidToolCalls []interface{}          `json:"invalid_tool_calls"`
	UsageMetadata    map[string]interface{} `json:"usage_metadata"`
}

// GetResponseString extracts the response string from the Response field
func (r *RafyAPIResponse) GetResponseString() (string, error) {
	if str, ok := r.Response.(string); ok {
		return str, nil
	}
	// Try to unmarshal as RafyDebugResponse
	bytes, err := json.Marshal(r.Response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}
	var debug RafyDebugResponse
	if err := json.Unmarshal(bytes, &debug); err != nil {
		return "", fmt.Errorf("failed to unmarshal debug response: %w", err)
	}
	return debug.Answer.Content, nil
}

// RafyAPIErrorDetail represents a simple error response
type RafyAPIErrorDetail struct {
	Detail string `json:"detail"`
}

// RafyAPIValidationError represents a single validation error
type RafyAPIValidationError struct {
	Type  string      `json:"type"`
	Loc   []string    `json:"loc"`
	Msg   string      `json:"msg"`
	Input interface{} `json:"input"`
}

// RafyAPIValidationErrorResponse represents a validation error response
type RafyAPIValidationErrorResponse struct {
	Detail []RafyAPIValidationError `json:"detail"`
}

// GetRafyAPIResponse sends a POST request to the given URL and returns the parsed response or error
func GetRafyAPIResponse(url string, reqBody APIRafyRequest) (resp *RafyAPIResponse, apiErr *RafyAPIErrorDetail, validationErr *RafyAPIValidationErrorResponse, err error) {
	// Marshal the request body
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create custom HTTP client with %d timeout and skip SSL verification
	timeout := config.GetConfig().API.APIRafyTimeout
	if timeout <= 0 {
		timeout = 120 // default to 120 seconds if not set or invalid
	}
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := client.Do(req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read the response body
	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Try to unmarshal as success response
	var successResp RafyAPIResponse
	if err := json.Unmarshal(respBytes, &successResp); err == nil && successResp.Response != nil {
		return &successResp, nil, nil, nil
	}

	// Try to unmarshal as validation error response
	var valErrResp RafyAPIValidationErrorResponse
	if err := json.Unmarshal(respBytes, &valErrResp); err == nil && len(valErrResp.Detail) > 0 {
		return nil, nil, &valErrResp, nil
	}

	// Try to unmarshal as simple error response
	var apiError RafyAPIErrorDetail
	if err := json.Unmarshal(respBytes, &apiError); err == nil && apiError.Detail != "" {
		return nil, &apiError, nil, nil
	}

	// Unknown response
	return nil, nil, nil, fmt.Errorf("unknown response: %s", string(respBytes))
}

func RafyODOOSOPPrompt(url string, question string, phoneNumber string) (string, error) {
	reqBody := APIRafyRequest{
		Question:    question,
		PhoneNumber: phoneNumber,
	}
	resp, apiErr, valErr, err := GetRafyAPIResponse(url, reqBody)
	if err != nil {
		return "", err
	}
	if apiErr != nil {
		return "", fmt.Errorf("API error: %s", apiErr.Detail)
	}
	if valErr != nil {
		var errMsgs []string
		for _, ve := range valErr.Detail {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", strings.Join(ve.Loc, "->"), ve.Msg))
		}
		return "", fmt.Errorf("validation errors: %s", strings.Join(errMsgs, "; "))
	}
	if resp != nil {
		responseStr, err := resp.GetResponseString()
		if err != nil {
			return "", err
		}
		return responseStr, nil
	}
	return "", fmt.Errorf("empty response from Rafy API")
}

func RafyODOOMSPrompt(url string, question string, phoneNumber string) (string, error) {
	reqBody := APIRafyRequest{
		Question:    question,
		PhoneNumber: phoneNumber,
	}
	resp, apiErr, valErr, err := GetRafyAPIResponse(url, reqBody)
	if err != nil {
		return "", err
	}
	if apiErr != nil {
		return "", fmt.Errorf("API error: %s", apiErr.Detail)
	}
	if valErr != nil {
		var errMsgs []string
		for _, ve := range valErr.Detail {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", strings.Join(ve.Loc, "->"), ve.Msg))
		}
		return "", fmt.Errorf("validation errors: %s", strings.Join(errMsgs, "; "))
	}
	if resp != nil {
		responseStr, err := resp.GetResponseString()
		if err != nil {
			return "", err
		}
		return responseStr, nil
	}
	return "", fmt.Errorf("empty response from Rafy API")
}

// RafyLLMPoweredChatBot interacts with the Rafy LLM-powered chatbot API
func RafyLLMPoweredChatBot(url []string, question string, phoneNumber string) (string, string, error) {
	responseLangID := "id"
	responseLangEN := "en"

	// Response always in Indonesian from RafyODOOSOPPrompt
	response, err := RafyODOOSOPPrompt(url[0], question, phoneNumber)
	if err != nil {
		return responseLangID, "", fmt.Errorf("failed to get response from Rafy API: %w", err)
	}

	var noResponse bool = false
	excludedResponses := []string{
		"mohon maaf, data atau sop terkait dengan pertanyaan",
		"saya tidak memiliki informasi tentang",
		"maaf, saya tidak dapat menemukan informasi yang relevan",
		"saya tidak tahu",
		"mohon maaf,",
		"maaf, saat ini sistem kami sedang terkendala",
		"i don't know",
		"i do not know",
		"there is no",
		"there are no",
		"i can't find",
		"i cannot find",
		"pardon me",
		"begging your pardon",
		"ok",
		"oke",
		"okay",
		"oky",
	}

	for _, word := range excludedResponses {
		if strings.EqualFold(strings.TrimSpace(response), word) {
			noResponse = true
			break
		}
	}

	for _, phrase := range excludedResponses {
		if strings.Contains(strings.ToLower(response), phrase) {
			noResponse = true
			break
		}
	}

	if noResponse {
		// try to get response from RafyODOOMS
		response, err = RafyODOOMSPrompt(url[1], question, phoneNumber)
		if err != nil {
			return responseLangEN, "", fmt.Errorf("failed to get response from Rafy ODOOMS API: %w", err)
		}

		// response from RafyODOOMSPrompt always in English
		return responseLangEN, response, nil
	}

	return responseLangID, response, nil
}

func ActiveAIRafy(v *events.Message, userLang string) {
	// eventToDO := "Active Rafy AI Featured for ChatBot"
	jid := v.Info.Sender.User + "@s.whatsapp.net"

	isUserUseAIRafy, isSet, err := GetUserUseAIRafy(jid)
	if err != nil {
		logrus.Errorf("Error checking AI Rafy status for %s: %v", v.Info.Sender.User, err)
		return
	}
	if isSet && isUserUseAIRafy {
		// Already active
		idMsg := "Fitur chatbot dengan AI sudah aktif.\nSilakan mulai mengajukan pertanyaan Anda 😉"
		enMsg := "AI-powered chatbot feature already active.\nPlease start asking your questions 😉"
		SendLangMessage(v.Info.Sender.String(), idMsg, enMsg, userLang)
		return
	}

	err = SetUserUseAIRafy(jid, true)
	if err != nil {
		logrus.Errorf("Error setting AI Rafy status for %s: %v", v.Info.Sender.User, err)
		idMsg := "❌ Gagal mengaktifkan fitur chatbot dengan AI. Silakan coba lagi nanti."
		enMsg := "❌ Failed to activate AI-powered chatbot feature. Please try again later."
		SendLangMessage(v.Info.Sender.String(), idMsg, enMsg, userLang)
		return
	}

	idMsg := "✅ Fitur chatbot dengan AI telah diaktifkan. Silakan mulai mengajukan pertanyaan Anda 😉"
	enMsg := "✅ AI-powered chatbot feature is activated. Please start asking your questions 😉"
	SendLangMessage(jid, idMsg, enMsg, userLang)
}

func DeactivateAIRafy(v *events.Message, userLang string) {
	// eventToDO := "Deactivate Rafy AI Featured for ChatBot"
	jid := v.Info.Sender.User + "@s.whatsapp.net"

	isUserUseAIRafy, isSet, err := GetUserUseAIRafy(jid)
	if err != nil {
		logrus.Errorf("Error checking AI Rafy status for %s: %v", v.Info.Sender.User, err)
		return
	}
	if !isSet || !isUserUseAIRafy {
		// Already inactive
		idMsg := "Fitur chatbot dengan AI sudah nonaktif.\nSilakan ketik *active ai* untuk mengaktifkannya kembali."
		enMsg := "AI-powered chatbot feature already deactivated.\nPlease type *active ai* to enable it again."
		SendLangMessage(jid, idMsg, enMsg, userLang)
		return
	}

	err = SetUserUseAIRafy(jid, false)
	if err != nil {
		logrus.Errorf("Error setting AI Rafy status for %s: %v", v.Info.Sender.User, err)
		idMsg := "❌ Gagal menonaktifkan fitur chatbot dengan AI. Silakan coba lagi nanti."
		enMsg := "❌ Failed to deactivate AI-powered chatbot feature. Please try again later."
		SendLangMessage(v.Info.Sender.String(), idMsg, enMsg, userLang)
		return
	}

	idMsg := "✅ Fitur chatbot dengan AI telah dinonaktifkan.\nSilakan ketik *active ai* untuk mengaktifkannya kembali."
	enMsg := "✅ AI-powered chatbot feature is deactivated.\nPlease type *active ai* to enable it again."
	SendLangMessage(jid, idMsg, enMsg, userLang)
}

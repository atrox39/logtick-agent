package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HTTPSender struct {
	client *http.Client
	url    string
}

func NewHTTPSender(url string) *HTTPSender {
	return &HTTPSender{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		url: url,
	}
}

func (s *HTTPSender) Send(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	req, err := http.NewRequest("POST", s.url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("failed to send request: %s", resp.Status)
}

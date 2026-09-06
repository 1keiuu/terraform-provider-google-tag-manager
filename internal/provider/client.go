package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/1keiuu/terraform-provider-google-tag-manager/internal/discovery"
)

type providerData struct {
	client    *apiClient
	documents map[string]*discovery.Document
}

type apiClient struct {
	httpClient *http.Client
	endpoint   string
	limiter    *rate.Limiter
	maxRetries int
	userAgent  string
}

type apiError struct {
	StatusCode int
	Status     string
	Message    string
	Reason     string
	Body       string
}

func (e *apiError) Error() string {
	message := e.Message
	if message == "" {
		message = strings.TrimSpace(e.Body)
	}
	if message == "" {
		message = e.Status
	}
	if e.Reason != "" {
		return fmt.Sprintf("Tag Manager API returned %s (%s): %s", e.Status, e.Reason, message)
	}
	return fmt.Sprintf("Tag Manager API returned %s: %s", e.Status, message)
}

func (e *apiError) NotFound() bool { return e.StatusCode == http.StatusNotFound }

func (c *apiClient) Call(ctx context.Context, document *discovery.Document, method *discovery.Method, values map[string]any, body map[string]any) (map[string]any, error) {
	requestURL, err := c.buildURL(document, method, values)
	if err != nil {
		return nil, err
	}

	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("wait for Tag Manager API quota: %w", err)
		}

		var requestBody io.Reader
		if payload != nil {
			requestBody = bytes.NewReader(payload)
		}
		request, err := http.NewRequestWithContext(ctx, method.HTTPMethod, requestURL, requestBody)
		if err != nil {
			return nil, fmt.Errorf("create Tag Manager API request: %w", err)
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", c.userAgent)
		if payload != nil {
			request.Header.Set("Content-Type", "application/json")
		}

		response, err := c.httpClient.Do(request)
		if err != nil {
			lastErr = err
			if attempt == c.maxRetries || !retryableTransportError(err) {
				return nil, fmt.Errorf("call Tag Manager API: %w", err)
			}
			if err := waitForRetry(ctx, retryDelay(attempt, "")); err != nil {
				return nil, err
			}
			continue
		}

		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 16<<20))
		closeErr := response.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read Tag Manager API response: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close Tag Manager API response: %w", closeErr)
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			if len(bytes.TrimSpace(responseBody)) == 0 {
				return map[string]any{}, nil
			}
			var result map[string]any
			if err := json.Unmarshal(responseBody, &result); err != nil {
				return nil, fmt.Errorf("decode Tag Manager API response: %w", err)
			}
			return result, nil
		}

		apiErr := decodeAPIError(response, responseBody)
		lastErr = apiErr
		if attempt == c.maxRetries || !retryableAPIError(apiErr) {
			return nil, apiErr
		}
		if err := waitForRetry(ctx, retryDelay(attempt, response.Header.Get("Retry-After"))); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

func (c *apiClient) buildURL(document *discovery.Document, method *discovery.Method, values map[string]any) (string, error) {
	pathValue := method.Path
	for name, parameter := range method.Parameters {
		if parameter.Location != "path" {
			continue
		}
		value, ok := values[name]
		if !ok || value == nil || fmt.Sprint(value) == "" {
			if parameter.Required {
				return "", fmt.Errorf("missing required path parameter %q for %s", name, method.ID)
			}
			continue
		}
		raw := fmt.Sprint(value)
		pathValue = strings.ReplaceAll(pathValue, "{+"+name+"}", escapePathPreservingSlashes(raw))
		pathValue = strings.ReplaceAll(pathValue, "{"+name+"}", url.PathEscape(raw))
	}
	if strings.Contains(pathValue, "{") {
		return "", fmt.Errorf("unresolved path parameter in %q for %s", pathValue, method.ID)
	}

	endpoint := c.endpoint
	if endpoint == "" {
		endpoint = document.RootURL
	}
	parsed, err := url.Parse(strings.TrimSuffix(endpoint, "/") + "/" + strings.TrimPrefix(pathValue, "/"))
	if err != nil {
		return "", fmt.Errorf("construct Tag Manager API URL: %w", err)
	}
	query := parsed.Query()
	query.Set("prettyPrint", "false")
	for name, parameter := range method.Parameters {
		if parameter.Location != "query" {
			continue
		}
		value, ok := values[name]
		if !ok || value == nil {
			continue
		}
		if parameter.Repeated {
			switch entries := value.(type) {
			case []string:
				for _, entry := range entries {
					query.Add(name, entry)
				}
			case []any:
				for _, entry := range entries {
					query.Add(name, fmt.Sprint(entry))
				}
			default:
				query.Add(name, fmt.Sprint(value))
			}
			continue
		}
		query.Set(name, fmt.Sprint(value))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func escapePathPreservingSlashes(value string) string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func decodeAPIError(response *http.Response, body []byte) *apiError {
	result := &apiError{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Body:       string(body),
	}
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		result.Message = envelope.Error.Message
		if len(envelope.Error.Errors) > 0 {
			result.Reason = envelope.Error.Errors[0].Reason
		}
	}
	return result
}

func retryableAPIError(err *apiError) bool {
	if err.StatusCode == http.StatusTooManyRequests || err.StatusCode >= 500 {
		return true
	}
	if err.StatusCode != http.StatusForbidden {
		return false
	}
	reason := strings.ToLower(err.Reason + " " + err.Message)
	for _, marker := range []string{"ratelimitexceeded", "userratelimitexceeded", "quota", "rate limit"} {
		if strings.Contains(reason, marker) {
			return true
		}
	}
	return false
}

func retryableTransportError(err error) bool {
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func retryDelay(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
			return time.Duration(seconds) * time.Second
		}
		if timestamp, err := http.ParseTime(retryAfter); err == nil {
			if delay := time.Until(timestamp); delay > 0 {
				return delay
			}
		}
	}
	base := math.Min(30, math.Pow(2, float64(attempt)))
	jitter := 0.5 + rand.Float64()
	return time.Duration(base * jitter * float64(time.Second))
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

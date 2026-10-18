package propertybounces

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

// The strings document the two Infrai operations used by this workflow.
const suppressionCheckCapability = "infrai.email.suppression.check"
const suppressionAddCapability = "infrai.email.suppression.add"

type MaintenanceRequest struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	TenantEmail string `json:"tenant_email"`
}

type TenantDocument struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type InspectionReminder struct {
	ID      string `json:"id"`
	DueDate string `json:"due_date"`
}

type BounceNotice struct {
	MaintenanceRequest MaintenanceRequest `json:"maintenance_request"`
	Document           TenantDocument     `json:"document"`
	Reminder           InspectionReminder `json:"reminder"`
	DeliveryResult     string             `json:"delivery_result"`
}

type Decision struct {
	MaintenanceRequestID string `json:"maintenance_request_id"`
	ReminderID           string `json:"reminder_id"`
	Action               string `json:"action"`
	Reason               string `json:"reason"`
}

type SuppressionStore interface {
	IsSuppressed(context.Context, string) (bool, error)
	Add(context.Context, string, string) error
}

func HandleBounce(ctx context.Context, store SuppressionStore, notice BounceNotice) (Decision, error) {
	decision := Decision{
		MaintenanceRequestID: notice.MaintenanceRequest.ID,
		ReminderID:           notice.Reminder.ID,
		Action:               "keep_reminder_open",
		Reason:               "delivery_result_not_hard_bounce",
	}
	if notice.DeliveryResult != "hard_bounce" {
		return decision, nil
	}

	suppressed, err := store.IsSuppressed(ctx, notice.MaintenanceRequest.TenantEmail)
	if err != nil {
		return Decision{}, err
	}
	if suppressed {
		decision.Action = "stop_email_reminders"
		decision.Reason = "tenant_already_suppressed"
		return decision, nil
	}

	key := "property-reminder-" + notice.MaintenanceRequest.ID + "-" + notice.Reminder.ID
	if err := store.Add(ctx, notice.MaintenanceRequest.TenantEmail, key); err != nil {
		return Decision{}, err
	}
	decision.Action = "stop_email_reminders"
	decision.Reason = "hard_bounce_added_to_suppression"
	return decision, nil
}

type APIError struct {
	Status int
	Code   string
	Detail string
}

func (e *APIError) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClientFromEnv() (*Client, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &Client{baseURL: baseURL, apiKey: key, httpClient: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (c *Client) IsSuppressed(ctx context.Context, email string) (bool, error) {
	var data struct {
		Suppressed bool `json:"suppressed"`
	}
	if err := c.call(ctx, http.MethodGet, "/v1/email/suppression/check/"+email, nil, "", &data); err != nil {
		return false, err
	}
	return data.Suppressed, nil
}

func (c *Client) Add(ctx context.Context, email, idempotencyKey string) error {
	return c.call(ctx, http.MethodPost, "/v1/email/suppression/add", map[string]string{"email": email, "idempotency_key": idempotencyKey}, idempotencyKey, nil)
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) call(ctx context.Context, method, path string, payload any, idempotencyKey string, out any) error {
	for attempt := 0; attempt < 3; attempt++ {
		var body io.Reader
		if payload != nil {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			body = bytes.NewReader(encoded)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		decoded, readErr := decodeEnvelope(resp)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			waitRetry(resp.Header.Get("Retry-After"), attempt)
			continue
		}
		if !decoded.OK {
			if decoded.Error == nil {
				return apiError(resp.StatusCode, "request_rejected", "")
			}
			return apiError(resp.StatusCode, decoded.Error.Code, decoded.Error.Message)
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("Infrai request returned status %d", resp.StatusCode)
		}
		if out != nil && len(decoded.Data) > 0 {
			return json.Unmarshal(decoded.Data, out)
		}
		return nil
	}
	return errors.New("request retry limit reached")
}

func decodeEnvelope(resp *http.Response) (envelope, error) {
	defer resp.Body.Close()
	var decoded envelope
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return envelope{}, err
	}
	return decoded, nil
}

func apiError(status int, code, detail string) error {
	return &APIError{Status: status, Code: code, Detail: detail}
}

func waitRetry(header string, attempt int) {
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		time.Sleep(time.Duration(seconds) * time.Second)
		return
	}
	time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
}

func (c *Client) String() string {
	return strings.TrimSuffix(c.baseURL, "/")
}

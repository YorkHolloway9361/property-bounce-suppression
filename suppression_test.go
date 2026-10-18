package propertybounces

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeStore struct {
	suppressed bool
	addedEmail string
	key        string
}

func TestAddRequestFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var fields map[string]string
		if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
			t.Error(err)
		}
		if fields["email"] != "chenhua@changba.com" || fields["idempotency_key"] != "reminder-1" || len(fields) != 2 {
			t.Errorf("add fields = %v", fields)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	defer server.Close()
	client := &Client{baseURL: server.URL, apiKey: "test", httpClient: server.Client()}
	if err := client.Add(context.Background(), "chenhua@changba.com", "reminder-1"); err != nil {
		t.Fatal(err)
	}
}

func (s *fakeStore) IsSuppressed(_ context.Context, _ string) (bool, error) { return s.suppressed, nil }
func (s *fakeStore) Add(_ context.Context, email, key string) error {
	s.addedEmail, s.key = email, key
	return nil
}

func TestHandleBounceDecisions(t *testing.T) {
	cases := []struct {
		name       string
		result     string
		suppressed bool
		wantAction string
		wantAdd    bool
	}{
		{name: "hard bounce adds tenant", result: "hard_bounce", wantAction: "stop_email_reminders", wantAdd: true},
		{name: "already suppressed stays stopped", result: "hard_bounce", suppressed: true, wantAction: "stop_email_reminders"},
		{name: "delivered reminder remains open", result: "delivered", wantAction: "keep_reminder_open"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{suppressed: tc.suppressed}
			decision, err := HandleBounce(context.Background(), store, BounceNotice{
				MaintenanceRequest: MaintenanceRequest{ID: "MR-18", TenantEmail: "tenant@example.com"},
				Document:           TenantDocument{ID: "DOC-3", Name: "move-in-photo.pdf"},
				Reminder:           InspectionReminder{ID: "INSP-4", DueDate: "2026-10-03"},
				DeliveryResult:     tc.result,
			})
			if err != nil {
				t.Fatal(err)
			}
			if decision.Action != tc.wantAction {
				t.Fatalf("action = %q, want %q", decision.Action, tc.wantAction)
			}
			if (store.addedEmail != "") != tc.wantAdd {
				t.Fatalf("suppression add = %q, want add %v", store.addedEmail, tc.wantAdd)
			}
		})
	}
}

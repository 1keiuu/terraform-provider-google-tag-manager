package provider

import (
	"strings"
	"testing"
)

func TestProviderUsesCanonicalEndpoint(t *testing.T) {
	t.Parallel()
	configured, ok := New("test")().(*tagManagerProvider)
	if !ok {
		t.Fatal("New returned an unexpected provider type")
	}
	if configured.endpoint != defaultEndpoint {
		t.Fatalf("endpoint = %q, want %q", configured.endpoint, defaultEndpoint)
	}
}

func TestValidatedCredentialOption(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		value     string
		wantError string
	}{
		{
			name:  "service account",
			value: `{"type":"service_account","token_uri":"https://oauth2.googleapis.com/token"}`,
		},
		{
			name:  "authorized user",
			value: `{"type":"authorized_user"}`,
		},
		{
			name:      "malformed JSON",
			value:     `{`,
			wantError: "valid inline JSON",
		},
		{
			name:      "missing type",
			value:     `{}`,
			wantError: "type field",
		},
		{
			name:      "external account",
			value:     `{"type":"external_account"}`,
			wantError: "not accepted directly",
		},
		{
			name:      "non-Google token endpoint",
			value:     `{"type":"service_account","token_uri":"https://example.com/token"}`,
			wantError: "token_uri",
		},
		{
			name:      "non-Google universe",
			value:     `{"type":"authorized_user","universe_domain":"example.com"}`,
			wantError: "universe_domain",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			credentialOption, err := validatedCredentialOption(test.value)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("validatedCredentialOption returned error: %v", err)
				}
				if credentialOption == nil {
					t.Fatal("validatedCredentialOption returned a nil option")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want error containing %q", err, test.wantError)
			}
		})
	}
}

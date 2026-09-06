package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/time/rate"

	"github.com/1keiuu/terraform-provider-google-tag-manager/internal/discovery"
)

func TestEveryDiscoveryMethodBuildsAndSendsARequest(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		version := version
		t.Run(version, func(t *testing.T) {
			document, err := discovery.Load(version)
			if err != nil {
				t.Fatal(err)
			}
			var received *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				received = request.Clone(request.Context())
				response.Header().Set("Content-Type", "application/json")
				_, _ = response.Write([]byte(`{}`))
			}))
			defer server.Close()
			client := &apiClient{httpClient: server.Client(), endpoint: server.URL, limiter: rate.NewLimiter(rate.Inf, 1), userAgent: "contract-test"}

			for _, id := range document.MethodIDs() {
				method, methodErr := document.Method(id)
				if methodErr != nil {
					t.Fatal(methodErr)
				}
				values := sampleMethodValues(method)
				received = nil
				if _, callErr := client.Call(context.Background(), document, method, values, map[string]any{"name": "contract-test"}); callErr != nil {
					t.Errorf("%s: %v", id, callErr)
					continue
				}
				if received == nil {
					t.Errorf("%s: no request received", id)
					continue
				}
				if received.Method != method.HTTPMethod {
					t.Errorf("%s: method = %s, want %s", id, received.Method, method.HTTPMethod)
				}
				if strings.Contains(received.URL.Path, "{") || !strings.HasPrefix(received.URL.Path, "/tagmanager/"+version+"/") {
					t.Errorf("%s: unresolved or invalid path %q", id, received.URL.Path)
				}
			}
		})
	}
}

func TestClientBuildsDiscoveryRequest(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath, gotQuery, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		gotMethod = request.Method
		gotPath = request.URL.Path
		gotQuery = request.URL.RawQuery
		body, _ := io.ReadAll(request.Body)
		gotBody = string(body)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"path":"accounts/1/containers/2/workspaces/3/tags/4","name":"example"}`))
	}))
	defer server.Close()

	document, err := discovery.Load("v2")
	if err != nil {
		t.Fatal(err)
	}
	method, err := document.Method("accounts.containers.workspaces.tags.update")
	if err != nil {
		t.Fatal(err)
	}
	client := &apiClient{httpClient: server.Client(), endpoint: server.URL, limiter: rate.NewLimiter(rate.Inf, 1), userAgent: "test"}
	result, err := client.Call(context.Background(), document, method, map[string]any{
		"path":        "accounts/1/containers/2/workspaces/3/tags/4",
		"fingerprint": "abc",
	}, map[string]any{"name": "example"})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s", gotMethod)
	}
	if gotPath != "/tagmanager/v2/accounts/1/containers/2/workspaces/3/tags/4" {
		t.Errorf("path = %s", gotPath)
	}
	if !strings.Contains(gotQuery, "fingerprint=abc") || !strings.Contains(gotQuery, "prettyPrint=false") {
		t.Errorf("query = %s", gotQuery)
	}
	if gotBody != `{"name":"example"}` {
		t.Errorf("body = %s", gotBody)
	}
	if result["name"] != "example" {
		t.Errorf("result = %#v", result)
	}
}

func TestClientRetriesQuotaErrors(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		attempts++
		response.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			response.Header().Set("Retry-After", "0")
			response.WriteHeader(http.StatusForbidden)
			_, _ = response.Write([]byte(`{"error":{"message":"quota exceeded","errors":[{"reason":"rateLimitExceeded"}]}}`))
			return
		}
		_, _ = response.Write([]byte(`{"account":[]}`))
	}))
	defer server.Close()

	document, _ := discovery.Load("v2")
	method, _ := document.Method("accounts.list")
	client := &apiClient{httpClient: server.Client(), endpoint: server.URL, limiter: rate.NewLimiter(rate.Inf, 1), maxRetries: 1, userAgent: "test"}
	if _, err := client.Call(context.Background(), document, method, nil, nil); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestPathValuesV1(t *testing.T) {
	t.Parallel()
	document, _ := discovery.Load("v1")
	method, _ := document.Method("accounts.containers.tags.get")
	values, err := pathValues(method, "accounts/10/containers/20/tags/30")
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"accountId": "10", "containerId": "20", "tagId": "30"} {
		if got := values[key]; got != want {
			t.Errorf("%s = %v, want %s", key, got, want)
		}
	}
}

func sampleMethodValues(method *discovery.Method) map[string]any {
	values := make(map[string]any)
	ids := map[string]string{
		"accountId": "1", "containerId": "2", "workspaceId": "3", "permissionId": "4",
		"environmentId": "5", "folderId": "6", "tagId": "7", "triggerId": "8",
		"variableId": "9", "containerVersionId": "10", "clientId": "11", "templateId": "12",
		"transformationId": "13", "zoneId": "14", "gtagConfigId": "15",
	}
	for name, parameter := range method.Parameters {
		if parameter.Location == "path" {
			switch name {
			case "path":
				values[name] = sampleCanonicalPath(method.Path)
			case "parent":
				values[name] = sampleParentPath(method.Path)
			default:
				values[name] = ids[name]
			}
			continue
		}
		if !parameter.Required {
			continue
		}
		if parameter.Repeated {
			values[name] = []string{"pageUrl"}
		} else if len(parameter.Enum) > 0 {
			values[name] = parameter.Enum[0]
		} else {
			values[name] = "test"
		}
	}
	return values
}

func sampleCanonicalPath(template string) string {
	collection := "accounts/1"
	for _, entry := range []struct {
		needle string
		path   string
	}{
		{"/containers", "/containers/2"},
		{"/workspaces", "/workspaces/3"},
		{"/user_permissions", "/user_permissions/4"},
		{"/environments", "/environments/5"},
		{"/destinations", "/destinations/6"},
		{"/versions", "/versions/10"},
		{"/clients", "/clients/11"},
		{"/folders", "/folders/6"},
		{"/gtag_config", "/gtag_config/15"},
		{"/tags", "/tags/7"},
		{"/templates", "/templates/12"},
		{"/transformations", "/transformations/13"},
		{"/triggers", "/triggers/8"},
		{"/variables", "/variables/9"},
		{"/zones", "/zones/14"},
		{"/built_in_variables", "/built_in_variables"},
	} {
		if strings.Contains(template, entry.needle) {
			collection += entry.path
		}
	}
	return collection
}

func sampleParentPath(template string) string {
	_ = template
	return "accounts/1/containers/2/workspaces/3"
}

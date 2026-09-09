package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestWorkspaceResourceLifecycle(t *testing.T) {
	var mutex sync.Mutex
	workspace := map[string]any{
		"accountId":   "1",
		"containerId": "2",
		"workspaceId": "3",
		"path":        "accounts/1/containers/2/workspaces/3",
		"name":        "initial",
		"fingerprint": "one",
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		response.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case http.MethodPost, http.MethodPut:
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				http.Error(response, err.Error(), http.StatusBadRequest)
				return
			}
			for key, value := range body {
				workspace[key] = value
			}
			if request.Method == http.MethodPut {
				workspace["fingerprint"] = "two"
			}
			_ = json.NewEncoder(response).Encode(workspace)
		case http.MethodGet:
			_ = json.NewEncoder(response).Encode(workspace)
		case http.MethodDelete:
			response.WriteHeader(http.StatusNoContent)
		default:
			http.Error(response, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	factories := testProviderFactories(server.URL)
	config := func(name, description string) string {
		descriptionConfiguration := ""
		if description != "" {
			descriptionConfiguration = fmt.Sprintf("  description = %q\n", description)
		}
		return fmt.Sprintf(`
provider "gtm" {
	access_token        = "test-token"
	requests_per_second = 1000
	max_retries         = 0
}

resource "gtm_workspace" "test" {
  parent = "accounts/1/containers/2"
  name   = %q
%s
}
`, name, descriptionConfiguration)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: config("initial", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gtm_workspace.test", "id", "accounts/1/containers/2/workspaces/3"),
					resource.TestCheckResourceAttr("gtm_workspace.test", "fingerprint", "one"),
				),
			},
			{
				Config: config("updated", "second"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gtm_workspace.test", "name", "updated"),
					resource.TestCheckResourceAttr("gtm_workspace.test", "fingerprint", "two"),
				),
			},
			{
				ResourceName:      "gtm_workspace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestCreateFailureDoesNotPersistEmptyState(t *testing.T) {
	var createCalls atomic.Int32
	workspace := map[string]any{
		"accountId":   "1",
		"containerId": "2",
		"workspaceId": "3",
		"path":        "accounts/1/containers/2/workspaces/3",
		"name":        "test",
		"fingerprint": "one",
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case http.MethodPost:
			if createCalls.Add(1) == 1 {
				response.WriteHeader(http.StatusBadRequest)
				_, _ = response.Write([]byte(`{"error":{"code":400,"message":"invalid test request","status":"INVALID_ARGUMENT"}}`))
				return
			}
			_ = json.NewEncoder(response).Encode(workspace)
		case http.MethodGet:
			_ = json.NewEncoder(response).Encode(workspace)
		case http.MethodDelete:
			response.WriteHeader(http.StatusNoContent)
		default:
			http.Error(response, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	config := testProviderConfig() + `
resource "gtm_workspace" "test" {
  parent = "accounts/1/containers/2"
  name   = "test"
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`Unable to create GTM resource`),
			},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gtm_workspace.test", "id", "accounts/1/containers/2/workspaces/3"),
				),
			},
		},
	})
}

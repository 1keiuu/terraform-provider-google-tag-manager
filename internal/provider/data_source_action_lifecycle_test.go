package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccountsDataSourcePagination(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("pageToken") == "next" {
			calls.Add(1)
			_, _ = response.Write([]byte(`{"account":[{"accountId":"2","name":"second"}]}`))
			return
		}
		calls.Add(1)
		_, _ = response.Write([]byte(`{"account":[{"accountId":"1","name":"first"}],"nextPageToken":"next"}`))
	}))
	defer server.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(),
		Steps: []resource.TestStep{{
			Config: testProviderConfig(server.URL) + `
data "gtm_accounts" "test" {}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.gtm_accounts.test", "account_json", `[{"accountId":"1","name":"first"},{"accountId":"2","name":"second"}]`),
				resource.TestCheckResourceAttr("data.gtm_accounts.test", "next_page_token", ""),
			),
		}},
	})
	if calls.Load() < 2 || calls.Load()%2 != 0 {
		t.Fatalf("API calls = %d, want complete two-page reads", calls.Load())
	}
}

func TestSyncWorkspaceAction(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/tagmanager/v2/accounts/1/containers/2/workspaces/3:sync" {
			http.Error(response, "unexpected request", http.StatusBadRequest)
			return
		}
		calls.Add(1)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"mergeConflict":[],"syncStatus":{"mergeConflict":false,"syncError":false}}`))
	}))
	defer server.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(),
		Steps: []resource.TestStep{{
			Config: testProviderConfig(server.URL) + `
action "gtm_sync_workspace" "test" {
  config {
    path = "accounts/1/containers/2/workspaces/3"
  }
}

resource "terraform_data" "invoke" {
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.gtm_sync_workspace.test]
    }
  }
}
`,
			PostApplyFunc: func() {
				if calls.Load() != 1 {
					t.Errorf("action API calls = %d, want 1", calls.Load())
				}
			},
		}},
	})
}

func testProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"gtm": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func testProviderConfig(endpoint string) string {
	return fmt.Sprintf(`
provider "gtm" {
  access_token        = "test-token"
  endpoint            = %q
  requests_per_second = 1000
  max_retries         = 0
}
`, endpoint)
}

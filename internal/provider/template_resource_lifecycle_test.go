package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestCustomTemplateResourceRewrittenData(t *testing.T) {
	const templatePath = "accounts/1/containers/2/workspaces/3/templates/4"
	const header = "___TERMS_OF_SERVICE___\n\nTerms here.\n\n"
	const initial = header + "___INFO___\n\n{\"help\":\"x > y\"}\n\n___SANDBOXED_JS_FOR_WEB_TEMPLATE___\n\nrun();\n"
	updated := strings.Replace(initial, "run();", "updated();", 1)
	var mutex sync.Mutex
	var writes int
	template := map[string]any{
		"path": templatePath, "templateId": "4", "name": "test", "fingerprint": "one",
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
			data, ok := body["templateData"].(string)
			if !ok || (data != initial && data != updated) {
				t.Errorf("unexpected templateData request: %q", data)
			}
			writes++
			// Emulate GTM rewriting JSON and stripping the gallery ToS header.
			template["templateData"] = strings.ReplaceAll(strings.TrimPrefix(data, header), ">", `\u003e`)
			_ = json.NewEncoder(response).Encode(template)
		case http.MethodGet:
			_ = json.NewEncoder(response).Encode(template)
		case http.MethodDelete:
			response.WriteHeader(http.StatusNoContent)
		default:
			http.Error(response, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	config := func(data string) string {
		return testProviderConfig() + fmt.Sprintf(`
resource "gtm_custom_template" "test" {
  parent        = "accounts/1/containers/2/workspaces/3"
  name          = "test"
  template_data = %q
}
`, data)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{Config: config(initial), Check: resource.TestCheckResourceAttr("gtm_custom_template.test", "template_data", initial)},
			{Config: config(initial), PlanOnly: true},
			{Config: config(updated), Check: resource.TestCheckResourceAttr("gtm_custom_template.test", "template_data", updated)},
			{Config: config(updated), PlanOnly: true},
			{
				Config: config(updated), PlanOnly: true, ExpectNonEmptyPlan: true,
				PreConfig: func() {
					mutex.Lock()
					defer mutex.Unlock()
					template["templateData"] = strings.Replace(template["templateData"].(string), "updated();", "drifted();", 1)
				},
			},
			{Config: config(updated)},
			{
				Config: config(updated), PlanOnly: true, ExpectNonEmptyPlan: true,
				PreConfig: func() {
					mutex.Lock()
					defer mutex.Unlock()
					template["templateData"] = strings.Replace(template["templateData"].(string), "x", "changed", 1)
				},
			},
			{Config: config(updated)},
		},
	})
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: config(updated), ResourceName: "gtm_custom_template.test",
				ImportState: true, ImportStateId: templatePath, ImportStatePersist: true,
			},
			// Import has no configured spelling yet; the first apply reconciles it.
			{Config: config(updated), Check: resource.TestCheckResourceAttr("gtm_custom_template.test", "template_data", updated)},
			{Config: config(updated), PlanOnly: true},
		},
	})
	mutex.Lock()
	defer mutex.Unlock()
	if writes != 5 {
		t.Errorf("got %d writes, want create, update, import reconciliation, and two drift repairs", writes)
	}
}

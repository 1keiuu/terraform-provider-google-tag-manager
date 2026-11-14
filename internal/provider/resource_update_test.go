package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"golang.org/x/time/rate"
)

func TestUpdateReturnedState(t *testing.T) {
	resources := []struct {
		name   string
		values map[string]string
	}{
		{
			name: "gtm_workspace",
			values: map[string]string{
				"id": "accounts/1/containers/2/workspaces/3", "path": "accounts/1/containers/2/workspaces/3",
				"parent": "accounts/1/containers/2", "account_id": "1", "container_id": "2", "workspace_id": "3",
			},
		},
		{
			name: "gtm_account_settings",
			values: map[string]string{
				"id": "accounts/1", "path": "accounts/1", "account_id": "1",
			},
		},
		{
			name: "gtm_v1_folder",
			values: map[string]string{
				"id": "accounts/1/containers/2/folders/3", "path": "accounts/1/containers/2/folders/3",
				"parent": "accounts/1/containers/2", "account_id": "1", "container_id": "2", "folder_id": "3",
			},
		},
	}
	tests := []struct {
		name         string
		status       int
		transportErr bool
		invalidJSON  bool
		invalidField bool
	}{
		{name: "forbidden", status: http.StatusForbidden},
		{name: "not found", status: http.StatusNotFound},
		{name: "stale fingerprint", status: http.StatusPreconditionFailed},
		{name: "server error", status: http.StatusInternalServerError},
		{name: "transport error", transportErr: true},
		{name: "invalid JSON", status: http.StatusOK, invalidJSON: true},
		{name: "invalid response field", status: http.StatusOK, invalidField: true},
		{name: "success", status: http.StatusOK},
	}
	for _, target := range resources {
		for _, test := range tests {
			t.Run(target.name+"/"+test.name, func(t *testing.T) {
				ctx := context.Background()
				var r *genericResource
				for _, spec := range resourceSpecs() {
					if spec.TypeName == target.name {
						r = &genericResource{spec: spec}
						break
					}
				}
				if r == nil {
					t.Fatalf("resource %q not found", target.name)
				}
				var schemaResponse resource.SchemaResponse
				r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
				if schemaResponse.Diagnostics.HasError() {
					t.Fatal(schemaResponse.Diagnostics)
				}
				stateType := schemaResponse.Schema.Type().TerraformType(ctx).(tftypes.Object)
				priorValues := make(map[string]tftypes.Value)
				planValues := make(map[string]tftypes.Value)
				result := make(map[string]any)
				for name, attributeType := range stateType.AttributeTypes {
					priorValues[name] = tftypes.NewValue(attributeType, nil)
					if value, ok := target.values[name]; ok {
						priorValues[name] = tftypes.NewValue(attributeType, value)
						result[r.definitions[name].APIName] = value
					}
					planValues[name] = priorValues[name].Copy()
					definition := r.definitions[name]
					if definition.Computed && !definition.Optional && !definition.Required {
						planValues[name] = tftypes.NewValue(attributeType, tftypes.UnknownValue)
					}
				}
				priorValues["name"] = tftypes.NewValue(tftypes.String, "initial")
				priorValues["fingerprint"] = tftypes.NewValue(tftypes.String, "one")
				planValues["name"] = tftypes.NewValue(tftypes.String, "updated")
				result["name"] = "updated"
				result["fingerprint"] = "two"
				if test.invalidField {
					result["fingerprint"] = []string{"invalid"}
				}
				body, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				if test.invalidJSON {
					body = []byte(`{"fingerprint":`)
				}
				if test.status >= 400 {
					body = []byte(`{"error":{"message":"test update failure"}}`)
				}
				calls := 0
				r.data = &providerData{client: &apiClient{
					httpClient: &http.Client{Transport: updateRoundTripper(func(request *http.Request) (*http.Response, error) {
						calls++
						if request.Method != http.MethodPut {
							t.Errorf("method = %q, want PUT", request.Method)
						}
						if got := request.URL.Query().Get("fingerprint"); got != "one" {
							t.Errorf("fingerprint = %q, want one", got)
						}
						var sent map[string]any
						if err := json.NewDecoder(request.Body).Decode(&sent); err != nil {
							t.Fatal(err)
						}
						if sent["name"] != "updated" {
							t.Errorf("request name = %v, want updated", sent["name"])
						}
						if test.transportErr {
							return nil, errors.New("test transport failure")
						}
						return &http.Response{
							StatusCode: test.status,
							Status:     http.StatusText(test.status),
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader(string(body))),
						}, nil
					})},
					limiter: rate.NewLimiter(rate.Inf, 1),
				}}
				prior := tfsdk.State{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(stateType, priorValues)}
				plan := tfsdk.Plan{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(stateType, planValues)}
				priorSnapshot, planSnapshot := prior.Raw.Copy(), plan.Raw.Copy()
				// The framework pre-populates the response with the prior state.
				response := resource.UpdateResponse{State: prior}
				r.Update(ctx, resource.UpdateRequest{State: prior, Plan: plan}, &response)
				if calls != 1 {
					t.Fatalf("API calls = %d, want 1", calls)
				}
				if !prior.Raw.Equal(priorSnapshot) || !plan.Raw.Equal(planSnapshot) {
					t.Fatal("Update mutated the request state or plan")
				}
				if test.status != http.StatusOK || test.invalidJSON || test.invalidField {
					if !response.Diagnostics.HasError() {
						t.Fatal("expected an update error")
					}
					// Compare before any Read can repair accidentally modified fields.
					if !response.State.Raw.Equal(priorSnapshot) {
						t.Fatalf("failed Update changed state:\ngot  %s\nwant %s", response.State.Raw, priorSnapshot)
					}
					return
				}
				if response.Diagnostics.HasError() {
					t.Fatal(response.Diagnostics)
				}
				priorValues["name"] = tftypes.NewValue(tftypes.String, "updated")
				priorValues["fingerprint"] = tftypes.NewValue(tftypes.String, "two")
				if expected := tftypes.NewValue(stateType, priorValues); !response.State.Raw.Equal(expected) {
					t.Fatalf("successful Update state:\ngot  %s\nwant %s", response.State.Raw, expected)
				}
			})
		}
	}
}

type updateRoundTripper func(*http.Request) (*http.Response, error)

func (f updateRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

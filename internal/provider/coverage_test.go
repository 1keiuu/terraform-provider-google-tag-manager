package provider

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/1keiuu/terraform-provider-google-tag-manager/internal/discovery"
)

func TestEveryDiscoveryMethodIsCovered(t *testing.T) {
	t.Parallel()
	covered := supportedMethodIDs()
	expectedCounts := map[string]int{"v1": 49, "v2": 106}
	for _, version := range []string{"v1", "v2"} {
		document, err := discovery.Load(version)
		if err != nil {
			t.Fatal(err)
		}
		ids := document.MethodIDs()
		if len(ids) != expectedCounts[version] {
			t.Fatalf("%s discovery method count changed: got %d, want %d; refresh provider coverage", version, len(ids), expectedCounts[version])
		}
		var missing []string
		for _, id := range ids {
			if _, ok := covered[version][id]; !ok {
				missing = append(missing, id)
			}
		}
		var unknown []string
		for id := range covered[version] {
			if _, err := document.Method(id); err != nil {
				unknown = append(unknown, id)
			}
		}
		sort.Strings(missing)
		sort.Strings(unknown)
		if len(missing) > 0 || len(unknown) > 0 {
			t.Errorf("%s coverage mismatch\nmissing: %v\nunknown: %v", version, missing, unknown)
		}
	}
}

func TestDefaultScopesCoverEveryDiscoveryMethod(t *testing.T) {
	t.Parallel()
	configured := make(map[string]struct{}, len(defaultScopes))
	for _, scope := range defaultScopes {
		configured[scope] = struct{}{}
	}
	for _, version := range []string{"v1", "v2"} {
		document, err := discovery.Load(version)
		if err != nil {
			t.Fatal(err)
		}
		for _, methodID := range document.MethodIDs() {
			method, err := document.Method(methodID)
			if err != nil {
				t.Fatal(err)
			}
			for _, scope := range method.Scopes {
				if _, covered := configured[scope]; !covered {
					t.Errorf("default scopes do not cover %s: %s", methodID, scope)
				}
			}
		}
	}
}

func TestProviderSchemaIsValid(t *testing.T) {
	t.Parallel()
	server := providerserver.NewProtocol6(New("test")())()
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("provider schema error: %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
	if response.Provider == nil {
		t.Fatal("provider schema is nil")
	}
	if got, want := len(response.ResourceSchemas), len(resourceSpecs()); got != want {
		t.Errorf("resource count = %d, want %d", got, want)
	}
	if got, want := len(response.DataSourceSchemas), len(dataSourceSpecs()); got != want {
		t.Errorf("data source count = %d, want %d", got, want)
	}
	if got, want := len(response.ActionSchemas), len(actionSpecs()); got != want {
		t.Errorf("action count = %d, want %d", got, want)
	}
}

func TestTypeNamesAreUnique(t *testing.T) {
	t.Parallel()
	check := func(kind string, names []string) {
		t.Helper()
		seen := make(map[string]struct{}, len(names))
		for _, name := range names {
			if _, duplicate := seen[name]; duplicate {
				t.Errorf("duplicate %s type name %q", kind, name)
			}
			seen[name] = struct{}{}
		}
	}
	resources := make([]string, 0, len(resourceSpecs()))
	for _, spec := range resourceSpecs() {
		resources = append(resources, spec.TypeName)
	}
	dataSources := make([]string, 0, len(dataSourceSpecs()))
	for _, spec := range dataSourceSpecs() {
		dataSources = append(dataSources, spec.TypeName)
	}
	actions := make([]string, 0, len(actionSpecs()))
	for _, spec := range actionSpecs() {
		actions = append(actions, spec.TypeName)
	}
	check("resource", resources)
	check("data source", dataSources)
	check("action", actions)
}

func TestResourcesWithoutUpdateReplaceConfiguredFields(t *testing.T) {
	t.Parallel()
	for _, spec := range resourceSpecs() {
		if spec.UpdateMethod != "" {
			continue
		}
		document, err := discovery.Load(spec.Version)
		if err != nil {
			t.Fatal(err)
		}
		attributes, definitions, err := resourceSchemaAttributes(spec, document)
		if err != nil {
			t.Fatal(err)
		}
		for name, definition := range definitions {
			if !definition.ForceNew {
				continue
			}
			modifiers := reflect.ValueOf(attributes[name]).FieldByName("PlanModifiers")
			if !modifiers.IsValid() || modifiers.Len() == 0 {
				t.Errorf("%s.%s must require replacement because the API has no update method", spec.TypeName, name)
			}
		}
	}
}

func TestResourceSchemasCoverRequestBodies(t *testing.T) {
	t.Parallel()
	for _, spec := range resourceSpecs() {
		document, err := discovery.Load(spec.Version)
		if err != nil {
			t.Fatal(err)
		}
		_, definitions, err := resourceSchemaAttributes(spec, document)
		if err != nil {
			t.Fatal(err)
		}
		apiNames := make(map[string]struct{}, len(definitions))
		for _, definition := range definitions {
			apiNames[definition.APIName] = struct{}{}
		}
		for _, methodID := range []string{spec.CreateMethod, spec.UpdateMethod} {
			if methodID == "" {
				continue
			}
			method, err := document.Method(methodID)
			if err != nil {
				t.Fatal(err)
			}
			if method.Request == nil || method.Request.Ref == "" {
				continue
			}
			requestSchema, err := document.Schema(method.Request.Ref)
			if err != nil {
				t.Fatal(err)
			}
			for name := range requestSchema.Properties {
				if _, covered := apiNames[name]; !covered {
					t.Errorf("%s does not expose %s request field %q", spec.TypeName, methodID, name)
				}
			}
		}
	}
}

func TestEnvironmentAndWorkspaceFieldSemantics(t *testing.T) {
	t.Parallel()
	document, err := discovery.Load("v2")
	if err != nil {
		t.Fatal(err)
	}
	specs := make(map[string]ResourceSpec)
	for _, spec := range resourceSpecs() {
		specs[spec.TypeName] = spec
	}

	environmentAttributes, _, err := resourceSchemaAttributes(specs["gtm_environment"], document)
	if err != nil {
		t.Fatal(err)
	}
	environmentURL := environmentAttributes["url"].(resourceschema.StringAttribute)
	if !environmentURL.Optional || !environmentURL.Computed {
		t.Error("gtm_environment.url must accept configuration and preserve API defaults")
	}

	workspaceAttributes, _, err := resourceSchemaAttributes(specs["gtm_workspace"], document)
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := workspaceAttributes["workspace_id"].(resourceschema.StringAttribute)
	if !workspaceID.Computed || workspaceID.Optional {
		t.Error("gtm_workspace.workspace_id must be read-only")
	}
}

func Example_supportedCoverage() {
	covered := supportedMethodIDs()
	fmt.Println(len(covered["v1"]), len(covered["v2"]))
	// Output: 49 106
}

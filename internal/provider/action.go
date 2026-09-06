package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"

	"github.com/1keiuu/terraform-provider-google-tag-manager/internal/discovery"
)

type genericAction struct {
	spec        ActionSpec
	data        *providerData
	document    *discovery.Document
	method      *discovery.Method
	definitions map[string]fieldDefinition
}

var _ action.Action = &genericAction{}
var _ action.ActionWithConfigure = &genericAction{}

func newGenericAction(spec ActionSpec) action.Action {
	return &genericAction{spec: spec}
}

func (a *genericAction) Metadata(_ context.Context, _ action.MetadataRequest, response *action.MetadataResponse) {
	response.TypeName = a.spec.TypeName
}

func (a *genericAction) Schema(_ context.Context, _ action.SchemaRequest, response *action.SchemaResponse) {
	document, err := discovery.Load(a.spec.Version)
	if err != nil {
		response.Diagnostics.AddError("Unable to load GTM API discovery", err.Error())
		return
	}
	method, err := document.Method(a.spec.Method)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve GTM action method", err.Error())
		return
	}
	attributes, definitions, err := actionSchemaAttributes(method, document)
	if err != nil {
		response.Diagnostics.AddError("Unable to build action schema", err.Error())
		return
	}
	a.document = document
	a.method = method
	a.definitions = definitions
	response.Schema = schema.Schema{
		Description: method.Description,
		Attributes:  attributes,
	}
}

func (a *genericAction) Configure(_ context.Context, request action.ConfigureRequest, response *action.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	data, ok := request.ProviderData.(*providerData)
	if !ok {
		response.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *providerData, got %T", request.ProviderData))
		return
	}
	a.data = data
	a.document = data.documents[a.spec.Version]
	method, err := a.document.Method(a.spec.Method)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve GTM action method", err.Error())
		return
	}
	a.method = method
	_, definitions, err := actionSchemaAttributes(method, a.document)
	if err != nil {
		response.Diagnostics.AddError("Unable to configure action schema", err.Error())
		return
	}
	a.definitions = definitions
}

func (a *genericAction) Invoke(ctx context.Context, request action.InvokeRequest, response *action.InvokeResponse) {
	if a.data == nil || a.data.client == nil || a.document == nil || a.method == nil {
		response.Diagnostics.AddError("Provider is not configured", "Configure the gtm provider before invoking this action.")
		return
	}
	fieldValues, diagnostics := readFieldValues(ctx, request.Config, a.definitions)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	methodValues := make(map[string]any)
	for name := range a.method.Parameters {
		if value, ok := fieldValues[name]; ok {
			methodValues[name] = value
		}
	}
	body := actionRequestBody(a.method, a.document, fieldValues, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	target := actionTarget(methodValues)
	if response.SendProgress != nil {
		response.SendProgress(action.InvokeProgressEvent{Message: fmt.Sprintf("Invoking %s for %s", a.method.ID, target)})
	}
	result, err := a.data.client.Call(ctx, a.document, a.method, methodValues, body)
	if err != nil {
		response.Diagnostics.AddError("Unable to invoke GTM action", err.Error())
		return
	}
	if compilerError, ok := result["compilerError"].(bool); ok && compilerError {
		response.Diagnostics.AddWarning("GTM compiler errors reported", "The API completed the operation but reported compiler errors. Inspect the affected workspace or container version in Tag Manager.")
	}
	if response.SendProgress != nil {
		response.SendProgress(action.InvokeProgressEvent{Message: actionCompletionMessage(a.method, target, result)})
	}
}

func actionRequestBody(method *discovery.Method, document *discovery.Document, values map[string]any, diagnostics interface{ AddError(string, string) }) map[string]any {
	if method.Request == nil || method.Request.Ref == "" {
		return nil
	}
	body := make(map[string]any)
	if raw, ok := values["requestJson"]; ok {
		object, valid := raw.(map[string]any)
		if !valid {
			diagnostics.AddError("Invalid action request JSON", "request_json must encode a JSON object.")
			return nil
		}
		for key, value := range object {
			body[key] = value
		}
	}
	requestSchema, err := document.Schema(method.Request.Ref)
	if err != nil {
		diagnostics.AddError("Unable to resolve GTM request schema", err.Error())
		return nil
	}
	for name := range requestSchema.Properties {
		if value, ok := values[name]; ok {
			body[name] = value
		}
	}
	return body
}

func actionTarget(values map[string]any) string {
	for _, name := range []string{"path", "parent"} {
		if value, ok := values[name]; ok && fmt.Sprint(value) != "" {
			return fmt.Sprint(value)
		}
	}
	parts := make([]string, 0, len(values))
	for _, name := range []string{"accountId", "containerId", "workspaceId", "containerVersionId"} {
		if value, ok := values[name]; ok {
			parts = append(parts, fmt.Sprintf("%s=%v", snakeCase(name), value))
		}
	}
	if len(parts) == 0 {
		return "the configured target"
	}
	return strings.Join(parts, ", ")
}

func actionCompletionMessage(method *discovery.Method, target string, result map[string]any) string {
	if object, ok := result["containerVersion"].(map[string]any); ok {
		if pathValue := fmt.Sprint(object["path"]); pathValue != "" && pathValue != "<nil>" {
			return fmt.Sprintf("Completed %s for %s; container version: %s", method.ID, target, pathValue)
		}
	}
	if conflicts, ok := result["mergeConflict"].([]any); ok {
		return fmt.Sprintf("Completed %s for %s; merge conflicts: %d", method.ID, target, len(conflicts))
	}
	return fmt.Sprintf("Completed %s for %s", method.ID, target)
}

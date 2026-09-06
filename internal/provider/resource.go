package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/1keiuu/terraform-provider-google-tag-manager/internal/discovery"
)

type genericResource struct {
	spec        ResourceSpec
	data        *providerData
	document    *discovery.Document
	definitions map[string]fieldDefinition
}

var _ resource.Resource = &genericResource{}
var _ resource.ResourceWithConfigure = &genericResource{}
var _ resource.ResourceWithImportState = &genericResource{}

func newGenericResource(spec ResourceSpec) resource.Resource {
	return &genericResource{spec: spec}
}

func (r *genericResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = r.spec.TypeName
}

func (r *genericResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	document, err := discovery.Load(r.spec.Version)
	if err != nil {
		response.Diagnostics.AddError("Unable to load GTM API discovery", err.Error())
		return
	}
	attributes, definitions, err := resourceSchemaAttributes(r.spec, document)
	if err != nil {
		response.Diagnostics.AddError("Unable to build resource schema", err.Error())
		return
	}
	r.document = document
	r.definitions = definitions
	response.Schema = schema.Schema{
		Description: fmt.Sprintf("Manages a Google Tag Manager %s object through API %s.", r.spec.SchemaRef, r.spec.Version),
		Attributes:  attributes,
	}
}

func (r *genericResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	data, ok := request.ProviderData.(*providerData)
	if !ok {
		response.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *providerData, got %T", request.ProviderData))
		return
	}
	r.data = data
	r.document = data.documents[r.spec.Version]
	_, definitions, err := resourceSchemaAttributes(r.spec, r.document)
	if err != nil {
		response.Diagnostics.AddError("Unable to configure resource schema", err.Error())
		return
	}
	r.definitions = definitions
}

func (r *genericResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	if !r.ready(&response.Diagnostics) {
		return
	}
	response.State.Raw = request.Plan.Raw
	response.State.Schema = request.Plan.Schema
	response.Diagnostics.Append(initializeComputedFields(ctx, &response.State, r.definitions)...)
	if r.spec.AdoptExisting {
		r.createAccountSettings(ctx, request, response)
		return
	}
	method, err := r.document.Method(r.spec.CreateMethod)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve GTM create method", err.Error())
		return
	}
	parent, diagnostics := stringAttribute(ctx, request.Plan, "parent")
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	values, err := pathValues(method, parent)
	if err != nil {
		response.Diagnostics.AddError("Invalid parent path", err.Error())
		return
	}
	if r.spec.BuiltInVariable {
		variableType, currentDiagnostics := stringAttribute(ctx, request.Plan, "type")
		response.Diagnostics.Append(currentDiagnostics...)
		values["type"] = []string{variableType}
	}
	body, diagnostics := r.requestBody(ctx, request.Plan, method)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	result, err := r.data.client.Call(ctx, r.document, method, values, body)
	if err != nil {
		response.Diagnostics.AddError("Unable to create GTM resource", err.Error())
		return
	}
	if r.spec.CreateResponseField != "" {
		result, err = objectField(result, r.spec.CreateResponseField)
		if err != nil {
			response.Diagnostics.AddError("Unexpected GTM create response", err.Error())
			return
		}
	}
	if r.spec.BuiltInVariable {
		result, err = firstArrayObject(result, "builtInVariable")
		if err != nil {
			response.Diagnostics.AddError("Unexpected built-in variable response", err.Error())
			return
		}
	}
	resourcePath, err := r.createdPath(parent, result)
	if err != nil {
		response.Diagnostics.AddError("Unable to determine GTM resource path", err.Error())
		return
	}
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), resourcePath)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("path"), resourcePath)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("parent"), parent)...)
	response.Diagnostics.Append(writeResponse(ctx, &response.State, r.definitions, result)...)
}

func (r *genericResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	if !r.ready(&response.Diagnostics) {
		return
	}
	if r.spec.BuiltInVariable {
		r.readBuiltInVariable(ctx, request, response)
		return
	}
	method, err := r.document.Method(r.spec.ReadMethod)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve GTM read method", err.Error())
		return
	}
	resourcePath, diagnostics := stringAttribute(ctx, request.State, "path")
	response.Diagnostics.Append(diagnostics...)
	values, err := pathValues(method, resourcePath)
	if err != nil {
		response.Diagnostics.AddError("Invalid resource path", err.Error())
		return
	}
	result, err := r.data.client.Call(ctx, r.document, method, values, nil)
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError("Unable to read GTM resource", err.Error())
		return
	}
	response.Diagnostics.Append(writeResponse(ctx, &response.State, r.definitions, result)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), resourcePath)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("path"), resourcePath)...)
}

func (r *genericResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	if !r.ready(&response.Diagnostics) {
		return
	}
	response.State.Raw = request.Plan.Raw
	response.State.Schema = request.Plan.Schema
	response.Diagnostics.Append(initializeComputedFields(ctx, &response.State, r.definitions)...)
	method, err := r.document.Method(r.spec.UpdateMethod)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve GTM update method", err.Error())
		return
	}
	resourcePath, diagnostics := stringAttribute(ctx, request.State, "path")
	response.Diagnostics.Append(diagnostics...)
	values, err := pathValues(method, resourcePath)
	if err != nil {
		response.Diagnostics.AddError("Invalid resource path", err.Error())
		return
	}
	if _, supported := method.Parameters["fingerprint"]; supported {
		fingerprint, currentDiagnostics := stringAttribute(ctx, request.State, "fingerprint")
		response.Diagnostics.Append(currentDiagnostics...)
		if fingerprint != "" {
			values["fingerprint"] = fingerprint
		}
	}
	body, currentDiagnostics := r.requestBody(ctx, request.Plan, method)
	response.Diagnostics.Append(currentDiagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	result, err := r.data.client.Call(ctx, r.document, method, values, body)
	if err != nil {
		response.Diagnostics.AddError("Unable to update GTM resource", updateErrorMessage(err))
		return
	}
	response.Diagnostics.Append(writeResponse(ctx, &response.State, r.definitions, result)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), resourcePath)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("path"), resourcePath)...)
	if !r.spec.AdoptExisting {
		parent, parentDiagnostics := stringAttribute(ctx, request.Plan, "parent")
		response.Diagnostics.Append(parentDiagnostics...)
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("parent"), parent)...)
	}
}

func (r *genericResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	if !r.ready(&response.Diagnostics) || r.spec.AdoptExisting {
		return
	}
	method, err := r.document.Method(r.spec.DeleteMethod)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve GTM delete method", err.Error())
		return
	}
	resourcePath, diagnostics := stringAttribute(ctx, request.State, "path")
	response.Diagnostics.Append(diagnostics...)
	if r.spec.BuiltInVariable {
		parent, parentDiagnostics := stringAttribute(ctx, request.State, "parent")
		variableType, typeDiagnostics := stringAttribute(ctx, request.State, "type")
		response.Diagnostics.Append(parentDiagnostics...)
		response.Diagnostics.Append(typeDiagnostics...)
		resourcePath = strings.TrimSuffix(parent, "/") + "/built_in_variables"
		values, currentErr := pathValues(method, resourcePath)
		if currentErr != nil {
			response.Diagnostics.AddError("Invalid built-in variable path", currentErr.Error())
			return
		}
		values["type"] = []string{variableType}
		_, err = r.data.client.Call(ctx, r.document, method, values, nil)
	} else {
		values, currentErr := pathValues(method, resourcePath)
		if currentErr != nil {
			response.Diagnostics.AddError("Invalid resource path", currentErr.Error())
			return
		}
		_, err = r.data.client.Call(ctx, r.document, method, values, nil)
	}
	if err != nil && !isNotFound(err) {
		response.Diagnostics.AddError("Unable to delete GTM resource", err.Error())
	}
}

func (r *genericResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	resourcePath := strings.Trim(request.ID, "/")
	if r.spec.AdoptExisting {
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("path"), resourcePath)...)
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), resourcePath)...)
		return
	}
	parent := parentPath(resourcePath)
	if r.spec.BuiltInVariable {
		parts := strings.Split(resourcePath, "/")
		if len(parts) < 3 || parts[len(parts)-2] != "built_in_variables" {
			response.Diagnostics.AddError("Invalid import ID", "Use <workspace-path>/built_in_variables/<type>.")
			return
		}
		parent = strings.Join(parts[:len(parts)-2], "/")
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("type"), parts[len(parts)-1])...)
	}
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("parent"), parent)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("path"), resourcePath)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), resourcePath)...)
}

func (r *genericResource) createAccountSettings(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	resourcePath, diagnostics := stringAttribute(ctx, request.Plan, "path")
	response.Diagnostics.Append(diagnostics...)
	readMethod, err := r.document.Method(r.spec.ReadMethod)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve GTM account read method", err.Error())
		return
	}
	values, err := pathValues(readMethod, resourcePath)
	if err != nil {
		response.Diagnostics.AddError("Invalid account path", err.Error())
		return
	}
	if _, err := r.data.client.Call(ctx, r.document, readMethod, values, nil); err != nil {
		response.Diagnostics.AddError("Unable to read existing GTM account", err.Error())
		return
	}
	updateMethod, err := r.document.Method(r.spec.UpdateMethod)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve GTM account update method", err.Error())
		return
	}
	body, currentDiagnostics := r.requestBody(ctx, request.Plan, updateMethod)
	response.Diagnostics.Append(currentDiagnostics...)
	result, err := r.data.client.Call(ctx, r.document, updateMethod, values, body)
	if err != nil {
		response.Diagnostics.AddError("Unable to update GTM account settings", err.Error())
		return
	}
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), resourcePath)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("path"), resourcePath)...)
	response.Diagnostics.Append(writeResponse(ctx, &response.State, r.definitions, result)...)
}

func (r *genericResource) readBuiltInVariable(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	method, err := r.document.Method(r.spec.ReadMethod)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve built-in variable list method", err.Error())
		return
	}
	parent, parentDiagnostics := stringAttribute(ctx, request.State, "parent")
	variableType, typeDiagnostics := stringAttribute(ctx, request.State, "type")
	response.Diagnostics.Append(parentDiagnostics...)
	response.Diagnostics.Append(typeDiagnostics...)
	values, err := pathValues(method, parent)
	if err != nil {
		response.Diagnostics.AddError("Invalid parent path", err.Error())
		return
	}
	result, err := r.data.client.Call(ctx, r.document, method, values, nil)
	if err != nil {
		response.Diagnostics.AddError("Unable to list built-in variables", err.Error())
		return
	}
	entries, _ := result["builtInVariable"].([]any)
	for _, entry := range entries {
		object, ok := entry.(map[string]any)
		if ok && fmt.Sprint(object["type"]) == variableType {
			response.Diagnostics.Append(writeResponse(ctx, &response.State, r.definitions, object)...)
			return
		}
	}
	response.State.RemoveResource(ctx)
}

func (r *genericResource) requestBody(ctx context.Context, getter attributeGetter, method *discovery.Method) (map[string]any, diag.Diagnostics) {
	if method.Request == nil || method.Request.Ref == "" {
		return nil, nil
	}
	requestSchema, err := r.document.Schema(method.Request.Ref)
	if err != nil {
		var diagnostics diag.Diagnostics
		diagnostics.AddError("Unable to resolve GTM request schema", err.Error())
		return nil, diagnostics
	}
	values, diagnostics := readFieldValues(ctx, getter, r.definitions)
	body := make(map[string]any)
	for name := range requestSchema.Properties {
		if value, ok := values[name]; ok {
			body[name] = value
		}
	}
	return body, diagnostics
}

func (r *genericResource) createdPath(parent string, result map[string]any) (string, error) {
	if r.spec.BuiltInVariable {
		return strings.TrimSuffix(parent, "/") + "/built_in_variables/" + fmt.Sprint(result["type"]), nil
	}
	if value := fmt.Sprint(result["path"]); value != "" && value != "<nil>" {
		return strings.Trim(value, "/"), nil
	}
	identifier := fmt.Sprint(result[r.spec.IDField])
	if identifier == "" || identifier == "<nil>" {
		return "", fmt.Errorf("response did not contain path or %s", r.spec.IDField)
	}
	return strings.TrimSuffix(parent, "/") + "/" + r.spec.Collection + "/" + identifier, nil
}

func (r *genericResource) ready(diagnostics *diag.Diagnostics) bool {
	if r.data == nil || r.data.client == nil || r.document == nil {
		diagnostics.AddError("Provider is not configured", "Configure the gtm provider before using this resource.")
		return false
	}
	return true
}

func pathValues(method *discovery.Method, canonicalPath string) (map[string]any, error) {
	canonicalPath = strings.Trim(canonicalPath, "/")
	values := make(map[string]any)
	if _, ok := method.Parameters["path"]; ok {
		values["path"] = canonicalPath
	}
	if _, ok := method.Parameters["parent"]; ok {
		values["parent"] = canonicalPath
	}
	identifiers := identifiersFromPath(canonicalPath)
	for name, value := range identifiers {
		if _, ok := method.Parameters[name]; ok {
			values[name] = value
		}
	}
	for name, parameter := range method.Parameters {
		if parameter.Location == "path" && parameter.Required {
			if value, ok := values[name]; !ok || fmt.Sprint(value) == "" {
				return nil, fmt.Errorf("path %q does not provide %s", canonicalPath, name)
			}
		}
	}
	return values, nil
}

func identifiersFromPath(value string) map[string]string {
	collectionIDs := map[string]string{
		"accounts":         "accountId",
		"containers":       "containerId",
		"permissions":      "permissionId",
		"user_permissions": "permissionId",
		"environments":     "environmentId",
		"workspaces":       "workspaceId",
		"clients":          "clientId",
		"folders":          "folderId",
		"gtag_config":      "gtagConfigId",
		"tags":             "tagId",
		"templates":        "templateId",
		"transformations":  "transformationId",
		"triggers":         "triggerId",
		"variables":        "variableId",
		"zones":            "zoneId",
		"versions":         "containerVersionId",
	}
	parts := strings.Split(strings.Trim(value, "/"), "/")
	result := make(map[string]string)
	for index := 0; index+1 < len(parts); index += 2 {
		if name, ok := collectionIDs[parts[index]]; ok {
			result[name] = parts[index+1]
		}
	}
	return result
}

func parentPath(resourcePath string) string {
	parts := strings.Split(strings.Trim(resourcePath, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:len(parts)-2], "/")
}

func objectField(result map[string]any, name string) (map[string]any, error) {
	value, ok := result[name]
	if !ok {
		return nil, fmt.Errorf("response does not contain %q", name)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response field %q has type %T", name, value)
	}
	return object, nil
}

func firstArrayObject(result map[string]any, name string) (map[string]any, error) {
	value, ok := result[name]
	if !ok {
		return nil, fmt.Errorf("response does not contain %q", name)
	}
	entries, ok := value.([]any)
	if !ok || len(entries) == 0 {
		return nil, fmt.Errorf("response field %q is empty or has type %T", name, value)
	}
	object, ok := entries[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response field %q entry has type %T", name, entries[0])
	}
	return object, nil
}

func isNotFound(err error) bool {
	var apiErr *apiError
	return errors.As(err, &apiErr) && apiErr.NotFound()
}

func updateErrorMessage(err error) string {
	var apiErr *apiError
	if errors.As(err, &apiErr) && (apiErr.StatusCode == 409 || apiErr.StatusCode == 412) {
		return err.Error() + ". The remote object changed after refresh; run terraform plan again before applying."
	}
	return err.Error()
}

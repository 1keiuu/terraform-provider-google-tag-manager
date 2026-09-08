package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/1keiuu/terraform-provider-google-tag-manager/internal/discovery"
)

type fieldKind int

const (
	fieldString fieldKind = iota
	fieldBool
	fieldInt64
	fieldFloat64
	fieldStringList
	fieldBoolList
	fieldInt64List
	fieldFloat64List
	fieldJSON
)

type fieldDefinition struct {
	APIName     string
	Terraform   string
	Description string
	Enum        []string
	Pattern     string
	Kind        fieldKind
	Required    bool
	Optional    bool
	Computed    bool
	Sensitive   bool
	ForceNew    bool
}

func resourceSchemaAttributes(spec ResourceSpec, document *discovery.Document) (map[string]resourceschema.Attribute, map[string]fieldDefinition, error) {
	apiSchema, err := document.Schema(spec.SchemaRef)
	if err != nil {
		return nil, nil, err
	}
	properties := make(map[string]*discovery.Property, len(apiSchema.Properties))
	responseProperties := make(map[string]struct{}, len(apiSchema.Properties))
	for name, property := range apiSchema.Properties {
		properties[name] = property
		responseProperties[name] = struct{}{}
	}
	for _, methodID := range []string{spec.CreateMethod, spec.UpdateMethod} {
		if methodID == "" {
			continue
		}
		method, currentErr := document.Method(methodID)
		if currentErr != nil {
			return nil, nil, currentErr
		}
		if method.Request == nil || method.Request.Ref == "" {
			continue
		}
		requestSchema, currentErr := document.Schema(method.Request.Ref)
		if currentErr != nil {
			return nil, nil, currentErr
		}
		for name, property := range requestSchema.Properties {
			if _, exists := properties[name]; !exists {
				properties[name] = property
			}
		}
	}
	definitions := map[string]fieldDefinition{
		"id": {
			APIName: "id", Terraform: "id", Kind: fieldString, Computed: true,
			Description: "Canonical API-relative resource path.",
		},
	}
	if spec.AdoptExisting {
		definitions["path"] = fieldDefinition{
			APIName: "path", Terraform: "path", Kind: fieldString, Required: true, ForceNew: true,
			Description: "Existing GTM Account API-relative path, for example accounts/123456.",
		}
	} else {
		definitions["parent"] = fieldDefinition{
			APIName: "parent", Terraform: "parent", Kind: fieldString, Required: true, ForceNew: true,
			Description: "Canonical API-relative path of the parent resource.",
		}
		definitions["path"] = fieldDefinition{
			APIName: "path", Terraform: "path", Kind: fieldString, Computed: true,
			Description: "Canonical API-relative resource path.",
		}
	}

	required := stringSet(spec.RequiredFields)
	computed := stringSet(spec.ComputedFields)
	propertyNames := sortedPropertyNames(properties)
	for _, apiName := range propertyNames {
		if apiName == "path" {
			continue
		}
		property := properties[apiName]
		definition := propertyDefinition(apiName, property)
		definition.Computed = isComputedResourceField(spec, apiName, computed)
		definition.Required = required[apiName]
		definition.Optional = !definition.Computed && !definition.Required
		definition.ForceNew = spec.UpdateMethod == "" && !definition.Computed
		if _, returnedByAPI := responseProperties[apiName]; definition.Optional && returnedByAPI {
			definition.Computed = true
		}
		definitions[definition.Terraform] = definition
	}

	attributes := make(map[string]resourceschema.Attribute, len(definitions))
	for name, definition := range definitions {
		attributes[name] = resourceAttribute(definition)
	}
	return attributes, definitions, nil
}

func dataSourceSchemaAttributes(method *discovery.Method, document *discovery.Document) (map[string]datasourceschema.Attribute, map[string]fieldDefinition, error) {
	definitions := make(map[string]fieldDefinition)
	for _, name := range sortedParameterNames(method.Parameters) {
		parameter := method.Parameters[name]
		definition := parameterDefinition(name, parameter)
		definition.Required = parameter.Required
		definition.Optional = !parameter.Required
		definitions[definition.Terraform] = definition
	}

	if method.Response != nil && method.Response.Ref != "" {
		responseSchema, err := document.Schema(method.Response.Ref)
		if err != nil {
			return nil, nil, err
		}
		for _, apiName := range sortedPropertyNames(responseSchema.Properties) {
			definition := propertyDefinition(apiName, responseSchema.Properties[apiName])
			if existing, ok := definitions[definition.Terraform]; ok {
				existing.Description += " The response value is returned in the same attribute."
				definitions[definition.Terraform] = existing
				continue
			}
			definition.Computed = true
			definitions[definition.Terraform] = definition
		}
	}
	if _, ok := method.Parameters["pageToken"]; ok {
		definitions["all_pages"] = fieldDefinition{
			APIName: "allPages", Terraform: "all_pages", Kind: fieldBool, Optional: true,
			Description: "Follow next_page_token until every page has been retrieved. Defaults to true.",
		}
	}

	attributes := make(map[string]datasourceschema.Attribute, len(definitions))
	for name, definition := range definitions {
		attributes[name] = dataSourceAttribute(definition)
	}
	return attributes, definitions, nil
}

func actionSchemaAttributes(method *discovery.Method, document *discovery.Document) (map[string]actionschema.Attribute, map[string]fieldDefinition, error) {
	definitions := make(map[string]fieldDefinition)
	for _, name := range sortedParameterNames(method.Parameters) {
		parameter := method.Parameters[name]
		definition := parameterDefinition(name, parameter)
		definition.Required = parameter.Required
		definition.Optional = !parameter.Required
		definitions[definition.Terraform] = definition
	}
	if method.Request != nil && method.Request.Ref != "" {
		requestSchema, err := document.Schema(method.Request.Ref)
		if err != nil {
			return nil, nil, err
		}
		for _, apiName := range sortedPropertyNames(requestSchema.Properties) {
			definition := propertyDefinition(apiName, requestSchema.Properties[apiName])
			if _, exists := definitions[definition.Terraform]; exists {
				continue
			}
			definition.Optional = true
			definitions[definition.Terraform] = definition
		}
		definitions["request_json"] = fieldDefinition{
			APIName: "requestJson", Terraform: "request_json", Kind: fieldJSON, Optional: true, Sensitive: true,
			Description: "Complete JSON request body. Typed request attributes override fields with the same JSON name.",
		}
	}

	attributes := make(map[string]actionschema.Attribute, len(definitions))
	for name, definition := range definitions {
		attributes[name] = actionAttribute(definition)
	}
	return attributes, definitions, nil
}

func propertyDefinition(apiName string, property *discovery.Property) fieldDefinition {
	kind := propertyKind(property)
	terraformName := snakeCase(apiName)
	if kind == fieldJSON {
		terraformName += "_json"
	}
	return fieldDefinition{
		APIName:     apiName,
		Terraform:   terraformName,
		Description: property.Description,
		Enum:        append([]string(nil), property.Enum...),
		Kind:        kind,
		Sensitive:   kind == fieldJSON || isSensitiveField(terraformName),
	}
}

func parameterDefinition(apiName string, parameter *discovery.Parameter) fieldDefinition {
	kind := scalarKind(parameter.Type, parameter.Format)
	if parameter.Repeated {
		kind = listKind(kind)
	}
	return fieldDefinition{
		APIName:     apiName,
		Terraform:   snakeCase(apiName),
		Description: parameter.Description,
		Enum:        append([]string(nil), parameter.Enum...),
		Pattern:     parameter.Pattern,
		Kind:        kind,
		Sensitive:   isSensitiveField(snakeCase(apiName)),
	}
}

func propertyKind(property *discovery.Property) fieldKind {
	if property.Ref != "" || property.Type == "object" || property.AdditionalProperties != nil || len(property.Properties) > 0 {
		return fieldJSON
	}
	if property.Type == "array" {
		if property.Items == nil || property.Items.Ref != "" || property.Items.Type == "object" || property.Items.Type == "array" {
			return fieldJSON
		}
		return listKind(scalarKind(property.Items.Type, property.Items.Format))
	}
	return scalarKind(property.Type, property.Format)
}

func scalarKind(valueType, format string) fieldKind {
	switch valueType {
	case "boolean":
		return fieldBool
	case "integer":
		return fieldInt64
	case "number":
		return fieldFloat64
	default:
		_ = format
		return fieldString
	}
}

func listKind(kind fieldKind) fieldKind {
	switch kind {
	case fieldBool:
		return fieldBoolList
	case fieldInt64:
		return fieldInt64List
	case fieldFloat64:
		return fieldFloat64List
	default:
		return fieldStringList
	}
}

func resourceAttribute(definition fieldDefinition) resourceschema.Attribute {
	stringValidators := stringValidators(definition)
	switch definition.Kind {
	case fieldBool:
		attribute := resourceschema.BoolAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description}
		if definition.ForceNew {
			attribute.PlanModifiers = []planmodifier.Bool{boolplanmodifier.RequiresReplaceIfConfigured()}
		}
		return attribute
	case fieldInt64:
		attribute := resourceschema.Int64Attribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description}
		if definition.ForceNew {
			attribute.PlanModifiers = []planmodifier.Int64{int64planmodifier.RequiresReplaceIfConfigured()}
		}
		return attribute
	case fieldFloat64:
		attribute := resourceschema.Float64Attribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description}
		if definition.ForceNew {
			attribute.PlanModifiers = []planmodifier.Float64{float64planmodifier.RequiresReplaceIfConfigured()}
		}
		return attribute
	case fieldStringList:
		attribute := resourceschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, ElementType: types.StringType}
		if definition.ForceNew {
			attribute.PlanModifiers = []planmodifier.List{listplanmodifier.RequiresReplaceIfConfigured()}
		}
		return attribute
	case fieldBoolList:
		attribute := resourceschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, ElementType: types.BoolType}
		if definition.ForceNew {
			attribute.PlanModifiers = []planmodifier.List{listplanmodifier.RequiresReplaceIfConfigured()}
		}
		return attribute
	case fieldInt64List:
		attribute := resourceschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, ElementType: types.Int64Type}
		if definition.ForceNew {
			attribute.PlanModifiers = []planmodifier.List{listplanmodifier.RequiresReplaceIfConfigured()}
		}
		return attribute
	case fieldFloat64List:
		attribute := resourceschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, ElementType: types.Float64Type}
		if definition.ForceNew {
			attribute.PlanModifiers = []planmodifier.List{listplanmodifier.RequiresReplaceIfConfigured()}
		}
		return attribute
	case fieldJSON:
		stringValidators = append(stringValidators, jsonValidator{})
		attribute := resourceschema.StringAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, Validators: stringValidators}
		if definition.ForceNew {
			attribute.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}
		}
		return attribute
	default:
		attribute := resourceschema.StringAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, Validators: stringValidators}
		if definition.ForceNew {
			attribute.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}
		}
		return attribute
	}
}

func dataSourceAttribute(definition fieldDefinition) datasourceschema.Attribute {
	validators := stringValidators(definition)
	switch definition.Kind {
	case fieldBool:
		return datasourceschema.BoolAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description}
	case fieldInt64:
		return datasourceschema.Int64Attribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description}
	case fieldFloat64:
		return datasourceschema.Float64Attribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description}
	case fieldStringList:
		return datasourceschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, ElementType: types.StringType}
	case fieldBoolList:
		return datasourceschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, ElementType: types.BoolType}
	case fieldInt64List:
		return datasourceschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, ElementType: types.Int64Type}
	case fieldFloat64List:
		return datasourceschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, ElementType: types.Float64Type}
	case fieldJSON:
		validators = append(validators, jsonValidator{})
		return datasourceschema.StringAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, Validators: validators}
	default:
		return datasourceschema.StringAttribute{Required: definition.Required, Optional: definition.Optional, Computed: definition.Computed, Sensitive: definition.Sensitive, Description: definition.Description, Validators: validators}
	}
}

func actionAttribute(definition fieldDefinition) actionschema.Attribute {
	validators := stringValidators(definition)
	switch definition.Kind {
	case fieldBool:
		return actionschema.BoolAttribute{Required: definition.Required, Optional: definition.Optional, WriteOnly: definition.Sensitive, Description: definition.Description}
	case fieldInt64:
		return actionschema.Int64Attribute{Required: definition.Required, Optional: definition.Optional, WriteOnly: definition.Sensitive, Description: definition.Description}
	case fieldFloat64:
		return actionschema.Float64Attribute{Required: definition.Required, Optional: definition.Optional, WriteOnly: definition.Sensitive, Description: definition.Description}
	case fieldStringList:
		return actionschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, WriteOnly: definition.Sensitive, Description: definition.Description, ElementType: types.StringType}
	case fieldBoolList:
		return actionschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, WriteOnly: definition.Sensitive, Description: definition.Description, ElementType: types.BoolType}
	case fieldInt64List:
		return actionschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, WriteOnly: definition.Sensitive, Description: definition.Description, ElementType: types.Int64Type}
	case fieldFloat64List:
		return actionschema.ListAttribute{Required: definition.Required, Optional: definition.Optional, WriteOnly: definition.Sensitive, Description: definition.Description, ElementType: types.Float64Type}
	case fieldJSON:
		validators = append(validators, jsonValidator{})
		return actionschema.StringAttribute{Required: definition.Required, Optional: definition.Optional, WriteOnly: definition.Sensitive, Description: definition.Description, Validators: validators}
	default:
		return actionschema.StringAttribute{Required: definition.Required, Optional: definition.Optional, WriteOnly: definition.Sensitive, Description: definition.Description, Validators: validators}
	}
}

func stringValidators(definition fieldDefinition) []validator.String {
	result := make([]validator.String, 0, 2)
	if len(definition.Enum) > 0 {
		result = append(result, stringvalidator.OneOf(definition.Enum...))
	}
	if definition.Pattern != "" {
		if pattern, err := regexp.Compile(definition.Pattern); err == nil {
			result = append(result, stringvalidator.RegexMatches(pattern, "value must match the GTM API path format"))
		}
	}
	return result
}

type jsonValidator struct{}

func (jsonValidator) Description(context.Context) string {
	return "value must be valid JSON"
}

func (jsonValidator) MarkdownDescription(context.Context) string {
	return "Value must be valid JSON."
}

func (jsonValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	var value any
	if err := json.Unmarshal([]byte(request.ConfigValue.ValueString()), &value); err != nil {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid JSON", err.Error())
	}
}

func isComputedResourceField(spec ResourceSpec, name string, extra map[string]bool) bool {
	if extra[name] || name == spec.IDField {
		return true
	}
	switch name {
	case "accountId", "containerId", "fingerprint", "path", "tagManagerUrl":
		return true
	case "workspaceId":
		return spec.Collection != "environments"
	default:
		return false
	}
}

func isSensitiveField(name string) bool {
	for _, fragment := range []string{"authorization_code", "container_config", "snippet", "access_token", "credentials"} {
		if strings.Contains(name, fragment) {
			return true
		}
	}
	return false
}

func sortedPropertyNames(properties map[string]*discovery.Property) []string {
	result := make([]string, 0, len(properties))
	for name := range properties {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func sortedParameterNames(parameters map[string]*discovery.Parameter) []string {
	result := make([]string, 0, len(parameters))
	for name := range parameters {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func snakeCase(value string) string {
	var result strings.Builder
	for index, character := range value {
		if unicode.IsUpper(character) {
			if index > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(unicode.ToLower(character))
			continue
		}
		result.WriteRune(character)
	}
	return result.String()
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func canonicalJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode canonical JSON: %w", err)
	}
	return string(encoded), nil
}

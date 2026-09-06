package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type attributeGetter interface {
	GetAttribute(context.Context, path.Path, any) diag.Diagnostics
}

type attributeSetter interface {
	SetAttribute(context.Context, path.Path, any) diag.Diagnostics
}

type attributeState interface {
	attributeGetter
	attributeSetter
}

func readFieldValues(ctx context.Context, getter attributeGetter, definitions map[string]fieldDefinition) (map[string]any, diag.Diagnostics) {
	values := make(map[string]any)
	var diagnostics diag.Diagnostics
	for _, definition := range definitions {
		value, present, currentDiagnostics := readFieldValue(ctx, getter, definition)
		diagnostics.Append(currentDiagnostics...)
		if present {
			values[definition.APIName] = value
		}
	}
	return values, diagnostics
}

func readFieldValue(ctx context.Context, getter attributeGetter, definition fieldDefinition) (any, bool, diag.Diagnostics) {
	attributePath := path.Root(definition.Terraform)
	var diagnostics diag.Diagnostics
	switch definition.Kind {
	case fieldBool:
		var value types.Bool
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		if value.IsNull() || value.IsUnknown() {
			return nil, false, diagnostics
		}
		return value.ValueBool(), true, diagnostics
	case fieldInt64:
		var value types.Int64
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		if value.IsNull() || value.IsUnknown() {
			return nil, false, diagnostics
		}
		return value.ValueInt64(), true, diagnostics
	case fieldFloat64:
		var value types.Float64
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		if value.IsNull() || value.IsUnknown() {
			return nil, false, diagnostics
		}
		return value.ValueFloat64(), true, diagnostics
	case fieldStringList:
		var value types.List
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		if value.IsNull() || value.IsUnknown() {
			return nil, false, diagnostics
		}
		var entries []string
		diagnostics.Append(value.ElementsAs(ctx, &entries, false)...)
		return entries, true, diagnostics
	case fieldBoolList:
		var value types.List
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		if value.IsNull() || value.IsUnknown() {
			return nil, false, diagnostics
		}
		var entries []bool
		diagnostics.Append(value.ElementsAs(ctx, &entries, false)...)
		return entries, true, diagnostics
	case fieldInt64List:
		var value types.List
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		if value.IsNull() || value.IsUnknown() {
			return nil, false, diagnostics
		}
		var entries []int64
		diagnostics.Append(value.ElementsAs(ctx, &entries, false)...)
		return entries, true, diagnostics
	case fieldFloat64List:
		var value types.List
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		if value.IsNull() || value.IsUnknown() {
			return nil, false, diagnostics
		}
		var entries []float64
		diagnostics.Append(value.ElementsAs(ctx, &entries, false)...)
		return entries, true, diagnostics
	case fieldJSON:
		var value types.String
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
			return nil, false, diagnostics
		}
		var decoded any
		if err := json.Unmarshal([]byte(value.ValueString()), &decoded); err != nil {
			diagnostics.AddAttributeError(attributePath, "Invalid JSON", err.Error())
			return nil, false, diagnostics
		}
		return decoded, true, diagnostics
	default:
		var value types.String
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		if value.IsNull() || value.IsUnknown() {
			return nil, false, diagnostics
		}
		return value.ValueString(), true, diagnostics
	}
}

func writeResponse(ctx context.Context, setter attributeSetter, definitions map[string]fieldDefinition, response map[string]any) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	for _, definition := range definitions {
		value, ok := response[definition.APIName]
		if !ok || value == nil {
			continue
		}
		converted, err := responseFieldValue(definition, value)
		if err != nil {
			diagnostics.AddAttributeError(path.Root(definition.Terraform), "Unable to store GTM API response", err.Error())
			continue
		}
		diagnostics.Append(setter.SetAttribute(ctx, path.Root(definition.Terraform), converted)...)
	}
	return diagnostics
}

func initializeComputedFields(ctx context.Context, state attributeState, definitions map[string]fieldDefinition) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	for _, definition := range definitions {
		if !definition.Computed || definition.Required {
			continue
		}
		unknown, currentDiagnostics := fieldValueIsUnknown(ctx, state, definition)
		diagnostics.Append(currentDiagnostics...)
		if unknown {
			diagnostics.Append(state.SetAttribute(ctx, path.Root(definition.Terraform), nullFieldValue(definition))...)
		}
	}
	return diagnostics
}

func fieldValueIsUnknown(ctx context.Context, getter attributeGetter, definition fieldDefinition) (bool, diag.Diagnostics) {
	attributePath := path.Root(definition.Terraform)
	var diagnostics diag.Diagnostics
	switch definition.Kind {
	case fieldBool:
		var value types.Bool
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		return value.IsUnknown(), diagnostics
	case fieldInt64:
		var value types.Int64
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		return value.IsUnknown(), diagnostics
	case fieldFloat64:
		var value types.Float64
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		return value.IsUnknown(), diagnostics
	case fieldStringList, fieldBoolList, fieldInt64List, fieldFloat64List:
		var value types.List
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		return value.IsUnknown(), diagnostics
	default:
		var value types.String
		diagnostics.Append(getter.GetAttribute(ctx, attributePath, &value)...)
		return value.IsUnknown(), diagnostics
	}
}

func nullFieldValue(definition fieldDefinition) any {
	switch definition.Kind {
	case fieldBool:
		return types.BoolNull()
	case fieldInt64:
		return types.Int64Null()
	case fieldFloat64:
		return types.Float64Null()
	case fieldStringList:
		return types.ListNull(types.StringType)
	case fieldBoolList:
		return types.ListNull(types.BoolType)
	case fieldInt64List:
		return types.ListNull(types.Int64Type)
	case fieldFloat64List:
		return types.ListNull(types.Float64Type)
	default:
		return types.StringNull()
	}
}

func responseFieldValue(definition fieldDefinition, value any) (any, error) {
	switch definition.Kind {
	case fieldJSON:
		return canonicalJSON(value)
	case fieldInt64:
		switch number := value.(type) {
		case float64:
			return int64(number), nil
		case json.Number:
			return number.Int64()
		default:
			return value, nil
		}
	case fieldInt64List:
		entries, ok := value.([]any)
		if !ok {
			return value, nil
		}
		result := make([]int64, 0, len(entries))
		for _, entry := range entries {
			number, ok := entry.(float64)
			if !ok {
				return nil, fmt.Errorf("expected integer list entry, got %T", entry)
			}
			result = append(result, int64(number))
		}
		return result, nil
	case fieldStringList:
		entries, ok := value.([]any)
		if !ok {
			return value, nil
		}
		result := make([]string, 0, len(entries))
		for _, entry := range entries {
			result = append(result, fmt.Sprint(entry))
		}
		return result, nil
	default:
		return value, nil
	}
}

func stringAttribute(ctx context.Context, getter attributeGetter, name string) (string, diag.Diagnostics) {
	var value types.String
	diagnostics := getter.GetAttribute(ctx, path.Root(name), &value)
	if value.IsNull() || value.IsUnknown() {
		return "", diagnostics
	}
	return value.ValueString(), diagnostics
}

func boolAttribute(ctx context.Context, getter attributeGetter, name string, fallback bool) (bool, diag.Diagnostics) {
	var value types.Bool
	diagnostics := getter.GetAttribute(ctx, path.Root(name), &value)
	if value.IsNull() || value.IsUnknown() {
		return fallback, diagnostics
	}
	return value.ValueBool(), diagnostics
}

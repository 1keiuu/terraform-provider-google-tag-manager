package provider

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// Keep the planned or prior spelling when GTM only rewrites template metadata.
// This is resource-specific: data sources must return the actual API value.
func (r *genericResource) writeResourceResponse(ctx context.Context, state attributeState, result map[string]any) diag.Diagnostics {
	if r.spec.TypeName == "gtm_custom_template" {
		if remote, ok := result["templateData"].(string); ok {
			prior, diagnostics := stringAttribute(ctx, state, "template_data")
			if diagnostics.HasError() {
				return diagnostics
			}
			if templateDataEqual(prior, remote) {
				// Do not change the API response map shared with the caller.
				copy := make(map[string]any, len(result))
				for key, value := range result {
					copy[key] = value
				}
				copy["templateData"] = prior
				result = copy
			}
		}
	}
	return writeResponse(ctx, state, r.definitions, result)
}

var templateSectionMarker = regexp.MustCompile(`(?m)^___[A-Z][A-Z0-9_]*___\r?$`)

type templateSection struct {
	name string
	data string
}

func templateDataEqual(left, right string) bool {
	if left == right {
		return true
	}
	leftSections := templateSections(left)
	rightSections := templateSections(right)
	if len(leftSections) != len(rightSections) {
		return false
	}
	for i, section := range leftSections {
		if section != rightSections[i] {
			return false
		}
	}
	return true
}

func templateSections(value string) []templateSection {
	markers := templateSectionMarker.FindAllStringIndex(value, -1)
	if len(markers) == 0 {
		return []templateSection{{data: value}}
	}
	sections := []templateSection{{data: value[:markers[0][0]]}}
	for i, marker := range markers {
		end := len(value)
		if i+1 < len(markers) {
			end = markers[i+1][0]
		}
		name := strings.TrimSuffix(value[marker[0]:marker[1]], "\r")
		if i == 0 && name == "___TERMS_OF_SERVICE___" {
			continue
		}
		data := value[marker[1]:end]
		switch name {
		case "___INFO___", "___TEMPLATE_PARAMETERS___", "___WEB_PERMISSIONS___", "___SERVER_PERMISSIONS___":
			// Decode only JSON sections. Unescaping the whole template would
			// incorrectly equate changes to JavaScript and literal backslashes.
			var decoded any
			decoder := json.NewDecoder(strings.NewReader(data))
			decoder.UseNumber()
			if json.Valid([]byte(data)) && decoder.Decode(&decoded) == nil {
				if canonical, err := json.Marshal(decoded); err == nil {
					data = string(canonical)
				}
			}
		}
		sections = append(sections, templateSection{name: name, data: data})
	}
	return sections
}

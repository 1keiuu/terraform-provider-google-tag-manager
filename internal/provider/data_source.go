package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/1keiuu/terraform-provider-google-tag-manager/internal/discovery"
)

type genericDataSource struct {
	spec        DataSourceSpec
	data        *providerData
	document    *discovery.Document
	method      *discovery.Method
	definitions map[string]fieldDefinition
}

var _ datasource.DataSource = &genericDataSource{}
var _ datasource.DataSourceWithConfigure = &genericDataSource{}

func newGenericDataSource(spec DataSourceSpec) datasource.DataSource {
	return &genericDataSource{spec: spec}
}

func (d *genericDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = d.spec.TypeName
}

func (d *genericDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, response *datasource.SchemaResponse) {
	document, err := discovery.Load(d.spec.Version)
	if err != nil {
		response.Diagnostics.AddError("Unable to load GTM API discovery", err.Error())
		return
	}
	method, err := document.Method(d.spec.Method)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve GTM data source method", err.Error())
		return
	}
	attributes, definitions, err := dataSourceSchemaAttributes(method, document)
	if err != nil {
		response.Diagnostics.AddError("Unable to build data source schema", err.Error())
		return
	}
	d.document = document
	d.method = method
	d.definitions = definitions
	response.Schema = schema.Schema{
		Description: method.Description,
		Attributes:  attributes,
	}
}

func (d *genericDataSource) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	data, ok := request.ProviderData.(*providerData)
	if !ok {
		response.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *providerData, got %T", request.ProviderData))
		return
	}
	d.data = data
	d.document = data.documents[d.spec.Version]
	method, err := d.document.Method(d.spec.Method)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve GTM data source method", err.Error())
		return
	}
	d.method = method
	_, definitions, err := dataSourceSchemaAttributes(method, d.document)
	if err != nil {
		response.Diagnostics.AddError("Unable to configure data source schema", err.Error())
		return
	}
	d.definitions = definitions
}

func (d *genericDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	if d.data == nil || d.data.client == nil || d.document == nil || d.method == nil {
		response.Diagnostics.AddError("Provider is not configured", "Configure the gtm provider before using this data source.")
		return
	}
	response.State.Raw = request.Config.Raw
	response.State.Schema = request.Config.Schema
	response.Diagnostics.Append(initializeComputedFields(ctx, &response.State, d.definitions)...)
	fieldValues, diagnostics := readFieldValues(ctx, request.Config, d.definitions)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	methodValues := make(map[string]any)
	for name := range d.method.Parameters {
		if value, ok := fieldValues[name]; ok {
			methodValues[name] = value
		}
	}
	result, err := d.data.client.Call(ctx, d.document, d.method, methodValues, nil)
	if err != nil {
		response.Diagnostics.AddError("Unable to read GTM data", err.Error())
		return
	}

	if _, paginated := d.method.Parameters["pageToken"]; paginated {
		allPages, currentDiagnostics := boolAttribute(ctx, request.Config, "all_pages", true)
		response.Diagnostics.Append(currentDiagnostics...)
		if allPages {
			result, err = d.readRemainingPages(ctx, methodValues, result)
			if err != nil {
				response.Diagnostics.AddError("Unable to read all GTM result pages", err.Error())
				return
			}
		}
	}
	response.Diagnostics.Append(writeResponse(ctx, &response.State, d.definitions, result)...)
}

func (d *genericDataSource) readRemainingPages(ctx context.Context, values map[string]any, combined map[string]any) (map[string]any, error) {
	seen := make(map[string]struct{})
	for {
		token := fmt.Sprint(combined["nextPageToken"])
		if token == "" || token == "<nil>" {
			combined["nextPageToken"] = ""
			return combined, nil
		}
		if _, duplicate := seen[token]; duplicate {
			return nil, fmt.Errorf("GTM API repeated page token %q", token)
		}
		seen[token] = struct{}{}
		values["pageToken"] = token
		page, err := d.data.client.Call(ctx, d.document, d.method, values, nil)
		if err != nil {
			return nil, err
		}
		mergePage(combined, page)
	}
}

func mergePage(combined, page map[string]any) {
	combined["nextPageToken"] = ""
	for key, value := range page {
		if key == "nextPageToken" {
			combined[key] = value
			continue
		}
		newEntries, isArray := value.([]any)
		if !isArray {
			combined[key] = value
			continue
		}
		existing, _ := combined[key].([]any)
		combined[key] = append(existing, newEntries...)
	}
}

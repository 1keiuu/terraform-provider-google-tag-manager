package discovery

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed v1.json v2.json
var documents embed.FS

type Document struct {
	Name        string                `json:"name"`
	Version     string                `json:"version"`
	Revision    string                `json:"revision"`
	RootURL     string                `json:"rootUrl"`
	ServicePath string                `json:"servicePath"`
	Resources   map[string]*Resource  `json:"resources"`
	Schemas     map[string]*Schema    `json:"schemas"`
	Parameters  map[string]*Parameter `json:"parameters"`

	methods map[string]*Method
}

type Resource struct {
	Methods   map[string]*Method   `json:"methods"`
	Resources map[string]*Resource `json:"resources"`
}

type Method struct {
	Description    string                `json:"description"`
	HTTPMethod     string                `json:"httpMethod"`
	ID             string                `json:"id"`
	Path           string                `json:"path"`
	FlatPath       string                `json:"flatPath"`
	ParameterOrder []string              `json:"parameterOrder"`
	Parameters     map[string]*Parameter `json:"parameters"`
	Request        *SchemaRef            `json:"request"`
	Response       *SchemaRef            `json:"response"`
	Scopes         []string              `json:"scopes"`
}

type Parameter struct {
	Description      string   `json:"description"`
	Enum             []string `json:"enum"`
	Format           string   `json:"format"`
	Location         string   `json:"location"`
	Pattern          string   `json:"pattern"`
	Repeated         bool     `json:"repeated"`
	Required         bool     `json:"required"`
	Type             string   `json:"type"`
	Default          string   `json:"default"`
	Minimum          string   `json:"minimum"`
	Maximum          string   `json:"maximum"`
	MinimumExclusive bool     `json:"minimumExclusive"`
	MaximumExclusive bool     `json:"maximumExclusive"`
}

type SchemaRef struct {
	Ref string `json:"$ref"`
}

type Schema struct {
	Description string               `json:"description"`
	ID          string               `json:"id"`
	Type        string               `json:"type"`
	Properties  map[string]*Property `json:"properties"`
}

type Property struct {
	Description          string               `json:"description"`
	Enum                 []string             `json:"enum"`
	Format               string               `json:"format"`
	Items                *Property            `json:"items"`
	Properties           map[string]*Property `json:"properties"`
	AdditionalProperties *Property            `json:"additionalProperties"`
	Ref                  string               `json:"$ref"`
	Type                 string               `json:"type"`
}

var (
	loadOnce sync.Once
	loaded   map[string]*Document
	loadErr  error
)

func Load(version string) (*Document, error) {
	loadOnce.Do(func() {
		loaded = make(map[string]*Document, 2)
		for _, current := range []string{"v1", "v2"} {
			contents, err := documents.ReadFile(current + ".json")
			if err != nil {
				loadErr = fmt.Errorf("read embedded %s discovery document: %w", current, err)
				return
			}
			var document Document
			if err := json.Unmarshal(contents, &document); err != nil {
				loadErr = fmt.Errorf("decode embedded %s discovery document: %w", current, err)
				return
			}
			document.methods = make(map[string]*Method)
			indexResources(document.Resources, document.methods)
			loaded[current] = &document
		}
	})
	if loadErr != nil {
		return nil, loadErr
	}
	document, ok := loaded[version]
	if !ok {
		return nil, fmt.Errorf("unsupported Tag Manager API version %q", version)
	}
	return document, nil
}

func indexResources(resources map[string]*Resource, methods map[string]*Method) {
	for _, resource := range resources {
		for _, method := range resource.Methods {
			methods[method.ID] = method
		}
		indexResources(resource.Resources, methods)
	}
}

func (d *Document) Method(id string) (*Method, error) {
	if !strings.HasPrefix(id, "tagmanager.") {
		id = "tagmanager." + id
	}
	method, ok := d.methods[id]
	if !ok {
		return nil, fmt.Errorf("method %q is absent from Tag Manager %s discovery", id, d.Version)
	}
	return method, nil
}

func (d *Document) MethodIDs() []string {
	ids := make([]string, 0, len(d.methods))
	for id := range d.methods {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (d *Document) Schema(ref string) (*Schema, error) {
	schema, ok := d.Schemas[ref]
	if !ok {
		return nil, fmt.Errorf("schema %q is absent from Tag Manager %s discovery", ref, d.Version)
	}
	return schema, nil
}

func (d *Document) BaseURL() string {
	return strings.TrimSuffix(d.RootURL, "/") + "/" + strings.TrimPrefix(d.ServicePath, "/")
}

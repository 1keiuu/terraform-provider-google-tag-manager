package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
	htransport "google.golang.org/api/transport/http"

	"github.com/1keiuu/terraform-provider-google-tag-manager/internal/discovery"
)

const defaultEndpoint = "https://tagmanager.googleapis.com/"

const (
	googleOAuthEndpoint  = "https://oauth2.googleapis.com/token"
	googleUniverseDomain = "googleapis.com"
)

var defaultScopes = []string{
	"https://www.googleapis.com/auth/tagmanager.readonly",
	"https://www.googleapis.com/auth/tagmanager.edit.containers",
	"https://www.googleapis.com/auth/tagmanager.delete.containers",
	"https://www.googleapis.com/auth/tagmanager.edit.containerversions",
	"https://www.googleapis.com/auth/tagmanager.publish",
	"https://www.googleapis.com/auth/tagmanager.manage.users",
	"https://www.googleapis.com/auth/tagmanager.manage.accounts",
}

type tagManagerProvider struct {
	version  string
	endpoint string
}

type providerModel struct {
	Credentials                        types.String  `tfsdk:"credentials"`
	AccessToken                        types.String  `tfsdk:"access_token"`
	ImpersonateServiceAccount          types.String  `tfsdk:"impersonate_service_account"`
	ImpersonateServiceAccountDelegates types.List    `tfsdk:"impersonate_service_account_delegates"`
	Scopes                             types.List    `tfsdk:"scopes"`
	RequestTimeout                     types.Int64   `tfsdk:"request_timeout"`
	RequestsPerSecond                  types.Float64 `tfsdk:"requests_per_second"`
	MaxRetries                         types.Int64   `tfsdk:"max_retries"`
}

var _ provider.Provider = &tagManagerProvider{}
var _ provider.ProviderWithActions = &tagManagerProvider{}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &tagManagerProvider{version: version, endpoint: defaultEndpoint}
	}
}

func (p *tagManagerProvider) Metadata(_ context.Context, _ provider.MetadataRequest, response *provider.MetadataResponse) {
	response.TypeName = "gtm"
	response.Version = p.version
}

func (p *tagManagerProvider) Schema(_ context.Context, _ provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "Manage Google Tag Manager accounts, containers, workspaces, entities, versions, and permissions through API v1 and v2.",
		Attributes: map[string]schema.Attribute{
			"credentials": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Inline Google service account or authorized user credential JSON. Application Default Credentials are used when omitted.",
			},
			"access_token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "A short-lived OAuth 2.0 access token.",
			},
			"impersonate_service_account": schema.StringAttribute{
				Optional:    true,
				Description: "Service account email to impersonate.",
			},
			"impersonate_service_account_delegates": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Delegation chain used for service account impersonation.",
			},
			"scopes": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "OAuth scopes. All Tag Manager scopes are requested by default.",
			},
			"request_timeout": schema.Int64Attribute{
				Optional:    true,
				Description: "HTTP request timeout in seconds. Defaults to 60.",
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"requests_per_second": schema.Float64Attribute{
				Optional:    true,
				Description: "Maximum request rate. Defaults to the standard GTM quota of 0.25 requests per second.",
				Validators: []validator.Float64{
					float64validator.AtLeast(0.01),
				},
			},
			"max_retries": schema.Int64Attribute{
				Optional:    true,
				Description: "Maximum retries for rate limiting, quota exhaustion, transient server failures, and transport errors. Defaults to 5.",
				Validators: []validator.Int64{
					int64validator.Between(0, 20),
				},
			},
		},
	}
}

func (p *tagManagerProvider) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	var config providerModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}

	documents := make(map[string]*discovery.Document, 2)
	for _, version := range []string{"v1", "v2"} {
		document, err := discovery.Load(version)
		if err != nil {
			response.Diagnostics.AddError("Unable to load GTM API discovery", err.Error())
			return
		}
		documents[version] = document
	}

	scopes := listStrings(ctx, config.Scopes, &response.Diagnostics)
	if len(scopes) == 0 {
		scopes = append([]string(nil), defaultScopes...)
	}
	credentials := configuredString(config.Credentials, "GOOGLE_CREDENTIALS")
	accessToken := configuredString(config.AccessToken, "GOOGLE_OAUTH_ACCESS_TOKEN")
	impersonatedAccount := configuredString(config.ImpersonateServiceAccount, "GOOGLE_IMPERSONATE_SERVICE_ACCOUNT")
	if credentials != "" && accessToken != "" {
		response.Diagnostics.AddError("Conflicting authentication settings", "Set only one of credentials or access_token.")
		return
	}

	baseOptions := []option.ClientOption{option.WithScopes(scopes...)}
	if credentials != "" {
		credentialOption, err := validatedCredentialOption(credentials)
		if err != nil {
			response.Diagnostics.AddError("Invalid Google credentials", err.Error())
			return
		}
		baseOptions = append(baseOptions, credentialOption)
	}
	if accessToken != "" {
		baseOptions = append(baseOptions, option.WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})))
	}

	if impersonatedAccount != "" {
		delegates := listStrings(ctx, config.ImpersonateServiceAccountDelegates, &response.Diagnostics)
		if response.Diagnostics.HasError() {
			return
		}
		tokenSource, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: impersonatedAccount,
			Scopes:          scopes,
			Delegates:       delegates,
		}, baseOptions...)
		if err != nil {
			response.Diagnostics.AddError("Unable to configure service account impersonation", err.Error())
			return
		}
		baseOptions = []option.ClientOption{option.WithTokenSource(tokenSource)}
	}

	httpClient, _, err := htransport.NewClient(ctx, baseOptions...)
	if err != nil {
		response.Diagnostics.AddError("Unable to configure Google authentication", err.Error())
		return
	}
	timeout := int64Value(config.RequestTimeout, 60)
	httpClient.Timeout = time.Duration(timeout) * time.Second
	requestRate := float64Value(config.RequestsPerSecond, 0.25)
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	data := &providerData{
		client: &apiClient{
			httpClient: httpClient,
			endpoint:   endpoint,
			limiter:    rate.NewLimiter(rate.Limit(requestRate), 1),
			maxRetries: int(int64Value(config.MaxRetries, 5)),
			userAgent:  fmt.Sprintf("terraform-provider-google-tag-manager/%s", p.version),
		},
		documents: documents,
	}
	response.ResourceData = data
	response.DataSourceData = data
	response.ActionData = data
}

func (p *tagManagerProvider) Resources(_ context.Context) []func() resource.Resource {
	specs := resourceSpecs()
	resources := make([]func() resource.Resource, 0, len(specs))
	for index := range specs {
		spec := specs[index]
		resources = append(resources, func() resource.Resource { return newGenericResource(spec) })
	}
	return resources
}

func (p *tagManagerProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	specs := dataSourceSpecs()
	dataSources := make([]func() datasource.DataSource, 0, len(specs))
	for index := range specs {
		spec := specs[index]
		dataSources = append(dataSources, func() datasource.DataSource { return newGenericDataSource(spec) })
	}
	return dataSources
}

func (p *tagManagerProvider) Actions(_ context.Context) []func() action.Action {
	specs := actionSpecs()
	actions := make([]func() action.Action, 0, len(specs))
	for index := range specs {
		spec := specs[index]
		actions = append(actions, func() action.Action { return newGenericAction(spec) })
	}
	return actions
}

func configuredString(value types.String, environmentVariable string) string {
	if !value.IsNull() && !value.IsUnknown() {
		return strings.TrimSpace(value.ValueString())
	}
	return strings.TrimSpace(os.Getenv(environmentVariable))
}

func validatedCredentialOption(value string) (option.ClientOption, error) {
	contents := []byte(strings.TrimSpace(value))
	var metadata struct {
		Type           string `json:"type"`
		TokenURI       string `json:"token_uri"`
		UniverseDomain string `json:"universe_domain"`
	}
	if err := json.Unmarshal(contents, &metadata); err != nil {
		return nil, fmt.Errorf("credentials must be valid inline JSON: %w", err)
	}
	if metadata.UniverseDomain != "" && metadata.UniverseDomain != googleUniverseDomain {
		return nil, fmt.Errorf("credentials universe_domain must be %q", googleUniverseDomain)
	}

	switch metadata.Type {
	case "service_account":
		if metadata.TokenURI != "" && metadata.TokenURI != googleOAuthEndpoint {
			return nil, fmt.Errorf("service account token_uri must be %q", googleOAuthEndpoint)
		}
		return option.WithAuthCredentialsJSON(option.ServiceAccount, contents), nil
	case "authorized_user":
		return option.WithAuthCredentialsJSON(option.AuthorizedUser, contents), nil
	case "":
		return nil, fmt.Errorf("credentials JSON must contain a type field")
	default:
		return nil, fmt.Errorf("credential type %q is not accepted directly; configure it through Application Default Credentials", metadata.Type)
	}
}

func listStrings(ctx context.Context, value types.List, diagnostics *diag.Diagnostics) []string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var result []string
	diagnostics.Append(value.ElementsAs(ctx, &result, false)...)
	return result
}

func int64Value(value types.Int64, fallback int64) int64 {
	if value.IsNull() || value.IsUnknown() {
		return fallback
	}
	return value.ValueInt64()
}

func float64Value(value types.Float64, fallback float64) float64 {
	if value.IsNull() || value.IsUnknown() {
		return fallback
	}
	return value.ValueFloat64()
}

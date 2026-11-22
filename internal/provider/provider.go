package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/providerdata"
	checkresource "github.com/ehsanmsb/terraform-provider-healthchecks/internal/resources/check"
	integrationresource "github.com/ehsanmsb/terraform-provider-healthchecks/internal/resources/integration"
	projectresource "github.com/ehsanmsb/terraform-provider-healthchecks/internal/resources/project"
	projectmemberresource "github.com/ehsanmsb/terraform-provider-healthchecks/internal/resources/projectmember"
)

var _ provider.Provider = (*healthchecksProvider)(nil)

func New() provider.Provider {
	return &healthchecksProvider{}
}

type healthchecksProvider struct{}

type providerModel struct {
	BaseURL            types.String `tfsdk:"base_url"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	APIKey             types.String `tfsdk:"api_key"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
	Timeout            types.String `tfsdk:"timeout"`
}

func (p *healthchecksProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "healthchecks"
}

func (p *healthchecksProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for Healthchecks.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Base URL of the Healthchecks instance.",
			},
			"username": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Healthchecks account email/username for web-session login.",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Healthchecks account password for web-session login.",
			},
			"api_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Read-write API key for one existing Healthchecks project. Use instead of username and password for check management and integration lookups.",
			},
			"insecure_skip_verify": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Skip TLS certificate verification for self-hosted instances.",
			},
			"timeout": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "HTTP timeout, for example `30s`.",
			},
		},
	}
}

func (p *healthchecksProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.BaseURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("base_url"), "Unknown Healthchecks Base URL", "Set the Healthchecks instance URL to a value known before planning.")
		return
	}

	username, password, apiKey := data.Username.ValueString(), data.Password.ValueString(), data.APIKey.ValueString()
	switch {
	case data.Username.IsUnknown() || data.Password.IsUnknown() || data.APIKey.IsUnknown():
		resp.Diagnostics.AddError("Unknown Healthchecks Credentials", "Configure credentials using values known before planning.")
		return
	case apiKey != "" && (username != "" || password != ""):
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Conflicting Healthchecks Credentials", "Use api_key or username and password.")
		return
	case apiKey == "" && (username == "" || password == ""):
		resp.Diagnostics.AddError("Missing Healthchecks Credentials", "Set api_key for Management API access, or set both username and password for web access.")
		return
	}

	timeout := 30 * time.Second
	if !data.Timeout.IsNull() && !data.Timeout.IsUnknown() {
		parsed, err := time.ParseDuration(data.Timeout.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("timeout"), "Invalid Timeout", err.Error())
			return
		}
		timeout = parsed
	}

	httpClient, err := client.New(client.Config{
		BaseURL:            data.BaseURL.ValueString(),
		Username:           username,
		Password:           password,
		APIKey:             apiKey,
		InsecureSkipVerify: !data.InsecureSkipVerify.IsNull() && data.InsecureSkipVerify.ValueBool(),
		Timeout:            timeout,
		UserAgent:          fmt.Sprintf("terraform-provider-healthchecks/%s", "dev"),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Configure Healthchecks Client", err.Error())
		return
	}

	cfg := &providerdata.ConfiguredClient{Client: httpClient}
	resp.DataSourceData = cfg
	resp.ResourceData = cfg
}

func (p *healthchecksProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		projectresource.New,
		checkresource.New,
		integrationresource.New,
		projectmemberresource.New,
	}
}

func (p *healthchecksProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		projectresource.NewDataSource,
		checkresource.NewDataSource,
		integrationresource.NewDataSource,
	}
}

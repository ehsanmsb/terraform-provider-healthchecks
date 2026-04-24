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
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Healthchecks account email/username for web-session login.",
			},
			"password": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Healthchecks account password for web-session login.",
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
		Username:           data.Username.ValueString(),
		Password:           data.Password.ValueString(),
		InsecureSkipVerify: !data.InsecureSkipVerify.IsNull() && data.InsecureSkipVerify.ValueBool(),
		Timeout:            timeout,
		UserAgent:          fmt.Sprintf("terraform-provider-healthchecks/%s", "dev"),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Configure Healthchecks Client", err.Error())
		return
	}

	if err := httpClient.Login(ctx); err != nil {
		resp.Diagnostics.AddError("Unable to Authenticate to Healthchecks", err.Error())
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
	return nil
}

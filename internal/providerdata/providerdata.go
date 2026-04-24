package providerdata

import "github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"

type ConfiguredClient struct {
	Client *client.Client
}

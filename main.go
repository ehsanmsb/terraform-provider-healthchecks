package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/provider"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/ehsanmsb/healthchecks",
	}

	if err := providerserver.Serve(context.Background(), provider.New, opts); err != nil {
		log.Fatal(err)
	}
}

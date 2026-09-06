package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/1keiuu/terraform-provider-google-tag-manager/internal/provider"
)

var version = "dev"

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.24.0 generate --provider-name gtm --rendered-provider-name google-tag-manager

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with debugger support")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/1keiuu/google-tag-manager",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}

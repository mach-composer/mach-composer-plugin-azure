package internal

import (
	"embed"

	"github.com/elliotchance/pie/v2"
	"github.com/flosch/pongo2/v5"
	"github.com/mach-composer/mach-composer-plugin-helpers/helpers"
)

//go:embed templates/*
var templates embed.FS

func renderResources(
	site, env string,
	cfg *SiteConfig,
	g *GlobalConfig,
	endpoints []EndpointConfig) (string, error) {
	templateSet := pongo2.NewSet("", &helpers.EmbedLoader{Content: templates})
	template := pongo2.Must(templateSet.FromFile("main.tf"))

	usedEndpoints := pie.Filter(endpoints, func(e EndpointConfig) bool {
		return e.Active
	})
	usedCustomEndpoints := pie.Filter(usedEndpoints, func(e EndpointConfig) bool {
		return e.URL != ""
	})

	return template.Execute(pongo2.Context{
		"azure":    cfg,
		"global":   g,
		"siteName": site,
		"envName":  env,

		"endpoints":           endpoints,
		"usedEndpoints":       usedEndpoints,
		"usedCustomEndpoints": usedCustomEndpoints,
	})
}

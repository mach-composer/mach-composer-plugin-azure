package internal

import (
	"fmt"
	"os"

	"github.com/creasty/defaults"
	"github.com/elliotchance/pie/v2"
	"github.com/mach-composer/mach-composer-plugin-helpers/helpers"
	"github.com/mach-composer/mach-composer-plugin-sdk/plugin"
	"github.com/mach-composer/mach-composer-plugin-sdk/schema"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
)

func NewPlugin() schema.MachComposerPlugin {
	state := &Plugin{
		provider:         "2.99.0",
		siteConfigs:      map[string]SiteConfig{},
		componentConfigs: map[string]*ComponentConfig{},
		endpointsConfigs: map[string]map[string]EndpointConfig{},
	}

	return plugin.NewPlugin(&schema.PluginSchema{
		Identifier: "azure",

		Configure: state.Configure,
		IsEnabled: state.IsEnabled,

		// Config
		SetRemoteStateBackend:  state.SetRemoteStateBackend,
		SetGlobalConfig:        state.SetGlobalConfig,
		SetSiteConfig:          state.SetSiteConfig,
		SetComponentConfig:     state.SetComponentConfig,
		SetSiteComponentConfig: state.SetSiteComponentConfig,

		// Config endpoints
		SetSiteEndpointsConfig:      state.SetSiteEndpointsConfig,
		SetComponentEndpointsConfig: state.SetComponentEndpointsConfig,

		// Renders
		RenderTerraformStateBackend: state.TerraformRenderStateBackend,
		RenderTerraformProviders:    state.TerraformRenderProviders,
		RenderTerraformResources:    state.TerraformRenderResources,
		RenderTerraformComponent:    state.RenderTerraformComponent,
	})
}

type Plugin struct {
	environment      string
	provider         string
	remoteState      *AzureTFState
	globalConfig     *GlobalConfig
	siteConfigs      map[string]SiteConfig
	componentConfigs map[string]*ComponentConfig
	endpointsConfigs map[string]map[string]EndpointConfig
}

func (p *Plugin) Configure(environment string, provider string) error {
	p.environment = environment
	if provider != "" {
		p.provider = provider
	}
	return nil
}

func (p *Plugin) IsEnabled() bool {
	return len(p.siteConfigs) > 0
}

func (p *Plugin) SetRemoteStateBackend(data map[string]any) error {
	state := &AzureTFState{}
	if err := mapstructure.Decode(data, state); err != nil {
		return err
	}
	if err := defaults.Set(state); err != nil {
		return err
	}
	p.remoteState = state
	return nil
}

func (p *Plugin) SetGlobalConfig(data map[string]any) error {
	if err := mapstructure.Decode(data, &p.globalConfig); err != nil {
		return err
	}
	return nil
}

func (p *Plugin) SetSiteConfig(site string, data map[string]any) error {
	// If data is empty then exit early since we only want to take action when
	// there is data.
	if len(data) == 0 {
		return nil
	}

	cfg := SiteConfig{}
	if err := mapstructure.Decode(data, &cfg); err != nil {
		return err
	}
	cfg.merge(p.globalConfig)

	if cfg.ResourceGroup != "" {
		fmt.Fprintf(
			os.Stderr,
			"WARNING: resource_group on %s is used (%s). "+
				"Make sure it wasn't managed by MACH before otherwise "+
				"the resource group will get deleted.",
			site, cfg.ResourceGroup,
		)
	}

	if cfg.ServicePlans == nil {
		cfg.ServicePlans = make(map[string]AzureServicePlan)
	}

	p.siteConfigs[site] = cfg
	return nil
}

func (p *Plugin) SetSiteComponentConfig(site, component string, data map[string]any) error {
	cfg, ok := p.siteConfigs[site]
	if !ok {
		return nil
	}

	cfg.Components = append(cfg.Components, component)
	p.siteConfigs[site] = cfg
	return nil
}

func (p *Plugin) SetSiteEndpointsConfig(site string, data map[string]any) error {
	configs := map[string]EndpointConfig{}
	for epId, epData := range data {
		cfg := EndpointConfig{}
		if url, ok := epData.(string); ok {
			cfg.URL = url
		} else {
			if mapData, ok := epData.(map[string]any); ok {
				if val, ok := mapData["azure"].(map[string]any); ok {
					logrus.Warnln("Warning: the azure node on the endpoint will be removed. Set the children directly in the endpoint")
					for key, value := range val {
						mapData[key] = value
					}
				}
			}

			if err := mapstructure.Decode(epData, &cfg); err != nil {
				return err
			}
		}

		if err := defaults.Set(&cfg); err != nil {
			return err
		}

		cfg.Key = epId
		configs[epId] = cfg
	}

	p.endpointsConfigs[site] = configs
	return nil
}

func (p *Plugin) SetComponentConfig(component string, data map[string]any) error {
	cfg, ok := p.componentConfigs[component]
	if !ok {
		cfg = &ComponentConfig{}
		p.componentConfigs[component] = cfg
	}
	if err := mapstructure.Decode(data, cfg); err != nil {
		return err
	}
	cfg.Name = component
	cfg.SetDefaults()
	return nil
}

func (p *Plugin) SetComponentEndpointsConfig(component string, endpoints map[string]string) error {
	cfg, ok := p.componentConfigs[component]
	if !ok {
		cfg = &ComponentConfig{}
		p.componentConfigs[component] = cfg
	}
	cfg.Endpoints = endpoints
	return nil
}

func (p *Plugin) TerraformRenderStateBackend(site string) (string, error) {
	templateContext := struct {
		State *AzureTFState
		Site  string
	}{
		State: p.remoteState,
		Site:  site,
	}

	template := `
	backend "azurerm" {
	  resource_group_name  = "{{ .State.ResourceGroup }}"
	  storage_account_name = "{{ .State.StorageAccount }}"
	  container_name       = "{{ .State.ContainerName }}"
	  key                  = "{{ .State.StateFolder}}/{{ .Site }}"
	}
	`
	return helpers.RenderGoTemplate(template, templateContext)
}

func (p *Plugin) TerraformRenderProviders(site string) (string, error) {
	cfg := p.getSiteConfig(site)
	if cfg == nil {
		return "", nil
	}

	result := fmt.Sprintf(`
		azurerm = {
			version = "%s"
		}`, helpers.VersionConstraint(p.provider))
	return result, nil
}

func (p *Plugin) TerraformRenderResources(site string) (string, error) {
	cfg := p.getSiteConfig(site)
	if cfg == nil {
		return "", nil
	}

	siteEndpoints := p.endpointsConfigs[site]
	defaultEndpoint := EndpointConfig{
		Key: "default",
	}

	for _, componentName := range cfg.Components {
		component, ok := p.componentConfigs[componentName]
		if !ok {
			continue
		}

		for internal, external := range component.Endpoints {
			endpointConfig, ok := siteEndpoints[external]
			if !ok {
				if external == "default" {
					endpointConfig = defaultEndpoint
				} else {
					return "", fmt.Errorf("component requires undeclared endpoint: %s", external)
				}
			}

			endpointConfig.Active = true

			sc := SiteComponent{
				InternalName: internal,
				ExternalName: external,
				Component:    component,
			}
			endpointConfig.Components = append(endpointConfig.Components, sc)
			siteEndpoints[external] = endpointConfig
		}
	}

	if _, ok := cfg.ServicePlans["default"]; !ok {
		cfg.ServicePlans["default"] = AzureServicePlan{
			Kind: "FunctionApp",
			Tier: "Dynamic",
			Size: "Y1",
		}
	}

	return renderResources(site, p.environment, cfg, p.globalConfig, pie.Values(siteEndpoints))
}

func (p *Plugin) RenderTerraformComponent(site string, component string) (*schema.ComponentSchema, error) {
	cfg := p.getSiteConfig(site)
	if cfg == nil {
		return nil, nil
	}
	componentCfg := p.getComponentConfig(component)

	result := &schema.ComponentSchema{
		Providers: []string{"azurerm = azurerm"},
	}

	value, err := terraformRenderComponentVars(cfg, componentCfg)
	if err != nil {
		return nil, err
	}
	result.Variables = value

	values, err := terraformRenderComponentDependsOn(cfg, componentCfg)
	if err != nil {
		return nil, err
	}
	result.DependsOn = values
	return result, nil
}

func (p *Plugin) getSiteConfig(site string) *SiteConfig {
	cfg, ok := p.siteConfigs[site]
	if !ok {
		return nil
	}
	cfg.merge(p.globalConfig)
	return &cfg
}

func (p *Plugin) getComponentConfig(name string) *ComponentConfig {
	componentConfig, ok := p.componentConfigs[name]
	if !ok {
		componentConfig = &ComponentConfig{} // TODO
	}
	return componentConfig
}

func terraformRenderComponentVars(cfg *SiteConfig, componentCfg *ComponentConfig) (string, error) {
	endpointNames := map[string]string{}
	for key, value := range componentCfg.Endpoints {
		endpointNames[helpers.Slugify(key)] = helpers.Slugify(value)
	}

	templateContext := struct {
		Config      *SiteConfig
		Component   *ComponentConfig
		ServicePlan string
		Endpoints   map[string]string
	}{
		Config:      cfg,
		Component:   componentCfg,
		ServicePlan: azureServicePlanResourceName(componentCfg.ServicePlan),
		Endpoints:   endpointNames,
	}

	template := `
		### azure related
		azure_short_name              = "{{ .Component.ShortName }}"
		azure_name_prefix             = local.name_prefix
		azure_subscription_id         = local.subscription_id
		azure_tenant_id               = local.tenant_id
		azure_region                  = local.region
		azure_service_object_ids      = local.service_object_ids
		azure_resource_group          = {
			name     = local.resource_group_name
			location = local.resource_group_location
		}

		{{ if .ServicePlan }}
		azure_app_service_plan = {
			id                  = azurerm_app_service_plan.{{ .ServicePlan }}.id
			name                = azurerm_app_service_plan.{{ .ServicePlan }}.name
			resource_group_name = azurerm_app_service_plan.{{ .ServicePlan }}.resource_group_name
		}
		{{ end }}

		{{ if .Config.AlertGroup }}
		azure_monitor_action_group_id = azurerm_monitor_action_group.alert_action_group.id
		{{ end }}

		{{ range $cEndpoint, $sEndpoint := .Endpoints }}
		azure_endpoint_{{ $cEndpoint }} = {
			url = local.endpoint_url_{{ $sEndpoint }}
			frontdoor_id = azurerm_frontdoor.app-service.header_frontdoor_id
		}
		{{ end }}
	`
	return helpers.RenderGoTemplate(template, templateContext)
}

func terraformRenderComponentDependsOn(cfg *SiteConfig, componentCfg *ComponentConfig) ([]string, error) {
	if componentCfg.ServicePlan != "" {
		if componentCfg.ServicePlan == "default" {
			return []string{"azurerm_app_service_plan.functionapps"}, nil
		} else {
			val := fmt.Sprintf("azurerm_app_service_plan.functionapps_%s", componentCfg.ServicePlan)
			return []string{val}, nil
		}
	}
	return []string{}, nil
}

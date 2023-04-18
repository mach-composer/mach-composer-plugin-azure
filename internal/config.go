package internal

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// AzureTFState Azure storage account state backend configuration.
type AzureTFState struct {
	ResourceGroup  string `mapstructure:"resource_group"`
	StorageAccount string `mapstructure:"storage_account"`
	ContainerName  string `mapstructure:"container_name"`
	StateFolder    string `mapstructure:"state_folder"`
}

type GlobalConfig struct {
	TenantID       string `mapstructure:"tenant_id"`
	SubscriptionID string `mapstructure:"subscription_id"`
	Region         string

	Frontdoor        *AzureFrontdoorSettings     `mapstructure:"frontdoor"`
	ResourcesPrefix  string                      `mapstructure:"resources_prefix"`
	ServiceObjectIds map[string]string           `mapstructure:"service_object_ids"`
	ServicePlans     map[string]AzureServicePlan `mapstructure:"service_plans"`
}

// SiteAzureSettings Site-specific Azure settings
type SiteConfig struct {
	Frontdoor  *AzureFrontdoorSettings `mapstructure:"frontdoor"`
	AlertGroup *AzureAlertGroup        `mapstructure:"alert_group"`

	// Can overwrite values from AzureConfig
	ResourceGroup  string `mapstructure:"resource_group"`
	TenantID       string `mapstructure:"tenant_id"`
	SubscriptionID string `mapstructure:"subscription_id"`

	Region           string
	ServiceObjectIds map[string]string           `mapstructure:"service_object_ids"`
	ServicePlans     map[string]AzureServicePlan `mapstructure:"service_plans"`

	Components map[string]SiteComponentConfig
}

func (a *SiteConfig) merge(c *GlobalConfig) {
	if a.Frontdoor == nil {
		a.Frontdoor = c.Frontdoor
	}
	if a.TenantID == "" {
		a.TenantID = c.TenantID
	}
	if a.SubscriptionID == "" {
		a.SubscriptionID = c.SubscriptionID
	}
	if a.Region == "" {
		a.Region = c.Region
	}

	if len(a.ServiceObjectIds) == 0 {
		a.ServiceObjectIds = c.ServiceObjectIds
	}

	for k, v := range c.ServicePlans {
		a.ServicePlans[k] = v
	}
}

func (a *SiteConfig) ShortRegionName() string {
	if val, ok := azureRegionDisplayMapShort[a.Region]; ok {
		return val
	}
	logrus.Fatalf("No short name for region %s", a.Region)
	return ""
}

func (a *SiteConfig) LongRegionName() string {
	if val, ok := azureRegionDisplayMapLon[a.Region]; ok {
		return val
	}
	logrus.Fatalf("No long name for region %s", a.Region)
	return ""
}

type ComponentConfig struct {
	Endpoints map[string]string `mapstructure:"-"`

	Name        string `mapstructure:"-"`
	ServicePlan string `mapstructure:"service_plan"`
	ShortName   string `mapstructure:"short_name"`
}

func (c *ComponentConfig) SetDefaults() {
	if c.ShortName == "" {
		c.ShortName = c.Name
	}
}

type SiteComponentConfig struct {
	ServicePlan string `mapstructure:"service_plan"`

	Name      string           `mapstructure:"-"`
	Component *ComponentConfig `mapstructure:"-"`
}

// getServicePlanResourceName returns the resource name for a Azure app service
// plan.
// The reason to make this conditional is because of backwards compatability;
// existing environments already have a `functionapp` resource. We want to keep
// that intact.
func (sc *SiteComponentConfig) getServicePlanResourceName() string {
	value := "default"
	if sc.ServicePlan != "" {
		value = sc.ServicePlan
	} else if sc.Component != nil && sc.Component.ServicePlan != "" {
		value = sc.Component.ServicePlan
	}

	if value == "default" {
		return "functionapps"
	}
	return fmt.Sprintf("functionapps_%s", value)
}

type SiteComponent struct {
	InternalName string
	ExternalName string
	Component    *ComponentConfig
}

type EndpointConfig struct {
	URL  string `mapstructure:"url"`
	Key  string `mapstructure:"key"`
	Zone string `mapstructure:"zone"`

	SessionAffinityEnabled bool   `mapstructure:"session_affinity_enabled"`
	SessionAffinityTTL     int    `mapstructure:"session_affinity_ttl_seconds"`
	WAFPolicyID            string `mapstructure:"waf_policy_id"`
	InternalName           string `mapstructure:"internal_name"`

	Active     bool            `mapstructure:"-"`
	Components []SiteComponent `mapstructure:"-"`
}

func (e *EndpointConfig) SetDefaults() {
	e.URL = StripProtocol(e.URL)

	if e.Zone == "" && e.URL != "" {
		e.Zone = ZoneFromURL(e.URL)
	}
}

func (e *EndpointConfig) IsRootDomain() bool {
	return e.URL == e.Zone
}

func (e EndpointConfig) Subdomain() string {
	if e.URL == "" {
		return ""
	}

	return SubdomainFromURL(e.URL)
}

type AzureFrontdoorSettings struct {
	DNSResourceGroup string                   `mapstructure:"dns_resource_group"`
	SslKeyVault      *AzureFrontdoorSslConfig `mapstructure:"ssl_key_vault"`

	// Undocumented option to work around some tenacious issues
	// with using Frontdoor in the Azure Terraform provider
	SuppressChanges bool `mapstructure:"suppress_changes"`
}

type AzureFrontdoorSslConfig struct {
	Name          string
	ResourceGroup string `mapstructure:"resource_group"`
	SecretName    string `mapstructure:"secret_name"`
}

type AzureServicePlan struct {
	Kind                   string
	Tier                   string
	Size                   string
	Capacity               int
	DedicatedResourceGroup bool `mapstructure:"dedicated_resource_group"`
	PerSiteScaling         bool `mapstructure:"per_site_scaling"`
}

type AzureAlertGroup struct {
	Name        string
	AlertEmails []string `mapstructure:"alert_emals"`
	WebhookURL  string   `mapstructure:"webhook_url"`
	LogicApp    string   `mapstructure:"logic_app"`
}

func (a *AzureAlertGroup) LogicAppName() string {
	if a.LogicApp != "" {
		parts := strings.Split(a.LogicApp, ".")
		return parts[1]
	}
	return ""
}

func (a *AzureAlertGroup) LogicAppResourceGroup() string {
	if a.LogicApp != "" {
		parts := strings.Split(a.LogicApp, ".")
		return parts[0]
	}
	return ""
}

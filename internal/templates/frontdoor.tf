locals {
  frontdoor_domain            = format("%s-fd.azurefd.net", local.name_prefix)
  frontdoor_domain_identifier = replace(local.frontdoor_domain, ".", "-")
}

{% for endpoint in usedCustomEndpoints %}
data "azurerm_dns_zone" "{{ endpoint.Key }}" {
    name                = "{{ endpoint.Zone }}"
    resource_group_name = "{{ azure.Frontdoor.DNSResourceGroup }}"
}

  {% if endpoint.IsRootDomain() %}
resource "azurerm_dns_a_record" "{{ endpoint.Key }}" {
  name                = "@"
  zone_name           = data.azurerm_dns_zone.{{ endpoint.Key }}.name
  resource_group_name = "{{ azure.Frontdoor.DNSResourceGroup }}"
  ttl                 = 600
  target_resource_id  = azurerm_frontdoor.app-service.id
}
  {% else %}
resource "azurerm_dns_cname_record" "{{ endpoint.Key }}" {
  name                = "{{ endpoint.Subdomain() }}"
  zone_name           = data.azurerm_dns_zone.{{ endpoint.Key }}.name
  resource_group_name = "{{ azure.Frontdoor.DNSResourceGroup }}"
  ttl                 = 600
  record              = local.frontdoor_domain
}
  {% endif %}
{% endfor %}

{% if usedEndpoints %}
locals {
  {% for endpoint in usedCustomEndpoints %}
    {% for sc in endpoint.Components %}
      {% set component = sc.Component %}
      {% set cep_key = sc.InternalName %}
  fd_{{ endpoint.Key }}_{{ component.Name }}_route_defs = lookup(
    module.{{ component.Name }}.azure_endpoint_{{ cep_key }},
    "routes",
    [{
      patterns = ["/{{ component.Name }}/*"]
    }]
  )

  fd_{{ endpoint.Key }}_{{ component.Name }}_routes = {
    for i in range(
      length(
        local.fd_{{ endpoint.Key }}_{{ component.Name }}_route_defs
      )
    ) :
    i => element(
      local.fd_{{ endpoint.Key }}_{{ component.Name }}_route_defs,
      i
    )
  }
    {% endfor %}
  {% endfor %}
}

resource "azurerm_frontdoor" "app-service" {
  name                                          = format("%s-fd", local.name_prefix)
  resource_group_name                           = local.resource_group_name
  tags = local.tags

  backend_pool_settings {
    enforce_backend_pools_certificate_name_check  = false
  }

  backend_pool_load_balancing {
    name = "lbSettings"
  }

  frontend_endpoint {
    name                              = local.frontdoor_domain_identifier
    host_name                         = local.frontdoor_domain
  }

  {% for endpoint in usedCustomEndpoints %}
  frontend_endpoint {
    name                              = "{{ endpoint.InternalName|default:endpoint.Key }}"
    host_name                         = "{{ endpoint.URL }}"
    {% if endpoint.WAFPolicyID %}
    web_application_firewall_policy_link_id = "{{ endpoint.WAFPolicyID }}"
    {% endif %}

    {% if endpoint.SessionAffinityEnabled %}
    session_affinity_enabled = {{ endpoint.SessionAffinityEnabled }}
    session_affinity_ttl_seconds = {{ endpoint.SessionAffinityTTL }}
    {% endif %}

  }
  {% endfor %}

  depends_on = [
    {% for endpoint in usedCustomEndpoints %}
    {% if not endpoint.IsRootDomain() %}
    azurerm_dns_cname_record.{{ endpoint.Key }},
    {% endif %}
    {% endfor %}
  ]

  routing_rule {
    name               = "http-https-redirect"
    accepted_protocols = ["Http"]
    patterns_to_match  = ["/*"]
    frontend_endpoints = [
      local.frontdoor_domain_identifier,
      {% for endpoint in usedCustomEndpoints %}
        "{{ endpoint.InternalName|default:endpoint.Key}}",
      {% endfor %}
    ]
    redirect_configuration {
      redirect_type     = "PermanentRedirect"
      redirect_protocol = "HttpsOnly"
    }
  }

  {% for endpoint in usedCustomEndpoints %}
    {% include "./frontdoor_endpoint.tf" %}
  {% endfor %}

  {% if azure.Frontdoor.SuppressChanges %}
  # Work-around for a very annoying bug in the Azure Frontdoor API
  # causing unwanted changes in Frontdoor and raising errors.
  lifecycle {
    ignore_changes = [
      routing_rule,
      backend_pool,
      backend_pool_health_probe,
      frontend_endpoint,
    ]
  }
  {% endif %}
}

{% if azure.Frontdoor.SSLKeyVault %}
data "azurerm_key_vault" "ssl" {
  name                = "{{ azure.Frontdoor.SSLKeyVault.Name }}"
  resource_group_name = "{{ azure.Frontdoor.SSLKeyVault.ResourceGroup }}"
}
{% endif %}

{% for endpoint in usedCustomEndpoints %}
resource "azurerm_frontdoor_custom_https_configuration" "{{ endpoint.Key|slugify }}" {
  frontend_endpoint_id              = azurerm_frontdoor.app-service.frontend_endpoints["{{ endpoint.InternalName|default:endpoint.Key }}"]
  custom_https_provisioning_enabled = true

  custom_https_configuration {
    {% if azure.Frontdoor.SslKeyVault %}
    certificate_source                         = "AzureKeyVault"
    azure_key_vault_certificate_vault_id       = data.azurerm_key_vault.ssl.id
    azure_key_vault_certificate_secret_name    = "{{ azure.Frontdoor.SSLKeyVault.SecretName }}"
    {% else %}
    certificate_source                      = "FrontDoor"
    {% endif %}
  }
}
{% endfor %}
{% endif %}

package internal

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/mach-composer/mach-composer-plugin-sdk/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type RawConfig struct {
	Global             map[string]any
	RemoteState        map[string]any
	Components         map[string]map[string]any
	ComponentEndpoints map[string]map[string]string
	Sites              map[string]map[string]any
	SiteEndpoints      map[string]map[string]map[string]any
	SiteComponents     map[string]map[string]map[string]any
}

func TestRender(t *testing.T) {
	source, err := ioutil.ReadFile("testdata/full-test-case.json")
	require.NoError(t, err)

	config := RawConfig{}
	err = json.Unmarshal(source, &config)
	require.NoError(t, err)

	plugin := NewAzurePlugin()
	setTestData(t, plugin, config)
	t.Run("state-backend", func(t *testing.T) {
		result, err := plugin.RenderTerraformStateBackend("my-site")
		require.NoError(t, err)
		cupaloy.SnapshotT(t, result)
	})

	t.Run("providers", func(t *testing.T) {
		result, err := plugin.RenderTerraformProviders("my-site")
		require.NoError(t, err)
		cupaloy.SnapshotT(t, result)
	})
	t.Run("resources", func(t *testing.T) {
		result, err := plugin.RenderTerraformResources("my-site")
		require.NoError(t, err)
		cupaloy.SnapshotT(t, result)
	})
	t.Run("component-payment", func(t *testing.T) {
		result, err := plugin.RenderTerraformComponent("my-site", "payment")
		require.NotNil(t, result)
		require.NoError(t, err)
		assert.NoError(t, cupaloy.SnapshotMulti("component-payment-dependson", result.DependsOn))
		assert.NoError(t, cupaloy.SnapshotMulti("component-payment-providers", result.Providers))
		assert.NoError(t, cupaloy.SnapshotMulti("component-payment-resources", result.Resources))
		assert.NoError(t, cupaloy.SnapshotMulti("component-payment-vars", result.Variables))
	})
	t.Run("component-api-extensions", func(t *testing.T) {
		result, err := plugin.RenderTerraformComponent("my-site", "api-extensions")
		require.NotNil(t, result)
		require.NoError(t, err)
		assert.NoError(t, cupaloy.SnapshotMulti("component-api-ext-dependson", result.DependsOn))
		assert.NoError(t, cupaloy.SnapshotMulti("component-api-ext-providers", result.Providers))
		assert.NoError(t, cupaloy.SnapshotMulti("component-api-ext-resources", result.Resources))
		assert.NoError(t, cupaloy.SnapshotMulti("component-api-ext-vars", result.Variables))
	})
}

func TestRenderNoData(t *testing.T) {
	source, err := ioutil.ReadFile("testdata/empty-test-case.json")
	require.NoError(t, err)

	config := RawConfig{}
	err = json.Unmarshal(source, &config)
	require.NoError(t, err)

	plugin := NewAzurePlugin()
	setTestData(t, plugin, config)
	t.Run("state-backend", func(t *testing.T) {
		result, err := plugin.RenderTerraformStateBackend("my-site")
		require.NoError(t, err)
		cupaloy.SnapshotT(t, result)
	})

	t.Run("providers", func(t *testing.T) {
		result, err := plugin.RenderTerraformProviders("my-site")
		require.NoError(t, err)
		cupaloy.SnapshotT(t, result)
	})
	t.Run("resources", func(t *testing.T) {
		result, err := plugin.RenderTerraformResources("my-site")
		require.NoError(t, err)
		cupaloy.SnapshotT(t, result)
	})
	t.Run("component-payment", func(t *testing.T) {
		result, err := plugin.RenderTerraformComponent("my-site", "payment")
		require.NoError(t, err)
		require.Nil(t, result)
	})
	t.Run("component-api-extensions", func(t *testing.T) {
		result, err := plugin.RenderTerraformComponent("my-site", "api-extensions")
		require.NoError(t, err)
		require.Nil(t, result)
	})
}

func setTestData(t *testing.T, plugin schema.MachComposerPlugin, config RawConfig) {
	err := plugin.SetGlobalConfig(config.Global)
	require.NoError(t, err)

	if config.RemoteState != nil {
		err := plugin.SetRemoteStateBackend(config.RemoteState)
		require.NoError(t, err)
	}
	if config.Components != nil {
		for name, c := range config.Components {
			err := plugin.SetComponentConfig(name, c)
			require.NoError(t, err)
		}
	}
	if config.ComponentEndpoints != nil {
		for name, c := range config.ComponentEndpoints {
			err := plugin.SetComponentEndpointsConfig(name, c)
			require.NoError(t, err)
		}
	}
	if config.Sites != nil {
		for name, s := range config.Sites {
			err := plugin.SetSiteConfig(name, s)
			require.NoError(t, err)
		}
	}
	if config.SiteEndpoints != nil {
		for site, e := range config.SiteEndpoints {
			for name, data := range e {
				err := plugin.SetSiteEndpointConfig(site, name, data)
				require.NoError(t, err)
			}
		}
	}
	if config.SiteComponents != nil {
		for site, sc := range config.SiteComponents {
			for c, data := range sc {
				err := plugin.SetSiteComponentConfig(site, c, data)
				require.NoError(t, err)
			}
		}
	}
}

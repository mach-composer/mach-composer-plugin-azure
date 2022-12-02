package azure

import (
	"github.com/mach-composer/mach-composer-plugin-sdk/plugin"

	"github.com/mach-composer/mach-composer-plugin-azure/internal"
)

func Serve() {
	p := internal.NewAzurePlugin()
	plugin.ServePlugin(p)
}

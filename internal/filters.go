package internal

import (
	"errors"
	"strings"

	"github.com/flosch/pongo2/v5"
	"github.com/mach-composer/mach-composer-plugin-helpers/helpers"
)

func init() {
	helpers.MustRegisterFilter("service_plan_resource_name", AzureServicePlanResourceName)
	helpers.MustRegisterFilter("short_prefix", filterShortPrefix)
	helpers.MustRegisterFilter("remove", filterRemove)
}

// AzureServicePlanResourceName Retrieve the resource name for a Azure app service plan.
// The reason to make this conditional is because of backwards compatability;
// existing environments already have a `functionapp` resource. We want to keep that intact.
func AzureServicePlanResourceName(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	val := azureServicePlanResourceName(in.String())
	return pongo2.AsSafeValue(val), nil
}

// Specific function created to be backwards compatible with Python version
// It replaces env names with 1 letter codes.
// TODO: Research why/if this is still needed
func filterShortPrefix(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	if !in.IsString() {
		return nil, &pongo2.Error{
			Sender:    "filter:short_string",
			OrigError: errors.New("filter only applicable on strings"),
		}
	}

	val := in.String()
	val = strings.Replace(val, "dev", "d", -1)
	val = strings.Replace(val, "tst", "t", -1)
	val = strings.Replace(val, "acc", "a", -1)
	val = strings.Replace(val, "prd", "p", -1)
	return pongo2.AsValue(val), nil
}

func filterRemove(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	if !in.IsString() {
		return nil, &pongo2.Error{
			Sender:    "filter:remove",
			OrigError: errors.New("filter only applicable on strings"),
		}
	}
	if !param.IsString() {
		return nil, &pongo2.Error{
			Sender:    "filter:remove",
			OrigError: errors.New("filter requires a param"),
		}
	}

	output := strings.Replace(in.String(), param.String(), "", -1)
	return pongo2.AsValue(output), nil
}

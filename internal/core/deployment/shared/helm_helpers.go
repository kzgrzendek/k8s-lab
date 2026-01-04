package shared

import (
	"context"
)

// DeployWithProgress deploys a Helm chart with optional dynamic values.
// The dynamicValuesFn parameter is optional and generates runtime values.
func DeployWithProgress(ctx context.Context, opts HelmDeploymentOptions,
	dynamicValuesFn func(context.Context) (map[string]interface{}, error)) error {

	if dynamicValuesFn != nil {
		dynamicValues, err := dynamicValuesFn(ctx)
		if err != nil {
			return err
		}
		if opts.Values == nil {
			opts.Values = dynamicValues
		} else {
			// Merge dynamic values with existing values
			for k, v := range dynamicValues {
				opts.Values[k] = v
			}
		}
	}

	return DeployHelmChart(ctx, opts)
}

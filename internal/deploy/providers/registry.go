package providers

import "fmt"

// Get returns the DeployProvider for the named provider.
func Get(providerName string) (DeployProvider, error) {
	switch providerName {
	case "vps":
		return &VPSProvider{}, nil
	case "dokploy":
		return &DokployProvider{}, nil
	case "coolify":
		return &CoolifyProvider{}, nil
	case "railway":
		return &RailwayProvider{}, nil
	case "render":
		return &RenderProvider{}, nil
	case "fly":
		return &FlyProvider{}, nil
	default:
		return nil, fmt.Errorf(
			"unknown provider %q\nSupported providers: vps, dokploy, coolify, railway, render, fly",
			providerName,
		)
	}
}

package plugin

import (
	"fmt"
	"strings"

	"github.com/opencode/plugin-cli/internal/marketplace"
)

type DependencyResolutionResult struct {
	Closure []string
}

func ResolveDependencyClosure(
	root string,
	lookup func(string) (*marketplace.ResolvedPlugin, error),
	alreadyInstalled map[string]bool,
	allowedCrossMarketplaces map[string]bool,
) (*DependencyResolutionResult, error) {
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var order []string

	var resolve func(id string) error

	resolve = func(id string) error {
		if inStack[id] {
			return fmt.Errorf("dependency cycle detected: %s", id)
		}
		if visited[id] {
			return nil
		}

		inStack[id] = true

		resolved, err := lookup(id)
		if err != nil {
			return fmt.Errorf("failed to resolve dependency %s: %w", id, err)
		}

		rootParts := strings.SplitN(root, "@", 2)
		rootMarket := ""
		if len(rootParts) == 2 {
			rootMarket = rootParts[1]
		}

		for _, dep := range resolved.Plugin.Dependencies {
			depID := qualifyDepID(dep, resolved.MarketName)

			depParts := strings.SplitN(depID, "@", 2)
			depMarket := ""
			if len(depParts) == 2 {
				depMarket = depParts[1]
			}

			if depMarket != "" && depMarket != rootMarket {
				if allowedCrossMarketplaces == nil || !allowedCrossMarketplaces[depMarket] {
					return fmt.Errorf("cross-marketplace dependency %s not allowed (root marketplace %s does not allow deps from %s)", depID, rootMarket, depMarket)
				}
			}

			if alreadyInstalled != nil && alreadyInstalled[depID] {
				continue
			}

			if err := resolve(depID); err != nil {
				return err
			}
		}

		inStack[id] = false
		visited[id] = true
		order = append(order, id)

		return nil
	}

	if err := resolve(root); err != nil {
		return nil, err
	}

	return &DependencyResolutionResult{Closure: order}, nil
}

func qualifyDepID(dep, marketName string) string {
	if strings.Contains(dep, "@") {
		return dep
	}
	return dep + "@" + marketName
}

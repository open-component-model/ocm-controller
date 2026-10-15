package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

func parseControllerConcurrency(value string) (map[string]int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	kinds := map[string]string{
		"componentVersion": "ComponentVersion",
		"resource":         "Resource",
		"snapshot":         "Snapshot",
		"localization":     "Localization",
		"configuration":    "Configuration",
		"fluxDeployer":     "FluxDeployer",
	}
	concurrency := make(map[string]int)
	for _, entry := range strings.Split(value, ",") {
		name, count, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if !ok {
			return nil, fmt.Errorf("expected controller=count, got %q", entry)
		}
		name = strings.TrimSpace(name)
		kind, ok := kinds[name]
		if !ok {
			return nil, fmt.Errorf("unknown controller %q", name)
		}
		key := v1alpha1.GroupVersion.WithKind(kind).GroupKind().String()
		if _, exists := concurrency[key]; exists {
			return nil, fmt.Errorf("duplicate controller %q", name)
		}
		workers, err := strconv.Atoi(strings.TrimSpace(count))
		if err != nil || workers < 1 {
			return nil, fmt.Errorf("concurrency for controller %q must be a positive integer, got %q", name, count)
		}
		concurrency[key] = workers
	}

	return concurrency, nil
}

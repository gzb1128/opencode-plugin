package plugin

import (
	"fmt"
	"strings"
	"testing"

	"github.com/opencode/plugin-cli/internal/marketplace"
)

func TestResolveDependencyClosure_BareDependency(t *testing.T) {
	plugins := map[string]*marketplace.ResolvedPlugin{
		"root@main": {
			Plugin:     &marketplace.Plugin{Name: "root", Dependencies: []string{"dep"}},
			MarketName: "main",
		},
		"dep@main": {
			Plugin:     &marketplace.Plugin{Name: "dep"},
			MarketName: "main",
		},
	}

	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf("not found: %s", id)
		}
		return p, nil
	}

	result, err := ResolveDependencyClosure("root@main", lookup, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Closure) != 2 {
		t.Fatalf("expected 2 items in closure, got %d: %v", len(result.Closure), result.Closure)
	}

	if result.Closure[0] != "dep@main" {
		t.Errorf("first item should be dep@main, got %s", result.Closure[0])
	}
	if result.Closure[1] != "root@main" {
		t.Errorf("second item should be root@main, got %s", result.Closure[1])
	}
}

func TestResolveDependencyClosure_AlreadyInstalled(t *testing.T) {
	plugins := map[string]*marketplace.ResolvedPlugin{
		"root@main": {
			Plugin:     &marketplace.Plugin{Name: "root", Dependencies: []string{"dep"}},
			MarketName: "main",
		},
		"dep@main": {
			Plugin:     &marketplace.Plugin{Name: "dep"},
			MarketName: "main",
		},
	}

	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf("not found: %s", id)
		}
		return p, nil
	}

	alreadyInstalled := map[string]bool{"dep@main": true}

	result, err := ResolveDependencyClosure("root@main", lookup, alreadyInstalled, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Closure) != 1 {
		t.Fatalf("expected 1 item (root only), got %d: %v", len(result.Closure), result.Closure)
	}

	if result.Closure[0] != "root@main" {
		t.Errorf("should only contain root, got %s", result.Closure[0])
	}
}

func TestResolveDependencyClosure_RootNeverSkipped(t *testing.T) {
	plugins := map[string]*marketplace.ResolvedPlugin{
		"root@main": {
			Plugin:     &marketplace.Plugin{Name: "root"},
			MarketName: "main",
		},
	}

	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf("not found: %s", id)
		}
		return p, nil
	}

	alreadyInstalled := map[string]bool{"root@main": true}

	result, err := ResolveDependencyClosure("root@main", lookup, alreadyInstalled, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Closure) != 1 {
		t.Fatalf("expected 1 item (root never skipped), got %d: %v", len(result.Closure), result.Closure)
	}

	if result.Closure[0] != "root@main" {
		t.Errorf("should contain root, got %s", result.Closure[0])
	}
}

func TestResolveDependencyClosure_Cycle(t *testing.T) {
	plugins := map[string]*marketplace.ResolvedPlugin{
		"a@main": {
			Plugin:     &marketplace.Plugin{Name: "a", Dependencies: []string{"b"}},
			MarketName: "main",
		},
		"b@main": {
			Plugin:     &marketplace.Plugin{Name: "b", Dependencies: []string{"a"}},
			MarketName: "main",
		},
	}

	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf("not found: %s", id)
		}
		return p, nil
	}

	_, err := ResolveDependencyClosure("a@main", lookup, nil, nil)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestResolveDependencyClosure_CrossMarketplaceBlocked(t *testing.T) {
	plugins := map[string]*marketplace.ResolvedPlugin{
		"root@main": {
			Plugin:     &marketplace.Plugin{Name: "root", Dependencies: []string{"dep@other"}},
			MarketName: "main",
		},
		"dep@other": {
			Plugin:     &marketplace.Plugin{Name: "dep"},
			MarketName: "other",
		},
	}

	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf("not found: %s", id)
		}
		return p, nil
	}

	_, err := ResolveDependencyClosure("root@main", lookup, nil, nil)
	if err == nil {
		t.Fatal("expected cross-marketplace error, got nil")
	}
	if !strings.Contains(err.Error(), "cross-marketplace") {
		t.Errorf("error should mention cross-marketplace, got: %v", err)
	}
}

func TestResolveDependencyClosure_CrossMarketplaceAllowed(t *testing.T) {
	plugins := map[string]*marketplace.ResolvedPlugin{
		"root@main": {
			Plugin:     &marketplace.Plugin{Name: "root", Dependencies: []string{"dep@other"}},
			MarketName: "main",
		},
		"dep@other": {
			Plugin:     &marketplace.Plugin{Name: "dep"},
			MarketName: "other",
		},
	}

	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf("not found: %s", id)
		}
		return p, nil
	}

	allowed := map[string]bool{"other": true}

	result, err := ResolveDependencyClosure("root@main", lookup, nil, allowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Closure) != 2 {
		t.Fatalf("expected 2 items, got %d: %v", len(result.Closure), result.Closure)
	}

	if result.Closure[0] != "dep@other" {
		t.Errorf("first should be dep@other, got %s", result.Closure[0])
	}
	if result.Closure[1] != "root@main" {
		t.Errorf("second should be root@main, got %s", result.Closure[1])
	}
}

func TestResolveDependencyClosure_TransitiveDeps(t *testing.T) {
	plugins := map[string]*marketplace.ResolvedPlugin{
		"a@main": {
			Plugin:     &marketplace.Plugin{Name: "a", Dependencies: []string{"b"}},
			MarketName: "main",
		},
		"b@main": {
			Plugin:     &marketplace.Plugin{Name: "b", Dependencies: []string{"c"}},
			MarketName: "main",
		},
		"c@main": {
			Plugin:     &marketplace.Plugin{Name: "c"},
			MarketName: "main",
		},
	}

	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf("not found: %s", id)
		}
		return p, nil
	}

	result, err := ResolveDependencyClosure("a@main", lookup, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"c@main", "b@main", "a@main"}
	if len(result.Closure) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, result.Closure)
	}
	for i, exp := range expected {
		if result.Closure[i] != exp {
			t.Errorf("closure[%d] = %q, want %q", i, result.Closure[i], exp)
		}
	}
}

func TestResolveDependencyClosure_DiamondDeps(t *testing.T) {
	plugins := map[string]*marketplace.ResolvedPlugin{
		"a@main": {
			Plugin:     &marketplace.Plugin{Name: "a", Dependencies: []string{"b", "c"}},
			MarketName: "main",
		},
		"b@main": {
			Plugin:     &marketplace.Plugin{Name: "b", Dependencies: []string{"d"}},
			MarketName: "main",
		},
		"c@main": {
			Plugin:     &marketplace.Plugin{Name: "c", Dependencies: []string{"d"}},
			MarketName: "main",
		},
		"d@main": {
			Plugin:     &marketplace.Plugin{Name: "d"},
			MarketName: "main",
		},
	}

	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf("not found: %s", id)
		}
		return p, nil
	}

	result, err := ResolveDependencyClosure("a@main", lookup, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	seen := map[string]bool{}
	for _, id := range result.Closure {
		if seen[id] {
			t.Errorf("duplicate id in closure: %s", id)
		}
		seen[id] = true
	}

	if !seen["d@main"] || !seen["b@main"] || !seen["c@main"] || !seen["a@main"] {
		t.Errorf("missing expected items in closure: %v", result.Closure)
	}

	if result.Closure[len(result.Closure)-1] != "a@main" {
		t.Errorf("root should be last, got closure: %v", result.Closure)
	}
}

func TestResolveDependencyClosure_DepLookupFails(t *testing.T) {
	plugins := map[string]*marketplace.ResolvedPlugin{
		"root@main": {
			Plugin:     &marketplace.Plugin{Name: "root", Dependencies: []string{"missing"}},
			MarketName: "main",
		},
	}

	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf("not found: %s", id)
		}
		return p, nil
	}

	_, err := ResolveDependencyClosure("root@main", lookup, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing dep")
	}
}

func TestResolveDependencyClosure_NoDeps(t *testing.T) {
	plugins := map[string]*marketplace.ResolvedPlugin{
		"root@main": {
			Plugin:     &marketplace.Plugin{Name: "root"},
			MarketName: "main",
		},
	}

	lookup := func(id string) (*marketplace.ResolvedPlugin, error) {
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf("not found: %s", id)
		}
		return p, nil
	}

	result, err := ResolveDependencyClosure("root@main", lookup, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Closure) != 1 || result.Closure[0] != "root@main" {
		t.Errorf("expected [root@main], got %v", result.Closure)
	}
}

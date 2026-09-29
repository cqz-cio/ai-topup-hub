package container

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestContainerAssemblyIsSplitByResponsibility(t *testing.T) {
	directory := currentContainerDirectory(t)
	tests := []struct {
		file     string
		required []string
	}{
		{file: "bootstrap.go", required: []string{"func NewContainer(", "func newPaymentProviderRegistry("}},
		{file: "repositories.go", required: []string{"func (c *Container) initRepositories()"}},
		{file: "services.go", required: []string{"func (c *Container) initServices()"}},
		{file: "services_foundation.go", required: []string{
			"func (c *Container) initPolicyAndSettingServices()",
			"func (c *Container) loadRuntimeSettings()",
			"func (c *Container) initIdentityAndCatalogServices()",
		}},
		{file: "services_application.go", required: []string{"func (c *Container) initApplicationServices()"}},
		{file: "services_integration.go", required: []string{"func (c *Container) initIntegrationServices()"}},
		{file: "services_wiring.go", required: []string{"func (c *Container) wireServiceDependencies()"}},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			source := readContainerSource(t, filepath.Join(directory, test.file))
			for _, required := range test.required {
				if !strings.Contains(source, required) {
					t.Errorf("%s must contain %q", test.file, required)
				}
			}
		})
	}

	containerSource := readContainerSource(t, filepath.Join(directory, "container.go"))
	for _, forbidden := range []string{
		"func NewContainer(",
		"func (c *Container) initRepositories()",
		"func (c *Container) initServices()",
	} {
		if strings.Contains(containerSource, forbidden) {
			t.Errorf("container.go must only declare the dependency surface; move %q to its assembly file", forbidden)
		}
	}
}

func TestServiceAssemblyDeclaresDependencyOrder(t *testing.T) {
	directory := currentContainerDirectory(t)
	source := readContainerSource(t, filepath.Join(directory, "services.go"))
	orderedCalls := []string{
		"c.initPolicyAndSettingServices()",
		"c.loadRuntimeSettings()",
		"c.initIdentityAndCatalogServices()",
		"c.initApplicationServices()",
		"c.initIntegrationServices()",
		"c.wireServiceDependencies()",
	}

	previous := -1
	for _, call := range orderedCalls {
		index := strings.Index(source, call)
		if index < 0 {
			t.Fatalf("services.go must call %s", call)
		}
		if index <= previous {
			t.Fatalf("service assembly call %s is out of dependency order", call)
		}
		previous = index
	}
}

func currentContainerDirectory(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve container test filename")
	}
	return filepath.Dir(filename)
}

func readContainerSource(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read container source %s: %v", path, err)
	}
	return string(raw)
}

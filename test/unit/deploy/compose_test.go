package deploy_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestTraefikImageIsVersionAndDigestPinned(t *testing.T) {
	t.Parallel()

	const want = "traefik:v3.6.23@sha256:f5dba1e65167778cd5f8d1b463fc5d200f49d40c6458fc9f4b391a68ebfb9534"
	for _, name := range []string{
		"docker-compose.ghcr.traefik.yaml",
		"docker-compose.traefik.yaml",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			contents, err := os.ReadFile(filepath.Join(repositoryRoot(t), "docker", name))
			if err != nil {
				t.Fatalf("read compose file: %v", err)
			}

			var document struct {
				Services map[string]struct {
					Image string `yaml:"image"`
				} `yaml:"services"`
			}
			if err := yaml.Unmarshal(contents, &document); err != nil {
				t.Fatalf("parse compose file: %v", err)
			}

			if got := document.Services["traefik"].Image; got != want {
				t.Fatalf("Traefik image = %q, want %q", got, want)
			}
		})
	}
}

func TestTraefikHealthcheckCarriesPingConfiguration(t *testing.T) {
	t.Parallel()

	want := []string{"CMD", "traefik", "healthcheck", "--ping=true"}
	for _, name := range []string{
		"docker-compose.ghcr.traefik.yaml",
		"docker-compose.traefik.yaml",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			contents, err := os.ReadFile(filepath.Join(repositoryRoot(t), "docker", name))
			if err != nil {
				t.Fatalf("read compose file: %v", err)
			}

			var document struct {
				Services map[string]struct {
					Command     []string `yaml:"command"`
					Healthcheck struct {
						Test []string `yaml:"test"`
					} `yaml:"healthcheck"`
				} `yaml:"services"`
			}
			if err := yaml.Unmarshal(contents, &document); err != nil {
				t.Fatalf("parse compose file: %v", err)
			}

			got := document.Services["traefik"].Healthcheck.Test
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Traefik healthcheck = %q, want %q", got, want)
			}
			if !contains(document.Services["traefik"].Command, "--ping=true") {
				t.Fatal("Traefik command does not enable ping")
			}
		})
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

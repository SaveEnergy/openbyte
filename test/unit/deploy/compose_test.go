package deploy_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

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

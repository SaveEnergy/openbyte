package deploy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraefikHealthcheckCarriesPingConfiguration(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile(filepath.Join(repositoryRoot(t), "docker", "docker-compose.traefik.yaml"))
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}

	compose := string(contents)
	if !strings.Contains(compose, `- "--ping=true"`) {
		t.Fatal("Traefik command does not enable ping")
	}
	if !strings.Contains(compose, `test: ["CMD", "traefik", "healthcheck", "--ping=true"]`) {
		t.Fatal("Traefik healthcheck does not use its ping endpoint")
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

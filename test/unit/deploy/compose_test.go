package deploy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraefikImageIsVersionAndDigestPinned(t *testing.T) {
	t.Parallel()

	const want = "traefik:v3.6.23@sha256:f5dba1e65167778cd5f8d1b463fc5d200f49d40c6458fc9f4b391a68ebfb9534"
	contents, err := os.ReadFile(filepath.Join(repositoryRoot(t), "docker", "docker-compose.traefik.yaml"))
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}

	var got string
	for line := range strings.SplitSeq(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "image: traefik:") {
			got = strings.TrimPrefix(line, "image: ")
			break
		}
	}
	if got != want {
		t.Fatalf("Traefik image = %q, want %q", got, want)
	}
}

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

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
	var got string
	for line := range strings.SplitSeq(readRepositoryFile(t, "docker", "docker-compose.traefik.yaml"), "\n") {
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

	compose := readRepositoryFile(t, "docker", "docker-compose.traefik.yaml")
	if !strings.Contains(compose, `- "--ping=true"`) {
		t.Fatal("Traefik command does not enable ping")
	}
	if !strings.Contains(compose, `test: ["CMD", "traefik", "healthcheck", "--ping=true"]`) {
		t.Fatal("Traefik healthcheck does not use its ping endpoint")
	}
}

func TestComposeSeparatesPublishedImageFromLocalBuild(t *testing.T) {
	t.Parallel()

	base := readRepositoryFile(t, "docker", "docker-compose.yaml")
	requireText(t, base, "image: ghcr.io/${GHCR_OWNER:-saveenergy}/openbyte:${IMAGE_TAG:-edge}")
	rejectText(t, base, "build:", "healthcheck:")

	local := readRepositoryFile(t, "docker", "docker-compose.local.yaml")
	requireText(t, local,
		"build:",
		"context: ..",
		"dockerfile: docker/Dockerfile",
		"target: server",
		"image: openbyte:latest",
	)
	rejectText(t, local, "ports:", "environment:", "volumes:", "restart:", "healthcheck:")
}

func TestTraefikComposeKeepsOnlyApplicationRouters(t *testing.T) {
	t.Parallel()

	compose := readRepositoryFile(t, "docker", "docker-compose.traefik.yaml")
	rejectText(t, compose,
		"--api.dashboard",
		"routers.dashboard",
		"api@internal",
		"openbyte-upload",
		"PathPrefix(`/api/v1/upload`)",
	)
	requireText(t, compose,
		"traefik.http.routers.openbyte.rule=",
		"traefik.http.routers.openbyte.entrypoints=web",
		"traefik.http.routers.openbyte-secure.rule=",
		"traefik.http.routers.openbyte-secure.entrypoints=websecure",
		"traefik.http.routers.openbyte-secure.tls.options=${TRAEFIK_TLS_OPTIONS:-openbyte-h1@file}",
		"traefik.http.services.openbyte.loadbalancer.server.port=8080",
		"\n    name: traefik\n",
	)
}

func TestDockerfileBuildInputsAndHealthcheck(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	dockerfile := readRepositoryFile(t, "docker", "Dockerfile")
	if !strings.Contains(dockerfile, "HEALTHCHECK") || !strings.Contains(dockerfile, "/health") {
		t.Fatal("Dockerfile does not own the openByte healthcheck")
	}

	for lineNumber, line := range strings.Split(dockerfile, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "COPY" || strings.HasPrefix(fields[1], "--") {
			continue
		}
		for _, source := range fields[1 : len(fields)-1] {
			if _, err := os.Stat(filepath.Join(root, source)); err != nil {
				t.Errorf("Dockerfile line %d COPY source %q: %v", lineNumber+1, source, err)
			}
		}
	}
}

func readRepositoryFile(t *testing.T, path ...string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(append([]string{repositoryRoot(t)}, path...)...))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(path...), err)
	}
	return string(contents)
}

func requireText(t *testing.T, contents string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(contents, value) {
			t.Errorf("missing %q", value)
		}
	}
}

func rejectText(t *testing.T, contents string, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(contents, value) {
			t.Errorf("unexpected %q", value)
		}
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

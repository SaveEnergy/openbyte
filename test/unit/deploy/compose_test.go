package deploy_test

import (
	"os"
	"path/filepath"
	"slices"
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

func TestComposeUsesCanonicalContainerContract(t *testing.T) {
	t.Parallel()

	compose := readRepositoryFile(t, "docker", "docker-compose.yaml")
	requireText(t, compose,
		`- "8080:8080"`,
		"openbyte-data:/app/data",
		"${BRAND_ASSETS_DIR:-../branding}:/app/branding:ro",
	)
	rejectText(t, compose,
		"DATA_DIR=",
		"TLS_CERT_FILE",
		"TLS_KEY_FILE",
		"TLS_AUTO_GEN",
		"HTTP2_ENABLED",
		"PPROF_ENABLED",
		"PPROF_ADDR",
	)

	wantEnvironment := []string{
		"SERVER_NAME",
		"BRAND_PRIMARY_COLOR_DARK",
		"BRAND_PRIMARY_COLOR_LIGHT",
		"BRAND_SECONDARY_COLOR_DARK",
		"BRAND_SECONDARY_COLOR_LIGHT",
		"BRAND_LOGO_PATH",
		"TRUST_PROXY_HEADERS",
		"TRUSTED_PROXY_CIDRS",
		"MAX_CONCURRENT_TRANSFERS",
		"MAX_CONCURRENT_PER_IP",
		"RATE_LIMIT_PER_IP",
		"GLOBAL_RATE_LIMIT",
		"MAX_TEST_DURATION",
		"MAX_STORED_RESULTS",
	}
	if got := composeEnvironmentEntries(t, compose); !slices.Equal(got, wantEnvironment) {
		t.Fatalf("Compose environment = %q, want explicit overrides %q", got, wantEnvironment)
	}
}

func TestExampleEnvDefersRuntimeDefaultsToBinary(t *testing.T) {
	t.Parallel()

	example := readRepositoryFile(t, ".env.example")
	requireText(t, example,
		"GHCR_OWNER=saveenergy",
		"SERVER_NAME=",
		"BRAND_ASSETS_DIR=../branding",
		"TRAEFIK_HOST_RULE=",
		"TRUST_PROXY_HEADERS=false",
		"README.md's environment-variable table",
	)
	rejectText(t, example,
		"\nPORT=",
		"\nBIND_ADDRESS=",
		"\nMAX_CONCURRENT_TRANSFERS=",
		"\nMAX_CONCURRENT_PER_IP=",
		"\nRATE_LIMIT_PER_IP=",
		"\nGLOBAL_RATE_LIMIT=",
		"\nMAX_TEST_DURATION=",
		"\nMAX_STORED_RESULTS=",
		"\nDATA_DIR=",
		"\nTLS_CERT_FILE=",
		"\nTLS_KEY_FILE=",
		"\nTLS_AUTO_GEN=",
		"\nHTTP2_ENABLED=",
		"\nPPROF_ENABLED=",
		"\nPPROF_ADDR=",
	)
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
	requireText(t, dockerfile,
		"VOLUME /app/data",
		"EXPOSE 8080",
		"ENV DATA_DIR=/app/data",
		"HEALTHCHECK",
		"http://localhost:8080/health",
	)

	var environmentInstructions []string
	for line := range strings.SplitSeq(dockerfile, "\n") {
		if strings.HasPrefix(line, "ENV ") {
			environmentInstructions = append(environmentInstructions, line)
		}
	}
	if want := []string{"ENV DATA_DIR=/app/data"}; !slices.Equal(environmentInstructions, want) {
		t.Fatalf("Dockerfile ENV instructions = %q, want %q", environmentInstructions, want)
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

func composeEnvironmentEntries(t *testing.T, compose string) []string {
	t.Helper()

	const section = "    environment:"
	lines := strings.Split(compose, "\n")
	for index, line := range lines {
		if line != section {
			continue
		}

		var entries []string
		for _, entry := range lines[index+1:] {
			const prefix = "      - "
			if !strings.HasPrefix(entry, prefix) {
				break
			}
			entries = append(entries, strings.TrimPrefix(entry, prefix))
		}
		return entries
	}

	t.Fatal("Compose environment section not found")
	return nil
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

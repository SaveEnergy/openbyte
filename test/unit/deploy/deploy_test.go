package deploy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHostDeployRollback(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("POSIX sh is required")
	}
	if runtime.GOOS == "windows" {
		t.Skip("deployment host script targets POSIX servers")
	}

	tests := []struct {
		name           string
		scenario       string
		wantSuccess    bool
		wantDeployment string
		wantRollback   bool
	}{
		{name: "success", scenario: "success", wantSuccess: true, wantDeployment: "new"},
		{name: "first deployment", scenario: "no_previous", wantSuccess: true, wantDeployment: "new"},
		{name: "missing Traefik subnet", scenario: "missing_proxy_subnet", wantDeployment: "old"},
		{name: "image mismatch", scenario: "image_mismatch", wantDeployment: "old", wantRollback: true},
		{name: "unhealthy openbyte", scenario: "app_unhealthy", wantDeployment: "old", wantRollback: true},
		{
			name:           "unhealthy traefik restores previous app",
			scenario:       "traefik_unhealthy",
			wantDeployment: "old",
			wantRollback:   true,
		},
		{name: "compose failure", scenario: "compose_failure", wantDeployment: "old", wantRollback: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateDir := t.TempDir()
			binDir := filepath.Join(stateDir, "bin")
			if err := os.Mkdir(binDir, 0o755); err != nil {
				t.Fatalf("create fake bin directory: %v", err)
			}
			initialDeployment := "old\n"
			if tt.scenario == "no_previous" {
				initialDeployment = "missing\n"
			}
			if err := os.WriteFile(filepath.Join(stateDir, "deployment"), []byte(initialDeployment), 0o600); err != nil {
				t.Fatalf("seed deployment state: %v", err)
			}
			writeExecutable(t, filepath.Join(binDir, "docker"), fakeDocker)
			writeExecutable(t, filepath.Join(binDir, "sleep"), "#!/bin/sh\nexit 0\n")
			envContents := "TRUST_PROXY_HEADERS=false\nTRUSTED_PROXY_CIDRS=\"10.0.0.0/8,192.168.0.0/16\"\n"
			if err := os.WriteFile(filepath.Join(stateDir, ".env"), []byte(envContents), 0o600); err != nil {
				t.Fatalf("write deployment env: %v", err)
			}

			cmd := exec.Command("sh", filepath.Join(repositoryRoot(t), "scripts", "deploy", "deploy_host.sh"))
			cmd.Env = append(os.Environ(),
				"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"FAKE_STATE_DIR="+stateDir,
				"FAKE_SCENARIO="+tt.scenario,
				"REMOTE_DIR="+stateDir,
				"GHCR_USERNAME=test-user",
				"GHCR_TOKEN=test-token",
				"OWNER_LC=saveenergy",
				"DEPLOY_TAG=new",
				"SERVER_NAME=",
			)
			output, err := cmd.CombinedOutput()
			if tt.wantSuccess && err != nil {
				t.Fatalf("deploy failed: %v\n%s", err, output)
			}
			if !tt.wantSuccess && err == nil {
				t.Fatalf("deploy succeeded unexpectedly:\n%s", output)
			}
			deployment, err := os.ReadFile(filepath.Join(stateDir, "deployment"))
			if err != nil {
				t.Fatalf("read deployment state: %v", err)
			}
			if got := strings.TrimSpace(string(deployment)); got != tt.wantDeployment {
				t.Fatalf("deployed image = %q, want %q\noutput:\n%s", got, tt.wantDeployment, output)
			}

			logContents, err := os.ReadFile(filepath.Join(stateDir, "docker.log"))
			if err != nil {
				t.Fatalf("read Docker log: %v", err)
			}
			logText := string(logContents)
			if tt.scenario != "missing_proxy_subnet" {
				wantCIDRs := "172.18.0.0/16,fd00:dead:beef::/64"
				newDeploy := newDeployCommand(logText)
				if !strings.Contains(newDeploy, "TRUST_PROXY_HEADERS=true") {
					t.Fatalf("new deployment does not trust proxy headers:\n%s", logText)
				}
				if !strings.Contains(newDeploy, "TRUSTED_PROXY_CIDRS="+wantCIDRs) {
					t.Fatalf("new deployment trusted proxy CIDRs missing %q:\n%s", wantCIDRs, logText)
				}
				if strings.Contains(newDeploy, "10.0.0.0/8") || strings.Contains(newDeploy, "192.168.0.0/16") {
					t.Fatalf("new deployment retained stale proxy CIDRs:\n%s", logText)
				}
			}
			rollbackCommand := "IMAGE_TAG=rollback-new compose "
			if got := strings.Contains(logText, rollbackCommand); got != tt.wantRollback {
				t.Fatalf("rollback command present = %v, want %v\nlog:\n%s", got, tt.wantRollback, logText)
			}
			if strings.Contains(logText, "IMAGE_TAG=new compose ") && strings.Contains(newDeployCommand(logText), "--force-recreate") {
				t.Fatalf("new deployment force-recreated the full stack:\n%s", logText)
			}
		})
	}
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newDeployCommand(logText string) string {
	for _, line := range strings.Split(logText, "\n") {
		if strings.Contains(line, "IMAGE_TAG=new compose ") && strings.Contains(line, " up ") {
			return line
		}
	}
	return ""
}

const fakeDocker = `#!/bin/sh
set -eu
printf 'IMAGE_TAG=%s %s TRUST_PROXY_HEADERS=%s TRUSTED_PROXY_CIDRS=%s\n' \
  "${IMAGE_TAG:-}" "$*" "${TRUST_PROXY_HEADERS:-}" "${TRUSTED_PROXY_CIDRS:-}" >> "$FAKE_STATE_DIR/docker.log"

command=$1
shift
case "$command" in
  login|logout)
    exit 0
    ;;
  network)
    case "$*" in
      inspect*--format*)
        if [ "$FAKE_SCENARIO" != missing_proxy_subnet ]; then
          printf '172.18.0.0/16,fd00:dead:beef::/64,'
        fi
        ;;
    esac
    exit 0
    ;;
  image)
    subcommand=$1
    shift
    case "$subcommand" in
      inspect)
        echo new-id
        ;;
      rm|tag)
        exit 0
        ;;
      *)
        echo "unexpected docker image command: $subcommand" >&2
        exit 1
        ;;
    esac
    ;;
  compose)
    action=
    for argument in "$@"; do
      case "$argument" in
        pull|up) action=$argument ;;
      esac
    done
    case "$action" in
      pull)
        exit 0
        ;;
      up)
        case "${IMAGE_TAG:-}" in
          new)
            if [ "$FAKE_SCENARIO" = compose_failure ]; then
              exit 1
            fi
            echo new > "$FAKE_STATE_DIR/deployment"
            ;;
          rollback-*)
            echo old > "$FAKE_STATE_DIR/deployment"
            ;;
        esac
        exit 0
        ;;
      *)
        echo "unexpected docker compose command" >&2
        exit 1
        ;;
    esac
    ;;
  inspect)
    container=
    for argument in "$@"; do
      container=$argument
    done
    deployment=$(cat "$FAKE_STATE_DIR/deployment")
    case "$*" in
      *'{{.Image}}'*)
        if [ "$deployment" = missing ]; then
          exit 0
        elif [ "$deployment" = old ]; then
          echo old-id
        elif [ "$FAKE_SCENARIO" = image_mismatch ]; then
          echo wrong-id
        else
          echo new-id
        fi
        ;;
      *'{{.State.Status}}'*)
        echo running
        ;;
      *'{{if .State.Health}}'*)
        if [ "$container" = traefik ] && [ "$FAKE_SCENARIO" = traefik_unhealthy ]; then
          echo unhealthy
        elif [ "$container" = openbyte ] && [ "$deployment" = new ] && [ "$FAKE_SCENARIO" = app_unhealthy ]; then
          echo unhealthy
        else
          echo healthy
        fi
        ;;
      *)
        echo '{}'
        ;;
    esac
    ;;
  *)
    echo "unexpected docker command: $command" >&2
    exit 1
    ;;
esac
`

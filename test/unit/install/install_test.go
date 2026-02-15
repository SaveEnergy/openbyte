package install_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallChecksumManifestParsing(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "checksums.txt")
	archive := "openbyte_1.0.0_linux_amd64.tar.gz"
	sum := strings.Repeat("a", 64)
	content := strings.Join([]string{
		sum + "  " + archive,
		strings.Repeat("b", 64) + "  another-file.tar.gz",
	}, "\n")
	if err := os.WriteFile(manifest, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command("sh", "-c", `
awk -v f="$1" '
	NF >= 2 {
		sum = tolower($1)
		$1 = ""
		sub(/^[[:space:]]+/, "", $0)
		if ($0 == f && sum ~ /^[0-9a-f]{64}$/) {
			print sum
			found = 1
			exit
		}
	}
	END {
		if (!found) exit 1
	}
' "$2"
`, "script", archive, manifest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("manifest parse command failed: %v (%s)", err, string(out))
	}
	got := strings.TrimSpace(string(out))
	if got != sum {
		t.Fatalf("parsed checksum = %q, want %q", got, sum)
	}
}

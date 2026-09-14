package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func requireIconTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"sips", "iconutil"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
}

func TestBuildICNSProducesEveryRendition(t *testing.T) {
	requireIconTools(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "AppIcon.png")
	writeTestPNG(t, src, 1024)

	dest := filepath.Join(dir, "app.icns")
	if err := BuildICNS(src, dest); err != nil {
		t.Fatalf("BuildICNS: %v", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("no icns written: %v", err)
	}
	if info.Size() == 0 {
		t.Error("icns is empty")
	}
}

func TestBuildICNSLeavesNoScratchDirectory(t *testing.T) {
	requireIconTools(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "AppIcon.png")
	writeTestPNG(t, src, 512)
	if err := BuildICNS(src, filepath.Join(dir, "app.icns")); err != nil {
		t.Fatalf("BuildICNS: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("left a scratch directory behind: %s", e.Name())
		}
	}
}

// writeTestPNG writes a square PNG with sips, so the test needs no fixture.
func writeTestPNG(t *testing.T, path string, size int) {
	t.Helper()
	seed := filepath.Join(t.TempDir(), "seed.png")
	if err := os.WriteFile(seed, onePixelPNG, 0o644); err != nil {
		t.Fatal(err)
	}
	n := strconv.Itoa(size)
	out, err := exec.Command("sips", "-z", n, n, seed, "--out", path).CombinedOutput()
	if err != nil {
		t.Fatalf("sips: %v\n%s", err, out)
	}
}

// onePixelPNG is the smallest valid PNG: 1x1, 8-bit RGB, black.
var onePixelPNG = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde,
	0x00, 0x00, 0x00, 0x0c, 'I', 'D', 'A', 'T',
	0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x03, 0x01, 0x01, 0x00,
	0x18, 0xdd, 0x8d, 0xb0,
	0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
}

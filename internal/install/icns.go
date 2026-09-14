package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// icnsSizes are the five point sizes macOS wants, each at 1x and 2x.
var icnsSizes = []int{16, 32, 128, 256, 512}

// BuildICNS renders srcPNG into the ten renditions macOS expects and packs them
// into destICNS. iconutil insists on a directory named *.iconset, so the
// scratch directory is made inside its own temp dir rather than beside dest.
func BuildICNS(srcPNG, destICNS string) error {
	tmp, err := os.MkdirTemp("", "perch-iconset-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	iconset := filepath.Join(tmp, "icon.iconset")
	if err := os.MkdirAll(iconset, 0o755); err != nil {
		return err
	}
	for _, size := range icnsSizes {
		for _, scale := range []int{1, 2} {
			px := size * scale
			name := fmt.Sprintf("icon_%dx%d.png", size, size)
			if scale == 2 {
				name = fmt.Sprintf("icon_%dx%d@2x.png", size, size)
			}
			out, err := exec.Command("sips",
				"-z", strconv.Itoa(px), strconv.Itoa(px), srcPNG,
				"--out", filepath.Join(iconset, name)).CombinedOutput()
			if err != nil {
				return fmt.Errorf("sips %dx%d: %w: %s", px, px, err, out)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(destICNS), 0o755); err != nil {
		return err
	}
	out, err := exec.Command("iconutil", "-c", "icns", iconset, "-o", destICNS).CombinedOutput()
	if err != nil {
		return fmt.Errorf("iconutil: %w: %s", err, out)
	}
	return nil
}

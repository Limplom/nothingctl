package magisk

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// MagiskCLIPatch pushes localImg to the device, patches it with the Magisk
// boot_patch.sh script (Magisk v21+), and pulls the patched image back to
// the same directory as localImg.
// Returns the local path of the patched image.
func MagiskCLIPatch(serial string, localImg string) (string, error) {
	temp := "/data/local/tmp"
	imgName := filepath.Base(localImg)
	remoteIn := temp + "/" + imgName

	fmt.Printf("  Pushing %s to device...\n", imgName)
	if err := adb.AdbPush(serial, localImg, remoteIn); err != nil {
		return "", err
	}

	fmt.Println("  Patching with Magisk boot_patch.sh...")
	stdout, _, _ := adb.Run([]string{
		"adb", "-s", serial, "shell",
		"su -c 'KEEPVERITY=true KEEPFORCEENCRYPT=true sh /data/adb/magisk/boot_patch.sh " + remoteIn + " && echo __PATCH_OK__'",
	})
	if !strings.Contains(stdout, "__PATCH_OK__") {
		adb.Run([]string{"adb", "-s", serial, "shell", "rm -f " + remoteIn})
		return "", nterrors.FlashError(
			fmt.Sprintf("Magisk boot_patch.sh failed — ensure Magisk is installed and root is granted.\nOutput: %s", strings.TrimSpace(stdout)),
		)
	}

	// Copy the patched image from Magisk's working directory to /data/local/tmp.
	remotePatched := temp + "/magisk_patched_boot.img"
	adb.Run([]string{
		"adb", "-s", serial, "shell",
		"su -c 'cp /data/adb/magisk/new-boot.img " + remotePatched + "'",
	})

	localPatched := filepath.Join(filepath.Dir(localImg), "magisk_patched_boot.img")

	fmt.Printf("  Pulling magisk_patched_boot.img...\n")
	if err := adb.AdbPull(serial, remotePatched, localPatched); err != nil {
		adb.Run([]string{"adb", "-s", serial, "shell", "rm -f " + remoteIn + " " + remotePatched})
		return "", err
	}

	// Clean up remote files.
	adb.Run([]string{"adb", "-s", serial, "shell", "rm -f " + remoteIn + " " + remotePatched})
	return localPatched, nil
}

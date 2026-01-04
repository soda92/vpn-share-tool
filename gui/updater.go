package gui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"github.com/soda92/vpn-share-tool/core"
)

func checkUpdate(w fyne.Window) {
	info, err := core.CheckForUpdates()
	if err != nil {
		log.Printf("Failed to check for updates: %v", err)
		return
	}

	if info.Version != Version && Version != "dev" {
		dialog.ShowConfirm(
			l("updateAvailableTitle"),
			l("updateAvailableContent", map[string]interface{}{"version": info.Version}),
			func(b bool) {
				if b {
					// Perform update via core logic
					if err := core.ApplyUpdate(info); err != nil {
						dialog.ShowError(err, w)
					}
					// ApplyUpdate exits on success, so we only reach here on error
				}
			},
			w,
		)
	}
}

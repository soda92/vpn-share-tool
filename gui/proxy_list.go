package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/soda92/vpn-share-tool/core"
	"github.com/soda92/vpn-share-tool/core/models"
	"github.com/soda92/vpn-share-tool/core/proxy"
)

// Global reference needed for adding proxies from CLI args or other places?
// Ideally we avoid globals. But sharedListData was local to Run.
var sharedListData = binding.NewStringList()

func addProxyToUI(newProxy *models.SharedProxy) {
	fyne.Do(func() {
		if core.MyIP != "" {
			sharedURL := fmt.Sprintf("http://%s:%d%s", core.MyIP, newProxy.RemotePort, newProxy.Path)
			displayString := l("sharedUrlFormat", map[string]interface{}{
				"originalUrl": newProxy.OriginalURL,
				"sharedUrl":   sharedURL,
			})
			sharedListData.Append(displayString)
		}
	})
}

func removeProxyFromUI(p *models.SharedProxy) {
	fyne.Do(func() {
		currentList, _ := sharedListData.Get()
		newList := []string{}
		for _, item := range currentList {
			if !strings.Contains(item, p.OriginalURL) {
				newList = append(newList, item)
			}
		}
		sharedListData.Set(newList)
	})
}

func setupProxyList(w fyne.Window) *widget.List {
	// Goroutine to handle UI updates from any part of the application
	go func() {
		for {
			select {
			case newProxy := <-proxy.ProxyAddedChan:
				addProxyToUI(newProxy)
			case removedProxy := <-proxy.ProxyRemovedChan:
				removeProxyFromUI(removedProxy)
			}
		}
	}()

	sharedList := widget.NewListWithData(
		sharedListData,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			o.(*widget.Label).Bind(i.(binding.String))
		},
	)

	sharedList.OnSelected = func(id widget.ListItemID) {
		itemText, err := sharedListData.GetValue(id)
		if err != nil {
			return
		}

		parts := strings.Split(itemText, " -> ")
		if len(parts) < 2 {
			return
		}
		urlToCopy := parts[1]

		w.Clipboard().SetContent(urlToCopy)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   l("copiedTitle"),
			Content: l("copiedContent"),
		})

		sharedList.Unselect(id)
	}

	return sharedList
}

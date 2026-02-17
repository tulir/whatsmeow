package main

import (
	"context"
	"fmt"
	"syscall/js"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	_ "github.com/ncruces/go-sqlite3/vfs/memdb"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go_client/pkg/shared"
)

func main() {
	runWasm()
}

func runWasm() {
	fmt.Println("[WASM] Initializing JS Bridge")
	js.Global().Set("startWhatsApp", js.FuncOf(jsStartWhatsApp))
	select {}
}

func jsStartWhatsApp(this js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].Type() != js.TypeFunction {
		return nil
	}
	qrCallback := args[0]
	go func() {
		ctx := context.Background()
		container, err := sqlstore.New(ctx, "sqlite3", "file:/examplestore.db?vfs=memdb", waLog.Stdout("Database", "DEBUG", true))
		if err != nil {
			fmt.Printf("Failed to init database: %v\n", err)
			return
		}
		deviceStore, _ := container.GetFirstDevice(ctx)
		client := whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "INFO", true))
		
		// Register WASM bridge logic locally
		shared.RegisterWASMBridge(client)

		// Add status update handlers
		client.AddEventHandler(func(evt interface{}) {
			switch evt.(type) {
			case *events.Connected:
				js.Global().Get("whatsappGateway").Call("updateStatus", "Connected", "#2e7d32", "#e8f5e9")
			case *events.LoggedOut:
				js.Global().Get("whatsappGateway").Call("updateStatus", "Logged Out", "#c62828", "#ffebee")
			}
		})

		if client.Store.ID == nil {
			qrChan, _ := client.GetQRChannel(ctx)
			client.Connect()
			for evt := range qrChan {
				if evt.Event == "code" {
					qrCallback.Invoke(evt.Code)
				}
			}
		} else {
			client.Connect()
		}
	}()
	return nil
}

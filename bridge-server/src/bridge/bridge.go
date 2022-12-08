package bridge

//////////////////
//    import    //
//////////////////

import (
	// internal packages
	context "context"
	sync "sync"
	// external packages
	whatsmeow "go.mau.fi/whatsmeow"
	store "go.mau.fi/whatsmeow/store"
	events "go.mau.fi/whatsmeow/types/events"
	// local packages
	env "bitaminco/support-whatsapp-bridge/src/environment"
)

///////////////////
//   variables   //
///////////////////

var (
	db  Database
	err error
)

// stored all clients with phone numbers
var mapAllClients = make(map[string]*whatsmeow.Client)

//////////////////
//    client    //
//////////////////

// method to establish client connection
func whatsappClientConnection(client *whatsmeow.Client, phone *string) {
	err = client.Connect()
	if err != nil {
		panic(err)
	}
	if client.Store.ID != nil {
		env.InfoLogger.Println("Connection Established:", client.Store.ID)
		// add user device to map
		mapAllClients[*phone] = client
	}
}

/////////////////////
//   all devices   //
/////////////////////

func StartSyncingToAllExistingDevices(wg *sync.WaitGroup) {
	// when done
	defer wg.Done()
	// connect to database
	if db.Container == nil {
		db.connectToDatabase()
	}
	// all connected devices updated
	db.getAllConnectedDevices()

	if len(db.DeviceStore) == 0 {
		// empty device store
		env.InfoLogger.Println("No connected device. Empty Store!")
	} else {
		// connect to all devices
		for _, device := range db.DeviceStore {
			// add job to wait group
			wg.Add(1)

			// run goroutines to sync devices
			go func(device *store.Device) {
				// when done
				defer wg.Done()
				// get client and connect one by one
				meowClient := whatsmeow.NewClient(device, nil)
				// add receive handler
				eventHandler := addEventHandlerWithDeviceInfo(device.ID.User)
				meowClient.AddEventHandler(eventHandler)
				// connect to client
				whatsappClientConnection(meowClient, &device.ID.User)
			}(device)
		}
	}
}

///////////////////////
//   single device   //
///////////////////////

func SyncWithGivenDevice(phone *string) *string {
	// connect to database
	if db.Container == nil {
		db.connectToDatabase()
	}
	// all connected devices updated
	db.getAllConnectedDevices()

	// search device and connect
	var userDevice *store.Device
	// check existing devices
	for _, device := range db.DeviceStore {
		if device.ID.User == *phone {
			userDevice = device
			break
		}
	}

	// if not add new device
	if userDevice == nil {
		userDevice = db.Container.NewDevice()
		userDevice.Save()
	}

	// create client
	meowClient := whatsmeow.NewClient(userDevice, nil)
	// add receive handler
	eventHandler := addEventHandlerWithDeviceInfo(*phone)
	meowClient.AddEventHandler(eventHandler)

	//
	// reconnection
	//
	if meowClient.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := meowClient.GetQRChannel(context.Background())
		// connect to client
		whatsappClientConnection(meowClient, phone)

		for evt := range qrChan {
			if evt.Event == "code" {
				// return the qr code
				return &evt.Code
			}
		}

	} else {
		// connect to client
		whatsappClientConnection(meowClient, phone)
	}
	emptyResponse := ""
	return &emptyResponse
}

//////////////////
//    events    //
//////////////////

// event handler
func addEventHandlerWithDeviceInfo(eventReceivedPhone string) func(event interface{}) {
	// here I have used closure to close the scope of the event listener
	// and added the receiver phone number in closed scope
	return func(event interface{}) {
		switch v := event.(type) {
		// messages
		case *events.Message:
			receiveMessageEventHandler(v, eventReceivedPhone)

		// connection
		case *events.PairSuccess:
			env.InfoLogger.Println("Paired Successfully!:", &v.ID)
		}
	}
}

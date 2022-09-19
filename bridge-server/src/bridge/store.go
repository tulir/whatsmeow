package bridge

//////////////////
//    import    //
//////////////////

import (
	// external packages
	_ "github.com/lib/pq"
	store "go.mau.fi/whatsmeow/store"
	sqlstore "go.mau.fi/whatsmeow/store/sqlstore"
	// local packages
	env "bitaminco/support-whatsapp-bridge/src/environment"
)

///////////////////
//    structs    //
///////////////////

type Database struct {
	Container   *sqlstore.Container
	DeviceStore []*store.Device
}

//////////////////
//   database   //
//////////////////

// method to connect database
func (db *Database) connectToDatabase() {
	// connection
	db.Container, err = sqlstore.New(env.BRIDGE_DATABASE_DIALECT, env.BRIDGE_DATABASE_URL, nil)
	if err != nil {
		panic(err)
	}
}

// method to get all connect devices
func (db *Database) getAllConnectedDevices() {
	db.DeviceStore, err = db.Container.GetAllDevices()
	if err != nil {
		panic(err)
	}
}

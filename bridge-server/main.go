package main

import (
	// internal packages
	http "net/http"
	sync "sync"
	time "time"
	// local packages
	api "bitaminco/support-whatsapp-bridge/src/api"
	bridge "bitaminco/support-whatsapp-bridge/src/bridge"
	database "bitaminco/support-whatsapp-bridge/src/database"
	env "bitaminco/support-whatsapp-bridge/src/environment"
)

/////////////////////
//   init method   //
/////////////////////

func init() {
	// wait group
	wg := &sync.WaitGroup{}
	wg.Add(2)

	// goroutines for logger initiation
	go env.CreateLoggerInstances(wg)

	// goroutines for syncing client connections
	go bridge.StartSyncingToAllExistingDevices(wg)

	// wait to complete the goroutines execution
	wg.Wait()
}

/////////////////////
//   main method   //
/////////////////////

func main() {
	// connect to database pool
	postgres := database.Connect()
	defer postgres.Close()

	// server
	app := &http.Server{
		Addr:     env.PORT,
		ErrorLog: env.ErrorLogger,
		Handler:  api.GetAPIRouter(),
		// enforce timeouts for server requests
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// initialize
	env.InfoLogger.Printf("Server is listening on Port URL: http://localhost%s", env.PORT)
	env.ErrorLogger.Fatal(app.ListenAndServe())
}

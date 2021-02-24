package main

import (
	"fmt"
	"log"
	"plugin"
	"strconv"

	"github.com/nitric-dev/membrane/membrane"
	"github.com/nitric-dev/membrane/plugins/sdk"
	"github.com/nitric-dev/membrane/utils"
)

// Pluggable version of the Nitric membrane
func main() {
	serviceAddress := utils.GetEnv("SERVICE_ADDRESS", "127.0.0.1:50051")
	childAddress := utils.GetEnv("CHILD_ADDRESS", "127.0.0.1:8080")
	pluginDir := utils.GetEnv("PLUGIN_DIR", "./plugins")
	serviceFactoryPluginFile := utils.GetEnv("SERVICE_FACTORY_PLUGIN", "default.so")
	childCommand := utils.GetEnv("INVOKE", "")
	tolerateMissingServices := utils.GetEnv("TOLERATE_MISSING_SERVICES", "false")

	tolerateMissing, err := strconv.ParseBool(tolerateMissingServices)
	// Set tolerate missing to false by default so missing plugins will cause a fatal error for safety.
	if err != nil {
		log.Println(fmt.Sprintf("failed to parse TOLERATE_MISSING_SERVICES environment variable with value [%s], defaulting to false", tolerateMissingServices))
		tolerateMissing = false
	}
	var serviceFactory sdk.ServiceFactory = nil

	// Load the Plugin Factory
	if plug, err := plugin.Open(fmt.Sprintf("%s/%s", pluginDir, serviceFactoryPluginFile)); err == nil {
		if symbol, err := plug.Lookup("New"); err == nil {
			if newFunc, ok := symbol.(func() (sdk.ServiceFactory, error)); ok {
				if serviceFactoryPlugin, err := newFunc(); err == nil {
					serviceFactory = serviceFactoryPlugin
				}
			}
		}
	}
	if serviceFactory == nil {
		log.Fatalf("failed to load Provider Factory Plugin: %s", serviceFactoryPluginFile)
	}

	// Load the concrete service implementations
	var authService sdk.UserService = nil
	var documentsService sdk.DocumentService = nil
	var eventingService sdk.EventService = nil
	var gatewayService sdk.GatewayService = nil
	var queueService sdk.QueueService = nil
	var storageService sdk.StorageService = nil

	// Load the auth service
	if authService, err = serviceFactory.NewAuthService(); err != nil {
		log.Fatal(err)
	}
	// Load the document service
	if documentsService, err = serviceFactory.NewDocumentService(); err != nil {
		log.Fatal(err)
	}
	// Load the eventing service
	if eventingService, err = serviceFactory.NewEventService(); err != nil {
		log.Fatal(err)
	}
	// Load the gateway service
	if gatewayService, err = serviceFactory.NewGatewayService(); err != nil {
		log.Fatal(err)
	}
	// Load the queue service
	if queueService, err = serviceFactory.NewQueueService(); err != nil {
		log.Fatal(err)
	}
	// Load the storage service
	if storageService, err = serviceFactory.NewStorageService(); err != nil {
		log.Fatal(err)
	}

	// Construct and validate the membrane server
	membraneServer, err := membrane.New(&membrane.MembraneOptions{
		ServiceAddress:          serviceAddress,
		ChildAddress:            childAddress,
		ChildCommand:            childCommand,
		AuthPlugin:              authService,
		EventingPlugin:          eventingService,
		DocumentsPlugin:         documentsService,
		StoragePlugin:           storageService,
		GatewayPlugin:           gatewayService,
		QueuePlugin:             queueService,
		TolerateMissingServices: tolerateMissing,
	})

	if err != nil {
		log.Fatalf("There was an error initialising the membraneServer server: %v", err)
	}

	// Start the Membrane server
	membraneServer.Start()
}

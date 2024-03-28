// Copyright 2021 Nitric Pty Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/nitrictech/nitric/cloud/oci/pkg/runtime/http"
	"github.com/nitrictech/nitric/cloud/oci/pkg/runtime/keyvalue"
	"github.com/nitrictech/nitric/cloud/oci/pkg/runtime/resource"
	"github.com/nitrictech/nitric/cloud/oci/pkg/runtime/secret"
	"github.com/nitrictech/nitric/cloud/oci/pkg/runtime/storage"
	"github.com/nitrictech/nitric/cloud/oci/pkg/runtime/topic"
	"github.com/nitrictech/nitric/cloud/oci/pkg/runtime/websocket"
	"github.com/nitrictech/nitric/core/pkg/logger"
	"github.com/nitrictech/nitric/core/pkg/membrane"
)

func main() {
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	signal.Notify(term, os.Interrupt, syscall.SIGINT)

	membraneOpts := membrane.DefaultMembraneOptions()

	provider, err := resource.New()
	if err != nil {
		logger.Fatalf("could not create aws provider: %v", err)
		return
	}

	// Load the appropriate gateway based on the environment.
	membraneOpts.GatewayPlugin, _ = http.NewHttpGateway(nil)

	membraneOpts.SecretManagerPlugin, _ = secret.New(provider)
	membraneOpts.KeyValuePlugin, _ = keyvalue.New(provider)
	membraneOpts.TopicsPlugin, _ = topic.New(provider)
	membraneOpts.StoragePlugin, _ = storage.New(provider)
	membraneOpts.ResourcesPlugin = provider
	membraneOpts.WebsocketPlugin, _ = websocket.New(provider)

	m, err := membrane.New(membraneOpts)
	if err != nil {
		logger.Fatalf("There was an error initializing the membrane server: %v", err)
	}

	errChan := make(chan error)
	// Start the Membrane server
	go func(chan error) {
		errChan <- m.Start()
	}(errChan)

	select {
	case membraneError := <-errChan:
		logger.Errorf("Membrane Error: %v, exiting\n", membraneError)
	case sigTerm := <-term:
		logger.Debugf("Received %v, exiting\n", sigTerm)
	}

	m.Stop()
}

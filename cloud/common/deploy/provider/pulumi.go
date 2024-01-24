// Copyright 2021 Nitric Technologies Pty Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"fmt"
	"log"
	"net"
	"runtime/debug"

	"github.com/nitrictech/nitric/cloud/common/deploy/env"
	"github.com/nitrictech/nitric/cloud/common/deploy/pulumix"
	deploymentspb "github.com/nitrictech/nitric/core/pkg/proto/deployments/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"google.golang.org/grpc"
)

type PulumiProviderServer struct {
	provider NitricPulumiProvider
}

func NewPulumiProviderServer(provider NitricPulumiProvider) *PulumiProviderServer {
	return &PulumiProviderServer{
		provider: provider,
	}
}

func createPulumiProgramForNitricProvider(req *deploymentspb.DeploymentUpRequest, nitricProvider NitricPulumiProvider) func(*pulumi.Context) error {
	return func(ctx *pulumi.Context) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				err = fmt.Errorf("recovered panic: %+v\n Stack: %s", r, stack)
			}
		}()

		// Pre-deployment Hook, used for validation, extension, spec modification, etc.
		err = nitricProvider.Pre(ctx, req.Spec.Resources)
		if err != nil {
			return err
		}

		for _, res := range nitricProvider.Order(req.Spec.Resources) {
			parent, err := pulumix.ParentResourceFromResourceId(ctx, res.Id)
			if err != nil {
				return err
			}

			switch t := res.Config.(type) {
			case *deploymentspb.Resource_Service:
				err = nitricProvider.Service(ctx, parent, res.Id.Name, t.Service)
			case *deploymentspb.Resource_Secret:
				err = nitricProvider.Secret(ctx, parent, res.Id.Name, t.Secret)
			case *deploymentspb.Resource_Topic:
				err = nitricProvider.Topic(ctx, parent, res.Id.Name, t.Topic)
			case *deploymentspb.Resource_Bucket:
				err = nitricProvider.Bucket(ctx, parent, res.Id.Name, t.Bucket)
			case *deploymentspb.Resource_Api:
				err = nitricProvider.Api(ctx, parent, res.Id.Name, t.Api)
			case *deploymentspb.Resource_Policy:
				err = nitricProvider.Policy(ctx, parent, res.Id.Name, t.Policy)
			case *deploymentspb.Resource_Http:
				err = nitricProvider.Http(ctx, parent, res.Id.Name, t.Http)
			}
			if err != nil {
				return err
			}
		}

		// Validate extract and whatever else
		return nitricProvider.Post(ctx)
	}
}

func stackAndProjectFromAttributes(attributesMap map[string]interface{}) (string, string, error) {
	projectName, ok := attributesMap["project"].(string)
	if !ok {
		return "", "", fmt.Errorf("")
	}

	stackName, ok := attributesMap["stack"].(string)
	if !ok {
		return "", "", fmt.Errorf("")
	}

	return projectName, stackName, nil
}

// Up - automatically called by the Nitric CLI via the `up` command
func (s *PulumiProviderServer) Up(req *deploymentspb.DeploymentUpRequest, stream deploymentspb.Deployment_UpServer) error {
	projectName, stackName, err := stackAndProjectFromAttributes(req.Attributes.AsMap())
	if err != nil {
		return err
	}

	err = s.provider.Init(req.Attributes.AsMap())
	if err != nil {
		return err
	}

	pulumiProgram := createPulumiProgramForNitricProvider(req, s.provider)

	autoStack, err := auto.UpsertStackInlineSource(context.TODO(), fmt.Sprintf("%s-%s", projectName, stackName), projectName, pulumiProgram)
	if err != nil {
		return err
	}

	pulumiEventsChan := make(chan events.EngineEvent)

	go func() {
		// output the stream
		_ = pulumix.StreamPulumiUpEngineEvents(stream, pulumiEventsChan)
	}()

	config, err := s.provider.Config()
	if err != nil {
		return err
	}

	err = autoStack.SetAllConfig(context.TODO(), config)
	if err != nil {
		return err
	}

	_, err = autoStack.Up(context.TODO(), optup.EventStreams(pulumiEventsChan))

	if err != nil {
		// TODO: remove when CLI is displaying this clearly.
		fmt.Println(err.Error())
		return err
	}

	return nil
}

// Down - automatically called by the Nitric CLI via the `down` command
func (s *PulumiProviderServer) Down(req *deploymentspb.DeploymentDownRequest, stream deploymentspb.Deployment_DownServer) error {
	projectName, stackName, err := stackAndProjectFromAttributes(req.Attributes.AsMap())
	if err != nil {
		return err
	}

	// run down on the stack
	err = s.provider.Init(req.Attributes.AsMap())
	if err != nil {
		return err
	}

	stack, err := auto.UpsertStackInlineSource(context.TODO(), fmt.Sprintf("%s-%s", projectName, stackName), projectName, nil)
	if err != nil {
		return err
	}

	pulumiEventsChan := make(chan events.EngineEvent)

	go func() {
		_ = pulumix.StreamPulumiDownEngineEvents(stream, pulumiEventsChan)
	}()

	config, err := s.provider.Config()
	if err != nil {
		return err
	}

	err = stack.SetAllConfig(context.TODO(), config)
	if err != nil {
		return err
	}

	_, err = stack.Destroy(context.TODO(), optdestroy.EventStreams(pulumiEventsChan))

	if err != nil {
		return err
	}

	return nil
}

// Start - starts the Nitric Provider gRPC server, making it callable by the Nitric CLI during deployments.
func (s *PulumiProviderServer) Start() {
	port := env.PORT.String()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("error listening on port %s %v", port, err)
	}

	srv := grpc.NewServer()

	deploymentspb.RegisterDeploymentServer(srv, s)

	fmt.Printf("Deployment server started on %s\n", lis.Addr().String())
	err = srv.Serve(lis)
	if err != nil {
		log.Fatalf("error serving requests %v", err)
	}
}

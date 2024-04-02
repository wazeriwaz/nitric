// Copyright Nitric Pty Ltd.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deploy

import (
	"fmt"
	"strings"

	_ "embed"

	"github.com/nitrictech/nitric/cloud/common/deploy"
	"github.com/nitrictech/nitric/cloud/common/deploy/provider"
	"github.com/nitrictech/nitric/cloud/common/deploy/pulumix"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/apigateway"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/functions"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/identity"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// type ConfigFile struct {
// 	UserID      string
// 	Fingerprint string
// 	Tenancy     string
// 	Region      string
// 	KeyFile     string
// }

type NitricOCIPulumiProvider struct {
	stackId     string
	projectName string
	stackName   string

	fullStackName string

	config *OCIConfig
	region string

	compartment    *identity.Compartment
	serviceAccount *identity.User

	apis      map[string]*apigateway.Api
	functions map[string]*functions.Function

	provider.NitricDefaultOrder
}

var _ provider.NitricPulumiProvider = (*NitricOCIPulumiProvider)(nil)

func (a *NitricOCIPulumiProvider) Config() (auto.ConfigMap, error) {
	return auto.ConfigMap{
		"docker:version": auto.ConfigValue{Value: deploy.PulumiDockerVersion},
	}, nil
}

func (a *NitricOCIPulumiProvider) Init(attributes map[string]interface{}) error {
	var err error

	a.config, err = ConfigFromAttributes(attributes)
	if err != nil {
		return err
	}

	var isString bool

	iProject, hasProject := attributes["project"]
	a.projectName, isString = iProject.(string)
	if !hasProject || !isString || a.projectName == "" {
		// need a valid project name
		return fmt.Errorf("project is not set or invalid")
	}

	iStack, hasStack := attributes["stack"]
	a.stackName, isString = iStack.(string)
	if !hasStack || !isString || a.stackName == "" {
		// need a valid stack name
		return fmt.Errorf("stack is not set or invalid")
	}

	// Backwards compatible stack name
	// The existing providers in the CLI
	// Use the combined project and stack name
	a.fullStackName = fmt.Sprintf("%s-%s", a.projectName, a.stackName)

	return nil
}

func (a *NitricOCIPulumiProvider) Pre(ctx *pulumi.Context, resources []*pulumix.NitricPulumiResource[any]) error {
	// make our random stackId
	stackRandId, err := random.NewRandomString(ctx, fmt.Sprintf("%s-stack-name", ctx.Stack()), &random.RandomStringArgs{
		Special: pulumi.Bool(false),
		Length:  pulumi.Int(8),
		Keepers: pulumi.ToMap(map[string]interface{}{
			"stack-name": ctx.Stack(),
		}),
	})
	if err != nil {
		return err
	}

	stackIdChan := make(chan string)
	pulumi.Sprintf("%s-%s", ctx.Stack(), stackRandId.Result).ApplyT(func(id string) string {
		stackIdChan <- id
		return id
	})

	a.stackId = <-stackIdChan

	compartmentName := fmt.Sprintf("compartment-%s", a.stackId)

	a.compartment, err = identity.NewCompartment(ctx, compartmentName, &identity.CompartmentArgs{
		Description:  pulumi.Sprintf("compartment for stack %s", ctx.Stack()),
		EnableDelete: pulumi.Bool(true),
	})
	if err != nil {
		return err
	}

	userId, ok := ctx.GetConfig("oci:userOcid")
	if !ok {
		return fmt.Errorf("user id not supplied")
	}

	a.serviceAccount, err = identity.GetUser(ctx, "user-account", pulumi.ID(userId), nil)
	if err != nil {
		return err
	}

	return nil
}

func (a *NitricOCIPulumiProvider) Post(ctx *pulumi.Context) error {
	return nil
}

func (a *NitricOCIPulumiProvider) Result(ctx *pulumi.Context) (pulumi.StringOutput, error) {
	outputs := []interface{}{}

	output, ok := pulumi.All(outputs...).ApplyT(func(deets []interface{}) string {
		stringyOutputs := make([]string, len(deets))
		for i, d := range deets {
			stringyOutputs[i] = d.(string)
		}

		return strings.Join(stringyOutputs, "\n")
	}).(pulumi.StringOutput)

	if !ok {
		return pulumi.StringOutput{}, fmt.Errorf("Failed to generate pulumi output")
	}

	return output, nil
}

func NewNitricOCIPulumiProvider() *NitricOCIPulumiProvider {
	return &NitricOCIPulumiProvider{}
}

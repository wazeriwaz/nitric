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
	"os"
	"strings"

	_ "embed"

	"github.com/nitrictech/nitric/cloud/common/deploy"
	"github.com/nitrictech/nitric/cloud/common/deploy/provider"
	"github.com/nitrictech/nitric/cloud/common/deploy/pulumix"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/apigateway"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/core"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/functions"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/identity"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ConfigFile struct {
	UserID  string
	Tenancy string
	Region  string
}

type NitricOCIPulumiProvider struct {
	stackId     string
	projectName string
	stackName   string

	fullStackName string

	config *ConfigFile

	compartment    *identity.Compartment
	serviceAccount *identity.User

	subnet *core.Subnet

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
		Name:         pulumi.String(compartmentName),
		Description:  pulumi.Sprintf("compartment for stack %s", ctx.Stack()),
		EnableDelete: pulumi.Bool(true),
	})
	if err != nil {
		return err
	}

	a.config, err = getConfig("default", fmt.Sprintf("%s/.oci/config", os.Getenv("HOME")))
	if err != nil {
		return err
	}

	a.serviceAccount, err = identity.GetUser(ctx, "user-account", pulumi.ID(a.config.UserID), nil)
	if err != nil {
		return fmt.Errorf("error getting user: %v", err)
	}

	cidrBlock := "10.0.1.0/24"

	vcnName := fmt.Sprintf("vcn-%s", a.stackId)

	vcn, err := core.NewVcn(ctx, vcnName, &core.VcnArgs{
		DisplayName:   pulumi.String(vcnName),
		CompartmentId: a.compartment.CompartmentId,
		CidrBlocks:    pulumi.ToStringArray([]string{cidrBlock}),
	})
	if err != nil {
		return err
	}

	subnetName := fmt.Sprintf("subnet-%s", a.stackId)

	a.subnet, err = core.NewSubnet(ctx, subnetName, &core.SubnetArgs{
		DisplayName:   pulumi.String(subnetName),
		CompartmentId: a.compartment.CompartmentId,
		CidrBlock:     pulumi.String(cidrBlock),
		VcnId:         vcn.ID(),
	})
	if err != nil {
		return err
	}

	return nil
}

func getConfig(profile string, filePath string) (*ConfigFile, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	configContents := strings.Split(string(contents), "\n")

	// Get where the specific profile config starts.
	profileConfig := -1
	for idx, line := range configContents {
		if line == fmt.Sprintf("[%s]", profile) {
			profileConfig = idx
		}
	}

	configFile := &ConfigFile{}

	// Get the user id from that section of profile config
	for idx := 1; idx <= 5; idx++ {
		keyValue := strings.Split(configContents[profileConfig+idx], "=")

		switch keyValue[0] {
		case "user":
			configFile.UserID = keyValue[1]
		case "tenancy":
			configFile.Tenancy = keyValue[1]
		case "region":
			configFile.Region = keyValue[1]
		}
	}

	return configFile, nil
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
	return &NitricOCIPulumiProvider{
		functions: make(map[string]*functions.Function),
		apis:      make(map[string]*apigateway.Api),
	}
}

package deploytf

import (
	"github.com/aws/jsii-runtime-go"
	"github.com/hashicorp/terraform-cdk-go/cdktf"
	"github.com/nitrictech/nitric/cloud/azure/deploytf/generated/service"
	"github.com/nitrictech/nitric/cloud/common/deploy/provider"
	deploymentspb "github.com/nitrictech/nitric/core/pkg/proto/deployments/v1"
)

// Service - Deploy an service (Service)
func (a *NitricAzureTerraformProvider) Service(stack cdktf.TerraformStack, name string, config *deploymentspb.Service, runtimeProvider provider.RuntimeProvider) error {
	a.Services[name] = service.NewService(stack, jsii.String(name), &service.ServiceConfig{
		Name: jsii.String(name),
		// ApplicationClientId: TODO,
		// ClientSecret: TODO,
		// ContainerAppEnvironmentId: TODO,
		ResourceGroupName: a.Stack.ResourceGroupNameOutput(),
		// TenantId:          TODO,
		
	})

	return nil
}

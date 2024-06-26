package deploy

import (
	"fmt"

	deploymentspb "github.com/nitrictech/nitric/core/pkg/proto/deployments/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func (p *NitricGcpPulumiProvider) Job(ctx *pulumi.Context, parent pulumi.Resource, name string, config *deploymentspb.Job) error {
	return fmt.Errorf("Not implemented")
}

package deploy

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	deploymentspb "github.com/nitrictech/nitric/core/pkg/proto/deployments/v1"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/apigateway"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Api struct {
	pulumi.ResourceState
	Name string
}

func (n *NitricOCIPulumiProvider) Api(ctx *pulumi.Context, parent pulumi.Resource, name string, config *deploymentspb.Api) error {
	opts := []pulumi.ResourceOption{pulumi.Parent(parent)}

	if config.GetOpenapi() == "" {
		return fmt.Errorf("oci provider can only deploy OpenAPI specs")
	}

	openapiDoc := &openapi3.T{}
	err := openapiDoc.UnmarshalJSON([]byte(config.GetOpenapi()))
	if err != nil {
		return fmt.Errorf("invalid document supplied for api: %s", name)
	}

	api, err := apigateway.NewApi(ctx, name, &apigateway.ApiArgs{
		CompartmentId: n.compartment.CompartmentId,
		Content:       pulumi.String(config.GetOpenapi()),
		DefinedTags: pulumi.Map{
			"Operations.CostCenter": pulumi.Any("42"),
		},
	}, opts...)
	if err != nil {
		return err
	}

	return nil
}

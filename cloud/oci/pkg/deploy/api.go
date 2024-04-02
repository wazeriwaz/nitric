package deploy

import (
	"encoding/base64"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/nitrictech/nitric/cloud/common/deploy/resources"
	"github.com/nitrictech/nitric/cloud/common/deploy/tags"
	"github.com/nitrictech/nitric/core/pkg/help"
	deploymentspb "github.com/nitrictech/nitric/core/pkg/proto/deployments/v1"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/apigateway"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Api struct {
	pulumi.ResourceState
	Name string
}

type nameUrlPair struct {
	name      string
	invokeUrl string
}

func (n *NitricOCIPulumiProvider) Api(ctx *pulumi.Context, parent pulumi.Resource, name string, config *deploymentspb.Api) error {
	var err error

	opts := []pulumi.ResourceOption{pulumi.Parent(parent)}

	if config.GetOpenapi() == "" {
		return fmt.Errorf("oci provider can only deploy OpenAPI specs")
	}

	nameUrlPairs := make([]interface{}, 0, len(n.functions))

	// collect name arn pairs for output iteration
	for k, v := range n.functions {
		nameUrlPairs = append(nameUrlPairs, pulumi.All(k, v.InvokeEndpoint).ApplyT(func(args []interface{}) (nameUrlPair, error) {
			name, nameOk := args[0].(string)
			url, urlOk := args[1].(string)

			if !nameOk || !urlOk {
				return nameUrlPair{}, fmt.Errorf("invalid data %T %v", args, args)
			}

			return nameUrlPair{
				name:      name,
				invokeUrl: url,
			}, nil
		}))
	}

	content := pulumi.All(nameUrlPairs).ApplyT(func(pairs []interface{}) (string, error) {
		openapiDoc := &openapi3.T{}
		err = openapiDoc.UnmarshalJSON([]byte(config.GetOpenapi()))
		if err != nil {
			return "", fmt.Errorf("invalid document supplied for api: %s", name)
		}

		naps := make(map[string]string)

		for _, p := range pairs {
			if pair, ok := p.(nameUrlPair); ok {
				naps[pair.name] = pair.invokeUrl
			} else {
				return "", fmt.Errorf("failed to resolve Cloud Run container URL for api %s, invalid name URL pair value %T %v, %s", name, p, p, help.BugInNitricHelpText())
			}
		}

		// Add x-oci-functions-backend to the openapi document
		// for relevant routes
		for _, pathItem := range openapiDoc.Paths {
			for _, operation := range pathItem.Operations() {
				serviceName := ""
				if v, ok := operation.Extensions["x-nitric-target"]; ok {
					targetMap, isMap := v.(map[string]interface{})
					if isMap {
						serviceName, _ = targetMap["name"].(string)
					}
				}

				operation.Extensions["x-oci-functions-backend"] = n.functions[serviceName].InvokeEndpoint
			}
		}

		b, err := openapiDoc.MarshalJSON()
		if err != nil {
			return "", err
		}

		return base64.StdEncoding.EncodeToString(b), nil
	}).(pulumi.StringOutput)

	n.apis[name], err = apigateway.NewApi(ctx, name, &apigateway.ApiArgs{
		CompartmentId: n.compartment.CompartmentId,
		Content:       content,
		DefinedTags:   pulumi.ToMap(tags.TagsAsInterface(n.stackId, name, resources.API)),
	}, opts...)
	if err != nil {
		return err
	}

	return nil
}

package deploy

import (
	"fmt"

	"github.com/nitrictech/nitric/cloud/common/deploy/image"
	"github.com/nitrictech/nitric/cloud/common/deploy/provider"
	"github.com/nitrictech/nitric/cloud/common/deploy/pulumix"
	"github.com/nitrictech/nitric/cloud/common/deploy/resources"
	"github.com/nitrictech/nitric/cloud/common/deploy/tags"
	deploymentspb "github.com/nitrictech/nitric/core/pkg/proto/deployments/v1"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/artifacts"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/functions"
	"github.com/pulumi/pulumi-oci/sdk/go/oci/identity"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// This represents a unique unit of execution at the moment this is a container but could also be many things e.g. WASM, Binary, source zip etc.
type Service struct {
	pulumi.ResourceState
	Name string
}

func (n *NitricOCIPulumiProvider) createContainerRepository(ctx *pulumi.Context, parent pulumi.Resource, name string) (*artifacts.ContainerRepository, error) {
	return artifacts.NewContainerRepository(ctx, name, &artifacts.ContainerRepositoryArgs{
		CompartmentId: n.compartment.CompartmentId,
		FreeformTags:  pulumi.ToMap(tags.TagsAsInterface(n.stackId, name, resources.Service)),
		IsPublic:      pulumi.Bool(false),
	})
}

func (n *NitricOCIPulumiProvider) createImage(ctx *pulumi.Context, parent pulumi.Resource, name string, repo *artifacts.ContainerRepository, config *deploymentspb.Service) (*image.Image, error) {
	if config.GetImage() == nil {
		return nil, fmt.Errorf("oci provider can only deploy service with an image source")
	}

	if config.GetImage().GetUri() == "" {
		return nil, fmt.Errorf("oci provider can only deploy service with an image source")
	}

	if config.Type == "" {
		config.Type = "default"
	}

	authToken, err := identity.NewAuthToken(ctx, "myAuthToken", &identity.AuthTokenArgs{
		UserId:      n.serviceAccount.ID(), // Replace with the actual User OCID
		Description: pulumi.String("AuthToken for OCI CLI authentication"),
	})
	if err != nil {
		return nil, err
	}

	return image.NewImage(ctx, name, &image.ImageArgs{
		SourceImage:   config.GetImage().GetUri(),
		RepositoryUrl: pulumi.Sprintf("%s.ocir.io/%s/%s:latest", n.region, repo.Namespace, repo.DisplayName),
		Username:      pulumi.Sprintf("%s/%s", repo.Namespace, n.serviceAccount.Email),
		Password:      authToken.Token,
		Runtime:       runtime,
	}, pulumi.Parent(parent), pulumi.DependsOn([]pulumi.Resource{repo}))
}

func (a *NitricOCIPulumiProvider) Service(ctx *pulumi.Context, parent pulumi.Resource, name string, config *pulumix.NitricPulumiServiceConfig, provider provider.RuntimeProvider) error {
	// Create the ECR repository to push the image to
	repo, err := a.createContainerRepository(ctx, parent, name)
	if err != nil {
		return err
	}

	image, err := a.createImage(ctx, parent, name, repo, config.Service)
	if err != nil {
		return err
	}

	app, err := functions.NewApplication(ctx, name, &functions.ApplicationArgs{
		CompartmentId: a.compartment.CompartmentId,
		FreeformTags:  pulumi.ToMap(tags.TagsAsInterface(a.stackId, name, resources.Service)),
		SubnetIds:     pulumi.ToStringArray([]string{}),
	})
	if err != nil {
		return err
	}

	function, err := functions.NewFunction(ctx, name, &functions.FunctionArgs{
		ApplicationId: app.ID(),
		MemoryInMbs:   pulumi.String(512),
		FreeformTags:  pulumi.ToMap(tags.TagsAsInterface(a.stackId, name, resources.Service)),
		Image:         image.URI(),
	})

	a.functions[name] = function

	return nil
}

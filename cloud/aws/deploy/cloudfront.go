package deploy

import (
	_ "embed"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/nitrictech/nitric/cloud/aws/deploy/embeds"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/cloudfront"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Deploy static assets to s3 to be connected to our cloudfront distribution
func (a *NitricAwsPulumiProvider) deployStaticAssets(ctx *pulumi.Context) error {
	var err error

	// if there is a public directory available
	// we will upload all files to the public bucket
	// Check if the "public" directory exists
	if _, err := os.Stat("public"); os.IsNotExist(err) {
		// Directory does not exist, skip deployment
		return nil
	}

	// Create the public assets bucket
	a.publicBucket, err = s3.NewBucket(ctx, "public", &s3.BucketArgs{
		Website: &s3.BucketWebsiteArgs{
			IndexDocument: pulumi.String("index.html"),
		},
	})
	if err != nil {
		return err
	}

	// Enumerate the public directory in pwd and upload all files to the public bucket
	// This will be the source for our cloudfront distribution
	err = filepath.WalkDir("public", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Determine the content type based on the file extension
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		_, err = s3.NewBucketObject(ctx, d.Name(), &s3.BucketObjectArgs{
			Bucket:      a.publicBucket.Bucket,
			Source:      pulumi.NewFileAsset(path),
			ContentType: pulumi.String(contentType),
		})

		return err
	})

	return err
}

// Deploy a cloudfront distribution for our stack
func (a *NitricAwsPulumiProvider) deployCloudfrontDistribution(ctx *pulumi.Context) error {
	origins := cloudfront.DistributionOriginArray{}
	var defaultCacheBehaviour *cloudfront.DistributionDefaultCacheBehaviorArgs = nil
	orderedCacheBeviours := cloudfront.DistributionOrderedCacheBehaviorArray{}

	oai, err := cloudfront.NewOriginAccessIdentity(ctx, "oai", &cloudfront.OriginAccessIdentityArgs{
		Comment: pulumi.String("OAI for accessing S3 bucket"),
	})
	if err != nil {
		return err
	}

	policy := pulumi.All(a.publicBucket.Arn, oai.IamArn).ApplyT(func(args []interface{}) (string, error) {
		bucketID, bucketIdOk := args[0].(string)
		oaiPath, oaiPathOk := args[1].(string)

		if !bucketIdOk || !oaiPathOk {
			return "", fmt.Errorf("failed to get bucket ID or OAI path")
		}

		return fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": {
						"AWS": "%s"
					},
					"Action": "s3:GetObject",
					"Resource": "%s/*"
				}
			]
		}`, oaiPath, bucketID), nil
	}).(pulumi.StringOutput)

	_, err = s3.NewBucketPolicy(ctx, "publicBucketPolicy", &s3.BucketPolicyArgs{
		Bucket: a.publicBucket.Bucket,
		Policy: policy,
	})
	if err != nil {
		return err
	}

	// We conventionally route to nitric resources from this distribution to create a single entry point
	// for the entire stack. e.g. /api/main/* will route to a nitric api named "main"
	fun, err := cloudfront.NewFunction(ctx, "url-rewrite-function", &cloudfront.FunctionArgs{
		Comment: pulumi.String("Rewrite URLs routed to nitric services"),
		Code:    embeds.GetUrlRewriteFunction(),
		Runtime: pulumi.String("cloudfront-js-1.0"),
	})
	if err != nil {
		return err
	}

	if a.publicBucket != nil {
		origins = append(origins, &cloudfront.DistributionOriginArgs{
			DomainName: a.publicBucket.BucketRegionalDomainName,
			OriginId:   pulumi.String("publicOrigin"),
			S3OriginConfig: &cloudfront.DistributionOriginS3OriginConfigArgs{
				OriginAccessIdentity: oai.CloudfrontAccessIdentityPath,
			},
		})

		defaultCacheBehaviour = &cloudfront.DistributionDefaultCacheBehaviorArgs{
			TargetOriginId:       pulumi.String("publicOrigin"),
			ViewerProtocolPolicy: pulumi.String("redirect-to-https"),
			AllowedMethods: pulumi.StringArray{
				pulumi.String("GET"),
				pulumi.String("HEAD"),
			},
			CachedMethods: pulumi.StringArray{
				pulumi.String("GET"),
				pulumi.String("HEAD"),
			},
			ForwardedValues: &cloudfront.DistributionDefaultCacheBehaviorForwardedValuesArgs{
				QueryString: pulumi.Bool(false),
				Cookies: &cloudfront.DistributionDefaultCacheBehaviorForwardedValuesCookiesArgs{
					Forward: pulumi.String("none"),
				},
			},
			MinTtl:     pulumi.Int(0),
			DefaultTtl: pulumi.Int(3600),
			MaxTtl:     pulumi.Int(86400),
		}
	}

	// For each API forward to the appropriate API gateway
	for name, api := range a.Apis {
		apiDomainName := api.ApiEndpoint.ApplyT(func(endpoint string) string {
			return strings.Replace(endpoint, "https://", "", 1)
		}).(pulumi.StringOutput)

		origins = append(origins, &cloudfront.DistributionOriginArgs{
			DomainName: apiDomainName,
			OriginId:   pulumi.String(name),
			CustomOriginConfig: &cloudfront.DistributionOriginCustomOriginConfigArgs{
				OriginReadTimeout:    pulumi.Int(30),
				OriginProtocolPolicy: pulumi.String("https-only"),
				OriginSslProtocols: pulumi.StringArray{
					pulumi.String("TLSv1.2"),
					pulumi.String("SSLv3"),
				},
				HttpPort:  pulumi.Int(80),
				HttpsPort: pulumi.Int(443),
			},
		})

		orderedCacheBeviours = append(orderedCacheBeviours,
			&cloudfront.DistributionOrderedCacheBehaviorArgs{
				PathPattern: pulumi.Sprintf("api/%s/*", name),
				// rewrite the URL to the nitric service
				FunctionAssociations: cloudfront.DistributionOrderedCacheBehaviorFunctionAssociationArray{
					&cloudfront.DistributionOrderedCacheBehaviorFunctionAssociationArgs{
						EventType:   pulumi.String("viewer-request"),
						FunctionArn: fun.Arn,
					},
				},
				AllowedMethods: pulumi.ToStringArray([]string{"GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"}),
				CachedMethods:  pulumi.ToStringArray([]string{"GET", "HEAD", "OPTIONS"}),
				TargetOriginId: pulumi.String(name),
				ForwardedValues: &cloudfront.DistributionOrderedCacheBehaviorForwardedValuesArgs{
					QueryString: pulumi.Bool(true),
					Cookies: &cloudfront.DistributionOrderedCacheBehaviorForwardedValuesCookiesArgs{
						Forward: pulumi.String("all"),
					},
					// Headers: pulumi.ToStringArray([]string{"*"}),
				},
				ViewerProtocolPolicy: pulumi.String("https-only"),
			},
		)
	}

	// Deploy a CloudFront distribution for the S3 bucket
	_, err = cloudfront.NewDistribution(ctx, "distribution", &cloudfront.DistributionArgs{
		Origins:               origins,
		Enabled:               pulumi.Bool(true),
		DefaultCacheBehavior:  defaultCacheBehaviour,
		DefaultRootObject:     pulumi.String("index.html"),
		OrderedCacheBehaviors: orderedCacheBeviours,
		Restrictions: &cloudfront.DistributionRestrictionsArgs{
			GeoRestriction: &cloudfront.DistributionRestrictionsGeoRestrictionArgs{
				RestrictionType: pulumi.String("none"),
			},
		},
		ViewerCertificate: &cloudfront.DistributionViewerCertificateArgs{
			CloudfrontDefaultCertificate: pulumi.Bool(true),
		},
	})

	if err != nil {
		return err
	}

	return nil
}

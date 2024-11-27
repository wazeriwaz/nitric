package embeds

import (
	_ "embed"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

//go:embed url-rewrite.js
var cloudfront_UrlRewriteFunction string

func GetUrlRewriteFunction() pulumi.StringInput {
	return pulumi.String(cloudfront_UrlRewriteFunction)
}

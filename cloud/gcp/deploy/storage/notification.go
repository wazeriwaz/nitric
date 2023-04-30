// Copyright 2021 Nitric Technologies Pty Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"fmt"

	"github.com/nitrictech/nitric/cloud/azure/deploy/utils"
	common "github.com/nitrictech/nitric/cloud/common/deploy/tags"
	"github.com/nitrictech/nitric/cloud/gcp/deploy/exec"
	v1 "github.com/nitrictech/nitric/core/pkg/api/nitric/v1"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/cloudrun"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/pubsub"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type CloudStorageNotification struct {
	pulumi.ResourceState

	Name         string
	Notification *storage.Notification
}

type CloudStorageNotificationArgs struct {
	Location string
	StackID  pulumi.StringInput

	Bucket   *CloudStorageBucket
	Config   *v1.BucketNotificationConfig
	Function *exec.CloudRunner
}

func notificationTypeToStorageEventType(eventType *v1.BucketNotificationType) []string {
	switch *eventType {
	case v1.BucketNotificationType_All:
		return []string{"OBJECT_FINALIZE", "OBJECT_DELETE"}
	case v1.BucketNotificationType_Created:
		return []string{"OBJECT_FINALIZE"}
	case v1.BucketNotificationType_Deleted:
		return []string{"OBJECT_DELETE"}
	default:
		return []string{}
	}
}

func NewCloudStorageNotification(ctx *pulumi.Context, name string, args *CloudStorageNotificationArgs, opts ...pulumi.ResourceOption) (*CloudStorageNotification, error) {
	res := &CloudStorageNotification{
		Name: name,
	}

	err := ctx.RegisterComponentResource("nitric:bucket:GCPCloudStorageNotification", name, res, opts...)
	if err != nil {
		return nil, err
	}

	topic, err := pubsub.NewTopic(ctx, name+"-topic", &pubsub.TopicArgs{
		Labels: common.Tags(ctx, args.StackID, name),
	})
	if err != nil {
		return nil, err
	}

	invokerAccount, err := serviceaccount.NewAccount(ctx, name+"acct", &serviceaccount.AccountArgs{
		// accountId accepts a max of 30 chars, limit our generated name to this length
		AccountId: pulumi.String(utils.StringTrunc(name, 25) + "acct"),
	}, append(opts, pulumi.Parent(res))...)
	if err != nil {
		return nil, errors.WithMessage(err, "invokerAccount "+name)
	}

	// Apply permissions for the above account to the newly deployed cloud run service
	_, err = cloudrun.NewIamMember(ctx, name+"-notifyrole", &cloudrun.IamMemberArgs{
		Member:   pulumi.Sprintf("serviceAccount:%s", invokerAccount.Email),
		Role:     pulumi.String("roles/run.invoker"),
		Service:  args.Function.Service.Name,
		Location: args.Function.Service.Location,
	}, append(opts, pulumi.Parent(res))...)
	if err != nil {
		return nil, errors.WithMessage(err, "iam member "+name)
	}

	_, err = pubsub.NewSubscription(ctx, name+"-notify", &pubsub.SubscriptionArgs{
		Topic:              topic.Name,
		AckDeadlineSeconds: pulumi.Int(300),
		RetryPolicy: pubsub.SubscriptionRetryPolicyArgs{
			MinimumBackoff: pulumi.String("15s"),
			MaximumBackoff: pulumi.String("600s"),
		},
		PushConfig: pubsub.SubscriptionPushConfigArgs{
			OidcToken: pubsub.SubscriptionPushConfigOidcTokenArgs{
				ServiceAccountEmail: invokerAccount.Email,
			},
			PushEndpoint: pulumi.Sprintf("%s/x-nitric-notification/bucket/%s", args.Function.Url, args.Bucket.Name),
		},
		ExpirationPolicy: &pubsub.SubscriptionExpirationPolicyArgs{
			Ttl: pulumi.String(""),
		},
	}, append(opts, pulumi.Parent(args.Function))...)
	if err != nil {
		return nil, errors.WithMessage(err, "subscription "+name+"-sub")
	}

	// Give the cloud storage service account publishing permissions
	gcsAccount, err := storage.GetProjectServiceAccount(ctx, nil, nil)
	if err != nil {
		return nil, err
	}

	binding, err := pubsub.NewTopicIAMBinding(ctx, name+"-binding", &pubsub.TopicIAMBindingArgs{
		Topic: topic.ID(),
		Role:  pulumi.String("roles/pubsub.publisher"),
		Members: pulumi.StringArray{
			pulumi.String(fmt.Sprintf("serviceAccount:%v", gcsAccount.EmailAddress)),
		},
	})
	if err != nil {
		return nil, errors.WithMessage(err, "topic binding "+name)
	}

	if args.Config == nil {
		return nil, fmt.Errorf("invalid config provided for bucket notification")
	}

	prefix := args.Config.NotificationPrefixFilter
	if prefix == "*" {
		prefix = ""
	}

	res.Notification, err = storage.NewNotification(ctx, name, &storage.NotificationArgs{
		Bucket:           args.Bucket.CloudStorage.Name,
		PayloadFormat:    pulumi.String("JSON_API_V1"),
		Topic:            topic.ID(),
		EventTypes:       pulumi.ToStringArray(notificationTypeToStorageEventType(&args.Config.NotificationType)),
		ObjectNamePrefix: pulumi.String(prefix),
	}, append(opts, pulumi.DependsOn([]pulumi.Resource{binding}))...)
	if err != nil {
		return nil, errors.WithMessage(err, "storage notification "+name)
	}

	return res, nil
}

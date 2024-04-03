package gateway

import (
	"context"
	"io"

	fdk "github.com/fnproject/fdk-go"
	"github.com/nitrictech/nitric/cloud/oci/runtime/resource"
	"github.com/nitrictech/nitric/core/pkg/gateway"
	apispb "github.com/nitrictech/nitric/core/pkg/proto/apis/v1"
)

// A nitric gateway for https://fnproject.io
type FnGateway struct {
	opts *gateway.GatewayStartOpts
}

type FdkHttpRequestContext struct {
}

const OciEventSourceHeader = "x-oci-event-source"

const OciEvents = "Oracle-Cloud-Infrastructure-Events"

func (f *FnGateway) handleApiGatewayRequest(ctx fdk.Context, in io.Reader, out io.Writer) {
	res, err := f.opts.ApiPlugin.HandleRequest("", &apispb.ServerMessage{
		Content: &apispb.ServerMessage_HttpRequest{
			HttpRequest: &apispb.HttpRequest{},
		},
	})

	if err != nil {
		fdk.WriteStatus(out, 500)
		return
	}

	// write out status and headers
	fdk.WriteStatus(out, int(res.GetHttpResponse().GetStatus()))
	for k, v := range res.GetHttpResponse().GetHeaders() {
		for _, val := range v.Value {
			fdk.SetHeader(out, k, val)
		}
	}

	// Write out the body
	out.Write(res.GetHttpResponse().GetBody())
}

func (f *FnGateway) gatewayHandler(ctx context.Context, in io.Reader, out io.Writer) {
	// Get the fn context from execution context
	fnContext := fdk.GetContext(ctx)

	fnContext.ContentType()

	eventSource := fnContext.Header().Get(OciEventSourceHeader)

	switch eventSource {
	case OciEvents:
		// Handle OCI Events

	default:
		// Assume HTTP Event
		// TODO: Locate documentation on event types
	}

	// p := &Person{Name: "World"}
	// json.NewDecoder(in).Decode(p)
	// msg := struct {
	// 	Msg string `json:"message"`
	// }{
	// 	Msg: fmt.Sprintf("Hello %s", p.Name),
	// }
	// json.NewEncoder(out).Encode(&msg)
}

func (f *FnGateway) Start(opts *gateway.GatewayStartOpts) error {
	f.opts = opts
	fdk.Handle(fdk.HandlerFunc(f.gatewayHandler))

	return nil
}

func (f *FnGateway) Stop() error {
	return nil
}

func NewFnGateway(provider *resource.ResourceServer) (gateway.GatewayService, error) {
	return &FnGateway{}, nil
}

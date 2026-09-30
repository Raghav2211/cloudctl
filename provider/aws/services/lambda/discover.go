package lambda

import (
	"cloudctl/snapshot"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// Discover fetches every Lambda function's full configuration (unfiltered),
// for `ctl discover aws`. ListFunctions already returns full detail per
// function in the list call itself, so no extra describe call is needed
// (§10), matching ec2/vpc/rds's list-already-has-everything discovery.
func Discover(ctx context.Context, client lambdaAPI) ([]snapshot.Resource, error) {
	var resources []snapshot.Resource
	var marker *string
	for {
		out, err := client.ListFunctions(ctx, &lambda.ListFunctionsInput{Marker: marker})
		if err != nil {
			return nil, err
		}
		for _, fn := range out.Functions {
			resources = append(resources, snapshot.Resource{ID: derefStr(fn.FunctionName), Type: "lambda:function", Attrs: fn})
		}
		if out.NextMarker == nil {
			break
		}
		marker = out.NextMarker
	}
	return resources, nil
}

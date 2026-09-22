package ec2

import (
	ctltime "cloudctl/time"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// fakeDescribeInstancesClient implements ec2.DescribeInstancesAPIClient,
// returning one page from pages per call, in order.
type fakeDescribeInstancesClient struct {
	pages []*ec2.DescribeInstancesOutput
	err   error
	calls int
}

func (f *fakeDescribeInstancesClient) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.calls >= len(f.pages) {
		return &ec2.DescribeInstancesOutput{}, nil
	}
	page := f.pages[f.calls]
	f.calls++
	return page, nil
}

func testInstance(id, state string) types.Instance {
	launchTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return types.Instance{
		InstanceId: aws.String(id),
		State:      &types.InstanceState{Name: types.InstanceStateName(state)},
		Placement:  &types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
		LaunchTime: &launchTime,
	}
}

func TestFetchInstanceList_HappyPath(t *testing.T) {
	client := &fakeDescribeInstancesClient{
		pages: []*ec2.DescribeInstancesOutput{
			{
				Reservations: []types.Reservation{
					{Instances: []types.Instance{testInstance("i-1", "running"), testInstance("i-2", "stopped")}},
				},
			},
		},
	}

	got, err := fetchInstanceList(context.Background(), client, InstanceListFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 instances, got %d: %+v", len(got), got)
	}
}

func TestFetchInstanceList_EmptyResult(t *testing.T) {
	client := &fakeDescribeInstancesClient{pages: []*ec2.DescribeInstancesOutput{{}}}

	got, err := fetchInstanceList(context.Background(), client, InstanceListFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 instances, got %d", len(got))
	}
}

func TestFetchInstanceList_MultiPageTermination(t *testing.T) {
	nextToken := "token-1"
	client := &fakeDescribeInstancesClient{
		pages: []*ec2.DescribeInstancesOutput{
			{
				NextToken:    &nextToken,
				Reservations: []types.Reservation{{Instances: []types.Instance{testInstance("i-1", "running")}}},
			},
			{
				// no NextToken: this must be the last page
				Reservations: []types.Reservation{{Instances: []types.Instance{testInstance("i-2", "running")}}},
			},
		},
	}

	got, err := fetchInstanceList(context.Background(), client, InstanceListFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 instances across 2 pages, got %d", len(got))
	}
	if client.calls != 2 {
		t.Fatalf("expected the paginator to stop after 2 calls, made %d", client.calls)
	}
}

func TestFetchInstanceList_APIError(t *testing.T) {
	client := &fakeDescribeInstancesClient{err: errors.New("boom")}

	if _, err := fetchInstanceList(context.Background(), client, InstanceListFilter{}); err == nil {
		t.Fatal("expected an error from a failing DescribeInstances call, got nil")
	}
}

func TestFetchInstanceList_AppliesCustomFilter(t *testing.T) {
	client := &fakeDescribeInstancesClient{
		pages: []*ec2.DescribeInstancesOutput{
			{
				Reservations: []types.Reservation{
					{Instances: []types.Instance{testInstance("i-1", "running")}}, // no public IP
				},
			},
		},
	}

	filter := *NewInstanceFilter(WithHasPublicIp())
	got, err := fetchInstanceList(context.Background(), client, filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected the instance without a public IP to be filtered out, got %d", len(got))
	}
}

func TestInstanceListFetcher_Fetch_HappyPath(t *testing.T) {
	client := &fakeDescribeInstancesClient{
		pages: []*ec2.DescribeInstancesOutput{
			{
				Reservations: []types.Reservation{
					{Instances: []types.Instance{testInstance("i-1", "running"), testInstance("i-2", "running")}},
				},
			},
		},
	}
	f := instanceListFetcher{client: client, tz: ctltime.GetTZ("utc"), filter: InstanceListFilter{}}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.instancesByState["running"]) != 2 {
		t.Fatalf("expected 2 running instances, got %+v", out.instancesByState)
	}
}

func TestInstanceListFetcher_Fetch_NoInstancesFound(t *testing.T) {
	client := &fakeDescribeInstancesClient{pages: []*ec2.DescribeInstancesOutput{{}}}
	f := instanceListFetcher{client: client, tz: ctltime.GetTZ("utc"), filter: InstanceListFilter{}}

	out, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when no instances are found, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil output alongside the error, got %+v", out)
	}
}

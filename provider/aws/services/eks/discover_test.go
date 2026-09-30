package eks

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/eks"
)

func TestDiscover_HappyPath(t *testing.T) {
	client := &fakeEKSClient{listOut: &eks.ListClustersOutput{Clusters: []string{"prod", "staging"}}}

	resources, err := Discover(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	for _, r := range resources {
		if r.Type != "eks:cluster" {
			t.Errorf("expected type eks:cluster, got %q", r.Type)
		}
	}
}

func TestDiscover_EmptyIsNotAnError(t *testing.T) {
	client := &fakeEKSClient{listOut: &eks.ListClustersOutput{}}

	resources, err := Discover(context.Background(), client)
	if err != nil {
		t.Fatalf("expected zero clusters to not be an error for discovery, got %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestDiscover_APIError(t *testing.T) {
	client := &fakeEKSClient{listErr: errors.New("boom")}

	_, err := Discover(context.Background(), client)
	if err == nil {
		t.Fatal("expected an error from a failing ListClusters call, got nil")
	}
}

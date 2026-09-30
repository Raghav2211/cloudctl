package concept

import "testing"

func TestOf_KnownAndUnknownTypes(t *testing.T) {
	cases := []struct {
		resourceType string
		want         Concept
	}{
		{"ec2:instance", Compute},
		{"s3:bucket", Storage},
		{"dynamodb:table", Database},
		{"rds:instance", Database},
		{"rds:cluster", Database},
		{"eks:cluster", ContainerCluster},
		{"vpc:vpc", Network},
		{"lambda:function", Function},
		{"gcp:some-future-type", Unknown},
	}
	for _, c := range cases {
		if got := Of(c.resourceType); got != c.want {
			t.Errorf("Of(%q) = %q, want %q", c.resourceType, got, c.want)
		}
	}
}

func TestResourceTypes_RoundTripsWithOf(t *testing.T) {
	for _, c := range All() {
		types := ResourceTypes(c)
		if len(types) == 0 {
			t.Errorf("expected at least one resource type mapped to concept %q", c)
		}
		for _, rt := range types {
			if got := Of(rt); got != c {
				t.Errorf("ResourceTypes(%q) included %q, but Of(%q) = %q", c, rt, rt, got)
			}
		}
	}
}

func TestAll_ExcludesUnknown(t *testing.T) {
	for _, c := range All() {
		if c == Unknown {
			t.Error("expected All() to never include Unknown")
		}
	}
}

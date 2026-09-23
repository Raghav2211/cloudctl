package eks

import "fmt"

func NoClusterFound() error {
	return fmt.Errorf("no EKS cluster found")
}

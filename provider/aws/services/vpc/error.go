package vpc

import "fmt"

func NoVPCFound() error {
	return fmt.Errorf("no VPC found")
}

func VPCNotFound(vpcID string) error {
	return fmt.Errorf("no VPC found with id %s", vpcID)
}

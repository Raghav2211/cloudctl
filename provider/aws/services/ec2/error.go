package ec2

import "fmt"

func NoInstanceFound() error {
	return fmt.Errorf("no instance found")
}

func NoSecurityGroupFound(sgId string) error {
	return fmt.Errorf("no security group found for %s", sgId)
}

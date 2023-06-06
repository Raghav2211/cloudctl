package ec2

import "fmt"

func NoInstanceFound() error {
	return fmt.Errorf("no instance found")
}

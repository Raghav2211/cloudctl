package rds

import "fmt"

func NoDatabaseFound() error {
	return fmt.Errorf("no RDS instance or cluster found")
}

func DatabaseNotFound(identifier string) error {
	return fmt.Errorf("no RDS instance or cluster found with identifier %s", identifier)
}

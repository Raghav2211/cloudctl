package rds

import "fmt"

func NoDatabaseFound() error {
	return fmt.Errorf("no RDS instance or cluster found")
}

func DatabaseNotFound(identifier string) error {
	return fmt.Errorf("no RDS instance or cluster found with identifier %s", identifier)
}

func NoEventsFound(identifier string) error {
	return fmt.Errorf("no events found for %s in the given window", identifier)
}

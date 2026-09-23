package dynamodb

import "fmt"

func NoTableFound() error {
	return fmt.Errorf("no table found")
}

func TableNotFound(tableName string) error {
	return fmt.Errorf("table %s not found", tableName)
}

package producer_test

import (
	"fmt"
	"testing"

	"github.com/yeastengine/ella/internal/ausf/producer"
)

func TestGenerateRandomNumber(t *testing.T) {
	value, err := producer.GenerateRandomNumber()
	if err != nil {
		t.Fatalf("GenerateRandomNumber() failed: %s", err)
	}

	fmt.Printf("Random number: %d\n", value)
}

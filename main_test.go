package main

import (
	"errors"
	"log"
	"testing"
)

func fakeHealthyPollClusterHealth(string, *log.Logger) (bool, []string, error) {
	return true, []string{}, nil
}

func fakeUnhealthyPollClusterHealth(string, *log.Logger) (bool, []string, error) {
	return false, []string{}, errors.New("failed")
}

func TestIsClusterHealthy(t *testing.T) {
	tests := []struct {
		fakePollClusterHealth func(string, *log.Logger) (bool, []string, error)
		expectedStatus        bool
		expectErr             bool
	}{
		{
			fakePollClusterHealth: fakeHealthyPollClusterHealth,
			expectedStatus:        true,
			expectErr:             false,
		},
		{
			fakePollClusterHealth: fakeUnhealthyPollClusterHealth,
			expectedStatus:        false,
			expectErr:             true,
		},
	}

	for _, test := range tests {
		actual, err := isClusterHealthy(test.fakePollClusterHealth, 2, 0)
		if err != nil {
			if !test.expectErr {
				t.Fatal(err)
			}
		}
		if actual != test.expectedStatus {
			t.Fatalf("expected: %v, got %v", test.expectedStatus, actual)
		}
	}
}

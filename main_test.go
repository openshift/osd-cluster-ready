package main

import (
	"errors"
	"log"
	"testing"
)

func TestGetEnvInt(t *testing.T) {
	t.Setenv("works", "0")
	t.Setenv("default", "")
	t.Setenv("error", "one")
	defaultVal := -1

	tests := []struct {
		key       string
		expected  int
		expectErr bool
	}{
		{
			key:       "works",
			expected:  0,
			expectErr: false,
		},
		{
			key:       "default",
			expected:  defaultVal,
			expectErr: false,
		},
		{
			key:       "error",
			expectErr: true,
		},
	}

	for _, test := range tests {
		actual, err := getEnvInt(test.key, defaultVal)
		if err != nil {
			if !test.expectErr {
				t.Fatal(err)
			}
		}
		if test.expected != actual {
			t.Fatalf("expected %d, got %d", test.expected, actual)
		}
	}
}

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

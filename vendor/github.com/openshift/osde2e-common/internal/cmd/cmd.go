package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Run executes the os.exec command provided
func Run(command *exec.Cmd) (bytes.Buffer, bytes.Buffer, error) {
	var stdout, stderr bytes.Buffer

	// TODO: Configure tee output to file and buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Start()
	if err != nil {
		return stdout, stderr, fmt.Errorf("failed to start command: %v", err)
	}

	err = command.Wait()
	if err != nil {
		return stdout, stderr, fmt.Errorf("failed to wait for command to finish: %v", err)
	}

	return stdout, stderr, nil
}

// ConvertOutputToMap converts a json string formatted to a map object
func ConvertOutputToMap(data bytes.Buffer) (map[string]any, error) {
	var result map[string]any
	err := json.Unmarshal(data.Bytes(), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ConvertOutputToListOfMaps converts a list of json string formatted to a list of map objects
func ConvertOutputToListOfMaps(data bytes.Buffer) ([]map[string]any, error) {
	var result []map[string]any
	err := json.Unmarshal(data.Bytes(), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

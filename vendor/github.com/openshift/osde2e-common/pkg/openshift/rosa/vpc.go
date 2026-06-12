package rosa

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// vpc represents the details of an aws vpc
type vpc struct {
	privateSubnet     string
	publicSubnet      string
	nodePrivateSubnet string
	stackName         string
}

// vpcError represents the custom error
type vpcError struct {
	action string
	err    error
}

// Error returns the formatted error message when vpcError is invoked
func (h *vpcError) Error() string {
	return fmt.Sprintf("%s vpc failed: %v", h.action, h.err)
}

// createVPC creates the aws vpc using rosa create network command
func (r *Provider) createVPC(ctx context.Context, clusterName string, hostedCP, privateLink bool) (*vpc, error) {
	const action = "create"

	var vpc vpc

	if clusterName == "" {
		return nil, &vpcError{action: action, err: errors.New("clusterName is empty")}
	}

	// Generate stack name
	stackName := fmt.Sprintf("%s-vpc", clusterName)
	vpc.stackName = stackName

	r.log.Info("Creating aws vpc using rosa create network", clusterNameLoggerKey, clusterName, awsRegionLoggerKey, r.awsConfig.Region)

	// Get availability zones for the region
	azs, err := r.getAvailabilityZones(ctx)
	if err != nil {
		return nil, &vpcError{action: action, err: fmt.Errorf("failed to get availability zones: %v", err)}
	}

	// Determine AZ count based on cluster type
	azCount := min(len(azs), 2) // Default for both hosted CP and private link

	// Build rosa create network command
	commandArgs := []string{
		"create", "network", "rosa-quickstart-default-vpc",
		"--region", r.awsConfig.Region,
		"--param", fmt.Sprintf("Region=%s", r.awsConfig.Region),
		"--param", fmt.Sprintf("Name=%s", stackName),
		"--param", fmt.Sprintf("AvailabilityZoneCount=%d", azCount),
		"--param", "VpcCidr=10.0.0.0/16",
		"--mode", "auto",
		"--yes",
	}

	// Add availability zones
	for i := 0; i < azCount && i < len(azs); i++ {
		commandArgs = append(commandArgs, "--param", fmt.Sprintf("AZ%d=%s", i+1, azs[i]))
	}

	// Execute rosa create network
	_, stderr, err := r.RunCommand(ctx, exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
	if err != nil {
		return nil, &vpcError{action: action, err: fmt.Errorf("rosa create network failed: %v, stderr: %v", err, stderr.String())}
	}

	outputs, err := r.getStackOutput(ctx, stackName)
	if err != nil {
		return nil, &vpcError{action: action, err: fmt.Errorf("get stack output: %v", err)}
	}

	// Extract subnet IDs from outputs
	if err = r.extractSubnetIds(&vpc, outputs, hostedCP, privateLink); err != nil {
		return nil, &vpcError{action: action, err: fmt.Errorf("extracting subnetids from %v: %v", outputs, err)}
	}

	r.log.Info("AWS vpc created", clusterNameLoggerKey, clusterName, "stackName", stackName)

	return &vpc, nil
}

// deleteVPC deletes the aws vpc by deleting the CloudFormation stack
func (r *Provider) deleteVPC(ctx context.Context, clusterName string) error {
	const action = "delete"

	if clusterName == "" {
		return &vpcError{action: action, err: errors.New("one or more parameters is empty")}
	}

	stackName := fmt.Sprintf("%s-vpc", clusterName)

	r.log.Info("Deleting AWS vpc", clusterNameLoggerKey, clusterName, awsRegionLoggerKey, r.awsConfig.Region, "stackName", stackName)

	cfnClient := cloudformation.NewFromConfig(r.awsConfig)

	_, err := cfnClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return &vpcError{action: action, err: fmt.Errorf("failed to delete CloudFormation stack: %v", err)}
	}

	if err = r.waitForStackDeletion(ctx, stackName); err != nil {
		return &vpcError{action: action, err: fmt.Errorf("failed waiting for stack deletion: %v", err)}
	}

	r.log.Info("AWS vpc deleted", clusterNameLoggerKey, clusterName, "stackName", stackName)

	return nil
}

// getAvailabilityZones retrieves available AZs for the region
func (r *Provider) getAvailabilityZones(ctx context.Context) ([]string, error) {
	ec2Client := ec2.NewFromConfig(r.awsConfig)

	result, err := ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []ec2Types.Filter{
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe availability zones: %v", err)
	}

	var azs []string
	for _, az := range result.AvailabilityZones {
		if az.ZoneName != nil {
			azs = append(azs, *az.ZoneName)
		}
	}

	return azs, nil
}

// waitForStackDeletion waits for CloudFormation stack to complete deletion
func (r *Provider) waitForStackDeletion(ctx context.Context, stackName string) error {
	cfnClient := cloudformation.NewFromConfig(r.awsConfig)

	waiter := cloudformation.NewStackDeleteCompleteWaiter(cfnClient)
	err := waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, 30*time.Minute) // 30 minute timeout

	if err != nil {
		return fmt.Errorf("failed waiting for stack deletion: %v", err)
	}

	return nil
}

// getStackOutput retrieves CloudFormation stack outputs
func (r *Provider) getStackOutput(ctx context.Context, stackName string) ([]types.Output, error) {
	cfnClient := cloudformation.NewFromConfig(r.awsConfig)

	// Describe stacks to get outputs
	result, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe stacks: %v", err)
	}

	if len(result.Stacks) == 0 {
		return nil, fmt.Errorf("no stacks found with name: %s", stackName)
	}

	return result.Stacks[0].Outputs, nil
}

// extractSubnetIds extracts subnet IDs from CloudFormation stack outputs
func (r *Provider) extractSubnetIds(vpc *vpc, outputs []types.Output, hostedCP, privateLink bool) error {
	outputMap := make(map[string]string)
	for _, output := range outputs {
		if output.OutputKey != nil && output.OutputValue != nil {
			outputMap[*output.OutputKey] = *output.OutputValue
		}
	}

	// Extract subnet IDs based on the template outputs
	// https://github.com/openshift/rosa/blob/88022b4b793571f66566efaecae86b6cf4392ed4/cmd/create/network/templates/rosa-quickstart-default-vpc/cloudformation.yaml#L601

	privateSubnets := strings.Split(outputMap["PrivateSubnets"], ",")
	publicSubnets := strings.Split(outputMap["PublicSubnets"], ",")

	vpc.privateSubnet = privateSubnets[0]
	vpc.publicSubnet = publicSubnets[0]

	// For hosted control plane, we need a second private subnet for nodes
	if hostedCP {
		if len(privateSubnets) < 2 {
			return fmt.Errorf("not enough private subnets created (required two): %v", privateSubnets)
		}
		vpc.nodePrivateSubnet = privateSubnets[1]
	}
	return nil
}

package aws

import (
	"errors"
	"fmt"
	"os"
)

// AWSCredentials contains the data to be used to authenticate with aws
type AWSCredentials struct {
	AccessKeyID     string
	Profile         string
	Region          string
	SecretAccessKey string
}

// priority determines the priority of which credentials are used
func (c *AWSCredentials) priority() (int, error) {
	switch {
	case c.Profile != "":
		return 0, nil
	case c.AccessKeyID != "" && c.SecretAccessKey != "":
		return 1, nil
	}

	return -1, errors.New("no credentials are set, unable to determine priority")
}

// Set validates the aws credentials/ensures they are set
// Data can be passed as a parameter or fetched from the environment
func (c *AWSCredentials) Set() error {
	if *c == (AWSCredentials{}) {
		c.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
		c.Profile = os.Getenv("AWS_PROFILE")
		c.Region = os.Getenv("AWS_REGION")
		c.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}

	setByAccessKeys := true
	setByProfile := true

	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		setByAccessKeys = false
	}

	if c.Profile == "" {
		setByProfile = false
	}

	if !setByAccessKeys && !setByProfile {
		return errors.New("credentials are not supplied")
	}

	if c.Region == "" {
		return errors.New("region is not supplied")
	}

	return nil
}

// CredentialsAsList returns aws credentials as a list formatted as key=value
func (c *AWSCredentials) CredentialsAsList() []string {
	priorityLevel, _ := c.priority()

	switch priorityLevel {
	case 0:
		return []string{
			fmt.Sprintf("AWS_PROFILE=%s", c.Profile),
			fmt.Sprintf("AWS_REGION=%s", c.Region),
		}
	case 1:
		return []string{
			fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", c.AccessKeyID),
			fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", c.SecretAccessKey),
			fmt.Sprintf("AWS_REGION=%s", c.Region),
		}
	default:
		return []string{}
	}
}

// CredentialsAsMap returns aws credentials as a map
func (c *AWSCredentials) CredentialsAsMap() map[string]string {
	priorityLevel, _ := c.priority()

	switch priorityLevel {
	case 0:
		return map[string]string{
			"AWS_PROFILE": c.Profile,
			"AWS_REGION":  c.Region,
		}
	case 1:
		return map[string]string{
			"AWS_ACCESS_KEY_ID":     c.AccessKeyID,
			"AWS_SECRET_ACCESS_KEY": c.SecretAccessKey,
			"AWS_REGION":            c.Region,
		}
	default:
		return map[string]string{}
	}
}

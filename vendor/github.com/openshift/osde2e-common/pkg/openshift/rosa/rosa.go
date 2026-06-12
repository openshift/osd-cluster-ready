package rosa

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/openshift/osde2e-common/internal/cmd"
	ocmclient "github.com/openshift/osde2e-common/pkg/clients/ocm"
	awscloud "github.com/openshift/osde2e-common/pkg/clouds/aws"
)

const (
	downloadURL = "https://mirror.openshift.com/pub/openshift-v4/clients/rosa"
)

// accountInfo represents the rosa whoami command response
type accountInfo struct {
	AWSAccountID     string `json:"AWS Account ID"`
	AWSDefaultRegion string `json:"AWS Default Region"`
}

// Provider is a rosa provider
type Provider struct {
	*ocmclient.Client
	awsCredentials *awscloud.AWSCredentials
	ocmEnvironment ocmclient.Environment
	log            logr.Logger

	AWSRegion  string
	rosaBinary string
	user       *accountInfo
	awsConfig  aws.Config

	fedRamp bool
}

// providerError represents the provider custom error
type providerError struct {
	err error
}

// Error returns the formatted error message when providerError is invoked
func (r *providerError) Error() string {
	return fmt.Sprintf("failed to construct rosa provider: %v", r.err)
}

// RunCommand runs the rosa command provided
func (r *Provider) RunCommand(ctx context.Context, command *exec.Cmd) (bytes.Buffer, bytes.Buffer, error) {
	command.Env = append(command.Environ(), r.awsCredentials.CredentialsAsList()...)
	r.log.Info("Command", rosaCommandLoggerKey, command.String())
	return cmd.Run(command)
}

// whoami runs 'rosa whoami -o json' and parses the response
func (r *Provider) whoami(ctx context.Context) (*accountInfo, error) {
	commandArgs := []string{"whoami", "-o", "json"}

	stdout, stderr, err := r.RunCommand(ctx, exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
	if err != nil {
		return nil, fmt.Errorf("failed to run rosa whoami: %v, stderr: %s", err, stderr.String())
	}

	acctInfo := &accountInfo{}
	if err := json.Unmarshal(stdout.Bytes(), acctInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal whoami output: %v", err)
	}

	return acctInfo, nil
}

// Uninstall removes the rosa cli that was downloaded to the systems temp directory
func (r *Provider) Uninstall(ctx context.Context) error {
	if strings.Contains(r.rosaBinary, os.TempDir()) {
		return os.Remove(r.rosaBinary)
	}
	return nil
}

// cliCheck checks if rosa cli is available else it will download it
func cliCheck() (string, error) {
	var (
		url             = fmt.Sprintf("%s/latest", downloadURL)
		rosaFilename    = fmt.Sprintf("%s/rosa", os.TempDir())
		rosaTarFilePath = fmt.Sprintf("%s/rosa.tar.gz", os.TempDir())
	)

	defer func() {
		_ = os.Remove(rosaTarFilePath)
	}()

	runtimeOS := runtime.GOOS
	switch runtimeOS {
	case "linux":
		url = fmt.Sprintf("%s/rosa-linux.tar.gz", url)
	case "darwin":
		url = fmt.Sprintf("%s/rosa-macosx.tar.gz", url)
	default:
		return "", fmt.Errorf("operating system %q is not supported", runtimeOS)
	}

	path, err := exec.LookPath("rosa")
	if path != "" && err == nil {
		return path, nil
	}

	retryClient := retryablehttp.NewClient()
	retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		ok, e := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
		if !ok && resp.StatusCode == http.StatusRequestTimeout {
			return true, nil
		}
		return ok, e
	}

	response, err := retryClient.Get(url)
	if err != nil || response.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("failed to download %s: %v", url, err)
	}
	defer func() {
		if err = response.Body.Close(); err != nil {
			panic(err)
		}
	}()

	tarFile, err := os.Create(rosaTarFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create %s tar file: %v", rosaTarFilePath, err)
	}
	defer func() {
		if err = tarFile.Close(); err != nil {
			panic(err)
		}
	}()

	rosaFile, err := os.Create(rosaFilename)
	if err != nil {
		return "", fmt.Errorf("failed to create %s tar file: %v", rosaFilename, err)
	}

	err = os.Chmod(rosaFilename, 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to set file permissions to 0755 for %s: %v", rosaFilename, err)
	}

	defer func() {
		if err = rosaFile.Close(); err != nil {
			panic(err)
		}
	}()

	_, err = io.Copy(tarFile, response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write content to %s: %v", rosaTarFilePath, err)
	}

	tarFileReader, err := os.Open(rosaTarFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open %s: %v", rosaTarFilePath, err)
	}
	defer func() {
		if err = tarFileReader.Close(); err != nil {
			panic(err)
		}
	}()

	gzipReader, err := gzip.NewReader(tarFileReader)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader for %s: %v", rosaTarFilePath, err)
	}
	defer func() {
		if err = gzipReader.Close(); err != nil {
			panic(err)
		}
	}()

	tarReader := tar.NewReader(gzipReader)

	for {
		_, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			break
		}
		_, err = io.Copy(rosaFile, tarReader)
		if err != nil {
			break
		}
	}

	return rosaFilename, nil
}

// getVersion gets the rosa cli version
func getVersion(ctx context.Context, rosaBinary string) (string, error) {
	stdout, _, err := cmd.Run(exec.CommandContext(ctx, rosaBinary, "version"))
	if err != nil {
		return "", err
	}

	versionSlice := strings.SplitAfter(stdout.String(), "\n")
	if len(versionSlice) == 0 {
		return "", errors.New("getVersion failed to get version from cli standard out")
	}

	return strings.ReplaceAll(versionSlice[0], "\n", ""), nil
}

// verifyLogin validates the authentication details provided are valid by logging in with rosa cli
func verifyLogin(ctx context.Context, rosaBinary string, token string, clientID string, clientSecret string, ocmEnvironment ocmclient.Environment, awsCredentials *awscloud.AWSCredentials) error {
	commandArgs := []string{"login"}

	command := exec.CommandContext(ctx, rosaBinary, commandArgs...)
	command.Env = append(command.Environ(), awsCredentials.CredentialsAsList()...)

	if clientID != "" && clientSecret != "" {
		command.Args = append(command.Args, "--client-id", clientID)
		command.Args = append(command.Args, "--client-secret", clientSecret)
		// TODO: Work around. The rosa cli for govcloud does not support the --env passing the api endpoint.
		// The environment selection can be handled with a data structure that maps the environment to the api endpoint.
		if ocmEnvironment == "https://api.int.openshiftusgov.com" {
			command.Args = append(command.Args, "--govcloud")
			ocmEnvironment = "integration"
		}
	} else if token != "" {
		command.Args = append(command.Args, "--token", token)
	} else {
		return fmt.Errorf("no authentication details provided")
	}
	/*
		The OCM_CONFIG variable is part of the ocm-cli functionality, for more information this is the description ocm-cli repo.
		https://github.com/openshift-online/ocm-cli?tab=readme-ov-file#multiple-concurrent-logins-with-ocm_config
	*/
	command.Env = append(command.Env, fmt.Sprintf("OCM_CONFIG=%s/ocm.json", os.TempDir()))
	command.Args = append(command.Args, "--env", string(ocmEnvironment))
	command.Args = append(command.Args, "--region", string(awsCredentials.Region))

	_, stderr, err := cmd.Run(command)
	if err != nil {
		return fmt.Errorf("login failed with %q: %w", stderr.String(), err)
	}

	return nil
}

// New handles constructing the rosa provider which creates a connection
// to openshift cluster manager "ocm". It is the callers responsibility
// to close the ocm connection when they are finished (defer provider.Connection.Close())
func New(ctx context.Context, token string, clientID string, clientSecret string, ocmEnvironment ocmclient.Environment, logger logr.Logger, args ...*awscloud.AWSCredentials) (*Provider, error) {
	if ocmEnvironment == "" || (token == "" && (clientID == "" || clientSecret == "")) {
		return nil, &providerError{err: errors.New("some parameters are undefined, unable to construct osd provider")}
	}

	rosaBinary, err := cliCheck()
	if err != nil {
		return nil, &providerError{err: err}
	}

	version, err := getVersion(ctx, rosaBinary)
	if err != nil {
		return nil, &providerError{err: err}
	}

	logger.Info("ROSA version", "version", version)

	awsCredentials := &awscloud.AWSCredentials{}
	if len(args) == 1 {
		awsCredentials = args[0]
	}

	err = awsCredentials.Set()
	if err != nil {
		return nil, &providerError{err: fmt.Errorf("aws credential set and validation failed: %v", err)}
	}
	isFedRamp := strings.Contains(awsCredentials.Region, "gov")

	err = verifyLogin(ctx, rosaBinary, token, clientID, clientSecret, ocmEnvironment, awsCredentials)
	if err != nil {
		return nil, &providerError{err: err}
	}

	provider := &Provider{
		awsCredentials: awsCredentials,
		fedRamp:        isFedRamp,
		ocmEnvironment: ocmEnvironment,
		rosaBinary:     rosaBinary,
		Client:         nil,
		log:            logger,
	}

	// Get user information via rosa whoami
	acctInfo, err := provider.whoami(ctx)
	if err != nil {
		return nil, &providerError{err: fmt.Errorf("failed to get user information: %v", err)}
	}
	provider.user = acctInfo

	if awsCredentials.Region == "random" {
		// Set a temporary region to select a random region later on
		awsCredentials.Region = "us-east-1"
		awsCredentials.Region, err = provider.selectRandomRegion(ctx)
		if err != nil {
			return nil, &providerError{err: err}
		}
	}

	provider.AWSRegion = awsCredentials.Region
	provider.awsConfig, err = provider.createAWSConfig(ctx, provider.AWSRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS config: %v", err)
	}

	provider.Client, err = ocmclient.New(ctx, token, clientID, clientSecret, ocmEnvironment)
	if err != nil {
		return nil, &providerError{err: err}
	}

	return provider, nil
}

// createAWSConfig creates AWS SDK configuration
func (r *Provider) createAWSConfig(ctx context.Context, awsRegion string) (aws.Config, error) {
	var awsConfig aws.Config
	var err error

	// Configure AWS SDK based on credential type
	awsCredentials := r.awsCredentials.CredentialsAsMap()

	if profile, exists := awsCredentials["AWS_PROFILE"]; exists {
		// Use profile-based configuration
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(awsRegion),
			config.WithSharedConfigProfile(profile),
		)
	} else {
		// Use access key configuration
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(awsRegion),
		)
		// The SDK will automatically use AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY from environment
	}

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %v", err)
	}

	return awsConfig, nil
}

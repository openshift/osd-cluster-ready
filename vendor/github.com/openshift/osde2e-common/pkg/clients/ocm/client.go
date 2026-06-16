package ocm

import (
	"context"
	"fmt"
	"strings"

	ocmsdk "github.com/openshift-online/ocm-sdk-go"
)

type Environment string

const (
	Production         Environment = "https://api.openshift.com"
	Stage              Environment = "https://api.stage.openshift.com"
	Integration        Environment = "https://api.integration.openshift.com"
	FedRampProduction  Environment = "https://api.openshiftusgov.com"
	FedRampStage       Environment = "https://api.stage.openshiftusgov.com"
	FedRampIntegration Environment = "https://api.int.openshiftusgov.com"
	tokenURL           string      = "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"
	fedrampTokenURL    string      = "https://sso.int.openshiftusgov.com/realms/redhat-external/protocol/openid-connect/token"
)

type Client struct {
	*ocmsdk.Connection
}

func New(ctx context.Context,
	token string,
	clientID string,
	clientSecret string,
	environment Environment,
) (*Client, error) {
	connectionBuilder := ocmsdk.NewConnectionBuilder().URL(string(environment))

	if strings.Contains(string(environment), "fr") {
		connectionBuilder.Client(clientID, clientSecret).
			TokenURL(fedrampTokenURL)
	} else if token != "" {
		connectionBuilder.TokenURL(tokenURL).Client("cloud-services", "").Tokens(token)
	} else {
		connectionBuilder.Client(clientID, clientSecret)
	}

	connection, err := connectionBuilder.BuildContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create ocm connection: %w", err)
	}

	return &Client{connection}, nil
}

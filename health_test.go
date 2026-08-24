package main

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	fakeConfig "github.com/openshift/client-go/config/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func apiServer(namedCerts ...configv1.APIServerNamedServingCert) *configv1.APIServer {
	return &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiServerName,
		},
		Spec: configv1.APIServerSpec{
			ServingCerts: configv1.APIServerServingCerts{
				NamedCertificates: namedCerts,
			},
		},
	}
}

func namedCert(secretName string) configv1.APIServerNamedServingCert {
	return configv1.APIServerNamedServingCert{
		ServingCertificate: configv1.SecretNameReference{Name: secretName},
	}
}

func TestCheckCerts(t *testing.T) {
	tests := []struct {
		description   string
		expected      bool
		expectedError bool
		objs          []runtime.Object
	}{
		{
			description:   "no APIServer",
			expected:      false,
			expectedError: true,
		},
		{
			description: "APIServer with no named certificates",
			expected:    false,
			objs:        []runtime.Object{apiServer()},
		},
		{
			description: "APIServer with empty serving certificate name",
			expected:    false,
			objs:        []runtime.Object{apiServer(namedCert(""))},
		},
		{
			description: "APIServer with one named certificate",
			expected:    true,
			objs:        []runtime.Object{apiServer(namedCert("test-primary-cert-bundle-secret"))},
		},
		{
			description: "APIServer with two named certificates",
			expected:    true,
			objs: []runtime.Object{apiServer(
				namedCert("test-primary-cert-bundle-secret"),
				namedCert("test-additional-cert-bundle-secret"),
			)},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			cfgClient := fakeConfig.NewSimpleClientset(test.objs...)
			state, err := checkCerts(cfgClient.ConfigV1(), nil)
			if err != nil && !test.expectedError {
				t.Fatalf("unexpected error: %s", err)
			}
			if err == nil && test.expectedError {
				t.Fatal("expected an error, got none")
			}
			if state != test.expected {
				t.Fatalf("expected %v, got %v", test.expected, state)
			}
		})
	}
}

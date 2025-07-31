package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// TestAWSCredentialValidation tests AWS credential validation against a mock STS service
func TestAWSCredentialValidation(t *testing.T) {
	// Create mock STS server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == "POST" {
			// Mock GetCallerIdentity response
			response := `<?xml version="1.0" encoding="UTF-8"?>
<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
    <GetCallerIdentityResult>
        <Arn>arn:aws:iam::123456789012:user/test-user</Arn>
        <UserId>AIDACKCEVSQ6C2EXAMPLE</UserId>
        <Account>123456789012</Account>
    </GetCallerIdentityResult>
    <ResponseMetadata>
        <RequestId>01234567-89ab-cdef-0123-456789abcdef</RequestId>
    </ResponseMetadata>
</GetCallerIdentityResponse>`
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	// Create AWS config with mock endpoint
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"AKIATEST123",
			"test-secret-key",
			"",
		)),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:           mockServer.URL,
					SigningRegion: region,
				}, nil
			})),
	)
	if err != nil {
		t.Fatalf("Failed to create AWS config: %v", err)
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Test credential validation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Fatalf("Expected successful credential validation, got error: %v", err)
	}

	// Verify response
	if result.Account == nil || *result.Account != "123456789012" {
		t.Errorf("Expected account 123456789012, got %v", result.Account)
	}
	if result.UserId == nil || *result.UserId != "AIDACKCEVSQ6C2EXAMPLE" {
		t.Errorf("Expected user ID AIDACKCEVSQ6C2EXAMPLE, got %v", result.UserId)
	}
}

// TestAWSCredentialValidationFailure tests AWS credential validation failure
func TestAWSCredentialValidationFailure(t *testing.T) {
	// Create mock STS server that returns unauthorized error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 403 Forbidden
		response := `<?xml version="1.0" encoding="UTF-8"?>
<ErrorResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
    <Error>
        <Type>Sender</Type>
        <Code>InvalidUserID.NotFound</Code>
        <Message>The security token included in the request is invalid</Message>
    </Error>
    <RequestId>01234567-89ab-cdef-0123-456789abcdef</RequestId>
</ErrorResponse>`
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(response))
	}))
	defer mockServer.Close()

	// Create AWS config with mock endpoint and invalid credentials
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"AKIAINVALID123",
			"invalid-secret-key",
			"",
		)),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:           mockServer.URL,
					SigningRegion: region,
				}, nil
			})),
	)
	if err != nil {
		t.Fatalf("Failed to create AWS config: %v", err)
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Test credential validation failure
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err == nil {
		t.Error("Expected credential validation to fail with invalid credentials")
	}

	// Check that we got an error (specific error type may vary by AWS SDK version)
	t.Logf("Got expected error: %v", err)
}

// TestAWSSessionTokenValidation tests temporary credential validation
func TestAWSSessionTokenValidation(t *testing.T) {
	// Create mock STS server for session token validation
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for session token in the request (can be in header or query)
		authHeader := r.Header.Get("Authorization")
		securityTokenHeader := r.Header.Get("X-Amz-Security-Token")
		securityTokenQuery := r.URL.Query().Get("X-Amz-Security-Token")

		// Accept if session token is present in any form
		if !strings.Contains(authHeader, "X-Amz-Security-Token") &&
			securityTokenHeader == "" && securityTokenQuery == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// Mock successful response with session token
		response := `<?xml version="1.0" encoding="UTF-8"?>
<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
    <GetCallerIdentityResult>
        <Arn>arn:aws:sts::123456789012:assumed-role/test-role/test-session</Arn>
        <UserId>AROAEXAMPLE:test-session</UserId>
        <Account>123456789012</Account>
    </GetCallerIdentityResult>
    <ResponseMetadata>
        <RequestId>01234567-89ab-cdef-0123-456789abcdef</RequestId>
    </ResponseMetadata>
</GetCallerIdentityResponse>`
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer mockServer.Close()

	// Create AWS config with session token
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"ASIATEST123",
			"test-secret-key",
			"test-session-token",
		)),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:           mockServer.URL,
					SigningRegion: region,
				}, nil
			})),
	)
	if err != nil {
		t.Fatalf("Failed to create AWS config: %v", err)
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Test session token validation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Fatalf("Expected successful session token validation, got error: %v", err)
	}

	// Verify it's an assumed role ARN (indicating session token usage)
	if result.Arn == nil || !strings.Contains(*result.Arn, "assumed-role") {
		t.Errorf("Expected assumed-role ARN indicating session token, got %v", result.Arn)
	}
}

// TestRoute53DNSChallenge tests Route53 DNS challenge simulation
func TestRoute53DNSChallenge(t *testing.T) {
	// This test simulates Route53 DNS challenge workflow
	// In practice, this would require mocking the Route53 API

	// Create mock Route53 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "hostedzone"):
			// Mock list hosted zones response
			response := `<?xml version="1.0" encoding="UTF-8"?>
<ListHostedZonesResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
    <HostedZones>
        <HostedZone>
            <Id>/hostedzone/Z123456789</Id>
            <Name>example.com.</Name>
            <CallerReference>test-ref-123</CallerReference>
            <Config>
                <PrivateZone>false</PrivateZone>
            </Config>
            <ResourceRecordSetCount>10</ResourceRecordSetCount>
        </HostedZone>
    </HostedZones>
    <IsTruncated>false</IsTruncated>
    <MaxItems>100</MaxItems>
</ListHostedZonesResponse>`
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))

		case r.Method == "POST" && strings.Contains(r.URL.Path, "rrset"):
			// Mock change resource record sets (create DNS challenge record)
			response := `<?xml version="1.0" encoding="UTF-8"?>
<ChangeResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
    <ChangeInfo>
        <Id>/change/C123456789</Id>
        <Status>INSYNC</Status>
        <SubmittedAt>2024-01-01T12:00:00.000Z</SubmittedAt>
    </ChangeInfo>
</ChangeResourceRecordSetsResponse>`
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	// Test DNS challenge workflow simulation
	// 1. Find hosted zone for domain
	// 2. Create TXT record for ACME challenge
	// 3. Wait for propagation
	// 4. Clean up TXT record

	t.Log("Mock Route53 server started at:", mockServer.URL)

	// In a real test, you would:
	// 1. Create Route53 client with mock endpoint
	// 2. Use lego Route53 provider with mock
	// 3. Simulate DNS challenge creation and cleanup
	// 4. Verify TXT records are created and removed correctly

	// For now, just verify the mock server responds correctly
	client := &http.Client{Timeout: 5 * time.Second}

	// Test hosted zone listing
	resp, err := client.Get(mockServer.URL + "/2013-04-01/hostedzone")
	if err != nil {
		t.Fatalf("Failed to get hosted zones: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestAWSRegionValidation tests different AWS regions
func TestAWSRegionValidation(t *testing.T) {
	regions := []string{
		"us-east-1",
		"us-west-2",
		"eu-west-1",
		"ap-southeast-1",
	}

	for _, region := range regions {
		t.Run(region, func(t *testing.T) {
			// Create mock server for this region
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := `<?xml version="1.0" encoding="UTF-8"?>
<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
    <GetCallerIdentityResult>
        <Arn>arn:aws:iam::123456789012:user/test-user</Arn>
        <UserId>AIDACKCEVSQ6C2EXAMPLE</UserId>
        <Account>123456789012</Account>
    </GetCallerIdentityResult>
</GetCallerIdentityResponse>`
				w.Header().Set("Content-Type", "text/xml")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(response))
			}))
			defer mockServer.Close()

			// Test AWS config with this region
			cfg, err := config.LoadDefaultConfig(context.TODO(),
				config.WithRegion(region),
				config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
					"AKIATEST123",
					"test-secret",
					"",
				)),
				config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
					func(service, regionParam string, options ...interface{}) (aws.Endpoint, error) {
						if regionParam != region {
							t.Errorf("Expected region %s, got %s", region, regionParam)
						}
						return aws.Endpoint{
							URL:           mockServer.URL,
							SigningRegion: region,
						}, nil
					})),
			)
			if err != nil {
				t.Fatalf("Failed to create AWS config for region %s: %v", region, err)
			}

			// Verify region is set correctly
			if cfg.Region != region {
				t.Errorf("Expected region %s, got %s", region, cfg.Region)
			}
		})
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"google.golang.org/api/idtoken"
)

var (
	roleArn    = flag.String("roleArn", "", "ARN of AWS role to assume")
	outputPath = flag.String("outputPath", "", "Path to output AWS credentials")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	idToken, err := generateGoogleIDToken(ctx, "gcp")
	if err != nil {
		log.Fatalf("failed to generate Google ID token: %v", err)
	}

	sessionName := "dataflow"
	credentials, err := assumeAWSRole(ctx, *roleArn, sessionName, idToken)
	if err != nil {
		log.Fatalf("failed to assume AWS role: %v", err)
	}

	if err := writeAWSCredentials(credentials, *outputPath); err != nil {
		log.Fatalf("failed to write AWS credentials to %s: %v", *outputPath, err)
	}

	log.Printf("successfully wrote AWS credentials to: %s", *outputPath)
}

func generateGoogleIDToken(ctx context.Context, audience string) (string, error) {
	tokenSource, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		return "", fmt.Errorf("error creating token source: %v", err)
	}

	token, err := tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("error retrieving token: %v", err)
	}
	return token.AccessToken, err
}

func assumeAWSRole(
	ctx context.Context,
	arn string,
	sessionName string,
	idToken string,
) (*types.Credentials, error) {
	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return nil, fmt.Errorf("error loading config: %v ", err)
	}

	client := sts.NewFromConfig(cfg)
	request := &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(arn),
		RoleSessionName:  aws.String(sessionName),
		WebIdentityToken: aws.String(idToken),
	}
	response, err := client.AssumeRoleWithWebIdentity(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("error assuming role: %v ", err)
	}

	return response.Credentials, nil
}

func writeAWSCredentials(credentials *types.Credentials, path string) error {
	template := "[default]\n" +
		"aws_access_key_id = %s\n" +
		"aws_secret_access_key = %s\n" +
		"aws_session_token = %s\n"

	content := fmt.Sprintf(
		template,
		aws.ToString(credentials.AccessKeyId),
		aws.ToString(credentials.SecretAccessKey),
		aws.ToString(credentials.SessionToken),
	)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %v", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("error writing file: %v ", err)
	}

	return nil
}

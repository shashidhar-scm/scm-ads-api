// internal/config/s3.go
package config

import (
    "context"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config holds S3 configuration
type S3Config struct {
    Client *s3.Client
    Bucket string
    PublicBaseURL string
}

// NewS3Config creates a new S3 configuration
func NewS3Config() (*S3Config, error) {
    region := getEnv("AWS_REGION", "us-east-1")
    bucket := getEnv("S3_BUCKET_NAME", "scm-ads")
    publicBaseURL := getEnv("CREATIVE_PUBLIC_BASE_URL", "https://scm-ads-posters.citypost.us/")

    accessKeyID := getEnv("AWS_ACCESS_KEY_ID", "")
    secretAccessKey := getEnv("AWS_SECRET_ACCESS_KEY", "")

    opts := []func(*config.LoadOptions) error{
        config.WithRegion(region),
    }

    // If explicit env credentials are present, use them; otherwise fall back to
    // the AWS SDK default credential chain (env/shared config/IMDS/etc.).
    if accessKeyID != "" && secretAccessKey != "" {
        opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            accessKeyID,
            secretAccessKey,
            "",
        )))
    }

    cfg, err := config.LoadDefaultConfig(context.TODO(), opts...)
    if err != nil {
        return nil, err
    }

    return &S3Config{
        Client: s3.NewFromConfig(cfg),
        Bucket: bucket,
        PublicBaseURL: publicBaseURL,
    }, nil
}
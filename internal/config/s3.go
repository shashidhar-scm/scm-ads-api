// internal/config/s3.go
package config

import (
    "context"
    "os"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config holds S3 configuration
type S3Config struct {
    Client *s3.Client
    Bucket string
}

// NewS3Config creates a new S3 configuration
func NewS3Config() (*S3Config, error) {
    cfg, err := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion(os.Getenv("AWS_REGION")),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            os.Getenv("AWS_ACCESS_KEY_ID"),
            os.Getenv("AWS_SECRET_ACCESS_KEY"),
            "",
        )),
    )
    if err != nil {
        return nil, err
    }

    return &S3Config{
        Client: s3.NewFromConfig(cfg),
        Bucket: os.Getenv("S3_BUCKET_NAME"),
    }, nil
}
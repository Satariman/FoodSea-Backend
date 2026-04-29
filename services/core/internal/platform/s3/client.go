package s3

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// Config holds S3/MinIO connection parameters.
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	UseSSL          bool
	PublicBaseURL   string
}

// Client wraps aws-sdk-go-v2 S3 with bucket management helpers.
type Client struct {
	s3      *awss3.Client
	bucket  string
	baseURL string
}

// NewClient creates a Client, ensures the bucket exists and applies a public-read policy.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	endpoint := cfg.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		endpoint = scheme + "://" + endpoint
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("s3: load config: %w", err)
	}

	s3Client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	})

	c := &Client{
		s3:      s3Client,
		bucket:  cfg.BucketName,
		baseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
	}

	if err := c.ensureBucket(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) ensureBucket(ctx context.Context) error {
	_, err := c.s3.HeadBucket(ctx, &awss3.HeadBucketInput{
		Bucket: aws.String(c.bucket),
	})
	if err == nil {
		return c.applyPublicReadPolicy(ctx)
	}

	_, err = c.s3.CreateBucket(ctx, &awss3.CreateBucketInput{
		Bucket: aws.String(c.bucket),
	})
	if err != nil {
		return fmt.Errorf("s3: create bucket %q: %w", c.bucket, err)
	}

	return c.applyPublicReadPolicy(ctx)
}

func (c *Client) applyPublicReadPolicy(ctx context.Context) error {
	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, c.bucket)

	_, err := c.s3.PutBucketPolicy(ctx, &awss3.PutBucketPolicyInput{
		Bucket: aws.String(c.bucket),
		Policy: aws.String(policy),
	})
	if err != nil {
		return fmt.Errorf("s3: set bucket policy: %w", err)
	}
	return nil
}

// Upload stores the object and returns its public URL.
func (c *Client) Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error) {
	_, err := c.s3.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3: upload %q: %w", key, err)
	}
	return c.PublicURL(key), nil
}

// Delete removes the object from S3.
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3: delete %q: %w", key, err)
	}
	return nil
}

// PublicURL forms a public URL for the given key without a network request.
func (c *Client) PublicURL(key string) string {
	return c.baseURL + "/" + key
}

// KeyFromURL strips the public base URL prefix to recover the S3 object key.
func (c *Client) KeyFromURL(rawURL string) string {
	prefix := c.baseURL + "/"
	if strings.HasPrefix(rawURL, prefix) {
		return rawURL[len(prefix):]
	}
	return rawURL
}

package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Radiushina/GophKeeper/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3 stores objects in an S3-compatible bucket (MinIO in docker-compose).
type S3 struct {
	client *s3.Client
	bucket string
}

// NewS3 builds an S3 client and ensures the bucket exists.
func NewS3(ctx context.Context, cfg *config.Config) (*S3, error) {
	region := cfg.S3.Region
	if region == "" {
		region = "us-east-1"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.S3.AccessKey, cfg.S3.SecretKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("s3 config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3.Endpoint)
		}
		o.UsePathStyle = cfg.S3.PathStyle
	})
	store := &S3{client: client, bucket: cfg.S3.Bucket}
	if err := store.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *S3) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		var exists *types.BucketAlreadyOwnedByYou
		var taken *types.BucketAlreadyExists
		if errors.As(err, &exists) || errors.As(err, &taken) {
			return nil
		}
		return fmt.Errorf("create bucket %s: %w", s.bucket, err)
	}
	return nil
}

// Put writes ciphertext to the bucket.
func (s *S3) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("s3 put %s: %w", key, err)
	}
	return nil
}

// Get reads ciphertext from the bucket.
func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return nil, fmt.Errorf("s3 get %s: %w", key, err)
	}
	defer out.Body.Close()
	b, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("s3 read %s: %w", key, err)
	}
	return b, nil
}

// Delete removes ciphertext. Missing keys are ignored.
func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 delete %s: %w", key, err)
	}
	return nil
}

// List returns object keys with the given prefix.
func (s *S3) List(ctx context.Context, prefix string) ([]string, error) {
	var out []string
	var token *string
	for {
		res, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("s3 list %s: %w", prefix, err)
		}
		for _, obj := range res.Contents {
			if obj.Key != nil {
				out = append(out, *obj.Key)
			}
		}
		if res.IsTruncated == nil || !*res.IsTruncated {
			return out, nil
		}
		token = res.NextContinuationToken
	}
}

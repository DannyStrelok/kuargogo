package config

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const s3ObjectName = "kuargogo-config.yaml.enc"
const legacyS3ObjectName = "rk-cli-config.yaml.enc"

// S3Provider implements SyncProvider using AWS S3 or any S3-compatible storage (like Cloudflare R2).
type S3Provider struct{}

func NewS3Provider() *S3Provider {
	return &S3Provider{}
}

func (r *S3Provider) getFullKey(legacy bool) string {
	sync := RootConfigGetSync()
	prefix := sync.S3.S3Prefix
	objName := s3ObjectName
	if legacy {
		objName = legacyS3ObjectName
	}
	if prefix == "" {
		return objName
	}
	// Ensure prefix ends with /
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix + objName
}

func (r *S3Provider) getClient() (*s3.Client, string, error) {
	sync := RootConfigGetSync()
	if sync.S3.S3AccessKey == "" || sync.S3.S3SecretKey == "" || sync.S3.S3Url == "" || sync.S3.S3Bucket == "" {
		return nil, "", fmt.Errorf("S3 credentials not fully configured in sync settings")
	}

	endpoint := strings.TrimSuffix(sync.S3.S3Url, "/")

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			string(sync.S3.S3AccessKey),
			string(sync.S3.S3SecretKey),
			"",
		)),
		config.WithRegion(sync.S3.S3Region),
	)
	if err != nil {
		return nil, "", err
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	}), sync.S3.S3Bucket, nil
}

func (r *S3Provider) Push(data []byte) error {
	client, bucket, err := r.getClient()
	if err != nil {
		return err
	}

	sync := RootConfigGetSync()
	metadata := map[string]string{
		"salt": sync.Salt,
	}

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(r.getFullKey(false)),
		Body:     bytes.NewReader(data),
		Metadata: metadata,
	})
	return err
}

func (r *S3Provider) Pull() ([]byte, error) {
	client, bucket, err := r.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(r.getFullKey(false)),
	})
	if err != nil {
		// Fallback: try legacy key if the new key is not found
		legacyResp, legacyErr := client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(r.getFullKey(true)),
		})
		if legacyErr == nil {
			resp = legacyResp
		} else {
			return nil, err
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Update local salt from metadata if present
	if salt, ok := resp.Metadata["salt"]; ok && salt != "" {
		configMutex.Lock()
		appConfig.Sync.Salt = salt
		configMutex.Unlock()
	}

	return io.ReadAll(resp.Body)
}

func (r *S3Provider) Logout() error {
	// Nothing to clear really as credentials are in the config file
	return nil
}

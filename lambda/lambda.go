package main

import (
	"context"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tarosky/gutenberg-cflog/cflog"
	"go.uber.org/zap"
)

// Task is used to convert specific image directly.
// Empty Path field means this execution is a regular cron job.
type Task struct {
	Records []TaskRecord `json:"Records"`
}

type TaskRecord struct {
	EventVersion string `json:"eventVersion"`
	EventSource  string `json:"eventSource"`
	EventName    string `json:"eventName"`
	S3           TaskS3 `json:"s3"`
}

type TaskS3 struct {
	S3SchemaVersion string       `json:"s3SchemaVersion"`
	Bucket          TaskS3Bucket `json:"bucket"`
	Object          TaskS3Object `json:"object"`
}

type TaskS3Bucket struct {
	Name string `json:"name"`
}

type TaskS3Object struct {
	Key string `json:"key"`
}

var env *environment

type environment struct {
	awsConfig *aws.Config
	s3Client  *s3.Client
	log       *zap.Logger
	config    *cflog.Config
}

func createAWSConfig(ctx context.Context, region string) *aws.Config {
	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region))
	if err != nil {
		panic(err)
	}

	return &awsCfg
}

func newEnvironment(
	ctx context.Context,
	log *zap.Logger,
	region, keys, commonPrefix string,
	samplingPercent float64,
) *environment {
	awsConfig := createAWSConfig(ctx, region)

	return &environment{
		awsConfig: awsConfig,
		s3Client:  s3.NewFromConfig(*awsConfig),
		log:       log,
		config: &cflog.Config{
			Log:             log,
			OutputFields:    cflog.ParseOutputFields(keys),
			CommonPrefix:    commonPrefix,
			SamplingPercent: samplingPercent,
		},
	}
}

func scanS3Object(ctx context.Context, bucket, key string) error {
	obj, err := env.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}

	body := obj.Body
	defer func() {
		if err := body.Close(); err != nil {
			env.log.Error("failed to close s3 object",
				cflog.ZapErrorLevel,
				zap.String("bucket", bucket),
				zap.String("key", key),
				zap.Error(err))
		}
	}()

	return cflog.Scan(body, env.config)
}

// HandleRequest handles requests from Lambda environment.
func HandleRequest(ctx context.Context, task Task) error {
	var lastErr error
	for _, r := range task.Records {
		if err := scanS3Object(ctx, r.S3.Bucket.Name, r.S3.Object.Key); err != nil {
			env.log.Error("failed to scan s3 object",
				cflog.ZapErrorLevel,
				zap.Any("task", task),
				zap.Error(err))
			lastErr = err
		}
	}
	return lastErr
}

func readEnvFloat64(key string, defaultValue float64) float64 {
	s := os.Getenv(key)
	if s == "" {
		return defaultValue
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}

	return f
}

func main() {
	env = newEnvironment(
		context.Background(),
		cflog.CreateLogger([]string{"stderr"}),
		os.Getenv("REGION"),
		os.Getenv("KEYS"),
		os.Getenv("COMMON_PREFIX"),
		readEnvFloat64("SAMPLING_PERCENT", 100.0))

	lambda.Start(HandleRequest)
}

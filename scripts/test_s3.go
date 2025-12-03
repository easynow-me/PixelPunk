//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	// 从环境变量读取配置
	bucket := os.Getenv("S3_BUCKET")
	region := os.Getenv("S3_REGION")
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	usePathStyle := os.Getenv("S3_USE_PATH_STYLE") == "true"

	if bucket == "" || accessKey == "" || secretKey == "" {
		fmt.Println("❌ 缺少必要的环境变量")
		fmt.Println("请设置以下环境变量:")
		fmt.Println("  S3_BUCKET     - 存储桶名称 (必填)")
		fmt.Println("  S3_ACCESS_KEY - Access Key (必填)")
		fmt.Println("  S3_SECRET_KEY - Secret Key (必填)")
		fmt.Println("  S3_REGION     - 区域 (可选, 默认 us-east-1)")
		fmt.Println("  S3_ENDPOINT   - 自定义端点 (可选, MinIO等兼容存储需要)")
		fmt.Println("  S3_USE_PATH_STYLE - 是否使用路径样式 (可选, true/false)")
		fmt.Println("")
		fmt.Println("示例:")
		fmt.Println("  # AWS S3")
		fmt.Println("  export S3_BUCKET=my-bucket S3_ACCESS_KEY=xxx S3_SECRET_KEY=xxx S3_REGION=ap-northeast-1")
		fmt.Println("  go run scripts/test_s3.go")
		fmt.Println("")
		fmt.Println("  # MinIO")
		fmt.Println("  export S3_BUCKET=my-bucket S3_ACCESS_KEY=xxx S3_SECRET_KEY=xxx S3_ENDPOINT=http://localhost:9000 S3_USE_PATH_STYLE=true")
		fmt.Println("  go run scripts/test_s3.go")
		os.Exit(1)
	}

	if region == "" {
		region = "us-east-1"
	}

	fmt.Println("🔍 S3 连接测试")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Bucket:     %s\n", bucket)
	fmt.Printf("  Region:     %s\n", region)
	if endpoint != "" {
		fmt.Printf("  Endpoint:   %s\n", endpoint)
	}
	fmt.Printf("  PathStyle:  %v\n", usePathStyle)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// 构建配置
	awsCfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}

	// 创建 S3 客户端，使用新的 BaseEndpoint 方式
	var client *s3.Client
	if endpoint != "" {
		client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = usePathStyle
		})
	} else {
		client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.UsePathStyle = usePathStyle
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 测试 1: ListBuckets
	fmt.Print("1️⃣  测试 ListBuckets... ")
	start := time.Now()
	listResult, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		fmt.Printf("❌ 失败\n   错误: %v\n", err)
	} else {
		fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))
		fmt.Printf("   找到 %d 个存储桶\n", len(listResult.Buckets))
	}

	// 测试 2: HeadBucket
	fmt.Print("2️⃣  测试 HeadBucket... ")
	start = time.Now()
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		fmt.Printf("❌ 失败\n   错误: %v\n", err)
	} else {
		fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))
	}

	// 测试 3: ListObjectsV2
	fmt.Print("3️⃣  测试 ListObjectsV2... ")
	start = time.Now()
	listObjResult, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int32(5),
	})
	if err != nil {
		fmt.Printf("❌ 失败\n   错误: %v\n", err)
	} else {
		fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))
		fmt.Printf("   存储桶中有 %d 个对象 (仅显示前5个)\n", len(listObjResult.Contents))
		for i, obj := range listObjResult.Contents {
			if i >= 5 {
				break
			}
			fmt.Printf("   - %s (%d bytes)\n", *obj.Key, obj.Size)
		}
	}

	// 测试 4: PutObject + DeleteObject (写入测试)
	testKey := "_connection_test_" + time.Now().Format("20060102150405") + ".txt"
	testContent := []byte("S3 connection test - " + time.Now().String())

	fmt.Print("4️⃣  测试 PutObject... ")
	start = time.Now()
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(testKey),
		Body:        bytes.NewReader(testContent),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		fmt.Printf("❌ 失败\n   错误: %v\n", err)
	} else {
		fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))

		// 清理测试文件
		fmt.Print("5️⃣  测试 DeleteObject... ")
		start = time.Now()
		_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(testKey),
		})
		if err != nil {
			fmt.Printf("❌ 失败\n   错误: %v\n", err)
		} else {
			fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))
		}
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🎉 S3 连接测试完成")
}

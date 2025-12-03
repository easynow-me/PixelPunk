//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	bucket := os.Getenv("S3_BUCKET")
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")

	if bucket == "" || accessKey == "" || secretKey == "" || endpoint == "" {
		fmt.Println("请设置环境变量: S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY, S3_ENDPOINT")
		os.Exit(1)
	}

	// 去掉 scheme
	host := strings.TrimPrefix(endpoint, "https://")
	host = strings.TrimPrefix(host, "http://")
	useSSL := strings.HasPrefix(endpoint, "https://")

	fmt.Println("🔍 S3 连接测试 (MinIO SDK)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Bucket:   %s\n", bucket)
	fmt.Printf("  Endpoint: %s\n", host)
	fmt.Printf("  UseSSL:   %v\n", useSSL)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		fmt.Printf("❌ 创建客户端失败: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 测试 1: BucketExists
	fmt.Print("1️⃣  测试 BucketExists... ")
	start := time.Now()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		fmt.Printf("❌ 失败\n   错误: %v\n", err)
	} else {
		fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))
		fmt.Printf("   存储桶存在: %v\n", exists)
	}

	// 测试 2: ListObjects
	fmt.Print("2️⃣  测试 ListObjects... ")
	start = time.Now()
	count := 0
	for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{MaxKeys: 5}) {
		if obj.Err != nil {
			fmt.Printf("❌ 失败\n   错误: %v\n", obj.Err)
			break
		}
		count++
		if count == 1 {
			fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))
		}
		if count <= 5 {
			fmt.Printf("   - %s (%d bytes)\n", obj.Key, obj.Size)
		}
	}
	if count == 0 {
		fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))
		fmt.Println("   存储桶为空")
	}

	// 测试 3: PutObject
	testKey := "_connection_test_" + time.Now().Format("20060102150405") + ".txt"
	testContent := []byte("S3 connection test - " + time.Now().String())

	fmt.Print("3️⃣  测试 PutObject... ")
	start = time.Now()
	_, err = client.PutObject(ctx, bucket, testKey, bytes.NewReader(testContent), int64(len(testContent)), minio.PutObjectOptions{
		ContentType: "text/plain",
	})
	if err != nil {
		fmt.Printf("❌ 失败\n   错误: %v\n", err)
	} else {
		fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))

		// 测试 4: RemoveObject
		fmt.Print("4️⃣  测试 RemoveObject... ")
		start = time.Now()
		err = client.RemoveObject(ctx, bucket, testKey, minio.RemoveObjectOptions{})
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

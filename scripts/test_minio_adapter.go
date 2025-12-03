//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"pixelpunk/pkg/storage/adapter"
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

	fmt.Println("🔍 MinIO 适配器连接测试")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Bucket:   %s\n", bucket)
	fmt.Printf("  Endpoint: %s\n", endpoint)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// 创建 MinIO 适配器
	minioAdapter := adapter.NewMinIOAdapter()
	err := minioAdapter.Initialize(map[string]interface{}{
		"bucket":     bucket,
		"endpoint":   endpoint,
		"access_key": accessKey,
		"secret_key": secretKey,
		"region":     "us-east-1",
		"use_https":  true,
	})
	if err != nil {
		fmt.Printf("❌ 初始化适配器失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ 适配器初始化成功")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 测试 1: HealthCheck
	fmt.Print("1️⃣  测试 HealthCheck... ")
	start := time.Now()
	err = minioAdapter.HealthCheck(ctx)
	if err != nil {
		fmt.Printf("❌ 失败\n   错误: %v\n", err)
	} else {
		fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))
	}

	// 测试 2: Exists (不存在的文件)
	fmt.Print("2️⃣  测试 Exists (不存在的文件)... ")
	start = time.Now()
	exists, err := minioAdapter.Exists(ctx, "_non_existent_file_12345.txt")
	if err != nil {
		fmt.Printf("❌ 失败\n   错误: %v\n", err)
	} else {
		fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))
		fmt.Printf("   文件存在: %v (预期: false)\n", exists)
	}

	// 测试 3: GetURL
	fmt.Print("3️⃣  测试 GetURL... ")
	start = time.Now()
	url, err := minioAdapter.GetURL("test/example.jpg", nil)
	if err != nil {
		fmt.Printf("❌ 失败\n   错误: %v\n", err)
	} else {
		fmt.Printf("✅ 成功 (%v)\n", time.Since(start).Round(time.Millisecond))
		fmt.Printf("   URL: %s\n", url)
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🎉 MinIO 适配器测试完成")
	fmt.Println()
	fmt.Println("适配器类型: " + minioAdapter.GetType())
	caps := minioAdapter.GetCapabilities()
	fmt.Printf("支持签名URL: %v\n", caps.SupportsSignedURL)
	fmt.Printf("支持格式: %v\n", caps.SupportedFormats)
}

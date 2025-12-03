package adapter

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"pixelpunk/pkg/imagex/formats"
	"pixelpunk/pkg/imagex/iox"
	"pixelpunk/pkg/storage/config"
	"pixelpunk/pkg/storage/tenant"
	"pixelpunk/pkg/storage/utils"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOAdapter MinIO/S3 兼容存储适配器（使用 minio-go SDK）
// 相比 S3Adapter，对某些非标准 S3 兼容存储（如 RustFS）有更好的兼容性
type MinIOAdapter struct {
	client        *minio.Client
	bucket        string
	region        string
	endpoint      string // 完整端点，如 https://fs.example.com
	accessKey     string
	secretKey     string
	customDomain  string
	useHTTPS      bool
	accessControl string // public-read/private
	initialized   bool
}

func NewMinIOAdapter() StorageAdapter   { return &MinIOAdapter{} }
func (a *MinIOAdapter) GetType() string { return "minio" }

// Initialize 初始化适配器
func (a *MinIOAdapter) Initialize(configData map[string]interface{}) error {
	cfg := config.NewMapConfig(configData)
	a.bucket = cfg.GetStringWithDefault("bucket", "")
	a.region = cfg.GetStringWithDefault("region", "us-east-1")
	a.endpoint = cfg.GetStringWithDefault("endpoint", "")
	a.accessKey = cfg.GetStringWithDefault("access_key", "")
	a.secretKey = cfg.GetStringWithDefault("secret_key", "")
	a.useHTTPS = cfg.GetBoolWithDefault("use_https", true)
	a.accessControl = cfg.GetString("access_control")
	a.customDomain, a.useHTTPS = normalizeDomainAndScheme(cfg.GetString("custom_domain"), a.useHTTPS)

	if a.bucket == "" {
		return NewStorageError(ErrorTypeInternal, "bucket is required", nil)
	}
	if a.endpoint == "" {
		return NewStorageError(ErrorTypeInternal, "endpoint is required for MinIO adapter", nil)
	}
	if a.accessKey == "" {
		return NewStorageError(ErrorTypeInternal, "access_key is required", nil)
	}
	if a.secretKey == "" {
		return NewStorageError(ErrorTypeInternal, "secret_key is required", nil)
	}

	// 解析端点，提取 host 和判断是否使用 SSL
	host, useSSL := a.parseEndpoint()

	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(a.accessKey, a.secretKey, ""),
		Secure: useSSL,
		Region: a.region,
	})
	if err != nil {
		return NewStorageError(ErrorTypeInternal, "failed to create MinIO client", err)
	}

	a.client = client
	a.initialized = true
	return nil
}

// parseEndpoint 解析端点，返回 host 和是否使用 SSL
func (a *MinIOAdapter) parseEndpoint() (host string, useSSL bool) {
	ep := strings.TrimSpace(a.endpoint)
	if ep == "" {
		return "", a.useHTTPS
	}

	// 检查是否包含 scheme
	if strings.HasPrefix(ep, "https://") {
		return strings.TrimPrefix(ep, "https://"), true
	}
	if strings.HasPrefix(ep, "http://") {
		return strings.TrimPrefix(ep, "http://"), false
	}

	// 没有 scheme，使用配置的 useHTTPS
	return ep, a.useHTTPS
}

// Upload 上传文件
func (a *MinIOAdapter) Upload(ctx context.Context, req *UploadRequest) (*UploadResult, error) {
	if !a.initialized {
		return nil, NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	var data []byte
	if len(req.ProcessedData) > 0 {
		data = req.ProcessedData
	} else {
		src, err := req.File.Open()
		if err != nil {
			return nil, NewStorageError(ErrorTypeInternal, "failed to open file", err)
		}
		defer src.Close()
		buf, err := iox.ReadAllWithLimit(src, iox.DefaultMaxReadBytes)
		if err != nil {
			return nil, NewStorageError(ErrorTypeInternal, "failed to read file data", err)
		}
		data = buf
	}

	processed, width, height, format := processUploadData(data, req)

	originalFileName := req.FileName
	objectPath, err := tenant.BuildObjectKey(req.UserID, req.FolderPath, originalFileName)
	if err != nil {
		return nil, NewStorageError(ErrorTypeInternal, "failed to build object key", err)
	}
	logicalPath := utils.BuildLogicalPath(req.FolderPath, originalFileName)

	contentType := a.getContentType(format)
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}

	_, err = a.client.PutObject(ctx, a.bucket, objectPath, bytes.NewReader(processed), int64(len(processed)), opts)
	if err != nil {
		return nil, NewStorageError(ErrorTypeInternal, "failed to upload to MinIO", err)
	}

	var thumbnailPath, thumbnailURL string
	if req.Options != nil && req.Options.GenerateThumb {
		if tpath, _, _, err := a.generateThumbnail(bytes.NewReader(data), req, objectPath); err == nil {
			thumbnailPath = tpath
			tf := "jpg"
			if ext := filepath.Ext(tpath); ext != "" {
				tf = strings.TrimPrefix(strings.ToLower(ext), ".")
			}
			thumbnailURL = utils.BuildLogicalPath(req.FolderPath, utils.MakeThumbName(originalFileName, tf))
		}
	}

	hash := fmt.Sprintf("%x", md5.Sum(processed))
	direct, _ := a.GetURL(objectPath, nil)
	thumbDirectURL := ""
	if thumbnailPath != "" {
		thumbDirectURL, _ = a.GetURL(thumbnailPath, nil)
	}

	return &UploadResult{
		OriginalPath:   objectPath,
		ThumbnailPath:  thumbnailPath,
		URL:            logicalPath,
		ThumbnailURL:   thumbnailURL,
		FullURL:        direct,
		FullThumbURL:   thumbDirectURL,
		RemoteURL:      objectPath,
		RemoteThumbURL: thumbnailPath,
		Size:           int64(len(processed)),
		Width:          width,
		Height:         height,
		Hash:           hash,
		ContentType:    contentType,
		Format:         format,
	}, nil
}

// Delete 删除对象
func (a *MinIOAdapter) Delete(ctx context.Context, path string) error {
	if !a.initialized {
		return NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}
	return a.client.RemoveObject(ctx, a.bucket, path, minio.RemoveObjectOptions{})
}

// GetURL 生成访问 URL
func (a *MinIOAdapter) GetURL(path string, options *URLOptions) (string, error) {
	if !a.initialized {
		return "", NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	// 私有访问控制使用预签名 URL
	if a.accessControl == "private" {
		return a.generatePresignedURL(path, options)
	}

	// 公开访问，构建直接 URL
	return a.buildPublicURL(path), nil
}

// buildPublicURL 构建公开访问 URL
func (a *MinIOAdapter) buildPublicURL(path string) string {
	scheme := "https"
	if !a.useHTTPS {
		scheme = "http"
	}

	// 如果有自定义域名，优先使用
	if a.customDomain != "" {
		return scheme + "://" + strings.TrimSuffix(a.customDomain, "/") + "/" + encodePathSegments(path)
	}

	// 使用端点 + bucket + path（path-style）
	host, _ := a.parseEndpoint()
	return scheme + "://" + host + "/" + a.bucket + "/" + encodePathSegments(path)
}

// generatePresignedURL 生成预签名 URL
func (a *MinIOAdapter) generatePresignedURL(path string, options *URLOptions) (string, error) {
	exp := time.Hour
	if options != nil && options.Expires > 0 {
		exp = time.Duration(options.Expires) * time.Second
	}

	reqParams := make(url.Values)
	presignedURL, err := a.client.PresignedGetObject(context.Background(), a.bucket, path, exp, reqParams)
	if err != nil {
		return "", NewStorageError(ErrorTypeInternal, "failed to generate presigned URL", err)
	}
	return presignedURL.String(), nil
}

// SetObjectACL 设置对象 ACL（MinIO 通过桶策略控制，此方法为空实现）
func (a *MinIOAdapter) SetObjectACL(ctx context.Context, path string, acl string) error {
	// MinIO 通常使用桶策略而非对象 ACL
	// 如需支持，可通过 SetBucketPolicy 实现
	return nil
}

// HealthCheck 健康检查
func (a *MinIOAdapter) HealthCheck(ctx context.Context) error {
	if !a.initialized {
		return NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	exists, err := a.client.BucketExists(ctx, a.bucket)
	if err != nil {
		return NewStorageError(ErrorTypeNetwork, "failed to check bucket", err)
	}
	if !exists {
		return NewStorageError(ErrorTypeNotFound, "bucket does not exist", nil)
	}
	return nil
}

// ReadFile 读取对象
func (a *MinIOAdapter) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if !a.initialized {
		return nil, NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	obj, err := a.client.GetObject(ctx, a.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// Exists 检查对象是否存在
func (a *MinIOAdapter) Exists(ctx context.Context, path string) (bool, error) {
	if !a.initialized {
		return false, NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	_, err := a.client.StatObject(ctx, a.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, nil
	}
	return true, nil
}

// generateThumbnail 生成缩略图
func (a *MinIOAdapter) generateThumbnail(src io.Reader, req *UploadRequest, originalPath string) (string, string, string, error) {
	srcFile, err := req.File.Open()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to reopen source file: %w", err)
	}
	defer srcFile.Close()

	data, err := iox.ReadAllWithLimit(srcFile, iox.DefaultMaxReadBytes)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read source data: %w", err)
	}

	thumbBytes, thumbFormat := buildThumbnailBytes(data, req)
	thumbFileName := utils.MakeThumbName(filepath.Base(originalPath), thumbFormat)
	thumbObjectPath, _ := tenant.BuildThumbObjectKey(req.UserID, req.FolderPath, thumbFileName)

	contentType := formats.GetContentType(thumbFormat)
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}

	_, err = a.client.PutObject(context.Background(), a.bucket, thumbObjectPath, bytes.NewReader(thumbBytes), int64(len(thumbBytes)), opts)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to upload thumbnail: %w", err)
	}

	thumbURL, _ := a.GetURL(thumbObjectPath, nil)
	return thumbObjectPath, thumbURL, thumbURL, nil
}

// GetCapabilities 返回适配器能力
func (a *MinIOAdapter) GetCapabilities() Capabilities {
	return Capabilities{
		SupportsSignedURL: true,
		SupportsCDN:       false,
		SupportsResize:    false,
		SupportsWebP:      true,
		MaxFileSize:       5 * 1024 * 1024 * 1024, // 5GB
		SupportedFormats:  []string{"jpg", "jpeg", "png", "gif", "webp"},
	}
}

func (a *MinIOAdapter) getContentType(format string) string {
	return formats.GetContentType(format)
}

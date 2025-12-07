package adapter

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"pixelpunk/pkg/imagex/compress"
	"pixelpunk/pkg/imagex/decode"
	"pixelpunk/pkg/imagex/formats"
	"pixelpunk/pkg/imagex/iox"
	"pixelpunk/pkg/logger"
	"pixelpunk/pkg/storage/config"
	"pixelpunk/pkg/storage/middleware"
	"pixelpunk/pkg/storage/tenant"
	"pixelpunk/pkg/storage/utils"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// COSAdapter 腾讯云COS存储适配器
type COSAdapter struct {
	client        *cos.Client
	bucket        string
	region        string
	secretID      string
	secretKey     string
	customDomain  string
	useHTTPS      bool
	accessControl string // 访问控制类型：public-read/private
	initialized   bool
}

func NewCOSAdapter() StorageAdapter {
	return &COSAdapter{}
}

func (a *COSAdapter) GetType() string {
	return "cos"
}

// Initialize 初始化适配器
func (a *COSAdapter) Initialize(configData map[string]interface{}) error {
	cfg := config.NewMapConfig(configData)

	a.bucket = cfg.GetStringWithDefault("bucket", "")
	a.region = cfg.GetStringWithDefault("region", "")
	a.secretID = cfg.GetStringWithDefault("secret_id", "")
	a.secretKey = cfg.GetStringWithDefault("secret_key", "")
	a.useHTTPS = cfg.GetBoolWithDefault("use_https", true)
	a.customDomain, a.useHTTPS = normalizeDomainAndScheme(cfg.GetString("custom_domain"), a.useHTTPS)
	a.accessControl = cfg.GetString("access_control")

	if a.bucket == "" {
		return NewStorageError(ErrorTypeInternal, "bucket is required", nil)
	}
	if a.region == "" {
		return NewStorageError(ErrorTypeInternal, "region is required", nil)
	}
	if a.secretID == "" {
		return NewStorageError(ErrorTypeInternal, "secret_id is required", nil)
	}
	if a.secretKey == "" {
		return NewStorageError(ErrorTypeInternal, "secret_key is required", nil)
	}

	scheme := "https"
	if !a.useHTTPS {
		scheme = "http"
	}

	bucketURL := fmt.Sprintf("%s://%s.cos.%s.myqcloud.com", scheme, a.bucket, a.region)
	serviceURL := fmt.Sprintf("%s://cos.%s.myqcloud.com", scheme, a.region)

	u, err := url.Parse(bucketURL)
	if err != nil {
		return NewStorageError(ErrorTypeInternal, "invalid bucket URL", err)
	}

	su, err := url.Parse(serviceURL)
	if err != nil {
		return NewStorageError(ErrorTypeInternal, "invalid service URL", err)
	}

	baseURL := &cos.BaseURL{
		BucketURL:  u,
		ServiceURL: su,
	}

	client := cos.NewClient(baseURL, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  a.secretID,
			SecretKey: a.secretKey,
		},
	})

	a.client = client
	a.initialized = true

	return nil
}

// Upload 上传文件
func (a *COSAdapter) Upload(ctx context.Context, req *UploadRequest) (*UploadResult, error) {
	if !a.initialized {
		return nil, NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	if err := a.validateFile(req); err != nil {
		return nil, NewStorageError(ErrorTypeInvalidFormat, "file validation failed", err)
	}

	src, err := req.File.Open()
	if err != nil {
		return nil, NewStorageError(ErrorTypeInternal, "failed to open source file", err)
	}
	defer src.Close()

	processedData, format, width, height, err := a.processImage(src, req)
	if err != nil {
		return nil, NewStorageError(ErrorTypeInternal, "image processing failed", err)
	}

	// 原图保持原始文件名
	originalFileName := req.FileName
	objectPath, err := tenant.BuildObjectKey(req.UserID, req.FolderPath, originalFileName)
	if err != nil {
		return nil, NewStorageError(ErrorTypeInternal, "failed to build object key", err)
	}
	logicalPath := utils.BuildLogicalPath(req.FolderPath, originalFileName)

	savedBytes, err := io.ReadAll(processedData)
	if err != nil {
		return nil, NewStorageError(ErrorTypeInternal, "failed to buffer processed data", err)
	}
	hash := fmt.Sprintf("%x", md5.Sum(savedBytes))

	uploadResult, err := a.uploadToCOS(savedBytes, objectPath, req.ContentType)
	if err != nil {
		return nil, NewStorageError(ErrorTypeInternal, "failed to upload to COS", err)
	}

	var thumbnailPath string
	var thumbnailURL string
	var thumbRemoteDirect string
	var thumbnailErr error

	if req.Options != nil && req.Options.GenerateThumb {
		// 重新读取源数据用于缩略图生成
		srcFile, err := req.File.Open()
		if err == nil {
			defer srcFile.Close()
			sourceData, err := iox.ReadAllWithLimit(srcFile, iox.DefaultMaxReadBytes)
			if err == nil {
				// 使用 getThumbnailData 获取缩略图数据（优先使用预生成的，否则自动生成）
				thumbBytes, thumbFormat, _ := getThumbnailData(req, sourceData)
				if len(thumbBytes) > 0 {
					thumbFileName := utils.MakeThumbName(originalFileName, thumbFormat)
					thumbObjectPath, _ := tenant.BuildThumbObjectKey(req.UserID, req.FolderPath, thumbFileName)

					uploadResult, uploadErr := a.uploadToCOS(thumbBytes, thumbObjectPath, formats.GetContentType(thumbFormat))
					if uploadErr == nil {
						thumbnailPath = thumbObjectPath
						thumbnailURL = utils.BuildLogicalPath(req.FolderPath, thumbFileName)
						thumbRemoteDirect = uploadResult.URL
					} else {
						logger.Warn("[COS] 缩略图上传失败: %v", uploadErr)
						thumbnailErr = uploadErr
					}
				}
			} else {
				thumbnailErr = err
			}
		} else {
			thumbnailErr = err
		}
	}

	result := &UploadResult{
		OriginalPath:  objectPath,
		ThumbnailPath: thumbnailPath,
		URL:           logicalPath,  // 修复: 使用逻辑路径，不含 user_N/
		ThumbnailURL:  thumbnailURL, // 修复: 缩略图也使用逻辑路径
		FullURL:       uploadResult.URL,
		FullThumbURL: func() string {
			if thumbRemoteDirect != "" {
				return thumbRemoteDirect
			}
			return ""
		}(),
		RemoteURL:                 objectPath,
		RemoteThumbURL:            thumbnailPath,
		Size:                      uploadResult.Size,
		Width:                     width,
		Height:                    height,
		Hash:                      hash,
		ContentType:               a.getContentType(format),
		Format:                    format,
		ThumbnailGenerationFailed: thumbnailErr != nil,
		ThumbnailFailureReason: func() string {
			if thumbnailErr != nil {
				return thumbnailErr.Error()
			}
			return ""
		}(),
	}

	return result, nil
}

// Delete 删除文件
func (a *COSAdapter) Delete(ctx context.Context, path string) error {
	if !a.initialized {
		return NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	_, err := a.client.Object.Delete(ctx, path)
	return err
}

func (a *COSAdapter) GetURL(path string, options *URLOptions) (string, error) {
	if !a.initialized {
		return "", NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	// 如果设置了自定义域名，使用自定义域名
	if a.customDomain != "" {
		scheme := "https"
		if !a.useHTTPS {
			scheme = "http"
		}
		return fmt.Sprintf("%s://%s/%s", scheme, a.customDomain, encodePathSegments(path)), nil
	}

	scheme := "https"
	if !a.useHTTPS {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s.cos.%s.myqcloud.com/%s", scheme, a.bucket, a.region, encodePathSegments(path)), nil
}

func (a *COSAdapter) GetCapabilities() Capabilities {
	return Capabilities{
		SupportsSignedURL: true,
		SupportsCDN:       true,
		SupportsResize:    true,
		SupportsWebP:      true,
		MaxFileSize:       5 * 1024 * 1024 * 1024, // 5GB
		SupportedFormats:  []string{"jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "ico", "apng", "jp2", "tiff", "tif", "tga"},
	}
}

// 私有方法

// validateFile 验证文件
func (a *COSAdapter) validateFile(req *UploadRequest) error {
	if req.File.Size > 5*1024*1024*1024 { // 5GB
		return fmt.Errorf("file size exceeds COS limit")
	}
	// 统一扩展名白名单（与本地一致）
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(req.FileName)), ".")
	if !formats.IsSupported(ext) {
		return fmt.Errorf("file format not supported: .%s", ext)
	}
	// 使用验证中间件（宽松头校验 + MIME 协助）
	options := &middleware.ValidationOptions{
		MaxFileSize:     20 * 1024 * 1024, // 20MB（具体限制仍由上层与服务端设置控制）
		AllowedFormats:  formats.SupportedExtensionsWithDot(),
		CheckFileHeader: true,
		CheckMimeType:   true,
	}
	return middleware.ValidateSingleFile(req.File, options)
}

// processImage 处理图像
func (a *COSAdapter) processImage(src io.Reader, req *UploadRequest) (io.Reader, string, int, int, error) {
	if req.Options == nil {
		// 没有处理选项，直接读取原始数据
		data, err := io.ReadAll(src)
		if err != nil {
			return nil, "", 0, 0, err
		}

		width, height, format, err := decode.DetectFormat(bytes.NewReader(data))
		if err != nil {
			// 如果无法获取尺寸，可能不是图像文件
			return bytes.NewReader(data), a.getFileFormat(req.FileName), 0, 0, nil
		}

		return bytes.NewReader(data), format, width, height, nil
	}

	data, err := io.ReadAll(src)
	if err != nil {
		return nil, "", 0, 0, err
	}

	originalFormat := a.getFileFormat(req.FileName)
	currentData := bytes.NewReader(data)
	currentFormat := originalFormat
	var width, height int

	if w, h, f, err := decode.DetectFormat(bytes.NewReader(data)); err == nil {
		width, height = w, h
		if f != "" {
			currentFormat = f
		}
	}

	if req.Options.Compress && (req.Options.MaxWidth > 0 || req.Options.MaxHeight > 0) {
		compressOptions := &compress.Options{
			MaxWidth:  req.Options.MaxWidth,
			MaxHeight: req.Options.MaxHeight,
			Quality:   req.Options.Quality,
			Preserve:  true,
		}

		compressResult, err := compress.CompressFile(currentData, compressOptions)
		if err == nil {
			compressData, readErr := io.ReadAll(compressResult.Reader)
			if readErr == nil {
				currentData = bytes.NewReader(compressData)
				width = compressResult.Width
				height = compressResult.Height
			}
		} else {
			logger.Warn("图像压缩失败: %v", err)
		}
	}

	// 注意：WebP 转换已在 storage_service.go 的 convertToNewStorageRequest 中完成

	return currentData, currentFormat, width, height, nil
}


// uploadToCOS 上传数据到COS
func (a *COSAdapter) uploadToCOS(dataBytes []byte, objectPath, contentType string) (*UploadResult, error) {
	reader := bytes.NewReader(dataBytes)

	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: contentType,
		},
		ACLHeaderOptions: &cos.ACLHeaderOptions{
			XCosACL: a.accessControl, // 设置访问控制权限
		},
	}

	ctx := context.Background()
	_, err := a.client.Object.Put(ctx, objectPath, reader, opt)
	if err != nil {
		return nil, err
	}

	url, err := a.GetURL(objectPath, nil)
	if err != nil {
		return nil, err
	}

	return &UploadResult{
		URL:  url,
		Size: int64(len(dataBytes)),
	}, nil
}


// getFileFormat 获取文件格式
func (a *COSAdapter) getFileFormat(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	if len(ext) > 1 {
		return ext[1:] // 移除点号
	}
	return "unknown"
}

// getContentType 根据格式获取内容类型
func (a *COSAdapter) getContentType(format string) string {
	return formats.GetContentType(format)
}

// Exists 检查文件是否存在
func (a *COSAdapter) Exists(ctx context.Context, path string) (bool, error) {
	if !a.initialized {
		return false, NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	_, err := a.client.Object.Head(ctx, path, nil)
	if err != nil {
		// COS返回404时表示文件不存在
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *COSAdapter) SetObjectACL(ctx context.Context, path string, acl string) error {
	if !a.initialized {
		return NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	opt := &cos.ObjectPutACLOptions{
		Header: &cos.ACLHeaderOptions{
			XCosACL: acl, // 直接使用ACL字符串 (public-read/private)
		},
	}

	_, err := a.client.Object.PutACL(ctx, path, opt)
	if err != nil {
		logger.Error("COS设置对象ACL失败: %v", err)
		return NewStorageError(ErrorTypeInternal, "failed to set object ACL", err)
	}

	return nil
}

//

// ReadFile 读取文件内容
func (a *COSAdapter) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if !a.initialized {
		return nil, NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	resp, err := a.client.Object.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// GetBase64 获取文件的Base64编码
// GetBase64 / GetThumbnailBase64 已统一到 Manager 层实现

// HealthCheck 健康检查
func (a *COSAdapter) HealthCheck(ctx context.Context) error {
	if !a.initialized {
		return NewStorageError(ErrorTypeInternal, "adapter not initialized", nil)
	}

	// 尝试获取存储桶信息来验证连接
	_, err := a.client.Bucket.Head(ctx)
	return err
}

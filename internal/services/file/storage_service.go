package file

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"pixelpunk/internal/models"
	"pixelpunk/internal/services/storage"
	"pixelpunk/pkg/imagex/convert"
	"pixelpunk/pkg/logger"
	newstorage "pixelpunk/pkg/storage"
	"pixelpunk/pkg/utils"

	"github.com/google/uuid"
)

/* StorageChannelRepository 存储渠道仓库实现 */
type StorageChannelRepository struct{}

/* GetChannel 获取存储渠道 */
func (r *StorageChannelRepository) GetChannel(channelID string) (*models.StorageChannel, error) {
	return storage.GetChannelByID(channelID)
}

/* GetChannelConfig 获取渠道配置 */
func (r *StorageChannelRepository) GetChannelConfig(channelID string) (map[string]interface{}, error) {
	return storage.GetChannelConfigMap(channelID)
}

/* GetDefaultChannel 获取默认渠道 */
func (r *StorageChannelRepository) GetDefaultChannel() (*models.StorageChannel, error) {
	return storage.GetDefaultChannel()
}

/* GetActiveChannels 获取活跃渠道列表 */
func (r *StorageChannelRepository) GetActiveChannels() ([]*models.StorageChannel, error) {
	defaultChannel, err := r.GetDefaultChannel()
	if err != nil {
		return nil, err
	}

	return []*models.StorageChannel{defaultChannel}, nil
}

var storageService *newstorage.Storage

func ensureStorageServiceInitialized() (*newstorage.Storage, error) {
	if storageService != nil {
		return storageService, nil
	}

	channelRepo := &StorageChannelRepository{}

	storageService = newstorage.New(channelRepo)
	return storageService, nil
}

/* GetStorageServiceInstance 获取存储服务实例 */
func GetStorageServiceInstance() (*newstorage.Storage, error) {
	return ensureStorageServiceInitialized()
}

func convertToNewStorageRequest(ctx *UploadContext) *newstorage.UploadRequest {
	// 决定是否启用WebP转换
	webpEnabled := false
	webpQuality := 80

	if ctx.WebPEnabled != nil {
		// 用户指定了WebP开关，使用用户设置
		webpEnabled = *ctx.WebPEnabled
		logger.Info("[WebP调试] 使用用户指定的WebP开关: %v", webpEnabled)
	} else {
		// 使用全局配置
		webpEnabled = utils.GetWebPConvertEnabled()
		logger.Info("[WebP调试] 使用全局配置WebP开关: %v", webpEnabled)
	}

	if ctx.WebPQuality != nil {
		// 用户指定了WebP质量，使用用户设置
		webpQuality = *ctx.WebPQuality
	} else {
		// 使用全局配置
		webpQuality = utils.GetWebPConvertQuality()
	}
	logger.Info("[WebP调试] WebP质量: %d", webpQuality)

	// 生成唯一文件名（使用原始扩展名）
	uniqueFileName := generateUniqueFileName(ctx.File.Filename)
	logger.Info("[WebP调试] 原始文件名: %s, 生成的唯一文件名: %s", ctx.File.Filename, uniqueFileName)

	// 获取要处理的数据
	var processedData []byte
	if ctx.WatermarkWrapper != nil {
		if data, ok := ctx.WatermarkWrapper.([]byte); ok {
			processedData = data
			logger.Info("[WebP调试] 从WatermarkWrapper获取数据, 大小: %d bytes", len(processedData))
		}
	}

	// 如果没有预处理数据，从文件读取
	if len(processedData) == 0 && ctx.OriginalFileData != nil {
		processedData = ctx.OriginalFileData
		logger.Info("[WebP调试] 从OriginalFileData获取数据, 大小: %d bytes", len(processedData))
	}

	logger.Info("[WebP调试] 转换前状态: webpEnabled=%v, processedData大小=%d", webpEnabled, len(processedData))

	// 在调用存储之前进行 WebP 转换
	if webpEnabled && len(processedData) > 0 {
		logger.Info("[WebP调试] 开始转换(有预处理数据)...")
		webpResult, err := convert.ToWebP(processedData, convert.WebPOptions{Quality: webpQuality})
		if err == nil && webpResult.Converted {
			webpData, readErr := io.ReadAll(webpResult.Reader)
			if readErr == nil {
				processedData = webpData
				uniqueFileName = replaceExtensionWithWebP(uniqueFileName)
				logger.Info("[WebP调试] 转换成功! 新文件名: %s, 新大小: %d bytes", uniqueFileName, len(processedData))
			} else {
				logger.Warn("[WebP调试] 读取转换结果失败: %v", readErr)
			}
		} else if err != nil {
			logger.Warn("[WebP调试] 转换失败: %v", err)
		} else {
			logger.Info("[WebP调试] 转换返回但未转换 (Converted=false)")
		}
	} else if webpEnabled && len(processedData) == 0 {
		// 没有预处理数据，需要从文件读取并转换
		logger.Info("[WebP调试] 开始转换(从文件读取)...")
		file, err := ctx.File.Open()
		if err == nil {
			defer file.Close()
			data, readErr := io.ReadAll(file)
			if readErr == nil {
				logger.Info("[WebP调试] 从文件读取成功, 大小: %d bytes", len(data))
				webpResult, convErr := convert.ToWebP(data, convert.WebPOptions{Quality: webpQuality})
				if convErr == nil && webpResult.Converted {
					webpData, _ := io.ReadAll(webpResult.Reader)
					processedData = webpData
					uniqueFileName = replaceExtensionWithWebP(uniqueFileName)
					logger.Info("[WebP调试] 转换成功! 新文件名: %s, 新大小: %d bytes", uniqueFileName, len(processedData))
				} else if convErr != nil {
					logger.Warn("[WebP调试] 转换失败: %v", convErr)
				} else {
					logger.Info("[WebP调试] 转换返回但未转换 (Converted=false)")
				}
			} else {
				logger.Warn("[WebP调试] 读取文件失败: %v", readErr)
			}
		} else {
			logger.Warn("[WebP调试] 打开文件失败: %v", err)
		}
	} else {
		logger.Info("[WebP调试] 跳过转换: webpEnabled=%v, processedData大小=%d", webpEnabled, len(processedData))
	}

	logger.Info("[WebP调试] 最终文件名: %s, 最终数据大小: %d", uniqueFileName, len(processedData))

	req := &newstorage.UploadRequest{
		File:          ctx.File,
		UserID:        ctx.UserID,
		FolderPath:    ctx.FolderPath,
		FileName:      uniqueFileName,
		ContentType:   "", // 将自动检测
		Quality:       webpQuality,
		GenerateThumb: true,
		Compress:      false, // 原图默认不压缩尺寸
		WebPEnabled:   false, // 已经在这里转换过了，不需要适配器再转换
	}

	// 设置处理后的数据
	if len(processedData) > 0 {
		req.ProcessedData = processedData
	}

	if ctx.StorageChannel != nil {
		req.ChannelID = ctx.StorageChannel.ID
	}

	if ctx.CompressOptions != nil {
		req.Quality = ctx.CompressOptions.Quality
		req.ThumbWidth = ctx.CompressOptions.MaxWidth
		req.ThumbHeight = ctx.CompressOptions.MaxHeight
		req.ThumbQuality = ctx.CompressOptions.Quality

		if ctx.CompressOptions.MaxWidth > 2000 || ctx.CompressOptions.MaxHeight > 2000 {
			req.MaxWidth = ctx.CompressOptions.MaxWidth
			req.MaxHeight = ctx.CompressOptions.MaxHeight
			req.Compress = true
		}
	}

	return req
}

// replaceExtensionWithWebP 将文件扩展名替换为 .webp
func replaceExtensionWithWebP(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[:i] + ".webp"
		}
	}
	return filename + ".webp"
}

func convertFromNewStorageResult(result *newstorage.UploadResult) *UploadResult {
	return &UploadResult{
		URL:                       result.URL,
		LocalUrlPath:              result.OriginalPath,
		ThumbUrl:                  result.ThumbnailURL,
		LocalThumbPath:            result.ThumbnailPath,
		RemoteUrl:                 result.RemoteURL,
		RemoteThumbUrl:            result.RemoteThumbURL,
		Width:                     result.Width,
		Height:                    result.Height,
		ThumbnailGenerationFailed: result.ThumbnailGenerationFailed,
		ThumbnailFailureReason:    result.ThumbnailFailureReason,
	}
}

func generateUniqueFileName(originalName string) string {
	ext := filepath.Ext(originalName)

	uuidStr := strings.ReplaceAll(uuid.New().String(), "-", "")
	timestamp := fmt.Sprintf("%04d", time.Now().Unix()%10000) // 取时间戳后4位
	uniqueName := uuidStr + timestamp

	return uniqueName + ext
}

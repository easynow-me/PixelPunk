package utils

import (
	"sync"
)

// 全局设置缓存结构体，统一管理各种系统设置
type globalSettingsCache struct {
	baseUrl              string
	hideRemoteUrl        bool
	aiAnalysisEnabled    bool   // AI分析功能是否启用
	adminEmail           string // 管理员邮箱
	strictFileValidation bool   // 严格文件验证（验证文件头与扩展名是否匹配）
	webpConvertEnabled   bool   // WebP转换功能是否启用
	webpConvertQuality   int    // WebP转换质量 (1-100)
	mutex                sync.RWMutex
}

// 全局单例
var globalSettings = &globalSettingsCache{
	hideRemoteUrl:        true,
	aiAnalysisEnabled:    true,
	adminEmail:           "",    // 默认管理员邮箱
	strictFileValidation: true,  // 默认启用严格文件验证
	webpConvertEnabled:   false, // 默认禁用WebP转换
	webpConvertQuality:   80,    // 默认WebP质量80%
}

func SetBaseUrl(baseUrl string) {
	globalSettings.mutex.Lock()
	defer globalSettings.mutex.Unlock()

	globalSettings.baseUrl = baseUrl
}

func GetBaseUrl() string {
	globalSettings.mutex.RLock()
	defer globalSettings.mutex.RUnlock()

	return globalSettings.baseUrl
}

func SetHideRemoteUrl(hide bool) {
	globalSettings.mutex.Lock()
	defer globalSettings.mutex.Unlock()

	globalSettings.hideRemoteUrl = hide
}

func GetHideRemoteUrl() bool {
	globalSettings.mutex.RLock()
	defer globalSettings.mutex.RUnlock()

	return globalSettings.hideRemoteUrl
}

func SetAiAnalysisEnabled(enabled bool) {
	globalSettings.mutex.Lock()
	defer globalSettings.mutex.Unlock()

	globalSettings.aiAnalysisEnabled = enabled
}

func GetAiAnalysisEnabled() bool {
	globalSettings.mutex.RLock()
	defer globalSettings.mutex.RUnlock()

	return globalSettings.aiAnalysisEnabled
}

func SetAdminEmail(email string) {
	globalSettings.mutex.Lock()
	defer globalSettings.mutex.Unlock()

	globalSettings.adminEmail = email
}

func GetAdminEmail() string {
	globalSettings.mutex.RLock()
	defer globalSettings.mutex.RUnlock()

	return globalSettings.adminEmail
}

func SetStrictFileValidation(enabled bool) {
	globalSettings.mutex.Lock()
	defer globalSettings.mutex.Unlock()

	globalSettings.strictFileValidation = enabled
}

func GetStrictFileValidation() bool {
	globalSettings.mutex.RLock()
	defer globalSettings.mutex.RUnlock()

	return globalSettings.strictFileValidation
}

func SetWebPConvertEnabled(enabled bool) {
	globalSettings.mutex.Lock()
	defer globalSettings.mutex.Unlock()

	globalSettings.webpConvertEnabled = enabled
}

func GetWebPConvertEnabled() bool {
	globalSettings.mutex.RLock()
	defer globalSettings.mutex.RUnlock()

	return globalSettings.webpConvertEnabled
}

func SetWebPConvertQuality(quality int) {
	globalSettings.mutex.Lock()
	defer globalSettings.mutex.Unlock()

	// 确保质量值在有效范围内
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}
	globalSettings.webpConvertQuality = quality
}

func GetWebPConvertQuality() int {
	globalSettings.mutex.RLock()
	defer globalSettings.mutex.RUnlock()

	return globalSettings.webpConvertQuality
}

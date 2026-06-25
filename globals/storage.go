package globals

import "time"

// 图片/文件存储相关配置（运维级，来源于 config.yaml，由 utils.ReadConf 在启动时写入）。
// 作为全局单一来源，被即梦适配器、Photo 图片处理、静态路由、清理任务统一引用，
// 避免各处硬编码导致目录/清理策略不一致。

// 公共访问前缀为常量，与静态路由挂载点保持一致；底层文件系统目录可经配置覆盖，
// 但访问 URL 始终以这两个前缀开头，因此修改目录不影响已生成的 URL。
const (
	StorageResultURLPrefix = "/storage/results/"
	StorageUploadURLPrefix = "/storage/uploads/"
)

var (
	// StorageResultDir 生成结果（图片/视频）落地目录。
	StorageResultDir = "storage/results"
	// StorageUploadDir 用户上传文件落地目录。
	StorageUploadDir = "storage/uploads"

	// StorageTTLHours 结果/上传文件保留小时数，超过即被清理任务删除；<=0 表示永久保留（关闭 TTL 清理）。
	StorageTTLHours int64 = 0
	// StorageMaxSizeMB 结果+上传目录总容量上限（MB），超过则按最旧优先删除直至低于上限；<=0 表示不限制。
	StorageMaxSizeMB int64 = 0
	// StorageCleanupIntervalMin 清理任务的执行间隔（分钟），<=0 时回退为 60。
	StorageCleanupIntervalMin int64 = 60

	// ImageMaxConcurrentPerUser 单用户同时进行中的图片任务上限（pending+processing），<=0 表示不限制。
	ImageMaxConcurrentPerUser int64 = 0

	// ImageTaskTimeoutMinutes 单个图片/视频生成任务的轮询最长等待（分钟），<=0 回退默认 10。
	ImageTaskTimeoutMinutes int64 = 10
	// ImagePollIntervalSeconds 任务状态轮询间隔（秒），<=0 回退默认 10。
	ImagePollIntervalSeconds int64 = 10
)

// ImageTaskTimeout 返回单任务轮询最长等待时长，未配置或非法时回退 10 分钟。
func ImageTaskTimeout() time.Duration {
	if ImageTaskTimeoutMinutes <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(ImageTaskTimeoutMinutes) * time.Minute
}

// ImagePollInterval 返回任务状态轮询间隔，未配置或非法时回退 10 秒。
func ImagePollInterval() time.Duration {
	if ImagePollIntervalSeconds <= 0 {
		return 10 * time.Second
	}
	return time.Duration(ImagePollIntervalSeconds) * time.Second
}

// ResultPublicURL 把结果文件名转换为对外可访问的 URL。
func ResultPublicURL(filename string) string {
	return StorageResultURLPrefix + filename
}

// UploadPublicURL 把上传文件名转换为对外可访问的 URL。
func UploadPublicURL(filename string) string {
	return StorageUploadURLPrefix + filename
}

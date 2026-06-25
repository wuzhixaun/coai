package utils

import (
	"chat/globals"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/viper"
)

// ApplyStorageConfig 读取 config.yaml 的 storage.* / image.* 配置并写入 globals，
// 未配置的键保留 globals 中的默认值。在 ReadConf 中调用。
func ApplyStorageConfig() {
	if v := viper.GetString("storage.result_dir"); v != "" {
		globals.StorageResultDir = v
	}
	if v := viper.GetString("storage.upload_dir"); v != "" {
		globals.StorageUploadDir = v
	}
	if viper.IsSet("storage.ttl_hours") {
		globals.StorageTTLHours = viper.GetInt64("storage.ttl_hours")
	}
	if viper.IsSet("storage.max_size_mb") {
		globals.StorageMaxSizeMB = viper.GetInt64("storage.max_size_mb")
	}
	if viper.IsSet("storage.cleanup_interval_minutes") {
		globals.StorageCleanupIntervalMin = viper.GetInt64("storage.cleanup_interval_minutes")
	}
	if viper.IsSet("image.max_concurrent_tasks_per_user") {
		globals.ImageMaxConcurrentPerUser = viper.GetInt64("image.max_concurrent_tasks_per_user")
	}
	if viper.IsSet("image.task_timeout_minutes") {
		globals.ImageTaskTimeoutMinutes = viper.GetInt64("image.task_timeout_minutes")
	}
	if viper.IsSet("image.poll_interval_seconds") {
		globals.ImagePollIntervalSeconds = viper.GetInt64("image.poll_interval_seconds")
	}
}

type storageEntry struct {
	path    string
	size    int64
	modTime time.Time
}

// collectStorageEntries 收集目录下的常规文件（仅顶层，不递归子目录）。
func collectStorageEntries(dir string) []storageEntry {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	entries := make([]storageEntry, 0, len(items))
	for _, it := range items {
		if it.IsDir() {
			continue
		}
		info, err := it.Info()
		if err != nil {
			continue
		}
		entries = append(entries, storageEntry{
			path:    filepath.Join(dir, it.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}
	return entries
}

// enforceStorageLimits 对给定目录应用 TTL 与总容量限制。
//   - ttl>0：删除修改时间早于 now-ttl 的文件；ttl<=0 关闭 TTL。
//   - maxBytes>0：若总大小超过 maxBytes，则按最旧优先删除直至不超过；maxBytes<=0 关闭。
//
// 返回删除文件数与释放字节数。该函数为纯逻辑（注入 now），便于测试。
func enforceStorageLimits(dirs []string, ttl time.Duration, maxBytes int64, now time.Time) (removed int, freed int64) {
	if ttl <= 0 && maxBytes <= 0 {
		return 0, 0
	}

	var entries []storageEntry
	for _, dir := range dirs {
		entries = append(entries, collectStorageEntries(dir)...)
	}

	// 按修改时间从旧到新排序，便于 TTL 与容量两轮都优先处理最旧文件。
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].modTime.Before(entries[j].modTime)
	})

	kept := entries[:0:0]
	// 第一轮：TTL。
	if ttl > 0 {
		cutoff := now.Add(-ttl)
		for _, e := range entries {
			if e.modTime.Before(cutoff) {
				if err := os.Remove(e.path); err == nil {
					removed++
					freed += e.size
				}
				continue
			}
			kept = append(kept, e)
		}
	} else {
		kept = append(kept, entries...)
	}

	// 第二轮：总容量上限（最旧优先删除）。
	if maxBytes > 0 {
		var total int64
		for _, e := range kept {
			total += e.size
		}
		for _, e := range kept {
			if total <= maxBytes {
				break
			}
			if err := os.Remove(e.path); err == nil {
				removed++
				freed += e.size
				total -= e.size
			}
		}
	}

	return removed, freed
}

// StartStorageCleanup 启动后台清理任务：启动时执行一次，之后按间隔周期执行。
// 当 TTL 与容量上限都未配置（均 <=0）时为关闭状态，直接返回，不产生任何删除行为。
func StartStorageCleanup() {
	ttl := time.Duration(globals.StorageTTLHours) * time.Hour
	maxBytes := globals.StorageMaxSizeMB * 1024 * 1024
	if ttl <= 0 && maxBytes <= 0 {
		globals.Debug("[storage] cleanup disabled (set storage.ttl_hours and/or storage.max_size_mb to enable)")
		return
	}

	interval := time.Duration(globals.StorageCleanupIntervalMin) * time.Minute
	if interval <= 0 {
		interval = time.Hour
	}
	dirs := []string{globals.StorageResultDir, globals.StorageUploadDir}

	runOnce := func() {
		removed, freed := enforceStorageLimits(dirs, ttl, maxBytes, time.Now())
		if removed > 0 {
			globals.Info(fmt.Sprintf("[storage] cleanup removed %d file(s), freed %.2f MB", removed, float64(freed)/1024/1024))
		}
	}

	go func() {
		runOnce()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			runOnce()
		}
	}()

	globals.Info(fmt.Sprintf("[storage] cleanup enabled (ttl=%dh, max=%dMB, interval=%dm)",
		globals.StorageTTLHours, globals.StorageMaxSizeMB, globals.StorageCleanupIntervalMin))
}

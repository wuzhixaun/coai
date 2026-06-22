package photo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// ── 提示词配置结构 ──────────────────────────────────────────

type PromptConfig struct {
	Defaults map[string]string        `json:"defaults"`
	Features map[string]FeatureConfig `json:"features"`
}

type FeatureConfig struct {
	ChannelType  string            `json:"channel_type"`
	Model        string            `json:"model"`
	SystemPrompt string            `json:"system_prompt"`
	Templates    []PromptTemplate  `json:"templates,omitempty"`
	Colors       []PromptOption    `json:"colors,omitempty"`
	Languages    []PromptOption    `json:"languages,omitempty"`
	SellingPoints []PromptOption   `json:"selling_points,omitempty"`
	Sizes        []PromptOption    `json:"sizes,omitempty"`
	Categories   []PromptOption    `json:"categories,omitempty"`
}

type PromptTemplate struct {
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
}

type PromptOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ── 全局单例 ───────────────────────────────────────────────

var (
	promptConfig     *PromptConfig
	promptConfigMu   sync.RWMutex
)

const promptsConfigFile = "config/prompts.json"

// ── 加载 ───────────────────────────────────────────────────

// resolvePromptsPath 多策略查找 prompts.json 路径：
// 1. 当前工作目录 (./config/prompts.json)
// 2. 可执行文件目录
// 3. 源码目录 (通过 runtime.Caller 定位，支持 go test)
func resolvePromptsPath() string {
	candidates := []string{
		promptsConfigFile, // 相对于工作目录
	}

	// 相对于可执行文件
	if execPath, err := os.Executable(); err == nil {
		candidates = append(candidates,
			filepath.Join(filepath.Dir(execPath), promptsConfigFile))
	}

	// 相对于源码文件 (通过 runtime.Caller 定位到 prompts.go 的位置)
	// prompts.go 位于 addition/photo/，项目根目录在其上两级
	if _, file, _, ok := runtime.Caller(0); ok {
		srcRoot := filepath.Dir(filepath.Dir(filepath.Dir(file))) // addition/photo → addition → project root
		candidates = append(candidates,
			filepath.Join(srcRoot, promptsConfigFile),
			filepath.Join(srcRoot, "config", "prompts.json"),
		)
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return promptsConfigFile // fallback to original path, will fail with clear error
}

// LoadPrompts 从 config/prompts.json 加载提示词配置
func LoadPrompts() error {
	path := resolvePromptsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg PromptConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	promptConfigMu.Lock()
	promptConfig = &cfg
	promptConfigMu.Unlock()

	return nil
}

// ReloadPrompts 热重载提示词配置
func ReloadPrompts() error {
	return LoadPrompts()
}

// getConfig 线程安全地获取配置副本
func getConfig() *PromptConfig {
	promptConfigMu.RLock()
	defer promptConfigMu.RUnlock()
	if promptConfig == nil {
		return nil
	}
	// 返回浅拷贝（只读场景安全）
	return promptConfig
}

// ── 查询接口 ───────────────────────────────────────────────

// GetFeatureConfig 获取某个功能的完整配置
func GetFeatureConfig(feature string) *FeatureConfig {
	cfg := getConfig()
	if cfg == nil {
		return nil
	}
	if f, ok := cfg.Features[feature]; ok {
		return &f
	}
	return nil
}

// GetSystemPrompt 获取替换占位符后的 system prompt
// 先替换 defaults 中的占位符，再替换 kwargs
func GetSystemPrompt(feature string, kwargs map[string]string) string {
	cfg := getConfig()
	if cfg == nil {
		return ""
	}

	fc, ok := cfg.Features[feature]
	if !ok {
		return ""
	}

	prompt := fc.SystemPrompt

	// 步骤1: 替换 defaults 中的占位符
	for key, value := range cfg.Defaults {
		prompt = strings.ReplaceAll(prompt, "{"+key+"}", value)
	}

	// 步骤2: 替换 kwargs 中的占位符
	for key, value := range kwargs {
		prompt = strings.ReplaceAll(prompt, "{"+key+"}", value)
	}

	return prompt
}

// GetTemplates 获取功能的提示词模板列表
func GetTemplates(feature string) []PromptTemplate {
	fc := GetFeatureConfig(feature)
	if fc == nil {
		return nil
	}
	return fc.Templates
}

// GetOptions 获取功能的选项列表（colors / languages / selling_points / sizes）
func GetOptions(feature string, optionKey string) []PromptOption {
	fc := GetFeatureConfig(feature)
	if fc == nil {
		return nil
	}
	switch optionKey {
	case "colors":
		return fc.Colors
	case "languages":
		return fc.Languages
	case "selling_points":
		return fc.SellingPoints
	case "sizes":
		return fc.Sizes
	case "categories":
		return fc.Categories
	default:
		return nil
	}
}

// GetChannelType 获取功能默认使用的 AI 渠道类型
func GetChannelType(feature string) string {
	fc := GetFeatureConfig(feature)
	if fc == nil {
		return ""
	}
	return fc.ChannelType
}

// GetModel 获取功能默认使用的模型
func GetModel(feature string) string {
	fc := GetFeatureConfig(feature)
	if fc == nil {
		return ""
	}
	return fc.Model
}

// GetDefaults 返回全局默认占位符
func GetDefaults() map[string]string {
	cfg := getConfig()
	if cfg == nil {
		return nil
	}
	return cfg.Defaults
}

// GetFeaturesConfig 返回完整配置（供前端 API 使用）
func GetFeaturesConfig() map[string]interface{} {
	cfg := getConfig()
	if cfg == nil {
		return nil
	}
	return map[string]interface{}{
		"features": cfg.Features,
		"defaults": cfg.Defaults,
	}
}

// ── 工具函数 ───────────────────────────────────────────────

// BuildPrompt 构建完整 prompt：可选自定义 system_prompt，否则从配置加载
func BuildPrompt(feature string, systemPrompt string, kwargs map[string]string) string {
	if systemPrompt != "" {
		return systemPrompt
	}
	return GetSystemPrompt(feature, kwargs)
}

// init 自动加载提示词配置
func init() {
	if err := LoadPrompts(); err != nil {
		println("[photo/prompts] failed to load prompts config: " + err.Error())
	}
}

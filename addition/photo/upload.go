package photo

import (
	"chat/globals"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

// ── 上传配置 ────────────────────────────────────────────────

const (
	MaxUploadSize = 50 * 1024 * 1024 // 50MB
	StorageBase   = "storage"
	MaxImageWidth = 2048
)

// 存储目录统一引用 globals（可经 config.yaml 覆盖），与即梦适配器、静态路由、清理任务保持一致。
// 通过函数在调用时读取全局变量，避免包初始化早于配置加载导致拿到旧默认值。
func UploadDir() string { return globals.StorageUploadDir }
func ResultDir() string { return globals.StorageResultDir }

var AllowedExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
	".bmp":  true,
	".tiff": true,
}

// ── 文件校验 ────────────────────────────────────────────────

// ValidateFileFormat 检查文件扩展名是否允许
func ValidateFileFormat(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return AllowedExtensions[ext]
}

// ValidateFileSize 检查文件大小是否超过限制
func ValidateFileSize(size int64) bool {
	return size <= MaxUploadSize
}

// ── 文件存储 ────────────────────────────────────────────────

// generateImageID 生成12位十六进制图片ID
func generateImageID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// generateFilename 生成UUID文件名（保留原始扩展名）
func generateFilename(originalName string) string {
	b := make([]byte, 16)
	rand.Read(b)
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		ext = ".png"
	}
	return fmt.Sprintf("%x%s", b, ext)
}

// ensureStorageDir 确保存储目录存在
func ensureStorageDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// SaveUploadFile 保存上传文件到 storage/uploads/，写入DB记录，返回 ImageInfo
// db: 数据库连接（从 gin.Context 获取）
func SaveUploadFile(file *multipart.FileHeader, db *sql.DB, userID int64, folderName string) (*ImageInfo, error) {
	// 1. 校验
	if !ValidateFileFormat(file.Filename) {
		return nil, fmt.Errorf("不支持的文件格式: %s", file.Filename)
	}
	if !ValidateFileSize(file.Size) {
		return nil, fmt.Errorf("文件过大: %s (最大 50MB)", file.Filename)
	}

	// 2. 确保目录存在
	if err := ensureStorageDir(UploadDir()); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}

	// 3. 打开源文件
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer src.Close()

	// 4. 生成唯一文件名
	saveName := generateFilename(file.Filename)
	savePath := filepath.Join(UploadDir(), saveName)

	// 5. 写入磁盘
	dst, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(savePath)
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	// 6. 生成图片ID + URL
	imageID := generateImageID()
	url := globals.UploadPublicURL(saveName)

	// 7. 写入数据库
	absPath, _ := filepath.Abs(savePath)
	if err := insertImageRecord(db, imageID, userID, file.Filename, file.Size, url, absPath, folderName); err != nil {
		os.Remove(savePath)
		return nil, fmt.Errorf("写入数据库失败: %w", err)
	}

	return &ImageInfo{
		Id:         imageID,
		Filename:   file.Filename,
		Size:       file.Size,
		Url:        url,
		FolderName: folderName,
	}, nil
}

// SaveImageBytes 把内存中的图片字节落盘并写库（供 URL 抓图等非 multipart 来源复用）。
func SaveImageBytes(db *sql.DB, userID int64, filename string, data []byte, folderName string) (*ImageInfo, error) {
	if !ValidateFileFormat(filename) {
		return nil, fmt.Errorf("不支持的文件格式: %s", filename)
	}
	if !ValidateFileSize(int64(len(data))) {
		return nil, fmt.Errorf("文件过大 (最大 50MB)")
	}
	if err := ensureStorageDir(UploadDir()); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}
	saveName := generateFilename(filename)
	savePath := filepath.Join(UploadDir(), saveName)
	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}
	imageID := generateImageID()
	url := globals.UploadPublicURL(saveName)
	absPath, _ := filepath.Abs(savePath)
	if err := insertImageRecord(db, imageID, userID, filename, int64(len(data)), url, absPath, folderName); err != nil {
		os.Remove(savePath)
		return nil, fmt.Errorf("写入数据库失败: %w", err)
	}
	return &ImageInfo{Id: imageID, Filename: filename, Size: int64(len(data)), Url: url, FolderName: folderName}, nil
}

// DeleteImageFile 删除图片文件和数据库记录
func DeleteImageFile(db *sql.DB, imageID string, userID int64) error {
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}

	// 查询文件路径
	var filePath string
	err := db.QueryRow(
		"SELECT file_path FROM photo_images WHERE id = ? AND user_id = ?",
		imageID, userID,
	).Scan(&filePath)
	if err != nil {
		return fmt.Errorf("图片不存在")
	}

	// 删除数据库记录
	_, err = db.Exec("DELETE FROM photo_images WHERE id = ? AND user_id = ?", imageID, userID)
	if err != nil {
		return fmt.Errorf("删除记录失败: %w", err)
	}

	// 删除文件
	if filePath != "" {
		os.Remove(filePath)
	}

	return nil
}

// ── 数据库操作 ──────────────────────────────────────────────

func insertImageRecord(db *sql.DB, id string, userID int64, filename string, size int64, url, filePath, folderName string) error {
	_, err := db.Exec(`
		INSERT INTO photo_images (id, user_id, filename, size, url, file_path, folder_name)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, userID, filename, size, url, filePath, folderName)
	return err
}

func queryImagesByUser(db *sql.DB, userID int64, limit, offset int) ([]ImageInfo, error) {
	rows, err := db.Query(`
		SELECT id, filename, size, width, height, url, folder_name, created_at
		FROM photo_images
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []ImageInfo
	for rows.Next() {
		var img ImageInfo
		var createdAt string
		if err := rows.Scan(&img.Id, &img.Filename, &img.Size, &img.Width, &img.Height, &img.Url, &img.FolderName, &createdAt); err != nil {
			continue
		}
		img.CreatedAt = createdAt
		images = append(images, img)
	}
	return images, nil
}

func queryImageByID(db *sql.DB, imageID string, userID int64) (*ImageInfo, error) {
	var img ImageInfo
	var createdAt string
	err := db.QueryRow(`
		SELECT id, filename, size, width, height, url, folder_name, created_at
		FROM photo_images
		WHERE id = ? AND user_id = ?
	`, imageID, userID).Scan(&img.Id, &img.Filename, &img.Size, &img.Width, &img.Height, &img.Url, &img.FolderName, &createdAt)
	if err != nil {
		return nil, err
	}
	img.CreatedAt = createdAt
	return &img, nil
}


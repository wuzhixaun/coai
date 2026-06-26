package connection

import (
	"chat/globals"
	"chat/utils"
	"crypto/tls"
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
)

var DB *sql.DB

func InitMySQLSafe() *sql.DB {
	ConnectDatabase()

	// using DB as a global variable to point to the latest db connection
	MysqlWorker(DB)
	return DB
}

func getConn() *sql.DB {
	if viper.GetString("mysql.host") == "" {
		globals.SqliteEngine = true
		globals.Warn("[connection] mysql host is not set, using sqlite (~/db/chatnio.db)")
		db, err := sql.Open("sqlite3", utils.FileSafe("./db/chatnio.db"))
		if err != nil {
			panic(err)
		}

		return db
	}

	mysqlUrl := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s",
		viper.GetString("mysql.user"),
		viper.GetString("mysql.password"),
		viper.GetString("mysql.host"),
		viper.GetInt("mysql.port"),
		utils.GetStringConfs("mysql.database", "mysql.db"),
	)
	if viper.GetBool("mysql.tls") {
		mysql.RegisterTLSConfig("tls", &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: viper.GetString("mysql.host"),
		})

		mysqlUrl += "?tls=tls"
	}

	// connect to MySQL
	db, err := sql.Open("mysql", mysqlUrl)

	if pingErr := db.Ping(); err != nil || pingErr != nil {
		errMsg := utils.Multi[string](err != nil, utils.GetError(err), utils.GetError(pingErr)) // err.Error() may contain nil pointer
		globals.Warn(
			fmt.Sprintf("[connection] failed to connect to mysql server: %s (message: %s), will retry in 5 seconds",
				viper.GetString("mysql.host"), errMsg,
			),
		)

		utils.Sleep(5000)
		db.Close()

		return getConn()
	}

	globals.Debug(fmt.Sprintf("[connection] connected to mysql server (host: %s)", viper.GetString("mysql.host")))
	return db
}

func ConnectDatabase() *sql.DB {
	db := getConn()

	db.SetMaxOpenConns(512)
	db.SetMaxIdleConns(64)

	CreateUserTable(db)
	CreateConversationTable(db)
	CreateMaskTable(db)
	CreateSharingTable(db)
	CreatePackageTable(db)
	CreateQuotaTable(db)
	CreateSubscriptionTable(db)
	CreateApiKeyTable(db)
	CreateInvitationTable(db)
	CreateRedeemTable(db)
	CreateBroadcastTable(db)
	CreateRecordTable(db)
	CreatePaymentTable(db)
	CreatePhotoImagesTable(db)
	CreatePhotoTasksTable(db)
	CreatePhotoIdentityTable(db)
	CreatePhotoRecipeTable(db)
	CreateImageGenerationTable(db)

	if err := doMigration(db); err != nil {
		fmt.Println(fmt.Sprintf("migration error: %s", err))
	}

	DB = db

	return db
}

func InitRootUser(db *sql.DB) {
	// create root user if totally empty
	var count int
	err := globals.QueryRowDb(db, "SELECT COUNT(*) FROM auth").Scan(&count)
	if err != nil {
		globals.Warn(fmt.Sprintf("[service] failed to query user count: %s", err.Error()))
		return
	}

	if count == 0 {
		globals.Debug("[service] no user found, creating root user (username: root, password: chatnio123456, email: root@example.com)")
		_, err := globals.ExecDb(db, `
			INSERT INTO auth (username, password, email, is_admin, bind_id, token)
			VALUES (?, ?, ?, ?, ?, ?)
		`, "root", utils.Sha2Encrypt("chatnio123456"), "root@example.com", true, 0, "root")
		if err != nil {
			globals.Warn(fmt.Sprintf("[service] failed to create root user: %s", err.Error()))
		}
	} else {
		globals.Debug(fmt.Sprintf("[service] %d user(s) found, skip creating root user", count))
	}
}

func CreateUserTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS auth (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  bind_id INT UNIQUE,
		  username VARCHAR(24) UNIQUE,
		  token VARCHAR(255) NOT NULL,
		  email VARCHAR(255) UNIQUE,
		  password VARCHAR(64) NOT NULL,
		  is_admin BOOLEAN DEFAULT FALSE,
		  is_banned BOOLEAN DEFAULT FALSE
		);
	`)
	if err != nil {
		fmt.Println(err)
	}

	InitRootUser(db)
}

func CreatePackageTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS package (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  user_id INT,
		  type VARCHAR(255),
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  FOREIGN KEY (user_id) REFERENCES auth(id),
		  UNIQUE KEY (user_id, type)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateQuotaTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS quota (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  user_id INT UNIQUE,
		  quota DECIMAL(24, 6),
		  used DECIMAL(24, 6),
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  FOREIGN KEY (user_id) REFERENCES auth(id)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateConversationTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS conversation (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  user_id INT,
		  conversation_id INT,
		  conversation_name VARCHAR(255),
		  data MEDIUMTEXT,
		  model VARCHAR(255) NOT NULL DEFAULT 'gpt-3.5-turbo-0613',
		  task_id VARCHAR(255) NULL,
		  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  UNIQUE KEY (user_id, conversation_id)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateMaskTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS mask (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  user_id INT,
		  avatar VARCHAR(255),
		  name VARCHAR(255),
		  description TEXT,
		  context TEXT,
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  FOREIGN KEY (user_id) REFERENCES auth(id)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateSharingTable(db *sql.DB) {
	// refs is an array of message id, separated by comma (-1 means all messages)
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS sharing (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  hash CHAR(32) UNIQUE,
		  user_id INT,
		  conversation_id INT,
		  refs TEXT,
		  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  FOREIGN KEY (user_id) REFERENCES auth(id)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateSubscriptionTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS subscription (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  level INT DEFAULT 1,
		  user_id INT UNIQUE,
		  expired_at DATETIME,
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  total_month INT DEFAULT 0,
		  enterprise BOOLEAN DEFAULT FALSE,
		  FOREIGN KEY (user_id) REFERENCES auth(id)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateApiKeyTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS apikey (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  user_id INT UNIQUE,
		  api_key VARCHAR(255) UNIQUE,
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  FOREIGN KEY (user_id) REFERENCES auth(id)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateInvitationTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS invitation (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  code VARCHAR(255) UNIQUE,
		  quota DECIMAL(16, 4),
		  type VARCHAR(255),
		  used BOOLEAN DEFAULT FALSE,
		  used_id INT,
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  UNIQUE KEY (used_id, type),
		  FOREIGN KEY (used_id) REFERENCES auth(id)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateRedeemTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS redeem (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  code VARCHAR(255) UNIQUE,
		  quota DECIMAL(16, 4),
		  used BOOLEAN DEFAULT FALSE,
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateBroadcastTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS broadcast (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  poster_id INT,
		  content TEXT,
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  FOREIGN KEY (poster_id) REFERENCES auth(id)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateRecordTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS record (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  user_id INT,
		  username VARCHAR(24),
		  type VARCHAR(24),
		  token_name VARCHAR(255),
		  model VARCHAR(255),
		  input_tokens INT DEFAULT 0,
		  output_tokens INT DEFAULT 0,
		  quota DECIMAL(16, 4) DEFAULT 0,
		  duration DECIMAL(10, 2) DEFAULT 0,
		  detail VARCHAR(255),
		  prompts TEXT,
		  response_prompts TEXT,
		  channel INT DEFAULT 0,
		  channel_name VARCHAR(255),
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  INDEX idx_user (user_id),
		  INDEX idx_username (username),
		  INDEX idx_created_at (created_at),
		  INDEX idx_type (type)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreatePaymentTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS payment (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  user_id INT,
		  username VARCHAR(24),
		  type VARCHAR(24),
		  service VARCHAR(255),
		  amount DECIMAL(16, 4),
		  order_id VARCHAR(255) UNIQUE,
		  name VARCHAR(255),
		  device VARCHAR(255),
		  state BOOLEAN DEFAULT FALSE,
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  INDEX idx_user (user_id),
		  INDEX idx_order (order_id),
		  INDEX idx_state (state)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreatePhotoImagesTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS photo_images (
		  id VARCHAR(12) PRIMARY KEY,
		  user_id BIGINT NOT NULL,
		  filename VARCHAR(255) NOT NULL,
		  size BIGINT NOT NULL,
		  width INT DEFAULT 0,
		  height INT DEFAULT 0,
		  url VARCHAR(512) NOT NULL,
		  file_path VARCHAR(512) NOT NULL,
		  folder_name VARCHAR(255) DEFAULT '',
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  INDEX idx_user (user_id),
		  INDEX idx_folder (folder_name)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

// CreateImageGenerationTable 图片生成观测表：记录三条入口（chat/api/photo）的图片任务，
// 保存张数、计费、火山 task_id/request_id/code 与失败原因，供后台用量统计与售后排查。
func CreateImageGenerationTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS image_generation (
		  id INT PRIMARY KEY AUTO_INCREMENT,
		  user_id INT,
		  username VARCHAR(24),
		  source VARCHAR(16) DEFAULT 'api',
		  model VARCHAR(255),
		  channel INT DEFAULT 0,
		  channel_name VARCHAR(255),
		  image_count INT DEFAULT 0,
		  quota DECIMAL(16, 4) DEFAULT 0,
		  duration DECIMAL(10, 2) DEFAULT 0,
		  status VARCHAR(16) DEFAULT 'success',
		  task_id VARCHAR(255),
		  request_id VARCHAR(255),
		  code INT DEFAULT 0,
		  message VARCHAR(512),
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  INDEX idx_user (user_id),
		  INDEX idx_model (model),
		  INDEX idx_status (status),
		  INDEX idx_source (source),
		  INDEX idx_created_at (created_at)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

// CreatePhotoIdentityTable 商品/模特一致性身份：保存一组参考图(引用 photo_images)、
// 锁定的 seed 与主体描述 prompt，用于跨场景/跨功能保持主体一致（Phase 1 一致性引擎）。
func CreatePhotoIdentityTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS photo_identity (
		  id VARCHAR(12) PRIMARY KEY,
		  user_id BIGINT NOT NULL,
		  type VARCHAR(16) NOT NULL DEFAULT 'product',
		  name VARCHAR(255) NOT NULL,
		  ref_image_ids TEXT NOT NULL,
		  seed INT DEFAULT -1,
		  subject_prompt TEXT,
		  meta TEXT,
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  INDEX idx_user (user_id),
		  INDEX idx_type (type)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

// CreatePhotoRecipeTable 配方：用户保存的命名工作流（有序步骤），便于复用标准化流程。
func CreatePhotoRecipeTable(db *sql.DB) {
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS photo_recipe (
		  id VARCHAR(12) PRIMARY KEY,
		  user_id BIGINT NOT NULL,
		  name VARCHAR(255) NOT NULL,
		  steps TEXT NOT NULL,
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  INDEX idx_user (user_id)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

func CreatePhotoTasksTable(db *sql.DB) {
	// MySQL strict mode 不允许 TEXT 列有 DEFAULT，所以 TEXT 列不设默认值
	_, err := globals.ExecDb(db, `
		CREATE TABLE IF NOT EXISTS photo_tasks (
		  task_id VARCHAR(12) PRIMARY KEY,
		  user_id BIGINT NOT NULL,
		  feature VARCHAR(32) NOT NULL,
		  status VARCHAR(16) DEFAULT 'pending',
		  image_ids TEXT NOT NULL,
		  result_urls TEXT,
		  error_message TEXT,
		  progress INT DEFAULT 0,
		  params TEXT,
		  total_images INT DEFAULT 0,
		  processed_images INT DEFAULT 0,
		  total_videos INT DEFAULT 0,
		  processed_videos INT DEFAULT 0,
		  submit_ids TEXT,
		  source_filenames TEXT,
		  source_paths TEXT,
		  folder_name VARCHAR(255) DEFAULT '',
		  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		  completed_at DATETIME,
		  INDEX idx_user (user_id),
		  INDEX idx_status (status)
		);
	`)
	if err != nil {
		fmt.Println(err)
	}
}

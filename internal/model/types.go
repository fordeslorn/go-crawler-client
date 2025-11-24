package model

// StartTaskRequest 启动任务请求
type StartTaskRequest struct {
	PixivUserID string `json:"pixiv_user_id" binding:"required"`
	Cookie      string `json:"cookie" binding:"required"`
}

// UserInfo 用户信息
type UserInfo struct {
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	AvatarURL  string `json:"avatar_url"`
	AvatarPath string `json:"avatar_path"`
	Premium    bool   `json:"premium"`
}

// StartTaskResponse 启动任务响应
type StartTaskResponse struct {
	Status   string   `json:"status"`
	TaskID   string   `json:"task_id"`
	UserInfo UserInfo `json:"user_info"`
}

// ImageInfo 图片信息
type ImageInfo struct {
	URL      string `json:"url"`
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
	Status   string `json:"status"`
}

// TaskResult 爬取结果
type TaskResult struct {
	UserID    string   `json:"user_id"`
	UserName  string   `json:"user_name"`
	ImageURLs []string `json:"image_urls"`
}

// TaskStatusResponse 任务状态响应
type TaskStatusResponse struct {
	Status   string       `json:"status"` // running, completed, failed
	Mode     string       `json:"mode"`   // image, data
	UserInfo UserInfo     `json:"user_info"`
	Logs     []string     `json:"logs"`
	Results  []TaskResult `json:"results,omitempty"`
	Images   []ImageInfo  `json:"images,omitempty"`
}

// LogResponse 日志响应
type LogResponse struct {
	TaskID     string   `json:"task_id"`
	Logs       []string `json:"logs"`
	TotalLines int      `json:"total_lines"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status     string `json:"status"`
	BaseDir    string `json:"base_dir"`
	TasksCount int    `json:"tasks_count"`
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	User   UserConfig   `json:"user"`
	Client ClientConfig `json:"client"`
}

type UserConfig struct {
	ServerURL string `json:"server_url"`
	ProxyHost string `json:"proxy_host"`
	ProxyPort int    `json:"proxy_port"`
}

type ClientConfig struct {
	Port      int    `json:"port"`
	BaseDir   string `json:"base_dir"`
	StartedAt string `json:"started_at"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Port    int    `json:"port"`
	BaseDir string `json:"base_dir"`
}

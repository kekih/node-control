package agent

import "time"

// HealthResponse represents agent system info
type HealthResponse struct {
	Status    string    `json:"status"`
	Hostname  string    `json:"hostname"`
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	Uptime    string    `json:"uptime"`
	StartTime time.Time `json:"start_time"`
	GoVersion string    `json:"go_version"`
	Version   string    `json:"version"`
}

// FileItem represents a single file or directory
type FileItem struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Ext     string    `json:"ext"`
}

// FilesListResponse represents directory contents
type FilesListResponse struct {
	CurrentPath string     `json:"current_path"`
	ParentPath  string     `json:"parent_path"`
	Items       []FileItem `json:"items"`
}

// ShareLink represents a shared file link
type ShareLink struct {
	Token     string    `json:"token"`
	FilePath  string    `json:"file_path"`
	FileName  string    `json:"file_name"`
	CreatedAt time.Time `json:"created_at"`
	Downloads int       `json:"downloads"`
}

// DBRequest represents database connection and command requests
type DBRequest struct {
	Driver   string `json:"driver"`   // "postgres" or "mysql"
	Host     string `json:"host"`     // default "localhost"
	Port     int    `json:"port"`     // default 5432 or 3306
	User     string `json:"user"`     // username
	Password string `json:"password"` // password
	DBName   string `json:"db_name"`  // target db name
	SSLMode  string `json:"ssl_mode"` // disable, require, etc.
	Query    string `json:"query"`    // SQL query
	
	// KV / Table helper fields matching reference UI
	Path     string `json:"path"`     // table or key/path
	Value    string `json:"value"`    // data/payload
	Merge    bool   `json:"merge"`
	Force    bool   `json:"force"`
}

// ScriptRunRequest represents a script launch request
type ScriptRunRequest struct {
	ID       string `json:"id"`       // script identifier (e.g. "main")
	Language string `json:"language"` // "python", "nodejs", "golang"
	Content  string `json:"content"`  // raw script code
	AutoDeps bool   `json:"auto_deps"`// auto-install missing packages
}

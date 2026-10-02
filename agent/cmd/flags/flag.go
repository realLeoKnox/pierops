package flags_pkg

type Config struct {
	EnableFileRead  bool   `json:"enable_file_read" env:"AGENT_ENABLE_FILE_READ"`
	EnableFileWrite bool   `json:"enable_file_write" env:"AGENT_ENABLE_FILE_WRITE"`
	EnableTerminal  bool   `json:"enable_terminal" env:"AGENT_ENABLE_TERMINAL"`
	EnableExec      bool   `json:"enable_exec" env:"AGENT_ENABLE_EXEC"`
	FileRoots       string `json:"file_roots" env:"AGENT_FILE_ROOTS"`
	ExecutionUser   string `json:"execution_user" env:"AGENT_EXECUTION_USER"`
	NodeUUID        string `json:"node_uuid" env:"AGENT_NODE_UUID"`

	AutoDiscoveryKey    string  `json:"auto_discovery_key" env:"AGENT_AUTO_DISCOVERY_KEY"`         // 自动发现密钥
	DisableAutoUpdate   bool    `json:"disable_auto_update" env:"AGENT_DISABLE_AUTO_UPDATE"`       // 禁用自动更新
	DisableWebSsh       bool    `json:"disable_web_ssh" env:"AGENT_DISABLE_WEB_SSH"`               // 禁用远程控制（web ssh 和 rce）
	MemoryModeAvailable bool    `json:"memory_mode_available" env:"AGENT_MEMORY_MODE_AVAILABLE"`   // [deprecated] 已弃用，请使用 MemoryIncludeCache
	Token               string  `json:"token" env:"AGENT_TOKEN"`                                   // Token
	Endpoint            string  `json:"endpoint" env:"AGENT_ENDPOINT"`                             // 面板地址
	Interval            float64 `json:"interval" env:"AGENT_INTERVAL"`                             // 数据采集间隔，单位秒
	IgnoreUnsafeCert    bool    `json:"ignore_unsafe_cert" env:"AGENT_IGNORE_UNSAFE_CERT"`         // 忽略不安全的证书
	MaxRetries          int     `json:"max_retries" env:"AGENT_MAX_RETRIES"`                       // 最大重试次数
	ReconnectInterval   int     `json:"reconnect_interval" env:"AGENT_RECONNECT_INTERVAL"`         // 重连间隔，单位秒
	InfoReportInterval  int     `json:"info_report_interval" env:"AGENT_INFO_REPORT_INTERVAL"`     // 基础信息上报间隔，单位分钟
	IncludeNics         string  `json:"include_nics" env:"AGENT_INCLUDE_NICS"`                     // 仅统计网卡，逗号分隔的网卡名称列表，支持通配符
	ExcludeNics         string  `json:"exclude_nics" env:"AGENT_EXCLUDE_NICS"`                     // 统计时排除的网卡，逗号分隔的网卡名称列表，支持通配符
	IncludeMountpoints  string  `json:"include_mountpoints" env:"AGENT_INCLUDE_MOUNTPOINTS"`       // 磁盘统计的包含挂载点列表，使用分号分隔
	MonthRotate         int     `json:"month_rotate" env:"AGENT_MONTH_ROTATE"`                     // 流量统计的月份重置日期（0表示禁用）
	MemoryIncludeCache  bool    `json:"memory_include_cache" env:"AGENT_MEMORY_INCLUDE_CACHE"`     // 包括缓存/缓冲区的内存使用情况
	MemoryReportRawUsed bool    `json:"memory_report_raw_used" env:"AGENT_MEMORY_REPORT_RAW_USED"` // 使用原始内存使用情况报告
	CustomDNS           string  `json:"custom_dns" env:"AGENT_CUSTOM_DNS"`                         // 使用的自定义DNS服务器
	EnableGPU           bool    `json:"enable_gpu" env:"AGENT_ENABLE_GPU"`                         // 启用详细GPU监控
	ShowWarning         bool    `json:"show_warning" env:"AGENT_SHOW_WARNING"`                     // Windows 上显示安全警告，作为子进程运行一次
	CustomIpv4          string  `json:"custom_ipv4" env:"AGENT_CUSTOM_IPV4"`                       // 自定义 IPv4 地址
	CustomIpv6          string  `json:"custom_ipv6" env:"AGENT_CUSTOM_IPV6"`                       // 自定义 IPv6 地址
	GetIpAddrFromNic    bool    `json:"get_ip_addr_from_nic" env:"AGENT_GET_IP_ADDR_FROM_NIC"`     // 从网卡获取IP地址
	HostProc            string  `json:"host_proc" env:"HOST_PROC"`                                 // 容器环境下宿主机/proc目录的挂载点，用于监控宿主机进程
	ConfigFile          string  `json:"config_file" env:"AGENT_CONFIG_FILE"`                       // JSON配置文件路径
	DisableCompression  bool    `json:"disable_compression" env:"AGENT_DISABLE_COMPRESSION"`       // 禁用v2传输压缩
	PreferIPVersion     string  `json:"prefer_ip_version" env:"AGENT_PREFER_IP_VERSION"`           // 面板连接优先使用的 IP 版本：4 或 6

}

var GlobalConfig = &Config{}

// FileAction classifies every supported operation; unknown operations fail closed.
func FileAction(op string) string {
	switch op {
	case "list", "list_roots", "stat", "search", "download_stream":
		return "file.read"
	case "create", "mkdir", "delete", "move", "copy", "chmod", "upload_stream", "upload_commit", "upload_cancel":
		return "file.write"
	default:
		return ""
	}
}
func (c *Config) Allows(action string) bool {
	if c.DisableWebSsh {
		return false
	}
	switch action {
	case "file.read":
		return c.EnableFileRead
	case "file.write":
		return c.EnableFileWrite
	case "terminal.open":
		return c.EnableTerminal
	case "command.exec":
		return c.EnableExec
	}
	return false
}
func (c *Config) Capabilities() []string {
	out := []string{"ping", "message", "event", "startup_config", "pierops_m2"}
	if c.Allows("command.exec") {
		out = append(out, "exec")
	}
	if c.Allows("terminal.open") {
		out = append(out, "terminal")
	}
	if c.Allows("file.read") || c.Allows("file.write") {
		out = append(out, "file")
	}
	if !c.DisableWebSsh && !c.DisableAutoUpdate {
		out = append(out, "switch_version")
	}
	return out
}

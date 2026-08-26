package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

type config struct {
	Addr     string
	Mode     string
	Timeout  time.Duration
	DataPath string
}

func defaultAddr() string {
	if value := os.Getenv("PORT"); value != "" {
		if port, err := strconv.Atoi(value); err == nil && port >= 1 && port <= 65535 {
			return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		}
	}
	return "127.0.0.1:19091"
}
func parseConfig(args []string) (config, error) {
	cfg := config{}
	fs := flag.NewFlagSet("riggingdesk", flag.ContinueOnError)
	fs.StringVar(&cfg.Addr, "addr", defaultAddr(), "回环监听地址")
	fs.StringVar(&cfg.Mode, "mode", "serve", "运行模式：serve 或 selfcheck")
	fs.DurationVar(&cfg.Timeout, "timeout", 20*time.Second, "自检或关闭超时")
	fs.StringVar(&cfg.DataPath, "data", "riggingdesk.db", "SQLite 数据文件")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		return cfg, fmt.Errorf("存在未识别参数")
	}
	if cfg.Mode != "serve" && cfg.Mode != "selfcheck" {
		return cfg, fmt.Errorf("mode 必须为 serve 或 selfcheck")
	}
	if cfg.Timeout <= 0 || cfg.Timeout > 5*time.Minute {
		return cfg, fmt.Errorf("timeout 必须在 0 到 5 分钟之间")
	}
	host, portText, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return cfg, fmt.Errorf("addr 格式无效: %w", err)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return cfg, fmt.Errorf("addr 必须使用回环地址")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 0 || port > 65535 {
		return cfg, fmt.Errorf("addr 端口无效")
	}
	if cfg.Mode == "serve" && port == 0 {
		return cfg, fmt.Errorf("serve 模式不得使用 0 端口")
	}
	if cfg.DataPath == "" {
		return cfg, fmt.Errorf("data 不能为空")
	}
	return cfg, nil
}

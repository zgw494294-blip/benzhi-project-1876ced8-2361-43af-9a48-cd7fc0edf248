package main

import "testing"

func TestParseConfigRejectsNonLoopback(t *testing.T) {
	if _, err := parseConfig([]string{"-addr=0.0.0.0:19091"}); err == nil {
		t.Fatal("应拒绝通配监听地址")
	}
	cfg, err := parseConfig([]string{"-mode=selfcheck", "-addr=127.0.0.1:0", "-timeout=2s"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "selfcheck" {
		t.Fatal("模式解析错误")
	}
}

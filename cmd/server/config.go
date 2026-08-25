package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultListenAddress = "127.0.0.1:19081"

func configuredDefaultAddress() (string, error) {
	portText := strings.TrimSpace(os.Getenv("PORT"))
	if portText == "" {
		return defaultListenAddress, nil
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return "", fmt.Errorf("PORT 必须是 1024 到 65535 之间的端口号")
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("-addr 必须为 host:port：%w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("监听端口无效")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("为保护本地作业数据，监听地址必须是回环地址")
	}
	return nil
}

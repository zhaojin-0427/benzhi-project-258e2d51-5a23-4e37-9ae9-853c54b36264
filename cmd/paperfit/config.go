package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

func addressDefault() string {
	port := os.Getenv("PORT")
	if port == "" {
		return defaultAddress
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return defaultAddress
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(n))
}

func validateAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("监听地址必须为 host:port: %w", err)
	}
	if host == "" || port == "" {
		return fmt.Errorf("监听地址的 host 和 port 不能为空")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("监听端口无效")
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("监听地址必须是回环地址")
	}
	return nil
}

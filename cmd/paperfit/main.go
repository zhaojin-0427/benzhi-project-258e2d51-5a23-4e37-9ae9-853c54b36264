package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"paperfit-release/internal/application"
	"paperfit-release/internal/eventstore"
	"paperfit-release/internal/httpapi"
)

func main() {
	if err := run(); err != nil {
		slog.Error("paperfit 退出", "error", err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet("paperfit", flag.ContinueOnError)
	address := flags.String("addr", addressDefault(), "HTTP 回环监听地址")
	dataDirectory := flags.String("data", "./data", "事件账本和投影目录")
	selfcheck := flags.Bool("selfcheck", false, "运行有界 HTTP 主流程自检")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if err := validateAddress(*address); err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if *selfcheck {
		logger.Info("开始 selfcheck", "addr", *address)
		if err := httpapi.RunSelfCheck(*address, logger); err != nil {
			return err
		}
		fmt.Println("paperfit selfcheck: ok")
		return nil
	}
	store, err := eventstore.Open(*dataDirectory)
	if err != nil {
		return fmt.Errorf("初始化事件存储: %w", err)
	}
	service := application.NewService(store)
	api := httpapi.New(service, logger)
	server := httpapi.NewHTTPServer(*address, api)
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", *address, err)
	}
	logger.Info("paperfit 已就绪", "addr", listener.Addr().String(), "event_sequence", store.LastSequence())
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case signalValue := <-signals:
		logger.Info("收到停止信号", "signal", signalValue.String())
	case serveErr := <-serveErrors:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("优雅关闭: %w", err)
	}
	select {
	case serveErr := <-serveErrors:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
	case <-time.After(time.Second):
		return fmt.Errorf("HTTP 服务未按时退出")
	}
	return nil
}

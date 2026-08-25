package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
	webadapter "benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/web"
)

func main() {
	if err := run(); err != nil {
		log.Printf("服务退出：%v", err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	address := flags.String("addr", "", "监听地址，仅允许回环地址；默认 127.0.0.1:19081")
	databasePath := flags.String("db", "restoration.db", "SQLite 数据库路径")
	selfcheck := flags.Bool("selfcheck", false, "启动实际 HTTP 服务并完成有界全流程自检")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	resolvedAddress := *address
	if resolvedAddress == "" {
		var err error
		resolvedAddress, err = configuredDefaultAddress()
		if err != nil {
			return err
		}
	}
	if err := validateAddress(resolvedAddress); err != nil {
		return err
	}
	if *selfcheck {
		return runSelfcheck(resolvedAddress)
	}
	repository, err := store.Open(*databasePath)
	if err != nil {
		return err
	}
	defer repository.Close()
	service := application.New(repository)
	handler := webadapter.New(service).Handler()
	server := newHTTPServer(resolvedAddress, handler)
	listener, err := net.Listen("tcp", resolvedAddress)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", resolvedAddress, err)
	}
	log.Printf("古树复壮作业放行台已启动：http://%s/workspace", listener.Addr())
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-signalContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}

func newHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 12 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}

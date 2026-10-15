package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"golang.org/x/term"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"yintonglian/internal/app"
)

func main() {
	command := "serve"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	opts, err := parseOptions(command, args)
	if err != nil {
		log.Fatal(err)
	}
	// Initialization and backup operate locally and do not need a deployed origin.
	cfg := app.Config{Root: opts.Root, DataDir: opts.Data, Origin: opts.Origin, Production: opts.Production, MaxUpload: opts.MaxUploadMB * 1024 * 1024}
	if command != "serve" {
		cfg.Production = false
		cfg.Origin = ""
	}
	server, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()
	switch command {
	case "init-admin":
		fmt.Printf("管理员账号：%s\n输入至少12位密码（不显示内容）：", opts.Username)
		var password []byte
		if term.IsTerminal(int(os.Stdin.Fd())) {
			password, err = term.ReadPassword(int(os.Stdin.Fd()))
		} else {
			var line string
			line, err = bufio.NewReader(os.Stdin).ReadString('\n')
			password = []byte(strings.TrimRight(line, "\r\n"))
		}
		fmt.Println()
		if err != nil {
			log.Fatal(err)
		}
		if err = server.CreateAdmin(opts.Username, string(password)); err != nil {
			log.Fatal(err)
		}
		fmt.Println("管理员已创建。")
		return
	case "backup":
		if opts.Output == "" {
			opts.Output = filepath.Join("backups", time.Now().UTC().Format("20060102-150405"))
		}
		if err = server.Backup(opts.Output); err != nil {
			log.Fatal(err)
		}
		fmt.Println("完整备份已保存：" + opts.Output)
		return
	case "serve":
	default:
		log.Fatal("commands: serve, init-admin, backup")
	}
	if info, e := os.Stat(filepath.Join(opts.Root, "public", "index.html")); e != nil || !info.Mode().IsRegular() {
		log.Fatal("public/index.html not found; set --root to the project directory")
	}
	listener, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		log.Fatal(err)
	}
	httpServer := &http.Server{Handler: server, ReadHeaderTimeout: 15 * time.Second, ReadTimeout: 10 * time.Minute, WriteTimeout: 10 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopped)
	go func() {
		<-stopped
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	log.Printf("Go服务已启动：http://%s · 后台 /admin", listener.Addr())
	if err = httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

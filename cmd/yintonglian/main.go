package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/term"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"yintonglian/internal/app"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func main() {
	command := "serve"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet(command, flag.ExitOnError)
	root := fs.String("root", env("APP_ROOT", "."), "directory containing public/")
	data := fs.String("data", env("DATA_DIR", ""), "persistent data directory")
	addr := fs.String("addr", net.JoinHostPort(env("HOST", "127.0.0.1"), env("PORT", "4317")), "listen address")
	origin := fs.String("origin", env("APP_ORIGIN", ""), "public origin")
	production := fs.Bool("production", env("APP_ENV", env("NODE_ENV", "development")) == "production", "HTTPS production mode")
	maxUpload := fs.Int64("max-upload-mb", 300, "upload size limit in MB")
	dest := fs.String("output", "", "backup destination (new directory)")
	username := fs.String("username", "admin", "admin account for init-admin")
	fs.Parse(args)
	if *data == "" {
		*data = filepath.Join(*root, ".data")
	}
	if value := os.Getenv("MAX_UPLOAD_MB"); value != "" {
		n, e := strconv.ParseInt(value, 10, 64)
		if e != nil || n < 1 {
			log.Fatal("invalid MAX_UPLOAD_MB")
		}
		*maxUpload = n
	}
	if *maxUpload < 1 || *maxUpload > 4096 {
		log.Fatal("upload limit must be 1..4096 MB")
	}
	// Initialization and backup operate locally and do not need a deployed origin.
	cfg := app.Config{Root: *root, DataDir: *data, Origin: *origin, Production: *production, MaxUpload: *maxUpload * 1024 * 1024}
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
		fmt.Printf("管理员账号：%s\n输入至少12位密码（不显示内容）：", *username)
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
		if err = server.CreateAdmin(*username, string(password)); err != nil {
			log.Fatal(err)
		}
		fmt.Println("管理员已创建。")
		return
	case "backup":
		if *dest == "" {
			*dest = filepath.Join("backups", time.Now().UTC().Format("20060102-150405"))
		}
		if err = server.Backup(*dest); err != nil {
			log.Fatal(err)
		}
		fmt.Println("完整备份已保存：" + *dest)
		return
	case "serve":
	default:
		log.Fatal("commands: serve, init-admin, backup")
	}
	if info, e := os.Stat(filepath.Join(*root, "public", "index.html")); e != nil || !info.Mode().IsRegular() {
		log.Fatal("public/index.html not found; set --root to the project directory")
	}
	listener, err := net.Listen("tcp", *addr)
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

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Relative paths in the file are resolved against the configuration directory.
type fileConfig struct {
	Root        string `json:"root"`
	Data        string `json:"data_dir"`
	Addr        string `json:"listen"`
	Origin      string `json:"origin"`
	Production  bool   `json:"production"`
	MaxUploadMB int64  `json:"max_upload_mb"`
}

type options struct {
	fileConfig
	Output   string
	Username string
}

func readConfig(path string, required bool) (fileConfig, error) {
	cfg := fileConfig{Root: ".", Addr: "127.0.0.1:4317", MaxUploadMB: 300}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) && !required {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode config %s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return cfg, fmt.Errorf("config %s must contain exactly one JSON object", path)
	}
	// Decode into a struct accepts null; require an object explicitly.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return cfg, err
	}
	var object map[string]json.RawMessage
	if err := json.NewDecoder(f).Decode(&object); err != nil || object == nil {
		return cfg, fmt.Errorf("config %s must be a JSON object", path)
	}
	if cfg.Root == "" {
		return cfg, fmt.Errorf("root cannot be empty")
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return cfg, err
	}
	if !filepath.IsAbs(cfg.Root) {
		cfg.Root = filepath.Join(base, cfg.Root)
	}
	if cfg.Data != "" && !filepath.IsAbs(cfg.Data) {
		cfg.Data = filepath.Join(base, cfg.Data)
	}
	return cfg, nil
}

func parseOptions(command string, args []string) (options, error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	configPath := fs.String("config", "config.json", "JSON configuration file")
	root := fs.String("root", ".", "directory containing public/")
	data := fs.String("data", "", "persistent data directory")
	addr := fs.String("addr", "127.0.0.1:4317", "listen address")
	origin := fs.String("origin", "", "public origin")
	production := fs.Bool("production", false, "HTTPS production mode")
	maxUpload := fs.Int64("max-upload-mb", 300, "upload size limit in MB")
	output := fs.String("output", "", "backup destination (new directory)")
	username := fs.String("username", "admin", "admin account for init-admin")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	explicitConfig := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicitConfig = true
		}
	})
	cfg, err := readConfig(*configPath, explicitConfig)
	if err != nil {
		return options{}, err
	}
	// Explicit flags override the file, including false and empty values.
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "root":
			cfg.Root = *root
		case "data":
			cfg.Data = *data
		case "addr":
			cfg.Addr = *addr
		case "origin":
			cfg.Origin = *origin
		case "production":
			cfg.Production = *production
		case "max-upload-mb":
			cfg.MaxUploadMB = *maxUpload
		}
	})
	if cfg.Root == "" {
		return options{}, fmt.Errorf("root cannot be empty")
	}
	if cfg.Data == "" {
		cfg.Data = filepath.Join(cfg.Root, ".data")
	}
	if cfg.MaxUploadMB < 1 || cfg.MaxUploadMB > 4096 {
		return options{}, fmt.Errorf("max_upload_mb must be 1..4096")
	}
	if cfg.Addr == "" {
		return options{}, fmt.Errorf("listen cannot be empty")
	}
	return options{fileConfig: cfg, Output: *output, Username: *username}, nil
}

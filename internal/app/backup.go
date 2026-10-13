package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (a *Server) Backup(destination string) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	dest, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	source := a.Store.DataDir
	if dest == source || strings.HasPrefix(dest, source+string(os.PathSeparator)) {
		return fmt.Errorf("备份目录不能位于数据目录内")
	}
	if _, err = os.Lstat(dest); err == nil {
		return fmt.Errorf("备份目录已存在，请选择新的目录")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	if err = os.Mkdir(dest, 0700); err != nil {
		return err
	}
	dbFile := filepath.Join(dest, "content.sqlite")
	if err = a.Store.DB.Exec("VACUUM INTO ?", dbFile).Error; err != nil {
		return err
	}
	if err = os.Chmod(dbFile, 0600); err != nil {
		return err
	}
	files, err := a.Store.Uploads()
	if err != nil {
		return err
	}
	uploadDest := filepath.Join(dest, "uploads")
	if err = os.Mkdir(uploadDest, 0700); err != nil {
		return err
	}
	for _, f := range files {
		if !uploadPattern.MatchString(f.Filename) {
			return fmt.Errorf("invalid stored file name")
		}
		if err = copyFile(filepath.Join(source, "uploads", f.Filename), filepath.Join(uploadDest, f.Filename)); err != nil {
			return err
		}
	}
	return nil
}
func copyFile(source, destination string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dest, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = io.Copy(dest, src); err != nil {
		dest.Close()
		return err
	}
	return dest.Close()
}

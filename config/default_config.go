package config

import (
	"os"
	"path/filepath"
)

var (
	TempDir           = os.TempDir()
	BackupDir         = filepath.Join(TempDir, "airpress-backup") + string(os.PathSeparator)
	BackupMarkdownDir = filepath.Join(TempDir, "airpress-backup-markdown") + string(os.PathSeparator)
	DataExportDir     = filepath.Join(TempDir, "airpress-data-export") + string(os.PathSeparator)
	ResourcesDir, _   = filepath.Abs("./resources")
)

package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/util"
)

func NewConfig() *Config {
	var configFile string
	flag.StringVar(&configFile, "config", "", "")
	flag.Parse()

	// 未指定 -config 时，默认加载 ./conf/config.yaml
	if configFile == "" {
		configFile = filepath.Join(".", "conf", "config.yaml")
	}

	k := koanf.New(".")
	if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		panic(err)
	}

	conf := &Config{}
	// koanf 默认反序列化 tag 为 "koanf"，这里沿用 config/model.go 中已有的 mapstructure tag，
	// 避免改动模型结构体
	if err := k.UnmarshalWithConf("", conf, koanf.UnmarshalConf{Tag: "mapstructure"}); err != nil {
		panic(err)
	}

	// admin_url_path 默认值兜底（对应原 viper.SetDefault 语义）
	if conf.AirPress.AdminURLPath == "" {
		conf.AirPress.AdminURLPath = "admin"
	}

	if conf.AirPress.WorkDir == "" {
		pwd, err := os.Getwd()
		if err != nil {
			panic(errors.Wrap(err, "init config: get current dir"))
		}
		conf.AirPress.WorkDir, _ = filepath.Abs(pwd)
	} else {
		workDir, err := filepath.Abs(conf.AirPress.WorkDir)
		if err != nil {
			panic(err)
		}
		conf.AirPress.WorkDir = workDir
	}
	normalizeDir := func(path *string, subDir string) {
		if *path == "" {
			*path = filepath.Join(conf.AirPress.WorkDir, subDir)
		} else {
			temp, err := filepath.Abs(*path)
			if err != nil {
				panic(err)
			}
			*path = temp
		}
	}
	normalizeDir(&conf.AirPress.LogDir, "log")
	normalizeDir(&conf.AirPress.TemplateDir, "resources/template")
	normalizeDir(&conf.AirPress.AdminResourcesDir, "resources/admin")
	normalizeDir(&conf.AirPress.UploadDir, consts.AirPressUploadDir)
	normalizeDir(&conf.AirPress.ThemeDir, "resources/template/theme")
	if conf.SQLite3 != nil && conf.SQLite3.Enable {
		normalizeDir(&conf.SQLite3.File, "airpress.db")
	}
	if !util.FileIsExisted(conf.AirPress.TemplateDir) {
		panic("template dir: " + conf.AirPress.TemplateDir + " not exist")
	}
	if !util.FileIsExisted(conf.AirPress.AdminResourcesDir) {
		panic("AdminResourcesDir: " + conf.AirPress.AdminResourcesDir + "not exist")
	}
	if !util.FileIsExisted(conf.AirPress.ThemeDir) {
		panic("theme dir: " + conf.AirPress.ThemeDir + " not exist")
	}

	initDirectory(conf)
	mode = conf.AirPress.Mode
	logMode = conf.AirPress.LogMode
	return conf
}

func initDirectory(conf *Config) {
	mkdirFunc := func(dir string, err error) error {
		if err == nil {
			if _, err = os.Stat(dir); os.IsNotExist(err) {
				err = os.MkdirAll(dir, os.ModePerm)
			}
		}
		return err
	}
	err := mkdirFunc(conf.AirPress.LogDir, nil)
	err = mkdirFunc(conf.AirPress.UploadDir, err)
	if err != nil {
		panic(fmt.Errorf("initDirectory err=%w", err))
	}
}

var (
	mode    string
	logMode LogMode
)

func IsDev() bool {
	return mode == "development"
}

func LogToConsole() bool {
	switch logMode {
	case Console:
		return true
	case File:
		return false
	default:
		return IsDev()
	}
}

package main

import (
	"github.com/prometheus/common/log"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Process struct {
}

type Session struct {
}

type WaitTime struct {
}

type Tablespace struct {
}

type PhysicalIO struct {
}

type Cache struct {
}

type Activity struct {
}

type TopSql struct {
	Rownum int `yaml:"rownum"`
}

type TopTable struct {
	Rownum int `yaml:"rownum"`
}

type Config struct {
	Connection string   `yaml:"connection"`
	Database   string   `yaml:"database"`
	Instance   string   `yaml:"instance"`
	Id         string   `yaml:"id"`
	Metrics    string   `yaml:"metrics"`
	TopSql     TopSql   `yaml:"topsql"`
	TopTable   TopTable `yaml:"toptable"`
}

type Configs struct {
	Cfgs []Config `yaml:"connections"`
}

var (
	config Configs
	pwd    string
)

func loadConfig() bool {
	path, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	pwd = path
	content, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("error: %v", err)
		return false
	} else {
		err := yaml.Unmarshal(content, &config)
		if err != nil {
			log.Fatalf("error: %v", err)
			return false
		}

		return true
	}
}

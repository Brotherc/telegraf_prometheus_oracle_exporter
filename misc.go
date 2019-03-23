package main

import (
	"github.com/prometheus/common/log"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Config struct {
	Connection string `yaml:"connection"`
	Database   string `yaml:"database"`
	Instance   string `yaml:"instance"`
	Id         string `yaml:"id"`
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

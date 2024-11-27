package main

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"sync"
)

type Config struct {
	BaseUrl     string `yaml:"base_url" env-default:"https://test.site/"`
	InputPath   string `yaml:"input_path" env-default:""`
	OutputPath  string `yaml:"output_path" env-default:""`
	BearerToken string `yaml:"bearer_token" env-default:""`
}

var instance *Config
var once sync.Once

func GetConfig(path string) (*Config, error) {
	var err error
	once.Do(func() {
		instance = &Config{}
		if err = cleanenv.ReadConfig(path, instance); err != nil {
			desc, _ := cleanenv.GetDescription(instance, nil)
			err = fmt.Errorf("%s; %s", err, desc)
			instance = nil
		}
	})
	return instance, err
}

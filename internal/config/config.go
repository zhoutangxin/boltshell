package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	AppName  string `json:"appName"`
	LogLevel string `json:"logLevel"`
	Port     int    `json:"port"`
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBPath   string `json:"dbPath"`
}

func Default() Config {
	return Config{
		AppName:  "ssh-go",
		LogLevel: "INFO",
		Port:     0,
		Host:     "",
		User:     "",
		Password: "",
		DBPath:   "",
	}
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	if c.AppName == "" {
		c.AppName = "ssh-go"
	}
	if c.LogLevel == "" {
		c.LogLevel = "INFO"
	}
	return c, nil
}

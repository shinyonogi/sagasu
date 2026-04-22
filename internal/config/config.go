package config

import (
	"encoding/json"
	"errors"
	"os"
)

type Config struct {
	Include    []string `json:"include,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
	IgnoreDirs []string `json:"ignore_dirs,omitempty"`
}

func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

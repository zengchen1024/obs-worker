package config

import "github.com/zengchen1024/obs-worker/build"

func Load(path string) (*Config, error) {
	v := new(Config)
	if err := LoadFromYaml(path, v); err != nil {
		return nil, err
	}

	v.setDefault()

	if err := v.validate(); err != nil {
		return nil, err
	}

	return v, nil
}

type Config struct {
	Build build.Config `json:"build"`
}

func (c *Config) setDefault() error {
	return c.Build.SetDefault()
}

func (c *Config) validate() error {
	return c.Build.Validate()
}

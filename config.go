package main

type Config struct {
	Listen string            `yaml:"listen"`
	Year   map[string]string `yaml:"year"`
	Cache  string            `yaml:"cache"`
}

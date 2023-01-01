package main

type Config struct {
	Listen       string            `yaml:"listen"`
	Cache        string            `yaml:"cache"`
	ClientId     string            `yaml:"clientId"`
	ClientSecret string            `yaml:"clientSecret"`
	RedirectUrl  string            `yaml:"redirectUrl"`
	Year         map[string]string `yaml:"year"`
}

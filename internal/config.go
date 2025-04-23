package internal

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type App struct {
	Path string   `yaml:"path"`
	Args []string `yaml:"args"`
}

type Config struct {
	Activate string         `yaml:"activate"`
	Apps     map[string]App `yaml:"apps"`
	AppOrder []string       `yaml:"-"`
}


var (
	cachedConfig *Config
	configMtime  int64
	configLock   sync.RWMutex
)

func GetConfig() *Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return cachedConfig
}

func LoadConfig(configPath string) (*Config, error) {
	paths := []string{configPath}
	if wd, err := os.Getwd(); err == nil {
		parent := wd
		if idx := lastIndexAny(wd, `/\\`); idx != -1 {
			parent = wd[:idx]
		}
		if parent != wd && parent != "" {
			paths = append(paths, parent+string(os.PathSeparator)+configPath)
		}
	}
	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("配置文件格式错误（%v）: %v", paths, err)
	}
	if cfg.Apps == nil {
		cfg.Apps = make(map[string]App)
	}
	cfg.AppOrder = parseAppOrderNode(data)
	return &cfg, nil
}

func ReloadConfig(configPath string) error {
	configLock.Lock()
	defer configLock.Unlock()
	fi, err := os.Stat(configPath)
	if err != nil {
		return err
	}
	if cachedConfig != nil && fi.ModTime().Unix() == configMtime {
		return nil // 未变更
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	cachedConfig = cfg
	configMtime = fi.ModTime().Unix()
	return nil
}

func SaveConfig(cfg *Config, configPath string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func lastIndexAny(s, chars string) int {
	for i := len(s) - 1; i >= 0; i-- {
		for _, c := range chars {
			if s[i] == byte(c) {
				return i
			}
		}
	}
	return -1
}

func parseAppOrderNode(yamlData []byte) []string {
	var root yaml.Node
	yaml.Unmarshal(yamlData, &root)
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil
	}
	m := root.Content[0]
	for i := 0; i < len(m.Content)-1; i += 2 {
		key := m.Content[i]
		if key.Value == "apps" {
			appsNode := m.Content[i+1]
			if appsNode.Kind == yaml.MappingNode {
				order := make([]string, 0, len(appsNode.Content)/2)
				for j := 0; j < len(appsNode.Content)-1; j += 2 {
					name := appsNode.Content[j].Value
					order = append(order, name)
				}
				return order
			}
		}
	}
	return nil
}

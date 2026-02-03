package config

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

var (
	config      YamlConfig
	configMutex sync.RWMutex
	configPath  string
)

var yamlFilePaths = []string{
	"/config/conf.yaml",
	"config/conf.yaml",
	"../config/conf.yaml",
	"/../config/conf.yaml",
	"../../config/conf.yaml",
	"/../../config/conf.yaml",
}

type YamlConfig struct {
	Email struct {
		Host       string `yaml:"HOST"`
		Port       int    `yaml:"PORT"`
		Username   string `yaml:"USERNAME"`
		Password   string `yaml:"PASSWORD"`
		MaxRetry   int    `yaml:"MAX_RETRY"`
		RetryDelay int    `yaml:"RETRY_DELAY"`
	} `yaml:"EMAIL"`

	Default struct {
		TaFeedbackURL string `yaml:"TA_FEEDBACK_URL"`
	} `yaml:"DEFAULT"`

	Odoo struct {
		JSONRPC string `yaml:"JSONRPC"`
		// Manage Service
		Login         string `yaml:"LOGIN"`
		Password      string `yaml:"PASSWORD"`
		Db            string `yaml:"DB"`
		UrlSession    string `yaml:"URL_SESSION"`
		UrlGetData    string `yaml:"URL_GETDATA"`
		LoginATM      string `yaml:"LOGIN_ATM"`
		PasswordATM   string `yaml:"PASSWORD_ATM"`
		DbATM         string `yaml:"DB_ATM"`
		UrlSessionATM string `yaml:"URL_SESSION_ATM"`
		UrlGetDataATM string `yaml:"URL_GETDATA_ATM"`
		// ATM
		MaxRetry       string `yaml:"MAX_RETRY"`
		RetryDelay     string `yaml:"RETRY_DELAY"`
		SessionTimeout string `yaml:"SESSION_TIMEOUT"`
		GetDataTimeout string `yaml:"GETDATA_TIMEOUT"`
		CompanyAllowed []int  `yaml:"COMPANY_ALLOWED"`
	} `yaml:"ODOO"`

	Report struct {
		To  []string `yaml:"TO"`
		Cc  []string `yaml:"CC"`
		Bcc []string `yaml:"BCC"`
	} `yaml:"REPORT"`
}

func YAMLLoad(filePath string) (*YamlConfig, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config YamlConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func InitConfig() (*YamlConfig, error) {
	yamlFilePaths := []string{
		"config/conf.yaml",
		"../config/conf.yaml",
		"../../config/conf.yaml",
	}

	var yamlConfig *YamlConfig

	for _, filePath := range yamlFilePaths {
		if _, err := os.Stat(filePath); err == nil {
			yamlConfig, err = YAMLLoad(filePath)
			if err != nil {
				log.Printf("failed to load configuration from '%s': %v", filePath, err)
				continue
			}
			// log.Printf("Configuration successfully loaded from '%s'", filePath)
			break
		} else if os.IsNotExist(err) {
			log.Printf("configuration file '%s' does not exist. Skipping.", filePath)
		} else {
			log.Printf("error checking file '%s': %v", filePath, err)
		}
	}

	if yamlConfig == nil {
		log.Fatalf("failed to load YAML configuration: no valid configuration file found in paths: %v", yamlFilePaths)
	}

	return yamlConfig, nil
}

func LoadConfig() error {
	// dir, err := os.Getwd()
	// if err != nil {
	// 	log.Fatalf("Error getting working directory: %v", err)
	// }

	// // Print to console
	// fmt.Println("Current Working Directory:", dir)
	// log.Println("Current Working Directory:", dir)
	for _, path := range yamlFilePaths {
		if _, err := os.Stat(path); err == nil {
			// log.Printf("Yaml path found in: %v", path)
			configPath = path
			break
		}
	}
	if configPath == "" {
		return fmt.Errorf("no valid config file found from paths: %v", yamlFilePaths)
	}

	file, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var newConfig YamlConfig
	if err := yaml.Unmarshal(file, &newConfig); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	configMutex.Lock()
	config = newConfig
	configMutex.Unlock()

	return nil
}

func WatchConfig() {
	if configPath == "" {
		log.Println("no valid config file found. Skipping watcher.")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("failed to initialize config watcher:%v", err)
	}
	defer watcher.Close()

	err = watcher.Add(configPath)
	if err != nil {
		log.Printf("failed to watch config file:%v", err)
	}

	fmt.Println("watching for yaml config changes:", configPath)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op == fsnotify.Write {
				fmt.Println("config file updated. Reloading...")
				if err := LoadConfig(); err != nil {
					log.Printf("failed to reload config:%v", err)
				} else {
					fmt.Println("config reloaded successfully.")
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("config watcher error:", err)
		}
	}
}

func GetConfig() YamlConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config
}

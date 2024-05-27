package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/viper"
)

type Config struct {
	Db      DbConfig           `mapstructure:"db-config"`
	Btc     BtcConfig          `mapstructure:"btc-config"`
	Signer  RemoteSignerConfig `mapstructure:"remote-signer-config"`
	Metrics MetricsConfig      `mapstructure:"metrics-config"`
}

func DefaultConfig() *Config {
	return &Config{
		Db:      *DefaultDBConfig(),
		Btc:     *DefaultBtcConfig(),
		Signer:  *DefaultRemoteSignerConfig(),
		Metrics: *DefaultMetricsConfig(),
	}
}

func (cfg *Config) Validate() error {
	if err := cfg.Db.Validate(); err != nil {
		return fmt.Errorf("invalid db config: %w", err)
	}

	if _, err := cfg.Signer.Parse(); err != nil {
		return fmt.Errorf("invalid remote signer config: %w", err)
	}

	if err := cfg.Metrics.Validate(); err != nil {
		return fmt.Errorf("invalid metrics config: %w", err)
	}

	return nil
}

const defaultConfigTemplate = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

[db-config]
# The network chain ID
db-name = "{{ .Db.DbName }}"
# The keyring's backend, where the keys are stored (os|file|kwallet|pass|test|memory)
address = "{{ .Db.Address }}"

[btc-config]
# Btc node host
host = "{{ .Btc.Host }}"
# Btc node user
user = "{{ .Btc.User }}"
# Btc node password
pass = "{{ .Btc.Pass }}"
# Btc network (testnet3|mainnet|regtest|simnet|signet)
network = "{{ .Btc.Network }}"

[remote-signer-config]
# The list of signer urls in the format http://covenant_pk@signer_host:port
urls = [{{ range .Signer.Urls }}{{ printf "%q, " . }}{{end}}]
# The timeout of each request to the remote signing server
timeout_seconds = {{ .Signer.TimeoutSeconds }}

[metrics-config]
# Enable reporting metrics
enabled = {{ .Metrics.Enabled }}
# IP of the Prometheus server
host = "{{ .Metrics.Host }}"
# Port of the Prometheus server
port = {{ .Metrics.Port }}

`

var configTemplate *template.Template

func init() {
	var err error
	tmpl := template.New("configFileTemplate").Funcs(template.FuncMap{
		"StringsJoin": strings.Join,
	})
	if configTemplate, err = tmpl.Parse(defaultConfigTemplate); err != nil {
		panic(err)
	}
}

func writeConfigToFile(configFilePath string, config *Config) error {
	var buffer bytes.Buffer

	if err := configTemplate.Execute(&buffer, config); err != nil {
		panic(err)
	}

	return os.WriteFile(configFilePath, buffer.Bytes(), 0o600)
}

func WriteConfigToFile(pathToConfFile string, conf *Config) error {
	dirPath, _ := filepath.Split(pathToConfFile)

	if _, err := os.Stat(pathToConfFile); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
			return fmt.Errorf("couldn't make config: %v", err)
		}

		if err := writeConfigToFile(pathToConfFile, conf); err != nil {
			return fmt.Errorf("could config to the file: %v", err)
		}
	}
	return nil
}

func fileNameWithoutExtension(fileName string) string {
	return strings.TrimSuffix(fileName, filepath.Ext(fileName))
}

func GetConfig(pathToConfFile string) (*Config, error) {
	dir, file := filepath.Split(pathToConfFile)
	configName := fileNameWithoutExtension(file)
	viper.SetConfigName(configName)
	viper.AddConfigPath(dir)
	viper.SetConfigType("toml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	conf := DefaultConfig()
	if err := viper.Unmarshal(conf); err != nil {
		return nil, err
	}

	return conf, nil
}

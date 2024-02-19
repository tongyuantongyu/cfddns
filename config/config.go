package config

import (
	"cfddns/common"

	"go.uber.org/zap/zapcore"
)

type Config struct {
	Service  Service          `toml:"service" json:"service" yaml:"service"`
	Log      Log              `toml:"log" json:"log" yaml:"log"`
	Provider CloudflareConfig `toml:"provider" json:"provider" yaml:"provider"`
	Address  []IPAddress      `toml:"address" json:"address" yaml:"address"`
	Domain   []Domain         `toml:"domain" json:"domain" yaml:"domain"`
}

type Service struct {
	Name        string          `toml:"name" json:"name" yaml:"name"`
	RefreshRate common.Duration `toml:"refresh_rate" json:"refresh_rate" yaml:"refresh_rate"`
}

type Log struct {
	Level     *zapcore.Level `toml:"level" json:"level" yaml:"level"`
	Encoding  *string        `toml:"encoding" json:"encoding" yaml:"encoding"`
	InfoPath  *[]string      `toml:"info_path" json:"info_path" yaml:"info_path"`
	ErrorPath *[]string      `toml:"error_path" json:"error_path" yaml:"error_path"`
}

type CloudflareConfig struct {
	APIToken  string   `toml:"api_token" json:"api_token" yaml:"api_token"`
	ZoneNames []string `toml:"zone_names" json:"zone_names" yaml:"zone_names"`
	TTL       int      `toml:"ttl" json:"ttl" yaml:"ttl"`
}

type IPAddress struct {
	Name         string          `toml:"name" json:"name" yaml:"name"`
	Sources      []IPSource      `toml:"sources" json:"sources" yaml:"sources"`
	Transformers []IPTransformer `toml:"transformers,omitempty" json:"transformers,omitempty" yaml:"transformers,omitempty"`
}

type IPSource struct {
	Type   string         `toml:"type" json:"type" yaml:"type"`
	Source string         `toml:"source" json:"source" yaml:"source"`
	Config map[string]any `toml:"config,omitempty" json:"config,omitempty" yaml:"config,omitempty"`
}

type IPSourceSimpleConfig struct {
	Type    common.Family   `mapstructure:"type"`
	Timeout common.Duration `mapstructure:"timeout"`
}

type IPSourceCloudflareTraceConfig struct {
	Type         *common.Family  `mapstructure:"type"`
	Timeout      common.Duration `mapstructure:"timeout"`
	ForceAddress string          `mapstructure:"force_address"`
	IPHost       bool            `mapstructure:"ip_host"`
}

type IPSourceInterfaceConfig struct {
	Type    common.Family         `mapstructure:"type"`
	Select  common.IPSelectMode   `mapstructure:"select"`
	Flags   []common.IPFilterFlag `mapstructure:"flags"`
	Exclude []common.CIDR         `mapstructure:"exclude"`
	Include []common.CIDR         `mapstructure:"include"`
}

type IPTransformer struct {
	Type   string         `toml:"type" json:"type" yaml:"type"`
	Config map[string]any `toml:"config,omitempty" json:"config,omitempty" yaml:"config"`
}

type IPTransformerMaskRewriteConfig struct {
	Mask      string    `mapstructure:"mask"`
	Overwrite common.IP `mapstructure:"overwrite"`
}

type Domain struct {
	Domain  string  `toml:"domain" json:"domain" yaml:"domain"`
	Type    string  `toml:"type" json:"type" yaml:"type"`
	Mark    *string `toml:"mark,omitempty" json:"mark,omitempty" yaml:"mark,omitempty"`
	Address string  `toml:"address" json:"address" yaml:"address"`
}

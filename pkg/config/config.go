package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/fluxcd/pkg/ssa"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"sigs.k8s.io/yaml"
)

const (
	KustomizerConfigKind        = "Config"
	KustomizerConfigApiVersion  = "kustomizer.dev/v1"
	KustomizerFieldManagerName  = "kustomizer"
	KustomizerFieldManagerGroup = "inventory.kustomizer.dev"
)

type Config struct {
	metav1.TypeMeta `json:",inline"`

	// ApplyOrder holds the list of the Kubernetes API Kinds that
	// describes in which order they are reconciled.
	ApplyOrder *KindOrder `json:"applyOrder,omitempty"`

	// FieldManager holds the manager name and group used for server-side apply.
	FieldManager *FieldManager `json:"fieldManager,omitempty"`
}

type FieldManager struct {
	// Name sets the field manager for the reconciled objects.
	Name string `json:"name"`

	// Group sets the owner label key prefix.
	Group string `json:"group"`
}

// KindOrder holds the list of the Kubernetes API Kinds that
// describes in which order they are reconciled.
type KindOrder struct {
	// First contains the list of Kubernetes API Kinds
	// that are applied first and delete last.
	First []string `json:"first"`

	// Last contains the list of Kubernetes API Kinds
	// that are applied last and delete first.
	Last []string `json:"last"`
}

// NewConfig returns a config with the default apply order.
func NewConfig() *Config {
	return &Config{
		TypeMeta: metav1.TypeMeta{
			Kind:       KustomizerConfigKind,
			APIVersion: KustomizerConfigApiVersion,
		},
		ApplyOrder:   defaultKindOrder(),
		FieldManager: defaultFieldManager(),
	}
}

func defaultKindOrder() *KindOrder {
	return &KindOrder{
		First: ssa.ReconcileOrder.First,
		Last:  ssa.ReconcileOrder.Last,
	}
}

func defaultFieldManager() *FieldManager {
	return &FieldManager{
		Name:  KustomizerFieldManagerName,
		Group: KustomizerFieldManagerGroup,
	}
}

// DefaultConfigPath returns '$HOME/.kustomizer/config'
func DefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".kustomizer/config"), nil
}

// Read loads the config from the specified path,
// if the config file is not found, a default is returned.
func Read(configPath string) (*Config, error) {
	if configPath == "" {
		p, err := DefaultConfigPath()
		if err != nil {
			return nil, fmt.Errorf("$HOME dir can't be determined, error: %w", err)
		}
		configPath = p
	}

	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		return NewConfig(), nil
	}

	cfgData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(cfgData, cfg); err != nil {
		return nil, err
	}

	if cfg.ApplyOrder == nil {
		cfg.ApplyOrder = defaultKindOrder()
	}

	if cfg.FieldManager == nil {
		cfg.FieldManager = defaultFieldManager()
	}

	if cfg.FieldManager.Name == "" {
		return nil, fmt.Errorf("the filed manager name can't be empty")
	}

	if cfg.FieldManager.Group == "" {
		return nil, fmt.Errorf("the filed manager group can't be empty")
	}

	return cfg, nil
}

// Write saves the config at the given path, if no path is specified
// it will create or override '$HOME/.kustomizer/config'.
func (c *Config) Write(configPath string) error {
	if configPath == "" {
		p, err := DefaultConfigPath()
		if err != nil {
			return err
		}
		configPath = p
	}

	if err := os.MkdirAll(filepath.Dir(configPath), os.FileMode(0755)); err != nil {
		return err
	}

	cfgData, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, cfgData, os.FileMode(0666)); err != nil {
		return err
	}

	return nil
}

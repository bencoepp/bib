package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

const (
	secretPrefixEnv  = "env://"
	secretPrefixFile = "file://"
)

// resolveSecrets processes a config struct and resolves any secret references
// Supported formats:
//   - env://ENV_VAR_NAME - reads from environment variable
//   - file:///path/to/secret - reads from file (trims whitespace)
func resolveSecrets(cfg interface{}) error {
	return resolveSecretsRecursive(reflect.ValueOf(cfg))
}

func resolveSecretsRecursive(v reflect.Value) error {
	// Dereference pointer
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if field.CanSet() {
				if err := resolveSecretsRecursive(field); err != nil {
					return err
				}
			}
		}
	case reflect.String:
		if v.CanSet() {
			resolved, err := resolveSecretValue(v.String())
			if err != nil {
				return err
			}
			v.SetString(resolved)
		}
	}

	return nil
}

// resolveSecretValue resolves a single secret value if it has a secret prefix
func resolveSecretValue(value string) (string, error) {
	switch {
	case strings.HasPrefix(value, secretPrefixEnv):
		envVar := strings.TrimPrefix(value, secretPrefixEnv)
		envValue := os.Getenv(envVar)
		if envValue == "" {
			return "", fmt.Errorf("environment variable %q not set", envVar)
		}
		return envValue, nil

	case strings.HasPrefix(value, secretPrefixFile):
		filePath := strings.TrimPrefix(value, secretPrefixFile)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read secret file %q: %w", filePath, err)
		}
		return strings.TrimSpace(string(data)), nil

	default:
		return value, nil
	}
}

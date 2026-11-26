package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/config"
)

// A plugin's config is kept where ws's own is: the team's in ws.toml
// ([plugin.<name>]), yours over it in ws.local.toml, which is the one ws
// writes. A secret is in neither: it's in a file only you can read,
// beside the plugin's data.

func secretsPath(root, name string) string { return filepath.Join(home(root, name), "secrets.json") }

func readSecrets(root, name string) map[string]any {
	out := map[string]any{}
	if data, err := os.ReadFile(secretsPath(root, name)); err == nil {
		json.Unmarshal(data, &out) //nolint:errcheck // a file that isn't JSON holds no secrets
	}
	return out
}

// Values are a plugin's config as it's set: what the files say, and its
// secrets. A key that isn't set is absent, whatever its default.
func Values(root, name string) (map[string]any, error) {
	cfg, _, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for k, v := range cfg.Plugin[name] {
		out[k] = jsonValue(v)
	}
	maps.Copy(out, readSecrets(root, name))
	return out, nil
}

// jsonValue is a value read from TOML as the protocol carries it.
func jsonValue(v any) any {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			out = append(out, fmt.Sprint(e))
		}
		return out
	}
	return v
}

// Missing are the keys a plugin requires that have no value: it can't be
// asked anything until they're set.
func Missing(r Registration, values map[string]any) []ConfigKey {
	var out []ConfigKey
	for _, k := range r.Config {
		if !k.Required || k.Default != nil {
			continue
		}
		if v, ok := values[k.Key]; !ok || v == "" {
			out = append(out, k)
		}
	}
	return out
}

// ParseValue is what someone typed for a key, as the value it is.
func ParseValue(k ConfigKey, s string) (any, error) {
	s = strings.TrimSpace(s)
	switch k.Type {
	case "bool":
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("%s is true or false", k.Key)
		}
		return b, nil
	case "number":
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("%s is a number", k.Key)
		}
		return n, nil
	case "list":
		out := []string{}
		for part := range strings.SplitSeq(s, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
		return out, nil
	case "enum":
		if !slices.Contains(k.Options, s) {
			return nil, fmt.Errorf("%s is one of: %s", k.Key, strings.Join(k.Options, ", "))
		}
		return s, nil
	case "url":
		u, err := url.Parse(s)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("%s is an address, such as https://example.com", k.Key)
		}
		return s, nil
	}
	return s, nil
}

// SetValue sets a plugin's key: a secret in its secrets, anything else in
// your ws.local.toml. A nil value unsets it.
func SetValue(root, name string, k ConfigKey, value any) error {
	if k.Secret() {
		secrets := readSecrets(root, name)
		if value == nil || value == "" {
			delete(secrets, k.Key)
		} else {
			secrets[k.Key] = value
		}
		data, err := json.MarshalIndent(secrets, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(secretsPath(root, name), append(data, '\n'), 0o600)
	}
	set := func(local *config.LocalConfig) {
		if value == nil {
			delete(local.Plugin[name], k.Key)
			return
		}
		if local.Plugin == nil {
			local.Plugin = map[string]map[string]any{}
		}
		if local.Plugin[name] == nil {
			local.Plugin[name] = map[string]any{}
		}
		local.Plugin[name][k.Key] = tomlValue(value)
	}
	err := config.EditLocal(root, set)
	if errors.Is(err, config.ErrHandEdit) {
		err = config.UpdateLocal(root, set)
	}
	return err
}

// tomlValue is a value as ws.local.toml holds it: a whole number as one.
func tomlValue(v any) any {
	if f, ok := v.(float64); ok && f == float64(int64(f)) {
		return int64(f)
	}
	return v
}

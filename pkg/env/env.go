package env

import (
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/caarlos0/env/v11"
)

var (
	deploymentEnv string

	isProduction bool
	isLocal      bool
	isDebug      bool
)

func init() {
	env := os.Getenv("DEPLOYMENT_ENV")
	validDeploymentEnvs := []string{"local", "dev", "sit", "uat", "staging", "prod"}
	if !slices.Contains(validDeploymentEnvs, env) {
		slog.Error("Detected invalid DEPLOYMENT_ENV value",
			slog.String("current_value", env),
			slog.String("valid_values", strings.Join(validDeploymentEnvs, ", ")),
		)
		os.Exit(1)
	}

	deploymentEnv = env
	isProduction = slices.Contains([]string{"staging", "prod"}, env)
	isLocal = env == "local"

	debug := os.Getenv("DEBUG")
	if debug == "true" {
		isDebug = true
	}
}

func DeploymentEnv() string {
	return deploymentEnv
}

func InProduction() bool {
	return isProduction
}

func InDevelopment() bool {
	return !isProduction
}

func InLocalMachine() bool {
	return isLocal
}

func IsDebug() bool {
	return isDebug
}

func Parse(cfg any) {
	err := env.ParseWithOptions(cfg, env.Options{
		RequiredIfNoDef: true,
	})
	if err != nil {
		if msg := err.Error(); strings.Contains(msg, "required environment variable") {
			re := regexp.MustCompile(`"(.*?)"`)
			matches := re.FindAllStringSubmatch(msg, -1)
			missingVars := make([]string, 0, len(matches))
			for _, match := range matches {
				missingVars = append(missingVars, match[1])
			}
			slog.Error("Missing required environment variables",
				slog.String("missing_envs", strings.Join(missingVars, ", ")),
			)
		} else {
			slog.Error("Failed to parse environment variables",
				slog.String("error", msg),
			)
		}
		os.Exit(1)
	}
}

type Optional string

func (o *Optional) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}

	specs := strings.SplitN(string(text), ":", 2)
	if len(specs) < 2 {
		*o = Optional(string(text))
		return nil
	}

	value := specs[1]
	for _, param := range strings.Split(specs[0], ",") {
		switch param {
		case "file":
			b, err := os.ReadFile(value)
			if err != nil {
				return err
			}
			value = string(b)
		case "base64":
			b, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				return err
			}
			value = string(b)
		case "hex":
			b, err := hex.DecodeString(value)
			if err != nil {
				return err
			}
			value = string(b)
		default:
			return nil
		}
	}

	*o = Optional(value)
	return nil
}

func (o *Optional) String() string {
	return string(*o)
}

func (o *Optional) Bytes() []byte {
	return []byte(*o)
}

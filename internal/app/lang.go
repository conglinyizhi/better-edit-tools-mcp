package app

import (
	"os"
	"strings"
)

const (
	Name    = "better-edit-tools"
	Version = "0.8.0"
)

func NormalizeLang(tag string) string {
	normalized := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(tag, "_", "-")))
	if normalized == "" {
		return ""
	}
	switch strings.Split(normalized, "-")[0] {
	case "zh":
		return "zh"
	case "en":
		return "en"
	default:
		return ""
	}
}

func LangFromEnv() string {
	if env := os.Getenv("LANG"); env != "" {
		if lang := NormalizeLang(env); lang != "" {
			return lang
		}
	}
	return "en"
}

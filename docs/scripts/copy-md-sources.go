// copy-md-sources 把 Hugo content/ 下的 Markdown 源文件复制到 public/ 对应路径，作为 index.md。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	contentDir := envOr("CONTENT_DIR", "content")
	publicDir := envOr("PUBLIC_DIR", "public")
	defaultLang := envOr("DEFAULT_LANG", "zh")

	chosen := map[string]string{} // key: relDir|stem|lang
	if err := filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}

		rel, err := filepath.Rel(contentDir, path)
		if err != nil {
			return err
		}
		relDir := filepath.Dir(rel)
		if relDir == "." {
			relDir = ""
		}
		filename := filepath.Base(rel)
		stem := strings.TrimSuffix(filename, ".md")

		lang := defaultLang
		if strings.HasSuffix(stem, ".zh") {
			lang = "zh"
			stem = strings.TrimSuffix(stem, ".zh")
		} else if strings.HasSuffix(stem, ".en") {
			lang = "en"
			stem = strings.TrimSuffix(stem, ".en")
		}

		key := relDir + "|" + stem + "|" + lang
		existing := chosen[key]
		if existing == "" {
			chosen[key] = path
			return nil
		}
		// 优先使用带明确默认语言后缀的版本
		if lang == defaultLang && strings.HasSuffix(filename, "."+defaultLang+".md") {
			chosen[key] = path
		}
		return nil
	}); err != nil {
		fmt.Fprintln(os.Stderr, "walk error:", err)
		os.Exit(1)
	}

	for key, src := range chosen {
		parts := strings.Split(key, "|")
		relDir, stem, lang := parts[0], parts[1], parts[2]

		var dstDir string
		if stem == "_index" {
			if lang == defaultLang {
				dstDir = filepath.Join(publicDir, relDir)
			} else {
				dstDir = filepath.Join(publicDir, lang, relDir)
			}
		} else {
			pageDir := strings.ToLower(stem)
			if lang == defaultLang {
				dstDir = filepath.Join(publicDir, relDir, pageDir)
			} else {
				dstDir = filepath.Join(publicDir, lang, relDir, pageDir)
			}
		}

		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "mkdir error:", err)
			os.Exit(1)
		}
		dst := filepath.Join(dstDir, "index.md")
		if err := copyFile(src, dst); err != nil {
			fmt.Fprintln(os.Stderr, "copy error:", err)
			os.Exit(1)
		}
		fmt.Printf("copied %s -> %s\n", src, dst)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

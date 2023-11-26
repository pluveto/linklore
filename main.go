package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type FileInfo struct {
	name     string
	basename string
	ext      string
	path     string
}

type Config struct {
	inputFile      string
	outputFile     string
	ignorePatterns []string
	baseDir        string
	prefix         string
	force          bool
	index          map[string]FileInfo
}

var (
	// Match an optional ! at the beginning.
	// Then [[ followed by a series of characters that are not |, [, ], #, or ^ (the base link).
	// Optionally match a | followed by a series of characters that are not |, [, ], #, or ^ (the alias).
	// Optionally match a # followed by a series of characters that are not |, [, ], #, or ^ (the anchor).
	// Optionally match a ^ followed by a series of characters that are not |, [, ], #, or ^ (the block).
	// Finally match the closing ]].
	linkComponentPattern = `([^|\[\]#^]+)`
	linkPattern          = regexp.MustCompile(`!?` +
		`\[\[` + linkComponentPattern +
		`(?:\|` + linkComponentPattern + `)?` +
		`(?:#` + linkComponentPattern + `)?` +
		`(?:\^` + linkComponentPattern + `)?` +
		`\]\]`)

	Version = "dev"
)

func main() {
	config := loadConfig()
	err := validateConfig(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid args:", err)
		os.Exit(1)
	}
	err = buildIndex(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error building index:", err)
		os.Exit(1)
	}

	err = processFile(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error processing file:", err)
		os.Exit(1)
	}
}

func validateConfig(config Config) error {
	if config.inputFile == "" {
		return errors.New("input file is not specified")
	}
	if config.outputFile == "" {
		return errors.New("output file is not specified")
	}
	if config.baseDir == "" {
		return errors.New("base directory is not specified")
	}
	if config.ignorePatterns == nil {
		return errors.New("bug: ignore patterns should not be nil, expect []")
	}

	for _, pattern := range config.ignorePatterns {
		_, err := filepath.Match(pattern, "")
		if err != nil {
			return fmt.Errorf("invalid ignore pattern: %s (cannot be used with "+
				"filepath.Match. see: https://golang.org/pkg/path/filepath/#Match)", pattern)
		}

		patternTrimmed := strings.TrimSpace(pattern)
		if patternTrimmed == "" {
			return fmt.Errorf("invalid ignore pattern: (emtpy string)")
		}

		if patternTrimmed != pattern {
			return fmt.Errorf("invalid ignore pattern: %s (leading or trailing whitespace)", pattern)
		}

	}
	return nil
}

func loadConfig() Config {
	config := Config{
		index:          make(map[string]FileInfo),
		ignorePatterns: []string{},
	}

	loadEnvVariables(&config)
	loadDotEnvVariables(&config)
	parseCommandLineFlags(&config)
	setDefaultValues(&config)

	return config
}

func loadEnvVariables(config *Config) {
	config.inputFile = getEnvOrDefault("LINKLORE_INPUT_FILE", "")
	config.outputFile = getEnvOrDefault("LINKLORE_OUTPUT_FILE", "")
	config.baseDir = getEnvOrDefault("LINKLORE_BASE_DIR", "")
	config.prefix = getEnvOrDefault("LINKLORE_PREFIX", "")
	config.prefix = getEnvOrDefault("LINKLORE_BASE_URL", config.prefix)
	ignorePatternsRaw := getEnvOrDefault("LINKLORE_IGNORE", "")
	if ignorePatternsRaw != "" {
		config.ignorePatterns = strings.Split(ignorePatternsRaw, ",")
	}
}

func parseCommandLineFlags(config *Config) {
	flag.StringVar(&config.inputFile, "i", "", "input file")
	flag.StringVar(&config.outputFile, "o", "", "output file")
	flag.StringVar(&config.baseDir, "d", "", "base directory")
	flag.StringVar(&config.prefix, "p", "", "prefix")
	ignorePatternsRaw := flag.String("x", "", "ignore patterns")
	if *ignorePatternsRaw != "" {
		config.ignorePatterns = strings.Split(*ignorePatternsRaw, ",")
	}
	flag.BoolVar(&config.force, "f", false, "force overwrite output file")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s -i <input> [options]\n", os.Args[0])
		flag.PrintDefaults()
	}

	version := flag.Bool("v", false, "show version")
	flag.Parse()

	if *version {
		fmt.Println(Version)
		os.Exit(0)
	}
}

func setDefaultValues(config *Config) {
	if config.baseDir == "" {
		config.baseDir = "."
	}
	if config.prefix == "" {
		config.prefix = "/"
	}
	if config.outputFile == "" {
		config.outputFile = strings.TrimSuffix(config.inputFile, filepath.Ext(config.inputFile)) + ".out.md"
	}
	if len(config.ignorePatterns) == 0 {
		config.ignorePatterns = []string{".git", ".github", ".vscode", ".idea", ".env", "node_modules", ".obsidian", "*.out.md"}
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}

func buildIndex(config Config) error {
	var count int

	err := filepath.Walk(config.baseDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		for _, pattern := range config.ignorePatterns {
			matched, err := filepath.Match(pattern, info.Name())
			if err != nil {
				return err
			}
			if matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if !info.IsDir() {
			ext := filepath.Ext(path)
			basename := strings.TrimSuffix(info.Name(), ext)

			if entry, exists := config.index[basename]; exists {
				context := fmt.Sprintf("path=%s", entry.path)
				return fmt.Errorf("duplicate key: %s (context: %s)", basename, context)
			}

			relativePath, err := filepath.Rel(config.baseDir, path)
			if err != nil {
				return fmt.Errorf("failed to get relative path: %v", err)
			}

			config.index[basename] = FileInfo{
				name:     info.Name(),
				basename: basename,
				ext:      ext,
				path:     relativePath,
			}

			count++
			if count > 10000 {
				return errors.New("too many files, limit is 10000")
			}
		}

		return nil
	})

	return err
}

func processFile(config Config) error {
	if !config.force {
		if _, err := os.Stat(config.outputFile); err == nil {
			return errors.New("output file already exists")
		}
	}

	content, err := os.ReadFile(config.inputFile)
	if err != nil {
		return err
	}

	processedContent := linkPattern.ReplaceAllStringFunc(string(content), replaceLink(config))

	err = os.WriteFile(config.outputFile, []byte(processedContent), 0644)
	if err != nil {
		return err
	}

	return nil
}

func replaceLink(config Config) func(string) string {
	return func(match string) string {
		submatches := linkPattern.FindStringSubmatch(match)

		base := submatches[1]
		alias := submatches[2]
		anchor := submatches[3]

		fileInfo, exists := config.index[base]
		if !exists {
			// try match without ext
			baseWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
			fileInfo, exists = config.index[baseWithoutExt]
			if !exists {
				fmt.Fprintf(os.Stderr, "error: file not found for link: %s\n", match)
				return match
			}
		}

		link := config.prefix + fileInfo.path
		if anchor != "" {
			link += "#" + anchor
		}

		if alias == "" {
			alias = base
		}

		return fmt.Sprintf("[%s](%s)", alias, link)
	}
}

func loadDotEnvVariables(config *Config) {
	envFile, err := os.Open(".env")
	if err != nil {
		return
	}

	defer envFile.Close()

	envScanner := bufio.NewScanner(envFile)
	for envScanner.Scan() {
		line := envScanner.Text()
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]
		switch strings.ToUpper(key) {
		case "LINKLORE_INPUT_FILE":
			config.inputFile = value
		case "LINKLORE_OUTPUT_FILE":
			config.outputFile = value
		case "LINKLORE_BASE_DIR":
			config.baseDir = value
		case "LINKLORE_PREFIX":
			config.prefix = value
		case "LINKLORE_BASE_URL":
			config.prefix = value
		case "LINKLORE_FORCE":
			config.force = value == "true" || value == "1"
		case "LINKLORE_IGNORE":
			config.ignorePatterns = strings.Split(value, ",")
		}
	}
}

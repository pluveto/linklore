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
	inputFile  string
	outputFile string
	baseDir    string
	prefix     string
	force      bool
	index      map[string]FileInfo
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
	return nil
}
func loadConfig() Config {
	config := Config{
		index: make(map[string]FileInfo),
	}

	var inputFile, outputFile, baseDir, prefix string
	var force bool

	loadEnvVariables(&inputFile, &outputFile, &baseDir, &prefix, &force)

	prefixDefault := "/"
	prefixEnv := os.Getenv("LINKLORE_PREFIX")
	prefixEnvAlias := os.Getenv("LINKLORE_BASE_URL")
	if prefixEnv != "" {
		prefixDefault = prefixEnv
	} else if prefixEnvAlias != "" {
		prefixDefault = prefixEnvAlias
	}

	flag.StringVar(&inputFile, "i", os.Getenv("LINKLORE_INPUT_FILE"), "input file")
	flag.StringVar(&outputFile, "o", os.Getenv("LINKLORE_OUTPUT_FILE"), "output file")
	flag.StringVar(&baseDir, "d", os.Getenv("LINKLORE_BASE_DIR"), "base directory")
	flag.StringVar(&prefix, "p", prefixDefault, "prefix")
	flag.BoolVar(&force, "f", false, "force overwrite output file")

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

	if baseDir == "" {
		baseDir = "."
	}
	if prefix == "" {
		prefix = "/"
	}

	config.inputFile = inputFile
	config.outputFile = outputFile
	config.baseDir = baseDir
	config.prefix = prefix
	config.force = force

	return config
}

func buildIndex(config Config) error {
	var count int

	err := filepath.Walk(config.baseDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			ext := filepath.Ext(path)
			basename := strings.TrimSuffix(info.Name(), ext)

			if _, exists := config.index[basename]; exists {
				return fmt.Errorf("duplicate key: %s", basename)
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
	if config.outputFile == "" {
		config.outputFile = strings.TrimSuffix(config.inputFile, filepath.Ext(config.inputFile)) + ".out.md"
	}

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
			fmt.Printf("Error: file not found for link: %s\n", match)
			return match
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

func loadEnvVariables(inputFile, outputFile, baseDir, prefix *string, force *bool) {
	envFile, err := os.Open(".env")
	if err != nil {
		return
	}
	defer envFile.Close()

	envScanner := bufio.NewScanner(envFile)
	for envScanner.Scan() {
		line := envScanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		switch strings.ToUpper(key) {
		case "LINKLORE_INPUT_FILE":
			*inputFile = value
		case "LINKLORE_OUTPUT_FILE":
			*outputFile = value
		case "LINKLORE_BASE_DIR":
			*baseDir = value
		case "LINKLORE_PREFIX":
			*prefix = value
		case "LINKLORE_BASE_URL":
			*prefix = value
		case "LINKLORE_FORCE":
			*force = value == "true" || value == "1"
		}
	}
}

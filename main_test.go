package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestLinkPattern(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
		base     string
		alias    string
		anchor   string
	}{
		{input: "[[Link]]", expected: true, base: "Link"},
		{input: "![[Link]]", expected: true, base: "Link"},
		{input: "![[Link#Anchor]]", expected: true, base: "Link", anchor: "Anchor"},
		{input: "![[Link^Block]]", expected: true, base: "Link"},
		{input: "[[Link|Alias]]", expected: true, base: "Link", alias: "Alias"},
		{input: "[[Link|Alias^Block]]", expected: true, base: "Link", alias: "Alias"},
		{input: "[[Link|Alias^Block^Extra]]", expected: false, base: "Link", alias: "Alias"},
		{input: "[[Link|Alias^#Anchor]]", expected: false},
		{input: "[[Link|Alias^#Anchor^Extra]]", expected: false},
		{input: "[[Link|Alias^Extra#Anchor]]", expected: false},
		{input: "[[Link|Alias^Extra#Anchor^Extra]]", expected: false},
		{input: "[Link]", expected: false},
		{input: "[[Link", expected: false},
		{input: "Link]]", expected: false},
	}

	for _, test := range tests {
		matched := linkPattern.MatchString(test.input)
		if matched != test.expected {
			t.Errorf("Input: %s, Expected: %v, Got: %v", test.input, test.expected, matched)
		}
		match := linkPattern.FindString(test.input)

		if !matched {
			continue
		}

		submatches := linkPattern.FindStringSubmatch(match)

		base := submatches[1]
		alias := submatches[2]
		anchor := submatches[3]

		if base != test.base {
			t.Errorf("Input: %s, Expected base: %s, Got: %s", test.input, test.base, base)
		}

		if alias != test.alias {
			t.Errorf("Input: %s, Expected alias: %s, Got: %s", test.input, test.alias, alias)
		}

		if anchor != test.anchor {
			t.Errorf("Input: %s, Expected anchor: %s, Got: %s", test.input, test.anchor, anchor)
		}
	}
}

func TestBuildIndex(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	createTestFile(tempDir, "file1.txt", "")
	createTestFile(tempDir, "file2.txt", "")

	config := Config{
		baseDir: tempDir,
		index:   make(map[string]FileInfo),
	}

	err := buildIndex(config)
	if err != nil {
		t.Errorf("buildIndex failed: %v", err)
	}

	expectedIndex := map[string]FileInfo{
		"file1": {
			name:     "file1.txt",
			basename: "file1",
			ext:      ".txt",
			path:     "file1.txt",
		},
		"file2": {
			name:     "file2.txt",
			basename: "file2",
			ext:      ".txt",
			path:     "file2.txt",
		},
	}

	if len(config.index) != len(expectedIndex) {
		t.Errorf("buildIndex failed: incorrect index size, got %d, want %d", len(config.index), len(expectedIndex))
	}

	for key, expectedFileInfo := range expectedIndex {
		fileInfo, exists := config.index[key]
		if !exists {
			t.Errorf("buildIndex failed: missing key %s in index", key)
			continue
		}

		if fileInfo != expectedFileInfo {
			t.Errorf("buildIndex failed: incorrect FileInfo for key %s, got %+v, want %+v", key, fileInfo, expectedFileInfo)
		}
	}
}

func TestProcessFile(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	createTestFile(tempDir, "input.txt", "[[file1]] [[file2]]")
	createTestFile(tempDir, "file1.txt", "")
	createTestFile(tempDir, "file2.txt", "")

	config := Config{
		inputFile:  filepath.Join(tempDir, "input.txt"),
		outputFile: filepath.Join(tempDir, "output.txt"),
		baseDir:    tempDir,
		prefix:     "/",
		force:      true,
		index: map[string]FileInfo{
			"file1": {
				name:     "file1.txt",
				basename: "file1",
				ext:      ".txt",
				path:     "file1.txt",
			},
			"file2": {
				name:     "file2.txt",
				basename: "file2",
				ext:      ".txt",
				path:     "file2.txt",
			},
		},
	}

	err := processFile(config)
	if err != nil {
		t.Errorf("processFile failed: %v", err)
	}

	expectedOutput := "[file1](/file1.txt) [file2](/file2.txt)"
	outputContent, err := ioutil.ReadFile(config.outputFile)
	if err != nil {
		t.Errorf("processFile failed: unable to read output file: %v", err)
	}

	if string(outputContent) != expectedOutput {
		t.Errorf("processFile failed: incorrect output content, got %s, want %s", outputContent, expectedOutput)
	}
}

func TestProcessFileNested(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)

	// Create test files in a nested directory
	nestedDir := filepath.Join(tempDir, "nested")
	os.Mkdir(nestedDir, 0755)
	createTestFile(nestedDir, "input.txt", "[[file1]] [[file2]]")
	createTestFile(nestedDir, "file1.txt", "")
	createTestFile(nestedDir, "file2.txt", "")

	config := Config{
		inputFile:  filepath.Join(nestedDir, "input.txt"),
		outputFile: filepath.Join(nestedDir, "output.txt"),
		baseDir:    nestedDir,
		prefix:     "/nested/",
		force:      true,
		index: map[string]FileInfo{
			"file1": {
				name:     "file1.txt",
				basename: "file1",
				ext:      ".txt",
				path:     "file1.txt",
			},
			"file2": {
				name:     "file2.txt",
				basename: "file2",
				ext:      ".txt",
				path:     "file2.txt",
			},
		},
	}

	err := processFile(config)
	if err != nil {
		t.Errorf("processFile failed: %v", err)
	}

	expectedOutput := "[file1](/nested/file1.txt) [file2](/nested/file2.txt)"
	outputContent, err := ioutil.ReadFile(config.outputFile)
	if err != nil {
		t.Errorf("processFile failed: unable to read output file: %v", err)
	}

	if string(outputContent) != expectedOutput {
		t.Errorf("processFile failed: incorrect output content, got %s, want %s", outputContent, expectedOutput)
	}
}

func createTempDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "linklore_test")
	if err != nil {
		t.Fatalf("createTempDir failed: %v", err)
	}
	return tempDir
}

func createTestFile(dir, filename, content string) {
	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		panic(err)
	}
}

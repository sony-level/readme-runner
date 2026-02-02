// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// README parsing functionality

package scanner

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

var (
	// Regex patterns for section detection
	sectionHeaderRegex = regexp.MustCompile(`^#{1,6}\s+(.+)$`)
	codeBlockRegex     = regexp.MustCompile("^```")
	shellBlockRegex    = regexp.MustCompile("^```(bash|sh|shell|console|zsh|powershell|cmd)")
)

// MaxReadmeSize is the maximum README content to load (512KB)
const MaxReadmeSize = 512 * 1024

// PreviewLines is the number of lines to show for truncated content
const PreviewLines = 50

// ParseReadme analyzes a README file and extracts metadata
func ParseReadme(path, relPath string) (*ReadmeInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	readme := &ReadmeInfo{
		Path:         path,
		RelPath:      relPath,
		Size:         info.Size(),
		OriginalSize: info.Size(),
		Sections:     []string{},
	}

	// Read content with size limiting
	if info.Size() <= MaxReadmeSize {
		content, err := os.ReadFile(path)
		if err == nil {
			readme.Content = string(content)
		}
	} else {
		// Load truncated content for large files
		readme.Truncated = true
		content, err := loadTruncatedContent(path, MaxReadmeSize)
		if err == nil {
			readme.Content = content
		}
	}

	// Parse the file line by line
	scanner := bufio.NewScanner(file)
	inCodeBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		// Detect code blocks
		if codeBlockRegex.MatchString(line) {
			if !inCodeBlock {
				readme.CodeBlocks++
				if shellBlockRegex.MatchString(line) {
					readme.ShellCommands++
				}
			}
			inCodeBlock = !inCodeBlock
			continue
		}

		// Skip content inside code blocks for section detection
		if inCodeBlock {
			continue
		}

		// Detect section headers
		if matches := sectionHeaderRegex.FindStringSubmatch(line); matches != nil {
			sectionTitle := strings.TrimSpace(matches[1])
			readme.Sections = append(readme.Sections, sectionTitle)

			// Check for key sections
			lowerTitle := strings.ToLower(sectionTitle)
			checkSectionType(lowerTitle, readme)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return readme, nil
}

// loadTruncatedContent loads the first maxBytes of a file
func loadTruncatedContent(path string, maxBytes int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read up to maxBytes
	content := make([]byte, maxBytes)
	n, err := file.Read(content)
	if err != nil {
		return "", err
	}

	// Find the last newline to avoid cutting mid-line
	result := string(content[:n])
	if lastNewline := strings.LastIndex(result, "\n"); lastNewline > 0 {
		result = result[:lastNewline]
	}

	return result, nil
}

// checkSectionType identifies the type of section based on title
func checkSectionType(lowerTitle string, readme *ReadmeInfo) {
	// Installation patterns
	installPatterns := []string{"install", "setup", "getting started", "prerequisites", "requirements"}
	for _, pattern := range installPatterns {
		if strings.Contains(lowerTitle, pattern) {
			readme.HasInstall = true
			break
		}
	}

	// Usage patterns
	usagePatterns := []string{"usage", "how to use", "example", "demo", "tutorial"}
	for _, pattern := range usagePatterns {
		if strings.Contains(lowerTitle, pattern) {
			readme.HasUsage = true
			break
		}
	}

	// Build patterns
	buildPatterns := []string{"build", "compile", "development", "contributing", "running"}
	for _, pattern := range buildPatterns {
		if strings.Contains(lowerTitle, pattern) {
			readme.HasBuild = true
			break
		}
	}

	// Quick start patterns
	quickStartPatterns := []string{"quick start", "quickstart", "tldr", "tl;dr", "quick"}
	for _, pattern := range quickStartPatterns {
		if strings.Contains(lowerTitle, pattern) {
			readme.HasQuickStart = true
			break
		}
	}
}

// ExtractCodeBlocks extracts all code blocks from README content
func ExtractCodeBlocks(content string) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(content, "\n")

	var currentBlock *CodeBlock
	inBlock := false

	for _, line := range lines {
		if codeBlockRegex.MatchString(line) {
			if !inBlock {
				// Start of code block
				lang := strings.TrimPrefix(line, "```")
				lang = strings.TrimSpace(lang)
				currentBlock = &CodeBlock{
					Language: lang,
					IsShell:  isShellLanguage(lang),
					Lines:    []string{},
				}
				inBlock = true
			} else {
				// End of code block
				if currentBlock != nil {
					currentBlock.Content = strings.Join(currentBlock.Lines, "\n")
					blocks = append(blocks, *currentBlock)
				}
				currentBlock = nil
				inBlock = false
			}
		} else if inBlock && currentBlock != nil {
			currentBlock.Lines = append(currentBlock.Lines, line)
		}
	}

	return blocks
}

// CodeBlock represents a code block extracted from README
type CodeBlock struct {
	Language string   // Language identifier (bash, go, python, etc.)
	IsShell  bool     // Whether this is a shell command block
	Content  string   // Full content of the block
	Lines    []string // Individual lines
}

// isShellLanguage checks if the language is a shell language
func isShellLanguage(lang string) bool {
	lang = strings.ToLower(lang)
	shellLangs := []string{"bash", "sh", "shell", "console", "zsh", "powershell", "cmd", "terminal"}
	for _, shellLang := range shellLangs {
		if lang == shellLang {
			return true
		}
	}
	return false
}

// GetShellCommands extracts shell commands from README content
func GetShellCommands(content string) []string {
	blocks := ExtractCodeBlocks(content)
	var commands []string

	for _, block := range blocks {
		if block.IsShell {
			// Split into individual commands
			for _, line := range block.Lines {
				line = strings.TrimSpace(line)
				// Skip empty lines and comments
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				// Remove common prompt prefixes
				line = strings.TrimPrefix(line, "$ ")
				line = strings.TrimPrefix(line, "> ")
				if line != "" {
					commands = append(commands, line)
				}
			}
		}
	}

	return commands
}

package backlinker

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	wikilinks "github.com/dangoor/goldmark-wikilinks"
	"github.com/naoina/toml"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

type backlink struct {
	OtherFile *markdownFile
	Context string
}

type markdownFile struct {
	OriginalName string
	Title string
	BackLinks []backlink
	IsNew bool
	newData *bytes.Buffer
}

func getFileList(sourceDir string) ([]string, error) {
	result := make([]string, 0)
	fileInfos, err := ioutil.ReadDir(sourceDir)
	if err != nil {
		return nil, err
	}
	for _, fileInfo := range fileInfos {
		if path.Ext(fileInfo.Name()) != ".md" {
			continue
		}
		result = append(result, fileInfo.Name())
	}
	return result, nil
}

func createMarkdownFile(originalFileName string, isNew bool) *markdownFile {
	return &markdownFile{
		OriginalName: originalFileName,
		Title:        removeExtension(originalFileName),
		BackLinks:    []backlink{},
		IsNew:        isNew,
		newData:      bytes.NewBuffer([]byte{}),
	}
}

func createFileMapping(files []string) map[string]*markdownFile {
	result := make(map[string]*markdownFile)
	for _, filename := range files {
		file := createMarkdownFile(filename, false)
		result[strings.ToLower(filename)] = file
	}
	return result
}

type backlinkCollector struct {
	currentFile *markdownFile
	fileMap map[string]*markdownFile
}

func (blc backlinkCollector) LinkWithContext(destText string, destFilename string, context string) {
	destFile, exists := blc.fileMap[destFilename]
	if !exists {
		destFile = createMarkdownFile(destText + ".md", true)
		blc.fileMap[destFilename] = destFile
	}
	destFile.BackLinks = append(destFile.BackLinks, backlink{
		OtherFile: blc.currentFile,
		Context:   context,
	})
}

func (blc backlinkCollector) Normalize(linkText string) string {
	return strings.ToLower(linkText) + ".md"
}

func collectBacklinksForFile(fileMap map[string]*markdownFile, currentFile *markdownFile, filetext []byte) {
	blc := backlinkCollector{
		currentFile: currentFile,
		fileMap:     fileMap,
	}

	wl := wikilinks.NewWikilinksParser().WithTracker(blc).WithNormalizer(blc)
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithInlineParsers(util.Prioritized(wl, 102)),
		),
	)
	reader := text.NewReader(filetext)
	md.Parser().Parse(reader)
}

func collectBacklinks(sourceDir string, fileMap map[string]*markdownFile) error {
	for _, file := range fileMap {
		if file.IsNew {
			continue
		}
		filename := path.Join(sourceDir, file.OriginalName)
		log.Printf("Collecting backlinks from %s\n", filename)
		filetext, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}
		collectBacklinksForFile(fileMap, file, filetext)
	}
	return nil
}

func adjustFrontmatter(file *markdownFile, scanner *bufio.Scanner, writer io.Writer) (string, error) {
	var front bytes.Buffer
	first := true
	noMeta := false
	foundEnd := false
	var line string
	for scanner.Scan() {
		line = scanner.Text()
		if first {
			first = false
			if line != "+++" {
				noMeta = true
				break
			}
			continue
		}
		if line == "+++" {
			foundEnd = true
			break
		}
		front.WriteString(line + "\n")
	}
	err := scanner.Err()
	if err != nil {
		return "", err
	}
	if !first && !noMeta && !foundEnd {
		return "", errors.New("no end tag found in frontmatter")
	}
	meta := make(map[string]interface{})
	if !noMeta {
		err = toml.Unmarshal(front.Bytes(), meta)
		if err != nil {
			return "", err
		}
	}

	isDateFile, err := regexp.MatchString(`\d\d\d\d-\d\d-\d\d.md`, file.OriginalName)
	if err != nil {
		return "", err
	}

	if isDateFile {
		_, hasTitle := meta["title"]
		plainFilename := removeExtension(file.OriginalName)
		if !hasTitle {
			meta["title"] = plainFilename
		}
		_, hasDate := meta["date"]
		if !hasDate {
			datetime, err := time.Parse(time.RFC3339, plainFilename+ "T21:00:00Z")
			if err != nil {
				return "", err
			}
			meta["date"] = datetime
		}
	}

	title, hasTitle := meta["title"]
	if hasTitle {
		file.Title = title.(string)
	} else {
		meta["title"] = file.Title
	}

	updatedMeta, err := toml.Marshal(meta)
	if err != nil {
		return "", err
	}
	writer.Write([]byte("+++\n"))
	writer.Write(updatedMeta)
	writer.Write([]byte("+++\n"))

	if !noMeta {
		line = ""
	}

	return line, nil
}

func removeExtension(filename string) string {
	return strings.TrimSuffix(filename, path.Ext(filename))
}

func createHugoLink(filename string) string {
	name := removeExtension(filename)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	return "../" + name + "/"
}

func convertLinksOnLine(line string, fileMap map[string]*markdownFile) string {
	replacer := func(s string) string {
		linkText := s[2:len(s)-2]

		expectedMappingName := strings.ToLower(linkText) + ".md"
		file, exists := fileMap[expectedMappingName]
		if !exists {
			file = createMarkdownFile(linkText + ".md", true)
			fileMap[expectedMappingName] = file
		}
		linkTo := createHugoLink(file.OriginalName)
		return fmt.Sprintf("[%s](%s)", linkText, linkTo)
	}
	re := regexp.MustCompile(`\[\[[^\]]+\]\]`)
	return re.ReplaceAllStringFunc(line, replacer)
}


func convertLinks(firstLine string, scanner *bufio.Scanner, fileMap map[string]*markdownFile,
	writer io.Writer) error {
	if firstLine != "" {
		updatedLine := convertLinksOnLine(firstLine, fileMap) + "\n"
		_, err := writer.Write([]byte(updatedLine))
		if err != nil {
			return err
		}
	}
	for scanner.Scan() {
		line := scanner.Text()
		updatedLine := convertLinksOnLine(line, fileMap) + "\n"
		_, err := writer.Write([]byte(updatedLine))
		if err != nil {
			return err
		}
	}
	err := scanner.Err()
	if err != nil {
		return err
	}
	return nil
}

func addBacklinks(file *markdownFile, fileMap map[string]*markdownFile, writer io.Writer) error {
	if len(file.BackLinks) == 0 {
		return nil
	}
	writer.Write([]byte(`
## Backlinks

`))
	for _, backlink := range file.BackLinks {
		title := backlink.OtherFile.Title
		link := createHugoLink(backlink.OtherFile.OriginalName)
		context := convertLinksOnLine(backlink.Context, fileMap)
		writer.Write([]byte(fmt.Sprintf(`* [%s](%s)
    * %s
`, title, link, context)))
	}
	return nil
}

func generateFileData(sourceDir string, fileMap map[string]*markdownFile) error {
	for _, file := range fileMap {
		file.newData = bytes.NewBuffer([]byte{})
		filename := path.Join(sourceDir, file.OriginalName)
		var scanner *bufio.Scanner
		if file.IsNew {
			log.Printf("%s is a new file\n", filename)
			scanner = bufio.NewScanner(strings.NewReader(""))
		} else {
			log.Printf("Reading %s\n", filename)
			fileOnDisk, err := os.Open(filename)
			if err != nil {
				return err
			}
			scanner = bufio.NewScanner(fileOnDisk)
		}
		line, err := adjustFrontmatter(file, scanner, file.newData)
		if err != nil {
			return err
		}
		err = convertLinks(line, scanner, fileMap, file.newData)
		if err != nil {
			return err
		}
	}

	// Backlinks need to be added after adjustFrontmatter has run in order to ensure
	// that the backlink titles are correct
	for _, file := range fileMap {
		err := addBacklinks(file, fileMap, file.newData)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeFiles(destDir string, fileMap map[string]*markdownFile) error {
	for _, file := range fileMap {
		writer, err := os.Create(path.Join(destDir, file.OriginalName))
		if err != nil {
			return err
		}
		_, err = writer.Write(file.newData.Bytes())
		if err != nil {
			return err
		}
		err = writer.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// ProcessBackLinks converts markdown files with backlinks to new markdown files that cross-reference
// properly.
//
// There are four steps:
// 1. Collect filenames so that link case can be normalized
// 2. Parse the file with goldmark to collect the backlinks and their context
// 3. Write out the new file:
//    a. Adjusted frontmatter
//    b. Text with links changed
//    c. Backlinks
// 4. Generate stub pages for those cases in which there is no existing page
func ProcessBackLinks(sourceDir string, destDir string) error {
	files, err := getFileList(sourceDir)
	if err != nil {
		return nil
	}
	fileMap := createFileMapping(files)
	err = collectBacklinks(sourceDir, fileMap)
	if err != nil {
		return err
	}
	err = generateFileData(sourceDir, fileMap)
	if err != nil {
		return err
	}
	err = writeFiles(destDir, fileMap)
	return err
}
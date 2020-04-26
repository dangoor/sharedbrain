package backlinker

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateFileMapping(t *testing.T) {
	require := require.New(t)
	files := []string{"First.md", "Second.md", "third.md", "2020-04-26.md"}
	result := createFileMapping(files)
	require.Equal(4, len(result))
	third, exists := result["third.md"]
	require.True(exists, "third.md should be in the map")
	require.Equal("third.md", third.OriginalName)
	require.Equal("third", third.Title)
	require.Equal(0, len(third.BackLinks))
	first, exists := result["first.md"]
	require.True(exists, "first.md should be in the map")
	require.Equal("First.md", first.OriginalName)
	require.Equal("First", first.Title)
	datefile, exists := result["2020-04-26.md"]
	require.True(exists, "Should have created date file as with the others")
	require.True(datefile.IsDateFile, "Should be marked as a date file")
}

func TestCollectBacklinksForFile(t *testing.T) {
	require := require.New(t)
	fileMap := map[string]*markdownFile{
		"first.md":  {OriginalName: "First.md", BackLinks: make([]backlink, 0)},
		"second.md": {OriginalName: "Second.md", BackLinks: make([]backlink, 0)},
		"third.md":  {OriginalName: "Third.md", BackLinks: make([]backlink, 0)},
	}

	collectBacklinksForFile(fileMap, fileMap["first.md"], []byte(`
* This is a line with no links
* This is a line [with a regular link](https://google.com)
* This is a line with a link to [[second]]
* This is another line with no links
* This links to an [[Unknown]]
`))
	second := fileMap["second.md"]
	require.Equal(1, len(second.BackLinks))
	bl := second.BackLinks[0]
	require.Equal("First.md", bl.OtherFile.OriginalName)
	require.Equal("This is a line with a link to [[second]]", bl.Context)

	unknown, exists := fileMap["unknown.md"]
	require.True(exists, "Unknown should have been added")
	require.Equal("Unknown.md", unknown.OriginalName)
	require.True(unknown.IsNew, "Should have been marked as new")
}

func TestNoFrontmatterReturnsFirstLine(t *testing.T) {
	require := require.New(t)
	file := markdownFile{
		OriginalName: "AFile.md",
		BackLinks:    nil,
	}
	scanner := bufio.NewScanner(strings.NewReader(`This is the first line
This is the second`))
	err := extractFrontmatter(&file, scanner)
	require.Nil(err)
	require.Equal("This is the first line", file.firstLine)
}

func TestFrontmatterMayPassThroughUnchanged(t *testing.T) {
	require := require.New(t)
	file := markdownFile{
		OriginalName: "AFile.md",
		BackLinks:    nil,
	}
	inputText := `+++
title = "There's Metadata"
date = 2019-08-26T19:34:48-04:00
+++
`
	scanner := bufio.NewScanner(strings.NewReader(inputText))
	writer := bytes.Buffer{}
	file.scanner = scanner
	err := extractFrontmatter(&file, scanner)
	err = adjustFrontmatter(&file, &writer)
	require.Nil(err)
	require.Equal("", file.firstLine)
	output := writer.String()
	require.Contains(output, "date = 2019-08-26T19:34:48")
	require.Contains(output, "title = \"There's")
	require.True(strings.HasPrefix(output, "+++\n"))
	require.Equal("There's Metadata", file.Title)
}

func TestFrontmatterForDatePages(t *testing.T) {
	require := require.New(t)
	file := createMarkdownFile("2020-04-19.md", false)
	inputText := `## This is an example

... of a typical date page.
`
	scanner := bufio.NewScanner(strings.NewReader(inputText))
	writer := bytes.Buffer{}
	file.scanner = scanner
	err := extractFrontmatter(file, scanner)
	require.Nil(err)
	err = adjustFrontmatter(file, &writer)
	require.Nil(err)
	require.Equal("## This is an example", file.firstLine)
	output := writer.String()
	require.True(strings.HasPrefix(output, "+++\n"))
	require.Contains(output, "date = 2020-04-19T21:00:00Z\n")
	require.Contains(output, "title = \"2020-04-19\"\n")
}

func TestDateAddedBasedOnBacklinks(t *testing.T) {
	require := require.New(t)
	timestamp, _ := time.Parse(time.RFC3339, "2020-04-25T19:00:00Z")
	otherFile := markdownFile{
		OriginalName: "2020-04-25.md",
		metadata: map[string]interface{}{
			"date": timestamp,
		},
	}
	file := createMarkdownFile("Unknown.md", true)
	file.BackLinks = append(file.BackLinks, backlink{
		OtherFile: &otherFile,
		Context:   "Linking to [[Unknown]]",
	})
	writer := bytes.Buffer{}
	err := adjustFrontmatter(file, &writer)
	require.Nil(err)
	output := writer.String()
	require.True(strings.HasPrefix(output, "+++\n"))
	require.Contains(output, "date = 2020-04-25T19:00:00Z\n")
}

func TestNoDateAddedIfBacklinkHasNoDate(t *testing.T) {
	require := require.New(t)
	otherFile := createMarkdownFile("NotImportant.md", false)
	file := createMarkdownFile("Unknown.md", true)
	file.BackLinks = append(file.BackLinks, backlink{
		OtherFile: otherFile,
		Context:   "Linking to [[Unknown]]",
	})
	writer := bytes.Buffer{}
	err := adjustFrontmatter(file, &writer)
	require.Nil(err)
	output := writer.String()
	require.True(strings.HasPrefix(output, "+++\n"))
	require.NotContains(output, "date\n")
}

func TestFrontmatterWithSimplifiedDate(t *testing.T) {
	require := require.New(t)
	file := markdownFile{
		OriginalName: "AFile.md",
		BackLinks:    nil,
	}
	inputText := `+++
title = "There's Metadata"
date = 2019-08-26
+++
`
	scanner := bufio.NewScanner(strings.NewReader(inputText))
	writer := bytes.Buffer{}
	file.scanner = scanner
	err := extractFrontmatter(&file, file.scanner)
	require.Nil(err)
	err = adjustFrontmatter(&file, &writer)
	require.Nil(err)
	require.Equal("", file.firstLine)
	output := writer.String()
	require.Contains(output, "date = 2019-08-26T00:00:00Z")
}

func TestConvertLinksOnLine(t *testing.T) {
	require := require.New(t)
	fileMap := map[string]*markdownFile{
		"first.md":            {OriginalName: "First.md", BackLinks: make([]backlink, 0)},
		"second.md":           {OriginalName: "Second.md", BackLinks: make([]backlink, 0)},
		"third.md":            {OriginalName: "Third.md", BackLinks: make([]backlink, 0)},
		"name with spaces.md": createMarkdownFile("Name With Spaces.md", false),
	}
	line := "This line links to [[First]] and [[third]] and [[name with spaces]]."
	result := convertLinksOnLine(line, fileMap)
	require.Equal("This line links to [First](../first/) and [third](../third/) and [name with spaces](../name-with-spaces/).", result)
}

func TestConvertLinksForUnknownFile(t *testing.T) {
	require := require.New(t)
	fileMap := map[string]*markdownFile{
		"first.md": {OriginalName: "First.md", Title: "First", BackLinks: make([]backlink, 0)},
	}
	line := "This line links to [[Unknown]]!"
	result := convertLinksOnLine(line, fileMap)
	require.Equal("This line links to [Unknown](../unknown/)!", result)
	unknown, exists := fileMap["unknown.md"]
	require.True(exists, "Unknown file should have been created")
	require.Equal("Unknown.md", unknown.OriginalName)
	require.True(unknown.IsNew, "Should have been marked as a new file")
}

func TestConvertLinks(t *testing.T) {
	require := require.New(t)
	fileMap := map[string]*markdownFile{
		"first.md":  {OriginalName: "First.md", BackLinks: make([]backlink, 0)},
		"second.md": {OriginalName: "Second.md", BackLinks: make([]backlink, 0)},
		"third.md":  {OriginalName: "Third.md", BackLinks: make([]backlink, 0)},
	}
	inputText := `## This is a heading
* And here's a reference to [[Second]] and [[third]]
* And another [[second]]
`
	scanner := bufio.NewScanner(strings.NewReader(inputText))
	writer := bytes.Buffer{}
	err := convertLinks("", scanner, fileMap, &writer)
	require.Nil(err)
	output := writer.String()
	require.Equal(`## This is a heading
* And here's a reference to [Second](../second/) and [third](../third/)
* And another [second](../second/)
`, output)
}

func Test_addBacklinks(t *testing.T) {
	require := require.New(t)
	type args struct {
		file    *markdownFile
		fileMap map[string]*markdownFile
	}
	fileMap := make(map[string]*markdownFile)
	fileMap["first.md"] = createMarkdownFile("First.md", false)
	fileMap["second.md"] = createMarkdownFile("Second.md", false)
	fileMap["second.md"].Title = "Being The Second"
	fileMap["first.md"].BackLinks = append(fileMap["first.md"].BackLinks, backlink{
		OtherFile: fileMap["second.md"],
		Context:   "This has a [[first]] link.",
	})

	fileMap["third.md"] = createMarkdownFile("Third.md", false)
	fileMap["2020-04-21.md"] = createMarkdownFile("2020-04-21.md", false)
	frontmatterWriter := bytes.Buffer{}
	err := adjustFrontmatter(fileMap["2020-04-21.md"], &frontmatterWriter)
	require.Nil(err, "Should not get an error when adjusting frontmatter")
	fileMap["2020-04-24.md"] = createMarkdownFile("2020-04-24.md", false)
	err = adjustFrontmatter(fileMap["2020-04-24.md"], &frontmatterWriter)
	require.Nil(err, "Should not get an error when adjusting frontmatter")

	fileMap["third.md"].BackLinks = append(fileMap["third.md"].BackLinks, backlink{
		OtherFile: fileMap["2020-04-21.md"],
		Context:   "This links to [[Third]].",
	})
	fileMap["third.md"].BackLinks = append(fileMap["third.md"].BackLinks, backlink{
		OtherFile: fileMap["2020-04-24.md"],
		Context:   "Also has a [[third]] link.",
	})

	tests := []struct {
		name       string
		args       args
		wantWriter string
		wantErr    bool
	}{
		{
			name: "Second has empty backlinks",
			args: args{
				file:    fileMap["second.md"],
				fileMap: fileMap,
			},
			wantWriter: "",
			wantErr:    false,
		},
		{
			name: "First has a backlink from second",
			args: args{
				file:    fileMap["first.md"],
				fileMap: fileMap,
			},
			wantWriter: `
## Backlinks

* [Being The Second](../second/)
    * This has a [first](../first/) link.
`,
			wantErr: false,
		},
		{
			name: "Third has two date backlinks that should be sorted",
			args: args{
				file:    fileMap["third.md"],
				fileMap: fileMap,
			},
			wantWriter: `
## Backlinks

* [2020-04-24](../2020-04-24/)
    * Also has a [third](../third/) link.
* [2020-04-21](../2020-04-21/)
    * This links to [Third](../third/).
`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			err := addBacklinks(tt.args.file, tt.args.fileMap, writer)
			if (err != nil) != tt.wantErr {
				t.Errorf("addBacklinks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotWriter := writer.String(); gotWriter != tt.wantWriter {
				t.Errorf("addBacklinks() gotWriter = %v, want %v", gotWriter, tt.wantWriter)
			}
		})
	}
}

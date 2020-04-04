package filecollector

import (
	"bytes"
	"fmt"
	"github.com/FooSoft/goldsmith"
	wikilinks "github.com/dangoor/goldmark-wikilinks"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"path"
	"strings"
	"sync"
)

type stage int

const (
	fileStage stage = iota
	linkCollectionStage
	backlinkAdditionStage
)

type backlink struct {
	origin string
	context string
}

type fileCollector struct {
	filemapping map[string]string
	files []*goldsmith.File
	unknownFiles []string
	lock sync.Mutex
	md goldmark.Markdown
	currentStage stage
	fileInProcess string
	backlinks map[string][]backlink
}

func New() *fileCollector {
	plugin := &fileCollector{
		filemapping: map[string]string{},
		currentStage: fileStage,
		backlinks: map[string][]backlink{},
	}
	wl := wikilinks.NewWikilinksParser().WithTracker(plugin)
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Typographer),
		goldmark.WithParserOptions(parser.WithAutoHeadingID(),
			parser.WithInlineParsers(util.Prioritized(wl, 102)),
		),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	plugin.md = md
	return plugin
}

func (plugin *fileCollector) Name() string {
	return "filecollector"
}

func (plugin *fileCollector) Process(context *goldsmith.Context, inputFile *goldsmith.File) error {
	plugin.lock.Lock()
	plugin.filemapping[strings.ToLower(inputFile.Name())] = inputFile.Name()
	plugin.files = append(plugin.files, inputFile)
	plugin.lock.Unlock()
	return nil
}

func (plugin *fileCollector) LinkWithContext(destination string, context string) {
	if plugin.currentStage != linkCollectionStage {
		return
	}

	// destination is of the normalized form (FileName.html)
	backlinks, exists := plugin.backlinks[destination]
	if !exists {
		backlinks = make([]backlink, 0)
		plugin.backlinks[destination] = backlinks
	}

	backlinks = append(backlinks, backlink{
		origin:  plugin.fileInProcess,
		context: context,
	})
	plugin.backlinks[destination] = backlinks
}


func (plugin *fileCollector) Finalize(context *goldsmith.Context) error {
	plugin.currentStage = linkCollectionStage
	md := plugin.md
	for _, f := range plugin.files {
		plugin.lock.Lock()
		plugin.fileInProcess = strings.ToLower(f.Name())
		var dataIn bytes.Buffer
		if _, err := dataIn.ReadFrom(f); err != nil {
			return err
		}
		var dataOut bytes.Buffer
		if err := md.Convert(dataIn.Bytes(), &dataOut); err != nil {
			return err
		}
		plugin.lock.Unlock()
	}

	plugin.currentStage = backlinkAdditionStage

	for _, f := range plugin.files {
		fn := strings.TrimRight(f.Name(), path.Ext(f.Name())) + ".html"
		backlinks, exists := plugin.backlinks[fn]
		if !exists {
			context.DispatchFile(f)
		}

		var dataIn bytes.Buffer
		if _, err := dataIn.ReadFrom(f); err != nil {
			return err
		}
		var dataOut bytes.Buffer
		dataOut.WriteString(`

# Related pages

`)
		for _, backlink := range backlinks {
			otherPage := strings.TrimRight(backlink.origin, path.Ext(backlink.origin))
			dataOut.WriteString(fmt.Sprintf("* [[%s]]: %s\n", otherPage, backlink.context))
		}
		outputFile := context.CreateFileFromData(f.Path(), dataOut.Bytes())
		outputFile.Meta = f.Meta
		context.DispatchFile(outputFile)
	}
	return nil
}

func (plugin *fileCollector) Normalize(linkText string) string {
	fn, exists := plugin.filemapping[strings.ToLower(linkText) + ".md"]
	if !exists {
		if plugin.currentStage == fileStage {
			plugin.unknownFiles = append(plugin.unknownFiles, linkText)
		}
		fn = linkText + ".html"
	} else {
		fn = strings.TrimRight(fn, path.Ext(fn)) + ".html"
	}
	return fn
}
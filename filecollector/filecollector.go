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
	"log"
	"path"
	"strings"
	"sync"
)

type stage int

const (
	fileStage stage = iota
	linkCollectionStage
	backlinkAdditionStage
	finalProcessing
)

type FCContext interface {
	DispatchFile(file *goldsmith.File)
	CreateFileFromData(sourcePath string, data []byte) *goldsmith.File
}

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
	wl := wikilinks.NewWikilinksParser().WithTracker(plugin).WithNormalizer(plugin)
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

func (plugin *fileCollector) Process(context FCContext, inputFile *goldsmith.File) error {
	plugin.lock.Lock()
	plugin.filemapping[strings.ToLower(inputFile.Name())] = inputFile.Name()
	plugin.files = append(plugin.files, inputFile)
	plugin.lock.Unlock()
	return nil
}

func (plugin *fileCollector) LinkWithContext(destination string, context string) {
	log.Printf("Stage: %d, currentFile: %s, destination: %s\n", plugin.currentStage, plugin.fileInProcess,
		destination)
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


func (plugin *fileCollector) Finalize(context FCContext) error {
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
		n, err := dataIn.ReadFrom(f)
		if err != nil {
			log.Printf("Error when reading %s: %s\n", f.Name(), err)
			return err
		}
		log.Printf("Content length read for %s: %d\n", f.Name(), n)

		var dataOut bytes.Buffer
		n2, err := dataOut.Write(dataIn.Bytes())
		if err != nil {
			log.Printf("Error when writing %s: %s\n", f.Name(), err)
			return err
		}
		log.Printf("Content length for %s: %d\n", f.Name(), n2)
		addReferences(dataOut, backlinks)
		outputFile := context.CreateFileFromData(f.Path(), dataOut.Bytes())
		outputFile.Meta = f.Meta
		context.DispatchFile(outputFile)
	}

	for _, fn := range plugin.unknownFiles {
		linkFn := fn + ".html"
		fn += ".md"
		backlinks, exists := plugin.backlinks[linkFn]
		if !exists {
			log.Printf("Unexpected unknown file with no backlinks: %s\n", fn)
			continue
		}
		log.Printf("For %s there are %d backlinks\n", linkFn, len(backlinks))
		var dataOut bytes.Buffer
		addReferences(dataOut, backlinks)
		log.Printf("Contents:\n%s\n", dataOut.String())
		outputFile := context.CreateFileFromData(fn, dataOut.Bytes())
		context.DispatchFile(outputFile)
	}

	plugin.currentStage = finalProcessing
	return nil
}

func addReferences(dataOut bytes.Buffer, backlinks []backlink) {
	dataOut.WriteString(`

# Related pages

`)
	for _, backlink := range backlinks {
		otherPage := strings.TrimRight(backlink.origin, path.Ext(backlink.origin))
		dataOut.WriteString(fmt.Sprintf("* [[%s]]: %s\n", otherPage, backlink.context))
	}
}

func (plugin *fileCollector) Normalize(linkText string) string {
	fn, exists := plugin.filemapping[strings.ToLower(linkText) + ".md"]
	if !exists {
		if plugin.currentStage == linkCollectionStage {
			plugin.unknownFiles = append(plugin.unknownFiles, linkText)
		}
		fn = linkText + ".html"
	} else {
		fn = strings.TrimRight(fn, path.Ext(fn)) + ".html"
	}
	return fn
}
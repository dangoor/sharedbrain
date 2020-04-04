package main

import (
	"flag"
	wikilinks "github.com/dangoor/goldmark-wikilinks"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"log"
	"sharedbrain/filecollector"

	"github.com/FooSoft/goldsmith"
	"github.com/FooSoft/goldsmith-components/devserver"
	"github.com/FooSoft/goldsmith-components/filters/condition"
	"github.com/FooSoft/goldsmith-components/plugins/frontmatter"
	"github.com/FooSoft/goldsmith-components/plugins/layout"
	"github.com/FooSoft/goldsmith-components/plugins/markdown"
	"github.com/FooSoft/goldsmith-components/plugins/minify"
)

type builder struct {
	dist bool
}

func (b *builder) Build(srcDir, dstDir, cacheDir string) {
	fc := filecollector.New()
	wl := wikilinks.NewWikilinksParser().WithNormalizer(fc)
 	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Typographer),
		goldmark.WithParserOptions(parser.WithAutoHeadingID(),
			parser.WithInlineParsers(util.Prioritized(wl, 102)),
		),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	errs := goldsmith.
		Begin(srcDir).                     // read files from srcDir
		Chain(fc).
		Chain(frontmatter.New()).          // extract frontmatter and store it as metadata
		Chain(markdown.New().			   // convert *.md files to *.html files
			WithGoldmark(md)).
		Chain(layout.New().				   // apply *.gohtml templates to *.html files
			DefaultLayout("page")).
		FilterPush(condition.New(b.dist)). // push a dist-only conditional filter onto the stack
		Chain(minify.New()).               // minify *.html, *.css, *.js, etc. files
		FilterPop().                       // pop off the last filter pushed onto the stack
		End(dstDir)                        // write files to dstDir

	for _, err := range errs {
		log.Print(err)
	}
}

func main() {
	port := flag.Int("port", 8080, "server port")
	dist := flag.Bool("dist", false, "final dist mode")
	content := flag.String("content", "content", "Source directory")
	flag.Parse()

	devserver.DevServe(&builder{*dist}, *port, *content, "build", "cache")
}
package main

import (
	"flag"
	"log"

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
	errs := goldsmith.
		Begin(srcDir).                     // read files from srcDir
		Cache(cacheDir).
		Chain(frontmatter.New()).          // extract frontmatter and store it as metadata
		Chain(markdown.New()).             // convert *.md files to *.html files
		Chain(layout.New()).               // apply *.gohtml templates to *.html files
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
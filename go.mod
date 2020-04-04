module sharedbrain

go 1.13

require (
	github.com/FooSoft/goldsmith v0.0.0-20200102021543-f410ad444d75
	github.com/FooSoft/goldsmith-components v0.0.0-00010101000000-000000000000
	github.com/dangoor/goldmark-wikilinks v0.0.0-20200328202359-1f43a0db702d
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/tdewolff/test v1.0.6 // indirect
	github.com/yuin/goldmark v1.1.25
)

replace github.com/FooSoft/goldsmith-components => github.com/dangoor/goldsmith-components v0.0.0-20200328165034-2c3e6e248684

replace github.com/dangoor/goldmark-wikilinks => ../goldmark-wikilinks

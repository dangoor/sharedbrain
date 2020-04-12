package filecollector

import (
	"bytes"
	"github.com/FooSoft/goldsmith"
	"github.com/stretchr/testify/require"
	"testing"
)

var firstFile = `

This file references [[Other File]].

It also references an [[Unknown File]].

`

var otherFile = `

Other file references [[First File]].
`

type FakeContext struct {
	goldsmith.Context
	dispatched map[string]*goldsmith.File
}

func (context *FakeContext) DispatchFile(file *goldsmith.File) {
	context.dispatched[file.Name()] = file
}

func TestBacklinks(t *testing.T) {
	require := require.New(t)

	context := FakeContext{dispatched: make(map[string]*goldsmith.File)}
	f1 := context.CreateFileFromData("First File.md", []byte(firstFile))
	f2 := context.CreateFileFromData("Other File.md", []byte(otherFile))
	plugin := New()
	plugin.Process(&context, f1)
	plugin.Process(&context, f2)
	require.Equal(2, len(plugin.files))

	plugin.Finalize(&context)
	require.Equal(3, len(plugin.backlinks))
	require.Equal(1, len(plugin.unknownFiles))

	require.Equal(3, len(context.dispatched))
	_, exists := context.dispatched["Unknown File.md"]
	require.True(exists, "Unknown file should have been found")

	f, exists := context.dispatched["First File.md"]
	require.True(exists, "First should have been dispatched")
	var dataIn bytes.Buffer
	_, err := dataIn.ReadFrom(f)
	require.Nil(err)
	dataString := dataIn.String()
	require.Contains(dataString, "This file references")
}
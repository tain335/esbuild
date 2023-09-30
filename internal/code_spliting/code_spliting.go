package codespliting

import (
	"fmt"
)

type ChunkKind uint8

const (
	JSChunk ChunkKind = iota
	CSSChunk
)

type ChunkNode struct {
	Name         string
	IsEntryPoint bool
	Kind         ChunkKind
	Async        bool
	Content      []byte
	SourceMap    []byte
	Dependencies []string
	SourceIndex  uint32
}

func FindAllChunkDependecies(chunks []ChunkNode, chunk string) []*ChunkNode {
	var chunkMap = make(map[string]*ChunkNode)
	for i := range chunks {
		chunkMap[chunks[i].Name] = &chunks[i]
	}
	var dependencies = make([]*ChunkNode, 0)
	var chunkNode = chunkMap[chunk]

	if chunkNode == nil {
		panic(fmt.Errorf("not found chunk name: %s", chunk))
	}

	dependencies = append(dependencies, chunkNode)
	if chunkNode.Dependencies != nil {
		for i := range chunkNode.Dependencies {
			dependencies = append(dependencies, FindAllChunkDependecies(chunks, chunkNode.Dependencies[i])...)
		}
	}

	return dependencies
}

package utils

import (
	"path/filepath"
	"strings"
)

type Node struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Children []*Node `json:"children,omitempty"`
}

// ParserTreeFolder parses sorted `find /workspace` output into a Node tree.
// Each line is an absolute path; a path is a directory if any other path starts with it + "/".
func ParserTreeFolder(input string) *Node {
	root := &Node{Name: ".", Type: "dir", Children: []*Node{}}

	lines := strings.Split(strings.TrimSpace(input), "\n")
	var paths []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "/workspace" {
			continue
		}
		paths = append(paths, line)
	}
	if len(paths) == 0 {
		return root
	}

	// First pass: identify directories (any path that has children)
	dirSet := make(map[string]bool)
	for _, p1 := range paths {
		prefix := p1 + "/"
		for _, p2 := range paths {
			if strings.HasPrefix(p2, prefix) {
				dirSet[p1] = true
				break
			}
		}
	}

	// Second pass: build tree (paths are sorted, so parents always come before children)
	nodes := map[string]*Node{"/workspace": root}
	for _, p := range paths {
		name := filepath.Base(p)
		isDir := dirSet[p]

		node := &Node{Name: name, Type: "file"}
		if isDir {
			node.Type = "dir"
			node.Children = []*Node{}
		}

		parent := filepath.Dir(p)
		parentNode, ok := nodes[parent]
		if !ok {
			continue
		}
		parentNode.Children = append(parentNode.Children, node)
		nodes[p] = node
	}

	return root
}

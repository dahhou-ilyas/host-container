package utils


import (
	"regexp"
	"strings"
)

type Node struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Children []*Node `json:"children,omitempty"`
}

func ParserTreeFolder(input string) *Node {
	if strings.TrimSpace(input) == "" {
		return &Node{Name: ".", Type: "dir", Children: []*Node{}}
	}

	stack := []*Node{}
	lines := strings.Split(input, "\n")
	directorRe := regexp.MustCompile(`^\d+ director`)

	for index, line := range lines {
		if strings.TrimSpace(line) == "" || directorRe.MatchString(line) {
			continue
		}

		if line == "." || index == 0 {
			root := &Node{Name: ".", Type: "dir", Children: []*Node{}}
			stack = append(stack, root)
			continue
		}

		var connecteurIndex int
		hasConnector := false
		if idx := strings.Index(line, "├──"); idx != -1 {
			connecteurIndex = idx
			hasConnector = true
		} else if idx := strings.Index(line, "└──"); idx != -1 {
			connecteurIndex = idx
			hasConnector = true
		}else if idx := strings.Index(line, "|--"); idx != -1 {
			connecteurIndex = idx
			hasConnector = true
		}else if idx := strings.Index(line, "`--"); idx != -1 {
			connecteurIndex = idx
			hasConnector = true
		}
		

		if !hasConnector || len(stack) == 0 {
			continue
		}

		depthString := line[:connecteurIndex]
		depth := countBarPlus3Spaces(depthString)
		if depth != 0 {
			// "│   " fait 4 bytes en UTF-8 pour │ (3) + 3 espaces = 6 bytes
			// Mais on travaille en runes pour être safe
			runes := []rune(depthString)
			if len(runes) > 4 {
				depthString = string(runes[4:])
			} else {
				depthString = ""
			}
		}
		depth += countFourSpaces(depthString)

		// +4 pour sauter "├── " (le connecteur + l'espace)
		runesLine := []rune(line)
		// "├──" = 3 runes, + 1 espace = index +4 en runes
		connecteurRuneIndex := len([]rune(line[:connecteurIndex]))
		nodeString := string(runesLine[connecteurRuneIndex+4:])

		isFolder := strings.Contains(nodeString, "/")
		nameNode := nodeString
		if isFolder {
			nameNode = nodeString[:len(nodeString)-1]
		}

		node := &Node{
			Name: nameNode,
			Type: "file",
		}
		if isFolder {
			node.Type = "dir"
			node.Children = []*Node{}
		}

		for len(stack) > depth+1 {
			stack = stack[:len(stack)-1]
		}

		parent := stack[len(stack)-1]
		parent.Children = append(parent.Children, node)

		if isFolder {
			stack = append(stack, node)
		}
	}

	if len(stack) == 0 {
		return &Node{Name: ".", Type: "dir", Children: []*Node{}}
	}
	return stack[0]
}

var fourSpacesRe = regexp.MustCompile(`    `)
var barPlus3SpacesRe = regexp.MustCompile(`[│|]   `)

func countFourSpaces(s string) int {
	return len(fourSpacesRe.FindAllString(s, -1))
}

func countBarPlus3Spaces(s string) int {
	return len(barPlus3SpacesRe.FindAllString(s, -1))
}
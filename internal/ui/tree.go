package ui

import (
	"path"
	"sort"
	"strings"

	"github.com/janpuc/koment/internal/anchor"
)

type treeNode struct {
	Name  string
	Path  string
	Dirs  []treeNode
	Files []entry
	Count int
	Worst anchor.Status
	Open  bool
}

var statusSeverity = map[anchor.Status]int{
	anchor.StatusOK:        0,
	anchor.StatusAmbiguous: 2,
	anchor.StatusDrifted:   3,
	anchor.StatusOrphaned:  4,
}

func buildTree(files []entry, current string) ([]treeNode, []entry) {
	root := &treeNode{}
	for _, file := range files {
		directory := findOrCreateDirectory(root, path.Dir(file.Path))
		directory.Files = append(directory.Files, file)
	}

	collapse(root)
	summariseTree(root, current)
	return root.Dirs, root.Files
}

func findOrCreateDirectory(root *treeNode, directory string) *treeNode {
	if directory == "." || directory == "" {
		return root
	}

	at := root
	for _, segment := range strings.Split(directory, "/") {
		next := (*treeNode)(nil)
		for i := range at.Dirs {
			if at.Dirs[i].Name == segment {
				next = &at.Dirs[i]
				break
			}
		}
		if next == nil {
			at.Dirs = append(at.Dirs, treeNode{Name: segment, Path: path.Join(at.Path, segment)})
			next = &at.Dirs[len(at.Dirs)-1]
		}
		at = next
	}
	return at
}

func collapse(at *treeNode) {
	for i := range at.Dirs {
		collapse(&at.Dirs[i])
	}

	if at.Path == "" {
		return
	}
	for len(at.Files) == 0 && len(at.Dirs) == 1 {
		only := at.Dirs[0]
		at.Name = at.Name + "/" + only.Name
		at.Path = only.Path
		at.Files = only.Files
		at.Dirs = only.Dirs
	}
}

func summariseTree(at *treeNode, current string) {
	sort.Slice(at.Dirs, func(i, j int) bool { return at.Dirs[i].Name < at.Dirs[j].Name })
	sort.Slice(at.Files, func(i, j int) bool { return at.Files[i].Name < at.Files[j].Name })

	for _, file := range at.Files {
		at.Count += file.Count
		if statusSeverity[file.Worst] > statusSeverity[at.Worst] {
			at.Worst = file.Worst
		}
		if file.Path == current {
			at.Open = true
		}
	}

	for i := range at.Dirs {
		child := &at.Dirs[i]
		summariseTree(child, current)

		at.Count += child.Count
		if statusSeverity[child.Worst] > statusSeverity[at.Worst] {
			at.Worst = child.Worst
		}
		if child.Open {
			at.Open = true
		}
	}
}

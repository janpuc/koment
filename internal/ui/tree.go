package ui

import (
	"path"
	"sort"
	"strings"

	"github.com/janpuc/koment/internal/anchor"
)

// node is a directory in the file tree.
type node struct {
	Name  string
	Path  string
	Dirs  []node
	Files []entry
	Count int
	Worst anchor.Status
	Open  bool
}

// severity orders statuses so a directory can advertise the worst one beneath it.
var severity = map[anchor.Status]int{
	anchor.StatusOK:       0,
	anchor.StatusMoved:    1,
	anchor.StatusDrifted:  2,
	anchor.StatusOrphaned: 3,
}

// treeOf builds the directory tree, returning the nested directories and the
// files that sit at the repository root.
func treeOf(files []entry, current string) ([]node, []entry) {
	root := &node{}
	for _, file := range files {
		directory := directoryOf(root, path.Dir(file.Path))
		directory.Files = append(directory.Files, file)
	}

	collapse(root)
	summarise(root, current)
	return root.Dirs, root.Files
}

// directoryOf finds or creates the node for a slash-separated directory,
// treating "." — what path.Dir gives a bare filename — as the root itself.
func directoryOf(root *node, directory string) *node {
	if directory == "." || directory == "" {
		return root
	}

	at := root
	for _, segment := range strings.Split(directory, "/") {
		next := (*node)(nil)
		for i := range at.Dirs {
			if at.Dirs[i].Name == segment {
				next = &at.Dirs[i]
				break
			}
		}
		if next == nil {
			at.Dirs = append(at.Dirs, node{Name: segment, Path: path.Join(at.Path, segment)})
			next = &at.Dirs[len(at.Dirs)-1]
		}
		at = next
	}
	return at
}

func collapse(at *node) {
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

// summarise rolls counts and the worst status up the tree, and opens the
// directories on the path to the file being shown.
func summarise(at *node, current string) {
	sort.Slice(at.Dirs, func(i, j int) bool { return at.Dirs[i].Name < at.Dirs[j].Name })
	sort.Slice(at.Files, func(i, j int) bool { return at.Files[i].Name < at.Files[j].Name })

	for _, file := range at.Files {
		at.Count += file.Count
		if severity[file.Worst] > severity[at.Worst] {
			at.Worst = file.Worst
		}
		if file.Path == current {
			at.Open = true
		}
	}

	for i := range at.Dirs {
		child := &at.Dirs[i]
		summarise(child, current)

		at.Count += child.Count
		if severity[child.Worst] > severity[at.Worst] {
			at.Worst = child.Worst
		}
		if child.Open {
			at.Open = true
		}
	}
}

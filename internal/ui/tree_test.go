package ui

import (
	"strings"
	"testing"

	"github.com/koment-dev/koment/internal/anchor"
)

func entriesFor(paths ...string) []entry {
	listed := make([]entry, 0, len(paths))
	for _, path := range paths {
		listed = append(listed, entry{Path: path, Name: baseName(path), Count: 1, Worst: anchor.StatusOK})
	}
	return listed
}

func find(nodes []treeNode, name string) (treeNode, bool) {
	for _, at := range nodes {
		if at.Name == name {
			return at, true
		}
	}
	return treeNode{}, false
}

// The rail used to render one flat row per directory path, so a Go layout came
// out as a list of unrelated strings with no structure between them.
func TestTheTreeIsNestedNotFlat(t *testing.T) {
	tree, _ := buildTree(entriesFor(
		"internal/ui/view.go",
		"internal/ui/tree.go",
		"internal/store/record.go",
	), "")

	if len(tree) != 1 {
		t.Fatalf("want one top-level directory, got %d: %+v", len(tree), tree)
	}
	internal := tree[0]
	if internal.Name != "internal" {
		t.Fatalf("want internal at the top, got %q", internal.Name)
	}
	if len(internal.Dirs) != 2 {
		t.Fatalf("want ui and store nested under internal, got %+v", internal.Dirs)
	}
	if _, ok := find(internal.Dirs, "ui"); !ok {
		t.Error("ui should be a child of internal, not a sibling")
	}
}

// A chain of directories with nothing in it but the next directory is
// scaffolding, and rendering each as its own row wastes the rail's width.
func TestASingleChildChainCollapsesIntoOneRow(t *testing.T) {
	tree, _ := buildTree(entriesFor("internal/ui/assets/style.css"), "")

	if len(tree) != 1 {
		t.Fatalf("want one row, got %d", len(tree))
	}
	if tree[0].Name != "internal/ui/assets" {
		t.Errorf("want the chain folded into one row, got %q", tree[0].Name)
	}
	if tree[0].Path != "internal/ui/assets" {
		t.Errorf("the folded row must keep the real path, got %q", tree[0].Path)
	}
	if len(tree[0].Files) != 1 {
		t.Errorf("the file should hang off the folded row, got %+v", tree[0].Files)
	}
}

func TestADirectoryWithItsOwnFilesDoesNotCollapse(t *testing.T) {
	tree, _ := buildTree(entriesFor("internal/go.mod", "internal/ui/view.go"), "")

	if tree[0].Name != "internal" {
		t.Fatalf("internal has a file of its own and must stay its own row, got %q", tree[0].Name)
	}
	if len(tree[0].Dirs) != 1 || tree[0].Dirs[0].Name != "ui" {
		t.Errorf("ui should still nest under internal, got %+v", tree[0].Dirs)
	}
}

func TestFilesAtTheRootAreListedNotBuriedInAFakeDirectory(t *testing.T) {
	tree, loose := buildTree(entriesFor("README.md", "internal/ui/view.go"), "")

	if _, ok := find(tree, "internal/ui"); !ok {
		t.Errorf("internal/ui is missing from the tree, got %+v", tree)
	}
	for _, at := range tree {
		if at.Name == "." || at.Name == "" {
			t.Errorf("a root file was filed under an empty directory: %+v", at)
		}
	}
	if len(loose) != 1 || loose[0].Path != "README.md" {
		t.Errorf("a root file must still be listed, got %+v", loose)
	}
}

// A directory advertises the worst status beneath it, so a drift buried three
// levels down is visible without opening anything.
func TestADirectoryCarriesTheWorstStatusBeneathIt(t *testing.T) {
	listed := entriesFor("internal/ui/view.go", "internal/ui/tree.go")
	listed[1].Worst = anchor.StatusDrifted

	tree, _ := buildTree(listed, "")

	if tree[0].Worst != anchor.StatusDrifted {
		t.Errorf("want drifted to surface at internal/ui, got %q", tree[0].Worst)
	}
	if tree[0].Count != 2 {
		t.Errorf("want the counts rolled up, got %d", tree[0].Count)
	}
}

// Everything closed is what keeps a large repository readable; the path to
// what you are reading is the exception.
func TestOnlyThePathToTheCurrentFileStartsOpen(t *testing.T) {
	tree, _ := buildTree(entriesFor(
		"internal/ui/view.go",
		"internal/store/record.go",
		"docs/publishing.md",
	), "internal/ui/view.go")

	internal, _ := find(tree, "internal")
	docs, _ := find(tree, "docs")

	if !internal.Open {
		t.Error("the directory holding the current file must start open")
	}
	if docs.Open {
		t.Error("an unrelated directory must start closed")
	}

	ui, ok := find(internal.Dirs, "ui")
	if !ok || !ui.Open {
		t.Error("every ancestor of the current file must start open")
	}
	if store, ok := find(internal.Dirs, "store"); ok && store.Open {
		t.Error("a sibling of the current file's directory must start closed")
	}
}

func TestTheTreeSurvivesADeepPath(t *testing.T) {
	tree, _ := buildTree(entriesFor("a/b/c/d/e/f/g.go"), "")

	if len(tree) != 1 || !strings.HasPrefix(tree[0].Name, "a/b") {
		t.Errorf("a deep single-child path should fold into one row, got %+v", tree)
	}
}

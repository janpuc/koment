package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpuc/koment/internal/policy"
	"github.com/janpuc/koment/internal/repository"
	"github.com/janpuc/koment/internal/store"
)

func TestConvertCommentWritesRecordBeforeRemovingSourceProse(t *testing.T) {
	root := t.TempDir()
	source := "package sample\n\nfunc run() {\n\t// Retry because the peer closes idle connections.\n\tretryOnce()\n}\n"
	file := filepath.Join(root, "sample.go")
	if err := os.WriteFile(file, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(repository.Repository{ID: "sample", Root: root})
	author := store.Author{Name: "Test Agent", Kind: store.AuthorAgent, Source: store.FromExplicit}
	mutation, err := service.ConvertComment(ConvertCommentInput{
		File: "sample.go", Comment: "// Retry because the peer closes idle connections.", Author: author,
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "Retry because") {
		t.Fatal("source comment remains after conversion")
	}
	if mutation.Record.Spec.Body != "Retry because the peer closes idle connections." || mutation.Record.Spec.Author.Kind != store.AuthorAgent {
		t.Fatalf("record = %#v", mutation.Record)
	}
}

func TestAcknowledgementIsRequiredAndPassesPolicyCheck(t *testing.T) {
	root := t.TempDir()
	source := "package sample\n\nfunc run() {\n\t// Generator contract.\n\trunGenerator()\n}\n"
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := policy.Save(root, policy.Default()); err != nil {
		t.Fatal(err)
	}
	service := NewService(repository.Repository{ID: "sample", Root: root})
	author := store.Author{Name: "Test Human", Kind: store.AuthorHuman, Source: store.FromExplicit}
	_, err := service.AcknowledgeComment(AcknowledgeCommentInput{
		File: "sample.go", Comment: "// Generator contract.", Body: "The generator reads this exact comment.", Author: author,
	})
	if err == nil {
		t.Fatal("missing acknowledgement accepted")
	}
	_, err = service.AcknowledgeComment(AcknowledgeCommentInput{
		File: "sample.go", Comment: "// Generator contract.", Body: "The generator reads this exact comment.",
		Author: author, Acknowledged: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	violations, err := service.CheckComments(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("violations = %#v", violations)
	}
}

package epubtree

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tree := New()
	if tree == nil {
		t.Fatal("New() returned nil")
	}
	root := tree.Root()
	if root == nil {
		t.Fatal("Root() returned nil")
	}
	if root.value != "." {
		t.Errorf("root value = %q, want %q", root.value, ".")
	}
	if root.ChildCount() != 0 {
		t.Errorf("root should have 0 children, got %d", root.ChildCount())
	}
}

func TestAdd_SingleFile(t *testing.T) {
	tree := New()
	tree.Add("OEBPS/metadata.xml")

	root := tree.Root()
	if root.ChildCount() != 1 {
		t.Fatalf("root child count = %d, want 1", root.ChildCount())
	}

	first := root.FirstChild()
	if first.value != "OEBPS" {
		t.Errorf("first child value = %q, want %q", first.value, "OEBPS")
	}
	if first.ChildCount() != 1 {
		t.Fatalf("OEBPS child count = %d, want 1", first.ChildCount())
	}

	second := first.FirstChild()
	if second.value != "metadata.xml" {
		t.Errorf("second child value = %q, want %q", second.value, "metadata.xml")
	}
}

func TestAdd_Duplicate(t *testing.T) {
	tree := New()
	tree.Add("a/b/c.xml")
	tree.Add("a/b/c.xml")

	root := tree.Root()
	if root.ChildCount() != 1 {
		t.Fatalf("root child count = %d, want 1", root.ChildCount())
	}
	// Deepest node should have exactly 1 child (the file)
	aNode := root.FirstChild()
	bNode := aNode.FirstChild()
	if bNode.ChildCount() != 1 {
		t.Errorf("b children after duplicate = %d, want 1", bNode.ChildCount())
	}
}

func TestWriteString(t *testing.T) {
	tree := New()
	tree.Add("OEBPS/metadata.xml")
	tree.Add("OEBPS/Images/cover.jpeg")
	tree.Add("OEBPS/Text/page.xhtml")

	output := tree.Root().WriteString("")
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) < 4 {
		t.Fatalf("WriteString produced %d lines, want at least 4", len(lines))
	}

	// Check tree structure in output
	if !strings.Contains(output, "OEBPS") {
		t.Errorf("WriteString output should contain %q", "OEBPS")
	}
	if !strings.Contains(output, "metadata.xml") {
		t.Errorf("WriteString output should contain %q", "metadata.xml")
	}
	if !strings.Contains(output, "Images") {
		t.Errorf("WriteString output should contain %q", "Images")
	}
	if !strings.Contains(output, "cover.jpeg") {
		t.Errorf("WriteString output should contain %q", "cover.jpeg")
	}
	if !strings.Contains(output, "Text") {
		t.Errorf("WriteString output should contain %q", "Text")
	}
	if !strings.Contains(output, "page.xhtml") {
		t.Errorf("WriteString output should contain %q", "page.xhtml")
	}
}

func TestWriteString_RootNoIndent(t *testing.T) {
	tree := New()
	tree.Add("file.txt")

	output := tree.Root().WriteString("")
	// Root with empty indent should produce no leading "- ." line
	if strings.HasPrefix(output, "- .") {
		t.Errorf("WriteString with empty indent should not include root line")
	}
}

func TestFirstChild_PanicsOnEmpty(t *testing.T) {
	tree := New()
	root := tree.Root()
	// FirstChild on empty root should panic (accesses index 0 of empty slice)
	defer func() {
		if r := recover(); r == nil {
			t.Error("FirstChild() on empty node should panic")
		}
	}()
	_ = root.FirstChild()
}

func TestChildCount(t *testing.T) {
	tree := New()
	tree.Add("a/1.xml")
	tree.Add("a/2.xml")
	tree.Add("b/3.xml")

	root := tree.Root()
	if root.ChildCount() != 2 {
		t.Errorf("root child count = %d, want 2", root.ChildCount())
	}

	// Find the node with 2 children
	var twoChildNode *Node
	for i := range root.ChildCount() {
		if root.children[i].ChildCount() == 2 {
			twoChildNode = root.children[i]
			break
		}
	}
	if twoChildNode == nil {
		t.Fatal("expected a node with 2 children, found none")
	}
}

func TestBuildMinimalEPUBTree(t *testing.T) {
	tree := New()
	tree.Add("OEBPS/metadata.xml")
	tree.Add("OEBPS/manifest.xml")
	tree.Add("OEBPS/spine.xml")
	tree.Add("OEBPS/Text/page1.xhtml")
	tree.Add("OEBPS/Images/cover.jpeg")

	root := tree.Root()
	if root.ChildCount() != 1 {
		t.Fatalf("root should have 1 child (OEBPS), got %d", root.ChildCount())
	}

	oebps := root.FirstChild()
	if oebps.value != "OEBPS" {
		t.Errorf("first child = %q, want %q", oebps.value, "OEBPS")
	}

	// OEBPS should have children: metadata.xml, manifest.xml, spine.xml, Text, Images
	if oebps.ChildCount() != 5 {
		t.Fatalf("OEBPS should have 5 children, got %d", oebps.ChildCount())
	}

	// Verify WriteString produces expected output
	output := tree.Root().WriteString("")
	if !strings.Contains(output, "metadata.xml") {
		t.Errorf("output should contain metadata.xml")
	}
	if !strings.Contains(output, "manifest.xml") {
		t.Errorf("output should contain manifest.xml")
	}
	if !strings.Contains(output, "spine.xml") {
		t.Errorf("output should contain spine.xml")
	}
	if !strings.Contains(output, "Text") {
		t.Errorf("output should contain Text")
	}
	if !strings.Contains(output, "Images") {
		t.Errorf("output should contain Images")
	}
}

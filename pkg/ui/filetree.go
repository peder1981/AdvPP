package ui

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type FileTree struct {
	list       *widget.List
	files      []string
	current    string
	currentDir string
	onSelect   func(string)
}

func NewFileTree() *FileTree {
	ft := &FileTree{
		files:      make([]string, 0),
		current:    "",
		currentDir: ".",
	}

	list := widget.NewList(
		func() int {
			return len(ft.files)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id < len(ft.files) {
				item.(*widget.Label).SetText(ft.files[id])
			}
		},
	)

	ft.list = list
	ft.loadDirectory(".")

	return ft
}

func (t *FileTree) GetWidget() fyne.CanvasObject {
	return container.NewBorder(nil, nil, nil, nil, t.list)
}

func (t *FileTree) SetCurrent(path string) {
	t.current = path
}

func (t *FileTree) GetCurrent() string {
	return t.current
}

func (t *FileTree) SetFiles(files []string) {
	t.files = files
	t.list.Refresh()
}

func (t *FileTree) SetOnSelect(callback func(string)) {
	t.onSelect = callback
}

func (t *FileTree) loadDirectory(dir string) error {
	t.currentDir = dir

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	t.files = make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name = "[" + name + "]"
		} else if strings.HasSuffix(strings.ToLower(name), ".prw") ||
			strings.HasSuffix(strings.ToLower(name), ".tlpp") ||
			strings.HasSuffix(strings.ToLower(name), ".prg") {
			// Highlight source files
			name = "*" + name
		}
		t.files = append(t.files, name)
	}

	t.list.Refresh()
	return nil
}

func (t *FileTree) Refresh() {
	t.loadDirectory(t.currentDir)
}

func (t *FileTree) GetSelectedFile() string {
	if t.current == "" {
		return ""
	}

	// Remove markers
	filename := strings.TrimPrefix(t.current, "*")
	filename = strings.TrimPrefix(filename, "[")
	filename = strings.TrimSuffix(filename, "]")

	return filepath.Join(t.currentDir, filename)
}

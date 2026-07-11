package ui

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

// CurrentDirLocation returns the process's current working directory as a
// fyne.ListableURI, suitable for FileDialog.SetLocation. Fyne's file
// dialogs default to the OS home directory otherwise, which is rarely
// where the file the user wants actually is — advplc/adveditor/advpp-ide
// all operate on the directory they were launched from (source files,
// ./advpp.db, etc.), so dialogs should start there too.
func CurrentDirLocation() fyne.ListableURI {
	wd, err := os.Getwd()
	if err != nil {
		return nil
	}
	uri, err := storage.ListerForURI(storage.NewFileURI(wd))
	if err != nil {
		return nil
	}
	return uri
}

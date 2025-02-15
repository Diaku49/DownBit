package main

import (
	"context"
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"
)

type ProgressInfo struct {
	bar        *widget.ProgressBar
	total      int64
	downloaded int64
}

type FileItem struct {
	FileNameLabel     *widget.Label
	Bar               *widget.ProgressBar
	ProgressContainer *fyne.Container
	ProgressSpeed     *widget.Label
	PauseButton       *widget.Button
	ResumeButton      *widget.Button
	CancelButton      *widget.Button
	ButtonContainer   *fyne.Container
	DownloadContainer *fyne.Container
	ID                string
	Ctx               context.Context
	CtxP              context.Context
}

type Download struct {
	ID         string  `json:"id"`
	FileName   string  `json:"file_name"`
	URL        string  `json:"url"`
	FilePath   string  `json:"file_path"`
	TotalSize  int64   `json:"total_size"`
	Downloaded int64   `json:"downloaded"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
	Chunks     []Chunk `json:"chunks"`
}

type Chunk struct {
	End           int64  `json:"end"`
	CurrentOffset int64  `json:"current_offset"`
	Status        string `json:"status"`
}

type ResumeFunc func(myapp *MyApp, url string)

func makeFileItem(myapp *MyApp, info FileInfo) (*FileItem, chan context.CancelFunc, chan context.CancelFunc) {
	var downloadContainer *fyne.Container
	var parent = myapp.CurrentDownloadsContainer
	var buttonsContainer *fyne.Container
	var fileItem *FileItem
	cancelChannel := make(chan context.CancelFunc, 1)
	pauseChannel := make(chan context.CancelFunc, 1)
	pauseFlag := false

	// File name label
	fileNameLabel := widget.NewLabelWithStyle(
		info.FileName,
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)

	// ProgressBar
	progressBar := widget.NewProgressBar()
	progressBar.SetValue(0.0)
	progressSpeed := widget.NewLabel("Speed: 0Mg")
	progressPercent := widget.NewLabel(fmt.Sprintf("(Size:%0.f)", info.FileSize))
	progressPercent.Alignment = fyne.TextAlignTrailing
	progressSpeed.Alignment = fyne.TextAlignTrailing

	progressContainer := container.NewBorder(
		nil, nil, nil,
		progressSpeed,
		progressPercent,
		progressBar,
	)

	// Pause and Cancel buttons
	pauseButton := widget.NewButtonWithIcon(
		"Pause", theme.MediaPauseIcon(),
		nil,
	)

	resumeButton := widget.NewButtonWithIcon(
		"Resume", theme.MediaPlayIcon(),
		nil,
	)
	resumeButton.Importance = widget.HighImportance
	resumeButton.Hide()

	cancelButton := widget.NewButtonWithIcon(
		"Cancel", theme.ContentClearIcon(),
		func() {
			method := <-cancelChannel
			method()
			parent.Remove(downloadContainer)
			parent.Refresh()
			if pauseFlag {
				if err := os.Remove(info.FilePath); err != nil {
					fmt.Printf("Error deleting file: %v\n", err)
				}
			}
		},
	)

	// Toggle visibility for pause and resume buttons
	pauseButton.OnTapped = func() {
		method := <-pauseChannel
		method()
		pauseButton.Hide()
		resumeButton.Show()
		pauseFlag = true
	}

	resumeButton.OnTapped = func() {
		resumeButton.Hide()
		pauseButton.Show()
		<-cancelChannel
		pauseFlag = false
		go ResumeDownload(myapp, fileItem, cancelChannel, pauseChannel)
	}

	buttonsContainer = container.NewHBox(pauseButton, resumeButton, cancelButton)

	// Final container for the download item
	downloadContainer = container.NewVBox(
		container.NewBorder(nil, nil, fileNameLabel, nil),
		progressContainer,
		buttonsContainer,
	)
	parent.Add(downloadContainer)

	id := uuid.New()

	fileItem = &FileItem{
		ID:                id.String(),
		FileNameLabel:     fileNameLabel,
		Bar:               progressBar,
		ProgressContainer: progressContainer,
		ProgressSpeed:     progressSpeed,
		PauseButton:       pauseButton,
		ResumeButton:      resumeButton,
		CancelButton:      cancelButton,
		ButtonContainer:   buttonsContainer,
		DownloadContainer: downloadContainer,
	}

	return fileItem, cancelChannel, pauseChannel
}

//--------------- extra functions

func downloadFinished(fileItem *FileItem, mainDContainer *fyne.Container) {
	fileItem.ButtonContainer.RemoveAll()
	doneButton := widget.NewButtonWithIcon("Done", resourceCheckSolidSvg, func() {
		mainDContainer.Remove(fileItem.DownloadContainer)
	})
	fileItem.ButtonContainer.Add(doneButton)
}

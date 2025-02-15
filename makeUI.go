package main

import (
	"context"
	"image/color"
	"net/http"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type MyApp struct {
	App                       fyne.App
	AppContext                context.Context
	Client                    *http.Client
	MainWindow                fyne.Window
	MainContainer             *fyne.Container
	CurrentDownloadsContainer *fyne.Container
	Storage                   *fyne.Storage
	DownloadStateFilePath     string
}

var mainBackgroundColor = color.RGBA{R: 0, G: 0, B: 0, A: 255}
var CDBackgroundColor = color.RGBA{R: 27, G: 42, B: 48, A: 255}
var CDTextColor = color.RGBA{R: 0, G: 191, B: 255, A: 255}

func (myapp *MyApp) makeUI() {
	mainMenu := fyne.NewMainMenu(makeAllMenu(myapp))
	myapp.MainWindow.SetMainMenu(mainMenu)

	// SubContainers
	topContainer := makeTopContainer(myapp)
	currentDownloadContainer := makeCurrentDownloadsContainer(myapp)

	// Main container
	backgroundWindow := canvas.NewRectangle(mainBackgroundColor)
	MainContainer := container.NewVBox(
		topContainer,
		currentDownloadContainer,
	)
	myapp.MainContainer = MainContainer

	// WindowContent
	windowContent := container.NewStack(
		backgroundWindow,
		MainContainer,
	)
	myapp.MainWindow.SetContent(windowContent)
}

func makeAllMenu(myapp *MyApp) (taskMenu, downloadMenu, helpMenu *fyne.Menu) {

	taskMenu = fyne.NewMenu("Task",
		fyne.NewMenuItem("Add new download", func() {}),
	)

	downloadMenu = fyne.NewMenu("Downloads",
		fyne.NewMenuItem("Start all downloads", func() {}),
		fyne.NewMenuItem("Stop all downloads", func() {}),
	)

	helpMenu = fyne.NewMenu("Help",
		fyne.NewMenuItem("Tutorials", func() {}),
		fyne.NewMenuItem("About DownBit", func() {}),
	)

	return taskMenu, downloadMenu, helpMenu
}

func makeTopContainer(myapp *MyApp) *fyne.Container {

	// top Buttons
	topContainer := container.NewHBox(
		&widget.Button{
			Text:       "Add URL",
			Icon:       theme.ContentAddIcon(),
			Importance: widget.HighImportance,
			OnTapped:   AddURLFunc(myapp),
		},
		&widget.Button{
			Text:       "Remove All",
			Icon:       theme.ContentRemoveIcon(),
			Importance: widget.HighImportance,
			OnTapped:   func() {},
		},
	)
	// Wrapper for Buttons
	return container.New(
		layout.NewBorderLayout(topContainer, nil, nil, nil),
		topContainer,
	)
}

func makeCurrentDownloadsContainer(myapp *MyApp) *fyne.Container {
	//Current Download Files
	innerEmptyContainer := container.NewVBox()
	// currentDownloadsTitle := widget.NewLabel("Current Downloads")
	// currentDownloadsTitle.Alignment = fyne.TextAlignCenter

	Title := currentDownloadsTitle()

	scrollableContainer := container.NewVScroll(
		innerEmptyContainer,
	)
	scrollableContainer.SetMinSize(fyne.NewSize(300, 400))

	myapp.CurrentDownloadsContainer = innerEmptyContainer
	return container.NewVBox(
		Title,
		scrollableContainer,
	)
}

func currentDownloadsTitle() *fyne.Container {
	// handling TextTitle
	headerText := canvas.NewText("Current Downloads", CDTextColor)
	headerText.TextSize = 20
	headerText.TextStyle = fyne.TextStyle{Italic: true}

	// Create a background rectangle
	background := canvas.NewRectangle(CDBackgroundColor)
	background.SetMinSize(fyne.NewSize(100, 35))

	// Combine the background and text
	return container.NewStack(
		background,
		container.NewCenter(headerText),
	)
}

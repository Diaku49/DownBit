package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {

	// create an fyne app and window
	myapp := app.NewWithID("com.Bardia49.DownBit")
	window := myapp.NewWindow("DownBit")

	// Ensure database directory
	databasePath, err := getDatabasePath()
	if err != nil {
		log.Fatalf("Failed to get database path: %v", err)
	}
	fmt.Println("Database Path:", databasePath)

	// Create a JSON file in the database directory
	jsonFilePath, err := createJSONFile(databasePath)
	if err != nil {
		log.Fatalf("Failed to create JSON file: %v", err)
	}
	fmt.Println("JSON file created successfully")

	//DownloadDirectory
	DownBitDownloadsDirectory()

	//set Icon
	window.SetIcon(resourceDownBitIconPng)

	// Config http Client ***
	transport := &http.Transport{
		MaxIdleConns:    100,
		MaxConnsPerHost: 10,
		IdleConnTimeout: 30 * time.Second,
	}
	client := &http.Client{Transport: transport}

	// create MyApp
	c := context.Background()
	myApp := MyApp{
		App:                   myapp,
		AppContext:            c,
		MainWindow:            window,
		Client:                client,
		DownloadStateFilePath: jsonFilePath,
	}

	// config the main window
	myApp.SetWindowConfig()
	myApp.makeUI()

	// Show and Run window
	myApp.MainWindow.ShowAndRun()

}

func (app *MyApp) SetWindowConfig() {
	app.MainWindow.Resize(fyne.Size{Height: 500, Width: 600})
	app.MainWindow.CenterOnScreen()
}

func DownBitDownloadsDirectory() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("unable to find home directory: %v", err)
	}

	err = os.MkdirAll(path.Join(homeDir, "Downloads", "/DownBitDownloads"), 0755)
	if err != nil {
		fmt.Println("Error creating directory:", err)
		return fmt.Errorf("unable to make the download directory, err: %v", err)
	}
	return nil
}

// Returns the database path in user-writable directory
func getDatabasePath() (string, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userHome, "DownBit", "database"), nil
}

// Creates a JSON file in the database directory
func createJSONFile(databasePath string) (string, error) {
	os.MkdirAll(databasePath, 0755)

	jsonFilePath := filepath.Join(databasePath, "downloads.json")
	file, err := os.Create(jsonFilePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = file.WriteString(`{}`)
	return jsonFilePath, err
}

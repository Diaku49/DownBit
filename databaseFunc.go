package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func saveDownloadFileInfo(newDownload Download, database string) {
	// Step 1: Open the JSON file
	file, err := os.OpenFile(database, os.O_RDWR, 0644)
	if err != nil {
		fmt.Printf("could not open file: %v", err)
	}
	defer file.Close()

	// Step 2: Read the existing downloads
	var downloads []Download
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&downloads); err != nil {
		// Handle empty or new file scenario
		if err != io.EOF {
			fmt.Printf("could not decode file: %v", err)
		}
		downloads = []Download{} // Initialize an empty slice if file is empty
	}

	// Step 3: Append the new download
	updated := false
	for i, download := range downloads {
		if download.ID == newDownload.ID {
			downloads[i] = newDownload // Update the existing download
			updated = true
			break
		}
	}

	if !updated {
		downloads = append(downloads, newDownload)
	}

	// Step 4: Overwrite the file with the updated data
	// Truncate the file to ensure old data is removed
	if err := file.Truncate(0); err != nil {
		fmt.Printf("could not truncate file: %v", err)
	}
	// Move the file pointer to the beginning
	if _, err := file.Seek(0, 0); err != nil {
		fmt.Printf("could not seek to beginning: %v", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty-print JSON
	if err := encoder.Encode(downloads); err != nil {
		fmt.Printf("could not write updated data: %v", err)
	}
}

func isFileExistByID(database string, id string) (Download, error) {
	downloads, _, err := loadDatabase(database)
	if err != nil {
		return Download{}, fmt.Errorf("database error: %v", err)
	}

	for _, download := range downloads {
		if download.ID == id {
			return download, nil
		}
	}
	return Download{}, fmt.Errorf("file doesnt exist%v", err)
}

// func isFileExist(database, url string) (int, error) {
// 	// 0 for newFile 1 for paused 2 for finished and 3 for db err
// 	downloads, err := loadDatabase(database)
// 	if err != nil {
// 		return 3, err
// 	}

// 	for _, download := range downloads {
// 		if download.URL == url {
// 			if download.Status == "Paused" {
// 				return 1, nil
// 			} else {
// 				return 2, nil
// 			}
// 		}
// 	}
// 	return 0, nil
// }

func loadDatabase(database string) ([]Download, *os.File, error) {
	file, err := os.Open(database)
	if err != nil {
		return nil, file, fmt.Errorf("could not open database file: %v", err)
	}
	defer file.Close()

	var downloads []Download
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&downloads); err != nil {
		return nil, file, fmt.Errorf("could not decode database: %v", err)
	}

	return downloads, file, nil
}

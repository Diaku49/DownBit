package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type FileInfo struct {
	FileName string
	FileSize float64
	Total    int
	FilePath string
	URL      string
}

func AddURLFunc(myapp *MyApp) func() {
	return func() {
		// handling Entry
		urlEntry := widget.NewEntry()
		urlEntry.SetPlaceHolder("Enter URL...")

		// Create a cancellable context for Cancelling
		ctx, cancel := context.WithCancel(myapp.AppContext)
		// Create a cancellable context for Pausing
		ctxP, cancelP := context.WithCancel(myapp.AppContext)

		//show dialog
		dialog.ShowCustomConfirm("Add URL", "OK", "Cancel",
			container.NewVBox(urlEntry),
			func(confirm bool) {
				if confirm {
					fileInfo, err := getFileInfo(myapp.Client, urlEntry.Text)
					if err != nil {
						fmt.Println("got an error: ", err)
						dialog.ShowError(fmt.Errorf("couldnt get fileInfo: %v", err), myapp.MainWindow)
					}
					fileItem, cancelC, pauseC := makeFileItem(myapp, fileInfo)
					cancelC <- cancel
					pauseC <- cancelP

					ConfirmURL(myapp, fileInfo, fileItem, ctx, ctxP, cancelC, pauseC)
					return
				} else {
					return
				}
			}, myapp.MainWindow)
	}
}

func ConfirmURL(myapp *MyApp, fileInfo FileInfo, fileItem *FileItem, ctx, ctxP context.Context, cancelC chan context.CancelFunc, pauseC chan context.CancelFunc) {
	canceled := false //flag for cancelling
	paused := false   // flag for pausing
	cancelCh := make(chan bool, 1)
	pauseCh := make(chan bool, 1)
	total := int64(fileInfo.Total)

	//determine the Requests
	numberOfRequests := 0
	switch {
	case total <= 10*1024*1024: // <= 10 MB
		numberOfRequests = 1
	case total <= 100*1024*1024: // 10 MB - 100 MB
		numberOfRequests = 5
	case total <= 1*1024*1024*1024: // 100 MB - 1 GB
		numberOfRequests = 8
	default: // > 1 GB
		numberOfRequests = 15
	}

	// calculating ChunkSize
	ChunkSize := total / int64(numberOfRequests)

	// Start the download in a goroutine
	go func() {
		// adding the number of request to waitgroup
		var wg sync.WaitGroup

		// Pre allocate slice
		chunkSlice := make([]Chunk, numberOfRequests)

		// Create file download
		outFile, err := os.Create(fileInfo.FilePath)
		if err != nil {
			fmt.Println("Error Creating file:", err)
			dialog.ShowError(fmt.Errorf("error creating file: %v", err), myapp.MainWindow)
			return
		}

		progressInfo := &ProgressInfo{
			bar:        fileItem.Bar,
			downloaded: 0,
			total:      total,
		}

		// Launch goroutines ForLoop
		for i := 0; i < numberOfRequests; i++ {
			start := int64(i) * ChunkSize
			end := start + ChunkSize - 1
			if i == numberOfRequests-1 {
				end = total - 1
			}
			wg.Add(1)
			// Launch a goroutine for each chunk
			go func(ctx, ctxP context.Context, start, end int64, index int, chunk []Chunk) {
				defer wg.Done()
				err := downloadChunk(myapp.Client, ctx, ctxP, fileInfo.URL, start, end, outFile, &progressInfo.downloaded, chunkSlice, i)
				if err != nil && err != context.Canceled {
					fmt.Printf("Error downloading chunk %v\n", err)
					dialog.ShowError(fmt.Errorf("error downloading chunk %d: %v", index, err), myapp.MainWindow)
				}
			}(ctx, ctxP, start, end, i, chunkSlice)
		}

		// Periodically update the progress bar, even if chunks aren't finished
		ticker := time.NewTicker(500 * time.Millisecond) // Update every 500ms
		go func() {
			var speed float64
			var timeDownload int64 = 0
			for range ticker.C {
				select {
				case <-ctxP.Done():
					// // Close the file in the main goroutine
					if err := outFile.Close(); err != nil {
						dialog.ShowError(fmt.Errorf("error closing file: %v", err), myapp.MainWindow)
						fmt.Printf("Error closing file: %v\n", err)
					}
					pauseCh <- true
					cancelCh <- false
					ticker.Stop()
					return
				case <-ctx.Done(): // Stop updates if canceled
					// // Close the file in the main goroutine
					if err := outFile.Close(); err != nil {
						fmt.Printf("Error closing file: %v\n", err)
						dialog.ShowError(fmt.Errorf("error closing file: %v", err), myapp.MainWindow)
					}
					cancelCh <- true
					pauseCh <- false
					ticker.Stop()
					return
				default:
					downloaded := atomic.LoadInt64(&progressInfo.downloaded)
					// Calculate progress based on atomic downloaded value
					value := float64(downloaded) / float64(total)

					// Update the progress bar
					fileItem.Bar.SetValue(value)
					// Update download speed
					speed = float64(downloaded-timeDownload) / 524288
					fileItem.ProgressSpeed.SetText(fmt.Sprintf("Speed: %.2f MB/s", speed))
					timeDownload = downloaded

					// Stop the ticker when the download is complete
					if atomic.LoadInt64(&progressInfo.downloaded) >= total {
						// // Close the file in the main goroutine
						if err := outFile.Close(); err != nil {
							fmt.Printf("Error closing file: %v\n", err)
							dialog.ShowError(fmt.Errorf("error closing file: %v", err), myapp.MainWindow)
						}
						cancelCh <- false
						pauseCh <- false
						ticker.Stop()
						return
					}
				}
			}
		}()
		wg.Wait()

		newFile := Download{
			ID:         fileItem.ID,
			FileName:   fileInfo.FileName,
			URL:        fileInfo.URL,
			FilePath:   fileInfo.FilePath,
			TotalSize:  total,
			Downloaded: progressInfo.downloaded,
			CreatedAt:  time.Now().String(),
			Chunks:     chunkSlice,
		}

		paused = <-pauseCh
		// Handle Pausing
		if paused {
			fmt.Println("Download Paused.")
			newFile.Status = "Paused"
			fileItem.Ctx = ctx
			fileItem.CtxP = ctxP
			go saveDownloadFileInfo(newFile, myapp.DownloadStateFilePath)
			return
		}

		canceled = <-cancelCh
		// Handle cancellation and file deletion
		if canceled {
			fmt.Println("Download Cancelled")
			if err := os.Remove(fileInfo.FilePath); err != nil {
				fmt.Printf("Error deleting file: %v\n", err)

			}
			return
		}

		fmt.Println("Download has Finished.")
		newFile.Status = "Finished"

		go downloadFinished(fileItem, myapp.CurrentDownloadsContainer)
		go saveDownloadFileInfo(newFile, myapp.DownloadStateFilePath)
	}()
}

func ResumeDownload(myapp *MyApp, fileItem *FileItem, cancelC chan context.CancelFunc, pauseC chan context.CancelFunc) {
	canceled := false //flag for cancelling
	paused := false   // flag for pausing
	cancelCh := make(chan bool, 1)
	pauseCh := make(chan bool, 1)

	// initialize contexts
	ctx, cancel := context.WithCancel(myapp.AppContext)
	ctxP, cancelP := context.WithCancel(myapp.AppContext)
	cancelC <- cancel
	pauseC <- cancelP

	// get downloadFile Info
	file, err := isFileExistByID(myapp.DownloadStateFilePath, fileItem.ID)
	if err != nil {
		fmt.Println(err)
	}

	go func(file *Download) {
		// adding the number of request to waitgroup
		var wg sync.WaitGroup

		outFile, err := os.OpenFile(file.FilePath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			fmt.Printf("failed to open JSON file: %v", err)
			dialog.ShowError(fmt.Errorf("error opening file: %v", err), myapp.MainWindow)
			return
		}

		for index, chunk := range file.Chunks {
			wg.Add(1)
			go func(index int, chunk Chunk) {
				defer wg.Done()
				err := downloadChunk(myapp.Client, ctx, ctxP, file.URL, chunk.CurrentOffset, chunk.End, outFile, &file.Downloaded, file.Chunks, index)
				if err != nil && err != context.Canceled {
					fmt.Printf("Error downloading chunk %v\n", err)
					dialog.ShowError(fmt.Errorf("error downloading chunk %d: %v", index, err), myapp.MainWindow)
				}
			}(index, chunk)
		}

		// Periodically update the progress bar, even if chunks aren't finished
		ticker := time.NewTicker(500 * time.Millisecond) // Update every 500ms
		go func() {
			var speed float64
			var timeDownload int64 = 0
			for range ticker.C {
				select {
				case <-ctxP.Done():
					// Closing file after pausing
					if err := outFile.Close(); err != nil {
						fmt.Printf("Error closing file: %v\n", err)
						dialog.ShowError(fmt.Errorf("error closing file: %v", err), myapp.MainWindow)
					}
					pauseCh <- true
					cancelCh <- false
					ticker.Stop()
					return
				case <-ctx.Done(): // Stop updates if canceled
					// Closing file after pausing
					if err := outFile.Close(); err != nil {
						fmt.Printf("Error closing file: %v\n", err)
						dialog.ShowError(fmt.Errorf("error closing file: %v", err), myapp.MainWindow)
					}
					cancelCh <- true
					pauseCh <- false
					ticker.Stop()
					return
				default:
					// Calculate progress based on atomic downloaded value
					downloaded := atomic.LoadInt64(&file.Downloaded)
					value := float64(downloaded) / float64(file.TotalSize)

					// Update the progress bar
					fileItem.Bar.SetValue(value)
					// Update download speed
					speed = float64(downloaded-timeDownload) / 524288
					fileItem.ProgressSpeed.SetText(fmt.Sprintf("Speed: %.2f MB/s", speed))
					timeDownload = downloaded

					// Stop the ticker when the download is complete
					if atomic.LoadInt64(&file.Downloaded) >= file.TotalSize {
						// Closing file after pausing
						if err := outFile.Close(); err != nil {
							fmt.Printf("Error closing file: %v\n", err)
							dialog.ShowError(fmt.Errorf("error closing file: %v", err), myapp.MainWindow)
						}
						cancelCh <- false
						pauseCh <- false
						ticker.Stop()
						return
					}
				}
			}
		}()
		wg.Wait()

		paused = <-pauseCh
		// Handle Pausing
		if paused {
			fmt.Println("Download Paused.")
			go saveDownloadFileInfo(*file, myapp.DownloadStateFilePath)
			return
		}

		canceled = <-cancelCh
		// Handle cancellation and file deletion
		if canceled {
			fmt.Println("Download Cancelled")
			// Close the file in the main goroutine
			if err := os.Remove(file.FilePath); err != nil {
				fmt.Printf("Error deleting file: %v\n", err)
				return
			} else {
				return
			}
		}
		file.UpdatedAt = time.Now().String()

		fmt.Println("Download has Finished.")

		go saveDownloadFileInfo(*file, myapp.DownloadStateFilePath)
		go downloadFinished(fileItem, myapp.CurrentDownloadsContainer)
	}(&file)
}

// ----------------------------------------------- Supplement

func downloadChunk(Client *http.Client, ctx, ctxP context.Context, url string, start, end int64, outFile *os.File, downloaded *int64, chunkSlice []Chunk, index int) error {

	// Prepare the HTTP request with a range header
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Printf("Error creating the request: %v\n", err)
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	// Send Request
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Printf("Error starting the download: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	// Track total bytes
	totalBytesToRead := end - start + 1
	totalRead := int64(0)

	var offset = start
	buf := make([]byte, 1024*256)             // 256KB buffer for reading
	writeBuffer := make([]byte, 0, 1024*1024) //1 MB buffer for batching

Loop:
	for {
		select {
		case <-ctxP.Done(): // Handle pause
			chunkSlice[index] = Chunk{
				End:           end,
				CurrentOffset: offset,
				Status:        "Paused",
			}
			return context.Canceled
		case <-ctx.Done(): // Handle cancellation
			return context.Canceled
		default:
			// Calculate bytes left to read
			bytesLeft := totalBytesToRead - totalRead
			if bytesLeft <= 0 {
				fmt.Printf("Chunk %d: All data read\n", start)
				break Loop // Exit the for loop
			}

			// Adjust read size if necessary
			readSize := int64(len(buf))
			if bytesLeft < readSize {
				readSize = bytesLeft
			}

			// Read data into the buffer
			n, err := resp.Body.Read(buf[:readSize])

			if n > 0 {
				totalRead += int64(n)

				writeBuffer = append(writeBuffer, buf[:n]...)

				// Update the downloaded bytes
				atomic.AddInt64(downloaded, int64(n))

				// If the write buffer exceeds the threshold or all data is read, write to the file
				if len(writeBuffer) >= 1024*1024 || totalRead >= totalBytesToRead {
					if _, writeErr := outFile.WriteAt(writeBuffer, offset); writeErr != nil {
						fmt.Printf("Error writing to file: %v\n", writeErr)
						return writeErr
					}
					offset += int64(len(writeBuffer)) // Update the offset
					writeBuffer = writeBuffer[:0]     // Reset the buffer
				}
			}

			if err != nil {
				if err == io.EOF {
					break Loop // Exit the for loop
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return context.Canceled
				}
				fmt.Printf("Error reading from response: %v\n", err)
				return err
			}

			if n == 0 {
				// No data read, but no error; avoid infinite loop
				fmt.Printf("Chunk %d: Zero-byte read, breaking loop\n", start)
				break Loop // Exit the for loop
			}
		}
	}

	// Write any remaining data in the buffer to the file (only if not canceled)
	if len(writeBuffer) > 0 {
		if _, writeErr := outFile.WriteAt(writeBuffer, offset); writeErr != nil {
			fmt.Printf("Error writing remaining data to file: %v\n", writeErr)
			return writeErr
		}
	}
	return nil
}

// ----------------------------------------------- Extra

func getFileInfo(client *http.Client, url string) (FileInfo, error) {
	// Get file Info
	resp, err := client.Head(url)
	if err != nil {
		fmt.Println("error making the request")
		return FileInfo{}, fmt.Errorf("error making the request, %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to download file: HTTP %d\n", resp.StatusCode)
		return FileInfo{}, fmt.Errorf("failed to download file %v", err)
	}
	defer resp.Body.Close() // Close response

	fileSize, total := getFileSize(resp)
	fileName := getFileName(resp, url)

	// Making filePath
	downloadsFolder, err := getDownloadD()
	if err != nil {
		fmt.Printf("Unable to find Downloads folder: %v\n", err)
		return FileInfo{}, fmt.Errorf("unable to find downloads folder %v", err)
	}
	filePath := filepath.Join(downloadsFolder, fmt.Sprintf("DownBitDownloads/%s", fileName))

	return FileInfo{
		FileName: fileName,
		FileSize: fileSize,
		Total:    int(total),
		FilePath: filePath,
		URL:      url,
	}, nil
}

func getDownloadD() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	downloadPath := filepath.Join(homeDir, "Downloads")
	return downloadPath, nil
}

func getFileSize(resp *http.Response) (float64, int64) {
	var size float64 = 0
	var cl int64 = -1

	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		cl, _ = strconv.ParseInt(contentLength, 10, 64)
		sizeInBytes, err := strconv.Atoi(contentLength)
		if err != nil {
			fmt.Println("Error parsing Content-Length:", err)
			return size, cl
		}
		size = float64(sizeInBytes) / (1024 * 1024)
		return size, cl
	} else {
		cl, _ = strconv.ParseInt(contentLength, 10, 64)
		fmt.Println("file Size is unkown")
		return size, cl
	}
}

func getFileName(resp *http.Response, urlStr string) string {
	// 1. Try to get the filename from Content-Disposition header
	if disposition := resp.Header.Get("Content-Disposition"); disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			if filename := params["filename"]; filename != "" {
				return sanitizeFileName(filename) // Sanitize the filename
			}
		}
	}

	// 2. Extract the filename from the URL path
	if parsedURL, err := url.Parse(urlStr); err == nil {
		if filename := path.Base(parsedURL.Path); filename != "" && filename != "/" {
			return sanitizeFileName(filename) // Sanitize the filename
		}
	}

	// 3. Fallback to a default filename if neither method succeeds
	return "unknown_file"
}

// sanitizeFileName sanitizes the filename for safe usage on the filesystem
func sanitizeFileName(filename string) string {
	// Remove unwanted intermediate extensions like ".ir"
	if parts := strings.Split(filename, "."); len(parts) > 2 {
		filename = strings.Join(parts[:len(parts)-1], ".") + "." + parts[len(parts)-1]
	}

	// Trim double quotes and replace unsafe characters
	replacer := strings.NewReplacer(
		`"`, "",
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(strings.TrimSpace(filename))
}

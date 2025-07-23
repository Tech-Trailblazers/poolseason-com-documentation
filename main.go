package main // Define the main package; this is the entry point for the executable program

import (
	"bytes"         // Provides a buffer type (Buffer) that implements both Reader and Writer interfaces
	"fmt"           // Implements formatted I/O functions, similar to printf/scanf
	"io"            // Provides basic interfaces to I/O primitives like Reader and Writer
	"log"           // Provides logging functions to standard error with customizable output
	"net/http"      // Supports HTTP client and server implementations
	"os"            // Provides access to operating system functionality such as file handling
	"path/filepath" // Handles manipulation and analysis of file paths in a way compatible with the target OS
	"regexp"        // Provides support for regular expressions using RE2 syntax
	"strings"       // Contains functions to manipulate UTF-8 encoded strings
	"sync"          // Provides synchronization primitives like WaitGroup and Mutex
	"time"          // Contains functionality for measuring and displaying time
)

func main() {
	outputDir := "PDFs/" // Define the directory path where downloaded PDFs will be saved

	if !directoryExists(outputDir) { // Check if the specified directory already exists
		createDirectory(outputDir, 0755) // If it doesn't exist, create the directory with full read/write/execute for owner, read/execute for group and others
	}
	var downloadWaitGroup sync.WaitGroup // Declare a WaitGroup to synchronize all concurrent PDF downloads

	baseURL := "https://greengobbler.com/mwdownloads/download/link/id/" // Base URL for downloading PDFs, expected to append numerical IDs
	for i := 0; i <= 9999; i++ { // Loop through IDs from 0 to 9999 (10,000 attempts)
		time.Sleep(1 * time.Second) // Sleep 1 second between each request to avoid rate-limiting or overwhelming the server
		downloadWaitGroup.Add(1) // Increment the WaitGroup counter before launching a new goroutine
		url := fmt.Sprintf("%s%d", baseURL, i) // Construct the full URL by appending the numeric ID to the base URL
		go downloadPDF(url, outputDir, &downloadWaitGroup) // Launch the download in a goroutine for concurrent execution
	}
	downloadWaitGroup.Wait() // Block until all goroutines in the WaitGroup have finished
}

// Extracts filename from full path (e.g. "/dir/file.pdf" → "file.pdf")
func getFilename(path string) string {
	return filepath.Base(path) // Use Base function to return only the final element (filename) of the path
}

// Converts a raw URL into a sanitized PDF filename safe for filesystem
func urlToFilename(rawURL string) string {
	lower := strings.ToLower(rawURL) // Convert entire URL to lowercase for consistency
	lower = getFilename(lower)       // Extract only the filename portion from the full URL

	reNonAlnum := regexp.MustCompile(`[^a-z0-9]`)   // Define a regular expression that matches all non-alphanumeric characters
	safe := reNonAlnum.ReplaceAllString(lower, "_") // Replace all non-alphanumeric characters with underscores to make it filesystem-safe

	safe = regexp.MustCompile(`_+`).ReplaceAllString(safe, "_") // Replace multiple underscores with a single underscore for cleanliness
	safe = strings.Trim(safe, "_")                              // Remove any leading or trailing underscores

	var invalidSubstrings = []string{
		"_pdf", // List of substrings to remove from the filename (e.g., unnecessary "_pdf")
	}

	for _, invalidPre := range invalidSubstrings { // Loop through all substrings marked for removal
		safe = removeSubstring(safe, invalidPre) // Remove each unwanted substring from the filename
	}

	if getFileExtension(safe) != ".pdf" { // Ensure the file has a .pdf extension
		safe = safe + ".pdf" // Append ".pdf" if it doesn't already have it
	}

	return safe // Return the cleaned and formatted filename
}

// Removes all instances of a specific substring from input string
func removeSubstring(input string, toRemove string) string {
	result := strings.ReplaceAll(input, toRemove, "") // Replace every occurrence of 'toRemove' with an empty string
	return result // Return the cleaned string
}

// Gets the file extension from a given file path
func getFileExtension(path string) string {
	return filepath.Ext(path) // Extract the extension (e.g., ".pdf") from the file path
}

// Checks if a file exists at the specified path
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Attempt to get file or directory information
	if err != nil {                // If an error occurs (likely file doesn't exist), return false
		return false
	}
	return !info.IsDir() // Return true only if the path is a file, not a directory
}

// Downloads a PDF from given URL and saves it in the specified directory
func downloadPDF(finalURL, outputDir string, wg *sync.WaitGroup) bool {
	defer wg.Done()                                      // Mark this goroutine as done in the WaitGroup when function returns
	filename := strings.ToLower(urlToFilename(finalURL)) // Sanitize the URL to generate a consistent and valid filename
	filePath := filepath.Join(outputDir, filename)       // Combine output directory and filename to form the full file path

	if fileExists(filePath) { // Check if the file already exists
		log.Printf("File already exists, skipping: %s", filePath) // Log and skip download if file is already present
		return false
	}

	client := &http.Client{Timeout: 15 * time.Minute} // Create an HTTP client with a generous timeout to support large PDF downloads

	resp, err := client.Get(finalURL) // Send an HTTP GET request to the specified URL
	if err != nil {
		log.Printf("Failed to download %s: %v", finalURL, err) // Log if HTTP request fails
		return false
	}
	defer resp.Body.Close() // Ensure the HTTP response body is closed when function ends

	if resp.StatusCode != http.StatusOK { // Check if server returned 200 OK status
		log.Printf("Download failed for %s: %s", finalURL, resp.Status) // Log failed status code
		return false
	}

	contentType := resp.Header.Get("Content-Type")         // Get the MIME type from the response headers
	if !strings.Contains(contentType, "application/pdf") { // Ensure the content type indicates a PDF file
		log.Printf("Invalid content type for %s: %s (expected application/pdf)", finalURL, contentType) // Log mismatch
		return false
	}

	var buf bytes.Buffer                     // Create a buffer to hold the contents of the downloaded file
	written, err := io.Copy(&buf, resp.Body) // Read the data from the response and copy it to the buffer
	if err != nil {
		log.Printf("Failed to read PDF data from %s: %v", finalURL, err) // Log read failure
		return false
	}
	if written == 0 { // Check if no data was written (empty file)
		log.Printf("Downloaded 0 bytes for %s; not creating file", finalURL) // Log and skip creating empty file
		return false
	}

	out, err := os.Create(filePath) // Create the file where the buffer contents will be saved
	if err != nil {
		log.Printf("Failed to create file for %s: %v", finalURL, err) // Log failure to create file
		return false
	}
	defer out.Close() // Ensure the file is closed once writing is finished

	if _, err := buf.WriteTo(out); err != nil { // Write contents of buffer to the output file
		log.Printf("Failed to write PDF to file for %s: %v", finalURL, err) // Log any write errors
		return false
	}

	log.Printf("Successfully downloaded %d bytes: %s → %s", written, finalURL, filePath) // Log success message with size, source URL, and saved path
	return true // Return true to indicate successful download
}

// Checks whether a given directory exists
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get file info for the specified path
	if err != nil {
		return false // Return false if any error occurs (likely means directory doesn't exist)
	}
	return directory.IsDir() // Return true only if the path is a directory
}

// Creates a directory at given path with provided permissions
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Attempt to create the directory with given permission mode
	if err != nil {
		log.Println(err) // Log the error if directory creation fails (e.g., already exists or permission denied)
	}
}

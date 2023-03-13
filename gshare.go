package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Usage: gshare <address> [file]

const (
	PORT = "1234"
	CHUNKSIZE = 1024
	SECONDS_BETWEEN_CONNECTION_ATTEMPTS = 0.5
	progressBarLength = 40
	asciiProgressBar = false
)

func checkerr(err error) {
	if err != nil {
		panic(err)
	}
}

func hideCursor() {
	fmt.Print("\033[?25l")
}

func showCursor() {
	fmt.Print("\033[?25h")
}

func UpdateProgressBar(completed, total int, startTime, lastUpdateTime time.Time, taskName, successMessage string) {
	// Based on Rich (a Python library)'s progress bar
	// The bar characters below can be found on https://www.w3.org/TR/xml-entity-names/025.html

	// Return if it's been less than half a second (for perfomance reasons)
	if time.Since(lastUpdateTime) < 500 * time.Millisecond {
		return
	}

	fullBarCompleted := "━"
	fullBarNotCompleted := fullBarCompleted
	halfBarLeft := "╸"
	halfBarRight := "╺"
	if asciiProgressBar {
		fullBarCompleted = "#"
		fullBarNotCompleted = "-"
		halfBarLeft = " "
		halfBarRight = " "
	}
	progressBarShadedColor := "\033[34m"
	progressBarNotShadedColor := "\033[30;2m"
	clearLineCode := "\033[2K"
	moveCursorToStartCode := "\r"
	resetFormattingCode := "\033[0m"
	successColorCode := "\033[92m"

	// If done
	if completed == total {
		fmt.Println(clearLineCode + moveCursorToStartCode + successColorCode + successMessage + resetFormattingCode)
		return
	}

	progressBar := ""

	// Add task name
	progressBar += taskName + "..." + " "

	// Add shaded color
	progressBar += progressBarShadedColor

	// halfBarLeft and halfBarRight chars make it possible for the progress bar to occupy half a terminal cell, which I'm going to call a subChar
	// This is the number of completed subChars
	numSubChars := progressBarLength * 2 * completed / total

	// Add completed full bars
	progressBar += strings.Repeat(fullBarCompleted, numSubChars / 2)


	// Add half bar and not-shaded-color
	if numSubChars % 2 == 0 {
		progressBar += progressBarNotShadedColor
		progressBar += halfBarRight
	} else {
		progressBar += halfBarLeft
		progressBar += progressBarNotShadedColor
	}

	// Add not-shaded full bars
	progressBar += strings.Repeat(fullBarNotCompleted, progressBarLength - numSubChars / 2 - 1)
	
	// Add eta estimate
	nanosecondsSinceStart := int(time.Since(startTime))
	progressBar += resetFormattingCode
	progressBar += "  " + "eta" + " "
	if completed != 0 {
		progressBar += (time.Duration(nanosecondsSinceStart / completed * (total - completed)) * time.Nanosecond).Truncate(time.Second).String()
	}
	
	// Draw bar
	fmt.Print(clearLineCode + moveCursorToStartCode + progressBar)
}

func fileExists(path string) (bool) {
	if _, err := os.Stat(path); err == nil {
		return true
	} else if errors.Is(err, os.ErrNotExist) {
		return false
	} else {
		// There was some other kind of error.
		// I don't know what other kind of error
		// there could be, so I'm just leaving panic here
		panic(err)
	}
}

func getIndexOfLastOccurrenceOfChar(stringToSearchThrough string, char byte) (n int, err error) {
	for n := len(stringToSearchThrough) - 1; n >= 0; n-- {
		if stringToSearchThrough[n] == char {
			return n, nil
		}
	}

	return -1, errors.New("char not in string")
}

func InfoPrint(info ...any) {
	fmt.Print("\033[2m") // set dim/faint mode
	for i, word := range info {
		fmt.Printf("%v", word)
		if i != len(info) - 1 {
			fmt.Print(" ")
		}
	}
	fmt.Println("\033[0m") // reset formatting
}

func InfoPrintReplaceLine(info ...any) {
	fmt.Print("\033[2K") // clear line
	fmt.Print("\r") // move cursor to start of line
	fmt.Print("\033[2m") // set dim/faint mode
	for i, word := range info {
		fmt.Printf("%v", word)
		if i != len(info) - 1 {
			fmt.Print(" ")
		}
	}
	fmt.Print("\033[0m") // reset formatting
}

func sendUint64(conn net.Conn, v uint64, infoPrintMessage string) {
	byteSlice := make([]byte, 8)
	binary.BigEndian.PutUint64(byteSlice, v)
	conn.Write(byteSlice)
	if infoPrintMessage != "" {
		InfoPrint(infoPrintMessage)
	}
}

func sendUint32(conn net.Conn, v uint32, infoPrintMessage string) {
	byteSlice := make([]byte, 4)
	binary.BigEndian.PutUint32(byteSlice, v)
	conn.Write(byteSlice)
	if infoPrintMessage != "" {
		InfoPrint(infoPrintMessage)
	}
}

func sendUint8(conn net.Conn, v uint8, infoPrintMessage string) {
	conn.Write([]byte{v})
	if infoPrintMessage != "" {
		InfoPrint(infoPrintMessage)
	}
}

func sendString(conn net.Conn, s, infoPrintMessage string) {
	conn.Write([]byte(s))
	if infoPrintMessage != "" {
		InfoPrint(infoPrintMessage)
	}
}

func recvUint64(conn net.Conn) uint64 {
	buffer := make([]byte, 8)
	_, err := conn.Read(buffer)
	checkerr(err)
	return binary.BigEndian.Uint64(buffer)
}

func recvUint32(conn net.Conn) uint32 {
	buffer := make([]byte, 4)
	_, err := conn.Read(buffer)
	checkerr(err)
	return binary.BigEndian.Uint32(buffer)
}

func recvUint8(conn net.Conn) uint8 {
	buffer := make([]byte, 1)
	_, err := conn.Read(buffer)
	checkerr(err)
	return buffer[0]
}

func recvString(conn net.Conn, numBytes int) string {
	buffer := make([]byte, numBytes)
	_, err := conn.Read(buffer)
	checkerr(err)
	return string(buffer)
}

func getUniqueFilename(filename string) string {
	newFilename := filename
	if fileExists(filename) {
		dotIndex, err := getIndexOfLastOccurrenceOfChar(filename, '.')
		
		var stem, extension string
		
		if err == nil {
			stem = filename[:dotIndex]
			extension = filename[dotIndex:]
		} else if err.Error() == "char not in string" {
			stem = filename
			extension = ""
		} else {
			checkerr(err)
		}

		filenameNumber := 1
		for {
			newFilename = stem + "(" + strconv.Itoa(filenameNumber) + ")" + extension
			if !fileExists(newFilename) {
				break
			}
			filenameNumber++
		}
	}
	return newFilename
}


func sendFile(ipAddress, filePath string) {
	// ---------- Get Socket Connection --------------------

	// Listen for connections
	ln, err := net.Listen("tcp", ":" + PORT)
	InfoPrint("Server hosted on port", PORT)
	checkerr(err)

	var conn net.Conn
	for {
		// Accept any incominging connections
		conn, err = ln.Accept()
		checkerr(err)
		
		
		// Get the remote address
		remoteAddress, _, _ := strings.Cut(conn.RemoteAddr().String(), ":")
		// Close the connection and redo the loop
		if remoteAddress != ipAddress {
			// Send a 0 to say they've been rejected, let them know that there're plenty of other fish in the sea
			sendUint8(conn, 1, "")

			conn.Close()

			InfoPrint("Rejected connection from", remoteAddress)
			continue
		}

		// Close the connection when done
		defer conn.Close()

		// Send a 1 to say that they've been accepted
		responseBytes := make([]byte, 1)
		responseBytes[0] = 1
		conn.Write(responseBytes)

		// Break out of the loop
		InfoPrint("Connection established with", remoteAddress)
		break
	}
	// Sync chunksizes
	chunksize := CHUNKSIZE
	receiverChunkSize := int(recvUint64(conn))
	if receiverChunkSize < chunksize {
		fmt.Println("Receiver is using a smaller chunksize", "(" + strconv.Itoa(receiverChunkSize) + "),", "using the receiver's chunksize")
		chunksize = receiverChunkSize
	}
	sendUint64(conn, uint64(chunksize), "Synced chunksize sent")
	
	receiverCanReceiveFile := recvUint8(conn)
	if receiverCanReceiveFile == 0 {
		fmt.Println("Receiver sent back a 0, meaning they can't receive the file. That should only be possible if the source code was modified, so I'm blaming you for this one")
		return
	}
	if receiverCanReceiveFile != 1 {
		fmt.Println("The receiver sent back", strconv.Itoa(int(receiverCanReceiveFile)), "when only a 1 or a 0 was expected, idk what's going on there but I'm just going to send the file")
	}

	// Open file for reading
	file, err := os.Open(filePath)
	checkerr(err)
	// Close it when done
	defer file.Close()

	// ---------- Send Filename --------------------
	filename := file.Name() // the .name method returns just the filename without the full path
	info, _ := os.Stat(filePath)
	filesize := uint64(info.Size())
	chunkCount := uint64((filesize + CHUNKSIZE - 1) / CHUNKSIZE)// Divide by chunksize, round up

	sendUint64(conn, uint64(len(filename)), "Filename-length sent")
	sendString(conn, filename, "Filename sent")
	sendUint32(conn, uint32(info.Mode()), "Permissions sent")
	sendUint64(conn, chunkCount, "Chunk count sent")
	sendUint64(conn, chunkCount * CHUNKSIZE - filesize, "Last-chunk-length sent")
	
	// ---------- Send File in Chunks --------------------

	// Create read buffer
	readBuffer := make([]byte, CHUNKSIZE)
	// Create reader
	reader := io.Reader(file)


	startTime := time.Now()
	timeOfLastProgressBarUpdate := time.Unix(0, 0) // The progress bar was last updated in 1970, because why not
	hideCursor()
	UpdateProgressBar(0, int(chunkCount), startTime, timeOfLastProgressBarUpdate, "Sending", "\"" + filename + "\"" + " sent!")

	for chunksSentCount := uint64(0); chunksSentCount < chunkCount; chunksSentCount++ {
		// Read next chunk
		n, err := reader.Read(readBuffer)
		checkerr(err)
		// Send chunk
		conn.Write(readBuffer[:n])
		UpdateProgressBar(int(chunksSentCount) + 1, int(chunkCount), startTime, timeOfLastProgressBarUpdate, "Sending", "\"" + filename + "\"" + " sent!")
	}
	showCursor()
}

func receiveFile(ipAddress string) {
	// ---------- Get Socket Connection --------------------
	InfoPrint("Trying to connect")

	var conn net.Conn
	var err error

	// Keep trying to connect
	for {
		conn, err = net.Dial("tcp", ipAddress + ":" + PORT)

		// If everything worked out continue with the rest of the program
		if err == nil {break}

		time.Sleep(time.Duration(SECONDS_BETWEEN_CONNECTION_ATTEMPTS * float64(time.Second)))
	}
	checkerr(err)
	// Close the connection when done
	defer conn.Close()

	// Checker whether accepted or rejected
	acceptedOrRejected := recvUint8(conn)
	if acceptedOrRejected == 0 {
		InfoPrint("Connection rejected, perhaps your address was mistyped on the other end?")
		return
	} else if acceptedOrRejected == 1 {
		InfoPrint("Connection accepted!")
	} else {
		InfoPrint("accepted/rejected value was not a 0/1. This program is confused and will now exit")
		return
	}

	// Sync chunksizes
	sendUint64(conn, CHUNKSIZE, "Chunksize sent")
	chunksize := int(recvUint64(conn))

	if chunksize > CHUNKSIZE {
		fmt.Println("The server sent back a larger chunksize", "(" + strconv.Itoa(chunksize) + ")", "the smaller chunksize is supposed to be chosen, so idk what's going on here, exiting")
		sendUint8(conn, 0, "")
		return
	}

	sendUint8(conn, 1, "Sent receiver-can-receive-file")


	filenameLength := int(recvUint64(conn))
	filename := recvString(conn, filenameLength)
	perm := os.FileMode(recvUint32(conn))
	chunkCount := recvUint64(conn)
	lastChunkLength := recvUint64(conn)
	
	InfoPrint("Receiving \"" + filename + "\"")
	
	// Example:
	// a.txt -> a(2).txt
	filename = getUniqueFilename(filename)

	f, err := os.OpenFile(filename, os.O_WRONLY | os.O_CREATE | os.O_EXCL, perm)
	checkerr(err)

	dataBuffer := make([]byte, CHUNKSIZE)


	startTime := time.Now()
	timeOfLastProgressBarUpdate := time.Unix(0, 0) // The progress bar was last updated in 1970, because why not
	hideCursor()
	UpdateProgressBar(0, int(chunkCount), startTime, timeOfLastProgressBarUpdate, "Receiving", "\"" + filename + "\"" + " received!")

	for chunksReceived := uint64(0); chunksReceived < chunkCount; chunksReceived++ {
		_, err := conn.Read(dataBuffer)
		checkerr(err)
		if chunksReceived == chunkCount - 1 {
			f.Write(dataBuffer[:lastChunkLength])
		} else {
			f.Write(dataBuffer)
		}
		UpdateProgressBar(int(chunksReceived) + 1, int(chunkCount), startTime, timeOfLastProgressBarUpdate, "Receiving", "\"" + filename + "\"" + " received!")
	}
	showCursor()
}


func main() {
	args := os.Args[1:]
	var mode string
	if len(args) == 2 {
		mode = "send"
	} else {
		mode = "receive"
	}

	if mode == "send" {
		sendFile(args[0], args[1])
	} else {
		receiveFile(args[0])
	}
}

package main

import (
	"encoding/binary"
	"time"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	PORT = "1234"
	CHUNKSIZE = 1024
	SECONDS_BETWEEN_CONNECTION_ATTEMPTS = 0.5
	DEBUG_MODE = true
)

func checkerr(err error) {
	if err != nil {
		panic(err)
	}
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
	fmt.Print("\033[K") // clear line
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

// gshare <address> [file]

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
			// Send a 0 to say that they've been rejected
			responseBytes := make([]byte, 1)
			responseBytes[0] = 0
			conn.Write(responseBytes)

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
	// Open file for reading
	file, err := os.Open(filePath)
	checkerr(err)
	// Close it when done
	defer file.Close()

	// ---------- Send Filename --------------------
	// Send the filename
	filename := file.Name() // the .name method (idc if it's technically a function it's a method) returns just the filename without the full path
	filenameBytes := []byte(filename)
	if DEBUG_MODE {
		InfoPrint("filename:", filename)
		InfoPrint("filename bytes:", filenameBytes)
		InfoPrint("filename bytes length:", len(filenameBytes))
	}
	conn.Write(filenameBytes)
	InfoPrint("Filename sent")


	// ---------- Send Permissions --------------------
	// Get permissions
	info, _ := os.Stat(filePath)
	perm := uint32(info.Mode())
	permBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(permBytes, perm)
	if DEBUG_MODE {
		InfoPrint("perm:", perm)
		InfoPrint("perm bytes:", permBytes)
	}
	conn.Write(permBytes)
	InfoPrint("Permissions sent")

	// ---------- Send Chunk Count --------------------
	chunkCount := (info.Size() + CHUNKSIZE - 1) / CHUNKSIZE // Divide by chunksize, round up
	chunkCountBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(chunkCountBytes, uint64(chunkCount))
	if DEBUG_MODE {
		InfoPrint("chunk-count:", chunkCount)
		InfoPrint("chunk-count bytes:", chunkCountBytes)
	}
	conn.Write(chunkCountBytes)
	InfoPrint("Chunk count sent")
	
	// ---------- Send File in Chunks --------------------


	// Create read buffer
	readBuffer := make([]byte, CHUNKSIZE)
	// Create reader
	reader := io.Reader(file)


	for chunksSentCount := int64(0); chunksSentCount < chunkCount; chunksSentCount++ {
		// Read next chunk
		n, err := reader.Read(readBuffer)
		checkerr(err)
		// Send chunk
		conn.Write(readBuffer[:n])
		InfoPrintReplaceLine(strconv.FormatInt(chunksSentCount, 10) + "/" + strconv.FormatInt(chunkCount, 10), "chunks sent")
	}
	fmt.Println() // the replace-line variation omits the newline, so print one back in

	InfoPrint(chunkCount, "File sent :)")
}

func receiveFile(ipAddress string) {
	// ---------- Get Socket Connection --------------------
	InfoPrint("Trying to connect")

	var conn net.Conn
	var err error

	// Keep trying to connect
	attemptNumber := 0
	for {
		attemptNumber++

		conn, err = net.Dial("tcp", ipAddress + ":" + PORT)

		if DEBUG_MODE {
			if err == nil {
				InfoPrintReplaceLine("Connected after", attemptNumber, "attempts")
				fmt.Println() // the replace-line variation omits the newline, so print one back in
			} else {
				InfoPrintReplaceLine("Attempt", attemptNumber, "error:", "\"" + err.Error() + "\"")
			}
		}

		// If everything worked out continue with the rest of the program
		if err == nil {break}

		time.Sleep(time.Duration(SECONDS_BETWEEN_CONNECTION_ATTEMPTS * float64(time.Second)))
	}
	checkerr(err)
	// Close the connection when done
	defer conn.Close()

	// Checker whether accepted or rejected
	acceptedOrRejectedBuffer := make([]byte, 1) // The name sounds bad but I can't think of a better one
	_, err = conn.Read(acceptedOrRejectedBuffer)
	checkerr(err)
	acceptedOrRejected := int(acceptedOrRejectedBuffer[0])
	if DEBUG_MODE {
		InfoPrint("accepted/rejected:", acceptedOrRejected)
		InfoPrint("accepted/rejected byte:", acceptedOrRejectedBuffer)
	}
	if acceptedOrRejected == 0 {
		InfoPrint("Connection rejected, perhaps your address was mistyped on the other end?")
		return
	} else if acceptedOrRejected == 1 {
		InfoPrint("Connection accepted!")
	} else {
		InfoPrint("accepted/rejected value was not a 0/1. This program is confused and will now exit")
		return
	}



	// Get filename
	filenameBuffer := make([]byte, 1024)
	n, err := conn.Read(filenameBuffer)
	checkerr(err)
	filename := strings.TrimSpace(string(filenameBuffer[:n]))
	if DEBUG_MODE {
		InfoPrint("received filename:", filename)
		InfoPrint("received filename bytes:", filenameBuffer[:n])
		InfoPrint("received filename bytes length:", n)
	}

	InfoPrint("Receiving \"" + filename + "\"")


	// Get permissions
	permBuffer := make([]byte, 4)
	conn.Read(permBuffer)
	perm := os.FileMode(binary.BigEndian.Uint32(permBuffer))
	if DEBUG_MODE {
		InfoPrint("received perm:", perm)
		InfoPrint("received perm bytes:", permBuffer)
	}

	// Get chunk count
	chunkCountBuffer := make([]byte, 8)
	conn.Read(chunkCountBuffer)
	chunkCount := int64(binary.BigEndian.Uint64(chunkCountBuffer))
	if DEBUG_MODE {
		InfoPrint("received chunk-count:", chunkCount)
		InfoPrint("received chunk-count bytes:", chunkCountBuffer)
	}


	// ---------- Get unique filename --------------------
	// Example:
	// a.txt -> a(2).txt
	newFilename := filename
	if fileExists(filename) {
		extensionSeperatorDotIndex, err := getIndexOfLastOccurrenceOfChar(filename, '.')
		var stem, extension string
		if err == nil {
			stem = filename[:extensionSeperatorDotIndex]
			extension = filename[extensionSeperatorDotIndex:]
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

	f, err := os.OpenFile(newFilename, os.O_WRONLY | os.O_CREATE | os.O_EXCL, perm)
	checkerr(err)

	dataBuffer := make([]byte, CHUNKSIZE)

	for chunksReceived := int64(0); chunksReceived < chunkCount; chunksReceived++ {
		n, err := conn.Read(dataBuffer)
		checkerr(err)
		f.Write(dataBuffer[:n])
		InfoPrintReplaceLine(strconv.FormatInt(chunksReceived + 1, 10) + "/" + strconv.FormatInt(chunkCount, 10), "chunks received")
	}
	fmt.Println() // the replace-line variation omits the newline, so print one back in
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
package main

import (
	// "bytes"
	"encoding/binary"
	// "encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	// "reflect"
	"strconv"
	"strings"
)

const (
	PORT = "1234"
	CHUNKSIZE = 1024
)

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

// gshare <address> [file]

func sendFile(ipAddress, filePath string) {
	// ---------- Get Socket Connection --------------------

	// Listen for connections
	ln, err := net.Listen("tcp", ":" + PORT)
	if err != nil {
		panic(err)
	}

	var conn net.Conn
	for {
		// Accept the incoming connection
		conn, err = ln.Accept()
		if err != nil {
			panic(err)
		}
		// Close it when done
		defer conn.Close()
		// Get the remote address
		remoteAddress, _, _ := strings.Cut(conn.RemoteAddr().String(), ":")
		// Look for new connections if this one isn't whitelisted
		if remoteAddress != ipAddress {
			fmt.Println("Not whitelisted connection from", remoteAddress)
			continue
		}
		// Break out of the loop
		fmt.Println("New connection from", remoteAddress)
		break
	}
	// Open file for reading
	f, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	// Close it when done
	defer f.Close()

	// ---------- Send Filename --------------------

	// Get filename
	filename := f.Name() // returns just the filename and not the path
	// fmt.Printf("%X\n", filename) // DEBUGGING
	filenameBytes := []byte(filename)
	// Send the filename
	conn.Write(filenameBytes)
	// fmt.Printf("%X", filename) // DEBUGGING
	// Get permissions
	info, _ := os.Stat(filePath)
	perm := uint32(info.Mode())
	permByte := make([]byte, 4)
	binary.BigEndian.PutUint32(permByte, perm)
	conn.Write(permByte)
	
	// ---------- Send File in Chunks --------------------


	// Create read buffer
	readBuffer := make([]byte, CHUNKSIZE)
	// Create reader
	r := io.Reader(f)


	for {
		// Read next chunk
		n, err := r.Read(readBuffer)
		if err != nil {
			break
		}
		conn.Write(readBuffer[:n])
		// fmt.Println("hi")
	}
}

func receiveFile(ipAddress string) {
	// ---------- Get Socket Connection --------------------
	// Connect to IP address through port
	conn, err := net.Dial("tcp", ipAddress + ":" + PORT)
	if err != nil {
		panic(err)
	}
	// Close the connection when done
	defer conn.Close()


	// Get filename
	filenameBuffer := make([]byte, 1024)
	// fmt.Println(filenameBuffer) // DEBUG
	n, err := conn.Read(filenameBuffer)
	// fmt.Println(n) // DEBUG
	if err != nil {
		panic(err)
	}
	filename := strings.TrimSpace(string(filenameBuffer)[:n])
	// fmt.Printf("%X\n", filename) // DEBUG

	// Get permissions
	permBuffer := make([]byte, 1024)
	conn.Read(permBuffer)
	perm := os.FileMode(binary.BigEndian.Uint32(permBuffer))

	// Print info
	fmt.Println("Getting the file \"" + filename + "\"")

	newFilename := filename
	// If a file with that filename already exists
	if fileExists(filename) {
		// Get index of the dot that separates the stem from the extension
		extensionSeperatorDotIndex := 0
		for n := len(filename) - 1; n >= 0; n-- {
			if filename[n] == '.' {
				extensionSeperatorDotIndex = n
				break
			}
		}

		// Use the index of the dot to get the stem and extension components
		stem := filename[:extensionSeperatorDotIndex]
		extension := filename[extensionSeperatorDotIndex + 1:]

		// Keep incrementing the number until a unique file is found
		filenameNumber := 1
		for {
			newFilename = stem + "(" + strconv.Itoa(filenameNumber) + ")" + "." + extension
			// If there's an error, that means that the file doesn't exist, so you've found a unique filename
			if !fileExists(newFilename) {
				break
			}
			filenameNumber++
		}
	}

	// fmt.Println("hi", newFilename) // DEBUG

	f, err := os.OpenFile(newFilename, os.O_WRONLY | os.O_CREATE | os.O_EXCL, perm)
	if err != nil {
		// fmt.Println(reflect.TypeOf(err)) // DEBUG
		// fmt.Println(err) // DEBUG
		var idk *fs.PathError
		if errors.As(err, &idk) {
			fmt.Println("hi") // DEBUG
		}
		panic(err)
	}

	dataBuffer := make([]byte, CHUNKSIZE)

	for {
		n, err := conn.Read(dataBuffer)
		// fmt.Println(n) // DEBUG
		if err != nil {
			fmt.Println(err) // DEBUG
			break
			// fmt.Println(reflect.TypeOf(err))
			// panic(err)
		}
		// fmt.Println("NEW WRITE") // DEBUG
		f.Write(dataBuffer[:n])
	}
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
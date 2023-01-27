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

func InfoPrint(info ...any) {
	fmt.Print("\033[2m")
	for i, word := range info {
		fmt.Printf("%v", word)
		if i != len(info) - 1 {
			fmt.Print(" ")
		}
	}
	fmt.Println("\033[0m")
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
			conn.Close()
			InfoPrint("Rejected connection from", remoteAddress)
			continue
		}

		// Close the connection when done
		defer conn.Close()

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
	// TODO: UDPATE
	a := []byte(file.Name())
	fmt.Println(a)
	fmt.Println(len(a))
	conn.Write(a) // f.Name gets just the filename, without the full path
	InfoPrint("Filename sent")



	// // ---------- Create Info Bytes --------------------
	// info := make([]byte, 4)

	// ---------- Send Permissions --------------------
	// Get permissions
	info, _ := os.Stat(filePath)
	perm := uint32(info.Mode())
	fmt.Println("mode", info.Mode())
	permByte := make([]byte, 4)
	binary.BigEndian.PutUint32(permByte, perm)
	conn.Write(permByte)
	InfoPrint("Permissions sent")

	// ---------- Send Chunk Count --------------------
	chunkCount := (info.Size() + CHUNKSIZE - 1) / CHUNKSIZE // Divide by chunksize, round up
	chunkCountByte := make([]byte, 8)
	binary.BigEndian.PutUint64(chunkCountByte, uint64(chunkCount))
	conn.Write(chunkCountByte)
	InfoPrint("Chunk count sent")
	
	// ---------- Send File in Chunks --------------------


	// Create read buffer
	readBuffer := make([]byte, CHUNKSIZE)
	// Create reader
	reader := io.Reader(file)


	for {
		// Read next chunk
		n, err := reader.Read(readBuffer)
		if err != nil {
			break
		}
		// Send chunk
		conn.Write(readBuffer[:n])
	}

	InfoPrint(chunkCount, "chunks sent")
}

func receiveFile(ipAddress string) {
	// ---------- Get Socket Connection --------------------
	// Connect to IP address through port
	conn, err := net.Dial("tcp", ipAddress + ":" + PORT)
	// fmt.Println(errors.Unwrap(err).Error())
	// errors.As(err, errors.Unwrap(err))
	// fmt.Println(err.Error() == "connect: no route to host")
	// if err.Error() == "no route to host" {
	// 	fmt.Println("hi")
	// 	return
	// }
	// var a *net.Error
	// if errors.As(err, &a) {
	// 	fmt.Println("hi", a)
	// }
	checkerr(err)
	// Close the connection when done
	defer conn.Close()


	// Get filename
	filenameBuffer := make([]byte, 1024)
	n, err := conn.Read(filenameBuffer)
	fmt.Println(filenameBuffer)
	fmt.Println(len(filenameBuffer))
	// The if the server closed the connection, conn.Read above will put an io.EOF into err
	if errors.Is(err, io.EOF) {
		InfoPrint("Server rejected connection; perhaps they mistyped your address")
		return
	}
	checkerr(err)
	filename := strings.TrimSpace(string(filenameBuffer[:n]))
	// fmt.Printf("%X\n", filename) // DEBUG

	// Get permissions
	permBuffer := make([]byte, 4)
	conn.Read(permBuffer)
	perm := os.FileMode(binary.BigEndian.Uint32(permBuffer))

	// Get chunk count
	chunkCountBuffer := make([]byte, 8)
	conn.Read(chunkCountBuffer)
	chunkCount := int64(binary.BigEndian.Uint64(chunkCountBuffer))
	fmt.Println(chunkCount)

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

	for i := int64(0); i < chunkCount; i++ {
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
package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path"
	"strconv"
	"time"

	fc "github.com/firecracker-microvm/firecracker-go-sdk"
)

// stdin, stdout, stderror
func getIO() (io.Reader, io.Writer, io.Writer) {
	stdout, _ := os.OpenFile("out.log", os.O_RDWR|os.O_CREATE, 0755)
	stderr, _ := os.OpenFile("err.log", os.O_RDWR|os.O_CREATE, 0755)
	return nil, stdout, stderr
}

func deleteDirContents(dirName string) error {
	files, err := ioutil.ReadDir(dirName)
	if err != nil {
		return err
	}

	for _, file := range files {
		err := os.RemoveAll(path.Join(dirName, file.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {

	socketRootDir := "/tmp/firecracker"

	// Create folder if doesn't exist and clean up after everything
	os.MkdirAll(socketRootDir, fs.ModePerm)
	defer deleteDirContents(socketRootDir)

	// Generate a socket file name
	rand.Seed(time.Now().Unix())
	id := strconv.Itoa(rand.Intn(10000000))
	sockName := id + ".sock"
	log.Println("Using sock", sockName)

	// _, stdout, stderr := getIO()

	command := fc.VMCommandBuilder{}.
		WithBin("firecracker").
		WithSocketPath(path.Join(socketRootDir, sockName)). // should autogenerate and clean up after exit
		WithArgs([]string{"--config-file", "vm_config.json", "--id", id}).
		WithStdin(os.Stdin).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		Build(context.Background())

	done := make(chan error)
	go func() {
		done <- command.Run()
	}()

	// How to handle interrupt? i.e. ctrl-c?

	// <-ctx.Done()
	<-done
	fmt.Printf("Done..")

}

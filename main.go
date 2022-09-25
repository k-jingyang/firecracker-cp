package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/rs/zerolog/log"
	"pault.ag/go/loopback"
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

func buildSquashFSImage(pathToBaseImage string, pathToInitScript string) (string, error) {

	randomDirName, err := os.MkdirTemp("/tmp", "*")
	if err != nil {
		log.Error().Msg("Unable to create random folder")
		return "", err
	}
	log.Debug().Msg("Random folder generated=" + randomDirName)
	defer os.RemoveAll(randomDirName)

	imageFile, err := os.OpenFile(pathToBaseImage, os.O_RDWR, 0644)
	if err != nil {
		log.Error().Msg("Unable to open " + pathToBaseImage + " for reading")
		return "", err
	}

	log.Debug().Msg("Mounting " + pathToBaseImage)

	// It should be possible to read this
	// Fails if I take away syscall.MS_RDONLY
	_, unmount, err := loopback.MountImage(imageFile, randomDirName, "ext4", 0, "")
	if err != nil {
		log.Error().Err(err).Msg("Unable to mount")
		return "", err
	}

	defer unmount()

	dirEntries, err := os.ReadDir(randomDirName)
	if err != nil {
		log.Error().Msg("Unable to read directory entries")
		return "", err
	}

	for _, entry := range dirEntries {
		fmt.Println(entry.Name())
	}

	return "test", nil

}

func main() {

	_, err := buildSquashFSImage("./bionic.rootfs.base.ext4", "")

	if err != nil {
		log.Error().Err(err).Send()
	}

	// socketRootDir := "/tmp/firecracker"

	// // Create folder if doesn't exist and clean up after everything
	// os.MkdirAll(socketRootDir, fs.ModePerm)
	// defer deleteDirContents(socketRootDir)

	// // Generate a socket file name
	// rand.Seed(time.Now().Unix())
	// id := strconv.Itoa(rand.Intn(10000000))
	// sockName := id + ".sock"
	// log.Println("Using sock", sockName)

	// // _, stdout, stderr := getIO()

	// command := fc.VMCommandBuilder{}.
	// 	WithBin("firecracker").
	// 	WithSocketPath(path.Join(socketRootDir, sockName)). // should autogenerate and clean up after exit
	// 	WithArgs([]string{"--config-file", "vm_config.json", "--id", id}).
	// 	WithStdin(os.Stdin).
	// 	WithStdout(os.Stdout).
	// 	WithStderr(os.Stderr).
	// 	Build(context.Background())

	// done := make(chan error)
	// go func() {
	// 	done <- command.Run()
	// }()

	// // How to handle interrupt? i.e. ctrl-c?

	// // <-ctx.Done()
	// <-done
	// fmt.Println("Done..")

}

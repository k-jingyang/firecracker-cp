package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

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

/*
buildSquashFSImage's logic, based off https://github.com/firecracker-microvm/firecracker/discussions/3061
1. Mount base image
2. Create directories in image to be used for overlay (https://windsock.io/the-overlay-filesystem/)
  i. 	/overlay/work - scratch space
  ii. 	/overlay/root - upperdir that provides the writeable layer
  iii.	/mnt - the new root
  iv. 	/mnt/rom - the old root
3. Copy overlay_init into /sbin/overlay-init
3. Make squashfs

Returns filepath to squashfs image
*/
func buildSquashFSImage(pathToBaseImage string, pathToInitScript string) (string, error) {

	mountDir, cleanUp, err := mountImageToRandomDir(pathToBaseImage)
	if err != nil {
		return "", err
	}
	defer cleanUp()

	// Create directories that will be used later for overlay
	os.MkdirAll(filepath.Join(mountDir, "overlay", "work"), 755)
	os.MkdirAll(filepath.Join(mountDir, "overlay", "root"), 755)
	os.MkdirAll(filepath.Join(mountDir, "mnt", "rom"), 755)

	dirEntries, err := os.ReadDir(filepath.Join(mountDir, "sbin"))
	if err != nil {
		log.Error().Msg("Unable to read directory entries")
		return "", err
	}

	for _, entry := range dirEntries {
		fmt.Println(entry.Name())
	}

	// Copy overlay_init
	destination, err := os.Create(filepath.Join(mountDir, "sbin", "overlay-init"))
	if err != nil {
		return "", err
	}

	overlay_init, err := os.ReadFile(filepath.Join(".", "overlay_init"))
	if err != nil {
		return "", err
	}

	_, err = destination.Write(overlay_init)
	if err != nil {
		return "", err
	}

	// TODO: Add squashfs implementation

	// List directories
	dirEntries, err = os.ReadDir(filepath.Join(mountDir, "sbin"))
	if err != nil {
		log.Error().Msg("Unable to read directory entries")
		return "", err
	}

	for _, entry := range dirEntries {
		fmt.Println(entry.Name())
	}

	return "test", nil

}

// mountImageToRandomDir returns the mountpoint
func mountImageToRandomDir(pathToBaseImage string) (string, func(), error) {
	randomDirName, err := os.MkdirTemp("/tmp", "*")
	if err != nil {
		log.Error().Msg("Unable to create random folder")
		return "", func() {}, err
	}
	log.Debug().Msg("Random folder generated=" + randomDirName)

	// Must open as RDWR
	imageFile, err := os.OpenFile(pathToBaseImage, os.O_RDWR, 0)
	if err != nil {
		log.Error().Msg("Unable to open " + pathToBaseImage + " for reading")
		return "", func() {}, err
	}

	log.Debug().Msg("Mounting " + pathToBaseImage)

	_, unmount, err := loopback.MountImage(imageFile, randomDirName, "ext4", 0, "")
	if err != nil {
		log.Error().Err(err).Msg("Unable to mount")
		return "", func() {}, err
	}

	cleanUp := func() {
		unmount()
		os.RemoveAll(randomDirName)
	}

	return randomDirName, cleanUp, nil
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

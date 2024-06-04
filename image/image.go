package image

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/ForAllSecure/rootfs_builder/rootfs"
	"github.com/diskfs/go-diskfs"
	"github.com/rs/zerolog/log"
	"pault.ag/go/loopback"
)

/*
BuildSquashFSImage builds the necessary rootfs image for the uVM

The logic, based off https://github.com/firecracker-microvm/firecracker/discussions/3061
 1. Mount base image
 2. Create directories in image to be used for overlay (https://windsock.io/the-overlay-filesystem/)
    i. 	/overlay/work - scratch space
    ii. 	/overlay/root - upperdir that provides the writeable layer
    iii.	/mnt - the new root
    iv. 	/mnt/rom - the old root
 3. Copy overlay_init into /sbin/overlay-init
 3. Make squashfs
*/
func BuildSquashFSImage(pathToBaseImage string, pathToInitScript string, pathToNewSquashImage string) error {
	mountDir, cleanUp, err := mountImageToRandomDir(pathToBaseImage)
	if err != nil {
		return err
	}
	defer cleanUp()

	err = addRequiredFiles(mountDir, pathToInitScript)
	if err != nil {
		return err
	}

	// TODO Use zstd?
	mksquashfs := exec.Command("mksquashfs", mountDir, pathToNewSquashImage, "-noappend")
	err = mksquashfs.Run()
	if err != nil {
		return err
	}

	printDir(mountDir)

	return nil
}

// mountImageToRandomDir mounts the image to a random directory and returns the mountpoint
func mountImageToRandomDir(imagePath string) (string, func(), error) {
	randomDirName, err := os.MkdirTemp("/tmp", "*")
	if err != nil {
		log.Error().Msg("Unable to create random folder")
		return "", func() {}, err
	}
	log.Debug().Msgf("Random folder %s generated", randomDirName)

	// Must open as RDWR
	imageFile, err := os.OpenFile(imagePath, os.O_RDWR, 0)
	if err != nil {
		log.Error().Msg("Unable to open " + imagePath + " for reading")
		return "", func() {}, err
	}

	log.Debug().Msgf("Mounting %s", imagePath)

	_, unmount, err := loopback.MountImage(imageFile, randomDirName, "ext4", 0, "")
	if err != nil {
		log.Error().Err(err).Msg("Unable to mount")
		return "", func() {}, err
	}

	cleanUp := func() {
		log.Debug().Msgf("Unmounting %s", imagePath)
		imageFile.Close()
		unmount()
		os.RemoveAll(randomDirName)
	}

	return randomDirName, cleanUp, nil
}

func printDir(dirName string) error {
	// List directories
	dirEntries, err := os.ReadDir(filepath.Join(dirName, "sbin"))
	if err != nil {
		log.Error().Err(err).Msg("Unable to read directory entries")
		return err
	}

	for _, entry := range dirEntries {
		fmt.Println(entry.Name())
	}

	return nil
}

func addRequiredFiles(mountDir string, pathToInitScript string) error {
	os.MkdirAll(filepath.Join(mountDir, "overlay", "work"), 0754)
	os.MkdirAll(filepath.Join(mountDir, "overlay", "root"), 0754)
	os.MkdirAll(filepath.Join(mountDir, "mnt"), 0754)
	os.MkdirAll(filepath.Join(mountDir, "rom"), 0754)

	// Copy overlay_init
	destination, err := os.Create(filepath.Join(mountDir, "sbin", "overlay-init"))
	if err != nil {
		return fmt.Errorf("unable to create overlay-init: %w", err)
	}
	os.Chmod(destination.Name(), os.FileMode(0754))
	defer destination.Close()

	overlay_init, err := os.ReadFile(filepath.Join(".", pathToInitScript))
	if err != nil {
		return fmt.Errorf("unable to read overlay-init: %w", err)

	}
	_, err = destination.Write(overlay_init)
	if err != nil {
		return fmt.Errorf("unable to write overlay-init: %w", err)
	}

	// Configure nameserver
	err = os.WriteFile(filepath.Join(mountDir, "etc", "resolv.conf"), []byte("nameserver 8.8.8.8\n"), 0644)
	if err != nil {
		return fmt.Errorf("unable to write /etc/resolv.conf: %w", err)
	}

	return nil
}

// Make SSH image from SSH key
func MakeSSHDiskImage(sshPubKey []byte) (string, error) {
	hash := (md5.Sum(sshPubKey))
	hashStr := hex.EncodeToString(hash[:])
	sshDiskImage := fmt.Sprintf("ssh_keys/%s.img", hashStr)

	_, err := os.Stat(sshDiskImage)
	fileExists := !errors.Is(err, fs.ErrNotExist)
	if fileExists {
		return sshDiskImage, nil
	}

	log.Debug().Msg(sshDiskImage + " does not exist. Creating...")

	// Create empty image file and create filesystem
	// TODO: Is there a more optimal size than 2MB
	disk, err := diskfs.Create(sshDiskImage, 2000000, diskfs.Raw, diskfs.SectorSizeDefault)
	if err != nil {
		return "", err
	}
	log.Debug().Msgf("Created disk image for SSH key: %s", disk.File.Name())
	mkext4 := exec.Command("mkfs.ext4", disk.File.Name())
	err = mkext4.Run()
	if err != nil {
		return "", err
	}

	// Add SSH key into image
	mountDir, cleanUp, err := mountImageToRandomDir(disk.File.Name())
	if err != nil {
		return "", err

	}
	defer cleanUp()
	os.MkdirAll(filepath.Join(mountDir, "root", ".ssh"), 0700)
	destination, err := os.Create(filepath.Join(mountDir, "root", ".ssh", "authorized_keys"))
	if err != nil {
		return "", err
	}
	os.Chmod(destination.Name(), os.FileMode(0644))
	defer destination.Close()

	_, err = destination.Write(sshPubKey)
	if err != nil {
		return "", err
	}

	return disk.File.Name(), nil
}

func MakeRootFS(dockerImage string, pathToInitScript string) (string, error) {
	blobFile := strings.ReplaceAll(dockerImage, ":", "-")
	blobFile = strings.ReplaceAll(blobFile, "/", "-")

	blobFSPath := path.Join("blobs", blobFile)
	// file alr exists, don't have to repeat
	if _, err := os.Stat(blobFSPath); err == nil {
		return blobFSPath, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("unable to stat %s: %w", blobFSPath, err)
	}

	// file does not exist, so we should build make the rootfs img
	randomDir, err := os.MkdirTemp("/tmp", "*")
	if err != nil {
		return "", fmt.Errorf("unable to create random folder: %w", err)
	}

	log.Debug().Msgf("created random folder for docker container fs: %s", randomDir)

	// defer os.RemoveAll(randomDir)

	imageToPull := rootfs.PullableImage{
		Name:    dockerImage,
		Retries: 3,
		Spec: rootfs.Spec{
			Dest: randomDir,
		},
	}

	// TODO: Need to inject init
	pulled, err := imageToPull.Pull()
	if err != nil {
		return "", fmt.Errorf("unable to pull image %s: %w", dockerImage, err)
	}

	err = pulled.Extract()
	extractedRootFS := path.Join(randomDir, "rootfs")
	if err != nil {
		return "", fmt.Errorf("unable to extract image %s: %w", dockerImage, err)
	}

	err = addRequiredFiles(extractedRootFS, pathToInitScript)
	if err != nil {
		return "", fmt.Errorf("unable to add required files: %w", err)
	}

	// TODO Use zstd?
	log.Debug().Msgf("running mksquashfs on %s, outputting to %s", extractedRootFS, blobFSPath)
	mksquashfs := exec.Command("mksquashfs", extractedRootFS, blobFSPath, "-noappend")
	err = mksquashfs.Run()
	if err != nil {
		return "", fmt.Errorf("unable to run mksquashfs: %w", err)
	}

	printDir(randomDir)

	return blobFSPath, nil
}

package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"time"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/firecracker-microvm/firecracker-go-sdk"
	fc "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"pault.ag/go/loopback"
)

type CreateVMResponse struct {
	IPAddress string `json:"ipAddress"`
}

type CreateVMRequest struct {
	SSHPubKey string `json:"sshPubKey"`
}

func (a *CreateVMRequest) Bind(r *http.Request) error {
	return nil
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
*/
func buildSquashFSImage(pathToBaseImage string, pathToInitScript string, pathToNewSquashImage string) error {
	mountDir, cleanUp, err := mountImageToRandomDir(pathToBaseImage)
	if err != nil {
		return err

	}
	defer cleanUp()

	// Create directories that will be used later for overlay
	os.MkdirAll(filepath.Join(mountDir, "overlay", "work"), 0755)
	os.MkdirAll(filepath.Join(mountDir, "overlay", "root"), 0755)
	os.MkdirAll(filepath.Join(mountDir, "mnt"), 0755)
	os.MkdirAll(filepath.Join(mountDir, "rom"), 0755)

	dirEntries, err := os.ReadDir(filepath.Join(mountDir))
	if err != nil {
		log.Error().Msg("Unable to read directory entries")
		return err
	}

	for _, entry := range dirEntries {
		fmt.Println(entry.Name())
	}

	// Copy overlay_init
	destination, err := os.Create(filepath.Join(mountDir, "sbin", "overlay-init"))
	if err != nil {
		return err
	}
	os.Chmod(destination.Name(), os.FileMode(0755))
	defer destination.Close()

	overlay_init, err := os.ReadFile(filepath.Join(".", pathToInitScript))
	if err != nil {
		return err

	}
	_, err = destination.Write(overlay_init)
	if err != nil {
		return err
	}

	// TODO Use zstd?
	mksquashfs := exec.Command("mksquashfs", mountDir, pathToNewSquashImage, "-noappend")
	err = mksquashfs.Run()
	if err != nil {
		return err
	}

	// List directories
	dirEntries, err = os.ReadDir(filepath.Join(mountDir, "sbin"))

	if err != nil {
		log.Error().Msg("Unable to read directory entries")
		return err

	}

	for _, entry := range dirEntries {
		fmt.Println(entry.Name())
	}

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

// Make SSH image from SSH key
func makeSSHDiskImage(sshPubKey []byte) (string, error) {
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
	// TODO: What's a good size?
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

func main() {
	// Make squashFS rootfs image
	const squashFsImage = "./squash-rootfs.img"
	_, err := os.Stat(squashFsImage)
	if errors.Is(err, fs.ErrNotExist) {
		log.Debug().Msg(squashFsImage + "does not exist. Creating...")

		err = buildSquashFSImage("./bionic.rootfs.base.ext4", "./overlay-init", squashFsImage)

		if err != nil {
			log.Error().Err(err).Msgf("Unable to create %s image", squashFsImage)
			os.Exit(1)
		}
	}

	// Setup directory to store socket files
	socketRootDir := "/tmp/firecracker"
	os.MkdirAll(socketRootDir, fs.ModePerm)
	defer deleteDirContents(socketRootDir)

	// Configure API server
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Post("/vm", func(w http.ResponseWriter, r *http.Request) {
		data := &CreateVMRequest{}
		render.Bind(r, data)
		sshPubKey := []byte(data.SSHPubKey)
		// Make SSH key image
		sshKeyImage, err := makeSSHDiskImage(sshPubKey)
		if err != nil {
			log.Panic().Err(err).Send()
		}
		log.Debug().Msgf("SSH pubkey is at %s", sshKeyImage)

		ipAddr := makeVM(socketRootDir, sshKeyImage)
		log.Debug().Msgf("IPaddr=%s", ipAddr)
		render.JSON(w, r, CreateVMResponse{IPAddress: ipAddr})
	})

	// Start API server
	const port = 3000
	log.Info().Msgf("Starting API server at %d", port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), r)

	if err != nil {
		log.Error().Msg(err.Error())
	}
}

func makeVM(socketDir string, sshKeyImage string) string {
	// Create a unique ID
	rand.Seed(time.Now().Unix())
	id := strconv.Itoa(rand.Intn(10000000))
	sockName := id + ".sock"
	log.Debug().Msgf("Creating uVM and using %s as API socket", sockName)

	// Create logs files
	// TODO Is there a better way to create logs inside logs/ other than pre-creating /logs
	stdout, _ := os.Create("logs/" + id + "-out.log")
	stderr, _ := os.Create("logs/" + id + "-err.log")

	defer stdout.Close()
	defer stderr.Close()

	config := fc.Config{
		SocketPath:      path.Join(socketDir, sockName),
		LogPath:         stdout.Name(),
		LogLevel:        "Info",
		KernelImagePath: "vmlinux.bin",
		KernelArgs:      "console=ttyS0 reboot=k panic=1 pci=off overlay_root=ram ssh_disk=/dev/vdb init=/sbin/overlay-init",
		Drives: []models.Drive{
			{
				DriveID:      lo.ToPtr("rootfs"),
				PathOnHost:   lo.ToPtr("squash-rootfs.img"),
				IsRootDevice: lo.ToPtr(true),
				IsReadOnly:   lo.ToPtr(true),
				CacheType:    lo.ToPtr("Unsafe"),
				IoEngine:     lo.ToPtr("Sync"),
				RateLimiter:  nil,
			}, {
				DriveID:      lo.ToPtr("vol2"),
				PathOnHost:   lo.ToPtr(sshKeyImage),
				IsRootDevice: lo.ToPtr(false),
				IsReadOnly:   lo.ToPtr(true),
				CacheType:    lo.ToPtr("Unsafe"),
				IoEngine:     lo.ToPtr("Sync"),
				RateLimiter:  nil,
			},
		},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:       lo.ToPtr(int64(2)),
			MemSizeMib:      lo.ToPtr(int64(1024)),
			Smt:             lo.ToPtr(false),
			TrackDirtyPages: false,
		},

		NetworkInterfaces: fc.NetworkInterfaces{
			fc.NetworkInterface{
				CNIConfiguration: &firecracker.CNIConfiguration{
					NetworkName: "fcnet",
					IfName:      "veth0",
				},
				AllowMMDS: true,
			},
		},
		// what is MetricsFifo and LogsFifo
	}

	uVM, err := fc.NewMachine(context.Background(), config)
	if err != nil {
		log.Error().Msg(err.Error())
	}

	uVM.Start(context.Background())

	if err != nil {
		log.Error().Msg(err.Error())
	}

	// Get allocated IP address from CNI
	ipBuf, err := ioutil.ReadFile("/var/lib/cni/networks/fcnet/last_reserved_ip.0")
	return string(ipBuf)
}

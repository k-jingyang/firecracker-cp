package main

import (
	"context"
	"errors"
	"firecracker-cp/image"
	"fmt"
	"io/fs"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	fc "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

const socketRootDir = "/tmp/firecracker"

var uVMs = make(map[string]VM)

type VM struct {
	ID      string
	IPAddr  string
	machine *fc.Machine
}

type CreateVMResponse struct {
	ID        string `json:"id"`
	IPAddress string `json:"ipAddress"`
}

type CreateVMRequest struct {
	SSHPubKey string `json:"sshPubKey"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (a *CreateVMRequest) Bind(r *http.Request) error {
	return nil
}

func deleteDirContents(dirName string) error {
	files, err := os.ReadDir(dirName)
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
	// Make squashFS rootfs image
	const squashFsImage = "./squash-rootfs.img"
	_, err := os.Stat(squashFsImage)
	if errors.Is(err, fs.ErrNotExist) {
		log.Debug().Msg(squashFsImage + "does not exist. Creating...")

		err = image.BuildSquashFSImage("./bionic.rootfs.base.ext4", "./overlay-init", squashFsImage)

		if err != nil {
			log.Panic().Err(err).Msgf("Unable to create %s image. Exiting", squashFsImage)
		}
	}

	// Setup directory to store socket files and clean up after
	os.MkdirAll(socketRootDir, fs.ModePerm)
	defer deleteDirContents(socketRootDir)

	// Configure API server
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Post("/vm", handleCreateVM)
	r.Delete("/vm/{id}", handleDeleteVM)

	// Start API server
	const port = 3000
	log.Info().Msgf("Starting API server at %d", port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), r)

	if err != nil {
		log.Error().Msg(err.Error())
	}
}

func handleCreateVM(w http.ResponseWriter, r *http.Request) {
	data := &CreateVMRequest{}
	render.Bind(r, data)
	sshPubKey := []byte(data.SSHPubKey)
	// Make SSH key image
	sshKeyImage, err := image.MakeSSHDiskImage(sshPubKey)
	if err != nil {
		log.Panic().Err(err).Send()
	}
	log.Debug().Msgf("SSH pubkey is at %s", sshKeyImage)

	vm := createVM(socketRootDir, sshKeyImage)
	uVMs[vm.ID] = vm
	log.Debug().Msgf("ID=%s IPaddr=%s", vm.ID, vm.IPAddr)
	render.JSON(w, r, CreateVMResponse{ID: vm.ID, IPAddress: vm.ID})
}

func handleDeleteVM(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vmID := chi.URLParam(r, "id")
	if vmID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	vm, ok := uVMs[vmID]
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	err := vm.machine.Shutdown(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r,
			ErrorResponse{
				Error: fmt.Errorf("unable to shutdown vm err=%w", err).Error(),
			})
		return
	}

	w.WriteHeader(http.StatusOK)
}

func createVM(socketDir string, sshKeyImage string) VM {
	// Create a unique ID
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
	ipBuf, _ := os.ReadFile("/var/lib/cni/networks/fcnet/last_reserved_ip.0")
	return VM{
		ID:      id,
		IPAddr:  string(ipBuf),
		machine: uVM,
	}
}

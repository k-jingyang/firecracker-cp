## Firecracker Control Plane

Control plane for spinning up Firecracker microVMs

## Objectives of this project
1. Play around, understand Firecracker
   1. Understand unfamiliar OS concepts
2. Try out API framework of Go

## Outcome
1. An API call to spin up a Firecracker VM
   - Params: SSH public key
   - Returns: An IP address that I can SSH in with a SSH secret key
## To-Do

1. How to have a new rootfs for each instance, without copying rootfs
   1. Copy-on-write - https://github.com/firecracker-microvm/firecracker/discussions/3061, allows SSH keys to be added
       -  Do we really need to use SquashFS
   1. Overlayfs seems to create the "merged" as a mount https://askubuntu.com/questions/109413/how-do-i-use-overlayfs. Need to investigate more.
2. Specify SSH pub key to put inside the microVM
   1. May consider sending SSH pub key into MMDS and have the microVM fetch the SSH pub key
      1. microVM has to be configured to fetch from MMDS on boot, https://github.com/firecracker-microvm/firecracker/issues/1947
3. Setup networking for microVM
   1. See https://github.com/firecracker-microvm/firecracker/blob/main/docs/network-setup.md
4. Look at [firecracker-containerd](https://github.com/firecracker-microvm/firecracker-containerd)
   1. See how firecracker VMs are created with the same root base image

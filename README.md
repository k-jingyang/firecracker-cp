## Firecracker Control Plane

Control plane for spinning up Firecracker microVMs

## To-Do

1. How to have a new rootfs for each instance, without copying rootfs
   1. Copy-on-write - https://github.com/firecracker-microvm/firecracker/discussions/3061, allows SSH keys to be added
   1. Overlayfs seems to create the "merged" as a mount https://askubuntu.com/questions/109413/how-do-i-use-overlayfs. Need to investigate more.
2. Specify SSH pub key to put inside the microVM
   1. May consider sending SSH pub key into MMDS and have the microVM fetch the SSH pub key
      1. microVM has to be configured to fetch from MMDS on boot, https://github.com/firecracker-microvm/firecracker/issues/1947
3. Setup networking for microVM
   1. See https://github.com/firecracker-microvm/firecracker/blob/main/docs/network-setup.md

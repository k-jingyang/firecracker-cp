## Firecracker Control Plane

Control plane for spinning up Firecracker microVMs

## To-Do

1. How to have a new rootfs for each instance, without copying rootfs
1. Specify SSH pub key to put inside the microVM
   1. Possible to do union fs for the rootfs? Something like Docker layers, thus adding our own SSH keys into the rootfs
      1. Overlayfs seems to create the "merged" as a mount https://askubuntu.com/questions/109413/how-do-i-use-overlayfs. Need to investigate more.
      2. How can I create a new rootfs image, **efficiently**?
   2. May consider sending SSH pub key into MMDS and have the microVM fetch the SSH pub key
      1. microVM has to be configured to fetch from MMDS on boot, https://github.com/firecracker-microvm/firecracker/issues/1947
1. Setup networking for microVM
   1. See https://github.com/firecracker-microvm/firecracker/blob/main/docs/network-setup.md

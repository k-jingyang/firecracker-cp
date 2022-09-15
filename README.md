## Firecracker Control Plane

Control plane for spinning up Firecracker microVMs

## To-Do

1. Specify SSH pub key to put inside the microVM
   1. Possible to do union fs for the rootfs? Something like Docker layers, thus adding our own SSH keys into the rootfs
      1. Overlayfs seems to create the "merged" as a mount https://askubuntu.com/questions/109413/how-do-i-use-overlayfs. Need to investigate more.
      2. How can I create a new rootfs image, **efficiently**
2. Setup networking for microVM
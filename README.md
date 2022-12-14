## Firecracker Control Plane

Control plane for spinning up Firecracker microVMs

## Objectives of this project
1. Play around, understand Firecracker
   1. Understand unfamiliar OS concepts
2. Try out [API framework of Go](https://github.com/go-chi/chi)

## Outcome
1. An API call to spin up a Firecracker VM
   - Params: SSH public key
   - Returns: An IP address that I can SSH in with the corresponding SSH secret key

## Program flow
1. Start process with base image
2. Create read-only squashfs image from base image
3. Start API server

## References

1. How to have a new rootfs for each instance, without copying rootfs
   1. Copy-on-write using overlayfs - https://github.com/firecracker-microvm/firecracker/discussions/3061
       -  Do we really need to use SquashFS? 
           -  Yes, as it is a read-only compressed (300MB to 58MB) image
2. Specify SSH pub key to put inside the microVM
   1. May consider sending SSH pub key into MMDS and have the microVM fetch the SSH pub key
      1. microVM has to be configured to fetch from MMDS on boot, https://github.com/firecracker-microvm/firecracker/issues/1947
3. Setup networking for microVM
   1. See https://github.com/firecracker-microvm/firecracker/blob/main/docs/network-setup.md
4. Look at [firecracker-containerd](https://github.com/firecracker-microvm/firecracker-containerd)
   1. See how firecracker VMs are created with the same root base image

## TODO
- [ ] Currently, we're unable to start multiple uVMs concurrently, because of `vm_config.json`. Have to find ways to create a tap per uVM and configure the uVM via `VMCommandBuilder` instead of `vm_config.json`
```json
"host_dev_name": "tap0" // unqiue resource
```

## Learnings
1. We can use overlayfs to layer a writable layer ontop of a read-only base image as the rootfs of the uVM (ala Docker)
2. squashFS makes a good filesystem for a read-only image (original was 300MB, compressed to 58MB)
3. Basics. But IP forwarding has to be enabled if the host is doing any form of packet routing (i.e. passing the packet on where the recipient is not itself)
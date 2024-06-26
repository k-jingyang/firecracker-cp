## Firecracker Control Plane

Control plane for spinning up Firecracker microVMs

## Objectives of this project
1. Play around and understand Firecracker
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
       - Do we really need to use SquashFS? 
           - Yes, as it is a read-only compressed (300MB to 58MB) image
2. Specify SSH pub key to put inside the microV
   1. May consider sending SSH pub key into MMDS and have the microVM fetch the SSH pub key
      1. microVM has to be configured to fetch from MMDS on boot, https://github.com/firecracker-microvm/firecracker/issues/1947
3. Setup networking for microVM
   1. See https://github.com/firecracker-microvm/firecracker/blob/main/docs/network-setup.md
4. Look at [firecracker-containerd](https://github.com/firecracker-microvm/firecracker-containerd)
   1. See how firecracker VMs are created with the same root base image

## Learnings
1. We can use overlayfs to layer a writable layer ontop of a read-only base image as the rootfs of the uVM (ala Docker)
2. squashFS makes a good filesystem for a read-only image (original was 300MB, compressed to 58MB)
3. Basics. But IP forwarding has to be enabled if the host is doing any form of packet routing (i.e. passing the packet on where the recipient is not itself)
   - Hence, IP forwarding via `/proc/sys/net/ipv4/ip_forward` not required unless uVM is going to the internet 
4. For host to reach uVM at 172.16.0.2
   1. Add a TAP device and setting its IP to 172.16.0.1
   ```bash
   sudo ip tuntap add tap0 mode tap
   sudo ip addr add 172.16.0.1/24 dev tap0 
   sudo ip link set tap0 up # Interface will only be active after a proccess uses your tap interface (i.e. firecracker)
   # once tap0 is given IP address and UP-ed, a route will be created. Run route command
   ```
   2. Configure firecracker with
   ```json
   "boot_args": "console=ttyS0 reboot=k panic=1 pci=off ip=172.16.0.2:::255.255.255.0::eth0:off overlay_root=ram init=/sbin/overlay-init",
   "network-interfaces": [
      {
         "host_dev_name": "tap0"
      }
   ],

5. Because each host TAP device routes for its subnet, we're unable to create another TAP device on the same host that uses the same subnet
   1. Since each uVM has to has its own TAP device, each uVM needs to be in its own subnet
   2. Unless we do a bridge interface.
6. [A linux bridge](https://developers.redhat.com/blog/2018/10/22/introduction-to-linux-interfaces-for-virtual-networking#bridge), where you can attach multiple virtual interface to the bridge (also an interface). Only the bridge requires an IP and it can forward packets to the respective attached interfaces. Hence, all the attached virtual interfaces can be in the same subnet.
   1. Same learnings apply here as a TAP device
      1. Give it an IP address, and UP both the bridge and the TAP
7. Firecracker does not support Multi-Queue TAP interfaces
   1. Multi-Queue allows parallelization of RX and TX
8. I was trying to do the aforementioned stuff manually using IPAM and a bunch of networking libraries, but it can all be done by using a [CNI](https://www.redhat.com/sysadmin/cni-kubernetes)
   1. Using CNI as documented [here](https://www.redhat.com/sysadmin/cni-kubernetes) does everything out of the box. I do have to build [tc-redirect-tap](https://github.com/awslabs/tc-redirect-tap) manually, just a `make all`
      1. Config is in `/etc/cni/conf.d/fcnet.conflist` and binaries are in `/opt/cni/bin` on my local PC
      2. `host-local` IPAM stores IP addresses in `/var/lib/cni/networks/fcnet`. Conveniently, `last_reserved_ip.0` has the last allocated IP address
9. MMDSv1 is configured easily using the sdk with a `AllowMMDS: true`
10. Using overlay-init with retrieving from MMDS will not work because host needs the socket (the guest VM) to be up before it can send the request to the MMDS, whereas overlay-init runs before the VM comes up
11. Why is it right to create the `/rom` folder while the `pivot_root` is `/mnt/rom`
    1. Because the `old_root` must be a path under `new_root` (e.g. if given `/mnt/rom`, when `/mnt` becomes the new root, old_root will be placed under `/rom` of the new `/mnt` root
12. When you use mkfs.ext4 on a file too small (2MB), it gives that error that says that `Filesystem too small for a journal` and creates an ext2 instead.
13. Ballooning
    1. Allows the hypervisor to get memory from provisioned guest VMs
14. DNS nameserver settings will only be effective if the VM's rootfs makes /etc/resolv.conf be a symlink to /proc/net/pnp.
15. nftables not enabled in kernel, need to enable when compiling kernel via `.config`
    1. don't understand what each config does, and why my `.config` was invalid, causing it to be overwritten each time I run `make vmlinux`
         - My `.config` was most probably wrong because some configurations depend on other configurations and I didn't enable the right dependencies
    2. https://stackoverflow.com/questions/22929065/difference-between-linux-loadable-and-built-in-modules explains the difference between `y` and `m` in `.config`
16. rootfs needs to have an init (most docker images do not have it)
17. nftables errors were due to NF/XTABLE not enabled in the compiled kernel (vmlinux). See point 15.

```text
iptables v1.8.7 (nf_tables): Could not fetch rule set generation id: Invalid argument
iptables v1.8.7 (nf_tables):  TABLE_ADD failed (Operation not supported): table filter
```


## On CNI
1. CNI is responsible for inserting an interface into a network namespace and configures the interface (e.g. assigning an IP address)
2. Is network namespace creation required or is it automatically created?
   - Should verify what happens if we don't create it. But seems like we have to create it.
3. CNI plugins are meant to be chained.
   - In the example provided by in [firecracker-go-sdk](https://github.com/firecracker-microvm/firecracker-go-sdk#), `ptp`, `host-local`, `firewall`, and `tc-redirect-tap` are used
4. Firecracker searches for CNI configuration under `/etc/cni/conf.d/`


## To run
1. `go build && sudo ./firecracker-cp`
2. On another terminal,
```bash
curl -X POST localhost:3000/vm -d @-  << EOF
{
   "sshPubKey" : "xxx"
}
EOF

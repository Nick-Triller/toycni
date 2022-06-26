# toycni

A minimal CNI implemented for a meetup presentation.
The slides are available at TBD.

## Build

Compile toycni for linux:

```bash
make build
```

## Demo setup

`make demo-setup` creates two Ubuntu VMs using [multipass](https://github.com/canonical/multipass) and initializes a
two node cluster with kubeadm.
cloud-init is used to prepare the VMs (install packages etc.).

A kubeconfig for the cluster within the VMs is placed at `demo/kubeconfig`.
Execute the following command to point kubectl at the kubeconfig file:
```bash
export KUBECONFIG="$(pwd)/demo/kubeconfig"
```

Important filesystem locations within the VMs:
- toycni logs are written to `/var/log/toycni`.
- CNI plugin binaries are stored in `/opt/cni/bin/`.
- Allocated IP addresses are stored as files in `/var/lib/cni/networks/toycni/`.
- CNI config is stored at `/etc/cni/net.d/10-toycni.conf`.

The VMs can be deleted with `make demo-cleanup`.

## Invoking toycni manually

```bash
# Get a shell in the VM
multipass shell node01

# Create network namespace
ip netns add testing

# CNI ADD command
cat /etc/cni/net.d/10-toycni.conf | CNI_COMMAND=ADD CNI_CONTAINERID=testing123 CNI_NETNS=/var/run/netns/testing \
    CNI_IFNAME=eth0 CNI_PATH=/opt/cni/bin /opt/cni/bin/toycni /var/ns/testing

# CNI DEL command
cat /etc/cni/net.d/10-toycni.conf | CNI_COMMAND=DEL CNI_CONTAINERID=testing123 CNI_NETNS=/var/run/netns/testing \
    CNI_IFNAME=eth0 CNI_PATH=/opt/cni/bin /opt/cni/bin/toycni /var/ns/testing

# Delete network namespace
ip netns del testing
```

#!/bin/bash
set -euo pipefail

# Path to directory containing this script.
SCRIPT_DIR="$(dirname "$(realpath "$0")")"

# Array for easier iteration.
NODES=("node01" "node02")

# Create VMs and install dependencies.
for NODE in "${NODES[@]}"
do
  echo "Creating VM $NODE"
  multipass launch --mem 2G --cpus 2 --name "$NODE" --disk 12G --cloud-init "$SCRIPT_DIR/resources/cloud-init.yaml" focal
done

# Setup cluster with kubeadm.
# Skip mark-control-plane prevents control-plane taint thus allowing pods to be scheduled on master.
echo "Bootstrapping control plane on node01"
# We skip phase mark-control-plane because we want workloads to get scheduled on the control plane node.
multipass exec node01 -- sudo kubeadm init --skip-phases mark-control-plane
JOIN_COMMAND=$(multipass exec node01 -- sudo kubeadm token create --print-join-command)
echo "Bootstrapping worker on node02"
multipass exec node02 -- bash -c "sudo $JOIN_COMMAND"

# Setup CNI.
for NODE in "${NODES[@]}"
do
	# Write CNI config.
	echo "Writing CNI config on node $NODE"
  multipass transfer "$SCRIPT_DIR/resources/toycni-$NODE.conf" "$NODE:10-toycni.conf"
  multipass exec "$NODE" -- sudo mkdir -p /etc/cni/net.d/ /opt/cni/bin/
  multipass exec "$NODE" -- sudo cp 10-toycni.conf /etc/cni/net.d/
  # Place toycni binary.
  echo "Transferring toycni binary to node $NODE"
  multipass transfer "$SCRIPT_DIR/../bin/toycni" "$NODE:"
  multipass exec "$NODE" chmod +x  toycni
  multipass exec "$NODE" -- sudo cp toycni /opt/cni/bin/
done

# Prepare kubeconfig.
multipass exec node01 -- bash -c 'mkdir -p $HOME/.kube && sudo cp /etc/kubernetes/admin.conf $HOME/.kube/config'
multipass exec node01 -- bash -c 'sudo chown $(id -u):$(id -g) $HOME/.kube/config'
echo "Writing kubeconfig to $SCRIPT_DIR/kubeconfig on host"
multipass transfer node01:.kube/config "$SCRIPT_DIR/kubeconfig"

# Create routes for cross-node communication.
IP_NODE01=$(multipass exec node01 -- ip route get 8.8.8.8 | head -1 | cut -d' ' -f7)
IP_NODE02=$(multipass exec node02 -- ip route get 8.8.8.8 | head -1 | cut -d' ' -f7)
echo "IP of node01 is $IP_NODE01"
echo "IP of node02 is $IP_NODE02"
echo "Configuring routes on VMs"
multipass exec node01 -- sudo ip route add 10.10.11.0/24 via "$IP_NODE02" dev enp0s2
multipass exec node02 -- sudo ip route add 10.10.10.0/24 via "$IP_NODE01" dev enp0s2

# Configure masquerading for external traffic.
echo "Configuring masquerading on VMs"
for NODE in "${NODES[@]}"
do
  # Do not masquerade RFC 1918 subnets.
  multipass exec "$NODE" -- sudo iptables -t nat -A POSTROUTING -d 10.10.10.0/8,172.16.0.0/12,192.168.0.0/16 -j RETURN
  # Masquerade everything else leaving primary interface.
  multipass exec "$NODE" -- sudo iptables -t nat -A POSTROUTING -o enp0s2 -j MASQUERADE
done

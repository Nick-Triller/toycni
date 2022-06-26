#!/bin/bash
set -euo pipefail

# Path to directory containing this script.
SCRIPT_DIR="$(dirname "$(realpath "$0")")"

for NODE in node01 node02
do
  echo "Deleting VM $NODE"
	multipass delete --purge $NODE || true
done

KCONFIG="$SCRIPT_DIR/kubeconfig"
echo "Deleting kubeconfig $KCONFIG"
rm "$KCONFIG" || true

#!/bin/bash

##############################################################################################################
# Name: 00-start-k8s.sh                                                                                      #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to start a multi-node local Minikube cluster.                                   #
############################################################################################################## 

echo -e "[INFO] Starting Minkube provisioning script v1.0"

echo -e "[INFO] Checking if minikube is installed..."
if command -v minikube &>/dev/null; then
    echo -e "[INFO] ...minikube is installed."
else
    echo -e "[ERROR] ...minikube is not installed! Please follow these instructions and launch the script again : https://minikube.sigs.k8s.io/docs/start/?arch=%2Flinux%2Fx86-64%2Fstable%2Fbinary+download"
    exit 1
fi

# Bootstraping K8S Cluster - Minikube flavour

## Minikube cluster creation
echo -e "\n[INFO] Starting Minikube cluster..."
minikube start \
    --install-addons=false \
    --driver docker \
    --cpus 4 \
    --memory 4096 \
    --container-runtime docker \
    --gpus all \
    --kubernetes-version v1.33.5 \
    --network-plugin cni \
    --cni false \
    --nodes 3 \
    --extra-config kubelet.node-ip=0.0.0.0 \
    --extra-config=kube-proxy.skip-headers=true
echo -e "[INFO] ...done"

## Mounting bpffs
echo -e "\n[INFO] Mounting BPFS filesystem into the containers..."
minikube ssh -n minikube "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
minikube ssh -n minikube-m02 "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
minikube ssh -n minikube-m03 "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
echo -e "[INFO] ...done"


## Mounting bpffs
echo -e "\n[INFO] Mounting bpffs filesystem on all minikube nodes..."

# Get all Kubernetes node names (they match minikube node names)
NODES=$(kubectl get nodes -o name | sed 's|node/||')

for NODE in $NODES; do
  echo "[INFO] Checking bpffs on node: $NODE"
  minikube ssh -n "$NODE" -- "grep -q 'bpffs /sys/fs/bpf' /proc/mounts || sudo mount -t bpf bpffs /sys/fs/bpf" || \
    echo "[WARN] Failed to mount bpffs on node: $NODE"
done

echo -e "[INFO] ...done"


# Applying master nodes taint only if the clutser has worker nodes
echo "[INFO] Checking worker nodes..."

# Workers = nodes without master/control-plane role
WORKERS=$(kubectl get nodes -l '!node-role.kubernetes.io/master,!node-role.kubernetes.io/control-plane' -o name 2>/dev/null || true)

if [ -z "$WORKERS" ]; then
  echo "[INFO] No worker nodes found, not tainting master/control-plane nodes."
  exit 0
fi

echo "[INFO] Worker nodes detected:"
echo "$WORKERS"

# Masters / control-plane nodes
MASTERS=$(
  (
    kubectl get nodes -l node-role.kubernetes.io/master= -o name 2>/dev/null
    kubectl get nodes -l node-role.kubernetes.io/control-plane= -o name 2>/dev/null
  ) | sort -u
)

if [ -z "$MASTERS" ]; then
  echo "[WARN] No master/control-plane nodes found, nothing to taint."
  exit 0
fi

for NODE in $MASTERS; do
  NAME=${NODE#node/}
  echo "[INFO] Applying taints to node $NAME"
  # New-style control-plane taint
  kubectl taint node "$NAME" node-role.kubernetes.io/control-plane=:NoSchedule --overwrite
  # Legacy master taint (best-effort)
  kubectl taint node "$NAME" node-role.kubernetes.io/master=:NoSchedule --overwrite || true
done

echo "[INFO] Done."

# NVidia Operator deployent strategy
echo -e "\n[INFO] Selecting the NVIDIA GPU node..."

# Get master / control-plane nodes
MASTERS=$(
  (
    kubectl get nodes -l node-role.kubernetes.io/master= -o name 2>/dev/null
    kubectl get nodes -l node-role.kubernetes.io/control-plane= -o name 2>/dev/null
  ) | sort -u
)

# Get worker nodes (nodes without master/control-plane role)
WORKERS=$(
  kubectl get nodes -l '!node-role.kubernetes.io/master,!node-role.kubernetes.io/control-plane' -o name 2>/dev/null || true
)

# Choose target GPU node:
# - Prefer a worker if any
# - Otherwise use a master/control-plane node
if [ -n "$WORKERS" ]; then
  TARGET_NODE=$(echo "$WORKERS" | head -n1)
  echo "[INFO] Workers detected, limiting GPU operands to worker node: $TARGET_NODE"
elif [ -n "$MASTERS" ]; then
  TARGET_NODE=$(echo "$MASTERS" | head -n1)
  echo "[INFO] Only master/control-plane nodes detected, limiting GPU operands to master node: $TARGET_NODE"
else
  # Fallback: no role labels, pick the first node
  TARGET_NODE=$(kubectl get nodes -o name | head -n1)
  echo "[WARN] No role labels detected, falling back to first node: $TARGET_NODE"
fi

# Disable GPU operands on all nodes
echo "[INFO] Disabling GPU operands on all nodes..."
kubectl label nodes --all nvidia.com/gpu.deploy.operands=false --overwrite

# Enable GPU operands on the selected node (remove the disabling label)
echo "[INFO] Enabling GPU operands on selected node: $TARGET_NODE"
kubectl label "$TARGET_NODE" nvidia.com/gpu.deploy.operands-

echo -e "[INFO] Done."

echo -e "[INFO] Minikube cluster deployed. \n"
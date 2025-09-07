# -*- mode: ruby -*-
# vi: set ft=ruby :

# Global configuration
NODES_PROVIDER = ENV.fetch("VAGRANT_NODES_PROVIDER", "virtualbox").to_sym
NODES_BOX_NAME = ENV.fetch("VAGRANT_NODES_BOX_NAME", "cloud-image/ubuntu-24.04")

WORKER_NODES_NUMBER = ENV.fetch("VAGRANT_WORKER_NODES_NUMBER", 3).to_i
WORKER_NODES_CPU = ENV.fetch("VAGRANT_WORKER_NODES_CPU", 4).to_i
WORKER_NODES_RAM = ENV.fetch("VAGRANT_WORKER_NODES_RAM", 4096).to_i
WORKER_NODES_DISKSIZE = ENV.fetch("VAGRANT_WORKER_NODES_DISKSIZE", "20GB")

CONTROL_NODES_NUMBER = ENV.fetch("VAGRANT_CONTROL_NODES_NUMBER", 1).to_i
CONTROL_NODES_CPU = ENV.fetch("VAGRANT_CONTROL_NODES_CPU", 2).to_i
CONTROL_NODES_RAM = ENV.fetch("VAGRANT_CONTROL_NODES_RAM", 2048).to_i
CONTROL_NODES_DISKSIZE = ENV.fetch("VAGRANT_CONTROL_NODES_DISKSIZE", "10GB")

unless Vagrant.has_plugin?("vagrant-disksize")
  raise "[ERROR] : Vagrant plugin `vagrant-disksize` is required to proceed with nodes provisioning." \
        " You can install it by typing the following command : `vagrant plugin install vagrant-disksize`."
end


Vagrant.configure(2) do |config|

  # Worker Nodes setup
  WORKER_NODES_IPS = []

  (1..WORKER_NODES_NUMBER).each do |i|

    worker_node_ip_address = "192.168.56.#{20 + i}"
    worker_node_ip_hostname = "worker#{i}"

    config.vm.define worker_node_ip_hostname do |wn|

      wn.vm.provider NODES_PROVIDER do |wnp|
        provision_node(wnp, WORKER_NODES_CPU, WORKER_NODES_RAM)
      end

      wn.disksize.size = WORKER_NODES_DISKSIZE
      wn.vm.box = NODES_BOX_NAME
      wn.vm.hostname = worker_node_ip_hostname
      wn.vm.network "private_network", ip: worker_node_ip_address
      #wn.vm.provision "shell", path: "scripts/provision-worker-node.sh"

      WORKER_NODES_IPS << worker_node_ip_address
    end
  end


  # Control Nodes setup
  CONTROL_NODES_IPS = []

  (1..CONTROL_NODES_NUMBER).each do |i|

    control_node_ip_address = "192.168.56.#{10 + i}"
    control_node_ip_hostname = "control#{i}"

    config.vm.define control_node_ip_hostname do |cn|

      cn.vm.provider NODES_PROVIDER do |cnp|
        provision_node(cnp, CONTROL_NODES_CPU, CONTROL_NODES_RAM)
      end
    
      cn.vm.box = NODES_BOX_NAME
      cn.vm.hostname = control_node_ip_hostname
      cn.disksize.size = CONTROL_NODES_DISKSIZE
      cn.vm.network "private_network", ip: control_node_ip_address

      CONTROL_NODES_IPS << control_node_ip_address
      
      # The last control node will be used to bootstrap the cluster, as the procedure needs to 
      # be aware of all the IPs of the nodes joining the cluster. Also, we're removing the trailing 
      # comma from the control nodes IP list if we reached the last node of the bucket.
      if i == CONTROL_NODES_NUMBER
        cn.vm.provision "shell", 
          path: "./scripts/control-nodes/01-setup_env.sh"
        cn.vm.provision "shell", 
          path: "./scripts/control-nodes/02-setup_ssh.sh",
          env: {
            "CLUSTER_NODES_IPS" => WORKER_NODES_IPS.join(",") + "," + CONTROL_NODES_IPS.join(",")
          }
      end
    end
  end
end

def provision_node(node_provider, cpus, memory)

  case NODES_PROVIDER
    when :virtualbox
      node_provider.customize ["modifyvm", :id, "--memory", memory]
      node_provider.customize ["modifyvm", :id, "--cpus", cpus]
      node_provider.customize ["modifyvm", :id, "--ioapic", "on"]

    when :libvirt 
      node_provider.memory = memory
      node_provider.cpus = cpus

    when :qemu 
      node_provider.memory = memory
      node_provider.smp = cpus
      
    else
      raise "[ERROR] : The provider you're trying to use is not compatible with the box used to provision nodes." \
            " Compatible providers are : `libvirt`, `qemu` and `virtualbox`."
  end
end
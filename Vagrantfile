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

    WORKER_NODES_IPS = ""
    CONTROL_NODES_IPS = ""

    (1..WORKER_NODES_NUMBER).each do |i|

      WORKER_NODE_IP_ADDRESS = "192.168.56.#{20 + i}"
      WORKER_NODE_HOSTNAME = "worker#{i}"

      config.vm.define WORKER_NODE_HOSTNAME do |wn|

        wn.vm.provider NODES_PROVIDER do |wnp|
          provision_node(wnp, WORKER_NODES_CPU, WORKER_NODES_RAM)
        end

        wn.disksize.size = WORKER_NODES_DISKSIZE
        wn.vm.box = NODES_BOX_NAME
        wn.vm.hostname = WORKER_NODE_HOSTNAME
        wn.vm.network "private_network", ip: WORKER_NODE_IP_ADDRESS
        #wn.vm.provision "shell", path: "scripts/provision-worker-node.sh"

        WORKER_NODES_IPS = WORKER_NODES_IPS + "," + WORKER_NODE_IP_ADDRESS
      end
    end


    (1..CONTROL_NODES_NUMBER).each do |i|

      CONTROL_NODE_IP_ADDRESS = "192.168.56.#{10 + i}"
      CONTROL_NODE_HOSTNAME = "control#{i}"

      config.vm.define CONTROL_NODE_HOSTNAME do |cn|

        cn.vm.provider NODES_PROVIDER do |cnp|
          provision_node(cnp, CONTROL_NODES_CPU, CONTROL_NODES_RAM)
        end
      
        cn.vm.box = NODES_BOX_NAME
        cn.vm.hostname = CONTROL_NODE_HOSTNAME
        cn.disksize.size = CONTROL_NODES_DISKSIZE
        cn.vm.network "private_network", ip: WORKER_NODE_IP_ADDRESS

        CONTROL_NODES_IPS = CONTROL_NODES_IPS + "," + CONTROL_NODE_IP_ADDRESS

        
        # The last control node will be used to bootstrap the cluster, as the procedure needs to 
        # be aware of all the IPs of the nodes joining the cluster
        if i = CONTROL_NODES_NUMBER
          cn.vm.provision "shell", 
            path: "./scripts/control-nodes/01-setup_env.sh",
            env: {
              "WORKER_NODES_IPS" => WORKER_NODES_IPS,
              "CONTROL_NODES_IPS" => CONTROL_NODES_IPS
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
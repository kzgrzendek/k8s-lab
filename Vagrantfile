# -*- mode: ruby -*-
# vi: set ft=ruby :

# Global configuration
BOX_NAME = "bento/ubuntu-24.04"

# Machines definition
worker_nodes=[
  {
    :hostname => "worker0",
    :ip => "192.168.56.20",
    :box => BOX_NAME,
    :ram => 4096,
    :cpu => 4
  },
  {
    :hostname => "worker1",
    :ip => "192.168.56.21",
    :box => BOX_NAME,
    :ram => 4096,
    :cpu => 4
  },
  {
    :hostname => "worker2",
    :ip => "192.168.56.22",
    :box => BOX_NAME,
    :ram => 4096,
    :cpu => 4
  }
]

control_nodes=[
  {
    :hostname => "control0",
    :ip => "192.168.56.10",
    :box => BOX_NAME,
    :ram => 3072,
    :cpu => 3,
  }
]


Vagrant.configure(2) do |config|

  worker_nodes.each do |machine|

    config.vm.define machine[:hostname] do |node|

      node.vm.box = machine[:box]
      node.vm.hostname = machine[:hostname]
      node.vm.network "private_network", ip: machine[:ip]

      node.vm.provider "virtualbox" do |vb|
          vb.customize ["modifyvm", :id, "--memory", machine[:ram]]
      end
    end
  end

  control_nodes.each do |machine|

    config.vm.define machine[:hostname] do |node|

      node.vm.box = machine[:box]
      node.vm.hostname = machine[:hostname]
      node.vm.network "private_network", ip: machine[:ip]

      node.vm.provider "virtualbox" do |vb|
          vb.customize ["modifyvm", :id, "--memory", machine[:ram]]
      end

      node.vm.provision "shell", path: "scripts/provision-control-node.sh"
    end
  end

end
# Set the required provider and versions
terraform {
  required_providers {
    # We recommend pinning to the specific version of the Docker Provider you're using
    # since new versions are released frequently
    docker = {
      source  = "kreuzwerker/docker"
      version = "3.6.2"
    }
  }
}

# Configure the docker provider
provider "docker" {
}

resource "docker_image" "k0s" {
  name         = "ghcr.io/k0sproject/k0s:v1.33.4-k0s.0"
  keep_locally = true
}

resource "docker_volume" "worker1-openebs" {
  name = "worker1-openebs"
}

resource "docker_volume" "worker2-openebs" {
  name = "worker2-openebs"
}


resource "docker_container" "k0s-control1" {
  name          = "k0s-control1"
  image         = docker_image.k0s.image_id

  privileged    = true

  #cpus          = 2.0
  memory        = 2048

  volumes {
    volume_name     = "kmsg"
    host_path       = "/dev/kmsg"
    container_path  = "/dev/kmsg"
    read_only       = true 
  }

  tmpfs = {
    "/run"  = "size=1G"
    "/tmp"  = "size=4G"
  }

  ports {
    external = 6443
    internal = 6443
  }
}

resource "docker_container" "k0s-worker1" {
  name          = "k0s-worker1"
  image         = docker_image.k0s.image_id

  privileged    = true

  #cpus          = 4.0
  memory        = 4096

  volumes {
    volume_name     = "kmsg"
    host_path       = "/dev/kmsg"
    container_path  =  "/dev/kmsg"
    read_only       = true 
  }

  tmpfs = {
    "/run"  = "size=1G"
    "/tmp"  = "size=4G"
    "/var/openebs/local"  = "size=20G"
  }


}

resource "docker_container" "k0s-worker2" {
  name          = "k0s-worker2"
  image         = docker_image.k0s.image_id

  privileged    = true

  #cpus          = 4.0
  memory        = 4096

  volumes {
    volume_name     = "kmsg"
    host_path       = "/dev/kmsg"
    container_path  = "/dev/kmsg"
    read_only       = true 
  }

  tmpfs = {
    "/run"  = "size=1G"
    "/tmp"  = "size=4G"
    "/var/openebs/local"  = "size=20G"
  }

}
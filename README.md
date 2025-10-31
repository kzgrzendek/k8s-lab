# k0s-lab

## Prerequisites

- Vagrant is installed
  - plugin disksize
- Virtualbox is installed


## Install

### Certificates

#### K8S-LAB CA

```sh
cd 00-host-setup/resources/k8s-lab
openssl req -new -x509 -config k8s-lab-ca.conf -keyout k8s-lab-ca.key -out k8s-lab-ca.crt -days 3600 -nodes
```

### DNS Config
Add the following to `/etc/systemd/resolved.conf.d/local-bind9-minikube.conf` :

```txt
[Resolve]
DNS=127.0.0.1:30053
Domains=~k8s-lab.local
```

## IOLimits
Add the following to `/etc/sysctl.conf` :

```txt
fs.inotify.max_user_instances=2280
sudo sysctl fs.inotify.max_user_watches=1255360
```
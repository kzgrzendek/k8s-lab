# k0s-lab

## Prerequisites

- Vagrant is installed
  - plugin disksize
- Virtualbox is installed


## Install

### Certificates

### DNS Config
Add the following to `/etc/systemd/resolved.conf.d/local-k8s-lab-bind9-dns.conf` :

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
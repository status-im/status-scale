#!/bin/bash

# Update golang to latest version
apt-get purge golang
add-apt-repository -y ppa:gophers/archive
apt-get update
apt-get install -y golang-1.10-go
mv /usr/lib/go-1.10/bin/* /usr/local/go/bin/
rm -rf /usr/local/go/src
ln -s /usr/lib/go-1.10/src /usr/local/go/src


# Update docker
sudo apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
sudo apt-get update
sudo apt-get purge -y lxc-docker*
sudo apt-get install -y docker-engine
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo apt-add-repository 'deb https://apt.dockerproject.org/repo ubuntu-trusty main'
sudo apt-get update
sudo apt-get install -y docker-ce

# Update git
sudo add-apt-repository -y ppa:git-core/ppa
sudo apt-get update
sudo apt-get install -y git

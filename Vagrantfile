Vagrant.configure("2") do |config|
  config.vm.box = "coverit/golang-dev"
  config.vm.provision :shell, path: "bootstrap.sh"
  config.vm.synced_folder ".", "/vagrant", disabled: true
  config.vm.synced_folder "./", "/home/vagrant/go/src/github.com/status-im/status-scale"

  $script = <<-SCRIPT
  docker pull golang:1.10-alpine
  export GOPATH=/home/vagrant/go
  export GOROOT=/usr/lib/go-1.10/
  echo "export GOROOT=/usr/lib/go-1.10/" >> /home/vagrant/.profile
SCRIPT

  config.vm.provision "shell", inline: $script, privileged: false
end

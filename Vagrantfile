# -*- mode: ruby -*-
# vi: set ft=ruby :

# All Vagrant configuration is done below. The "2" in Vagrant.configure
# configures the configuration version (we support older styles for
# backwards compatibility). Please don't change it unless you know what
# you're doing.

$provision_script = <<SCRIPT
apt-get -y update
apt-get -y dist-upgrade
apt-get -y install wget
apt-get -y install build-essential zlib1g-dev libssl-dev libcurl4-openssl-dev libexpat1-dev gettext
wget --progress=dot:giga "https://www.kernel.org/pub/software/scm/git/git-2.9.3.tar.gz"
tar xzf git-2.9.3.tar.gz
(cd git-2.9.3 && make prefix=/usr/local install)
adduser git --disabled-password
ln -s /vagrant/gitlab-mirror /usr/local/bin/
ln -s /vagrant/gitlab-mirror.conf /home/git/
SCRIPT

Vagrant.configure(2) do |config|
  config.vm.box = "ubuntu/trusty64"

  config.vm.provider "virtualbox" do |vb|
    vb.memory = "1024"
  end

  config.vm.provision :shell, inline: $provision_script
end

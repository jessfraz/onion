#!/usr/bin/env bash
set -e

cd "$(dirname "$BASH_SOURCE")/.."
rm -rf vendor/
source 'hack/.vendor-helpers.sh'

# the following lines are in sorted order, FYI
clone git github.com/docker/docker 8ed14c207256836f86308f76d88fc800bb34e5f1
clone git github.com/docker/libnetwork bbd6e6d8ca1e7c9b42f6f53277b0bde72847ff90
clone git github.com/godbus/dbus 881625768bcd4ca46046c618551413fa51e07a0d
clone git github.com/gopher-net/dknet cf5ced3dd308df2c4fa13c71e009729f9551e53d
clone git github.com/opencontainers/runc v0.0.5
clone git github.com/samalba/dockerclient 4656b1bc6cbc06b75d65983475e4809cbd53ebb5
clone git github.com/Sirupsen/logrus v0.8.7
clone git github.com/vishvananda/netlink 3e530fc6dc11bcbfcb993b51d9cbc5b0279e4297

clean

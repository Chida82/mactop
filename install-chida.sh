#!/bin/sh
# Compila il fork e lo installa in /usr/local/bin/mactop (sudo chiede la password).
# Disinstallare: sudo rm /usr/local/bin/mactop
set -e
cd "$(dirname "$0")"
go build -o mactop .
sudo install -o root -g wheel -m 755 mactop /usr/local/bin/mactop
echo "installato: $(/usr/local/bin/mactop --version)"

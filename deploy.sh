#!/bin/bash
# Run on EC2 server: bash deploy.sh
set -e

# Create directories
sudo mkdir -p /usr/local/mdm/nanoca/pki
sudo mkdir -p /usr/local/mdm/nanoca/data

# Copy binary
sudo cp nanoca /usr/local/bin/nanoca
sudo chmod +x /usr/local/bin/nanoca

# Copy PKI files
sudo cp pki/rootCA.crt     /usr/local/mdm/nanoca/pki/
sudo cp pki/rootCA-pkcs8.key /usr/local/mdm/nanoca/pki/
sudo chmod 600 /usr/local/mdm/nanoca/pki/rootCA-pkcs8.key

# Install systemd service
sudo cp nanoca.service /etc/systemd/system/nanoca.service
sudo systemctl daemon-reload
sudo systemctl enable nanoca
sudo systemctl start nanoca
sudo systemctl status nanoca

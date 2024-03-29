# Raspberry Pi Network Share Setup Script
This script configures a network share drive on a raspberry pi. It requires sudo priviledges. 
You can run the following command to get the drive path required for the `-d` option: `lsblk -f`
You will also need to restart the PI after the script finishes running.

## Format Drive
You should format the usb drive before executing this script, which can be done with the following commands:
```
sudo apt-get install dosfstools
sudo mkfs.vfat {{path_to_drive}} -n USB
```

## Prerequisites
The script will install the following prerequisites:
- nfs-server
- nfs-common
- autofs
- samba
- samba-common-bin

## Usage:
```
Usage: raspiNetworkShareSetup [options...]

Options:
	-n	share name. Defaults to "smb-pi"
	-d	usb drive path (required)
	-u	user id used for file ownership (required)
	-g	group id used for file ownership (required)

** should be run with root priviledges
```

#!/usr/bin/env bash

# source https://github.com/rook/rook/blob/v1.19.1/tests/scripts/create-bluestore-partitions.sh

set -ex

#############
# VARIABLES #
#############
SIZE=${PARTITION_SIZE:-2048M}  # Default to 2048M, can be overridden via environment variable

#############
# FUNCTIONS #
#############

function usage {
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Create bluestore partitions for Ceph OSDs"
  echo ""
  echo "Options:"
  echo "  --disk DEVICE           Disk device to partition (required)"
  echo "  --bluestore-type TYPE   Bluestore type: block.db or block.wal"
  echo "  --osd-count COUNT       Number of OSDs to create (default: 1)"
  echo "  --wipe-only             Only wipe the disk without creating partitions"
  echo "  -h, --help              Show this help message"
  echo ""
  echo "Examples:"
  echo "  $0 --disk /dev/sda --bluestore-type block.db"
  echo "  $0 --disk /dev/sda --osd-count 2"
  echo "  $0 --disk /dev/sda --wipe-only"
  echo ""
  echo "Environment Variables:"
  echo "  PARTITION_SIZE          Size of partitions (default: 2048M)"
  echo ""
}

function wipe_disk {
  sudo sgdisk --zap-all -- "$DISK"
  sudo dd if=/dev/zero of="$DISK" bs=1M count=10
  if [ -n "$WIPE_ONLY" ]; then
    # `parted "$DISK -s print" exits with 1 if the partition label doesn't exist.
    # It's no problem in "--wipe-only" mode
    sudo parted "$DISK" -s print || :
    return
  fi
  set +e
  sudo parted -s "$DISK" mklabel gpt
  set -e
  sudo partprobe -d "$DISK"
  sudo udevadm settle
  sudo parted "$DISK" -s print
}

function create_partition {
  sudo sgdisk --new=0:0:+"$SIZE" --change-name=0:"$1" --mbrtogpt -- "$DISK"
}

function create_block_partition {
  local osd_count=$1
  if [ "$osd_count" -eq 1 ]; then
    sudo sgdisk --largest-new=0 --change-name=0:'block' --mbrtogpt -- "$DISK"
  elif [ "$osd_count" -gt 1 ]; then
    SIZE=6144M
    for osd in $(seq 1 "$osd_count"); do
      echo "$osd"
      create_partition osd-"$osd"
      echo "SUBSYSTEM==\"block\", ATTR{size}==\"12582912\", ATTR{partition}==\"$osd\", ACTION==\"add\", RUN+=\"/bin/chown 167:167 ${DISK}${osd}\"" | sudo tee -a /etc/udev/rules.d/01-rook-"$osd".rules
    done
  fi
}

########
# MAIN #
########
if [ "$#" -lt 1 ]; then
  echo "Error: No arguments provided" >&2
  usage
  exit 1
fi

while [ "$1" != "" ]; do
  case $1 in
  --disk)
    shift
    DISK="$1"
    ;;
  --bluestore-type)
    shift
    BLUESTORE_TYPE="$1"
    ;;
  --osd-count)
    shift
    OSD_COUNT="$1"
    ;;
  --wipe-only)
    WIPE_ONLY=1
    ;;
  -h | --help)
    usage
    exit
    ;;
  *)
    usage
    exit 1
    ;;
  esac
  shift
done

# Validate required parameters
if [ -z "$DISK" ]; then
  echo "Error: --disk parameter is required" >&2
  usage
  exit 1
fi

if [ ! -b "$DISK" ]; then
  echo "Error: $DISK is not a valid block device" >&2
  exit 1
fi

# First wipe the disk
wipe_disk

if [ -n "$WIPE_ONLY" ]; then
  exit
fi

if [ -z "$WIPE_ONLY" ]; then
  # Set default OSD count if not specified
  OSD_COUNT=${OSD_COUNT:-1}

  if [ -n "$BLUESTORE_TYPE" ]; then
    case "$BLUESTORE_TYPE" in
    block.db)
      create_partition block.db
      ;;
    block.wal)
      create_partition block.db
      create_partition block.wal
      ;;
    *)
      echo "Error: Invalid bluestore configuration '$BLUESTORE_TYPE'" >&2
      echo "Valid options: block.db, block.wal" >&2
      exit 1
      ;;
    esac
  fi

  # Create final block partitions
  create_block_partition "$OSD_COUNT"
fi

# Inform the kernel of partition table changes
sudo partprobe "$DISK"

# Wait the udev event queue, and exits if all current events are handled.
sudo udevadm settle

# Print drives
sudo lsblk
sudo parted "$DISK" -s print

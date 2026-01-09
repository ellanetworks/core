---
description: Instructions to tune network performance.
---

This guide describes the steps and settings required to tune network
performance for Ella Core. It was written with 10Gbps NICs in mind. The exact
values required will depend on the hardware used.

## Configure transmit queue length

Increase the transmit queue length to 10000 by adding the following line
to `/etc/udev/rules.d/60-ella-core.rules` for each interfaces handling N3 or
N6 traffic:

```
KERNEL=="enp4s0f0", RUN+="/sbin/ip link set %k txqueuelen 10000"
```

## Configure multiqueue

For all interfaces handling N3 or N6 traffic, configure multiple queues. The
appropriate number of queues is the larger between the number of cores and the
maximum number of queues supported by the NIC:

```bash
nic=enp4s0f0
nproc=$(nproc)
max_allowed=$(sudo ethtool -l $nic | grep Combined | awk '{ print $2 }' | sort -n | head -n 1)
if [ $nproc -ge $max_allowed ]; then
    echo $nproc
else
    echo $max_allowed
fi
```

Assuming the above script output was 16, add the following line to
`/etc/udev/rules.d/60-ella-core.rules`:

```
KERNEL=="enp4s0f0", RUN+="/usr/sbin/ethtool -L %k combined 16"
```

## Increase the NIC ring buffer sizes

Find the maximum allowed ring buffer sizes allowed by your hardware:

```bash
sudo ethtool -g enp4s0f0
```

The output should be similar to this:

```
Ring parameters for enp4s0f0:
Pre-set maximums:
RX:			8192
RX Mini:		n/a
RX Jumbo:		n/a
TX:			8192
TX push buff len:	n/a
Current hardware settings:
RX:			512
RX Mini:		n/a
RX Jumbo:		n/a
TX:			512
RX Buf Len:		n/a
CQE Size:		n/a
TX Push:		off
RX Push:		off
TX push buff len:	n/a
TCP data split:		n/a
```

Add the following line to `/etc/udev/rules.d/60-ella-core.rules`, using
the values from the `Pre-set maximums` section:

```
KERNEL=="enp4s0f0", RUN+="/usr/sbin/ethtool -G %k rx 8192 tx 8192"
```

## Tune maximum backlog in the receive queues

Increase the maximum backlog in receive queues by adding the following
content to `/etc/sysctl.d/99-ella-core.conf`:

```
# increase the maximum backlog
net.core.netdev_max_backlog = 182757
```

## Apply the changes

The changes in this guide can be applied by either rebooting, or running the
following commands:

```bash
sudo udevadm trigger
sudo sysctl --system
```

Ella Core UPF XDP implementation
================================

This directory contains the XDP implementation of the UPF. It contains
both the userspace go code and the kernel space XDP code.

Compilation
===========

In most cases, you should use `go generate` from the top-level directory
to build the project.

For development of the C XDP code, cmake can be used to create files to
support the `clangd` LSP. Run the following command in this directory to
generate the files:

`cmake .`

Inspecting UPF state
====================

Maps used to drive the XDP code can be inspected with `bpftool`.

For example, it is possible to list all the uplink PDRs with the following
command:


```shell
sudo bpftool map dump name pdrs_uplink
```

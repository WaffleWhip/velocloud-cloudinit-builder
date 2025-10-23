# CloudInit Builder

CloudInit Builder is a portable Windows utility that automates the creation of `cloud-init.iso` files and validates them against a lightweight virtual machine. It was designed to streamline VeloCloud testing workflows, but it works for any cloud-init payload you want to inject into a VM.

The executable ships as a single file. When run, it pulls down self-contained copies of Podman and QEMU, keeps them in the same working directory, and never touches system-wide installations or the registry.

## Requirements

- 64-bit Windows 10/11 with virtualization enabled in firmware.
- Internet access on first run (for downloading portable Podman, Debian container layers, and QEMU).
- A base QCOW2 disk image placed at `images/velocloud.qcow2` (create the folder if it does not exist yet).
- Validated with VeloCloud 4.5.0; other software versions have not been smoke-tested yet.

## Quick Start

1. Create an empty folder and copy `cloudinit-builder.exe` into it.
2. Add your base image at `images/velocloud.qcow2`.
3. Double-click the executable (or run it without arguments from PowerShell).
4. Choose **Build cloud-init ISO** to produce `images/cloud-init.iso`.
5. (Optional) choose **Jalankan VM test** to boot the generated ISO against a cloned disk.

The interactive menu is intentionally simple. Prompts are shown in Indonesian, but the actions are self-explanatory:

```
=== CloudInit Builder ===
1) Build cloud-init ISO
2) Jalankan VM test
3) Uninstall & bersihkan
4) Keluar
Pilih menu [1-4]:
```

## Workflow Overview

- **Build cloud-init ISO**  
  Ensures the folders `templates/`, `images/`, `runtime/`, `tools/`, `cache/`, and `logs/` exist. Downloads portable Podman as needed, starts a dedicated Podman machine, pulls the Debian Bookworm image, installs `genisoimage`, and writes `images/cloud-init.iso` from `templates/user-data.txt` and `templates/meta-data.txt`.

- **Jalankan VM test**  
  Downloads a portable QEMU bundle the first time you run it (cached afterwards). The base QCOW2 is copied into `runtime/vm/velocloud-<timestamp>.qcow2`, attached together with `images/cloud-init.iso`, and launched with 4 GiB RAM, two vCPUs, NAT networking, and a virtio NIC. The cloned disk is deleted automatically when QEMU exits.

- **Uninstall & bersihkan**  
  Stops the Podman machine, removes `tools/`, `images/`, `runtime/`, `cache/`, `templates/`, and `logs/`, and optionally deletes the executable when invoked via CLI with `--self-delete`.

Every operation writes a timestamped log under `logs/` (for example `logs/build-20241023-134500.txt`).

## Command-Line Reference

The executable also exposes explicit commands for automation or CI:

```text
cloudinit-builder [-q|--quiet] build
cloudinit-builder [-q|--quiet] test [--vm <path-to-portable-vm>] [-- <extra-vm-args>]
cloudinit-builder [-q|--quiet] uninstall [--self-delete]
```

- `-q/--quiet` suppresses console progress messages while keeping log files intact.
- `test --vm` lets you supply a custom VM executable instead of the bundled QEMU.
- Extra arguments after `--` are passed directly to the VM executable.

## Template Customization

The first `build` run generates default templates if they are not present.

`templates/meta-data.txt`
```
instance-id: vce
local-hostname: vce
```

`templates/user-data.txt`
```
#cloud-config
hostname: vce
password: Velocloud123
chpasswd: {expire: False}
ssh_pwauth: True
```

Edit these files before rebuilding the ISO to inject custom users, SSH keys, configuration snippets, or cloud-init modules required by your lab.

## Directory Layout

```
./
|-- cloudinit-builder.exe
|-- cache/                     (downloaded ZIP archives)
|-- images/
|   |-- velocloud.qcow2        (base disk you provide)
|   `-- cloud-init.iso         (generated ISO)
|-- logs/                      (operation transcripts)
|-- runtime/
|   `-- vm/                    (temporary QCOW clones)
|-- templates/
|   |-- user-data.txt
|   `-- meta-data.txt
`-- tools/
    |-- podman/...
    `-- qemu/...
```

You can move the entire folder to another machine and continue working; the tool reuses cached binaries if they are already present.

## Configuration Options

| Environment Variable              | Description                                               | Default |
|----------------------------------|-----------------------------------------------------------|---------|
| `CLOUDINIT_BUILDER_QEMU_ACCEL`   | Override the QEMU accelerator (`tcg`, `whpx`, `kvm`, ...) | `tcg`   |

Example (enable WHPX if available):

```powershell
$env:CLOUDINIT_BUILDER_QEMU_ACCEL = "whpx"
cloudinit-builder.exe test
```

## Troubleshooting

- **Podman fails to start**: Ensure Hyper-V or WSL2 is enabled; Podman machine management requires at least one virtualization backend.
- **VM window closes immediately**: Check `logs/test-*.txt` for QEMU output. Invalid cloud-init syntax or missing ISO usually shows up there.
- **Download errors**: Verify that outbound HTTPS traffic is allowed. Re-running the same action safely retries the download and resumes cached artifacts.
- **Wrong base disk path**: Confirm that `images/velocloud.qcow2` exists and is a regular file; the tool will refuse to overwrite it.

## Building from Source

The repository includes the Go sources that produce `cloudinit-builder.exe`. To build your own binary:

```powershell
go build -o cloudinit-builder.exe ./cmd/cloudinit-builder
```

The generated executable is fully self-contained and can replace the prebuilt binary included in this project folder.

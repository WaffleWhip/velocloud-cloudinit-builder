# CloudInit Builder

Portable Windows utility for producing `cloud-init.iso` images and validating them against a lightweight QEMU smoke test—no installers, admin rights, or external dependencies required.

## Highlights

- **Self-contained toolchain** – Downloads portable Podman and QEMU builds into the working directory and keeps them isolated from the rest of the system.
- **Guided workflow** – Launch the executable and follow the interactive menu to build the ISO, run a VM test, or clean everything up. No CLI knowledge needed.
- **Disposable VM disks** – The base QCOW2 image in `images/` is never mutated; every test run clones it into `runtime/vm/` and removes the clone once QEMU exits.
- **One-click clean-up** – Uninstall menu option shuts down the Podman machine, wipes generated artifacts, and optionally deletes the executable itself.

## Getting Started

1. **Download & prepare**
   - Copy `cloudinit-builder.exe` into an empty folder on your Windows workstation.
   - Place your reference QCOW2 disk at `images/velocloud.qcow2` (create the `images/` folder if it does not yet exist). The first ISO build will also generate default templates under `templates/`—edit them after the initial run if you need custom cloud-init data.

2. **Launch the application**
   - Double-click `cloudinit-builder.exe` (or run it without arguments). You will see an interactive menu:
     ```
     1) Build cloud-init ISO
     2) Run VM smoke test
     3) Uninstall & clean up
     4) Exit
     ```

3. **Build the ISO (Menu 1)**
   - The tool ensures the directory structure exists, downloads portable Podman if needed, and runs `genisoimage` inside a Debian container. The result is written to `images/cloud-init.iso`. Build logs are stored in `logs/build-*.txt`.
   - After a successful build the menu offers to run the VM smoke test immediately.

4. **Run the VM smoke test (Menu 2 or post-build prompt)**
   - QEMU portable is downloaded on first use (cached afterwards).
   - The base QCOW2 disk is cloned into `runtime/vm/velocloud-<timestamp>.qcow2`, attached to QEMU together with `images/cloud-init.iso`, and booted with 4 GiB RAM, two vCPUs, and a single NAT-backed virtio NIC (DHCP enabled).
   - The temporary clone is deleted automatically once QEMU exits. Console output is captured under `logs/test-*.txt`.
   - When asked for a VM executable path, press **Enter** to use the bundled QEMU or provide a custom path if preferred.

5. **Uninstall / clean (Menu 3)**
   - Stops and deletes the CloudInit Builder Podman machine.
   - Removes `tools/`, `images/`, `runtime/`, `cache/`, `templates/`, and—after logging—`logs/`.
   - When launched via CLI you may add `--self-delete` to remove the executable after cleanup.

6. **Exit (Menu 4)** – Simply closes the application.

> Tip: hold **Shift + Right Click** inside the working folder and choose “Open PowerShell window here” if you want to run the executable with flags (see below) while still benefiting from the interactive menu.

## Optional CLI & Flags

Although the interactive menu is the recommended experience, the binary still exposes the original commands:

```text
cloudinit-builder [-q|--quiet] build
cloudinit-builder [-q|--quiet] test [--vm "<path-to-vm-executable>"] [-- <extra-qemu-args>]
cloudinit-builder [-q|--quiet] uninstall [--self-delete]
```

- `-q/--quiet` suppresses console status output (logs remain unchanged).
- `--vm` lets you point the smoke test to another hypervisor executable.
- Additional arguments after `--` are forwarded to the VM executable.

## Configuration Knobs

| Environment Variable | Purpose | Default |
| -------------------- | ------- | ------- |
| `CLOUDINIT_BUILDER_QEMU_ACCEL` | Override the QEMU accelerator (e.g. `whpx`, `tcg`, `kvm`). | `tcg` |

Example:

```powershell
$env:CLOUDINIT_BUILDER_QEMU_ACCEL = "whpx"
cloudinit-builder.exe
```

## Directory Layout

```
./
|-- cloudinit-builder.exe
|-- tools/
|   |-- podman/…
|   `-- qemu/…
|-- templates/
|   |-- user-data.txt
|   `-- meta-data.txt
|-- images/
|   |-- velocloud.qcow2        (base disk you provide)
|   `-- cloud-init.iso         (generated output)
|-- runtime/
|   `-- vm/                    (temporary QCOW clones)
|-- cache/                     (downloaded ZIP archives)
`-- logs/
```

## Template Defaults

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

Feel free to edit these files before rebuilding the ISO to fit your deployment scenario.

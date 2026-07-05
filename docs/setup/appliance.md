# OffGrid Appliance Deployment

OffGrid can run as a local AI appliance for classrooms, libraries, clinics,
community centers, and field teams. The goal is simple: a local box on the LAN
that people can use through a browser even when internet access is weak,
expensive, or unavailable.

## Product Shape

Think of the system in two layers:

- OffGrid LLM is the engine: model management, chat, RAG, API, CLI, and web UI.
- OffGrid Box is the appliance: hardware profile, local portal, offline content,
  repair flow, and USB-based updates.

The appliance command helps plan and prepare these boxes:

```bash
offgrid appliance status
offgrid appliance plan hub
offgrid appliance profile lite
offgrid appliance init hub
```

## Hardware Profiles

| Profile | Native Target | Recommended Hardware | Best Use |
|---------|---------------|----------------------|----------|
| `lite` | Linux arm64 | Raspberry Pi 5 16GB, 256GB+ SSD, active cooling | Small classrooms, demos, workshops |
| `hub` | Linux x86_64 | Refurbished mini PC, 32GB RAM, 1TB SSD | Schools, libraries, clinics, local knowledge hubs |
| `gpu` | Linux x86_64 + NVIDIA | 32GB+ RAM, NVIDIA GPU with 12GB+ VRAM | Multi-user labs, coding clubs, faster 7B/8B models |
| `jetson` | JetPack Linux arm64 | Jetson Orin Nano class device, 256GB+ NVMe | Robotics, camera, voice, edge AI prototypes |

## Recommended Starting Point

For broad community deployment, start with `hub`.

Minimum:

- Refurbished x86_64 mini PC
- 32GB RAM
- 1TB SSD
- Ethernet
- Local Wi-Fi router or access point
- Ubuntu Server or Debian

This profile gives enough room for 3B and 7B quantized models, document search,
and several users over a local network.

## Classroom Flow

1. Install Linux on the device.
2. Install OffGrid.
3. Initialize the appliance profile.
4. Import or download models once.
5. Add local documents and learning packs.
6. Start the server.
7. Students connect to the local portal.

```bash
offgrid appliance init hub
offgrid import /media/usb
offgrid kb add ./learning-packs/
offgrid serve
```

Users open:

```text
http://offgrid.local
```

If local DNS is not configured yet, use the device IP address and port:

```text
http://<device-ip>:11611
```

## Model Guidance

| Hardware | Models |
|----------|--------|
| 8GB RAM | `tiny`, `phi`, small 3B Q4 models |
| 16GB RAM | `llama3`, `qwen`, Gemma-class small models |
| 32GB RAM | `mistral`, `codellama`, 7B/8B Q4 models |
| GPU 12GB+ VRAM | Faster 7B/8B inference and vision experiments |

For schools and community centers, prefer fast and helpful over large and slow.
The best local AI box is the one that answers quickly enough that people keep
using it.

## Offline Content Packs

The appliance becomes much more useful when it ships with local knowledge:

- basic math and science
- coding lessons
- agriculture guides
- public health documents
- teacher support material
- exam prep
- local-language notes
- local policy and service information

Add content with:

```bash
offgrid kb add ./content/
```

## Field Requirements

Before distributing a box, verify:

- it boots without a keyboard or monitor
- `offgrid serve` starts on boot
- the portal is reachable on the LAN
- at least one model works offline
- the knowledge base has useful local content
- logs do not fill the disk
- power loss does not corrupt the setup
- there is a USB update and repair process

## Product Roadmap

The next product milestones are:

- `offgrid appliance repair`
- `offgrid appliance update-usb`
- local `offgrid.local` setup helper
- admin dashboard for storage, users, models, and health
- first-run web onboarding
- read-only student mode
- signed offline update bundles

# Authorized generated-build cleanup — 2026-11-23

The user authorized removal of unnecessary files to make room for Chromium
qualification. Cleanup was limited to reviewed generated directories inside
`D:\offgrid-llm\build`. Resolved targets, ancestry, links and active process paths
were checked before removal with native PowerShell `Remove-Item -LiteralPath`.

| Removed material | Count | Logical bytes reclaimed |
| --- | ---: | ---: |
| Repeated speech prepare staging directories | 126 | 11.9529 GiB |
| Obsolete unpacked desktop build directories | 63 | 57.9769 GiB |
| Byte-verified duplicate prior speech runtimes | 31 | 1.1635 GiB |

Total logical removal: approximately **71.09 GiB**. Final observed D: free space
was **96.02 GiB**; concurrent builds affect the exact free-space reading.

Source and uncommitted work, models, installed-user data, backups, failure logs,
source snapshots, outer installers and the two newest enumerated preview packages
were retained. Small package verification manifests were copied beside each
removed package; the complete path/size inventory is retained. Old unpacked
packages are no longer runnable at their historical paths, and the original
evidence must not imply those binary copies are still available. Their recorded
historical test results were not altered.

Audit records under `build/`: `cleanup-speech-scratch-20261123.json`,
`obsolete-packages-inventory-20261123.json`,
`removed-obsolete-packages-20261123.json`,
`duplicate-speech-runtimes-20261123.json` and
`removed-duplicate-speech-20261123.json`. The current `win-x64` runtime was
verified against every retained manifest digest before duplicate removal.

WSL Ubuntu's virtual disk is backed by `D:\WSL\Ubuntu`; its large logical free
space does not provide additional physical capacity. Native Chromium build
qualification is still unrun. Cleanup removes the initial 25 GiB free-space
constraint but does not prove the whole checkout/toolchain/build will fit.

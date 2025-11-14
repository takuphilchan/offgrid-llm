# Release Status - v0.9.0-rc1

**Date:** November 14, 2025  
**Status:** 🟡 In Progress - GitHub Actions Building

---

## ✅ Completed

1. **Code Changes**
   - ✅ Created universal installer (`install.sh`)
   - ✅ Created unified release workflow (`.github/workflows/release-unified.yml`)
   - ✅ Organized documentation (guides/, advanced/)
   - ✅ Added desktop app (Electron)
   - ✅ Cleaned up project structure
   - ✅ Updated README with single installation method

2. **Git Operations**
   - ✅ Committed all changes (868a79b)
   - ✅ Created tag v0.9.0-rc1
   - ✅ Pushed to GitHub

3. **GitHub Actions**
   - 🟡 Triggered (waiting for completion)
   - 🔗 https://github.com/takuphilchan/offgrid-llm/actions

---

## 🔄 In Progress

### GitHub Actions Workflow

**Expected to build:**
- [ ] offgrid-v0.9.0-rc1-linux-amd64-cpu.tar.gz
- [ ] offgrid-v0.9.0-rc1-linux-amd64-vulkan.tar.gz
- [ ] offgrid-v0.9.0-rc1-linux-arm64-cpu.tar.gz
- [ ] offgrid-v0.9.0-rc1-darwin-arm64-metal.tar.gz
- [ ] offgrid-v0.9.0-rc1-darwin-amd64-cpu.tar.gz
- [ ] offgrid-v0.9.0-rc1-windows-amd64-cpu.zip
- [ ] Desktop app - Linux (AppImage)
- [ ] Desktop app - macOS (DMG)
- [ ] Desktop app - Windows (exe)
- [ ] checksums-v0.9.0-rc1.sha256

**Monitor at:**
- Actions: https://github.com/takuphilchan/offgrid-llm/actions
- Release: https://github.com/takuphilchan/offgrid-llm/releases/tag/v0.9.0-rc1

---

## ⏭️ Next Steps

### If Build Succeeds ✅

1. **Test the installer:**
   ```bash
   VERSION=v0.9.0-rc1 ./install.sh
   # Verify offgrid and llama-server install correctly
   ```

2. **Test desktop app:**
   - Download AppImage/DMG/exe from release
   - Verify it runs and works

3. **Create official v1.0.0 release:**
   ```bash
   git tag -a v1.0.0 -m "v1.0.0 - Unified Installation System"
   git push origin v1.0.0
   ```

4. **Announce:**
   - GitHub Discussions
   - Update project description
   - Tweet/share (optional)

### If Build Fails ❌

1. **Check GitHub Actions logs:**
   - Click on failing job
   - Read error messages
   - Common issues:
     - CMake errors → Check dependencies
     - Vulkan SDK issues → Check installation
     - electron-builder errors → Check desktop/package.json

2. **Fix and retry:**
   ```bash
   # Make fixes
   git add .
   git commit -m "fix: resolve build issues"
   git tag -d v0.9.0-rc1
   git push --delete origin v0.9.0-rc1
   git tag -a v0.9.0-rc2 -m "RC2 with fixes"
   git push origin main v0.9.0-rc2
   ```

---

## 📊 Expected Timeline

- **Build time:** ~30-45 minutes (all platforms + desktop apps)
- **Test time:** ~15 minutes
- **Total to v1.0.0:** ~1 hour (if all goes well)

---

## 🧪 Testing Checklist

Once release is available:

### Installer Testing
- [ ] Linux AMD64 + Vulkan
- [ ] Linux AMD64 + CPU fallback
- [ ] macOS Apple Silicon
- [ ] Windows (if available)
- [ ] Checksum verification works
- [ ] Binaries install to /usr/local/bin
- [ ] Systemd setup works (Linux)

### Desktop App Testing
- [ ] AppImage runs on Linux
- [ ] DMG installs on macOS
- [ ] Servers start automatically
- [ ] UI loads correctly
- [ ] Can download models
- [ ] Can run inference

### Documentation
- [ ] README install command works
- [ ] Links to releases work
- [ ] Desktop app download links work

---

## 📝 Quick Commands

```bash
# Check build status
curl -s https://api.github.com/repos/takuphilchan/offgrid-llm/actions/runs | jq '.workflow_runs[0] | {status, conclusion}'

# Download release assets
gh release download v0.9.0-rc1

# Test installer
VERSION=v0.9.0-rc1 ./install.sh

# View logs if needed
gh run view --log
```

---

## 🎯 Success Criteria

Before declaring v1.0.0 ready:

1. ✅ All bundles build successfully
2. ✅ Desktop apps build successfully
3. ✅ Installer works on at least 2 platforms
4. ✅ Desktop app works on at least 1 platform
5. ✅ No critical bugs in fresh install
6. ✅ Documentation is accurate

---

**Update this file as you progress through testing!**

# nothingctl-glyph-helper Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

---

## Preamble — Read before starting

You are an **orchestrator**. Your job is to decompose every non-trivial task into focused sub-tasks and delegate them to specialized sub-agents. Never do heavy lifting (research, coding, review) yourself in the main context.

**Default agent roster** (spawn as needed, in parallel where possible):

| Agent | Type | Responsibility |
|-------|------|----------------|
| Researcher | `Explore` or `general-purpose` | Reads docs, searches the web, explores repos |
| Planner | `Plan` | Designs approach, identifies risks |
| Coder | `general-purpose` (isolation: worktree) | Implements — one per independent task group |
| Reviewer | `superpowers:code-reviewer` | Validates implementation against plan |

**Rule:** If a task touches more than 2 files OR requires external research OR has a security/correctness surface, spawn agents — do not handle it inline.

---

## Context — What this builds and why

**Parent project:** [`nothingctl`](https://github.com/Limplom/nothingctl) — a Go CLI tool that controls Nothing phones via ADB. It manages firmware flashing, Magisk root, partition backups, Glyph LED feedback, and deep device diagnostics.

**Problem being solved:**
nothingctl has a Glyph LED feedback system — during long operations like `backup`, the LEDs pulse to indicate progress, then turn off sequentially when done. On the **Nothing Phone 1**, this works via direct sysfs writes to the `aw210xx_led` kernel driver. On all newer devices (Phone 2, 2a, 3a, 3a Lite, 3), the kernel driver returns `EOPNOTSUPP` for direct writes — LED control is locked behind the proprietary Glyph SDK which requires an Android app context.

**Solution:**
A minimal Android Java helper (`nothingctl-glyph-helper`) that:
1. Uses the official Nothing Glyph SDKs to control LEDs
2. Runs via `app_process` (the mechanism used by tools like Shizuku) — no APK install needed
3. Is compiled to a standalone DEX/JAR
4. Gets embedded in the nothingctl Go binary and pushed to the device on demand
5. Is called by nothingctl's `feedback.go` with simple shell arguments

**New GitHub repo:** `Limplom/nothingctl-glyph-helper` (create this repo as the first step)

---

## Goal

**One sentence:** Build a minimal Android Java helper that wraps the Nothing Glyph SDK into a `app_process`-runnable DEX, and integrate it into nothingctl's Glyph feedback system so LED animations work on all supported Nothing devices.

**Architecture:**
The helper is a standalone Java project (Gradle) that produces a fat JAR/DEX. nothingctl embeds this DEX via Go's `//go:embed`, pushes it to `/data/local/tmp/` via ADB root, and invokes it via `adb shell su -c 'app_process -cp /data/local/tmp/glyph-helper.dex / com.nothingctl.GlyphHelper <cmd>'`. The helper detects the device at runtime and routes to the Glyph Developer Kit (Phone 1–3a) or GlyphMatrix Developer Kit (Phone 3).

**Tech Stack:**
- Java 11, Gradle 8, Android SDK (compile-only, no runtime Android framework needed beyond what's on the device)
- Nothing Glyph Developer Kit (`com.nothing.ketchum:glyph-sdk`)
- Nothing GlyphMatrix Developer Kit (for Phone 3)
- Go `//go:embed` for bundling the DEX into nothingctl
- `app_process` on-device runtime

---

## File Structure

### New repo: `nothingctl-glyph-helper/`

```
nothingctl-glyph-helper/
  app/
    src/main/java/com/nothingctl/
      GlyphHelper.java          ← main() entry point, CLI argument dispatch
      ZoneController.java       ← Glyph SDK wrapper (Phone 1–3a)
      MatrixController.java     ← GlyphMatrix SDK wrapper (Phone 3)
      DeviceInfo.java           ← device detection (ro.product.device)
      LightState.java           ← data class: zone ID, brightness, duration
    build.gradle                ← fat-JAR/DEX build config
  build.gradle                  ← root Gradle config
  settings.gradle
  README.md
  .github/
    workflows/
      build.yml                 ← CI: build DEX on every push, attach to release
```

### Changes in existing repo: `nothingctl/`

```
go/internal/glyph/
  feedback.go                   ← MODIFY: add helper-based fallback path
  helper.go                     ← NEW: push + invoke DEX via ADB root
  helper_dex.go                 ← NEW: //go:embed assets/glyph-helper.dex
go/internal/assets/
  glyph-helper.dex              ← NEW: embedded DEX built from the helper repo
```

---

## Tasks

---

### Task 1: Create the new GitHub repo and scaffold

**Spawn:** Coder agent (no isolation needed — new repo)

**Files:**
- Create: `nothingctl-glyph-helper/settings.gradle`
- Create: `nothingctl-glyph-helper/build.gradle`
- Create: `nothingctl-glyph-helper/app/build.gradle`
- Create: `nothingctl-glyph-helper/.github/workflows/build.yml`

- [ ] **Step 1: Create the GitHub repo**

```bash
gh repo create Limplom/nothingctl-glyph-helper \
  --public \
  --description "Android DEX helper for nothingctl — Glyph LED control via Nothing Glyph SDK" \
  --clone
cd nothingctl-glyph-helper
```

- [ ] **Step 2: Write `settings.gradle`**

```groovy
rootProject.name = "nothingctl-glyph-helper"
include ':app'
```

- [ ] **Step 3: Write root `build.gradle`**

```groovy
buildscript {
    repositories {
        google()
        mavenCentral()
    }
    dependencies {
        classpath 'com.android.tools.build:gradle:8.3.0'
    }
}

allprojects {
    repositories {
        google()
        mavenCentral()
        maven { url 'https://jitpack.io' }
    }
}
```

- [ ] **Step 4: Write `app/build.gradle` (fat-JAR/DEX target)**

Research the exact Maven coordinates for the Nothing Glyph SDK before filling in the dependency version. **Spawn a Researcher agent** to fetch:
- `https://github.com/Nothing-Developer-Programme/Glyph-Developer-Kit` — README, build.gradle, Maven coords
- `https://github.com/Nothing-Developer-Programme/GlyphMatrix-Developer-Kit` — same

Then write:

```groovy
plugins {
    id 'com.android.library'
}

android {
    compileSdk 34
    defaultConfig {
        minSdk 29
    }
    compileOptions {
        sourceCompatibility JavaVersion.VERSION_11
        targetCompatibility JavaVersion.VERSION_11
    }
}

dependencies {
    // Fill in correct coordinates from Researcher findings:
    compileOnly 'com.nothing.ketchum:glyph-sdk:<VERSION>'
    compileOnly 'com.nothing.ketchum:glyphmatrix-sdk:<VERSION>'
}

// Fat-DEX task: merge all classes into a single dex
task fatDex(type: Exec, dependsOn: 'assembleRelease') {
    // d8 merges the release AAR classes + deps into a single dex
    commandLine 'bash', '-c',
        'd8 --release --output build/outputs/dex/ ' +
        'build/intermediates/javac/release/classes'
}
```

- [ ] **Step 5: Create directory structure**

```bash
mkdir -p app/src/main/java/com/nothingctl
mkdir -p .github/workflows
```

- [ ] **Step 6: Initial commit**

```bash
git add .
git commit -m "chore: initial repo scaffold — Gradle + CI skeleton"
git push -u origin main
```

---

### Task 2: Research — Nothing Glyph SDK APIs

**Spawn:** Researcher agent

**Goal:** Understand the exact API calls needed from both SDKs. The Coder agent in Task 3 needs concrete method names, class names, and initialization flow.

**Researcher prompt:**

> Fetch and summarize the following:
> 1. `https://github.com/Nothing-Developer-Programme/Glyph-Developer-Kit` — README + any example Java/Kotlin files. Extract: how to initialize GlyphManager, how to open a session, how to set a GlyphFrame (all zones on, specific zone, specific brightness), how to close a session.
> 2. `https://github.com/Nothing-Developer-Programme/GlyphMatrix-Developer-Kit` — same for GlyphMatrix. Extract: how to light up all pixels, a subset, set brightness.
> 3. Which devices each SDK supports (exact model list).
> 4. Whether `app_process` or non-Activity contexts are documented as supported or unsupported.
> 5. The exact Gradle/Maven dependency coordinates and latest version for both SDKs.
>
> Return a summary with: class names, method signatures, initialization sequence, and shutdown sequence for each SDK. Under 400 words.

**Researcher findings feed directly into Task 3.**

---

### Task 3: Implement `DeviceInfo.java` + `GlyphHelper.java`

**Spawn:** Coder agent (worktree isolation)

**Prerequisite:** Task 2 (Researcher) complete — SDK API names must be known.

**Files:**
- Create: `app/src/main/java/com/nothingctl/DeviceInfo.java`
- Create: `app/src/main/java/com/nothingctl/GlyphHelper.java`
- Create: `app/src/main/java/com/nothingctl/LightState.java`

**Background for Coder agent:**
- `app_process` starts the Dalvik runtime and calls `main(String[] args)` on the specified class
- There is no Android `Context` available — only the system Binder is accessible
- The Nothing Glyph SDK may require a Context; if so, use `ActivityThread.systemMain().getSystemContext()` (reflection) to obtain a minimal system context — this is the standard `app_process` trick
- `ro.product.device` (lowercase) gives the hardware codename: `spacewar`=Phone 1, `pong`=Phone 2, `pacman`=Phone 2a, `galaxian`=Phone 3a/3a Lite, (check Phone 3 codename from research)

- [ ] **Step 1: Write `LightState.java`**

```java
package com.nothingctl;

public class LightState {
    public final int brightness; // 0–4095 for Glyph, 0–255 for Matrix
    public final long durationMs;

    public LightState(int brightness, long durationMs) {
        this.brightness = brightness;
        this.durationMs = durationMs;
    }
}
```

- [ ] **Step 2: Write `DeviceInfo.java`**

```java
package com.nothingctl;

import android.os.SystemProperties;

public class DeviceInfo {

    public enum GlyphType { ZONES, MATRIX, UNSUPPORTED }

    public static String codename() {
        return SystemProperties.get("ro.product.device", "unknown").toLowerCase();
    }

    public static GlyphType glyphType() {
        String device = codename();
        switch (device) {
            case "spacewar":   // Phone 1
            case "pong":       // Phone 2
            case "pacman":     // Phone 2a
            case "galaxian":   // Phone 3a / 3a Lite
                return GlyphType.ZONES;
            // Add Phone 3 codename here once confirmed by Researcher
            default:
                return GlyphType.UNSUPPORTED;
        }
    }
}
```

- [ ] **Step 3: Write `GlyphHelper.java` main entry point**

```java
package com.nothingctl;

public class GlyphHelper {

    private static final String CMD_ON    = "on";
    private static final String CMD_OFF   = "off";
    private static final String CMD_PULSE = "pulse";
    private static final String CMD_INFO  = "info";

    public static void main(String[] args) {
        if (args.length == 0) {
            printUsage();
            System.exit(1);
        }

        String cmd = args[0];
        DeviceInfo.GlyphType type = DeviceInfo.glyphType();

        if (cmd.equals(CMD_INFO)) {
            System.out.println("device=" + DeviceInfo.codename());
            System.out.println("glyph_type=" + type.name().toLowerCase());
            System.exit(0);
        }

        if (type == DeviceInfo.GlyphType.UNSUPPORTED) {
            System.err.println("[WARN] Device not supported: " + DeviceInfo.codename());
            System.exit(2);
        }

        try {
            switch (type) {
                case ZONES:
                    ZoneController ctrl = new ZoneController();
                    ctrl.init();
                    dispatch(ctrl, cmd, args);
                    ctrl.close();
                    break;
                case MATRIX:
                    MatrixController mctrl = new MatrixController();
                    mctrl.init();
                    dispatchMatrix(mctrl, cmd, args);
                    mctrl.close();
                    break;
            }
        } catch (Exception e) {
            System.err.println("[ERROR] " + e.getMessage());
            System.exit(1);
        }
        System.exit(0);
    }

    private static void dispatch(ZoneController ctrl, String cmd, String[] args) throws Exception {
        switch (cmd) {
            case CMD_ON:
                int brightness = args.length > 1 ? Integer.parseInt(args[1]) : 4000;
                ctrl.allOn(brightness);
                break;
            case CMD_OFF:
                ctrl.allOff();
                break;
            case CMD_PULSE:
                int b = args.length > 1 ? Integer.parseInt(args[1]) : 4000;
                int steps = args.length > 2 ? Integer.parseInt(args[2]) : 10;
                ctrl.pulse(b, steps);
                break;
            default:
                System.err.println("Unknown command: " + cmd);
                System.exit(1);
        }
    }

    private static void dispatchMatrix(MatrixController ctrl, String cmd, String[] args) throws Exception {
        switch (cmd) {
            case CMD_ON:
                int brightness = args.length > 1 ? Integer.parseInt(args[1]) : 200;
                ctrl.allOn(brightness);
                break;
            case CMD_OFF:
                ctrl.allOff();
                break;
            case CMD_PULSE:
                int b = args.length > 1 ? Integer.parseInt(args[1]) : 200;
                ctrl.pulse(b);
                break;
            default:
                System.err.println("Unknown command: " + cmd);
                System.exit(1);
        }
    }

    private static void printUsage() {
        System.err.println("Usage: GlyphHelper <cmd> [args]");
        System.err.println("  info               — print device codename and glyph type");
        System.err.println("  on [brightness]    — turn all zones on");
        System.err.println("  off                — turn all zones off");
        System.err.println("  pulse [brightness] [steps] — one pulse cycle");
    }
}
```

- [ ] **Step 4: Commit skeleton**

```bash
git add app/src/main/java/com/nothingctl/
git commit -m "feat: add GlyphHelper entry point, DeviceInfo, LightState skeleton"
```

---

### Task 4: Implement `ZoneController.java` (Glyph SDK)

**Spawn:** Coder agent (worktree isolation)

**Prerequisite:** Task 2 complete (SDK API names), Task 3 commit merged.

**Files:**
- Create: `app/src/main/java/com/nothingctl/ZoneController.java`

**Background:**
Fill in the exact SDK class names from the Task 2 Researcher findings. The skeleton below uses placeholder method names — the Coder must replace them with the actual SDK API.

```java
// Template — replace SDK calls with actual API from Researcher findings:
package com.nothingctl;

// import com.nothing.ketchum.GlyphManager;
// import com.nothing.ketchum.GlyphFrame;
// import com.nothing.ketchum.Common;

public class ZoneController {

    // TODO: declare GlyphManager instance from SDK

    public void init() throws Exception {
        // TODO: GlyphManager.getInstance() / init(context)
        // If Context needed: use ActivityThread.systemMain().getSystemContext()
        //   via reflection:
        //   Class<?> at = Class.forName("android.app.ActivityThread");
        //   Object thread = at.getMethod("systemMain").invoke(null);
        //   Context ctx = (Context) at.getMethod("getSystemContext").invoke(thread);
    }

    public void allOn(int brightness) throws Exception {
        // TODO: build GlyphFrame with all zones at brightness, call session.animate()
    }

    public void allOff() throws Exception {
        // TODO: GlyphFrame with all zones at 0
    }

    public void pulse(int maxBrightness, int steps) throws Exception {
        // TODO: loop through brightness curve, animate each step
        int[] curve = buildCurve(maxBrightness, steps);
        for (int b : curve) {
            allOn(b);
            Thread.sleep(150);
        }
        allOff();
    }

    public void close() {
        // TODO: session.closeSession() / GlyphManager.closeSession()
    }

    private int[] buildCurve(int max, int steps) {
        // sine-shaped curve up then back down
        int[] curve = new int[steps * 2];
        for (int i = 0; i < steps; i++) {
            double angle = Math.PI * i / steps;
            curve[i] = (int)(max * Math.sin(angle));
            curve[steps * 2 - 1 - i] = curve[i];
        }
        return curve;
    }
}
```

- [ ] **Step 1:** Replace all TODO sections with actual Nothing Glyph SDK calls based on Researcher findings from Task 2
- [ ] **Step 2:** Test on a connected Phone 1, 2, or 3a via:

```bash
# Build DEX
./gradlew fatDex

# Push and test
adb push build/outputs/dex/classes.dex /data/local/tmp/glyph-helper.dex
adb shell su -c 'app_process -cp /data/local/tmp/glyph-helper.dex / com.nothingctl.GlyphHelper info'
adb shell su -c 'app_process -cp /data/local/tmp/glyph-helper.dex / com.nothingctl.GlyphHelper on 3000'
# Observe: LEDs turn on
adb shell su -c 'app_process -cp /data/local/tmp/glyph-helper.dex / com.nothingctl.GlyphHelper off'
# Observe: LEDs turn off
adb shell su -c 'app_process -cp /data/local/tmp/glyph-helper.dex / com.nothingctl.GlyphHelper pulse 4000 10'
# Observe: LEDs pulse once
```

- [ ] **Step 3:** Commit

```bash
git add app/src/main/java/com/nothingctl/ZoneController.java
git commit -m "feat: implement ZoneController using Nothing Glyph SDK"
```

---

### Task 5: Implement `MatrixController.java` (GlyphMatrix SDK — Phone 3)

**Spawn:** Coder agent (worktree isolation)

**Prerequisite:** Task 2 complete. Can run in parallel with Task 4 if a Phone 3 is available for testing; otherwise implement and mark as "untested — no Phone 3 available."

**Files:**
- Create: `app/src/main/java/com/nothingctl/MatrixController.java`

The structure mirrors `ZoneController.java` but uses GlyphMatrix SDK classes. Fill in from Task 2 Researcher findings:

```java
package com.nothingctl;

// import com.nothing.ketchum.GlyphMatrixManager; // or whatever the class is
// import com.nothing.ketchum.GlyphMatrixFrame;

public class MatrixController {

    public void init() throws Exception {
        // TODO: GlyphMatrixManager.getInstance() / init
    }

    public void allOn(int brightness) throws Exception {
        // TODO: build a GlyphMatrixFrame with all pixels at brightness
    }

    public void allOff() throws Exception {
        // TODO: all pixels to 0
    }

    public void pulse(int maxBrightness) throws Exception {
        int[] curve = buildCurve(maxBrightness, 10);
        for (int b : curve) {
            allOn(b);
            Thread.sleep(150);
        }
        allOff();
    }

    public void close() {
        // TODO: close session
    }

    private int[] buildCurve(int max, int steps) {
        int[] curve = new int[steps * 2];
        for (int i = 0; i < steps; i++) {
            double angle = Math.PI * i / steps;
            curve[i] = (int)(max * Math.sin(angle));
            curve[steps * 2 - 1 - i] = curve[i];
        }
        return curve;
    }
}
```

- [ ] **Step 1:** Fill in MatrixController from Researcher findings
- [ ] **Step 2:** Commit with note if untested:

```bash
git commit -m "feat: implement MatrixController using GlyphMatrix SDK [untested — no Phone 3]"
```

---

### Task 6: CI — GitHub Actions build pipeline

**Spawn:** Coder agent

**Files:**
- Create: `.github/workflows/build.yml`

- [ ] **Step 1: Write build workflow**

```yaml
name: Build DEX

on:
  push:
    branches: [main]
  release:
    types: [created]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up JDK 11
        uses: actions/setup-java@v4
        with:
          java-version: '11'
          distribution: 'temurin'

      - name: Set up Android SDK
        uses: android-actions/setup-android@v3

      - name: Build fat DEX
        run: ./gradlew fatDex

      - name: Upload DEX artifact
        uses: actions/upload-artifact@v4
        with:
          name: glyph-helper-dex
          path: app/build/outputs/dex/classes.dex

      - name: Attach DEX to release
        if: github.event_name == 'release'
        uses: softprops/action-gh-release@v2
        with:
          files: app/build/outputs/dex/classes.dex
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2:** Push and verify CI passes

```bash
git add .github/workflows/build.yml
git commit -m "ci: add GitHub Actions build pipeline — DEX artifact on push + release attach"
git push
# Then: gh run watch --exit-status
```

---

### Task 7: nothingctl integration — embed DEX and call from feedback.go

**Spawn:** Coder agent (worktree isolation in `nothingctl` repo)

**Prerequisite:** Tasks 4–6 complete — a working DEX must exist.

**Context for Coder agent:**
- Working directory: `nothingctl/go/`
- The current feedback system lives in `go/internal/glyph/feedback.go`
- Phone 1 uses `writeSysfsLED()` (direct sysfs) — keep this path, the helper is the fallback
- New files go in `go/internal/glyph/`

**Files:**
- Create: `go/internal/glyph/helper.go`
- Create: `go/internal/glyph/helper_dex.go`
- Create: `go/internal/assets/glyph-helper.dex` ← copy the built DEX here
- Modify: `go/internal/glyph/feedback.go`

- [ ] **Step 1: Copy built DEX into nothingctl assets**

```bash
mkdir -p go/internal/assets
cp /path/to/nothingctl-glyph-helper/app/build/outputs/dex/classes.dex \
   go/internal/assets/glyph-helper.dex
```

- [ ] **Step 2: Write `helper_dex.go` (embed)**

```go
package glyph

import _ "embed"

//go:embed ../../internal/assets/glyph-helper.dex
var glyphHelperDex []byte
```

- [ ] **Step 3: Write `helper.go`**

```go
package glyph

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/Limplom/nothingctl/internal/adb"
)

const helperRemotePath = "/data/local/tmp/nothingctl-glyph-helper.dex"
const helperClass = "com.nothingctl.GlyphHelper"

// pushHelper writes the embedded DEX to a temp file and pushes it to the device.
// Returns a cleanup func that removes the remote file.
func pushHelper(serial string) (cleanup func(), err error) {
    tmp, err := os.CreateTemp("", "glyph-helper-*.dex")
    if err != nil {
        return nil, err
    }
    defer os.Remove(tmp.Name())

    if _, err := tmp.Write(glyphHelperDex); err != nil {
        return nil, err
    }
    tmp.Close()

    if err := adb.AdbPush(serial, tmp.Name(), helperRemotePath); err != nil {
        return nil, fmt.Errorf("push glyph helper: %w", err)
    }

    cleanup = func() {
        adb.Run([]string{"adb", "-s", serial, "shell",
            fmt.Sprintf("su -c 'rm -f %s'", helperRemotePath)})
    }
    return cleanup, nil
}

// runHelper runs a command on the remote glyph helper DEX.
// Requires root. Returns true on success.
func runHelper(serial, cmd string, extraArgs ...string) bool {
    args := append([]string{cmd}, extraArgs...)
    shellCmd := fmt.Sprintf(
        "su -c 'app_process -cp %s / %s %s'",
        helperRemotePath, helperClass,
        strings.Join(args, " "),
    )
    _, _, code := adb.Run([]string{"adb", "-s", serial, "shell", shellCmd})
    return code == 0
}

// HelperAvailable returns true if the device supports the glyph helper
// (i.e. app_process works and the helper responds to info).
func HelperAvailable(serial string) bool {
    cleanup, err := pushHelper(serial)
    if err != nil {
        return false
    }
    defer cleanup()
    return runHelper(serial, "info")
}

// HelperAllOn turns all Glyph zones on at the given brightness via the helper.
func HelperAllOn(serial string, brightness int) bool {
    return runHelper(serial, "on", fmt.Sprintf("%d", brightness))
}

// HelperAllOff turns all Glyph zones off via the helper.
func HelperAllOff(serial string) bool {
    return runHelper(serial, "off")
}

// HelperPulseOnce runs one pulse cycle (up + down) at the given brightness.
func HelperPulseOnce(serial string, brightness int) bool {
    return runHelper(serial, "pulse", fmt.Sprintf("%d", brightness))
}

// helperPath returns the remote path if the helper is already pushed.
func helperPushedPath() string {
    return helperRemotePath
}

// EnsureHelperPushed pushes the DEX once and returns a cleanup function.
// Callers that run multiple commands should push once and reuse.
func EnsureHelperPushed(serial string) (cleanup func(), ok bool) {
    cleanup, err := pushHelper(serial)
    if err != nil {
        return func() {}, false
    }
    return cleanup, true
}
```

- [ ] **Step 4: Modify `feedback.go` to use helper as fallback**

In `feedback.go`, the `writeAllBr` method currently only writes sysfs zones. Add a helper-based path for devices where sysfs returns nothing:

```go
// In the Feedback struct, add:
type Feedback struct {
    serial       string
    zones        []feedbackZone   // sysfs zones (Phone 1 only)
    useHelper    bool             // true if helper DEX should be used
    helperPushed bool
    helperCleanup func()
    // ... existing fields ...
}

// In NewFeedback(), after the existing sysfs zone check:
func NewFeedback(serial, codename string) *Feedback {
    // existing sysfs zone lookup ...

    // If no sysfs zones found, try helper
    if len(zones) == 0 {
        f.useHelper = adb.CheckAdbRoot(serial)
        // helper is pushed lazily on first use in Start()
    }
    return f
}
```

In the goroutine that runs the pulse loop, add the helper branch:

```go
// pulse step (inside the goroutine):
if f.useHelper {
    if !f.helperPushed {
        cleanup, ok := EnsureHelperPushed(f.serial)
        if ok {
            f.helperCleanup = cleanup
            f.helperPushed = true
        } else {
            f.useHelper = false
        }
    }
    if f.helperPushed {
        HelperPulseOnce(f.serial, feedbackBrightness)
    }
} else {
    f.writeAllBr(brightness) // existing sysfs path
}
```

In `Done()` / `Cancel()`, after existing cleanup:
```go
if f.helperCleanup != nil {
    HelperAllOff(f.serial)
    f.helperCleanup()
}
```

- [ ] **Step 5: Build and verify compilation**

```bash
export PATH="$PATH:/c/Program Files/Go/bin"
go build ./...
# Must compile cleanly with no errors
```

- [ ] **Step 6: Live test — backup on 3a Lite (or any non-Phone-1 device)**

```bash
go run ./cmd/nothingctl/ backup
# Expected: LEDs pulse during backup, turn off sequentially when done
```

If no device available, document explicitly in commit message.

- [ ] **Step 7: Commit**

```bash
git add go/internal/glyph/helper.go \
        go/internal/glyph/helper_dex.go \
        go/internal/assets/glyph-helper.dex \
        go/internal/glyph/feedback.go
git commit -m "feat: embed glyph-helper DEX, add helper-based LED feedback for non-sysfs devices"
```

---

### Task 8: Review

**Spawn:** `superpowers:code-reviewer` agent

**Scope:** Review Tasks 3–7 together. The reviewer should check:
- `GlyphHelper.java` exit codes are consistent (0=success, 1=error, 2=unsupported device)
- The `pushHelper` + `runHelper` Go functions handle ADB failures gracefully — a missing or broken helper must never crash nothingctl, only silently skip LED feedback
- `helperCleanup` is always called even if feedback is cancelled mid-pulse
- The embedded DEX path (`go/internal/assets/`) is correct relative to the `//go:embed` directive
- No race conditions in `feedback.go` around `helperPushed`

---

## Known Risks

| Risk | Mitigation |
|------|------------|
| SELinux blocks `app_process` with custom DEX on some Nothing OS versions | `HelperAvailable()` detects this on first call; feedback silently skips |
| SDK requires Activity context that can't be obtained via reflection | ZoneController.init() fails → caught by try/catch → exit(1) → Go side sees failure |
| GlyphMatrix SDK coordinates unknown at plan time | Task 2 Researcher must find them before Task 5 starts |
| Phone 3 codename unknown | Task 2 Researcher confirms; add to `DeviceInfo.java` in Task 3 |
| DEX size increases nothingctl binary significantly | Acceptable: typical DEX for this use case is 50–150 KB |

---

## README for new repo

The README for `nothingctl-glyph-helper` should include:
- One-paragraph explanation of what it is and why it exists (link to nothingctl)
- Supported devices table
- How to build: `./gradlew fatDex`
- How to test manually via `adb shell app_process`
- Note that this is not a standalone app — it's a nothingctl component

# SchoolSync Modules Implementation Status

Date: 2026-02-21

## 1) Viral Client Distribution (VCD) - Implemented

### Backend (Go) - Implemented

- VCD API contracts and session models:
  - `main.go:159` (`VCDStartRequest`)
  - `main.go:167` (`VCDShareSession`)
- VCD endpoints registered:
  - `main.go:277` (`/vcd/start`)
  - `main.go:278` (`/vcd/status`)
  - `main.go:279` (`/vcd/stop`)
- Core VCD logic:
  - `main.go:450` (`handleVCDStart`)
  - `main.go:570` (`handleVCDStatus`)
  - `main.go:588` (`handleVCDStop`)
  - `main.go:604` (`stopVCDSession`)
  - `main.go:629` (`buildVCDQRPayload`)

Delivered behavior:
- Starts temporary local HTTP artifact server on random free port.
- Serves tokenized download URL.
- Builds QR payload with `downloadUrl`, `serverIp`, `port`, `ssid`, `password`, and `bluetoothFallback`.
- Exposes active share session status.
- Stops session manually and auto-expires with TTL.
- Supports optional `artifactPath` so sender can share APK instead of only current executable.

### Frontend (React) - Implemented

- VCD API methods:
  - `frontend/src/utils/api.js:198` (`startViralShare`)
  - `frontend/src/utils/api.js:214` (`getViralShareStatus`)
  - `frontend/src/utils/api.js:222` (`stopViralShare`)
  - `frontend/src/utils/api.js:202` (`artifactPath` parameter)
- Dashboard UX for VCD:
  - `frontend/src/pages/Dashboard.jsx:224` (Share SchoolSync section)
  - `frontend/src/pages/Dashboard.jsx:126` (`handleStartShare`)
  - `frontend/src/pages/Dashboard.jsx:146` (`handleStopShare`)
  - `frontend/src/pages/Dashboard.jsx:240` (Artifact Path input)
  - `frontend/src/pages/Dashboard.jsx:248` (Bluetooth OPP fallback toggle)
  - `frontend/src/pages/Dashboard.jsx:270` (QR render from backend payload)

Delivered behavior:
- User can start/stop sharing from dashboard.
- User can set SSID/password and optional APK path.
- QR is shown for receiver handshake and local download.
- Session metadata displayed (URL/size/expiry).

## 2) Android Mobile Bridge Module - Implemented (Starter App)

Project:
- `mobile-bridge/schoolsync-android-bridge`

### Android app scaffolding
- `mobile-bridge/schoolsync-android-bridge/settings.gradle`
- `mobile-bridge/schoolsync-android-bridge/build.gradle`
- `mobile-bridge/schoolsync-android-bridge/app/build.gradle`
- `mobile-bridge/schoolsync-android-bridge/app/src/main/AndroidManifest.xml`
- `mobile-bridge/schoolsync-android-bridge/app/src/main/res/layout/activity_main.xml`
- `mobile-bridge/schoolsync-android-bridge/app/src/main/res/xml/file_paths.xml`

### Receiver flow implementation
- `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/MainActivity.kt:22` (Main activity)
- `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/MainActivity.kt:35` (QR scan registration)
- `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/MainActivity.kt:100` (VCD payload parsing)
- `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/MainActivity.kt:107` (Wi-Fi Direct start)
- `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/MainActivity.kt:111` (HTTP artifact download)
- `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/MainActivity.kt:72` (APK install intent)

### Bridge classes
- Payload parser:
  - `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/vcd/VcdPayload.kt`
- Wi-Fi Direct manager:
  - `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/vcd/WifiDirectVcdManager.kt:17`
  - `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/vcd/WifiDirectVcdManager.kt:71`
  - `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/vcd/WifiDirectVcdManager.kt:91`
  - `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/vcd/WifiDirectVcdManager.kt:100`
- Downloader:
  - `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/vcd/VcdDownloader.kt`
- Installer helper:
  - `mobile-bridge/schoolsync-android-bridge/app/src/main/java/com/schoolsync/vcdbridge/vcd/ApkInstaller.kt`

### Permissions and provider
- Permissions listed:
  - `mobile-bridge/schoolsync-android-bridge/app/src/main/AndroidManifest.xml:4`
  - `mobile-bridge/schoolsync-android-bridge/app/src/main/AndroidManifest.xml:14`
- FileProvider:
  - `mobile-bridge/schoolsync-android-bridge/app/src/main/AndroidManifest.xml:23`

## 3) Status Summary

- VCD backend: Implemented
- VCD frontend share flow: Implemented
- Android bridge starter module: Implemented
- Bluetooth OPP runtime transfer workflow: Partially implemented (fallback flag + guidance present; full OPP transfer service still to be added if required)

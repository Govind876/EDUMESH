# SchoolSync Android Bridge Module

Import this folder into Android Studio as a project, or copy the `app` module into an existing Android workspace.

## Features wired

- QR scan via ZXing (`zxing-android-embedded`)
- VCD payload parsing (`schoolsync-vcd` JSON)
- Wi-Fi Direct discovery/connect skeleton
- Local HTTP download of APK/binary
- APK install launch using `FileProvider`

## Quick run

1. Open `mobile-bridge/schoolsync-android-bridge` in Android Studio.
2. Build and run on a physical Android device.
3. In your SchoolSync web dashboard, press `Share SchoolSync` and set `Artifact Path` to your APK path.
4. In Android app, tap `Scan QR`, then `Start Receive`.
5. After download, tap `Install Downloaded APK`.

## Runtime permissions

The sample requests camera/location/nearby/bluetooth permissions at startup for a simple demo flow.
